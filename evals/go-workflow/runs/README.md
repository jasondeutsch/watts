# Run records

Create `<run-id>/` for each evaluated configuration and repetition. Copy [TEMPLATE.md](TEMPLATE.md) to `<run-id>/<case-id>.md` for each case execution.

Suggested layout:

```text
<run-id>/
  configuration/       # Values, rendered bundle, task workflow, project settings; no secrets
  <case-id>.md         # Execution record and review
  <case-id>/
    final-project/     # Resulting application and task artifacts
    evidence/          # History, logs, independent checks, and review notes
```

Use unique run IDs, for example `2026-10-08-baseline-r01`. If a case is rerun from its starting project, assign a distinct execution ID. Record all executions rather than keeping only the best outcome.

Retain evidence behind each score. Remove credential values and unrelated sensitive data before storing configuration or logs. Record missing evidence explicitly. Completed records should not be silently overwritten; identify later corrections.

Add one row per case execution to [results.csv](../results.csv). Blank numeric fields mean unavailable, not zero. The combination of run ID and execution ID must be unique. `record_path` is relative to the eval directory.

## Recorded executions

| Run | Application checks | Workflow outcome |
| --- | --- | --- |
| [2026-10-10 tic-tac-toe](2026-10-10-tic-tac-toe/README.md) | Passed tests, lint, and terminal checks | Canceled during review after a runtime metadata false positive |
| [2026-10-12 container tic-tac-toe](2026-10-12-tic-tac-toe/README.md) | Passed tests and 20 terminal checks; configured lint blocked by toolchain incompatibility | Build accepted; lint failed and waits for retry; review not reached |

These are exploratory executions with documented interventions and different runtime configurations, not comparable performance baselines.
