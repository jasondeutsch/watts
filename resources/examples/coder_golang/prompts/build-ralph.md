---
max_iterations: 10
timeout: 900
completion_promise: DONE
completion_gate: required
stop_on_error: true
commands:
  - name: completion
    run: node /inputs/resources/team/scripts/check.mjs stories /runtime/task
    acceptance: true
---
Implement PLAN.md stories in /workspace. Read /runtime/task/SPEC.md and PLAN.md. Run the quality gates from /workspace. Append DONE US-### to /runtime/task/STORY_LOG.md only after tests and lint pass. Address previous workflow feedback. Stop with <promise>DONE</promise> when all stories are done.

Use the declared command tools for quality checks; do not install or substitute verification tools. The executor runs required gates against the final candidate. Put build products and caches under /runtime.
