# Completed and superseded issues

Closed issues are recorded here when removed from [issues.md](issues.md). Preserve issue numbers; do not infer completion from partial work.

For historical entries, the closure date is the date of the backlog-removal commit, not an inferred implementation date. Verification records describe retained evidence; tests were not rerun for this backfill. Superseded proposals are explicitly distinguished from completed work.

## wiss-052: Keep host runtime paths under `$HOME/.watts/` and separate execution contexts

**Status:** Completed · **Completed:** 2026-10-15

### Findings and changes

- Tracing the current executor found host paths derived from the host HOME and
  container `/runtime/task` passed only to image-owned commands. No current
  host-launch path reproducing the other-machine report was identified.
- Validate host HOME before resolving state: require an existing absolute user
  directory, reject filesystem-root/container-runtime values and aliases, and
  retain support for a non-default home.
- Validate final attempt paths against symlink redirection before preparation;
  parent execution bookkeeping alone did not cover the final attempt path.
- Reject relative, root, absent, non-directory, or container-only Docker bind
  sources before daemon calls. Preserve per-attempt mounts and separate container
  HOME/TMPDIR, without mounting the whole host home or changing image conventions.
- Annotate host filesystem errors with operation, path, and category while
  preserving `errors.Is/As`. Distinguish EROFS from permission denial; label Docker
  creation/startup failures separately from workload process failures.
- Keep retention's missing-record handling compatible with wrapped errors.
- Document exact host/container mappings and diagnostic interpretation.

### Verification

- Full Go suite passed.
- Docker Engine and application integration suites passed, including an ordinary
  user on this Mac with an alternate HOME, private container `/runtime/task` and
  HOME writes, unchanged host HOME, accepted-attempt reattachment, publication,
  cancellation, and retained evidence.
- Public CLI Docker/Temporal lifecycle passed again after these changes, covering
  worker restart, failure/retry, approvals, completion, and cancellation.
- Regression tests cover invalid HOME/aliases, attempt symlink redirection,
  invalid mount sources before Docker access, actual host permission denial,
  EROFS/access distinction, and retained wrapped error identity. Retention tests
  passed under the existing home layout.

### Limitations

- The original command/process/trace from the other machine was unavailable.
  This verifies the current path contract and hardening; it does **not** establish
  the cause of that abandoned test or prove it fixed on that machine.
- Validation used isolated fresh homes/projects on an existing Mac, not a newly
  provisioned machine. Mount validation does not solve distributed filesystems or
  remove every possible concurrent filesystem race.

### Documentation

- [Host/container paths](docs/package-runtime-layout.md#host-and-container-paths)
- [Container interface](docs/container-workloads.md)

## wiss-051: Reduce task setup and execution mental overhead

**Status:** Completed · **Completed:** 2026-10-15

### Changes

- Add `wctl new <directory> --from <package>` to scaffold a team-owned importing
  package with typed inputs, resolved editable values, an entrypoint export, and
  usage guidance. It creates files only and never overwrites an existing directory.
- Let `wctl exec <package>` compile, capture, and submit directly. Explicit compile
  and frozen-artifact execution remain available; selected workflow identity is
  recorded in artifacts and printed with the package/task at submission.
- Add mutually exclusive request-text/file/stdin inputs, preserving file bytes
  without shell argument-size dependence and reporting read errors explicitly.
- Normalize new mixed-case/spaced task names into safe slugs; retain millisecond
  ordering and collision arbitration. Existing paths are never normalized.
- Distinguish package directories from tasks and reject source/request options on
  retries, which retain captured policy and assets.
- Update the runbook, language spec, example, coordinator skill, and setup guidance.

### Verification

- Full Go suite passed, including package scaffolding, namespaced example reuse,
  missing inputs, request-file errors, multiline input, and pinned task behavior.
- Isolated public CLI Docker/Temporal lifecycle passed with a large request and
  mixed-case task name, direct package execution, source edits after submission,
  rejection/rework, stale decisions, worker restart, failure/retry, completion,
  inspection, and cancellation. No hosted inference or eval product was tested.
- Coordinator frontmatter and diff checks passed. The Python skill validator
  could not run because PyYAML was unavailable; YAML was checked using Ruby.

### Limitations

- Scaffolding references a local dependency; it does not vendor/update modules.
- Worker/Temporal and images remain explicit operational prerequisites (wiss-014).
- Non-ASCII/punctuated task names are not supported; empty request files fail.

### Documentation

- [Runbook](docs/runbook.md)
- [Coordinator](resources/skills/watts-coordinator/SKILL.md)

## wiss-050: Make local workload image builds reliable on a fresh machine

**Status:** Completed · **Completed:** 2026-10-15

### Changes

- Add `make images`, backed by Docker Bake's explicit Pi/Ralph -> Go image graph.
- Supply the unpublished Agent base as a target context instead of resolving a
  Watts tag through a remote registry; load and inspect both final local tags.
- Check Engine/context agreement, Docker driver, native platform, and output
  availability. Report dependency steps and stop on failure without changing
  Docker context/builder defaults.
- Update source-build guidance in the runbook, image READMEs, root README, and
  standalone setup guide. Keep image building separate from package authoring.

### Verification

- The full graph built and loaded both isolated `wiss-050-check` image tags on
  this native ARM64 Docker Engine, without replacing the normal 0.1.0 tags.
- Eight script tests passed, covering success, graph failure, missing Buildx,
  context/Engine mismatch, wrong driver/platform, and missing/wrong output images.
- Shell syntax and diff checks passed.

### Limitations

- Verification used an existing ARM64 builder cache, not a clean-machine build;
  native AMD64 was unavailable. Native AMD64 remains unverified.
- Docker Buildx and the context-owned Docker driver are required for this path.
- Upstream image/package network access remains necessary. Corporate distribution
  and CA customization remain open under wiss-053.

### Documentation

- [Image builds](registry/README.md#build-locally)
- [Runbook](docs/runbook.md)

## wiss-049: Make Agent configuration explicit through global and namespaced variables

**Status:** Completed · **Completed:** 2026-10-15

### Changes

- Give `coder_golang` typed `agents`, `providers`, and `globals` inputs.
- Bind each Agent's provider profile, credential source, model, thinking, image,
  and model capabilities explicitly in ACL; only an empty image selects the
  shared image. Provider profiles keep endpoint, protocol, and credentials together.
- Export scoped Secret references from the provider module without reading keys.
- Document whole-variable replacement and independent configuration in the example,
  language specification, and standalone setup guidance.

### Verification

- Compiler and manifest suites passed; the default example compiled successfully.
- Consumer-module coverage uses two providers, endpoints, credential sources,
  models, reasoning settings, images, and model limits.
- Tests cover shared-image changes, missing bindings, compiled round trips, and
  absence of credential values from captured configuration.

### Limitations

- Model metadata is declared by the team, not discovered from providers.
- Overrides replace top-level objects in full; no automatic deep merge.
- Configuration tests do not establish compatibility with every provider/image.

### Documentation

- [Coder example](resources/examples/coder_golang/README.md)
- [Language specification](docs/language-spec.md)

## wiss-048: Prevent secrets leaking into logs, telemetry, and exported evidence

**Status:** Completed · **Completed:** 2026-10-13

Protect declared credentials at workload capture and publication boundaries, with image-owned protection before tool results enter model context.

### Changes

- Redact stdout/stderr across streaming chunk boundaries and diagnostic errors.
- Sanitize retained results, events, tool evidence, and completion reports.
- Sanitize Pi and nested Ralph tool results, provider context, and telemetry.
- Reject credential-bearing source and task artifacts before publication.
- Add private-state ignore rules during task creation and worker startup.
- Document image responsibilities, evidence sharing, and incident cleanup.

### Verification

- Full Go test suite passed.
- Adapter unit tests passed; optional native MCP integration tests were skipped.
- Container tests verified Pi and nested Ralph context with synthetic credentials.
- SDK activity logs and actual Temporal history retained sanitized failures.
- Synthetic evidence archive and report checks passed.
- Git tracking checks excluded private state and preserved authored ACL and reports.

### Limitations

- Matches known declared values and common escaped forms, not arbitrary transformations.
- Does not scrub historical leaks or rotate credentials.
- Private workload files and container-engine storage require separate protection.
- Updated binaries and derived images must be rebuilt; workers must be restarted.

### Documentation

- [docs/secret-protection.md](docs/secret-protection.md)

## wiss-001: Build execution and completion evidence

**Status:** Completed · **Closed:** 2026-10-12

**Closure commit:** `726265daa5b89852c5033d260f9adc71fff1d299`

Preserve Ralph extension-command arguments and recognize nested execution evidence without requiring an ordinary Pi session record.

### Verification

- Historical October 12 container eval accepted and published the build after two Ralph iterations.
- The historical eval README is retained in the source commit under evals/go-workflow/runs/2026-10-12-tic-tac-toe/README.md.

### Limitations

- That eval failed at lint and never reached review; it did not establish full workflow success.
- Remaining runner-contract and telemetry work is tracked in wiss-030 and wiss-025.

## wiss-002: Application code created inside the task directory

**Status:** Completed · **Closed:** 2026-10-12

**Closure commit:** `726265daa5b89852c5033d260f9adc71fff1d299`

Separate application work from task documents. Workloads operate on the project candidate mounted at /workspace; task inputs and outputs have separate mounts and explicit publication paths.

### Verification

- The historical container eval published application code to the project workspace.
- Container publication tests cover project changes and separate task outputs.

### Limitations

- Publication of incidental generated binaries remains tracked in wiss-031.

## wiss-005: Duplicated kit directory name

**Status:** Completed · **Closed:** 2026-10-12

**Closure commit:** `726265daa5b89852c5033d260f9adc71fff1d299`

Remove the duplicated internal/kit/kit layout as part of moving agent-specific assets and preparation out of core and into workload images.

### Verification

- Core no longer contains an internal/kit package.
- Architecture tests reject core dependencies on runtime-specific kit packages.

### Documentation

- [registry/pi_ralph/README.md](registry/pi_ralph/README.md)

## wiss-007: Executor abstraction

**Status:** Completed · **Closed:** 2026-10-12

**Closure commit:** `726265daa5b89852c5033d260f9adc71fff1d299`

Execute generic container workloads through a common input, result, evidence, cancellation, and publication contract. Pi and Ralph integration belongs inside an image rather than Watts core.

### Verification

- Generic Procedure and Pi/Ralph image integration tests exercise the shared execution path.
- Architecture tests reject core imports of agent-specific adapters.

### Limitations

- Local Docker Engine is the supported backend; this does not imply Podman or Kubernetes support.
- Distributed workload execution remains open under wiss-013 and wiss-027.

### Documentation

- [docs/container-workloads.md](docs/container-workloads.md)

## wiss-009: Worker terminal output

**Status:** Completed · **Closed:** 2026-10-10

**Closure commit:** `13ce24f`

Use the Temporal SDK structured logging adapter for readable attempt boundaries, failures, cancellation, and worker shutdown, with next-action guidance and debug heartbeats excluded from normal output.

### Verification

- The closure commit added worker_logging.go and worker_logging_test.go.
- Worker and orchestration logging tests cover the implemented logging foundation.

### Limitations

- Detailed live Agent progress and evidence remain open under wiss-003.
- Concise failure presentation remains open under wiss-037.

## wiss-015: TOML for configuration and workflow authoring

**Status:** Superseded · **Closed:** 2026-10-10

**Closure commit:** `13ce24f`

The TOML proposal was replaced by YAML authoring and templates. The project subsequently adopted ACL for source authoring; compiled execution contracts remain machine-facing JSON. Do not interpret closure as current TOML support.

### Verification

- The closure commit replaced authoring formats with YAML and removed the TOML issue.
- Commit a596dc7 introduced the current ACL authoring implementation and specification.

### Documentation

- [docs/language-spec.md](docs/language-spec.md)

## wiss-020: Manifest-driven runtime preparation; remove legacy initialization

**Status:** Completed · **Closed:** 2026-10-12

**Closure commit:** `726265daa5b89852c5033d260f9adc71fff1d299`

Remove legacy init and unconditional project-wide Agent installation. Create private state as needed and obtain runtime dependencies from selected workload images, with Agent-specific preparation owned by those images.

### Verification

- The closure commit removed legacy Agent/setup commands and added container CLI coverage.
- CLI lifecycle and workload tests cover external package/project separation and deterministic Procedures.
- Image tests cover Pi/Ralph execution without host Agent installation.

### Limitations

- Package scaffolding later evolved into ACL authoring; historical template commands are not current usage instructions.
- Images must be explicitly built or pulled; worker startup does not install an Agent stack.

### Documentation

- [docs/runbook.md](docs/runbook.md)
- [docs/container-workloads.md](docs/container-workloads.md)

## wiss-028: Separate Watts packages, task executions, and worker runtime

**Status:** Completed · **Completed:** 2026-10-13

Separate authored ACL packages, durable task captures/evidence, and disposable
host execution files. New projects no longer need a project `.watts/` directory.

### Changes

- Store shared worker preferences in `~/.watts/config/settings.json`.
- Move service records, detached logs, and workspace/service locks into home-directory project state.
- Register canonical application roots independently of authored package locations; resource modules do not become application roots.
- Capture compiled package assets under each task's private state and select named workflows with `compile -workflow`.
- Retain every attempt of the latest execution per task by default; `all` disables runtime pruning.
- Protect active leases, uncertain publication, incomplete evidence, and unreadable records from cleanup.
- Persist terminal reports and sync attempt logs before runtime becomes eligible for deletion.
- Show `finalizing` until Temporal confirms closure, preventing premature worker shutdown.
- Reject shared global task-queue overrides and explain setup with existing project-local settings/workers without silently migrating or deleting evidence.

### Verification

- Full Go suite and race checks passed.
- Isolated Docker/Temporal integration tests covered worker restart, approvals, retry, publication, cancellation, and synthetic secret protection.
- The public CLI lifecycle test passed with the race detector, including inspection after the worker stopped.
- Retention tests covered multiple tasks/projects, active and incomplete executions, repeated cleanup, restart ordering, counter recovery, and symlink redirection.
- Task creation tests covered external package roots, multiple named workflows, immutable compiled inputs, task-owned assets, and absence of project `.watts/`.
- Coordinator skill frontmatter and updated documentation links were checked.

### Limitations

- Local filesystem task evidence remains the durable application record; distributed artifact storage is separate work.
- Ambiguous/interrupted state is preserved, even above the retention limit; cleanup does not force recovery or erase uncatalogued historical runtime directories.
- Service-log rotation, retention, and cross-project access remain open under wiss-006.

### Documentation

- [Storage layout](docs/package-runtime-layout.md)
- [Runbook](docs/runbook.md)
- [Temporal operations](docs/temporal.md)
