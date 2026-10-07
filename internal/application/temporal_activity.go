package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// prepareActivity confines activities to a task submitted by this project and checks the
// local pinned runtime config. Remote workflow requests cannot supply arbitrary commands.
func (app *Service) prepareActivity(ctx context.Context, request orchestration.ActivityInput) (*activityRuntime, error) {
	taskPath, err := app.ResolveTask(filepath.Join(app.Root, filepath.FromSlash(request.Input.Task)))
	if err != nil || taskPath != request.Input.Task {
		return nil, errors.New("activity task must resolve inside this project")
	}
	binding, err := app.LoadWorkflowBinding(taskPath)
	if err != nil {
		return nil, err
	}
	actualJSON, _ := json.Marshal(request.Input)
	expectedJSON, _ := json.Marshal(binding.Input)
	if string(actualJSON) != string(expectedJSON) {
		return nil, errors.New("activity input differs from the submitted task binding")
	}
	found := false
	for _, step := range request.Input.Definition.Steps {
		left, _ := json.Marshal(step)
		right, _ := json.Marshal(request.Step)
		if string(left) == string(right) {
			found = true
		}
	}
	if !found {
		return nil, errors.New("activity step is not in the pinned workflow")
	}
	data, err := os.ReadFile(app.WorkflowConfigPath(taskPath))
	if err != nil {
		return nil, err
	}
	if kit.Digest(data) != request.Input.ConfigSHA256 {
		return nil, errors.New("the pinned runtime config was modified; restore it before retrying")
	}
	var config projectconfig.Config
	if err = json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if err = config.Validate(); err != nil {
		return nil, err
	}
	config = config.WithWorkflowAgents(orchestration.Definition{Steps: []orchestration.Step{request.Step}})
	return &activityRuntime{project: app, config: config, request: request, context: ctx, output: app.Output, errorOutput: app.ErrorOutput}, nil
}

func startActivityHeartbeat(ctx context.Context) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	activity.RecordHeartbeat(ctx)
	go func() {
		defer close(done)
		heartbeatTicker := time.NewTicker(5 * time.Second)
		defer heartbeatTicker.Stop()
		for {
			select {
			case <-heartbeatTicker.C:
				activity.RecordHeartbeat(ctx)
			case <-stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() { close(stop); <-done }
}

func (app *Service) CaptureApprovalArtifacts(ctx context.Context, request orchestration.ActivityInput) (orchestration.Result, error) {
	defer startActivityHeartbeat(ctx)()
	_, err := app.prepareActivity(ctx, request)
	if err != nil {
		return orchestration.Result{}, err
	}
	unlock, err := workspace.LockWorkflowWorkspace(app.Wd())
	if err != nil {
		return orchestration.Result{}, err
	}
	defer unlock()
	hashes, err := workspace.HashTaskArtifacts(app.Root, request.Input.Task, request.Step.Inputs)
	return orchestration.Result{Artifacts: hashes}, err
}

func (app *Service) ValidateApprovalArtifacts(ctx context.Context, request orchestration.ActivityInput) error {
	defer startActivityHeartbeat(ctx)()
	_, err := app.prepareActivity(ctx, request)
	if err != nil {
		return err
	}
	unlock, err := workspace.LockWorkflowWorkspace(app.Wd())
	if err != nil {
		return err
	}
	defer unlock()
	hashes, err := workspace.HashTaskArtifacts(app.Root, request.Input.Task, request.Step.Inputs)
	if err != nil {
		return err
	}
	actualJSON, _ := json.Marshal(hashes)
	expectedJSON, _ := json.Marshal(request.Artifacts)
	if string(actualJSON) != string(expectedJSON) {
		return errors.New("approval inputs changed while waiting; reject this stage or restore the expected contents before deciding")
	}
	if request.Decision != nil {
		if request.Decision.Reject || strings.TrimSpace(request.Decision.By) == "" {
			return errors.New("invalid approval decision")
		}
		var receipt strings.Builder
		fmt.Fprintf(&receipt, "approved_by %s\napproved_at %s\n", strings.ReplaceAll(request.Decision.By, "\n", " "), time.Now().UTC().Format(time.RFC3339))
		for _, artifact := range hashes {
			fmt.Fprintf(&receipt, "%s %s\n", artifact.Path, artifact.SHA256)
		}
		receiptPath := filepath.Join(app.StateDir(request.Input.Task), "approvals", request.Step.Name+".txt")
		if err := os.MkdirAll(filepath.Dir(receiptPath), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(receiptPath, []byte(receipt.String()), 0600); err != nil {
			return err
		}
	}
	return nil
}

func (app *Service) ExecuteWorkflowStage(ctx context.Context, request orchestration.ActivityInput) (result orchestration.Result, err error) {
	defer startActivityHeartbeat(ctx)()
	runtime, err := app.prepareActivity(ctx, request)
	if err != nil {
		return result, err
	}
	unlock, err := workspace.LockWorkflowWorkspace(app.Wd())
	if err != nil {
		return result, err
	}
	defer unlock()
	if _, err = runtime.prepareInput(); err != nil {
		return result, err
	}
	logPath := filepath.Join(app.StateDir(request.Input.Task), "logs", fmt.Sprintf("%s-%d.log", request.Step.Name, request.Attempt))
	if err = os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return result, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return result, err
	}
	defer logFile.Close()
	runtime.output = io.MultiWriter(app.Output, logFile)
	runtime.errorOutput = io.MultiWriter(app.ErrorOutput, logFile)
	protectedPaths := append([]string(nil), projectconfig.DefaultForbidden...)
	if agent, ok := runtime.config.Agent(request.Step.Agent); ok {
		protectedPaths = append(protectedPaths, agent.Forbidden...)
	}
	protectedPaths = append(protectedPaths, filepath.Join(request.Input.Task, "workflow.json"), filepath.Join(request.Input.Task, ".watts-state", "approvals"))
	for _, input := range request.Step.Inputs {
		if !projectconfig.Contains(request.Step.Outputs, input) {
			protectedPaths = append(protectedPaths, filepath.Join(request.Input.Task, input))
		}
	}
	before := app.SnapshotPaths(protectedPaths)
	output, err := runtime.execute()
	if err == nil {
		result, err = runtime.validateResult(output)
	}
	if changed := DiffSnapshots(before, app.SnapshotPaths(protectedPaths)); len(changed) > 0 {
		message := "stage changed protected paths: "
		if request.Step.Agent != "" {
			message = "the agent changed paths it may not change: "
		}
		err = errors.Join(err, errors.New(message+strings.Join(changed, ", ")))
	}
	if err == nil {
		err = storage.WriteJSONAtomically(filepath.Join(filepath.Dir(app.stepInputPath(request)), "result.json"), result)
	}
	return result, err
}
