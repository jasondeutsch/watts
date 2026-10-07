package workspace

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

type ExitError struct {
	Code    int
	Message string
}

func (e ExitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "exit status " + strconv.Itoa(e.Code)
}

func RunCommand(cmd *exec.Cmd) error { return RunCommandTimeout(cmd, 0) }

// runCommandTimeout is runCommand with a wall-clock limit. A limit of zero means none. When the
// limit is reached the child is killed and the error says so.
func RunCommandTimeout(cmd *exec.Cmd, limit time.Duration) error {
	sigc := make(chan os.Signal, 4)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigc)
	if limit > 0 {
		// A limited run gets its own process group, so the limit can stop everything Pi started
		// and not just Pi, and Wait does not hang on a grandchild that still holds the pipes.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.WaitDelay = 2 * time.Second
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var timedOut atomic.Bool
	if limit > 0 {
		timer := time.AfterFunc(limit, func() {
			timedOut.Store(true)
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				_ = cmd.Process.Kill()
			}
		})
		defer timer.Stop()
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigc:
				if cmd.Process == nil {
					break
				}
				if limit > 0 {
					// In its own process group the child no longer gets Ctrl-C from the terminal.
					_ = syscall.Kill(-cmd.Process.Pid, s.(syscall.Signal))
				} else if s == syscall.SIGTERM {
					_ = cmd.Process.Signal(s)
				}
			case <-done:
				return
			}
		}
	}()
	err := cmd.Wait()
	close(done)
	if timedOut.Load() {
		return ExitError{Code: 124, Message: "the run was stopped because it reached the time limit (limits.max_minutes)"}
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code := ee.ExitCode()
		if code < 0 {
			code = 1
		}
		return ExitError{Code: code}
	}
	return err
}

// kitScript runs one of the check scripts under .watts/kit/scripts with bash, in the repository
// root. The scripts take their settings from WATTS_* variables, so the CLI is the only place that
// reads config.json.
