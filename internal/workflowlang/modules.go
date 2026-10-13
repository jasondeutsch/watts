package workflowlang

import (
	"fmt"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// resolveModules follows explicit references rather than source-file order.
func (resolver *moduleResolver) resolveModules(blocks []*hclsyntax.Block, source, prefix string, ctx *hcl.EvalContext, result *resolvedModule) error {
	modules := map[string]cty.Value{}
	pendingModules := map[string]*hclsyntax.Block{}
	for _, block := range blocks {
		pendingModules[block.Labels[0]] = block
	}
	ctx.Variables["module"] = object(modules)
	for len(pendingModules) > 0 {
		progress := false
		var unresolved error
		for _, moduleName := range sortedKeys(pendingModules) {
			block := pendingModules[moduleName]
			if err := checkArguments(block.Body, "source", "version", "variables"); err != nil {
				return err
			}
			if len(block.Body.Blocks) > 0 {
				return fmt.Errorf("module does not accept nested blocks")
			}
			moduleSource, err := stringArgument(block.Body, "source", ctx, true)
			if err != nil {
				unresolved = err
				continue
			}
			requiredVersion, err := stringArgument(block.Body, "version", ctx, false)
			if err != nil {
				unresolved = err
				continue
			}
			if !filepath.IsAbs(moduleSource) {
				moduleSource = filepath.Join(filepath.Dir(source), moduleSource)
			}
			supplied := map[string]cty.Value{}
			if argument := block.Body.Attributes["variables"]; argument != nil {
				value, err := evaluate(argument.Expr, ctx)
				if err != nil {
					unresolved = err
					continue
				}
				if !value.Type().IsObjectType() && !value.Type().IsMapType() {
					return fmt.Errorf("%s: module variables must be an object", argument.Range())
				}
				supplied = value.AsValueMap()
			}
			child, err := resolver.resolveModule(moduleSource, prefix+block.Labels[0]+"-", Options{Variables: supplied})
			if err != nil {
				return fmt.Errorf("module %s: %w", block.Labels[0], err)
			}
			if requiredVersion != "" && requiredVersion != child.version {
				return fmt.Errorf("module %s version %s does not match required %s", block.Labels[0], child.version, requiredVersion)
			}
			modules[block.Labels[0]] = child.exports
			ctx.Variables["module"] = object(modules)
			result.resources = append(result.resources, child.resources...)
			result.assets = append(result.assets, child.assets...)
			result.paths = append(result.paths, child.paths...)
			result.dependencies = append(result.dependencies, Dependency{Kind: "module", Name: prefix + block.Labels[0], Source: child.source, Version: child.version, Digest: child.digest})
			result.dependencies = append(result.dependencies, child.dependencies...)
			for name, workflow := range child.workflows {
				result.workflows[name] = workflow
			}
			delete(pendingModules, moduleName)
			progress = true
		}
		if !progress {
			return fmt.Errorf("modules have unresolved references or a cycle: %w", unresolved)
		}
	}
	return nil
}
