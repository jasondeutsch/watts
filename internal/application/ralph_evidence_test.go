package application

import (
	"encoding/json"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRalphEvidenceRequiresCurrentSuccessfulRun(t *testing.T) {
	for _, test := range []struct {
		name, status, event string
		old                 bool
		valid               bool
	}{
		{"completed", "complete", "complete", false, true},
		{"stale", "complete", "complete", true, false},
		{"failed", "failed", "failed", false, false},
		{"missing terminal", "complete", "", false, false},
		{"mismatched token", "complete", "other", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			started := time.Now()
			runStart := started
			if test.old {
				runStart = started.Add(-time.Hour)
			}
			dir := filepath.Join(root, "tasks/example/.ralph-runner")
			require.NoError(t, os.MkdirAll(dir, 0700))
			data, _ := json.Marshal(map[string]any{"loopToken": "current", "status": test.status, "startedAt": runStart, "currentIteration": 5})
			require.NoError(t, os.WriteFile(filepath.Join(dir, "status.json"), data, 0600))
			token := "current"
			if test.event == "other" {
				token = "other"
			}
			data, _ = json.Marshal(map[string]any{"type": "runner.finished", "loopToken": token, "status": test.event})
			require.NoError(t, os.WriteFile(filepath.Join(dir, "events.jsonl"), data, 0600))
			evidence := &agentEvidence{started: started, args: []string{"-p", "/ralph --path ./tasks/example"}, runtime: &activityRuntime{project: &Service{Root: root}, output: io.Discard, request: orchestration.ActivityInput{Input: orchestration.Input{Task: "tasks/example"}}}}
			handled, err := evidence.ralphEvidence()
			require.True(t, handled)
			if test.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
