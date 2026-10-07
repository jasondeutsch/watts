#!/usr/bin/env bash
# After a REJECT verdict, require a newer agent snapshot before the build loop may finish.
# usage: check-rework.sh <task-dir>
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"

V="$TASK_DIR/review/VERDICT.md"
if [ ! -f "$V" ]; then echo "REWORK: no review yet"; exit 0; fi

last="$(grep -v '^[[:space:]]*$' "$V" | tail -n 1 || true)"
case "$last" in
  "VERDICT: REJECT"*) ;;
  *) echo "REWORK: not needed (latest verdict is not a rejection)"; exit 0 ;;
esac

t_agent="$(agent_last_time)"
t_verdict="$(file_mtime "$V")"
if [ -n "$t_agent" ] && [ "$t_agent" -gt "$t_verdict" ]; then
  echo "REWORK: ok, the agent took a snapshot after the rejection"
  exit 0
fi
echo "REWORK: pending. The reviewer rejected the work and no agent snapshot is newer than the verdict: $last"
exit 1
