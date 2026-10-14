package application

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/encoding/prototext"
)

// Exercises an actual persistent Temporal service and ordinary containers.
func TestTemporalLive(t *testing.T) {
	if os.Getenv("WATTS_TEMPORAL_INTEGRATION") != "1" {
		t.Skip("set WATTS_TEMPORAL_INTEGRATION=1 with Docker and ubuntu:latest available")
	}
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	options := testsuite.DevServerOptions{CachedDownload: testsuite.CachedDownload{DestDir: filepath.Join(os.TempDir(), "watts-temporal-cli")}, DBFilename: filepath.Join(t.TempDir(), "temporal.db"), Stdout: io.Discard, Stderr: io.Discard}
	require.NoError(t, os.MkdirAll(options.CachedDownload.DestDir, 0700))
	server, err := testsuite.StartDevServer(ctx, options)
	require.NoError(t, err)
	t.Cleanup(func() {
		if server != nil {
			_ = server.Stop()
		}
	})
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, rootErr)
	app := &Service{Root: root, WorkingDirectory: root, Output: io.Discard, ErrorOutput: io.Discard}
	cfg := config.Default()
	cfg.Temporal = &config.TemporalSettings{Address: server.FrontendHostPort()}
	require.NoError(t, app.SaveConfig(cfg))
	queue := app.ResolvedTemporalSettings(cfg).TaskQueue
	worker := app.NewTemporalWorker(server.Client(), queue)
	require.NoError(t, worker.Start())
	t.Cleanup(func() { worker.Stop() })
	submit := func(name string, steps ...orchestration.Step) WorkflowBinding {
		t.Helper()
		task := "tasks/" + name
		require.NoError(t, os.MkdirAll(filepath.Join(root, task), 0700))
		require.NoError(t, manifest.Write(app.TaskWorkflowPath(task), manifest.Workflow, name, orchestration.Definition{Steps: steps}))
		require.NoError(t, app.RunTask(cfg, task, &RunOptions{}))
		binding, err := app.LoadWorkflowBinding(task)
		require.NoError(t, err)
		return binding
	}
	await := func(binding WorkflowBinding, predicate func(orchestration.State) bool) orchestration.State {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			state, err := QueryTemporalWorkflowState(ctx, server.Client(), binding)
			if err == nil && predicate(state) {
				return state
			}
			time.Sleep(100 * time.Millisecond)
		}
		state, err := QueryTemporalWorkflowState(ctx, server.Client(), binding)
		t.Fatalf("workflow %s did not reach expected state: %+v %v", binding.WorkflowID, state, err)
		return orchestration.State{}
	}
	draft := containerStep(`printf x >> /workspace/draft-calls; printf proposal > /outputs/task/proposal.md`)
	draft.Name = "draft"
	draft.Outputs = []string{"proposal.md"}
	publish := containerStep(`cat /inputs/task/proposal.md > /outputs/task/result.md`)
	publish.Name = "publish"
	publish.Inputs = []string{"proposal.md"}
	publish.Outputs = []string{"result.md"}
	publish.Container.Workspace = "read-only"
	binding := submit("approval", draft, orchestration.Step{Name: "authorize", Human: true, Inputs: []string{"proposal.md"}, OnFailure: "draft"}, publish)
	await(binding, func(state orchestration.State) bool { return state.Status == "waiting_approval" })
	require.Contains(t, binding.Input.Definition.Steps[0].Container.Image, "sha256:")
	require.Equal(t, "x", readFile(t, filepath.Join(root, "draft-calls")))
	// Restart both service and worker over the same persistent history.
	host := server.FrontendHostPort()
	worker.Stop()
	require.NoError(t, server.Stop())
	options.ClientOptions = &client.Options{HostPort: host}
	server, err = testsuite.StartDevServer(ctx, options)
	require.NoError(t, err)
	worker = app.NewTemporalWorker(server.Client(), queue)
	require.NoError(t, worker.Start())
	await(binding, func(state orchestration.State) bool { return state.Status == "waiting_approval" })
	require.Equal(t, "x", readFile(t, filepath.Join(root, "draft-calls")), "restart repeated a completed activity")
	t.Setenv("USER", "Human Tester")
	_, err = app.SubmitTaskDecision(ctx, binding.Input.Task, "authorize", true, "Refine the proposal")
	require.NoError(t, err)
	await(binding, func(state orchestration.State) bool {
		return state.Status == "waiting_approval" && state.Stages[1].Attempts == 2
	})
	_, err = app.SubmitTaskDecision(ctx, binding.Input.Task, "authorize", false)
	require.NoError(t, err)
	state := await(binding, func(state orchestration.State) bool { return state.Status == "completed" })
	require.Equal(t, 2, state.Stages[0].Attempts)
	require.Equal(t, "Human Tester", state.Stages[1].ApprovedBy)
	require.Equal(t, "xx", readFile(t, filepath.Join(root, "draft-calls")))
	require.Equal(t, "proposal", readFile(t, filepath.Join(root, binding.Input.Task, "result.md")))
	const secret = "synthetic-temporal-credential-123"
	t.Setenv("FIXTURE_TEMPORAL_KEY", secret)
	retry := containerStep("true")
	retry.SecretEnvironment = map[string]string{"TOKEN": "FIXTURE_TEMPORAL_KEY"}
	retry.Container.Image = "node:22.22.1-bookworm-slim@sha256:4f77a690f2f8946ab16fe1e791a3ac0667ae1c3575c3e4d0d4589e9ed5bfaf3d"
	retry.Container.Command = []string{"node", "-e", `const fs=require('fs');const input=JSON.parse(fs.readFileSync('/inputs/input.json'));if(input.attempt===1){console.error('recoverable-error: '+process.env.TOKEN);process.exit(7)}fs.writeFileSync('/outputs/task/result.md','recovered')`}
	retry.Name = "recover"
	retry.Outputs = []string{"result.md"}
	recovery := submit("retry", retry)
	state = await(recovery, func(state orchestration.State) bool { return state.Status == "waiting_retry" })
	require.Equal(t, 1, state.Stages[0].Attempts)
	require.Contains(t, state.Stages[0].LastError, "recoverable-error")
	require.Contains(t, state.Stages[0].LastError, "[REDACTED]")
	require.NotContains(t, state.Stages[0].LastError, secret)
	require.NotContains(t, readFile(t, filepath.Join(app.StateDir(recovery.Input.Task), "logs", "recover-1.log")), secret)
	history := server.Client().GetWorkflowHistory(ctx, recovery.WorkflowID, recovery.RunID, false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	for history.HasNext() {
		event, err := history.Next()
		require.NoError(t, err)
		require.NotContains(t, prototext.Format(event), secret, "Temporal history must not retain credential-bearing error causes or payloads")
	}
	require.NoError(t, app.RunTask(cfg, recovery.Input.Task, &RunOptions{}))
	await(recovery, func(state orchestration.State) bool { return state.Status == "completed" })
	require.Equal(t, "recovered", readFile(t, filepath.Join(root, recovery.Input.Task, "result.md")))
	// External events route through the same graph and enforce attempt identity.
	eventBinding := submit("event", orchestration.Step{Name: "ci", Event: true, Transitions: map[string]string{"passed": "end"}})
	await(eventBinding, func(state orchestration.State) bool { return state.Status == "waiting_event" })
	require.Error(t, app.SubmitTaskEvent(ctx, eventBinding.Input.Task, orchestration.Event{ID: "stale", Step: "ci", Attempt: 0, Outcome: "passed"}))
	require.NoError(t, app.SubmitTaskEvent(ctx, eventBinding.Input.Task, orchestration.Event{ID: "ci-1", Step: "ci", Attempt: 1, Outcome: "passed"}))
	await(eventBinding, func(state orchestration.State) bool { return state.Status == "completed" })
	slow := containerStep(`echo WORKLOAD_STARTED; sleep 45; printf late > /workspace/late`)
	slow.Name = "slow"
	cancelled := submit("cancel", slow)
	require.Eventually(t, func() bool {
		data, _ := os.ReadFile(filepath.Join(app.StateDir(cancelled.Input.Task), "logs", "slow-1.log"))
		return string(data) == "WORKLOAD_STARTED\n"
	}, 15*time.Second, 100*time.Millisecond)
	require.NoError(t, app.CancelTemporalWorkflow(cancelled.Input.Task))
	err = server.Client().GetWorkflow(ctx, cancelled.WorkflowID, cancelled.RunID).Get(ctx, nil)
	require.True(t, temporal.IsCanceledError(err), fmt.Sprint(err))
	require.NoFileExists(t, filepath.Join(root, "late"))
}
