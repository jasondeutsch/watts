package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRunsLoopThroughIsolatedPi(t *testing.T) {
	needTools(t, "awk", "timeout")
	ta, task := taskRepo(t)
	logFile := stubBin(t, "pi 1.0.1")
	t.Setenv("SECRET_TEST_KEY", "leak-me")

	code, out, errs := executeAgentActivity(t, ta, task, "build", true)
	require.Equal(t, 0, code,
		"build: exit %d\n%s%s", code, out, errs)

	log := readFile(t, logFile)
	assert.Contains(t, log, "ARGS: -p /ralph --path ./"+task+"\n",
		"the loop must be started with the slash command:\n%s", log)

	for _, want := range []string{"WATTS_ROLE=build", "PI_OFFLINE=1", "STDIN_BYTES: 0", "HOME=" + ta.HomeDir("build")} {
		assert.Contains(t, log, want,
			"stub log lacks %q", want)
	}
	assert.NotContains(t, log, "SECRET_TEST_KEY",
		"a host secret reached the loop")

	writeFile(t, filepath.Join(ta.Root, "stray.txt"), "dirty")
	{
		code, _, errs := executeCheckActivity(t, ta, task, "preflight")
		assert.Equal(t, 0, code,
			"a stray file must not stop the build: exit %d, %q", code, errs)
	}

	logBefore := readFile(t, logFile)

	require.NoError(t, os.Remove(filepath.Join(ta.Root, task, "PLAN.md")))

	{
		code, _, errs := executeCheckActivity(t, ta, task, "preflight")
		assert.False(t, code == 0 || !strings.Contains(errs, "PLAN.md"),
			"a failed check must stop the build: exit %d, %q", code, errs)
	}
	assert.Equal(t, logBefore, readFile(t, logFile),
		"Pi must not be started when preflight fails")
}
func TestReviewRequiresFinishedBuild(t *testing.T) {
	needTools(t, "awk", "timeout")
	ta, task := taskRepo(t)
	logFile := stubBin(t, "pi 1.0.1")
	code, _, errs := executeCheckActivity(t, ta, task, "pre-review")
	require.False(t, code == 0 || !strings.Contains(errs, "build is not finished"),
		"review before the build is done must fail: exit %d, %q", code, errs)
	assert.False(t, exists(logFile) && strings.Contains(readFile(t, logFile), "--path"),
		"the reviewer must not start before every story is done")
}
