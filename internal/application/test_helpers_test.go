package application

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/kit"
)

func mustRun(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if name == "git" {
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Human Dev", "GIT_AUTHOR_EMAIL=h@example.com",
			"GIT_COMMITTER_NAME=Human Dev", "GIT_COMMITTER_EMAIL=h@example.com")
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err,
		"%s %v: %v\n%s", name, args, err, out)

	return string(out)
}

func needTools(t *testing.T, names ...string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the check scripts need bash; use WSL on Windows")
	}
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("%s not available", n)
		}
	}
}

type testApp struct {
	*Service
	Output, ErrorOutput *bytes.Buffer
}

// newProject makes a plain project folder and returns an App rooted in it.
func newProject(t *testing.T) *testApp {
	t.Setenv("OPENROUTER_API_KEY", "test-key-not-a-real-credential")
	t.Helper()
	needTools(t, "bash")
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644))

	ta := &testApp{Output: &bytes.Buffer{}, ErrorOutput: &bytes.Buffer{}}
	ta.Service = testService(dir, strings.NewReader(""), ta.Output, ta.ErrorOutput)
	return ta
}

// initRepo runs `watts init --no-install` in the test repository.
func (ta *testApp) initRepo(t *testing.T, args ...string) {
	t.Helper()
	// Most tests use the classic tasks folder. Tests of the default pass their own flags, which come
	// later and win.
	cfg := projectconfig.Default()
	cfg.TasksDir = "tasks"
	for i := 0; i < len(args); i += 2 {
		switch args[i] {
		case "--provider":
			setWorkflowModels(&cfg, args[i+1], "", "")
		case "--build-model":
			setWorkflowModels(&cfg, "", args[i+1], "")
		case "--review-model":
			setWorkflowModels(&cfg, "", "", args[i+1])
		default:
			require.
				FailNow(t, fmt.Sprintf("unsupported application fixture option: %s", args[i]))
		}
	}
	{
		err := kit.Initialize(ta.KitDir())
		require.NoError(t, err)
	}

	require.NoError(t, ta.SaveConfig(cfg))

	require.NoError(t, ta.WriteLocalIgnores())

	require.NoError(t, ta.ConfigAgents(cfg, false))
}

func (ta *testApp) reset() { ta.Output.Reset(); ta.ErrorOutput.Reset() }

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(data)
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

// stubBin writes executable stand-ins into a fresh directory and puts it first on PATH.
// pi records its arguments, stdin length and environment in the returned log file.
func stubBin(t *testing.T, piVersionOut string) (logFile string) {
	t.Helper()
	bin := t.TempDir()
	logFile = filepath.Join(bin, "pi.log")
	pi := `#!/usr/bin/env bash
case "${1:-}" in
  --version) echo "` + piVersionOut + `"; exit 0 ;;
  list)      cat "$PI_CODING_AGENT_DIR/installed.txt" 2>/dev/null; exit 0 ;;
  install)   echo "$2" >> "$PI_CODING_AGENT_DIR/installed.txt"; exit 0 ;;
esac
{ echo "ARGS: $*"; echo "STDIN_BYTES: $(timeout 2 cat | wc -c | tr -d ' ')"; echo "PWD: $PWD"; env | sort; echo "---"; } >> "` + logFile + `"
`
	for name, body := range map[string]string{
		"pi":   pi,
		"node": "#!/bin/sh\necho v22.22.1\n",
		"npm":  "#!/bin/sh\necho \"npm $*\" >> \"" + logFile + "\"\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755))
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logFile
}

// defaultConfigFor is the configuration of a test repository, with the classic tasks folder.
func defaultConfigFor(ta *testApp) projectconfig.Config {
	cfg := projectconfig.Default()
	cfg.TasksDir = "tasks"
	if c, err := ta.LoadConfig(); err == nil {
		return c
	}
	return cfg
}

func testService(root string, input io.Reader, output, errorOutput io.Writer) *Service {
	service := &Service{Root: root, WorkingDirectory: root, Release: "0.1.0", Input: input, Output: output, ErrorOutput: errorOutput}
	service.Report = func(event Event) {
		writer := output
		if event.Warning {
			writer = errorOutput
		}
		for _, finding := range event.Findings {
			fmt.Fprintf(writer, "%s: %s. Fix: %s\n", finding.Level, finding.Message, finding.Fix)
		}
		fmt.Fprintln(writer, event.Message)
	}
	return service
}
