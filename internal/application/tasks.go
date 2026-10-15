package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

type RunOptions struct {
	DryRun     bool
	Force      bool
	JSONOutput bool
}

func (a *Service) TaskArg(cfg projectconfig.Config, arg string) (string, error) {
	resolve := func(argument string) (string, error) {
		path, err := a.ResolveTask(argument)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(a.TaskWorkflowPath(path))
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("%s is not a task folder (missing workflow definition)", argument)
		}
		return path, nil
	}
	taskPath, err := resolve(arg)
	if err == nil {
		return taskPath, nil
	}
	if arg != "" && !strings.Contains(arg, "/") {
		if r2, err2 := resolve(cfg.TasksDir + "/" + arg); err2 == nil {
			return r2, nil
		}
	}
	return "", err
}

// SelectTask resolves an explicit task or the project’s only task. Ambiguous selection never mutates a task.
func (a *Service) SelectTask(cfg projectconfig.Config, argument string) (string, error) {
	if argument != "" {
		return a.TaskArg(cfg, argument)
	}
	entries, err := os.ReadDir(a.TasksPath(cfg))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		task := filepath.ToSlash(filepath.Join(cfg.TasksDir, entry.Name()))
		if _, err := os.Stat(a.TaskWorkflowPath(task)); err == nil {
			candidates = append(candidates, task)
		}
	}
	switch len(candidates) {
	case 0:
		return "", errors.New("no tasks found; create one with: wctl task new <slug>")
	case 1:
		return a.TaskArg(cfg, candidates[0])
	default:
		return "", fmt.Errorf("specify a task folder or name\nAvailable tasks:\n  %s", strings.Join(candidates, "\n  "))
	}
}

// RunTask starts an unsubmitted task, retries a failed stage, or reports an existing execution.
func (a *Service) RunTask(cfg projectconfig.Config, taskPath string, options *RunOptions) error {
	if options.DryRun || !a.HasTemporalTask(taskPath) {
		return a.StartTemporalWorkflow(cfg, taskPath, options)
	}
	view, err := a.TemporalTaskStatus(context.Background(), taskPath)
	if err != nil {
		return err
	}
	switch view.State.Status {
	case "not_started":
		return a.StartTemporalWorkflow(cfg, taskPath, options)
	case "waiting_retry":
		return a.retryFailedStage(taskPath, options)
	case "failed", "cancelled", "canceled", "terminated", "timed_out", "timedout":
		return fmt.Errorf("task is %s; create a new task to start another execution", view.State.Status)
	default:
		a.say("Task %s is %s at %s. Use: wctl task status %s", taskPath, view.State.Status, view.State.Current, taskPath)
		return nil
	}
}
