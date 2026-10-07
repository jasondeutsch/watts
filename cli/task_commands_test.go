package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskCommandsAreLimitedToMVP(t *testing.T) {
	var commands []string
	for _, command := range tree {
		if command.name == "task" {
			for _, child := range command.children {
				commands = append(commands, child.name)
			}
		}
	}
	require.ElementsMatch(t, []string{"new", "run", "status", "decide", "cancel"}, commands)
	for _, removed := range []string{"approve", "check", "close", "resume", "retry", "signal", "snapshot", "verify"} {
		code, _, errors := runCLI(t, t.TempDir(), "task", removed)
		require.NotZero(t, code, removed)
		require.Contains(t, errors, "has no subcommand", removed)
	}
}

func TestExistingTaskCommandsUseConsistentSelection(t *testing.T) {
	project := newProject(t)
	project.initRepo(t)
	for _, command := range []string{"run", "status", "decide", "cancel"} {
		code, _, errors := runCLI(t, project.Root, "task", command)
		require.NotZero(t, code, command)
		require.Contains(t, errors, "no tasks found", command)
	}
	cfg := loadCfg(t, project)
	first, err := project.CreateTask(cfg, "first", "")
	require.NoError(t, err)
	code, output, errors := runCLI(t, project.Root, "task", "run", "--dry-run")
	require.Zero(t, code, errors)
	require.Contains(t, output, "specification")
	require.NoFileExists(t, project.WorkflowBindingPath(first))
	for _, command := range []string{"decide", "cancel"} {
		code, _, errors = runCLI(t, project.Root, "task", command)
		require.NotZero(t, code, command)
		require.Contains(t, errors, project.WorkflowBindingPath(first), command)
	}
	second, err := project.CreateTask(cfg, "second", "")
	require.NoError(t, err)
	for _, command := range []string{"run", "status", "decide", "cancel"} {
		code, _, errors = runCLI(t, project.Root, "task", command)
		require.NotZero(t, code, command)
		require.Contains(t, errors, first, command)
		require.Contains(t, errors, second, command)
	}
	require.NoFileExists(t, project.WorkflowBindingPath(first))
	require.NoFileExists(t, project.WorkflowBindingPath(second))
	code, output, errors = runCLI(t, project.Root, "task", "run", first, "--dry-run")
	require.Zero(t, code, errors)
	require.Contains(t, output, "specification")
}
