package application

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasondeutsch/watts/internal/redact"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestPrivateStateIgnoredAndAuthoredFilesVisible(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git required only for ignore-rule verification")
	}
	app := &Service{Root: t.TempDir()}
	require.NoError(t, os.WriteFile(filepath.Join(app.Root, ".gitignore"), []byte("# existing rules\n/vendor/\n"), 0644))
	require.NoError(t, app.ProtectRuntimeFiles())
	first := readFile(t, filepath.Join(app.Root, ".gitignore"))
	require.NoError(t, app.ProtectRuntimeFiles())
	require.Equal(t, first, readFile(t, filepath.Join(app.Root, ".gitignore")))
	require.Contains(t, first, "# existing rules")
	for _, path := range []string{".watts/service.log", "tasks/example/.watts-state/logs/build.log", "watts/main.acl", "tasks/example/report.html"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(app.Root, path)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(app.Root, path), []byte("fixture"), 0600))
	}
	run := func(args ...string) string {
		command := exec.Command("git", args...)
		command.Dir = app.Root
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
		return string(output)
	}
	run("init", "--quiet")
	run("add", ".")
	tracked := run("ls-files")
	require.NotContains(t, tracked, ".watts/")
	require.NotContains(t, tracked, ".watts-state/")
	require.Contains(t, tracked, "watts/main.acl")
	require.Contains(t, tracked, "tasks/example/report.html")
}

func TestSecretEvidenceAndPublication(t *testing.T) {
	root := t.TempDir()
	attempt := attemptWorkspace{output: filepath.Join(root, "outputs"), evidence: filepath.Join(root, "evidence"), candidate: filepath.Join(root, "workspace"), secrets: redact.New("synthetic-key")}
	for _, path := range []string{attempt.output, attempt.evidence, attempt.candidate} {
		require.NoError(t, os.MkdirAll(path, 0700))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(attempt.output, "tools"), 0700))
	require.NoError(t, storage.WriteJSONAtomically(filepath.Join(attempt.output, "result.json"), map[string]any{"outcome": "success", "feedback": "synthetic-key"}))
	require.NoError(t, os.WriteFile(filepath.Join(attempt.output, "events.jsonl"), []byte("{\"id\":\"error\",\"type\":\"error\",\"message\":\"synthetic-key\"}\n"), 0600))
	require.NoError(t, storage.WriteJSONAtomically(filepath.Join(attempt.output, "tools", "call.json"), map[string]any{"stderr": "synthetic-key"}))
	events, err := attempt.preserveEvidence()
	require.NoError(t, err)
	require.Equal(t, "[REDACTED]", events[0].Message)
	var bundle bytes.Buffer
	archive := tar.NewWriter(&bundle)
	for _, name := range []string{"workload-result.json", "workload-events.jsonl", "events.json", "tools/call.json"} {
		data := readFile(t, filepath.Join(attempt.evidence, name))
		require.NotContains(t, data, "synthetic-key")
		require.Contains(t, data, "[REDACTED]")
		require.NoError(t, archive.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data))}))
		_, err := archive.Write([]byte(data))
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())
	require.NotContains(t, bundle.String(), "synthetic-key", "selected evidence exported from the retained copy must remain sanitized")
	require.Contains(t, bundle.String(), `"outcome":"success"`, "redaction must preserve the workload outcome")
	require.NoError(t, os.WriteFile(filepath.Join(attempt.candidate, "main.go"), []byte("synthetic-key"), 0644))
	require.ErrorContains(t, attempt.rejectSecretPublication(nil, true), "publication refused")
	require.Equal(t, "synthetic-key", readFile(t, filepath.Join(attempt.candidate, "main.go")), "must not silently rewrite a validated candidate")
}

func TestWorkerActivitySecretErrorAndLogs(t *testing.T) {
	requireDocker(t)
	t.Setenv("FIXTURE_SECRET", "synthetic-key-123")
	step := containerStep(`printf '%s' "$TOKEN"; printf '%s' "$TOKEN" >&2; exit 9`)
	step.SecretEnvironment = map[string]string{"TOKEN": "FIXTURE_SECRET"}
	app, request := boundTemporalTask(t, step)
	var output strings.Builder
	app.Output = &output
	app.ErrorOutput = &output
	var suite testsuite.WorkflowTestSuite
	suite.SetLogger(app.WorkerLogger())
	environment := suite.NewTestActivityEnvironment()
	environment.RegisterActivity(app.ExecuteWorkflowStage)
	_, err := environment.ExecuteActivity(app.ExecuteWorkflowStage, request)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "synthetic-key-123")
	require.Contains(t, err.Error(), "[REDACTED]")
	require.NotContains(t, output.String(), "synthetic-key-123")
	require.Contains(t, output.String(), "[REDACTED]")
	require.Contains(t, output.String(), "Attempt execution failed")
	require.NotContains(t, readFile(t, filepath.Join(app.StateDir(request.Input.Task), "logs", "execute-1.log")), "synthetic-key-123")
	binding, err := json.Marshal(request)
	require.NoError(t, err)
	require.NotContains(t, string(binding), "synthetic-key-123")
}
