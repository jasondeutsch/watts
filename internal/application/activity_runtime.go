package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/jasondeutsch/watts/internal/workspace"
	temporallog "go.temporal.io/sdk/log"
)

type activityRuntime struct {
	project             *Service
	config              projectconfig.Config
	request             orchestration.ActivityInput
	context             context.Context
	output, errorOutput io.Writer
	logger              temporallog.Logger
	execution           string
}
type StepInput struct {
	Version    int                        `json:"version"`
	Execution  string                     `json:"execution"`
	Task       string                     `json:"task"`
	Step       string                     `json:"step"`
	Attempt    int                        `json:"attempt"`
	Parameters map[string]any             `json:"parameters,omitempty"`
	Outputs    []string                   `json:"outputs,omitempty"`
	Inputs     []orchestration.Artifact   `json:"inputs,omitempty"`
	Previous   *orchestration.StageResult `json:"previous,omitempty"`
	Agent      *AgentInput                `json:"agent,omitempty"`
}

// AgentInput is descriptive configuration. Only the workload image interprets it.
type AgentInput struct {
	Tools      []workload.Tool              `json:"tools,omitempty"`
	Gates      []workload.Gate              `json:"gates,omitempty"`
	Skills     []workload.Skill             `json:"skills,omitempty"`
	GatePolicy *workload.GatePolicy         `json:"gate_policy,omitempty"`
	Name       string                       `json:"name"`
	Identity   string                       `json:"identity"`
	Provider   string                       `json:"provider"`
	Model      string                       `json:"model"`
	Prompt     string                       `json:"prompt"`
	MCPs       []string                     `json:"mcps,omitempty"`
	Settings   *orchestration.AgentSettings `json:"settings,omitempty"`
}

func (app *Service) stepInputPath(request orchestration.ActivityInput) string {
	return filepath.Join(app.StateDir(request.Input.Task), "steps", fmt.Sprintf("%s-%d", request.Step.Name, request.Attempt), "input.json")
}
func (runtime *activityRuntime) prepareInput() (StepInput, error) {
	request := runtime.request
	existing, readErr := os.ReadFile(runtime.project.stepInputPath(request))
	if readErr == nil {
		var input StepInput
		if err := json.Unmarshal(existing, &input); err != nil {
			return input, err
		}
		if input.Execution == runtime.executionIdentity() {
			return input, nil
		}
	} else if !os.IsNotExist(readErr) {
		return StepInput{}, readErr
	}
	artifacts, err := workspace.HashTaskArtifacts(runtime.project.Root, request.Input.Task, request.Step.Inputs)
	if err != nil {
		return StepInput{}, err
	}
	input := StepInput{Version: 1, Execution: runtime.executionIdentity(), Task: request.Input.Task, Step: request.Step.Name, Attempt: request.Attempt, Parameters: request.Step.Parameters, Inputs: artifacts, Outputs: request.Step.Outputs, Previous: request.Previous}
	if request.Step.Agent != "" {
		input.Agent = &AgentInput{Tools: request.Step.Tools, Gates: request.Step.Gates, Skills: request.Step.Skills, GatePolicy: request.Step.GatePolicy, Name: request.Step.Agent, Identity: request.Step.Identity, Provider: request.Step.Provider, Model: request.Step.Model, Prompt: request.Step.Prompt, MCPs: request.Step.MCPs, Settings: request.Step.AgentSettings}
	}
	return input, storage.WriteJSONAtomically(runtime.project.stepInputPath(request), input)
}
func (runtime *activityRuntime) executionIdentity() string {
	if runtime.execution != "" {
		return storage.Digest([]byte(runtime.project.Root))[:16] + "/" + runtime.execution + "/" + runtime.request.Step.Name + "/" + fmt.Sprint(runtime.request.Attempt)
	}
	binding, _ := runtime.project.LoadWorkflowBinding(runtime.request.Input.Task)
	return storage.Digest([]byte(runtime.project.Root))[:16] + "/" + binding.WorkflowID + "/" + binding.RunID + "/" + runtime.request.Step.Name + "/" + fmt.Sprint(runtime.request.Attempt)
}
func (runtime *activityRuntime) logProgress(message string, fields ...any) {
	fields = append([]any{"task", runtime.request.Input.Task, "stage", runtime.request.Step.Name, "attempt", runtime.request.Attempt}, fields...)
	logger := runtime.logger
	if logger == nil {
		logger = runtime.project.WorkerLogger()
	}
	logger.Info(message, fields...)
}

// A bounded diagnostic tail supplements the full durable attempt log.
type checkOutput struct{ tail []byte }

func (output *checkOutput) Write(data []byte) (int, error) {
	const limit = 4096
	n := len(data)
	output.tail = append(output.tail, data...)
	if len(output.tail) > limit {
		output.tail = output.tail[len(output.tail)-limit:]
	}
	return n, nil
}

func (runtime *activityRuntime) exclusions() []string {
	return append([]string{".git", ".watts", runtime.config.TasksDir, "watts.yaml"}, runtime.config.SnapshotExclude...)
}
func (runtime *activityRuntime) validateResult(result orchestration.Result) (orchestration.Result, error) {
	request := runtime.request
	if _, err := request.Input.Definition.Target(stepIndex(request), result.Outcome); err != nil {
		return result, err
	}
	names := append([]string{}, request.Step.Outputs...)
	for _, artifact := range result.Artifacts {
		if !projectconfig.Contains(names, artifact.Path) {
			names = append(names, artifact.Path)
		}
	}
	artifacts, err := workspace.HashTaskArtifacts(runtime.project.Root, request.Input.Task, names)
	result.Artifacts = artifacts
	return result, err
}
func stepIndex(request orchestration.ActivityInput) int {
	for i, step := range request.Input.Definition.Steps {
		if step.Name == request.Step.Name {
			return i
		}
	}
	return 0
}
