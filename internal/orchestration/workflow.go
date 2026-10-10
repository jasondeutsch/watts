package orchestration

import (
	"errors"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// workflowExecution holds replayable state for one pinned workflow. Only SDK workflow
// operations belong here; local files and subprocesses are handled by activities.
type workflowExecution struct {
	input              Input
	state              State
	currentStepIndex   int
	decision           *Decision
	event              *Event
	previous           *StageResult
	acceptedEvents     map[string]bool
	retryRequested     bool
	allowExtraAttempt  bool
	decisionInProgress bool
}

// Run is the SDK entry point. Keep the registered workflow name stable across refactors.
func Run(ctx workflow.Context, input Input) (State, error) {
	if err := input.Definition.Validate(); err != nil {
		return State{}, err
	}
	execution := workflowExecution{input: input, state: State{Status: "running"}, acceptedEvents: map[string]bool{}}
	for _, step := range input.Definition.Steps {
		execution.state.Stages = append(execution.state.Stages, Stage{Name: step.Name, Status: "pending"})
	}
	if err := execution.registerHandlers(ctx); err != nil {
		return execution.state, err
	}
	workflow.GetLogger(ctx).Info("Workflow started", "task", input.Task)
	state, err := execution.runStages(ctx)
	logger := workflow.GetLogger(ctx)
	if temporal.IsCanceledError(err) {
		logger.Warn("Workflow cancelled", "task", input.Task, "next_action", "Inspect task evidence before creating a new task")
	} else if err != nil {
		logger.Error("Workflow failed", "task", input.Task, "error", failureMessage(err), "next_action", "Inspect task status and evidence; create a new task for a terminal execution")
	} else {
		logger.Info("Workflow completed", "task", input.Task, "attempts", state.Transitions)
	}
	return state, err
}

func (execution *workflowExecution) runStages(ctx workflow.Context) (State, error) {
	var err error
	transitionLimit := execution.input.Definition.MaxTransitions
	if transitionLimit == 0 {
		transitionLimit = 100
	}
	for execution.state.Transitions < transitionLimit {
		step := execution.input.Definition.Steps[execution.currentStepIndex]
		stage := &execution.state.Stages[execution.currentStepIndex]
		execution.state.Current = step.Name
		maxAttempts := step.MaxAttempts
		if maxAttempts == 0 {
			maxAttempts = 3
		}
		if stage.Attempts >= maxAttempts && !execution.allowExtraAttempt {
			execution.state.Status = "waiting_retry"
			stage.Status = "failed"
			stage.LastError = "stage attempt limit reached"
			workflow.GetLogger(ctx).Warn("Attempt limit reached", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts, "next_action", retryCommand(execution.input.Task)+" --ignore-attempt-limit (after inspecting the failure)")
			execution.retryRequested = false
			if err = workflow.Await(ctx, func() bool { return execution.retryRequested }); err != nil {
				return execution.state, err
			}
			continue
		}
		execution.allowExtraAttempt = false
		execution.state.Status = "running"
		stage.Status = "running"
		stage.Attempts++
		stage.LastError = ""
		stage.ApprovedBy = ""
		stage.Artifacts = nil
		stage.Outcome = ""
		execution.state.Transitions++
		started := workflow.Now(ctx)
		logger := workflow.GetLogger(ctx)
		logger.Info("Stage started", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts)
		result, stageErr := execution.executeStage(ctx, step, stage)
		if result.Outcome == "" {
			result.Outcome = "success"
			if stageErr != nil {
				result.Outcome = "failure"
			}
			if step.Human && stageErr == nil {
				result.Outcome = "approved"
			}
		}
		target, transitionErr := execution.input.Definition.Target(execution.currentStepIndex, result.Outcome)
		if stageErr == nil {
			stageErr = transitionErr
		}
		stage.Outcome = result.Outcome
		record := StageResult{Step: step.Name, Attempt: stage.Attempts, Result: result}
		if stageErr != nil {
			record.Error = failureMessage(stageErr)
		}
		execution.state.History = append(execution.state.History, record)
		execution.previous = &record
		if temporal.IsCanceledError(stageErr) {
			logger.Warn("Stage cancelled", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts)
			execution.state.Status = "cancelled"
			stage.Status = "cancelled"
			return execution.state, stageErr
		}
		if stageErr != nil {
			stage.Status = "failed"
			stage.LastError = failureMessage(stageErr)
			logger.Error("Stage failed", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts, "elapsed", workflow.Now(ctx).Sub(started).String(), "error", stage.LastError)
			if failureTarget, ok := step.Transitions["failure"]; ok {
				target = failureTarget
			} else if step.OnFailure != "" {
				target = step.OnFailure
			} else if step.Optional {
				target = execution.input.Definition.successTransition(execution.currentStepIndex)
				stage.Status = "skipped"
				logger.Warn("Optional stage skipped", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts, "next_stage", target)
			} else {
				execution.state.Status = "waiting_retry"
				action := retryCommand(execution.input.Task)
				if stage.Attempts >= maxAttempts {
					action += " --ignore-attempt-limit"
				}
				logger.Warn("Waiting for retry", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts, "next_action", "Inspect the attempt log, fix the cause, then "+action)
				execution.retryRequested = false
				if err = workflow.Await(ctx, func() bool { return execution.retryRequested }); err != nil {
					return execution.state, err
				}
				continue
			}
		} else {
			stage.Status = "completed"
			logger.Info("Stage completed", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts, "elapsed", workflow.Now(ctx).Sub(started).String(), "outcome", result.Outcome)
		}
		logger.Info("Workflow routing", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts, "outcome", result.Outcome, "next_stage", target)
		if target == "end" {
			// A failure transition to end is a failure, not a passing completion.
			if stageErr != nil && !step.Optional {
				execution.state.Status = "failed"
				return execution.state, errors.New(stage.LastError)
			}
			execution.state.Status = "completed"
			execution.state.Current = ""
			return execution.state, nil
		}
		for candidateIndex, candidateStep := range execution.input.Definition.Steps {
			if candidateStep.Name == target {
				execution.currentStepIndex = candidateIndex
				break
			}
		}
	}
	execution.state.Status = "failed"
	return execution.state, errors.New("workflow transition limit reached")
}

func (execution *workflowExecution) executeStage(ctx workflow.Context, step Step, stage *Stage) (Result, error) {
	timeout := time.Hour
	if step.TimeoutSeconds > 0 {
		timeout = time.Duration(step.TimeoutSeconds) * time.Second
	}
	activityOptions := workflow.ActivityOptions{StartToCloseTimeout: timeout, HeartbeatTimeout: 30 * time.Second, WaitForCancellation: true, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}}
	activityContext := workflow.WithActivityOptions(ctx, activityOptions)
	var result Result
	var stageErr error
	if step.Event {
		if len(step.Inputs) > 0 {
			stageErr = workflow.ExecuteActivity(activityContext, HashActivity, ActivityInput{Input: execution.input, Step: step, Attempt: stage.Attempts}).Get(activityContext, &result)
			if stageErr != nil {
				return result, stageErr
			}
			stage.Artifacts = result.Artifacts
		}
		stage.Status = "waiting_event"
		execution.state.Status = "waiting_event"
		workflow.GetLogger(ctx).Info("Waiting for event", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts, "next_action", "Deliver a declared outcome through the Temporal event update")
		execution.event = nil
		stageErr = workflow.Await(ctx, func() bool { return execution.event != nil })
		if stageErr == nil {
			result = Result{Outcome: execution.event.Outcome, Feedback: execution.event.Feedback, Data: execution.event.Data, Artifacts: result.Artifacts}
		}
	} else if step.Human {
		stageErr = workflow.ExecuteActivity(activityContext, HashActivity, ActivityInput{Input: execution.input, Step: step, Attempt: stage.Attempts, Previous: execution.previous}).Get(activityContext, &result)
		if stageErr == nil {
			stage.Artifacts = result.Artifacts
			stage.Status = "waiting_approval"
			execution.state.Status = "waiting_approval"
			workflow.GetLogger(ctx).Info("Waiting for approval", "task", execution.input.Task, "stage", step.Name, "attempt", stage.Attempts, "next_action", "Review the input artifacts, then watts task decide "+quotedTask(execution.input.Task)+" "+step.Name)
			execution.decision = nil
			stageErr = workflow.Await(ctx, func() bool { return execution.decision != nil })
			if stageErr == nil {
				result.Feedback = execution.decision.Feedback
				result.Data = map[string]any{"approved_by": execution.decision.By}
				if execution.decision.Reject {
					result.Outcome = "rejected"
					if _, routed := step.Transitions["rejected"]; !routed {
						stageErr = errors.New("human decision: rejected")
					}
				} else {
					stage.ApprovedBy = execution.decision.By
					result.Outcome = "approved"
				}
			}
		}
	} else {
		stageErr = workflow.ExecuteActivity(activityContext, ExecuteActivity, ActivityInput{Input: execution.input, Step: step, Attempt: stage.Attempts, Previous: execution.previous}).Get(activityContext, &result)
		stage.Artifacts = result.Artifacts
	}
	return result, stageErr
}

// failureMessage preserves the activity’s explanation without SDK execution metadata.
func failureMessage(err error) string {
	var failure *temporal.ApplicationError
	if errors.As(err, &failure) {
		return failure.Message()
	}
	return err.Error()
}

func quotedTask(task string) string {
	return "'" + strings.ReplaceAll(task, "'", "'\"'\"'") + "'"
}

func retryCommand(task string) string { return "watts task run " + quotedTask(task) }
