package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/application"
	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/workflowlang"
)

func New(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "task new")
	compiled := flags.String("compiled", "", "frozen JSON artifact saved from wctl exec <package> --dry-run")
	positionals, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	slug, err := arguments.One(positionals, "task slug")
	if err != nil {
		return err
	}
	if *compiled == "" {
		return fmt.Errorf("task new requires --compiled <artifact.json>; save wctl exec <package> --dry-run output first")
	}
	app.Root = app.WorkingDirectory
	pkg, err := workflowlang.ReadCompiled(*compiled)
	if err != nil {
		return err
	}
	cfg, err := app.LoadServiceConfig()
	if err != nil {
		return err
	}
	_, err = app.CreateCompiledTask(cfg, slug, pkg)
	return err
}

func Run(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "task run")
	var options application.RunOptions
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
	actor := flags.String("actor", "", "coordinator identity for a delegated decision")
	authorizedBy := flags.String("authorized-by", "", "human who explicitly authorized this coordinator decision")
	reviewedStatus := flags.String("reviewed-status", "", "JSON status snapshot reviewed before deciding (required for delegation)")
	positionals, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	if len(positionals) > 2 {
		return fmt.Errorf("usage: wctl task decide [task] [stage] [--reject] [--feedback text]")
	}
	task, stage := "", ""
	if len(positionals) > 0 {
		task = positionals[0]
	}
	if len(positionals) > 1 {
		stage = positionals[1]
	}
	options := application.DecisionOptions{Reject: *reject, Feedback: *feedback, Actor: *actor, AuthorizedBy: *authorizedBy}
	if *reviewedStatus != "" {
		data, err := os.ReadFile(*reviewedStatus)
		if err != nil {
			return fmt.Errorf("read reviewed status: %w", err)
		}
		var view application.TemporalTaskStatus
		if err := json.Unmarshal(data, &view); err != nil {
			return fmt.Errorf("decode reviewed status: %w", err)
		}
		options.ReviewedStatus = &view
	}
	decision, err := app.SubmitTaskDecisionWithOptions(context.Background(), task, stage, options)
	if err != nil {
		return err
	}
	if decision.AuthorizedBy != "" {
		app.Say("Recorded delegated decision by %s, authorized by %s, for %s attempt %d (rejected: %v).", decision.By, decision.AuthorizedBy, decision.Step, decision.Attempt, decision.Reject)
	} else {
		app.Say("Recorded decision by %s for %s attempt %d (rejected: %v).", decision.By, decision.Step, decision.Attempt, decision.Reject)
	}
	return nil
}

func selectTask(app *terminal.Context, positionals []string, command string) (projectconfig.Config, string, error) {
	if len(positionals) > 1 {
		return projectconfig.Config{}, "", fmt.Errorf("usage: wctl task %s [task]", command)
	}
	task := ""
	if len(positionals) == 1 {
		task = positionals[0]
	}
	cfg, err := app.LoadServiceConfig()
	if err != nil {
		return cfg, "", err
	}
	selected, err := app.SelectTask(cfg, task)
	return cfg, selected, err
}
