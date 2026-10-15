package language

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/application"
	"github.com/jasondeutsch/watts/internal/workflowlang"
)

func Exec(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "exec")
	name := flags.String("name", "", "name for a new task (spaces and uppercase are accepted)")
	request := flags.String("request", "", "initial SPEC.md text for the new task")
	requestFile := flags.String("request-file", "", "read initial SPEC.md from a file; use - for stdin")
	dryRun := flags.Bool("dry-run", false, "describe the execution without creating or submitting a task")
	force := flags.Bool("ignore-attempt-limit", false, "allow an explicit retry past the attempt limit")
	var compilation compilationFlags
	compilation.register(flags)
	flags.Usage = func() {
		fmt.Fprintln(app.ErrorOutput, "Usage: wctl exec <package|compiled.json> --name <task-name> [--request text | --request-file file|-]\n       wctl exec <task-folder> [--ignore-attempt-limit]")
		flags.PrintDefaults()
	}
	positional, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	source, err := arguments.One(positional, "package, compiled artifact, or existing task folder")
	if err != nil {
		return arguments.UsageError(err)
	}
	options := &application.RunOptions{DryRun: *dryRun, Force: *force}
	selectionSupplied := suppliedFlag(flags, "workflow", "var-file", "var")
	newTaskSupplied := suppliedFlag(flags, "name", "request", "request-file") || selectionSupplied
	cfg, err := app.LoadServiceConfig()
	if err != nil {
		return err
	}
	// Existing tasks take precedence, even if somebody added ACL to their folder.
	// Reject new-task flags before reading stdin or touching the captured definition.
	if task, taskErr := app.TaskArg(cfg, source); taskErr == nil {
		if newTaskSupplied {
			return fmt.Errorf("--name, --request, --request-file and compilation flags apply only to a new task; retry uses the existing task's pinned configuration")
		}
		return app.RunTask(cfg, task, options)
	}
	if *force {
		return fmt.Errorf("--ignore-attempt-limit applies to an existing task")
	}
	if *name == "" && !*dryRun {
		return fmt.Errorf("starting a new task requires --name <task-name>")
	}
	text, err := requestText(*request, *requestFile, suppliedFlag(flags, "request"), suppliedFlag(flags, "request-file"), app.Input)
	if err != nil {
		return err
	}
	app.Root = app.WorkingDirectory // Package location never chooses the application root.
	if workflowlang.IsSource(source) {
		compiled, err := compilation.resolve()
		if err != nil {
			return err
		}
		_, err = app.ExecutePackage(context.Background(), source, *name, text, compiled, options)
		return err
	}
	if selectionSupplied {
		return fmt.Errorf("compilation flags apply to ACL packages; compiled artifacts are already resolved")
	}
	if info, err := os.Stat(source); err != nil {
		return err
	} else if info.IsDir() {
		return fmt.Errorf("%s is neither an existing task nor an ACL package (expected watts.acl)", source)
	}
	_, err = app.ExecuteCompiled(context.Background(), source, *name, text, options)
	return err
}

func requestText(text, filename string, hasText, hasFile bool, input io.Reader) (string, error) {
	if hasText && hasFile {
		return "", fmt.Errorf("choose either --request or --request-file; they cannot be combined")
	}
	if !hasFile {
		return text, nil
	}
	if filename == "" {
		return "", fmt.Errorf("--request-file needs a filename or - for stdin")
	}
	var data []byte
	var err error
	if filename == "-" {
		data, err = io.ReadAll(input)
	} else {
		data, err = os.ReadFile(filename)
	}
	if err != nil {
		return "", fmt.Errorf("read request from %s: %w", filename, err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("request from %s is empty", filename)
	}
	return string(data), nil
}
