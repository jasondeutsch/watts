# Container workload interface

Watts core is independent of agents, models, and tools. An executable Agent or
Procedure selects an OCI image and command. Watts requires a container execution
environment; the currently supported backend is a local Docker Engine, accessed
through the official Docker Go SDK. Temporal owns routing, retries, approvals,
and external-event waits.

An ordinary command image needs no Watts base image. Use `result: exit-code` for
simple deterministic Procedures. An Agent or a Procedure with custom outcomes
must implement the structured input/result contract. Its launcher can live in
an image you build or extend; no runtime-specific code belongs in Watts core.

## Mounted interface

| Path | Access | Contents |
| --- | --- | --- |
| `/inputs/input.json` | Read-only | Versioned execution input |
| `/inputs/task/` | Read-only | Captured declared task inputs |
| `/inputs/config/` | Read-only | Literal `container.files` |
| `/inputs/resources/` | Read-only | Captured `container.resources` |
| `/outputs/` | Writable | Structured result, event stream, and output artifacts |
| `/outputs/task/` | Writable | Declared task outputs; inputs are seeded here |
| `/runtime/` | Writable | Workload-private settings, caches, tools, and state |
| `/workspace/` | Read-only by default | Isolated copy of application files |

The image root filesystem is read-only, capabilities are dropped, privilege
escalation is disabled, and the user is nonroot. `/tmp` is a bounded, non-executable
tmpfs. Executable scratch files, such as Go test binaries, can use `/runtime/tmp`.
No live project mount, Docker socket, or worker home directory is exposed.

The environment names the interface with `WATTS_INPUT`, `WATTS_OUTPUTS`,
`WATTS_RESULT`, `WATTS_EVENTS`, and `WATTS_WORKSPACE`. `HOME=/runtime/home` and
`TMPDIR=/runtime/tmp`. Only explicitly declared environment and secret bindings
are added to the image environment.

## Input and result

`input.json` contains `version: 1`, an opaque `execution` identity, `task`, `step`,
`attempt`, optional `parameters`, hashed `inputs`, declared `outputs`, and
`previous` result/feedback. Agents additionally receive an `agent` object with
name, identity, provider, model, prompt, resolved `mcp_services`, and descriptive settings. Selected MCP service configuration and scoped Secret bindings are pinned; credential values are supplied only at launch. The workload adapter must honor transport, discovery/consumer restrictions, request limits, and cleanup.
The `task` field is a logical project identifier; use mounted paths for file access.

The command reads its configuration, performs work, and writes `/outputs/result.json`:

```json
{"outcome":"success","artifacts":["report.md"],"feedback":"Checks passed"}
```

Optional `data` holds structured output. Artifact names identify files beneath
`/outputs/task/`. The result must contain one JSON object, use a declared outcome,
and name only declared outputs. Exiting zero without a required result is a
failure. Nonzero exit, invalid output, failed checks, and error events fail the
attempt and prevent application publication.

With `result: exit-code`, exit zero supplies the outcome `success`; declared
outputs are still validated. Agent workloads always require a result file.

## Agent capabilities and gate evidence

The Agent input also carries `tools`, `gates`, `gate_policy`, and `skills`; see [the manifest fields](language-spec.md#agent-tools-gates-and-skills). Workload adapters expose tools/skills natively and implement gate repair. Watts core does not register Pi tools or interpret Ralph state.

When gates are declared, the executor must run every gate against a stable final candidate and write `/outputs/gates.json`:

```json
{"version":1,"execution":"<input execution>","workspace":{"main.go":{"sha256":"<content hash>","mode":420}},"task_outputs":{},"results":[{"name":"lint-clean","tool":"lint","status":"passed","exit_code":0,"evidence":"tools/<call-id>.json"}]}
```

`workspace` contains every regular candidate file and its permission bits, using the same snapshot contract as publication. `task_outputs` snapshots the files under `/outputs/task` using the same hash/mode representation. Checks must leave both candidate and published task outputs unchanged. Each referenced call record contains `tool`, fixed `argv`, resolved `working_dir`, execution `status` (`exited`, `error`, `timed_out`, or `canceled`), nullable `exit_code`, stdout/stderr, duration, truncation, and evidence path. An exited command may still fail a gate. Partial output is bounded and marked as truncated.

The worker checks execution identity, complete gate coverage, expected exit codes, declared commands, and equality with the final candidate and task outputs. Missing/stale evidence fails before publication. Legacy post-checks cannot invalidate a gate silently by changing the candidate afterward. Gate reports and tool records are retained with attempt evidence, including failed attempts. No standalone workflow stage or duplicate checker container is required.

Receipts are evidence from the trusted workload executor, not attestations against a malicious image or unrestricted process. A gated Agent image must implement this interface; simply writing a normal result file is insufficient.

Tool/gate events use common event records with `details` containing call/gate identity, status, and evidence references. Recoverable tool/gate failures are not terminal `error` events; exhausted repair or an unrecoverable executor failure produces a terminal error. Reported model usage includes repair passes.

## Events and usage

`/outputs/events.jsonl` is optional. Each line needs a stable `id` and `type`:

```json
{"id":"turn-1","type":"usage","usage":{"provider":"example","model":"coding","input_tokens":100,"output_tokens":20}}
{"id":"progress-1","type":"progress","message":"Running tests"}
{"id":"error-1","type":"error","message":"Provider rate limited"}
```

Repeated identical IDs are deduplicated; conflicting IDs fail validation. An
`error` event fails even a zero-exit command. Events and reported usage are retained
on failed attempts. Missing counters and prices remain unknown. Input includes
cached input when the runtime can account for it; cache counters are a breakdown,
not extra tokens to add to input. Watts does not invent provider prices.

The worker saves event evidence, a usage summary, container identity/exit records,
input, result, workspace hash manifests, acceptance receipt, and logs under `<task>/.watts-state/steps/` and
`logs/`. Private attempt directories live under `~/.watts/runtime/`. Full retention
and archival policy remain tracked separately in [issues](../issues.md).

## Publication and recovery

Images and resource contents are captured at submission. Each attempt captures
its input files and candidate workspace before launch. Editing a tag, template,
or package afterward does not alter that execution.

Containers have stable execution identities. Activity redelivery reattaches to an
existing container or reads its completed state; it does not restart a completed
process. Durable acceptance receipts also prevent execution after container cleanup.
Cancellation stops the owned container. A failed attempt stays available for
inspection; an explicit retry receives a new attempt environment.

After validating the result and checks, Watts publishes declared task outputs.
A `workspace: write` candidate is published only where the live application still
matches its captured baseline or already matches the same candidate. Conflicting
human edits are preserved and reported. Publication uses confined, atomic file
writes and can resume partially applied matching changes. This is per-file
publication, not an atomic replacement of an entire repository.

## Runtime-specific images

[Pi/Ralph](../registry/pi_ralph/README.md) translates the common input into Pi
settings and runs Ralph as a Pi tool. Its configuration, extensions, nested usage,
registry adaptation, and completion checks live in the image source. The
[Go image](../registry/coder_golang/README.md) extends it with a toolchain, lint,
and gopls. The [Go package](../resources/examples/coder_golang/README.md) declares
the team's skills, checks, credentials, and routing.

## Execution backends and current limits

The workload contract separates image behavior from the execution backend. Podman
is a potential backend that requires compatibility testing or an adapter.
Kubernetes would require a backend for workload scheduling, input delivery,
observation, cancellation, and output collection. Neither is currently supported;
using OCI images alone does not establish backend compatibility. Temporal would
continue to own workflow routing and durable state.

The implemented backend uses local Docker with bind-mounted attempt storage. The worker and
Docker daemon must share that filesystem; remote daemons and distinct-node
scheduling are not supported. Containers reduce accidental host access but do
not constitute a hardened multi-tenant sandbox. Outbound `bridge` networking is
not an egress allowlist. Kubernetes scheduling, immutable artifact transport,
strong identities, network policy, and storage lifecycle remain future work;
see [stage communication](stage-communication.md) and [sandboxing](sandbox.md).
