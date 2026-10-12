# Base resource templates

This package renders all five Watts [Kinds](../docs/kinds.md): Workflow, Agent, Procedure, ConfigMap, and Secret. Start from [values.yaml](values.yaml) and supply team overrides rather than editing the templates.

```sh
wctl template new team-workflow
wctl template ./team-workflow -f ./team-values.yaml
wctl task new my-task --package ./team-workflow/watts.yaml
```

Scaffolding uses this package's descriptor, values, and templates as its source. It refuses to overwrite an existing directory. Edit the scaffolded source and values; rendered YAML is generated content. Task creation resolves the selected package and captures its workflow. Rendering does not install tools, start workers, or submit tasks.

| Values key | Contents |
| --- | --- |
| `workflow` | `name` and `spec`, including ordered steps and routes |
| `agents` | Resource names mapped to Agent specifications |
| `procedures` | Resource names mapped to Procedure specifications |
| `config_maps` | Resource names mapped to `{data: ...}` specifications |
| `secrets` | Resource names mapped to `{env: ...}` credential-source specifications |

Templates render explicit fields rather than accepting a whole resource specification as opaque YAML. The bundled `values.schema.json` checks configurable fields; unsupported properties fail before rendering. `_contract.tpl` shares execution-contract fields between Agents and Procedures; `_agent.tpl` renders named tools, gates, repair policy, and skill sources. Unused resource mappings may be omitted or empty. Shared defaults use `config_ref`; explicit fields on an Agent or Procedure replace those defaults. Credentials are environment source names, never key contents. These templates generate manifests rather than execution containers.

The default example builds from the task specification and runs `go vet ./...`. The [Go generalist package](../resources/examples/go_generalist/README.md) demonstrates a fuller SDLC, lint configuration, skills, and MCP tools. See [templating](../docs/templates.md) for merging values, output behavior, and validation.
