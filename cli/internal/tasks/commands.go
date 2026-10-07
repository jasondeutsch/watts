package tasks

import (
	"context"
	"fmt"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/application"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

func New(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "task new")
	positionals, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	slug, err := arguments.One(positionals, "task slug")
	if err != nil {
		return err
	}
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
	_, err = app.CreateTask(cfg, slug, "")
	return err
}

func Run(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "task run")
	var options application.RunOptions
	arguments.RunFlags(flags, &options, false)
	arguments.WorkflowFlags(flags, &options)
	flags.BoolVar(&options.Force, "ignore-attempt-limit", false, "authorize another attempt after inspecting the failure")
	positionals, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	cfg, task, err := selectTask(app, positionals, "run")
	if err != nil {
		return err
	}
	return app.RunTask(cfg, task, &options)
}

func Status(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "task status")
	jsonOutput := flags.Bool("json", false, "print workflow status and attempt history as JSON")
	positionals, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	_, task, err := selectTask(app, positionals, "status")
	if err != nil {
		return err
	}
	return app.TemporalStatus(task, *jsonOutput)
}

func Cancel(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "task cancel")
	positionals, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	_, task, err := selectTask(app, positionals, "cancel")
	if err != nil {
		return err
	}
	return app.CancelTemporalWorkflow(task)
}

func Decide(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "task decide")
	feedback := flags.String("feedback", "", "feedback for the next agent attempt")
	reject := flags.Bool("reject", false, "reject the pending human stage")
	positionals, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	if len(positionals) > 2 {
		return fmt.Errorf("usage: watts task decide [task] [stage] [--reject] [--feedback text]")
	}
	task, stage := "", ""
	if len(positionals) > 0 {
		task = positionals[0]
	}
	if len(positionals) > 1 {
		stage = positionals[1]
	}
	decision, err := app.SubmitTaskDecision(context.Background(), task, stage, *reject, *feedback)
	if err != nil {
		return err
	}
	app.Say("Recorded decision by %s for %s (rejected: %v).", decision.By, decision.Step, decision.Reject)
	return nil
}

func selectTask(app *terminal.Context, positionals []string, command string) (projectconfig.Config, string, error) {
	if len(positionals) > 1 {
		return projectconfig.Config{}, "", fmt.Errorf("usage: watts task %s [task]", command)
	}
	task := ""
	if len(positionals) == 1 {
		task = positionals[0]
	}
	cfg, err := app.LoadConfig()
	if err != nil {
		return cfg, "", err
	}
	selected, err := app.SelectTask(cfg, task)
	return cfg, selected, err
}
