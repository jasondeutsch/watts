# Workflow stage communication

Status: proposal for discussion. This document defines a direction for stage interfaces, including compilation into portable execution packages and a build/review loop. It does not introduce new configuration fields or claim container execution is implemented.

## Purpose

A stage describes work and its contract. An executor performs that work using Pi/Ralph, another agent, a script, or eventually a container. Temporal and Watts own scheduling, recorded results, routing, retries, and approvals.

Stages communicate through engine-delivered results and referenced artifacts. They do not launch or call another stage directly. The interface should remain consistent across executor implementations.

## Authoring, compilation, and execution

`workflow.yaml` is the authoring format. Compilation combines it with project configuration and required resources to produce an immutable, resolved execution plan. Applying the compiled plan persists its identity and starts execution through Temporal.

Compilation validates stage contracts and connections, resolves executor settings and MCP definitions, and packages instructions, skills, scripts, and configuration. The source workflow remains available for editing and provenance, but workers do not reread it during that execution. Retries use the same compiled plan; configuration changes require a new compilation and application.

The conceptual output is:

```text
compiled-plan/
  manifest.json
  stages/
    build/
      manifest.json
      instructions.md
      skills/
      scripts/
      configs/
    lint/
      manifest.json
      scripts/
      configs/
    review/
      manifest.json
      instructions.md
      skills/
      configs/
```

Directories are illustrative, not a required on-disk format. A stage package contains everything needed to define its execution: resolved instructions, resources, input/output contracts, allowed outcomes, executor requirements, workspace permissions, and lifecycle limits. Runtime dependencies can be supplied by a referenced container image. Packages and images need immutable identities so retries do not silently execute changed resources.

The orchestration manifest contains the entry stage, stage package references, outcome-to-stage transitions, upstream input bindings, human approval and event waits, and workflow-wide limits. It supplies run instructions to a common Watts Temporal interpreter; compilation does not generate custom Temporal code for each workflow. A stage declares outcomes; the orchestration manifest owns their destinations.

Compilation resolves configuration, not future execution values. Concrete workspace revisions, upstream artifacts, attempt identities, and feedback are bound when an attempt starts. Credentials remain references resolved at launch, not values embedded in packages or Temporal history.

The resulting layers are authoring configuration, compiled manifests and stage packages, then attempt-specific execution requests. The Kubernetes analogy describes declarative configuration and execution resources; continuous reconciliation and live mutation of running plans are outside this proposal.

## Execution request

Each attempt receives:

- Task, stage, and attempt identifiers.
- Stage parameters and declared outcome names.
- Required task artifacts, such as the approved SPEC.md and PLAN.md, with their identities and hashes.
- A reference to the workspace revision to inspect or modify, and the permitted access mode.
- Relevant upstream attempt results and artifacts, selected by the workflow's input bindings.

Inputs should identify their producing attempt explicitly. A build stage should be able to consume the latest review findings even when lint or QA ran between review and build. Depending exclusively on the immediately preceding stage would make that relationship fragile.

The syntax for input bindings, selection of prior attempts, and workspace references remains to be defined. These are conceptual requirements, not a proposed final JSON schema.

## Execution result

An executor returns:

- **Outcome:** a value declared by the workflow, such as `success`, `approved`, or `changes-required`.
- **Feedback:** a concise explanation for the next consumer.
- **Artifacts:** references to produced reports or other evidence.
- **Data:** optional structured information specific to the stage.

The engine validates declared outcomes and artifact references, calculates hashes, and records the result with the producing stage and attempt. The executor supplies content; the engine supplies trusted execution identity and evidence bindings.

stdout and stderr are diagnostic logs, not the routing interface. Required outputs and checks must pass before a successful executor result is accepted.

## Build and review loop

1. Build consumes the approved specification and plan, the current workspace, and any applicable review findings.
2. Build produces implementation changes and verification evidence. On rework, it reports which finding IDs were addressed, which remain unresolved, and supporting checks.
3. Review consumes that build's workspace revision, build evidence, approved documents, and applicable prior findings.
4. Review returns `approved` or `changes-required`, with actionable findings when changes are required.
5. The workflow routes the result to completion, another acceptance stage, or another build attempt according to its declared transitions.

A conceptual review result:

```json
{
  "outcome": "changes-required",
  "feedback": "Handle request cancellation before persisting the order.",
  "artifacts": ["review/findings.json"],
  "data": {
    "reviewed_workspace": "workspace-004"
  }
}
```

A corresponding findings artifact:

```json
{
  "findings": [
    {
      "id": "R-001",
      "criterion": "AC-3",
      "location": "internal/orders/service.go:84",
      "problem": "A cancelled request can still persist an order.",
      "expected": "Cancellation prevents persistence.",
      "verification": "Add a test that cancels before the repository call."
    }
  ]
}
```

Paths in these examples are logical artifact names. Their physical storage must be scoped to the producing attempt so a later review cannot overwrite earlier evidence. A finding ID should remain stable through rework; its review-attempt identity disambiguates it from findings in other tasks or reviews.

An agent review outcome is distinct from a human approval. The workflow must make the acceptance authority explicit.

## Communication invariants

- Results and accepted evidence are immutable per attempt. Later attempts produce new records.
- Review identifies the exact workspace revision it inspected. Its approval does not apply to later changes.
- The workflow declares which upstream results each stage consumes. Missing required evidence fails explicitly rather than silently substituting unrelated output.
- Executors report outcomes; the engine selects destinations from declared transitions.
- Execution errors are distinct from completed business outcomes. A reviewer crash, missing tool, or provider timeout is a failed review attempt, not `changes-required` evidence against the code.
- Rework and retries have stage attempt budgets and a total transition budget. Exhaustion pauses for intervention and preserves all attempts.
- Timeout or cancellation stops the attempt and its executor. Partial workspace changes must be identified; cancellation does not imply rollback.

## Isolated workspaces and distributed execution

The target contract assumes isolated workspaces with explicit publication. A stage may run in a container or sandbox on a distinct node. It must not depend on another node's filesystem, the compiler's checkout paths, or a shared writable project mount.

Before launching an attempt, its executor materializes the stage package, the selected workspace revision, and bound artifacts locally. Application paths refer to that materialized workspace; task artifact names remain logical names independent of a node's physical directories. Build receives writable application access. Review and QA receive read-only application access and writable report/output locations.

Build publishes a new workspace revision or change set with verification evidence. Review consumes that exact published revision. Changes become visible to downstream stages only through accepted publication, not by observing live writes from another process. Updating the user's project is an explicit operation with a defined destination and conflict policy; it does not follow automatically from the worker's local file edits. No Git dependency is required by this contract.

A shared local workspace may be an initial execution implementation, but it must preserve the same revision and publication semantics rather than define the interface around local paths. Containerization alone does not provide isolation when containers share writable mounts.

Distributed execution also requires:

- Durable, addressable packages, workspace revisions, and artifacts, with integrity hashes and retention rules. Large content lives in artifact storage; orchestration retains references.
- Output publication before reporting an accepted result. Interrupted uploads or repeated completion messages must not produce duplicate or ambiguous accepted outputs.
- Stage runtime requirements and routing to compatible workers or nodes. The mechanism is not yet chosen.
- Credentials scoped to the stage and injected at launch. Local MCP executables run in the stage environment; remote endpoints must be reachable from its node.
- Logs, status, timeout, and cancellation bound to an attempt across node boundaries. Node loss must leave a diagnosable execution state and avoid overlapping replacement attempts.
- Explicit treatment of partial changes after failure or cancellation. They must not silently become the next accepted workspace revision.

Temporal coordinates orchestration. Watts executor adapters own package materialization, isolation, subprocess/container lifecycle, and output publication. Human approvals and external event waits remain orchestration stages and do not require execution containers.

## Relationship to current Watts

Watts already has stage inputs and outputs, a shared executor result containing outcome/feedback/artifacts/data, validated transitions, attempt records, and previous-stage feedback. See [workflow configuration](workflows.md) and [workflow architecture](workflow-design.md).

Watts currently pins workflow definitions and runtime settings, but it does not compile self-contained stage packages. Explicit upstream bindings, immutable artifact delivery across executors, a workspace revision and publication contract, and distributed container execution require further design and implementation. Current task-relative output files and snapshots should not be mistaken for the complete proposed interface.

## Relationship to tracked issues

The proposal overlaps with these items in [issues.md](issues.md), without implying that implementing it resolves them:

| Issue | Connection |
| --- | --- |
| [wiss-007](issues.md#wiss-007-executor-abstraction) | A common stage contract separates workflow behavior from Pi/Ralph and future container executors |
| [wiss-014](issues.md#wiss-014-watts-worker) | Portable packages and external artifact persistence support workers without project-local state dependencies; persistence choices remain open |
| [wiss-006](issues.md#wiss-006-shared-logging-across-projects), [wiss-009](issues.md#wiss-009-worker-terminal-output) | Distributed logs need consistent project, task, stage, and attempt identities |
| [wiss-003](issues.md#wiss-003-web-ui-status-and-execution-output), [wiss-008](issues.md#wiss-008-global-web-ui) | UI visibility should follow execution identities rather than a single worker's local files |
| [wiss-013](issues.md#wiss-013-ralph-artifact-location) | Executor evidence becomes attempt-scoped published artifacts |
| [wiss-011](issues.md#wiss-011-completion-report) | Recorded results and evidence supply verification, timing, and usage reporting |

## Open decisions

- Does independent review finish a task or lead to final human acceptance? This should be a workflow choice.
- How do input bindings select results across loops, and what happens when a producer has not run yet?
- How are immutable artifacts stored, addressed, and delivered to local processes and containers?
- How are workspace revisions captured, identified, and published back to a user project, including conflicts with external edits?
- How are partial changes from failed or cancelled build attempts handled on retry?
- What is the minimum executor lifecycle contract for containers, including credentials, MCP resources, logs, and cancellation?
- What are the compiled manifest schemas, package format, integrity rules, and compile/apply interfaces?
- Which artifact store and transfer protocol support local and distributed execution, and who owns retention?
- How are executable dependencies and container images pinned, and how are runtime requirements matched to workers?
- How does publication remain atomic and idempotent across node loss or repeated completion delivery?
