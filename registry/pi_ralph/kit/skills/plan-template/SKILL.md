---
name: plan-template
description: Produces PLAN.md from the bundled template once the spec-review workflow gate is approved. Use during Design and Architecture to define a technical approach, data model, risk tier recheck, quality gates, and the user stories the build loop will work through.
---

# Plan Template

Read the existing task `PLAN.md` before editing it. Use the bundled
[plan-template.md](plan-template.md) as a structure reference; copy it only when
PLAN.md is absent. Preserve existing human content and incorporate review
feedback. Watts scaffolds only files declared by the selected workflow; the
planning Agent is responsible for writing its declared PLAN.md output.

Plan against the SPEC.md approved by the preceding workflow gate. Approval lives
in workflow history, not an agent-written signature. In `coder_golang`, a
coordinator can scaffold a task before submission with
`wctl task new <name> --compiled <artifact.json>`; the planning Agent works on the
existing task supplied to its stage.

Use the application workspace supplied by Watts, `/workspace` in the provided
image, as the project root. The authored package may live elsewhere. Application
source, tests, modules, and build files belong in that workspace and its normal
source directories. Task documents use the private task path supplied by the
launcher (`/runtime/task` in this image). Run project quality checks from the
application workspace; keep workflow documents and evidence in the task path.
The planning Agent writes PLAN.md, not application code.

Rules worth repeating here rather than assuming they're obvious:

- Every section needs an actual answer, not a placeholder. "None" is a valid answer for Data Model and Interface Changes, a blank field is not, a blank field is indistinguishable from forgetting to check.
- Technical Decisions and Alternatives Considered is not allowed to collapse silently for anything above Tier 0. If there's nothing in it, that's worth noticing, not skipping past.
- Risk Tier Recheck is mandatory, not a formality. Run the risk-tiering skill again against what this plan actually describes, not what SPEC.md assumed before the technical approach was known. A tier can move up here. It should never move down without saying why.
- Open Questions for the author agent exists so ambiguity gets resolved here, in writing, rather than surfacing for the first time as an admission-rubric rejection.
- The planning agent drafts this file. The supplied `coder_golang` workflow requires a human plan-review gate before implementation at every tier. Follow the selected workflow's declared approval policy.

## The two sections the build loop runs on

There is no separate task list. The build loop (pi-ralph-loop) reads this file every iteration. Keep the quality
commands explicit and preserve the story heading format used by configured checks.

Quality gates:

- One command per bullet, written as `- \`command\``, for example `- \`go test ./...\``. These are real commands for this repository, run from the repository root.
- Select commands from the Agent's declared tools and gates. Replace template examples with the workflow's actual commands and working directories; use its private build-output locations.
- The builder and reviewer run their configured checks. Listing a command in PLAN.md does not register a tool or gate with Watts; those contracts are authored in ACL. Follow the selected workflow's network and command policy.

User stories:

- Each story is a heading `### US-001: Short title`, numbered in the order the builder should take them.
- Each story has a `Covers: AC-1, AC-2` line naming the SPEC criteria it satisfies, and a `Depends on:` line (`none` or story ids).
- Cover every acceptance criterion in SPEC.md with at least one story. In `coder_golang`, the declared `check-plan` tool runs `scripts/check.mjs plan` from captured resources to check story headings, lint inclusion, and acceptance-criterion references. Treat passing that check as a structural check, not proof of semantic coverage.
- Each story must be small enough for one agent iteration and have checkable acceptance criteria. Nothing downstream splits a story that is too big, so split it here. A story that cannot finish in one iteration will fail repeatedly.
- Put the riskiest or approach-proving story first, and never make a story depend on a later one.

## Document conventions and approval

- `Risk tier after planning: N` in the Risk Tier Recheck section, where N is `0`, `1`, or `2`. The higher of this and SPEC.md's tier governs.
- Keep `Spec ref: SPEC.md vN` consistent with SPEC.md's `Version:` for traceability. Watts approval freshness is enforced through reviewed artifact hashes and stage-attempt identity, not this text field.
- The plan-review human workflow gate records approval of the specification and plan hashes. An agent never fills in a human signature or approves itself.

After writing PLAN.md, return to the workflow. In `coder_golang`, plan-review
waits for approval; approval continues to build and rejection returns to planning
with feedback. Other workflows may declare different routes.
