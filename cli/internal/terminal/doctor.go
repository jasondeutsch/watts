package terminal

import (
	projectconfig "github.com/jasondeutsch/watts/internal/config"
	"github.com/jasondeutsch/watts/internal/workspace"
)

type DoctorOptions struct {
	Live bool
	JSON bool
}

// Doctor is the text form, kept for callers that do not need options.
func (a *Context) Doctor(cfg projectconfig.Config, haveConfig bool) error {
	return a.DoctorWith(cfg, haveConfig, DoctorOptions{})
}

func (a *Context) DoctorWith(cfg projectconfig.Config, haveConfig bool, opt DoctorOptions) error {
	checks := a.DoctorChecks(cfg, haveConfig, opt.Live)
	failed := false
	for _, c := range checks {
		if c.Level == "fail" {
			failed = true
		}
	}
	if opt.JSON {
		_ = Encoder(a.Output).Encode(map[string]any{"ok": !failed, "checks": checks})
		if failed {
			return workspace.ExitError{Code: 1}
		}
		return nil
	}
	for _, c := range checks {
		label := map[string]string{"ok": "ok:   ", "warn": "warn: ", "fail": "FAIL: "}[c.Level]
		text := c.Name
		if c.Detail != "" {
			text += ": " + c.Detail
		}
		if c.Fix != "" {
			text += ". Fix: " + c.Fix
		}
		a.Say("%s %s", label, text)
	}
	if failed {
		a.Say("DOCTOR: problems found")
		return workspace.ExitError{Code: 1}
	}
	a.Say("DOCTOR: OK")
	return nil
}
