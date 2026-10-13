package taskreport

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

func TestReportRetainsReworkAndMissingUsage(t *testing.T) {
	directory := t.TempDir()
	write := func(attempt, name string, value any) {
		t.Helper()
		target := filepath.Join(directory, ".watts-state", "steps", attempt, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0755))
		data, err := json.Marshal(value)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(target, data, 0600))
	}
	input, output, cache := int64(100), int64(20), int64(80)
	write("build-1", "usage.json", workload.Usage{Input: &input, Output: &output, CacheRead: &cache})
	write("build-1", "events.json", []workload.Event{{Type: "usage", Usage: &workload.Usage{Provider: "provider", Model: "observed-model"}}})
	write("build-1", "workload-gates.json", workload.GateReport{Results: []workload.GateResult{{Name: "lint", Tool: "check", Status: "failed", Evidence: "tools/lint.json"}}})
	request := orchestration.ReportInput{
		Input: orchestration.Input{Task: "tasks/test", ConfigSHA256: "config-digest", Definition: orchestration.Definition{Steps: []orchestration.Step{
			{Name: "build", Agent: "generic", Model: "configured-model", Environment: map[string]string{"KEY": "SECRET_NOT_FOR_HTML"}, Gates: []workload.Gate{{Name: "lint"}}, Tools: []workload.Tool{{Name: "language-server", MCP: &workload.MCPTool{Ref: "gopls"}}}},
			{Name: "approve", Human: true}, {Name: "unused", Agent: "unused"},
		}}},
		State: orchestration.State{Status: "completed", History: []orchestration.StageResult{
			{Step: "build", Attempt: 1, Error: "bad <script>alert(1)</script>", Result: orchestration.Result{Outcome: "failure"}},
			{Step: "build", Attempt: 2, Result: orchestration.Result{Outcome: "success", Feedback: "fixed"}},
			{Step: "approve", Attempt: 1, Result: orchestration.Result{Outcome: "approved", Data: map[string]any{"approved_by": "Human"}}},
		}},
	}
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	defer root.Close()
	var html bytes.Buffer
	require.NoError(t, Render(&html, root, request))
	text := html.String()
	for _, expected := range []string{"120 (partial 1/2)", "100 (partial 1/2)", "80 (partial 1/2)", "Unknown", "provider / observed-model", "configured-model", "MCP gopls", "Decision by: Human", "bad &lt;script&gt;", "config-digest", "Gate lint · failed", "2. build", "3. approve", "Usage evidence unavailable"} {
		require.Contains(t, text, expected)
	}
	require.NotContains(t, text, "SECRET_NOT_FOR_HTML")
	require.NotContains(t, text, "<script>")
	require.NotContains(t, text, "4. unused")
	var repeated bytes.Buffer
	require.NoError(t, Render(&repeated, root, request))
	require.Equal(t, text, repeated.String())
}
