package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// execute dispatches each executable step through the same result contract.
func (runtime *activityRuntime) execute() (orchestration.Result, error) {
	request := runtime.request
	var err error
	for _, check := range request.Step.PreChecks {
		runtime.logProgress("Precondition check", "check", check.Script)
		if err := runtime.runCheck(check.Script, append([]string{request.Input.Task}, check.Args...)...); err != nil {
			return orchestration.Result{}, fmt.Errorf("precondition check %s: %w", check.Script, err)
		}
	}
	switch {
	case request.Step.Capability != "":
		runtime.logProgress("Capability starting", "capability", request.Step.Capability)
		return runtime.runCapability()
	case request.Step.Agent != "":
		runtime.logProgress("Agent starting", "agent", request.Step.Agent, "provider", request.Step.Provider, "model", request.Step.Model)
		err = runtime.runAgent()
	case request.Step.Command != "":
		runtime.logProgress("Command starting")
		command := exec.Command("bash", "-c", request.Step.Command)
		command.Dir = runtime.project.Root
		command.Env = append(os.Environ(), runtime.environment...)
		command.Env = append(command.Env, "WATTS_REPO="+runtime.project.Root, "WATTS_TASK="+request.Input.Task, "WATTS_ROLE="+request.Step.Name)
		command.Stdin = EmptyWorkerInput()
		var output, diagnostics checkOutput
		command.Stdout = io.MultiWriter(runtime.output, &output)
		command.Stderr = io.MultiWriter(runtime.errorOutput, &diagnostics)
		err = runtime.runProcess(command, 0)
		if err != nil {
			detail := strings.TrimSpace(string(diagnostics.tail))
			if detail == "" {
				detail = strings.TrimSpace(string(output.tail))
			}
			if detail != "" {
				err = fmt.Errorf("command: %s (%w)", detail, err)
			}
		}
	case request.Step.Check != "":
		runtime.logProgress("Check starting", "check", request.Step.Check)
		err = runtime.runCheck(request.Step.Check, append([]string{request.Input.Task}, request.Step.Args...)...)
	default:
		err = errors.New("this step must be handled by the workflow, not an execution activity")
	}
	if err != nil {
		return orchestration.Result{}, err
	}
	return runtime.project.readStepResult(request)
}

// validateResult applies routing, completion checks and evidence validation once,
// regardless of the executor that produced the result. Watts supplies artifact hashes.
func (runtime *activityRuntime) validateResult(result orchestration.Result) (orchestration.Result, error) {
	request := runtime.request
	runtime.logProgress("Validating stage result")
	if _, err := request.Input.Definition.Target(stepIndex(request), result.Outcome); err != nil {
		return result, err
	}
	for _, check := range request.Step.Checks {
		runtime.logProgress("Completion check", "check", check.Script)
		if err := runtime.runCheck(check.Script, append([]string{request.Input.Task}, check.Args...)...); err != nil {
			return result, fmt.Errorf("completion check %s: %w", check.Script, err)
		}
	}
	paths := map[string]bool{}
	for _, path := range request.Step.Outputs {
		paths[path] = true
	}
	for _, artifact := range result.Artifacts {
		paths[artifact.Path] = true
	}
	if request.Step.ResultFile != "" {
		paths[request.Step.ResultFile] = true
	}
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	artifacts, err := workspace.HashTaskArtifacts(runtime.project.Root, request.Input.Task, sorted)
	result.Artifacts = artifacts
	return result, err
}

const maximumCapabilityResult = 1024 * 1024

type boundedResult struct{ bytes.Buffer }

func (output *boundedResult) Write(data []byte) (int, error) {
	if output.Len()+len(data) > maximumCapabilityResult {
		return 0, errors.New("capability result exceeds 1 MiB")
	}
	return output.Buffer.Write(data)
}

func (runtime *activityRuntime) runCapability() (orchestration.Result, error) {
	app, cfg, request := runtime.project, runtime.config, runtime.request
	capability, ok := cfg.Capabilities[request.Step.Capability]
	if !ok || len(capability.Command) == 0 {
		return orchestration.Result{}, fmt.Errorf("unknown capability %q", request.Step.Capability)
	}
	data, err := os.ReadFile(app.stepInputPath(request))
	if err != nil {
		return orchestration.Result{}, err
	}
	process := exec.Command(capability.Command[0], capability.Command[1:]...)
	process.Dir = app.Root
	process.Env = append(os.Environ(), runtime.environment...)
	process.Env = append(process.Env, "WATTS_REPO="+app.Root, "WATTS_TASK="+request.Input.Task, "WATTS_ROLE="+request.Step.Name)
	process.Stdin = bytes.NewReader(data)
	var output boundedResult
	var diagnostics checkOutput
	process.Stdout, process.Stderr = &output, io.MultiWriter(runtime.errorOutput, &diagnostics)
	if err := runtime.runProcess(process, 0); err != nil {
		if detail := strings.TrimSpace(string(diagnostics.tail)); detail != "" {
			err = fmt.Errorf("capability %s: %s (%w)", request.Step.Capability, detail, err)
		}
		return orchestration.Result{}, err
	}
	return parseStepResult(output.Bytes())
}

func parseStepResult(data []byte) (orchestration.Result, error) {
	var wire struct {
		Outcome   string         `json:"outcome"`
		Feedback  string         `json:"feedback,omitempty"`
		Data      map[string]any `json:"data,omitempty"`
		Artifacts []string       `json:"artifacts,omitempty"`
	}
	var result orchestration.Result
	if len(data) > maximumCapabilityResult {
		return result, errors.New("capability result exceeds 1 MiB")
	}
	if err := decodeExecutorJSON(data, &wire); err != nil {
		return result, fmt.Errorf("invalid capability result: %w", err)
	}
	result.Outcome, result.Feedback, result.Data = wire.Outcome, wire.Feedback, wire.Data
	if !orchestration.ValidName(result.Outcome) {
		return result, fmt.Errorf("invalid capability outcome %q", result.Outcome)
	}
	for _, artifact := range wire.Artifacts {
		if !orchestration.ValidArtifact(artifact) {
			return result, fmt.Errorf("invalid capability artifact %q", artifact)
		}
		result.Artifacts = append(result.Artifacts, orchestration.Artifact{Path: artifact})
	}
	return result, nil
}

func (app *Service) readStepResult(request orchestration.ActivityInput) (orchestration.Result, error) {
	if request.Step.ResultFile == "" {
		return orchestration.Result{Outcome: "success"}, nil
	}
	// Hashing validates the declared result file's containment and regular-file type before reading.
	if _, err := workspace.HashTaskArtifacts(app.Root, request.Input.Task, []string{request.Step.ResultFile}); err != nil {
		return orchestration.Result{}, err
	}
	file, err := os.Open(filepath.Join(app.Root, request.Input.Task, request.Step.ResultFile))
	if err != nil {
		return orchestration.Result{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximumCapabilityResult+1))
	if err != nil {
		return orchestration.Result{}, err
	}
	return parseStepResult(data)
}

func stepIndex(request orchestration.ActivityInput) int {
	for index, step := range request.Input.Definition.Steps {
		if step.Name == request.Step.Name {
			return index
		}
	}
	return 0 // prepareActivity requires membership in the pinned definition.
}

func decodeExecutorJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("expected exactly one JSON document")
	}
	return nil
}
