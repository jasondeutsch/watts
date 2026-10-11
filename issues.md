# Watts issues and proposed improvements

Watts remains an MVP at version 0.1.0. This list distinguishes observed problems from proposed capabilities and unresolved design decisions. A code change does not establish that the full task lifecycle works; validate fixes through an end-to-end run.

## Observed issues

### wiss-001: Build execution and completion evidence

A build attempt passed its preflight checks but completed no stories:

```text
Stage: build · attempt 2
Completion check: check-stories
Done: none
Remaining: US-001 US-002 US-003 US-004 US-005
Agent evidence: no Pi session record found
```

Two distinct problems were identified:

- Watts appended prose to the `/ralph --path ...` command, changing its arguments and preventing meaningful execution.
- Ralph's nested Pi processes use `--no-session`. Requiring a normal Pi session falsely rejected a later run that completed the stories and quality gates.

Code changes now preserve extension command arguments and validate Ralph's current run token and successful terminal event. Confirm that a complete SDLC run proceeds through build and review without false failures. Missing evidence, stale evidence, and genuine provider failures must remain failures.

### wiss-002: Application code created inside the task directory

The test implementation was created under `tasks/<task>/` rather than in the project rooted at `watts.yaml`.

Application source, tests, dependencies, and build files belong in the project root or its normal source directories. Task directories hold specifications, plans, workflow definitions, review documents, and execution evidence. Planning must use project-root quality gates rather than direct application commands into a task directory.

The planning skill, build instructions, and scaffolded plan now state this separation. Verify it with a future task. The completed test task does not need migration or further work.

### wiss-003: Web UI status and execution output

Status appeared incorrect or incomplete during testing. Identify whether each discrepancy comes from stale UI assets, unavailable Temporal state, or missing executor output.

The UI should clearly show the current stage, attempt, latest error, and available next action. Ralph writes nested events and transcripts that the normal stage-output view does not expose; this made an active build appear stuck. Integrate that output without treating an idle log as proof that execution has stalled.

### wiss-004: Coordinator cannot complete the full lifecycle

It has not been possible to drive the entire task lifecycle from a coordinator agent. Test creation, submission, status polling, approval, rejection, retry, and completion reporting as one scenario.

Clarify which actions the coordinator may perform and how it presents human decisions. Watts currently rejects approval commands inside its agent environment. Moving the same command to another terminal is not a substitute for human authorization. Starting an executor should be explicit so callers understand when work is delegated to another agent.

### wiss-005: Duplicated kit directory name

Simplify `internal/kit/kit/`. The outer directory contains the Go package; the inner directory holds embedded scripts, templates, and skills. Use a clearer asset directory name and update embedding and resource paths together.

## Proposed improvements

### wiss-006: Shared logging across projects

Provide a global log location outside project directories for all Watts workers. Include project, task, stage, and attempt identifiers so concurrent runs can be distinguished. Define retention and avoid logging credentials.

### wiss-007: Executor abstraction

Remove tight coupling to Pi and Ralph so engineers can use their preferred tools. Pi with Ralph can remain the default implementation. Keep execution results, cancellation, failure reporting, and evidence requirements consistent across executors.

### wiss-008: Global web UI

Allow one UI to display multiple projects and workers. Decouple the web server's lifetime from an individual project worker. Project roots, task queues, and configuration must remain distinct when the UI aggregates them.

### wiss-020: Manifest-driven runtime preparation; remove legacy initialization

Project setup still reflects Watts' origin as a Pi/Ralph wrapper. `watts init` mixes local runtime preparation with workflow package scaffolding, and `watts install` assumes Pi and Ralph are project-wide dependencies. Agents, models, tools, and execution settings now belong to manifests; setup must follow the selected resources rather than impose one agent stack on every project.

Make `watts template new` the package scaffolding command and remove `watts init`. Create private Watts state automatically when an operation needs it. Projects may author a package or extend an existing package without first generating a default workflow or installing agent tools. Keep the project workspace distinct from the package source directory, including when the package is external.

Move runtime-specific configuration, dependency checks, and preparation behind execution adapters selected by resolved Agent or Procedure manifests. Pi/Ralph remains a supported adapter rather than the core execution model. Remove the unconditional project-wide Pi/Ralph installation command; define how an adapter obtains its dependencies before removing the only working setup path. Templates describe dependencies but do not install or supply executables by themselves.

Acceptance criteria:

- A project can create and run a task from its selected package without `watts init`; required private directories are created automatically without overwriting authoring files.
- A workflow containing only deterministic Procedures runs without Pi, Ralph, Node.js, model credentials, or unrelated agent dependencies.
- A Pi/Ralph Agent prepares and validates only its own runtime requirements. Shared orchestration code does not generate Pi configuration or interpret Ralph-specific evidence.
- Missing runtimes, tools, or credential sources produce actionable errors identifying the affected resource and dependency. Setup failures appear in CLI results and the web UI; they do not silently select another runtime.
- Dependency installation policy is explicit: distinguish validation, preparing adapter-owned files, and downloading or installing software. Do not introduce an unconditional installation step in `watts start`.
- Update commands, documentation, examples, and coordinator guidance together. Test a fresh project, an external package, a Procedure-only workflow, and the supported Pi/Ralph execution path.

Coordinate with wiss-007 for adapter interfaces and wiss-016 for portable packages and pinned runtime dependencies. Containerized execution remains a separate follow-on; removing initialization must not require completing it first.

### wiss-010: MCP interface for coordinators

Add a thin MCP adapter over the existing application services. Initial tools should cover task creation, status, output, and retry. Return structured errors and available next actions; preserve human approval boundaries. Do not implement a second workflow engine or expose every internal operation.

### wiss-011: Evolving workflow report

Produce a task-level report early in execution and update it throughout the workflow, rather than generating it only at completion. Refresh it at stage and attempt boundaries, failures, retries, approval/event waits, and terminal outcomes. Include the current state, latest update time, elapsed time, completed stages, attempts and retries, errors, artifacts, verification results, and the next action when one is required.

Retain stage and attempt history as the report evolves, including rework and failed attempts. Include cumulative token usage and cost when the provider or executor supplies them; distinguish unavailable values from zero and include failed attempts in totals where possible. Repeated updates or worker restarts must not double-count usage.

Derive the report from Temporal state and recorded execution evidence; it is a readable view, not a separate source of workflow truth. Keep it useful for humans and coordinator agents during execution. The final report should be the completed version of the same report, preserving the history gathered along the way.

## Temporal integration completion

Temporal remains the workflow engine. It owns durable progress, routing, waits, and decisions; Watts owns executor lifecycle, workspace changes, evidence, and publication. Kubernetes may later supply execution capacity, but it does not replace orchestration. These issues complete that boundary rather than introduce another runner.

Prioritize wiss-021 through wiss-025 for reliable local execution. Remote connectivity and portable execution can follow. The [tic-tac-toe eval](evals/go-workflow/runs/2026-10-10-tic-tac-toe/README.md) provides observed failure evidence; distributed requirements below are proposed capabilities, not demonstrated failures.

### wiss-021: Reconcile interrupted executors before retrying

The eval's heartbeat timeout left Ralph state marked active. The next attempt rejected stale evidence and required manual archiving. A durable Temporal state does not establish that the previous subprocess stopped or that its workspace is safe to reuse.

Define an attempt lifecycle shared by executor adapters. Reconcile process ownership and adapter state after timeout, cancellation, worker shutdown, or restart. Preserve partial output and identify unaccepted workspace changes; cancellation must not imply rollback. Distinguish an ordinary worker stop from canceling the task, and document how work continues after each.

Acceptance criteria:

- Every launch and recovery uses workflow/run, stage, attempt, and execution identities. A stale process or Ralph loop cannot supply evidence for a new attempt.
- Cancellation stops the owned process tree and records cleanup status. When cleanup cannot be confirmed, report the uncertainty and refuse overlapping mutation.
- Retrying an interrupted attempt requires no manual deletion or archiving in the supported Pi/Ralph path. Preserve prior evidence rather than overwrite it.
- Heartbeat, stage execution, and worker shutdown limits have documented purposes. Local sleep and server disconnection produce actionable diagnoses; they do not silently extend execution limits.
- Exercise heartbeat timeout, worker termination, cancellation, and local interruption with live Temporal and a controlled subprocess. Confirm recovery without surviving writers or fabricated success.

Coordinate with wiss-007, wiss-013, and wiss-017. Existing workspace locks and process journals are safeguards to build on, not proof that adapter recovery is complete.

### wiss-022: Make execution and result publication idempotent

An executor can finish its work while Temporal fails to receive its completion. Explicit retry or future automatic redelivery must not blindly repeat side effects or accept output from an obsolete execution. Disabling activity retries does not resolve this boundary.

Use stable execution identities and durable attempt receipts. Separate executor completion, validated output publication, and Temporal acceptance. Define recovery at each boundary, including reuse of a valid existing result and refusal of ambiguous partial work. External actions require their own idempotency strategy; do not promise exactly-once effects.

Acceptance criteria:

- Repeated delivery of the same execution request cannot launch concurrent writers or publish duplicate accepted results.
- A validated result is atomically recorded with its input/configuration identities and artifact hashes before reporting success. Recovery verifies it before reuse.
- Output from a timed-out or canceled execution cannot supersede the current accepted result. Replacement executions fence obsolete writers or remain blocked until termination is confirmed.
- Fault-injection tests cover crashes before launch, after execution, during publication, and after publication but before activity completion is acknowledged.
- Keep application code and workspace handling independent of Git.

Coordinate with wiss-016 for immutable packages and workspace publication. Implement the local receipt/recovery contract before introducing remote storage.

### wiss-023: Prevent incompatible workers from consuming a run

A stale installed Watts worker consumed the first build attempt in the eval while the rebuilt worker consumed other activities on the same queue. Project queue isolation alone does not protect protocol or adapter compatibility.

Define and record worker build identity, execution protocol version, and required capabilities. Use the official Temporal SDK's supported worker deployment/versioning facilities where appropriate; otherwise make queue isolation explicit. Do not build a parallel scheduler. Enforce project/runtime compatibility when accepting activity requests.

Acceptance criteria:

- Submissions are routed to compatible workers; an older binary cannot silently execute a newer run's activities.
- CLI/UI diagnostics expose the required execution version, connected worker identity where available, and an actionable incompatible/no-worker state.
- Document an upgrade policy for in-flight runs and pinned packages. Preserve replay compatibility or explicitly reject unsupported history; do not silently reinterpret it.
- Test two worker builds, restart, upgrade during an approval wait, and replay of captured histories. Use SDK replay tests without replaying external side effects.

Coordinate with wiss-014 and wiss-016. Watts is still 0.1.0; an internal protocol/build identity does not require a product version change.

### wiss-024: Separate runtime evidence from application changes

The eval reviewer wrote PASS, but the read-only completion gate rejected Ralph's project-root `.ralph-runner/active-loops/` registry as an unauthorized code change. Ralph then retried work it could not repair.

Define explicit locations and ownership for application files, task outputs, and executor/control metadata. Review and approval checks must compare the intended application revision while still detecting real unauthorized writes. Classify runtime or evidence failures separately from a completed review's `changes-required` outcome.

Acceptance criteria:

- Supported runtime metadata writes do not invalidate a read-only review. Exclusions are narrowly owned paths, not blanket permission to modify hidden directories.
- A reviewer change to application code or protected inputs still fails and retains evidence.
- Runtime bookkeeping failures surface as execution errors with their cause and next action. They do not become code findings or trigger futile agent rework.
- Retain attempt-specific verdicts and findings; subsequent attempts cannot overwrite the evidence used for an earlier decision.
- Repeat the minimal Go eval through final workflow completion without manual metadata cleanup or weakened gates.

Coordinate with wiss-001, wiss-013, wiss-016, and wiss-021.

### wiss-025: Persist complete attempt telemetry across failures

Drafting sessions supplied token usage, but Ralph's nested Pi processes ran with `--no-session` and retained no usage totals. Full workflow tokens and cost could not be evaluated. Temporal history should carry orchestration references, not substitute for missing executor telemetry.

Collect structured lifecycle events, logs, provider usage, and failures through the common executor contract. Persist them incrementally so cancellation or a crash does not erase earlier responses. Use these records to support wiss-003 and wiss-011 rather than creating independent reporting state.

Acceptance criteria:

- Capture usage for every supported agent path, including nested Ralph iterations, failed responses, rework, and canceled attempts when the provider supplies it.
- Stable response/event identities prevent double counting on restart or repeated delivery. Report input, output, and cache usage with their provider semantics; missing data remains unknown rather than zero.
- Record reported cost separately from estimated cost, including pricing provenance. Never infer actual billing from an unset model price.
- Preserve failure causes and cleanup status after terminal cancellation; CLI, UI, and reports retain stage/attempt history even when a live workflow query is unavailable.
- Store bulk transcripts and logs outside Temporal history, with bounded references and protected credential handling. A controlled multi-stage failure/retry test verifies cumulative accounting.

### wiss-026: Support authenticated remote Temporal connections

The current connection is documented for local or trusted self-hosted endpoints; TLS and Temporal Cloud authentication are not configured. Running Watts remotely requires a defined connection and credential policy.

Acceptance criteria:

- Use official SDK connection options for the supported TLS/authentication modes, namespace, endpoint, and connection deadlines. Document the supported deployment targets.
- Keep service connection settings outside portable workflow manifests and credentials outside packages, logs, and history. Reuse Secret/credential-source abstractions where appropriate without treating Temporal credentials as agent credentials.
- CLI and worker use consistent settings and produce distinct actionable errors for authentication, connection failure, missing namespace, and missing compatible worker.
- Verify against an authenticated test service. Keep the local development defaults usable.

### wiss-027: Execute portable stages through Temporal

The local worker depends on project files and local configuration. Complete the proposed distributed boundary so Temporal orchestrates resolved execution plans and executor adapters materialize stage packages, workspaces, and bound artifacts. This is a follow-on deployment capability, not a prerequisite for local recovery fixes.

Acceptance criteria:

- Temporal receives an immutable orchestration plan with package references, routing, upstream bindings, approval policies, and limits. Retries do not reread mutable authoring templates.
- Large artifacts and workspace revisions live in durable storage; orchestration records integrity-checked references. A worker on another node can materialize the same inputs without the originating project's absolute paths.
- Build publishes a candidate revision; review consumes that exact revision read-only. Failed or partial publication does not become accepted input for downstream stages.
- Human approvals bind to exact artifact/revision identities and remain durable without keeping an execution container alive.
- Define executor launch, observation, cancellation, and publication interfaces. A future Kubernetes adapter creates/reattaches to Jobs using stable execution identity, validates their output, and prevents duplicate launch; Kubernetes does not own Watts workflow routing.
- Verify node loss and replacement, repeated completion delivery, interrupted artifact transfer, and a build/review rework loop across isolated workspaces.

Coordinate with wiss-007, wiss-014, wiss-016, and wiss-017. The [stage communication proposal](docs/stage-communication.md) owns the contract; this issue wires it into Temporal rather than defining a competing schema.

### wiss-028: Separate Watts packages, task executions, and worker runtime

Project roots currently accumulate `.watts`, `watts.yaml`, `values.yaml`, `tasks`, `templates`, and `.ralph-runner`. Separate authored workflow design from task records and machine-owned execution state, taking inspiration from a repository's `helm/` chart directory.

The project's `watts/` directory is the **Watts package**, not a parent directory containing another default package:

```text
project/
  main.go
  go.mod
  watts/
    watts.yaml
    values.yaml       # optional
    templates/
    resources/        # skills, scripts, and other authored assets
```

`watts.yaml` describes the package; values and templates describe its resources. A package may define multiple named Workflow resources and their supporting Agent, Procedure, ConfigMap, and Secret resources. No additional grouping Kind is needed. Select a workflow explicitly or through a declared default when several exist; filenames carry no semantic meaning. Continue supporting external package references and composition.

Tasks and their durable execution records are separate from this package. Preserve SPEC.md, PLAN.md, pinned plans, approvals, results, and attempt history independently of worker working files. The exact task-storage location remains to be selected; this issue does not prescribe moving tasks into the package or treating runtime files as durable task storage.

Machine-level worker configuration lives at `~/.watts/config/settings.json`. Host execution environments and worker bookkeeping live at `~/.watts/runtime/`, with stable identities preventing collisions between projects or tasks. Do not put worker settings in package manifests. The only new worker setting needed for this scope is runtime retention; do not add speculative configuration options.

Default retention is **the latest execution per task**, including every attempt in that execution. It is not the last run across the whole worker. Preserve active executions and state required for recovery even when they exceed the retention limit. Remove older runtime data only after required evidence has been durably published. Durable task documents, approvals, reports, and published artifacts are outside this cleanup policy. Define ordering of executions and safe cleanup after restart; do not infer ownership solely from directory names or modification times.

Acceptance criteria:

- A project can author or extend its package using `<project>/watts/watts.yaml`, optional values, templates, and resources without generating runtime or execution data there.
- Multiple named workflows can be selected without an additional Kind or filename-based inference. Package location and application workspace root remain distinct, including for external packages.
- Worker settings and working files use the agreed home-directory locations. Do not create project-root `.watts` or tool-private directories for the new execution path.
- Define task/evidence storage before implementation, including which records are durable and how CLI/UI resolve them after worker cleanup or on another worker.
- Default cleanup retains all attempts of the latest execution for each task, never deletes active/recovery-required state, and never removes durable evidence. Test concurrent tasks, failed/canceled executions, restart, and repeated cleanup.
- Agent runtimes and their tools use generic attempt-scoped environments with the container/sandbox boundary tracked in wiss-013. Future containers/sandboxes preserve this contract rather than introducing another workflow lifecycle.
- Update discovery, scaffolding, examples, documentation, and diagnostics together. Provide an explicit safe transition for existing local projects and retain their execution evidence; no compatibility layer is required solely to preserve the old architecture.

Coordinate with wiss-006, wiss-007, wiss-011, wiss-013, wiss-014, wiss-016, wiss-020, wiss-021, and wiss-027. Retention configuration controls worker runtime only; artifact-store retention is a separate concern.

## Design questions

### wiss-012: Approval and task readiness

`watts task decide` is cumbersome. Explore a clearer way to express document readiness and workflow decisions. A ready `SPEC.md` should advance only according to the task's workflow; document readiness and human approval are separate concepts.

Consider whether a generated JSON or YAML task manifest should record readiness and decisions alongside the documents. Avoid introducing a second source of truth that conflicts with Temporal history or the approved document hashes.

A manual override may be useful, but its scope remains undefined. Specify which stages may be overridden and how the override is recorded before adding it.

### wiss-014: watts worker

Consider central watts worker or a stateless watts worker with persistence abstracted (sqllite for local, pg in remote)

### wiss-013: Containerized workloads across Kinds

Establish a container or sandbox execution boundary for Watts workloads, rather than centering execution on host-installed Pi/Ralph. The model must cover every Kind according to its role: Workflow owns orchestration; Agent and Procedure define executable work; ConfigMap and Secret supply scoped configuration and credentials. This does not require a separate running container for each resource. Approval and event waits remain durable orchestration states rather than idle containers.

Each Agent or Procedure attempt receives an isolated execution environment containing its resolved configuration, instructions, tools, and dependencies. Pi is an Agent runtime; Ralph is a tool used by Pi. Their private files belong inside that environment and must not define core orchestration or leak into the application workspace. Deterministic Procedures use the same lifecycle and publication contract without requiring an agent runtime or model.

Acceptance criteria:

- Define a common container/sandbox lifecycle for materialization, launch, observation, cancellation, recovery, and output publication. Use the resolved stage contract rather than a second workflow engine.
- Agent and Procedure workloads declare immutable runtime/image identities and execution requirements. Workflow schedules them through Temporal; ConfigMap and Secret resources are bound at launch with explicit scope.
- Supply upstream artifacts and an identified workspace revision explicitly. Build may write a candidate workspace; review/QA receive read-only application access and separate writable output locations. Accepted outputs are published before downstream consumption.
- Constrain filesystem, network, resource, and credential access. A writable project mount or host directory alone is not a security boundary. Keep unrelated host files and privileged runtime sockets inaccessible by default.
- Tool-private state, including Ralph registries, remains inside its execution environment. Shared orchestration consumes generic results and evidence rather than interpreting tool directory layouts.
- Preserve logs, usage, results, and failure evidence outside disposable containers. Cancellation and node loss cannot leave uncontrolled writers or let obsolete attempts publish accepted changes.
- Demonstrate an Agent workload, a deterministic Procedure, ConfigMap/Secret delivery without credential leakage, and a build/review loop across isolated environments. Verify timeout, cancellation, restart, and failed publication.

Select an initial container/sandbox backend explicitly; Kubernetes is a possible deployment backend, not a requirement for the contract. Host-runtime organization and retention are tracked in wiss-028. Coordinate with wiss-007, wiss-016, wiss-017, wiss-020, wiss-021, wiss-022, wiss-024, and wiss-027.

### wiss-016: Typed workflow resources and compiled execution packages

The authoring model uses **Workflow, Procedure, Agent, ConfigMap, and Secret**. A Workflow places reusable Agent or Procedure resources and owns routing. An Agent specializes the common work contract with model, runtime, identity, instructions, skills, and tools. Model and prompt are fields, not separate Kinds. See the [Kinds reference](docs/kinds.md) for implemented fields and the [Go generalist example](resources/examples/go_generalist/README.md) for usage.

Implemented authoring support includes bundle-local references, typed decoding, values-driven templates, ConfigMap specification defaults, environment-backed Secret references, and resolution into pinned local execution definitions. This does not complete portable execution packaging.

Remaining work:

- Compile immutable orchestration manifests and self-contained execution packages containing instructions, skills, scripts, and configuration, with integrity identities and pinned runtime dependencies.
- Define explicit upstream evidence bindings, workspace revisions, artifact transfer, and publication of accepted changes for isolated executors on distinct nodes.
- Extend shared configuration bindings when concrete needs justify parameter or file-mount support; preserve snapshots so edits cannot silently change active runs.
- Extend credential sources beyond environment variables, with scoped delivery, rotation/revocation semantics, and protected-file or managed-store support. Keep values out of packages and history.
- Define package schemas, compile/apply interfaces, and runtime requirements. Workers should execute resolved packages without depending on mutable authoring assets.

Keep resource definitions distinct from execution records and preserve the build/review loop with attempt-specific findings and exact workspace revision bindings. Coordinate with wiss-007 and wiss-014. The [stage communication proposal](docs/stage-communication.md) owns the distributed contract; Kubernetes-style continuous reconciliation is not implied.

## Long-term features

These items describe future capabilities beyond the MVP. Implementation choices remain open; develop them around the stage contracts and workflow policies rather than adding separate orchestration paths.

### wiss-017: Sandboxed execution in a developer's local environment

Let developers run workflow stages against a local project while viewing code changes, diffs, stage output, and generated artifacts in real time through their editor and the Watts UI. Support the same experience for build, review, QA, and other stages, with workspace permissions appropriate to each stage.

Evaluate local containers, microVMs, and operating-system sandboxes. Compare a constrained project mount with an isolated project copy that the editor can inspect and Watts can apply back to the project. A mounted directory alone does not provide a security boundary. Define access to project files, dependencies, network services, MCP tools, and credentials explicitly; keep unrelated host files, host credentials, and privileged runtime sockets inaccessible by default. Watts' current agent guardrails are not a sandbox. See [sandbox research](docs/sandbox.md).

Preserve the [stage communication contract](docs/stage-communication.md): live changes are a candidate revision, and downstream stages consume explicitly published revisions. Builder stages may write application code; review and QA stages should normally inspect that code read-only and write their own reports separately. Seeing a change in the editor must not count as accepting it or approving the task.

Define how concurrent developer edits, cancellation, failed attempts, and partial changes are handled. Pin the revision under review, detect conflicts before applying accepted changes, and invalidate approval when the reviewed content changes. Demonstrate that a developer can follow an attempt live, stop it, inspect its changes, and choose whether to apply them without exposing the rest of their machine. Coordinate with wiss-007 and wiss-016.

### wiss-018: Source-host integrations

Provide optional adapters for GitHub, GitLab, and other source hosts. Let teams map a Watts project to a repository, select a base revision, publish accepted changes as a pull or merge request, and link task reports and verification evidence to that request. Keep Git and source-host services optional; local workflows must continue to work without them.

Allow workflows to consume remote check results, review decisions, and other configured events. Bind each result to the exact revision it concerns so a passing check or approval for older code cannot advance a newer candidate. Define how external review relates to Watts approval policy, and make merging or deployment a separately authorized action.

Use scoped credentials and verified webhook delivery. Handle duplicate events, retries, and interrupted publication without creating duplicate requests or advancing a task twice. Show integration failures and the next action in the task report and UI. Keep provider-specific behavior in adapters over the same application services used by the CLI and proposed MCP interface.

### wiss-019: Ticket intake and automated task initiation

Let teams create Watts tasks from Jira, Linear, and other issue trackers. Map the ticket to a project and workflow, retain a source link and ticket identity, and capture the relevant title, description, and acceptance criteria as the initial `SPEC.md` draft. The configured specification stage refines that draft; planning then follows the workflow's normal `PLAN.md` process.

Support opt-in automation rules, such as starting a task when a ticket moves to “In Progress,” matches a label, or receives an explicit command. For example, a transition could create the task, run specification refinement, and pause with a draft ready for approval. Whether approval is required and which stages may run automatically must come from workflow and intake policy. A ticket status change is not itself specification approval.

Define project/workflow selection, execution authorization, concurrency limits, and spending limits before unattended execution. Verify incoming events and deduplicate them using stable ticket and event identities. Distinguish a repeated delivery from an intentional new run or a later transition back to “In Progress.”

Record the ticket version used to create the draft. Later ticket edits should produce a visible update request or new revision, without silently replacing a submitted or approved specification. Optionally report task progress, errors, approval requests, and completion back to the tracker; prevent those updates from retriggering intake in a loop. Reuse the coordinator services in wiss-010 and the evolving report in wiss-011, keeping ticket retrieval and provider-specific automation outside the core workflow engine.
