package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/jasondeutsch/watts/internal/render"
	"github.com/stretchr/testify/require"
)

const resourceBundle = `kind: Workflow
schema_version: 1
name: example
spec:
  steps:
    - name: build
      use: {kind: Agent, name: builder}
    - name: lint
      use: {kind: Procedure, name: lint}
      on_failure: build
---
kind: Agent
schema_version: 1
name: builder
spec:
  config_ref: common
  identity: team-builder
  prompt: Read SPEC.md and build.
  auth:
    env: MODEL_KEY
    secret: {name: credentials, key: api-key}
---
kind: Procedure
schema_version: 1
name: lint
spec:
  execution: {type: container}
  container: {image: team/tools:0.1.0, command: [go, vet, ./...], result: exit-code}
---
kind: ConfigMap
schema_version: 1
name: common
spec:
  data:
    container: {image: team/agent:0.1.0, command: [agent]}
    provider: openrouter
    model: coding-model
    thinking: medium
---
kind: Secret
schema_version: 1
name: credentials
spec:
  env: {api-key: USER_MODEL_KEY}
`

func TestAllKindsCompileAndRoundTrip(t *testing.T) {
	t.Setenv("USER_MODEL_KEY", "do-not-publish-this-value")
	bundle, err := manifest.ParseResources([]byte(resourceBundle))
	require.NoError(t, err)
	require.Len(t, bundle.Resources, 5)
	definition, err := bundle.Compile()
	require.NoError(t, err)
	require.Equal(t, "build", definition.Steps[0].Agent)
	require.Equal(t, "coding-model", definition.Steps[0].Model)
	require.Equal(t, "medium", definition.Steps[0].AgentSettings.Thinking)
	require.Equal(t, "USER_MODEL_KEY", definition.Steps[0].SecretEnvironment["MODEL_KEY"])
	require.Equal(t, []string{"go", "vet", "./..."}, definition.Steps[1].Container.Command)
	require.Empty(t, definition.Steps[1].Model)
	require.Equal(t, "build", definition.Steps[1].OnFailure)
	encoded, err := manifest.EncodeWorkflow("task-example", definition)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "do-not-publish-this-value")
	require.Contains(t, string(encoded), "kind: Secret")
	reloaded, err := manifest.CompileWorkflow(encoded)
	require.NoError(t, err)
	require.Equal(t, definition, reloaded)
}

func TestResourceValidation(t *testing.T) {
	for _, test := range []struct{ name, source, message string }{
		{"unknown kind", strings.Replace(resourceBundle, "---\nkind: Procedure", "---\nkind: Model", 1), "unsupported resource kind"},
		{"unknown field", strings.Replace(resourceBundle, "identity: team-builder", "identity: team-builder\n  identty: typo", 1), "identty"},
		{"missing ref", strings.Replace(resourceBundle, "name: builder}", "name: unknown}", 1), "references unknown"},
		{"wrong ref kind", strings.Replace(resourceBundle, "kind: Agent, name: builder", "kind: Procedure, name: builder", 1), "references unknown"},
		{"missing config", strings.Replace(resourceBundle, "config_ref: common", "config_ref: unknown", 1), "unknown ConfigMap"},
		{"missing secret", strings.Replace(resourceBundle, "name: credentials, key: api-key", "name: credentials, key: missing", 1), "missing Secret"},
		{"model on procedure", strings.Replace(resourceBundle, "execution: {type: container", "model: unexpected\n  execution: {type: container", 1), "field model"},
		{"missing identity", strings.Replace(resourceBundle, "identity: team-builder", "identity: ''", 1), "requires identity"},
		{"reserved env", strings.Replace(resourceBundle, "env: MODEL_KEY", "env: HOME", 1), "invalid secret target"},
		{"mismatched execution", strings.Replace(resourceBundle, "type: container", "type: approval", 1), "exactly one container"},
		{"unknown routing", strings.Replace(resourceBundle, "on_failure: build", "on_failure: unknown", 1), "unknown transition"},
		{"duplicate resource", resourceBundle + "---\n" + resourceBundle, "duplicate resource"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := manifest.CompileWorkflow([]byte(test.source))
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestTemplatesForEveryKindAndGoGeneralist(t *testing.T) {
	for _, directory := range []string{"resource-templates", "resources/examples/go_generalist"} {
		t.Run(directory, func(t *testing.T) {
			root, err := os.OpenRoot(filepath.Join("..", "..", directory))
			require.NoError(t, err)
			defer root.Close()
			rendered, err := render.Render(root.FS())
			require.NoError(t, err)
			var sources [][]byte
			kinds := map[string]bool{}
			for _, file := range rendered.Files {
				sources = append(sources, file.Data)
			}
			data, err := manifest.JoinDocuments(sources...)
			require.NoError(t, err)
			bundle, err := manifest.ParseResources(data)
			require.NoError(t, err)
			for _, resource := range bundle.Resources {
				kinds[resource.Kind] = true
			}
			require.Len(t, kinds, 5)
			definition, err := bundle.Compile()
			require.NoError(t, err)
			require.NotEmpty(t, definition.Steps)
			override := render.ValuesSource{Name: "team.yaml", Data: []byte("config_maps:\n  agent-defaults:\n    data:\n      model: team/coding-model\n")}
			customized, err := render.Render(root.FS(), override)
			require.NoError(t, err)
			sources = nil
			for _, file := range customized.Files {
				sources = append(sources, file.Data)
			}
			data, err = manifest.JoinDocuments(sources...)
			require.NoError(t, err)
			changed, err := manifest.CompileWorkflow(data)
			require.NoError(t, err)
			for _, step := range changed.Steps {
				if step.Agent != "" {
					require.Equal(t, "team/coding-model", step.Model)
				}
			}
		})
	}
}

func TestContainerConfigurationSurvivesResolutionAndTaskEncoding(t *testing.T) {
	definition, err := manifest.CompileWorkflow([]byte(resourceBundle))
	require.NoError(t, err)
	data, err := manifest.EncodeWorkflow("pinned", definition)
	require.NoError(t, err)
	decoded, err := manifest.LoadWorkflow(data)
	require.NoError(t, err)
	require.Equal(t, definition.Steps[0].Container, decoded.Spec.Steps[0].Container)
	require.NotContains(t, string(data), "runtime_agent")
}

func TestAgentCapabilitiesRoundTrip(t *testing.T) {
	data := []byte(`kind: Workflow
schema_version: 1
name: team
spec:
  steps:
    - name: build
      use: {kind: Agent, name: builder}
---
kind: Agent
schema_version: 1
name: builder
spec:
  identity: builder
  provider: fixture
  model: fixture
  prompt: Implement the task.
  container:
    image: team/image
    command: [launch]
    resources: {team: team}
  tools:
    - name: lint
      command: {argv: [lint], timeout_seconds: 30}
    - name: symbols
      mcp: {ref: gopls}
  mcp_servers:
    gopls: {command: gopls, args: [mcp]}
  gates:
    - name: lint-clean
      tool: lint
      expect: {exit_code: 0}
  gate_policy: {max_repairs: 1}
  skills:
    - name: team-guidance
      source: {resource: team, path: skills/go}
`)
	definition, err := manifest.CompileWorkflow(data)
	require.NoError(t, err)
	require.Len(t, definition.Steps[0].Tools, 2)
	encoded, err := manifest.EncodeWorkflow("team", definition)
	require.NoError(t, err)
	reloaded, err := manifest.LoadWorkflow(encoded)
	require.NoError(t, err)
	require.Equal(t, definition, reloaded.Spec)
	for _, broken := range []string{
		strings.Replace(string(data), "tool: lint", "tool: absent", 1),
		strings.Replace(string(data), "timeout_seconds: 30", "timeout_seconds: -1", 1),
		strings.Replace(string(data), "resource: team, path: skills/go", "resource: team, path: ../outside", 1),
		strings.Replace(string(data), "command: {argv: [lint], timeout_seconds: 30}", "command: {argv: [lint], typo: 1}", 1),
	} {
		_, err := manifest.CompileWorkflow([]byte(broken))
		require.Error(t, err)
	}
}
