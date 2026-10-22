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
#
# The test runs the script's own status-file section (status_merge and its
# helpers), extracted by its section markers, so it tests the shipped code.
#
# Run: bash scripts/off-host-backup-status-lock-test.sh
set -u

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

[ "$fails" = 0 ] && echo "all status lock checks passed" || { echo "$fails failed"; exit 1; }
