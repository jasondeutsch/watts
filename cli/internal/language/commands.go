// Package language adapts workflow compilation and execution services to the CLI.
package language

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jasondeutsch/watts/cli/internal/arguments"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
	"github.com/jasondeutsch/watts/internal/storage"
)

func Compile(app *terminal.Context, args []string) error {
	flags := arguments.NewFlags(app.ErrorOutput, "compile")
	var compilation compilationFlags
	compilation.register(flags)
	output := flags.String("o", "", "compiled JSON artifact (default: stdout)")
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
	options, err := compilation.resolve()
	if err != nil {
		return err
	}
	pkg, err := app.CompileWorkflow(source, options)
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
