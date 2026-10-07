#!/usr/bin/env bash
# Evidence for the loop prompts, as plain commands without shell metacharacters so they pass the
# extension's shell allowlist (which must match the whole command).
# usage: evidence.sh <task-dir> <snapshots|diffstat|diff|feedback>
#   snapshots the snapshots taken for the task, oldest first
#   diffstat  which files were added (A), modified (M) or deleted (D) since the baseline
#   diff      the same, with a unified diff for each modified text file
#   feedback  the end of the last verdict
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"
st="$TASK_DIR/.watts-state"
case "${2:-}" in
  snapshots)
    for f in "$st"/snapshots/S*; do
      [ -f "$f" ] || continue
      id="${f##*/}"; echo "$id $(head -n 1 "$f" | sed 's/^# label //')"
    done ;;
  diffstat)
    cur="$(mktemp)"; snap_manifest > "$cur"
    manifest_diff "$st/snapshots/$BASE" "$cur"
    rm -f "$cur" ;;
  diff)
    cur="$(mktemp)"; snap_manifest > "$cur"
    manifest_diff "$st/snapshots/$BASE" "$cur" | while IFS= read -r line; do
      kind="${line%% *}"; path="${line#* }"
      case "$kind" in
        A) echo "added: $path" ;;
        D) echo "removed: $path" ;;
        M)
          old="$(grep -F -- "  $path" "$st/snapshots/$BASE" | head -n 1 | cut -c1-64)"
          if [ -n "$old" ] && [ -f "$st/objects/$old" ]; then diff -u --label "a/$path" --label "b/$path" "$st/objects/$old" "$path" || true
          else echo "modified: $path (too large for a stored copy)"; fi ;;
      esac
    done | head -n 3000
    rm -f "$cur" ;;
  feedback)
    f="$TASK_DIR/review/VERDICT.md"
    if [ -f "$f" ]; then tail -n 40 "$f"; else echo "No review yet."; fi ;;
  *) die "usage: evidence.sh <task-dir> <snapshots|diffstat|diff|feedback>" ;;
esac
