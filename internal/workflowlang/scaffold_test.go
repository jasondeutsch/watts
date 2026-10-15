package workflowlang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestScaffoldImportsTypedInputsAndSelectedWorkflow(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	write(t, source, Filename, basic)
	target := filepath.Join(root, "team")
	created, err := Scaffold(target, source, Options{Variables: map[string]cty.Value{"image": cty.StringVal("team:0.1.0")}})
	require.NoError(t, err)
	canonical, err := filepath.EvalSymlinks(target)
	require.NoError(t, err)
	require.Equal(t, canonical, created)
	pkg, err := Load(target, Options{})
	require.NoError(t, err)
	require.Equal(t, "base-coding", pkg.Workflow)
	require.Equal(t, "team:0.1.0", pkg.Definition.Steps[0].Container.Image)
	data, err := os.ReadFile(filepath.Join(target, "variables.acl"))
	require.NoError(t, err)
	require.Contains(t, string(data), "type")
	require.Contains(t, string(data), "string")
	require.NoDirExists(t, filepath.Join(root, "tasks"))
	_, err = Scaffold(target, source, Options{})
	require.ErrorContains(t, err, "destination must not exist")
	// Generated packages export their entrypoint and can themselves be composed.
	next := filepath.Join(root, "next")
	_, err = Scaffold(next, target, Options{})
	require.NoError(t, err)
	_, err = Load(next, Options{})
	require.NoError(t, err)
}

func TestScaffoldCoderGolangInputs(t *testing.T) {
	source := filepath.Join("..", "..", "resources", "examples", "coder_golang")
	target := filepath.Join(t.TempDir(), "go-team")
	_, err := Scaffold(target, source, Options{})
	require.NoError(t, err)
	pkg, err := Load(target, Options{})
	require.NoError(t, err)
	require.Len(t, pkg.Definition.Steps, 6)
	for _, filename := range []string{Filename, "variables.acl", "values.wvars", "README.md"} {
		require.FileExists(t, filepath.Join(target, filename))
	}
}

func TestScaffoldValidationDoesNotLeaveDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	write(t, source, Filename, basic)
	target := filepath.Join(root, "target")
	_, err := Scaffold(target, source, Options{Variables: map[string]cty.Value{"unknown": cty.True}})
	require.Error(t, err)
	require.NoDirExists(t, target)
	_, err = Scaffold(filepath.Join(source, "nested"), source, Options{})
	require.ErrorContains(t, err, "outside the source")
	require.NoDirExists(t, filepath.Join(source, "nested"))
}
