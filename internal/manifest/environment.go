package manifest

import "regexp"

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validEnvironment(name string) bool {
	if !environmentName.MatchString(name) {
		return false
	}
	switch name {
	case "HOME", "PATH", "TERM", "LANG", "PI_CODING_AGENT_DIR", "PI_SKIP_VERSION_CHECK", "PI_TELEMETRY", "PI_OFFLINE":
		return false
	}
	return len(name) < 6 || name[:6] != "WATTS_"
}
