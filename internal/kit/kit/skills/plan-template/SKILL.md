---
name: plan-template
description: Produces PLAN.md from the bundled template once the spec-review workflow gate is approved. Use during Design and Architecture to define a technical approach, data model, risk tier recheck, quality gates, and the user stories the build loop will work through.
---

# Plan Template

Copy `plan-template.md`, bundled in this skill's folder, into the task directory as `PLAN.md`, then fill it in against SPEC.md approved by the preceding workflow gate. `watts task new <slug>` puts a copy in place. The spec-review workflow gate must approve the specification before planning starts; approval lives in workflow history, not in an agent-written signature.

The project root is the current working directory containing watts.yaml. Implement application source, tests, modules, and other project files in the project root and its normal source directories. The task directory holds workflow documents, story logs, review artifacts, and execution evidence only; do not create application code or a separate application module inside it. Quality gates run from the project root, not with a task-directory working directory.

Rules worth repeating here rather than assuming they're obvious:

- Every section needs an actual answer, not a placeholder. "None" is a valid answer for Data Model and Interface Changes, a blank field is not, a blank field is indistinguishable from forgetting to check.
- Technical Decisions and Alternatives Considered is not allowed to collapse silently for anything above Tier 0. If there's nothing in it, that's worth noticing, not skipping past.
- Risk Tier Recheck is mandatory, not a formality. Run the risk-tiering skill again against what this plan actually describes, not what SPEC.md assumed before the technical approach was known. A tier can move up here. It should never move down without saying why.
- Open Questions for the author agent exists so ambiguity gets resolved here, in writing, rather than surfacing for the first time as an admission-rubric rejection.
- The planning agent drafts this file. The default SDLC requires a human plan-review gate before implementation at every tier.

## The two sections the build loop runs on

There is no separate task list. The build loop (pi-ralph-loop) reads this file every iteration, and scripts parse two sections mechanically, so their format is fixed.

Quality gates:

- One command per bullet, written as `- \`command\``, for example `- \`go test ./...\``. These are real commands for this repository, run from the repository root.
- The builder runs them before finishing a story, and the reviewer and the checks run them again independently. Avoid network access, `sudo`, `curl`, `ssh` and Git commands in a gate. `gates.sh` refuses commands that look dangerous.

User stories:

- Each story is a heading `### US-001: Short title`, numbered in the order the builder should take them.
- Each story has a `Covers: AC-1, AC-2` line naming the SPEC criteria it satisfies, and a `Depends on:` line (`none` or story ids).
- Every acceptance criterion in SPEC.md must be covered by at least one story, or `check-plan.sh` fails.
- Each story must be small enough for one agent iteration and have checkable acceptance criteria. Nothing downstream splits a story that is too big, so split it here. A story that cannot finish in one iteration will fail repeatedly.
- Put the riskiest or approach-proving story first, and never make a story depend on a later one.

## Fill-in rules the Watts checks read

- `Risk tier after planning: N` in the Risk Tier Recheck section, where N is `0`, `1`, or `2`. The higher of this and SPEC.md's tier governs.
- `Spec ref: SPEC.md vN` near the top must match SPEC.md's `Version:`, or the approval is treated as stale.
- The plan-review human workflow gate records approval of the specification and plan hashes. An agent never fills in a human signature or approves itself.

After writing PLAN.md, the agent returns. The workflow pauses at plan-review; approval continues to build and rejection returns to planning with feedback.
