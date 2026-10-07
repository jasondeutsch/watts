package runtime

import (
	"context"
	"github.com/jasondeutsch/watts/internal/application"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"testing"
)

func TestProjectLockPreventsDuplicateServices(t *testing.T) {
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
	app := &application.Service{Root: t.TempDir(), Output: io.Discard, ErrorOutput: io.Discard}
	file, err := lock(app)
	require.NoError(t, err)
	release(file)
	_, path := paths(app)
	require.NoError(t, os.WriteFile(path, []byte(`{"pid":1}`), 0600))
	require.ErrorContains(t, Stop(context.Background(), app), "not running")
}
func TestStopRejectsInvalidPIDWhileLocked(t *testing.T) {
	app := &application.Service{Root: t.TempDir(), Output: io.Discard, ErrorOutput: io.Discard}
	file, err := lock(app)
	require.NoError(t, err)
	defer release(file)
	_, path := paths(app)
	require.NoError(t, os.WriteFile(path, []byte(`{"pid":1}`), 0600))
	require.ErrorContains(t, Stop(context.Background(), app), "invalid service PID")
}
