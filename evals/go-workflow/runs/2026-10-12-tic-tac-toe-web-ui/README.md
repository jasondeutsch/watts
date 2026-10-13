# Eval: Tic-tac-toe web UI

**Workflow: completed with Agent PASS. Independent eval: fail.** The web game works and terminal behavior is preserved, but not every acceptance criterion was satisfied. No application source was edited by the coordinator.

## Configuration and identity

- Case `tic-tac-toe-web-ui` / `0.1.0`; eval/rubric `go-workflow` / `0.1.0`.
- Configuration `coder-golang-low-tools-gates`; project `/Users/j/watts_project/test`.
- Task `tasks/2026-10-12-tic-tac-toe-web-ui-eval`.
- Workflow `watts-c5fb15192d2c2dc3f509809971a3d605`; run `01a13230-ce84-77d5-91a7-ca2909ec9d79`.
- OpenRouter `deepseek/deepseek-v4-flash`, thinking `low`; alias recorded, resolved model version unavailable.
- Six configured stages: specification → spec-review → planning → plan-review → build → review. Build owns build/test/vet/lint/story gates; no separate lint stage.
- Images, CLI hash, source package, consumer values, rendered task bundle, worker settings and input identities are retained in this directory. Credential values are excluded.

This is an exploratory incremental eval, not a repeatable performance baseline or evidence of productivity gains. [CASE.md](CASE.md), [baseline source](baseline/), [baseline hashes](baseline-digests.json), and [final source](final-project/) define the comparison.

## Execution and decisions

The user delegated resolution of server-held state and SPEC/PLAN review for this eval. The coordinator reviewed artifacts and verified their hashes before decisions, using `--actor Codex --authorized-by j`. Those labels declare provenance; they do not authenticate identities. [Interventions](interventions.json), [execution record](execution.json), and [status/history](status.json) retain the exact decisions.

Specification ran twice: the first review requested recording the resolved shared in-memory state, refresh retention, serialized updates, and reset to X. Planning ran twice: the first plan passed its structural check but omitted state hydration and used conflicting startup interfaces. Review requested corrections, then approved the revised `GET /state` and `-addr` plan.

Build completed in one workflow attempt with three internal Ralph iterations. The first produced no durable changes and included a 16,000-output-token response. Later iterations implemented the feature and corrected story-log formatting. All five final-candidate gates passed. Review completed in one workflow attempt with two Ralph iterations: verdict validation caught Markdown around the required final line, and the next iteration corrected it. Agent review returned PASS. No explicit workflow retry or model/policy substitution occurred.

## Independent results

| Check | Result | Evidence |
| --- | --- | --- |
| Existing terminal regression cases | 20/20 pass | [terminal-checks.json](terminal-checks.json) |
| Actual executable HTTP behavior | 21/21 pass | [http-checks.json](http-checks.json), [verifier](verify-http.py) |
| Host Go tests and vet | Pass | [host checks](host-checks.json) |
| Build Agent final gates | All five pass | [gate receipt](evidence/build-1/workload-gates.json) |
| Browser play, both winners, draw, reset, occupied/post-completion rejection | Pass | [browser checks](browser-checks.json) |
| Refresh and second-tab hydration | Pass | [browser checks](browser-checks.json) |
| Narrow 360px layout | No horizontal overflow | [mobile screenshot](browser-mobile.jpg) |
| Empty-cell accessible names | Fail: nine unnamed buttons | [browser checks](browser-checks.json), [HTML](final-project/index.html) |
| Meaningful production HTTP tests | Fail: test setup copies the handlers | [server_test.go](final-project/server_test.go) |
| Private build-output policy | Fail: Linux executable published into project | [artifact metadata](published-build-artifact.json), [workspace receipt](evidence/build-1/workspace.json) |
| Original native executable preservation | Fail at checkpoint; restored after eval | [checkpoint](protected-artifact-check-build-1.json), [interventions](interventions.json) |

The original `ttt` is present in specification-2's baseline/candidate receipt but absent from planning-1's baseline. The deleting actor/operation is not established; do not attribute that disappearance to build. Build did demonstrably publish a separate 9,079,875-byte Linux executable. After completion, the coordinator quarantined that incidental file under `/private/tmp` and restored the original `ttt` byte-for-byte from retained baseline storage. Cleanup does not turn this into an unassisted pass.

The [Agent verdict](agent-VERDICT.md) incorrectly asserts accessible labels and calls the new Linux binary pre-existing. Command success and verdict syntax were enforced, but story/review completion claims were false for required criteria. Therefore verified success and semantic workflow compliance are recorded as **fail**. Numeric human rubric scores and human minutes remain unavailable; delegated coordinator inspection is not an independent human rating.

Host tests initially failed because the tool sandbox blocked local listeners; rerunning with local network access passed. This was a verification-environment issue, not an application failure. Independent verification builds used `/private/tmp`, preserving the original executable.

## Resource consumption

All six completed model-attempt records, including rejected drafting/planning, are in [usage-summary.json](usage-summary.json).

| Measurement | Reported value |
| --- | ---: |
| Input tokens | 7,178,007 |
| Output tokens | 122,780 |
| Input + output | 7,300,787 |
| Cache-read tokens, already included in input | 6,204,889 |
| Input excluding reported cache reads | 973,118 |
| Cost | Unavailable |

Recorded submission to first observed completion was approximately 35 minutes, including approval waits and polling delay. Build's Ralph loop took about 14 minutes; review's took about 4.4 minutes. Token totals describe context processing, not a billed-cost estimate. Cache must not be added to input again.

Four delegated review decisions and one post-eval artifact recovery are recorded beyond setup/submission. Specification and planning each repeated once; human decision stages each also ran twice. Internal Ralph iterations are separate from workflow-stage attempts.

## Findings and follow-up

- **wiss-003:** the UI showed task/attempt/history correctly, but nested build activity and live usage stayed in private evidence while the outer log exposed only session/completed-iteration records. [Live observations](live-observations.json) retain the discrepancy.
- **wiss-031:** private-output guidance and the declared build tool did not stop another Agent command's generated binary from being published. Candidate publication policy needs an explicit boundary.
- **wiss-046:** resource budgets and useful consumption feedback. This small feature processed 7.3 million reported tokens; correctness alone cannot establish efficiency.
- **wiss-047:** production-path tests, reviewer baseline context, and evidence-backed acceptance judgments. The reviewer missed substantive defects while reporting PASS.
- **wiss-042 / wiss-032:** this execution supplies an existing-code case and a hosted run of Agent tools/gates; it does not establish repeated-run comparability or close all recovery/repair scenarios.

## Run the result

From `/Users/j/watts_project/test`: `go run . -addr=127.0.0.1:8091` serves the web UI; `go run .` runs terminal mode. The eval preview was left running at [127.0.0.1:8091](http://127.0.0.1:8091). The restored `ttt` is the original terminal executable, not a rebuilt web executable.
