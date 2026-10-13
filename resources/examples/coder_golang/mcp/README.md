# Golang coder MCP configuration

The gopls connection is defined in [values.yaml](../values.yaml), under `agents.build.mcp_servers.gopls`. The same Agent selects it through `tools: [{name: code-intelligence, mcp: {ref: gopls}}]`. There is no separate MCP configuration fragment to merge into a project descriptor.

```yaml
agents:
  build:
    tools:
      - name: code-intelligence
        mcp: {ref: gopls}
    mcp_servers:
      gopls:
        command: gopls
        args: [mcp]
        cwd: .
```

This is an excerpt, not a consumer override replacing the entire tool list. A full list override must retain the command tools referenced by the Agent's gates or change those gates too.

The example team image installs gopls v0.23.0 with MCP support; see the [Go instructions](https://go.dev/gopls/features/mcp). It runs over stdio inside the container from `/workspace`. Teams may override the connection and selection in their own values file. To remove MCP use from the builder, replace `agents.build.tools` with the desired command tools, omitting the MCP entry. List overrides replace the entire list.

The Pi/Ralph image prepares selected connections in `/runtime/pi/mcp.json`; Ralph children inherit them. This is image behavior, not worker or template logic. The resolved task captures the connection configuration.

Verify an actual gopls tool call in a builder transcript. Interactive registration alone does not establish that workflow execution used the server. This example's model-backed MCP execution has not yet been verified.
