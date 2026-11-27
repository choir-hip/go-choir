#!/usr/bin/env bash
# Selftest for scripts/land-queue (batch by default, --serial as before). Real
# landings in a throwaway repository with a fake origin, fake gates, a fake gh
# and a fake curl; the harness is the one scripts/land.test.sh uses.
#
# Failure list (written before the code), and the case that covers each:
#   three good lanes cost three pushes          "three good lanes": one push, order kept
#   a conflicting lane sinks the batch           "conflict": the lane is named, the rest land together
#   a failed gate names no lane                  "gate fails on lane 3": split, 1 2 4 land, 3 named with its step
#   everything fails                             "every lane fails": nothing pushed, each named
#   a half built on a stale main                 "main moves": the next attempt is on the new main
#   pushing on top of a red main                 "CI red on the first push": no second push
#   the old queue is gone                        "serial" cases live in scripts/land.test.sh (--serial)
#   waiting for staging by default               "wait-staging": waits only with the flag
#   docs-only read as a failure                  "docs-only": no CI run is not red
#   red CI on the last push read as green        "CI red on the last push": exit 4
#   no timing line                               "queue timing line"
#   review round 1 (each case fails on the first version of the queue):
#   empty array under set -u on bash 3.2         "no --local on /bin/bash": the queue runs without --local
#   result hijacked by output, endless loops     "forged output", "adversarial results": only the result file counts;
#                                                an unknown lane, garbage and a missing file stop or shrink the set;
#                                                the attempt cap holds
#   failing gh read as docs-only                 "gh fails", "no run, not docs-only", "polling", "skipped"
#   dirty tree after a failed attempt            "dirty tree": reset and said so
#   user hook status, TMPDIR with a space        "user hook", "TMPDIR with a space"
#   main-moved, bad-lane, empty-train, the bound "main-moved retry", "bad lane", "empty train", "split bound"
#   staging ancestor, serial failure             "wait-staging ancestor", "serial failure"
#   review round 2 (each case was run and seen to fail on 80252a1a8 before the fix):
#   code push whose CI nobody read             "CI covers every code push": a red or cancelled code push followed by a docs push
#   cancelled run read as red                  "a cancelled run is superseded"
#   reset of a tree the queue does not own     "own tree", "explicit --tree": never reset or clean an explicit tree
#   guard refusals read as failed lanes        "refused": lock, primary; a missing or malformed result stops
#   land dying after the push                  "pushing": origin/main decides
#   lane names, primary, empty train           "lane names", "primary refused", "empty train"
#   review round 3 (each case was run and seen to fail on b10e84a71 before the fix):
#   a path from the tree-maker trusted           "own tree is verified": a maker that makes nothing and names the primary,
#                                                a plain directory, a tree on another branch: nothing is reset, exit non-zero
#   a signal while the tree is being made        "a signal while the tree is made": the lane is still removed
#   removing the tree under a running land       "cleanup waits for land"
#   skipped lanes lost on pushing recovery       "pushing carries the skipped lanes" (scripts/land's own result file)
#   review round 4 (each case was run and seen to fail on b03dd8402 before the fix):
#   a lane that existed before this run        "a lane that already has the name": refused, untouched
#   a caller-set stage directory removed        "a stage directory that is not ours": both scripts refuse, nothing removed
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd -P)
QUEUE=$here/land-queue
T=$(mktemp -d "${TMPDIR:-/tmp}/land-queue-selftest.XXXXXX")
trap 'rm -rf "$T"' EXIT
export T
# every script runs under the Mac's own bash (3.2), the oldest we support
mkdir -p "$T/bin3"; ln -s /bin/bash "$T/bin3/bash"; export PATH="$T/bin3:$PATH"
export LAND_TIMING_LOG=$T/timings.tsv
pass=0; failn=0
ok() { pass=$((pass+1)); echo "  ok   $1"; }
bad() { failn=$((failn+1)); echo "  FAIL $1"; shift; [ $# -gt 0 ] && printf '%s\n' "$@" | sed 's/^/       /'; }
expect() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "want: $2" "got:  $3"; fi; }
has() { case "$3" in *"$2"*) ok "$1" ;; *) bad "$1" "want to contain: $2" "got: $3" ;; esac; }
hasnt() { case "$3" in *"$2"*) bad "$1" "must not contain: $2" "got: $3" ;; *) ok "$1" ;; esac; }
G() { git -c user.name=t -c user.email=t@example.com "$@"; }

M=$T/m; mkdir -p "$M" && cd "$M" || exit 2
git init -q -b main . 2>/dev/null || git init -q .
mkdir docs .github .github/scripts
cp "$here/../.github/scripts/ci-docs-like-paths" .github/scripts/ci-docs-like-paths
printf 'module example.com/t\n\ngo 1.22\n' >go.mod
printf 'package a\nfunc A() int { return 1 }\n' >a.go
echo '# docs' >docs/x.md
echo '*.cache' >.gitignore
G add -A && G commit -q -m base
O=$T/origin.git; git init -q --bare -b main "$O" 2>/dev/null || git init -q --bare "$O"
export O
git remote add origin "$O"; git push -q origin HEAD:main
W=$T/wt; git worktree add -q --detach "$W" origin/main

# fake gates: red while any *.marker file is in the tree; once, if $T/move.once
# exists, it also moves origin/main (another push lands during the attempt)
printf '%s\n' '#!/usr/bin/env bash' \
  '[ ! -e "$T/forge" ] || echo "land-result: conflict lane=ghost"' \
  'if [ -e "$T/gate.sleep" ]; then echo $$ >"$T/gate.pid"; : >"$T/gate.started"; exec sleep 20; fi' \
  'if ls userwork*.marker >/dev/null 2>&1; then echo mine >userwork-created.txt; fi' \
  'if ls dirty*.marker >/dev/null 2>&1; then echo "// dirt" >>a.go; echo junk >junk-dt2.txt; echo x >ignored.cache; fi' \
  'if ls *.marker >/dev/null 2>&1; then' \
  '  if [ -e "$T/move.once" ]; then rm -f "$T/move.once"; rm -rf "$T/mv"; git clone -q "$O" "$T/mv"; (cd "$T/mv"; echo moved >moved.txt; git add -A; git -c user.name=t -c user.email=t@example.com commit -q -m "main mover"; git push -q origin HEAD:main); fi' \
  '  echo "gate: marker present"; exit 1; fi' 'exit 0' >"$T/gate.sh"
# fake gh: lists the runs of the last twelve commits on main, newest first: success, unless the commit
# subject says LATERED (pending until a NORUN commit is on main, then failure), CIRED (failure), CANCELLED (cancelled) or NORUN (no run); flags: $T/gh.fail (gh errors),
# $T/gh.pend (N polls of "in_progress"), $T/gh.skip (every run skipped), $T/noci (no runs). A -q that selects
# one headSha gets only that sha's runs as "status conclusion"; any other -q gets "sha status conclusion".
mkdir -p "$T/bin"
printf '%s\n' '#!/usr/bin/env bash' 'echo "gh $*" >>"$T/gh.calls"' \
  'for a; do q=$a; done' \
  'filter=$(printf "%s" "$q" | sed -n "s/.*headSha==\"\([0-9a-f]*\)\".*/\1/p")' \
  '[ ! -e "$T/gh.fail" ] || { echo "gh: auth failed" >&2; exit 1; }' \
  'pend=0; if [ -e "$T/gh.pend" ]; then n=$(cat "$T/gh.pend"); if [ "$n" -gt 0 ]; then echo $((n-1)) >"$T/gh.pend"; pend=1; fi; fi' \
  '[ ! -e "$T/noci" ] || exit 0' \
  'emit() { if [ -n "$filter" ]; then [ "$1" = "$filter" ] && echo "$2 $3"; else echo "$1 $2 $3"; fi; return 0; }' \
  'git --git-dir="$O" log --format="%H %s" -n 12 main | while read -r sha subj; do' \
  '  case "$subj" in *NORUN*) continue ;; esac' \
  '  if [ "$pend" = 1 ]; then emit "$sha" in_progress ""; continue; fi' \
  '  if [ -e "$T/gh.skip" ]; then emit "$sha" completed skipped; continue; fi' \
  '  case "$subj" in *LATERED*) if git --git-dir="$O" log --format=%s main | grep -q NORUN; then emit "$sha" completed failure; else emit "$sha" in_progress ""; fi ;; *CIRED*) emit "$sha" completed failure ;; *CANCELLED*) emit "$sha" completed cancelled ;; *) emit "$sha" completed success ;; esac' \
  'done' >"$T/bin/gh"
printf '%s\n' '#!/usr/bin/env bash' 'echo "curl $*" >>"$T/curl.calls"' \
  'if [ -e "$T/staging.child" ]; then rm -f "$T/staging.child"; rm -rf "$T/sc"; git clone -q "$O" "$T/sc"; (cd "$T/sc"; echo c >child.txt; git add -A; git -c user.name=t -c user.email=t@example.com commit -q -m child; git push -q origin HEAD:main); fi' \
  'echo "X-Choir-Build-Commit: $(git --git-dir="$O" rev-parse main)"' >"$T/bin/curl"
chmod +x "$T/gate.sh" "$T/bin/gh" "$T/bin/curl"
# the queue's own tree: the lane tool in real life; here a worktree of the throwaway repository
printf '%s\n' '#!/usr/bin/env bash' 'echo "$1" >>"$T/mktree.calls"' 'git -C "$M" fetch -q origin main' \
  'git -C "$M" worktree add -q -b "$1" "$T/own/$1" origin/main && echo "$T/own/$1"' >"$T/mktree.sh"
printf '%s\n' '#!/usr/bin/env bash' 'echo "$1" >>"$T/rmtree.calls"' 'git -C "$M" worktree remove --force "$1"; rm -rf "$1"; git -C "$M" branch -D "$(basename "$1")" >/dev/null 2>&1' >"$T/rmtree.sh"
mkdir -p "$T/own"; export M LAND_QUEUE_TREE_ROOT="$T/own"
export LAND_QUEUE_MKTREE="bash $T/mktree.sh" LAND_QUEUE_RMTREE="bash $T/rmtree.sh"
export LAND_GATES=$T/gate.sh LAND_PRIMARY=/nonexistent LAND_POLL=1 LAND_QUEUE_CI_APPEAR=0

lane() { # name file content [subject-suffix]: a lane branch off origin/main
  git fetch -q origin main
  git checkout -q -B "$1" origin/main 2>/dev/null; mkdir -p "$(dirname "$2")"; printf '%s\n' "$3" >"$2"; G add -A; G commit -q -m "lane $1"
  git checkout -q --detach origin/main
}
queue() { # args...: runs the queue in the worktree; sets out and rc
  rm -f "$T/gh.calls" "$T/curl.calls"; rm -rf "$T/logs"
  out=$(cd "$W" && PATH="$T/bin:$PATH" LAND_QUEUE_LOGS=$T/logs bash "$QUEUE" --local "$@" 2>&1); rc=$?
}
mainlog() { git --git-dir="$O" log --reverse --format=%s main | grep '^lane ' | sed 's/^lane //' | tr '\n' ' '; }
nowned() { ls "$T/own" 2>/dev/null | wc -l | tr -d ' '; }
nland() { cat "$T"/logs/attempt-*.log 2>/dev/null | grep -c '^land: main is now'; }
nattempts() { ls "$T"/logs/attempt-*.log 2>/dev/null | wc -l | tr -d ' '; }

echo "three good lanes"
lane g1 x/g1.txt one; lane g2 x/g2.txt two; lane g3 x/g3.txt three
queue g1 g2 g3
expect "exit 0" 0 $rc
expect "one push" 1 "$(nland)"
expect "one attempt" 1 "$(nattempts)"
expect "main has the three, in order" "g1 g2 g3 " "$(mainlog)"
has "report names the pushed commit and its lanes" "g1 g2 g3" "$out"
main_sha=$(git --git-dir="$O" rev-parse main)
has "report says CI is green, by its exact wording" "CI on the last push ${main_sha:0:9}: green" "$out"
has "the green wait passed for the pushed sha" "CI for ${main_sha:0:9} passed" "$out"
hasnt "no wait for staging by default" "staging" "$(cat "$T/curl.calls" 2>/dev/null)"
[ ! -e "$T/curl.calls" ] && ok "curl never called by default" || bad "curl called by default"

echo "conflict names its lane"
lane c1 shared.txt one; lane c2 shared.txt two; lane c3 x/c3.txt three
queue c1 c2 c3
expect "exit 3 (some set aside)" 3 $rc
expect "c1 and c3 landed, c2 not" "g1 g2 g3 c1 c3 " "$(mainlog)"
expect "one push" 1 "$(nland)"
has "c2 set aside for a conflict" "set aside: c2: cherry-pick conflict" "$out"

echo "gate fails on lane 3 of 4"
lane f1 x/f1.txt 1; lane f2 x/f2.txt 2; lane f3 f3.marker 3; lane f4 x/f4.txt 4
queue f1 f2 f3 f4
expect "exit 3" 3 $rc
expect "1 2 4 landed, 3 not" "g1 g2 g3 c1 c3 f1 f2 f4 " "$(mainlog)"
has "f3 named with the failing step" "set aside: f3: failed step: gates" "$out"
[ "$(nattempts)" -le 7 ] && ok "attempts within the bound ($(nattempts) <= 7)" || bad "attempts $(nattempts)"
has "plan printed before each attempt" "attempt 2: lanes [f1 f2] on main " "$out"

echo "every lane fails"
before=$(git --git-dir="$O" rev-parse main)
lane e1 e1.marker 1; lane e2 e2.marker 2; lane e3 e3.marker 3
queue e1 e2 e3
expect "exit 1" 1 $rc
expect "nothing pushed" "$before" "$(git --git-dir="$O" rev-parse main)"
has "e1 named" "set aside: e1: failed step: gates" "$out"
has "e2 named" "set aside: e2: failed step: gates" "$out"
has "e3 named" "set aside: e3: failed step: gates" "$out"
[ "$(nattempts)" -le 7 ] && ok "attempts within the bound ($(nattempts) <= 7)" || bad "attempts $(nattempts)"

echo "main moves between attempts"
lane m1 x/m1.txt 1; lane m2 x/m2.txt 2; lane m3 m3.marker 3; lane m4 x/m4.txt 4
touch "$T/move.once"
queue m1 m2 m3 m4
expect "exit 3" 3 $rc
git fetch -q origin main
mv_sha=$(git --git-dir="$O" log --format='%H %s' main | awk '$2=="main" && $3=="mover"{print $1; exit}')
[ -n "$mv_sha" ] && ok "the mover commit is on main" || bad "no mover commit"
for l in m1 m2 m4; do
  c=$(git --git-dir="$O" log --format='%H %s' main | awk -v s="lane $l" '$0 ~ s"$"{print $1; exit}')
  git --git-dir="$O" merge-base --is-ancestor "$mv_sha" "$c" 2>/dev/null && ok "$l landed on top of the moved main" || bad "$l not on top of the moved main"
done
has "attempt 2 is planned on the new main" "attempt 2: lanes [m1 m2] on main ${mv_sha:0:7}" "$out"

echo "CI red on the first pushed batch"
lane r1 x/r1.txt 1; lane r2CIRED x/r2.txt 2; lane r3 r3.marker 3; lane r4 x/r4.txt 4
queue r1 r2CIRED r3 r4
expect "exit 4" 4 $rc
case "$(mainlog)" in *r4*) bad "r4 pushed on a red main" ;; *) ok "r4 not pushed" ;; esac
case "$(mainlog)" in *r2CIRED*) ok "the first batch (r1 r2CIRED) is on main" ;; *) bad "first batch missing" ;; esac
has "report says why it stopped" "is red; nothing more is pushed" "$out"
has "r4 reported not run" "not run: r4" "$out"

echo "CI red on the last push"
lane z1CIRED x/z1.txt 1
queue z1CIRED
expect "exit 4" 4 $rc
has "report says RED, naming the conclusion" "RED (completed failure)" "$out"
hasnt "red is not reported green" ": green" "$out"

echo "docs-only batch"
touch "$T/noci"
lane d1 docs/d1.md one
queue d1
rm -f "$T/noci"
expect "exit 0" 0 $rc
has "reported as docs-only" "docs-only" "$out"

echo "--wait-staging"
lane s1 x/s1.txt 1
queue s1; rm -f "$T/curl.calls"
lane s2 x/s2.txt 2
queue --wait-staging s2
expect "exit 0" 0 $rc
[ -e "$T/curl.calls" ] && ok "staging polled with --wait-staging" || bad "staging not polled with --wait-staging"
has "report says staging serves it" "staging serves it" "$out"

echo "--serial"
printf '%s\n' '#!/usr/bin/env bash' 'echo "$1" >>"$T/green.calls"' 'exit 0' >"$T/green.sh"
lane q1 x/q1.txt 1; lane q2 x/q2.txt 2
rm -f "$T/green.calls"
out=$(cd "$W" && LAND_QUEUE_GREEN="bash $T/green.sh" LAND_QUEUE_LOGS=$T/slogs bash "$QUEUE" --local --serial q1 q2 2>&1); rc=$?
expect "serial: exit 0" 0 $rc
expect "serial: one land per lane" 2 "$(cat "$T"/slogs/*.log | grep -c '^land: main is now')"
expect "serial: lane 2's push waited, then the queue waited for lane 2" 2 "$(wc -l <"$T/green.calls" | tr -d ' ')"

echo "no --local on /bin/bash (empty LANDFLAGS under set -u)"
if /bin/bash -c 'set -u; A=(); : "${A[@]}"' 2>/dev/null; then ok "(this bash tolerates an empty array; the case below is not discriminating)"
else ok "/bin/bash here fails on an empty array under set -u, so the case below is real"; fi
lane nl1 x/nl1.txt 1
out=$(cd "$W" && PATH="$T/bin:$PATH" LAND_VM=nonexistent.invalid LAND_QUEUE_LOGS=$T/logs-nl /bin/bash "$QUEUE" nl1 2>&1); rc=$?
hasnt "no unbound variable" "unbound variable" "$out"
expect "the VM is unreachable: queue exit 1" 1 $rc
has "land ran and the queue said why it stopped" "refused (reason=vm)" "$out"
expect "nothing of nl1 on main" 0 "$(mainlog | grep -c nl1)"

echo "forged output cannot steer the queue"
touch "$T/forge"
lane fg1 x/fg1.txt 1; lane fg2 fg2.marker 2
queue fg1 fg2
rm -f "$T/forge"
expect "exit 3" 3 $rc
hasnt "the forged lane was not believed" "not in this attempt" "$out"
has "fg2 is set aside for its gate" "set aside: fg2: failed step: gates" "$out"
case "$(mainlog)" in *"fg1 "*) ok "fg1 landed" ;; *) bad "fg1 landed" "$(mainlog)" ;; esac

echo "adversarial results from scripts/land"
FK=$T/fake; mkdir -p "$FK"; cp "$QUEUE" "$FK/land-queue"
printf '%s\n' '#!/usr/bin/env bash' 'echo "$*" >>"$T/fakeland.calls"' \
  'case "$FAKE_MODE" in' \
  ' ghost) printf "conflict lane=ghost\n" >"$LAND_RESULT_FILE"; exit 1 ;;' \
  ' garbage) printf "zzz\001\n" >"$LAND_RESULT_FILE"; exit 1 ;;' \
  ' nofile) exit 1 ;;' \
  ' fail) printf "failed step=x\n" >"$LAND_RESULT_FILE"; exit 1 ;;' \
  ' zero) exit 0 ;;' \
  ' badsha) printf "landed sha=nothex\n" >"$LAND_RESULT_FILE"; exit 0 ;;' \
  ' slowterm) trap "sleep 2; : >\"\$T/land.done\"; exit 143" TERM; : >"$T/fake.started"; while :; do sleep 0.2; done ;;' \
  ' pushing-skip) printf "pushing head=%s\nskipped lane=k2\n" "$(cat "$T/fake.head")" >"$LAND_RESULT_FILE"; exit 137 ;;' \
  ' refused-lock) printf "refused reason=lock\n" >"$LAND_RESULT_FILE"; exit 1 ;;' \
  ' pushing-landed) printf "pushing head=%s\n" "$(cat "$T/fake.head")" >"$LAND_RESULT_FILE"; exit 137 ;;' \
  ' pushing-lost) printf "pushing head=%s\n" "$(cat "$T/fake.lost")" >"$LAND_RESULT_FILE"; exit 137 ;;' \
  'esac' >"$FK/land"
# a commit on main that "landed" while the fake land died, and one that never reached main
git fetch -q origin main; git checkout -q --detach origin/main; echo pushed >fake-pushed.txt; G add -A; G commit -q -m "lane fakepushed"
git push -q origin HEAD:main; git rev-parse HEAD >"$T/fake.head"
git checkout -q -B fakelost "$(git rev-parse HEAD~1)" 2>/dev/null; echo lost >fake-lost.txt; G add -A; G commit -q -m "lane fakelost"; git rev-parse HEAD >"$T/fake.lost"; git checkout -q --detach origin/main
fake() { rm -f "$T/fakeland.calls"; rm -rf "$T/logs"; out=$(cd "$W" && PATH="$T/bin:$PATH" FAKE_MODE=$1 LAND_QUEUE_LOGS=$T/logs bash "$FK/land-queue" --local --tree "$W" "${@:2}" 2>&1); rc=$?; }
nfake() { wc -l <"$T/fakeland.calls" | tr -d ' '; }
fake ghost k1 k2 k3
expect "a lane not in the set: stops, exit 1" 1 $rc
expect "one attempt, no loop" 1 "$(nfake)"
has "says why" "which is not in this attempt" "$out"
fake garbage k1 k2
expect "garbage result: the queue stops, exit 1" 1 $rc
has "reported as a stop, not as failed lanes" "without a usable result" "$out"
hasnt "no lane is blamed for it" "set aside:" "$out"
expect "one attempt, no bisecting" 1 "$(nfake)"
fake nofile k1 k2
expect "a missing result file with a non-zero exit stops too" 1 "$(nfake)"
has "named" "without a usable result" "$out"
fake refused-lock k1 k2 k3
expect "refused: exit 1" 1 $rc
expect "refused: one attempt" 1 "$(nfake)"
has "refused: its reason" "refused (reason=lock)" "$out"
has "refused: lanes reported not attempted" "not attempted: k1 k2 k3" "$out"
hasnt "refused: no lane set aside" "set aside:" "$out"
fake pushing-landed k1
expect "land died after pushing, and origin/main holds the sha: landed, exit 0" 0 $rc
has "reported pushed" "pushed" "$out"
fake pushing-lost k1
expect "land died after writing pushing, but the sha is not on main: stop, exit 1" 1 $rc
has "reported" "not on origin/main" "$out"
fake zero k1
expect "exit 0 without a landed result: exit 1" 1 $rc
has "reported" "exited 0 without a landed result" "$out"
fake badsha k1
has "a malformed sha is not a landing" "exited 0 without a landed result" "$out"
LAND_QUEUE_MAX_ATTEMPTS=2 fake fail k1 k2 k3 k4
expect "the attempt cap stops it: exit 1" 1 $rc
expect "exactly the cap in attempts" 2 "$(nfake)"
has "cap reported" "attempt cap (2)" "$out"
fake fail k1 k2 k3 k4
expect "uncapped, four failing lanes need 7 attempts" 7 "$(nfake)"

echo "main-moved retry"
lane mm1 x/mm1.txt 1; lane mm2 x/mm2.txt 2
rm -f "$T/mover.count"
printf '%s\n' '#!/usr/bin/env bash' 'set -e' \
  'limit=$1; file=$2; cnt=$T/mover.count; n=$(cat "$cnt" 2>/dev/null || echo 0)' \
  '[ "$n" -lt "$limit" ] || exit 0' 'echo $((n+1)) >"$cnt"' \
  'rm -rf "$T/mv2"; git clone -q "$O" "$T/mv2"; cd "$T/mv2"' \
  'mkdir -p "$(dirname "$file")"; echo "// $RANDOM" >>"$file"' \
  'git add -A; git -c user.name=t -c user.email=t@example.com commit -q -m "mover $n"' \
  'git push -q origin HEAD:main' >"$T/mover.sh"
LAND_HOOK_AFTER_VERIFY="bash $T/mover.sh 2 docs/m.md" queue mm1 mm2
expect "exit 0" 0 $rc
has "land reported main-moved and the queue retried" "main moved under the landing; retrying" "$out"
expect "two attempts" 2 "$(nattempts)"
expect "both landed, in order" 1 "$(mainlog | grep -c 'mm1 mm2 ')"

echo "user hook keeps its exit status and is not read as red CI"
lane uh1 x/uh1.txt 1
LAND_HOOK_AFTER_VERIFY='exit 6' queue uh1
hasnt "not reported as red CI" "is red" "$out"
has "reported by its exit status" "scripts/land exited 6 without a usable result" "$out"
lane uh2 x/uh2.txt 2
LAND_HOOK_AFTER_VERIFY='exit 7' queue uh2
has "exit 7 kept" "scripts/land exited 7" "$out"

echo "TMPDIR with a space"
mkdir -p "$T/tmp dir"
lane sp1 x/sp1.txt 1; lane sp2 sp2.marker 2; lane sp3 x/sp3.txt 3
TMPDIR="$T/tmp dir" queue sp1 sp2 sp3
expect "exit 3" 3 $rc
has "a later push ran its red check through the quoted hook" "set aside: sp2" "$out"
expect "sp1 and sp3 landed, sp2 did not" "1 0" "$(mainlog | grep -c 'sp1 sp3 ') $(mainlog | grep -c 'sp2')"
lane sp4 x/sp4.txt 4
out=$(cd "$W" && TMPDIR="$T/tmp dir" PATH="$T/bin:$PATH" LAND_QUEUE_GREEN="bash $T/green.sh" LAND_QUEUE_LOGS=$T/slogsp bash "$QUEUE" --local --serial sp4 2>&1); rc=$?
expect "serial with a spaced TMPDIR: exit 0" 0 $rc
expect "serial with a spaced TMPDIR: sp4 landed" 1 "$(mainlog | grep -c 'sp4 ')"
expect "the queue's stage copy under the spaced TMPDIR was removed" 0 "$(ls "$T/tmp dir" | grep -c '^land-queue\.')"

echo "bad lane"
lane bl1 x/bl1.txt 1
queue nosuchlane bl1
expect "exit 3" 3 $rc
has "named, does not resolve" "set aside: nosuchlane: does not resolve to a commit" "$out"

echo "empty train: every lane already on main"
before=$(git --git-dir="$O" rev-parse main)
queue g1 g2
expect "exit 0" 0 $rc
expect "nothing pushed" "$before" "$(git --git-dir="$O" rev-parse main)"
has "reported" "already on main: g1" "$out"
has "no push, no CI wait" "nothing was pushed" "$out"

echo "split bound"
for i in 1 2 3 4 5 6 7 8; do lane sb$i sb$i.marker $i; done
queue sb1 sb2 sb3 sb4 sb5 sb6 sb7 sb8
expect "exit 1" 1 $rc
expect "all failing: 15 attempts (2n-1)" 15 "$(nattempts)"
[ "$(nattempts)" -le $((2*8+3+2)) ] && ok "within 2n+ceil(log2 n)+2" || bad "over the cap"

echo "own tree: cleaned between attempts, ignored files kept, kept on request"
lane dt1 x/dt1.txt 1; lane dt2 dirty-dt2.marker 2
queue --keep-tree dt1 dt2
expect "exit 3" 3 $rc
has "the reset is said, naming the dirty paths" "M a.go" "$out"
has "and the reset command" "git reset --hard and git clean -fd in" "$out"
case "$(mainlog)" in *"dt1 "*) ok "dt1 landed after the reset" ;; *) bad "dt1 landed" "$out" ;; esac
has "dt2 named" "set aside: dt2: failed step: gates" "$out"
kept=$(printf '%s\n' "$out" | sed -n 's/^land-queue: kept tree: //p' | head -1)
[ -d "$kept" ] && ok "--keep-tree kept the tree after a failed attempt and printed it" || bad "kept tree" "$out"
expect "own tree: the untracked file was removed" "no" "$([ -e "$kept/junk-dt2.txt" ] && echo yes || echo no)"
expect "own tree: the ignored file survived" "yes" "$([ -e "$kept/ignored.cache" ] && echo yes || echo no)"
expect "own tree: the tracked file is clean" "" "$(git -C "$kept" status --porcelain)"
bash "$T/rmtree.sh" "$kept"; expect "no own trees left" 0 "$(nowned)"

echo "own tree: removed on success, on failure"
lane ot1 x/ot1.txt 1
queue ot1; expect "success: exit 0" 0 $rc; expect "success: own tree removed" 0 "$(nowned)"
lane ot2 ot2.marker 2
queue ot2; expect "failure: exit 1" 1 $rc; expect "failure: own tree removed" 0 "$(nowned)"
lane ot3 ot3.marker 3
queue --keep-tree ot3; kept=$(printf '%s\n' "$out" | sed -n 's/^land-queue: kept tree: //p' | head -1)
[ -d "$kept" ] && ok "kept after a failed attempt with --keep-tree" || bad "kept" "$out"
bash "$T/rmtree.sh" "$kept"
lane ot4 x/ot4.txt 4
queue --keep-tree ot4; expect "--keep-tree on a success still removes it" 0 "$(nowned)"

echo "own tree: removed on a signal, and nothing is pushed afterwards"
lane sg1 x/sg1.txt 1
before=$(git --git-dir="$O" rev-parse main)
touch "$T/gate.sleep"; rm -f "$T/gate.started"
( cd "$W" && PATH="$T/bin:$PATH" LAND_QUEUE_LOGS=$T/logs-sig exec bash "$QUEUE" --local sg1 ) >"$T/sig.out" 2>&1 &
qpid=$!
n=0; until [ -e "$T/gate.started" ] || [ "$n" -ge 100 ]; do sleep 0.3; n=$((n+1)); done
expect "the gate is running in the queue's own tree" 1 "$(nowned)"
kill -TERM "$qpid"; wait "$qpid" 2>/dev/null
rm -f "$T/gate.sleep"; kill "$(cat "$T/gate.pid")" 2>/dev/null
sleep 1
expect "signal: own tree removed" 0 "$(nowned)"
expect "signal: nothing pushed" "$before" "$(git --git-dir="$O" rev-parse main)"

echo "explicit --tree is never reset or cleaned"
lane uw1 x/uw1.txt 1; lane uw2 userwork-2.marker 2
queue --tree "$W" uw1 uw2
expect "stops, exit non-zero" 1 $rc
expect "the file made in the tree during the failing attempt survives" "mine" "$(cat "$W/userwork-created.txt" 2>/dev/null)"
has "the message names the dirty path" "userwork-created.txt" "$out"
has "and says it is not cleaned" "is not clean" "$out"
hasnt "no reset" "git reset --hard" "$out"
rm -f "$W/userwork-created.txt"
echo dirty >"$W/stray.txt"
queue --tree "$W" uw1
rm -f "$W/stray.txt"
expect "a tree dirty at the start is refused: exit 1" 1 $rc
has "refused with a message" "is not clean" "$out"

echo "land refuses at a guard: the queue stops, lanes not attempted"
gd=$(cd "$W" && cd "$(git rev-parse --git-dir)" && pwd -P)
mkdir "$gd/land.lock"; echo $$ >"$gd/land.lock/pid"
lane rf1 x/rf1.txt 1; lane rf2 x/rf2.txt 2
queue --tree "$W" rf1 rf2
rm -rf "$gd/land.lock"
expect "lock: exit 1" 1 $rc
expect "lock: one attempt" 1 "$(nattempts)"
has "lock: refused, with the reason" "refused (reason=lock)" "$out"
has "lock: not attempted" "not attempted: rf1 rf2" "$out"
hasnt "lock: no lane set aside" "set aside:" "$out"
LAND_PRIMARY=$W queue --tree "$W" rf1
expect "primary: exit 1" 1 $rc
has "primary: refused" "refused (reason=primary)" "$out"

echo "primary checkout refused before any git command"
head0=$(git -C "$M" rev-parse HEAD); br0=$(git -C "$M" symbolic-ref -q HEAD || echo detached)
out=$(cd "$M" && PATH="$T/bin:$PATH" LAND_QUEUE_LOGS=$T/logs-pr bash "$QUEUE" --local --tree "$M" rf1 2>&1); rc=$?
expect "exit 2" 2 $rc
has "message" "primary checkout" "$out"
expect "the primary's HEAD is untouched" "$head0 $br0" "$(git -C "$M" rev-parse HEAD) $(git -C "$M" symbolic-ref -q HEAD || echo detached)"

echo "lane names"
queue 'bad name' ; expect "a name with a space: exit 2" 2 $rc
queue 'a;b'; expect "a name with a metacharacter: exit 2" 2 $rc
queue rf1 rf1; expect "a lane named twice: exit 2" 2 $rc
has "duplicate named" "named twice" "$out"

echo "CI covers every code push"
lane cv1LATERED x/cv1.txt 1; lane cv2NORUN docs/cv2.md 2; lane cv3 cv3.marker 3
queue cv1LATERED cv2NORUN cv3
expect "a red code push followed by a docs push: exit 4" 4 $rc
case "$(mainlog)" in *cv1LATERED*cv2NORUN*) ok "both pushes happened" ;; *) bad "pushes" "$out" ;; esac
has "the red CI of the code push is read" "RED (completed failure)" "$out"
expect "two pushes" 2 "$(nland)"

echo "a cancelled run is superseded, not red"
lane xc1CANCELLED x/xc1.txt 1; lane xc2 xc2.marker 2; lane xc3 x/xc3.txt 3
queue xc1CANCELLED xc2 xc3
expect "a cancelled earlier push does not stop the queue or turn it red: exit 3 (xc2 aside)" 3 $rc
case "$(mainlog)" in *xc1CANCELLED*xc3*) ok "xc3 was pushed after the cancelled push" ;; *) bad "xc3 pushed" "$out" ;; esac
hasnt "no false red" "is red" "$out"
lane xd1CANCELLED x/xd1.txt 1
LAND_QUEUE_TIMEOUT=2 LAND_QUEUE_POLL=1 queue xd1CANCELLED
expect "a cancelled run with nothing after it: unknown, exit 4" 4 $rc
has "reported by sha" "CI unknown for" "$out"
lane xe1CANCELLED x/xe1.txt 1; lane xe2NORUN docs/xe2.md 2; lane xe3 xe3.marker 3
LAND_QUEUE_TIMEOUT=2 LAND_QUEUE_POLL=1 queue xe1CANCELLED xe2NORUN xe3
expect "a cancelled code push then a docs push: unknown, exit 4" 4 $rc
has "CI unknown for the code push" "CI unknown for" "$out"

echo "a later run on main covers an earlier push"
git fetch -q origin main; git checkout -q --detach origin/main; echo a >cov-a.txt; G add -A; G commit -q -m "cov NORUN"; cova=$(git rev-parse HEAD)
echo b >cov-b.txt; G add -A; G commit -q -m "cov later"; git push -q origin HEAD:main; git checkout -q --detach origin/main
out=$(cd "$W" && PATH="$T/bin:$PATH" LAND_QUEUE_CI_APPEAR=1 LAND_QUEUE_POLL=1 bash "$QUEUE" --wait-green "$cova" --no-staging 2>&1); rc=$?
expect "--wait-green: no run for the sha, a successful run for a descendant covers it" 0 $rc

echo "empty train reports the CI state of main without waiting"
queue g1 g2
expect "exit 0" 0 $rc
has "says so" "nothing was pushed" "$out"
has "CI state of origin/main's head" "CI on origin/main" "$out"

echo "gh fails"
touch "$T/gh.fail"; lane gf1 x/gf1.txt 1
LAND_QUEUE_TIMEOUT=2 LAND_QUEUE_POLL=1 queue gf1
rm -f "$T/gh.fail"
expect "exit 4: CI unknown" 4 $rc
has "reported unknown" "UNKNOWN" "$out"
hasnt "never concluded docs-only" "no CI run: docs-only push" "$out"
hasnt "never concluded docs-only (wait line)" "a docs-only push, which" "$out"

echo "no CI run for a push that is not docs-only"
touch "$T/noci"; lane nd1 x/nd1.txt 1
LAND_QUEUE_CI_APPEAR=1 LAND_QUEUE_POLL=1 queue nd1
rm -f "$T/noci"
expect "exit 4" 4 $rc
has "unknown, not docs-only" "CI unknown" "$out"

echo "polling: pending, then success"
echo 2 >"$T/gh.pend"; lane po1 x/po1.txt 1
LAND_QUEUE_CI_APPEAR=5 LAND_QUEUE_POLL=1 queue po1
rm -f "$T/gh.pend"
expect "exit 0" 0 $rc
main_sha=$(git --git-dir="$O" rev-parse main)
has "green by wording" "CI on the last push ${main_sha:0:9}: green" "$out"
expect "gh polled three times" 3 "$(grep -c 'run list' "$T/gh.calls")"

echo "a skipped run is not green"
touch "$T/gh.skip"; lane sk1 x/sk1.txt 1
queue sk1
rm -f "$T/gh.skip"
expect "exit 4" 4 $rc
has "named" "RED (completed skipped)" "$out"

echo "wait-staging ancestor"
touch "$T/staging.child"; lane wa1 x/wa1.txt 1
queue --wait-staging wa1
expect "exit 0" 0 $rc
has "staging serves a later commit that holds it" "which holds" "$out"

echo "serial failure"
lane sv1 x/sv1.txt 1; lane sv2 sv2.marker 2
out=$(cd "$W" && PATH="$T/bin:$PATH" LAND_QUEUE_GREEN="bash $T/green.sh" LAND_QUEUE_LOGS=$T/slogs2 bash "$QUEUE" --local --serial sv1 sv2 2>&1); rc=$?
expect "serial: a failed lane exits 1" 1 $rc
has "serial: named" "did not land: sv2" "$out"
case "$(mainlog)" in *"sv1 "*) ok "serial: sv1 landed" ;; *) bad "serial: sv1" "$out" ;; esac
lane sv3 sv3.marker 3
out=$(cd "$W" && PATH="$T/bin:$PATH" LAND_QUEUE_GREEN="bash $T/green.sh" LAND_QUEUE_LOGS=$T/slogs3 bash "$QUEUE" --local --serial sv3 2>&1); rc=$?
expect "serial: nothing landed: exit 1, no unbound array" 1 $rc
has "serial: landed none" "landed: none" "$out"

echo "own tree is verified before anything can reset or remove it"
for kind in none plain branch; do
  case $kind in
    none)   printf '%s\n' '#!/usr/bin/env bash' 'echo "$M"' >"$T/mk-$kind.sh" ;;
    plain)  printf '%s\n' '#!/usr/bin/env bash' 'mkdir -p "$T/own/$1"; echo "$T/own/$1"' >"$T/mk-$kind.sh" ;;
    branch) printf '%s\n' '#!/usr/bin/env bash' 'git -C "$M" fetch -q origin main; git -C "$M" worktree add -q -b "other-$1" "$T/own/$1" origin/main && echo "$T/own/$1"' >"$T/mk-$kind.sh" ;;
  esac
  lane vt-$kind x/vt-$kind.txt 1
  # work that must survive: a tracked edit and an untracked file in the primary and in another worktree
  echo "// mine" >>"$M/go.mod"; echo keep >"$M/primary-untracked.txt"
  echo "// mine" >>"$W/a.go";   echo keep >"$W/other-untracked.txt"
  LAND_QUEUE_MKTREE="bash $T/mk-$kind.sh" queue --keep-tree vt-$kind
  [ "$rc" -ne 0 ] && ok "$kind: exit non-zero ($rc)" || bad "$kind: exit non-zero" "$out"
  has "$kind: says what the check found" "ownership check failed" "$out"
  expect "$kind: primary tracked edit survives" "// mine" "$(tail -n 1 "$M/go.mod")"
  expect "$kind: primary untracked file survives" "keep" "$(cat "$M/primary-untracked.txt" 2>/dev/null)"
  expect "$kind: other worktree tracked edit survives" "// mine" "$(tail -n 1 "$W/a.go")"
  expect "$kind: other worktree untracked file survives" "keep" "$(cat "$W/other-untracked.txt" 2>/dev/null)"
  expect "$kind: nothing landed" 0 "$(nland)"
  git -C "$M" checkout -q go.mod; rm -f "$M/primary-untracked.txt"; git -C "$W" checkout -q a.go; rm -f "$W/other-untracked.txt"
  rm -rf "$T/own/"*; git -C "$M" worktree prune; git -C "$M" branch -D "other-land-queue"* >/dev/null 2>&1
done

echo "a signal while the tree is made: the lane is still removed"
printf '%s\n' '#!/usr/bin/env bash' ': >"$T/mk.started"' 'git -C "$M" fetch -q origin main' \
  'git -C "$M" worktree add -q -b "$1" "$T/own/$1" origin/main' 'sleep 3' 'echo "$T/own/$1"' >"$T/mk-slow.sh"
lane sm1 x/sm1.txt 1; rm -f "$T/mk.started"
( cd "$W" && PATH="$T/bin:$PATH" LAND_QUEUE_MKTREE="bash $T/mk-slow.sh" LAND_QUEUE_LOGS=$T/logs-sm exec bash "$QUEUE" --local sm1 ) >"$T/sm.out" 2>&1 &
qpid=$!
n=0; until [ -e "$T/mk.started" ] || [ "$n" -ge 100 ]; do sleep 0.2; n=$((n+1)); done
sleep 0.5; kill -TERM "$qpid"; wait "$qpid" 2>/dev/null
expect "the lane made just before the signal was removed" 0 "$(nowned)"
expect "and its branch" 0 "$(git -C "$M" branch --list 'land-queue-*' | wc -l | tr -d ' ')"

echo "cleanup waits for land before removing anything"
rm -f "$T/land.done" "$T/fake.started"
( cd "$W" && PATH="$T/bin:$PATH" FAKE_MODE=slowterm LAND_QUEUE_LOGS=$T/logs-wt exec bash "$FK/land-queue" --local --tree "$W" wt1 ) >"$T/wt.out" 2>&1 &
qpid=$!
n=0; until [ -e "$T/fake.started" ] || [ "$n" -ge 100 ]; do sleep 0.2; n=$((n+1)); done
kill -TERM "$qpid"; wait "$qpid" 2>/dev/null
expect "land had finished its own shutdown when the queue exited" "yes" "$([ -e "$T/land.done" ] && echo yes || echo no)"

echo "pushing carries the skipped lanes, and the queue reads them"
fake pushing-skip k1 k2
expect "exit 0" 0 $rc
has "k2 was already on main, not pushed" "already on main: k2" "$out"
lane ps_new x/ps_new.txt 1
mkdir -p "$M/.git/hooks"; printf '%s\n' '#!/usr/bin/env bash' 'cp "$T/res.file" "$T/res.prepush" 2>/dev/null; exit 0' >"$M/.git/hooks/pre-push"; chmod +x "$M/.git/hooks/pre-push"
rm -f "$T/res.prepush" "$T/res.file"
out=$(cd "$W" && LAND_RESULT_FILE=$T/res.file "$here/land" --local g1 ps_new 2>&1); rc=$?
rm -f "$M/.git/hooks/pre-push"
expect "land with an already-merged lane and a new one: exit 0" 0 $rc
expect "its result just before the push is pushing" "pushing" "$(head -n 1 "$T/res.prepush" | cut -d' ' -f1)"
expect "and names the skipped lane" "skipped lane=g1" "$(sed -n '2,$p' "$T/res.prepush" | head -n 1)"
expect "after the push it is landed" "landed" "$(head -n 1 "$T/res.file" | cut -d' ' -f1)"

echo "a lane that already has the name is refused and left alone"
git -C "$M" fetch -q origin main
git -C "$M" worktree add -q -b land-queue-collide "$T/own/land-queue-collide" origin/main
echo precious >"$T/own/land-queue-collide/marker.txt"
lane co1 x/co1.txt 1
LAND_QUEUE_OWN_NAME=land-queue-collide queue co1
[ "$rc" -ne 0 ] && ok "exit non-zero ($rc)" || bad "exit non-zero" "$out"
has "refused, saying why" "already exists" "$out"
expect "the existing worktree survives" "precious" "$(cat "$T/own/land-queue-collide/marker.txt" 2>/dev/null)"
expect "its branch survives" 1 "$(git -C "$M" branch --list land-queue-collide | wc -l | tr -d ' ')"
expect "nothing landed" 0 "$(mainlog | grep -c co1)"
git -C "$M" worktree remove --force "$T/own/land-queue-collide"; git -C "$M" branch -D land-queue-collide >/dev/null 2>&1
git -C "$M" branch -f land-queue-bonly origin/main
LAND_QUEUE_OWN_NAME=land-queue-bonly queue co1
[ "$rc" -ne 0 ] && ok "a branch of that name alone also refuses ($rc)" || bad "branch refusal" "$out"
expect "and the branch survives" 1 "$(git -C "$M" branch --list land-queue-bonly | wc -l | tr -d ' ')"
git -C "$M" branch -D land-queue-bonly >/dev/null 2>&1
LAND_QUEUE_OWN_NAME=not-ours queue co1
has "a name not matching land-queue-* is ignored" "own tree" "$out"

echo "a stage directory that is not ours is refused and survives"
mkdir -p "$T/fakestage"; echo precious >"$T/fakestage/file.txt"
out=$(LAND_QUEUE_STAGE="$T/fakestage" bash "$QUEUE" --help 2>&1); rc=$?
expect "queue: exit 2" 2 $rc
expect "queue: the directory and its file survive" "precious" "$(cat "$T/fakestage/file.txt" 2>/dev/null)"
out=$(LAND_QUEUE_STAGE="$T/fakestage" bash "$QUEUE" nosuchlane 2>&1); rc=$?
expect "queue: an invalid run too: exit 2" 2 $rc
expect "queue: file still there" "precious" "$(cat "$T/fakestage/file.txt" 2>/dev/null)"
mkdir -p "$T/fakestage2"; echo precious >"$T/fakestage2/file.txt"
out=$(LAND_STAGE_DIR="$T/fakestage2" bash "$here/land" --help 2>&1); rc=$?
expect "land: exit 2" 2 $rc
expect "land: the directory and its file survive" "precious" "$(cat "$T/fakestage2/file.txt" 2>/dev/null)"
out=$(LAND_STAGE_DIR="$T/fakestage2" bash "$here/land" 2>&1); rc=$?
expect "land: an invalid run too: exit 2" 2 $rc
expect "land: file still there" "precious" "$(cat "$T/fakestage2/file.txt" 2>/dev/null)"
out=$(bash "$QUEUE" --help 2>&1); rc=$?
expect "a genuine --help still works" 0 $rc

echo "queue timing line"
expect "queue lines written, 8 columns" "yes" "$(awk -F'\t' '$1=="queue" && NF==8{n++} END{print (n>=8)?"yes":"no"}' "$T/timings.tsv")"
expect "no queue line has other than 8 columns" 0 "$(awk -F'\t' '$1=="queue" && NF!=8{n++} END{print n+0}' "$T/timings.tsv")"
expect "first queue line counts" "given:3|landed:3|aside:0|attempts:1|pushes:1|ci_wait:" "$(awk -F'\t' '$1=="queue"{print substr($8,1,index($8,"ci_wait:")+7); exit}' "$T/timings.tsv")"

echo; echo "land-queue selftest: $pass passed, $failn failed"
[ "$failn" = 0 ]
