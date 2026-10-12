package application

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/containerengine"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workload"
	"github.com/stretchr/testify/require"
)

func boundTemporalTask(t *testing.T, step orchestration.Step) (*Service, orchestration.ActivityInput) {
	t.Helper()
	root, rootErr := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, rootErr)
	t.Setenv("HOME", t.TempDir())
	app := &Service{Root: root, WorkingDirectory: root, Output: io.Discard, ErrorOutput: io.Discard}
	task := "tasks/test"
	require.NoError(t, os.MkdirAll(filepath.Join(root, task), 0700))
	cfg := config.Default()
	require.NoError(t, app.SaveConfig(cfg))
	require.NoError(t, storage.WriteJSONAtomically(app.WorkflowConfigPath(task), cfg))
	data, err := os.ReadFile(app.WorkflowConfigPath(task))
	require.NoError(t, err)
	input := orchestration.Input{Task: task, Definition: orchestration.Definition{Steps: []orchestration.Step{step}}, ConfigSHA256: storage.Digest(data)}
	binding := WorkflowBinding{WorkflowID: "test-" + storage.Digest([]byte(root))[:16], RunID: "test-run", Input: input}
	require.NoError(t, storage.WriteJSONAtomically(app.WorkflowBindingPath(task), binding))
	t.Cleanup(func() {
		engine, err := containerengine.Connect()
		if err != nil {
			return
		}
		defer engine.Close()
		files, _ := filepath.Glob(filepath.Join(app.StateDir(task), "steps", "*", "*-container.json"))
		for _, name := range files {
			data, err := os.ReadFile(name)
			if err != nil {
				continue
			}
			var exit containerengine.Exit
			if json.Unmarshal(data, &exit) == nil && exit.ContainerID != "" {
				_ = engine.Remove(context.Background(), exit.ContainerID)
			}
		}
	})
	return app, orchestration.ActivityInput{Input: input, Step: step, Attempt: 1}
}
func readFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(data)
}
func requireDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("WATTS_DOCKER_INTEGRATION") != "1" {
		t.Skip("set WATTS_DOCKER_INTEGRATION=1 with local fixture images")
	}
}
func containerStep(script string) orchestration.Step {
	return orchestration.Step{Name: "execute", Container: &workload.Spec{Image: "ubuntu:latest", Command: []string{"sh", "-c", script}, Result: "exit-code", Workspace: "write"}}
}
func executeBound(t *testing.T, app *Service, request orchestration.ActivityInput) (orchestration.Result, error) {
	t.Helper()
	runtime, err := app.prepareActivity(context.Background(), request)
	require.NoError(t, err)
	_, err = runtime.prepareInput()
	require.NoError(t, err)
	return runtime.execute()
}

func TestContainerPublishesAndAcceptedAttemptDoesNotRepeat(t *testing.T) {
	requireDocker(t)
	step := containerStep(`printf x >> /workspace/count; printf report > /outputs/task/report.md`)
	step.Outputs = []string{"report.md"}
	app, request := boundTemporalTask(t, step)
	result, err := executeBound(t, app, request)
	require.NoError(t, err)
	require.Len(t, result.Artifacts, 1)
	require.Equal(t, "x", readFile(t, filepath.Join(app.Root, "count")))
	_, err = executeBound(t, app, request)
	require.NoError(t, err)
	require.Equal(t, "x", readFile(t, filepath.Join(app.Root, "count")))
	require.Equal(t, "report", readFile(t, filepath.Join(app.Root, request.Input.Task, "report.md")))
}

func TestErrorEvidenceRejectsZeroExitAndRetainsUsage(t *testing.T) {
	requireDocker(t)
	step := containerStep(`printf unsafe > /workspace/unaccepted; printf '%s\n' '{"id":"turn","type":"usage","usage":{"input_tokens":20,"output_tokens":5}}' '{"id":"error","type":"error","message":"provider rate limited"}' > /outputs/events.jsonl`)
	app, request := boundTemporalTask(t, step)
	_, err := executeBound(t, app, request)
	require.ErrorContains(t, err, "provider rate limited")
	require.NoFileExists(t, filepath.Join(app.Root, "unaccepted"))
	evidence := filepath.Dir(app.stepInputPath(request))
	require.Contains(t, readFile(t, filepath.Join(evidence, "usage.json")), `"input_tokens": 20`)
	require.Contains(t, readFile(t, filepath.Join(evidence, "events.json")), "provider rate limited")
}

func TestContainerRejectsArtifactSymlinkEscape(t *testing.T) {
	requireDocker(t)
	step := containerStep(`ln -s /etc /outputs/task/escape; printf '{"outcome":"success","artifacts":["escape/passwd"]}' > /outputs/result.json`)
	step.Container.Result = "file"
	step.Outputs = []string{"escape/passwd"}
	app, request := boundTemporalTask(t, step)
	_, err := executeBound(t, app, request)
	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(app.Root, request.Input.Task, "escape", "passwd"))
}

func TestActivityRejectsModifiedPinnedDefinitionAndSettings(t *testing.T) {
	app, request := boundTemporalTask(t, containerStep("true"))
	changed := request
	changed.Step.Name = "injected"
	_, err := app.prepareActivity(context.Background(), changed)
	require.ErrorContains(t, err, "pinned workflow")
	require.NoError(t, os.WriteFile(app.WorkflowConfigPath(request.Input.Task), []byte("{}"), 0600))
	_, err = app.prepareActivity(context.Background(), request)
	require.ErrorContains(t, err, "pinned runtime config was modified")
}

func TestResultProtocolRejectsAmbiguousAndUnsafeResults(t *testing.T) {
	for _, data := range []string{`{}`, `{"outcome":"success","unknown":true}`, `{"outcome":"success"} {}`, `{"outcome":"success","artifacts":["../escape"]}`} {
		_, err := parseStepResult([]byte(data))
		require.Error(t, err, data)
	}
	result, err := parseStepResult([]byte(`{"outcome":"changes-required","feedback":"missing test","data":{"score":2},"artifacts":["review.md"]}`))
	require.NoError(t, err)
	require.Equal(t, "missing test", result.Feedback)
	require.Equal(t, "review.md", result.Artifacts[0].Path)
	_, err = parseStepResult(bytes.Repeat([]byte("x"), maximumWorkloadResult+1))
	require.Error(t, err)
	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotEmpty(t, data)
}

func TestSubmissionCapturesResourcesAndRejectsTampering(t *testing.T) {
	app, request := boundTemporalTask(t, containerStep("true"))
	require.NoError(t, os.MkdirAll(filepath.Join(app.Root, "tools"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(app.Root, "tools", "check"), []byte("original"), 0644))
	spec := workload.Spec{Resources: map[string]string{"team": "tools"}}
	require.NoError(t, app.captureContainerResources(request.Input.Task, "check", &spec))
	require.NoError(t, os.WriteFile(filepath.Join(app.Root, "tools", "check"), []byte("changed"), 0644))
	captured := spec.Resources["team"]
	require.Equal(t, "original", readFile(t, filepath.Join(app.Root, captured, "check")))
	digest, err := workload.ResourceHash(app.Root, captured)
	require.NoError(t, err)
	require.Equal(t, spec.ResourceHashes["team"], digest)
	require.NoError(t, os.WriteFile(filepath.Join(app.Root, captured, "check"), []byte("tampered"), 0644))
	digest, err = workload.ResourceHash(app.Root, captured)
	require.NoError(t, err)
	require.NotEqual(t, spec.ResourceHashes["team"], digest)
}
