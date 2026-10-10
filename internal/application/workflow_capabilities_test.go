package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutableCapabilityReturnsEvidenceAndReceivesFeedback(t *testing.T) {
	app, cfg := workflowProject(t)
	task := "tasks/example"

	require.NoError(t, os.MkdirAll(filepath.Join(app.Root, task), 0755))

	cfg.Capabilities = map[string]projectconfig.Capability{"qa-tool": {Command: []string{"bash", "-c", `cat > "$WATTS_TASK/plugin-input.json"; printf 'test details' > "$WATTS_TASK/qa-report.md"; printf '{"outcome":"changes-required","feedback":"Fix regression","artifacts":["qa-report.md"],"data":{"failed":1}}'`}}}
	step := orchestration.Step{Name: "qa", Capability: "qa-tool", Parameters: map[string]any{"suite": "integration"}, Transitions: map[string]string{"changes-required": "end"}}
	request := orchestration.ActivityInput{Input: orchestration.Input{Task: task, Definition: orchestration.Definition{Steps: []orchestration.Step{step}}}, Step: step, Attempt: 1, Previous: &orchestration.StageResult{Step: "coding", Result: orchestration.Result{Feedback: "Added edge cases"}}}
	runtime := &activityRuntime{project: app, config: cfg, request: request, context: context.Background(), output: app.Output, errorOutput: app.ErrorOutput}
	{
		_, err := runtime.prepareInput()
		require.NoError(t, err)
	}

	result, err := runtime.execute()
	require.NoError(t, err)

	validated, err := runtime.validateResult(result)
	require.NoError(t, err)
	require.False(t, validated.Outcome != "changes-required" || validated.Feedback != "Fix regression" || len(validated.Artifacts) != 1 || validated.Artifacts[0].SHA256 == "",
		"result=%+v", validated)

	var input StepInput
	data, err := os.ReadFile(filepath.Join(app.Root, task, "plugin-input.json"))
	require.NoError(t, err)

	require.NoError(t, decodeExecutorJSON(data, &input))

	require.False(t, input.Previous == nil || input.Previous.Feedback != "Added edge cases" || input.Parameters["suite"] != "integration",
		"plugin input=%+v", input)
}

func TestCapabilityResultsRejectMalformedAndEscapingArtifacts(t *testing.T) {
	for _, data := range []string{`{}`, `{"outcome":"Bad Name"}`, `{"outcome":"passed","artifacts":["../outside"]}`, `{"outcome":"passed","unknown":1}`, `{"outcome":"passed"} {}`} {
		{
			_, err := parseStepResult([]byte(data))
			assert.Error(t, err,
				"accepted malformed result %s", data)
		}
	}
	var output boundedResult
	{
		_, err := output.Write(make([]byte, maximumCapabilityResult+1))
		require.Error(t, err,
			"unbounded plugin output accepted")
	}
}

func TestEveryAttemptRequiresFreshResultFile(t *testing.T) {
	app, cfg := workflowProject(t)
	task := "tasks/example"

	require.NoError(t, os.MkdirAll(filepath.Join(app.Root, task), 0755))

	step := orchestration.Step{Name: "qa", Command: "true", ResultFile: "qa-result.json"}
	request := orchestration.ActivityInput{Input: orchestration.Input{Task: task}, Step: step, Attempt: 2}

	require.NoError(t, os.WriteFile(filepath.Join(app.Root, task, step.ResultFile), []byte(`{"outcome":"passed"}`), 0644))

	runtime := &activityRuntime{project: app, config: cfg, request: request, context: context.Background(), output: app.Output, errorOutput: app.ErrorOutput}
	{
		_, err := runtime.prepareInput()
		require.NoError(t, err)
	}
	{
		_, err := app.readStepResult(request)
		require.Error(t, err,
			"prior attempt's result was accepted")
	}
}

func TestResultFileCannotRemoveFilesThroughSymlink(t *testing.T) {
	app, cfg := workflowProject(t)
	task := "tasks/example"

	require.NoError(t, os.MkdirAll(filepath.Join(app.Root, task), 0755))

	outside := t.TempDir()
	sentinel := filepath.Join(outside, "result.json")

	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0644))

	require.NoError(t, os.Symlink(outside, filepath.Join(app.Root, task, "linked")))

	request := orchestration.ActivityInput{Input: orchestration.Input{Task: task}, Step: orchestration.Step{Name: "qa", Command: "true", ResultFile: "linked/result.json"}, Attempt: 1}
	runtime := &activityRuntime{project: app, config: cfg, request: request, context: context.Background(), output: app.Output, errorOutput: app.ErrorOutput}
	{
		_, err := runtime.prepareInput()
		require.Error(t, err,
			"result output followed an escaping symlink")
	}

	data, err := os.ReadFile(sentinel)
	require.False(t, err != nil || string(data) != "keep",
		"outside result changed: %q %v", data, err)
}
