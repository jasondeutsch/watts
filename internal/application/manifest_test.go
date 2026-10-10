package application

import (
	"os"
	"path/filepath"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	workflowtemplates "github.com/jasondeutsch/watts/workflow-templates"
	"github.com/stretchr/testify/require"
)

func TestBundledYAMLAndGoGeneralistWorkflow(t *testing.T) {
	entries, err := workflowtemplates.Files.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := workflowtemplates.Files.ReadFile(entry.Name())
			require.NoError(t, err)
			doc, err := manifest.Decode(data, manifest.Workflow, orchestration.Definition{})
			require.NoError(t, err)
			require.NoError(t, projectconfig.Default().ValidateWorkflow(doc.Spec))
		})
	}
	example := filepath.Join("..", "..", "resources", "examples", "go_generalist")
	data, err := os.ReadFile(filepath.Join(example, "mcp", "mcp.yaml"))
	require.NoError(t, err)
	project, err := manifest.Decode(data, manifest.Project, projectconfig.Default())
	require.NoError(t, err)
	data, err = os.ReadFile(filepath.Join(example, "workflows", "go-generalist.yaml"))
	require.NoError(t, err)
	workflow, err := manifest.Decode(data, manifest.Workflow, orchestration.Definition{})
	require.NoError(t, err)
	require.NoError(t, project.Spec.ValidateWorkflow(workflow.Spec))
	require.Contains(t, workflow.Spec.Templates["PLAN.md"], "lint.sh")
	var build, lint orchestration.Step
	for _, step := range workflow.Spec.Steps {
		if step.Name == "build" {
			build = step
		}
		if step.Name == "lint" {
			lint = step
		}
	}
	require.Equal(t, []string{"gopls"}, build.MCPs)
	require.Contains(t, lint.Command, "lint.sh")
	require.NotEmpty(t, build.PreChecks)
	require.NotEmpty(t, build.Checks)
}

func TestYAMLFormattingDoesNotChangePinnedRuntime(t *testing.T) {
	app := newProject(t)
	app.initRepo(t)
	cfg := loadCfg(t, app)
	task, err := app.CreateTask(cfg, "formatting", "research")
	require.NoError(t, err)
	first, err := app.CreateWorkflowBinding(cfg, task, &RunOptions{})
	require.NoError(t, err)
	projectData := readFile(t, app.ConfigPath())
	writeFile(t, app.ConfigPath(), "# Team configuration\n"+projectData+"\n# No semantic changes\n")
	workflowData := readFile(t, app.TaskWorkflowPath(task))
	writeFile(t, app.TaskWorkflowPath(task), "# Team workflow\n"+workflowData)
	cfg, err = app.LoadConfig()
	require.NoError(t, err)
	second, err := app.CreateWorkflowBinding(cfg, task, &RunOptions{})
	require.NoError(t, err)
	require.Equal(t, first.Input, second.Input)
	require.Equal(t, first.Settings, second.Settings)
	cfg.Limits.MaxMinutes = 120
	third, err := app.CreateWorkflowBinding(cfg, task, &RunOptions{})
	require.NoError(t, err)
	require.NotEqual(t, first.Input.ConfigSHA256, third.Input.ConfigSHA256)
	require.Equal(t, 7200, third.Input.Definition.Steps[0].TimeoutSeconds)
}

func TestProjectManifestNameSurvivesSettingsUpdate(t *testing.T) {
	app := newProject(t)
	cfg := projectconfig.Default()
	require.NoError(t, manifest.Write(app.ConfigPath(), manifest.Project, "team-services", cfg))
	cfg, err := app.LoadConfig()
	require.NoError(t, err)
	cfg.TasksDir = "work"
	require.NoError(t, app.SaveConfig(cfg))
	document, err := manifest.Decode([]byte(readFile(t, app.ConfigPath())), manifest.Project, projectconfig.Config{})
	require.NoError(t, err)
	require.Equal(t, "team-services", document.Name)
	require.Equal(t, "work", document.Spec.TasksDir)
}
