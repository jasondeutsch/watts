#!/usr/bin/env bash
# Everything that must be true before the build loop may start.
# usage: preflight.sh <task-dir>
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"

for f in SPEC.md PLAN.md RALPH.md; do
  [ -f "$TASK_DIR/$f" ] || die "$TASK_DIR/$f is missing"
done

bash "$WATTS_KIT/scripts/check-plan.sh" "$TASK_DIR"
bash "$WATTS_KIT/scripts/check-approval.sh" "$TASK_DIR"
echo "PREFLIGHT: OK"
