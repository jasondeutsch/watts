package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectDefaultWorkflowScaffoldsTask(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	definition := orchestration.Definition{Steps: []orchestration.Step{{Name: "research", Agent: "build", Provider: "ollama", Model: "qwen2.5-coder:7b", Identity: "research-agent", Prompt: "Investigate this task"}}}
	require.NoError(t, storage.WriteJSON(filepath.Join(project.Root, "research.json"), definition))
	cfg, err := project.LoadConfig()
	require.NoError(t, err)
	cfg.DefaultWorkflow = "research.json"
	require.NoError(t, project.SaveConfig(cfg))
	code, _, errors := runCLI(t, project.Root, "task", "new", "question")
	require.Equal(t, 0, code, errors)
	matches, err := filepath.Glob(filepath.Join(project.Root, "tasks", "*-question"))
	require.NoError(t, err)
	require.Len(t, matches, 1)
	data, err := os.ReadFile(filepath.Join(matches[0], "workflow.json"))
	require.NoError(t, err)
	require.Contains(t, string(data), "research-agent")
	_, err = os.Stat(filepath.Join(matches[0], "SPEC.md"))
	require.True(t, os.IsNotExist(err))
}

func TestTasksExposeOnlyWorkflowExecution(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	for _, args := range [][]string{{"task", "build"}, {"task", "review"}, {"task", "stop"}, {"task", "logs"}, {"task", "run", "x", "--step", "coding"}, {"task", "retry", "x", "coding", "--reset"}} {
		{
			code, _, errs := runCLI(t, project.Root, args...)
			assert.Equal(t, 2, code,
				"%v: expected argument rejection, got %d %s", args, code, errs)
		}
	}
}
