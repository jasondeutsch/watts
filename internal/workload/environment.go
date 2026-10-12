package workload

import (
	"regexp"
	"strings"
)

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func EnvironmentSource(name string) bool { return environmentName.MatchString(name) }

func EnvironmentTarget(name string) bool {
	if !EnvironmentSource(name) || strings.HasPrefix(name, "WATTS_") {
		return false
	}
	switch name {
	case "HOME", "TMPDIR", "PATH", "TERM", "LANG":
		return false
	}
	return true
}
