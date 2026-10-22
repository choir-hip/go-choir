#!/usr/bin/env bash
# status.json survives concurrent writers. The daily backup and the weekly
# verify are separate units and can overlap; both write status.json at
# start, end and exit (problem record offhost-backup-reads-terabytes-and-
# needs-swap-2026-10-22, review round 3, finding 1).
# Failure modes (written before the code):
#
#  1. Two writers read the same file, merge, and the second rename drops
#     the first one's keys (a lost running mark): never; every key written
#     by either writer is present at the end.
#  2. Two writers share one temp file and the installed status.json is
#     invalid JSON (every later merge then fails): never; it stays valid.
#  3. A temp file is left beside status.json: never.
#  4. A writer that waits on a slow holder gives up and writes without the
#     lock, so the holder's rename drops its key (the old 60 s fail-open):
#     never; it waits as long as the holder holds (round 4, finding 1).
#  5. A temp left by a writer killed mid-merge stays forever: never; a
#     writer under the lock removes status.json.?????? files over an hour
#     old (round 4, finding 4).
#
# Needs flock(1) (util-linux, on the units' PATH). Without it the test
# skips and says so: the Linux run is the evidence.
#
# The test runs the script's own status-file section (status_merge and its
# helpers), extracted by its section markers, so it tests the shipped code.
#
# Run: bash scripts/off-host-backup-status-lock-test.sh
set -u

command -v flock >/dev/null 2>&1 || { echo "SKIP: no flock on this host"; exit 0; }

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0
N=60

STATE="$WORK/state"; STATUS="$STATE/status.json"
log() { :; }; err() { printf '%s\n' "$*" >&2; }
eval "$(sed -n '/^# ---- status file/,/^# ---- attempt marks/p' "$SCRIPT")"

writer() { # $1 key prefix
  local i
  for i in $(seq 1 "$N"); do status_merge <<<"{\"$1$i\":$i}"; done
}
writer a & writer b & wait

check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; fails=$((fails + 1)); fi; }
check "status.json is valid JSON after concurrent writers" "$(jq -e . "$STATUS" >/dev/null 2>&1; echo $?)"
got="$(jq -r 'keys | length' "$STATUS" 2>/dev/null || echo 0)"
check "every key from both writers survives ($got of $((2 * N)))" "$([ "$got" = $((2 * N)) ]; echo $?)"
check "no temp file is left beside status.json" "$([ -z "$(ls "$STATE" | grep -v -x -e status.json -e status.json.lock)" ]; echo $?)"

# 4: a holder that keeps the lock for 65 s, longer than the old 60 s wait,
# and does its own read-merge-write with what it read before sleeping.
rm -rf "$STATE"; mkdir -p "$STATE"; echo '{}' > "$STATUS"
( flock 9; cur="$(cat "$STATUS")"; sleep 65
  printf '%s' "$cur" | jq -c '. + {holder: 1}' > "$STATUS.h" && mv "$STATUS.h" "$STATUS" ) 9>>"$STATUS.lock" &
sleep 1
status_merge <<<'{"waiter":1}'
wait
check "a writer waits out a slow holder: both keys survive" "$(jq -e '.holder == 1 and .waiter == 1' "$STATUS" >/dev/null 2>&1; echo $?)"

# 5: stale temps are reaped under the lock; a fresh one is left alone.
printf x > "$STATUS.AbC123"; touch -d '2 hours ago' "$STATUS.AbC123"
printf x > "$STATUS.XyZ789"
status_merge <<<'{"reap":1}'
check "a temp over an hour old is removed" "$([ ! -e "$STATUS.AbC123" ]; echo $?)"
check "a recent temp (a live writer's) is kept" "$([ -e "$STATUS.XyZ789" ]; echo $?)"

[ "$fails" = 0 ] && echo "all status lock checks passed" || { echo "$fails failed"; exit 1; }
