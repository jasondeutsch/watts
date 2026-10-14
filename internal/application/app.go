package application

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/workerstate"
)

// Service coordinates project operations and reports results to the caller.
type Service struct {
	Release string
	Report  func(Event)

	Root             string // absolute, symlinks resolved
	WorkingDirectory string
	Output           io.Writer
	ErrorOutput      io.Writer
	Input            io.Reader

	// TimeoutUnit is the length of one limits.max_minutes unit. Tests shorten it.
	TimeoutUnit time.Duration
}

// Locate finds a registered workspace or authored package above the invocation.
// Runtime directories and Git never define the application root.
func (a *Service) Locate() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if r, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = r
	}
	a.WorkingDirectory = cwd
	root := FindProjectRoot(cwd)
	if root == "" {
		root = cwd
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	a.Root = root
	return nil
}

// findProjectRoot walks up from dir looking for a Watts project.
func FindProjectRoot(dir string) string {
	store, _ := workerstate.Default()
	fallback := ""
	for {
		if store.Root != "" && store.HasProject(dir) {
			return dir
		}
		if info, err := os.Stat(filepath.Join(dir, "watts", projectconfig.Filename)); err == nil && info.Mode().IsRegular() {
			return dir
		}
		if info, err := os.Stat(filepath.Join(dir, projectconfig.Filename)); err == nil && info.Mode().IsRegular() {
			if fallback == "" {
				fallback = dir
				if filepath.Base(dir) == "watts" {
					fallback = filepath.Dir(dir)
				}
			}
		}
		// Durable tasks are also a discovery boundary when a project is copied to a
		// new machine. A package source directory is not required for inspection.
		tasks, _ := filepath.Glob(filepath.Join(dir, projectconfig.DefaultTasksDir, "*", "workflow.json"))
		if len(tasks) > 0 {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fallback
		}
		dir = parent
	}
}

func (app *Service) ProjectWorkerDir() (string, error) {
	store, err := workerstate.Default()
	if err != nil {
		return "", err
	}
	project, err := filepath.EvalSymlinks(app.Root)
	if err != nil {
		return "", err
	}
	directory := store.WorkerDir(project)
	if err = store.CheckDirectory(directory); err != nil {
		return "", err
	}
	return directory, nil
}
func (app *Service) RegisterWorkspace(cfg projectconfig.Config) error {
	store, err := workerstate.Default()
	if err != nil {
		return err
	}
	project, err := filepath.EvalSymlinks(app.Root)
	if err != nil {
		return err
	}
	return store.RegisterProject(project, cfg.TasksDir)
}

// tasksPath is the absolute path of the tasks directory.
func (a *Service) TasksPath(cfg projectconfig.Config) string {
	return filepath.Join(a.Root, filepath.FromSlash(cfg.TasksDir))
}

// rel shows a path relative to the repository root, for messages.
func (a *Service) Rel(p string) string {
	if r, err := filepath.Rel(a.Root, p); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return p
}

// resolveTask turns what the user typed (tasks/x, ./tasks/x, an absolute path, or a path relative
// to the current directory) into a clean path relative to the repository root.
func (a *Service) ResolveTask(arg string) (string, error) {
	if arg == "" {
		return "", errors.New("missing task folder, for example: tasks/2026-10-03-http-retry")
	}
	var candidates []string
	if filepath.IsAbs(arg) {
		candidates = []string{arg}
	} else {
		candidates = []string{filepath.Join(a.WorkingDirectory, arg), filepath.Join(a.Root, arg)}
	}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if r, err := filepath.EvalSymlinks(c); err == nil {
			c = r
		}
		st, err := os.Stat(c)
		if err != nil || !st.IsDir() {
			continue
		}
		taskPath, err := filepath.Rel(a.Root, c)
		if err != nil || taskPath == "." || taskPath == ".." || strings.HasPrefix(taskPath, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("%s is outside this repository", arg)
		}
		return filepath.ToSlash(taskPath), nil
	}
	return "", fmt.Errorf("no such task folder: %s (create one with: wctl task new <slug>)", arg)
}

type Event struct {
	Message string
	Warning bool
}

func (service *Service) say(format string, args ...any) {
	if service.Report != nil {
		service.Report(Event{Message: fmt.Sprintf(format, args...)})
	}
}
func (service *Service) warn(format string, args ...any) {
	if service.Report != nil {
		service.Report(Event{Message: fmt.Sprintf(format, args...), Warning: true})
	}
}
