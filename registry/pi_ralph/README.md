# Pi/Ralph workload image

This image implements the [common workload interface](../../docs/container-workloads.md)
with Pi 1.0.3 and Ralph 2.1.0. Watts core has no Pi adapter or host-install logic.
Teams may extend this image or provide a different image implementing the contract.

```sh
docker build -t watts/pi-ralph:0.1.0 registry/pi_ralph
```

An Agent selects `command: [node, /opt/watts/launcher.mjs]` and `network: bridge`
when using a hosted provider. The launcher reads `/inputs/input.json`, creates
private Pi settings, resolves its declared auth environment, configures selected
MCPs/skills, and runs from `/workspace`. Task documents are copied to
`/runtime/task`; only declared outputs are copied to `/outputs/task`.

Optional `/inputs/config/launcher.json` is image-specific configuration:

```json
{"ralph":true,"verdict":"review/VERDICT.md"}
```

`ralph: true` requires a declared `/inputs/config/ralph.md`, copied into the private
task environment. `verdict` maps a valid final `VERDICT: REJECT` to
`changes-required`; a pass becomes `success`. `model_config` can override the
custom provider model's context window, token limit, reasoning, and optional
explicit prices. Unconfigured cost is reported as unknown.

The image owns exact, version-checked upstream patches that keep Ralph's registry
in `/runtime` and load usage/error telemetry in nested RPC agents. A successful
process must provide assistant-turn evidence; Ralph must also complete its loop.
Provider errors are explicit error events even when Pi exits zero. No project
`.ralph-runner`, Pi settings, or extension installation is needed.

Tests in `protocol.test.mjs` check telemetry mapping. The application image tests
exercise plain Pi, Ralph tool execution, and provider failures with a fake gateway.
The [Go team image](../coder_golang/README.md) adds Go, gopls, and golangci-lint.
Upstream dependencies retain their licenses; Watts-owned source follows the
repository license.

## Declared tools and gates

The launcher loads named skill sources and exposes fixed command tools using an image-owned Pi extension, including nested Ralph agents. MCP tool references select native server configurations. Commands return bounded structured output and retained evidence; timeouts/cancellation terminate their process group.

After the Agent finishes, the launcher runs required gates against the final candidate. Failure returns detailed feedback to a new bounded repair pass in the same container/workspace. Every pass reruns all gates; exhaustion fails without publishing application changes. The final gate report binds checks to candidate hashes/modes and is validated by Watts before publication. `gate_policy.max_repairs` defaults to two. Ralph helper metadata is prepared privately; repair loop history remains under `/runtime/ralph-history`.

Declared command tools/gates use the image's original PATH, excluding Pi's private binary overrides. This does not prohibit all installations or constrain native shell access; stronger workload policy remains separate work. The image implements the generic [Agent capability contract](../../docs/container-workloads.md#agent-capabilities-and-gate-evidence).

## MCP resource adapter

`mcp.mjs` translates resolved `agent.mcp_services` into private Pi MCP settings.
It uses the MCP library bundled with pinned Pi for initialize/discovery probes;
it does not implement the protocol. Exact service/consumer allowlists become
native tool-exposure restrictions, and Secret placeholders keep credential values
out of configuration/evidence. Readiness errors identify connection, authentication,
timeout, missing tools, or platform mismatch before model invocation.

Pi owns session connections and stdio process-group cleanup; the probe is always
closed. Stdio services run inside this image/derived images, with per-attempt
scope. External streamable HTTP endpoints are not launched or stopped by Watts.
See [MCP fields](../../docs/kinds.md#mcp) for scope and isolation limits.

Run adapter unit tests with `node --test registry/pi_ralph/mcp.test.mjs`.
For native integration, mount this directory at `/test`, a small Go module at
`/workspace`, and run `node --test /test/mcp.integration.test.mjs` inside
`watts/coder-golang:0.1.0` with `WATTS_MCP_INTEGRATION=1`. No model calls are made.
