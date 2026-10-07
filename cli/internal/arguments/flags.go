package arguments

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jasondeutsch/watts/internal/workspace"
)

// StringList is a repeatable flag that also accepts comma separated values.
type StringList []string

func (s *StringList) String() string { return strings.Join(*s, ",") }

func (s *StringList) Set(v string) error {
	for _, n := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
		*s = append(*s, n)
	}
	return nil
}

func NewFlags(errorOutput io.Writer, name string) *flag.FlagSet {
	fs := flag.NewFlagSet("watts "+name, flag.ContinueOnError)
	fs.SetOutput(errorOutput)
	return fs
}

// Parse lets flags appear before or after positional arguments. Everything after a bare "--"
// is passed through untouched as positional arguments, so `watts agent pi -- mcp add x -- npx -y y`
// works.
func Parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var verbatim []string
	for i, a := range args {
		if a == "--" {
			verbatim = append([]string(nil), args[i+1:]...)
			args = args[:i]
			break
		}
	}
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return append(pos, verbatim...), nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func One(pos []string, what string) (string, error) {
	switch len(pos) {
	case 1:
		return pos[0], nil
	case 0:
		return "", fmt.Errorf("missing %s", what)
	default:
		return "", fmt.Errorf("expected one %s, got %d arguments", what, len(pos))
	}
}

type Error struct{ err error }

func (u Error) Error() string { return u.err.Error() }

func UsageError(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return workspace.ExitError{Code: 0}
	}
	return Error{err}
}
