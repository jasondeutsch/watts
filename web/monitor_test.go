package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasondeutsch/watts/internal/application"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
)

func TestRetryRejectsCrossOriginAndUnstartedTasks(t *testing.T) {
	m := fixture(t)
	for _, test := range []struct {
		origin string
		code   int
	}{
		{"http://another-site.example", http.StatusForbidden},
		{"http://example.com", http.StatusConflict},
	} {
		request := httptest.NewRequest("POST", "http://example.com/api/retry?task=tasks/sample", nil)
		request.Header.Set("Origin", test.origin)
		response := httptest.NewRecorder()
		m.routes().ServeHTTP(response, request)
		require.Equal(t, test.code, response.Code)
	}
}

func TestStreamDeliversNewLogOutput(t *testing.T) {
	m := fixture(t)
	logDir := filepath.Join(m.app.StateDir("tasks/sample"), "logs")
	require.NoError(t, os.MkdirAll(logDir, 0700))
	logFile := filepath.Join(logDir, "coding-1.log")
	require.NoError(t, os.WriteFile(logFile, []byte("first output"), 0600))
	server := httptest.NewServer(m.routes())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/events?task=tasks/sample", nil)
	require.NoError(t, err)
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	nextSnapshot := func() string {
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "data: ") {
				return scanner.Text()
			}
		}
		require.NoError(t, scanner.Err())
		t.Fatal("stream ended before the next snapshot")
		return ""
	}
	require.Contains(t, nextSnapshot(), "first output")
	require.NoError(t, os.WriteFile(logFile, []byte("new output"), 0600))
	require.Contains(t, nextSnapshot(), "new output")
}

func fixture(t *testing.T) *monitor {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "watts.yaml"), []byte("kind: Project\nschema_version: 1\nname: test\nspec:\n  version: 1\n  tasks_dir: tasks\n"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tasks", "sample"), 0700))
	definition := orchestration.Definition{Version: 1, Steps: []orchestration.Step{{Name: "coding", Human: true, Inputs: []string{"SPEC.md"}}}}
	data, err := manifest.Encode(manifest.Workflow, "sample", definition)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "tasks", "sample", "workflow.yaml"), data, 0600))
	m := newMonitor(&application.Service{Root: root})
	t.Cleanup(m.close)
	return m
}

func TestCatalogIncludesIncompleteTasks(t *testing.T) {
	m := fixture(t)
	require.NoError(t, os.MkdirAll(filepath.Join(m.app.Root, "tasks", "incomplete"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(m.app.Root, "tasks", "not-a-task"), nil, 0600))
	tasks, err := m.tasks()
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	require.Equal(t, "incomplete", tasks[0].Name)
	require.Contains(t, m.readSnapshot(context.Background(), tasks[0].Path).Error, "workflow.yaml")
}

func TestPinnedWorkflowAndOfflineLogs(t *testing.T) {
	m := fixture(t)
	stateDir := m.app.StateDir("tasks/sample")
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, "logs"), 0700))
	binding := application.WorkflowBinding{WorkflowID: "workflow", RunID: "run", Input: orchestration.Input{Task: "tasks/sample", Definition: orchestration.Definition{Version: 1, Steps: []orchestration.Step{{Name: "qa", Human: true, Inputs: []string{"SPEC.md"}}}}}}
	data, err := json.Marshal(binding)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "temporal.json"), data, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "logs", "qa-1.log"), []byte("checking spec\n"), 0600))
	m.query = func(context.Context, application.WorkflowBinding) (orchestration.State, error) {
		return orchestration.State{}, errors.New("Temporal offline")
	}
	snapshot := m.readSnapshot(context.Background(), "tasks/sample")
	require.True(t, snapshot.Pinned)
	require.Equal(t, "qa", snapshot.Definition.Steps[0].Name)
	require.Equal(t, "unavailable", snapshot.State.Status)
	require.Equal(t, "Temporal offline", snapshot.Error)
	require.Equal(t, "checking spec\n", snapshot.Logs[0].Text)
}

func TestRoutesAndStream(t *testing.T) {
	m := fixture(t)
	for _, path := range []string{"/", "/app.js", "/style.css", "/api/tasks"} {
		response := httptest.NewRecorder()
		m.routes().ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 200, response.Code, path)
	}
	response := httptest.NewRecorder()
	m.routes().ServeHTTP(response, httptest.NewRequest("GET", "/api/events?task=../watts.yaml", nil))
	require.Equal(t, 404, response.Code)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response = httptest.NewRecorder()
	m.routes().ServeHTTP(response, httptest.NewRequest("GET", "/api/events?task=tasks/sample", nil).WithContext(ctx))
	require.Contains(t, response.Body.String(), "event: snapshot")
	require.Contains(t, response.Body.String(), `"status":"not_started"`)
}

func TestLogTailBoundedAndConfined(t *testing.T) {
	m := fixture(t)
	root, err := os.OpenRoot(m.app.Root)
	require.NoError(t, err)
	defer root.Close()
	require.NoError(t, os.WriteFile(filepath.Join(m.app.Root, "large.log"), []byte(strings.Repeat("line\n", 10000)), 0600))
	text, err := tail(root, "large.log")
	require.NoError(t, err)
	require.LessOrEqual(t, len(text), 32*1024)
	outside := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(m.app.Root, "escape.log")))
	_, err = tail(root, "escape.log")
	require.Error(t, err)
}
