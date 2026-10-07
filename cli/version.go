package cli

import (
	"github.com/jasondeutsch/watts/cli/internal/terminal"
)

// Watts is a proof of concept. Architecture changes do not change its release number.
const version = "0.1.0"

func cmdVersion(a *terminal.Context, args []string) error { a.Say("watts %s", version); return nil }
