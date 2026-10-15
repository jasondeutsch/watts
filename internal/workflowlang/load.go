package workflowlang

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	"gopkg.in/yaml.v3"
)

type moduleResolver struct{ active map[string]bool }
type resolvedModule struct {
	name, version, source, digest, selected string
	paths                                   []string
	dependencies                            []Dependency
	assets                                  []Asset
	resources                               []manifest.Resource
	workflows                               map[string]workflowBindings
	exports                                 cty.Value
	inputs                                  map[string]cty.Value
	inputDeclarations                       []byte
}

var resourceKinds = map[string]string{"agent": manifest.Agent, "procedure": manifest.Procedure, "mcp": manifest.MCP, "configmap": manifest.ConfigMap, "secret": manifest.Secret, "workflow": manifest.Workflow}
var resourceSchemas = map[string]reflect.Type{
	"agent": reflect.TypeFor[manifest.AgentSpec](), "procedure": reflect.TypeFor[manifest.ProcedureSpec](),
	"mcp": reflect.TypeFor[manifest.MCPSpec](), "configmap": reflect.TypeFor[manifest.ConfigMapSpec](), "secret": reflect.TypeFor[manifest.SecretSpec](),
}

func (resolver *moduleResolver) resolveModule(source, prefix string, options Options) (resolvedModule, error) {
	result := resolvedModule{workflows: map[string]workflowBindings{}}
	source, err := sourcePath(source)
	if err != nil {
		return result, err
	}
	if resolver.active[source] {
		return result, fmt.Errorf("module dependency cycle at %s", source)
	}
	if len(resolver.active) >= 32 {
		return result, fmt.Errorf("module nesting exceeds 32 levels")
	}
	resolver.active[source] = true
	defer delete(resolver.active, source)
	result.source = source
	root, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return result, err
	}
	defer root.Close()
	captured := map[string][]byte{}
	read := func(name string) ([]byte, error) {
		if data, ok := captured[name]; ok {
			return data, nil
		}
		data, err := root.ReadFile(name)
		if err != nil {
			return nil, err
		}
		if len(data) > 16<<20 {
			return nil, fmt.Errorf("source %s exceeds 16 MiB", name)
		}
		captured[name] = data
		return data, nil
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return result, err
	}
	body := &hclsyntax.Body{Attributes: hclsyntax.Attributes{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".acl") {
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return result, fmt.Errorf("source symlink unsupported: %s", entry.Name())
		}
		data, err := read(entry.Name())
		if err != nil {
			return result, err
		}
		file, diagnostics := hclsyntax.ParseConfig(data, filepath.Join(filepath.Dir(source), entry.Name()), hcl.InitialPos)
		if diagnostics.HasErrors() {
			return result, diagnostics
		}
		parsed := file.Body.(*hclsyntax.Body)
		if len(parsed.Attributes) > 0 {
			return result, fmt.Errorf("%s: top-level arguments belong inside blocks", parsed.Range())
		}
		body.Blocks = append(body.Blocks, parsed.Blocks...)
	}
	groups := map[string][]*hclsyntax.Block{}
	declarations := map[string]map[string]cty.Value{}
	var metadata *hclsyntax.Block
	seen := map[string]bool{}
	for _, block := range body.Blocks {
		if block.Type == "locals" {
			if len(block.Labels) > 0 {
				return result, fmt.Errorf("locals does not accept labels")
			}
			groups[block.Type] = append(groups[block.Type], block)
			continue
		}
		if block.Type != "package" && block.Type != "variable" && block.Type != "module" && block.Type != "output" && resourceKinds[block.Type] == "" {
			return result, fmt.Errorf("%s: unsupported declaration %q", block.TypeRange, block.Type)
		}
		if len(block.Labels) != 1 {
			return result, fmt.Errorf("%s: %s requires one name label", block.TypeRange, block.Type)
		}
		name := block.Labels[0]
		if (resourceKinds[block.Type] != "" || block.Type == "package" || block.Type == "module") && !validResourceName(name) || !hclsyntax.ValidIdentifier(name) {
			return result, fmt.Errorf("%s: invalid name %q", block.TypeRange, name)
		}
		id := block.Type + "." + name
		if seen[id] {
			return result, fmt.Errorf("%s: duplicate %s", block.TypeRange, id)
		}
		seen[id] = true
		groups[block.Type] = append(groups[block.Type], block)
		if block.Type == "package" {
			if metadata != nil {
				return result, fmt.Errorf("one package declaration is required")
			}
			metadata = block
			continue
		}
		if kind := resourceKinds[block.Type]; kind != "" {
			if declarations[block.Type] == nil {
				declarations[block.Type] = map[string]cty.Value{}
			}
			declarations[block.Type][name] = reference(kind, prefix+name)
		}
	}
	if metadata == nil {
		return result, fmt.Errorf("%s: package declaration is required", source)
	}
	result.name = metadata.Labels[0]
	if err := checkArguments(metadata.Body, "version", "description", "assets", "var_files", "entrypoint"); err != nil {
		return result, err
	}
	if len(metadata.Body.Blocks) > 0 {
		return result, fmt.Errorf("package does not accept nested blocks")
	}
	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}, Functions: functions(read)}
	for kind, values := range declarations {
		ctx.Variables[kind] = object(values)
	}
	version, err := stringArgument(metadata.Body, "version", ctx, true)
	if err != nil {
		return result, err
	}
	result.version = version
	if _, err := stringArgument(metadata.Body, "description", ctx, false); err != nil {
		return result, err
	}
	varFiles, err := stringListArgument(metadata.Body, "var_files", ctx)
	if err != nil {
		return result, err
	}
	overrides := map[string]cty.Value{}
	for _, name := range varFiles {
		if !fs.ValidPath(name) {
			return result, fmt.Errorf("var_files path must be inside the package")
		}
		data, err := read(name)
		if err != nil {
			return result, err
		}
		if err := decodeVariables(data, filepath.Join(filepath.Dir(source), name), overrides); err != nil {
			return result, err
		}
	}
	for _, name := range options.VariableFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			return result, err
		}
		if err := decodeVariables(data, name, overrides); err != nil {
			return result, err
		}
		absolute, _ := filepath.Abs(name)
		result.paths = append(result.paths, absolute)
		captured["override:"+absolute] = data
	}
	for name, value := range options.Variables {
		overrides[name] = value
	}
	variables, err := resolveVariables(groups["variable"], overrides)
	if err != nil {
		return result, err
	}
	result.inputs = variables
	for _, block := range groups["variable"] {
		data := captured[filepath.Base(block.TypeRange.Filename)]
		result.inputDeclarations = append(result.inputDeclarations, block.Range().SliceBytes(data)...)
		result.inputDeclarations = append(result.inputDeclarations, '\n', '\n')
	}
	ctx.Variables["var"] = object(variables)
	locals, err := resolveLocals(groups["locals"], ctx)
	if err != nil {
		return result, err
	}
	ctx.Variables["local"] = object(locals)
	assetPaths, err := stringListArgument(metadata.Body, "assets", ctx)
	if err != nil {
		return result, err
	}
	assets, err := collectAssets(root, assetPaths)
	if err != nil {
		return result, err
	}
	// The private asset root is determined before resource expressions use it.
	encodedVariables, err := ctyjson.Marshal(ctx.Variables["var"], ctx.Variables["var"].Type())
	if err != nil {
		return result, err
	}
	fingerprint := append([]byte{}, encodedVariables...)
	for _, name := range sortedKeys(captured) {
		data, _ := json.Marshal(struct {
			Name string
			Data []byte
		}{name, captured[name]})
		fingerprint = append(fingerprint, data...)
	}
	for _, asset := range assets {
		data, _ := json.Marshal(struct {
			Name string
			Data []byte
			Mode fs.FileMode
		}{asset.Path, asset.Data, asset.Mode})
		fingerprint = append(fingerprint, data...)
	}
	assetRoot := filepath.ToSlash(filepath.Join("packages", result.name, digest(fingerprint)))
	for i := range assets {
		assets[i].Root = assetRoot
	}
	result.assets = assets
	ctx.Variables["package"] = object(map[string]cty.Value{"name": cty.StringVal(result.name), "version": cty.StringVal(result.version), "root": cty.StringVal(assetRoot)})
	if err := resolver.resolveModules(groups["module"], source, prefix, ctx, &result); err != nil {
		return result, err
	}
	// Secret references expose source bindings, never environment values.
	for _, block := range groups["secret"] {
		spec, err := bodyFields(block.Body, resourceSchemas["secret"], ctx)
		if err != nil {
			return result, err
		}
		keys := map[string]cty.Value{}
		env, ok := spec["env"].(map[string]any)
		if !ok {
			return result, fmt.Errorf("Secret %s needs an env object", block.Labels[0])
		}
		for key := range env {
			keys[key] = object(map[string]cty.Value{"name": cty.StringVal(prefix + block.Labels[0]), "key": cty.StringVal(key)})
		}
		declarations["secret"][block.Labels[0]] = object(map[string]cty.Value{"kind": cty.StringVal(manifest.Secret), "name": cty.StringVal(prefix + block.Labels[0]), "keys": object(keys)})
	}
	if declarations["secret"] != nil {
		ctx.Variables["secret"] = object(declarations["secret"])
	}
	for _, block := range body.Blocks {
		schema, ok := resourceSchemas[block.Type]
		if !ok {
			continue
		}
		spec, err := bodyFields(block.Body, schema, ctx)
		if err != nil {
			return result, err
		}
		resource, err := makeResource(resourceKinds[block.Type], prefix+block.Labels[0], spec)
		if err != nil {
			return result, err
		}
		result.resources = append(result.resources, resource)
	}
	for _, block := range groups["workflow"] {
		resource, bindings, err := compileWorkflow(block, prefix, ctx)
		if err != nil {
			return result, err
		}
		result.resources = append(result.resources, resource)
		result.workflows[resource.Name] = bindings
	}
	selected, err := stringReferenceArgument(metadata.Body, "entrypoint", ctx)
	if err != nil {
		return result, err
	}
	if prefix == "" && options.Workflow != "" {
		result.selected = options.Workflow
	} else if selected != "" {
		result.selected = selected
	} else if len(groups["workflow"]) == 1 {
		result.selected = prefix + groups["workflow"][0].Labels[0]
	} else if prefix == "" {
		return result, fmt.Errorf("package requires one workflow or an explicit entrypoint reference")
	}
	exports := map[string]cty.Value{}
	for kind, values := range declarations {
		exports[kind] = object(values)
	}
	outputs := map[string]cty.Value{}
	for _, block := range groups["output"] {
		if err := checkArguments(block.Body, "value"); err != nil {
			return result, err
		}
		attribute := block.Body.Attributes["value"]
		if attribute == nil || len(block.Body.Blocks) > 0 {
			return result, fmt.Errorf("output requires a value argument")
		}
		value, err := evaluate(attribute.Expr, ctx)
		if err != nil {
			return result, err
		}
		outputs[block.Labels[0]] = value
	}
	exports["outputs"] = object(outputs)
	result.exports = object(exports)
	for _, name := range sortedKeys(captured) {
		if !strings.HasPrefix(name, "override:") {
			result.paths = append(result.paths, filepath.Join(filepath.Dir(source), name))
		}
		data, _ := json.Marshal(struct {
			Name string
			Data []byte
		}{name, captured[name]})
		fingerprint = append(fingerprint, data...)
	}
	for _, assetPath := range assetPaths {
		result.paths = append(result.paths, filepath.Join(filepath.Dir(source), assetPath))
	}
	data, _ := json.Marshal(result.dependencies)
	fingerprint = append(fingerprint, data...)
	result.digest = digest(fingerprint)
	return result, nil
}
func validResourceName(name string) bool {
	return name != "" && name != "end" && !strings.ContainsAny(name, "_ ./:\\") && strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789-") == "" && name[0] >= 'a' && name[0] <= 'z'
}
func makeResource(kind, name string, spec map[string]any) (manifest.Resource, error) {
	var node yaml.Node
	err := node.Encode(spec)
	return manifest.Resource{Kind: kind, Name: name, SchemaVersion: manifest.SchemaVersion, Spec: node}, err
}
func checkArguments(body *hclsyntax.Body, names ...string) error {
	allowed := map[string]bool{}
	for _, name := range names {
		allowed[name] = true
	}
	for _, name := range sortedKeys(body.Attributes) {
		if !allowed[name] {
			return fmt.Errorf("%s: unsupported argument %q", body.Attributes[name].Range(), name)
		}
	}
	return nil
}
func stringArgument(body *hclsyntax.Body, name string, ctx *hcl.EvalContext, required bool) (string, error) {
	attribute := body.Attributes[name]
	if attribute == nil {
		if required {
			return "", fmt.Errorf("%s: %s is required", body.Range(), name)
		}
		return "", nil
	}
	value, err := evaluate(attribute.Expr, ctx)
	if err != nil {
		return "", err
	}
	if value.Type() != cty.String {
		return "", fmt.Errorf("%s: %s must be a string", attribute.Range(), name)
	}
	return value.AsString(), nil
}
func stringListArgument(body *hclsyntax.Body, name string, ctx *hcl.EvalContext) ([]string, error) {
	attribute := body.Attributes[name]
	if attribute == nil {
		return nil, nil
	}
	value, err := evaluate(attribute.Expr, ctx)
	if err != nil {
		return nil, err
	}
	if !value.Type().IsListType() && !value.Type().IsTupleType() {
		return nil, fmt.Errorf("%s: %s must be a list of strings", attribute.Range(), name)
	}
	var result []string
	for iterator := value.ElementIterator(); iterator.Next(); {
		_, item := iterator.Element()
		if item.IsNull() || item.Type() != cty.String {
			return nil, fmt.Errorf("%s: %s must be a list of strings", attribute.Range(), name)
		}
		result = append(result, item.AsString())
	}
	return result, nil
}
func stringReferenceArgument(body *hclsyntax.Body, name string, ctx *hcl.EvalContext) (string, error) {
	attribute := body.Attributes[name]
	if attribute == nil {
		return "", nil
	}
	value, err := evaluate(attribute.Expr, ctx)
	if err != nil {
		return "", err
	}
	decoded, err := decodeValue(value)
	if err != nil {
		return "", err
	}
	kind, ref, err := referenceValue(decoded)
	if err != nil || kind != manifest.Workflow {
		return "", fmt.Errorf("%s: %s must reference a Workflow", attribute.Range(), name)
	}
	return ref, nil
}
