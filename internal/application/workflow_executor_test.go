package application

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

func pinActivityConfig(t *testing.T, project *Service, request *orchestration.ActivityInput, cfg projectconfig.Config) {
	t.Helper()
	require.NoError(t, storage.WriteJSONAtomically(project.WorkflowConfigPath(request.Input.Task), cfg))
	data, err := os.ReadFile(project.WorkflowConfigPath(request.Input.Task))
	require.NoError(t, err)
	request.Input.ConfigSHA256 = kit.Digest(data)
	binding, err := project.LoadWorkflowBinding(request.Input.Task)
	require.NoError(t, err)
	binding.Input = request.Input
	require.NoError(t, storage.WriteJSONAtomically(project.WorkflowBindingPath(request.Input.Task), binding))
}

func runActivity(t *testing.T, project *Service, request orchestration.ActivityInput) (orchestration.Result, error) {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestActivityEnvironment()
	environment.RegisterActivity(project.ExecuteWorkflowStage)
	value, err := environment.ExecuteActivity(project.ExecuteWorkflowStage, request)
	if err != nil {
		return orchestration.Result{}, err
	}
	var result orchestration.Result
	err = value.Get(&result)
	return result, err
}

func TestExecutorsShareResultsAndValidateOnce(t *testing.T) {
	const resultJSON = `{"outcome":"passed","feedback":"Ready","data":{"count":1},"artifacts":["output.txt"]}`
	for _, kind := range []string{"command", "agent", "check", "capability"} {
		t.Run(kind, func(t *testing.T) {
			script := `printf evidence > "$WATTS_TASK/output.txt"; printf '%s' '` + resultJSON + `' > "$WATTS_STEP_RESULT"`
			step := orchestration.Step{Name: "execute", ResultFile: "executor-result.json", Outputs: []string{"output.txt"}, Checks: []orchestration.Check{{Script: "count-validation"}}, Transitions: map[string]string{"passed": "end"}}
			switch kind {
			case "command":
				step.Command = script
			case "agent":
				step.Agent, step.Prompt = "build", "Do the work"
				step.Provider, step.Model = "ollama", "qwen2.5-coder:7b"
			case "check":
				step.Check = "produce-result"
			case "capability":
				step.Capability = "produce-result"
				step.ResultFile = ""
			}
			// Bind with a valid capability declaration before configuration validation.
			project, request := boundTemporalTask(t, orchestration.Step{Name: "execute", Command: "true"})
			cfg := loadCfg(t, project)
			cfg.Capabilities = map[string]projectconfig.Capability{"produce-result": {Command: []string{"bash", "-c", script + `; cat "$WATTS_STEP_RESULT"`}}}
			cfg.Workflow = &orchestration.Definition{Steps: []orchestration.Step{step}}
			request.Input.Definition, request.Step = *cfg.Workflow, step
			pinActivityConfig(t, project.Service, &request, cfg)
			writeFile(t, filepath.Join(project.KitDir(), "scripts", "produce-result.sh"), script)
			writeFile(t, filepath.Join(project.KitDir(), "scripts", "count-validation.sh"), `printf checked >> "$1/check-count"`)
			customPi(t, script)

			result, err := runActivity(t, project.Service, request)
			require.NoError(t, err)
			assert.Equal(t, "passed", result.Outcome)
			assert.Equal(t, "Ready", result.Feedback)
			assert.Equal(t, float64(1), result.Data["count"])
			expectedArtifacts := 1
			if step.ResultFile != "" {
				expectedArtifacts++
			}
			require.Len(t, result.Artifacts, expectedArtifacts, "declared and returned artifacts must be deduplicated")
			artifact := result.Artifacts[len(result.Artifacts)-1]
			assert.Equal(t, "output.txt", artifact.Path)
			assert.Equal(t, kit.Digest([]byte("evidence")), artifact.SHA256)
			assert.Equal(t, "checked", readFile(t, filepath.Join(project.Root, request.Input.Task, "check-count")))
			assert.NoFileExists(t, filepath.Join(project.StateDir(request.Input.Task), "state.json"))
			var persisted orchestration.Result
			data, err := os.ReadFile(filepath.Join(project.StateDir(request.Input.Task), "steps", "execute-1", "result.json"))
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &persisted))
			assert.Equal(t, result, persisted)
		})
	}
}

func TestActivityUsesPinnedAgentAndTemporalAttempt(t *testing.T) {
	project, request := boundTemporalTask(t, orchestration.Step{Name: "coding", Agent: "build", Provider: "ollama", Model: "qwen2.5-coder:7b", Prompt: "Work"})
	cfg := loadCfg(t, project)
	cfg.Agents = map[string]projectconfig.AgentConfig{"build": {Path: ".watts.agents/pinned-build"}}
	require.NoError(t, project.ConfigAgents(cfg, false))
	pinActivityConfig(t, project.Service, &request, cfg)
	// The project config now points elsewhere; the submitted activity must use its pinned config.
	changed := cfg
	changed.Agents = map[string]projectconfig.AgentConfig{"build": {Path: ".watts.agents/current-build"}}
	require.NoError(t, project.SaveConfig(changed))
	request.Attempt = 7
	customPi(t, `printf '%s' "$PI_CODING_AGENT_DIR" > "$WATTS_STEP_RESULT.agent-path"; echo activity-log`)
	originalInput, originalOutput := project.Input, project.Service.Output
	result, err := runActivity(t, project.Service, request)
	require.NoError(t, err)
	assert.Equal(t, "success", result.Outcome)
	inputPath := filepath.Join(project.StateDir(request.Input.Task), "steps", "coding-7", "result.json.agent-path")
	assert.Equal(t, filepath.Join(project.Root, ".watts.agents/pinned-build"), readFile(t, inputPath))
	assert.Contains(t, readFile(t, filepath.Join(project.StateDir(request.Input.Task), "logs", "coding-7.log")), "activity-log")
	assert.Same(t, originalInput, project.Input)
	assert.Same(t, originalOutput, project.Service.Output)
	var provenance map[string]any
	data, err := os.ReadFile(filepath.Join(project.StateDir(request.Input.Task), "runs", "coding-7.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &provenance))
	assert.Equal(t, float64(7), provenance["attempt"])
	assert.Equal(t, request.Input.ConfigSHA256, provenance["config_sha256"])
	assert.NoFileExists(t, filepath.Join(project.StateDir(request.Input.Task), "state.json"))
	runtime, err := project.prepareActivity(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, cfg.Agents, runtime.config.Agents)
}

func TestCompletionChecksCannotChangeProtectedPaths(t *testing.T) {
	project, request := boundTemporalTask(t, orchestration.Step{Name: "coding", Agent: "build", Provider: "ollama", Model: "qwen2.5-coder:7b", Prompt: "Work", Checks: []orchestration.Check{{Script: "bad-check"}}})
	customPi(t, "true")
	writeFile(t, filepath.Join(project.KitDir(), "scripts", "bad-check.sh"), `printf changed >> "$WATTS_REPO/watts.json"`)
	_, err := runActivity(t, project.Service, request)
	require.ErrorContains(t, err, "the agent changed paths it may not change: watts.json")
	assert.NoFileExists(t, filepath.Join(project.StateDir(request.Input.Task), "steps", "coding-1", "result.json"))
}
