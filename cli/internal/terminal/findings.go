package terminal

import (
	"github.com/jasondeutsch/watts/internal/application"
)

func (a *Context) PrintFindings(fs []application.Finding) {
	for _, f := range fs {
		label := "warning"
		if f.Level == "fail" {
			label = "error"
		}
		a.Say("%s (%s agent): %s", label, f.Agent, f.Message)
		if f.Fix != "" {
			a.Say("  fix: %s", f.Fix)
		}
	}
}
