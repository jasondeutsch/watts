# Tic-tac-toe container eval — 2026-10-12

The generated game passed **20/20 independent terminal checks**, `go test`, and `go vet`. The full workflow **did not pass**: the deterministic lint stage failed because the pinned linter cannot read the image's Go 1.27 export data. No application code was written or repaired by the coordinator.

## Configuration and execution

- Case/eval/rubric version: 0.1.0; [case](../../cases/tic-tac-toe/README.md).
- Fresh project: `/Users/j/watts_project/test`, initially empty. Unlike the previous run, there was no Makefile to preserve. This is a documented starting-project deviation.
- Package: Go generalist, all seven stages, consumer `thinking: low`; [consumer descriptor](watts.yaml), [overrides](values.yaml), [frozen package source](package-source/values.yaml), [resolved workflow](resolved-workflow.yaml).
- Watts 0.1.0, current dirty checkout; source/build identity and digests: [identity](identity.json), [source hashes](source-digests.json). Installed CLI was not used or replaced.
- Pinned image: `sha256:3deed56682efc0c3b79ba0add97237323a549a7101af6f8a4216cf08736169e1`.
- Image: Pi 1.0.3, Ralph 2.1.0; Go 1.27.0 linux/arm64, golangci-lint 2.11.3, gopls 0.23.0. [Observed versions](image-tool-versions.txt). macOS 26.3.1 arm64 host; local Docker Engine 20.10.21.
- Model: `deepseek/deepseek-v4-flash` via OpenRouter. Resolved upstream model revision and billed provider cost unavailable. Credential source name: `OPENROUTER_API_KEY`; no credential values retained.
- Workflow: `watts-62caf69653b0e9dc8d38e28f1152d74b`; run: `01a13084-5ea2-782e-ab76-4db6a5f8ae59`.
- Started: 2026-10-12T14:29:03.778538129Z (09:29 CDT). Eval cutoff: 2026-10-12T14:46:27.648474626Z (09:46 CDT). **1044 seconds** to the lint failure, including approval waits and diagnostics.
- State at cutoff: `waiting_retry`, lint attempt 1. This is a resumable failed-stage wait, **not a terminal Temporal execution**. Review was not reached. No explicit stage retries or cancellations; no eval workload remains running. The project worker/web UI remains available at `http://127.0.0.1:50584`.

## Stage outcomes

| Stage | Attempts | Outcome |
| --- | --- | --- |
| Specification | 1 | Passed; delegated approval. Minor owner/date placeholders retained. |
| Spec approval | 1 | Approved under prior delegated eval authorization. |
| Planning | 1 | Passed; five-story single-package plan approved. |
| Plan approval | 1 | Approved under prior delegated eval authorization. |
| Build | 1 | Accepted and published; two internal Ralph iterations. |
| Lint | 1 | Failed with export-data incompatibility; waiting for retry. |
| Review | 0 | Not reached. |

Build's first internal iteration took 647 seconds, emitted `DONE`, and marked all stories done. Its private completion gate rejected `Missing OPEN_QUESTIONS.md`. The second iteration created that helper file and completed at 14:46:02 UTC. [Private gate evidence](build-private-evidence/.ralph-runner/events.jsonl) preserves both outcomes.

## Independent verification

The application contains a 165-line `main.go`, a 253-line `main_test.go`, and standard-library-only `go.mod` in the project root. Task documents remained under `tasks/`. No Pi/Ralph runtime directory leaked into the application root.

- [Go tests](go-test.txt): passed; host coverage 97.2% (the agent reported 97.8% in its story log; coverage is toolchain-dependent and not a correctness score).
- [Go vet](go-vet.txt): passed.
- [Terminal verification](terminal-checks.json): all eight winning lines for each player, draw, invalid moves retaining turns, immediate EOF, and partial-game EOF passed. The [verifier](verify-terminal.py) was prepared before build and compiled the game separately into `/private/tmp`.
- [Module list](module-list.txt): no external application dependencies.
- Configured lint: failed. [Actual lint attempt](logs/lint-1.log) and [read-only reproduction](lint-toolchain-failure.txt) identify image/tooling failure, not application lint findings.
- Overall acceptance: **fail**, because the required configured lint gate did not pass. Behavioral verification passed; full workflow completion remains unverified. Human rubric scores remain unscored; delegated coordinator review is not an independent human rating.
- [Source snapshot](project/main.go), [tests](project/main_test.go), and [hashes](final-source-digests.json) preserve the generated implementation.

## Findings and issues

1. **Incompatible image tools — wiss-029.** The image's golangci-lint 2.11.3 cannot decode Go export data version 4. Builder installed 2.14.0 privately and ran its gates successfully, but the separate lint Procedure used the original image and failed. A private workaround cannot fix another workload. Add behavioral image smoke checks, rather than checking tool versions/help alone.
2. **Undocumented completion artifact — wiss-030.** Ralph required `OPEN_QUESTIONS.md`, absent from launcher preparation/instructions. Completed application work incurred a metadata-only second iteration. Prepare the private runtime contract inside the image and expose exact blocked-gate reasons.
3. **Incidental binary publication — wiss-031.** The builder's final build recreated and published a 2,367,736-byte Linux arm64 executable into a macOS project. It was not created by independent verification. [Artifact evidence](published-build-artifact.json). Keep verification products private and make publication disposition explicit.
4. **Hidden active output — expanded wiss-003.** During active build, its durable log held only a session header while private usage events and tool activity continued. The [web snapshot](web-build-snapshot.sse) exposed neither. Large final drafting JSON events also exceeded the 32 KiB tail and disappeared, leaving only settled/check messages; [log sizes](log-shape.json).
5. **Decision provenance — expanded wiss-004.** Authorized delegated approvals appeared as `approved_by: j` (the OS account), without distinguishing the acting coordinator from its authorizing human. No unauthorized approval occurred; the audit representation is incomplete.
6. **Accounting improvement and limits — updated wiss-025.** All completed model-backed attempts retained reported usage, including nested build turns. Cost remains unknown; live usage and crash/cancellation completeness are not established by this run.

The CLI correctly returned exit code **1** for the failed lint stage and printed its detailed error; [stderr](cli-failure.txt), [exit code](cli-status-exit-code.txt). The web API retained the failed stage, attempt, error, and `waiting_retry` state; [failed snapshot](web-failed-snapshot.sse). Browser interaction with the retry button was not exercised, and no retry was requested.

Nonblocking observations: the planning scaffold initially referenced a host package lint path; the planning agent corrected it to the container resource path. Specification placeholders survived structural checks. The five-story plan and repeated gate runs are an efficiency question, not evidence that fewer stages would improve quality. gopls was configured, but successful model-backed MCP usage was not demonstrated by retained output.

## Usage and interventions

| Attempt | Input tokens | Output tokens | Cache-read tokens (included in input) |
| --- | ---: | ---: | ---: |
| Specification 1 | 31,336 | 2,530 | 22,784 |
| Planning 1 | 46,359 | 4,870 | 35,328 |
| Build 1 (both iterations) | 1,753,701 | 32,681 | 1,481,188 |
| **Total** | **1,831,396** | **40,081** | **1,539,300** |

**1,871,477 reported input + output tokens**, including cached input; do not add cache-read again. Review consumed no tokens because it did not run. Deterministic lint has no reported model usage. Provider billing/cost is unknown, not zero. [Usage summary](usage-summary.json) and [attempt evidence](steps/build-1/usage.json).

Five grouped operational interventions: consumer-package setup, interactive-shell worker launch, spec approval, plan approval, and read-only runtime diagnostics/history collection. Exact shell-call count is not the intervention count. Active human minutes were not measured. [Intervention log](intervention-log.md) records the actions and deviations. Diagnostic lint runs used private caches and shared CPU, so elapsed time is not a clean performance baseline.

## Interpretation and next run

The container boundary, source placement, build publication, nested usage capture, and explicit failed-stage reporting worked in this execution. The full Go workflow remains blocked by its image and runtime integration. This does not establish review correctness, model-backed MCP behavior, recovery under node loss, or complete provider accounting.

Fix wiss-029 and wiss-030 first, then rerun from a fresh starting project with the corrected source package and image. Rebuilding an image tag does not update this pinned task's image ID; create a fresh task for the new configuration. Keep this failed execution as a comparison record rather than overwrite its manifests or silently repair it.

See [issues](../../../../issues.md), [workflow history](temporal-history.json), and [status at cutoff](status.json).
