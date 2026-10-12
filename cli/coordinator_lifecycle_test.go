package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasondeutsch/watts/internal/application"
	"github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/containerengine"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// Deterministic workloads exercise coordination through the public CLI without
// hosted inference. Real agent behavior is a separate eval, not simulated here.
func TestCoordinatorCLILifecycle(t *testing.T) {
	if os.Getenv("WATTS_TEMPORAL_INTEGRATION") != "1" {
		t.Skip("set WATTS_TEMPORAL_INTEGRATION=1 with Docker and ubuntu:latest available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cache := filepath.Join(os.TempDir(), "watts-temporal-cli")
	require.NoError(t, os.MkdirAll(cache, 0700))
	server, err := testsuite.StartDevServer(ctx, testsuite.DevServerOptions{CachedDownload: testsuite.CachedDownload{DestDir: cache}, DBFilename: filepath.Join(t.TempDir(), "temporal.db"), Stdout: io.Discard, Stderr: io.Discard})
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Stop() })
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Chdir(project)
	app := &application.Service{Root: project, WorkingDirectory: project, Output: io.Discard, ErrorOutput: io.Discard}
	settings := config.Default()
	settings.Temporal = &config.TemporalSettings{Address: server.FrontendHostPort()}
	require.NoError(t, app.SaveConfig(settings))
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	t.Cleanup(func() {
		engine, err := containerengine.Connect()
		if err == nil {
			defer engine.Close()
			records, _ := filepath.Glob(filepath.Join(project, "tasks", "*", ".watts-state", "steps", "*", "*-container.json"))
			for _, record := range records {
				data, err := os.ReadFile(record)
				if err != nil {
					continue
				}
				var exit containerengine.Exit
				if json.Unmarshal(data, &exit) == nil && exit.ContainerID != "" {
					_ = engine.Remove(context.Background(), exit.ContainerID)
				}
			}
		}
		_ = os.RemoveAll(filepath.Join(home, ".watts", "runtime", storage.Digest([]byte(project))[:16]))
	})
	queue := app.ResolvedTemporalSettings(settings).TaskQueue
	worker := app.NewTemporalWorker(server.Client(), queue)
	require.NoError(t, worker.Start())
	t.Cleanup(func() { worker.Stop() })
	procedure := func(name, script string) orchestration.Step {
		return orchestration.Step{Name: name, Container: &workload.Spec{Image: "ubuntu:latest", Command: []string{"sh", "-c", script}, Workspace: "write", Result: "exit-code"}}
	}
	draft := procedure("specification", `printf '# Refined specification\n\nWrite a project-root greeting.\n' > /outputs/task/SPEC.md`)
	draft.Inputs, draft.Outputs = []string{"SPEC.md"}, []string{"SPEC.md"}
	build := procedure("build", `if grep -m 1 '"attempt":' /inputs/input.json | grep -Eq '"attempt":[[:space:]]*1[,}]'; then echo deliberate-build-failure >&2; exit 7; fi; printf 'hello\n' > /workspace/greeting.txt`)
	review := procedure("review", `test "$(cat /workspace/greeting.txt)" = hello; printf '# Review\n\nGreeting verified.\n' > /outputs/task/REVIEW.md`)
	review.Outputs = []string{"REVIEW.md"}
	definition := orchestration.Definition{Templates: map[string]string{"SPEC.md": "# Initial request\n"}, Steps: []orchestration.Step{draft, {Name: "spec-review", Human: true, Inputs: []string{"SPEC.md"}, Transitions: map[string]string{"rejected": "specification", "approved": "build"}}, build, review}}
	pkg := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(pkg, "templates"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(pkg, "watts.yaml"), []byte("name: coordinator-fixture\nversion: 0.1.0\ntemplates: templates\n"), 0600))
	data, err := manifest.EncodeWorkflow("coordinator-fixture", definition)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pkg, "templates", "workflow.yaml.gotmpl"), data, 0600))
	create := func(slug string) string {
		t.Helper()
		code, out, errOut := command(t, "task", "new", slug, "--package", filepath.Join(pkg, "watts.yaml"))
		require.Zero(t, code, errOut)
		fields := strings.Fields(out)
		require.GreaterOrEqual(t, len(fields), 2)
		task := strings.TrimSuffix(fields[1], ".")
		require.FileExists(t, filepath.Join(project, task, "SPEC.md"))
		require.NoDirExists(t, filepath.Join(pkg, "tasks"), "external package must not become the application root")
		return task
	}
	status := func(task string) (application.TemporalTaskStatus, int, string) {
		t.Helper()
		code, out, errOut := command(t, "task", "status", task, "--json")
		var view application.TemporalTaskStatus
		require.NoError(t, json.Unmarshal([]byte(out), &view), "%s", errOut)
		return view, code, out
	}
	await := func(task, state string) (application.TemporalTaskStatus, string) {
		t.Helper()
		var view application.TemporalTaskStatus
		var out string
		require.Eventually(t, func() bool {
			view, _, out = status(task)
			return view.State.Status == state
		}, 40*time.Second, 100*time.Millisecond)
		return view, out
	}
	decide := func(task, snapshot string, reject bool) (int, string, string) {
		t.Helper()
		args := []string{"task", "decide", task, "spec-review", "--actor", "test-coordinator", "--authorized-by", "test-human", "--reviewed-status", snapshot}
		if reject {
			args = append(args, "--reject", "--feedback", "Refine the specification")
		}
		return command(t, args...)
	}
	task := create("lifecycle")
	initial, code, _ := status(task)
	require.Zero(t, code)
	require.Equal(t, "not_started", initial.State.Status)
	require.NoError(t, os.WriteFile(filepath.Join(project, task, "SPEC.md"), []byte("# Request\nWrite a greeting at the project root.\n"), 0600))
	code, _, errOut := command(t, "task", "run", task, "--dry-run")
	require.Zero(t, code, errOut)
	code, _, errOut = command(t, "task", "run", task)
	require.Zero(t, code, errOut)
	first, snapshot := await(task, "waiting_approval")
	reviewed := filepath.Join(t.TempDir(), "reviewed.json")
	require.NoError(t, os.WriteFile(reviewed, []byte(snapshot), 0600))
	code, _, errOut = decide(task, reviewed, true)
	require.Zero(t, code, errOut)
	require.Eventually(t, func() bool {
		view, _, _ := status(task)
		return view.State.Status == "waiting_approval" && view.State.Stages[1].Attempts == 2
	}, 30*time.Second, 100*time.Millisecond)
	code, _, errOut = decide(task, reviewed, false)
	require.NotZero(t, code, "an old coordinator snapshot must not approve a new attempt")
	require.Contains(t, errOut, "reviewed approval attempt or artifact hashes changed")
	// A coordinator reconnect uses current state; a worker restart does not rerun drafting.
	worker.Stop()
	worker = app.NewTemporalWorker(server.Client(), queue)
	require.NoError(t, worker.Start())
	current, _, snapshot := status(task)
	require.Equal(t, first.WorkflowID, current.WorkflowID)
	require.Equal(t, first.RunID, current.RunID)
	require.Equal(t, 2, current.State.Stages[0].Attempts)
	require.NoError(t, os.WriteFile(reviewed, []byte(snapshot), 0600))
	code, _, errOut = decide(task, reviewed, false)
	require.Zero(t, code, errOut)
	failed, _ := await(task, "waiting_retry")
	_, code, _ = status(task)
	require.NotZero(t, code, "failed task status retains structured output and a nonzero exit")
	require.Contains(t, failed.State.Stages[2].LastError, "deliberate-build-failure")
	code, _, errOut = command(t, "task", "run", task)
	require.Zero(t, code, errOut)
	finished, _ := await(task, "completed")
	require.Equal(t, first.RunID, finished.RunID)
	require.Equal(t, 2, finished.State.Stages[2].Attempts)
	require.Equal(t, "test-coordinator", finished.State.Stages[1].ApprovedBy)
	require.Equal(t, "test-human", finished.State.Stages[1].AuthorizedBy)
	for _, entry := range finished.State.History {
		if entry.Step == "spec-review" {
			require.Equal(t, "test-coordinator", entry.Data["approved_by"])
			require.Equal(t, "test-human", entry.Data["authorized_by"])
		}
	}
	receipt, err := os.ReadFile(filepath.Join(project, task, ".watts-state", "approvals", "spec-review.txt"))
	require.NoError(t, err)
	require.Contains(t, string(receipt), "authorized_by test-human")
	require.Contains(t, string(receipt), "attempt 2")
	require.FileExists(t, filepath.Join(project, "greeting.txt"))
	require.NoFileExists(t, filepath.Join(project, task, "greeting.txt"))
	require.FileExists(t, filepath.Join(project, task, "REVIEW.md"))
	// A completed result remains inspectable without a worker.
	worker.Stop()
	completed, code, _ := status(task)
	require.Zero(t, code)
	require.Equal(t, "completed", completed.State.Status)
	worker = app.NewTemporalWorker(server.Client(), queue)
	require.NoError(t, worker.Start())
	cancelTask := create("cancel")
	code, _, errOut = command(t, "task", "run", cancelTask)
	require.Zero(t, code, errOut)
	await(cancelTask, "waiting_approval")
	code, _, errOut = command(t, "task", "cancel", cancelTask)
	require.Zero(t, code, errOut)
	await(cancelTask, "cancelled")
	_, code, _ = status(cancelTask)
	require.NotZero(t, code, "terminal cancellation must not be reported as success")
	code, _, errOut = command(t, "task", "run", cancelTask)
	require.NotZero(t, code, "terminal cancellation is not a recoverable stage retry")
}
