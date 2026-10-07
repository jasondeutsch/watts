package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

func (app *Service) StartTemporalWorkflow(config projectconfig.Config, taskPath string, flags *RunOptions) error {
	// Serialize first submission so two clients cannot bind different definitions to one task.
	if !flags.DryRun {
		if err := os.MkdirAll(app.StateDir(taskPath), 0700); err != nil {
			return err
		}
		lock, err := os.OpenFile(filepath.Join(app.StateDir(taskPath), "temporal-start.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		defer lock.Close()
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return errors.New("another client is submitting this task; wait for it to finish")
		}
		defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	}
	if flags.DryRun {
		settings := app.ResolvedTemporalSettings(config)
		var definition orchestration.Definition
		binding, err := app.LoadWorkflowBinding(taskPath)
		if err == nil {
			definition, settings = binding.Input.Definition, binding.Settings
		} else if errors.Is(err, os.ErrNotExist) {
			selected, err := app.TaskWorkflow(config, taskPath)
			if err != nil {
				return err
			}
			definition = selected.Definition
		} else {
			return err
		}
		if err := definition.Validate(); err != nil {
			return err
		}
		app.say("Temporal workflow on %s, namespace %s, queue %s", settings.Address, settings.Namespace, settings.TaskQueue)
		encoder := json.NewEncoder(app.Output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(definition)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	binding, err := app.LoadWorkflowBinding(taskPath)
	if err == nil && binding.RunID != "" {
		app.say("Temporal workflow already submitted: %s (run %s). Use: watts task status %s", binding.WorkflowID, binding.RunID, taskPath)
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		binding, err = app.CreateWorkflowBinding(config, taskPath, flags)
		if err != nil {
			return err
		}
	}
	temporalClient, err := ConnectTemporal(ctx, binding.Settings)
	if err != nil {
		return err
	}
	defer temporalClient.Close()
	run, err := temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: binding.WorkflowID, TaskQueue: binding.Settings.TaskQueue, WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE, WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING}, orchestration.WorkflowName, binding.Input)
	if err != nil {
		var already *serviceerror.WorkflowExecutionAlreadyStarted
		if !errors.As(err, &already) {
			return err
		}
		execution, describeErr := temporalClient.DescribeWorkflowExecution(ctx, binding.WorkflowID, "")
		if describeErr != nil {
			return describeErr
		}
		binding.RunID = execution.WorkflowExecutionInfo.Execution.RunId
	} else {
		binding.RunID = run.GetRunID()
	}
	if err = storage.WriteJSONAtomically(app.WorkflowBindingPath(taskPath), binding); err != nil {
		return fmt.Errorf("workflow started but recording its run ID failed: %w", err)
	}
	app.say("Started Temporal workflow %s (run %s).", binding.WorkflowID, binding.RunID)
	app.say("Worker queue: %s. Start its worker with: watts start", binding.Settings.TaskQueue)
	app.say("Inspect progress with: watts task status %s", taskPath)
	return nil
}

func (app *Service) retryFailedStage(taskPath string, flags *RunOptions) error {
	binding, err := app.LoadWorkflowBinding(taskPath)
	if err != nil {
		return err
	}
	if binding.RunID == "" {
		return errors.New("this task has not been submitted to Temporal; use watts task run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	temporalClient, err := ConnectTemporal(ctx, binding.Settings)
	if err != nil {
		return err
	}
	defer temporalClient.Close()
	state, err := QueryTemporalWorkflowState(ctx, temporalClient, binding)
	if err != nil {
		return err
	}
	if state.Status == "completed" {
		app.say("Nothing to run: every workflow stage is complete.")
		return nil
	}
	if state.Status != "waiting_retry" {
		return fmt.Errorf("workflow is %s at %s; an explicit retry is available only for a failed stage", state.Status, state.Current)
	}
	stage := state.Current
	attempt := 0
	for _, stageState := range state.Stages {
		if stageState.Name == stage {
			attempt = stageState.Attempts
		}
	}
	handle, err := temporalClient.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{WorkflowID: binding.WorkflowID, RunID: binding.RunID, UpdateName: orchestration.RetryUpdate, WaitForStage: client.WorkflowUpdateStageCompleted, Args: []any{orchestration.Retry{Step: stage, Attempt: attempt, Force: flags.Force}}})
	if err != nil {
		return err
	}
	if err = handle.Get(ctx, nil); err != nil {
		return err
	}
	app.say("Requested retry of %s. Inspect progress with: watts task status %s", stage, taskPath)
	return nil
}

func (app *Service) CancelTemporalWorkflow(taskPath string) error {
	binding, err := app.LoadWorkflowBinding(taskPath)
	if err != nil {
		return err
	}
	if binding.RunID == "" {
		return errors.New("this task has not been submitted to Temporal; use watts task run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	temporalClient, err := ConnectTemporal(ctx, binding.Settings)
	if err != nil {
		return err
	}
	defer temporalClient.Close()
	if err = temporalClient.CancelWorkflow(ctx, binding.WorkflowID, binding.RunID); err != nil {
		return err
	}
	app.say("Requested Temporal cancellation for %s. The worker will stop active subprocesses.", taskPath)
	return nil
}
