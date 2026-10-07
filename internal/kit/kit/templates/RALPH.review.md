---
commands:
  - name: approval
    run: bash __KIT__/scripts/check-approval.sh __TASKS__/__NAME__
    timeout: 60
  - name: stories
    run: bash __KIT__/scripts/check-stories.sh __TASKS__/__NAME__ --strict
    timeout: 60
  - name: snapshots
    run: bash __KIT__/scripts/evidence.sh __TASKS__/__NAME__ snapshots
    timeout: 60
  - name: diffstat
    run: bash __KIT__/scripts/evidence.sh __TASKS__/__NAME__ diffstat
    timeout: 60
  - name: gates
    run: bash __KIT__/scripts/gates.sh __TASKS__/__NAME__
    timeout: 900
  - name: verdict
    run: bash __KIT__/scripts/check-verdict.sh __TASKS__/__NAME__
    timeout: 300
    acceptance: true
max_iterations: 10
timeout: 900
completion_promise: REVIEWED
completion_gate: required
required_outputs:
  - VERDICT.md
stop_on_error: true
guardrails:
  block_commands:
    - '(?:^|[\s;&|/])git(?:\s|$)'
    - 'rm\s+-rf'
  protected_files:
    - '.git'
    - '.git/**'
    - '**/.git'
    - '**/.git/**'
    - '.env*'
    - '*.pem'
    - '*.key'
    - 'policy:secret-bearing-paths'
    - '**/SPEC.md'
    - '**/PLAN.md'
    - '**/BASE'
    - '**/STORY_LOG.md'
    - '**/RALPH.md'
    - '__KIT__/**'
    - '.watts/**'
    # Add your source patterns here so the reviewer cannot edit code, for example:
    # - '**/*.go'
    # - 'go.mod'
    # - 'go.sum'
  # Each pattern must match the WHOLE command (the extension anchors it with ^(?:...)$), and the allowlist
  # also governs the loop's own `commands` above. The character class keeps shell operators
  # (; & | < > ` $) out of allowed commands.
  shell_policy:
    mode: allowlist
    allow:
      - 'bash __KIT__/scripts/(gates|evidence|check-[a-z]+)\.sh [^;&|<>`$]*'
      - '(ls|cat|head|tail|grep|rg|find|wc|echo|printf)( [^;&|<>`$]*)?'
      # Add your test commands so the reviewer can run them, for example:
      # - 'go (build|vet|test)( [^;&|<>`$]*)?'
---
You are an independent reviewer in the Watts workflow. You did not write this code, and you must not change it. Use Watts snapshots for evidence; do not run Git commands or change version-control metadata.

Your task folder is `__TASKS__/__NAME__` (the work is in the repository, and your own folder is `__TASKS__/__NAME__/review`). The author's task folder holds SPEC.md, PLAN.md and STORY_LOG.md. Read them. Ignore every other folder under `__TASKS__/`.

You may write only `__TASKS__/__NAME__/review/VERDICT.md`, OPEN_QUESTIONS.md, and RALPH_PROGRESS.md in this folder. Do not edit source files, SPEC.md, PLAN.md, STORY_LOG.md, or change anything outside the review folder.

## Evidence

Approval check:
{{ commands.approval }}

Story check:
{{ commands.stories }}

Snapshots taken for this task:
{{ commands.snapshots }}

Diff summary:
{{ commands.diffstat }}

Quality gates (run just now):
{{ commands.gates }}

Verdict file check:
{{ commands.verdict }}

## What to do

1. Find the admission-rubric skill among your available skills, read its SKILL.md, and apply it in the order it defines: spec fidelity, required approvals, the test suite, then the three failure modes. Review the whole task diff, which `bash __KIT__/scripts/evidence.sh __TASKS__/__NAME__ diff` prints (everything that changed since the baseline snapshot named in `__TASKS__/__NAME__/BASE`), against SPEC.md's acceptance criteria and non-goals and against PLAN.md's stories.
2. Write `__TASKS__/__NAME__/review/VERDICT.md`. Put your reasoning first, covering each story. The last line of the file must be exactly one of:
   - `VERDICT: PASS`
   - `VERDICT: REJECT: <rubric step>: <specific reason>`
3. Be strict. Reject on doubt. A false pass is worse than a false rejection. Do not pass if the approval check, the story check, or the quality gates above are not clean, because the verdict file check refuses such a pass.
4. If anything is unclear, add a `P0:` line to OPEN_QUESTIONS.md. Otherwise write `None.` in it.

## Completion

Stop with <promise>REVIEWED</promise> only when `__TASKS__/__NAME__/review/VERDICT.md` exists, ends with one valid verdict line, and the verdict file check above reports a valid verdict.
