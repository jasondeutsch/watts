package terminal

import (
	"fmt"
	"io"

	"github.com/jasondeutsch/watts/internal/application"
)

func (a *Context) Say(format string, args ...any) { fmt.Fprintf(a.Output, format+"\n", args...) }

func (a *Context) Warn(format string, args ...any) {
	fmt.Fprintf(a.ErrorOutput, "warning: "+format+"\n", args...)
}

// Context adapts application services to terminal input and output.
type Context struct{ *application.Service }

func New(root, release string, input io.Reader, output, errorOutput io.Writer) *Context {
	service := &application.Service{Release: release, Root: root, WorkingDirectory: root, Input: input, Output: output, ErrorOutput: errorOutput}
	app := &Context{Service: service}
	service.Report = func(event application.Event) {
		if len(event.Findings) > 0 {
			app.PrintFindings(event.Findings)
			return
		}
		if event.Warning {
			app.Warn("%s", event.Message)
		} else {
			app.Say("%s", event.Message)
		}
	}
	return app
}
