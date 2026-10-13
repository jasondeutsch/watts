package setup

import (
	"context"
	"errors"
	"github.com/jasondeutsch/watts/internal/workflowlang"
	"os"
	"os/signal"
	"syscall"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	projectruntime "github.com/jasondeutsch/watts/internal/runtime"
)

func Doctor(a *terminal.Context, args []string) error {
	fs := arguments.NewFlags(a.ErrorOutput, "doctor")
	compiled := fs.String("compiled", "", "compiled workflow artifact to check")

	asJSON := fs.Bool("json", false, "print the checks as JSON")
	if _, err := arguments.Parse(fs, args); err != nil {
		return arguments.UsageError(err)
	}
	if *compiled == "" {
		return errors.New("doctor requires --compiled <artifact.json>")
	}
	pkg, err := workflowlang.ReadCompiled(*compiled)
	if err != nil {
		return err
	}
	cfg, err := a.LoadServiceConfig()
	if err != nil {
		return err
	}
	cfg.Workflow = &pkg.Definition
	return a.DoctorWith(cfg, true, terminal.DoctorOptions{JSON: *asJSON})
}

func Start(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "start")
	detached := flags.Bool("d", false, "run worker and web UI in the background")
	positional, err := arguments.Parse(flags, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return errors.New("usage: wctl start [-d]")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return projectruntime.Start(ctx, app.Service, *detached)
}
func Stop(app *terminal.Context, args []string) error {
	if len(args) > 0 {
		return errors.New("usage: wctl stop")
	}
	return projectruntime.Stop(context.Background(), app.Service)
}
