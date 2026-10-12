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
queue derived from the project path. Local `.watts/settings.yaml` may override:

```yaml
version: 1
tasks_dir: tasks
limits: {max_minutes: 30, max_attempts: 3}
temporal:
  address: localhost:7233
  namespace: default
  task_queue: my-project
```

Keep queues dedicated to the project whose files the worker owns. The current
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

A running workflow waiting for retry can receive `wctl task run <task>`. Completed,
failed, cancelled, and terminated executions are terminal states. Inspect status
before starting a replacement execution; retained local evidence is not a signal
to advance workflow state. Workers and the Temporal service may be restarted;
completed activities and durable waits are recovered from history.

## Stored state

- `<task>/workflow.yaml`: resolved task snapshot; author changes in source values/templates before creating a new task.
- `<task>/.watts-state/temporal.json`: execution binding and pinned definition.
- `<task>/.watts-state/workflow-config.json`: hash-verified runtime preferences.
- `<task>/.watts-state/resources/`: captured workload resources.
- `<task>/.watts-state/approvals/`: hash-bound approval receipts.
- `<task>/.watts-state/logs/`: durable attempt output.
- `<task>/.watts-state/steps/`: input, result, events, reported usage, container records, and acceptance receipts.
- `.watts/workflow.lock`: publication/activity serialization.
- `~/.watts/runtime/`: private candidates, caches, agent settings, and tool state.

Keep task evidence and the Temporal database. History does not replace application
files or credentials. Definitions and prompt/command strings enter history; use
Secret references instead of embedding credentials. Runtime retention and the
consolidated home-directory settings layout remain tracked in [issues](../issues.md).

## Verification

```sh
go test ./...
WATTS_DOCKER_INTEGRATION=1 go test ./internal/containerengine ./internal/application
WATTS_TEMPORAL_INTEGRATION=1 go test ./internal/application -run TestTemporalLive -v
```

The container tests require locally available Ubuntu the pinned Node base image, and the built Pi/Ralph image.
They run real Pi/Ralph against a deterministic fake provider. The live test starts
an independent persistent Temporal development service; it may download its CLI.
It covers service/worker restart, approval rework, explicit retry with an ordinary
Node image, external events, and cancellation. It does not use your test project
or hosted model credentials.
