# Drive Watts through an external agent

A user can coordinate Watts through an agent such as Claude Code or another application with shell and file access. Watts owns workflow state, execution, checks, routing, and artifact publication. The supervising agent drafts the initial request, communicates progress, and submits authorized actions. It does not replace workflow stages or require a particular model.

Give the agent the [coordinator skill](../resources/skills/watts-coordinator/SKILL.md), the exact application project path, and the selected ACL package or compiled artifact. The source and artifact can live outside the application project. Do not assume the project contains the Watts checkout. Ticket fetching is outside Watts.

## Current interface

The CLI is available today; MCP and bounded wait-for-change remain proposed. No background messaging is guaranteed: the agent's host must support continued monitoring to relay updates while the user is idle.

From the verified application directory:

```sh
wctl compile /path/to/package -o /private/path/coding.json
wctl task new cancellation-fix --compiled /private/path/coding.json
# Use the returned task path; draft its SPEC.md from the user's request.
wctl exec <task> --dry-run
wctl exec <task>
wctl task status <task> --json
```

Customize ACL/modules and variable files, then compile before task creation. The task's `workflow.json` is a resolved snapshot. Workload agents refine SPEC.md and create PLAN.md according to the selected workflow; the coordinator does not implement the application inside the task directory.

A successful submission returns before completion. Each JSON status includes `task`, `workflow_id`, `run_id`, and `state`, with current stage, attempts, errors, artifact hashes, and history. Read feedback from `state.history`. Failure states such as `waiting_retry` also print valid JSON and return nonzero; connectivity/query errors may provide no JSON. Do not confuse those cases.

Evidence is retained under `<task>/.watts-state/logs/` and `steps/<stage>-<attempt>/`. Inspect the matching attempt's output, results, events, and gate/tool records. Available usage is evidence, not necessarily complete accounting. Preserve earlier failed attempts in reports.

## Progress and reconnect

Report stage/attempt changes, meaningful findings, decision requests, recovery, and completion with evidence references. A useful update names the task and stage, explains what happened, and identifies any action needed. Avoid unchanged-status chatter. A silent log does not prove a stall; an unavailable query does not prove workflow failure.

After reconnecting, locate the existing task and read current status/history. Match workflow/run identity against the retained binding before acting. Pending decisions come from current Watts state, even if the prior conversation was lost. The CLI has no cursor-based event stream yet; polling and inspection are the current mechanisms. Stopping observation or closing the agent session does not cancel the workflow.

## Delegated decisions

The user may authorize a particular decision or explicitly delegate review within a stated task/scope. Permission to start execution alone is insufficient. Present the pending artifact, findings, unresolved questions, and requested decision when human input is needed.

Save the status JSON for the exact pending approval to a private temporary file outside the application workspace. Review the declared files and verify their SHA-256 values against the snapshot. Then:

```sh
wctl task decide <task> <stage> \
  --actor coordinator-session \
  --authorized-by <human-id> \
  --reviewed-status /path/to/private/reviewed-status.json

# Request changes with the same provenance and snapshot flags:
wctl task decide <task> <stage> --reject --feedback 'Clarify cancellation behavior' \
  --actor coordinator-session --authorized-by <human-id> \
  --reviewed-status /path/to/private/reviewed-status.json
```

All three delegation flags are required together. Watts rejects snapshots for another task/execution or a superseded approval attempt/hash set. Approval also validates current artifact contents. A rejected or stale request needs renewed inspection, not identity changes or rewriting recorded hashes.

Status and history retain the acting coordinator under `approved_by` and the declared authorizing principal under `authorized_by`; rejected outcomes retain both in history. Approval receipts record stage, attempt, and hashes. These are self-declared local provenance labels, **not authentication or verified delegation enforcement**. Direct human CLI decisions retain the existing OS-user attribution. Workload-agent identity/role checks remain prototype safeguards.

## Recovery and completion

A workflow waiting for retry is still running. Inspect the failed attempt, address an authorized cause, then use `wctl exec <task>`. A retry uses pinned configuration/images/resources, retains earlier evidence, and creates a new attempt at the current stage. Do not repeatedly retry an unchanged failure or interpret retry as rollback. Exceeding the attempt limit needs explicit authorization through `--ignore-attempt-limit`.

A terminal failed, cancelled, terminated, or timed-out execution cannot be resumed by that command. Discuss a new task to adopt changed configuration or recover an ended execution. If cancellation is requested, use `wctl task cancel <task>` and confirm status.

The final report should distinguish accepted work from partial output, cite published artifacts and verification, explain failures/rework, and report available usage without substituting zero for missing measurements. See the [runbook](runbook.md) for setup and recovery details.

## Developer verification

With Docker running and `ubuntu:latest` available locally:

```sh
WATTS_TEMPORAL_INTEGRATION=1 go test -race ./cli -run '^TestCoordinatorCLILifecycle$' -count=1
```

The test uses a temporary Temporal server and isolated project/package directories. It verifies rejection/rework, stale decisions, worker restart/reconnect, failed build/retry, review/publication, and cancellation through the public CLI. Workloads are deterministic Procedures; no hosted inference occurs. This establishes the coordination contract, not the behavior of an autonomous supervising agent. The external-agent/model eval remains tracked in wiss-004.
