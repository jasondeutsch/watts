package runtime

import (
	"context"
	"github.com/jasondeutsch/watts/internal/application"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestProjectLockPreventsDuplicateServices(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app := &application.Service{Root: t.TempDir(), Output: io.Discard, ErrorOutput: io.Discard}
	first, err := lock(app)
	require.NoError(t, err)
	_, err = lock(app)
	require.ErrorContains(t, err, "already running")
	release(first)
	second, err := lock(app)
	require.NoError(t, err)
	release(second)
}
func TestStopDoesNotSignalStalePID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app := &application.Service{Root: t.TempDir(), Output: io.Discard, ErrorOutput: io.Discard}
	file, err := lock(app)
	require.NoError(t, err)
	release(file)
	_, path, err := paths(app)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"pid":1}`), 0600))
	require.ErrorContains(t, Stop(context.Background(), app), "not running")
}
func TestStopRejectsInvalidPIDWhileLocked(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app := &application.Service{Root: t.TempDir(), Output: io.Discard, ErrorOutput: io.Discard}
	file, err := lock(app)
	require.NoError(t, err)
	defer release(file)
	_, path, err := paths(app)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"pid":1}`), 0600))
	require.ErrorContains(t, Stop(context.Background(), app), "invalid service PID")
}

func TestServiceStateLivesOutsideProjectAndIsIsolated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	first := &application.Service{Root: t.TempDir()}
	second := &application.Service{Root: t.TempDir()}
	lease, err := lock(first)
	require.NoError(t, err)
	defer release(lease)
	other, err := lock(second)
	require.NoError(t, err)
	defer release(other)
	firstLock, firstRecord, err := paths(first)
	require.NoError(t, err)
	secondLock, _, err := paths(second)
	require.NoError(t, err)
	require.NotEqual(t, firstLock, secondLock)
	require.NotContains(t, firstRecord, first.Root)
	require.NoDirExists(t, filepath.Join(first.Root, ".watts"))
}
func TestLegacyWorkerMustBeStoppedBeforeStartup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	app := &application.Service{Root: t.TempDir()}
	path := filepath.Join(app.Root, ".watts", "service.lock")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(t, err)
	defer file.Close()
	require.NoError(t, syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB))
	require.ErrorContains(t, checkLegacyWorker(app), "stop the old")
	require.NoError(t, syscall.Flock(int(file.Fd()), syscall.LOCK_UN))
	require.NoError(t, checkLegacyWorker(app))
	require.FileExists(t, path)
}
