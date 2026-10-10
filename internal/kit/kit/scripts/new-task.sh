#!/usr/bin/env bash
# Create a task folder with the spec and plan templates and both loop prompts, and record the
# baseline snapshot the review will compare against.
# usage: new-task.sh <slug>      (lowercase letters, digits and dashes)
set -euo pipefail
. "$(dirname "$0")/lib.sh"

[ $# -eq 1 ] || die "usage: new-task.sh <slug>"
slug="$1"
case "$slug" in
  ""|*[!a-z0-9-]*) die "slug may contain only lowercase letters, digits and dashes" ;;
esac

name="$(date +%F)-$slug"
dir="$TASKS_DIR/$name"
[ ! -e "$dir" ] || die "$dir already exists"

mkdir -p "$dir/review"
TASK_DIR="$dir"
snapshot_take base > "$dir/BASE"

# Keep the run state out of any version control the project may use, from inside the task folder.
# Nothing outside the Watts folders is touched.
cat > "$dir/.gitignore" <<'IGN'
.watts-state/
.ralph-runner/
.ralph-runner-archive/
review/.ralph-runner/
review/.ralph-runner-archive/
IGN

kit="$(kit_rel)"
render() { # template destination
  sed -e "s#__NAME__#$name#g" -e "s#__TASKS__#$TASKS_DIR#g" -e "s#__KIT__#$kit#g" -e "s#__AGENT__#$AGENT_NAME#g" -e "s#__REVIEWER__#$REVIEWER_NAME#g" "$1" > "$2"
}
render "$WATTS_KIT/templates/RALPH.build.md"  "$dir/RALPH.md"
render "$WATTS_KIT/templates/RALPH.review.md" "$dir/review/RALPH.md"
sed -e "s#tasks/<date>-<slug>/#$dir/#" "$WATTS_KIT/skills/spec-template/spec-template.md" > "$dir/SPEC.md"
sed -e "s#tasks/<date>-<slug>/#$dir/#" "$WATTS_KIT/skills/plan-template/plan-template.md" > "$dir/PLAN.md"

echo "Created $dir (baseline snapshot $(cat "$dir/BASE"))."
echo
echo "Next:"
echo "  1. describe the task in $dir/SPEC.md"
echo "  2. customize $dir/workflow.yaml if needed"
echo "  3. watts task run $dir"
echo "  4. review drafts with watts task decide $dir <stage> [--reject --feedback 'changes needed']"
