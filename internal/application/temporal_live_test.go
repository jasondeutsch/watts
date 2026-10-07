package application

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

// Opt-in because this starts an actual service and may download the Temporal CLI.
func TestTemporalLive(t *testing.T) {
	if os.Getenv("WATTS_TEMPORAL_INTEGRATION") != "1" {
		t.Skip("set WATTS_TEMPORAL_INTEGRATION=1 to run the real Temporal server test")
	}
	needTools(t, "bash")
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
	ta := newProject(t)
	ta.initRepo(t)
	cfg, err := ta.LoadConfig()
	require.NoError(t, err)

	cfg.Temporal = &projectconfig.TemporalSettings{Address: server.FrontendHostPort()}
	cfg.Workflow = &orchestration.Definition{Steps: []orchestration.Step{
		{Name: "draft", Command: `printf 'draft\n' >> "$WATTS_TASK/calls"; printf 'proposal\n' > "$WATTS_TASK/proposal.md"`, Outputs: []string{"proposal.md"}},
		{Name: "authorize", Human: true, Inputs: []string{"proposal.md"}, OnFailure: "draft"},
		{Name: "publish", Command: `cp "$WATTS_TASK/proposal.md" "$WATTS_TASK/result.md"`, Outputs: []string{"result.md"}},
	}}

	require.NoError(t, ta.SaveConfig(cfg))

	workerService := *ta.Service
	workerService.Output, workerService.ErrorOutput = io.Discard, io.Discard
	workerService.Report = nil
	workerApp := &workerService
	queue := ta.ResolvedTemporalSettings(cfg).TaskQueue
	w := workerApp.NewTemporalWorker(server.Client(), queue)

	require.NoError(t, w.Start())

	t.Cleanup(func() { w.Stop() })
	taskPath := "tasks/live"
	_ = os.MkdirAll(filepath.Join(ta.Root, taskPath), 0700)

	require.NoError(t, storage.WriteJSON(ta.TaskWorkflowPath(taskPath), cfg.Workflow))
	require.NoError(t, ta.RunTask(cfg, taskPath, &RunOptions{}))

	rec, err := ta.LoadWorkflowBinding(taskPath)
	require.NoError(t, err)

	await := func(rec WorkflowBinding, predicate func(orchestration.State) bool) orchestration.State {
		t.Helper()
		deadline := time.Now().Add(35 * time.Second)
		for time.Now().Before(deadline) {
			qctx, qcancel := context.WithTimeout(ctx, 2*time.Second)
			st, e := QueryTemporalWorkflowState(qctx, server.Client(), rec)
			qcancel()
			if e == nil && predicate(st) {
				return st
			}
			time.Sleep(100 * time.Millisecond)
		}
		require.FailNow(t, fmt.Sprintf("workflow %s did not reach expected state", rec.WorkflowID))
		return orchestration.State{}
	}
	await(rec, func(st orchestration.State) bool { return st.Status == "waiting_approval" })
	require.NoError(t, ta.RunTask(cfg, taskPath, &RunOptions{}))
	require.Equal(t, "draft\n", readFile(t, filepath.Join(ta.Root, taskPath, "calls")))
	// Change the project default, then restart both the worker and persistent service. The
	// submitted task must replay its original definition without re-running the draft activity.
	cfg.Workflow = &orchestration.Definition{Steps: []orchestration.Step{{Name: "changed-default", Command: "exit 99"}}}

	require.NoError(t, ta.SaveConfig(cfg))

	host := server.FrontendHostPort()
	w.Stop()

	require.NoError(t, server.Stop())

	options.ClientOptions = &client.Options{HostPort: host}
	server, err = testsuite.StartDevServer(ctx, options)
	require.NoError(t, err)

	w = workerApp.NewTemporalWorker(server.Client(), queue)

	require.NoError(t, w.Start())

	st := await(rec, func(st orchestration.State) bool { return st.Status == "waiting_approval" })
	require.False(t, st.Stages[0].Attempts != 1 || readFile(t, filepath.Join(ta.Root, taskPath, "calls")) != "draft\n",
		"restart repeated a completed file-mutating activity")

	t.Setenv("USER", "Human Tester")
	{
		_, err = ta.SubmitTaskDecision(ctx, taskPath, "authorize", true)
		require.NoError(t, err)
	}

	await(rec, func(st orchestration.State) bool {
		return st.Status == "waiting_approval" && st.Stages[1].Attempts == 2
	})
	{
		_, err = ta.SubmitTaskDecision(ctx, "", "", false)
		require.NoError(t, err)
	}

	st = await(rec, func(st orchestration.State) bool { return st.Status == "completed" })
	require.False(t, st.Stages[0].Attempts != 2 || st.Stages[1].ApprovedBy != "Human Tester",
		st)

	require.NoError(t, ta.RunTask(cfg, taskPath, &RunOptions{}))

	require.Equal(t, "proposal\n", readFile(t, filepath.Join(ta.Root, taskPath, "result.md")),
		"published output missing")

	// A command failure is not automatically retried; a validated explicit retry completes it.
	cfg.Workflow = &orchestration.Definition{Steps: []orchestration.Step{{Name: "recover", Command: `if test -f "$WATTS_TASK/once"; then printf recovered > "$WATTS_TASK/result.md"; else touch "$WATTS_TASK/once"; exit 7; fi`, Outputs: []string{"result.md"}}}}

	require.NoError(t, ta.SaveConfig(cfg))

	retryRel := "tasks/retry"
	_ = os.MkdirAll(filepath.Join(ta.Root, retryRel), 0700)

	require.NoError(t, storage.WriteJSON(ta.TaskWorkflowPath(retryRel), cfg.Workflow))
	require.NoError(t, ta.RunTask(cfg, retryRel, &RunOptions{}))

	retryRec, _ := ta.LoadWorkflowBinding(retryRel)
	st = await(retryRec, func(st orchestration.State) bool { return st.Status == "waiting_retry" })
	require.Equal(t, 1, st.Stages[0].Attempts,
		"unexpected automatic activity retry")

	require.NoError(t, ta.RunTask(cfg, retryRel, &RunOptions{}))

	await(retryRec, func(st orchestration.State) bool { return st.Status == "completed" })
	// Cancellation must stop a real process and its descendants before the late write.
	cfg.Workflow = &orchestration.Definition{Steps: []orchestration.Step{{Name: "long-command", Command: `touch "$WATTS_TASK/started"; sleep 45; touch "$WATTS_TASK/late"`}}}

	require.NoError(t, ta.SaveConfig(cfg))

	cancelRel := "tasks/cancel"
	_ = os.MkdirAll(filepath.Join(ta.Root, cancelRel), 0700)

	require.NoError(t, storage.WriteJSON(ta.TaskWorkflowPath(cancelRel), cfg.Workflow))
	require.NoError(t, ta.RunTask(cfg, cancelRel, &RunOptions{}))

	cancelRec, _ := ta.LoadWorkflowBinding(cancelRel)
	deadline := time.Now().Add(15 * time.Second)
	for !exists(filepath.Join(ta.Root, cancelRel, "started")) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	require.True(t, exists(filepath.Join(ta.Root, cancelRel, "started")),
		"long command did not start")

	require.NoError(t, ta.CancelTemporalWorkflow(cancelRel))

	err = server.Client().GetWorkflow(ctx, cancelRec.WorkflowID, cancelRec.RunID).Get(ctx, nil)
	require.True(t, temporal.IsCanceledError(err),
		"expected canceled execution: %v", err)
	require.False(t, exists(filepath.Join(ta.Root, cancelRel, "late")),
		"late write occurred after cancellation")
	require.False(t, exists(ta.Wd("workflow-active-process.json")),
		"active process journal survived cancellation")

	status, err := ta.TemporalTaskStatus(ctx, taskPath)
	require.False(t, err != nil || status.State.Status != "completed", "durable status: %+v %v", status, err)

	// Execute a named workflow with a real executable capability and durable external events.
	cfg.Workflow, cfg.DefaultWorkflow = nil, "workflows/qa-loop.json"
	cfg.Capabilities = map[string]projectconfig.Capability{"qa-tool": {Command: []string{"bash", "-c", `cat > "$WATTS_TASK/qa-input.json"; printf 'QA evidence' > "$WATTS_TASK/qa-report.md"; if test -f "$WATTS_TASK/qa-once"; then printf '{"outcome":"passed","artifacts":["qa-report.md"]}'; else touch "$WATTS_TASK/qa-once"; printf '{"outcome":"changes-required","feedback":"Fix QA regression","artifacts":["qa-report.md"]}'; fi`}}}
	definition := orchestration.Definition{Version: 1, Steps: []orchestration.Step{
		{Name: "coding", Command: `cat "$WATTS_STEP_INPUT" >> "$WATTS_TASK/coding-contexts.jsonl"`},
		{Name: "qa", Capability: "qa-tool", Transitions: map[string]string{"changes-required": "coding", "passed": "ci"}},
		{Name: "ci", Event: true, Transitions: map[string]string{"changes-required": "coding", "passed": "end"}},
	}}

	require.NoError(t, storage.WriteJSON(filepath.Join(ta.Root, "workflows", "qa-loop.json"), definition))

	require.NoError(t, ta.SaveConfig(cfg))

	customTask, err := ta.CreateTask(cfg, "live-qa", "")
	require.NoError(t, err)

	require.NoError(t, ta.RunTask(cfg, customTask, &RunOptions{}))

	customBinding, err := ta.LoadWorkflowBinding(customTask)
	require.NoError(t, err)

	st = await(customBinding, func(st orchestration.State) bool { return st.Status == "waiting_event" })
	require.False(t, st.Stages[0].Attempts != 2 || st.Stages[1].Attempts != 2 || !strings.Contains(readFile(t, filepath.Join(ta.Root, customTask, "coding-contexts.jsonl")), "Fix QA regression"),
		"QA rework/context=%+v", st)

	// A worker restart keeps the event wait and must not replay completed plugin effects.
	w.Stop()
	w = workerApp.NewTemporalWorker(server.Client(), queue)

	require.NoError(t, w.Start())

	await(customBinding, func(st orchestration.State) bool { return st.Status == "waiting_event" })
	{
		err = ta.SubmitTaskEvent(ctx, customTask, orchestration.Event{ID: "ci-1", Step: "ci", Attempt: 0, Outcome: "passed"})
		require.Error(t, err,
			"stale external event accepted")
	}

	require.NoError(t, ta.SubmitTaskEvent(ctx, customTask, orchestration.Event{Step: "ci", Outcome: "changes-required", ID: "ci-1", Attempt: 1, Feedback: "Fix CI regression"}))

	st = await(customBinding, func(st orchestration.State) bool { return st.Status == "waiting_event" && st.Stages[2].Attempts == 2 })
	require.False(t, st.Stages[0].Attempts != 3 || !strings.Contains(readFile(t, filepath.Join(ta.Root, customTask, "coding-contexts.jsonl")), "Fix CI regression"),
		"external rework=%+v", st)

	require.NoError(t, ta.SubmitTaskEvent(ctx, customTask, orchestration.Event{Step: "ci", Outcome: "passed", ID: "ci-2", Attempt: 2}))

	st = await(customBinding, func(st orchestration.State) bool { return st.Status == "completed" })
	require.Len(t, st.History, 8,
		"attempt history=%+v", st.History)

	require.NoError(t, ta.RunTask(cfg, customTask, &RunOptions{}))
}
