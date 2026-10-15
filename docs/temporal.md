# Temporal operations

Temporal is required for execution. Its Go SDK owns workflow history, queries,
updates, activity scheduling, and replay. A Watts worker executes container
activities against its local application workspace. The Temporal service does
not launch containers or hold the application's files.

## Local service

From the Watts checkout with Docker running:

```sh
docker compose up -d --wait
```

The development service API is at `localhost:7233`, with its UI at
[localhost:8233](http://localhost:8233). Compose binds these ports locally and
creates the `default` namespace. Preserve its data volume to retain history.
`wctl start` manages the Watts worker and web UI, not the Temporal service.

## Connection

Defaults are `localhost:7233`, namespace `default`, and a project-specific task
queue derived from the project path. Shared `~/.watts/config/settings.json` may override the endpoint and namespace:

```json
{
  "version": 1,
  "tasks_dir": "tasks",
  "runtime_retention": "last-run",
  "limits": {"max_minutes": 30, "max_attempts": 3},
  "temporal": {"address": "localhost:7233", "namespace": "default"}
}
```

Queues remain derived from each project path; global queue overrides are rejected. The current
connection supports a local or trusted self-hosted endpoint. TLS/Temporal Cloud
authentication and distinct-node artifact delivery remain future work.

## Execution and recovery

Task creation resolves a bundle. First submission pins its definition, image IDs,
resource captures, and runtime preferences. Temporal owns ordered routing,
approvals, events, attempts, rework, and completion. A retry retains that plan;
use a new task to change execution policy.

Activities have automatic retries disabled; explicit workflow retry/routing
controls rework. Heartbeats deliver cancellation to running activities. The Docker
backend stops the owned container, reattaches by stable execution identity, and
never implicitly restarts a completed process. Durable acceptance receipts prevent
reexecution after container cleanup. Publication checks baseline content and modes,
so external edits cause explicit conflicts. See [the workload interface](container-workloads.md).

A running workflow waiting for retry can receive `wctl exec <task>`. Completed,
failed, cancelled, and terminated executions are terminal states. Inspect status
before starting a replacement execution; retained local evidence is not a signal
to advance workflow state. Workers and the Temporal service may be restarted;
completed activities and durable waits are recovered from history.

## Stored state

- `<task>/workflow.json`: resolved task snapshot. New package execution compiles current ACL; existing-task execution keeps the captured definition.
- `<task>/.watts-state/temporal.json`: execution binding and pinned definition.
- `<task>/.watts-state/workflow-config.json`: hash-verified runtime preferences.
- `<task>/.watts-state/package/`: task-owned compiled package assets.
- `<task>/.watts-state/resources/`: captured workload resources.
- `<task>/.watts-state/approvals/`: hash-bound approval receipts.
- `<task>/.watts-state/logs/`: durable attempt output.
- `<task>/.watts-state/steps/`: input, result, events, reported usage, container records, and acceptance receipts.
- `~/.watts/runtime/projects/<project-id>/worker/workflow.lock`: publication/activity serialization.
- `~/.watts/runtime/`: private candidates, caches, agent settings, and tool state.

Keep task evidence and the Temporal database. History does not replace application
files or credentials. Definitions and prompt/command strings enter history; use
Secret references instead of embedding credentials. See [storage layout](package-runtime-layout.md)
for settings and runtime retention.

## Verification

```sh
go test ./...
WATTS_DOCKER_INTEGRATION=1 go test ./internal/containerengine ./internal/application
WATTS_TEMPORAL_INTEGRATION=1 go test ./internal/application -run '^TestTemporalLive$' -v
WATTS_TEMPORAL_INTEGRATION=1 go test ./cli -run '^TestCoordinatorCLILifecycle$' -v
```

The Docker integration suite requires local `ubuntu:latest`, the pinned Node base
image, and the built Pi/Ralph image. It includes deterministic container workloads
and real Pi/Ralph against a fake provider. Pull the external fixture images
explicitly and build the Watts images with `make images`; see
[image prerequisites](../registry/README.md#build-locally).

Both Temporal integration tests require local `ubuntu:latest`; the application
test also requires the pinned Node base image. They start isolated persistent
development servers and may download the Temporal CLI. The application test
covers service/worker restart, approval rework, explicit retry,
external events, and cancellation. The public CLI test covers package scaffolding,
direct execution with a large request and mixed-case task name, rejection/rework,
stale decisions, worker restart, retry, completion, and cancellation. See
[coordinator verification](coordinators.md#developer-verification).

These tests use isolated temporary projects and deterministic workloads. They do
not run against your application project or use hosted model credentials.
