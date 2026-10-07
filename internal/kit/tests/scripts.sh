#!/usr/bin/env bash
# Exercise the bundled checks in a plain project. Any Git invocation fails the suite.
set -u
KIT="$(cd "$(dirname "$0")/../kit" && pwd)"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/watts-kit-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT
pass=0; failn=0
ok_() { pass=$((pass + 1)); echo "  ok   $1"; }
bad_() { failn=$((failn + 1)); echo "  FAIL $1"; [ -z "${2:-}" ] || printf '%s\n' "$2"; }
t_ok() { local name="$1" output; shift; if output="$("$@" 2>&1)"; then ok_ "$name"; else bad_ "$name" "$output"; fi; }
t_fail() { local name="$1" output; shift; if output="$("$@" 2>&1)"; then bad_ "$name (expected failure)" "$output"; else ok_ "$name"; fi; }
t_has() { if printf '%s' "$2" | grep -Fq -- "$3"; then ok_ "$1"; else bad_ "$1" "missing: $3"; fi; }
mkdir -p "$TEST_ROOT/bin" "$TEST_ROOT/project/.watts"
export GIT_CALLED="$TEST_ROOT/git-called"
printf '#!/usr/bin/env bash\ntouch "$GIT_CALLED"\nexit 91\n' > "$TEST_ROOT/bin/git"
chmod +x "$TEST_ROOT/bin/git"
export PATH="$TEST_ROOT/bin:$PATH" WATTS_TASKS_DIR=tasks
R="$TEST_ROOT/project"
cp -R "$KIT" "$R/.watts/kit"
printf 'original\n' > "$R/README.md"
cd "$R" || exit 1
S() { bash "$R/.watts/kit/scripts/$1" "${@:2}"; }
hash() { if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }
record_approval() {
 printf 'SPEC.md %s\nPLAN.md %s\napproved_by Human Tester\napproved_epoch 1\n' "$(hash "$TASK/SPEC.md")" "$(hash "$TASK/PLAN.md")" > "$TASK/APPROVAL"
}
write_spec() { # tier status approvedby version
  cat > "$TASK/SPEC.md" <<EOT
# SPEC: demo

## Status
$2
Version: ${4:-v1}
Supersedes: none

## Acceptance criteria
- AC-1: While idle, when polled, the system shall answer.
- AC-2: While busy, when polled, the system shall queue.

## Risk tier
$1

## Approved by
$3

*note*
EOT
}
write_plan() { # tier specver approvedby covers2
  cat > "$TASK/PLAN.md" <<EOT
# PLAN: demo

Spec ref: SPEC.md ${2:-v1}
Risk tier at Planning: $1

## Risk tier recheck

Risk tier after planning: $1

## Quality gates

- \`true\`

## User stories

### US-001: First

Covers: AC-1
Depends on: none

#### Acceptance Criteria
- [ ] it works

### US-002: Second

Covers: ${4:-AC-2}
Depends on: US-001

#### Acceptance Criteria
- [ ] it also works

## Approved by

$3

*note*
EOT
}

t_ok "create a task without version control" S new-task.sh demo
TASK="$(find tasks -maxdepth 1 -type d -name '*-demo' | head -n 1)"
for file in SPEC.md PLAN.md RALPH.md review/RALPH.md BASE .gitignore; do t_ok "$file created" test -f "$TASK/$file"; done
BASE="$(cat "$TASK/BASE")"
t_ok "baseline snapshot exists" test -f "$TASK/.watts-state/snapshots/$BASE"
t_has "baseline uses a snapshot id" "$BASE" S
t_has "build prompt uses snapshot evidence" "$(cat "$TASK/RALPH.md")" snapshot.sh
t_fail "duplicate task is refused" S new-task.sh demo
t_fail "invalid slug is refused" S new-task.sh 'Bad Slug'
t_ok "project has no version-control folder" test ! -e .git
write_spec 0 Draft '<name>'
write_plan 0 v1 '<name>'
t_ok "tier zero needs no human approval" S check-approval.sh "$TASK"
t_ok "valid plan covers both criteria" S check-plan.sh "$TASK"
write_plan 0 v1 '<name>' AC-1
t_fail "missing acceptance-criterion coverage fails" S check-plan.sh "$TASK"
write_spec 1 Approved 'Human Tester'
write_plan 1 v1 'Human Tester'
t_fail "tier one needs an approval record" S check-approval.sh "$TASK"
record_approval
t_ok "human approval validates content hashes" S check-approval.sh "$TASK"
printf '\nchanged\n' >> "$TASK/SPEC.md"
t_fail "editing the spec invalidates approval" S check-approval.sh "$TASK"
write_spec 1 Approved 'Human Tester'
record_approval
write_plan 1 v2 'Human Tester'
record_approval
t_fail "mismatched spec versions fail" S check-approval.sh "$TASK"
write_spec 0 Draft '<name>'
write_plan 0 v1 '<name>'
t_ok "preflight accepts a valid tier-zero task" S preflight.sh "$TASK"
t_ok "partial progress is reportable" S check-stories.sh "$TASK"
t_fail "strict completion rejects missing stories" S check-stories.sh "$TASK" --strict
printf 'first\n' > source.txt
printf 'second\n' >> README.md
FIRST="$(S snapshot.sh "$TASK" agent)"
printf 'DONE US-001 %s\nDONE US-002 S1-00000000\n' "$FIRST" > "$TASK/STORY_LOG.md"
t_fail "a fabricated snapshot cannot complete a story" S check-stories.sh "$TASK" --strict
printf 'second\n' >> source.txt
SECOND="$(S snapshot.sh "$TASK" agent)"
printf 'DONE US-001 %s\nDONE US-002 %s\n' "$FIRST" "$SECOND" > "$TASK/STORY_LOG.md"
t_ok "real agent snapshots complete stories" S check-stories.sh "$TASK" --strict
t_ok "snapshot IDs are distinct" test "$FIRST" != "$SECOND"
t_has "evidence shows added source" "$(S evidence.sh "$TASK" diffstat)" source.txt
t_has "text evidence contains the change" "$(S evidence.sh "$TASK" diff)" second
t_ok "quality gates pass" S gates.sh "$TASK"
cp "$TASK/PLAN.md" "$TEST_ROOT/plan-backup"
sed 's/`true`/`false`/' "$TEST_ROOT/plan-backup" > "$TASK/PLAN.md"
t_fail "failed quality gate blocks completion" S gates.sh "$TASK"
cp "$TEST_ROOT/plan-backup" "$TASK/PLAN.md"
t_fail "missing review verdict fails" S check-verdict.sh "$TASK"
t_ok "review preflight records its baseline" S pre-review.sh "$TASK"
printf 'looks fine\nVERDICT: maybe\n' > "$TASK/review/VERDICT.md"
t_fail "malformed verdict fails" S check-verdict.sh "$TASK"
printf 'looks fine\nVERDICT: PASS\n' > "$TASK/review/VERDICT.md"
touch -t 203001010000 "$TASK/review/VERDICT.md"
t_ok "passing verdict agrees with deterministic checks" S check-verdict.sh "$TASK" --require-pass
printf 'tampered\n' > source.txt
t_fail "reviewer source changes invalidate its verdict" S check-verdict.sh "$TASK"
printf 'first\nsecond\n' > source.txt
printf 'needs work\nVERDICT: REJECT: spec fidelity: missing behavior\n' > "$TASK/review/VERDICT.md"
touch -t 203001010000 "$TASK/review/VERDICT.md"
t_ok "valid rejection is accepted" S check-verdict.sh "$TASK"
S check-verdict.sh "$TASK" --require-pass >/dev/null 2>&1
status=$?
t_ok "requiring PASS gives rejection exit status three" test "$status" = 3
t_fail "rejection requires newer agent work" S check-rework.sh "$TASK"
touch -t 200001010000 "$TASK/review/VERDICT.md"
t_ok "newer agent snapshot satisfies rework" S check-rework.sh "$TASK"
t_ok "review preflight archives an old verdict" S pre-review.sh "$TASK"
t_fail "closure requires a verdict" S close-task.sh "$TASK"
printf 'looks fine\nVERDICT: PASS\n' > "$TASK/review/VERDICT.md"
touch -t 203001010000 "$TASK/review/VERDICT.md"
t_ok "finished task can close" S close-task.sh "$TASK"
t_ok "verification runs all checks" S verify.sh "$TASK"
t_ok "no check invoked Git" test ! -e "$GIT_CALLED"
echo "passed: $pass  failed: $failn"
[ "$failn" -eq 0 ]
