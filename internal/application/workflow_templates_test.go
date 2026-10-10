package application

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func workflowProject(t *testing.T) (*Service, projectconfig.Config) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	return &Service{Root: root, Input: strings.NewReader(""), Output: &bytes.Buffer{}, ErrorOutput: &bytes.Buffer{}}, projectconfig.Default()
}

func TestProjectWorkflowSelectionAndPinnedTaskDefinition(t *testing.T) {
	app, cfg := workflowProject(t)
	definition := orchestration.Definition{Version: 1, Templates: map[string]string{"proposal.md": "Research question\n"}, Steps: []orchestration.Step{{Name: "research", Command: "true"}}}

	require.NoError(t, manifest.Write(filepath.Join(app.Root, "workflows", "research.yaml"), manifest.Workflow, "test-workflow", definition))

	cfg.DefaultWorkflow = "workflows/research.yaml"
	task, err := app.CreateTask(cfg, "question", "")
	require.NoError(t, err)
	{
		_, err := os.Stat(filepath.Join(app.Root, task, "SPEC.md"))
		require.True(t, os.IsNotExist(err),
			"a research task received SDLC documents")
	}

	data, _ := os.ReadFile(filepath.Join(app.Root, task, "proposal.md"))
	require.Equal(t, "Research question\n", string(data),
		"template=%q", data)

	definition.Steps[0].Name = "changed"

	require.NoError(t, manifest.Write(filepath.Join(app.Root, "workflows", "research.yaml"), manifest.Workflow, "test-workflow", definition))

	selected, err := app.TaskWorkflow(cfg, task)
	require.NoError(t, err)
	require.Equal(t, "research", selected.Definition.Steps[0].Name,
		"editing the project changed a task's pinned definition")

	next, err := app.CreateTask(cfg, "next-question", "workflows/research.yaml")
	require.NoError(t, err)

	selected, err = app.TaskWorkflow(cfg, next)
	require.False(t, err != nil || selected.Definition.Steps[0].Name != "changed",
		"new task did not use current definition: %+v %v", selected, err)
}

func TestWorkflowFileValidation(t *testing.T) {
	app, cfg := workflowProject(t)
	for _, name := range []string{"../outside.yaml", "Bad Name", "missing.yaml"} {
		_, err := app.ResolveWorkflow(cfg, name)
		require.Error(t, err)
	}
	require.NoError(t, os.WriteFile(filepath.Join(app.Root, "custom.yaml"), []byte(`kind: Workflow
schema_version: 1
name: test
spec:
  typo: 1
  steps:
  - name: work
    command: 'true'
`), 0644))
	_, err := app.ResolveWorkflow(cfg, "custom.yaml")
	require.Error(t, err, "unknown definition fields accepted")
}

func TestTaskWorkflowCanBeCustomizedBeforeSubmission(t *testing.T) {
	app, cfg := workflowProject(t)

	require.NoError(t, manifest.Write(filepath.Join(app.Root, "custom.yaml"), manifest.Workflow, "test-workflow", orchestration.Definition{Steps: []orchestration.Step{{Name: "work", Command: "true"}}}))

	task, err := app.CreateTask(cfg, "question", "custom.yaml")
	require.NoError(t, err)

	selected, err := app.TaskWorkflow(cfg, task)
	require.NoError(t, err)

	selected.Definition.Steps[0].Command = "other"

	require.NoError(t, manifest.Write(app.TaskWorkflowPath(task), manifest.Workflow, "test-workflow", selected.Definition))

	edited, err := app.TaskWorkflow(cfg, task)
	require.NoError(t, err)
	require.Equal(t, "other", edited.Definition.Steps[0].Command)
	binding, err := app.CreateWorkflowBinding(cfg, task, &RunOptions{})
	require.NoError(t, err)
	edited.Definition.Steps[0].Command = "later"
	require.NoError(t, manifest.Write(app.TaskWorkflowPath(task), manifest.Workflow, "test-workflow", edited.Definition))
	frozen, err := app.LoadWorkflowBinding(task)
	require.NoError(t, err)
	require.Equal(t, binding.Input.Definition, frozen.Input.Definition)
	require.Equal(t, "other", frozen.Input.Definition.Steps[0].Command)
}

func TestProjectCanOverrideBundledDefaultWorkflow(t *testing.T) {
	app, cfg := workflowProject(t)
	definition := orchestration.Definition{Steps: []orchestration.Step{{Name: "custom-work", Command: "true"}}}

	require.NoError(t, manifest.Write(filepath.Join(app.Root, "workflows", "sdlc.yaml"), manifest.Workflow, "test-workflow", definition))
	cfg.DefaultWorkflow = "workflows/sdlc.yaml"

	selected, err := app.ResolveWorkflow(cfg, "")
	require.False(t, err != nil || selected.Definition.Steps[0].Name != "custom-work",
		"default override=%+v %v", selected, err)

	task, err := app.CreateTask(cfg, "question", "")
	require.NoError(t, err)
	{
		_, err := os.Stat(filepath.Join(app.Root, task, "SPEC.md"))
		require.True(t, os.IsNotExist(err),
			"overridden workflow used bundled templates")
	}
}

func TestClonedSDLCIncludesDeclaredTemplatesAndBaseline(t *testing.T) {
	app, cfg := workflowProject(t)
	err := kit.Initialize(app.KitDir())
	require.NoError(t, err)
	require.NoError(t, manifest.Write(filepath.Join(app.Root, "bugfix.yaml"), manifest.Workflow, "test-workflow", projectconfig.DefaultWorkflow()))
	task, err := app.CreateTask(cfg, "clone", "bugfix.yaml")
	require.NoError(t, err)
	for _, artifact := range []string{"workflow.yaml", "SPEC.md", "PLAN.md", "RALPH.md", "review/RALPH.md", "BASE"} {
		info, err := os.Stat(filepath.Join(app.Root, task, artifact))
		require.NoError(t, err, artifact)
		assert.Positive(t, info.Size(), artifact)
	}
	assert.NoFileExists(t, filepath.Join(app.Root, task, "REQUIREMENTS.md"))
	assert.Contains(t, readFile(t, filepath.Join(app.Root, task, "RALPH.md")), task)
	selected, err := app.TaskWorkflow(cfg, task)
	require.NoError(t, err)
	assert.Equal(t, "specification", selected.Definition.Steps[0].Name)
	require.NoError(t, os.Remove(app.TaskWorkflowPath(task)))
	_, err = app.TaskWorkflow(cfg, task)
	require.Error(t, err, "missing task workflow must not fall back to a project template")
}

func TestBundledDefaultsAreExplicitAgentStages(t *testing.T) {
	app, cfg := workflowProject(t)
	for _, name := range []string{"sdlc", "research"} {
		selected, err := app.ResolveWorkflow(cfg, name)
		require.NoError(t, err)
		for _, step := range selected.Definition.Steps {
			if step.Human {
				continue
			}
			require.NotEmpty(t, step.Agent, step.Name)
			require.NotEmpty(t, step.Identity, step.Name)
			require.Equal(t, "openrouter", step.Provider, step.Name)
			require.Equal(t, "deepseek/deepseek-v4-flash", step.Model, step.Name)
		}
	}
}

func TestWorkflowIdentityIsRecordedByWorker(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	cfg := loadCfg(t, project)
	task, err := project.CreateTask(cfg, "identity", "research")
	require.NoError(t, err)
	customPi(t, `printf 'findings' > "$WATTS_TASK/REPORT.md"`)
	step := orchestration.Step{Name: "research", Agent: "build", Provider: "ollama", Model: "qwen2.5-coder:7b", Identity: "task-researcher", Prompt: "Inspect this task", Outputs: []string{"REPORT.md"}}
	code, _, errors := executeActivity(t, project, task, step, false)
	require.Equal(t, 0, code, errors)
	provenance := readFile(t, filepath.Join(project.StateDir(task), "runs", "research-1.json"))
	require.Contains(t, provenance, `"identity": "task-researcher"`)
}

func TestSDLCScaffoldSeparatesTaskDocumentsFromImplementation(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	app, cfg := project.Service, loadCfg(t, project)
	task, err := app.CreateTask(cfg, "project-location", "")
	require.NoError(t, err)
	plan, err := os.ReadFile(filepath.Join(app.Root, task, "PLAN.md"))
	require.NoError(t, err)
	require.Contains(t, string(plan), "Implementation root: the project directory containing watts.yaml")
	require.Contains(t, string(plan), "Do not create a separate application or module under the task directory")
	build, err := os.ReadFile(filepath.Join(app.Root, task, "RALPH.md"))
	require.NoError(t, err)
	require.Contains(t, string(build), "Implement application source, tests, modules, and other project files in the project root")
	definition, err := app.TaskWorkflow(cfg, task)
	require.NoError(t, err)
	for _, step := range definition.Definition.Steps {
		if step.Name == "planning" {
			require.Contains(t, step.Prompt, "Quality gates run from the project root")
		}
	}
}
