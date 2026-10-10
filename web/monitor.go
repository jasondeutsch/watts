package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jasondeutsch/watts/internal/application"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"go.temporal.io/sdk/client"
)

//go:embed assets/*
var assets embed.FS

type taskSummary struct {
	Path string `json:"path"`
	Name string `json:"name"`
}
type logTail struct {
	Name string `json:"name"`
	Text string `json:"text"`
}
type snapshot struct {
	Task       string                   `json:"task"`
	Definition orchestration.Definition `json:"definition"`
	State      orchestration.State      `json:"state"`
	WorkflowID string                   `json:"workflow_id,omitempty"`
	RunID      string                   `json:"run_id,omitempty"`
	Pinned     bool                     `json:"pinned"`
	Error      string                   `json:"error,omitempty"`
	Logs       []logTail                `json:"logs"`
	Updated    time.Time                `json:"updated"`
}
type monitor struct {
	app     *application.Service
	mu      sync.Mutex
	clients map[string]client.Client
	query   func(context.Context, application.WorkflowBinding) (orchestration.State, error)
}

func newMonitor(app *application.Service) *monitor {
	m := &monitor{app: app, clients: make(map[string]client.Client)}
	m.query = m.queryState
	return m
}
func (m *monitor) close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.Close()
	}
}
func (m *monitor) queryState(ctx context.Context, binding application.WorkflowBinding) (orchestration.State, error) {
	key := binding.Settings.Address + "\x00" + binding.Settings.Namespace
	m.mu.Lock()
	c := m.clients[key]
	if c == nil {
		var err error
		c, err = client.NewLazyClient(client.Options{HostPort: binding.Settings.Address, Namespace: binding.Settings.Namespace})
		if err != nil {
			m.mu.Unlock()
			return orchestration.State{}, err
		}
		m.clients[key] = c
	}
	m.mu.Unlock()
	return application.QueryTemporalWorkflowState(ctx, c, binding)
}

// OpenRoot confines all monitor reads, including symlinks, to the project.
func (m *monitor) tasks() ([]taskSummary, error) {
	cfg, err := m.app.LoadConfig()
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(m.app.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.Open(cfg.TasksDir)
	if errors.Is(err, os.ErrNotExist) {
		return []taskSummary{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	tasks := []taskSummary{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tasks = append(tasks, taskSummary{Path: filepath.ToSlash(filepath.Join(cfg.TasksDir, entry.Name())), Name: entry.Name()})
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	return tasks, nil
}
func (m *monitor) readSnapshot(ctx context.Context, task string) snapshot {
	s := snapshot{Task: task, Updated: time.Now().UTC(), Logs: []logTail{}, State: orchestration.State{Status: "unavailable"}}
	root, err := os.OpenRoot(filepath.Join(m.app.Root, filepath.FromSlash(task)))
	if err != nil {
		s.Error = err.Error()
		return s
	}
	defer root.Close()
	var binding application.WorkflowBinding
	data, err := root.ReadFile(".watts-state/temporal.json")
	if err == nil {
		err = json.Unmarshal(data, &binding)
		if err == nil && (binding.Input.Task != task || binding.WorkflowID == "") {
			err = errors.New("Temporal task record does not match this task")
		}
		if err == nil {
			s.Definition = binding.Input.Definition
			s.Pinned = true
			s.WorkflowID = binding.WorkflowID
			s.RunID = binding.RunID
		}
	} else if errors.Is(err, os.ErrNotExist) {
		data, err = root.ReadFile("workflow.yaml")
		if err == nil {
			document, decodeErr := manifest.Decode(data, manifest.Workflow, orchestration.Definition{})
			err = decodeErr
			s.Definition = document.Spec
		}
	}
	if err == nil {
		err = s.Definition.Validate()
	}
	if err != nil {
		s.Error = err.Error()
		return s
	}
	if binding.RunID == "" {
		s.State.Status = "not_started"
		for _, step := range s.Definition.Steps {
			s.State.Stages = append(s.State.Stages, orchestration.Stage{Name: step.Name, Status: "pending"})
		}
	} else {
		queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		s.State, err = m.query(queryCtx, binding)
		cancel()
		if err != nil {
			s.State = orchestration.State{Status: "unavailable"}
			s.Error = err.Error()
		}
	}
	// Keep previous attempts visible, including while Temporal is unavailable.
	for _, step := range s.Definition.Steps {
		matches, _ := fs.Glob(root.FS(), ".watts-state/logs/"+step.Name+"-*.log")
		for _, name := range matches {
			text, err := tail(root, name)
			if err != nil {
				s.Logs = append(s.Logs, logTail{Name: filepath.Base(name), Text: "Cannot read log: " + err.Error()})
				continue
			}
			s.Logs = append(s.Logs, logTail{Name: filepath.Base(name), Text: text})
		}
	}
	return s
}
func tail(root *os.Root, name string) (string, error) {
	f, err := root.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	const limit = 32 * 1024
	start := max(int64(0), info.Size()-limit)
	if _, err = f.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if start > 0 {
		if newline := strings.IndexByte(string(data), '\n'); newline >= 0 {
			data = data[newline+1:]
		}
	}
	return string(data), err
}
func (m *monitor) routes() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "assets")
	mux.Handle("GET /", http.FileServer(http.FS(static)))
	mux.HandleFunc("GET /api/tasks", func(w http.ResponseWriter, r *http.Request) {
		tasks, err := m.tasks()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(struct {
			Project string        `json:"project"`
			Tasks   []taskSummary `json:"tasks"`
		}{m.app.Root, tasks})
	})
	mux.HandleFunc("GET /api/events", m.events)
	mux.HandleFunc("POST /api/retry", m.retry)
	return mux
}

func (m *monitor) retry(w http.ResponseWriter, r *http.Request) {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Host != r.Host || (origin.Scheme != "http" && origin.Scheme != "https") {
		http.Error(w, "retry requires a same-origin request", http.StatusForbidden)
		return
	}
	task := r.URL.Query().Get("task")
	tasks, err := m.tasks()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	found := false
	for _, candidate := range tasks {
		if candidate.Path == task {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "unknown task", 404)
		return
	}
	state := m.readSnapshot(r.Context(), task)
	if state.Error != "" || state.State.Status != "waiting_retry" {
		http.Error(w, "only a task waiting for retry can be retried", http.StatusConflict)
		return
	}
	config, err := m.app.LoadConfig()
	if err == nil {
		err = m.app.RunTask(config, task, &application.RunOptions{Force: true})
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
func (m *monitor) events(w http.ResponseWriter, r *http.Request) {
	tasks, err := m.tasks()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	task := r.URL.Query().Get("task")
	found := false
	for _, candidate := range tasks {
		if candidate.Path == task {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "unknown task", 404)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		data, err := json.Marshal(m.readSnapshot(r.Context(), task))
		if err != nil {
			return
		}
		if _, err = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", data); err != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
