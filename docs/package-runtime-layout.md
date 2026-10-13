# Package, task, and worker storage

Status: proposed layout, tracked in [wiss-028](../issues.md#wiss-028-separate-watts-packages-task-executions-and-worker-runtime). These paths are not implemented setup instructions. Current paths and commands remain documented in the [runbook](runbook.md) and [Temporal guide](temporal.md).

## Watts package

A project's `watts/` directory is its Watts package, analogous to a repository's `helm/` chart directory:

```text
project/
  main.go
  go.mod
  watts/
    watts.acl
    values.wvars       # optional
    modules/
    resources/        # authored skills, scripts, and other assets
```

`watts.acl` describes package identity and composition. Typed variables customize reusable ACL modules; compilation resolves their resources and artifact connections. The package contains authored workflow design, not tasks, execution output, worker configuration, or tool-private state. Local modules compose packages; compiled artifacts capture their inputs for execution. The application's workspace is the project root, independent of the package's location.

One Watts package may contain multiple named Workflows and their supporting Agents, Procedures, ConfigMaps, and Secrets. Select a Workflow explicitly or through a declared default; filenames have no semantic meaning. No additional grouping declaration is needed. ACL package entrypoints select the workflow for each compiled artifact.

## Tasks and durable execution evidence

Tasks contain SPEC.md and PLAN.md. Execution records preserve the pinned plan, approval bindings, attempts, logs, usage, results, verification evidence, and published artifacts. These records are separate from package authoring and disposable worker files.

The task/evidence storage location remains an open implementation decision. Do not assume tasks belong inside `watts/` or that home-directory runtime storage is their durable store. Temporal owns orchestration progress; published evidence and workspace revisions must remain available independently of worker cleanup.

## Worker configuration and runtime

Machine-level worker settings live at `~/.watts/config/settings.json`. Host execution environments and worker bookkeeping live under `~/.watts/runtime/`, isolated by stable project, task, execution, stage, and attempt identities. Worker settings do not belong in portable package manifests.

Only runtime retention needs a new setting for this scope; its exact key and value syntax are not yet defined. The default retains **the latest execution per task**, including all of its attempts. It does not retain only the most recent execution across the worker.

Cleanup preserves active executions and anything required for recovery, even if that exceeds the retention limit. Older runtime files may be removed only after required evidence is durably published. Task documents, approvals, reports, and published artifacts are outside this policy. Execution ordering and ownership must be explicit rather than inferred from filesystem timestamps.

## Execution environment ownership

The target execution primitive is an isolated workload; see the [stage communication proposal](stage-communication.md#isolated-workload-primitive). Host directories initially provide its storage boundary, not a security sandbox. Containers or sandboxes will enforce access and resource boundaries.

An Agent owns its runtime and tools: Pi may implement that runtime, and Ralph may be a tool used by Pi. Their private state belongs inside the attempt's execution environment. Neither `.ralph-runner` nor Pi-specific configuration should leak into the application workspace or become an orchestration dependency. Deterministic Procedures use the same generic lifecycle without requiring Pi, Ralph, or a model.

Moving execution files does not move the application workspace. Build writes a candidate workspace; review inspects the specified revision and publishes reports separately. Adapter-owned evidence must be published before the environment is removed.

Container/sandbox execution is tracked in [wiss-013](../issues.md#wiss-013-containerized-workloads); recovery and publication are covered by [Temporal integration issues](../issues.md#orchestration-and-recovery).
