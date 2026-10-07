#!/usr/bin/env bash
# Run the quality gate commands listed in PLAN.md ('## Quality gates', one backticked command per bullet).
# usage: gates.sh <task-dir>
# Refuses obviously dangerous commands unless WATTS_GATES_UNSAFE=1.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"

PLAN="$TASK_DIR/PLAN.md"
[ -f "$PLAN" ] || die "$PLAN is missing"

cmds="$(md_section "$PLAN" "Quality gates" | sed -n 's/^- `\(.*\)`[[:space:]]*$/\1/p')"
[ -n "$cmds" ] || { echo "GATES: none listed in $PLAN"; exit 1; }

deny='(^|[;&|[:space:]])(sudo|ssh|scp|curl|wget|eval)([[:space:]]|$)|rm[[:space:]]+-[a-z]*r[a-z]*f|(^|[;&|[:space:]/])git([[:space:]]|$)|\|[[:space:]]*(sh|bash)'
failed=0
while IFS= read -r cmd; do
  [ -n "$cmd" ] || continue
  echo "\$ $cmd"
  if [ "${WATTS_GATES_UNSAFE:-0}" != "1" ] && grep -Eq "$deny" <<<"$cmd"; then
    echo "  refused: matches the dangerous-command denylist (set WATTS_GATES_UNSAFE=1 to override)"
    failed=1
    continue
  fi
  rc=0
  out="$(bash -c "$cmd" 2>&1)" || rc=$?
  echo "$out" | tail -n 40
  if [ "$rc" -eq 0 ]; then echo "  -> pass"; else echo "  -> FAIL (exit $rc)"; failed=1; fi
done <<EOT
$cmds
EOT

if [ "$failed" -eq 0 ]; then echo "GATES: all passed"; else echo "GATES: FAILED"; exit 1; fi
