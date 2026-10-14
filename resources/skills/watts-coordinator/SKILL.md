---
name: watts-coordinator
description: "Coordinate a Watts task through an external agent using the CLI: draft the request, monitor execution, explain findings, handle authorized decisions, and recover failures. Use when the user wants work performed through Watts."
---

# Coordinate a Watts task

Watts owns workflow execution and state. The coordinator prepares the request, explains recorded progress, brings decisions to the user, and acts within delegated authority. These instructions apply to any agent with command and file access. Use `wctl`; Watts does not launch or configure the supervising agent. Context gathering, including fetching a ticket, uses the user's existing capabilities.

The current interface is the CLI. MCP and wait-for-change are proposed, not available operations. Read [CLI lifecycle](../../../docs/coordinators.md) when running, monitoring, deciding, or recovering a task. If installed without the checkout, use the self-contained instructions below and `wctl help`; do not invent missing tools.

## Select the project and package

- Verify the user's intended application directory with `pwd` and a listing. Set that absolute directory explicitly for every command; a previous `cd` may not persist. If needed, use `cd '/verified/project' && wctl ...` in the same invocation.
- The application project and package/source checkout can be different locations. Compile an external ACL package with `wctl compile /verified/package -o /private/path/coding.json`, or use a provided compiled artifact. Do not infer the project from this skill's location or rely on ancestor discovery.
- Resolve documentation links relative to this skill. After a missing-file error, list the nearest known existing directory instead of guessing alternative paths. There is no required `resources/docs` directory.
- Worker preferences live in `~/.watts/config/settings.json` and affect all projects. Keep task-specific changes in ACL rather than adjusting global settings for one task. Project `.watts/` is not a required setup marker; task evidence remains task-owned.
- Configure agents, models, tools, skills, and workflow policy in ACL/modules and variable files, then compile **before task creation**. Task `workflow.json` is a resolved execution contract. Do not edit compiled artifacts or pinned records to change policy.

## Create the request

1. If the user names an existing task, inspect it. Otherwise run `wctl task new <slug> --compiled <artifact.json>` from the application directory, retain the exact returned task path, and verify it exists beneath the configured task directory before writing.
2. Read the entire task `workflow.json` execution definition. Identify actual stages, inputs, outputs, routing, and approval policy; do not assume Golang coder or fixed names.
3. Preserve existing human content in `SPEC.md`. Draft the requested behavior, scope, constraints, observable acceptance criteria, and unresolved questions using project evidence. A rough request is sufficient; ask only for missing intent. A verified team spec skill or [bundled spec guidance](../../../registry/pi_ralph/kit/skills/spec-template/SKILL.md) may help when available.
4. SPEC.md describes what and why; PLAN.md describes approach and sequence. Do not create REQUIREMENTS.md, implement the application yourself, or bypass workflow-owned refinement/planning. Application changes belong in the project root; task directories hold documents and evidence.

If the user only requested a draft, stop after drafting. A workflow configuration change requires a new task from updated source, with authorization for that additional execution.

## Submit and follow progress

Use `wctl doctor --compiled <artifact.json>` for container/image/secret checks; it does not establish Temporal or worker health. A configured Temporal server and a project worker are needed. `wctl start -d` runs the worker/UI; start it only within the user's authorization and avoid duplicate workers. Export required credential sources before starting; a detached worker does not inherit later shell changes.

```sh
wctl exec <task> --dry-run
wctl exec <task>
wctl task status <task> --json
```

Submission success is not completion. Parse status JSON even if the command returns nonzero: `waiting_retry` and failed/cancelled/terminated workflows still print state. Infrastructure errors can instead produce no JSON; report that uncertainty separately.

Track task, workflow/run IDs, current stage, stage attempts/errors, artifacts, and history. Inspect the relevant `<task>/.watts-state/logs/<stage>-<attempt>.log` and `.watts-state/steps/<stage>-<attempt>/` evidence. Result feedback lives in history; reported usage may be incomplete. Never treat missing measurements as zero or an agent's completion narrative as accepted success.

Report meaningful changes: a new stage/attempt, checks/findings, a decision wait, failure/recovery, or completion. Include task/stage identity, what is known, evidence, and the next action. Avoid repeating unchanged snapshots. Poll with bounded waits appropriate to the execution; quiet output alone does not establish a stall. If the host cannot monitor in the background, say so and provide a checkpoint rather than promising later unsolicited updates.

## Decisions and authority

A request to run a task does not authorize all approval gates. Present the pending artifacts, relevant changes/findings/questions, and intended decision unless explicit delegation already covers it. A coordinator may review its drafted spec under the user's explicit delegation; it must disclose that role. A workload agent inside a Watts stage cannot submit human decisions.

For an authorized coordinator decision, save the `waiting_approval` JSON to a private temporary file **outside the application workspace**, inspect its artifact paths and verify their hashes against the files reviewed, then submit:

```sh
wctl task decide <task> <stage> --actor <coordinator-id> --authorized-by <human-id> --reviewed-status <status-json>
# To request changes, also add: --reject --feedback "Specific requested changes"
```

Use the actual actor and authorizing human labels; do not change USER/LOGNAME or fabricate a signature. Labels record declared provenance, not authenticated identities. Direct human decisions may use `wctl task decide <task> <stage>` without delegation flags.

Delegated decisions are bound to the reviewed execution, placement, attempt, and artifact hashes. If the snapshot is stale or approval files changed, inspect status and evidence again; do not replace the snapshot merely to bypass the check. Rejection follows the workflow route, including any return to drafting/planning.

## Recover, reconnect, and finish

- `not_started`: submit if authorized. `running`: monitor; do not create another execution.
- `waiting_approval`: obtain or apply a covered delegated decision. `waiting_event`: report the required integration; the current CLI cannot deliver arbitrary events.
- `waiting_retry`: diagnose the error, fix only an authorized cause, explain the side effects of retry, and run `wctl exec <task>`. Do not blindly repeat an unchanged failure. Use `--ignore-attempt-limit` only with explicit authorization after inspecting exhaustion.
- Retry uses the pinned definition, images, resources, and settings. It creates another attempt at the current stage, not an arbitrary stage jump or rollback. Updating a tag or ACL source does not change that execution.
- `completed`: inspect `report.html`, report acceptance evidence, published work, checks, rework, available usage, and remaining limitations. `failed`, `cancelled`/`canceled`, `terminated`, and `timed_out`/`timedout`: the execution ended; discuss a new task rather than silently creating one.
- On reconnect, recover the task path and execution IDs from task bindings/status, then read history and current evidence. Match identity before acting and reconstruct pending decisions from Watts, not old conversation notes. No event cursor/catch-up API exists yet.
- If asked to cancel, run `wctl task cancel <task>` and confirm the resulting state. Stop observing separately from canceling execution; a disconnected coordinator does not cancel Watts.

Use the public CLI. Never edit `.watts-state` or send ad hoc Temporal updates to advance the task. Explain unavailable state as unavailable, and preserve the user's task scope throughout recovery.
