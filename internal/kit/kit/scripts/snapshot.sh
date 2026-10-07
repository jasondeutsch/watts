#!/usr/bin/env bash
# Record the project as it is now. The build agent runs this after finishing a story and logs the
# id it prints in STORY_LOG.md.
# usage: snapshot.sh <task-dir> [label]      label is "agent" unless a human says otherwise
set -euo pipefail
. "$(dirname "$0")/lib.sh"
label="${2:-agent}"
if [ "$label" = "base" ]; then task_init "${1:-}" creating; else task_init "${1:-}"; fi
case "$label" in agent|human|review-start|base) ;; *) die "label must be base, agent, human or review-start" ;; esac
snapshot_take "$label"
