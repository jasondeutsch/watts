# Run tasks

Build Watts with Go 1.27 or later. Execution requires a container execution
environment, available workload images, and a running Temporal service. Currently,
Watts supports a local Docker Engine. Podman and Kubernetes are potential future
backends; see [execution backends](container-workloads.md#execution-backends-and-current-limits).
Node, Pi, Ralph, language servers, and quality-gate tools belong in selected images.

## Set up

From the Watts checkout:

```sh
make build
docker compose up -d --wait
docker build -t watts/pi-ralph:0.1.0 registry/pi_ralph
docker build -t watts/coder-golang:0.1.0 registry/coder_golang
```

Use `make install` for `/usr/local/bin/wctl`, or
`make install INSTALL_DIR="$HOME/.local/bin" SUDO=`. Building alone produces
`bin/wctl`. Temporal is managed separately; see [Temporal operations](temporal.md).

From the application project, create or select a workflow package:

```sh
wctl template new team-workflow
# Customize values and templates; build its selected images.
export OPENROUTER_API_KEY='your-key'
wctl doctor --package ./team-workflow/watts.yaml
wctl start -d
```

`start` runs the project's worker and web UI and opens the default browser.
Without `-d` it remains in the foreground. `wctl stop` stops both; detached logs
are in `.watts/service.log`. Export credential sources before starting the worker.
Restart the service after changing its environment or rebuilding Watts.

See the [Golang coder example](../resources/examples/coder_golang/README.md) for
a fuller coding workflow. A root package descriptor lets you omit `--package`;
other descriptor filenames are selected explicitly and carry no special meaning.

## Configure the project

A package descriptor names the package, its version, values, templates, assets,
and optional local base package. Agent software, models, MCPs, skills, and checks
are manifest policy, usually supplied through values. See [templating](templates.md)
and [Kinds](kinds.md). There is no initialization or host agent-install command.

Local operational overrides remain in `.watts/settings.yaml`: `tasks_dir`,
`snapshot_exclude`, `limits.max_minutes`, `limits.max_attempts`, and Temporal
connection settings. Use `wctl config show` or `wctl config set <key> <value>`;
supported set keys are `tasks_dir`, `max_minutes`, and `max_attempts`.
The home-directory settings/retention consolidation remains a separate proposal.

## Create and run

```sh
wctl task new hello-server --package ./team-workflow/watts.yaml
```

Task creation uses the invocation directory as the application root when a
package is explicitly selected. It does not implement application code. Use the
printed task path. Names use a sortable UTC timestamp with millisecond resolution,
for example `tasks/2026-10-12t153042-127z-hello-server`. Concurrent requests for
the same slug reserve distinct directories. In a coding package, write your request or rough notes in
`SPEC.md`; agents refine the spec and create `PLAN.md` as declared by the workflow.
Customize workflow policy in the package's source values/templates before task creation.
The task's `workflow.yaml` is a resolved snapshot; do not edit rendered output.
To adopt a changed configuration, create a new task from the updated package.

```sh
wctl task run <task>
wctl task status <task>
wctl task status <task> --json
```

Submission returns before execution finishes. The worker captures image IDs,
resources, and runtime preferences. Later template or image-tag changes do not
change that pinned execution. Container work happens in a private candidate
workspace. Validated application changes are published to the project root;
task folders hold documents and evidence. See [the workload contract](container-workloads.md).

## Review and recovery

Inspect a pending approval's documents, then:

```sh
wctl task decide <task> spec-review
wctl task decide <task> plan-review
wctl task decide <task> spec-review --reject --feedback 'Clarify the error cases'
```

External supervising agents use explicit delegation and a reviewed status snapshot; see [coordinator decisions](coordinators.md#delegated-decisions).

Approvals are bound to artifact hashes, placement, and attempt. Rejection follows
the workflow's route; it does not silently approve changed documents.

If a stage fails, inspect the web UI or task status and its attempt log under
`<task>/.watts-state/logs/`. Container exit/error records, workspace hash manifests, result/event evidence, and
reported usage are under `.watts-state/steps/<step>-<attempt>/`. Missing provider
usage remains unknown. Private runtime data is in `~/.watts/runtime/`.

Fix the cause, then retry the failed stage:

```sh
wctl task run <task>
# Only after inspecting an exhausted attempt budget:
wctl task run <task> --ignore-attempt-limit
```

Retry creates a new attempt using the pinned plan. It preserves earlier evidence
and completed workflow stages. To adopt different images, commands, or workflow
policy, create a new task. Restore conflicting publication files to the expected
state or inspect the retained candidate before deciding how to recover.

```sh
wctl task cancel <task>
```

Cancellation stops the active owned container. A terminal cancelled, terminated,
or failed workflow is distinct from a running workflow waiting for explicit
retry. See [Temporal](temporal.md) for restart and execution-state details.

## Troubleshooting

- Missing image: build or pull the exact image on the worker's Docker engine.
- Missing secret: export its named source, then restart the worker. Credentials are not copied from interactive Pi settings.
- Provider error: inspect the retained error event and attempt log; zero process exit cannot override an error event.
- Missing result/output: fix the image's contract implementation or package declarations.
- Publication conflict: a live project file changed since capture. Watts preserves it and retains the candidate.
- Docker permission or mount failure: verify the worker can access its local engine and attempt filesystem.
- Temporal unavailable: start the service and inspect the configured endpoint/namespace/queue.

## Completion report

When a workflow closes successfully, fails terminally, or is gracefully cancelled,
the worker writes `<task>/report.html`. Open it in a browser; it is a self-contained,
offline HTML report with links to retained task evidence.

The report contains the actual attempt sequence (including feedback loops and
retries), timing including human waits, errors, decisions, artifact hashes, gate
results, token and cost breakdowns by stage and attempt, and run/configuration
identity. It distinguishes observed models from configured models and lists
available tools, MCP services, skills, and pinned container images. Declared
capabilities do not prove invocation; workflow completion does not replace an eval.

Token totals include failed attempts and rework. Missing counters or costs are
**unknown**, and partial totals show measurement coverage. Cache counts appear
separately and are not added to input + output again. Raw logs and credential
configuration are not embedded. Evidence links work while their files are retained.

Report writes are atomic and retryable. A report write failure appears as
`report_error` in workflow state and in worker logs, without changing the workload
outcome. The worker must remain running for finalization. Forced Temporal
termination cannot run this activity, and already closed tasks are not backfilled.
Reports are currently final snapshots; live report refresh is tracked in wiss-011.
