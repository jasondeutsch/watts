---
name: watts-coordinator
description: Guide a user through creating, drafting, running, monitoring, and recovering a Watts task using the CLI and task files. Use when asked to coordinate work through Watts.
---

# Coordinate a Watts task

Use Watts to execute the user's task through its declared workflow. You coordinate the task; Watts and its worker execute the stages. This skill applies to an interactive agent with filesystem and command access, regardless of its model or agent application.

Start from the user's request, such as “create a hello-world server in Go.” Additional context is optional. Gathering that context is handled by the user or their agent's existing capabilities.

## Locate files before reading them

The target project and the Watts source repository are different locations. The target project contains `watts.yaml` and the task files; it need not contain the Watts repository or its resource directory.

- Use the supplied project path and confirm its directory contents. Do not derive it from an assumed home directory or from the location of this skill.
- Resolve relative resource links against the directory containing this `SKILL.md`, not the shell's working directory.
- Use directory listings and exact returned filenames to discover files. Preserve spelling, case, underscores, and absolute-path prefixes.
- After a missing-file error, list the nearest known existing directory. Do not retry guessed spellings or add invented directories. If the file is absent, explain that once and continue with the available task context when possible.

When reading this skill from a Watts source checkout, these paths are relative to this skill's directory:

| Resource | Exact relative path |
| --- | --- |
| Toolbox index | [../../readme.md](../../readme.md) |
| Spec drafting skill | [../../../internal/kit/kit/skills/spec-template/SKILL.md](../../../internal/kit/kit/skills/spec-template/SKILL.md) |
| Task runbook | [../../../runbook.md](../../../runbook.md) |
| Workflow documentation | [../../../workflows.md](../../../workflows.md) |

There is no `resources/docs/README.md`. The toolbox index is `resources/readme.md` in the Watts checkout. Reading that index is optional, not a prerequisite for creating a task.

If this skill was installed separately, checkout-relative links may be unavailable. For an initialized project, use `watts config show --resolved` to discover agent directory paths; an installed spec skill is at `<agent-path>/skills/spec-template/SKILL.md`, relative to the project root. Confirm that file exists before reading it. If no spec skill is available, use the scaffolded `SPEC.md` and the drafting guidance below instead of searching repeatedly for repository docs.

## Keep the task explicit

- Work from the intended project root containing `watts.yaml`.
- Record its verified absolute path as the target project. Give every command invocation that directory explicitly through the execution tool's working-directory option. A `cd` in a previous tool call may not persist.
- If the execution tool cannot set a working directory, use `cd '/absolute/project/path' && watts ...` in the same command invocation. Do not run `cd ..` to locate documentation; read documentation by its absolute path instead.
- Before creating a task, confirm `pwd` matches the intended project and read that directory's `watts.yaml`. Watts searches ancestors for a project, so running from the wrong directory can select a different project. Do not rely on that search to select the target.
- Keep the exact task path returned by task creation throughout the session. Pass it explicitly to every task command.
- Read the task's `workflow.yaml`; do not assume the default stages or artifact names when a workflow is customized.
- Use the public CLI. Do not edit `.watts-state` or send ad hoc Temporal updates to advance stages.
- Never claim execution, approval, or completion from an agent's narrative alone. Confirm with task status and evidence.

## Create and draft

1. Confirm that the user's request identifies the intended behavior. A rough request is sufficient; ask for missing intent rather than inventing a task.
2. Run `watts task new <slug>` with the target project as the explicit working directory and retain the reported path. Verify that the task folder and `workflow.yaml` exist beneath the target project's configured `tasks_dir` before writing anything. Resolve reported relative paths against the target project, never against the Watts source checkout or an unrelated current directory. If the task was created elsewhere, stop and report the mismatch; do not create more tasks or move files silently. If the user names an existing task, inspect it instead of creating another.
3. Read the scaffolded `SPEC.md` and preserve any existing human content. Use the spec skill located by the instructions above when available. Draft from the request and relevant project evidence: state the intent, scope, non-goals, constraints, numbered observable acceptance criteria (`- AC-1: ...`), and unresolved questions. Separate facts from assumptions; do not invent requirements. Use the scaffold as a structure reference, never as a replacement for existing human input.
4. Keep what and why in `SPEC.md`; implementation approach and sequencing belong in `PLAN.md`. Record assumptions and unresolved questions explicitly. Do not create a separate requirements document.
5. If the user requested a different workflow, customize the task's `workflow.yaml` before submission. Do not change workflow policy merely to bypass a failure or approval gate.

Do not start execution when the user asked only for a draft. When execution is authorized, continue below.

## Start and monitor

```sh
watts task run <task> --dry-run
watts task run <task>
watts task status <task> --json
```

The dry run previews submission. A successful `task run` submits or requests a retry and returns; it does not mean the work has completed. A configured Temporal server and a project worker must be available. If the worker is absent, explain that `watts start` must run from the project root in a separate long-lived session; do not launch duplicate workers or change infrastructure without authorization.

Read `state.status`, `state.current`, `state.stages`, and `state.history` from status JSON. Use each stage's `last_error`, attempt count, result feedback, and artifacts to explain progress. Workflow and run IDs identify the execution.

Stage logs are in `<task>/.watts-state/logs/<stage>-<attempt>.log`. Inspect the relevant attempt, not another task's agent output. The optional Watts web monitor shows status, workflow routes, history, and log tails. Poll at reasonable intervals while the user asks you to monitor; do not busy-loop or infer failure from quiet model output.

If status cannot be queried, report that uncertainty. Check connectivity and worker availability before assuming the workflow failed.

## Respond to the execution state

| State | Coordinator action |
| --- | --- |
| `not_started` | Submit with `task run` if authorized. |
| `running` | Monitor. Do not start another execution. |
| `waiting_approval` | Show the human the pending stage's declared inputs, changes, and open questions. Obtain their decision. |
| `waiting_retry` | Diagnose the error, fix the authorized cause, and retry the current stage. |
| `waiting_event` | Explain which integration or event is needed. The MVP CLI cannot deliver arbitrary events. |
| `completed` | Summarize the outcome and evidence; mention unresolved limitations. |
| `failed`, `cancelled`/`canceled`, `terminated`, or `timed_out`/`timedout` | The execution has ended. Explain the cause; the CLI cannot resume it. Discuss a new task with the user rather than creating one silently. |

A custom workflow may route a failure or rejected outcome directly to another stage. Follow status and the submitted workflow's routing; do not impose a separate coordinator loop.

## Human review

After the human authorizes the specific decision, submit it:

```sh
watts task decide <task> <stage>
watts task decide <task> <stage> --reject --feedback "Specific changes requested by the human"
```

Never approve your own work, fabricate a human signature, change identity variables to bypass checks, or interpret a request to run the task as blanket approval of future gates. The user may enter the command themselves. An agent running inside a Watts stage must not act as the human reviewer.

Approval is tied to the reviewed artifact hashes. If the reviewed document changes while waiting, explain the hash mismatch and the need for a fresh review attempt; do not alter the recorded hashes. In the default SDLC, rejecting spec review returns to specification, and rejecting plan review returns to planning with the supplied feedback.

## Diagnose and retry

1. Inspect status JSON, the failed attempt's log, and affected inputs or outputs.
2. Explain the concrete cause and distinguish a stage failure from an ended execution.
3. Fix only what the user has authorized. Preserve task intent and earlier valid work.
4. For `waiting_retry`, run `watts task run <task>`, then check status again.
5. If the attempt limit is reached, explain why another attempt is justified and obtain authorization before using `watts task run <task> --ignore-attempt-limit`.

Retry reruns the current stage with current files, increments its attempt count, and retains the same execution. It does not roll back file changes or offer an arbitrary jump to a chosen stage. Avoid blind retries of an unchanged failure, especially when the stage has external side effects.

Workflow definitions and runtime settings are frozen at submission. Editing the task's workflow or changing project model settings does not change the submitted execution. Do not rewrite pinned records to force a change.

Cancellation is available with `watts task cancel <task>` when the user asks to stop the task. Confirm the resulting status and report any incomplete work.

## Report clearly

Tell the user which task and stage are active, what happened, what evidence supports it, and what action is needed next. Distinguish submitted, running, waiting, and completed. Never claim that a test passed unless its actual result supports that claim.


Application source and tests belong to the project rooted at watts.yaml. Task folders hold SPEC.md, PLAN.md, workflow.yaml, logs and evidence. Do not scaffold a separate application inside a task folder. Ensure plan quality gates target the project root.
