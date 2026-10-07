#!/usr/bin/env bash
# Validate the reviewer's verdict file and refuse a PASS that the deterministic checks contradict.
# usage: check-verdict.sh <task-dir> [--require-pass]
#   exit 0  valid verdict (PASS, or REJECT without --require-pass)
#   exit 1  missing, malformed, stale, or inconsistent verdict
#   exit 3  valid REJECT with --require-pass
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"
require_pass=0
[ "${2:-}" = "--require-pass" ] && require_pass=1

KIT_SCRIPTS="$WATTS_KIT/scripts"
V="$TASK_DIR/review/VERDICT.md"
[ -f "$V" ] || { echo "VERDICT: missing ($V)"; exit 1; }

last="$(grep -v '^[[:space:]]*$' "$V" | tail -n 1 || true)"
kind=""
case "$last" in
  "VERDICT: PASS") kind="PASS" ;;
  "VERDICT: REJECT: "?*": "?*) kind="REJECT" ;;
  *) fail "last line must be exactly 'VERDICT: PASS' or 'VERDICT: REJECT: <step>: <reason>', found: $last" ;;
esac

t_agent="$(agent_last_time)"
if [ -n "$t_agent" ] && [ "$(file_mtime "$V")" -lt "$t_agent" ]; then
  fail "the verdict is older than the latest agent snapshot, so it did not review the current code"
fi

rs="$TASK_DIR/.watts-state/review-start"
if [ -f "$rs" ]; then
  cur="$(mktemp)"; snap_manifest > "$cur"
  others="$(manifest_diff "$TASK_DIR/.watts-state/snapshots/$(cat "$rs")" "$cur" | awk '{ print $2 }' | tr '\n' ' ')"
  rm -f "$cur"
  if [ -n "$others" ]; then fail "files changed outside $TASK_DIR/review/ during review: $others"; fi
fi

if [ "$kind" = "PASS" ]; then
  for check in "check-approval.sh" "check-plan.sh"; do
    if ! bash "$KIT_SCRIPTS/$check" "$TASK_DIR" >/dev/null 2>&1; then fail "verdict is PASS but $check fails"; fi
  done
  if ! bash "$KIT_SCRIPTS/check-stories.sh" "$TASK_DIR" --strict >/dev/null 2>&1; then fail "verdict is PASS but not every story is logged DONE with a real snapshot"; fi
  if ! bash "$KIT_SCRIPTS/gates.sh" "$TASK_DIR" >/dev/null 2>&1; then fail "verdict is PASS but the quality gates fail"; fi
fi

if [ "$PROBLEMS" -gt 0 ]; then echo "VERDICT: INVALID ($PROBLEMS problems)"; exit 1; fi
echo "VERDICT: $kind"
if [ "$kind" = "REJECT" ] && [ "$require_pass" -eq 1 ]; then echo "$last"; exit 3; fi
exit 0
