# Golang coder MCP service

[values.yaml](../values.yaml) defines `mcps.gopls`, rendered as **Kind: MCP** by
[50-mcps.yaml.gotmpl](../templates/50-mcps.yaml.gotmpl). The build Agent selects
that resource through `tools[].mcp.ref: gopls`; connection details are not
embedded in the Agent.

```yaml
mcps:
  gopls:
    description: Go workspace navigation, symbols, references, and diagnostics
    connection:
      transport: stdio
      command:
        argv: [gopls, mcp]
        working_dir: /workspace
      timeout_seconds: 60
    lifecycle: attempt
    tools: {discovery: all}
    requirements:
      os: [linux]
      arch: [amd64, arm64]
      workspace: read-only
agents:
  build:
    tools:
      - name: code-intelligence
        mcp: {ref: gopls, exposure: deferred}
```

This is an excerpt, not an override replacing the entire builder tool list.
List overrides must retain the command tools referenced by its required gates.
Another Agent can select the same resource with its own exposure and narrower
`allow_tools` list. An MCP service allowlist sets the maximum exposed tool set;
consumer selections cannot widen it.

The [team image](../../../../registry/coder_golang/Dockerfile) supplies gopls.
The MCP server runs in the Agent's candidate `/workspace`, inside Linux even
when Docker is hosted on macOS. No host installation or project `.pi/mcp.json`
is needed. The adapter uses Pi's existing MCP library, checks readiness and
selected tool availability before the model starts, and closes attempt-local
sessions. The current backend does not launch a separate gopls container or
share its process across workflow stages.

Change connection/policy values before creating a task; existing task definitions
remain pinned. For transports, Secret bindings, restrictions, and runtime limits,
see [MCP manifest fields](../../../../docs/kinds.md#mcp). Real gopls tool calls,
remote HTTP auth, allowlist exposure, timeout, cancellation, and process cleanup
have deterministic integration coverage without provider/model calls. Actual
model choices about when to use gopls remain subject to workflow evaluation.
