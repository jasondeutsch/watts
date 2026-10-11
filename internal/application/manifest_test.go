package application

import (
	"path/filepath"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workflowpackage"
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
	pkg, err := workflowpackage.Load(example)
	require.NoError(t, err)
	resolved, err := pkg.Render()
	require.NoError(t, err)
	definition, err := manifest.CompileWorkflow(resolved.Data)
	require.NoError(t, err)
	require.NoError(t, projectconfig.Default().WithWorkflowAgents(definition).ValidateWorkflow(definition))
	require.Contains(t, definition.Templates["PLAN.md"], "lint.sh")
	var build, lint orchestration.Step
	for _, step := range definition.Steps {
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

func TestPackageDescriptorSurvivesSettingsUpdate(t *testing.T) {
	app := newProject(t)
	cfg := projectconfig.Default()
	require.NoError(t, app.SaveConfig(cfg))
	source := "name: team-services\nversion: 0.1.0\ntemplates: templates\n"
	writeFile(t, app.ConfigPath(), source)
	cfg, err := app.LoadConfig()
	require.NoError(t, err)
	cfg.TasksDir = "work"
	require.NoError(t, app.SaveConfig(cfg))
	require.Equal(t, source, readFile(t, app.ConfigPath()))
	descriptor, err := workflowpackage.ReadDescriptor(app.ConfigPath())
	require.NoError(t, err)
	require.Equal(t, "team-services", descriptor.Name)
	settings, err := app.LoadServiceConfig()
	require.NoError(t, err)
	require.Equal(t, "work", settings.TasksDir)
}
