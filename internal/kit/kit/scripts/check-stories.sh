#!/usr/bin/env bash
# Compare PLAN.md's stories with STORY_LOG.md and the snapshots.
# usage: check-stories.sh <task-dir> [--strict]
#   default  prints progress and always exits 0
#   --strict exits 1 unless every story is logged DONE with the id of a snapshot the agent took
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"
strict=0
[ "${2:-}" = "--strict" ] && strict=1

PLAN="$TASK_DIR/PLAN.md"
LOG="$TASK_DIR/STORY_LOG.md"
[ -f "$PLAN" ] || die "$PLAN is missing"

ids="$(grep -E '^### US-[0-9]+:' "$PLAN" | sed -E 's/^### (US-[0-9]+):.*/\1/' || true)"
[ -n "$ids" ] || { echo "STORIES: none found in PLAN.md"; [ "$strict" -eq 1 ] && exit 1; exit 0; }

done_list=""; remaining=""; invalid=""
for id in $ids; do
  line=""
  [ -f "$LOG" ] && line="$(grep -E "^DONE $id S[0-9]+-[0-9a-f]{8}\$" "$LOG" | tail -n 1 || true)"
  if [ -z "$line" ]; then remaining="$remaining $id"; continue; fi
  sha="${line##* }"
  snap="$TASK_DIR/.watts-state/snapshots/$sha"
  if [ -f "$snap" ] && [ "$(head -n 1 "$snap")" = "# label agent" ] && [ "$(snap_epoch "$sha")" -ge "$(snap_epoch "$BASE")" ]; then
    done_list="$done_list $id"
  else
    invalid="$invalid $id"
  fi
done

echo "STORIES: done:${done_list:- none} | remaining:${remaining:- none} | logged but snapshot not found:${invalid:- none}"

if [ "$strict" -eq 1 ]; then
  [ -z "$remaining" ] && [ -z "$invalid" ] || exit 1
fi
exit 0
