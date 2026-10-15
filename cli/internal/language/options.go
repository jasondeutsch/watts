package language

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jasondeutsch/watts/internal/workflowlang"
	"github.com/zclconf/go-cty/cty"
)

type repeated []string

func (values *repeated) String() string         { return strings.Join(*values, ", ") }
func (values *repeated) Set(value string) error { *values = append(*values, value); return nil }

type compilationFlags struct {
	workflow         string
	files, variables repeated
}

func (options *compilationFlags) register(flags *flag.FlagSet) {
	flags.StringVar(&options.workflow, "workflow", "", "named workflow entrypoint (overrides the package default)")
	flags.Var(&options.files, "var-file", "ACL variable file; repeat for overrides")
	flags.Var(&options.variables, "var", "name=ACL-expression; repeat to override variables")
}

func (options compilationFlags) resolve() (workflowlang.Options, error) {
	supplied := map[string]cty.Value{}
	for _, assignment := range options.variables {
		name, expression, ok := strings.Cut(assignment, "=")
		if !ok || !hclsyntax.ValidIdentifier(name) {
			return workflowlang.Options{}, fmt.Errorf("variable override must be name=ACL-expression")
		}
		parsed, diagnostics := hclsyntax.ParseExpression([]byte(expression), "-var "+name, hcl.InitialPos)
		if diagnostics.HasErrors() {
			return workflowlang.Options{}, diagnostics
		}
		value, diagnostics := parsed.Value(nil)
		if diagnostics.HasErrors() {
			return workflowlang.Options{}, diagnostics
		}
		supplied[name] = value
	}
	return workflowlang.Options{Workflow: options.workflow, VariableFiles: options.files, Variables: supplied}, nil
}

func suppliedFlag(flags *flag.FlagSet, names ...string) bool {
	found := false
	flags.Visit(func(f *flag.Flag) {
		for _, name := range names {
			if f.Name == name {
				found = true
			}
		}
	})
	return found
}
