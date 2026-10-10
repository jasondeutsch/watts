# Workflow execution

A Workflow orders named steps and routes their results. Each step references an Agent or Procedure using `use`. See [Kinds](kinds.md) for resource fields and [templating](templates.md) for values-driven packages. Temporal owns execution state; a Watts worker executes local work from the project root containing `watts.yaml`.

## Select and customize

Bundled definitions are in [workflow-templates/](../workflow-templates/): `sdlc` is the default; `research` is a smaller agent workflow. They are embedded in the binary. Select a bundled name, project-relative bundle file, or directory in Project `spec.default_workflow`:

```yaml
spec:
  default_workflow: workflows/team-v1
```

`watts task new <slug>` resolves that source, creates its declared artifacts, and writes a self-contained task `workflow.yaml`. Edit that file before first submission. Project-default changes affect future tasks; first submission pins the resolved task definition and runtime configuration. Retry uses the pinned settings. See the [runbook](runbook.md) for execution, approval, and recovery commands.

The default SDLC runs specification, human spec review, planning, human plan review, build, and agent review. The human starts with rough intent in `SPEC.md`; the specification Agent refines it. `SPEC.md` describes what and why; `PLAN.md` describes implementation and verification. Custom workflows define their own artifacts and gates.

## Routing and rework

The first listed step runs first. Without explicit transitions, successful work follows `next`, or the next listed step. The last step finishes at `end`.

`transitions` maps outcomes to step names or `end`. A nonempty mapping is exhaustive: an undeclared outcome fails the attempt. It can route review findings back to build:

```yaml
kind: Workflow
schema_version: 1
name: qa-loop
spec:
  version: 1
  max_transitions: 30
  steps:
    - name: coding
      use: {kind: Agent, name: builder}
    - name: qa
      use: {kind: Procedure, name: test-suite}
      parameters:
        suite: integration
      transitions:
        passed: end
        changes-required: coding
---
kind: Agent
schema_version: 1
name: builder
spec:
  runtime: pi
  runtime_agent: build
  identity: team-builder
  provider: openrouter
  model: deepseek/deepseek-v4-flash
  sandbox: {mode: local}
  prompt: Implement {task}/SPEC.md in the project root. Read previous QA feedback.
---
kind: Procedure
schema_version: 1
name: test-suite
spec:
  execution:
    type: capability
    capability: test-suite
```

This bundle requires a Project capability named `test-suite`, described below. Agent provider credentials must also be available to the worker.

An execution error follows the placement's `failure` transition, then `on_failure` if no such transition is defined. Without a failure route it waits for retry. `optional: true` permits non-approval/non-event work to skip an execution error when no failure route handles it. Routing a failure to `end` still fails the workflow.

Approvals return `approved` or `rejected`. Declare both transitions to make the gate's routing explicit. A rejection without a `rejected` transition is an execution failure and follows the failure/retry policy. Event waits require declared transitions and accept only their named outcomes. Neither human decisions nor external events can select arbitrary destinations.

Step attempt budgets and Workflow `max_transitions` bound loops. All attempts remain in history. Execution is sequential; parallel work and expression conditions are not implemented.

## Results and artifacts

All executors return the same result shape:

```json
{
  "outcome": "changes-required",
  "feedback": "The parser fails on empty input. Fix it and rerun QA.",
  "artifacts": ["qa-report.json"],
  "data": {"failed_tests": 1}
}
```

Artifacts must exist within the task folder; Watts validates paths and computes hashes. Required outputs and completion checks must also pass. stdout/stderr are diagnostic logs, separate from structured routing results.

The next attempt receives the immediately preceding result, including feedback, data, and artifact hashes. Agent prompts include the path to this context. Explicit bindings to earlier producers across intervening steps remain part of the [stage communication proposal](stage-communication.md).

Agent and command/check Procedures can declare a task-relative `result_file`. Watts removes the previous file before execution, sets `WATTS_STEP_INPUT` and `WATTS_STEP_RESULT`, and requires a fresh JSON result. Without a result file these executors return `success` after output checks pass. Capabilities return their result on stdout instead. Attempt inputs and validated results are recorded under `<task>/.watts-state/steps/<step>-<attempt>/`.

## Capability plugins

Register an executable in Project `spec.capabilities`:

```yaml
spec:
  capabilities:
    test-suite:
      command: [python3, tools/qa_plugin.py]
```

Watts launches this argv from the project root without an implicit shell. It reads one JSON request on stdin:

```json
{
  "version": 1,
  "project": "/path/to/project",
  "task": "tasks/2026-10-06-parser-fix",
  "step": "qa",
  "attempt": 1,
  "parameters": {"suite": "integration"},
  "previous": {"step": "coding", "attempt": 1, "outcome": "success"}
}
```

Declared input hashes appear in `inputs`. Emit exactly one result object on stdout, at most 1 MiB, and logs on stderr. A nonzero exit is an execution error. An expected outcome such as failed QA exits zero and returns `changes-required`; the Workflow decides where it goes. Plugins run with the worker's environment and cancellation context.

## External events

Reference a Procedure with `execution: {type: event}` and placement transitions such as `passed: end` and `changes-required: coding`. While status is `waiting_event`, an integration delivers an `orchestration.Event` through Temporal's validated Workflow Update. Event delivery has no MVP CLI command.

The event must identify the waiting step and attempt, a unique event ID, and a declared outcome. Watts rejects stale attempts, wrong steps, reused IDs, and undeclared outcomes. Use an approval Procedure for human decisions over artifact hashes, rather than an event wait.

## MCP tools

Declare named server connections in Project `spec.mcps`:

```yaml
spec:
  mcps:
    gopls:
      command: gopls
      args: [mcp]
      cwd: .
```

Select them on the Agent resource with `spec.mcps: [gopls]`. Procedures cannot select MCPs. Undefined and duplicate selections fail validation; omitting the list supplies no configured servers.

Stdio connections support `command`, `args`, `env`, and `cwd`; HTTP connections use `url` and `headers`. Both support `exposure` and `description`; exposure defaults to `direct`. Relative working directories resolve from the project root. MCP credential references use `${NAME}` and Project `env_passthrough`; export values before starting the worker. MCP connections are not declared in `pi_settings`, and Secret resource bindings do not replace MCP's credential-reference syntax.

Server definitions and selections are pinned at submission. Watts writes the selected servers to the runtime role's Pi MCP configuration for an activity and restores its interactive configuration afterward. Ralph children inherit that directory. Pi's built-in MCP support must be enabled. See the [Go example](../resources/examples/go_generalist/mcp/README.md).

## Workspace boundaries

Application source, tests, dependencies, and build files live in the project root or its normal source directories. Task folders hold specifications, plans, reviews, logs, and execution evidence. Checks run from the project root. Local execution changes project files directly; cancellation does not roll back edits. Portable packages, isolated workspaces, and explicit publication remain [proposed](stage-communication.md).
