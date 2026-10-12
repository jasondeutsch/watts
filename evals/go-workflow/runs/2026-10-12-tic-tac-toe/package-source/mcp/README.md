# Go generalist MCP configuration

The gopls connection is defined in [values.yaml](../values.yaml), under `agents.build.mcp_servers.gopls`. The same Agent selects it with `mcps: [gopls]`. There is no separate MCP configuration fragment to merge into a project descriptor.

```yaml
agents:
  build:
    mcps: [gopls]
    mcp_servers:
      gopls:
        command: gopls
        args: [mcp]
        cwd: .
```

The example team image installs gopls v0.23.0 with MCP support; see the [Go instructions](https://go.dev/gopls/features/mcp). It runs over stdio inside the container from `/workspace`. Teams may override the connection and selection in their own values file. To remove MCP use from the builder, override `agents.build.mcps` with `[]`.

The Pi/Ralph image prepares selected connections in `/runtime/pi/mcp.json`; Ralph children inherit them. This is image behavior, not worker or template logic. The resolved task captures the connection configuration.

Verify an actual gopls tool call in a builder transcript. Interactive registration alone does not establish that workflow execution used the server. This example's model-backed MCP execution has not yet been verified.
