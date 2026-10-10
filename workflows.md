# Managing workflows and capabilities

Every task workflow runs on Temporal. Start a Temporal service and a Watts worker as described in [temporal.md](temporal.md). The `temporal` configuration object only overrides connection settings; an omitted object uses localhost:7233 and the default namespace.

## YAML manifests

Human-authored configuration uses one format: YAML. The project file is `watts.yaml` (`kind: Project`), and task definitions are `workflow.yaml` (`kind: Workflow`). Both require `kind`, `schema_version: 1`, a nonempty `name`, and a `spec` mapping. The name labels the document; filenames and project-relative paths select resources. `schema_version` versions the envelope; `spec.version` versions the existing configuration or workflow schema.

Each file contains one resource. Bundles and standalone Stage/Agent manifests are not supported yet; their design is tracked in wiss-016. Stages remain ordered entries in `spec.steps`. Agent, MCP and capability names resolve against the Project's `spec.agents`, `spec.mcps` and `spec.capabilities` mappings. `default_workflow` selects a bundled name or a project-relative YAML file. An inline workflow uses `spec.workflow` with a `steps` list rather than a second envelope.

Unknown fields, duplicate YAML keys, wrong kinds and unsupported schema versions fail loading. Syntax and unknown-field errors show the source location. Comments and mapping order do not affect runtime settings; step order and instruction text do. Configuration updates regenerate YAML and do not preserve authoring comments. On first submission Watts resolves settings and freezes the definition and runtime configuration as JSON; hashes cover those resolved values rather than YAML source bytes. Compiled portable stage packages remain proposed in [stage communication](stage-communication.md).

Use YAML block scalars for multiline instructions and document templates:

```yaml
spec:
  templates:
    SPEC.md: |
      # SPEC
      Describe the requested change here.
```

## Rendering parameterized packages

Use `watts template` to render a local package independently of a Watts project or Temporal:

```sh
watts template ./team-workflow
watts template ./team-workflow -f team.yaml --values project.yaml
watts template ./team-workflow -f project.yaml -o ./workflows
```

A package contains a `templates/` directory, optional `values.yaml` defaults, and an optional `values.schema.json`. For example:

```text
team-workflow/
├── values.yaml
└── templates/
    ├── workflow.yaml.gotmpl
    └── _helpers.tpl
```

Templates use Go template syntax and `.Values`. `.yaml.gotmpl` files render to `.yaml`; `.tpl` files define helpers without producing files. Literal `.yaml` files pass through. Defaults load first, then repeatable `-f`/`--values` files in command-line order. Nested mappings merge; lists and scalar values replace. Override paths resolve from the invocation directory, not the package directory.

Without `-o`/`--output`, the command prints a YAML document stream to stdout. Directory output preserves paths beneath `templates/` and refuses existing files. Argument errors exit with status 2; loading, rendering, validation, and output failures exit with status 1. The command checks YAML and the optional values schema; normal Watts loaders validate resource fields and workflow rules when those files are used.

Rendering does not start or submit a task. Select the resulting Workflow file using `spec.default_workflow`, or use it as a task's `workflow.yaml` before submission. Existing submitted tasks retain their pinned definitions. See the [rendering package reference](internal/render/README.md) for supported functions, helper syntax, and filesystem boundaries. Keep credentials as references rather than values supplied to templates.

## Base templates and project defaults

Base workflow templates live in the Watts repository's [workflow-templates/](workflow-templates/) directory. [sdlc.yaml](workflow-templates/sdlc.yaml) is the default; [research.yaml](workflow-templates/research.yaml) is a smaller agent workflow. The binary embeds these YAML manifests.

Every `watts task new <slug>` copies the project default into `<task>/workflow.yaml`. With no override, it uses SDLC. To provide your own default, save a Workflow manifest in your project and set its project-relative path in watts.yaml:

```yaml
spec:
  default_workflow: my-workflow.yaml
```

You can also select a bundled base by name, such as `default_workflow: research`. Edit files directly; workflow list, create, clone, edit and inspection commands are outside the MVP.

Edit `<task>/workflow.yaml` to customize that task's agents, prompts, inputs, outputs and checks. Editing the project default affects future tasks. First submission freezes the task definition and runtime settings for that execution.

Stages run sequentially in the order listed in `steps`. Explicit review-rejection or failure transitions can route execution back to an earlier stage. Each agent stage declares its own `identity` in the workflow:

```yaml
kind: Workflow
schema_version: 1
name: implement-and-review
spec:
  version: 1
  steps:
  - name: implement
    agent: build
    provider: openrouter
    model: deepseek/deepseek-v4-flash
    identity: my-builder
    prompt: Implement the change described in {task}/SPEC.md
  - name: review
    agent: review
    provider: openrouter
    model: deepseek/deepseek-v4-flash
    identity: my-reviewer
    prompt: Review the implementation and write findings in {task}/REVIEW.md
    outputs:
    - REVIEW.md
```

Each agent stage declares `provider` and `model` alongside its `agent`, `identity`, and `prompt`. Provider endpoints and credential sources remain in watts.yaml. Using the same model for multiple stages is allowed without a warning or an opt-in flag.

A workflow's `templates` map scaffolds task artifacts; `capture_baseline: true` records an initial snapshot. `pre_checks` run before an agent, and `checks` run afterwards. The default SDLC declares its drafting, human review, implementation and independent review stages explicitly.

## Outcomes and rework

A stage sets exactly one executor: `agent`, `command`, `check`, `capability`, `human: true`, or `event: true`. Steps execute sequentially. `transitions` maps named outcomes to stage IDs or `end`. An explicit mapping is exhaustive: an undeclared outcome fails the attempt rather than silently advancing. Without a mapping, `next` or list order supplies the success path. `on_failure` supplies an execution-error path; a `failure` entry in `transitions` takes precedence. Optional non-human/non-event steps may skip execution errors.

Put this definition in `workflows/qa-loop.yaml`:

```yaml
kind: Workflow
schema_version: 1
name: qa-loop
spec:
  version: 1
  max_transitions: 30
  steps:
  - name: coding
    agent: build
    provider: openrouter
    model: deepseek/deepseek-v4-flash
    prompt: |
      Implement the requested change in {task}.
      Read previous QA feedback before proceeding.
  - name: qa
    capability: test-suite
    parameters:
      suite: integration
    transitions:
      passed: end
      changes-required: coding
```

Each rework attempt is retained in Temporal history and in `task status --json`. The next stage receives the preceding result, including outcome, feedback, data and artifact hashes. Agents receive the path to that context in their prompt. Stage attempt limits and the workflow's total transition budget bound loops. `max_attempts` defaults to the configured limit or three; an exhausted stage waits for an explicitly authorized retry.

## Executable capability plugins

Register capabilities in `watts.yaml`:

```yaml
spec:
  capabilities:
    test-suite:
      command:
      - python3
      - tools/qa_plugin.py
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

```yaml
spec:
  steps:
  - name: ci
    event: true
    transitions:
      passed: end
      changes-required: coding
```

When task status shows `waiting_event`, an external integration sends an `orchestration.Event` through the validated Temporal update. It includes the waiting stage's attempt number, a unique event ID, and a declared outcome. Event delivery is not exposed in the MVP CLI.

Watts delivers this through a validated Temporal Workflow Update so the sender receives acceptance or rejection. The workflow rejects undeclared outcomes, wrong stages, stale attempts and reused event IDs. Events route only through the waiting stage's declared transitions; they cannot interrupt an arbitrary running activity or jump to an undeclared destination. Use a distinct ID for each new event. Event stages are automation inputs, not human approvals; approvals use `task decide` and artifact-hash validation.


## Implementation location

The project root is the directory containing `watts.yaml`. Application source, tests, dependencies and build files live there or in its normal source directories. A task directory holds `SPEC.md`, `PLAN.md`, `workflow.yaml`, review documents, logs and execution evidence. Default planning and build instructions use this separation, and quality gates run from the project root.

## MCP servers for agent stages

Declare named MCP connections under `mcps` in watts.yaml:

```yaml
spec:
  mcps:
    gopls:
      command: gopls
      args:
      - mcp
      cwd: .
```

Select them on agent stages with `mcps: [gopls]`. Other executor types cannot select MCPs; undefined or duplicate names are rejected. Omitting the list gives that stage no configured MCP servers. Connections and selections are pinned at submission.

Connections support stdio `command`, `args`, `env`, `cwd`, or HTTP `url`, `headers`, plus `exposure` and `description`. Default exposure is `direct`. Relative working directories resolve from the project root. Use `${NAME}` credential references and `env_passthrough`; values must exist in the worker environment. MCP definitions belong in watts.yaml, not `pi_settings`.

Watts replaces the role's Pi MCP configuration for the activity and restores its interactive configuration afterward. Ralph children inherit the same agent directory; Pi's built-in MCP support must be enabled. See the [Go example](resources/examples/go_generalist/mcp/README.md).
