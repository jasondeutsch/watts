package manifest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jasondeutsch/watts/internal/manifest"
	"github.com/stretchr/testify/require"
)

const mcpBundle = `kind: Workflow
schema_version: 1
name: example
spec:
  steps:
    - name: build
      use: {kind: Agent, name: builder}
    - name: review
      use: {kind: Agent, name: reviewer}
---
kind: Agent
schema_version: 1
name: builder
spec:
  identity: builder
  provider: test
  model: test
  prompt: Build.
  container: {image: team/agent, command: [agent], network: bridge, workspace: write}
  tools:
    - name: code
      mcp: {ref: code, exposure: deferred, allow_tools: [search]}
    - name: docs
      mcp: {ref: docs, exposure: direct}
---
kind: Agent
schema_version: 1
name: reviewer
spec:
  identity: reviewer
  provider: test
  model: test
  prompt: Review.
  container: {image: team/agent, command: [agent], network: bridge, workspace: read-only}
  tools:
    - name: code
      mcp: {ref: code, exposure: direct, allow_tools: [diagnostics]}
---
kind: MCP
schema_version: 1
name: code
spec:
  config_ref: mcp-defaults
  connection:
    transport: stdio
    command: {argv: [code-server, serve], working_dir: /workspace}
    timeout_seconds: 30
  lifecycle: attempt
  environment: {MODE: analysis}
  secrets:
    API_KEY: {name: credentials, key: code}
  tools: {discovery: allowlist, allow: [search, diagnostics]}
  requirements: {os: [linux], arch: [amd64, arm64], workspace: read-only}
---
kind: MCP
schema_version: 1
name: docs
spec:
  connection: {transport: streamable-http, url: 'https://docs.example/mcp', timeout_seconds: 20}
  lifecycle: external
  tools: {discovery: all}
  headers: {Accept: application/json}
  header_secrets:
    Authorization:
      secret: {name: credentials, key: docs}
      prefix: 'Bearer '
---
kind: ConfigMap
schema_version: 1
name: mcp-defaults
spec:
  data: {description: Project code intelligence}
---
kind: Secret
schema_version: 1
name: credentials
spec:
  env: {code: TEAM_CODE_KEY, docs: TEAM_DOCS_KEY}
`

func TestMCPResolutionReuseSecretsAndRoundTrip(t *testing.T) {
	t.Setenv("TEAM_CODE_KEY", "must-not-be-recorded")
	t.Setenv("TEAM_DOCS_KEY", "private-docs-key")
	definition, err := manifest.CompileWorkflow([]byte(mcpBundle))
	require.NoError(t, err)
	build, review := definition.Steps[0], definition.Steps[1]
	require.Len(t, build.MCPServices, 2)
	require.Len(t, review.MCPServices, 1)
	require.Equal(t, "Project code intelligence", build.MCPServices["code"].Description)
	require.Equal(t, build.MCPServices["code"], review.MCPServices["code"])
	require.NotEqual(t, build.Tools[0].MCP.AllowTools, review.Tools[0].MCP.AllowTools)
	codeAlias := build.MCPServices["code"].SecretEnvironment["API_KEY"]
	docsAlias := build.MCPServices["docs"].SecretHeaders["Authorization"].Environment
	require.NotEqual(t, codeAlias, docsAlias)
	require.Equal(t, "TEAM_CODE_KEY", build.SecretEnvironment[codeAlias])
	require.Equal(t, "TEAM_DOCS_KEY", build.SecretEnvironment[docsAlias])
	require.Equal(t, "Bearer ", build.MCPServices["docs"].SecretHeaders["Authorization"].Prefix)
	encoded, err := manifest.EncodeWorkflow("task", definition)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "kind: MCP")
	require.NotContains(t, string(encoded), "mcp_servers:")
	require.NotContains(t, string(encoded), "must-not-be-recorded")
	require.NotContains(t, string(encoded), "private-docs-key")
	for range 10 {
		again, err := manifest.EncodeWorkflow("task", definition)
		require.NoError(t, err)
		require.Equal(t, encoded, again, "resource encoding must be deterministic")
	}
	decoded, err := manifest.CompileWorkflow(encoded)
	require.NoError(t, err)
	require.Equal(t, definition, decoded)
	data, err := json.Marshal(definition)
	require.NoError(t, err)
	require.NotContains(t, string(data), "private-docs-key")
	var restored = definition
	require.NoError(t, json.Unmarshal(data, &restored))
	require.NoError(t, restored.Validate())
}

func TestMCPInvalidResourcesAndConsumers(t *testing.T) {
	for _, test := range []struct{ name, old, replacement, message string }{
		{"missing MCP", "ref: code", "ref: absent", "unknown MCP"},
		{"broadening policy", "allow_tools: [search]", "allow_tools: [delete]", "disallowed tool"},
		{"empty consumer selection", "allow_tools: [search]", "allow_tools: []", "allow_tools"},
		{"duplicate tool selection", "mcp: {ref: docs, exposure: direct}", "mcp: {ref: code}", "more than once"},
		{"missing secret", "key: code}", "key: absent}", "missing Secret"},
		{"mixed transport", "transport: stdio\n", "transport: stdio\n    url: https://example/mcp\n", "no URL"},
		{"unsupported shared lifecycle", "lifecycle: attempt", "lifecycle: execution", "lifecycle: attempt"},
		{"unknown field", "lifecycle: attempt", "lifecycle: attempt\n  lifecyle: typo", "lifecyle"},
		{"missing list", "allow: [search, diagnostics]", "allow: []", "tools.allow"},
		{"duplicate allow", "allow: [search, diagnostics]", "allow: [search, search]", "duplicate MCP tool"},
		{"unsafe cwd", "working_dir: /workspace", "working_dir: /etc", "working_dir"},
		{"remote network", "network: bridge", "network: none", "network: bridge"},
		{"write requirement", "arch: [amd64, arm64], workspace: read-only}", "arch: [amd64, arm64], workspace: write}", "writable workspace"},
		{"shell interpolation", "MODE: analysis", "MODE: '!echo secret'", "Secret bindings"},
		{"raw credential URL", "https://docs.example/mcp", "https://user:password@docs.example/mcp", "without credentials"},
		{"invalid timeout", "timeout_seconds: 30", "timeout_seconds: 0", "timeout_seconds"},
		{"inline server removed", "identity: builder", "identity: builder\n  mcp_servers: {}", "mcp_servers"},
		{"MCP is not a workflow executor", "use: {kind: Agent, name: builder}", "use: {kind: MCP, name: code}", "must use Agent or Procedure"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := manifest.CompileWorkflow([]byte(strings.Replace(mcpBundle, test.old, test.replacement, 1)))
			require.ErrorContains(t, err, test.message)
		})
	}
}
