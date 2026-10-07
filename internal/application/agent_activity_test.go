package application

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

// These tests exercise worker activities through the SDK, never a second task runner.
func executeAgentActivity(t *testing.T, project *testApp, task, agent string, offline bool) (int, string, string) {
	prompt := "/ralph --path ./" + task
	if agent == "review" {
		prompt += "/review"
	}
	cfg := loadCfg(t, project)
	resolved, _ := cfg.Agent(agent)
	step := orchestration.Step{Name: agent, Agent: agent, Provider: resolved.Provider, Model: resolved.Model, Prompt: prompt}
	if agent == "review" {
		if code, out, errs := executeCheckActivity(t, project, task, "pre-review"); code != 0 {
			return code, out, errs
		}
		step.Outputs = []string{"review/VERDICT.md"}
		step.Checks = []orchestration.Check{{Script: "check-verdict", Args: []string{"--require-pass"}}}
	}
	return executeActivity(t, project, task, step, offline)
}

func executeCheckActivity(t *testing.T, project *testApp, task, check string) (int, string, string) {
	return executeActivity(t, project, task, orchestration.Step{Name: check, Check: check}, false)
}

func executeActivity(t *testing.T, project *testApp, task string, step orchestration.Step, offline bool) (int, string, string) {
	t.Helper()
	cfg := loadCfg(t, project)
	cfg.Workflow, cfg.DefaultWorkflow = &orchestration.Definition{Steps: []orchestration.Step{step}}, ""

	require.NoError(t, storage.WriteJSONAtomically(project.WorkflowConfigPath(task), cfg))

	data, err := os.ReadFile(project.WorkflowConfigPath(task))
	require.NoError(t, err)

	input := orchestration.Input{Task: task, Definition: *cfg.Workflow, ConfigSHA256: kit.Digest(data), Offline: offline}

	require.NoError(t, storage.WriteJSONAtomically(project.WorkflowBindingPath(task), WorkflowBinding{WorkflowID: "sdk-test", Settings: project.ResolvedTemporalSettings(cfg), Input: input}))

	var stdout, stderr bytes.Buffer
	service := *project.Service
	service.Output, service.ErrorOutput = &stdout, &stderr
	service.Report = nil
	service.Report = testService(service.Root, service.Input, &stdout, &stderr).Report
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestActivityEnvironment()
	environment.RegisterActivity(service.ExecuteWorkflowStage)
	_, err = environment.ExecuteActivity(service.ExecuteWorkflowStage, orchestration.ActivityInput{Input: input, Step: step, Attempt: 1})
	if err != nil {
		return 1, stdout.String(), stderr.String() + err.Error()
	}
	return 0, stdout.String(), stderr.String()
}
