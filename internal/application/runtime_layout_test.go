package application

import (
	"context"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/workerstate"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/workflowlang"
	"github.com/stretchr/testify/require"
)

func TestWorkerSettingsAreSharedStrictJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	first := &Service{Root: t.TempDir()}
	second := &Service{Root: t.TempDir()}
	settings := config.Default()
	require.Equal(t, "last-run", settings.RuntimeRetention)
	settings.Temporal = &config.TemporalSettings{TaskQueue: "shared"}
	require.ErrorContains(t, first.SaveConfig(settings), "global task_queue")
	settings.Temporal = nil
	settings.RuntimeRetention = "all"
	require.NoError(t, first.SaveConfig(settings))
	loaded, err := second.LoadServiceConfig()
	require.NoError(t, err)
	require.Equal(t, "all", loaded.RuntimeRetention)
	path, err := first.ServiceSettingsPath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".watts", "config", "settings.json"), path)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	require.NoDirExists(t, filepath.Join(first.Root, ".watts"))
	require.NoDirExists(t, filepath.Join(second.Root, ".watts"))
	require.NoError(t, os.WriteFile(path, []byte(`{"version":1,"tasks_dir":"tasks","unknown":true}`), 0600))
	_, err = second.LoadServiceConfig()
	require.ErrorContains(t, err, "unknown")
}
func TestLegacySettingsAreNotImportedOrDeleted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app := &Service{Root: t.TempDir()}
	legacy := filepath.Join(app.Root, ".watts", "settings.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(legacy), 0700))
	require.NoError(t, os.WriteFile(legacy, []byte("legacy preferences"), 0600))
	require.ErrorContains(t, app.CheckLegacySettings(), "no longer used")
	loaded, err := app.LoadServiceConfig()
	require.NoError(t, err)
	require.Equal(t, config.Default(), loaded, "legacy preferences must not be imported")
	require.Equal(t, "legacy preferences", readFile(t, legacy))
	require.Equal(t, "", FindProjectRoot(app.Root), "legacy runtime is not a project marker")
}
func TestDiscoveryKeepsPackageAndApplicationRootsSeparate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	parent := t.TempDir()
	project := filepath.Join(parent, "application")
	require.NoError(t, os.WriteFile(filepath.Join(parent, "watts.acl"), []byte("ancestor package"), 0644))
	nested := filepath.Join(project, "src")
	require.NoError(t, os.MkdirAll(nested, 0755))
	app := &Service{Root: project}
	require.NoError(t, app.RegisterWorkspace(config.Default()))
	require.Equal(t, project, FindProjectRoot(nested))
	packageDir := filepath.Join(project, "watts")
	require.NoError(t, os.MkdirAll(packageDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "watts.acl"), []byte("package"), 0644))
	require.Equal(t, project, FindProjectRoot(packageDir))
	module := filepath.Join(packageDir, "modules", "shared")
	require.NoError(t, os.MkdirAll(module, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(module, "watts.acl"), []byte("module"), 0644))
	require.Equal(t, project, FindProjectRoot(module), "a resource module must not become the application root")
	require.NoDirExists(t, filepath.Join(project, ".watts"))
}
func TestCompiledAssetsAreTaskOwnedAndReusable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	packageDir := t.TempDir()
	project := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(packageDir, "scripts"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "scripts", "check.sh"), []byte("true"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "watts.acl"), []byte(`package "fixture" {
 version="0.1.0"
 assets=["scripts"]
}
procedure "check" {
 execution {type="container"}
 container {
 image="fixture:latest"
 command=["sh","/inputs/resources/team/scripts/check.sh"]
 resources={team=package.root, project="context"}
 }
}
workflow "first" {
 stage "one" { executor=procedure.check }
}
workflow "second" {
 stage "two" { executor=procedure.check }
}
`), 0644))
	first, err := workflowlang.Load(packageDir, workflowlang.Options{Workflow: "first"})
	require.NoError(t, err)
	second, err := workflowlang.Load(packageDir, workflowlang.Options{Workflow: "second"})
	require.NoError(t, err)
	require.NotEqual(t, first.Digest, second.Digest)
	app := &Service{Root: project}
	require.NoError(t, os.Mkdir(filepath.Join(project, "context"), 0755))
	for _, pkg := range []workflowlang.Package{first, first, second} {
		task, err := app.CreateCompiledTask(config.Default(), "fixture", pkg)
		require.NoError(t, err)
		require.NoError(t, pkg.Validate(), "task binding must not mutate the compiled package")
		definition, err := app.TaskWorkflow(config.Default(), task)
		require.NoError(t, err)
		require.Equal(t, "context", definition.Definition.Steps[0].Container.Resources["project"])
		resource := definition.Definition.Steps[0].Container.Resources["team"]
		require.Contains(t, resource, task+"/.watts-state/package/packages/")
		require.FileExists(t, filepath.Join(project, resource, "scripts/check.sh"))
	}
	require.NoDirExists(t, filepath.Join(project, ".watts"))
	require.NoDirExists(t, filepath.Join(project, "watts"), "external authoring must not be copied into the application")
}

func TestTerminalReportTriggersOnlySafeRuntimeCleanup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	app := &Service{Root: project}
	task := "tasks/example"
	require.NoError(t, os.MkdirAll(filepath.Join(project, task), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(project, task, "SPEC.md"), []byte("durable request"), 0644))
	store, err := workerstate.Default()
	require.NoError(t, err)
	for _, run := range []string{"old", "latest"} {
		lease, err := store.Begin(project, task, "workflow/"+run, "build-1")
		require.NoError(t, err)
		require.NoError(t, lease.RetainEvidence())
		lease.Close()
	}
	request := orchestration.ReportInput{Input: orchestration.Input{Task: task}, WorkflowID: "workflow", RunID: "old", State: orchestration.State{Status: "waiting_retry"}}
	require.NoError(t, app.WriteTaskReport(context.Background(), request))
	require.DirExists(t, store.ExecutionDir(project, task, "workflow/old"))
	request.State.Status = "failed"
	require.NoError(t, app.WriteTaskReport(context.Background(), request))
	require.NoDirExists(t, store.ExecutionDir(project, task, "workflow/old"))
	require.DirExists(t, store.ExecutionDir(project, task, "workflow/latest"))
	require.Equal(t, "durable request", readFile(t, filepath.Join(project, task, "SPEC.md")))
	require.Contains(t, readFile(t, filepath.Join(project, task, "report.html")), "Status: failed")
}
