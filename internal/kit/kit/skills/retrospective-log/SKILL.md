---
name: retrospective-log
description: Writes a structured retrospective entry after a task reaches Deployment and Release, or is rejected and abandoned. Use at the end of any Tier 1 or Tier 2 task, or any Tier 0 task that failed in an unexpected way.
---

# Retrospective Log

Write an entry in this shape:

```
Date:
Spec ref (file + version):
Risk tier assigned:
Outcome: success / failure
Failure mode (if any): no output / failed CI / passed CI but wrong / other
Review rounds (number of verdicts, and what each rejection named):
What changed as a result:
  - spec template updated? (Y/N, what)
  - risk tier rule updated? (Y/N, what)
  - nothing, logged for pattern-spotting only
```

## Where to read the facts from

Under Watts there is no single record, so read all of these before writing. Everything for the task lives in `<tasks dir>/<name>/`:

- `STORY_LOG.md` for which stories were finished and the snapshot IDs that finished them.
- `review/VERDICT.md` for the latest verdict, and `review/history/` for earlier ones. The number of verdicts is the number of review rounds.
- `RALPH_PROGRESS.md` and `OPEN_QUESTIONS.md` for the builder's learnings and any questions it raised.
- `bash <kit>/scripts/evidence.sh <task> snapshots` for the recorded snapshots and their labels. BASE names the baseline snapshot; use the same script with `diff` to inspect changes.
- The loop's own summary: `watts task status <name>` prints a deterministic run summary, and `.ralph-runner/final-summary.md` holds the same record.

Write the entry to `<tasks dir>/<name>/retrospective.md` and keep it with the rest of the folder. Run this manually after the review passes, or when a task is rejected and abandoned. Nothing triggers it automatically.

## When to write one

- Every Tier 1 and Tier 2 outcome, whichever way it went.
- Any Tier 0 outcome that failed in a way nobody expected, that's a sign the tiering itself was wrong, not just this one run.
- Do not log routine Tier 0 successes. That's noise nobody rereads, and it buries the entries that actually matter.

## The point of the "what changed" field

This is not a summary of what happened, the Outcome and Failure mode fields already cover that. This field is for whether this outcome taught the factory something durable, a line added to the spec-template skill, a rule added to the risk-tiering skill. If a real failure produced no change to either, that's worth being honest about rather than papering over, it usually means the failure mode needs a second look before it's actually closed out.

Changes to a skill or to the loop prompts follow the same rule as everything else: whoever drafts the change does not approve it. An agent may propose it, and a human reviews and applies it.
