package workflowlang

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

func decodeVariables(data []byte, name string, result map[string]cty.Value) error {
	file, diagnostics := hclsyntax.ParseConfig(data, name, hcl.InitialPos)
	if diagnostics.HasErrors() {
		return diagnostics
	}
	body := file.Body.(*hclsyntax.Body)
	if len(body.Blocks) > 0 {
		return fmt.Errorf("%s: variable files contain arguments, not blocks", name)
	}
	for _, key := range sortedKeys(body.Attributes) {
		value, err := evaluate(body.Attributes[key].Expr, nil)
		if err != nil {
			return err
		}
		result[key] = value
	}
	return nil
}
func resolveVariables(blocks []*hclsyntax.Block, overrides map[string]cty.Value) (map[string]cty.Value, error) {
	values := map[string]cty.Value{}
	declared := map[string]bool{}
	for _, block := range blocks {
		name := block.Labels[0]
		declared[name] = true
		if err := checkArguments(block.Body, "type", "default", "description"); err != nil {
			return nil, err
		}
		if len(block.Body.Blocks) > 0 {
			return nil, fmt.Errorf("variable %s does not accept nested blocks", name)
		}
		attribute := block.Body.Attributes["type"]
		if attribute == nil {
			return nil, fmt.Errorf("variable %s requires type", name)
		}
		kind, diagnostics := typeexpr.TypeConstraint(attribute.Expr)
		if diagnostics.HasErrors() {
			return nil, diagnostics
		}
		value, provided := overrides[name]
		if !provided {
			attribute := block.Body.Attributes["default"]
			if attribute == nil {
				return nil, fmt.Errorf("variable %s requires a value", name)
			}
			var err error
			value, err = evaluate(attribute.Expr, nil)
			if err != nil {
				return nil, err
			}
		}
		converted, err := convert.Convert(value, kind)
		if err != nil || converted.IsNull() || !converted.IsWhollyKnown() {
			return nil, fmt.Errorf("variable %s must conform to %s: %v", name, kind.FriendlyName(), err)
		}
		values[name] = converted
	}
	for name := range overrides {
		if !declared[name] {
			return nil, fmt.Errorf("unknown variable %q", name)
		}
	}
	return values, nil
}
func resolveLocals(blocks []*hclsyntax.Block, ctx *hcl.EvalContext) (map[string]cty.Value, error) {
	pending := map[string]*hclsyntax.Attribute{}
	for _, block := range blocks {
		if len(block.Body.Blocks) > 0 {
			return nil, fmt.Errorf("locals contains arguments only")
		}
		for name, attribute := range block.Body.Attributes {
			if pending[name] != nil {
				return nil, fmt.Errorf("duplicate local %s", name)
			}
			pending[name] = attribute
		}
	}
	values := map[string]cty.Value{}
	for len(pending) > 0 {
		progress := false
		var last error
		ctx.Variables["local"] = object(values)
		for _, name := range sortedKeys(pending) {
			value, err := evaluate(pending[name].Expr, ctx)
			if err != nil {
				last = err
				continue
			}
			values[name] = value
			delete(pending, name)
			ctx.Variables["local"] = object(values)
			progress = true
		}
		if !progress {
			return nil, fmt.Errorf("locals have unresolved references or a cycle: %w", last)
		}
	}
	return values, nil
}
