package orchestration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestQAOutcomeLoopsBackWithFeedbackAndRetainsHistory(t *testing.T) {
	codingCalls, qaCalls := 0, 0
	environment := environment(func(_ context.Context, request ActivityInput) (Result, error) {
		switch request.Step.Name {
		case "coding":
			codingCalls++
			if codingCalls == 2 && (request.Previous == nil || request.Previous.Step != "qa" || request.Previous.Feedback != "Fix missing edge case") {
				assert.Fail(t, fmt.Sprintf("missing QA feedback: %+v", request.Previous))
			}
			return Result{Outcome: "success"}, nil
		default:
			qaCalls++
			if qaCalls == 1 {
				return Result{Outcome: "changes-required", Feedback: "Fix missing edge case"}, nil
			}
			return Result{Outcome: "passed"}, nil
		}
	})
	definition := Definition{Steps: []Step{{Name: "coding", Command: "code"}, {Name: "qa", Capability: "test-suite", Transitions: map[string]string{"changes-required": "coding", "passed": "end"}}}}
	environment.ExecuteWorkflow(Run, Input{Definition: definition})
	state := resultState(t, environment)
	require.False(t, state.Status != "completed" || codingCalls != 2 || qaCalls != 2 || len(state.History) != 4,
		"state=%+v coding=%d qa=%d", state, codingCalls, qaCalls)
	require.False(t, state.History[1].Outcome != "changes-required" || state.History[3].Outcome != "passed",
		"lost outcomes: %+v", state.History)
}

func TestExternalEventsValidateAttemptOutcomeAndUniqueID(t *testing.T) {
	codingCalls := 0
	environment := environment(func(_ context.Context, request ActivityInput) (Result, error) {
		codingCalls++
		assert.False(t, codingCalls == 2 && (request.Previous == nil || request.Previous.Feedback != "CI found a regression"),
			"event feedback=%+v", request.Previous)

		return Result{}, nil
	})
	rejected := 0
	reject := func(event Event) {
		environment.UpdateWorkflow(EventUpdate, "reject-"+event.ID, &testsuite.TestUpdateCallback{
			OnReject: func(error) { rejected++ }, OnAccept: func() { assert.Fail(t, fmt.Sprintf("accepted invalid event %+v", event)) }, OnComplete: func(any, error) {},
		}, event)
	}
	environment.RegisterDelayedCallback(func() {
		state := queryState(t, environment)
		assert.False(t, state.Status != "waiting_event" || state.Current != "ci",
			"state=%+v", state)

		reject(Event{ID: "stale", Step: "ci", Attempt: 0, Outcome: "passed"})
		reject(Event{ID: "wrong-outcome", Step: "ci", Attempt: 1, Outcome: "other"})
		reject(Event{ID: "wrong-stage", Step: "coding", Attempt: 1, Outcome: "passed"})
		environment.UpdateWorkflowNoRejection(EventUpdate, "ci-1", t, Event{ID: "ci-run-1", Step: "ci", Attempt: 1, Outcome: "changes-required", Feedback: "CI found a regression"})
	}, time.Second)
	environment.RegisterDelayedCallback(func() {
		reject(Event{ID: "ci-run-1", Step: "ci", Attempt: 2, Outcome: "passed"})
		environment.UpdateWorkflowNoRejection(EventUpdate, "ci-2", t, Event{ID: "ci-run-2", Step: "ci", Attempt: 2, Outcome: "passed", Data: map[string]any{"url": "https://ci.example/run/2"}})
	}, 2*time.Second)
	definition := Definition{Steps: []Step{{Name: "coding", Command: "code"}, {Name: "ci", Event: true, Transitions: map[string]string{"changes-required": "coding", "passed": "end"}}}}
	environment.ExecuteWorkflow(Run, Input{Definition: definition})
	state := resultState(t, environment)
	require.False(t, rejected != 4 || codingCalls != 2 || len(state.History) != 4 || state.History[3].Data["url"] == nil,
		"rejected=%d calls=%d state=%+v", rejected, codingCalls, state)
}

func TestUndeclaredOutcomeWaitsForExplicitRetry(t *testing.T) {
	calls := 0
	environment := environment(func(context.Context, ActivityInput) (Result, error) {
		calls++
		if calls == 1 {
			return Result{Outcome: "surprise"}, nil
		}
		return Result{Outcome: "passed"}, nil
	})
	environment.RegisterDelayedCallback(func() {
		state := queryState(t, environment)
		assert.Equal(t, "waiting_retry", state.Status,
			"unmapped outcome silently advanced: %+v", state)

		environment.UpdateWorkflowNoRejection(RetryUpdate, "retry", t, Retry{Step: "qa", Attempt: 1})
	}, time.Second)
	environment.ExecuteWorkflow(Run, Input{Definition: Definition{Steps: []Step{{Name: "qa", Capability: "test-suite", Transitions: map[string]string{"passed": "end"}}}}})
	{
		state := resultState(t, environment)
		require.False(t, calls != 2 || len(state.History) != 2 || state.History[0].Error == "",
			"state=%+v", state)
	}
}

func TestOutcomeLoopHonorsTotalTransitionBudget(t *testing.T) {
	environment := environment(func(context.Context, ActivityInput) (Result, error) { return Result{Outcome: "again"}, nil })
	environment.ExecuteWorkflow(Run, Input{Definition: Definition{MaxTransitions: 2, Steps: []Step{{Name: "qa", Capability: "test-suite", Transitions: map[string]string{"again": "qa"}}}}})
	require.NotEqual(t, nil, environment.GetWorkflowError(),
		"an endless loop escaped its transition budget")
}

func TestRejectInvalidOutcomeDefinitions(t *testing.T) {
	for _, definition := range []Definition{
		{Steps: []Step{{Name: "qa", Capability: "test-suite", Command: "true"}}},
		{Steps: []Step{{Name: "qa", Capability: "test-suite", Transitions: map[string]string{"passed": "missing"}}}},
		{Steps: []Step{{Name: "qa", Event: true}}},
		{Steps: []Step{{Name: "qa", Event: true, Human: true, Inputs: []string{"x"}, Transitions: map[string]string{"passed": "end"}}}},
		{Steps: []Step{{Name: "qa", Command: "true", ResultFile: "../result.json"}}},
		{Templates: map[string]string{"../outside": "bad"}, Steps: []Step{{Name: "work", Command: "true"}}},
		{Steps: []Step{{Name: "qa", Capability: "test-suite", Transitions: map[string]string{"passed": "end"}}, {Name: "unused", Command: "true"}}},
	} {
		{
			err := definition.Validate()
			assert.Error(t, err,
				"accepted invalid definition %+v", definition)
		}
	}
}
