package workflowpackage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/render"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, root, name, contents string) {
	t.Helper()
	target := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0755))
	require.NoError(t, os.WriteFile(target, []byte(contents), 0644))
}

func basePackage(t *testing.T, root string) string {
	t.Helper()
	write(t, root, "arbitrary.yaml", "name: checks\nversion: 0.1.0\nvalues: [defaults.yaml]\ntemplates: source\nassets: [tools]\n")
	write(t, root, "defaults.yaml", "message: base\n")
	write(t, root, "source/workflow.yaml.gotmpl", `kind: Workflow
schema_version: 1
name: checks
spec:
  steps:
    - name: check
      use: {kind: Procedure, name: check}
---
kind: Procedure
schema_version: 1
name: check
spec:
  execution:
    type: container
  container:
    image: fixture:0.1.0
    command: [{{ .Values.message | quote }}]
    result: exit-code
`)
	write(t, root, "tools/check.sh", "#!/bin/sh\necho checked\n")
	return filepath.Join(root, "arbitrary.yaml")
}

func TestConsumerValuesAndDescriptorFilenameAreIndependent(t *testing.T) {
	workspace := t.TempDir()
	basePackage(t, filepath.Join(workspace, "base"))
	consumer := filepath.Join(workspace, "consumer")
	write(t, consumer, "watts.golang.yaml", "name: team\nversion: 0.1.0\nextends:\n  path: ../base/arbitrary.yaml\nvalues: [team.yaml]\n")
	write(t, consumer, "team.yaml", "message: team\n")
	pkg, err := Load(filepath.Join(consumer, "watts.golang.yaml"))
	require.NoError(t, err)
	result, err := pkg.Render(render.ValuesSource{Name: "override", Data: []byte("message: invocation\n")})
	require.NoError(t, err)
	definition, err := manifest.CompileWorkflow(result.Data)
	require.NoError(t, err)
	require.Equal(t, []string{"invocation"}, definition.Steps[0].Container.Command)
	require.Equal(t, "team", result.Metadata.Name)
	require.Len(t, result.Metadata.Layers, 2)
	require.NoError(t, os.Rename(filepath.Join(consumer, "watts.golang.yaml"), filepath.Join(consumer, "unrelated-name.yaml")))
	renamed, err := Load(filepath.Join(consumer, "unrelated-name.yaml"))
	require.NoError(t, err)
	renamedResult, err := renamed.Render(render.ValuesSource{Name: "override", Data: []byte("message: invocation\n")})
	require.NoError(t, err)
	require.Equal(t, result.Data, renamedResult.Data)
	require.Equal(t, result.Metadata.Digest, renamedResult.Metadata.Digest)
}

func TestExtensionsUseExplicitResourceReplacement(t *testing.T) {
	workspace := t.TempDir()
	basePackage(t, filepath.Join(workspace, "base"))
	consumer := filepath.Join(workspace, "consumer")
	descriptor := "name: team\nversion: 0.1.0\nextends:\n  path: ../base/arbitrary.yaml\ntemplates: extensions\n"
	write(t, consumer, "watts.yaml", descriptor)
	write(t, consumer, "extensions/workflow.yaml", `kind: Workflow
schema_version: 1
name: checks
spec:
  steps:
    - name: check
      use: {kind: Procedure, name: check}
    - name: audit
      use: {kind: Procedure, name: audit}
---
kind: Procedure
schema_version: 1
name: audit
spec:
  execution:
    type: container
  container: {image: team/tools:0.1.0, command: [go, test, ./...], result: exit-code}
`)
	pkg, err := Load(consumer)
	require.NoError(t, err)
	_, err = pkg.Render()
	require.ErrorContains(t, err, "declare it in replaces")
	write(t, consumer, "watts.yaml", descriptor+"replaces:\n  - kind: Workflow\n    name: checks\n")
	pkg, err = Load(consumer)
	require.NoError(t, err)
	result, err := pkg.Render()
	require.NoError(t, err)
	definition, err := manifest.CompileWorkflow(result.Data)
	require.NoError(t, err)
	require.Len(t, definition.Steps, 2)
	require.Equal(t, []string{"go", "test", "./..."}, definition.Steps[1].Container.Command)
}

func TestCapturedAssetsAndSourceChangesHaveDistinctDigests(t *testing.T) {
	source := t.TempDir()
	filename := basePackage(t, source)
	original, err := Load(filename)
	require.NoError(t, err)
	write(t, source, "tools/check.sh", "changed\n")
	changed, err := Load(filename)
	require.NoError(t, err)
	require.NotEqual(t, original.Layers[0].Digest, changed.Layers[0].Digest)
	project := t.TempDir()
	require.NoError(t, original.Materialize(project))
	installed := filepath.Join(project, original.Layers[0].Root, "tools/check.sh")
	contents, err := os.ReadFile(installed)
	require.NoError(t, err)
	require.Equal(t, "#!/bin/sh\necho checked\n", string(contents))
	require.NoError(t, original.Materialize(project))
	require.NoError(t, os.WriteFile(installed, []byte("tampered"), 0644))
	require.ErrorContains(t, original.Materialize(project), "modified")
}

func TestInheritanceCyclesAndMissingInputsAreErrors(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.yaml", "name: a\nversion: 0.1.0\nextends: {path: b.yaml}\n")
	write(t, root, "b.yaml", "name: b\nversion: 0.1.0\nextends: {path: a.yaml}\n")
	_, err := Load(filepath.Join(root, "a.yaml"))
	require.ErrorContains(t, err, "cycle")
	write(t, root, "a.yaml", "name: a\nversion: 0.1.0\nextends: {path: absent.yaml}\n")
	_, err = Load(filepath.Join(root, "a.yaml"))
	require.Error(t, err)
	write(t, root, "a.yaml", "name: a\nversion: 0.1.0\ntemplates: source\nprovider: leaked\n")
	_, err = Load(filepath.Join(root, "a.yaml"))
	require.ErrorContains(t, err, "provider")
}

func TestPackageSchemaValidatesFinalConsumerValues(t *testing.T) {
	source := t.TempDir()
	filename := basePackage(t, source)
	write(t, source, "values.schema.json", `{"type":"object","properties":{"message":{"enum":["base"]}}}`)
	pkg, err := Load(filename)
	require.NoError(t, err)
	_, err = pkg.Render(render.ValuesSource{Name: "bad", Data: []byte("message: other\n")})
	require.ErrorContains(t, err, "values validation")
}

func TestAssetsAndValuesCannotEscapePackageRoot(t *testing.T) {
	source := t.TempDir()
	filename := basePackage(t, source)
	outside := t.TempDir()
	write(t, outside, "secret", "outside")
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret"), filepath.Join(source, "tools", "link")))
	_, err := Load(filename)
	require.ErrorContains(t, err, "symlinks")
	require.NoError(t, os.Remove(filepath.Join(source, "tools", "link")))
	require.NoError(t, os.Remove(filepath.Join(source, "defaults.yaml")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret"), filepath.Join(source, "defaults.yaml")))
	_, err = Load(filename)
	require.Error(t, err)
}

func TestCoderGolangTemplatesDoNotRequireExampleTools(t *testing.T) {
	pkg, err := Load(filepath.Join("..", "..", "resources", "examples", "coder_golang"))
	require.NoError(t, err)
	// Supply a complete alternative policy rather than merge example defaults.
	values, err := render.MergeValues(render.ValuesSource{Name: "alternative.yaml", Data: []byte(`workflow:
  name: deterministic-only
  spec:
    version: 1
    steps:
      - name: verify
        use: {kind: Procedure, name: verify}
agents: {}
procedures:
  verify:
    execution:
      type: container
    container: {image: team/checker:0.1.0, command: [echo, verified], result: exit-code}
config_maps: {}
secrets: {}
`)})
	require.NoError(t, err)
	rendered, err := render.RenderSources(pkg.Layers[0].Templates, values)
	require.NoError(t, err)
	var sources [][]byte
	for _, file := range rendered.Files {
		sources = append(sources, file.Data)
	}
	data, err := manifest.JoinDocuments(sources...)
	require.NoError(t, err)
	bundle, err := manifest.ParseResources(data)
	require.NoError(t, err)
	require.Len(t, bundle.Resources, 2)
	definition, err := bundle.Compile()
	require.NoError(t, err)
	require.Len(t, definition.Steps, 1)
	require.Equal(t, []string{"echo", "verified"}, definition.Steps[0].Container.Command)
	for _, dependency := range []string{"pi", "ralph", "gopls", "lint", "OPENROUTER"} {
		require.NotContains(t, string(data), dependency)
	}
}

func TestCoderGolangValuesControlExplicitManifestFields(t *testing.T) {
	pkg, err := Load(filepath.Join("..", "..", "resources", "examples", "coder_golang"))
	require.NoError(t, err)
	result, err := pkg.Render(render.ValuesSource{Name: "team.yaml", Data: []byte(`agents:
  build:
    identity: alternate-builder
    prompt: Implement this task without Ralph.
    skills:
      - name: team-guidance
        source: {resource: team, path: skills/go-builder}
    tools:
      - name: team-lint
        command: {argv: [team-lint, ./...], timeout_seconds: 90}
      - name: code-intelligence
        mcp: {ref: symbols}
    gates:
      - name: team-lint-passes
        tool: team-lint
        expect: {exit_code: 0}
    checks:
      - command: [team-check, --team]
mcps:
  symbols:
    connection:
      transport: stdio
      command: {argv: [team-language-server, serve]}
      timeout_seconds: 30
    lifecycle: attempt
    tools: {discovery: all}
config_maps:
  agent-defaults:
    data:
      model: team/model
`)})
	require.NoError(t, err)
	definition, err := manifest.CompileWorkflow(result.Data)
	require.NoError(t, err)
	for _, step := range definition.Steps {
		if step.Name == "build" {
			require.Equal(t, "alternate-builder", step.Identity)
			require.Equal(t, "Implement this task without Ralph.", step.Prompt)
			require.Equal(t, "team/model", step.Model)
			require.Equal(t, "team", step.Skills[0].Source.Resource)
			require.Equal(t, "skills/go-builder", step.Skills[0].Source.Path)
			require.Equal(t, []string{"team-lint", "./..."}, step.Tools[0].Command.Argv)
			require.Equal(t, 90, step.Tools[0].Command.TimeoutSeconds)
			require.Equal(t, "symbols", step.Tools[1].MCP.Ref)
			require.Equal(t, "team-lint", step.Gates[0].Tool)
			require.Equal(t, "team-language-server", step.MCPServices["symbols"].Connection.Command.Argv[0])
			require.Equal(t, []string{"team-check", "--team"}, step.Checks[0].Command)
		}
		require.NotEqual(t, "lint", step.Name)
		if step.Name == "spec-review" {
			require.Equal(t, "specification", step.Transitions["rejected"])
		}
	}
}

func TestTemplatesRejectUnsupportedValuesRatherThanDroppingThem(t *testing.T) {
	for _, directory := range []string{"resource-templates", "resources/examples/coder_golang"} {
		pkg, err := Load(filepath.Join("..", "..", directory))
		require.NoError(t, err)
		for _, values := range []string{
			"agents:\n  builder:\n    identty: typo\n",
			"procedures:\n  lint:\n    execution:\n      commmand: typo\n",
			"workflow:\n  spec:\n    unexpected: true\n",
		} {
			_, err := pkg.Render(render.ValuesSource{Name: "invalid.yaml", Data: []byte(values)})
			require.Error(t, err, directory+": "+values)
		}
	}
}
