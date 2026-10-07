package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/workspace"
)

// Run executes one watts command and returns the process exit status. args excludes the program
// name. It never calls os.Exit and reads and writes only through the streams it is given, so
// callers (and tests) decide what happens to the status and the output.
func Run(args []string, in io.Reader, stdout, stderr io.Writer) int {
	r := resolve(args)
	if r.helpText != "" {
		fmt.Fprint(stdout, r.helpText)
		return 0
	}
	if r.run == nil {
		fmt.Fprint(stderr, r.errText)
		return r.code
	}
	a := terminal.New("", version, in, stdout, stderr)
	if err := a.Locate(); err != nil && r.needsRepo {
		fmt.Fprintf(stderr, "watts: %v\n", err)
		return 1
	}
	err := r.run(a, r.args)
	if err == nil {
		return 0
	}
	var ee workspace.ExitError
	if errors.As(err, &ee) {
		if ee.Message != "" {
			fmt.Fprintf(stderr, "watts: %s\n", ee.Message)
		}
		return ee.Code
	}
	var ue arguments.Error
	if errors.As(err, &ue) {
		fmt.Fprintf(stderr, "watts: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "watts: %v\n", err)
	return 1
}
