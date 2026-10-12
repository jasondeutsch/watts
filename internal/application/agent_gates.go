package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"

	"github.com/jasondeutsch/watts/internal/workload"
)

// Gate evidence is supplied by the workload executor and bound to its final
// candidate. Watts consumes this common contract without knowing an agent SDK.
func (runtime *activityRuntime) validateAgentGates(attempt attemptWorkspace) error {
	gates := runtime.request.Step.Gates
	if len(gates) == 0 {
		return nil
	}
	data, _, err := workload.ReadFile(attempt.output, "gates.json", workload.MaximumFileBytes)
	if err != nil {
		return fmt.Errorf("Agent must publish final gate evidence: %w", err)
	}
	var report workload.GateReport
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&report); err != nil {
		return fmt.Errorf("invalid gate report: %w", err)
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one gate report")
	}
	candidate, err := workload.Snapshot(attempt.candidate, nil)
	if err != nil {
		return err
	}
	if err = report.Validate(runtime.executionIdentity(), gates, candidate); err != nil {
		return err
	}
	outputs, err := workload.Snapshot(filepath.Join(attempt.output, "task"), nil)
	if err != nil {
		return err
	}
	if report.TaskOutputs == nil || !reflect.DeepEqual(report.TaskOutputs, outputs) {
		return fmt.Errorf("gate evidence is stale: task outputs changed")
	}
	tools := map[string]workload.Tool{}
	for _, tool := range runtime.request.Step.Tools {
		tools[tool.Name] = tool
	}
	for _, gate := range report.Results {
		data, _, err := workload.ReadFile(attempt.output, gate.Evidence, maximumWorkloadResult)
		if err != nil {
			return fmt.Errorf("gate %s evidence: %w", gate.Name, err)
		}
		var call struct {
			Tool       string   `json:"tool"`
			Argv       []string `json:"argv"`
			WorkingDir string   `json:"working_dir"`
			Status     string   `json:"status"`
			ExitCode   *int     `json:"exit_code"`
		}
		if err = json.Unmarshal(data, &call); err != nil {
			return err
		}
		command := tools[gate.Tool].Command
		if command == nil {
			return fmt.Errorf("gate %s has no declared command tool", gate.Name)
		}
		dir := command.WorkingDir
		if dir == "" {
			dir = "/workspace"
		}
		if call.Tool != gate.Tool || call.ExitCode == nil || *call.ExitCode != *gate.ExitCode || call.Status != "exited" || call.WorkingDir != dir || !reflect.DeepEqual(call.Argv, command.Argv) {
			return fmt.Errorf("gate %s evidence does not match declared tool", gate.Name)
		}
	}
	return nil
}

func preserveToolEvidence(attempt attemptWorkspace) error {
	// Missing directories are handled by the caller; CopyResource rejects escaped
	// symlinks and oversized files just like other published workload artifacts.
	return workload.CopyResource(attempt.output, "tools", filepath.Join(attempt.evidence, "tools"))
}
