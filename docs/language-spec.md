# ACL language specification

This specification describes **Agent Configuration Language (ACL)** as implemented
by Watts 0.1.0. ACL declares reusable executors, their capabilities, and the
workflow that connects them. Declarations are language constructs; there is no
`kind`, `schema_version`, or `spec` envelope in ACL source.

HashiCorp HCL 2 and cty provide syntax, expression evaluation, and type checking.
Watts defines the declarations, reference namespaces, compilation, and execution
semantics described here. This is a prototype specification of current behavior,
not a promise of language compatibility across future revisions.

- [Syntax and declarations](#syntax-and-declarations)
- [Evaluation and references](#evaluation-and-references)
- [Packages and variables](#packages-and-variables)
- [Modules and exports](#modules-and-exports)
- [Stages, artifacts, and feedback](#stages-artifacts-and-feedback)
- [Executor contracts](#executor-contracts)
- [Procedure](#procedure), [Agent](#agent), and [container contract](#container-contract)
- [MCP](#mcp), [ConfigMap](#configmap), and [Secret](#secret)
- [Compiled artifacts and task execution](#compiled-artifacts-and-task-execution)
- [Complete example](#complete-example)

For authoring conventions, see the [ACL style skill](../resources/skills/acl-style/SKILL.md).
The conventions guide readability; the rules below define the language.

## Syntax and declarations

An attribute assigns an expression: `model = var.model`. A block contains
attributes and nested blocks: `agent "builder" { ... }`. Quoted strings, numbers,
booleans, lists, objects, heredoc strings, interpolation (`"${var.name}"`),
conditionals, operators, and `for` expressions use HCL syntax. Comments may use
`#`, `//`, or `/* ... */`. Examples use HCL syntax highlighting; source files
have the `.acl` extension.

| Top-level declaration | Label | Purpose |
| --- | --- | --- |
| `package` | Name | Package identity, assets, variable files, and entrypoint |
| `variable` | Identifier | Typed configurable input |
| `locals` | None | Derived compile-time values |
| `module` | Name | Local package composition |
| `output` | Identifier | Exported value or declaration reference |
| `workflow` | Name | Ordered stages, artifact connections, and routing |
| `procedure` | Name | Container command, approval, or event wait |
| `agent` | Name | Container executor with model, identity, tools, skills, and gates |
| `mcp` | Name | Reusable tool-service connection and requirements |
| `configmap` | Name | Nonsecret executor or MCP defaults |
| `secret` | Name | Worker environment source bindings |

Top-level attributes and unrecognized declarations are errors. All declarations
except `locals` require exactly one label. Package, module, workflow, executor,
MCP, ConfigMap, Secret, and stage names match `[a-z][a-z0-9-]*`; `end` is reserved.
Variable and output labels use valid HCL identifiers, including underscores.
A declaration type/name pair must be unique within its module. Multiple `locals`
blocks are allowed, but local attribute names must be unique.

Argument names and nested blocks are checked against typed contracts. Unknown
fields fail compilation. A nested object can generally be written as a block or
an object-valued attribute, but not both for the same field:

```hcl
execution { type = "container" }
# Equivalent spelling: execution = { type = "container" }
```

Repeated named `tool`, `gate`, and `skill` blocks supply their `name` through the
label. Their list-valued alternatives are `tools`, `gates`, and `skills`, with
explicit `name` fields. Repeated unnamed `check` and `pre_check` blocks correspond
to `checks` and `pre_checks` lists. Workflow `stage` and `on` declarations use
blocks, rather than these executor list alternatives.

## Evaluation and references

Compilation resolves concrete values; it does not create deferred runtime
expressions. An evaluated argument must be wholly known and must not itself be
null. Values must conform to the receiving field's type and validation rules.
Declarations can be referenced before their textual definition, except that
artifact dependencies must point to earlier workflow stages.

| Namespace | Available values |
| --- | --- |
| `var.<identifier>` | Resolved variable value |
| `local.<identifier>` | Resolved local value |
| `package.name`, `package.version`, `package.root` | Current module's identity and captured-asset root |
| `agent.<name>`, `procedure.<name>`, `workflow.<name>` | Typed declaration reference; `.name` gives its qualified name |
| `mcp.<name>`, `configmap.<name>` | Declaration reference; `.name` selects a named service or defaults |
| `secret.<name>.keys.<key>` | Typed credential-source reference |
| `module.<name>.<declaration-type>.<name>` | Imported declaration reference |
| `module.<name>.outputs.<identifier>` | Exported value or reference |
| `stage.<name>.name`, `stage.<name>.outputs.<identifier>` | Stage identity and explicitly named artifact output |

Declaration references are handles, not access to arbitrary declaration fields:
`agent.builder.model` is not a supported lookup. Stage references are scoped to
the containing workflow. The strings returned by `.name` include any module
qualification; use references rather than reconstructing qualified names.

Evaluation proceeds through these phases:

1. Parse immediate source files and register declaration names.
2. Read package version, description, and variable-file declarations; these
   cannot depend on variables, locals, modules, or `package` values.
3. Resolve typed variables and their overrides. Defaults are constant expressions.
4. Resolve locals and their dependencies. Locals cannot depend on modules,
   `package` values, stage outputs, or resolved Secret keys.
5. Capture declared assets, establish `package` values, and resolve modules.
6. Resolve Secret keys, executor/service contracts, workflows, exports, and the entrypoint.

Modules resolve according to dependencies, not file order. Cycles in local or
module dependencies fail compilation. Module nesting is limited to 32 active
packages. Individual source and expression-read files are limited to 16 MiB.

## Packages and variables

| Package argument | Type | Meaning |
| --- | --- | --- |
| `version` | String | Required package version; independent of runtime schema versions |
| `description` | String | Optional description |
| `assets` | List of strings | Package-relative files/directories to embed |
| `var_files` | List of strings | Package-relative variable files, read in order |
| `entrypoint` | Workflow reference | Workflow selected for the compiled artifact |


A directory entry point is `watts.acl`. An explicit `.acl` filename also selects
its containing package. All immediate `.acl` files in that directory form one
module; filenames carry no meaning. Each module has one package declaration.
Subdirectories are loaded only through module references. Variable files use
`.wvars` so they are not mistaken for declarations.

```hcl
package "team" {
  version    = "0.1.0"
  var_files  = ["values.wvars"]
  assets     = ["scripts", "skills", "prompts"]
  entrypoint = workflow.coding
}

variable "model" {
  type    = string
  default = "team/default-model"
}

locals {
  model_name = var.model
}
```

`version` is required; `description`, `assets`, `var_files`, and `entrypoint` are
optional. Without `entrypoint`, a runnable package must declare exactly one
workflow. A module may export resources without declaring any workflow.

Variables require `type`, using the underlying HCL type constraints such as `string`, `number`,
`bool`, `list(string)`, `map(string)`, or `object({ image = string })`. `default`
is optional. Unknown variable overrides and missing required values fail.
Precedence is defaults, declared variable files, CLI variable files in order,
then `-var`. CLI expressions use ACL's HCL-based expression syntax; quote string values.

Locals may reference variables and other locals; cycles fail. In this first
implementation, locals resolve before modules and cannot read module outputs.
Expressions support HCL-based literals, interpolation, conditionals, comprehensions,
and the functions `merge`, `concat`, `coalesce`, `join`, `format`, `length`,
`jsonencode`, `jsondecode`, and `file`.

`file("prompts/build.md")` reads a UTF-8 file within the current package and
records it as a compilation input. It cannot read outside the package through
parent paths or symlinks. There is no shell, environment, or network function.
`package.root` refers to captured assets under a content-addressed project path;
use it in `container.resources`. Declared assets are embedded in the artifact,
including their file modes; execution does not require the source directory.
Asset paths must stay inside the package, cannot be `.` or traverse parents, and
cannot include symlinks. Source files cannot be symlinks either. Captured resources
are addressed by content; changing a source or asset requires recompilation.

## Modules and exports

A module has required `source` (local path), optional `version` (exact package
version match), and optional `variables` (object of input values). An `output`
declaration requires `value`, which can be a concrete value or a typed declaration
reference. Neither declaration accepts arbitrary additional fields or child blocks.


Modules compose reusable resources or entire workflows. Sources are local paths,
relative to the declaring package; absolute local paths are accepted. Remote
registries and automatic image downloads are not implemented.

```hcl
module "checks" {
  source    = "./modules/checks"
  version   = "0.1.0"
  variables = { image = "team/checks:0.1.0" }
}

workflow "coding" {
  stage "verify" {
    executor = module.checks.outputs.checker
  }
}
```

The reusable module can contain:

```hcl
package "checks" { version = "0.1.0" }
variable "image" { type = string }

procedure "verify" {
  execution { type = "container" }
  container {
    image   = var.image
    command = ["verify"]
    result  = "exit-code"
  }
}

output "checker" { value = procedure.verify }
```

`module.checks.procedure.verify` also references the declaration directly.
`output` creates an explicit public alias under `module.checks.outputs`.
Modules can pass another module's outputs as variable values; declaration order
does not determine resolution. Cycles, unknown references, and excessive nesting
fail. An optional module `version` requires an exact match with its package.
Imported resources receive qualified names to avoid accidental collisions.

To reuse an entire workflow, set the consumer package's
`entrypoint = module.team.workflow.coding`. Variables supplied to that module
customize it before compilation. A consumer can instead stitch selected exported
Agents and Procedures into its own workflow.

## Stages, artifacts, and feedback

```hcl
workflow "coding" {
  max_transitions = 100
  templates = { "SPEC.md" = "Describe the request here.\n" }

  stage "build" {
    executor = agent.builder
    inputs   = ["SPEC.md"]
    outputs  = { stories = "STORY_LOG.md" }
  }

  stage "review" {
    executor = agent.reviewer
    inputs   = [stage.build.outputs.stories]
    on "changes-required" {
      return_to      = stage.build.name
      max_iterations = 2
    }
    on "success" { return_to = "end" }
  }
}
```

Stages execute in declaration order unless a route changes the next stage.
`executor` must reference an Agent or Procedure. `inputs` is a list of task
artifact paths; `outputs` maps descriptive names to paths. Explicit stage inputs
and outputs override the executor's contract. Output references connect artifacts
and record data dependencies; they are artifact identities, not deferred model
values or promises. Data inputs may only reference earlier stages. Stages with
no explicit inputs/outputs inherit the executor's declarations. Only explicitly
named stage outputs are accessible through `stage.<name>.outputs`; inherited
executor output lists do not acquire names automatically.

`on` blocks map outcomes to stage names or `end`. When present, routes are
exhaustive for returned results; undeclared outcomes fail. Execution errors use
an `on "failure"` route, then `on_failure`, when configured. Otherwise an optional
stage may be skipped, or execution waits for an explicit retry. Backward or same-stage `on` routes require `max_iterations`,
which counts traversals of that particular route across the execution, not
container retries. Requesting another traversal after its limit fails the workflow visibly. The global
`max_transitions` and per-executor `max_attempts` remain independent limits.
Use an `on "failure"` route to bound recovery from execution errors.

Other stage arguments are `parameters`, `next`, `on_failure`, and `optional`.
`next` selects the success target; `on_failure` selects the error target. Prefer
bounded `on` routes for feedback. `templates` only scaffolds task documents; it
is not a separate text templating engine.


### Workflow and stage fields

| Workflow field | Type | Meaning |
| --- | --- | --- |
| `version` | Integer | Execution-definition version; omitted or `1` is supported |
| `max_transitions` | Integer | Total stage-entry/attempt bound; omitted or zero uses 100 |
| `templates` | Map of strings | Task-relative filename to initial content |
| `stage "name"` | Repeated block | Ordered executable placements; at least one is required |

| Stage field | Type | Meaning |
| --- | --- | --- |
| `executor` | Agent or Procedure reference | Required executable declaration |
| `inputs` | List of strings | Task artifact paths; overrides executor inputs when present |
| `outputs` | Map of strings | Output alias to task artifact path; overrides executor outputs |
| `parameters` | Object | Additional workload parameters |
| `next` | String | Success target; omitted means next declared stage, then `end` |
| `on_failure` | String | Execution-error target |
| `optional` | Boolean | Allows an execution error to skip the stage when no failure target exists |
| `on "outcome"` | Repeated block | Explicit outcome route |

An `on` block requires `return_to` (stage name or `end`) and optionally
`max_iterations` (integer 1–10000). A backward or same-stage route must provide
that bound. The count belongs to the particular stage/outcome route across the
execution. It is separate from stage attempts and Agent gate repairs. `next` and
`on_failure` also support feedback, but do not have a per-route iteration field;
the global transition bound still applies. Every stage must be reachable from
the first stage, and every target must exist or be `end`.

Artifact paths are canonical relative task paths. Absolute paths, parent
traversal, backslashes, NUL, and reserved `.watts-state` paths are invalid.
Named output references record dependencies; they do not introduce parallel
execution. File existence is checked during execution, not during compilation.
A workspace code change and a published task artifact are distinct outputs;
see [container publication](container-workloads.md).

## Executor contracts

Agent and Procedure share these fields:

| Field | Type | Meaning |
| --- | --- | --- |
| `container` | Object/block | Container execution contract |
| `inputs`, `outputs` | Lists of strings | Required input and published output artifact paths |
| `pre_checks`, `checks` | Lists of objects | Each has `command`, an explicit argument array |
| `timeout_seconds` | Nonnegative integer | Execution activity timeout; zero uses runtime policy |
| `max_attempts` | Nonnegative integer | Stage attempt bound; zero uses runtime policy |
| `environment` | Map of strings | Nonsecret workload environment values |
| `secrets` | Map of Secret key references | Workload environment targets to credential sources |
| `config_ref` | String | Optional ConfigMap name for contract defaults |

Checks belong to execution policy. Agent gates are preferable when the Agent
must run and repair a check inside its own attempt. Timeouts do not put a deadline
on a human approval or event wait. Worker policy supplies omitted execution limits;
those resolved limits are captured when execution is first submitted.

## Procedure

```hcl
procedure "verify" {
  execution { type = "container" }
  container {
    image       = "team/checks:0.1.0"
    command     = ["go", "test", "./..."]
    working_dir = "/workspace"
    workspace   = "read-only"
    result      = "exit-code"
  }
  timeout_seconds = 300
}

procedure "approve" {
  execution { type = "approval" }
  inputs = ["SPEC.md"]
}
```

`execution.type` is required and must be `container`, `approval`, or `event`.
Approval requires input artifacts; approval/event executors cannot publish
outputs, run checks, or be optional. Event stages require explicit outcome routes. Container execution
requires `container`. Approval/event Procedures wait through Temporal without
an idle container. Shared contract fields are `inputs`, `outputs`, `pre_checks`,
`checks`, `timeout_seconds`, `max_attempts`, `environment`, and `secrets`.
`config_ref` optionally references a ConfigMap name.

Checks are lists of `{ command = ["program", "argument"] }` objects. Checks
belong to execution policy; Agent quality gates are preferred for checks that
an Agent must run and repair as part of its own work.

## Agent

```hcl
secret "credentials" { env = { api_key = "MODEL_API_KEY" } }

agent "builder" {
  identity = "team-builder"
  provider = "team-provider"
  model    = "team/model"
  prompt   = file("prompts/build.md")
  auth {
    env    = "MODEL_API_KEY"
    secret = secret.credentials.keys.api_key
  }
  container {
    image     = "team/agent:0.1.0"
    command   = ["/agent"]
    workspace = "write"
    network   = "bridge"
    resources = { team = package.root }
  }
  tool "test" {
    command {
      argv            = ["go", "test", "./..."]
      working_dir     = "/workspace"
      timeout_seconds = 300
    }
  }
  gate "tests-pass" {
    tool = "test"
    expect { exit_code = 0 }
  }
  skill "team-guidance" {
    source {
      resource = "team"
      path     = "skills"
    }
  }
}
```

Agent requires nonempty `identity`, `provider`, `model`, `prompt`, and a container
contract. Optional adapter settings are `thinking`, `base_url`, `api`, and
`skills_dirs` (a list of paths). Provider/model names are adapter configuration;
Watts does not enforce that build and review models differ.
The `auth` block requires `env` (the workload environment target) and `secret`
(a Secret key reference).
A ConfigMap may provide defaults; explicit Agent fields override defaults.
Watts does not interpret agent-specific protocols or install host software.

### Agent tools, gates, and skills

A named `tool` has an optional `description` and exactly one `command` or `mcp` capability.
Command tools declare fixed `argv`, optional `working_dir`, and `timeout_seconds`.
They execute inside the Agent image. MCP tools use a service name, optional
`exposure` (`direct`, `deferred`, or `codemode`), and optional nonempty `allow_tools`.

A `gate` references a command tool and expects an exit code. The executor checks
gate evidence against the final candidate; an earlier successful call is
insufficient. `gate_policy { max_repairs = 2 }` bounds in-attempt repair.
The repair bound must be 0–10; omitted policy allows two repairs. Gate exit codes
are 0–255, with zero as the default expectation. Command tool timeouts are
0–86400 seconds, where zero leaves timeout selection to the adapter. Tool, gate,
and skill names match `[a-z][a-z0-9-]*` and must be unique within their collection.
MCP gate predicates are not supported.

A `skill` selects one source: an image-owned absolute `image_path` outside
workload mounts, or a resource name and package-relative `path`. Images own the
mechanism for exposing skills and tools to their agent. No Pi/Ralph behavior is
required by the core runtime. The [coder example](../resources/examples/coder_golang/README.md)
uses both image and team skills.

## Container contract

| Field | Meaning |
| --- | --- |
| `image` | Required OCI reference; execution pins its local image identity |
| `command` | Required explicit argument array |
| `working_dir` | `/workspace`, `/runtime`, or a subdirectory within them |
| `workspace` | `read-only` or `write`; defaults to read-only |
| `network` | `none` or `bridge`; defaults to none |
| `user` | Optional nonroot identity |
| `result` | Structured result `file` (default) or `exit-code`; Agents require `file` |
| `memory_mb`, `cpus` | Optional nonnegative resource limits; zero leaves the limit unset |
| `files` | Named files supplied privately to the workload |
| `resources` | Named project-relative resource directories; use `package.root` for package assets |

Images supply all software and MCP subprocesses. Build or pull them explicitly
on the worker engine. Tools cannot rely on the host toolchain. Output publication
is governed by [the workload interface](container-workloads.md).

## MCP

```hcl
mcp "gopls" {
  connection {
    transport       = "stdio"
    timeout_seconds = 60
    command {
      argv        = ["gopls", "mcp"]
      working_dir = "/workspace"
    }
  }
  lifecycle = "attempt"
  tools { discovery = "all" }
  requirements {
    os        = ["linux"]
    arch      = ["amd64", "arm64"]
    workspace = "read-only"
  }
}
```

An Agent selects a service through a named MCP tool. A module can export the
service; `ref = module.gopls.outputs.service.name` then selects it. Only selected
services enter the Agent's resolved input:

```hcl
tool "code-intelligence" {
  mcp {
    ref         = mcp.gopls.name
    exposure    = "deferred"
    allow_tools = ["go_diagnostics"]
  }
}
```

Tool names must match the service's actual discovery results; this example is a
consumer selection, not a guarantee that every gopls release exposes that name.

Supported transports are `stdio` with attempt lifecycle and an explicit command,
and `streamable-http` with external lifecycle and a URL. Connection timeout must
be 1–3600 seconds. Stdio runs inside the consuming Agent image. External services
are not started or stopped by Watts. Independent MCP containers are future work.

`tools.discovery` is `all` or `allowlist`; allowlists require exact names in
`tools.allow`. An Agent selection can restrict but cannot broaden that policy.
Requirements describe the workload image's OS/architecture, workspace access
(`none`, `read-only`, `write`), and outbound network needs. These are requirements,
not independent permissions or proof of sandbox isolation.

Literal `environment` and `headers` are nonsecret configuration. `secrets` maps
server environment names to Secret key references. `header_secrets` maps header
names to `{ secret = secret.credentials.keys.api_key, prefix = "Bearer " }`.
Only source bindings are compiled; workers supply values at launch. No shell
credential expressions are evaluated. Readiness/tool discovery happens through
the Agent adapter's MCP library, with visible categorized errors.

### MCP image and workspace ownership

A stdio server and its dependencies must be installed in the consuming Agent
image. Its `working_dir = "/workspace"` points to that attempt's candidate
workspace. Server execution uses the image's operating system, not the worker
host's: a Linux image runs Linux gopls even when Docker is hosted on macOS.
No host server installation or project `.pi/mcp.json` is required by Watts.
The image adapter translates the resolved connection into its own client settings
and owns readiness checks, session lifetime, and stdio process cleanup.

The [coder_golang gopls module](../resources/examples/coder_golang/modules/gopls/watts.acl)
exports its service through `output "service" { value = mcp.service }`.
The [build Agent](../resources/examples/coder_golang/agents.acl) selects
`module.gopls.outputs.service.name` with deferred exposure; the
[team image](../registry/coder_golang/Dockerfile) supplies the executable.
Other Agents may reuse that export with their own exposure and narrower tool
selection. Connection policy belongs to the module, while each Agent declares
which capabilities it consumes.

Change the authored module or Agent, recompile, and create a new task to change
MCP configuration. Submitted tasks retain their resolved configuration.
Adapter-specific implementation details belong with the workload image; the
[Pi/Ralph adapter](../registry/pi_ralph/README.md#mcp-resource-adapter) is one implementation.

### MCP field reference

| Field | Type | Requirement |
| --- | --- | --- |
| `description` | String | Optional description |
| `config_ref` | String | Optional ConfigMap defaults |
| `connection.transport` | String | `stdio` or `streamable-http` |
| `connection.timeout_seconds` | Integer | Required, 1–3600 seconds |
| `connection.command` | Command-tool object | Required for stdio; no command-level timeout |
| `connection.url` | String | Required for HTTP; HTTP(S), no user credentials or fragment |
| `lifecycle` | String | `attempt` for stdio, `external` for HTTP |
| `tools.discovery` | String | Required: `all` or `allowlist` |
| `tools.allow` | List of strings | Required only for `allowlist`; exact unique names, no wildcards |
| `requirements.os` | List of strings | Optional: `linux`, `darwin`, `windows` |
| `requirements.arch` | List of strings | Optional: `amd64`, `arm64` |
| `requirements.workspace` | String | Optional: `none`, `read-only`, `write` |
| `requirements.network` | Boolean | Whether the consumer needs outbound networking |
| `environment`, `secrets` | Maps | Stdio server values and Secret key bindings |
| `headers`, `header_secrets` | Maps | HTTP header values and `{ secret, prefix }` bindings |

HTTP services cannot declare server environment bindings. Stdio services cannot
have HTTP headers. Literal MCP values do not evaluate shell expressions or
credential interpolation. Secret and literal bindings must not conflict.
A remote service or `requirements.network = true` requires the consumer's
`container.network = "bridge"`; a writable workspace requirement requires
`container.workspace = "write"`. OS/architecture requirements are checked against
the workload image. An Agent's `allow_tools`, when supplied, must be nonempty
and cannot broaden a service allowlist. Exposure is `direct`, `deferred`, or
`codemode`; how those modes appear to the model belongs to the image adapter.

## ConfigMap

```hcl
configmap "defaults" {
  data = {
    provider = "team-provider"
    model    = "team/model"
    thinking = "medium"
  }
}
```

`config_ref = configmap.defaults.name` applies defaults to an Agent, Procedure,
or MCP. Explicit top-level fields win; nested objects are replaced, not deeply
merged. Referencing a missing ConfigMap fails compilation.

## Secret

```hcl
secret "credentials" { env = { api_key = "MODEL_API_KEY" } }
```

`secret.credentials.keys.api_key` produces a typed source reference for Agent
authentication, workload secret bindings, or MCP credentials. Literal key values
are not a Secret feature. Declared values are redacted at workload capture
boundaries; image adapters must also protect model-facing tool results. See
[secret protection](secret-protection.md) for coverage and limits. Compilation
does not look up the environment, and compiled artifacts contain source names rather than credentials. Missing required
worker sources fail readiness checks or execution.

## Compiled artifacts and task execution

Without `-o`, compile prints JSON. With `-o`, it atomically replaces that artifact
and prints its package identity, digest, stage count, and dependency count. Source
and asset files cannot be output targets. The artifact contains schema version,
resolved definition, captured assets, package/module provenance, and declared
image/MCP dependencies. A SHA256 fingerprint detects accidental changes; it is
not a signature or trust assessment. Inspect execution without side effects:

```sh
wctl exec coding.json --dry-run
```

For a coordinator or human who wants to draft the request before submission:

```sh
wctl task new hello-server --compiled coding.json
# Edit the printed task's SPEC.md.
wctl exec tasks/<printed-task-name>
```

For immediate execution, use `wctl exec coding.json --name hello-server
--request "..."`. Every execution creates a distinct task. Existing-task exec
continues the same execution or retries its failed stage; it does not reload
source configuration. Tasks store a resolved `workflow.json` with a private definition fingerprint.
Execution rejects a task definition that has been edited since scaffolding. Do not hand-edit
compiled artifacts, task definitions, or `.watts-state` to change policy.

The application root is the invocation directory for new compiled executions,
independent of the package/artifact location. Source edits require recompilation
and a new task. Monitor, approvals, conflict-aware publication, and `report.html`
use recorded execution evidence. See [the runbook](runbook.md).

## Complete example

Save this as `watts.acl`. It compiles without an Agent, credential, model service,
container engine, or Temporal server. Execution requires the declared image on
the worker engine and a running Watts worker/Temporal service.

```hcl
package "hello" {
  version    = "0.1.0"
  entrypoint = workflow.hello
}

variable "image" {
  type    = string
  default = "alpine:3.20"
}

procedure "greet" {
  execution { type = "container" }
  container {
    image   = var.image
    command = ["sh", "-c", "printf 'Hello from Watts\n'"]
    result  = "exit-code"
  }
  timeout_seconds = 60
  max_attempts    = 1
}

workflow "hello" {
  max_transitions = 10
  stage "greet" {
    executor = procedure.greet
  }
}
```

```sh
wctl compile ./watts -o hello.json
wctl exec hello.json --dry-run
wctl exec hello.json --name hello
```

For a complete configurable coding package with modules, Agents, tools, skills,
MCP, approvals, and feedback, see [coder_golang](../resources/examples/coder_golang/README.md).
Operational commands are in the [runbook](runbook.md); image input/result and
publication contracts are in [container workloads](container-workloads.md).
The language implementation lives in [workflowlang](../internal/workflowlang/),
with executor validation in [workload](../internal/workload/) and workflow
validation in [orchestration](../internal/orchestration/definition.go).
