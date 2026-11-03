#!/usr/bin/env bash
# choir-scope-check.sh [--base REV] <allowed-prefix>...
#
# Lists every changed and untracked path in the current git tree against the
# base and exits 1 naming each path outside the allowed prefixes. A prefix is a
# file or a directory: "internal/x" allows internal/x and internal/x/y.go but
# not internal/xy.go. Exit 0: every path is in scope (or nothing changed).
# Exit 1: a path is out of scope. Exit 2: bad usage, not a git tree, or a base
# that does not resolve (a scope check that cannot run must not read as a pass).
#
# The base is --base, else $CHOIR_SCOPE_BASE, else HEAD. A capsule sees no
# /selfdev/base mount yet (the ledger holds choir-diff back for that), so HEAD
# is the base there: the capsule's work is uncommitted changes on HEAD. Outside
# a capsule pass --base $(git merge-base HEAD origin/main) to cover commits.
# Renames count as a delete plus an add, so both the old and new path are
# checked (the file-to-dir swap that dropped choir-diff.sh cannot hide one).
set -u -o pipefail

base="${CHOIR_SCOPE_BASE:-HEAD}"
if [ "${1:-}" = "--base" ]; then
	[ $# -ge 2 ] || { echo "choir-scope-check: --base needs a revision" >&2; exit 2; }
	base="$2"
	shift 2
fi
if [ $# -eq 0 ] || [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
	echo "usage: choir-scope-check.sh [--base REV] <allowed-prefix>..." >&2
	exit 2
fi

git rev-parse --git-dir >/dev/null 2>&1 || { echo "choir-scope-check: not a git work tree: $PWD" >&2; exit 2; }
git rev-parse --verify --quiet "$base^{commit}" >/dev/null || { echo "choir-scope-check: base $base does not resolve to a commit" >&2; exit 2; }
cd "$(git rev-parse --show-toplevel)" || exit 2

allowed=()
for p in "$@"; do
	p="${p#./}"
	p="${p%/}"
	[ -n "$p" ] || { echo "choir-scope-check: empty or root prefix refused" >&2; exit 2; }
	allowed+=("$p")
done

changed="$(git diff --name-only -z --no-renames "$base" -- | tr '\0' '\n')" || { echo "choir-scope-check: git diff failed; cannot check scope" >&2; exit 2; }
untracked="$(git ls-files -z --others --exclude-standard | tr '\0' '\n')" || { echo "choir-scope-check: git ls-files failed; cannot check scope" >&2; exit 2; }
paths="$(printf '%s\n%s\n' "$changed" "$untracked" | sed '/^$/d' | sort -u)" || { echo "choir-scope-check: cannot enumerate paths; cannot check scope" >&2; exit 2; }
[ -n "$paths" ] || { echo "choir-scope-check: no changes against $base"; exit 0; }

bad=0
total=0
while IFS= read -r path; do
	[ -n "$path" ] || continue
	total=$((total + 1))
	ok=0
	for p in "${allowed[@]}"; do
		case "$path" in
		"$p" | "$p"/*) ok=1; break ;;
		esac
	done
	if [ "$ok" -eq 0 ]; then
		echo "OUT OF SCOPE: $path"
		bad=$((bad + 1))
	fi
done <<<"$paths"

if [ "$bad" -gt 0 ]; then
	echo "choir-scope-check: $bad of $total changed paths are outside: ${allowed[*]}" >&2
	exit 1
fi
echo "choir-scope-check: $total changed paths, all inside: ${allowed[*]}"
