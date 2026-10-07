# Temporal workflows

Watts uses Temporal for durable execution of a configurable workflow. The Go SDK is compiled into Watts. A Temporal Service stores workflow history, and a Watts worker runs activities against your local project files. The service does not run Pi or hold a copy of the project.

Temporal is required for every workflow execution. The optional `temporal` object overrides connection defaults; omitting it uses localhost:7233, namespace default and a project-specific task queue.

## Local setup

With Docker running and Docker Compose v2 installed, start the local server from the Watts source directory:

```sh
docker compose up -d --wait
```

[compose.yaml](compose.yaml) runs a pinned Temporal CLI image with its development server and Web UI. The API is available at `localhost:7233`, and the UI at [localhost:8233](http://localhost:8233). Both ports are published only on the local machine. The `default` namespace is created automatically, so Watts' default connection settings work without adding a `temporal` object to `watts.json`.

Workflow history is stored in the named Docker volume `watts-temporal-data`, independently of project `.watts` directories. One server can serve multiple Watts projects, each with its own worker queue. This setup is for local development; deployed installations should use a production Temporal service. See the [server command reference](https://docs.temporal.io/cli/command-reference/server).

Manage the server from the Watts source directory:

```sh
docker compose ps
docker compose logs -f temporal
docker compose down
```

`down` stops the server and preserves history. `docker compose down -v` also deletes the database and all locally stored workflow history. Restart with `docker compose up -d --wait`.

From another directory, use `docker compose -f /absolute/path/to/watts/compose.yaml` followed by the same command. Stop any existing service using ports 7233 or 8233 before starting this stack.

You can also install the [Temporal CLI](https://docs.temporal.io/cli/setup-cli) and run it directly, keeping the database outside disposable `.watts` directories:

```sh
temporal server start-dev --db-filename /absolute/path/to/temporal-dev.db
```

Defaults are `localhost:7233`, namespace `default`, and a task queue derived from the absolute project path. An optional `task_queue` overrides the queue name; keep queues dedicated to one project because their workers need the correct filesystem. The current connection supports a local or trusted self-hosted endpoint; TLS and Temporal Cloud authentication are not configured by this first integration.

Rebuild Watts after upgrading the source. Start a worker from inside the project in another terminal:

```sh
watts start
```

Start and inspect a task from another terminal:

```sh
watts task run <task>
watts task status <task>
watts task status <task> --json
```

`run` submits the workflow and returns. The worker continues independently. A repeated `run` reports the existing execution instead of starting duplicate work. The local development Web UI is normally at http://localhost:8233.

Agent credentials must be available to the worker's environment or its private Pi authentication files. Flags such as `--env-passthrough` pass variable names; they do not transmit the invoking shell's credential values to the worker.

## Definition

A `workflow` object in `watts.json` replaces the default SDLC step list. Stable stage names need not be `build` or `review`. A minimal workflow that exercises drafting, human approval and publication without a model is:

```json
{
  "temporal": {"address": "localhost:7233", "namespace": "default"},
  "workflow": {
    "version": 1,
    "max_transitions": 20,
    "steps": [
      {
        "name": "draft",
        "command": "printf 'Proposal\n' > \"$WATTS_TASK/proposal.md\"",
        "outputs": ["proposal.md"]
      },
      {
        "name": "authorize",
        "human": true,
        "inputs": ["proposal.md"],
        "on_failure": "draft"
      },
      {
        "name": "publish",
        "command": "cp \"$WATTS_TASK/proposal.md\" \"$WATTS_TASK/result.md\"",
        "inputs": ["proposal.md"],
        "outputs": ["result.md"]
      }
    ]
  }
}
```

Merge this example into the existing configuration; `watts init --no-install` can initialize a project first. Task creation copies the selected definition to <task>/workflow.json and creates only that workflow's declared templates. The default SDLC includes specification and planning agents with explicit human review gates.

Task-local workflow.json is editable before submission. Temporal freezes its definition at first submission. Each step has exactly one executor:

- `agent`: a configured role plus `prompt`. `{task}` in the prompt expands to the task folder.
- `command`: Bash executed from the project root, with `WATTS_REPO`, `WATTS_TASK` and `WATTS_ROLE` set.
- `check`: a bundled script name with optional `args`.
- `human: true`: waits for an explicit decision over the declared input file hashes. It requires `inputs` and cannot be optional. Put outputs and completion checks in an execution stage, rather than a human stage.

Additional fields:

- `inputs` and `outputs`: regular files relative to the task folder. Missing files, traversal and symlinks escaping the task folder fail validation. `.watts-state` is reserved for orchestration records.
- `checks`: completion checks such as `[{"script":"check-stories","args":["--strict"]}]`. Outputs and checks must pass in addition to a successful executor exit.
- `next`: the next stage on success; omission means the next listed step. `end` finishes the workflow.
- `on_failure`: a named rework stage. Without one, failure waits for an explicit retry. An optional failed stage is recorded as skipped and continues; a failure transition to `end` still fails the workflow.
- `optional`: allows a non-human stage to fail without blocking the normal next transition.
- `timeout_seconds`: activity time limit; otherwise the project's `limits.max_minutes` applies, or one hour if unset. Human decision waits have no expiration in this version.
- `max_attempts`: stage execution budget; otherwise `limits.max_attempts` applies, or three if unset. Exhaustion waits for an explicitly authorized extra attempt.

`max_transitions` bounds total stage executions, including rework. Its default is 100. Workflow schema version 1 supports sequential execution and success/failure transitions; expression conditions and parallel stages are not implemented yet. Task creation supports a `templates` map of artifact paths to initial text.

## Decisions and recovery

When status reports `waiting_approval`, inspect the named input files and submit a decision from a human terminal:

```sh
watts task decide <task> authorize
watts task decide <task> authorize --reject
```

Approval is accepted only if the input hashes still match those recorded when the stage began waiting. Rejection can send the workflow through `on_failure` to regenerate changed inputs. `USER` (or `LOGNAME`) supplies the decision maker's name. Agent identities and a set `WATTS_ROLE` are rejected by the CLI; this local check is not authenticated identity enforcement or a sandbox.

After an execution failure, inspect status and the activity logs before retrying:

```sh
watts task run [task]
watts task run [task] --ignore-attempt-limit
watts task cancel <task>
```

Activities have automatic retries disabled. Workflow rework transitions can run a stage again, preserving all attempts. `resume` retries the current failed stage; an approval wait needs `decide`, an event wait needs `signal`, and an active workflow needs its worker. `retry --ignore-attempt-limit` explicitly authorizes an additional attempt after its budget is exhausted. All task execution uses this engine.


Cancellation reaches worker activities through heartbeats and stops subprocess groups. A project workspace lock prevents concurrent Temporal activities from editing the same project. A process journal detects surviving subprocesses after worker loss and refuses overlapping execution; inspect and stop a reported process group before retrying. Interactive drafting, artifact checks and external editors do not participate in this lock. File edits are not rolled back automatically.

## Stored state

Task creation copies the selected workflow template to editable workflow.json. First submission freezes that definition, runtime configuration and capability commands. Use a new task to adopt a different definition. See [workflows.md](workflows.md) for workflow templates, plugin outcomes and external events.

- `<task>/workflow.json`: authoritative task definition, editable before submission.
- `<task>/.watts-state/approvals/<stage>.txt`: human artifact approval receipts.
- `<task>/.watts-state/temporal.json`: workflow/run IDs, endpoint, queue and the pinned definition.
- `<task>/.watts-state/workflow-config.json`: local runtime settings, verified by content hash on every activity.
- `<task>/.watts-state/logs/<stage>-<attempt>.log`: activity output.
- `<task>/.watts-state/steps/<stage>-<attempt>/input.json` and `result.json`: executor input and validated result evidence.
- `<task>/.watts-state/runs/<stage>-<attempt>.json`: agent provenance, with the attempt number supplied by Temporal.
- `.watts/workflow.lock` and `.watts/workflow-active-process.json`: workspace ownership and active-process recovery.

Temporal owns all progress, attempt counts, retries and completion. Local files contain evidence and execution bindings; they do not infer workflow status from process IDs. The active-process journal is used only to prevent overlapping processes.

Retain the task state directory and the Temporal database. A durable execution record does not replace the project's files, snapshots or credentials. Definitions and command/prompt strings are recorded in Temporal history, so use named credentials rather than embedding secrets in them.

## Verification

Run `go test ./...` for CLI integration tests, application behavior tests, Temporal SDK workflow tests and worker/process tests. Go tests use Testify `assert` and `require`; the `testing` package supplies the test harness. The opt-in live test starts a real development service and downloads a compatible Temporal CLI if needed:

```sh
WATTS_TEMPORAL_INTEGRATION=1 go test ./internal/application -run TestTemporalLive -v
```

## Code organization

The CLI is a terminal adapter: it parses flags, gathers user input, invokes typed application operations and renders their results. Internal packages never import `cli` or `flag`.

- `internal/application` owns task and agent operations, runtime configuration loading, activity execution, SDK connections and worker lifecycle. Each activity gets a concrete runtime containing its pinned configuration, cancellation context, environment and log streams. Agent, command, check and capability executors return `orchestration.Result`, followed by a shared validation and persistence path. `RunOptions` is a typed request independent of CLI flags. Human decisions and Temporal status return structured results. Progress and diagnostic findings are sent to an injected reporter; dry-run presentation remains in the CLI.
- `internal/config` owns the configuration schema, defaults, agent resolution and validation, including the default workflow definition.
- `internal/kit` owns embedded scripts, templates and skills, plus project asset initialization. The source assets live in `internal/kit/kit/`.
- `internal/storage` owns JSON persistence primitives.
- `internal/workspace` owns artifact hashing, exclusive workspace access, command execution and cancellation of subprocess groups.
- `internal/orchestration` owns the workflow schema/protocol, deterministic interpreter, transition selection and validated approval/retry handlers.

Task bookkeeping, provenance and executor coordination remain in the application layer. Terminal formatting stays in `cli`. Configuration, kit, application and orchestration tests live beside their implementations. CLI tests exercise commands and output; the live Temporal integration test belongs to the application package. Architecture tests enforce the dependency direction and prevent direct SDK/process infrastructure imports in the CLI.

Workflow names, activity names and serialized fields remain stable across internal Go refactors so recorded Temporal histories retain their protocol.
