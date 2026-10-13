# Golang coder workflow package

A configurable example for Go business applications and web services. The templates define resource envelopes, fields, and wiring, populated by configurable values. They contain no agent, model, MCP, check, or lint policy.

## Configuration source

[values.yaml](values.yaml) defines the complete example policy:

- Workflow order, routes, approval gates, and task document scaffolding.
- Agent identities, prompts, container image selection, and private Ralph configuration.
- Model, provider, reasoning settings, and environment-backed credential references.
- Agent checks, skills, and the reusable gopls MCP resource and builder selection.
- Callable build/test/vet/lint tools and formal gates inside the build Agent; approval waits remain Procedures.

[templates/](templates/) explicitly renders Workflow placements and routes, Agent container and descriptive tool configuration, Procedure execution, MCP connection/lifecycle/tool policy, ConfigMap data, and Secret source bindings. `_contract.tpl` shares inputs, outputs, checks, limits, and environment bindings between Agents and Procedures. [values.schema.json](values.schema.json) validates configurable fields and rejects typos before rendering. Empty resource mappings produce no resources of that Kind. Change the values to select different tools or define different work; rendering itself does not require Pi, Ralph, or model credentials. Execution uses the common container contract. The values select the Pi/Ralph image; another image can implement the same interface.

[watts.yaml](watts.yaml) identifies the package, selects its values and templates, and declares scripts, skills, and lint configuration as assets. It contains no agent or tool selection. Keep credential values outside the package.

## Use from another project

For a project next to the Watts checkout, create this project-root `watts.yaml`:

```yaml
name: my-go-team
version: 0.1.0
extends:
  path: ../watts/resources/examples/coder_golang/watts.yaml
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
wctl template ./watts.yaml
wctl task new hello-world
```

Base values merge with team overrides; later scalars and lists replace earlier ones, while maps merge recursively. Changing workflow steps therefore replaces the step list. Use an empty mapping in a standalone package to omit a resource Kind; an empty override mapping does not delete inherited resources. Adding or replacing inherited resources through additional templates follows the explicit replacement rules in the [template reference](../../../docs/templates.md).

Task creation resolves templates and captures declared assets. Edit source values or templates, never generated task manifests. Changes apply to newly created tasks; existing tasks retain their resolved definitions. No copied example directory, manually rendered default workflow, or separate project MCP registration is needed.

## Default example

See the [workflow diagram](workflow-diagram.md) for stage order, approval/rework routes, and Agent completion gates.

**Specification → human spec review → planning → human plan review → build → agent review**

The current values select Pi, Ralph for build/review, OpenRouter, a builder skill, gopls MCP, and golangci-lint v2. These are example choices, not template requirements. Specification and planning use required document gates with bounded repair. Build runs its declared build/test/vet/lint and story-completion gates before publication, with up to two gate repair passes. Review has read-only test/vet/lint tools and a required verdict-format gate with no automatic repair; failing application checks can still produce a valid REJECT verdict and return work to build. Review rework returns to build, which reruns its gates. There is no standalone lint stage. Export `OPENROUTER_API_KEY` before starting the worker. The default lint script expects a Go module at the project root.

Build the example images from the Watts checkout before starting the worker:

```sh
docker build -t watts/pi-ralph:0.1.0 registry/pi_ralph
docker build -t watts/coder-golang:0.1.0 registry/coder_golang
```

The team image contains Pi, Ralph, Go, gopls, and golangci-lint. The currently supported worker backend is local Docker Engine; agent/tool installations belong in the image. Package scripts, lint configuration, and skills are captured and mounted at `/inputs/resources/team`. Private task documents and tool state live under `/runtime`. Application work is performed in a candidate `/workspace` and published after result/check validation. See the [runbook](../../../docs/runbook.md), [image definition](../../../registry/coder_golang/README.md), and [MCP guide](mcp/README.md).

The [lint script](scripts/lint.sh) explicitly uses [.golangci.yml](.golangci.yml), preserves diagnostics and exit status, and does not automatically fix files. The [builder skill](skills/go-builder/SKILL.md) includes attribution and the MIT notice for guidance derived from samber/cc-skills-golang.

Real MCP readiness/tool calls and transport behavior have deterministic integration coverage. Model-selected MCP use has not been verified, and the workflow is not a performance benchmark. Use the [Go workflow eval](../../../evals/go-workflow/README.md) to compare configurations over time.

### Agent capability configuration

`agents.build.tools` names fixed commands and the `gopls` MCP reference; `agents.build.gates` names required checks and expected exit codes. `gate_policy.max_repairs` bounds repairs inside that Agent execution. `skills` selects image-owned guidance or captured resource directories independently of tool access. Templates render each field explicitly. See [Agent tools, gates, and skills](../../../docs/kinds.md#agent-tools-gates-and-skills).

The [build script](scripts/build.sh) puts compilation products under `/runtime/build` so verification does not publish a Linux executable into the developer's source tree. Declared command tools use the original image PATH; private replacement binaries do not override these checks. Other agent shell capabilities remain subject to the image's policy.
