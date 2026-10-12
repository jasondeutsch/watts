package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"go.temporal.io/sdk/client"
)

// DecisionOptions records declared delegation and the execution snapshot reviewed
// by the caller. Identity labels provide provenance, not authentication.
type DecisionOptions struct {
	Reject         bool
	Feedback       string
	Actor          string
	AuthorizedBy   string
	ReviewedStatus *TemporalTaskStatus
}

func (app *Service) SubmitTaskDecision(ctx context.Context, task, stage string, reject bool, feedback ...string) (orchestration.Decision, error) {
	options := DecisionOptions{Reject: reject}
	if len(feedback) > 0 {
		options.Feedback = feedback[0]
	}
	return app.SubmitTaskDecisionWithOptions(ctx, task, stage, options)
}

func (options DecisionOptions) validate() error {
	if options.Actor == "" && options.AuthorizedBy == "" {
		return nil
	}
	if !validDecisionIdentity(options.Actor) || !validDecisionIdentity(options.AuthorizedBy) {
		return errors.New("delegated decisions require both --actor and --authorized-by as nonempty, single-line identities")
	}
	if options.ReviewedStatus == nil {
		return errors.New("delegated decisions require --reviewed-status from the pending approval reviewed by the coordinator")
	}
	return nil
}

func validDecisionIdentity(identity string) bool {
	return strings.TrimSpace(identity) != "" && !strings.ContainsAny(identity, "\r\n\x00")
}

func reviewedApproval(view TemporalTaskStatus, binding WorkflowBinding, stage orchestration.Stage) error {
	if view.Task != binding.Input.Task || view.WorkflowID != binding.WorkflowID || view.RunID != binding.RunID ||
		view.State.Status != "waiting_approval" || view.State.Current != stage.Name {
		return errors.New("reviewed status does not match the pending execution; inspect current status and artifacts again")
	}
	for _, reviewed := range view.State.Stages {
		if reviewed.Name == stage.Name && reviewed.Status == "waiting_approval" && reviewed.Attempts == stage.Attempts && slices.Equal(reviewed.Artifacts, stage.Artifacts) {
			return nil
		}
	}
	return errors.New("reviewed approval attempt or artifact hashes changed; inspect current status and artifacts again")
}

func (app *Service) SubmitTaskDecisionWithOptions(parent context.Context, taskArgument, stageName string, options DecisionOptions) (orchestration.Decision, error) {
	if os.Getenv("WATTS_ROLE") != "" {
		return orchestration.Decision{}, errors.New("a human decision cannot be submitted from inside a Watts agent")
	}
	if err := options.validate(); err != nil {
		return orchestration.Decision{}, err
	}
	config, err := app.LoadServiceConfig()
	if err != nil {
		return orchestration.Decision{}, err
	}
	taskPath, err := app.SelectTask(config, taskArgument)
	if err != nil {
		return orchestration.Decision{}, err
	}
	binding, err := app.LoadWorkflowBinding(taskPath)
	if err != nil {
		return orchestration.Decision{}, err
	}
	actor := options.Actor
	if actor == "" {
		actor = os.Getenv("USER")
		if actor == "" {
			actor = os.Getenv("LOGNAME")
		}
	}
	if !validDecisionIdentity(actor) {
		return orchestration.Decision{}, errors.New("set USER to the single-line identity of the human making this decision")
	}
	for _, step := range binding.Input.Definition.Steps {
		if step.Identity != "" && (actor == step.Identity || options.AuthorizedBy == step.Identity) {
			return orchestration.Decision{}, errors.New("a workload agent identity cannot submit or authorize a human decision")
		}
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	temporalClient, err := ConnectTemporal(ctx, binding.Settings)
	if err != nil {
		return orchestration.Decision{}, err
	}
	defer temporalClient.Close()
	state, err := QueryTemporalWorkflowState(ctx, temporalClient, binding)
	if err != nil {
		return orchestration.Decision{}, err
	}
	if stageName == "" {
		stageName = state.Current
	}
	if state.Status != "waiting_approval" || state.Current != stageName {
		return orchestration.Decision{}, fmt.Errorf("no pending approval for %s (workflow is %s at %s)", stageName, state.Status, state.Current)
	}
	var stage orchestration.Stage
	for _, candidate := range state.Stages {
		if candidate.Name == stageName {
			stage = candidate
			break
		}
	}
	if options.ReviewedStatus != nil {
		if err := reviewedApproval(*options.ReviewedStatus, binding, stage); err != nil {
			return orchestration.Decision{}, err
		}
	}
	decision := orchestration.Decision{Step: stage.Name, Attempt: stage.Attempts, By: actor, AuthorizedBy: options.AuthorizedBy, Reject: options.Reject, Feedback: options.Feedback, Artifacts: stage.Artifacts}
	handle, err := temporalClient.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{WorkflowID: binding.WorkflowID, RunID: binding.RunID, UpdateName: orchestration.DecisionUpdate, WaitForStage: client.WorkflowUpdateStageCompleted, Args: []any{decision}})
	if err != nil {
		return orchestration.Decision{}, err
	}
	if err = handle.Get(ctx, nil); err != nil {
		return orchestration.Decision{}, err
	}
	return decision, nil
}
