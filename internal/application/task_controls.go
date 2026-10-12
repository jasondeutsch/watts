package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/jasondeutsch/watts/internal/orchestration"
)

type TemporalTaskStatus struct {
	Task       string              `json:"task"`
	WorkflowID string              `json:"workflow_id"`
	RunID      string              `json:"run_id"`
	State      orchestration.State `json:"state"`
}

func (app *Service) TemporalTaskStatus(parent context.Context, taskPath string) (TemporalTaskStatus, error) {
	binding, err := app.LoadWorkflowBinding(taskPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg, loadErr := app.LoadServiceConfig()
		if loadErr != nil {
			return TemporalTaskStatus{}, loadErr
		}
		selected, loadErr := app.TaskWorkflow(cfg, taskPath)
		if loadErr != nil {
			return TemporalTaskStatus{}, loadErr
		}
		binding.Input.Definition = selected.Definition
	} else if err != nil {
		return TemporalTaskStatus{}, err
	}
	if binding.RunID == "" {
		state := orchestration.State{Status: "not_started"}
		for _, step := range binding.Input.Definition.Steps {
			state.Stages = append(state.Stages, orchestration.Stage{Name: step.Name, Status: "pending"})
		}
		return TemporalTaskStatus{Task: taskPath, WorkflowID: binding.WorkflowID, State: state}, nil
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	temporalClient, err := ConnectTemporal(ctx, binding.Settings)
	if err != nil {
		return TemporalTaskStatus{}, err
	}
	defer temporalClient.Close()
	state, err := QueryTemporalWorkflowState(ctx, temporalClient, binding)
	if err != nil {
		return TemporalTaskStatus{}, err
	}
	return TemporalTaskStatus{Task: taskPath, WorkflowID: binding.WorkflowID, RunID: binding.RunID, State: state}, nil
}
func (app *Service) SubmitTaskEvent(parent context.Context, taskArgument string, event orchestration.Event) error {
	cfg, err := app.LoadServiceConfig()
	if err != nil {
		return err
	}
	taskPath, err := app.TaskArg(cfg, taskArgument)
	if err != nil {
		return err
	}
	binding, err := app.LoadWorkflowBinding(taskPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	temporalClient, err := ConnectTemporal(ctx, binding.Settings)
	if err != nil {
		return err
	}
	defer temporalClient.Close()
	handle, err := temporalClient.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{WorkflowID: binding.WorkflowID, RunID: binding.RunID, UpdateName: orchestration.EventUpdate, WaitForStage: client.WorkflowUpdateStageCompleted, Args: []any{event}})
	if err != nil {
		return err
	}
	if err := handle.Get(ctx, nil); err != nil {
		return fmt.Errorf("event rejected: %w", err)
	}
	return nil
}
