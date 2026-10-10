#!/usr/bin/env bash
# Shared helpers for the Watts scripts. Source this file, do not run it.
# Works on bash 3.2 (macOS) and later. Every path is relative to the project root.
#
# Watts does not use git or any other version control. The record of the work is a set of
# snapshots (see snapshot_take below) kept under <task>/.watts-state. Whether the project happens
# to be tracked by git is none of Watts' business.

set -euo pipefail

# pwd -P resolves symlinks. Without it, a kit reached through a symlinked path (macOS /var is a link to
# /private/var) would not compare equal to the project root.
WATTS_KIT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
REPO="${WATTS_REPO:-}"
if [ -z "$REPO" ]; then
  d="$PWD"
  while [ "$d" != "/" ] && [ ! -e "$d/watts.yaml" ] && [ ! -d "$d/.watts" ]; do d="$(dirname "$d")"; done
  [ "$d" != "/" ] && REPO="$d"
fi
[ -n "$REPO" ] || { echo "error: run this from inside a Watts project (a folder with watts.yaml), or set WATTS_REPO" >&2; exit 1; }
cd "$REPO"
REPO="$(pwd -P)"

# shellcheck disable=SC2034  # used by the scripts that source this file
AGENT_NAME="${WATTS_AGENT_NAME:-watts-agent}"
# shellcheck disable=SC2034
REVIEWER_NAME="${WATTS_REVIEWER_NAME:-watts-reviewer}"
# Where task folders live. watts sets it from watts.yaml.
# shellcheck disable=SC2034
TASKS_DIR="${WATTS_TASKS_DIR:-tasks}"

die()  { echo "error: $*" >&2; exit 1; }
info() { echo "$*"; }

# Path of the kit relative to the project root, or absolute if it lives elsewhere.
kit_rel() {
  case "$WATTS_KIT" in
    "$REPO"/*) echo "${WATTS_KIT#"$REPO"/}" ;;
    *)         echo "$WATTS_KIT" ;;
  esac
}

# task_init <task-dir>
# Sets TASK_DIR (relative, no trailing slash), TASK_NAME and BASE (the id of the baseline snapshot).
task_init() {
  [ -n "${1:-}" ] || die "usage: $(basename "$0") <task-dir>   (for example $TASKS_DIR/2026-10-03-http-retry)"
  local d="${1%/}"
  d="${d#./}"
  case "$d" in "$REPO"/*) d="${d#"$REPO"/}" ;; esac
  [ -d "$d" ] || die "no such task folder: $d (paths are relative to the project root)"
  TASK_DIR="$d"
  TASK_NAME="$(basename "$d")"
  if [ "${2:-}" = "creating" ]; then return; fi
  [ -f "$TASK_DIR/BASE" ] || die "$TASK_DIR/BASE is missing. Create task folders with: watts task new <slug>"
  BASE="$(tr -d '[:space:]' < "$TASK_DIR/BASE")"
  [ -f "$TASK_DIR/.watts-state/snapshots/$BASE" ] || die "the baseline snapshot $BASE is missing from $TASK_DIR/.watts-state (it is local state; create the task again if it was deleted)"
}

# md_first_line <file> <heading>
# First non-empty line under "## <heading>", up to the next "## " heading.
md_first_line() {
  awk -v h="$2" '
    /^## / { if (insec) exit; if ($0 == "## " h) { insec = 1; next } }
    insec && NF { sub(/\r$/, ""); print; exit }
  ' "$1"
}

# md_section <file> <heading>  -> every line under "## <heading>"
md_section() {
  awk -v h="$2" '
    /^## / { if (insec) exit; if ($0 == "## " h) { insec = 1; next } }
    insec { sub(/\r$/, ""); print }
  ' "$1"
}

# file modification time as epoch seconds (works with GNU and BSD date)
file_mtime() { date -r "$1" +%s; }

if command -v sha256sum >/dev/null 2>&1; then SHA=(sha256sum); else SHA=(shasum -a 256); fi

# sha256 of a file
sha256_of() { "${SHA[@]}" "$1" | awk '{print $1}'; }

# ---- snapshots --------------------------------------------------------------------------------
# A snapshot is what Watts uses where a commit would otherwise be: a list of the SHA-256 of every
# project file, plus a copy of each file up to 1 MB (stored by hash, so unchanged files cost
# nothing). Each snapshot has a label: base (the start of the task), agent (taken by the build
# agent after a story), human, or review-start (taken just before the review).

# snap_files lists the project files a snapshot covers: everything except Watts' own files, the
# tasks folder, common dependency folders, and WATTS_SNAPSHOT_EXCLUDE (one path per line).
snap_files() {
  local ex=(-path ./.git -o -path ./.watts -o -path ./watts.yaml -o -path "./$TASKS_DIR" -o -path ./node_modules -o -path ./.venv) x
  while IFS= read -r x; do
    if [ -n "$x" ]; then ex+=(-o -path "./$x"); fi
  done <<EOT
${WATTS_SNAPSHOT_EXCLUDE:-}
EOT
  find . \( "${ex[@]}" \) -prune -o -type f -print | sed 's#^\./##' | LC_ALL=C sort
}

# snap_manifest prints "<sha256>  <path>" for every file in the project as it is now.
snap_manifest() {
  local list
  list="$(mktemp)"
  snap_files > "$list"
  if [ -s "$list" ]; then tr '\n' '\0' < "$list" | xargs -0 "${SHA[@]}"; fi
  rm -f "$list"
}

# snapshot_take <label> records the project as it is now and prints the snapshot id.
snapshot_take() {
  local label="$1" st dir obj tmp id line h f
  st="$TASK_DIR/.watts-state"; dir="$st/snapshots"; obj="$st/objects"
  mkdir -p "$dir" "$obj"
  tmp="$(mktemp)"
  snap_manifest > "$tmp"
  while IFS= read -r line; do
    h="${line%%  *}"; f="${line#*  }"
    if [ ! -e "$obj/$h" ] && [ "$(( $(wc -c < "$f") ))" -le 1048576 ]; then cp "$f" "$obj/$h"; fi
  done < "$tmp"
  # The label is part of the id, so two snapshots of an unchanged project taken in the same second
  # (the agent's, then the reviewer's start) can never overwrite each other.
  id="S$(date +%s)-$( { echo "$label"; cat "$tmp"; } | "${SHA[@]}" | cut -c1-8)"
  { echo "# label $label"; cat "$tmp"; } > "$dir/$id"
  rm -f "$tmp"
  echo "$id"
}

# snap_times <label> prints the epoch second of every snapshot with that label, oldest first.
snap_times() {
  local f b d="$TASK_DIR/.watts-state/snapshots"
  [ -d "$d" ] || return 0
  for f in "$d"/S*; do
    [ -f "$f" ] || continue
    if [ "$(head -n 1 "$f")" = "# label $1" ]; then b="${f##*/}"; b="${b#S}"; echo "${b%%-*}"; fi
  done | sort -n
}

snap_epoch() { local b="${1#S}"; echo "${b%%-*}"; }

# manifest_diff <manifest-a> <manifest-b> prints "A path", "M path" or "D path" for every
# difference. Lines starting with # are ignored.
manifest_diff() {
  awk -v f1="$1" '/^#/ { next } { h = substr($0, 1, 64); p = substr($0, 67); if (FILENAME == f1) a[p] = h; else b[p] = h }
    END { for (p in b) { if (!(p in a)) print "A " p; else if (a[p] != b[p]) print "M " p } for (p in a) if (!(p in b)) print "D " p }' "$1" "$2" | LC_ALL=C sort -k2
}

# When the build agent first and last took a snapshot.
agent_first_time() { snap_times agent | head -n 1 || true; }
agent_last_time()  { snap_times agent | tail -n 1 || true; }

PROBLEMS=0
ok()   { echo "ok:   $*"; }
fail() { echo "FAIL: $*"; PROBLEMS=$((PROBLEMS + 1)); }
