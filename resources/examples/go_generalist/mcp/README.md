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

Install a gopls release with MCP support on the worker PATH; see the [Go instructions](https://go.dev/gopls/features/mcp). It runs over stdio from the project root. Teams may override the connection and selection in their own values file. To remove MCP use from the builder, override `agents.build.mcps` with `[]`.

The current Pi adapter prepares the selected connections for each activity and restores the previous private configuration afterward. This is adapter behavior, not a requirement of the templates. The resolved task captures the connection configuration.

Verify an actual gopls tool call in a builder transcript. Interactive registration alone does not establish that workflow execution used the server. This example's model-backed MCP execution has not yet been verified.
