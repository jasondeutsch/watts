---
max_iterations: 10
timeout: 900
completion_promise: REVIEWED
completion_gate: required
stop_on_error: true
commands:
  - name: completion
    run: node /inputs/resources/team/scripts/check.mjs verdict /runtime/task
    acceptance: true
---
Review /workspace against SPEC.md and PLAN.md in /runtime/task. Run the quality gates. Do not change the application. Write /runtime/task/review/VERDICT.md with specific findings and a final line VERDICT: PASS or VERDICT: REJECT. Stop with <promise>REVIEWED</promise> after writing the verdict.

Use the declared command tools for quality checks; do not install or substitute verification tools. The executor runs required gates against the final candidate. Put build products and caches under /runtime.
