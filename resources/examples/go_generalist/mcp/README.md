# Workflow MCP setup

Install gopls v0.20 or newer, using the [Go instructions](https://go.dev/gopls/features/mcp), and put it on the worker PATH. Pin a team release compatible with your Go toolchain. The interface is experimental.

Merge the `spec.mcps` mapping from [mcp.yaml](mcp.yaml) into the project's watts.yaml. This file is a Watts configuration fragment, not a Pi configuration file. The connection declares the executable and arguments; the build stage in [go-generalist.yaml](../workflows/go-generalist.yaml) selects it with `mcps: [gopls]`. No manual private Pi registration is needed.

Watts writes only the stage-selected servers into the role's Pi mcp.json for the activity and restores the previous file afterward. Ralph child processes inherit that private agent directory. Stages without `mcps` get no configured servers, even if the role's interactive setup has servers. gopls runs locally over stdio from the project root and needs no API key.

Create a new task after adopting the configuration: first submission pins server definitions and stage selections. For another MCP, add a named connection under watts.yaml's `mcps` and select it on the intended agent stages. Remote servers use `url` and optional `headers`; stdio servers use `command`, `args`, optional `env`, and `cwd`. Credential references use Pi's `${VARIABLE}` syntax; pass variable names through `env_passthrough` and export values before starting the worker. Never embed secrets.

Default exposure is `direct`; configure `codemode` or `deferred` only with the corresponding Pi tools enabled. Keep built-in MCP support enabled. Project `.pi/mcp.json` is ignored by Watts' default project trust setting.

To verify, run a small task and inspect the builder iteration transcript for an actual gopls workspace or symbol tool call. Successful interactive registration is not evidence that Ralph used it. This example's model-backed MCP execution has not yet been verified. Use gopls diagnostics alongside the workflow's lint and test gates, not as a replacement.
