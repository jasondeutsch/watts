# Kinds and manifest fields

A package renders a bundle of YAML documents. Each resource has `kind`,
`schema_version: 1`, `name`, and `spec`. Names use lowercase letters, digits,
and hyphens. References are bundle-local. A runnable bundle contains one Workflow.

| Kind | Responsibility |
| --- | --- |
| Workflow | Ordered placements, routing, and task scaffolds |
| Procedure | Container execution, approval, or external-event wait |
| Agent | A container workload with identity, model, prompt, skills, and MCP configuration |
| MCP | Reusable tool-service connection, lifecycle, discovery, and workload requirements |
| ConfigMap | Named nonsecret defaults for Agent, Procedure, or MCP specifications |
| Secret | Named worker environment-variable sources; never literal credentials |

Agent specializes the executable Procedure contract. Watts runs both through
[the same container interface](container-workloads.md). Approval and event waits
remain in Temporal; ConfigMap and Secret resources supply configuration.

## Workflow

```yaml
kind: Workflow
schema_version: 1
name: team
spec:
  version: 1
  steps:
    - name: build
      use: {kind: Agent, name: builder}
    - name: verify
      use: {kind: Procedure, name: checks}
      on_failure: build
```

`spec.steps` determines order. A placement has required `name` and `use: {kind,
name}`. Optional fields are `parameters`, `next`, `on_failure`, `transitions`,
and `optional`. `transitions` maps declared outcomes to placement names or `end`;
it is exhaustive. Without it, a successful result advances in order, unless
`next` is set. `on_failure` routes execution errors. Unknown targets and
unreachable placements are rejected.

`max_transitions` bounds loops. `templates` maps task-relative artifact names
to scaffold text. Paths cannot escape the task or overwrite `.watts-state`.
Task creation renders a resolved `workflow.yaml`; first submission pins its
execution definition, image IDs, resource captures, and worker preferences.

## Shared workload contract

Agent and Procedure specifications share these fields:

| Field | Meaning |
| --- | --- |
| `container` | OCI image, command, access, limits, and mounted configuration |
| `inputs`, `outputs` | Task-relative files consumed and published |
| `pre_checks`, `checks` | Lists of `{command: [executable, argument, ...]}` run before/after the main command in the same image and candidate workspace |
| `timeout_seconds`, `max_attempts` | Execution timeout and explicit-attempt budget |
| `environment` | Nonsecret environment values |
| `secrets` | Target environment names mapped to `{name, key}` Secret references |
| `config_ref` | ConfigMap supplying specification defaults |

Checks succeed only on exit zero. Failed checks prevent publication. Inputs not
also declared as outputs are protected. Reported artifact paths must be declared
outputs. Credentials are resolved at launch; values do not enter the pinned plan.
Watts reserves `WATTS_*`, `HOME`, `TMPDIR`, `PATH`, `TERM`, and `LANG` targets.

### Container

```yaml
container:
  image: team/tools:0.1.0
  command: [go, test, ./...]
  workspace: read-only
  result: exit-code
  network: none
  memory_mb: 2048
  cpus: 2
  resources:
    team: "{package}"
  files:
    tool.json: '{"mode":"strict"}'
```

`image` and an explicit `command` array are required. Defaults are `workspace:
read-only`, `network: none`, and `result: file`. `network: bridge` enables outbound
connectivity. `workspace: write` permits candidate changes and their validated
publication; it never mounts the live application for writing.

`working_dir` must be inside `/workspace` or `/runtime`; it defaults to
`/workspace`. `user` may select a nonroot image user; the default is the worker's
numeric UID/GID. A custom user must have access to its attempt directories.
Resource limits are optional and nonnegative.

`files` places literal configuration under `/inputs/config/<name>`. `resources`
selects project-relative files/directories, captured at submission and mounted
under `/inputs/resources/<name>`. `{package}` resolves to captured package assets
when rendering a package. Parent traversal, unsafe artifacts, and unsupported
workspace symlinks are rejected. The local implementation limits individual
files to 64 MiB; structured results are limited to 1 MiB.

## Procedure

```yaml
kind: Procedure
schema_version: 1
name: checks
spec:
  execution: {type: container}
  container:
    image: team/tools:0.1.0
    command: [go, test, ./...]
    result: exit-code
```

`execution.type` is `container`, `approval`, or `event`. Container Procedures
may use a structured result for custom outcomes, or `exit-code` for ordinary
commands. There are no host commands, kit-script selectors, or capability registry.

Approval Procedures require `inputs` and wait for a decision tied to their
hashes. They cannot produce outputs or checks. Event Procedures require declared
transitions and wait for an event tied to a placement and attempt. Neither wait
may declare a container. See [workflow execution](workflows.md).

## Agent

An Agent requires `identity`, `provider`, `model`, `prompt`, and `container`,
directly or through ConfigMap defaults. Agents must publish a structured result.

```yaml
kind: Agent
schema_version: 1
name: builder
spec:
  identity: team-builder
  provider: openrouter
  model: deepseek/deepseek-v4-flash
  prompt: Implement the requested change in /workspace.
  container:
    image: watts/pi-ralph:0.1.0
    command: [node, /opt/watts/launcher.mjs]
    workspace: write
    network: bridge
  auth:
    env: OPENROUTER_API_KEY
    secret: {name: credentials, key: api-key}
```

Optional descriptive fields are `base_url`, `api`, `thinking`, and `skills_dirs`.
Watts sends these in the common input envelope; the image interprets them.
Skill paths must be available inside the image or a declared resource mount.
`identity` is an evidence label, not authenticated access control. Build and
review may use the same model. MCP connections belong in named MCP resources;
Agents select them through their `tools` entries.

## Agent tools, gates, and skills

A tool defines a callable capability. A gate defines a required completion condition using that tool. Skills supply instructions and supporting material; loading a skill does not grant tool access.

```yaml
# Fields inside an Agent spec:
tools:
  - name: lint
    description: Check team lint policy.
    command:
      argv: [bash, /inputs/resources/team/scripts/lint.sh]
      working_dir: /workspace
      timeout_seconds: 300
  - name: code-intelligence
    mcp: {ref: gopls}
gates:
  - name: lint-clean
    tool: lint
    expect: {exit_code: 0}
gate_policy:
  max_repairs: 2
skills:
  - name: go-builder
    source: {resource: team, path: skills/go-builder}
  - name: spec-guidance
    source: {image_path: /opt/watts/kit/skills/spec-template}
```

Command arguments are fixed arrays, not model-supplied shell strings. Scripts use an explicit interpreter. `working_dir` defaults to `/workspace` and must remain inside `/workspace` or `/runtime`; timeout defaults to 300 seconds and is capped at 86,400. Tools inherit workload permissions and explicitly bound environment/credentials. Native file/edit/shell tools remain an image policy; declaring these capabilities is not a complete sandbox or tool allowlist.

An `mcp.ref` selects a bundle-local MCP resource. It may expose multiple native tools. Optional `exposure` (`direct`, `deferred`, or `codemode`) selects how the image presents them; the Pi adapter defaults to `deferred`. `allow_tools`, when provided, must be a nonempty list of exact tool names; omission inherits the service policy. It narrows that policy and cannot broaden an MCP allowlist. Select each service only once per Agent; reuse across Agents is supported. Missing references, conflicting selections, and incompatible access requirements are errors. MCP gate predicates and model-selected command arguments are not supported yet.

All declared gates are required and currently accept command-tool exit codes (default zero, configurable 0–255). The image executor runs them after Agent completion and returns failures for repair inside the same execution. `max_repairs` excludes the initial pass; omission allows two repairs, zero allows none, and the maximum is ten. Each command timeout and the enclosing stage deadline still apply. Gates must leave application files unchanged; a mutation invalidates the suite. Command results and a final workspace-bound gate report are required before Watts accepts publication.

A skill selects one source: an absolute `image_path` outside workload mounts, or a `resource` key declared under `container.resources` plus a relative `path`. Captured resource hashes pin package-owned skills; the pinned image supplies image-owned skills. Source directories must exist when the executor loads them. The adapter exposes them through the runtime's native skill mechanism. These fields supplement the existing descriptive settings; the [Go example](../resources/examples/coder_golang/values.yaml) uses named skills and tools.

## MCP

```yaml
kind: MCP
schema_version: 1
name: gopls
spec:
  description: Go workspace navigation and diagnostics
  connection:
    transport: stdio
    command:
      argv: [gopls, mcp]
      working_dir: /workspace
    timeout_seconds: 60
  lifecycle: attempt
  tools: {discovery: all}
  requirements:
    os: [linux]
    arch: [amd64, arm64]
    workspace: read-only
---
kind: Agent
schema_version: 1
name: builder
spec:
  # Other required Agent fields omitted from this excerpt.
  tools:
    - name: code-intelligence
      mcp:
        ref: gopls
        exposure: deferred
        allow_tools: [go_workspace, go_search]
```

| Field | Meaning |
| --- | --- |
| `connection.transport` | `stdio` or `streamable-http`; legacy SSE is unsupported |
| `connection.command` | Stdio `argv` and optional `working_dir`; executable belongs in the consumer image |
| `connection.url` | External HTTP(S) endpoint, without URL credentials or fragment |
| `connection.timeout_seconds` | Required 1–3600 seconds; bounded readiness probe and native request timeout |
| `lifecycle` | `attempt` for stdio, `external` for remote endpoints |
| `tools.discovery` | `all` discovers server tools; `allowlist` requires a nonempty `allow` of exact names |
| `environment`, `secrets` | Stdio server literals and `{name, key}` Secret references |
| `headers`, `header_secrets` | HTTP header literals and `{secret: {name, key}, prefix}` credential bindings |
| `requirements` | Optional allowed workload `os`/`arch`, minimum `workspace` access, and outbound `network` requirement |
| `config_ref` | ConfigMap supplying shallow defaults |

Stdio runs inside the selected Agent image and inherits its workspace/container
permissions. It does not use a host-installed program or an independently
selected server image. `requirements.workspace` declares minimum access, not a
new isolated mount: `none` imposes no workspace requirement, `read-only` works
with either consumer mode, and `write` requires `container.workspace: write`.
Remote endpoints and services requiring networking need `container.network:
bridge`. OS/architecture checks use the actual workload image, not the Docker
host; a macOS worker may therefore run Linux gopls normally. Remote requirements
refer to the consuming client environment, not unverifiable remote-server facts.

The current adapter probes initialize/tool discovery before invoking the model,
fails on connection/authentication/timeout/unavailable-tool errors, and closes
the probe. Pi's MCP library owns actual session connections, tool exposure,
request handling, and process-group shutdown. A Ralph attempt can open multiple
session connections; servers must support restart. Stdio children are confined
to that attempt and stopped with their sessions/container. Watts does not stop
an externally managed HTTP service. Shared execution-wide services, separately
managed server containers, and interactive OAuth enrollment are not implemented;
unsupported lifecycle values fail validation.

For example, a remote service can bind credentials without embedding them:

```yaml
kind: MCP
schema_version: 1
name: team-docs
spec:
  connection:
    transport: streamable-http
    url: https://docs.example.com/mcp
    timeout_seconds: 30
  lifecycle: external
  tools: {discovery: allowlist, allow: [search_docs]}
  header_secrets:
    Authorization:
      secret: {name: team-credentials, key: docs-token}
      prefix: 'Bearer '
```

The compiler resolves selected services into the pinned Agent input and records
scoped credential-source bindings; the worker reads values only at launch. The
adapter writes environment placeholders into private native configuration.
Literal environment/headers cannot use `${...}` or `!command` interpolation;
use Secret bindings instead. Pinning a remote URL/configuration does not freeze
its implementation or responses. Tool exposure limits what the MCP client
presents, not what an arbitrary trusted workload image could do. Credentials
and native shell/file tools share that image's trust boundary; separate service
isolation remains future runtime work.

See the [Golang coder MCP example](../resources/examples/coder_golang/mcp/README.md).

## ConfigMap and Secret

```yaml
kind: ConfigMap
schema_version: 1
name: common
spec:
  data:
    provider: openrouter
    model: deepseek/deepseek-v4-flash
---
kind: Secret
schema_version: 1
name: credentials
spec:
  env:
    api-key: OPENROUTER_API_KEY
```

A `config_ref` applies a shallow specification overlay: explicit fields replace
whole default fields. Resolved specifications receive the same strict validation.
Values rendering supports nested merges before this resolution. Secret entries
name environment sources available to the worker; export them before `wctl start`.
