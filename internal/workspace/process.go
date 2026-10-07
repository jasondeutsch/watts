package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/jasondeutsch/watts/internal/storage"
)

type ActiveProcess struct {
	PID int `json:"pid"`
}

// runProcess binds worker subprocess groups to Activity cancellation. The process journal
// lets a restarted worker refuse overlapping edits by a surviving process from an older run.
func RunProcess(ctx context.Context, journalPath string, cmd *exec.Cmd, limit time.Duration) error {
	if ctx == nil {
		return RunCommandTimeout(cmd, limit)
	}
	cancel := func() {}
	if limit > 0 {
		ctx, cancel = context.WithTimeout(ctx, limit)
	}
	defer cancel()
	if err := ctx.Err(); err != nil {
		return context.Cause(ctx)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		return err
	}
	if journalPath != "" {
		if err := storage.WriteJSONAtomically(journalPath, ActiveProcess{PID: cmd.Process.Pid}); err != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Wait()
			return err
		}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		err = context.Cause(ctx)
	}
	// Ensure detached descendants do not outlive a completed activity subprocess.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if journalPath != "" {
		if operationErr := os.Remove(journalPath); operationErr != nil && !errors.Is(operationErr, os.ErrNotExist) && err == nil {
			err = operationErr
		}
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return ExitError{Code: exit.ExitCode()}
	}
	return err
}
