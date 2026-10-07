package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
)

func TestTemporalConfigAndCommandsWithoutAApp(t *testing.T) {
	ta := newProject(t)
	ta.initRepo(t)
	cfg, err := ta.LoadConfig()
	require.NoError(t, err)

	cfg.Temporal = &projectconfig.TemporalSettings{}
	cfg.Workflow = &orchestration.Definition{Steps: []orchestration.Step{{Name: "draft", Command: "true"}, {Name: "authorize", Human: true, Inputs: []string{"proposal.md"}}}}

	require.NoError(t, ta.SaveConfig(cfg))

	_ = os.MkdirAll(filepath.Join(ta.Root, "tasks", "custom"), 0700)
	{
		require.NoError(t, storage.WriteJSON(ta.TaskWorkflowPath("tasks/custom"), cfg.Workflow))
		code, out, errs := runCLI(t, ta.Root, "task", "run", "tasks/custom", "--dry-run")
		require.False(t, code != 0 || !strings.Contains(out, "authorize"),
			"dry run: %d %s%s", code, out, errs)
	}
	require.False(t, exists(ta.WorkflowBindingPath("tasks/custom")),
		"dry run must not bind a task")
	{
		code, _, errs := runCLI(t, ta.Root, "task", "run", "tasks/custom", "--step", "authorize")
		require.False(t, code == 0 || !strings.Contains(errs, "flag provided but not defined"),
			"step bypass: %d %s", code, errs)
	}

	var decoded projectconfig.Config
	data, _ := os.ReadFile(ta.ConfigPath())
	{
		err = json.Unmarshal(data, &decoded)
		require.False(t, err != nil || decoded.Temporal == nil,
			"temporal settings not persisted", err)
	}
}

func TestStatusWithoutTaskUsesOnlyTaskAndListsAmbiguousChoices(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	code, _, errors := runCLI(t, project.Root, "task", "status")
	require.Equal(t, 1, code)
	require.Contains(t, errors, "no tasks found")
	cfg, err := project.LoadConfig()
	require.NoError(t, err)
	first, err := project.CreateTask(cfg, "first", "")
	require.NoError(t, err)
	code, output, errors := runCLI(t, project.Root, "task", "status", "--json")
	require.Equal(t, 0, code, errors)
	require.Contains(t, output, first)
	require.Contains(t, output, "not_started")
	second, err := project.CreateTask(cfg, "second", "")
	require.NoError(t, err)
	code, _, errors = runCLI(t, project.Root, "task", "status")
	require.Equal(t, 1, code)
	require.Contains(t, errors, first)
	require.Contains(t, errors, second)
	code, output, errors = runCLI(t, project.Root, "task", "status", first, "--json")
	require.Equal(t, 0, code, errors)
	require.Contains(t, output, first)
}
