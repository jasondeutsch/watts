# Custom local agent directory

This asset demonstrates local Pi runtime customization. `watts agent new <name>` creates a runtime role and registers its directory in Project `spec.agents`. A runtime directory is distinct from an [Agent resource](../../../../../docs/kinds.md#agent), which describes workflow work, including instructions, model, identity, and tools.

## Files

| File | Purpose |
| --- | --- |
| `skills/` | Skills copied into the private Pi directory by configuration setup |
| `watts.example.yaml` | Project/runtime configuration fragment; Watts does not read it automatically |
| `README.md` | Human guidance |

The YAML fragment illustrates Project settings and an inline execution definition. For reusable Kind-based authoring, use the [resource templates](../../../../../resource-templates/README.md) or [Go team example](../../../../../resources/examples/go_generalist/README.md).

## Using the runtime role

Reference the created role on an Agent with `spec.runtime_agent: <name>`. The Workflow step references that Agent with `use: {kind: Agent, name: <resource-name>}`. Set `provider`, `model`, `identity`, `prompt`, `runtime: pi`, and `sandbox: {mode: local}` on the Agent, directly or through ConfigMap defaults.

Agent `skills_dirs` and `thinking` customize execution; Project agent settings supply directory and Pi defaults. Register MCP connections in Project `spec.mcps`, then select names in Agent `spec.mcps`; MCPs do not belong in `pi_settings`. Bind provider credentials through Agent `auth` and a Secret resource, or use Project provider credential sources.

Watts generates private Pi settings and model definitions from resolved configuration. User-written `models.json` files are preserved unless configuration is applied with `--force`. Private HOME and agent directories separate local runtime state; they are not a filesystem sandbox. See the [runbook](../../../../../docs/runbook.md) for setup and diagnostics.
