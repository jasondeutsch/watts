// Package workerstate owns host-local worker paths and disposable execution state.
// Authored packages and durable task artifacts never live under these paths.
package workerstate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/internal/storage"
)

type Store struct{ Root string }
type Project struct {
	Root     string `json:"root"`
	TasksDir string `json:"tasks_dir"`
}

func Default() (Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Store{}, fmt.Errorf("host HOME is unavailable: %w", err)
	}
	home = filepath.Clean(home)
	if !filepath.IsAbs(home) || home == string(filepath.Separator) {
		return Store{}, fmt.Errorf("host HOME %q must be an absolute user directory, not the filesystem root", home)
	}
	if home == "/runtime" || strings.HasPrefix(home, "/runtime/") {
		return Store{}, fmt.Errorf("host HOME %q uses the container-only /runtime path; start wctl with the host user's HOME (container HOME is separate)", home)
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		return Store{}, storage.DescribeHostError(err)
	}
	if resolved == string(filepath.Separator) || resolved == "/runtime" || strings.HasPrefix(resolved, "/runtime/") {
		return Store{}, fmt.Errorf("host HOME %q resolves to %q; use the host user's home directory, separate from container paths", home, resolved)
	}
	info, err := os.Stat(home)
	if err != nil {
		return Store{}, storage.DescribeHostError(err)
	}
	if !info.IsDir() {
		return Store{}, fmt.Errorf("host HOME %q must be a directory", home)
	}
	return Store{Root: filepath.Join(home, ".watts")}, nil
}
func key(value string) string            { sum := sha256.Sum256([]byte(value)); return hex.EncodeToString(sum[:]) }
func (store Store) SettingsPath() string { return filepath.Join(store.Root, "config", "settings.json") }
func (store Store) ProjectDir(project string) string {
	return filepath.Join(store.Root, "runtime", "projects", key(projectIdentity(project)))
}
func (store Store) WorkerDir(project string) string {
	return filepath.Join(store.ProjectDir(project), "worker")
}
func (store Store) TaskDir(project, task string) string {
	return filepath.Join(store.ProjectDir(project), "tasks", key(task))
}
func (store Store) ExecutionDir(project, task, execution string) string {
	return filepath.Join(store.TaskDir(project, task), "executions", key(execution))
}
func (store Store) AttemptDir(project, task, execution, attempt string) string {
	return filepath.Join(store.ExecutionDir(project, task, execution), "attempts", key(attempt))
}
func (store Store) RegisterProject(project, tasksDir string) error {
	canonical, err := canonicalProject(project)
	if err != nil {
		return err
	}
	if err = store.CheckDirectory(store.ProjectDir(canonical)); err != nil {
		return err
	}
	return writeJSON(filepath.Join(store.ProjectDir(canonical), "project.json"), Project{Root: canonical, TasksDir: tasksDir})
}
func (store Store) HasProject(project string) bool {
	var record Project
	return readJSON(filepath.Join(store.ProjectDir(project), "project.json"), &record) == nil && record.Root == projectIdentity(project)
}
func readJSON(path string, target any) (err error) {
	defer func() { err = storage.DescribeHostError(err) }()
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("worker record must be a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("worker record must contain one JSON document")
	}
	return nil
}
func writeJSON(path string, value any) (err error) {
	defer func() { err = storage.DescribeHostError(err) }()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(append(data, '\n'))
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
func canonicalProject(project string) (string, error) {
	absolute, err := filepath.Abs(project)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("project root: %w", err)
	}
	return resolved, nil
}

// CheckDirectory rejects redirected state directories before touching runtime
// files. Workload mounts never include these bookkeeping parents.
func (store Store) CheckDirectory(directory string) (err error) {
	defer func() { err = storage.DescribeHostError(err) }()
	if !filepath.IsAbs(store.Root) {
		return fmt.Errorf("worker state root must be absolute")
	}
	relative, err := filepath.Rel(store.Root, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("worker path is outside its state root")
	}
	current := store.Root
	parts := []string{""}
	if relative != "." {
		parts = append(parts, strings.Split(relative, string(filepath.Separator))...)
	}
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("worker state directory must not be a symlink: %s", current)
		}
	}
	return nil
}

func projectIdentity(project string) string {
	if resolved, err := canonicalProject(project); err == nil {
		return resolved
	}
	absolute, err := filepath.Abs(project)
	if err == nil {
		return absolute
	}
	return filepath.Clean(project)
}
