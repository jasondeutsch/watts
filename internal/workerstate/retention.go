package workerstate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const LastRun = "last-run"
const KeepAll = "all"

type executionRecord struct {
	ID        string    `json:"id"`
	Ordinal   uint64    `json:"ordinal"`
	StartedAt time.Time `json:"started_at"`
	Terminal  bool      `json:"terminal"`
}
type attemptRecord struct {
	EvidenceReady bool `json:"evidence_ready"`
}
type Lease struct {
	file    *os.File
	capture string
}

func locked(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}
func unlock(file *os.File)  { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN); _ = file.Close() }
func (lease *Lease) Close() { unlock(lease.file) }
func (lease *Lease) RetainEvidence() error {
	return writeJSON(lease.capture, attemptRecord{EvidenceReady: true})
}

// Begin protects an execution from cleanup. A durable ordinal records execution
// order; restarting an older execution does not make it the latest one.
func (store Store) Begin(project, task, execution, attempt string) (*Lease, error) {
	if err := store.CheckDirectory(store.TaskDir(project, task)); err != nil {
		return nil, err
	}
	guard, err := locked(filepath.Join(store.TaskDir(project, task), "retention.lock"))
	if err != nil {
		return nil, err
	}
	defer unlock(guard)
	if err = writeJSON(filepath.Join(store.TaskDir(project, task), "task.json"), struct {
		Task string `json:"task"`
	}{task}); err != nil {
		return nil, err
	}
	directory := store.ExecutionDir(project, task, execution)
	if err = store.CheckDirectory(directory); err != nil {
		return nil, err
	}
	file, err := locked(filepath.Join(directory, "execution.lock"))
	if err != nil {
		return nil, err
	}
	lease := &Lease{file: file, capture: filepath.Join(directory, "captures", key(attempt)+".json")}
	var record executionRecord
	err = readJSON(filepath.Join(directory, "execution.json"), &record)
	if errors.Is(err, os.ErrNotExist) {
		var ordinal uint64
		ordinal, err = store.nextOrdinal(project, task, execution)
		if err == nil {
			err = writeJSON(filepath.Join(directory, "execution.json"), executionRecord{ID: execution, Ordinal: ordinal, StartedAt: time.Now().UTC()})
		}
	} else if err == nil && (record.ID != execution || record.Ordinal == 0) {
		err = fmt.Errorf("execution record does not match its identity")
	}
	if err != nil {
		lease.Close()
		return nil, err
	}
	if err = writeJSON(lease.capture, attemptRecord{}); err != nil {
		lease.Close()
		return nil, err
	}
	return lease, nil
}

// Complete is called only after the terminal task report has been persisted.
// An absent local execution is normal for workflows with only approvals/events.
func (store Store) Complete(project, task, execution string) error {
	if err := store.CheckDirectory(store.TaskDir(project, task)); err != nil {
		return err
	}
	guard, err := locked(filepath.Join(store.TaskDir(project, task), "retention.lock"))
	if err != nil {
		return err
	}
	defer unlock(guard)
	directory := store.ExecutionDir(project, task, execution)
	if err = store.CheckDirectory(directory); err != nil {
		return err
	}
	name := filepath.Join(directory, "execution.json")
	var record executionRecord
	if err = readJSON(name, &record); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	record.Terminal = true
	return writeJSON(name, record)
}

// Prune only removes known, terminal executions with retained attempt evidence.
// Malformed records, active leases and uncertain captures are preserved.
func (store Store) Prune(project, task, policy string) error {
	if policy == KeepAll {
		return nil
	}
	if policy != LastRun {
		return fmt.Errorf("unknown runtime retention policy %q", policy)
	}
	if err := store.CheckDirectory(store.TaskDir(project, task)); err != nil {
		return err
	}
	guard, err := locked(filepath.Join(store.TaskDir(project, task), "retention.lock"))
	if err != nil {
		return err
	}
	defer unlock(guard)
	directory := filepath.Join(store.TaskDir(project, task), "executions")
	if err := store.CheckDirectory(directory); err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	removeRoot, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer removeRoot.Close()
	records := map[string]executionRecord{}
	var latest uint64
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		var record executionRecord
		if readJSON(filepath.Join(directory, entry.Name(), "execution.json"), &record) != nil || entry.Name() != key(record.ID) || record.Ordinal == 0 {
			continue
		}
		records[entry.Name()] = record
		if record.Ordinal > latest {
			latest = record.Ordinal
		}
	}
	for name, record := range records {
		if record.Ordinal == latest || !record.Terminal {
			continue
		}
		executionDir := filepath.Join(directory, name)
		lease, err := locked(filepath.Join(executionDir, "execution.lock"))
		if errors.Is(err, syscall.EWOULDBLOCK) {
			continue
		}
		if err != nil {
			return err
		}
		safe := capturesReady(executionDir)
		if safe {
			err = removeRoot.RemoveAll(name)
		}
		unlock(lease)
		if err != nil {
			return err
		}
	}
	return nil
}
func capturesReady(directory string) bool {
	path := filepath.Join(directory, "captures")
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	captures, err := os.ReadDir(path)
	if err != nil || len(captures) == 0 {
		return false
	}
	for _, capture := range captures {
		var record attemptRecord
		if !capture.Type().IsRegular() || readJSON(filepath.Join(directory, "captures", capture.Name()), &record) != nil || !record.EvidenceReady {
			return false
		}
	}
	return true
}

// RecordEvidence is separate from release so the worker can sync its durable
// attempt log before declaring the disposable candidate eligible for cleanup.
func (store Store) RecordEvidence(project, task, execution, attempt string) error {
	if err := store.CheckDirectory(store.TaskDir(project, task)); err != nil {
		return err
	}
	guard, err := locked(filepath.Join(store.TaskDir(project, task), "retention.lock"))
	if err != nil {
		return err
	}
	defer unlock(guard)
	directory := store.ExecutionDir(project, task, execution)
	if err = store.CheckDirectory(directory); err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(directory, "execution.json")); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return writeJSON(filepath.Join(directory, "captures", key(attempt)+".json"), attemptRecord{EvidenceReady: true})
}
func (store Store) PruneProject(project, policy string) error {
	directory := filepath.Join(store.ProjectDir(project), "tasks")
	if err := store.CheckDirectory(directory); err != nil {
		return err
	}
	tasks, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if !task.IsDir() {
			continue
		}
		var record struct {
			Task string `json:"task"`
		}
		if readJSON(filepath.Join(directory, task.Name(), "task.json"), &record) != nil || task.Name() != key(record.Task) {
			continue
		}
		if err = store.Prune(project, record.Task, policy); err != nil && !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
	}
	return nil
}

// nextOrdinal runs under the task retention lock. A missing counter is recovered
// only from readable records; an ambiguous order must never authorize deletion.
func (store Store) nextOrdinal(project, task, current string) (uint64, error) {
	var ordinal uint64
	counter := filepath.Join(store.TaskDir(project, task), "ordinal.json")
	err := readJSON(counter, &ordinal)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return 0, err
	}
	directory := filepath.Join(store.TaskDir(project, task), "executions")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == key(current) {
			continue
		}
		var previous executionRecord
		if readJSON(filepath.Join(directory, entry.Name(), "execution.json"), &previous) != nil || entry.Name() != key(previous.ID) || previous.Ordinal == 0 {
			if missing {
				return 0, fmt.Errorf("cannot recover execution ordering with unreadable runtime records")
			}
			continue
		}
		if previous.Ordinal > ordinal {
			ordinal = previous.Ordinal
		}
	}
	if ordinal == ^uint64(0) {
		return 0, fmt.Errorf("execution ordinal exhausted")
	}
	ordinal++
	if err = writeJSON(counter, ordinal); err != nil {
		return 0, err
	}
	return ordinal, nil
}
