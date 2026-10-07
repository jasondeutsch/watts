package arguments

import (
	"flag"

	"github.com/jasondeutsch/watts/internal/application"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

func RunFlags(fs *flag.FlagSet, rf *application.RunOptions, withAgent bool) {
	fs.Var((*StringList)(&rf.Pass), "env-passthrough", "environment variable name to pass into Pi, in addition to env_passthrough in "+projectconfig.Filename+" (repeatable)")
	fs.BoolVar(&rf.Offline, "offline", false, "set PI_OFFLINE=1 for this run")
	if withAgent {
		fs.StringVar(&rf.Agent, "agent", "build", "which agent to open (build, review, or a custom agent)")
	}
}

func WorkflowFlags(flags *flag.FlagSet, options *application.RunOptions) {
	flags.BoolVar(&options.DryRun, "dry-run", false, "show the pinned workflow without submitting or retrying it")
}
