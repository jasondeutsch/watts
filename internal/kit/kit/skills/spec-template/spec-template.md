# SPEC: <name>

## Status
Draft / In Review / Approved / Superseded
Version: v1
Supersedes: <none, or tasks/<name>/SPEC.md vN>
Owner: <human name>
Last updated: <date>

## Intent
Why this exists, what problem it solves, for whom. One or two paragraphs, not a restatement of the acceptance criteria.

## Background and context
Relevant prior art, related systems, why this is being done now rather than earlier or later.

## Acceptance criteria
Write each criterion using the EARS pattern that fits: an invariant, event, state, unwanted condition, or optional feature followed by the system response. Combine a state and event only when both matter. Not "handles errors gracefully," that's not testable. Number each one (AC-1, AC-2, ...) so PLAN.md's user stories can name which criteria they cover. Example:

- AC-1: While the queue is non-empty, when a worker polls, the dispatcher shall return exactly one unclaimed task.
- AC-2: While no tasks are available, when a worker polls, the dispatcher shall return an empty result within 200ms, not block.

- AC-3:

## Explicit non-goals
What this deliberately does not do. Required, not optional, this is what the admission rubric checks a diff against for invented scope.

-

## Assumptions
What's being taken as given. Worth stating explicitly, because if one of these is wrong, the spec is wrong, not the implementation.

-

## Constraints
Technical, regulatory, timeline, or resourcing limits that shape the acceptance criteria above.

-

## Open questions
Ambiguity that has to be resolved by a human before PLAN.md gets written, not silently assumed by whoever implements this. If this section is empty, that's worth a second look, not a sign everything's clear.

-

## Risk-relevant context
- Touches:
- Does not touch:

## Risk tier
<0 / 1 / 2, from the risk-tiering skill>

## Review

An agent drafts this document. The human approves or rejects it at the spec-review workflow gate with `watts task decide <task> spec-review`, optionally using `--reject --feedback 'changes needed'`.
