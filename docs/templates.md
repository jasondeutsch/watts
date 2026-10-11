# Template packages

`watts template` renders YAML from a local package. It does not require an initialized project or Temporal and does not submit work.

```sh
watts template new team-workflow
watts template ./team-workflow
watts template ./team-workflow -f team.yaml --values project.yaml
watts template ./team-workflow -f project.yaml -o ./workflows/team
```

Without `-o`/`--output`, the command prints a YAML document stream. Directory output is generated content: each render replaces the YAML directory, removing stale files. Use a dedicated output directory. Edit source templates or values, then rerun the same command; do not edit rendered manifests. Rendering and validation finish before the previous output is replaced. Input/output overlap, symlinks, and existing non-YAML files are rejected. Argument errors exit with status 2; loading, rendering, validation, and output failures exit with status 1.

## Package layout

```text
team-workflow/
  watts.yaml                  # Package descriptor
  values.yaml                 # Optional defaults
  values.schema.json          # Optional JSON Schema
  templates/
    00-workflow.yaml.gotmpl
    10-agents.yaml.gotmpl
    20-procedures.yaml.gotmpl
    30-configmaps.yaml.gotmpl
    40-secrets.yaml.gotmpl
    _helpers.tpl              # Optional named helpers
```

Templates use Go `text/template` and `.Values`. `.yaml.gotmpl` files produce `.yaml`; `.tpl` files define helpers without producing files. Literal `.yaml` files pass through. Nested template directories are supported by rendering, but workflow directory loading only reads immediate YAML files; keep runnable resources together at the output root.

```yaml
kind: Agent
schema_version: 1
name: {{ .Values.builder.name | quote }}
spec:
  runtime: {{ .Values.builder.runtime | quote }}
  identity: {{ .Values.builder.identity | quote }}
  config_ref: {{ .Values.builder.config_ref | quote }}
  prompt: {{ .Values.builder.prompt | quote }}
```

Defaults load first. Repeatable `-f`/`--values` files apply in command-line order; later values win. Nested maps merge recursively; lists and scalars replace. Override paths resolve from the invocation directory. Missing direct map lookups fail; use `get` or `hasKey` for optional entries.

The command validates YAML and an optional values schema before output. If rendered documents declare Kinds, it also validates their combined resource bundle, references, and workflow graph. Project-specific availability, such as named MCPs and installed tools, is checked when the workflow is used. Generic YAML without Kinds is also supported. Credentials must remain named references in values files.

## Create a package

```sh
watts template new go-team
watts template ./go-team
```

The first command creates a new directory containing `watts.yaml`, `values.yaml`, and templates for Workflow, Agent, Procedure, ConfigMap, and Secret. The package and workflow names use the directory name. Use lowercase letters, digits, and hyphens, starting with a letter. Existing directories are never overwritten. No worker starts and no task is submitted.

Edit values and source templates to customize the package. The starter includes a builder Agent, a `go vet ./...` Procedure, shared Agent defaults, and an environment-backed Secret. It uses `OPENROUTER_API_KEY`; credentials are never written into the package.

## Package descriptor and composition

```yaml
name: go-team
version: 0.1.0
extends:
  path: ../watts/resources/examples/go_generalist/watts.yaml
values:
  - values.yaml
templates: templates
```

A descriptor identifies a local package. Its filename has no semantic meaning; `name` identifies it. `extends.path` selects another descriptor or a directory containing `watts.yaml`. Paths are relative to the descriptor declaring them. Base values merge first, followed by child values and command-line overrides. Each package may declare `assets` to capture scripts and skills needed for execution.

Templates from the base and child render together. Child templates may add resources. Replacing an inherited resource requires an explicit declaration:

```yaml
replaces:
  - kind: Workflow
    name: go-generalist
```

The child must provide the replacement with the same Kind and name. The final bundle is validated together. Templates can access `.Values`, `.Package.Name`, `.Package.Version`, and `.Package.Root`. The latter identifies the captured asset directory. `{package}` in rendered content also resolves to that directory.

Select the package when creating a task:

```sh
watts task new hello-world --package ./go-team/watts.yaml
```

A project-root `watts.yaml` is the default selection. Task creation resolves the package, captures declared assets, and saves the task's workflow and package provenance. Existing tasks retain their resolved workflow. Rendering with `watts template` is useful for inspection and does not install assets. Operational preferences belong in `.watts/settings.yaml`, separate from the portable package descriptor.

The base values file contains `workflow: {name, spec}` and `agents`, `procedures`, `config_maps`, and `secrets` mappings keyed by resource name. Templates explicitly select and render the supported fields from each mapping; they do not insert whole specifications. The shipped `values.schema.json` rejects unsupported fields and invalid value types before rendering. Common inputs, outputs, checks, limits, and environment bindings use a shared `_contract.tpl` helper. Omit unused resource mappings or supply empty mappings. ConfigMap data and workflow parameters intentionally remain open mappings because those Kinds support user-defined data. See the [Go team example](../resources/examples/go_generalist/README.md) for a complete setup.

## Implementation boundary

Rendering uses established Go template, Sprig, YAML, and JSON Schema libraries; it does not use Helm. The renderer performs no shell execution, environment lookup, or network access. It is separate from Watts resource resolution and Temporal execution. Templates are trusted inputs, not a process sandbox.

The [renderer developer reference](../internal/render/README.md) documents the API, supported helper functions, merge behavior, and filesystem limits. Helm-specific context, `tpl`, and `required` are not supported.
