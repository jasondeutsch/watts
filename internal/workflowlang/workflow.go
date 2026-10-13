package workflowlang

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/orchestration"
	"github.com/zclconf/go-cty/cty"
)

type stageBindings struct {
	Inputs                []string
	Outputs               []string
	HasInputs, HasOutputs bool
	Dependencies          []string
}
type workflowBindings map[string]stageBindings

func compileWorkflow(block *hclsyntax.Block, prefix string, parent *hcl.EvalContext) (manifest.Resource, workflowBindings, error) {
	bindings := workflowBindings{}
	if err := checkArguments(block.Body, "version", "max_transitions", "templates"); err != nil {
		return manifest.Resource{}, bindings, err
	}
	fields, err := attributes(block.Body, parent)
	if err != nil {
		return manifest.Resource{}, bindings, err
	}
	stages := map[string]cty.Value{}
	positions := map[string]int{}
	for i, stage := range block.Body.Blocks {
		if stage.Type != "stage" || len(stage.Labels) != 1 || !orchestration.ValidName(stage.Labels[0]) {
			return manifest.Resource{}, bindings, fmt.Errorf("%s: workflow requires named stage blocks", stage.TypeRange)
		}
		name := stage.Labels[0]
		if _, exists := positions[name]; exists {
			return manifest.Resource{}, bindings, fmt.Errorf("duplicate stage %s", name)
		}
		positions[name] = i
		stages[name] = object(map[string]cty.Value{"name": cty.StringVal(name), "outputs": cty.EmptyObjectVal})
	}
	ctx := parent.NewChild()
	ctx.Variables = map[string]cty.Value{"stage": object(stages)}
	var placements []any
	for index, stage := range block.Body.Blocks {
		name := stage.Labels[0]
		if err := checkArguments(stage.Body, "executor", "inputs", "outputs", "parameters", "next", "on_failure", "optional"); err != nil {
			return manifest.Resource{}, bindings, err
		}
		values, err := attributes(stage.Body, ctx)
		if err != nil {
			return manifest.Resource{}, bindings, err
		}
		kind, executor, err := referenceValue(values["executor"])
		if err != nil {
			return manifest.Resource{}, bindings, fmt.Errorf("stage %s: %w", name, err)
		}
		if kind != manifest.Agent && kind != manifest.Procedure {
			return manifest.Resource{}, bindings, fmt.Errorf("stage %s executor must be an Agent or Procedure", name)
		}
		item := stageBindings{}
		if attribute := stage.Body.Attributes["inputs"]; attribute != nil {
			item.HasInputs = true
			item.Inputs, err = stringListArgument(stage.Body, "inputs", ctx)
			if err != nil {
				return manifest.Resource{}, bindings, err
			}
			dependencies := map[string]bool{}
			for _, traversal := range attribute.Expr.Variables() {
				if traversal.RootName() != "stage" {
					continue
				}
				if len(traversal) < 4 {
					return manifest.Resource{}, bindings, fmt.Errorf("%s: stage inputs reference stage.<name>.outputs.<artifact>", traversal.SourceRange())
				}
				dependency, ok := traversal[1].(hcl.TraverseAttr)
				if !ok {
					return manifest.Resource{}, bindings, fmt.Errorf("stage inputs require a named stage reference")
				}
				position, exists := positions[dependency.Name]
				if !exists || position >= index {
					return manifest.Resource{}, bindings, fmt.Errorf("stage %s has a forward or cyclic input dependency on %s", name, dependency.Name)
				}
				dependencies[dependency.Name] = true
			}
			item.Dependencies = sortedKeys(dependencies)
		}
		outputs := map[string]cty.Value{}
		if attribute := stage.Body.Attributes["outputs"]; attribute != nil {
			value, err := evaluate(attribute.Expr, ctx)
			if err != nil {
				return manifest.Resource{}, bindings, err
			}
			if !value.Type().IsObjectType() && !value.Type().IsMapType() {
				return manifest.Resource{}, bindings, fmt.Errorf("%s: outputs must map artifact names to task-relative paths", attribute.Range())
			}
			item.HasOutputs = true
			for _, key := range sortedKeys(value.AsValueMap()) {
				artifact := value.AsValueMap()[key]
				if artifact.IsNull() || artifact.Type() != cty.String || !orchestration.ValidArtifact(artifact.AsString()) {
					return manifest.Resource{}, bindings, fmt.Errorf("stage %s has invalid output %s", name, key)
				}
				outputs[key] = artifact
				item.Outputs = append(item.Outputs, artifact.AsString())
			}
		}
		placement := map[string]any{"name": name, "use": map[string]any{"kind": kind, "name": executor}, "depends_on": item.Dependencies}
		for _, key := range []string{"parameters", "next", "on_failure", "optional"} {
			if value, ok := values[key]; ok {
				placement[key] = value
			}
		}
		transitions := map[string]string{}
		limits := map[string]int{}
		for _, route := range stage.Body.Blocks {
			if route.Type != "on" || len(route.Labels) != 1 {
				return manifest.Resource{}, bindings, fmt.Errorf("%s: stage accepts named on blocks", route.TypeRange)
			}
			outcome := route.Labels[0]
			if !orchestration.ValidName(outcome) {
				return manifest.Resource{}, bindings, fmt.Errorf("invalid outcome %s", outcome)
			}
			if _, exists := transitions[outcome]; exists {
				return manifest.Resource{}, bindings, fmt.Errorf("duplicate outcome %s", outcome)
			}
			if err := checkArguments(route.Body, "return_to", "max_iterations"); err != nil {
				return manifest.Resource{}, bindings, err
			}
			if len(route.Body.Blocks) > 0 {
				return manifest.Resource{}, bindings, fmt.Errorf("on does not accept nested blocks")
			}
			target, err := stringArgument(route.Body, "return_to", ctx, true)
			if err != nil {
				return manifest.Resource{}, bindings, err
			}
			targetIndex, exists := positions[target]
			if !exists && target != "end" {
				return manifest.Resource{}, bindings, fmt.Errorf("stage %s references unknown target %s", name, target)
			}
			if attribute := route.Body.Attributes["max_iterations"]; attribute != nil {
				value, err := evaluate(attribute.Expr, ctx)
				if err != nil {
					return manifest.Resource{}, bindings, err
				}
				if value.Type() != cty.Number {
					return manifest.Resource{}, bindings, fmt.Errorf("max_iterations must be a positive integer")
				}
				number, accuracy := value.AsBigFloat().Int64()
				if accuracy != 0 || number < 1 || number > 10000 {
					return manifest.Resource{}, bindings, fmt.Errorf("max_iterations must be an integer between 1 and 10000")
				}
				limits[outcome] = int(number)
			}
			if target != "end" && targetIndex <= index && limits[outcome] == 0 {
				return manifest.Resource{}, bindings, fmt.Errorf("stage %s outcome %s loops backward; max_iterations is required", name, outcome)
			}
			transitions[outcome] = target
		}
		placement["transitions"] = transitions
		placement["transition_limits"] = limits
		placements = append(placements, placement)
		bindings[name] = item
		stages[name] = object(map[string]cty.Value{"name": cty.StringVal(name), "outputs": object(outputs)})
		ctx.Variables["stage"] = object(stages)
	}
	fields["steps"] = placements
	resource, err := makeResource(manifest.Workflow, prefix+block.Labels[0], fields)
	return resource, bindings, err
}

func (module resolvedModule) compileDefinition() (orchestration.Definition, error) {
	var resources []manifest.Resource
	selected := false
	seen := map[manifest.Reference]bool{}
	for _, resource := range module.resources {
		ref := manifest.Reference{Kind: resource.Kind, Name: resource.Name}
		if seen[ref] {
			return orchestration.Definition{}, fmt.Errorf("duplicate qualified resource %s %s", ref.Kind, ref.Name)
		}
		seen[ref] = true
		if resource.Kind == manifest.Workflow {
			if resource.Name != module.selected {
				continue
			}
			selected = true
		}
		resources = append(resources, resource)
	}
	if !selected {
		return orchestration.Definition{}, fmt.Errorf("entrypoint %s does not identify a workflow", module.selected)
	}
	definition, err := (manifest.Bundle{Resources: resources}).Compile()
	if err != nil {
		return definition, err
	}
	for i := range definition.Steps {
		step := &definition.Steps[i]
		binding := module.workflows[module.selected][step.Name]
		if binding.HasInputs {
			step.Inputs = binding.Inputs
		}
		if binding.HasOutputs {
			step.Outputs = binding.Outputs
		}
		step.DependsOn = binding.Dependencies
	}
	return definition, definition.Validate()
}
