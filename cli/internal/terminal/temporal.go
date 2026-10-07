package terminal

import (
	"context"
	"fmt"

	"github.com/jasondeutsch/watts/internal/application"
)

func (app *Context) TemporalStatus(taskPath string, jsonOut bool) error {
	view, err := app.TemporalTaskStatus(context.Background(), taskPath)
	if err != nil {
		return err
	}
	state := view.State
	var failure error
	if state.Status == "waiting_retry" || state.Status == "failed" || state.Status == "cancelled" || state.Status == "terminated" {
		failure = fmt.Errorf("TASK FAILED: %s is %s; inspect stage errors before retrying", taskPath, state.Status)
		for _, stage := range state.Stages {
			if stage.Status == "failed" && stage.LastError != "" {
				failure = fmt.Errorf("TASK FAILED: %s, stage %s, attempt %d: %s", taskPath, stage.Name, stage.Attempts, stage.LastError)
				break
			}
		}
	}
	if jsonOut {
		if err := Encoder(app.Output).Encode(view); err != nil {
			return err
		}
		return failure
	}
	app.Say("Task %s: %s (Temporal)", taskPath, state.Status)
	for _, stage := range state.Stages {
		app.Say("  %-18s %-18s attempts: %d", stage.Name, stage.Status, stage.Attempts)
		if stage.Outcome != "" {
			app.Say("    outcome: %s", stage.Outcome)
		}
		if stage.LastError != "" {
			app.Say("    %s", application.OneLine(stage.LastError, 1024))
		}
	}
	if state.Current != "" {
		app.Say("Current stage: %s", state.Current)
	}
	switch state.Status {
	case "not_started", "waiting_retry":
		app.Say("Next: watts task run %s", taskPath)
	case "waiting_approval":
		app.Say("Next: inspect the stage inputs, then watts task decide %s (or --reject --feedback text)", taskPath)
	}
	return failure
}
