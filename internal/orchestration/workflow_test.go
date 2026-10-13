package orchestration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jasondeutsch/watts/internal/workload"
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
	env.RegisterActivityWithOptions(func(context.Context, ReportInput) error { return nil }, activity.RegisterOptions{Name: ReportActivity})
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
	d := Definition{Steps: []Step{{Name: "draft", Container: fixtureContainer("prepare")}, {Name: "authorize", Human: true, Inputs: []string{"proposal.md"}, OnFailure: "draft"}, {Name: "publish", Container: fixtureContainer("publish")}}}
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
	env.ExecuteWorkflow(Run, Input{Definition: Definition{Steps: []Step{{Name: "write", Container: fixtureContainer("write")}}}})
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
	env.ExecuteWorkflow(Run, Input{Definition: Definition{MaxTransitions: 2, Steps: []Step{{Name: "review", Container: fixtureContainer("review"), OnFailure: "review"}}}})
	assert.NotEqual(t, nil, env.GetWorkflowError(),
		"unbounded rework must fail")
}
func TestDefinitionValidation(t *testing.T) {
	for _, d := range []Definition{
		{Steps: []Step{{Name: "first", Container: fixtureContainer("true"), Next: "missing"}}},
		{Steps: []Step{{Name: "first", Container: fixtureContainer("true"), Next: "end"}, {Name: "unreachable", Container: fixtureContainer("true")}}},
		{Steps: []Step{{Name: "approve", Human: true}}},
		{Steps: []Step{{Name: "approve", Human: true, Inputs: []string{"proposal.md"}, Outputs: []string{"result.md"}}}},
		{Steps: []Step{{Name: "approve", Human: true, Inputs: []string{"proposal.md"}, Checks: []Check{{Command: []string{"verify"}}}}}},
		{Steps: []Step{{Name: "write", Container: fixtureContainer("true"), Outputs: []string{"../outside"}}}},
		{Steps: []Step{{Name: "write", Container: fixtureContainer("true"), Outputs: []string{".watts-state/state.json"}}}},
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
		{Name: "design", Agent: "designer", Identity: "design-agent", Prompt: "Design", Container: fixtureContainer("design")},
		{Name: "code", Agent: "builder", Identity: "code-agent", Prompt: "Build", Container: fixtureContainer("build")},
		{Name: "qa", Agent: "tester", Identity: "qa-agent", Prompt: "Test", Container: fixtureContainer("test")},
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
	env.ExecuteWorkflow(Run, Input{Definition: Definition{Steps: []Step{{Name: "specification", Container: fixtureContainer("true"), OnFailure: "end"}}}})
	require.ErrorContains(t, env.GetWorkflowError(), message)
	value, err := env.QueryWorkflow(StatusQuery)
	require.NoError(t, err)
	var state State
	require.NoError(t, value.Get(&state))
	require.Equal(t, message, state.Stages[0].LastError)
	require.Equal(t, message, state.History[0].Error)
}

func fixtureContainer(command string) *workload.Spec {
	return &workload.Spec{Image: "fixture:0.1.0", Command: []string{command}}
}

func TestDelegatedDecisionProvenanceSurvivesRejectionAndApproval(t *testing.T) {
	env := environment(func(context.Context, ActivityInput) (Result, error) { return Result{}, nil })
	definition := Definition{Steps: []Step{{Name: "draft", Container: fixtureContainer("draft")}, {Name: "approve", Human: true, Inputs: []string{"SPEC.md"}, Transitions: map[string]string{"rejected": "draft", "approved": "end"}}}}
	for attempt := 1; attempt <= 2; attempt++ {
		env.RegisterDelayedCallback(func() {
			state := queryState(t, env)
			decision := Decision{Step: "approve", Attempt: attempt, By: "external-coordinator", AuthorizedBy: "human", Reject: attempt == 1, Artifacts: state.Stages[1].Artifacts, Feedback: "Clarify acceptance"}
			env.UpdateWorkflowNoRejection(DecisionUpdate, fmt.Sprint(attempt), t, decision)
		}, time.Duration(attempt)*time.Second)
	}
	env.ExecuteWorkflow(Run, Input{Definition: definition})
	state := resultState(t, env)
	require.Equal(t, "external-coordinator", state.Stages[1].ApprovedBy)
	require.Equal(t, "human", state.Stages[1].AuthorizedBy)
	var decisions []StageResult
	for _, entry := range state.History {
		if entry.Step == "approve" {
			decisions = append(decisions, entry)
			require.Equal(t, "external-coordinator", entry.Data["approved_by"])
			require.Equal(t, "human", entry.Data["authorized_by"])
		}
	}
	require.Len(t, decisions, 2)
	require.Equal(t, "rejected", decisions[0].Outcome)
	require.Equal(t, "approved", decisions[1].Outcome)
}

func TestCompletionReportIncludesActualAttemptsAndTiming(t *testing.T) {
	env := environment(func(_ context.Context, in ActivityInput) (Result, error) {
		if in.Attempt == 1 {
			return Result{}, errors.New("compile failed")
		}
		return Result{Outcome: "success"}, nil
	})
	var reports []ReportInput
	env.RegisterActivityWithOptions(func(_ context.Context, request ReportInput) error { reports = append(reports, request); return nil }, activity.RegisterOptions{Name: ReportActivity})
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflowNoRejection(RetryUpdate, "retry-report", t, Retry{Step: "build", Attempt: 1})
	}, time.Second)
	env.ExecuteWorkflow(Run, Input{Task: "tasks/report", Definition: Definition{Steps: []Step{{Name: "build", Container: fixtureContainer("build")}}}})
	require.NoError(t, env.GetWorkflowError())
	require.Len(t, reports, 1)
	require.Equal(t, "completed", reports[0].State.Status)
	require.Len(t, reports[0].State.History, 2)
	require.Contains(t, reports[0].State.History[0].Error, "compile failed")
	require.False(t, reports[0].State.StartedAt.IsZero())
	require.False(t, reports[0].State.History[0].FinishedAt.IsZero())
	require.NotEmpty(t, reports[0].RunID)
}

func TestFailedReportPreservesWorkflowResultAndExposesError(t *testing.T) {
	env := environment(func(context.Context, ActivityInput) (Result, error) { return Result{}, nil })
	env.RegisterActivityWithOptions(func(context.Context, ReportInput) error { return errors.New("disk full") }, activity.RegisterOptions{Name: ReportActivity})
	env.ExecuteWorkflow(Run, Input{Definition: Definition{Steps: []Step{{Name: "build", Container: fixtureContainer("build")}}}})
	state := resultState(t, env)
	require.Equal(t, "completed", state.Status)
	require.Contains(t, state.ReportError, "disk full")
}

func TestTerminalFailureAndCancellationGenerateReports(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint("cancelled=", cancelled), func(t *testing.T) {
			env := environment(func(context.Context, ActivityInput) (Result, error) { return Result{}, errors.New("broken build") })
			var reports []ReportInput
			env.RegisterActivityWithOptions(func(_ context.Context, request ReportInput) error { reports = append(reports, request); return nil }, activity.RegisterOptions{Name: ReportActivity})
			step := Step{Name: "build", Container: fixtureContainer("build"), OnFailure: "end"}
			if cancelled {
				step.OnFailure = ""
				env.RegisterDelayedCallback(env.CancelWorkflow, time.Second)
			}
			env.ExecuteWorkflow(Run, Input{Task: "tasks/terminal", Definition: Definition{Steps: []Step{step}}})
			require.Error(t, env.GetWorkflowError())
			require.Len(t, reports, 1)
			require.NotEmpty(t, reports[0].Error)
			require.Len(t, reports[0].State.History, 1)
			if cancelled {
				require.Equal(t, "cancelled", reports[0].State.Status)
			} else {
				require.Equal(t, "failed", reports[0].State.Status)
			}
		})
	}
}
