package workflowlang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// Scaffold creates a team-owned importing package, never installs dependencies
// or starts services. Its module is an explicit local reference, not a hidden
// vendoring/update mechanism. All source validation precedes destination writes.
func Scaffold(destination, source string, options Options) (string, error) {
	resolver := moduleResolver{active: map[string]bool{}}
	module, err := resolver.resolveModule(source, "", options)
	if err != nil {
		return "", err
	}
	if _, err := module.compileDefinition(); err != nil {
		return "", err
	}
	reference, err := module.entrypointReference()
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return "", fmt.Errorf("package parent directory must exist: %w", err)
	}
	target = filepath.Join(parent, filepath.Base(target))
	relative, err := filepath.Rel(target, filepath.Dir(module.source))
	if err != nil {
		return "", err
	}
	if relative == "." || !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != ".." {
		return "", fmt.Errorf("destination must not contain the source package")
	}
	if inside, _ := filepath.Rel(filepath.Dir(module.source), target); inside != ".." && !strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("destination must be outside the source package")
	}
	name := strings.ToLower(strings.ReplaceAll(filepath.Base(target), "_", "-"))
	if !validResourceName(name) {
		return "", fmt.Errorf("destination directory must form a package name (letters, digits and dashes)")
	}

	main := hclwrite.NewEmptyFile()
	metadata := main.Body().AppendNewBlock("package", []string{name}).Body()
	metadata.SetAttributeValue("version", cty.StringVal("0.1.0"))
	metadata.SetAttributeValue("var_files", cty.ListVal([]cty.Value{cty.StringVal("values.wvars")}))
	traversal, diagnostics := hclsyntax.ParseTraversalAbs([]byte(reference), "entrypoint", hcl.InitialPos)
	if diagnostics.HasErrors() {
		return "", diagnostics
	}
	metadata.SetAttributeTraversal("entrypoint", traversal)
	main.Body().AppendNewline()
	dependency := main.Body().AppendNewBlock("module", []string{"base"}).Body()
	dependency.SetAttributeValue("source", cty.StringVal(filepath.ToSlash(relative)))
	dependency.SetAttributeValue("version", cty.StringVal(module.version))
	bindings := map[string]hclwrite.Tokens{}
	values := hclwrite.NewEmptyFile()
	for _, name := range sortedKeys(module.inputs) {
		bindings[name] = hclwrite.TokensForTraversal(hcl.Traversal{hcl.TraverseRoot{Name: "var"}, hcl.TraverseAttr{Name: name}})
		values.Body().SetAttributeValue(name, module.inputs[name])
	}
	var attributes []hclwrite.ObjectAttrTokens
	for _, name := range sortedKeys(bindings) {
		attributes = append(attributes, hclwrite.ObjectAttrTokens{Name: hclwrite.TokensForIdentifier(name), Value: bindings[name]})
	}
	dependency.SetAttributeRaw("variables", hclwrite.TokensForObject(attributes))
	main.Body().AppendNewline()
	main.Body().AppendNewBlock("output", []string{"workflow"}).Body().SetAttributeTraversal("value", traversal)
	files := map[string][]byte{
		Filename:        main.Bytes(),
		"variables.acl": hclwrite.Format(module.inputDeclarations),
		"values.wvars":  values.Bytes(),
		"README.md":     []byte(fmt.Sprintf("# %s\n\nThis package imports `%s@%s` from `%s`.\nThe dependency remains at that path; it is not copied or installed. Keep it\navailable for compilation, or copy it into your repo and update module.source.\n\nEdit values.wvars for team defaults. Variables are typed in variables.acl and\npassed explicitly to the base module; object overrides replace entire values.\nCustomize workflow policy in authored ACL, never in captured task definitions.\n\nFrom the application root, with its worker running:\n\n```sh\nwctl exec ./%s --name 'First task' --request-file request.md\n```\n\nUse `wctl exec <task-folder>` to retry a task with its pinned configuration.\nUse `wctl exec <package> --dry-run` to print the complete compiled artifact\nwithout creating files or contacting runtime services. Redirect stdout explicitly\nwhen you need to save an artifact.\nScaffolding installs no images, credentials, toolchains, or services.\n", name, module.name, module.version, filepath.ToSlash(relative), filepath.Base(target))),
	}
	if err := os.Mkdir(target, 0755); err != nil {
		return "", fmt.Errorf("create package (destination must not exist): %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(target)
		}
	}()
	for _, name := range sortedKeys(files) {
		if err := os.WriteFile(filepath.Join(target, name), files[name], 0644); err != nil {
			return "", err
		}
	}
	complete = true
	return target, nil
}

func (module resolvedModule) entrypointReference() (string, error) {
	if module.exports.Type().HasAttribute("workflow") && module.exports.GetAttr("workflow").Type().HasAttribute(module.selected) {
		return "module.base.workflow." + module.selected, nil
	}
	// A composed package can explicitly export its selected workflow.
	for name, value := range module.exports.GetAttr("outputs").AsValueMap() {
		if value.Type().IsObjectType() && value.Type().HasAttribute("kind") && value.Type().HasAttribute("name") && value.GetAttr("kind").RawEquals(cty.StringVal("Workflow")) && value.GetAttr("name").RawEquals(cty.StringVal(module.selected)) {
			return "module.base.outputs." + name, nil
		}
	}
	return "", fmt.Errorf("selected workflow %s is not exported; add an output referencing the entrypoint to the source package", module.selected)
}
