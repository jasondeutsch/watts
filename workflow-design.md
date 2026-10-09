# First-class workflows

Watts is a 0.1.0 proof of concept. Workflow and configuration schema versions identify data formats independently of the fixed Watts release number.

Temporal is the workflow engine. Each task maps to one Workflow Execution. The official Go SDK supplies durable execution, queries, validated updates, cancellation and activity scheduling. Watts supplies a deterministic sequential interpreter for project-owned definitions and worker activities that access project files and processes.

The source repository keeps base JSON templates in workflow-templates/. A project selects a default template in watts.json; task creation copies its definition to an editable workflow.json. First submission freezes the definition and runtime settings for execution. Stages specify their agents, identities, prompts, artifacts and checks, and run in list order unless explicit feedback routing changes that order. See [workflows.md](workflows.md) for file configuration and executor contracts.

Capabilities return structured outcomes and feedback; definitions own routing. A QA result can route back to coding, pass its findings to the next attempt, and rerun QA. Every attempt remains in history. Human decisions approve specific artifact hashes. Event stages wait for integration events bound to a unique ID and stage attempt. Neither plugins nor event senders choose undeclared destinations.

The root CLI routes commands and renders exit status. Private domain packages under `cli/internal` depend on shared argument and terminal helpers and on application services. `internal/application` coordinates workflow template loading, task artifacts, pinned bindings and worker activities. `internal/orchestration` owns definitions, protocol and deterministic workflow logic. `internal/application` uses the official Temporal SDK directly for connections and queries. Configuration, embedded kit, storage and workspace packages own their corresponding responsibilities. [Dependency checks](internal/application/architecture_test.go) enforce the UI boundary.

Temporal history is authoritative for progress and completion. Local task files hold artifacts, snapshots, executor inputs/results and agent provenance. Attempts come from Temporal. Each activity has its own execution context, and all executable steps return a common result followed by shared completion checks and artifact hashing. Workspace locks serialize mutating activities. Heartbeats propagate cancellation to subprocess groups, and a process journal detects surviving processes after worker loss. File edits are not rolled back automatically. Specification and planning are agent stages within the default SDLC workflow. Human gates review their artifacts and route revision feedback to the drafting stages. Interactive Pi sessions remain available for exploration.

The bundled SDLC explicitly declares specification, specification review, planning, plan review, implementation and independent review. Deterministic checks run before or after the declared agents. Its documents and rules are supplied by the kit. Custom workflows use their own templates and completion requirements. No workflow operation requires Git.

Execution is sequential. Parallel stages, expression conditions and authenticated approval integrations remain future capabilities. Shared-file ownership and cancellation semantics must be defined before parallel scheduling.

Project workflow files are reusable templates. Task creation copies a definition into the visible task directory as workflow.json. Users may customize that file before first submission; the Temporal binding freezes the execution contract when the task is submitted.

The MVP uses file-based configuration: bundled templates are visible in workflow-templates/, watts.json selects the project default, and each task scaffolds its own workflow.json. Agent identities belong to stages. Stage order is list order; workflow management commands are deferred.

See [stage communication](stage-communication.md) for the proposed executor-independent interface and build/review feedback contract.
