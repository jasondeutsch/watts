package application

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/workspace"
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

// Locate finds the project root: the nearest folder above the current one that holds watts.yaml
// or .watts; failing that, the current folder, so `watts init` works anywhere. It never asks git.
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
	for {
		for _, marker := range []string{projectconfig.Filename, ".watts"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// wd joins a path under <repo>/.watts.
func (a *Service) Wd(parts ...string) string {
	return filepath.Join(append([]string{a.Root, ".watts"}, parts...)...)
}

func (a *Service) KitDir() string { return a.Wd("kit") }

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
	return "", fmt.Errorf("no such task folder: %s (create one with: watts task new <slug>)", arg)
}

// KitScript runs a bundled check with the project configuration.
func (a *Service) KitScript(cfg projectconfig.Config, name string, args ...string) error {
	return a.KitScriptTo(cfg, a.Output, name, args...)
}

// kitScriptTo is kitScript with the script's output sent to w.
func (a *Service) KitScriptTo(cfg projectconfig.Config, w io.Writer, name string, args ...string) error {
	return a.KitScriptEnv(cfg, w, nil, name, args...)
}

// kitScriptEnv is kitScriptTo with extra environment variables for the script.
func (a *Service) KitScriptEnv(cfg projectconfig.Config, w io.Writer, extraEnv []string, name string, args ...string) error {
	cmd, err := a.KitCommand(cfg, w, extraEnv, name, args...)
	if err != nil {
		return err
	}
	return workspace.RunCommand(cmd)
}

// KitCommand prepares a check command; callers choose interactive or activity execution.
func (a *Service) KitCommand(cfg projectconfig.Config, w io.Writer, extraEnv []string, name string, args ...string) (*exec.Cmd, error) {
	path := filepath.Join(a.KitDir(), "scripts", name)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("kit script %s is missing. Run: watts init", a.Rel(path))
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		return nil, errors.New("bash is required to run the check scripts")
	}
	cmd := exec.Command(bash, append([]string{path}, args...)...)
	cmd.Dir = a.Root
	cmd.Env = append(os.Environ(),
		"WATTS_REPO="+a.Root,
		"WATTS_AGENT_NAME="+a.Identity(cfg, "build"),
		"WATTS_REVIEWER_NAME="+a.Identity(cfg, "review"),
		"WATTS_TASKS_DIR="+cfg.TasksDir,
		"WATTS_SNAPSHOT_EXCLUDE="+strings.Join(cfg.SnapshotExclude, "\n"),
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	cmd.Stdin = a.Input
	cmd.Stdout = w
	cmd.Stderr = a.ErrorOutput
	return cmd, nil
}

var ReDotted = regexp.MustCompile(`\d+\.\d+\.\d+`)

// findVersion pulls the first x.y.z out of a tool's --version output.
func FindVersion(s string) string { return ReDotted.FindString(s) }

// verGE reports whether dotted version a is at least b.
func VersionAtLeast(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return true
}

type Event struct {
	Message  string
	Warning  bool
	Findings []Finding
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
func (service *Service) PrintFindings(findings []Finding) {
	if service.Report != nil {
		service.Report(Event{Findings: findings})
	}
}
