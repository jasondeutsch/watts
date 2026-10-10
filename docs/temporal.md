# Temporal workflows

Watts uses Temporal for durable execution of a configurable workflow. The Go SDK is compiled into Watts. A Temporal Service stores workflow history, and a Watts worker runs activities against your local project files. The service does not run Pi or hold a copy of the project.

Temporal is required for every workflow execution. The optional `temporal` object overrides connection defaults; omitting it uses localhost:7233, namespace default and a project-specific task queue.

## Local setup

With Docker running and Docker Compose v2 installed, start the local server from the Watts source directory:

```sh
docker compose up -d --wait
```

[compose.yaml](../compose.yaml) runs a pinned Temporal CLI image with its development server and Web UI. The API is available at `localhost:7233`, and the UI at [localhost:8233](http://localhost:8233). Both ports are published only on the local machine. The `default` namespace is created automatically, so Watts' default connection settings work without adding a `temporal` object to `watts.yaml`.

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

## Worker and recovery

Start Watts from the project with `watts start` or `watts start -d`. Each project currently needs a worker with access to its files and dedicated task queue. A worker polls Temporal and executes scheduled activities; the Temporal server stores history and does not execute agent processes. Stopping a worker preserves workflow state, but work requires a worker to continue.

See the [runbook](runbook.md) for startup, credentials, approvals, retries, cancellation, and diagnosis. See [Kinds](kinds.md) and [workflow execution](workflows.md) for authoring. Activities have Temporal automatic retries disabled; workflow routing and explicit retries control rework.

Cancellation reaches activities through heartbeats and stops subprocess groups. A project workspace lock serializes mutating activities. A process journal detects surviving subprocesses after worker loss and refuses overlapping execution; inspect and stop the reported process before retrying. Interactive agent sessions and external editors do not participate in this lock. File edits are not rolled back.

## Stored state

Task creation resolves the selected resource bundle into an editable task `workflow.yaml`. First submission freezes that definition, runtime configuration and capability commands. Use a new task to adopt a different definition. See [workflows.md](workflows.md) for workflow templates, plugin outcomes and external events.

- `<task>/workflow.yaml`: authoritative task definition, editable before submission.
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

## Implementation

The official Temporal Go SDK handles connections, workers, workflow queries and updates, and activity scheduling. See [workflow architecture](workflow-design.md) for package responsibilities. Workflow/activity names and serialized execution fields are stable protocol elements for recorded history.
