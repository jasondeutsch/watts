# Go generalist team workflow

An example for teams developing Go business applications and web services. It composes [Watts Kinds](../../../docs/kinds.md) with shared model defaults, credential references, a builder skill, gopls MCP, and a deterministic golangci-lint gate. It is a starting point for team policy, not an evaluated benchmark.

## Contents

| Path | Purpose |
| --- | --- |
| [values.yaml](values.yaml) | Agents, Procedures, Workflow routing/scaffolding, ConfigMap defaults, and Secret sources |
| [templates/](templates/) | Templates for all five Kinds, following the [base package](../../../resource-templates/README.md) |
| [workflows/go-generalist.yaml](workflows/go-generalist.yaml) | Rendered bundle supplied for inspection and immediate use |
| [skills/go-builder/](skills/go-builder/SKILL.md) | Go implementation and verification guidance |
| [scripts/lint.sh](scripts/lint.sh), [.golangci.yml](.golangci.yml) | Team lint command and golangci-lint v2 policy |
| [mcp/](mcp/README.md) | Project gopls connection setup |

## Adopt and customize

Initialize your Go project with Watts. Copy this directory to `resources/examples/go_generalist` within that project; script and skill paths depend on this layout. Edit the copied values or add a team override file:

```yaml
config_maps:
  agent-defaults:
    data:
      thinking: low
agents:
  build:
    identity: my-team-builder
```

From the target project root, render into a fresh directory:

```sh
watts template ./resources/examples/go_generalist -f ./team-values.yaml -o ./workflows/go-team-v1
```

Omit `-f` to use the package defaults. Values files merge recursively; explicit resource fields override their ConfigMap defaults with a shallow merge. See [templating](../../../docs/templates.md) for details.

Merge these settings into your existing `watts.yaml` Project specification:

```yaml
spec:
  default_workflow: workflows/go-team-v1
  mcps:
    gopls:
      command: gopls
      args: [mcp]
      cwd: .
```

Alternatively, select the supplied bundle at `resources/examples/go_generalist/workflows/go-generalist.yaml`. Changes to values do not update that file automatically.

The Agent defaults select OpenRouter and declare its endpoint, model, reasoning setting, local runtime, and auth binding. The Secret declares `OPENROUTER_API_KEY` as a source name. Export its value before starting the worker. The build Agent declares its skill directory and `mcps: [gopls]`; these selections belong in the workflow bundle, not duplicate Project agent settings.

Install golangci-lint v2 and gopls on the worker PATH. Pin releases compatible with the team's Go toolchain, and use the same lint version locally and in CI. See [golangci-lint installation](https://golangci-lint.run/docs/welcome/install/) and [gopls setup](mcp/README.md).

Follow the [runbook](../../../docs/runbook.md) to install agent dependencies, start Watts, and create a new task. Its stages are:

**Specification → human spec review → planning → human plan review → build → lint → agent review**

Agents perform drafting, build, and review; Procedures implement human gates and lint. Review rework returns to build, so lint runs again. Existing submitted tasks retain their pinned configuration; create a new task to evaluate a changed bundle.

## Team policies

Run lint directly from the project root:

```sh
bash resources/examples/go_generalist/scripts/lint.sh
```

The script explicitly selects [.golangci.yml](.golangci.yml), checks all packages including tests, preserves exit status and diagnostics, and does not automatically fix files. Planning includes this command in quality gates; the lint Procedure checks again between build and review. Failures block progress. Inspect findings and retry after correcting the cause. The script assumes one Go module at the project root.

The [builder skill](skills/go-builder/SKILL.md) condenses Go style, error handling, service reliability, and testing guidance from samber/cc-skills-golang. Attribution and its MIT notice are retained. Build instructions direct the agent to read it; the review Agent does not select this skill directory.

The [gopls setup guide](mcp/README.md) explains connection registration and verification. This example's model-backed MCP execution has not been verified. Lint and tool access supplement tests and independent review. Use the [Go workflow eval](../../../evals/go-workflow/README.md) to compare team configurations over time.
