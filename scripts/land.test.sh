#!/usr/bin/env bash
# Selftest for scripts/land. Builds a throwaway git repo with a tiny Go module
# and asserts the reverse-dependency computation and the train/push mechanics.
#
# Failure list (written before the code), and how each is covered:
#   stale origin            land fetches origin first and dies if the fetch fails
#                           (untested here: needs a broken remote)
#   lane already merged     case "merged lane": skipped, and a train of only merged
#                           lanes is an empty train
#   empty train             case "empty train": exit 1, nothing pushed
#   package deleted by the  case "deleted package": a NOTE, no crash, importers are
#     change                left to go build ./... in the real run
#   test-only importer      package d imports a only from d_test.go: selected as
#                           testimporter; a _test.go change in a selects a alone
#   build-tagged files      package e imports a only in a linux-only file: still
#                           selected (graph = linux/amd64 union darwin/arm64)
#   VM unreachable          chosen: FAIL with a hint to use --local (exit 4), never
#                           a silent fallback; case "VM unreachable"
#   ssh drop mid-run        the run is detached (nohup setsid) and the poll loop
#                           tolerates LAND_POLL_FAILS consecutive failed polls;
#                           untested here (needs a real VM)
#   main moving twice       case "main moves once" (rebase + push) and case
#                           "main moves twice" (gives up, pushes nothing), via
#                           LAND_HOOK_AFTER_VERIFY
#   conflict                case "conflict": names the lane, exits 1
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd -P)
LAND=$here/land
T=$(mktemp -d "${TMPDIR:-/tmp}/land-selftest.XXXXXX")
trap 'rm -rf "$T"' EXIT
pass=0; failn=0
ok() { pass=$((pass+1)); echo "  ok   $1"; }
bad() { failn=$((failn+1)); echo "  FAIL $1"; shift; [ $# -gt 0 ] && printf '%s\n' "$@" | sed 's/^/       /'; }
expect() { # name expected actual
  if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "want: $2" "got:  $3"; fi
}
G() { git -c user.name=t -c user.email=t@example.com "$@"; }

# ---- the throwaway module
M=$T/m; mkdir -p "$M" && cd "$M" || exit 2
git init -q -b main . 2>/dev/null || git init -q .
mkdir a b c d e docs
echo 'module example.com/t

go 1.22' >go.mod
echo 'package a
func A() int { return 1 }' >a/a.go
echo 'package a
import "testing"
func TestA(t *testing.T) { _ = A() }' >a/a_test.go
echo 'package b
import "example.com/t/a"
func B() int { return a.A() }' >b/b.go
echo 'package c
func C() int { return 3 }' >c/c.go
echo 'package d' >d/d.go
echo 'package d
import ("testing"; "example.com/t/a")
func TestD(t *testing.T) { _ = a.A() }' >d/d_test.go
echo 'package e' >e/e.go
printf '//go:build linux\n\npackage e\nimport "example.com/t/a"\nvar _ = a.A\n' >e/e_linux.go
echo '# docs' >docs/x.md
G add -A && G commit -q -m base
base=$(git rev-parse HEAD)

sel() { # from-ref -> "rel:class" sorted, NOTE lines as note:...
  "$LAND" --select "$1" 2>&1 | awk -F'\t' '$1=="PKG"{print $2":"$3} $1=="NOTE"{print "note"}' | sort | tr '\n' ' '
}
change() { # file content msg
  mkdir -p "$(dirname "$1")"; printf '%s\n' "$2" >"$1"; G add -A; G commit -q -m "$3"
}

echo "reverse-dependency computation"
change a/a.go 'package a
func A() int { return 2 }' "change a"
expect "change a selects a,b,d(test import),e(tagged); not c" "./a:changed ./b:importer ./d:testimporter ./e:importer " "$(sel "$base")"
G reset -q --hard "$base"

change a/a_test.go 'package a
import "testing"
func TestA(t *testing.T) { _ = A() + 0 }' "test only"
expect "only a_test.go in a selects a alone" "./a:test " "$(sel "$base")"
G reset -q --hard "$base"

change a/testdata/x.txt 'x' "testdata"
expect "testdata in a selects a alone" "./a:test " "$(sel "$base")"
G reset -q --hard "$base"

change docs/y.md 'more' "docs"
expect "docs change selects nothing" "note " "$(sel "$base")"
G reset -q --hard "$base"

change c/c.go 'package c
func C() int { return 4 }' "change c"
expect "change c selects c only" "./c:changed " "$(sel "$base")"
G reset -q --hard "$base"

change go.mod 'module example.com/t

go 1.22
' "gomod"
expect "go.mod change selects every package" "./a:changed ./b:changed ./c:changed ./d:changed ./e:changed " "$(sel "$base")"
G reset -q --hard "$base"

git rm -rq c && G commit -q -m "delete c"
expect "deleted package: a note, nothing else, no crash" "note " "$(sel "$base")"
G reset -q --hard "$base"

# ---- the land flow: bare origin, a worktree, lanes
echo "train and push mechanics"
O=$T/origin.git; git init -q --bare -b main "$O" 2>/dev/null || git init -q --bare "$O"
git remote add origin "$O"; git push -q origin HEAD:main
W=$T/wt; git worktree add -q --detach "$W" origin/main
export LAND_GATES=true LAND_PRIMARY=/nonexistent LAND_POLL=1
land() { ( cd "$W" && "$LAND" "$@" 2>&1 ); }
lane() { # name file content : a lane branch off origin/main
  git checkout -q -B "$1" origin/main 2>/dev/null; mkdir -p "$(dirname "$2")"; printf '%s\n' "$3" >"$2"; G add -A; G commit -q -m "lane $1"
}
lane laneA a/a.go 'package a
func A() int { return 7 }'
lane laneC c/c.go 'package c
func C() int { return 9 }'
git checkout -q --detach "$base"

out=$(land --dry-run laneA laneC); rc=$?
expect "dry-run exits 0" 0 $rc
case "$out" in *"./b "*importer*"imports ./a (./a changed)"*) ok "dry-run names the importer and why";; *) bad "dry-run plan" "$out";; esac
case "$out" in *"./c "*changed*) ok "dry-run lists lane C's package";; *) bad "dry-run lists c" "$out";; esac
expect "dry-run pushed nothing" "$base" "$(git --git-dir="$O" rev-parse main)"

out=$(land --local laneA --no-push); rc=$?
expect "local verify of a train passes" 0 $rc
case "$out" in *"suites passed"*) ok "suites ran";; *) bad "suites ran" "$out";; esac
expect "--no-push left main alone" "$base" "$(git --git-dir="$O" rev-parse main)"

out=$(land nosuchlane); expect "unresolvable lane refused" 1 $?

out=$(land --local laneA laneC); rc=$?
expect "local land pushes" 0 $rc
newmain=$(git --git-dir="$O" rev-parse main)
expect "main advanced by the two lane commits" 2 "$(git --git-dir="$O" rev-list --count "$base..$newmain")"
case "$out" in *"Ring-Expedite: NO"*) ok "report says which commits lack Ring-Expedite";; *) bad "expedite report" "$out";; esac

out=$(land --local laneA laneC); rc=$?
expect "lane already merged: empty train, exit 1" 1 $rc
case "$out" in *"empty train"*) ok "empty train named";; *) bad "empty train message" "$out";; esac

# conflict: a lane editing the same line as main now has
git fetch -q origin main
git checkout -q -B laneX "$base" 2>/dev/null   # branched before laneA landed
printf 'package a\nfunc A() int { return 99 }\n' >a/a.go; G add -A; G commit -q -m "lane laneX"
git checkout -q --detach "$base"
out=$(land --local laneX); rc=$?
expect "conflict exits 1" 1 $rc
case "$out" in *"CONFLICT in lane 'laneX'"*) ok "conflict names the lane";; *) bad "conflict message" "$out";; esac

# a failing test: nothing is pushed
git fetch -q origin main; cur=$(git --git-dir="$O" rev-parse main)
lane laneBad a/a_test.go 'package a
import "testing"
func TestA(t *testing.T) { t.Fatal("boom") }'
out=$(land --local laneBad); rc=$?
expect "red suite exits 1" 1 $rc
case "$out" in *"FAIL"*"example.com/t/a"*|*"--- FAIL: TestA"*) ok "failing package shown";; *) bad "failure output" "$out";; esac
expect "red suite pushed nothing" "$cur" "$(git --git-dir="$O" rev-parse main)"

# main moves during the push window. mover.sh <limit> <file> advances main by one
# commit editing <file>, at most <limit> times per case (counter file).
printf '%s\n' '#!/usr/bin/env bash' 'set -e' \
  'limit=$1; file=$2; cnt=$T/mover.count; n=$(cat "$cnt" 2>/dev/null || echo 0)' \
  '[ "$n" -lt "$limit" ] || exit 0' 'echo $((n+1)) >"$cnt"' \
  'rm -rf "$T/mv"; git clone -q "$O" "$T/mv"; cd "$T/mv"' \
  'mkdir -p "$(dirname "$file")"; echo "// $RANDOM" >>"$file"' \
  'git add -A; git -c user.name=t -c user.email=t@example.com commit -q -m "mover $n"' \
  'git push -q origin HEAD:main' >"$T/mover.sh"
export T O
resetmover() { rm -f "$T/mover.count"; }
moved_run() { # lane file limit -> runs land with main moving
  resetmover
  out=$(cd "$W" && LAND_HOOK_AFTER_VERIFY="bash $T/mover.sh $3 $2" "$LAND" --local "$1" 2>&1); rc=$?
}
newlane() { git fetch -q origin main; lane "$1" b/b.go "package b
import \"example.com/t/a\"
func B() int { return a.A() + $2 }"; }

newlane laneB2 1; moved_run laneB2 docs/m.md 1
expect "main moves once (docs): rebased and pushed" 0 $rc
case "$out" in *"rebasing"*"touch nothing under test"*) ok "unrelated move skips the rerun";; *) bad "docs move" "$out";; esac

newlane laneB4 4; moved_run laneB4 c/c.go 1
expect "main moves once (package c, not a dependency): pushed" 0 $rc
case "$out" in *"touch nothing under test"*) ok "c is not a dependency of the train";; *) bad "c move" "$out";; esac

newlane laneB5 5; moved_run laneB5 a/a.go 1
expect "main moves once (a, a dependency): pushed after rerun" 0 $rc
case "$out" in *"rerunning from the plan"*) ok "moved dependency forces the rerun";; *) bad "dependency move" "$out";; esac

newlane laneB3 3; moved_run laneB3 docs/m.md 5
expect "main moves twice: gives up" 1 $rc
case "$out" in *"moved again"*) ok "give-up message";; *) bad "twice message" "$out";; esac
expect "main moves twice: nothing of the train pushed" 0 "$(git --git-dir="$O" log --format=%s main | grep -c '^lane laneB3$')"

# VM unreachable: fail with a hint, do not fall back
git fetch -q origin main
lane laneVM c/c.go 'package c
func C() int { return 11 }'
out=$(cd "$W" && LAND_VM=nonexistent.invalid "$LAND" laneVM 2>&1); rc=$?
expect "VM unreachable: exit 4" 4 $rc
case "$out" in *"--local"*) ok "hint names --local";; *) bad "VM hint" "$out";; esac

# guards
out=$(cd "$M" && "$LAND" --dry-run laneA 2>&1); rc=$?
expect "primary checkout refused" 1 $rc
echo dirty >"$W/untracked.txt"
out=$(land --dry-run laneC); expect "dirty tree refused" 1 $?
rm -f "$W/untracked.txt"

echo
echo "selftest: $pass passed, $failn failed"
[ "$failn" = 0 ]
