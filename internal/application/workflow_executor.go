package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jasondeutsch/watts/internal/containerengine"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/redact"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workerstate"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/jasondeutsch/watts/internal/workspace"
)

type attemptPreparation struct {
	InputHash            string                              `json:"input_sha256"`
	Baseline             map[string]workload.File            `json:"baseline"`
	Image                string                              `json:"image"`
	BaselineMeasurements map[string]workload.FileMeasurement `json:"baseline_measurements,omitempty"`
	MeasurementsVersion  int                                 `json:"measurements_version,omitempty"`
}
type acceptedAttempt struct {
	Execution string               `json:"execution"`
	InputHash string               `json:"input_sha256"`
	Result    orchestration.Result `json:"result"`
}
type attemptWorkspace struct {
	root, input, output, private, candidate, evidence string
	preparation                                       attemptPreparation
	secrets                                           *redact.Set
}

func (runtime *activityRuntime) attemptRoot() (string, error) {
	store, err := workerstate.Default()
	if err != nil {
		return "", err
	}
	return store.AttemptDir(runtime.project.Root, runtime.request.Input.Task, runtime.runIdentity(), runtime.executionIdentity()), nil
}

func (runtime *activityRuntime) execute() (result orchestration.Result, runErr error) {
	step := runtime.request.Step
	secrets := runtime.secretRedaction()
	defer func() { runErr = secrets.Error(runErr) }()
	if step.Container == nil {
		return result, fmt.Errorf("stage %s requires a container", step.Name)
	}
	root, err := runtime.attemptRoot()
	if err != nil {
		return result, err
	}
	attempt := attemptWorkspace{secrets: secrets, root: root, input: filepath.Join(root, "inputs"), output: filepath.Join(root, "outputs"), private: filepath.Join(root, "private"), candidate: filepath.Join(root, "workspace"), evidence: filepath.Dir(runtime.project.stepInputPath(runtime.request))}
	input, err := os.ReadFile(runtime.project.stepInputPath(runtime.request))
	if err != nil {
		return result, err
	}
	contract, err := json.Marshal(step)
	if err != nil {
		return result, err
	}
	inputHash := storage.Digest(append(input, contract...))
	// The durable receipt outlives disposable containers and candidate directories.
	if data, err := os.ReadFile(filepath.Join(attempt.evidence, "accepted.json")); err == nil {
		var receipt acceptedAttempt
		if err = json.Unmarshal(secrets.JSON(data), &receipt); err != nil {
			return result, err
		}
		if receipt.Execution == runtime.executionIdentity() {
			if receipt.InputHash != inputHash {
				return result, fmt.Errorf("accepted attempt contract changed")
			}
			hashes, err := workspace.HashTaskArtifacts(runtime.project.Root, runtime.request.Input.Task, step.Outputs)
			if err != nil {
				return result, err
			}
			if !equalArtifacts(hashes, receipt.Result.Artifacts) {
				return result, fmt.Errorf("accepted attempt outputs changed; refusing to repeat execution")
			}
			runtime.evidenceReady = true
			return receipt.Result, nil
		}
	} else if !os.IsNotExist(err) {
		return result, err
	}
	engine, err := containerengine.Connect()
	if err != nil {
		return result, err
	}
	defer engine.Close()
	store, err := workerstate.Default()
	if err != nil {
		return result, err
	}
	lease, err := store.Begin(runtime.project.Root, runtime.request.Input.Task, runtime.runIdentity(), runtime.executionIdentity())
	if err != nil {
		return result, err
	}
	defer lease.Close()
	if err = runtime.prepareWorkspace(engine, &attempt, input, inputHash); err != nil {
		return result, err
	}
	var containers []string
	publicationUncertain := false
	defer func() {
		retained := true
		if runErr != nil {
			_, err := attempt.preserveEvidence()
			runErr = errors.Join(runErr, err)
			retained = err == nil
		}
		for _, id := range containers {
			if id == "" {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cleanupErr := engine.Remove(ctx, id)
			cancel()
			if cleanupErr != nil {
				retained = false
				runtime.logProgress("Container cleanup pending", "container", id, "error", cleanupErr)
			}
		}
		runtime.evidenceReady = retained && !publicationUncertain
	}()
	environment, err := runtime.containerEnvironment()
	if err != nil {
		return result, err
	}
	phases := []struct {
		name    string
		command []string
	}{}
	for i, check := range step.PreChecks {
		phases = append(phases, struct {
			name    string
			command []string
		}{fmt.Sprintf("pre-%d", i), check.Command})
	}
	phases = append(phases, struct {
		name    string
		command []string
	}{"main", step.Container.Command})
	for i, check := range step.Checks {
		phases = append(phases, struct {
			name    string
			command []string
		}{fmt.Sprintf("post-%d", i), check.Command})
	}
	for _, phase := range phases {
		spec := *step.Container
		spec.Image = attempt.preparation.Image
		spec.Command = phase.command
		var tail checkOutput
		runtime.logProgress("Container executing", "phase", phase.name, "image", spec.Image)
		stdout := secrets.Writer(io.MultiWriter(runtime.output, &tail))
		stderr := secrets.Writer(io.MultiWriter(runtime.errorOutput, &tail))
		exit, err := engine.Run(runtime.context, containerengine.Request{Identity: runtime.executionIdentity() + "/" + phase.name, Spec: spec, InputDir: attempt.input, OutputDir: attempt.output, RuntimeDir: attempt.private, WorkspaceDir: attempt.candidate, Environment: environment, Stdout: stdout, Stderr: stderr})
		err = errors.Join(err, stdout.Flush(), stderr.Flush())
		exit.Error = secrets.Text(exit.Error)
		containers = append(containers, exit.ContainerID)
		if saveErr := storage.WriteJSONAtomically(filepath.Join(attempt.evidence, phase.name+"-container.json"), exit); saveErr != nil {
			return result, errors.Join(err, saveErr)
		}
		if err != nil {
			return result, err
		}
		if exit.Code == nil {
			return result, fmt.Errorf("container %s returned no exit status", phase.name)
		}
		if *exit.Code != 0 {
			return result, fmt.Errorf("container %s exited %d: %s", phase.name, *exit.Code, strings.TrimSpace(string(tail.tail)))
		}
	}
	events, err := attempt.preserveEvidence()
	if err != nil {
		return result, err
	}
	for _, event := range events {
		if event.Type == "error" {
			return result, fmt.Errorf("workload error: %s", event.Message)
		}
	}
	if step.Container.Result == "exit-code" {
		result.Outcome = "success"
	} else {
		data, _, err := workload.ReadFile(attempt.output, "result.json", maximumWorkloadResult)
		if err != nil {
			return result, fmt.Errorf("workload must publish result.json: %w", err)
		}
		result, err = parseStepResult(secrets.JSON(data))
		if err != nil {
			return result, err
		}
	}
	if _, err = runtime.request.Input.Definition.Target(stepIndex(runtime.request), result.Outcome); err != nil {
		return result, err
	}
	if err = runtime.validateOutputs(attempt, result); err != nil {
		return result, err
	}
	if err = runtime.validateAgentGates(attempt); err != nil {
		return result, err
	}
	if err = runtime.context.Err(); err != nil {
		return result, err
	}
	if err = storage.WriteJSONAtomically(filepath.Join(attempt.evidence, "publication.json"), result); err != nil {
		return result, err
	}
	if err = attempt.rejectSecretPublication(step.Outputs, step.Container.Workspace == "write"); err != nil {
		return result, err
	}
	publicationUncertain = true
	if step.Container.Workspace == "write" {
		if err = workload.Publish(runtime.project.Root, attempt.candidate, attempt.preparation.Baseline, runtime.exclusions()); err != nil {
			return result, err
		}
	}
	for _, name := range step.Outputs {
		data, mode, err := workload.ReadFile(attempt.output, "task/"+name, workload.MaximumFileBytes)
		if err != nil {
			return result, err
		}
		if _, err = workspace.TaskArtifactOutput(runtime.project.Root, runtime.request.Input.Task, name); err != nil {
			return result, err
		}
		if err = workload.WriteFile(runtime.project.Root, filepath.Join(runtime.request.Input.Task, name), data, mode); err != nil {
			return result, err
		}
	}
	result.Artifacts, err = workspace.HashTaskArtifacts(runtime.project.Root, runtime.request.Input.Task, step.Outputs)
	if err != nil {
		return result, err
	}
	receipt := acceptedAttempt{Execution: runtime.executionIdentity(), InputHash: inputHash, Result: result}
	if err = storage.WriteJSONAtomically(filepath.Join(attempt.evidence, "accepted.json"), receipt); err != nil {
		return result, err
	}
	publicationUncertain = false
	return result, nil
}

func (runtime *activityRuntime) containerEnvironment() ([]string, error) {
	environment := []string{"HOME=/runtime/home", "TMPDIR=/runtime/tmp", "WATTS_INPUT=/inputs/input.json", "WATTS_OUTPUTS=/outputs", "WATTS_RESULT=/outputs/result.json", "WATTS_EVENTS=/outputs/events.jsonl", "WATTS_WORKSPACE=/workspace"}
	for name, value := range runtime.request.Step.Environment {
		environment = append(environment, name+"="+value)
	}
	for target, source := range runtime.request.Step.SecretEnvironment {
		value, exists := os.LookupEnv(source)
		if !exists || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("stage %s: required secret source %s is missing", runtime.request.Step.Name, source)
		}
		environment = append(environment, target+"="+value)
	}
	names := []string{}
	for target := range runtime.request.Step.SecretEnvironment {
		names = append(names, target)
	}
	sort.Strings(names)
	encoded, _ := json.Marshal(names)
	environment = append(environment, "WATTS_SECRET_ENV_NAMES="+string(encoded))
	sort.Strings(environment)
	return environment, nil
}

func (runtime *activityRuntime) validateOutputs(attempt attemptWorkspace, result orchestration.Result) error {
	step := runtime.request.Step
	for _, name := range step.Inputs {
		if contains(step.Outputs, name) {
			continue
		}
		before, _, err := workload.ReadFile(attempt.input, "task/"+name, workload.MaximumFileBytes)
		if err != nil {
			return err
		}
		after, _, err := workload.ReadFile(attempt.output, "task/"+name, workload.MaximumFileBytes)
		if err != nil || !bytes.Equal(before, after) {
			return fmt.Errorf("workload changed protected input %s", name)
		}
	}
	for _, name := range step.Outputs {
		if _, _, err := workload.ReadFile(attempt.output, "task/"+name, workload.MaximumFileBytes); err != nil {
			return fmt.Errorf("missing or unsafe declared output %s: %w", name, err)
		}
	}
	for _, artifact := range result.Artifacts {
		if !contains(step.Outputs, artifact.Path) {
			return fmt.Errorf("undeclared output artifact %s", artifact.Path)
		}
	}
	return nil
}

func (attempt attemptWorkspace) preserveEvidence() ([]workload.Event, error) {
	if candidate, err := workload.Snapshot(attempt.candidate, nil); err != nil {
		return nil, err
	} else {
		workspaceEvidence := workload.WorkspaceEvidence{
			Baseline: attempt.preparation.Baseline, Candidate: candidate,
			MeasurementsVersion:  attempt.preparation.MeasurementsVersion,
			BaselineMeasurements: attempt.preparation.BaselineMeasurements,
		}
		if workspaceEvidence.MeasurementsVersion == 1 {
			workspaceEvidence.CandidateMeasurements, err = workload.MeasureFiles(attempt.candidate, candidate)
			if err != nil {
				return nil, err
			}
		}
		metadata, err := json.Marshal(workspaceEvidence)
		if err != nil {
			return nil, err
		}
		if err = workload.WriteFile(attempt.evidence, "workspace.json", attempt.secrets.JSON(metadata), 0600); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"result.json", "events.jsonl", "gates.json"} {
		data, _, err := workload.ReadFile(attempt.output, name, workload.MaximumFileBytes)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err = workload.WriteFile(attempt.evidence, "workload-"+name, attempt.secrets.JSON(data), 0600); err != nil {
			return nil, err
		}
	}
	if _, err := os.Lstat(filepath.Join(attempt.output, "tools")); err == nil {
		if err = preserveToolEvidence(attempt); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	events, err := workload.ReadEvents(filepath.Join(attempt.evidence, "workload-events.jsonl"))
	if err != nil {
		return events, err
	}
	if err = storage.WriteJSONAtomically(filepath.Join(attempt.evidence, "events.json"), events); err != nil {
		return events, err
	}
	err = storage.WriteJSONAtomically(filepath.Join(attempt.evidence, "usage.json"), workload.Summarize(events))
	return events, err
}

func contains(values []string, value string) bool {
	for _, entry := range values {
		if entry == value {
			return true
		}
	}
	return false
}
func equalArtifacts(left, right []orchestration.Artifact) bool {
	a, _ := json.Marshal(left)
	b, _ := json.Marshal(right)
	return bytes.Equal(a, b)
}

const maximumWorkloadResult = 1 << 20

func parseStepResult(data []byte) (orchestration.Result, error) {
	var result orchestration.Result
	var wire struct {
		Outcome   string         `json:"outcome"`
		Feedback  string         `json:"feedback,omitempty"`
		Data      map[string]any `json:"data,omitempty"`
		Artifacts []string       `json:"artifacts,omitempty"`
	}
	if len(data) > maximumWorkloadResult {
		return result, fmt.Errorf("workload result exceeds 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return result, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return result, fmt.Errorf("expected exactly one result document")
	}
	if !orchestration.ValidName(wire.Outcome) {
		return result, fmt.Errorf("invalid workload outcome %q", wire.Outcome)
	}
	result.Outcome, result.Feedback, result.Data = wire.Outcome, wire.Feedback, wire.Data
	for _, name := range wire.Artifacts {
		if !orchestration.ValidArtifact(name) {
			return result, fmt.Errorf("invalid artifact %q", name)
		}
		result.Artifacts = append(result.Artifacts, orchestration.Artifact{Path: name})
	}
	return result, nil
}
