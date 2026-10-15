# Watts

![Watts process improvement loop](resources/diagrams/watts-architecture.svg)

**Define, execute, measure, compare, and improve a repeatable process for agent work.**

A team cannot reliably improve a process it cannot describe. When each engineer
improvises prompts, tools, checks, and review, a successful result is difficult
to explain or reproduce. Watts makes that process explicit, executable, and
versionable so teams can evaluate changes against outcomes and resource use.

Teams author workflows in **ACL**, composing reusable modules, Agents,
Procedures, MCP services, skills, gates, approvals, and bounded feedback loops.
Compilation resolves those choices into a self-contained artifact. Execution
records attempts, decisions, failures, published artifacts, and available usage.
The web monitor and HTML report show what actually happened.

## Compile, then execute

The CLI is `wctl`. Watts remains a proof of concept at **0.1.0**.

```sh
make build
# Compilation requires no Docker, Temporal, provider credentials, or Git.
bin/wctl compile resources/examples/coder_golang -o /tmp/coding.json
# `wctl c` is the short form.
```

Execution requires a Temporal service, a project worker, and the selected images
on its Docker engine. Pi/Ralph is an example image, not a requirement of Watts.
From this checkout:

```sh
docker compose up -d --wait
make images
make install
```

From the application project:

```sh
export OPENROUTER_API_KEY='your-key' # The coder example's declared credential source.
wctl doctor --compiled /tmp/coding.json
wctl start -d
wctl exec /tmp/coding.json --name hello-server \
  --request 'Create a minimal Go HTTP server.'
```

Use the task path printed by exec. The coder example refines `SPEC.md`, waits for
spec approval, drafts `PLAN.md`, waits for plan approval, implements, and reviews.

```sh
wctl task status <task> --json
wctl task decide <task> spec-review
wctl task decide <task> plan-review
# After inspecting and fixing a failed attempt:
wctl exec <task>
```

To draft before submitting, use `wctl task new <slug> --compiled /tmp/coding.json`,
edit its `SPEC.md`, then `wctl exec <task>`. Completed tasks have `report.html`.
A submitted task retains its definition and runtime inputs; source changes
require compilation and a new task. Never hand-edit compiled output to change policy.

## How it works

- **Author:** ACL packages with typed variables, local modules, and explicit references.
- **Compile:** Resolve and validate stages, routing, dependencies, and captured assets into JSON.
- **Execute:** Temporal manages durable state; workers run generic container workloads.
- **Review:** Inspect actual attempts, decisions, checks, token coverage, and reports; compare evals over time.

Compilation is language processing. Execution pins local image identities and
worker preferences at submission. There is no apply step or infrastructure
reconciliation engine. CLI and future MCP adapters share application services.

The current backend is local Docker. Remote module registries, distributed
execution, complete telemetry, and hardened multi-tenant isolation remain work
in progress. Declarative workflows do not guarantee identical model output or
better results; they give teams a process they can inspect and improve.

## Documentation

- [ACL language specification](docs/language-spec.md)
- [Task runbook](docs/runbook.md)
- [Golang coder example](resources/examples/coder_golang/README.md)
- [Coordinator skill](resources/skills/watts-coordinator/SKILL.md)
- [Architecture](docs/workflow-design.md), [container interface](docs/container-workloads.md), and [Temporal](docs/temporal.md)
- [Workflow evals](evals/go-workflow/README.md), and [known issues](issues.md)

Watts does not require Git. The source is licensed under [MPL 2.0](LICENSE).
