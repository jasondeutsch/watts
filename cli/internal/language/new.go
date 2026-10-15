package language

import (
	"fmt"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
)

func New(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "new")
	source := flags.String("from", "", "local ACL package to import (required)")
	var compilation compilationFlags
	compilation.register(flags)
	flags.Usage = func() {
		fmt.Fprintln(app.ErrorOutput, "Usage: wctl new <directory> --from <package> [--workflow name] [--var-file file.wvars]")
		flags.PrintDefaults()
	}
	positional, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	destination, err := arguments.One(positional, "new package directory")
	if err != nil {
		return arguments.UsageError(err)
	}
	if *source == "" {
		return fmt.Errorf("--from <package> is required; scaffolding imports a local package")
	}
	options, err := compilation.resolve()
	if err != nil {
		return err
	}
	created, err := app.NewPackage(destination, *source, options)
	if err != nil {
		return err
	}
	app.Say("Created package %s. Edit values.wvars, then from the application root run: wctl exec %s --name <task-name> --request-file <request.md>", created, destination)
	return nil
}
