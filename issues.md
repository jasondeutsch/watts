# Watts issues and proposed improvements

Watts remains a proof of concept at **0.1.0**. This backlog contains open work; completed items move to [issues.done.md](issues.done.md) without reusing their numbers. Record the changes, verification, and remaining limitations when closing an issue. Each issue distinguishes observed failures, implemented foundations, and proposed capabilities. A passing component test does not establish full workflow completion.

Watts core executes a generic workload contract. Agent/tool integrations belong in images; Temporal owns durable orchestration. Currently, only local Docker Engine execution is supported. Podman and Kubernetes remain possible future backends.

The [October 10 eval](evals/go-workflow/runs/2026-10-10-tic-tac-toe/README.md) and [October 12 container eval](evals/go-workflow/runs/2026-10-12-tic-tac-toe/README.md) retain historical evidence. Prioritize the current output, image integration, and publication problems below before treating Golang coder as a reliable end-to-end example.

## Candidate improvements

These candidates capture small, focused improvements from the repository review. They are proposals, not scheduled work. Where a candidate narrows an existing issue, keep implementation and evidence coordinated with that issue rather than creating a parallel subsystem.

### wiss-035: Surface existing usage and gate evidence

Attempt usage records, command-tool results, and gate receipts already exist, but users should not need to inspect private files to understand them.

- Add cumulative reported token counts, attempt counts, gate outcomes, and evidence links to existing CLI status and web views.
- Include failed/retried attempts; keep missing metrics distinct from zero and avoid double-counting cache reads already included in input totals.
- Start with retained post-attempt evidence. Durable live capture and crash accounting remain wiss-025; coordinate presentation with wiss-003 and wiss-011.

### wiss-036: Preserve visibility of oversized log records

The web monitor's 32 KiB tail can discard a large partial JSON record, hiding useful output even when it was retained.

- Mark truncation explicitly and show a useful bounded preview without treating a partial record as complete structured data.
- Provide access to the complete retained attempt output and verify records larger than the tail limit.
- This is a focused first delivery for wiss-003; full live event streaming remains separate work.

### wiss-037: Make failure messages concise and actionable

Long diagnostics can appear repeatedly in the workflow failure and attempt output, making the actual cause difficult to find.

- Lead with stage, attempt, concise cause, and available action; preserve readable line breaks.
- Keep full diagnostics accessible through expandable UI details and retained evidence. Preserve nonzero CLI failure status and structured error fields.
- Explain when retry uses the original pinned configuration/image rather than newly edited ACL. Coordinate with wiss-003 and wiss-012.

### wiss-038: Identify stale locally managed workers

The local service record contains a PID and UI URL but no build identity. Rebuilding the CLI does not update an already running worker, and the product version remains 0.1.0.

- Record and display worker start time, build identity, and workload protocol identity alongside its PID and URL.
- Detect a mismatch between the invoking CLI and the locally managed worker before submission, with a clear restart instruction.
- This is a local diagnostic slice of wiss-023, not a solution to compatible routing across remote Temporal workers.

### wiss-039: Expand infrastructure preflight checks

`doctor` currently checks the container engine, images, and required secret sources in the calling process. Those checks do not establish that Temporal is reachable or that a running worker has the same environment.

- Check configured Temporal connectivity/namespace and locally managed worker status with bounded timeouts.
- Distinguish checks of the calling shell from verified worker configuration; do not imply a shell's key is present in a detached worker.
- Report specific remedies without executing a model workload. Coordinate build/protocol checks with wiss-038 and remote connection support with wiss-026.

### wiss-040: Create task scaffolds atomically

Task creation currently creates the final directory before writing artifacts, the resolved workflow, and package metadata. A later failure can leave a partial task that blocks another attempt with the same name.

- Prepare and validate the complete scaffold in a private staging directory, then publish it into the final task location.
- Clean up staging after failure and preserve an existing destination, including concurrent creation attempts.
- Verify failures during artifact/metadata writing leave no visible partial task and do not damage existing tasks.

### wiss-042: Evaluate changes to an existing application

The original tic-tac-toe eval starts from an empty project. The [web-UI eval](evals/go-workflow/runs/2026-10-12-tic-tac-toe-web-ui/README.md) now exercises an incremental change against captured source and native-binary baselines. The workflow completed, and independent HTTP/terminal checks passed, but accessibility, test quality, and artifact handling prevent a clean verified success. Repeat this case from the same baseline before drawing performance conclusions.

- Define a small feature or intentional bug fix against a known application baseline, with regression checks for existing behavior.
- Record the package/values, images, context, starting code, approvals, outcomes, attempts, elapsed time, and available usage evidence using the existing eval structure.
- Compare repeated runs from the same baseline; keep human assessment alongside deterministic checks. This is an eval scenario, not a new eval framework.

### wiss-046: Bound and explain workflow resource consumption

The [incremental web-UI eval](evals/go-workflow/runs/2026-10-12-tic-tac-toe-web-ui/README.md) reported more than 2.5 million input-plus-output tokens during its first build attempt before gate acceptance. One Ralph iteration produced no durable changes and included a 16,000-output-token response. Iteration limits and timeouts alone do not describe the resource cost of repeated long contexts.

- Consider optional declarative per-workload and whole-run limits for reported tokens and elapsed execution time, covering retries and internal repair loops. Preserve generic orchestration; image adapters report normalized consumption.
- Expose consumption, cache treatment, remaining known budget, and the reason for a limit being reached to coordinators and the UI. Unknown cost or missing telemetry must remain explicit.
- Define the stop/recovery contract: preserve candidate/evidence, stop safely at a supported boundary, and require an explicit decision before extending an exhausted budget. Do not silently switch models or weaken gates.
- Record whether limits are enforceable live or only checked after an attempt; durable telemetry under wiss-025 is a prerequisite for reliable live enforcement. Evaluate changes to example prompt/context size using repeated comparable runs, independently of correctness.

### wiss-047: Make Golang coder review verify claims against real evidence

The [incremental web-UI eval](evals/go-workflow/runs/2026-10-12-tic-tac-toe-web-ui/README.md) completed with reviewer PASS, but independent inspection found unnamed empty-cell buttons and HTTP tests that copied handlers instead of testing the production router. The reviewer also described the newly published Linux binary as pre-existing, contradicting the captured original baseline. Passing command gates and a syntactically valid verdict did not establish all acceptance criteria.

- Refine example builder/reviewer guidance to exercise production code paths. Factor reusable handler construction when needed; test the real handler or executable rather than recreating implementation in tests.
- Supply a captured baseline/change inventory as explicit reviewer context so preservation claims compare before and after. Use generic artifacts/resources and provenance; keep Go-specific review policy in the example.
- Require evidence for each substantive claim, including browser accessibility/responsiveness and protected-artifact integrity where the case requires them. Missing evidence is an unresolved finding, not PASS.
- Retain a challenge case with a passing copied-handler test, unnamed controls, and an incidental build product; verify the configured reviewer identifies defects and returns `changes-required` to build.
- Compare fresh runs before accepting improved reviewer guidance. Coordinate publication policy with wiss-031 and context primitives with wiss-033/wiss-043; Watts cannot guarantee model judgment merely by validating verdict syntax.

### wiss-043: Demonstrate explicit project context with existing primitives

Teams need a practical starting point for supplying architecture notes, conventions, and constraints to an ephemeral builder.

- Extend the Golang coder example with a small versioned context resource and named skill that explicitly tells the Agent how to use it.
- Demonstrate captured/pinned inputs and keep context subordinate to task scope, approvals, and verification results.
- Start through existing resources and skills, without adding a memory service or declaration. This is the initial example slice of wiss-033.

### wiss-044: Reconcile issue status with implemented foundations

Image-owned Ralph completion preparation and private build-output handling have improved since the container eval. The backlog should distinguish those changes from the remaining failures and verification work.

- Reconcile wiss-030 and wiss-031 with current implementation and component evidence; retain specific unresolved acceptance criteria.
- Keep historical eval results unchanged and link follow-up verification separately.
- Do not close an issue solely because a component test passes; identify where a fresh full-workflow eval is still needed.

Suggested order: configuration guidance and atomic scaffolding; log/failure visibility and retained metrics; worker identity and preflight; incremental evaluation and project-context example. Backlog reconciliation can accompany each change.

## Observed issues

### wiss-003: Live execution progress and evidence

The container eval's builder actively edited files and ran tools while its durable attempt log contained only a 133-byte session header. Private progress/usage events were absent from the web snapshot. Completed drafting logs also contained 44,419- and 56,663-byte JSON events: the API's 32 KiB tail discarded those partial records, leaving only `agent_settled` and the check result. See [log shape](evals/go-workflow/runs/2026-10-12-tic-tac-toe/log-shape.json) and [web snapshot](evals/go-workflow/runs/2026-10-12-tic-tac-toe/web-build-snapshot.sse).

Failed-stage reporting has improved: the October 12 CLI returned a nonzero status with the lint error, and the web API retained its stage, attempt, failure, and retry state. Remaining work is useful live output and access to retained evidence.

External coordinating agents are primary consumers of this information. Useful
progress must be accessible through MCP, not just the web UI or private log files.
The CSS eval required read-only Docker/private runtime inspection to discover
nested builder progress; that is evidence of an incomplete public interface.

Responsibility boundaries: this issue defines the normalized progress/evidence
contract and its presentation; wiss-025 owns durable event capture and accounting;
wiss-010 exposes the contract through a thin MCP adapter. CLI, MCP, and web UI must
consume the same application services and retained records. Do not implement
agent-specific inspection or separate progress state in each interface.

Acceptance criteria:

- Expose structured progress, tool activity, gate/repair outcomes, and reported usage during execution through the common workload interface. Keep runtime-specific translation in the image; preserve attribution to execution, stage, attempt, and relevant nested activity.
- Display stage/attempt identity, last recorded activity time, failure cause, pending decisions, and available actions. Distinguish partial/unavailable usage from zero, and a quiet execution from unavailable infrastructure. An idle log alone must not imply a stalled execution; do not invent completion percentages.
- Mark truncation and provide access to full output; do not silently discard oversized records. Preserve earlier attempts when querying Temporal fails.
- Make the same information consumable by an external agent through MCP under wiss-010: current status, a bounded wait for meaningful changes with a reconnect cursor, and scoped retrieval of detailed evidence. Default to useful summaries and references, not full transcript replay.
- Verify active nested-agent output, oversized events, failures, decisions, retries, and reconnect through MCP and the CLI/web views. An external coordinator must follow a task without Docker access, private runtime paths, or knowledge of a particular Agent implementation.
- Use the implemented [secret protection](docs/secret-protection.md) before progress is retained or exposed. Coordinate with wiss-011 and wiss-025; the report and interfaces are views over the same evidence.

The [web-UI incremental eval](evals/go-workflow/runs/2026-10-12-tic-tac-toe-web-ui/README.md) confirms the nested-agent visibility gap: while Ralph iteration 2 was writing source and reporting assistant-turn usage in private outputs, the public task log and UI showed only the outer session and completed iteration 1 summary. Task identity, attempts, and rejection feedback were visible. Stream useful normalized progress from image-owned events rather than presenting quiet outer-process output as the only activity evidence.

### wiss-004: Coordinator lifecycle and approval provenance

The [coordinator skill](resources/skills/watts-coordinator/SKILL.md) and [CLI coordination guide](docs/coordinators.md) now cover creation, initial drafting, evidence-backed progress, decisions, recovery, and reconnecting. Delegated CLI decisions explicitly record the coordinator and authorizing human and require the reviewed execution/attempt/artifact snapshot. The earlier container eval attributed delegated approvals to the OS user; new decisions need not repeat that ambiguity. Identity/delegation labels remain self-declared local provenance, not authentication.

An external supervising agent should be a primary user interface: a user can describe work, receive progress and findings, make decisions, and recover a task through an arbitrary capable agent without operating the Watts CLI directly. Watts owns execution state and policy; the coordinator interprets that state and communicates with the user. Ticket fetching and agent-specific session management remain outside core.

**Verified foundation:** a race-enabled [CLI lifecycle integration test](cli/coordinator_lifecycle_test.go) runs real Temporal and Docker with deterministic workloads: external package/project separation, creation and draft input, submission, rejection/rework, stale approval rejection, worker restart/reconnect, failed build/retry, final review/output, inspection without a worker, and cancellation. It confirms coordinator/authorizer attribution in status, history, and approval receipts. This validates the public CLI contract, not autonomous model behavior; a full external-agent/model eval remains before closure. Cancellation/timeout status and cancelled-task retry handling were corrected during this verification.

Remaining validation:

- Run a complete task through an autonomous external supervising agent using the revised skill, including rejection, failure recovery, reconnection, and final reporting, without requiring the user to operate `wctl`. Include workflow-owned specification refinement and planning; deterministic fixture stages do not validate model judgment.
- Verify that the coordinator reports evidence-backed stage/attempt changes and findings, distinguishes quiet output from failure, presents the exact decision context, and seeks human input unless explicit delegation covers the action.
- Resume with lost conversation history and recover pending decisions from Watts. Document the host's monitoring limits; Watts cannot guarantee unsolicited updates from an idle agent session.
- Retain observed behavior and remaining failures from that eval. Continue treating OS-user labels, explicit actor/authorizer labels, and role checks as prototype provenance rather than verified authentication.
- Coordinate MCP/catch-up work with wiss-010 and decision UX with wiss-012; use the evolving report under wiss-011 for summaries.

### wiss-029: Verify image toolchains and control tool replacement

The container eval's Go image paired Go 1.27.0 with golangci-lint 2.11.3. The linter could not decode Go export-data version 4; its reader supported only version 2. Build installed 2.14.0 privately and passed its checks, but the separate lint Procedure used the original image and failed. See [failure](evals/go-workflow/runs/2026-10-12-tic-tac-toe/lint-toolchain-failure.txt) and [private versions](evals/go-workflow/runs/2026-10-12-tic-tac-toe/build-private-tool-versions.txt).

**Image fix verified:** the Dockerfile now pins 2.14.0 and performs a compile/lint smoke check. The rebuilt image passed the generated application's tests, vet, and configured lint gate. [Follow-up verification](evals/go-workflow/runs/2026-10-12-tic-tac-toe/FOLLOW-UP.md) is separate from the failed eval; the existing task still pins its original image.

Remaining work:

- Repeat the complete Go workflow using the corrected image. Verify build, deterministic checks, and review use the intended tools without ad hoc installation.
- Keep behavioral image checks alongside pinned versions. Compiling the linter with the same Go release does not prove compatibility of its analysis libraries.
- Define a workflow/image policy for tool installation or replacement. Under a fixed-toolchain policy, agents report environmental incompatibility and stop rather than silently install `@latest`.
- Verify enforcement and executable provenance. Read-only image directories alone are insufficient: a writable runtime directory can provide an overriding executable. Keep tool-specific policy out of Watts orchestration; distinguish tool failures from application findings.

### wiss-030: Prepare and surface the image-owned Ralph completion contract

The builder's first iteration marked all stories done and emitted `DONE`, but Ralph rejected `Missing OPEN_QUESTIONS.md`. The launcher/instructions had not prepared or explained that mandatory helper file. A second iteration created it and completed. See [gate evidence](evals/go-workflow/runs/2026-10-12-tic-tac-toe/build-private-evidence/.ralph-runner/events.jsonl).

Acceptance criteria:

- Make required helper artifacts and completion rules explicit in the image. Prepare runtime-owned files privately without fabricating model findings or changing application files.
- Publish exact blocked-gate reasons through common progress/error evidence while the loop runs. Do not present a completion promise as accepted success.
- Run final acceptance checks against completed outputs; distinguish earlier preflight failures from final results.
- Test build and review in the real image without metadata-only recovery iterations. Retain diagnostics after failure/cancellation. Coordinate with wiss-003 and wiss-024.

### wiss-031: Separate workspace source changes from generated build artifacts

Build published a 2,367,736-byte Linux arm64 executable into the macOS project alongside the Go source/tests. The plan's `go build ./...` gate produced it, and workspace publication accepted it as an application change. See [artifact metadata](evals/go-workflow/runs/2026-10-12-tic-tac-toe/published-build-artifact.json) and [workspace evidence](evals/go-workflow/runs/2026-10-12-tic-tac-toe/steps/build-1/workspace.json).

Acceptance criteria:

- Put example verification executables, caches, and temporary reports in private runtime storage; include that policy in planning/build guidance.
- Clarify configurable publication exclusions and declared artifact destinations. Do not hard-code Go binary names or reject all generated files.
- Retain requested deliverables with provenance while excluding incidental tool output from source publication.
- Verify both source-only changes and intentionally requested build artifacts. Coordinate with wiss-022 and wiss-028.

The [web-UI eval](evals/go-workflow/runs/2026-10-12-tic-tac-toe-web-ui/README.md) reproduced publication leakage despite private-output guidance and a correct configured build tool: the Agent also ran a workspace build, and a 9,079,875-byte Linux executable was accepted/published alongside source. The final gate receipt included it in the candidate inventory. Explicit publication policy must cover the whole candidate, not just the declared build command. The original native `ttt` was missing before planning; deletion attribution is unknown and should not be assigned to build without evidence.

## Orchestration and recovery

Temporal remains the workflow engine. Container execution, stable identities, local acceptance receipts, and baseline-aware publication are implemented. The issues below concern remaining guarantees, failure coverage, and distributed deployment—not a second runner.

### wiss-021: Reconcile interrupted executors before retrying

Local Docker execution can reattach using stable identities, stop owned containers on cancellation, and isolate new attempts. Live integration tests cover service/worker restart and cancellation. The old host-runner cleanup incident is historical evidence, not the current execution design.

Remaining work:

- Define recovery after abrupt worker loss, heartbeat timeout, machine sleep, and uncertain container termination. Refuse overlapping writers when ownership or cleanup cannot be confirmed.
- Record cleanup status and preserve partial evidence/candidates. Cancellation must not imply rollback of accepted publication.
- Document worker stop versus task cancellation, shutdown grace, heartbeat limits, and stage limits with actionable diagnoses.
- Inject those failures with real Temporal/container execution; confirm recovery needs no manual runtime-state deletion. Coordinate with wiss-013, wiss-022, and wiss-027.

### wiss-022: Make execution and result publication idempotent

Stable container identities and durable acceptance receipts already prevent repeating accepted local execution. Baseline checks and a publication journal support recovery of partially applied matching changes. Publication is per-file, not an atomic whole-project transaction; external side effects require their own idempotency strategy.

Remaining work:

- Fault-inject crashes before launch, after process completion, during publication, and before Temporal acknowledges completion. Verify receipts, input identities, and output hashes at each boundary.
- Define how canceled/obsolete attempts are prevented from publishing, including uncertain termination and concurrent delivery.
- State the supported guarantees for partial publication and external effects; do not promise general exactly-once execution.
- Extend the same contract to remote artifact storage without introducing Git dependence. Coordinate with wiss-021, wiss-027, and wiss-031.

### wiss-023: Prevent incompatible workers from consuming a run

The earlier eval mixed an installed stale worker with a rebuilt worker on the same queue. Project queue isolation and pinned workload images do not establish worker protocol compatibility.

Acceptance criteria:

- Record worker build/protocol identity and required capabilities. Route submissions to compatible workers using supported Temporal facilities or explicit queue isolation.
- Expose incompatible/no-worker states and required versions in diagnostics.
- Define upgrade and replay policy for in-flight runs and pinned packages; reject unsupported histories rather than reinterpret them.
- Test mixed workers, upgrades during approval waits, and SDK history replay without executing external side effects. Coordinate with wiss-014 and wiss-016; Watts may remain at 0.1.0.

### wiss-024: Separate runtime evidence from application changes

Runtime metadata now lives in private attempt storage, and read-only review uses an isolated workspace. The container eval published application source in the project root without leaking Pi/Ralph directories. It stopped at lint, so final review and rework remain unverified.

Remaining work:

- Complete the Go eval through review and a deliberate build/review loop without manual metadata cleanup.
- Verify real reviewer writes to application files or protected inputs fail, while permitted private bookkeeping and review outputs succeed.
- Classify runtime/gate errors separately from application `changes-required`; infrastructure faults must not cause futile code rework.
- Preserve attempt-specific findings/verdicts and the revision they assessed. Coordinate with wiss-030 and wiss-027.

### wiss-025: Persist complete attempt telemetry across failures

The container eval retained usage for drafting and both nested build iterations: 1,831,396 input and 40,081 output tokens; cached input is included in that input count. Cost remained unknown. [Usage evidence](evals/go-workflow/runs/2026-10-12-tic-tac-toe/usage-summary.json). Post-attempt capture is improved; live visibility and abrupt-loss completeness remain open.

Provide the durable foundation for live progress under wiss-003 and MCP status,
wait, and evidence operations under wiss-010. Event persistence and read services
are transport-independent: MCP must not create its own telemetry store or depend
on disposable container files. Durable capture improves completeness but cannot
recover usage an adapter/provider never reported; surface that limitation.

Acceptance criteria:

- Persist events incrementally outside disposable workload storage. Preserve available usage, logs, and failure/cleanup evidence after cancellation or worker loss.
- Preserve a common event identity and attribution to task/execution, stage, attempt, and relevant nested operation, with event/receipt timestamps. Capture progress, tool lifecycle, gate/repair results, usage, and errors; keep bulk content outside Temporal history.
- Deduplicate by stable response/event identity, including rework and redelivery. Document ordering and cache semantics; missing counters remain unknown.
- Provide bounded queries and retained cursor/catch-up semantics for shared application services. A cursor outside retention produces an explicit resynchronization requirement and current snapshot; it must not silently lose or duplicate observed activity.
- Distinguish reported billing from estimates, with pricing provenance. Unset prices do not imply free execution.
- Redact declared secrets at capture boundaries before persistence, summaries, or exports, following the implemented [secret protection](docs/secret-protection.md); access through MCP does not justify retaining raw credentials.
- Verify cumulative accounting and evidence preservation through failure, cancellation, worker restart, and duplicate event delivery. Check that reconnecting coordinator reads and multiple consumers do not double-count usage or alter workflow execution. Coordinate with wiss-003, wiss-010, and wiss-011.

### wiss-026: Support authenticated remote Temporal connections

Connection settings currently cover local/trusted endpoints without configured TLS or Temporal Cloud authentication.

Acceptance criteria:

- Use official SDK TLS/authentication options and consistent connection deadlines for supported deployment targets.
- Keep service settings outside portable packages and credentials outside manifests, logs, and history. Distinguish worker/service credentials from workload secrets.
- Report authentication, connection, namespace, and worker-compatibility failures separately.
- Verify an authenticated service while retaining usable local defaults.

### wiss-027: Execute portable stages through Temporal

Local execution pins definitions, images, and resources, but requires worker-local project/attempt storage. Distinct-node execution needs durable artifact transport and revision bindings.

Acceptance criteria:

- Orchestrate immutable plans and integrity-checked package/artifact references without rereading authoring files.
- Materialize identical inputs on another node without originating-project absolute paths. Keep large content outside Temporal history.
- Publish a candidate revision before downstream consumption; review and approvals bind to that exact revision.
- Implement backend launch, observation, cancellation, and recovery over the common contract. A Kubernetes adapter would manage Jobs; Temporal would retain routing and waits.
- Verify node loss, repeated completion, interrupted transfers, and a build/review loop. Coordinate with wiss-013 and wiss-016; see [stage communication](docs/stage-communication.md).

## Packages and worker services

### wiss-053: Support corporate image distribution and organization-owned customization

Building the workload images on a second Mac in a corporate network was reported
as a major onboarding blocker. The user encountered certificate trust problems
and found the Dockerfiles difficult to extend with organizational CA certificates
and other files. The exact failing TLS endpoint and download step were not
captured, so do not assign the certificate failure to Docker pulls, a package
manager, or the model gateway without further evidence.

The current setup requires local builds involving upstream images, apt, npm,
and Go downloads. The Pi/Ralph and Go Dockerfiles hard-code their base images,
and the Go tool-install stage has a separate filesystem/trust store from the
final Agent stage. Extending only the final image cannot repair trust failures
that occur while building its prerequisites.

Paths forward:

- Evaluate publishing versioned multi-architecture workload images with digest
  references, allowing teams to import them into approved corporate registries
  or transfer them offline. Keep reproducible source builds available. Whether
  the user's organization permits imported images or requires source builds
  remains an open deployment-policy question.
- Support organization-owned base/derived Dockerfiles with explicit base-image
  selection. Teams should be able to add certificates and arbitrary organization
  files through normal Dockerfile composition without repeatedly editing Watts'
  image sources. Do not invent executable COPY/RUN strings in ACL.
- Document trust separately for the host/Docker image-pull path, each networked
  build stage, and runtime tools such as Node. Install approved CA trust before
  the downloads that need it; preserve TLS verification and keep private keys
  and provider credentials out of image layers.
- Provide a tested corporate-network customization example and a concise setup
  path that does not require ordinary workflow users to assemble every dependency
  ecosystem. Distinguish public image publishing, corporate mirroring, and local
  building instead of treating a remote registry as mandatory.

Acceptance criteria: demonstrate installation from prepared images and a
source-build/customization path with an organizational CA; verify build-time and
runtime trust independently; configure the resulting image through ACL; and
record precise failures when trust or dependencies are unavailable. Coordinate
local image dependencies with wiss-050, toolchains with wiss-029, and preflight
with wiss-039. Keep organization-specific trust and Agent integration in images,
not generic orchestration.

### wiss-006: Shared logging across projects

Worker service logs now live under `~/.watts/runtime/projects/<project-id>/worker/service.log`
following wiss-028. Project-local service logging is no longer the supported path.
Remaining work is rotation, log retention, and cross-project access.

- Preserve project/task/stage/attempt identities and credential protection when aggregating worker-level logs.
- Define rotation, retention, and access to completed attempts without depending on a running project UI.
- Align with the [implemented storage layout](docs/package-runtime-layout.md) and visibility under wiss-003 rather than introducing a separate event ledger.

### wiss-008: Global web UI

Allow one UI to inspect multiple projects and workers independently of any project's service lifetime.

- Keep project roots, task queues, permissions, and execution identities distinct.
- Make disconnected-worker and unavailable-Temporal states explicit while retaining accessible evidence.
- Coordinate with wiss-014 and wiss-028; aggregation must not create another workflow engine.

### wiss-010: MCP interface for coordinators

Expose a thin MCP adapter over the application services used by the CLI/UI. Define a generic coordination contract so an external agent can understand what is happening, what needs attention, and which actions are available. MCP supplies structured operations; the coordinator skill supplies operating guidance. Neither introduces another workflow runner or an agent-specific dependency into core.

Live task observation is a first-class part of this adapter, not an optional web
UI feature. Consume the progress contract from wiss-003 and durable records/read
services from wiss-025. Implement business rules, event selection, accounting,
and evidence lookup once in application services; MCP translates their inputs and
outputs, as do CLI and web adapters.

The initial observation interface should stay small: current task status; bounded
wait-for-change with an opaque cursor; and scoped evidence retrieval for a tool
result, gate, failure, artifact, or detailed output. Tool names and wire schemas
remain implementation decisions. A coordinator must explain progress and decisions
without accessing worker files or invoking runtime-specific commands.

Acceptance criteria:

- Expose a focused lifecycle: task creation, access to declared artifacts, submission, status/evidence inspection, decisions, retry, and cancellation. Artifact operations remain scoped to the task and existing publication rules; the adapter is not a general filesystem or shell interface.
- Return task/execution/stage/attempt identities, configuration identity, state, last recorded activity time, completed work, pending decisions, findings, failures, and available actions. Include evidence references and reported usage where available, distinguishing unavailable data from zero and partial telemetry from complete accounting.
- Describe actions with their preconditions and target identities. Approval requests include artifact hashes/revision, required authority, and relevant findings. Retry explains its target stage and pinned configuration; invalid or stale actions return structured errors and current state.
- Provide a bounded wait-for-change operation with an opaque cursor and meaningful events for progress, decisions, failures, retries, cancellation, and completion. Reuse retained execution state/evidence; avoid busy polling, raw transcript replay by default, or a separate scheduler/broker merely for this interface. Select an event-stream transport only if a concrete client needs it.
- Support reconnect and catch-up from a cursor, stable event identities, and documented duplicate/order behavior. If a cursor is no longer retained, return an explicit resynchronization response with a current snapshot and accessible history. Derive available actions from current state, not stale notifications.
- A disconnected coordinator must not cancel the workflow. Canceling a status wait cancels only that observation. Distinguish a quiet execution from an unavailable worker/Temporal connection, and state the limits of available liveness evidence.
- Handle repeated mutation requests after a lost response without creating duplicate executions or decisions; coordinate with wiss-022. Reject stale decisions and preserve the authorization boundary in wiss-004 and wiss-012.
- Keep responses bounded with pagination or evidence references for large output. Retrieve evidence through execution-bound references and existing access controls, not arbitrary host paths; expose redacted summaries/output using the implemented [secret protection](docs/secret-protection.md). Return structured failures even when a tool call cannot complete; retain accessible evidence where possible during infrastructure outages.
- Test the same application lifecycle and progress facts through MCP, CLI, and web services, plus timeout, reconnect, missed/duplicate events, stale decisions, and unavailable infrastructure. Include an external-agent scenario under wiss-004 that follows nested workload activity using only public operations; do not expose every internal operation.
- Document client setup and capability limits, including hosts that cannot wait or deliver background updates. This exposes Watts to coordinators; workload tool services remain the separate MCP declaration under wiss-034.

### wiss-011: Evolving workflow report

Implemented foundation: the worker generates task-local `report.html` on completion,
terminal failure, or graceful cancellation. It includes actual attempts and rework,
timings, usage coverage, models, capabilities, gates, artifacts, and run provenance.
Remaining work is updating that same report during execution and exposing it to
coordinators and the web UI. Forced termination and crashes still need recovery
reporting and durable telemetry (wiss-025).

Create a readable task report early and update it at stage/attempt boundaries, waits, failures, retries, and completion.

- Include configuration identity, current state, elapsed time, artifacts, verification, errors, next actions, and cumulative reported usage/cost.
- Preserve earlier attempts and rework; do not double-count on restart or confuse missing measurements with zero.
- Derive the report from Temporal state and recorded evidence. It is a view, not a second source of truth.
- Make the same summary and evidence references available to external coordinators, so user-facing updates do not require reconstructing raw logs. Include pending decisions and available actions, with the revision/time of the summarized state; the coordinator checks current preconditions before acting.
- Keep the final report as the completed version of the same evolving record. Coordinate with wiss-003 and wiss-025.

### wiss-014: Watts worker

Workers currently serve one project and depend on local storage. Define a worker service that can handle explicitly registered projects without requiring a separate manually managed process for each.

- Specify project registration, queue routing, concurrency, credentials, and workspace ownership.
- Separate service lifecycle from individual task execution and UI lifetime.
- Evaluate a stateless worker backed by durable task/artifact storage. SQLite or PostgreSQL may support service bookkeeping, but neither replaces Temporal's orchestration database.
- Reuse official clients and storage interfaces; do not add a parallel scheduler. Coordinate with wiss-008, wiss-026, wiss-027, and wiss-028.

### wiss-016: Typed workflow resources and compiled execution packages

ACL workflow, procedure, agent, mcp, configmap, and secret declarations and compilation are implemented, as are reference/default resolution, captured assets, and pinned local execution definitions. Agent/tool behavior remains image-owned. See [ACL declarations](docs/language-spec.md).

Remaining work:

- Compiled JSON now carries the resolved local definition, captured assets, module provenance, and integrity fingerprint. Extend this into portable workload packages with runtime identities and remote artifact delivery.
- Define explicit upstream artifact/revision bindings and publication semantics; coordinate with wiss-027 rather than duplicate its transport work.
- Extend configuration and secret sources only for concrete needs, including protected files or managed stores, scoped delivery, and rotation/revocation. Keep secret values out of packages/history.
- Extend the implemented compile/exec interfaces and schema/runtime compatibility. Resource definitions remain separate from execution records; continuous reconciliation is not implied.

### wiss-034: MCP service lifecycle and deployment extensions

**Implemented foundation:** The `mcp` declaration defines a reusable connection, discovery policy, workload requirements, ConfigMap defaults, and Secret bindings. Agents select it through named tools with per-consumer exposure/allowlists. Selected services resolve into pinned inputs with scoped launch bindings; gopls is a separate MCP resource in Golang coder. The image adapter uses Pi's existing MCP library. Parser/renderer/round-trip tests and native gopls/HTTP integration cover reuse, credentials, tool restrictions, readiness, authentication errors, deadlines, cancellation, and process cleanup without model calls.

Current scopes are attempt-local stdio inside the consuming Agent image and externally managed streamable HTTP. Workload OS/architecture, workspace, and network requirements are checked; unsupported transport/lifecycle values are rejected. Tool exposure is client policy within a trusted image, not independent service isolation.

Remaining work:

- Define separately managed MCP workloads with their own image/command, identity, workspace bindings, and readiness contract. Choose supported placement/backends explicitly; do not fall back to host executables.
- Support shared execution-wide service scopes when needed, including concurrent consumers, access policy, cancellation ownership, cleanup, and interrupted-worker recovery. Temporal still owns workflow execution.
- Add independently isolated service credentials/workspace access rather than relying on co-location within an Agent's container.
- Define remote credential enrollment/refresh where required; interactive OAuth is not part of the current noninteractive adapter.
- Evaluate model-selected gopls use in a fresh workflow run; native protocol/tool integration tests do not establish that an Agent uses the service effectively.

The resource abstracts workload tool services independently of ownership. It is distinct from wiss-010, which exposes Watts operations to coordinators. Coordinate managed/shared lifecycle work with wiss-013, wiss-016, wiss-027, and context delivery under wiss-033.

## Workflow policy questions

### wiss-045: Consider HTML specifications and plans with visual review

**Consideration, not a core format change.** Evaluate HTML as an authored specification/plan artifact for an interactive coding package. Agents can construct diagrams, graphs, charts, tables, and text together, then revise the complete proposal after user feedback. Visual content may make behavior, architecture, dependencies, and verification easier to assess than text alone; this needs evaluation rather than an assumption that HTML is universally preferable.

This may be package-specific Agent/Procedure implementations rather than new Watts primitives. Model-assisted drafting belongs in an Agent; deterministic generation, rendering, and validation can be Procedures or declared Agent tools/gates. Workflow manifests select artifact names and acceptance rules. Keep core independent of document format and retain Markdown workflows as valid choices.

Options to evaluate:

- Native `SPEC.html` and `PLAN.html` authoring, with meaningful diagrams and visual explanations as part of the documents.
- Markdown source with generated HTML review views, where source and presentation identities are linked explicitly.
- One proposal view presenting spec and plan together, while preserving their distinct purposes: requested behavior/acceptance versus implementation/verification.

Evaluation criteria:

- Compare the approaches on representative interactive tasks: human understanding, review effort, agent interpretation, revision quality, token/time use, and rendering failures. Avoid decorative visuals or unsupported claims about agent capabilities.
- Distinguish reading HTML/SVG source from inspecting a rendered page. Declare the browser/rendering tools needed by authors, coordinators, and reviewers; preserve text labels, acceptance criteria, and structured tables for file-only consumers.
- Prefer self-contained documents and inline SVG. Define supported scripts, external dependencies, and rendering behavior so review does not depend on mutable remote assets or execute untrusted document code with broader privileges.
- Bind approval to the complete artifact set, including diagrams and any referenced assets. For generated views, record the source hashes and generation inputs; substantive changes invalidate the relevant approval. Support useful semantic/visual comparison between review revisions.
- Configure prompts, skills, output declarations, and validation through the package/image. The Pi/Ralph example currently assumes `SPEC.md` and `PLAN.md`; a working HTML variant requires adapting those contracts, not merely renaming files. Do not introduce a separate requirements document.
- Demonstrate coordinator presentation, human approval/rejection, and feedback-driven revision before deciding whether any reusable core support is necessary. Coordinate with wiss-004, wiss-012, wiss-016, and wiss-032.

### wiss-012: Approval and task readiness

Document readiness and human approval are different. A ready SPEC.md advances only through its declared workflow policy.

- Evaluate clearer decision UX without adding a task manifest that competes with Temporal state or document hashes.
- Define any override's permitted stages, authorization, and recorded reason before implementing it.
- Preserve exact artifact/attempt binding and delegated decision provenance. Coordinate with wiss-004.
- Represent a pending decision consistently across CLI, UI, and coordinator interfaces: reason for the wait, document/revision identity, review findings, permitted choices, and required authority. Reject an approval targeting superseded evidence and return the current decision context.

### wiss-032: Agent tools, skills, and formal quality gates

Extend the Agent declaration with reusable callable capabilities and explicit completion policy. A tool is something an Agent can call; a gate is a condition that must pass before its result is accepted. A command tool may serve both roles. Skills provide instructions/material, not permission to invoke tools.

The Go example's separate lint stage duplicated the builder's responsibility and added another environment/handoff. Lint and tests belong in the build Agent's configured toolset and gates, with failures returned to that Agent for repair.

Manifest contract:

- `tools`: named entries with one implementation: `command` (fixed `argv`, `working_dir`, `timeout_seconds`) or `mcp: {ref: ...}`. Scripts are commands invoking an interpreter.
- `gates`: named references to command tools with an expected exit code. Callable tools are optional capabilities; every declared gate is required.
- `gate_policy.max_repairs`: a bounded number of repair passes after the initial execution. Omission permits two; zero disables automatic repair.
- `skills`: named sources from an image path or a captured resource/path. Record and deliver them as pinned context independently of tool access.

Acceptance criteria:

- Parse, validate, compile, render, and round-trip these fields, rejecting unknown/duplicate references, mixed implementations, escaped paths, unsupported gates, and unbounded limits.
- Expose command tools through image-owned agent adapters, returning structured status, stdout/stderr, timeout/cancellation information, and retained call evidence. MCP tool references select existing configured servers; the MCP declaration is separate work under wiss-034.
- Evaluate required gates after the Agent finishes against the final candidate. Bind gate evidence to execution identity and workspace content/modes; missing, stale, or failed evidence prevents publication.
- Return failed checks to the same Agent execution for bounded repair, rerun all gates after edits, and fail explicitly when repair is exhausted. Required checks must not mutate the checked workspace; put build products under private runtime storage.
- Keep commands and native agent integration out of generic orchestration. Gate receipts are trusted-executor evidence, not cryptographic attestations against a malicious workload image.
- Remove the standalone Golang coder lint stage; demonstrate build/review command tools, gates, named skills, and MCP selection in source ACL/modules. Document fixed-tool arguments and current limitations.
- Test real command-tool exposure, repair, exhaustion, cancellation/timeouts, and stale/missing evidence without hosted model calls. Follow with a fresh full-workflow eval; do not rewrite the earlier pinned execution.

Implementation now includes the typed contract, ACL resources, Pi/Ralph tool and gate support, final-candidate evidence validation, and the updated example. Compiler/resource tests, real Pi/Ralph command and gate tests, and race-enabled Docker/Temporal integration checks pass. The fresh [hosted web-UI eval](evals/go-workflow/runs/2026-10-12-tic-tac-toe-web-ui/README.md) completed: build final-candidate gates and review verdict validation passed, including internal correction of story/verdict formatting. It also exposed unrelated command-created publication leakage and false semantic review claims (wiss-031/wiss-047). Final-gate repair/exhaustion remain covered by component/integration tests rather than this hosted run. Coordinate with wiss-029, wiss-030, wiss-031, and wiss-033.

## Long-term execution and integrations

### wiss-013: Containerized workloads

The [local container contract](docs/container-workloads.md) is implemented: Agent and executable Procedure attempts share OCI execution, scoped mounts/secrets, private state, and publication. Workflow owns orchestration; ConfigMap/Secret provide inputs. Approval/event waits do not require idle containers.

Remaining work:

- Support additional execution environments deliberately. Podman needs compatibility testing or an adapter; Kubernetes needs scheduling, lifecycle, and artifact delivery. Neither is supported today.
- Strengthen isolation, network policy, identity, resource limits, and credential controls for the intended trust model. Current containers are not a hardened multi-tenant sandbox.
- Preserve evidence and fence publication across node loss and replacement. Coordinate with wiss-021, wiss-022, and wiss-027.
- Keep runtime integrations and tool-private metadata inside images. Storage lifecycle belongs to wiss-028.

### wiss-017: Sandboxed execution in a developer's local environment

Let developers inspect candidate code changes, diffs, outputs, and artifacts live in their editor/UI across build, review, QA, and other stages. Local isolated candidates already exist; safe live inspection and acceptance need an explicit interface.

- Compare container, microVM, and OS-sandbox options with declared filesystem/network/MCP/credential access. A writable mount alone is not a security boundary; see [sandboxing](docs/sandbox.md).
- Keep visible candidate changes distinct from accepted publication. Review/QA normally inspect application code read-only and write separate reports.
- Handle concurrent editor changes, cancellation, partial publication, and invalidated approvals without exposing unrelated host files or privileged sockets.
- Demonstrate inspection, stopping, and intentional application of changes. Coordinate with wiss-013, wiss-022, and wiss-027.

### wiss-018: Source-host integrations

Provide optional GitHub, GitLab, and other source-host adapters for selecting revisions, publishing accepted changes as pull/merge requests, and linking task evidence. Local Watts workflows remain independent of Git.

- Bind external checks/reviews to exact revisions; old approvals cannot advance changed code.
- Make merge/deployment separately authorized actions and define their relationship to workflow approvals.
- Use scoped credentials, verified events, and idempotent publication. Surface integration failures and duplicate delivery without duplicate requests.
- Build adapters over shared application services; coordinate with wiss-010 and wiss-011.

### wiss-019: Ticket intake and automated task initiation

Optional Jira/Linear/other adapters can capture a ticket's identity, source link, title, description, and acceptance criteria into an initial SPEC.md draft. Normal specification refinement and planning still follow the selected workflow.

- Support opt-in triggers such as an “In Progress” transition, label, or command. A ticket transition is not specification approval.
- Define project/workflow mapping, authorization, concurrency, and spending limits before unattended execution.
- Verify/deduplicate events; distinguish redelivery from an intentional new task or later transition. Pin the captured ticket version.
- Report later ticket edits explicitly rather than mutate approved artifacts. Avoid feedback loops when posting progress back to the tracker. Coordinate with wiss-010 and wiss-011; ticket-specific logic remains outside core orchestration.

## Agent knowledge across executions

### wiss-033: Durable, scoped memory for ephemeral agents

An ephemeral builder starts with the declared inputs, current workspace, instructions/skills, and previous-stage feedback. Those inputs provide task context, but they do not automatically preserve useful knowledge from earlier attempts or tasks. A fresh agent may repeat discovery, forget architectural decisions, retry failed approaches, or miss team-specific constraints. Private sessions/caches are disposable runtime state, not a dependable long-term memory contract. This is a design gap, not proof that the lint failure was caused by missing memory.

Preserve useful knowledge as explicit, inspectable inputs while keeping executions isolated and reproducible. Memory can supplement SPEC.md and PLAN.md; it cannot override their scope, approval decisions, tool policy, or current verification results.

Options to evaluate:

- **Versioned project context:** repository/package-owned architecture notes, decisions, conventions, and known environment constraints mounted as declared resources. This is the simplest starting point and works through existing manifests without a new declaration or service.
- **Task handover artifacts:** agents publish concise decisions, failed approaches, remaining questions, and evidence references for later attempts/stages. Summarization may be a configured Agent; deterministic Procedures can validate its structure. Keep the summary separate from the raw evidence it cites.
- **Curated team memory:** promote verified lessons from completed tasks into reviewed/versioned project or team guidance. Require an explicit promotion policy so a transient failure or model conjecture does not become permanent instruction.
- **External retrieval:** a team-managed store or MCP may retrieve relevant knowledge with project/role access controls. Record the retrieved content and version used by each attempt; a mutable query result alone is not reproducible context. Evaluate this only if file/artifact-based context proves insufficient.

Acceptance criteria:

- Define scopes separately: attempt, task, project, and team. Select relevant context explicitly in workflow/Agent configuration; avoid a hidden shared session or unrestricted writable home directory.
- Give entries provenance, evidence references, status (observed, proposed, or verified), and a version/content identity. Define correction, supersession, expiry, and deletion so stale lessons do not remain authoritative.
- Record the exact memory snapshot consumed by an attempt. New runs may opt into updated knowledge; pinned executions must not silently ingest mutable memory on retry. Define how task-local handovers fit that rule.
- Bound retrieval/context size and measure token overhead. Do not replay every transcript or treat model summaries as verified facts.
- Scope access and publication permissions; exclude credentials and prevent cross-project leakage or uncontrolled instruction promotion. Memory remains data subject to the workflow's trust policy.
- Separate durable knowledge from worker retention under wiss-028. Runtime cleanup must not erase published handovers or approved team knowledge.
- Evaluate repeated related tasks and retries with/without the same curated memory. Compare rediscovery, repeated mistakes, correctness, token/time use, and stale-context failures. Keep Watts generic: no Pi-specific session database in core.
