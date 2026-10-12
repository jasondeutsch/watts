# Workflow execution

A package defines resources, references, and routing. Task creation resolves them
into a task's `workflow.yaml`. First submission pins the plan, image IDs, captured
resources, and worker preferences. Temporal interprets the ordered graph; the
worker executes container workloads. See [Kinds](kinds.md) for the YAML fields
and [the workload interface](container-workloads.md) for inputs/results.

## Routes and feedback

Without `transitions`, a successful stage advances to the next placement or its
explicit `next` target. An execution error follows `on_failure` when configured;
otherwise the workflow waits for an explicit retry. A terminal route is `end`.

Use exhaustive outcome routes for a build/review loop:

```yaml
steps:
  - name: build
    use: {kind: Agent, name: builder}
  - name: qa
    use: {kind: Procedure, name: qa}
    transitions:
      changes-required: build
      passed: end
```

The QA image writes a structured result such as
`{"outcome":"changes-required","feedback":"Add the empty-input case"}`.
The next workload receives the preceding result and feedback in `input.json`.
Unknown outcomes fail; they cannot jump to arbitrary stages. `max_transitions`
bounds rework, and attempt history retains each visit.

A deterministic Procedure can use an ordinary image and `result: exit-code`.
A Procedure returning custom outcomes uses the same result-file interface as an
Agent. No host plugin registration or agent-specific worker adapter is required.

## Approval and external events

Approval Procedures declare input artifacts. Temporal waits for a human decision
bound to the placement, attempt, and current hashes. Use `wctl task decide` or
the web UI. `approved` and `rejected` may have separate routes. Rejection feedback
is passed to the next workload. Changes to approval inputs invalidate acceptance.

An `execution: {type: event}` Procedure declares outcome routes and waits for a
structured external event with `id`, `step`, `attempt`, and `outcome`, plus optional
`feedback` and `data`. The application API validates stage/attempt identity,
duplicate IDs, and declared outcomes. There is no event CLI command; connectors
can use the application API. No idle container is kept for either kind of wait.

## Agent tools, gates, skills, and MCPs

Agent manifests declare callable command tools, required gates, named skill sources, and selected MCP services. Gates run inside the Agent execution and return failures for bounded repair; they do not require separate workflow stages. Gate acceptance binds to the final candidate, not an earlier successful tool call. See [Agent fields](kinds.md#agent-tools-gates-and-skills).

The image resolves
those settings inside its environment. A subprocess MCP, such as gopls, must be
installed in the image; skills can be baked into it or mounted from captured
package resources. The Pi/Ralph example image writes its own private MCP and Pi
settings and forwards those settings to Ralph children.

Changing a tool or model means changing values/image policy and creating a new
task. A retry uses the existing pinned execution. See the [Go package](../resources/examples/go_generalist/README.md)
for a concrete setup with lint, skills, gopls, and review routing.

## Errors and observation

`wctl task status --json`, the web UI, and attempt logs expose failures. A command
must exit successfully and satisfy its output contract and checks before
publication. Error events fail even a zero-exit process. Invalid/missing result
files and unsafe artifacts are explicit failures.

Per-attempt input, results, events, reported usage, container records, and
acceptance receipts are stored under `<task>/.watts-state/steps/`. Logs are in
`.watts-state/logs/`. Unknown provider counters/prices remain unknown. Follow the
[runbook](runbook.md) to inspect, fix, retry, or cancel a task.
