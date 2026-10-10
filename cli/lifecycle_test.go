package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plainProject is a folder with nothing in it: no repository, no history.
func plainProject(t *testing.T) (string, *terminal.Context) {
	t.Helper()
	needTools(t, "bash", "awk", "find", "xargs")
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	a := newApp(dir, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	return dir, a
}

// gitTripwire puts a fake git first on PATH. It records every call and fails it, so a test can
// prove that Watts never ran git. It returns the file the calls are written to.
func gitTripwire(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "git-was-called.txt")
	script := "#!/bin/sh\necho \"$@\" >> \"" + log + "\"\nexit 127\n"

	require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755))

	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(func() {
		assert.NoFileExists(t, log, "Watts must never run Git")
	})
	return log
}

// lifecycle runs a whole task, from init to close, in dir. It is run in an empty folder and in a
// git repository with uncommitted files, and must behave the same in both.
func lifecycle(t *testing.T, dir string, a *terminal.Context) {
	t.Helper()
	t.Setenv("USER", "Jane Doe")
	{
		code, out, errs := runCLI(t, dir, "init", "--no-install")
		require.Equal(t, 0, code,
			"init: %d\n%s%s", code, out, errs)
	}

	cfg, err := a.LoadConfig()
	require.NoError(t, err)

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		switch e.Name() {
		case ".git", ".watts", "watts.yaml", "README.md", "wip.txt":
		default:
			assert.
				Fail(t, fmt.Sprintf("init created %s", e.Name()))
		}
	}
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")
	writeFile(t, filepath.Join(dir, "node_modules", "dep", "x.js"), "x")
	{
		code, out, errs := runCLI(t, dir, "task", "new", "demo")
		require.Equal(t, 0, code,
			"task new: %d\n%s%s", code, out, errs)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "tasks", "*-demo"))
	require.Len(t, matches, 1,
		"task folder: %v", matches)

	task := "tasks/" + filepath.Base(matches[0])
	base := strings.TrimSpace(readFile(t, filepath.Join(matches[0], "BASE")))
	require.False(t, !strings.HasPrefix(base, "S") || !exists(filepath.Join(matches[0], ".watts-state", "snapshots", base)),
		"BASE must name a snapshot: %q", base)

	snapshotManifest := readFile(t, filepath.Join(matches[0], ".watts-state", "snapshots", base))
	assert.False(t, !strings.Contains(snapshotManifest, "  main.go") || strings.Contains(snapshotManifest, "node_modules") || strings.Contains(snapshotManifest, "watts.yaml") || strings.Contains(snapshotManifest, "SPEC.md"),
		"a snapshot covers the project's own files only:\n%s", snapshotManifest)

	ralph := readFile(t, filepath.Join(matches[0], "RALPH.md"))
	assert.False(t, !strings.Contains(ralph, "snapshot.sh") || strings.Contains(ralph, "git commit") || strings.Contains(ralph, "git rev-parse"),
		"the loop prompt must use snapshots, not commits:\n%s", ralph)
	{
		ign := readFile(t, filepath.Join(matches[0], ".gitignore"))
		assert.Contains(t, ign, ".watts-state/",
			"the task folder keeps its own run state out of version control: %q", ign)
	}

	writeFile(t, filepath.Join(matches[0], "SPEC.md"), approvedSpec)
	writeFile(t, filepath.Join(matches[0], "PLAN.md"), approvedPlan)
	{
		code, _, _ := runTaskScript(t, dir, "preflight.sh", task)
		require.NotEqual(t, 0, code,
			"an unapproved task must not pass")
	}
	{
		code, out, errs := recordTaskApproval(t, dir, task)
		require.Equal(t, 0, code,
			"approve: %d %s%s", code, out, errs)
	}
	{
		code, out, errs := runTaskScript(t, dir, "preflight.sh", task)
		require.False(t, code != 0 || !strings.Contains(out, "PREFLIGHT: OK"),
			"check: %d\n%s%s", code, out, errs)
	}

	// The build: the agent finishes the story and takes a snapshot.
	customPi(t, sessionScript("deepseek/deepseek-v4-flash")+"\n"+`echo "package main // done" > main.go`)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main // done\n"), 0o644)
	var buf bytes.Buffer
	{
		err := a.KitScriptTo(cfg, &buf, "snapshot.sh", task, "agent")
		require.NoError(t, err,
			"snapshot: %v", err)
	}

	id := strings.TrimSpace(buf.String())
	appendFile(t, filepath.Join(matches[0], "STORY_LOG.md"), "DONE US-001 "+id+"\n")
	{
		code, out, _ := runTaskScript(t, dir, "verify.sh", task)
		assert.Contains(t, out, "STORIES: done: US-001 | remaining: none | logged but snapshot not found: none",
			"a logged snapshot must count as the story's record (exit %d):\n%s", code, out)
	}

	// A story logged with an id that was never taken does not count.
	log := filepath.Join(matches[0], "STORY_LOG.md")
	_ = os.WriteFile(log, []byte("DONE US-001 S1-deadbeef\n"), 0o644)
	{
		_, out, _ := runTaskScript(t, dir, "verify.sh", task)
		assert.Contains(t, out, "logged but snapshot not found: US-001",
			"a made-up snapshot id must be refused:\n%s", out)
	}

	_ = os.WriteFile(log, []byte("DONE US-001 "+id+"\n"), 0o644)
	{
		code, out, errs := executeAgentActivity(t, &testApp{Context: a}, task, "build", false)
		require.Equal(t, 0, code,
			"build: %d\n%s%s", code, out, errs)
	}

	// What changed since the start can be read without any history.
	buf.Reset()
	{
		err := a.KitScriptTo(cfg, &buf, "evidence.sh", task, "diff")
		assert.False(t, err != nil || !strings.Contains(buf.String(), "+package main // done"),
			"evidence diff: %v\n%s", err, buf.String())
	}

	buf.Reset()
	_ = a.KitScriptTo(cfg, &buf, "evidence.sh", task, "diffstat")
	assert.Contains(t, buf.String(), "M main.go",
		"diffstat: %s", buf.String())

	// Review: the reviewer may write only its verdict.
	verdict := filepath.Join(matches[0], "review", "VERDICT.md")
	customPi(t, sessionScript("deepseek/deepseek-v4-flash")+"\n"+`echo "Looks right." > "`+verdict+`"; echo "VERDICT: PASS" >> "`+verdict+`"`)
	{
		code, out, errs := executeAgentActivity(t, &testApp{Context: a}, task, "review", false)
		require.Equal(t, 0, code,
			"review: %d\n%s%s", code, out, errs)
	}

	customPi(t, sessionScript("deepseek/deepseek-v4-flash")+"\n"+`echo "package main // tampered" > main.go; echo "VERDICT: PASS" > "`+verdict+`"`)
	code, out, errs := executeAgentActivity(t, &testApp{Context: a}, task, "review", false)
	assert.False(t, code == 0 || !strings.Contains(out+errs, "files changed outside") || !strings.Contains(out+errs, "main.go"),
		"a reviewer that edits source must be caught: %d\n%s%s", code, out, errs)

	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main // done\n"), 0o644)

	writeFile(t, filepath.Join(matches[0], "retrospective.md"), "ok\n")
	customPi(t, sessionScript("deepseek/deepseek-v4-flash")+"\n"+`echo "VERDICT: PASS" > "`+verdict+`"`)
	{
		code, out, errs := executeAgentActivity(t, &testApp{Context: a}, task, "review", false)
		require.Equal(t, 0, code,
			"second review: %d\n%s%s", code, out, errs)
	}

}

func TestLifecycleInAFolderWithNoVersionControl(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key-not-a-real-credential")
	dir, a := plainProject(t)
	gitTripwire(t)
	lifecycle(t, dir, a)
}

// The case that matters to people who do use git: a tracked project with uncommitted work in it.
// Watts must not care, must not switch branches, and must not touch .gitignore.
func TestLifecycleInATrackedRepository(t *testing.T) {
	needTools(t, "git")
	ta := newProject(t)
	mustRun(t, ta.Root, "git", "init", "-q", "-b", "main", ".")
	mustRun(t, ta.Root, "git", "add", "README.md")
	mustRun(t, ta.Root, "git", "commit", "-qm", "initial")
	writeFile(t, filepath.Join(ta.Root, "wip.txt"), "uncommitted work\n")
	before := ta.SnapshotPaths([]string{".git"})
	ignoreBefore, _ := os.ReadFile(filepath.Join(ta.Root, ".gitignore"))
	gitTripwire(t) // after the setup above, which needed git
	lifecycle(t, ta.Root, ta.Context)
	if _, err := os.Stat(filepath.Join(ta.Root, ".gitignore")); err == nil {
		after, _ := os.ReadFile(filepath.Join(ta.Root, ".gitignore"))
		assert.Equal(t, string(ignoreBefore), string(after),
			"the project's .gitignore must not be touched")
	}
	assert.True(t, reflect.DeepEqual(before, ta.SnapshotPaths([]string{".git"})),
		"Watts must leave all Git metadata unchanged")
}

func TestSnapshotIdsNeverCollide(t *testing.T) {
	dir, a := plainProject(t)
	{
		code, _, _ := runCLI(t, dir, "init", "--no-install")
		require.Equal(t, 0, code,
			"init")
	}

	cfg, _ := a.LoadConfig()
	{
		code, _, _ := runCLI(t, dir, "task", "new", "demo")
		require.Equal(t, 0, code,
			"task new")
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "tasks", "*-demo"))
	task := "tasks/" + filepath.Base(matches[0])
	take := func(label string) string {
		var buf bytes.Buffer

		require.NoError(t, a.KitScriptTo(cfg, &buf, "snapshot.sh", task, label))

		return strings.TrimSpace(buf.String())
	}
	agent, review := take("agent"), take("review-start")
	require.NotEqual(t, review, agent,
		"snapshots with different labels must have different ids")

	for id, label := range map[string]string{agent: "# label agent", review: "# label review-start"} {
		{
			got := strings.SplitN(readFile(t, filepath.Join(matches[0], ".watts-state", "snapshots", id)), "\n", 2)[0]
			assert.Equal(t, label, got,
				"snapshot %s has label line %q, want %q", id, got, label)
		}
	}
	{
		err := a.KitScriptTo(cfg, &bytes.Buffer{}, "snapshot.sh", task, "bogus")
		assert.Error(t, err,
			"an unknown label must be refused")
	}
}

func TestProjectRootIsFoundFromSubfolders(t *testing.T) {
	dir, _ := plainProject(t)
	{
		code, _, _ := runCLI(t, dir, "init", "--no-install")
		require.Equal(t, 0, code,
			"init")
	}

	sub := filepath.Join(dir, "a", "b")
	_ = os.MkdirAll(sub, 0o755)
	{
		code, out, _ := runCLI(t, sub, "config", "show")
		assert.False(t, code != 0 || !strings.Contains(out, `tasks_dir: tasks`),
			"the project root must be found from a subfolder: %d %s", code, out)
	}
}
