# Template packages

`watts template` renders YAML from a local package. It does not require an initialized project or Temporal and does not submit work.

```sh
watts template ./team-workflow
watts template ./team-workflow -f team.yaml --values project.yaml
watts template ./team-workflow -f project.yaml -o ./workflows/team-v1
```

Without `-o`/`--output`, the command prints a YAML document stream. Directory output preserves template-relative paths and refuses existing files. Render into a fresh directory for each revision, then select that directory in `watts.yaml`. Argument errors exit with status 2; loading, rendering, validation, and output failures exit with status 1.

## Package layout

```text
team-workflow/
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
name: {{ .Values.name | quote }}
spec: {{ .Values.agent | toYaml | nindent 2 }}
```

Defaults load first. Repeatable `-f`/`--values` files apply in command-line order; later values win. Nested maps merge recursively; lists and scalars replace. Override paths resolve from the invocation directory. Missing direct map lookups fail; use `get` or `hasKey` for optional entries.

The command validates YAML and an optional values schema before output. If rendered documents declare Kinds, it also validates their combined resource bundle, references, and workflow graph. Project-specific availability, such as named MCPs and installed tools, is checked when the workflow is used. Generic YAML without Kinds is also supported. Credentials must remain named references in values files.

## Render and use the base package

From a project with a copy of the [base resource package](../resource-templates/README.md):

```sh
watts template ./resource-templates -f ./team-values.yaml -o ./workflows/team-v1
```

Set the resulting directory as the project default:

```yaml
spec:
  default_workflow: workflows/team-v1
```

New tasks resolve this bundle into their own editable `workflow.yaml`. Edit the task bundle before first submission to customize one task. Changes to source values or project defaults affect future tasks; submitted executions retain pinned settings. Templates are processed explicitly, not whenever a worker runs a step.

The base values file contains `workflow: {name, spec}` and `agents`, `procedures`, `config_maps`, and `secrets` mappings keyed by resource name. Each mapped value is that resource's `spec`. Supply empty mappings for unused Kinds. See the [Go team example](../resources/examples/go_generalist/README.md) for a complete setup.

## Implementation boundary

Rendering uses established Go template, Sprig, YAML, and JSON Schema libraries; it does not use Helm. The renderer performs no shell execution, environment lookup, or network access. It is separate from Watts resource resolution and Temporal execution. Templates are trusted inputs, not a process sandbox.

The [renderer developer reference](../internal/render/README.md) documents the API, supported helper functions, merge behavior, and filesystem limits. Helm-specific context, `tpl`, and `required` are not supported.
