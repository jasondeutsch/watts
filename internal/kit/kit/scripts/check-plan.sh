#!/usr/bin/env bash
# Deterministic structure check for PLAN.md against SPEC.md.
# usage: check-plan.sh <task-dir>
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"

SPEC="$TASK_DIR/SPEC.md"
PLAN="$TASK_DIR/PLAN.md"
[ -f "$SPEC" ] || die "$SPEC is missing"
[ -f "$PLAN" ] || die "$PLAN is missing"

gates="$(md_section "$PLAN" "Quality gates" | grep -Ec '^- `.+`[[:space:]]*$' || true)"
if [ "$gates" -ge 1 ]; then ok "$gates quality gate command(s) listed"; else fail "PLAN.md '## Quality gates' lists no backticked commands (one per line, as '- \`cmd\`')"; fi

ids="$(grep -E '^### US-[0-9]+:' "$PLAN" | sed -E 's/^### (US-[0-9]+):.*/\1/' || true)"
if [ -z "$ids" ]; then
  fail "PLAN.md has no '### US-001: Title' story headings"
else
  ok "$(echo "$ids" | wc -l | tr -d ' ') stories found"
  dup="$(echo "$ids" | sort | uniq -d | tr '\n' ' ')"
  if [ -n "$dup" ]; then fail "duplicate story ids: $dup"; fi
fi

if grep -Eq '<short title>|<checkable criterion>|<user type>|<capability>|<benefit>' "$PLAN"; then
  fail "PLAN.md still contains template placeholders in the user stories"
fi

acs="$(grep -E '^- AC-[0-9]+:[[:space:]]*[^[:space:]]' "$SPEC" | sed -E 's/^- (AC-[0-9]+):.*/\1/' | sort -u || true)"
if [ -z "$acs" ]; then
  fail "SPEC.md has no numbered acceptance criteria ('- AC-1: ...')"
else
  covered="$(grep -E '^Covers:' "$PLAN" | grep -Eo 'AC-[0-9]+' | sort -u || true)"
  for ac in $acs; do
    if grep -Fxq "$ac" <<<"$covered"; then ok "$ac is covered by a story"; else fail "$ac is not covered by any story's 'Covers:' line"; fi
  done
fi

stories_without_covers="$(awk '
  /^### US-[0-9]+:/ { if (id && !c) print id; id = $2; sub(":", "", id); c = 0; next }
  /^Covers:[[:space:]]*AC-[0-9]+/ { c = 1 }
  /^## / { if (id && !c) print id; id = ""; c = 0 }
  END { if (id && !c) print id }
' "$PLAN" | tr '\n' ' ')"
if [ -n "$stories_without_covers" ]; then fail "stories without a 'Covers:' line: $stories_without_covers"; fi

if [ "$PROBLEMS" -gt 0 ]; then echo "PLAN: FAILED ($PROBLEMS problems)"; exit 1; fi
echo "PLAN: OK"
