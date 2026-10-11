# Tic-tac-toe exploratory eval — 2026-10-10

The application passed independent acceptance checks. The workflow did **not** complete: review was blocked by a runtime metadata false positive, and the execution was canceled to stop futile reviewer iterations. No application code was written or repaired manually.

## Application result

Watts produced a standard-library-only terminal game in the project root: one 143-line main.go, one main_test.go, and go.mod. Two human players choose numbered cells; invalid moves retain the turn; wins, draws, and clean EOF are handled.

- [Go tests](go-test.txt): passed.
- [Configured lint](lint.txt): zero issues.
- [Independent terminal checks](terminal-checks.json): all 20 passed, including every winning line for both players, a draw, invalid inputs, and EOF.
- [Source snapshot](project/main.go) and [source digests](final-source-digests.json): unchanged during review. The existing Makefile remained unchanged.

Run the game from `/Users/j/watts_project/test` with `go run .`.

## Workflow result

| Stage | Attempts | Outcome |
| --- | --- | --- |
| Specification | 2 | First draft rejected for an incorrect Go-history claim and unnecessary questions; corrected draft approved. |
| Specification approval | 2 | Delegated rejection, then approval. |
| Planning | 2 | First plan rejected for excessive abstractions and incorrect Scanner EOF semantics; minimal two-story plan approved. |
| Plan approval | 2 | Delegated rejection, then approval. |
| Build | 4 | Stale worker failure; sleep heartbeat timeout; stale interrupted Ralph state; successful fourth attempt. |
| Lint | 1 | Passed. |
| Review | 1 | Reviewer wrote PASS, but its completion check rejected Ralph runtime metadata. Canceled during the next internal iteration. |

Ralph writes `.ralph-runner/active-loops/a78d7ff854a2eee77e40f1218d2ef1143653c5a7c091c567409e017d22a12833.json` in the project root. The read-only review gate classified that runtime registry as an unauthorized change outside the task review directory. The reviewer had no application defect to fix. See [review iteration evidence](review-iterations.jsonl), [review verdict](review-VERDICT.md), and [final Temporal history](temporal-history.json). No completion check was weakened and no pinned runtime configuration was edited.

Cancellation is confirmed by [final status](status.json); active Pi/Ralph subprocesses stopped. The temporary sleep inhibitor was removed. The Watts worker and web UI remain available for inspection.

## Configuration and accounting

- Case, eval, and rubric version: 0.1.0.
- Package: Go generalist; all seven stages retained; consumer override `thinking: low`.
- Model: `deepseek/deepseek-v4-flash` through OpenRouter.
- Pi 1.0.3; Ralph 2.1.0; Go 1.26.1; golangci-lint 2.11.3; gopls 0.20.0.
- Workflow ID: `watts-dce70720603e3c023b266bd313fccd82`.
- Temporal run ID: `01a128a1-5ac7-757a-ae31-3b1218828d64`.
- Start: 2026-10-11T01:43:45.607359884Z; end: 2026-10-11T02:17:54.038344671Z; elapsed: 2048 seconds, including review waits and machine sleep.
- Configuration, resolved manifests, and build identity: [watts.yaml](watts.yaml), [values.yaml](values.yaml), [resolved workflow](resolved-workflow.yaml), [package metadata](package.json), [digests](digests.json).
- Eleven operational interventions: four delegated gate decisions, three accepted build retry requests, stopping a stale worker, enabling temporary idle-sleep prevention, archiving stale interrupted runner state, and canceling the blocked review.
- Five execution-stage retries: one specification, one planning, three build retries. Approval decisions and Ralph internal iterations are counted separately.
- Partial drafting telemetry: 136,901 input tokens; 33,430 output tokens; 513,728 cache-read tokens; 43 responses. These are separately reported categories. Build/review totals and actual provider cost are unavailable; zero reported prices are not evidence of free execution. See [partial accounting](partial-usage-summary.json).
- Human active time was not measured. Human rubric scores remain unscored; delegated Codex decisions are not independent human ratings.

## Interpretation and next improvements

This is an exploratory case, not a clean performance baseline. Mixed worker versions, two machine sleep events, manual runtime-state recovery, and delegated feedback affected elapsed time and retries. The independent verifier was added before implementation but after the workflow started. Preserve these deviations rather than treating this run as an unattended success.

The principal Watts finding is that review must distinguish application changes from runtime control metadata. Further operational findings are stale Ralph state after interruption and mixed-version workers sharing a task queue. These require follow-up work; no core fixes were made as part of this eval. Structural spec/plan checks also missed factual errors and excessive scope, which delegated review corrected.

[Detailed intervention log](intervention-log.md) retains intermediate observations and failures. [Results index](../../results.csv) records this execution alongside future runs.
