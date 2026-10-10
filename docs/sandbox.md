# Execution isolation

Watts currently supports Agent `sandbox.mode: local`. This explicitly selects local execution; it does not enforce a container or security sandbox. Unsupported modes fail validation. See [Agent configuration](kinds.md#agent).

## Current boundaries

Agents use a constructed environment, private HOME, and private Pi directories. Default Pi settings disable project trust. Protected paths and task evidence checks provide guardrails, but the process still has the operating-system permissions of the worker. Private directories do not prevent arbitrary file reads or writes.

Commands and capability plugins run locally with the worker's environment. Workspace locks serialize Watts activities, not external editor changes. Credentials can be bound at launch through Secret references; this does not restrict filesystem or network access or prevent an executor from printing them. Cancellation stops subprocess groups without rolling back file edits.

## Proposed isolation

The [stage communication proposal](stage-communication.md) assumes isolated workspaces and explicit publication. Build should receive writable application access; review and QA should receive read-only application access and writable report locations. Credentials, network access, dependencies, tools, and MCP resources must be scoped to each executor.

Possible boundaries include containers, microVMs, and operating-system sandboxes. A project mount alone is not a complete boundary. Any design must address host credentials and runtime sockets, child processes, live developer edits, partial changes, cancellation, and node loss. These choices are tracked in [wiss-017](../issues.md#wiss-017-sandboxed-execution-in-a-developers-local-environment).

External sandbox tooling can be investigated independently, but Watts has no supported sandbox adapter or verified installation recipe yet. Preserve project files, task evidence, runtime state, and access to Temporal when experimenting; reinitialization is not a replacement for retained state.

## Research references

- [Pi security](https://pi.dev/docs/latest/security) and [containerization](https://pi.dev/docs/latest/containerization)
- [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/)
- [Pi sandbox kit](https://github.com/docker/sbx-kits-contrib/tree/main/pi)
- [OpenShell](https://docs.nvidia.com/openshell/about/overview)
- [Gondolin](https://github.com/earendil-works/gondolin)

These links are research inputs, not validated Watts deployment instructions.
