package workflowpackage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScaffoldProducesUsablePackage(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "go-team")
	require.NoError(t, Scaffold(directory))
	descriptor, err := ReadDescriptor(filepath.Join(directory, Filename))
	require.NoError(t, err)
	require.Equal(t, "go-team", descriptor.Name)
	require.FileExists(t, filepath.Join(directory, "values.schema.json"))
	for _, filename := range []string{"00-workflow.yaml.gotmpl", "10-agents.yaml.gotmpl", "20-procedures.yaml.gotmpl", "30-configmaps.yaml.gotmpl", "40-secrets.yaml.gotmpl"} {
		require.FileExists(t, filepath.Join(directory, "templates", filename))
	}
	pkg, err := Load(directory)
	require.NoError(t, err)
	result, err := pkg.Render()
	require.NoError(t, err)
	require.Contains(t, string(result.Data), "name: go-team")
	for _, kind := range []string{"Workflow", "Agent", "Procedure", "ConfigMap", "Secret"} {
		require.Contains(t, string(result.Data), "kind: "+kind)
	}
	before, err := os.ReadFile(filepath.Join(directory, "values.yaml"))
	require.NoError(t, err)
	require.ErrorContains(t, Scaffold(directory), "existing directories are never overwritten")
	after, err := os.ReadFile(filepath.Join(directory, "values.yaml"))
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestScaffoldRejectsInvalidName(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "Invalid Name")
	require.ErrorContains(t, Scaffold(directory), "invalid package name")
	require.NoDirExists(t, directory)
}
