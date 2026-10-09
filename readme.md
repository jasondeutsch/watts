# Watts

Watts runs configurable coding workflows with AI agents, human review gates, and automated checks. Temporal manages workflow state; project workers execute the stages. The current implementation uses Pi and Ralph for agent execution.

This is an MVP and proof of concept. The application version remains **0.1.0** while the design evolves. Watts does not require Git.

## Get started

You need Go 1.27 or later to build Watts, Node.js for Pi, and a running Temporal server. See the [runbook](runbook.md) for prerequisites and provider setup.

From this checkout, install Watts and start local Temporal with Docker Compose:

```sh
go install .
docker compose up -d --wait
```

Ensure the Go binary directory is on your `PATH`. Temporal can also run remotely; see [Temporal setup](temporal.md).

From the project directory where you want application work performed:

```sh
watts init
watts install
watts doctor
watts start -d
```

The bundled workflows currently select `deepseek/deepseek-v4-flash` through OpenRouter. Export `OPENROUTER_API_KEY` in the shell **before starting Watts**. Configure another provider or model through [project configuration](runbook.md#configure-the-project) and [workflow templates](workflows.md).

`watts start` starts the project's worker and web UI, opens your default browser, and stays in the foreground. Use `-d` to detach and `watts stop` to stop both. Detached logs are in `.watts/service.log`. Temporal is managed separately.

## Complete a task

```sh
watts task new hello-server
```

Use the task path printed by the command. Write the request or rough notes in its `SPEC.md`, and customize its `workflow.json` before submission.

```sh
watts task run <task>
watts task status <task>
```

The default workflow is:

**Specification → human spec review → planning → human plan review → build → agent review**

When the workflow pauses for human review, inspect the document and approve it:

```sh
watts task decide <task> spec-review
# Later, when planning is ready:
watts task decide <task> plan-review
```

To request changes, add `--reject --feedback "Changes needed"`. Agent review can return work to build.

Task submission returns before execution finishes. Follow progress in the web UI or with `task status`. If a stage fails, inspect its error and output, then use `watts task run <task>` or the UI's retry button. Submitted workflow definitions and runtime configuration are pinned; retry does not load subsequent edits.

See the [task runbook](runbook.md) for the full procedure, approval rules, cancellation, and recovery.

## Project layout

The directory containing `watts.json` is the project root. Application source, tests, dependencies, and build files belong there or in its normal source directories.

- `watts.json`: provider connections, agent settings, task location, and default workflow selection.
- `.watts/`: local agent state, generated assets, and service files.
- `tasks/<task>/`: `SPEC.md`, `PLAN.md`, `workflow.json`, review documents, logs, and execution evidence.

A project can contain multiple tasks. Each task receives its own workflow definition. Bundled templates live in [workflow-templates/](workflow-templates/); projects can supply a default using `default_workflow` in `watts.json`.

## Documentation

| Document | Covers |
| --- | --- |
| [Runbook](runbook.md) | Setup, configuration, task execution, and recovery |
| [Workflows](workflows.md) | Templates, stages, capabilities, routing, and implementation location |
| [Temporal](temporal.md) | Infrastructure, workers, durable execution, and recovery |
| [Web UI](web/readme.md) | Task monitoring, output, and retry controls |
| [Agent resources](resources/readme.md) | Coordinator skill and supporting resources |
| [Go workflow eval](evals/go-workflow/README.md) | Repeatable task cases, human scoring, and comparison records |
| [CLI organization](cli/readme.md) | Command-layer structure |
| [Workflow design](workflow-design.md) | Architecture and design rationale |
| [Stage communication](stage-communication.md) | Proposed stage interfaces and build/review feedback |
| [Sandboxing](sandbox.md) | Isolation considerations |
| [Issues](issues.md) | Known problems, proposed improvements, and design questions |

## Development

Run the repository checks:

```sh
go test ./...
```

Rebuild Watts and restart its service after changing the implementation. Tests validate software behavior; the [eval scaffold](evals/go-workflow/README.md) supports evaluating a configured workflow's results over time.

## License

[MPL 2.0](LICENSE). Third-party dependencies and attributed material retain their respective licenses.
