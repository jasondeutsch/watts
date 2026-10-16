package cli

import (
	"bytes"
	"encoding/json"
	"io/fs"
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
	require.Contains(t, out, "--dry-run")
	for _, name := range []string{"init", "install", "agent", "compile", "c"} {
		code, _, err = command(t, name)
		require.Equal(t, 2, code)
		require.Contains(t, err, "unknown command")
	}
}

func TestCompilationAndExecutionUseCapturedInputs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
	code, compiled, errOut := command(t, "exec", source, "--dry-run")
	require.Zero(t, code, errOut)
	require.NoFileExists(t, "compiled.json")
	require.NoError(t, os.WriteFile("compiled.json", []byte(compiled), 0600))
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
	code, compiled, errOut = command(t, "exec", "compiled.json", "--dry-run")
	require.Zero(t, code, errOut)
	var inspected workflowlang.Package
	require.NoError(t, json.Unmarshal([]byte(compiled), &inspected))
	require.NoError(t, inspected.Validate())
	require.Equal(t, pkg, inspected)
	require.NoDirExists(t, filepath.Join(project, ".ralph-runner"))
	require.NoDirExists(t, filepath.Join(project, ".watts"))
	definition.Steps[0].Container.Command = []string{"changed"}
	changed, err := json.Marshal(definition)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(task, "workflow.json"), changed, 0644))
	code, _, errOut = command(t, "exec", task, "--dry-run")
	require.NotZero(t, code)
	require.Contains(t, errOut, "definition was modified")

}

func TestPackageScaffoldingAndPinnedTaskRetry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	source := t.TempDir()
	t.Chdir(project)
	require.NoError(t, os.WriteFile(filepath.Join(source, "watts.acl"), []byte(`package "example" { version="0.1.0" }
procedure "approval" {
 inputs=["SPEC.md"]
 execution { type="approval" }
}
workflow "example" {
 templates={"SPEC.md"="Draft"}
 stage "approval" { executor=procedure.approval }
}
`), 0600))
	code, _, errOut := command(t, "new", "watts", "--from", source)
	require.Zero(t, code, errOut)
	require.FileExists(t, filepath.Join(project, "watts", "values.wvars"))
	code, _, errOut = command(t, "exec", "./watts", "--name", "Improve CSS", "--dry-run")
	require.Zero(t, code, errOut)
	require.NoDirExists(t, filepath.Join(project, "tasks"))
	code, _, errOut = command(t, "exec", "./watts", "--name", "Improve CSS", "--request-file", "missing.md")
	require.NotZero(t, code)
	require.Contains(t, errOut, "read request from")
	require.NoDirExists(t, filepath.Join(project, "tasks"))
	code, compiled, errOut := command(t, "exec", "./watts", "--dry-run")
	require.Zero(t, code, errOut)
	require.NoError(t, os.WriteFile("workflow.json", []byte(compiled), 0600))
	code, out, errOut := command(t, "task", "new", "Improve CSS", "--compiled", "workflow.json")
	require.Zero(t, code, errOut)
	entries, err := os.ReadDir(filepath.Join(project, "tasks"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Contains(t, entries[0].Name(), "-improve-css")
	task := filepath.Join("tasks", entries[0].Name())
	require.Contains(t, out, task)
	require.NoError(t, os.WriteFile(filepath.Join(source, "watts.acl"), []byte("invalid source"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(task, "watts.acl"), []byte("invalid source inside an existing task"), 0600))
	code, _, errOut = command(t, "exec", task, "--dry-run")
	require.Zero(t, code, errOut)
	for _, flags := range [][]string{{"--workflow", "changed"}, {"--request-file", "-"}, {"--name", "Other"}} {
		code, _, errOut = command(t, append([]string{"exec", task}, flags...)...)
		require.NotZero(t, code)
		require.Contains(t, errOut, "pinned configuration")
	}
	code, _, errOut = command(t, "exec", "./watts", "--name", "Another", "--dry-run")
	require.NotZero(t, code)
	entries, err = os.ReadDir(filepath.Join(project, "tasks"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestExecDryRunCompilesWithoutRuntimeOrFilesystemChanges(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	source := filepath.Join(root, "package")
	home := filepath.Join(root, "home")
	require.NoError(t, os.Mkdir(project, 0700))
	require.NoError(t, os.Mkdir(source, 0700))
	t.Chdir(project)
	t.Setenv("HOME", home)
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/watts-dry-run.sock")
	t.Setenv("MODEL_API_KEY", "")
	require.NoError(t, os.WriteFile(filepath.Join(source, "watts.acl"), []byte(`package "offline" {
 version="0.1.0"
 assets=["instructions.txt"]
 entrypoint=workflow.initial
}
variable "image" { type=string }
secret "credentials" { env={ api_key="MODEL_API_KEY" } }
procedure "check" {
 execution {type="container"}
 container {
  image=var.image
  command=["sh","-c","echo checked"]
 }
}
workflow "initial" {
 stage "first" {executor=procedure.check}
}
workflow "selected" {
 stage "second" {executor=procedure.check}
}
`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(source, "instructions.txt"), []byte("Captured instructions\n"), 0600))
	variables := filepath.Join(root, "team.wvars")
	require.NoError(t, os.WriteFile(variables, []byte("image=\"from-file:0.1.0\"\n"), 0600))
	snapshot := func() map[string]string {
		files := map[string]string{}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				files[path] = "directory"
				return nil
			}
			data, err := os.ReadFile(path)
			files[path] = string(data)
			return err
		})
		require.NoError(t, err)
		return files
	}
	for _, environment := range []string{"no worker home", "invalid worker settings"} {
		t.Run(environment, func(t *testing.T) {
			if environment == "invalid worker settings" {
				directory := filepath.Join(home, ".watts", "config")
				require.NoError(t, os.MkdirAll(directory, 0700))
				require.NoError(t, os.WriteFile(filepath.Join(directory, "settings.json"), []byte("invalid worker settings"), 0600))
			}
			before := snapshot()
			code, output, errOut := command(t, "exec", source, "--dry-run", "--workflow", "selected", "--var-file", variables, "--var", `image="override:0.1.0"`)
			require.Zero(t, code, errOut)
			require.Empty(t, errOut)
			var pkg workflowlang.Package
			require.NoError(t, json.Unmarshal([]byte(output), &pkg), "stdout must contain only the compiled artifact")
			require.NoError(t, pkg.Validate())
			require.Equal(t, "selected", pkg.Workflow)
			require.Equal(t, "override:0.1.0", pkg.Definition.Steps[0].Container.Image)
			require.Len(t, pkg.Assets, 1)
			require.Equal(t, "Captured instructions\n", string(pkg.Assets[0].Data))
			require.NotEmpty(t, pkg.Dependencies)
			require.Equal(t, before, snapshot())
			if environment == "no worker home" {
				require.NoDirExists(t, home)
			}
			code, output, errOut = command(t, "exec", source, "--dry-run", "--var", "not-an-assignment")
			require.NotZero(t, code)
			require.Empty(t, output)
			require.Contains(t, errOut, "variable override")
			require.Equal(t, before, snapshot())
		})
	}
	before := snapshot()
	code, output, errOut := command(t, "exec", source, "--dry-run", "--var", `image={invalid=true}`)
	require.NotZero(t, code)
	require.Empty(t, output)
	require.Contains(t, errOut, "image")
	require.Equal(t, before, snapshot())
}
