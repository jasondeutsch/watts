# Watts documentation

| Guide | Use it for |
| --- | --- |
| [Runbook](runbook.md) | Install, configure a project, run tasks, approve, recover, and read completion reports |
| [Task reports](task-reports.md) | Dashboard metrics, charts, output/token ratios, measurement coverage, and team-defined complexity |
| [External coordinators](coordinators.md) | Drive tasks through an agent, report progress, and record delegated decisions |
| [Workflow execution](workflows.md) | Routing, results, events, and MCP selection |
| [ACL language specification](language-spec.md) | Syntax, declarations, typed contracts, modules, routing, and compiled artifacts |
| [Temporal](temporal.md) | Server operations, worker responsibilities, and stored state |
| [Architecture](workflow-design.md) | Authoring-to-execution flow and package responsibilities |
| [Stage communication proposal](stage-communication.md) | Future portable packages, isolated workspaces, and publication |
| [Container workloads](container-workloads.md) | Container interface, supported and proposed backends, publication, and image responsibilities |
| [Package/runtime storage](package-runtime-layout.md) | Current package/task/worker layout and retention |
| [Secret protection](secret-protection.md) | Credential redaction, image responsibilities, publication, exports, and incident cleanup |
| [Sandboxing](sandbox.md) | Current security boundaries and isolation research |

Start with the [repository quick start](../readme.md). For a team setup, use the [Golang coder example](../resources/examples/coder_golang/README.md); for an external coordinating agent, use the [resources toolbox](../resources/readme.md). Go workflow eval cases, rubrics, and run evidence live in the separate `evals`
project under `go-workflow/`. Proposed features are tracked in [issues](../issues.md).
