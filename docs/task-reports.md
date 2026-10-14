# Task reports and workflow metrics

`<task>/report.html` is an offline view of recorded execution. Its dashboard shows
stage time and token charts, approval time, repeat workload attempts, failures,
reported cost, gate coverage, workspace change volume, and optional measurements.
The execution diagram and individual attempts retain routing, decisions, and
evidence. Reports do not run or test the application.

The usage dashboard uses compact panels with explicit units and independent
scales for input, output, total tokens, cache reads/writes, first versus repeat
visits, recorded time, and reported cost by stage. Hover a row for its values and
evidence coverage. Gray bars denote partial subtotals; striped tracks denote
unknown observations. Zero is shown only when reported.

The attempt graph shows input/output for each model attempt in execution order,
including failures and rework. Click a bar or label to reach that attempt's
details. Approval stages have timing but no model-usage bars. This is a series
of recorded attempt totals, not continuous telemetry or an inference-rate graph.
Charts use inline HTML/CSS/SVG and work offline without JavaScript or Grafana.

## Metric definitions

| Metric | Meaning |
| --- | --- |
| Reported tokens | Input + output across recorded model attempts, including failures and rework. Cache counters are not added again. |
| Repeat workload attempts | Subsequent visits to a non-human stage. Repeated approval visits are counted as decisions instead. |
| Repeat token share | Tokens in repeat model attempts divided by all reported model tokens; unavailable if either total is incomplete. |
| Elapsed | Workflow start to finish, including approval stages. Stage durations also include tools and checks; they are not inference-only timing. |
| Approval time | Recorded human-stage duration, including waiting and review. It is not a measurement of active human effort. |
| Gate coverage | Recorded results matching declared gates. A passed result requires the expected exit code. This is not test coverage. |
| File-change events | Added, modified, or removed files in accepted writable workspace attempts. Repeated edits to a file count again. |
| New-file text lines | Physical lines in newly added text files. |
| Net text-line change | Candidate text lines minus baseline text lines, summed across accepted writes. It does not count insertions/deletions within a modified file separately. |
| Volume / tokens | Net text-line change per 1,000 reported workflow tokens, and tokens per net added text line. The latter is N/A without a positive net increase. |
| Non-text / bytes | Non-text file changes and net workspace byte change. |

Workspace volume is captured by the executor against the same content identities
as its snapshots. Text means valid UTF-8 without NUL bytes; lines include comments,
blank lines, tests, configuration, and documentation. Binary content contributes
no text lines. Counts are language-independent and need no Git repository.

These ratios measure change volume, not productivity or quality. Refactoring or
deletion can be valuable with little or negative net growth. For source-only or
test-only measures, a team can supply a scoped workload measurement.

Only accepted publications contribute to output volume. Failed or uncertain
publication, missing measurements, and incomplete usage prevent an efficiency
ratio. Coverage is shown beside subtotals; unavailable cost is unknown, not zero.
Task artifacts are separate from application-workspace volume. External coordinator
usage and provider billing are not inferred from workload counters.

## Complexity and other team measurements

Watts does not invent a universal complexity score or embed a language analyzer.
An Agent or Procedure can use its configured tools to emit a `measurement` event
in `/outputs/events.jsonl`, using the existing workload event interface:

```json
{"id":"complexity-check-1","type":"measurement","details":{"name":"Cyclomatic complexity","kind":"complexity","method":"team-check/v1","unit":"points","scope":"changed functions","baseline":12,"value":15}}
```

This is an illustrative record, not a result from a particular task. `name`,
`method`, `unit`, `scope`, and a finite numeric `value` are required. `baseline`
is optional; an explicit zero remains a measured zero. `kind: complexity` makes
the latest observation visible in the complexity summary. Other kinds can report
coverage, dependency changes, benchmark results, or team-defined outcomes.

The report displays baseline/value charts, deltas, method, scope, and stage/attempt
provenance. Measurements remain workload-reported evidence, not an independent
attestation. Unlike methods or scopes are not combined into an overall grade.
Malformed observations are marked unavailable in the attempt details.

Compare the same case, baseline, workflow/model/tool configuration, metric method,
scope, and intervention policy over time. A single run does not establish a trend.
