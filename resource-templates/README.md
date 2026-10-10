# Base resource templates

This package renders all five Watts [Kinds](../docs/kinds.md): Workflow, Agent, Procedure, ConfigMap, and Secret. Start from [values.yaml](values.yaml) and supply team overrides rather than editing the templates.

```sh
watts template ./resource-templates -f ./team-values.yaml -o ./workflows/team-v1
```

Run from a project containing a copy of this package. Set Project `spec.default_workflow: workflows/team-v1`, then create a new task. Rendering does not install tools, start workers, or submit tasks. The command refuses existing output files; use a fresh output directory for another revision.

| Values key | Contents |
| --- | --- |
| `workflow` | `name` and `spec`, including ordered steps and routes |
| `agents` | Resource names mapped to Agent specifications |
| `procedures` | Resource names mapped to Procedure specifications |
| `config_maps` | Resource names mapped to `{data: ...}` specifications |
| `secrets` | Resource names mapped to `{env: ...}` credential-source specifications |

Keep all five keys present; unused resource mappings may be empty. Shared defaults use `config_ref`; explicit fields on an Agent or Procedure replace those defaults. Credentials are environment source names, never key contents. These templates generate manifests rather than execution containers.

The default example builds from the task specification and runs `go vet ./...`. The [Go generalist package](../resources/examples/go_generalist/README.md) demonstrates a fuller SDLC, lint configuration, skills, and MCP tools. See [templating](../docs/templates.md) for merging values, output behavior, and validation.
