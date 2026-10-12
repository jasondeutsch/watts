---
name: spec-template
description: Draft or refine a task's SPEC.md from human intent, project evidence, and review feedback before implementation planning.
---

# Draft a software specification

Turn the human's request into a clear, reviewable `SPEC.md`. A rough request is enough to begin. The human supplies intent; the agent drafts the specification.

## Preserve the request

- Read the existing task `SPEC.md` before editing it.
- Preserve human intent, constraints, non-goals, and substantive content when reorganizing the draft.
- Use [spec-template.md](spec-template.md) as a structure reference. Copy it only if `SPEC.md` does not exist; never replace an existing draft with a blank template.
- If the file contains only boilerplate, record the missing intent under Open Questions and ask the human for their request. Do not invent a task.
- Write only in the selected task directory, never at the project root or in another task.

## Drafting checklist

1. **Establish the purpose.** Identify the affected user, their problem, and the observable result they need. Preserve the meaning of the human's request.
2. **Inspect the project.** Read relevant code and documentation. Describe current behavior and cite relevant project paths. Separate observed facts, assumptions, and unanswered questions.
3. **Bound the scope.** State what the change includes and excludes. Carry forward supplied constraints. Do not add adjacent features simply because they seem useful.
4. **Specify behavior.** Write numbered acceptance criteria for the requested behavior, including relevant failure cases and boundary conditions. Every criterion must be verifiable.
5. **Expose uncertainty.** Put unresolved decisions under Open Questions, explaining what needs an answer. Do not silently turn guesses into requirements or invent thresholds.
6. **Assess risk.** Use the `risk-tiering` skill to assign the tier from the actual affected behavior.
7. **Edit for clarity.** Apply the writing checklist below, then save the completed draft to `SPEC.md`.

`SPEC.md` explains what the system must do and why. Implementation approach, code structure, story sequencing, and test execution belong in `PLAN.md`. Keep human-mandated technical constraints in the spec, but do not invent an implementation there.

## Acceptance criteria

Use `- AC-1: ...`, `- AC-2: ...`, and so on. Give each criterion one observable response and a concrete system actor. Choose the EARS pattern that fits the behavior:

| Behavior | Pattern |
| --- | --- |
| Always required | The system shall perform the response. |
| Triggered by an event | When the event occurs, the system shall perform the response. |
| Applies during a state | While the state holds, the system shall perform the response. |
| Handles unwanted behavior | If the unwanted condition occurs, then the system shall perform the response. |
| Depends on an optional feature | Where the feature is enabled, the system shall perform the response. |

Combine a state and event when both matter. Do not force every criterion into the same pattern.

For example:

- AC-1: When the user selects a task, the monitor shall display that task's workflow stages.
- AC-2: If the execution service cannot answer a status query, then the monitor shall identify the task's execution status as unavailable.

Replace vague claims such as “fast,” “robust,” or “user-friendly” with observable behavior or a human-supplied measurable target. If a target is missing, record the question. The plan must map its stories to these criterion IDs.

## Writing checklist

- Lead with the purpose and write for the human who will approve the work.
- Use consistent names for users, components, and actions. Define unfamiliar terms when first used.
- Prefer active voice with an explicit actor. Replace ambiguous pronouns with the component's name.
- Keep sentences short and paragraphs focused on one topic.
- Use headings to separate concerns, lists for parallel items, and tables for comparisons.
- Remove repeated statements, template instructions, and unfilled placeholders from the draft.
- Keep facts, assumptions, constraints, and questions distinguishable.

## Save and review

Use the bundled template's headings and metadata format:

| Field | Required format |
| --- | --- |
| Status | `Draft` or `In Review` on the line below `## Status`; agents never mark their own work approved. |
| Version | `Version: vN` in the Status section. |
| Risk tier | A single `0`, `1`, or `2` on the line below `## Risk tier`. |
| Acceptance criteria | Numbered `- AC-N: ...` entries with substantive content. |
| Explicit non-goals | Concrete scope exclusions rather than template filler. |
| Open questions | Actual unresolved decisions, or an explicit statement that none remain. |

The default workflow pauses at `spec-review`. Human approval is recorded by the workflow gate against the document hash; a signature in the document does not approve the workflow. Never sign for a human or approve your own output.

On rejection, revise the existing draft using the feedback while preserving the original intent. Follow-up work on a completed task belongs in a new task folder; reference the preceding specification in `Supersedes`. Do not create a new task merely to revise a draft during its review loop.

## Sources

This checklist condenses the following guidance into Watts' existing spec and plan workflow. It does not require installing their tools.

- [GitHub Spec Kit](https://github.com/github/spec-kit), MIT: separate what and why from technical planning.
- [Microsoft EARS acceptance guidance](https://github.com/microsoft/hve-core/blob/main/.github/skills/project-planning/requirements-author/references/prd/ears-acceptance.md), CC BY 4.0: select a suitable sentence pattern and specify observable behavior. EARS originates with Alistair Mavin, Philip Wilkinson, Adrian Harwood, and Mark Novak (2009).
- [Google Technical Writing One](https://developers.google.com/tech-writing/one), CC BY 4.0: write for the reader using clear structure, consistent terminology, and concise active sentences.

The guidance above is adapted and paraphrased for Watts; examples are specific to Watts.
