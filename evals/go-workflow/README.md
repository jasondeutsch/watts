# Go workflow eval

Evaluate Watts together with a team's configured Go workflow. Compare verified outcomes and human review scores across configurations and over time.

This directory defines a manual eval protocol. It does not provide an automated runner or a populated benchmark suite.

## Definition

- Eval ID: `go-workflow`
- Eval version: `0.1.0`
- Rubric version: `0.1.0`
- Status: draft; establish a baseline after selecting and freezing cases.

The eval version is independent of the Watts application version. Change the eval version when cases, starting projects, scoring, or the run procedure change materially.

## Files

- [rubric.md](rubric.md): scoring criteria and human review procedure.
- [cases/README.md](cases/README.md): case structure and selection guidance.
- [cases/TEMPLATE.md](cases/TEMPLATE.md): task-case definition template.
- [runs/README.md](runs/README.md): run records and evidence layout.
- [runs/TEMPLATE.md](runs/TEMPLATE.md): one task attempt within an eval run.
- [results.csv](results.csv): one row per evaluated case execution; initially header-only.

## Procedure

1. Select representative cases and freeze each request, starting project, acceptance criteria, and permitted scope. Record the revision or content digest of the case suite.
2. Save the workflow and relevant configuration being evaluated. Record Watts, executor, model, Go, and toolchain versions. Record credential variable names, never credential values.
3. Create a fresh isolated project from each case's starting files. Run the case through the configured workflow using the same entry point and approval rules as normal team tasks. Task documents live in the task directory; application work belongs in the project root containing `watts.yaml`.
4. Record clarification, approval, rejection, retry, and manual-edit interventions. Apply the case's intervention policy consistently. Do not silently repair outputs or discard failed runs.
5. Retain the resulting project, workflow history, checks, and available executor output. Independently check acceptance criteria and regressions; the agent's completion report is not verification.
6. Review using the rubric. Mark unavailable measurements as unavailable rather than zero. Record deviations from the procedure.
7. Create a run record and add a results row for every case execution, including failures and timeouts.

An eval run evaluates a configuration against a case suite. A case execution may contain multiple workflow stage attempts; those retries belong to the same results row. Repeating the entire case from its starting project creates another case execution and row.

## Comparing results

Compare like-for-like case suites, rubric versions, intervention policies, and run conditions. Give each configuration a stable ID and preserve its actual configuration files. Repeat cases where feasible and report variability and sample counts alongside averages.

Start with a baseline configuration. When the eval changes materially, rerun that baseline before comparing scores from the new definition. Record model aliases as well as resolved model versions when available; the same alias may change over time.

Report verified success rate, dimension scores, elapsed time, human effort, and cost when known. Keep per-case results visible. Do not introduce an overall weighted score without documenting its weights and missing-data treatment.
