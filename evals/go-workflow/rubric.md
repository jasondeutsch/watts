# Human review rubric

Rubric version: `0.1.0`

Review the final project against the case definition. Cite evidence for each score. Score each dimension independently; use `N/A` only when the case explicitly makes a dimension inapplicable, and explain why.

## Scoring anchors

| Score | Correctness | Maintainability | Test quality | Scope adherence |
| --- | --- | --- | --- | --- |
| 0 | No usable implementation | Cannot reasonably be maintained | No usable verification | Fundamental scope violation |
| 1 | Major requirements unmet | Major structural or clarity problems | Major behaviors untested or checks ineffective | Major unrelated changes or boundary violations |
| 2 | Mostly correct; substantive fixes needed | Substantive restructuring or clarification needed | Meaningful coverage, but important gaps remain | Substantive scope corrections needed |
| 3 | Requirements met; minor fixes needed | Clear overall; minor improvements needed | Relevant verification with minor gaps | Minor unnecessary changes |
| 4 | Requirements met without corrective changes | Clear, well-organized, consistent with the project | Meaningful behavior and failure checks appropriate to the task | Changes confined to the permitted scope |

Use the existing project's conventions when judging maintainability. Test quality concerns the usefulness of verification, not a target test count or coverage percentage. A score of 4 does not imply universal perfection beyond the case's requirements.

## Verified success

Record `pass`, `fail`, or `unverified` separately from review scores.

- `pass`: independent verification confirms every required acceptance criterion and required regression check.
- `fail`: at least one required criterion or regression check fails.
- `unverified`: available evidence cannot establish success or failure.

Human inspection may be the verification method where appropriate, but document what was checked and the evidence. Record whether corrective human edits were necessary; a verified final result after intervention differs from an unassisted success.

## Workflow compliance

Record `pass`, `fail`, or `unverified`. Compare actual stages, approvals, retries, and completion with the configured workflow. Do not penalize a configuration for intentionally different stages. Skipped required approval, false completion, or failure to enforce a declared gate is a failure.

Record terminal task outcome separately: `completed`, `failed`, `cancelled`, `timed_out`, or `incomplete`. Workflow completion alone does not establish verified success.

## Reviewer consistency

Apply the same rubric throughout a comparison period. Record reviewer identity and short justifications. Where feasible, review artifacts without knowing which configuration produced them. Periodically have two reviewers independently score the same output; retain initial scores and document how differences were resolved.

## Measurements

- Elapsed seconds: initial task submission to terminal outcome or evaluation cutoff; includes human waiting and retries.
- Human minutes: active review, clarification, diagnosis, and corrective work; excludes unattended waiting. Record how it was measured or estimated.
- Interventions: human actions beyond the initial request; classify approvals, rejections, clarifications, retries, and manual edits in the run record.
- Tokens and cost: totals across stages and failed attempts when available. Leave aggregate values unavailable if accounting is incomplete, and record known partial values separately.
