# Run tasks

Build with Go 1.27 or later. Execution requires Temporal, a Watts worker, and
selected workload images on a Docker engine. Compilation requires none of those
services. Images supply agents, toolchains, language servers, and verification.

## Set up

From the Watts checkout:

```sh
make build
make install # or: make install INSTALL_DIR="$HOME/.local/bin" SUDO=
docker compose up -d --wait
docker build -t watts/pi-ralph:0.1.0 registry/pi_ralph
docker build -t watts/coder-golang:0.1.0 registry/coder_golang
wctl compile resources/examples/coder_golang -o /tmp/coding.json
```

`bin/wctl` is the local build. Compile accepts local ACL packages, `-var-file`
and `-var` overrides, and `-o` for a compiled artifact. `wctl c` is an alias.
See [ACL language specification](language-spec.md) and [the coder example](../resources/examples/coder_golang/README.md).

From the application project:

```sh
export OPENROUTER_API_KEY='your-key'
wctl doctor --compiled /tmp/coding.json
wctl start -d
```

Doctor checks the compiled contract's images and required environment sources;
it does not prove Temporal/worker health. `start` runs the project's worker and
web monitor and opens the default browser. Without `-d`, it stays in foreground.
`wctl stop` stops both. Detached logs are `.watts/service.log`. Temporal is
managed separately; see [Temporal operations](temporal.md).

Export declared secret sources before starting the worker. A detached worker
keeps its original environment; restart after credential or binary changes.
Watts never installs Pi/Ralph or toolchains on the host.

## Create and execute

```sh
wctl task new hello-server --compiled /tmp/coding.json
# Edit SPEC.md in the exact task path printed above.
wctl exec tasks/<printed-task-name> --dry-run
wctl exec tasks/<printed-task-name>
```

Or submit immediately with `wctl exec /tmp/coding.json --name hello-server
--request 'Create a minimal Go HTTP server.'`. New execution uses the invocation
directory as the application root, independent of package/artifact location.
Application files belong in that root and normal source directories. Task files
hold workflow documents, evidence, and reports.

Task names use sortable UTC millisecond timestamps, such as
`tasks/2026-10-12t153042-127z-hello-server`; same-name collisions reserve distinct
directories. Each task stores `workflow.json` and package provenance. The artifact
captures all declared assets; execution never rereads the ACL source or modules.
Do not hand-edit compiled artifacts, task definitions, or pinned state. Recompile
source changes and create a new task.

Submission returns before completion. Follow the web monitor or:

```sh
wctl task status <task> --json
```

Status prints available state even when returning nonzero for failure. Preserve
that JSON for an external coordinator. If there is no JSON, report a connection
or infrastructure failure separately from workload failure.

## Decisions and recovery

For the coder example:

```sh
wctl task decide <task> spec-review
wctl task decide <task> plan-review
# To request revisions:
wctl task decide <task> spec-review --reject --feedback 'Specific changes needed'
```

Use actual stages declared by the package. A coordinator needs explicit authority
for human gates; delegated decisions bind to reviewed state and artifact hashes.
See [coordinators](coordinators.md).

A failed stage normally enters `waiting_retry`. Inspect its error, attempt log,
and `.watts-state/steps/<stage>-<attempt>/` evidence. Fix an authorized cause,
then `wctl exec <task>`. `wctl task run <task>` calls the same execution service.
For an exhausted attempt limit, use `--ignore-attempt-limit` only after deliberate
review. Retry uses the pinned definition, images, assets, and runtime preferences;
it does not recompile or start from an arbitrary stage.

Bounded feedback routes count their traversals independently of stage attempts
and container repairs. Exceeding a feedback/global transition limit fails the
workflow terminally. Failed, cancelled, terminated, or timed-out executions need
a new task rather than another retry. `wctl task cancel <task>` cancels the workflow
and stops its active workload; stopping observation does not cancel execution.

## Worker preferences and storage

`wctl config show` displays local operational preferences. `wctl config set`
supports `tasks_dir`, `max_minutes`, and `max_attempts`. Preferences live in
`.watts/settings.yaml`, with optional Temporal overrides. They do not select
agents, providers, or tools. Submitted tasks retain their captured settings.

Package assets are cached under `.watts/packages/`; task evidence lives in
`tasks/`; private candidates, caches, and agent/tool state live under
`~/.watts/runtime/`. The [package/runtime layout proposal](package-runtime-layout.md)
tracks further consolidation and retention.

## Completion report

The worker writes `<task>/report.html` on successful completion, terminal failure,
or graceful cancellation. It is an offline report of actual attempts and rework,
timings including human waits, errors, decisions, artifacts, gate results,
reported token/cost breakdowns, observed/configured models, capabilities, and
run identity. Links address retained evidence files.

Missing usage or cost is unknown; partial figures show measurement coverage.
Totals include failed attempts and rework. Cache counts are separate and are not
added to input + output again. Declared capabilities do not prove invocation,
and completion does not independently establish correctness or business value.

Report writes are atomic/retryable. Failures appear in `report_error`, CLI status,
and worker logs without changing the workload outcome. Keep the worker running
for finalization. Forced Temporal termination cannot run it. Live report updates
and crash accounting remain tracked in wiss-011/wiss-025.
