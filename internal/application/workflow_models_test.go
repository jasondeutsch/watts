package application

import (
	"path/filepath"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestWorkerUsesCurrentStageModelForSharedAgent(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	cfg := loadCfg(t, project)
	cfg.MCPs = map[string]projectconfig.MCPServer{"gopls": {Command: "gopls", Args: []string{"mcp"}}}
	definition := orchestration.Definition{Steps: []orchestration.Step{
		{Name: "first", MCPs: []string{"gopls"}, Agent: "build", Provider: "ollama", Model: "first:7b", Prompt: "First stage"},
		{Name: "second", Agent: "build", Provider: "ollama", Model: "second:7b", Prompt: "Second stage"},
	}}
	cfg.Workflow = &definition
	cfg = cfg.WithWorkflowAgents(definition)
	task, err := project.CreateTask(cfg, "models", "")
	require.NoError(t, err)
	customPi(t, `case "$2" in "First stage"*) stage=first ;; *) stage=second ;; esac
printf '%s\n' "$*" >> model-launches.txt
cp "$PI_CODING_AGENT_DIR/mcp.json" "$WATTS_TASK/$stage-mcp.json"
cp "$PI_CODING_AGENT_DIR/settings.json" "$WATTS_TASK/$stage-settings.json"
cp "$PI_CODING_AGENT_DIR/models.json" "$WATTS_TASK/$stage-models.json"`)
	binding, err := project.CreateWorkflowBinding(cfg, task, &RunOptions{})
	require.NoError(t, err)
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(project.ExecuteWorkflowStage, activity.RegisterOptions{Name: orchestration.ExecuteActivity})
	env.ExecuteWorkflow(orchestration.Run, binding.Input)
	require.NoError(t, env.GetWorkflowError())
	require.Contains(t, readFile(t, filepath.Join(project.Root, task, "first-mcp.json")), `"gopls"`)
	require.NotContains(t, readFile(t, filepath.Join(project.Root, task, "second-mcp.json")), "gopls")
	launches := readFile(t, filepath.Join(project.Root, "model-launches.txt"))
	require.Contains(t, launches, "--provider ollama --model first:7b")
	require.Contains(t, launches, "--provider ollama --model second:7b")
	for _, stage := range []string{"first", "second"} {
		require.Contains(t, readFile(t, filepath.Join(project.Root, task, stage+"-settings.json")), `"defaultModel": "`+stage+`:7b"`)
		require.Contains(t, readFile(t, filepath.Join(project.Root, task, stage+"-models.json")), `"id": "`+stage+`:7b"`)
	}
}

func TestModelSelectionFieldsAreRejectedInProjectConfig(t *testing.T) {
	project := newProject(t)
	for _, field := range []string{"provider", "build_model", "review_model"} {
		writeFile(t, project.ConfigPath(), `kind: Project
schema_version: 1
name: test
spec:
  version: 1
`+"  "+field+`: obsolete`)
		_, err := project.LoadConfig()
		require.ErrorContains(t, err, field)
	}
}
