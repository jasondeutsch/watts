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

### wiss-010: MCP interface for coordinators

Add a thin MCP adapter over the existing application services. Initial tools should cover task creation, status, output, and retry. Return structured errors and available next actions; preserve human approval boundaries. Do not implement a second workflow engine or expose every internal operation.

### wiss-011: Evolving workflow report

Produce a task-level report early in execution and update it throughout the workflow, rather than generating it only at completion. Refresh it at stage and attempt boundaries, failures, retries, approval/event waits, and terminal outcomes. Include the current state, latest update time, elapsed time, completed stages, attempts and retries, errors, artifacts, verification results, and the next action when one is required.

Retain stage and attempt history as the report evolves, including rework and failed attempts. Include cumulative token usage and cost when the provider or executor supplies them; distinguish unavailable values from zero and include failed attempts in totals where possible. Repeated updates or worker restarts must not double-count usage.

Derive the report from Temporal state and recorded execution evidence; it is a readable view, not a separate source of workflow truth. Keep it useful for humans and coordinator agents during execution. The final report should be the completed version of the same report, preserving the history gathered along the way.

## Design questions

### wiss-012: Approval and task readiness

`watts task decide` is cumbersome. Explore a clearer way to express document readiness and workflow decisions. A ready `SPEC.md` should advance only according to the task's workflow; document readiness and human approval are separate concepts.

Consider whether a generated JSON or YAML task manifest should record readiness and decisions alongside the documents. Avoid introducing a second source of truth that conflicts with Temporal history or the approved document hashes.

A manual override may be useful, but its scope remains undefined. Specify which stages may be overridden and how the override is recorded before adding it.

### wiss-014: watts worker

Consider central watts worker or a stateless watts worker with persistence abstracted (sqllite for local, pg in remote)

### wiss-013: Ralph artifact location

Determine where `.ralph-runner` belongs. It currently contains task-specific status, events, iteration records, and transcripts under the task directory.

Task-scoped execution evidence should remain easy to associate with its task. Consider placing executor artifacts under the task's `.watts-state/` directory rather than exposing a separate runtime directory. A project-level `.watts/` location would need to preserve task and attempt isolation.

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
