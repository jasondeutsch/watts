package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// activityRuntime holds dependencies scoped to one Temporal activity. Project services
// remain shared; cancellation, environment and log streams never mutate the project.
type activityRuntime struct {
	project     *Service
	config      projectconfig.Config
	request     orchestration.ActivityInput
	context     context.Context
	output      io.Writer
	errorOutput io.Writer
	environment []string
}

func (runtime *activityRuntime) runProcess(command *exec.Cmd, limit time.Duration) error {
	return workspace.RunProcess(runtime.context, runtime.project.Wd("workflow-active-process.json"), command, limit)
}

func (runtime *activityRuntime) runCheck(script string, args ...string) error {
	if !strings.HasSuffix(script, ".sh") {
		script += ".sh"
	}
	var output, diagnostics checkOutput
	command, err := runtime.project.KitCommand(runtime.config, io.MultiWriter(runtime.output, &output), runtime.environment, script, args...)
	if err != nil {
		return err
	}
	command.Stderr = io.MultiWriter(runtime.errorOutput, &diagnostics)
	command.Stdin = EmptyWorkerInput()
	if err := runtime.runProcess(command, 0); err != nil {
		detail := strings.TrimSpace(string(diagnostics.tail))
		if stdout := strings.TrimSpace(string(output.tail)); stdout != "" {
			if detail != "" {
				detail += "\n"
			}
			detail += stdout
		}
		if detail != "" {
			return fmt.Errorf("%s (%w)", detail, err)
		}
		return err
	}
	return nil
}

// checkOutput keeps a bounded tail for the failure message; complete output still goes to the stage log.
type checkOutput struct{ tail []byte }

func (output *checkOutput) Write(data []byte) (int, error) {
	const limit = 4096
	count := len(data)
	if count >= limit {
		output.tail = append(output.tail[:0], data[count-limit:]...)
	} else {
		excess := len(output.tail) + count - limit
		if excess > 0 {
			output.tail = output.tail[excess:]
		}
		output.tail = append(output.tail, data...)
	}
	return count, nil
}

func (runtime *activityRuntime) runAgent() (err error) {
	project, request := runtime.project, runtime.request
	if err := project.configureAgents(runtime.config, []string{request.Step.Agent}, false); err != nil {
		return err
	}
	restoreMCPs, err := project.configureStageMCPs(runtime.config, request.Step)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, restoreMCPs()) }()
	if err := project.RequireCredentials(runtime.config, request.Step.Agent, request.Input.Pass); err != nil {
		return err
	}
	prompt := project.stagePrompt(strings.ReplaceAll(request.Step.Prompt, "{task}", request.Input.Task), request)
	agent, _ := runtime.config.Agent(request.Step.Agent)
	args := []string{"-p", prompt, "--provider", agent.Provider, "--model", agent.Model}
	evidence, err := runtime.beginAgentEvidence(args)
	if err != nil {
		return err
	}
	defer func() { err = evidence.finish(err) }()
	command, err := project.NewPiCmd(runtime.config, request.Step.Agent, request.Input.Pass, request.Input.Offline, args...)
	if err != nil {
		return err
	}
	command.Env = append(command.Env, runtime.environment...)
	command.Stdout, command.Stderr = runtime.output, runtime.errorOutput
	return runtime.runProcess(command, project.RunLimit(runtime.config))
}

// StepInput is the language-independent input contract shared by all executors.
type StepInput struct {
	Version    int                        `json:"version"`
	Project    string                     `json:"project"`
	Task       string                     `json:"task"`
	Step       string                     `json:"step"`
	Attempt    int                        `json:"attempt"`
	Parameters map[string]any             `json:"parameters,omitempty"`
	Inputs     []orchestration.Artifact   `json:"inputs,omitempty"`
	Previous   *orchestration.StageResult `json:"previous,omitempty"`
}

func (app *Service) stepInputPath(request orchestration.ActivityInput) string {
	return filepath.Join(app.StateDir(request.Input.Task), "steps", fmt.Sprintf("%s-%d", request.Step.Name, request.Attempt), "input.json")
}

func (runtime *activityRuntime) prepareInput() (StepInput, error) {
	app, request := runtime.project, runtime.request
	artifacts, err := workspace.HashTaskArtifacts(app.Root, request.Input.Task, request.Step.Inputs)
	if err != nil {
		return StepInput{}, err
	}
	input := StepInput{Version: 1, Project: app.Root, Task: request.Input.Task, Step: request.Step.Name, Attempt: request.Attempt, Parameters: request.Step.Parameters, Inputs: artifacts, Previous: request.Previous}
	inputPath := app.stepInputPath(request)
	resultPath := filepath.Join(filepath.Dir(inputPath), "result.json")
	if request.Step.ResultFile != "" {
		resultPath, err = workspace.TaskArtifactOutput(app.Root, request.Input.Task, request.Step.ResultFile)
		if err != nil {
			return input, err
		}
	}
	if err := storage.WriteJSONAtomically(inputPath, input); err != nil {
		return input, err
	}
	// Every attempt must produce its own result, never consume a prior attempt's outcome.
	if err := os.Remove(resultPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return input, err
	}
	runtime.environment = []string{"WATTS_STEP_INPUT=" + inputPath, "WATTS_STEP_RESULT=" + resultPath, "WATTS_REPO=" + app.Root, "WATTS_TASK=" + request.Input.Task, "WATTS_AGENT_NAME=" + request.Input.Definition.AgentIdentity("build"), "WATTS_REVIEWER_NAME=" + request.Input.Definition.AgentIdentity("review")}
	return input, nil
}

func (app *Service) stagePrompt(prompt string, request orchestration.ActivityInput) string {
	// Extension commands have their own argument grammar. Appending agent
	// instructions changes the command arguments instead of supplying context.
	if strings.HasPrefix(strings.TrimSpace(prompt), "/") {
		return prompt
	}
	contextPath := app.stepInputPath(request)
	resultPath := filepath.Join(app.Root, request.Input.Task, filepath.FromSlash(request.Step.ResultFile))
	if request.Previous != nil {
		prompt += "\nRead the previous workflow result and feedback in " + contextPath + " before starting."
	}
	if request.Step.ResultFile != "" {
		prompt += "\nWrite a JSON result to " + resultPath + " containing outcome, optional feedback, data and task-relative artifact paths. Declared outcomes: " + strings.Join(sortedOutcomes(request.Step.Transitions), ", ")
	}
	return prompt
}

func sortedOutcomes(transitions map[string]string) []string {
	var outcomes []string
	for outcome := range transitions {
		outcomes = append(outcomes, outcome)
	}
	sort.Strings(outcomes)
	return outcomes
}
