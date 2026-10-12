# Kinds and manifest fields

A package renders a bundle of YAML documents. Each resource has `kind`,
`schema_version: 1`, `name`, and `spec`. Names use lowercase letters, digits,
and hyphens. References are bundle-local. A runnable bundle contains one Workflow.

| Kind | Responsibility |
| --- | --- |
| Workflow | Ordered placements, routing, and task scaffolds |
| Procedure | Container execution, approval, or external-event wait |
| Agent | A container workload with identity, model, prompt, skills, and MCP configuration |
| ConfigMap | Named nonsecret defaults for Agent or Procedure specifications |
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

Optional descriptive fields are `base_url`, `api`, `thinking`, `skills_dirs`,
`mcps`, and `mcp_servers`. Watts sends these in the common input envelope;
the image interprets them. Watts does not generate Pi settings or enforce a
particular agent's tools. Skill paths must be available inside the image or a
declared resource mount. `identity` is an evidence label, not authenticated access
control. Build and review may use the same model.

`mcps` selects names from `mcp_servers`. Each server may declare `command`,
`args`, `cwd`, `env`, `url`, `headers`, `exposure`, and `description`. A local MCP
executable belongs in the workload image. See the [Go example MCP guidance](../resources/examples/go_generalist/mcp/README.md).

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

An `mcp.ref` currently selects a name from the Agent's `mcp_servers`. It may expose multiple native MCP tools; it is a service selection, not a renamed single command. A separate MCP Kind is proposed in wiss-034. MCP gate predicates and model-selected command arguments are not supported yet.

All declared gates are required and currently accept command-tool exit codes (default zero, configurable 0–255). The image executor runs them after Agent completion and returns failures for repair inside the same execution. `max_repairs` excludes the initial pass; omission allows two repairs, zero allows none, and the maximum is ten. Each command timeout and the enclosing stage deadline still apply. Gates must leave application files unchanged; a mutation invalidates the suite. Command results and a final workspace-bound gate report are required before Watts accepts publication.

A skill selects one source: an absolute `image_path` outside workload mounts, or a `resource` key declared under `container.resources` plus a relative `path`. Captured resource hashes pin package-owned skills; the pinned image supplies image-owned skills. Source directories must exist when the executor loads them. The adapter exposes them through the runtime's native skill mechanism. These fields supplement the existing descriptive settings; the [Go example](../resources/examples/go_generalist/values.yaml) uses named skills and tools.

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
