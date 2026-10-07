package application

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
)

func TestCheckFailuresPreserveDiagnosticOutput(t *testing.T) {
	for _, test := range []struct {
		name, script, expected string
		precondition           bool
	}{
		{"completion", "printf 'SPEC.md still contains template placeholders\\n' >&2; exit 1", "completion check explain: SPEC.md still contains template placeholders", false},
		{"precondition", "printf 'Missing input specification\\n' >&2; exit 1", "precondition check explain: Missing input specification", true},
		{"stdout", "printf 'FAIL: acceptance criterion AC-1 is missing\\n'; exit 1", "completion check explain: FAIL: acceptance criterion AC-1 is missing", false},
		{"silent", "exit 1", "completion check explain: exit status 1", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			step := orchestration.Step{Name: "validate", Command: "true"}
			check := []orchestration.Check{{Script: "explain"}}
			if test.precondition {
				step.PreChecks = check
			} else {
				step.Checks = check
			}
			project, request := boundTemporalTask(t, step)
			writeFile(t, filepath.Join(project.KitDir(), "scripts", "explain.sh"), test.script)
			_, err := runActivity(t, project.Service, request)
			require.ErrorContains(t, err, test.expected)
			if test.name != "silent" {
				require.Contains(t, readFile(t, filepath.Join(project.StateDir(request.Input.Task), "logs", "validate-1.log")), strings.SplitN(test.expected, ": ", 2)[1])
			}
		})
	}
}

func TestCheckDiagnosticTailIsBounded(t *testing.T) {
	var output checkOutput
	_, err := output.Write([]byte(strings.Repeat("x", 10000)))
	require.NoError(t, err)
	_, err = output.Write([]byte("last diagnostic"))
	require.NoError(t, err)
	require.Len(t, output.tail, 4096)
	require.True(t, strings.HasSuffix(string(output.tail), "last diagnostic"))
}
