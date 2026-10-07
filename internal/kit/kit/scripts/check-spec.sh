#!/usr/bin/env bash
# A drafting step must produce a usable specification, not leave the template untouched.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"
[ -s "$TASK_DIR/SPEC.md" ] || die "$TASK_DIR/SPEC.md is missing or empty"
if ! grep -Eq '^[-*] AC-[0-9]+: .+' "$TASK_DIR/SPEC.md"; then die "SPEC.md needs numbered acceptance criteria"; fi
if grep -Eq '<name>|Draft / In Review / Approved|^[-*] AC-[0-9]+:[[:space:]]*$' "$TASK_DIR/SPEC.md"; then die "SPEC.md still contains template placeholders"; fi
echo "SPEC: OK"
