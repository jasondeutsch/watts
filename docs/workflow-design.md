# Workflow architecture

Watts separates authoring, durable orchestration, generic execution, and image-owned
agent behavior. The CLI and web UI call the application layer; neither owns a runner.

1. ACL packages compose typed variables, local modules, resources, and captured assets.
2. `wctl compile` resolves references/defaults into a frozen JSON artifact with a graph and workload contracts.
3. `wctl exec` consumes that artifact without rereading source packages.
4. Submission freezes image identities, resources, and worker preferences.
5. Temporal schedules activities and retains routes, waits, attempts, and outcomes.
6. The worker captures an isolated candidate and launches its image with the common interface.
7. Results, checks, and conflict detection control publication; durable receipts prevent duplicate execution.

| Package | Responsibility |
| --- | --- |
| `cli`, `web` | User interfaces |
| `internal/workflowlang` | ACL expressions, modules, artifact connections, captured assets, and compilation |
| `internal/manifest` | Shared typed resource contract validation and resolution |
| `internal/config` | Local operational preferences |
| `internal/orchestration` | Serializable graph/result protocol and deterministic Temporal interpreter |
| `internal/application` | Task lifecycle, submission, activities, evidence, and publication |
| `internal/workload` | Runtime-neutral container specification, file publication, and events |
| `internal/containerengine` | Official Docker SDK lifecycle and reattachment |
| `internal/storage`, `internal/workspace` | Persistence, artifact paths, and workspace locking |
| `registry/` | Image source and runtime-specific launchers/tool integration |

The compiler and orchestration data types do not import the Docker SDK. Core
execution does not import Pi/Ralph code, install software, or interpret their
private state. An Agent image translates descriptive configuration into its own
runtime. A deterministic Procedure can run an ordinary OCI image without a wrapper.

A workload is the execution primitive, analogous to a Pod. Agent and executable
Procedure attempts specialize it. ConfigMap/Secret supply inputs; Workflow and
approval/event waits remain orchestration resources. Workloads require a container
execution environment; currently only the local Docker Engine backend is supported.
Podman would require compatibility testing or an adapter. Kubernetes would require
a scheduling and artifact-delivery backend while Temporal continues to manage
durable workflow state.

The current Docker backend requires a shared local filesystem. Immutable artifact
transport, distinct-node execution, stronger identities, network controls, and
full retention remain future work. The [container interface](container-workloads.md)
describes current behavior; [stage communication](stage-communication.md) and
[package/runtime layout](package-runtime-layout.md) describe the broader proposal.

Architecture tests protect the UI dependency boundary. Testify-based tests cover
compilation, routing, CLI/package lifecycle, workload safety, ordinary images,
Pi/Ralph against a fake provider, and opt-in persistent Temporal lifecycle tests.
[Workflow evals](../evals/go-workflow/README.md) separately compare configured
process outcomes over time.
