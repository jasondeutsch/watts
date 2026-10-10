# Resource Kinds

Watts separates a workflow's routing from the work it runs. A **step** is a named placement in a Workflow; its `use` field references an Agent or Procedure. The same resource can be placed more than once, with different step names and routes.

| Kind | Responsibility |
| --- | --- |
| `Workflow` | Ordered steps, parameters, outcome routing, scaffolding, and overall limits |
| `Procedure` | Work without a model: commands, checks, plugins, approvals, or event waits |
| `Agent` | A specialized work definition with a model, runtime, identity, instructions, skills, and tools |
| `ConfigMap` | Shared nonsecret defaults for Agent or Procedure specifications |
| `Secret` | Named credential sources, resolved from the worker environment at launch |

An Agent shares the Procedure input/output and execution-limit contract, but has its own Kind and fields. It does not reference a separate Procedure. There are no Stage, Model, or Prompt Kinds. The project settings file uses a separate `kind: Project` envelope; it configures local infrastructure rather than a unit of workflow work.

## Envelope and bundles

Each resource has this envelope:

```yaml
kind: Procedure
schema_version: 1
name: lint
spec:
  execution:
    type: command
    command: go vet ./...
```

Kind names are case-sensitive. Resource names use lowercase letters, digits, and dashes; `end` is reserved. Identity is the pair `(kind, name)` within a bundle. Duplicate identities, unsupported schema versions, unknown specification fields, and unresolved references are rejected.

A runnable bundle contains exactly one Workflow and all resources it references. Put documents in one YAML file separated by `---`, or put YAML files directly in a directory. Directory loading reads `.yaml` and `.yml` files in filename order, without descending into subdirectories. References are bundle-local; there is no implicit search, resource registry, or include mechanism.

Use a project-relative file or directory as `spec.default_workflow` in `watts.yaml`. Directory paths should include a slash, for example `workflows/team`, to distinguish them from bundled names such as `sdlc`. Task creation resolves the source and writes a self-contained `<task>/workflow.yaml`, materializing ConfigMap defaults. The task file need not preserve the original resource names or shared defaults. First submission pins the resolved execution definition and runtime settings.

## Workflow

```yaml
kind: Workflow
schema_version: 1
name: checks
spec:
  version: 1
  max_transitions: 20
  steps:
    - name: lint
      use:
        kind: Procedure
        name: lint
```

| Specification field | Meaning |
| --- | --- |
| `steps` | Required, nonempty ordered list of named placements |
| `version` | Workflow schema version; use `1` |
| `templates` | Task-relative artifact paths mapped to initial text |
| `capture_baseline` | Capture a project snapshot at task creation |
| `max_transitions` | Total execution budget including rework; default `100` |

Each placement requires `name` and `use: {kind, name}`. Step names begin with a lowercase letter and must be unique. `use.kind` is Agent or Procedure. Optional placement fields are `parameters`, `transitions`, `next`, `on_failure`, and `optional`. Parameters are executor input, not template substitutions or arbitrary resource overrides. Routing belongs to the placement; execution settings belong to the referenced resource. See [workflow execution](workflows.md) for routing and results.

## Shared work contract

These fields are available directly under Agent and Procedure `spec`:

| Field | Meaning |
| --- | --- |
| `inputs`, `outputs` | Task-relative regular-file artifact paths |
| `pre_checks`, `checks` | Checks before execution and after completion, using `{script, args}` entries |
| `timeout_seconds` | Activity timeout; falls back to project limits, then one hour |
| `max_attempts` | Attempt budget; falls back to project limits, then three |
| `result_file` | Task-relative JSON executor result for Agent or command/check Procedure |
| `environment` | Nonsecret environment variable name/value bindings |
| `secrets` | Environment variable targets mapped to `{name, key}` Secret references |
| `config_ref` | Name of a ConfigMap supplying specification defaults |

Artifact paths cannot be absolute, traverse outside the task, or use reserved `.watts-state` paths. Inputs are hashed at attempt start; required outputs and completion checks must pass. Application code and command working directories belong to the project root, not the task folder.

Environment bindings cannot override Watts' reserved variables, HOME, PATH, TERM, LANG, or reserved Pi settings. A variable cannot be bound in both `environment` and `secrets`.

## Procedure

`execution.type` selects the work:

| Type | Additional execution fields | Behavior |
| --- | --- | --- |
| `command` | `command` | Bash command executed from the project root |
| `check` | `check`, optional `args` | Installed kit script |
| `capability` | `capability` | Named plugin registered in Project `spec.capabilities` |
| `approval` | None | Wait for a human decision over declared input hashes |
| `event` | None | Wait for a validated external event |

Only fields belonging to the selected execution type are accepted. Procedures do not have model, prompt, runtime, or MCP fields.

An approval Procedure requires inputs and cannot declare outputs, checks, a result file, or an optional placement. An event Procedure requires placement transitions and cannot declare outputs, checks, a result file, or an optional placement. Neither wait uses an activity timeout. Capability results arrive on stdout, so a capability Procedure does not use `result_file`.

```yaml
kind: Procedure
schema_version: 1
name: spec-approval
spec:
  inputs: [SPEC.md]
  execution:
    type: approval
```

## Agent

```yaml
kind: Agent
schema_version: 1
name: builder
spec:
  runtime: pi
  runtime_agent: build
  identity: team-builder
  provider: openrouter
  model: deepseek/deepseek-v4-flash
  prompt: Read {task}/SPEC.md and implement the change in the project root.
  sandbox:
    mode: local
```

Required fields, whether supplied directly or through ConfigMap defaults, are `runtime`, `identity`, `provider`, `model`, `prompt`, and `sandbox.mode`. Currently `runtime: pi` and `sandbox.mode: local` are the only supported choices. **Local mode is not a security sandbox.** Container execution and distributed packages remain [proposed](stage-communication.md).

| Optional Agent field | Meaning |
| --- | --- |
| `runtime_agent` | Local Pi role/directory selector; defaults to the resource name |
| `thinking` | Pi reasoning setting |
| `skills_dirs` | Project-relative skill directories |
| `mcps` | Named connections from Project `spec.mcps` |
| `base_url`, `api` | Provider connection overrides |
| `auth` | Provider credential target `env` and Secret `{name, key}` |

`identity` identifies the author for task evidence; it is not authenticated access control. `runtime_agent` selects local runtime state, rather than workflow routing. Multiple Agents can use `runtime_agent: build` while declaring different prompts and tools. Project agent settings supply local directory, installation, and Pi defaults; Agent fields customize execution. Using the same model for build and review is allowed.

## ConfigMap

```yaml
kind: ConfigMap
schema_version: 1
name: agent-defaults
spec:
  data:
    runtime: pi
    provider: openrouter
    model: deepseek/deepseek-v4-flash
    thinking: medium
    sandbox:
      mode: local
```

Reference it with `spec.config_ref: agent-defaults` on an Agent or Procedure. `data` supplies top-level specification defaults. Explicit resource fields replace those defaults, including an entire nested map or list. This is a **shallow merge**, unlike the recursive values-file merge used by templating. Defaults must be valid fields for the consuming Kind. ConfigMaps cannot chain `config_ref` or currently provide arbitrary file mounts or Workflow defaults.

## Secret

```yaml
kind: Secret
schema_version: 1
name: model-credentials
spec:
  env:
    api-key: OPENROUTER_API_KEY
```

`api-key` is a logical key; `OPENROUTER_API_KEY` is the source variable name in the worker environment. No credential value is stored in this resource. Export the source before starting the worker.

An Agent can bind that key as its provider credential:

```yaml
spec:
  auth:
    env: OPENROUTER_API_KEY
    secret:
      name: model-credentials
      key: api-key
```

An Agent or Procedure can instead declare general credential bindings:

```yaml
spec:
  secrets:
    SERVICE_TOKEN:
      name: service-credentials
      key: token
```

The source and target variable names may differ. Missing resources and keys fail resolution; missing values fail at launch. Source names are pinned, while values are read for each attempt, so a retry can use rotated credentials. An already-running worker must have the updated environment to use a new value. Watts does not embed these values in the compiled definition; executors are still responsible for keeping credentials out of output and artifacts. Protected-file and managed-store sources are not implemented.

## Templates and a complete example

[resource-templates/](../resource-templates/README.md) contains templates for all five Kinds. The [Go generalist example](../resources/examples/go_generalist/README.md) uses them with a values file, shared Agent defaults, Secret references, a lint Procedure, skills, and gopls MCP. See [templating](templates.md) for rendering and selecting the resulting bundle.
