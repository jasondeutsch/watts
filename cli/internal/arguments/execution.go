package arguments

import (
	"flag"

	"github.com/jasondeutsch/watts/internal/application"
)

func WorkflowFlags(flags *flag.FlagSet, options *application.RunOptions) {
	flags.BoolVar(&options.DryRun, "dry-run", false, "show the pinned workflow without submitting or retrying it")
}
