package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func templatePackage(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(directory, "templates", "nested"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "values.yaml"), []byte("name: default\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "templates", "a.yaml.gotmpl"), []byte("name: {{.Values.name | quote}}\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "templates", "nested", "b.yaml"), []byte("---\nname: second\n---\nname: third\n"), 0644))
	return directory
}

func TestTemplateNew(t *testing.T) {
	work := t.TempDir()
	code, output, diagnostic := runCLI(t, work, "template", "new", "go-team")
	require.Equal(t, 0, code, diagnostic)
	require.Contains(t, output, "Created go-team")
	require.FileExists(t, filepath.Join(work, "go-team", "watts.yaml"))
	code, output, diagnostic = runCLI(t, work, "template", "go-team")
	require.Equal(t, 0, code, diagnostic)
	require.Contains(t, output, "kind: Workflow")
	code, _, diagnostic = runCLI(t, work, "template", "new", "go-team")
	require.NotEqual(t, 0, code)
	require.Contains(t, diagnostic, "existing directories are never overwritten")
}

func TestTemplateOutsideProjectAndOverrideOrder(t *testing.T) {
	directory := templatePackage(t)
	work := t.TempDir()
	first := filepath.Join(work, "team values.yaml")
	second := filepath.Join(work, "project.yaml")
	require.NoError(t, os.WriteFile(first, []byte("name: team\n"), 0644))
	require.NoError(t, os.WriteFile(second, []byte("name: project\n"), 0644))
	code, output, diagnostic := runCLI(t, work, "template", directory, "-f", first, "--values", second)
	require.Equal(t, 0, code, diagnostic)
	decoder := yaml.NewDecoder(bytes.NewBufferString(output))
	var names []string
	for {
		var document map[string]string
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, document["name"])
	}
	require.Equal(t, []string{"project", "second", "third"}, names)
}

func TestTemplateRegeneratesOutputAndRemovesStaleFiles(t *testing.T) {
	directory := templatePackage(t)
	work := t.TempDir()
	output := filepath.Join(work, "rendered")
	code, stdout, diagnostic := runCLI(t, work, "template", directory, "--output", output)
	require.Equal(t, 0, code, diagnostic)
	require.Empty(t, stdout)
	require.FileExists(t, filepath.Join(output, "nested", "b.yaml"))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "values.yaml"), []byte("name: updated\n"), 0644))
	require.NoError(t, os.Remove(filepath.Join(directory, "templates", "nested", "b.yaml")))
	code, _, diagnostic = runCLI(t, work, "template", directory, "-o", output)
	require.Equal(t, 0, code, diagnostic)
	after, err := os.ReadFile(filepath.Join(output, "a.yaml"))
	require.NoError(t, err)
	require.Equal(t, "name: \"updated\"\n", string(after))
	require.NoDirExists(t, filepath.Join(output, "nested"))

	// A broken source cannot replace the last successful render.
	require.NoError(t, os.WriteFile(filepath.Join(directory, "templates", "a.yaml.gotmpl"), []byte("name: {{.Values.missing}}"), 0644))
	code, _, diagnostic = runCLI(t, work, "template", directory, "-o", output)
	require.Equal(t, 1, code, diagnostic)
	preserved, err := os.ReadFile(filepath.Join(output, "a.yaml"))
	require.NoError(t, err)
	require.Equal(t, after, preserved)
}

func TestTemplateRejectsOutputContainingUnrelatedFiles(t *testing.T) {
	directory := templatePackage(t)
	work := t.TempDir()
	output := filepath.Join(work, "rendered")
	require.NoError(t, os.MkdirAll(output, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(output, "notes.txt"), []byte("preserve"), 0644))
	code, _, diagnostic := runCLI(t, work, "template", directory, "-o", output)
	require.Equal(t, 1, code)
	require.Contains(t, diagnostic, "non-YAML")
	require.NoFileExists(t, filepath.Join(output, "a.yaml"))
	content, err := os.ReadFile(filepath.Join(output, "notes.txt"))
	require.NoError(t, err)
	require.Equal(t, "preserve", string(content))
}

func TestTemplateRejectsOutputOverlappingSources(t *testing.T) {
	directory := templatePackage(t)
	work := t.TempDir()
	for _, output := range []string{directory, filepath.Join(directory, "generated"), filepath.Dir(directory), work} {
		code, _, diagnostic := runCLI(t, work, "template", directory, "-o", output)
		require.Equal(t, 1, code, diagnostic)
	}
	require.FileExists(t, filepath.Join(directory, "values.yaml"))
	require.FileExists(t, filepath.Join(directory, "templates", "a.yaml.gotmpl"))
}

func TestTemplateFailuresHaveNoOutput(t *testing.T) {
	directory := templatePackage(t)
	work := t.TempDir()
	for _, args := range [][]string{{"template"}, {"template", directory, "extra"}, {"template", directory, "--unknown"}, {"template", directory, "-f"}} {
		code, stdout, _ := runCLI(t, work, args...)
		require.Equal(t, 2, code)
		require.Empty(t, stdout)
	}
	require.NoError(t, os.WriteFile(filepath.Join(directory, "templates", "a.yaml.gotmpl"), []byte("name: {{.Values.missing}}"), 0644))
	output := filepath.Join(work, "rendered")
	code, stdout, diagnostic := runCLI(t, work, "template", directory, "-o", output)
	require.Equal(t, 1, code)
	require.Empty(t, stdout)
	require.Contains(t, diagnostic, "missing")
	require.NoDirExists(t, output)
}

func TestTemplateRejectsPackageEscapeAndOutputSymlink(t *testing.T) {
	directory := templatePackage(t)
	work := t.TempDir()
	outside := filepath.Join(work, "outside.yaml")
	require.NoError(t, os.WriteFile(outside, []byte("name: outside"), 0644))
	require.NoError(t, os.Remove(filepath.Join(directory, "values.yaml")))
	require.NoError(t, os.Symlink(outside, filepath.Join(directory, "values.yaml")))
	code, _, diagnostic := runCLI(t, work, "template", directory)
	require.Equal(t, 1, code, diagnostic)

	directory = templatePackage(t)
	output := filepath.Join(work, "output")
	require.NoError(t, os.MkdirAll(output, 0755))
	require.NoError(t, os.Symlink(work, filepath.Join(output, "nested")))
	code, _, diagnostic = runCLI(t, work, "template", directory, "-o", output)
	require.Equal(t, 1, code, diagnostic)
	require.NoFileExists(t, filepath.Join(output, "a.yaml"))
	require.NoFileExists(t, filepath.Join(work, "b.yaml"))
}

func TestTemplateHelp(t *testing.T) {
	code, _, diagnostic := runCLI(t, t.TempDir(), "template", "-h")
	require.Equal(t, 0, code)
	require.Contains(t, diagnostic, "wctl template <package>")
	require.Contains(t, diagnostic, "-values")
	require.Contains(t, diagnostic, "-output")
}
