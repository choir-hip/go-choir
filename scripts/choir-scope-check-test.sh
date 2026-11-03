#!/usr/bin/env bash
# choir-scope-check.sh against a temp git repo. Ways it could fail, written
# first: an out-of-scope edit, an untracked file, a deletion or a rename
# source passes; a sibling dir sharing a prefix (internal/xy vs internal/x)
# passes; a bad base or non-repo reads as a pass; an in-scope change fails.
set -u
script="$(cd "$(dirname "$0")" && pwd)/choir-scope-check.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail=0
check() { # name expected-exit actual-exit [output-needle output]
	if [ "$2" != "$3" ]; then echo "FAIL $1: exit $3, want $2"; fail=1; return; fi
	if [ -n "${4:-}" ] && ! printf '%s' "$5" | grep -qF -- "$4"; then echo "FAIL $1: output lacks '$4': $5"; fail=1; return; fi
	echo "ok   $1"
}
run() { out="$(cd "$tmp/r" && bash "$script" "$@" 2>&1)"; rc=$?; }

git init -q "$tmp/r" && cd "$tmp/r" || exit 1
git config user.email t@t && git config user.name t
mkdir -p internal/x internal/xy docs
echo a >internal/x/a.go; echo b >internal/xy/b.go; echo c >docs/c.md; echo d >internal/x/gone.go
git add -A && git commit -qm base

run internal/x;                                   check "clean tree" 0 $rc "no changes" "$out"
echo a2 >>internal/x/a.go;                        run internal/x; check "in-scope edit" 0 $rc "all inside" "$out"
echo b2 >>internal/xy/b.go;                       run internal/x; check "sibling prefix is out" 1 $rc "OUT OF SCOPE: internal/xy/b.go" "$out"
git checkout -q internal/xy/b.go
echo n >docs/new.md;                              run internal/x; check "untracked out" 1 $rc "OUT OF SCOPE: docs/new.md" "$out"
rm docs/new.md
git rm -q docs/c.md;                              run internal/x; check "staged deletion out" 1 $rc "OUT OF SCOPE: docs/c.md" "$out"
git reset -q --hard && echo a2 >>internal/x/a.go
git mv internal/x/gone.go docs/moved.go;          run internal/x; check "rename out names the new path" 1 $rc "OUT OF SCOPE: docs/moved.go" "$out"
git reset -q --hard
echo a2 >>internal/x/a.go;                        run internal/x/;  check "trailing slash prefix" 0 $rc "" "$out"
run internal/x/a.go;                              check "file prefix" 0 $rc "" "$out"
run --base nope internal/x;                       check "bad base is not a pass" 2 $rc "does not resolve" "$out"
run;                                              check "no prefix" 2 $rc "usage" "$out"
git add -A && git commit -qm change
run --base HEAD~1 docs;                           check "committed change vs --base" 1 $rc "OUT OF SCOPE: internal/x/a.go" "$out"
out="$(cd "$tmp" && bash "$script" internal 2>&1)"; rc=$?; check "not a repo" 2 $rc "not a git work tree" "$out"
git init -q "$tmp/bad" && (cd "$tmp/bad" && git commit -q --allow-empty -m x && echo junk >.git/index)
out="$(cd "$tmp/bad" && bash "$script" a 2>&1)"; rc=$?; check "corrupt index is not a pass" 2 $rc "cannot check scope" "$out"
exit $fail
