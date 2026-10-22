#!/usr/bin/env bash
# Behavioural test for scripts/dolt-history-audit.sh (the daily Dolt history
# audit). No Dolt server: a stub dolt answers the dolt_log count.
#
# Failure modes, written before the code (docs/problems/platform-dolt-oldgen-
# rate-and-squash-window-2026-10-22.md, review round 1 R4):
#  1. With Store A's snapshots off, growth moves to newgen and the journal;
#     an audit that checks only oldgen never fails: newgen over its line fails.
#  2. Auto GC stops (a server version that no longer runs it on working-set
#     updates): the journal grows past its line and fails.
#  3. The audit line does not say newgen and journal sizes: it must.
#  4. The old checks regress: oldgen over its line and a commit delta over
#     its line still fail; a store under every line passes.
#  5. A missing store is a failure: never; it is skipped.
# Run: bash scripts/test-dolt-history-audit.sh
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
AUDIT="$HERE/dolt-history-audit.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0
JOURNAL=vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv

mkstore() { # dir oldgen_kib journal_kib other_newgen_kib
  mkdir -p "$1/home" "$1/db/.dolt/noms/oldgen"
  head -c $(($2 * 1024)) /dev/urandom > "$1/db/.dolt/noms/oldgen/a.darc"
  head -c $(($3 * 1024)) /dev/urandom > "$1/db/.dolt/noms/$JOURNAL"
  head -c $(($4 * 1024)) /dev/urandom > "$1/db/.dolt/noms/b.table"
}
run() { # name commits [env...]; the remaining args are env assignments
  local name="$1" commits="$2"; shift 2
  mkdir -p "$WORK/bin"
  printf '#!/bin/sh\necho "+----------+"\necho "| COUNT(*) |"\necho "| %s |"\n' "$commits" > "$WORK/bin/dolt"
  chmod +x "$WORK/bin/dolt"
  OUT="$(env PATH="$WORK/bin:$PATH" "$@" bash "$AUDIT" "platform:$WORK/$name/db:$WORK/$name/home" "corpus:$WORK/absent/db:$WORK/absent/home" 2>&1)"; RC=$?
}
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC out=$OUT"; fails=$((fails + 1)); fi; }

mkstore "$WORK/healthy" 64 32 32
run healthy 10 GO_CHOIR_DOLT_MAX_NEWGEN_KIB=1024 GO_CHOIR_DOLT_MAX_JOURNAL_KIB=1024
check "a store under every line passes" "$([ "$RC" = 0 ]; echo $?)"
check "the audit line names newgen and the journal" "$(printf '%s' "$OUT" | grep -q 'platform-dolt history audit: dolt_log commits=10 oldgen=[0-9]*KiB noms=[0-9]*KiB newgen=[0-9]*KiB journal=[0-9]*KiB'; echo $?)"
check "a missing store is skipped" "$(printf '%s' "$OUT" | grep -q 'corpus-dolt history audit: skipped'; echo $?)"

mkstore "$WORK/newgen" 64 32 400
run newgen 10 GO_CHOIR_DOLT_MAX_NEWGEN_KIB=256 GO_CHOIR_DOLT_MAX_JOURNAL_KIB=1024
check "newgen over its line fails" "$([ "$RC" = 1 ] && printf '%s' "$OUT" | grep -q 'newgen .*KiB > 256KiB'; echo $?)"

mkstore "$WORK/journal" 64 400 0
run journal 10 GO_CHOIR_DOLT_MAX_NEWGEN_KIB=4096 GO_CHOIR_DOLT_MAX_JOURNAL_KIB=256
check "the journal over its line fails (auto GC not running)" "$([ "$RC" = 1 ] && printf '%s' "$OUT" | grep -q 'journal .*KiB > 256KiB'; echo $?)"

mkstore "$WORK/oldgen" 400 32 0
run oldgen 10 GO_CHOIR_DOLT_MAX_OLDGEN_KIB=256
check "oldgen over its line still fails" "$([ "$RC" = 1 ] && printf '%s' "$OUT" | grep -q 'oldgen .*KiB > 256KiB'; echo $?)"

mkstore "$WORK/delta" 64 32 0
echo 5 > "$WORK/delta/home/history-audit-last-count"
run delta 50 GO_CHOIR_DOLT_MAX_COMMIT_DELTA=20
check "a commit delta over its line still fails" "$([ "$RC" = 1 ] && printf '%s' "$OUT" | grep -q 'grew 45 commits'; echo $?)"

[ "$fails" = 0 ] && echo "all history audit checks passed" || { echo "$fails failed"; exit 1; }
