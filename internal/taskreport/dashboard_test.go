package taskreport

import (
	"bytes"
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

func writeReportEvidence(t *testing.T, directory, attempt, name string, value any) {
	t.Helper()
	target := filepath.Join(directory, ".watts-state", "steps", attempt, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0700))
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, encoded, 0600))
}

func TestDashboardSeparatesReworkApprovalTimeAndPublishedVolume(t *testing.T) {
	directory := t.TempDir()
	zero := int64(0)
	for attempt, count := range map[string]int64{"draft-1": 100, "draft-2": 80, "build-1": 820} {
		writeReportEvidence(t, directory, attempt, "usage.json", workload.Usage{Input: &count, Output: &zero})
		writeReportEvidence(t, directory, attempt, "events.json", []workload.Event{})
	}
	writeReportEvidence(t, directory, "draft-2", "events.json", []workload.Event{{Type: "measurement", Details: map[string]any{"name": "Complexity <script>", "kind": "complexity", "method": "team/v1", "unit": "points", "scope": "changed functions", "baseline": 0, "value": 2}}})
	writeReportEvidence(t, directory, "build-1", "accepted.json", map[string]any{"execution": "fixture/build/1", "result": map[string]any{"outcome": "success"}})
	lines := func(value int64) *int64 { return &value }
	writeReportEvidence(t, directory, "build-1", "workspace.json", workload.WorkspaceEvidence{
		Baseline:              map[string]workload.File{"a.txt": {SHA256: "before"}, "removed.txt": {SHA256: "removed"}},
		Candidate:             map[string]workload.File{"a.txt": {SHA256: "after"}, "new.txt": {SHA256: "new"}, "binary": {SHA256: "binary"}},
		MeasurementsVersion:   1,
		BaselineMeasurements:  map[string]workload.FileMeasurement{"a.txt": {Bytes: 4, TextLines: lines(2)}, "removed.txt": {Bytes: 4, TextLines: lines(2)}},
		CandidateMeasurements: map[string]workload.FileMeasurement{"a.txt": {Bytes: 6, TextLines: lines(3)}, "new.txt": {Bytes: 10, TextLines: lines(5)}, "binary": {Bytes: 2}},
	})
	exit := 0
	writeReportEvidence(t, directory, "build-1", "workload-gates.json", workload.GateReport{Results: []workload.GateResult{{Name: "test", Tool: "test", Status: "passed", ExitCode: &exit}}})
	start := time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)
	record := func(name string, attempt, from, to int, outcome string) orchestration.StageResult {
		return orchestration.StageResult{Step: name, Attempt: attempt, StartedAt: start.Add(time.Duration(from) * time.Second), FinishedAt: start.Add(time.Duration(to) * time.Second), Result: orchestration.Result{Outcome: outcome}}
	}
	request := orchestration.ReportInput{
		Input: orchestration.Input{Task: "tasks/fixture", Definition: orchestration.Definition{Steps: []orchestration.Step{
			{Name: "draft", Agent: "generic"}, {Name: "approve", Human: true},
			{Name: "build", Agent: "generic", Container: &workload.Spec{Workspace: "write"}, Gates: []workload.Gate{{Name: "test", Tool: "test"}}},
		}}},
		State: orchestration.State{Status: "completed", StartedAt: start, FinishedAt: start.Add(10 * time.Minute), History: []orchestration.StageResult{
			record("draft", 1, 0, 60, "success"), record("approve", 1, 60, 180, "rejected"), record("draft", 2, 180, 240, "success"), record("approve", 2, 240, 300, "approved"), record("build", 1, 300, 600, "success"),
		}},
	}
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	defer root.Close()
	var document bytes.Buffer
	require.NoError(t, Render(&document, root, request))
	for _, text := range []string{"10.0 min", "1,000", "8.0%", "approval stages: 3.0 min", "250.0", "+4.00", "2.00 points", "Baseline", "0.00", "team/v1", "changed functions"} {
		require.Contains(t, html.UnescapeString(document.String()), text)
	}
	require.Contains(t, document.String(), "Complexity &lt;script&gt;")
	require.NotContains(t, document.String(), "<script>")
	require.Contains(t, document.String(), "1</strong>repeat workload attempts")
	require.Contains(t, document.String(), "1 rejected / 2 human decisions")
	require.Contains(t, document.String(), "4 / 4 changed files have volume measurements")
	require.NotContains(t, document.String(), `href=".watts-state/steps/approve-1/result.json"`)
	require.NotContains(t, document.String(), `href=".watts-state/logs/approve-1.log"`)
	// Removing proof of publication makes output ratios unknown, not zero.
	require.NoError(t, os.Remove(filepath.Join(directory, ".watts-state/steps/build-1/accepted.json")))
	document.Reset()
	require.NoError(t, Render(&document, root, request))
	require.Contains(t, document.String(), "Unknown</strong>net text lines / 1,000 tokens")
}

func TestUnknownVolumeAndUsageNeverBecomeEfficiencyScores(t *testing.T) {
	changes := WorkspaceChanges{ExpectedAttempts: 1, KnownAttempts: 1, Added: 1, ExpectedFiles: 1}
	perLine, perThousand := changes.ratios(Counter{Value: 1000, Known: 1, Expected: 1})
	require.Equal(t, "Unknown", perLine)
	require.Equal(t, "Unknown", perThousand)
	changes.KnownFiles = 1
	changes.NetTextLines = 4
	perLine, _ = changes.ratios(Counter{Value: 1000, Known: 1, Expected: 2})
	require.Equal(t, "Unknown", perLine)
	changes.NetTextLines = -4
	perLine, perThousand = changes.ratios(Counter{Value: 1000, Known: 1, Expected: 1})
	require.Equal(t, "N/A (no net increase)", perLine)
	require.Equal(t, "-4.00", perThousand)
	perLine, perThousand = changes.ratios(Counter{Value: 0, Known: 1, Expected: 1})
	require.Equal(t, "N/A", perLine)
	require.Equal(t, "N/A", perThousand)
	coverage := GateCoverage{}
	definition := orchestration.Step{Gates: []workload.Gate{{Name: "test", Tool: "test"}}}
	countGates(&coverage, definition, []workload.GateResult{{Name: "test", Tool: "test", Status: "passed"}, {Name: "test", Tool: "test", Status: "passed"}})
	require.Equal(t, 1, coverage.Expected)
	require.Zero(t, coverage.Known)
}
