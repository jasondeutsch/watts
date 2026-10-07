package orchestration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func environment(exec func(context.Context, ActivityInput) (Result, error)) *testsuite.TestWorkflowEnvironment {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(exec, activity.RegisterOptions{Name: ExecuteActivity})
	env.RegisterActivityWithOptions(func(_ context.Context, in ActivityInput) (Result, error) {
		return Result{Artifacts: []Artifact{{Path: in.Step.Inputs[0], SHA256: fmt.Sprint(in.Attempt)}}}, nil
	}, activity.RegisterOptions{Name: HashActivity})
	env.RegisterActivityWithOptions(func(context.Context, ActivityInput) error { return nil }, activity.RegisterOptions{Name: ValidateActivity})
	return env
}
func queryState(t *testing.T, env *testsuite.TestWorkflowEnvironment) State {
	t.Helper()
	v, e := env.QueryWorkflow(StatusQuery)
	require.Equal(t, nil, e,
		e)

	var st State
	{
		e = v.Get(&st)
		require.Equal(t, nil, e,
			e)
	}

	return st
}
func resultState(t *testing.T, env *testsuite.TestWorkflowEnvironment) State {
	t.Helper()

	require.NoError(t, env.GetWorkflowError())

	var st State

	require.NoError(t, env.GetWorkflowResult(&st))

	return st
}
func TestApprovalRejectionReturnsToCustomStage(t *testing.T) {
	calls := 0
	env := environment(func(context.Context, ActivityInput) (Result, error) { calls++; return Result{}, nil })
	d := Definition{Steps: []Step{{Name: "draft", Command: "prepare"}, {Name: "authorize", Human: true, Inputs: []string{"proposal.md"}, OnFailure: "draft"}, {Name: "publish", Command: "publish"}}}
	env.RegisterDelayedCallback(func() {
		st := queryState(t, env)
		assert.Equal(t, "waiting_approval", st.Status,
			"state=%+v", st)

		env.UpdateWorkflowNoRejection(DecisionUpdate, "reject", t, Decision{Step: "authorize", Attempt: 1, By: "Human", Reject: true, Artifacts: st.Stages[1].Artifacts})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		st := queryState(t, env)
		assert.False(t, st.Stages[0].Attempts != 2 || st.Stages[1].Attempts != 2,
			"rework not recorded: %+v", st)

		env.UpdateWorkflowNoRejection(DecisionUpdate, "approve", t, Decision{Step: "authorize", Attempt: 2, By: "Human", Artifacts: st.Stages[1].Artifacts})
	}, 2*time.Second)
	env.ExecuteWorkflow(Run, Input{Task: "tasks/example", Definition: d})
	st := resultState(t, env)
	require.False(t, st.Status != "completed" || calls != 3 || st.Stages[1].ApprovedBy != "Human",
		"state=%+v calls=%d", st, calls)
}
func TestFailureWaitsForExplicitRetryAndRejectsStaleAttempt(t *testing.T) {
	calls := 0
	rejected := false
	env := environment(func(context.Context, ActivityInput) (Result, error) {
		calls++
		if calls == 1 {
			return Result{}, errors.New("partial write")
		}
		return Result{}, nil
	})
	env.RegisterDelayedCallback(func() {
		st := queryState(t, env)
		assert.False(t, calls != 1 || st.Status != "waiting_retry",
			"activity was retried automatically: calls=%d state=%+v", calls, st)

		env.UpdateWorkflow(RetryUpdate, "stale", &testsuite.TestUpdateCallback{OnReject: func(error) { rejected = true }, OnAccept: func() { assert.Fail(t, fmt.Sprint("stale retry was accepted")) }, OnComplete: func(any, error) {}}, Retry{Step: "write", Attempt: 0})
		env.UpdateWorkflowNoRejection(RetryUpdate, "retry", t, Retry{Step: "write", Attempt: 1})
	}, time.Second)
	env.ExecuteWorkflow(Run, Input{Definition: Definition{Steps: []Step{{Name: "write", Command: "write"}}}})
	st := resultState(t, env)
	assert.True(t, rejected,
		"stale attempt must be rejected")
	require.False(t, calls != 2 || st.Stages[0].Attempts != 2,
		"calls=%d state=%+v", calls, st)
}
func TestApprovalRejectsWrongHashes(t *testing.T) {
	rejected := false
	env := environment(func(context.Context, ActivityInput) (Result, error) { return Result{}, nil })
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(DecisionUpdate, "stale", &testsuite.TestUpdateCallback{OnReject: func(error) { rejected = true }, OnAccept: func() { assert.Fail(t, fmt.Sprint("wrong hashes accepted")) }, OnComplete: func(any, error) {}}, Decision{Step: "approve", Attempt: 1, By: "Human", Artifacts: []Artifact{{Path: "proposal.md", SHA256: "wrong"}}})
		st := queryState(t, env)
		env.UpdateWorkflowNoRejection(DecisionUpdate, "good", t, Decision{Step: "approve", Attempt: 1, By: "Human", Artifacts: st.Stages[0].Artifacts})
	}, time.Second)
	env.ExecuteWorkflow(Run, Input{Definition: Definition{Steps: []Step{{Name: "approve", Human: true, Inputs: []string{"proposal.md"}}}}})
	{
		st := resultState(t, env)
		require.Equal(t, "completed", st.Status,
			st)
	}
	assert.True(t, rejected,
		"wrong hashes must be rejected")
}
func TestCancellationDuringHumanWait(t *testing.T) {
	env := environment(func(context.Context, ActivityInput) (Result, error) { return Result{}, nil })
	env.RegisterDelayedCallback(func() { env.CancelWorkflow() }, time.Second)
	env.ExecuteWorkflow(Run, Input{Definition: Definition{Steps: []Step{{Name: "approve", Human: true, Inputs: []string{"proposal.md"}}}}})
	{
		err := env.GetWorkflowError()
		require.True(t, temporal.IsCanceledError(err),
			"expected cancellation: %v", err)
	}
}
func TestTransitionLimitBoundsRework(t *testing.T) {
	env := environment(func(context.Context, ActivityInput) (Result, error) { return Result{}, errors.New("rejected") })
	env.ExecuteWorkflow(Run, Input{Definition: Definition{MaxTransitions: 2, Steps: []Step{{Name: "review", Command: "review", OnFailure: "review"}}}})
	assert.NotEqual(t, nil, env.GetWorkflowError(),
		"unbounded rework must fail")
}
func TestDefinitionValidation(t *testing.T) {
	for _, d := range []Definition{
		{Steps: []Step{{Name: "first", Command: "true", Next: "missing"}}},
		{Steps: []Step{{Name: "first", Command: "true", Next: "end"}, {Name: "unreachable", Command: "true"}}},
		{Steps: []Step{{Name: "approve", Human: true}}},
		{Steps: []Step{{Name: "approve", Human: true, Inputs: []string{"proposal.md"}, Outputs: []string{"result.md"}}}},
		{Steps: []Step{{Name: "approve", Human: true, Inputs: []string{"proposal.md"}, Checks: []Check{{Script: "verify"}}}}},
		{Steps: []Step{{Name: "write", Command: "true", Outputs: []string{"../outside"}}}},
		{Steps: []Step{{Name: "write", Command: "true", Outputs: []string{".watts-state/state.json"}}}},
	} {
		{
			err := d.Validate()
			assert.Error(t, err,
				"expected invalid definition: %+v", d)
		}
	}
}

func TestAgentStagesRunInDeclaredOrder(t *testing.T) {
	var executed []string
	env := environment(func(_ context.Context, input ActivityInput) (Result, error) {
		executed = append(executed, input.Step.Name)
		return Result{Outcome: "success"}, nil
	})
	definition := Definition{Steps: []Step{
		{Name: "design", Agent: "designer", Identity: "design-agent", Prompt: "Design"},
		{Name: "code", Agent: "builder", Identity: "code-agent", Prompt: "Build"},
		{Name: "qa", Agent: "tester", Identity: "qa-agent", Prompt: "Test"},
	}}
	env.ExecuteWorkflow(Run, Input{Definition: definition})
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, []string{"design", "code", "qa"}, executed)
}

func TestWorkflowStatusPreservesCheckExplanation(t *testing.T) {
	message := "completion check check-spec: SPEC.md still contains template placeholders (exit status 1)"
	env := environment(func(context.Context, ActivityInput) (Result, error) {
		return Result{}, fmt.Errorf("completion check check-spec: SPEC.md still contains template placeholders (%w)", errors.New("exit status 1"))
	})
	env.ExecuteWorkflow(Run, Input{Definition: Definition{Steps: []Step{{Name: "specification", Command: "true", OnFailure: "end"}}}})
	require.ErrorContains(t, env.GetWorkflowError(), message)
	value, err := env.QueryWorkflow(StatusQuery)
	require.NoError(t, err)
	var state State
	require.NoError(t, value.Get(&state))
	require.Equal(t, message, state.Stages[0].LastError)
	require.Equal(t, message, state.History[0].Error)
}
