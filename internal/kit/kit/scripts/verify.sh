#!/usr/bin/env bash
# All deterministic checks in one place. Safe to run at any time.
# usage: verify.sh <task-dir>
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"
S="$WATTS_KIT/scripts"
rc=0
bash "$S/check-plan.sh" "$TASK_DIR"            || rc=1
bash "$S/check-approval.sh" "$TASK_DIR"        || rc=1
bash "$S/check-stories.sh" "$TASK_DIR"         || true
bash "$S/check-stories.sh" "$TASK_DIR" --strict >/dev/null || { echo "STORIES: not complete"; rc=1; }
bash "$S/check-rework.sh" "$TASK_DIR"          || rc=1
if [ -f "$TASK_DIR/review/VERDICT.md" ]; then bash "$S/check-verdict.sh" "$TASK_DIR" || rc=1; else echo "VERDICT: none yet"; fi
exit "$rc"
