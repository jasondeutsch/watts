# Resources 

Resources for people and agents working with Watts. Start here for guidance on coordinating tasks, drafting task documents, and using the workflow. Teams can adapt these resources to their own practices.

## Skills

- [Watts coordinator](skills/watts-coordinator/SKILL.md): create tasks from a user request, prepare a specification, start execution, monitor progress, diagnose failures, and bring decisions to the human.
- [Skill setup](skills/readme.md): make the coordinator skill available to your agent application or have the agent read it directly.

## Examples and templates

- [Go generalist team](examples/go_generalist/README.md): values-driven workflow, builder skill, gopls MCP, and lint script.
- [Base resource templates](../resource-templates/README.md): all five Kinds.
- [Kinds reference](../docs/kinds.md) and [templating guide](../docs/templates.md).

## References

- [Spec drafting skill](../internal/kit/kit/skills/spec-template/SKILL.md)
- [Plan drafting skill](../internal/kit/kit/skills/plan-template/SKILL.md)
- [Task runbook](../docs/runbook.md)
- [Workflow configuration](../docs/workflows.md)

Add reusable examples, guides, and templates here as they become necessary. Runtime assets used by workflow workers remain in `internal/kit`; this directory is the entry point for users and their coordinating agents.
