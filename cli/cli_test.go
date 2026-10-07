package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCLI runs the real entry point, Run, inside dir, as a user would.
func runCLI(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	old, _ := os.Getwd()

	require.NoError(t, os.Chdir(dir))

	defer os.Chdir(old)
	var o, e bytes.Buffer
	code = Run(args, strings.NewReader(""), &o, &e)
	return code, o.String(), e.String()
}
func TestHelpAndDispatch(t *testing.T) {
	dir := t.TempDir()
	code, out, _ := runCLI(t, dir)
	require.Equal(t, 0, code,
		"no arguments should print help, exit %d", code)

	var check func(path string, n *node)
	check = func(path string, n *node) {
		assert.False(t, n.summary == "" || (n.run == nil && len(n.children) == 0),
			"command %q is incomplete", path)

		for _, c := range n.children {
			check(path+" "+c.name, c)
		}
	}
	for _, n := range tree {
		assert.Contains(t, out, "  "+n.name+" ",
			"help does not list %q", n.name)

		check(n.name, n)

		if len(n.children) > 0 {
			code, gout, _ := runCLI(t, dir, n.name)
			if n.runOnEmpty {
				continue
			}
			assert.Equal(t, 0, code,
				"%s with no subcommand exited %d", n.name, code)

			for _, c := range n.children {
				assert.Contains(t, gout, "  "+c.name+" ",
					"`watts %s` does not list %q", n.name, c.name)
			}
		}
	}
	assert.False(t, strings.ContainsAny(out, "\u2013\u2014"),
		"help text contains an en or em dash")

	for _, flag := range []string{"help", "-h", "--help"} {
		{
			code, _, _ := runCLI(t, dir, flag)
			assert.Equal(t, 0, code,
				"%s exited %d", flag, code)
		}
	}
	{
		code, _, errs := runCLI(t, dir, "frobnicate")
		assert.False(t, code != 2 || !strings.Contains(errs, "unknown command"),
			"unknown command: exit %d, %q", code, errs)
	}

	for _, v := range []string{"version", "--version"} {
		{
			code, out, _ := runCLI(t, dir, v)
			assert.False(t, code != 0 || !strings.Contains(out, "watts "+version),
				"%s: exit %d, %q", v, code, out)
		}
	}

	for _, c := range [][]string{{"doctor"}, {"task", "new", "x"}, {"config", "show"}, {"agent", "list"}} {
		fresh := t.TempDir()
		{
			code, _, errs := runCLI(t, fresh, c...)
			assert.False(t, code != 1 || !strings.Contains(errs+"watts init", "watts init"),
				"%v outside a project: exit %d, %q", c, code, errs)
		}
	}
	{
		code, _, errs := runCLI(t, t.TempDir(), "config", "show")
		assert.False(t, code != 1 || !strings.Contains(errs, "watts init"),
			"config show outside a project: exit %d, %q", code, errs)
	}
}
func TestInitConfigSetClean(t *testing.T) {
	ta := newProject(t)
	{
		code, _, errs := runCLI(t, ta.Root, "config", "show")
		require.False(t, code != 1 || !strings.Contains(errs, "watts init"),
			"config before init: exit %d, %q", code, errs)
	}
	{
		code, out, errs := runCLI(t, ta.Root, "init", "--no-install")
		require.Equal(t, 0, code,
			"init: exit %d\n%s%s", code, out, errs)
	}

	for _, p := range []string{"watts.json", ".watts/kit/scripts/lib.sh", ".watts/pi-agent-build/settings.json", ".watts/pi-agent-review/settings.json"} {
		assert.True(t, exists(filepath.Join(ta.Root, p)),
			"init did not create %s", p)
	}

	entries, _ := os.ReadDir(ta.Root)
	for _, e := range entries {
		switch e.Name() {
		case ".git", "README.md", ".watts", "watts.json":
		default:
			assert.
				Fail(t, fmt.Sprintf("init touched %s", e.Name()))
		}
	}
	{
		ign := readFile(t, filepath.Join(ta.Root, ".watts", ".gitignore"))
		assert.False(t, !strings.Contains(ign, "pi-agent-*/") || !strings.Contains(ign, "home-*/"),
			".watts/.gitignore: %q", ign)
	}
	{
		code, out, _ := runCLI(t, ta.Root, "config", "show")
		assert.False(t, code != 0 || !strings.Contains(out, `"tasks_dir": "tasks"`),
			"config --show: exit %d, %q", code, out)
	}
	{
		code, _, _ := runCLI(t, ta.Root, "config", "set", "max_attempts", "3")
		require.Equal(t, 0, code,
			"set max_attempts failed")
	}
	{
		code, out, _ := runCLI(t, ta.Root, "config", "apply")
		require.False(t, code != 0 || !strings.Contains(out, "configured build agent"),
			"config apply: exit %d, %q", code, out)
	}
	assert.Contains(t, readFile(t, filepath.Join(ta.Root, ".watts/pi-agent-review/settings.json")), `"medium"`,
		"the agent thinking default must reach its directory")

	for _, bad := range [][]string{{"config", "set"}, {"config", "set", "bogus", "x"}, {"config", "set", "thinking", "extreme"}, {"config", "set", "env_passthrough", "HOME"}} {
		{
			code, _, _ := runCLI(t, ta.Root, bad...)
			assert.Equal(t, 1, code,
				"%v should fail, exit %d", bad, code)
		}
	}
	{
		code, _, _ := runCLI(t, ta.Root, "agent", "clean")
		assert.Equal(t, 1, code,
			"clean without --force must refuse")
	}
	{
		code, _, _ := runCLI(t, ta.Root, "agent", "clean", "--force")
		require.Equal(t, 0, code,
			"clean --force failed")
	}
	assert.False(t, exists(filepath.Join(ta.Root, ".watts/pi-agent-build")) || exists(filepath.Join(ta.Root, ".watts/home-build")),
		"clean must remove the agent directories")
	assert.False(t, !exists(filepath.Join(ta.Root, ".watts/kit/scripts/lib.sh")) || !exists(filepath.Join(ta.Root, "watts.json")),
		"clean must keep the kit and the config")
	{
		code, _, _ := runCLI(t, ta.Root, "config", "apply")
		assert.False(t, code != 0 || !exists(filepath.Join(ta.Root, ".watts/pi-agent-build/settings.json")),
			"config must recreate the agent directories")
	}
}

// taskRepo returns an initialised repository with a Tier 0 task whose plan has two stories.
func taskRepo(t *testing.T) (*testApp, string) {
	t.Helper()
	ta := newProject(t)
	ta.initRepo(t)
	{
		code, out, errs := runCLI(t, ta.Root, "task", "new", "demo")
		require.Equal(t, 0, code,
			"task: exit %d\n%s%s", code, out, errs)
	}

	matches, _ := filepath.Glob(filepath.Join(ta.Root, "tasks", "*-demo"))
	require.Len(t, matches, 1,
		"task folder not created: %v", matches)

	task := "tasks/" + filepath.Base(matches[0])
	writeFile(t, filepath.Join(ta.Root, task, "SPEC.md"), `# SPEC: demo

## Status
Draft
Version: v1

## Acceptance criteria
- AC-1: While idle, when polled, the system shall answer.
- AC-2: While busy, when polled, the system shall queue.

## Risk tier
0

## Approved by
<name, date>
`)
	writeFile(t, filepath.Join(ta.Root, task, "PLAN.md"), `# PLAN: demo

Spec ref: SPEC.md v1

## Risk tier recheck

Risk tier after planning: 0

## Quality gates

- `+"`true`"+`

## User stories

### US-001: First

Covers: AC-1
Depends on: none

### US-002: Second

Covers: AC-2
Depends on: US-001

## Approved by

<name, date>
`)

	return ta, task
}
func TestTaskAndCheck(t *testing.T) {
	needTools(t, "awk")
	ta, task := taskRepo(t)
	{
		base := strings.TrimSpace(readFile(t, filepath.Join(ta.Root, task, "BASE")))
		assert.True(t, strings.HasPrefix(base, "S"),
			"BASE must name a snapshot, not a commit: %q", base)
	}
	assert.False(t, exists(filepath.Join(ta.Root, ".git")),
		"starting a task must not initialize version control")

	for _, f := range []string{"BASE", "RALPH.md", "review/RALPH.md"} {
		assert.True(t, exists(filepath.Join(ta.Root, task, f)),
			"task is missing %s", f)
	}
	rendered := readFile(t, filepath.Join(ta.Root, task, "RALPH.md")) + readFile(t, filepath.Join(ta.Root, task, "review/RALPH.md"))
	for _, ph := range []string{"__KIT__", "__NAME__", "__AGENT__"} {
		assert.NotContains(t, rendered, ph,
			"unreplaced placeholder %s", ph)
	}
	assert.Contains(t, rendered, "bash .watts/kit/scripts/check-approval.sh "+task,
		"loop commands must call the kit under .watts/kit")

	sub := filepath.Join(ta.Root, "sub")
	_ = os.MkdirAll(sub, 0o755)
	{
		code, out, errs := runTaskScript(t, sub, "preflight.sh", "../"+task)
		require.False(t, code != 0 || !strings.Contains(out, "PREFLIGHT: OK"),
			"check from a subdirectory: exit %d\n%s%s", code, out, errs)
	}

	for _, args := range [][]string{{"task", "check"}, {"task", "check", "tasks/missing"}, {"task", "check", task, "extra"}, {"task", "new"}, {"task", "new", "Bad Slug"}, {"task", "Frobnicate"}, {"task", "check"}} {
		{
			code, _, _ := runCLI(t, ta.Root, args...)
			assert.NotEqual(t, 0, code,
				"%v should fail", args)
		}
	}
}

// finish records two stories as done: the agent changes a file, takes a snapshot, and logs its id.
func Finish(t *testing.T, ta *testApp, task string) {
	t.Helper()
	cfg := loadCfg(t, ta)
	var ids []string
	for i, id := range []string{"US-001", "US-002"} {
		writeFile(t, filepath.Join(ta.Root, fmt.Sprintf("feature%d.txt", i)), id)
		var buf bytes.Buffer
		{
			err := ta.KitScriptTo(cfg, &buf, "snapshot.sh", task, "agent")
			require.NoError(t, err,
				"snapshot: %v", err)
		}

		ids = append(ids, strings.TrimSpace(buf.String()))
	}
	writeFile(t, filepath.Join(ta.Root, task, "STORY_LOG.md"), fmt.Sprintf("DONE US-001 %s\nDONE US-002 %s\n", ids[0], ids[1]))
}
func TestPiPassThroughAndFlags(t *testing.T) {
	needTools(t, "timeout")
	ta := newProject(t)
	ta.initRepo(t)
	logFile := stubBin(t, "pi 1.0.1")
	{
		code, out, errs := runCLI(t, ta.Root, "agent", "pi", "--agent", "review", "--", "mcp", "add", "context7", "--", "npx", "-y", "@upstash/context7-mcp")
		require.Equal(t, 0, code,
			"pi: exit %d\n%s%s", code, out, errs)
	}
	{
		code, _, _ := runCLI(t, ta.Root, "agent", "pi", "--", "--model", "ollama/devstral:24b")
		require.Equal(t, 0, code,
			"pi with its own flags failed")
	}
	{
		code, _, _ := runCLI(t, ta.Root, "agent", "pi", "--offline", "--env-passthrough", "NOT_SET")
		require.Equal(t, 0, code,
			"pi with watts flags failed")
	}

	log := readFile(t, logFile)
	assert.Contains(t, log, "ARGS: mcp add context7 -- npx -y @upstash/context7-mcp\n",
		"pass-through arguments were altered:\n%s", log)
	assert.Contains(t, log, "ARGS: --model ollama/devstral:24b\n",
		"Pi's own flags were lost:\n%s", log)
	assert.False(t, !strings.Contains(log, "WATTS_ROLE=review") || !strings.Contains(log, "PI_OFFLINE=1"),
		"--agent and --offline were not honoured:\n%s", log)
	{
		code, _, errs := runCLI(t, ta.Root, "agent", "pi", "--model", "x")
		assert.False(t, code != 2 || !strings.Contains(errs, "flag provided but not defined"),
			"unknown flag: exit %d, %q", code, errs)
	}
	{
		code, _, _ := runCLI(t, ta.Root, "agent", "pi", "--agent", "admin")
		assert.Equal(t, 1, code,
			"an unknown role must fail")
	}
}

// The kit scripts must work when the repository is reached through a symlink (macOS keeps temp
// directories behind /var -> /private/var) and when they are run by hand, without the CLI having
// resolved the paths first. The CLI resolves symlinks itself, so this runs the script directly.
func TestKitScriptThroughSymlinkedPath(t *testing.T) {
	needTools(t, "awk")
	ta := newProject(t)
	ta.initRepo(t)

	link := filepath.Join(t.TempDir(), "via-link")
	if err := os.Symlink(ta.Root, link); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	cmd := exec.Command("bash", filepath.Join(link, ".watts", "kit", "scripts", "new-task.sh"), "linked")
	cmd.Dir = link
	cmd.Env = append(os.Environ(), "WATTS_TASKS_DIR=tasks")
	{
		out, err := cmd.CombinedOutput()
		require.NoError(t, err,
			"new-task.sh through a symlink: %v\n%s", err, out)
	}

	matches, _ := filepath.Glob(filepath.Join(ta.Root, "tasks", "*-linked"))
	require.Len(t, matches, 1,
		"task folder not created: %v", matches)

	prompt := readFile(t, filepath.Join(matches[0], "RALPH.md"))
	assert.Contains(t, prompt, "bash .watts/kit/scripts/check-approval.sh tasks/",
		"kit paths must stay relative when the repository is reached through a symlink:\n%s", prompt[:600])
	assert.False(t, strings.Contains(prompt, ta.Root) || strings.Contains(prompt, link),
		"an absolute path leaked into the generated prompt")
}
func TestCommandTreeRejectsUndeclaredCommands(t *testing.T) {
	for _, args := range [][]string{{"build"}, {"set"}, {"task", "demo"}, {"config", "migrate"}, {"config", "--show"}} {
		code, _, errs := runCLI(t, t.TempDir(), args...)
		assert.False(t, code != 2 || (!strings.Contains(errs, "unknown command") && !strings.Contains(errs, "has no subcommand")),
			"%v: expected command error, got %d %s", args, code, errs)
	}
}
