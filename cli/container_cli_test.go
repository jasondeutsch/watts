package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workflowlang"
	"github.com/stretchr/testify/require"
)

func command(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errors bytes.Buffer
	code := Run(args, bytes.NewReader(nil), &out, &errors)
	return code, out.String(), errors.String()
}

func TestCLIHelpAndRemovedHostAgentCommands(t *testing.T) {
	code, out, err := command(t, "help")
	require.Zero(t, code, err)
	require.Contains(t, out, "start        ")
	require.Contains(t, out, "wctl compile")
	for _, name := range []string{"init", "install", "agent"} {
		code, _, err = command(t, name)
		require.Equal(t, 2, code)
		require.Contains(t, err, "unknown command")
	}
}

func TestCompilationAndExecutionUseCapturedInputs(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	source := filepath.Join(project, "source")
	require.NoError(t, os.MkdirAll(filepath.Join(source, "scripts"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "scripts", "check.sh"), []byte("echo captured\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(source, "watts.acl"), []byte(`package "checks" {
 version="0.1.0"
 assets=["scripts"]
}
procedure "check" {
 execution {type="container"}
 container {
  image="fixture:0.1.0"
  command=["sh","/inputs/resources/team/scripts/check.sh"]
  resources={team=package.root}
 }
}
workflow "checks" {
 templates={"SPEC.md"="Initial request"}
 stage "check" {executor=procedure.check}
}
`), 0644))
	code, _, errOut := command(t, "c", source, "-o", "compiled.json")
	require.Zero(t, code, errOut)
	pkg, err := workflowlang.ReadCompiled("compiled.json")
	require.NoError(t, err)
	require.Len(t, pkg.Definition.Steps, 1)
	// Remove the source: task creation/execution must consume only the artifact.
	require.NoError(t, os.RemoveAll(source))
	code, _, errOut = command(t, "task", "new", "hello", "--compiled", "compiled.json")
	require.Zero(t, code, errOut)
	entries, err := os.ReadDir(filepath.Join(project, "tasks"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	task := filepath.Join("tasks", entries[0].Name())
	data, err := os.ReadFile(filepath.Join(task, "workflow.json"))
	require.NoError(t, err)
	var definition orchestration.Definition
	require.NoError(t, json.Unmarshal(data, &definition))
	require.NotNil(t, definition.Steps[0].Container)
	require.FileExists(t, filepath.Join(project, definition.Steps[0].Container.Resources["team"], "scripts/check.sh"))
	code, _, errOut = command(t, "exec", task, "--dry-run")
	require.Zero(t, code, errOut)
	code, _, errOut = command(t, "exec", "compiled.json", "--dry-run")
	require.Zero(t, code, errOut)
	require.NoDirExists(t, filepath.Join(project, ".ralph-runner"))
	definition.Steps[0].Container.Command = []string{"changed"}
	changed, err := json.Marshal(definition)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(task, "workflow.json"), changed, 0644))
	code, _, errOut = command(t, "exec", task, "--dry-run")
	require.NotZero(t, code)
	require.Contains(t, errOut, "definition was modified")

}
