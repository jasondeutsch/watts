// Package language adapts workflow compilation and execution services to the CLI.
package language

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/application"
	"github.com/jasondeutsch/watts/internal/storage"
	"github.com/jasondeutsch/watts/internal/workflowlang"
	"github.com/zclconf/go-cty/cty"
)

type repeated []string

func (values *repeated) String() string         { return strings.Join(*values, ", ") }
func (values *repeated) Set(value string) error { *values = append(*values, value); return nil }

func Compile(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "compile")
	selected := flags.String("workflow", "", "named workflow entrypoint (overrides the package default)")
	output := flags.String("o", "", "compiled JSON artifact (default: stdout)")
	var variableFiles, variables repeated
	flags.Var(&variableFiles, "var-file", "ACL variable file; repeat for overrides")
	flags.Var(&variables, "var", "name=ACL-expression; repeat to override variables")
	flags.Usage = func() {
		fmt.Fprintln(app.ErrorOutput, "Usage: wctl compile <package> [-workflow name] [-var-file file.wvars] [-var 'model=\"...\"'] [-o workflow.json]")
		flags.PrintDefaults()
	}
	positional, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	source, err := arguments.One(positional, "ACL package path")
	if err != nil {
		return arguments.UsageError(err)
	}
	supplied := map[string]cty.Value{}
	for _, assignment := range variables {
		name, expression, ok := strings.Cut(assignment, "=")
		if !ok || !hclsyntax.ValidIdentifier(name) {
			return fmt.Errorf("variable override must be name=ACL-expression")
		}
		parsed, diagnostics := hclsyntax.ParseExpression([]byte(expression), "-var "+name, hcl.InitialPos)
		if diagnostics.HasErrors() {
			return diagnostics
		}
		value, diagnostics := parsed.Value(nil)
		if diagnostics.HasErrors() {
			return diagnostics
		}
		supplied[name] = value
	}
	pkg, err := app.CompileWorkflow(source, workflowlang.Options{Workflow: *selected, VariableFiles: variableFiles, Variables: supplied})
	if err != nil {
		return err
	}
	if *output != "" {
		target, err := filepath.Abs(*output)
		if err != nil {
			return err
		}
		// Compilation must never overwrite a source or asset, even through symlinks.
		if resolved, err := filepath.EvalSymlinks(target); err == nil {
			target = resolved
		}
		for _, input := range pkg.SourcePaths {
			input, err := filepath.EvalSymlinks(input)
			if err != nil {
				continue
			}
			relative, err := filepath.Rel(input, target)
			if err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
				return fmt.Errorf("compiled output overlaps source %s", input)
			}
		}
		if err := storage.WriteJSONAtomically(target, pkg); err != nil {
			return err
		}
		fmt.Fprintf(app.Output, "Compiled %s@%s: %d stages, %d dependencies\nArtifact: %s\nSHA256: %s\n", pkg.Name, pkg.Version, len(pkg.Definition.Steps), len(pkg.Dependencies), *output, pkg.Digest)
		return nil
	}
	encoder := json.NewEncoder(app.Output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(pkg)
}

func Exec(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "exec")
	name := flags.String("name", "", "task slug for a new execution of a compiled artifact")
	request := flags.String("request", "", "initial SPEC.md text for the new task")
	dryRun := flags.Bool("dry-run", false, "describe the compiled execution without creating or submitting a task")
	force := flags.Bool("ignore-attempt-limit", false, "allow an explicit retry past the attempt limit")
	flags.Usage = func() {
		fmt.Fprintln(app.ErrorOutput, "Usage: wctl exec <compiled.json> --name <task-slug> [--request text]\n       wctl exec <task-folder> [--ignore-attempt-limit]")
		flags.PrintDefaults()
	}
	positional, err := arguments.Parse(flags, args)
	if err != nil {
		return arguments.UsageError(err)
	}
	source, err := arguments.One(positional, "compiled artifact or task folder")
	if err != nil {
		return arguments.UsageError(err)
	}
	options := &application.RunOptions{DryRun: *dryRun, Force: *force}
	if info, err := os.Stat(source); err == nil && !info.IsDir() {
		if *force {
			return fmt.Errorf("--ignore-attempt-limit applies to an existing task")
		}
		if *name == "" && !*dryRun {
			return fmt.Errorf("executing a compiled artifact requires --name <task-slug>")
		}
		app.Root = app.WorkingDirectory
		_, err := app.ExecuteCompiled(context.Background(), source, *name, *request, options)
		return err
	}
	if *name != "" || *request != "" {
		return fmt.Errorf("--name and --request apply to a new compiled-artifact execution")
	}
	cfg, err := app.LoadServiceConfig()
	if err != nil {
		return err
	}
	task, err := app.TaskArg(cfg, source)
	if err != nil {
		return err
	}
	return app.RunTask(cfg, task, options)
}
