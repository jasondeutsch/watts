# Package, task, and worker storage

Implemented for local execution in wiss-028. Authored packages, durable task
records, and disposable worker files have separate owners and locations.
Remote artifact storage and distributed workers remain future work.

## Authored package

```text
project/
  main.go
  go.mod
  watts/
    watts.acl
    values.wvars      # optional
    modules/
    resources/
```

`watts/` is a recommended authoring convention, not a required application-root
marker. Compile a local or external package; new task creation uses the invocation
directory as the application workspace. Compilation never creates worker state.
Local modules compose resources, and declared assets are embedded in the artifact.

One package may contain multiple named workflows. Its `entrypoint` defines the
default. Select a different named workflow without editing it:

```sh
wctl compile ./watts -workflow coding -o /tmp/coding.json
```

Each compiled artifact contains one resolved workflow. Names, not filenames,
select workflows. Agent, Procedure, MCP, ConfigMap, and Secret declarations remain
package-owned. See the [language specification](language-spec.md).

## Durable task records

The default task directory is `tasks/`, separate from `watts/`:

```text
tasks/<timestamp>-<slug>/
  SPEC.md
  PLAN.md
  workflow.json
  report.html
  .watts-state/
    package/          # captured package assets
    resources/        # resource snapshot pinned at submission
    approvals/
    logs/
    steps/
    temporal.json
    workflow-config.json
```

Documents, compiled definitions, approval bindings, results, sanitized evidence,
and reports belong to the task. Temporal retains orchestration history; task
records retain application evidence. Neither replaces the other. Runtime cleanup
never removes task directories or changes published application files.

## Worker settings and bookkeeping

Worker preferences live at `~/.watts/config/settings.json`. They apply to all
projects using that home directory; submitted tasks keep their frozen settings.
The file is optional: missing settings use defaults. Agent/model/tool configuration
belongs in ACL. Project queues are derived from the canonical application path;
global task-queue overrides are rejected to prevent workers consuming another
project's work. Temporal address and namespace remain configurable.

```sh
wctl config show
wctl config set runtime_retention last-run
# To preserve all execution runtime files:
wctl config set runtime_retention all
```

`runtime_retention` is the only new setting. `last-run` is the default. Existing
preferences such as `tasks_dir` and execution-limit defaults remain available.
Prefer ACL for workflow-specific limits rather than global worker defaults.

```text
~/.watts/
  config/settings.json
  runtime/projects/<project-id>/
    project.json
    worker/
      service.json
      service.lock
      service.log
      workflow.lock
    tasks/<task-id>/
      task.json
      ordinal.json
      retention.lock
      executions/<execution-id>/
        execution.json
        execution.lock
        captures/
        attempts/<attempt-id>/
          inputs/
          outputs/
          private/
          workspace/
```

Identifiers are hashes of canonical project and execution identities. The worker
registers the workspace at task creation or startup; commands from descendants
find the nearest registered project. `watts/watts.acl` and durable tasks provide
additional discovery boundaries. Project `.watts/` is no longer created or used
for discovery, settings, service records, locks, or package assets.

## Host and container paths

Host state is rooted at the host user's `$HOME/.watts/`. HOME must be an existing,
absolute user directory. Filesystem-root HOME and container-only `/runtime/...`
values (including symlink aliases) are rejected before host state is created.
A non-default home is supported; no fallback writes to `/runtime` or `/` occur.

| Purpose | Worker host | Inside a workload container |
| --- | --- | --- |
| Preferences | `$HOME/.watts/config/settings.json` | Not mounted |
| Private attempt state | `.../attempts/<id>/private/` | `/runtime/` |
| Workload HOME | Private attempt's `private/home/` | `/runtime/home` |
| Temporary executables | Private attempt's `private/tmp/` | `/runtime/tmp` |
| Pi example's task copy | Private attempt's `private/task/` | `/runtime/task` |
| Captured inputs | Private attempt's `inputs/` | `/inputs/` (read-only) |
| Declared output staging | Private attempt's `outputs/` | `/outputs/` |
| Application candidate | Private attempt's `workspace/` | `/workspace/` |

`...` above is under `$HOME/.watts/runtime/projects/<project-id>/tasks/<task-id>/executions/<execution-id>/`.
The worker never mounts the whole host home into a workload. Container HOME/TMPDIR
are explicit launch inputs, separate from the worker's environment. The workload
root filesystem remains read-only; private runtime/output mounts are writable.
Host mount sources must already exist, be absolute directories, and must not be
container-only paths. Attempt paths reject symlink redirection before preparation.

Diagnostics distinguish the process that observed an error:

- `host filesystem: <operation> "<path>" (<category>)`: a local Watts OS operation.
  Read-only filesystem and permission-denied errors are separate categories.
- `Docker runtime: create/start container`: Docker daemon/OCI startup diagnostics.
- `container <phase> exited ...`: output from the workload process.

Use the reported operation/path and execution context to investigate. For host
errors, check the worker's HOME and access to that specific filesystem. A
read-only filesystem is not an ownership problem. Changing container paths or
permissions does not repair a host filesystem error. Workload paths do not need
to exist at filesystem root on the Mac host.

## Retention and recovery

`last-run` retains **every attempt of the latest execution per task**, plus older
active or recovery-required executions. It does not retain only the latest task
or stage, nor delete all but the latest execution across the worker.

A durable ordinal assigns execution order on first acquisition. Restarting an
older execution does not make it latest. Cleanup runs at worker startup and after
successful persistence of a terminal task report. A nonterminal approval, event,
or retry wait does not make an execution eligible.

An older execution is removable only when it is terminal, every attempt's
required evidence has been retained, and no execution lease is active. The worker
syncs its attempt log before marking evidence ready. Failed container cleanup,
uncertain publication, missing/corrupt records, and interrupted capture preserve
runtime files for inspection and recovery. This can exceed the configured limit;
there is no forced age-based deletion or automatic resolution of ambiguous state.

Cleanup is confined to the managed runtime tree and rejects symlink redirection.
It does not touch settings, authored packages, task documents, approvals, reports,
published artifacts, or durable memory. Operator-maintained, uncatalogued runtime
files are not silently removed. `all` disables execution-runtime pruning; service
log rotation remains separate work under wiss-006.

## Workload ownership

Agents own their tools and private state inside the attempt's execution
environment. Pi/Ralph is an example image, not a core storage abstraction.
Deterministic Procedures use the same lifecycle. Container isolation is not a
claim of hardened multi-tenant sandboxing. See [container workloads](container-workloads.md)
and the [stage communication proposal](stage-communication.md).
