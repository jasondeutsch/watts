package application

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workspace"
)

func boundTemporalTask(t *testing.T, step orchestration.Step) (*testApp, orchestration.ActivityInput) {
	t.Helper()
	ta := newProject(t)
	ta.initRepo(t)
	cfg, err := ta.LoadConfig()
	require.NoError(t, err)

	cfg.Temporal = &projectconfig.TemporalSettings{}
	cfg.Workflow = &orchestration.Definition{Steps: []orchestration.Step{step}}
	taskPath := "tasks/custom"

	require.NoError(t, os.MkdirAll(filepath.Join(ta.Root, taskPath), 0700))

	require.NoError(t, storage.WriteJSONAtomically(ta.WorkflowConfigPath(taskPath), cfg))

	data, err := os.ReadFile(ta.WorkflowConfigPath(taskPath))
	require.NoError(t, err)

	in := orchestration.Input{Task: taskPath, Definition: *cfg.Workflow, ConfigSHA256: kit.Digest(data)}

	require.NoError(t, storage.WriteJSONAtomically(ta.WorkflowBindingPath(taskPath), WorkflowBinding{WorkflowID: "test", Settings: ta.ResolvedTemporalSettings(cfg), Input: in}))

	return ta, orchestration.ActivityInput{Input: in, Step: step, Attempt: 1}
}
func TestTemporalActivityRecordsCommandAndRequiresOutputs(t *testing.T) {
	ta, in := boundTemporalTask(t, orchestration.Step{Name: "write", Command: `printf 'done' > "$WATTS_TASK/result.md"`, Outputs: []string{"result.md"}})
	gitTripwire(t)
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(ta.ExecuteWorkflowStage)
	value, err := env.ExecuteActivity(ta.ExecuteWorkflowStage, in)
	require.NoError(t, err)

	var result orchestration.Result

	require.NoError(t, value.Get(&result))

	require.False(t, len(result.Artifacts) != 1 || result.Artifacts[0].SHA256 != kit.Digest([]byte("done")),
		result)
	require.False(t, exists(filepath.Join(ta.StateDir(in.Input.Task), "state.json")),
		"activities must not persist workflow progress")

	persisted := readFile(t, filepath.Join(ta.StateDir(in.Input.Task), "steps", "write-1", "result.json"))
	require.Contains(t, persisted, result.Artifacts[0].SHA256,
		"result evidence missing")

	ta2, in2 := boundTemporalTask(t, orchestration.Step{Name: "missing", Command: "true", Outputs: []string{"missing.md"}})
	env2 := suite.NewTestActivityEnvironment()
	env2.RegisterActivity(ta2.ExecuteWorkflowStage)
	{
		_, err = env2.ExecuteActivity(ta2.ExecuteWorkflowStage, in2)
		require.Error(t, err,
			"missing required output must fail")
	}
	require.False(t, exists(filepath.Join(ta2.StateDir(in2.Input.Task), "steps", "missing-1", "result.json")),
		"invalid result must not be persisted")
}
func TestTemporalActivityRejectsModifiedBinding(t *testing.T) {
	ta, in := boundTemporalTask(t, orchestration.Step{Name: "write", Command: "true"})
	in.Step.Command = "touch outside"
	{
		_, err := ta.prepareActivity(context.Background(), in)
		require.Error(t, err,
			"activity must match its pinned definition")
	}

	in.Step = in.Input.Definition.Steps[0]
	writeFile(t, ta.WorkflowConfigPath(in.Input.Task), "{}")
	{
		_, err := ta.prepareActivity(context.Background(), in)
		require.Error(t, err,
			"modified runtime config must fail")
	}
}
func TestTemporalApprovalDetectsChangedArtifact(t *testing.T) {
	ta, in := boundTemporalTask(t, orchestration.Step{Name: "authorize", Human: true, Inputs: []string{"proposal.md"}})
	p := filepath.Join(ta.Root, in.Input.Task, "proposal.md")
	writeFile(t, p, "original")
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(ta.CaptureApprovalArtifacts)
	env.RegisterActivity(ta.ValidateApprovalArtifacts)
	value, err := env.ExecuteActivity(ta.CaptureApprovalArtifacts, in)
	require.NoError(t, err)

	var result orchestration.Result

	require.NoError(t, value.Get(&result))

	in.Artifacts = result.Artifacts
	writeFile(t, p, "changed")
	{
		_, err = env.ExecuteActivity(ta.ValidateApprovalArtifacts, in)
		require.Error(t, err,
			"stale approval must fail")
	}
}
func TestTaskArtifactSymlinkCannotEscape(t *testing.T) {
	ta, in := boundTemporalTask(t, orchestration.Step{Name: "write", Command: "true"})
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeFile(t, outside, "outside")

	require.NoError(t, os.Symlink(outside, filepath.Join(ta.Root, in.Input.Task, "link.md")))

	{
		_, err := workspace.HashTaskArtifacts(ta.Root, in.Input.Task, []string{"link.md"})
		require.Error(t, err,
			"symlink escape must fail")
	}
}
func TestTemporalProcessCancellationStopsChildren(t *testing.T) {
	ta := newProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	journal := filepath.Join(ta.Root, "active.json")
	late := filepath.Join(ta.Root, "late")
	cmd := exec.Command("bash", "-c", `sleep 1; printf late > "$1"`, "bash", late)
	done := make(chan error, 1)
	go func() { done <- workspace.RunProcess(ctx, journal, cmd, 0) }()
	deadline := time.Now().Add(3 * time.Second)
	for !exists(journal) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, exists(journal),
		"process did not start")

	cancel()
	select {
	case err := <-done:
		if err == nil {
			assert.Fail(t, fmt.Sprint("cancelled process must fail"))
		}
	case <-time.After(3 * time.Second):
		require.
			FailNow(t, fmt.Sprint("cancellation did not stop process"))
	}
	time.Sleep(1100 * time.Millisecond)
	require.False(t, exists(late),
		"a child survived cancellation")
	require.False(t, exists(journal),
		"completed cancellation must clear journal")
}
func TestTemporalWorkspaceLockAndOrphanDetection(t *testing.T) {
	ta := newProject(t)
	unlock, err := workspace.LockWorkflowWorkspace(ta.Wd())
	require.NoError(t, err)

	if unlock2, err := workspace.LockWorkflowWorkspace(ta.Wd()); err == nil {
		unlock2()
		require.FailNow(t, fmt.Sprint("second activity acquired same workspace"))
	}
	unlock()
	cmd := exec.Command("sleep", "10")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	require.NoError(t, cmd.Start())

	defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() }()

	require.NoError(t, storage.WriteJSONAtomically(ta.Wd("workflow-active-process.json"), workspace.ActiveProcess{PID: cmd.Process.Pid}))

	if unlock2, err := workspace.LockWorkflowWorkspace(ta.Wd()); err == nil {
		unlock2()
		require.FailNow(t, fmt.Sprint("surviving process must block new activity"))
	} else if !strings.Contains(err.Error(), "may still be running") {
		require.FailNow(t, fmt.Sprint(err))
	}
}
