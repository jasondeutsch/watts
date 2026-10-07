package kit

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKitScripts runs the bash suite for the check scripts in kit/scripts. They are the part of
// Watts that the loop prompts call by path, so they stay bash and are tested in their own suite.
func TestKitScripts(t *testing.T) {
	needTools(t, "bash", "awk")
	out, err := exec.Command("bash", "tests/scripts.sh").CombinedOutput()
	text := string(out)
	if err != nil || !strings.Contains(text, "failed: 0") {
		var failed []string
		for _, l := range strings.Split(text, "\n") {
			if strings.Contains(l, "FAIL") {
				failed = append(failed, l)
			}
		}
		require.FailNow(t, fmt.Sprintf("bash suite failed: %v\n%s\nRaw output:\n%s", err, strings.Join(failed, "\n"), text))
	}
	t.Log(text[strings.LastIndex(text, "passed:"):])
}

func needTools(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s unavailable", name)
		}
	}
}
