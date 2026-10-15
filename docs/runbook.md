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
make images
```

`make images` builds and loads both local workload images through a dependency
graph. It requires Docker Buildx and the context-owned Docker driver; see
[image build requirements and diagnostics](../registry/README.md#build-locally).
No published Watts registry is required.

`bin/wctl` is the local build. Compile accepts local ACL packages, `-var-file`
and `-var` overrides, `-workflow` selection, and `-o` for a compiled artifact. `wctl c` is an alias.
See [ACL language specification](language-spec.md) and [the coder example](../resources/examples/coder_golang/README.md).

From the application project:

```sh
wctl new ./watts --from /path/to/watts/resources/examples/coder_golang
# Edit watts/values.wvars for the team and each Agent.
export OPENROUTER_API_KEY='your-key'
wctl start -d
```

`wctl new` creates watts.acl, typed variables.acl, editable values.wvars, and a
README, importing the source through an explicit relative module path. The parent
must exist and the destination must not exist. It does not copy the dependency,
install images, or start services. Keep the module available for compilation or
copy it into the repo and update its source path deliberately.

For separate preflight, `wctl compile ./watts -o /tmp/coding.json` followed by
`wctl doctor --compiled /tmp/coding.json` checks images and required environment sources;
it does not prove Temporal/worker health. `start` runs the project's worker and
web monitor and opens the default browser. Without `-d`, it stays in foreground.
`wctl stop` stops both. Detached logs live under `~/.watts/runtime/projects/<project-id>/worker/service.log`;
startup prints the exact path. Temporal is
managed separately; see [Temporal operations](temporal.md).

Export declared secret sources before starting the worker. A detached worker
keeps its original environment; restart after credential or binary changes.
Watts never installs Pi/Ralph or toolchains on the host.

## Create and execute

With the project's worker running:

```sh
wctl exec ./watts --name 'Hello server' --request-file request.md
# Or provide text directly:
wctl exec ./watts --name 'Hello server' --request 'Create a minimal Go HTTP server.'
# Or use stdin explicitly:
cat request.md | wctl exec ./watts --name 'Hello server' --request-file -
```

Choose one request source; `--request` and `--request-file` are mutually exclusive.
File/stdin contents are preserved exactly, including newlines, without shell
argument-size limits. A missing, unreadable, or empty request file is an error.
Watts reads stdin only when explicitly selected with `--request-file -`.

`exec <package>` compiles and validates ACL before creating/submitting a new task.
It accepts the same `--workflow`, `--var-file`, and `--var` flags as compile.
`--dry-run` prints the resolved graph without creating a task. Explicit
`wctl compile ... -o /tmp/coding.json` and `wctl exec /tmp/coding.json --name ...`
remain useful for inspecting/distributing a frozen contract. For draft-only work,
use `wctl task new <name> --compiled /tmp/coding.json`, then edit SPEC.md and
`wctl exec <printed-task-path>`.

New tasks use the invocation directory as the application root, independent of
package/artifact location. The selected package/workflow and created task path
are printed at submission. Application code belongs in the project root; tasks
hold documents, evidence, and reports.

Task names accept ASCII letters (including uppercase), digits, spaces,
underscores, and dashes. New names normalize to lowercase with single dashes;
`Improve CSS` becomes `improve-css`. Punctuation/path separators are rejected.
The resulting slug must have 1–100 characters. UTC millisecond prefixes order
tasks; collisions reserve distinct directories. Existing task paths keep their
original spelling and are never normalized during lookup or retry.

Every task captures its resolved workflow, assets, and provenance. To retry,
`wctl exec <task-folder>` uses those captured inputs; source-selection/request
flags are rejected for existing tasks. Source edits require a new task and are
never silently compiled into a retry. Do not edit captured definitions or state.

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

`wctl config show` displays machine-level operational preferences from
`~/.watts/config/settings.json`. `wctl config set` supports `tasks_dir`,
`max_minutes`, `max_attempts`, and `runtime_retention`. Settings are shared by
projects; submitted tasks retain captured settings. Agents, providers, tools,
and workflow-specific limits belong in ACL.

```sh
wctl config set runtime_retention last-run # default: every attempt of latest run per task
wctl config set runtime_retention all      # preserve every run's working files
```

Task-owned package assets and evidence live under `<task>/.watts-state/`.
Worker records, logs, locks, private candidates, and tool state live under
`~/.watts/runtime/`. Host filesystem diagnostics identify the operation, path,
and error category. Container `/runtime/task` is a path inside a private mount,
not a directory to create at the Mac filesystem root. See the
[host/container path mapping](package-runtime-layout.md#host-and-container-paths). No project `.watts/` is created. Retention preserves active
and recovery-required state and never deletes task records. See
[storage layout](package-runtime-layout.md).

## Completion report

The worker writes `<task>/report.html` on successful completion, terminal failure,
or graceful cancellation. It is an offline report of actual attempts and rework,
timings including human waits, errors, decisions, artifacts, gate results,
reported token/cost breakdowns, observed/configured models, capabilities, and
run identity. Its dashboard charts stage time and token use, separates approval
waits and repeat attempts, and measures accepted workspace output against token
use. Complexity and other scoped scores come from workload measurements. See
[task report metrics](task-reports.md). Links address retained evidence files.

Missing usage or cost is unknown; partial figures show measurement coverage.
Totals include failed attempts and rework. Cache counts are separate and are not
added to input + output again. Declared capabilities do not prove invocation,
and completion does not independently establish correctness or business value.

While Temporal is still recording the final report, status shows `finalizing`.
Wait for a confirmed terminal state before stopping the worker. Report writes are atomic/retryable. Failures appear in `report_error`, CLI status,
and worker logs without changing the workload outcome. Keep the worker running
for finalization. Forced Temporal termination cannot run it. Live report updates
and crash accounting remain tracked in wiss-011/wiss-025.

## Credentials and private evidence

Use ACL Secret bindings for credentials. New tasks and worker startup add ignore
rules for `.watts/` and `.watts-state/`; existing tracked files require explicit
cleanup. Share sanitized reports, not whole runtime directories. See
[secret protection](secret-protection.md) for image rebuilding, export limits,
and handling an existing leak.
