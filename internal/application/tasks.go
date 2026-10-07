package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

type RunOptions struct {
	Pass       []string
	Offline    bool
	Agent      string
	DryRun     bool
	Force      bool
	JSONOutput bool
}

func (a *Service) TaskArg(cfg projectconfig.Config, arg string) (string, error) {
	taskPath, err := a.ResolveTask(arg)
	if err == nil {
		return taskPath, nil
	}
	if arg != "" && !strings.Contains(arg, "/") {
		if r2, err2 := a.ResolveTask(cfg.TasksDir + "/" + arg); err2 == nil {
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
		return "", errors.New("no tasks found; create one with: watts task new <slug>")
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
	case "failed", "canceled", "terminated", "timed_out", "timedout":
		return fmt.Errorf("task is %s; create a new task to start another execution", view.State.Status)
	default:
		a.say("Task %s is %s at %s. Use: watts task status %s", taskPath, view.State.Status, view.State.Current, taskPath)
		return nil
	}
}

func (a *Service) ApproveTask(cfg projectconfig.Config, taskPath string, rf *RunOptions) error {
	if os.Getenv("WATTS_ROLE") != "" {
		return errors.New("approval is a human decision and cannot be given from inside an agent")
	}
	if err := a.KitScriptEnv(cfg, a.Output, []string{"WATTS_APPROVAL_SKIP_HASH=1"}, "check-approval.sh", taskPath); err != nil {
		return fmt.Errorf("not approved: fix the problems above first (%w)", err)
	}
	spec, err := Sha256File(filepath.Join(a.Root, taskPath, "SPEC.md"))
	if err != nil {
		return err
	}
	plan, err := Sha256File(filepath.Join(a.Root, taskPath, "PLAN.md"))
	if err != nil {
		return err
	}
	who := os.Getenv("USER")
	if who == "" {
		who = os.Getenv("LOGNAME")
	}
	if who == "" || who == a.Identity(cfg, "build") || who == a.Identity(cfg, "review") {
		return errors.New("cannot tell who is approving: set the USER variable to your own name")
	}
	now := time.Now()
	rec := fmt.Sprintf("SPEC.md %s\nPLAN.md %s\napproved_by %s\napproved_epoch %d\napproved_at %s\n", spec, plan, strings.ReplaceAll(who, "\n", " "), now.Unix(), now.UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(a.Root, taskPath, "APPROVAL"), []byte(rec), 0o644); err != nil {
		return err
	}
	a.say("Recorded the approval of %s/SPEC.md and PLAN.md by %s in %s/APPROVAL. If either file changes, approve again.", taskPath, who, taskPath)
	return nil
}
