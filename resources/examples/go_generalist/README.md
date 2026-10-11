# Go generalist workflow package

A configurable example for Go business applications and web services. The templates define resource envelopes, fields, and wiring, populated by configurable values. They contain no agent, model, MCP, check, or lint policy.

## Configuration source

[values.yaml](values.yaml) defines the complete example policy:

- Workflow order, routes, approval gates, and task document scaffolding.
- Agent identities, prompts, Pi runtime selection, and Ralph commands and configuration.
- Model, provider, reasoning settings, and environment-backed credential references.
- Agent checks, skills, and the builder's gopls MCP connection.
- Deterministic Procedures, including the golangci-lint command.

[templates/](templates/) explicitly renders Workflow placements and routes, Agent runtime and tool configuration, Procedure execution, ConfigMap data, and Secret source bindings. `_contract.tpl` shares inputs, outputs, checks, limits, and environment bindings between Agents and Procedures. [values.schema.json](values.schema.json) validates configurable fields and rejects typos before rendering. Empty resource mappings produce no resources of that Kind. Change the values to select different tools or define different work; rendering itself does not require Pi, Ralph, or model credentials. Execution still requires an adapter supporting the selected runtime; currently the supported Agent runtime is Pi.

[watts.yaml](watts.yaml) identifies the package, selects its values and templates, and declares scripts, skills, and lint configuration as assets. It contains no agent or tool selection. Keep credential values outside the package.

## Use from another project

For a project next to the Watts checkout, create this project-root `watts.yaml`:

```yaml
name: my-go-team
version: 0.1.0
extends:
  path: ../watts/resources/examples/go_generalist/watts.yaml
values:
  - values.yaml
```

Create a local `values.yaml` with team overrides:

```yaml
config_maps:
  agent-defaults:
    data:
      thinking: low
agents:
  build:
    identity: my-team-builder
```

Render for inspection, then create a task:

```sh
watts template ./watts.yaml
watts task new hello-world
```

Base values merge with team overrides; later scalars and lists replace earlier ones, while maps merge recursively. Changing workflow steps therefore replaces the step list. Use an empty mapping in a standalone package to omit a resource Kind; an empty override mapping does not delete inherited resources. Adding or replacing inherited resources through additional templates follows the explicit replacement rules in the [template reference](../../../docs/templates.md).

Task creation resolves templates and captures declared assets. Edit source values or templates, never generated task manifests. Changes apply to newly created tasks; existing tasks retain their resolved definitions. No copied example directory, manually rendered default workflow, or separate project MCP registration is needed.

## Default example

**Specification → human spec review → planning → human plan review → build → lint → agent review**

The current values select Pi, Ralph for build/review, OpenRouter, a builder skill, gopls MCP, and golangci-lint v2. These are example choices, not template requirements. Review rework returns to build so lint runs again. Export `OPENROUTER_API_KEY` before starting the worker. The default lint script expects a Go module at the project root.

The current execution path needs the Pi/Ralph runtime dependencies, gopls, and golangci-lint on the worker PATH. See the [runbook](../../../docs/runbook.md) for execution and the [MCP guide](mcp/README.md) for the connection defined in values. Removing legacy project initialization and moving preparation into adapters is tracked in wiss-020.

The [lint script](scripts/lint.sh) explicitly uses [.golangci.yml](.golangci.yml), preserves diagnostics and exit status, and does not automatically fix files. The [builder skill](skills/go-builder/SKILL.md) includes attribution and the MIT notice for guidance derived from samber/cc-skills-golang.

This example's model-backed MCP execution has not been verified, and the workflow is not an evaluated benchmark. Use the [Go workflow eval](../../../evals/go-workflow/README.md) to compare configurations over time.
