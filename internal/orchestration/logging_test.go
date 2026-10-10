package orchestration

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	temporallog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/testsuite"
)

func loggingEnvironment(output *bytes.Buffer, execute func(context.Context, ActivityInput) (Result, error)) *testsuite.TestWorkflowEnvironment {
	var suite testsuite.WorkflowTestSuite
	suite.SetLogger(temporallog.NewStructuredLogger(slog.New(slog.NewTextHandler(output, nil))))
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(execute, activity.RegisterOptions{Name: ExecuteActivity})
	env.RegisterActivityWithOptions(func(context.Context, ActivityInput) (Result, error) {
		return Result{Artifacts: []Artifact{{Path: "SPEC.md", SHA256: "spec-hash"}}}, nil
	}, activity.RegisterOptions{Name: HashActivity})
	env.RegisterActivityWithOptions(func(context.Context, ActivityInput) error { return nil }, activity.RegisterOptions{Name: ValidateActivity})
	return env
}

func TestLogsExplainFailedAttemptAndAuthorizedRetry(t *testing.T) {
	var output bytes.Buffer
	env := loggingEnvironment(&output, func(_ context.Context, in ActivityInput) (Result, error) {
		if in.Attempt == 1 {
			return Result{}, errors.New("compiler: undefined symbol")
		}
		return Result{Outcome: "success"}, nil
	})
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflowNoRejection(RetryUpdate, "retry", t, Retry{Step: "build", Attempt: 1, Force: true})
	}, time.Second)
	env.ExecuteWorkflow(Run, Input{Task: "tasks/demo", Definition: Definition{Steps: []Step{{Name: "build", Command: "build", MaxAttempts: 1}}}})
	require.NoError(t, env.GetWorkflowError())
	text := output.String()
	for _, value := range []string{`msg="Stage started" task=tasks/demo stage=build attempt=1`, `msg="Stage failed"`, "compiler: undefined symbol", `msg="Waiting for retry"`, "watts task run 'tasks/demo' --ignore-attempt-limit", `msg="Retry authorized"`, "attempt=2", `msg="Stage completed"`, `msg="Workflow completed"`} {
		require.Contains(t, text, value)
	}
}

func TestLogsExplainApprovalAndReworkRouting(t *testing.T) {
	var output bytes.Buffer
	env := loggingEnvironment(&output, func(context.Context, ActivityInput) (Result, error) { return Result{}, nil })
	env.RegisterDelayedCallback(func() {
		state := queryState(t, env)
		env.UpdateWorkflowNoRejection(DecisionUpdate, "reject", t, Decision{Step: "approve", Attempt: 1, By: "Human", Reject: true, Artifacts: state.Stages[1].Artifacts})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		state := queryState(t, env)
		env.UpdateWorkflowNoRejection(DecisionUpdate, "approve", t, Decision{Step: "approve", Attempt: 2, By: "Human", Artifacts: state.Stages[1].Artifacts})
	}, 2*time.Second)
	env.ExecuteWorkflow(Run, Input{Task: "tasks/demo", Definition: Definition{Steps: []Step{
		{Name: "draft", Command: "draft"},
		{Name: "approve", Human: true, Inputs: []string{"SPEC.md"}, Transitions: map[string]string{"approved": "end", "rejected": "draft"}},
	}}})
	require.NoError(t, env.GetWorkflowError())
	text := output.String()
	require.Contains(t, text, `msg="Waiting for approval"`)
	require.Contains(t, text, "watts task decide 'tasks/demo' approve")
	require.Contains(t, text, "rejected=true")
	require.Contains(t, text, "outcome=rejected next_stage=draft")
	require.NotContains(t, text, "Waiting for retry")
}

func TestLogsDistinguishCancellationFromTerminalFailure(t *testing.T) {
	var output bytes.Buffer
	env := loggingEnvironment(&output, func(context.Context, ActivityInput) (Result, error) { return Result{}, nil })
	env.RegisterDelayedCallback(env.CancelWorkflow, time.Second)
	env.ExecuteWorkflow(Run, Input{Task: "tasks/demo", Definition: Definition{Steps: []Step{{Name: "ci", Event: true, Transitions: map[string]string{"passed": "end"}}}}})
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, output.String(), "Waiting for event")
	require.Contains(t, output.String(), "Workflow cancelled")
	require.NotContains(t, output.String(), `msg="Workflow failed"`)
}
