#!/usr/bin/env bash
# Everything that must be true before the review loop may start. Archives any earlier verdict and
# takes the review-start snapshot that the verdict check compares against.
# usage: pre-review.sh <task-dir>
set -euo pipefail
. "$(dirname "$0")/lib.sh"
task_init "${1:-}"

[ -f "$TASK_DIR/review/RALPH.md" ] || die "$TASK_DIR/review/RALPH.md is missing"

if ! bash "$WATTS_KIT/scripts/check-stories.sh" "$TASK_DIR" --strict >/dev/null; then
  bash "$WATTS_KIT/scripts/check-stories.sh" "$TASK_DIR" || true
  die "the build is not finished: every story must be logged DONE with a snapshot before review"
fi

mkdir -p "$TASK_DIR/.watts-state"
snapshot_take review-start > "$TASK_DIR/.watts-state/review-start"

if [ -f "$TASK_DIR/review/VERDICT.md" ]; then
  mkdir -p "$TASK_DIR/review/history"
  dest="$TASK_DIR/review/history/VERDICT-$(date +%s).md"
  mv "$TASK_DIR/review/VERDICT.md" "$dest"
  info "archived the previous verdict to $dest"
fi
echo "PRE-REVIEW: OK"
