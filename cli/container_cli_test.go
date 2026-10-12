package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
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
	require.Contains(t, out, "wctl template")
	for _, name := range []string{"init", "install", "agent"} {
		code, _, err = command(t, name)
		require.Equal(t, 2, code)
		require.Contains(t, err, "unknown command")
	}
}

func TestPackageScaffoldingRenderingAndTaskCreation(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	code, _, err := command(t, "template", "new", "team")
	require.Zero(t, code, err)
	code, out, err := command(t, "template", "team")
	require.Zero(t, code, err)
	definition, parseErr := manifest.CompileWorkflow([]byte(out))
	require.NoError(t, parseErr)
	require.NotNil(t, definition.Steps[0].Container)
	code, _, err = command(t, "template", "new", "team")
	require.NotZero(t, code)
	require.NotEmpty(t, err)
	code, _, err = command(t, "template", "team", "-o", "generated")
	require.Zero(t, code, err)
	code, _, err = command(t, "template", "team", "-o", "generated")
	require.Zero(t, code, err, "generated output is safely regenerated")
	code, _, err = command(t, "task", "new", "hello", "--package", filepath.Join(project, "team", "watts.yaml"))
	require.Zero(t, code, err)
	entries, readErr := os.ReadDir(filepath.Join(project, "tasks"))
	require.NoError(t, readErr)
	require.Len(t, entries, 1)
	workflow, readErr := os.ReadFile(filepath.Join(project, "tasks", entries[0].Name(), "workflow.yaml"))
	require.NoError(t, readErr)
	pinned, parseErr := manifest.Decode(workflow, manifest.Workflow, orchestration.Definition{})
	require.NoError(t, parseErr)
	require.NotNil(t, pinned.Spec.Steps[0].Container)
	require.NoDirExists(t, filepath.Join(project, ".ralph-runner"))
	require.NoDirExists(t, filepath.Join(project, ".watts", "pi-agent-build"))
	code, _, err = command(t, "task", "run", entries[0].Name(), "--dry-run")
	require.Zero(t, code, err)
}
