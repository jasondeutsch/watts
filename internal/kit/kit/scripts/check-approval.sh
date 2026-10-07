#!/usr/bin/env bash
# Deterministic approval check. Exit 0 when the approval record is valid for the task's tier.
# An approval is a record of the SHA-256 of SPEC.md and PLAN.md, made by `watts task decide`
# after a human has filled in the Approved by sections. Editing either file afterwards
# invalidates it.
# usage: check-approval.sh <task-dir>
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"

SPEC="$TASK_DIR/SPEC.md"
PLAN="$TASK_DIR/PLAN.md"
[ -f "$SPEC" ] || die "$SPEC is missing"
[ -f "$PLAN" ] || die "$PLAN is missing"

# Managed workflows approve immutable artifact hashes at their human gates.
if [ -f "$TASK_DIR/.watts-state/temporal.json" ]; then
  for f in SPEC.md PLAN.md; do
    hash="$(sha256_of "$TASK_DIR/$f")"
    found=0
    for receipt in "$TASK_DIR"/.watts-state/approvals/*.txt; do
      [ -f "$receipt" ] || continue
      if awk -v f="$f" -v h="$hash" '$1 == f && $2 == h { found=1 } END { exit !found }' "$receipt"; then found=1; break; fi
    done
    [ "$found" -eq 1 ] || fail "$f has no matching workflow approval. Use watts task decide $TASK_DIR <review-stage>."
  done
  if [ "$PROBLEMS" -gt 0 ]; then echo "APPROVAL: FAILED ($PROBLEMS problems)"; exit 1; fi
  echo "APPROVAL: OK (workflow artifact approvals)"
  exit 0
fi

tier_spec="$(md_first_line "$SPEC" "Risk tier" | sed -n 's/^\([0-2]\)\([^0-9].*\)\{0,1\}$/\1/p')"
tier_plan="$(sed -n 's/^Risk tier after planning:[[:space:]]*\([0-2]\)\([^0-9].*\)\{0,1\}$/\1/p' "$PLAN" | head -n 1 || true)"

if [ -z "$tier_spec" ]; then fail "cannot read the risk tier from $SPEC (expected a line starting with 0, 1 or 2 under '## Risk tier')"; fi
if [ -z "$tier_plan" ]; then fail "cannot read 'Risk tier after planning:' from $PLAN"; fi
if [ "$PROBLEMS" -gt 0 ]; then echo "APPROVAL: FAILED ($PROBLEMS problems)"; exit 1; fi

tier="$tier_spec"
[ "$tier_plan" -gt "$tier" ] && tier="$tier_plan"

if [ "$tier" -eq 0 ]; then
  echo "APPROVAL: OK (tier 0, no human approval required)"
  exit 0
fi

status="$(md_first_line "$SPEC" "Status")"
case "$status" in
  Approved*) case "$status" in */*) fail "SPEC.md Status still looks like the template placeholder: $status" ;; *) ok "SPEC.md Status is Approved" ;; esac ;;
  *) fail "SPEC.md Status must be Approved, found: ${status:-<empty>}" ;;
esac

for f in "$SPEC" "$PLAN"; do
  line="$(md_first_line "$f" "Approved by")"
  case "$line" in
    ""|"<"*|"*"*) fail "$f has no filled-in 'Approved by' section" ;;
    *) ok "$f approved by: $line" ;;
  esac
done

ver_spec="$(sed -n 's/^Version:[[:space:]]*\(v[0-9][0-9]*\).*/\1/p' "$SPEC" | head -n 1 || true)"
ver_plan="$(sed -n 's/^Spec ref:[[:space:]]*SPEC\.md[[:space:]]*\(v[0-9][0-9]*\).*/\1/p' "$PLAN" | head -n 1 || true)"
if [ -z "$ver_spec" ] || [ -z "$ver_plan" ]; then
  fail "cannot compare spec versions (SPEC.md 'Version: vN' and PLAN.md 'Spec ref: SPEC.md vN')"
elif [ "$ver_spec" != "$ver_plan" ]; then
  fail "PLAN.md refers to SPEC.md $ver_plan but SPEC.md is $ver_spec (stale approval)"
else
  ok "PLAN.md refers to SPEC.md $ver_spec"
fi

REC="$TASK_DIR/APPROVAL"
if [ "${WATTS_APPROVAL_SKIP_HASH:-0}" = "1" ]; then
  : # `watts task decide` is about to record the hashes
elif [ ! -f "$REC" ]; then
  fail "$REC is missing. A human must run: watts task decide $TASK_DIR"
else
  for f in SPEC.md PLAN.md; do
    want="$(awk -v f="$f" '$1 == f { print $2 }' "$REC")"
    if [ -z "$want" ]; then
      fail "$REC has no recorded hash for $f"
    elif [ "$want" != "$(sha256_of "$TASK_DIR/$f")" ]; then
      fail "$TASK_DIR/$f has changed since it was approved. Approve it again with: watts task decide $TASK_DIR"
    else
      ok "$TASK_DIR/$f matches its recorded approval"
    fi
  done
  who="$(awk '$1 == "approved_by" { $1 = ""; sub(/^ /, ""); print }' "$REC")"
  for bad in "$AGENT_NAME" "$REVIEWER_NAME"; do
    if [ "$who" = "$bad" ]; then fail "$REC was written by the agent identity '$bad'; a human must approve"; fi
  done
  if [ "$tier" -eq 2 ]; then
    t_first="$(agent_first_time)"
    t_appr="$(awk '$1 == "approved_epoch" { print $2 }' "$REC")"
    if [ -n "$t_first" ] && [ -n "$t_appr" ]; then
      if [ "$t_appr" -ge "$t_first" ]; then
        fail "tier 2: the approval was recorded at or after the agent's first snapshot, so approval did not come first"
      else
        ok "tier 2: the approval was recorded before the agent's first snapshot"
      fi
    fi
  fi
fi

if [ "$PROBLEMS" -gt 0 ]; then echo "APPROVAL: FAILED (tier $tier, $PROBLEMS problems)"; exit 1; fi
echo "APPROVAL: OK (tier $tier)"
