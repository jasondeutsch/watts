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

The test implementation was created under `tasks/<task>/` rather than in the project rooted at `watts.json`.

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

### wiss-009: Worker terminal output

Provide readable, well-formatted logs showing stage and attempt boundaries, meaningful progress, failures, and shutdown. Include a concise failure explanation and a next action rather than only a process exit code.

### wiss-010: MCP interface for coordinators

Add a thin MCP adapter over the existing application services. Initial tools should cover task creation, status, output, and retry. Return structured errors and available next actions; preserve human approval boundaries. Do not implement a second workflow engine or expose every internal operation.

### wiss-011: Completion report

Report elapsed time, completed stages, attempts and retries, artifacts, and verification results. Include token usage and cost when the provider or executor supplies them; distinguish unavailable values from zero. Include failed attempts in usage totals where possible.

## Design questions

### wiss-012: Approval and task readiness

`watts task decide` is cumbersome. Explore a clearer way to express document readiness and workflow decisions. A ready `SPEC.md` should advance only according to the task's workflow; document readiness and human approval are separate concepts.

Consider whether a generated JSON or TOML task manifest should record readiness and decisions alongside the documents. Avoid introducing a second source of truth that conflicts with Temporal history or the approved document hashes.

A manual override may be useful, but its scope remains undefined. Specify which stages may be overridden and how the override is recorded before adding it.

### wiss-014: watts worker

Consider central watts worker or a stateless watts worker with persistence abstracted (sqllite for local, pg in remote)

### wiss-013: Ralph artifact location

Determine where `.ralph-runner` belongs. It currently contains task-specific status, events, iteration records, and transcripts under the task directory.

Task-scoped execution evidence should remain easy to associate with its task. Consider placing executor artifacts under the task's `.watts-state/` directory rather than exposing a separate runtime directory. A project-level `.watts/` location would need to preserve task and attempt isolation.

### wiss-015: TOML for configuration and workflow authoring

Use TOML as the preferred direction for replacing JSON in human-authored project configuration and workflow definitions. Prioritize readable stage declarations, comments, multiline instructions, clear validation errors, and tooling support. Validate the representation against the Go generalist workflow, including nested checks, transitions, and MCP connections.

Distinguish TOML authoring files from generated stage manifests and executor request/result protocols. Those machine-facing formats may remain JSON. Define how TOML inputs are resolved into deterministic compiled plans with stable content hashes. See [stage communication](stage-communication.md) for the proposed compilation boundary.

Define filenames, schema, and the scope of the change before implementation. Prefer one authoring format rather than adding YAML or Pkl support without a concrete need.
