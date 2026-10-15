package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestAttemptPathsStayUnderHostHomeAndRejectRedirection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	app := &Service{Root: t.TempDir()}
	runtime := &activityRuntime{project: app, execution: "workflow/run", request: orchestration.ActivityInput{Input: orchestration.Input{Task: "tasks/fixture"}, Step: orchestration.Step{Name: "build"}, Attempt: 1}}
	directory, err := runtime.attemptRoot()
	require.NoError(t, err)
	relative, err := filepath.Rel(filepath.Join(home, ".watts", "runtime"), directory)
	require.NoError(t, err)
	require.NotContains(t, relative, "..")
	require.NoDirExists(t, directory)
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(directory), 0700))
	require.NoError(t, os.Symlink(outside, directory))
	_, err = runtime.attemptRoot()
	require.ErrorContains(t, err, "symlink")
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestHostPermissionErrorIdentifiesOperationAndPath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission check requires an ordinary user")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	app := &Service{Root: t.TempDir()}
	require.NoError(t, app.SaveConfig(config.Default()))
	directory := filepath.Join(home, ".watts", "config")
	require.NoError(t, os.Chmod(directory, 0500))
	defer os.Chmod(directory, 0700)
	err := app.SaveConfig(config.Default())
	require.ErrorIs(t, err, os.ErrPermission)
	var diagnostic *storage.HostFilesystemError
	require.ErrorAs(t, err, &diagnostic)
	require.Equal(t, "permission denied", diagnostic.Category)
	require.Contains(t, diagnostic.Path, directory)
	require.NotEmpty(t, diagnostic.Operation)
}

func TestContainerRuntimeUsesPrivateHomeWithoutChangingHostHome(t *testing.T) {
	requireDocker(t)
	step := containerStep(`test "$HOME" = /runtime/home; test "$TMPDIR" = /runtime/tmp; mkdir -p /runtime/task; printf private > "$HOME/marker"; printf task > /runtime/task/marker; test ! -w /; printf checked > /outputs/task/runtime.txt`)
	step.Outputs = []string{"runtime.txt"}
	app, request := boundTemporalTask(t, step)
	home := filepath.Join(t.TempDir(), "Alternate Home")
	require.NoError(t, os.Mkdir(home, 0700))
	t.Setenv("HOME", home)
	require.NoError(t, app.SaveConfig(config.Default()))
	runtime, err := app.prepareActivity(context.Background(), request)
	require.NoError(t, err)
	directory, err := runtime.attemptRoot()
	require.NoError(t, err)
	_, err = runtime.prepareInput()
	require.NoError(t, err)
	_, err = runtime.execute()
	require.NoError(t, err)
	require.Equal(t, home, os.Getenv("HOME"))
	require.Equal(t, "private", readFile(t, filepath.Join(directory, "private", "home", "marker")))
	require.Equal(t, "task", readFile(t, filepath.Join(directory, "private", "task", "marker")))
	require.NoFileExists(t, filepath.Join(home, "marker"))
	require.Equal(t, "checked", readFile(t, filepath.Join(app.Root, request.Input.Task, "runtime.txt")))
	// Reattachment uses the accepted receipt, not another container execution.
	_, err = runtime.execute()
	require.NoError(t, err)
}
