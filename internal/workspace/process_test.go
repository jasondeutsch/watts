package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunProcessPreservesCancellationCause(t *testing.T) {
	cause := errors.New("activity heartbeat failed: namespace unavailable")
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	journal := filepath.Join(t.TempDir(), "active.json")
	done := make(chan error, 1)
	go func() { done <- RunProcess(ctx, journal, exec.Command("sleep", "30"), 0) }()
	require.Eventually(t, func() bool {
		_, err := os.Stat(journal)
		return err == nil
	}, 3*time.Second, 10*time.Millisecond)
	cancel(cause)
	select {
	case err := <-done:
		require.ErrorIs(t, err, cause)
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not stop process")
	}
	require.NoFileExists(t, journal)
}

func TestRunProcessPreservesCauseBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("activity reset by server")
	cancel(cause)
	require.ErrorIs(t, RunProcess(ctx, "", exec.Command("sleep", "30"), 0), cause)
}
