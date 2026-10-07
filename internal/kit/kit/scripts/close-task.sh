#!/usr/bin/env bash
# Verify that a task is really finished. Changes nothing except printing next steps.
# usage: close-task.sh <task-dir>
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"
S="$WATTS_KIT/scripts"


bash "$S/check-plan.sh" "$TASK_DIR"
bash "$S/check-approval.sh" "$TASK_DIR"
bash "$S/check-stories.sh" "$TASK_DIR" --strict || die "not every story is done"
bash "$S/check-verdict.sh" "$TASK_DIR" --require-pass || die "no passing review verdict"

tier="$(md_first_line "$TASK_DIR/SPEC.md" "Risk tier" | sed -n 's/^\([0-2]\).*/\1/p')"
if [ ! -f "$TASK_DIR/retrospective.md" ]; then
  if [ "${tier:-0}" -ge 1 ] && [ "${WATTS_SKIP_RETRO:-0}" != "1" ]; then
    die "tier $tier tasks need $TASK_DIR/retrospective.md (use the retrospective-log skill)"
  fi
  echo "note: no $TASK_DIR/retrospective.md (optional at tier 0 unless something failed unexpectedly)"
fi

DIFF_HINT="bash $(kit_rel)/scripts/evidence.sh $TASK_DIR diff"
cat <<MSG

Task $TASK_NAME is finished and checked.

Left for you:
  1. Read the final diff:   $DIFF_HINT
  2. Copy or merge the result wherever it belongs. Watts never does that for you.
  3. Keep $TASK_DIR in the repository. It is the audit trail.
  4. Delete the run records when you no longer need them: rm -r $TASK_DIR/.watts-state
MSG
