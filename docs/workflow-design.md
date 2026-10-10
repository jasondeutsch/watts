# Workflow architecture

Watts is a 0.1.0 proof of concept. Schema versions identify formats independently of the application release number. No operation requires Git.

## Authoring to execution

1. Users author [Workflow, Procedure, Agent, ConfigMap, and Secret resources](kinds.md), optionally through [templates](templates.md). Project settings select a default bundle and configure infrastructure, local runtime state, MCP connections, and plugins.
2. Task creation resolves references and defaults, scaffolds artifacts, and writes an editable task bundle. First submission pins the resolved definition and runtime configuration as JSON with integrity hashes.
3. A common deterministic Temporal interpreter executes the ordered graph, routes outcomes, waits for approvals/events, and records attempts. It does not generate Temporal code for each workflow.
4. Worker activities read pinned settings, launch local executors, validate results and artifacts, and record evidence. Secret source names are pinned; values are resolved at launch.

An Agent is a specialized unit of work with model/runtime fields. A Procedure handles model-free work. A Workflow step places either resource and owns routing; resource definitions are distinct from running attempts. Project defaults and task customization are described in the [workflow guide](workflows.md).

Temporal history is authoritative for progress and completion. Task files hold artifacts, snapshots, executor requests/results, logs, and provenance. Approvals bind to artifact hashes. Workspace locks serialize mutating activities, and heartbeats propagate cancellation. Local execution uses a shared project workspace and does not roll back edits.

## Package responsibilities

| Package | Responsibility |
| --- | --- |
| `cmd/cli`, `cli`, `cli/internal` | Entry point, arguments, terminal presentation, and application calls |
| `web` | Task monitor and controls over application services |
| `internal/render` | Generic template/values processing, independent of Watts resource semantics |
| `internal/manifest` | Typed YAML decoding, bundle resolution, and authoring serialization |
| `internal/config` | Project settings, defaults, runtime role resolution, and validation |
| `internal/application` | Task lifecycle, pinned bindings, executor activities, and Temporal SDK integration |
| `internal/orchestration` | Runtime definition/result protocol, graph validation, and deterministic interpreter |
| `internal/kit` | Embedded checks, templates, skills, and asset setup |
| `internal/storage`, `internal/workspace` | Persistence, artifacts, workspace locking, and process lifecycle |

[Architecture tests](../internal/application/architecture_test.go) enforce the CLI dependency boundary. Tests live beside implementations; [evals](../evals/go-workflow/README.md) compare a configured workflow's outcomes over time.

## Proposed next boundary

Resource resolution exists today; self-contained execution packages, immutable cross-node artifact delivery, explicit upstream bindings, isolated workspaces, and containers do not. Those contracts are described in the [stage communication proposal](stage-communication.md). Parallel scheduling and expression conditions also remain future work.
