# Coordinator skills

[watts-coordinator/SKILL.md](watts-coordinator/SKILL.md) is the portable coordination contract for an interactive agent working with Watts. A user or team can install it using their agent application's skill discovery mechanism, or tell the agent to read this file directly.

Supply the exact skill path and target project path. The target project does not need to contain this repository. Resource links resolve relative to the skill file, not the agent's working directory; the skill also works without checkout documentation by using the project's scaffolded files.

Keep the skill available to the coordinating session. It is separate from the bundled worker skills under `internal/kit/kit/skills`, which agents use while executing workflow stages. The coordinator can also read the bundled [spec drafting skill](../../internal/kit/kit/skills/spec-template/SKILL.md) to prepare the first draft.

The coordinator needs access to the project files and Watts CLI. Watts does not launch or configure the user's coordinating agent. Ticket fetching and other external integrations remain the user's or team's responsibility. Teams can extend the coordination instructions with their own context and policies.
