# Container images and workload execution

Status: proposal for [wiss-013](../issues.md#wiss-013-containerized-workloads-across-kinds). Container execution and the illustrative manifest fields below are not implemented. This document clarifies how images, resource manifests, and executor adapters fit together; the [stage communication proposal](stage-communication.md) defines inputs, publication, and routing.

## The execution primitive

An isolated workload is Watts' basic execution primitive, analogous to a Kubernetes Pod. Agent and executable Procedure attempts specialize its lifecycle. Workflow orchestrates them through Temporal; ConfigMap and Secret provide scoped configuration and credential references. Approval and event waits do not require running containers.

A container image supplies installed software. A manifest defines the work and its runtime requirements. The resolved execution request supplies attempt identity, inputs, workspace revision, configuration, and output bindings. An image does not replace the Agent or Procedure definition.

| Component | Responsibility |
| --- | --- |
| OCI image | Executables, libraries, extensions, and installed tools |
| Agent/Procedure manifest | Runtime selection, work configuration, permissions, inputs, and outputs |
| Container backend | Materialize inputs, launch, observe, cancel, collect outputs, and clean up |
| Agent adapter | Translate Agent configuration into runtime-specific files/commands and translate events into results and usage |
| Workflow/Temporal | Durable routing, approval/event waits, decisions, and attempt scheduling |

The container backend and agent adapter are separate responsibilities. The backend need not know Pi's model configuration; the Pi adapter need not implement container scheduling. A local container engine and Kubernetes could implement the same execution boundary later.

## Procedures can use existing images

A command-based Procedure can run an ordinary image containing the required executable. Watts supplies its command, working directory, workspace access, configuration, and declared output locations, then records its exit status and logs.

```yaml
# Illustrative proposal, not current schema or a copyable configuration.
kind: Procedure
name: test
spec:
  runtime:
    container:
      image: golang:<pinned-version-or-digest>
  execution:
    command: [go, test, ./...]
```

No Watts daemon or wrapper must be installed in that image. The command must still obey its declared input/output contract, and successful exit alone does not replace required checks or artifact validation. Native structured outcomes may require an output file or adapter; stdout is diagnostic output, not an implicit routing protocol.

## Agents need suitable runtime images

An Agent's image must contain its selected agent software and required tools. An existing suitable image can be used directly. Otherwise, Watts or the team must build and maintain a custom image.

For the supported Pi/Ralph path, Watts should provide a ready-to-run image containing pinned Pi, its required extensions including Ralph, and their runtime dependencies. Ralph is a tool used by Pi inside an Agent workload, not an orchestration primitive. The Go generalist example should select a suitable image containing its declared Go toolchain, lint tool, gopls MCP executable, and other required dependencies, or extend the supported base image to supply them.

```yaml
# Illustrative proposal; field names and launch contract remain to be designed.
kind: Agent
name: builder
spec:
  runtime:
    adapter: pi
    container:
      image: <registry>/<pi-go-image>@sha256:<digest>
  # Model, instructions, skills, MCPs, and credential references remain
  # typed Agent configuration, supplied from the resolved manifest.
```

The Pi adapter prepares Pi configuration and its launch command inside the attempt environment, consumes runtime events, and records structured results, usage, and evidence. How the adapter communicates across the container boundary remains to be specified. Do not assume arbitrary images support Pi or that every agent follows a universal command protocol.

A derived image is packaging, not necessarily a Watts wrapper. Team images can add tools or use a different agent runtime. A different runtime needs a compatible adapter; an image reference by itself does not supply that integration.

## Launch and publication

1. Resolve and pin the workload definition, adapter requirements, and image identity.
2. Materialize the bound workspace revision and upstream artifacts into the execution environment.
3. Supply nonsecret configuration and inject scoped credentials at launch. Keep credentials out of image layers, packages, logs, and Temporal history.
4. Run the declared command or let the agent adapter configure and invoke its runtime.
5. Observe progress, collect incremental logs and usage, and stop the owned workload on cancellation or timeout.
6. Validate and publish outputs and any candidate workspace revision before returning an accepted result to Temporal.
7. Clean up disposable state according to worker retention policy after durable evidence is recorded.

Application access is explicit: build may modify a candidate workspace; review and QA inspect its identified revision read-only and write reports separately. A shared writable mount is not sufficient isolation. Pi/Ralph private configuration and registries remain in the workload's private environment and must not appear in the application workspace.

## Initial scope

- Consume existing OCI images; pin image digests in resolved execution plans.
- Support command-based Procedures and the supported Pi Agent adapter inside the common container lifecycle.
- Provide a maintained Pi/Ralph runtime image and document the Go generalist image requirements.
- Keep image building outside Watts initially. Teams can use standard image build tools and reference the resulting image.
- Treat image contents as executable code: define filesystem, network, credential, and resource permissions. Containerization alone is not a complete security policy.
- Preserve failed and canceled attempt evidence and usage. Do not equate container restart or Temporal retry with exactly-once side effects.

Remote registries are image distribution infrastructure; this proposal does not require a Watts package registry. Runtime configuration and retention follow the [package/runtime layout proposal](package-runtime-layout.md).

## Decisions before implementation

- Select the initial local container backend and supported platforms.
- Specify the command/adapter launch contract, entrypoint handling, working directories, mounts, user identity, and output collection.
- Define adapter compatibility checks, image architecture requirements, and image release/maintenance ownership.
- Specify network/resource policies, credential delivery, cancellation, and interrupted publication recovery.
- Verify a deterministic Procedure and Pi/Ralph Agent through a complete Go build/review loop, with no tool-private files leaking into the project.

Related work: [executor abstraction](../issues.md#wiss-007-executor-abstraction), [portable execution packages](../issues.md#wiss-016-typed-workflow-resources-and-compiled-execution-packages), [local sandboxing](../issues.md#wiss-017-sandboxed-execution-in-a-developers-local-environment), and [Temporal integration](../issues.md#temporal-integration-completion).
