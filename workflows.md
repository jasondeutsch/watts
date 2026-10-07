# Managing workflows and capabilities

Every task workflow runs on Temporal. Start a Temporal service and a Watts worker as described in [temporal.md](temporal.md). The `temporal` configuration object only overrides connection settings; an omitted object uses localhost:7233 and the default namespace.

## Base templates and project defaults

Base workflow templates live in the Watts repository's [workflow-templates/](workflow-templates/) directory. [sdlc.json](workflow-templates/sdlc.json) is the default; [research.json](workflow-templates/research.json) is a smaller agent workflow. The binary embeds these JSON files.

Every `watts task new <slug>` copies the project default into `<task>/workflow.json`. With no override, it uses SDLC. To provide your own default, save a workflow JSON file in your project and set its project-relative path in watts.json:

```json
{
  "default_workflow": "my-workflow.json"
}
```

You can also select a bundled base by name, such as `"default_workflow": "research"`. Edit files directly; workflow list, create, clone, edit and inspection commands are outside the MVP.

Edit `<task>/workflow.json` to customize that task's agents, prompts, inputs, outputs and checks. Editing the project default affects future tasks. First submission freezes the task definition and runtime settings for that execution.

Stages run sequentially in the order listed in `steps`. Explicit review-rejection or failure transitions can route execution back to an earlier stage. Each agent stage declares its own `identity` in the workflow:

```json
{
  "version": 1,
  "steps": [
    {
      "name": "implement",
      "agent": "build", "provider": "ollama", "model": "devstral-small-2:24b",
      "identity": "my-builder",
      "prompt": "Implement the change described in {task}/SPEC.md"
    },
    {
      "name": "review",
      "agent": "review",
      "identity": "my-reviewer",
      "prompt": "Review the implementation and write findings in {task}/REVIEW.md",
      "outputs": ["REVIEW.md"]
    }
  ]
}
```

Each agent stage declares `provider` and `model` alongside its `agent`, `identity`, and `prompt`. Provider endpoints and credential sources remain in watts.json. Using the same model for multiple stages is allowed without a warning or an opt-in flag.

A workflow's `templates` map scaffolds task artifacts; `capture_baseline: true` records an initial snapshot. `pre_checks` run before an agent, and `checks` run afterwards. The default SDLC declares its drafting, human review, implementation and independent review stages explicitly.

## Outcomes and rework

A stage sets exactly one executor: `agent`, `command`, `check`, `capability`, `human: true`, or `event: true`. Steps execute sequentially. `transitions` maps named outcomes to stage IDs or `end`. An explicit table is exhaustive: an undeclared outcome fails the attempt rather than silently advancing. Without a table, `next` or list order supplies the success path. `on_failure` supplies an execution-error path; a `failure` entry in `transitions` takes precedence. Optional non-human/non-event steps may skip execution errors.

Put this definition in `workflows/qa-loop.json`:

```json
{
  "version": 1,
  "max_transitions": 30,
  "steps": [
    {
      "name": "coding",
      "agent": "build", "provider": "ollama", "model": "devstral-small-2:24b",
      "prompt": "Implement the requested change in {task}. Read previous QA feedback before proceeding."
    },
    {
      "name": "qa",
      "capability": "test-suite",
      "parameters": {"suite": "integration"},
      "transitions": {
        "passed": "end",
        "changes-required": "coding"
      }
    }
  ]
}
```

Each rework attempt is retained in Temporal history and in `task status --json`. The next stage receives the preceding result, including outcome, feedback, data and artifact hashes. Agents receive the path to that context in their prompt. Stage attempt limits and the workflow's total transition budget bound loops. `max_attempts` defaults to the configured limit or three; an exhausted stage waits for an explicitly authorized retry.

## Executable capability plugins

Register capabilities in `watts.json`:

```json
"capabilities": {
  "test-suite": {"command": ["python3", "tools/qa_plugin.py"]}
}
```

A capability can invoke a test suite, service, model, or other integration. Watts launches its argv from the project root; no implicit shell is involved. To use a shell, register it explicitly. The command is project-configured and runs with the worker's environment and process cancellation. It reads one JSON request on stdin:

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

Declared input artifact hashes appear in `inputs`. Emit exactly one JSON object on stdout and send logs to stderr. A nonzero exit is an execution error. An expected business outcome such as failed QA should exit zero and return:

```json
{
  "outcome": "changes-required",
  "feedback": "The parser fails on empty input. Fix it and rerun QA.",
  "artifacts": ["qa-report.json"],
  "data": {"failed_tests": 1}
}
```

Artifacts must exist inside the task folder. Watts validates their paths and calculates hashes; plugins never supply trusted hashes. Declared outputs and completion checks must also pass. Results are limited to 1 MiB. Attempt requests and validated results are recorded in `.watts-state/steps/<stage>-<attempt>/`.

Agents and shell commands can return the same result format by declaring `result_file`, a task-relative JSON path. Watts removes the previous result before each attempt, sets `WATTS_STEP_INPUT` and `WATTS_STEP_RESULT`, and requires a fresh file. An agent's prompt includes the result path and declared outcomes. Executors without a result file return `success` after their output checks pass. Human decisions return `approved` or `rejected`; map those outcomes to route an approval or rejection explicitly.

## External events

Use an event stage when the workflow must wait for CI or another integration:

```json
{
  "name": "ci",
  "event": true,
  "transitions": {"passed": "end", "changes-required": "coding"}
}
```

When task status shows `waiting_event`, an external integration sends an `orchestration.Event` through the validated Temporal update. It includes the waiting stage's attempt number, a unique event ID, and a declared outcome. Event delivery is not exposed in the MVP CLI.

Watts delivers this through a validated Temporal Workflow Update so the sender receives acceptance or rejection. The workflow rejects undeclared outcomes, wrong stages, stale attempts and reused event IDs. Events route only through the waiting stage's declared transitions; they cannot interrupt an arbitrary running activity or jump to an undeclared destination. Use a distinct ID for each new event. Event stages are automation inputs, not human approvals; approvals use `task decide` and artifact-hash validation.


## Implementation location

The project root is the directory containing `watts.json`. Application source, tests, dependencies and build files live there or in its normal source directories. A task directory holds `SPEC.md`, `PLAN.md`, `workflow.json`, review documents, logs and execution evidence. Default planning and build instructions use this separation, and quality gates run from the project root.
