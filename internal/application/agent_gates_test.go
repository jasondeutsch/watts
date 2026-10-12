package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

func TestAgentGateAcceptanceRejectsStaleMissingAndMismatchedEvidence(t *testing.T) {
	root := t.TempDir()
	attempt := attemptWorkspace{candidate: filepath.Join(root, "candidate"), output: filepath.Join(root, "outputs")}
	require.NoError(t, os.MkdirAll(attempt.candidate, 0700))
	require.NoError(t, os.MkdirAll(filepath.Join(attempt.output, "tools"), 0700))
	require.NoError(t, os.MkdirAll(filepath.Join(attempt.output, "task"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(attempt.candidate, "main.go"), []byte("checked source"), 0644))
	gate := workload.Gate{Name: "verified", Tool: "verify"}
	runtime := activityRuntime{project: &Service{Root: root}, execution: "fixture", request: orchestration.ActivityInput{Attempt: 1, Step: orchestration.Step{Name: "agent", Tools: []workload.Tool{{Name: "verify", Command: &workload.CommandTool{Argv: []string{"verify"}}}}, Gates: []workload.Gate{gate}}}}
	snapshot, err := workload.Snapshot(attempt.candidate, nil)
	require.NoError(t, err)
	code := 0
	report := workload.GateReport{TaskOutputs: map[string]workload.File{}, Version: 1, Execution: runtime.executionIdentity(), Workspace: snapshot, Results: []workload.GateResult{{Name: gate.Name, Tool: gate.Tool, Status: "passed", ExitCode: &code, Evidence: "tools/call.json"}}}
	write := func(name string, value any) {
		t.Helper()
		data, err := json.Marshal(value)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(attempt.output, name), data, 0600))
	}
	call := map[string]any{"tool": "verify", "argv": []string{"verify"}, "working_dir": "/workspace", "status": "exited", "exit_code": 0}
	write("gates.json", report)
	write("tools/call.json", call)
	require.NoError(t, runtime.validateAgentGates(attempt))
	require.NoError(t, os.WriteFile(filepath.Join(attempt.output, "task", "verdict.md"), []byte("later edit"), 0600))
	require.ErrorContains(t, runtime.validateAgentGates(attempt), "task outputs changed")
	require.NoError(t, os.Remove(filepath.Join(attempt.output, "task", "verdict.md")))
	require.NoError(t, os.WriteFile(filepath.Join(attempt.candidate, "main.go"), []byte("edited after check"), 0644))
	require.ErrorContains(t, runtime.validateAgentGates(attempt), "stale")
	require.NoError(t, os.WriteFile(filepath.Join(attempt.candidate, "main.go"), []byte("checked source"), 0644))
	call["argv"] = []string{"replacement-check"}
	write("tools/call.json", call)
	require.ErrorContains(t, runtime.validateAgentGates(attempt), "does not match")
	call["argv"] = []string{"verify"}
	write("tools/call.json", call)
	report.Execution = "different-run"
	write("gates.json", report)
	require.ErrorContains(t, runtime.validateAgentGates(attempt), "execution")
	require.NoError(t, os.Remove(filepath.Join(attempt.output, "gates.json")))
	require.ErrorContains(t, runtime.validateAgentGates(attempt), "must publish")
}
