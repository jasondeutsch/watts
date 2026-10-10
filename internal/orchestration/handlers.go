package orchestration

import (
	"errors"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// registerHandlers exposes status and validated human actions to Temporal clients.
func (execution *workflowExecution) registerHandlers(ctx workflow.Context) error {
	if err := workflow.SetQueryHandler(ctx, StatusQuery, execution.queryStatus); err != nil {
		return err
	}
	if err := workflow.SetUpdateHandlerWithOptions(ctx, DecisionUpdate, execution.acceptDecision,
		workflow.UpdateHandlerOptions{Validator: execution.validateDecision}); err != nil {
		return err
	}
	if err := workflow.SetUpdateHandlerWithOptions(ctx, RetryUpdate, execution.acceptRetry, workflow.UpdateHandlerOptions{Validator: execution.validateRetry}); err != nil {
		return err
	}
	return workflow.SetUpdateHandlerWithOptions(ctx, EventUpdate, execution.acceptEvent, workflow.UpdateHandlerOptions{Validator: execution.validateEvent})
}

func (execution *workflowExecution) queryStatus() (State, error) { return execution.state, nil }

func (execution *workflowExecution) acceptDecision(updateContext workflow.Context, request Decision) error {
	execution.decisionInProgress = true
	defer func() { execution.decisionInProgress = false }()
	activityContext := workflow.WithActivityOptions(updateContext, workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	step := execution.input.Definition.Steps[execution.currentStepIndex]
	if !request.Reject {
		if validationErr := workflow.ExecuteActivity(activityContext, ValidateActivity, ActivityInput{Input: execution.input, Step: step, Attempt: request.Attempt, Artifacts: request.Artifacts, Decision: &request}).Get(activityContext, nil); validationErr != nil {
			return validationErr
		}
	}
	workflow.GetLogger(updateContext).Info("Human decision received", "task", execution.input.Task, "stage", step.Name, "attempt", request.Attempt, "rejected", request.Reject)
	execution.decision = &request
	return nil
}

func (execution *workflowExecution) validateDecision(request Decision) error {
	stage := execution.state.Stages[execution.currentStepIndex]
	if execution.state.Status != "waiting_approval" || execution.decisionInProgress || execution.decision != nil || request.Step != stage.Name || request.Attempt != stage.Attempts {
		return errors.New("no matching pending approval")
	}
	if strings.TrimSpace(request.By) == "" || !sameArtifacts(request.Artifacts, stage.Artifacts) {
		return errors.New("approval identity or artifact hashes do not match")
	}
	return nil
}

func (execution *workflowExecution) acceptRetry(ctx workflow.Context, request Retry) error {
	workflow.GetLogger(ctx).Info("Retry authorized", "task", execution.input.Task, "stage", request.Step, "attempt", request.Attempt, "extra_attempt", request.Force)
	execution.retryRequested = true
	execution.allowExtraAttempt = request.Force
	return nil
}

func (execution *workflowExecution) validateRetry(request Retry) error {
	stage := execution.state.Stages[execution.currentStepIndex]
	step := execution.input.Definition.Steps[execution.currentStepIndex]
	if execution.state.Status != "waiting_retry" || execution.retryRequested || request.Step != stage.Name || request.Attempt != stage.Attempts {
		return errors.New("no matching failed stage to retry")
	}
	max := step.MaxAttempts
	if max == 0 {
		max = 3
	}
	if stage.Attempts >= max && !request.Force {
		return errors.New("stage attempt limit reached; use --ignore-attempt-limit after inspecting the failure")
	}
	return nil
}

func (execution *workflowExecution) validateEvent(event Event) error {
	stage := execution.state.Stages[execution.currentStepIndex]
	step := execution.input.Definition.Steps[execution.currentStepIndex]
	if execution.state.Status != "waiting_event" || execution.event != nil || event.Step != stage.Name || event.Attempt != stage.Attempts {
		return errors.New("no matching pending event stage")
	}
	if strings.TrimSpace(event.ID) == "" || len(event.ID) > 200 || execution.acceptedEvents[event.ID] {
		return errors.New("event ID must be unique and nonempty (maximum 200 characters)")
	}
	if _, ok := step.Transitions[event.Outcome]; !ok {
		return errors.New("event outcome is not declared by the waiting stage")
	}
	return nil
}

func (execution *workflowExecution) acceptEvent(ctx workflow.Context, event Event) error {
	workflow.GetLogger(ctx).Info("Event received", "task", execution.input.Task, "stage", event.Step, "attempt", event.Attempt, "outcome", event.Outcome)
	execution.acceptedEvents[event.ID] = true
	execution.event = &event
	return nil
}
