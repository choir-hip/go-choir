#!/usr/bin/env bash
# status.json tells the truth about a run that never finished. No network,
# no secrets, no real btrfs or systemd: stub tools.
#
# Design: docs/problems/offhost-backup-reads-terabytes-and-needs-swap-
# 2026-10-22.md (review round 2, findings 1 and 2). With no swap, a cgroup
# OOM kill is a designed failure mode, so it must read as one.
# Failure modes (written before the code):
#
#  1. restic is killed (a cgroup OOM kill sends SIGKILL: exit 137) and
#     status.json still says the last attempt was ok: never; it says
#     failed, and the cause names the signal.
#  2. The unit is stopped while restic runs (systemd's OOMPolicy=stop sends
#     SIGTERM to the whole cgroup) and status.json keeps the previous
#     night's ok: never; the exit trap writes failed, interrupted.
#  3. bash itself is SIGKILLed (no trap can run) and the previous success
#     stands: never; the attempt is marked running at its start, and the
#     freshness check reads a running attempt whose process is gone as
#     not fresh.
#  4. A run in progress makes the freshness check fail: never; a running
#     attempt whose process is alive is reported, not flagged.
#  5. The weekly verify, killed the same ways, leaves the previous verify's
#     ok standing: never; the same marks apply, and the freshness check
#     flags a failed or interrupted verify at once instead of after 10 days.
#  6. A clean run and a clean verify read as fresh.
#
# Run: bash scripts/off-host-backup-attempt-status-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
WORK="$(mktemp -d)"
LIVE=""
trap '[ -n "$LIVE" ] && kill "$LIVE" 2>/dev/null; rm -rf "$WORK"' EXIT
fails=0

setup() { # name, restic mode for backup and check: ok | kill9 | term | killbash
  R="$WORK/$1"; local root="$R/root"
  mkdir -p "$R/bin" "$root/platform-artifacts" "$root/secrets" "$root/vm-state" \
    "$R/sec"
  for t in ssh sqlite3; do printf '#!/bin/sh\nexit 0\n' > "$R/bin/$t"; done
  # pg_dump writes its -f file; pg_restore --list prints two entries.
  printf '#!/bin/sh\nwhile [ "$#" -gt 0 ]; do [ "$1" = -f ] && { echo dump > "$2"; exit 0; }; shift; done\nexit 1\n' > "$R/bin/pg_dump"
  printf '#!/bin/sh\necho "; header"\necho "1; 0 0 TABLE public t choir"\necho "2; 0 0 TABLE public u choir"\n' > "$R/bin/pg_restore"
  printf pw > "$R/sec/pg-app-password"
  printf '#!/bin/sh\ncat >/dev/null\necho "[]"\n' > "$R/bin/curl"
  printf '#!/bin/sh\n[ "$1" = show ] && echo 3221225472\nexit 0\n' > "$R/bin/systemctl"
  cat > "$R/bin/restic" <<EOF
#!/usr/bin/env bash
case " \$* " in *" backup "*|*" check "*) ;; *) exit 0 ;; esac
case "$2" in
  kill9) kill -KILL \$\$ ;;
  term) kill -TERM \$PPID; sleep 1; exit 143 ;;
  killbash) kill -KILL \$PPID; exit 137 ;;
esac
case " \$* " in *" backup "*) echo '{"message_type":"summary","snapshot_id":"s1","data_added":5}' ;; esac
exit 0
EOF
  chmod +x "$R/bin/"*
  printf x > "$R/sec/restic-password"; printf x > "$R/sec/rest-password"
  ST="$root/offhost-backup/status.json"
}
seed_ok() { # a previous night's success and verify
  local t; t="$(date -u +%s)"
  mkdir -p "$(dirname "$ST")"
  printf '{"last_attempt_result":"ok","last_success_epoch":%s,"first_success_epoch":%s,"last_verify_result":"ok","last_verify_epoch":%s}\n' \
    "$((t - 3600))" "$((t - 86400 * 20))" "$((t - 86400))" > "$ST"
}
run() { # subcommand
  OUT="$(PATH="$R/bin:$PATH" OFFHOST_DATA_ROOT="$R/root" OFFHOST_SECRETS_DIR="$R/sec" \
    OFFHOST_PG_PASSWORD_FILE="$R/sec/pg-app-password" OFFHOST_NO_TUNNEL=1 OFFHOST_MIN_FREE_GIB=0 \
    bash "$SCRIPT" "$1" 2>&1)"; RC=$?
}
field() { jq -r ".$1 // \"\"" "$ST"; }
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC status=$(cat "$ST" 2>/dev/null) out=$(printf '%s' "$OUT" | tail -3)"; fails=$((fails + 1)); fi; }

# 1: restic SIGKILLed.
setup kill9 kill9; seed_ok; run backup
check "a killed restic marks the attempt failed" "$([ "$(field last_attempt_result)" = failed ]; echo $?)"
check "the cause names the signal" "$(field last_attempt_cause | grep -q 'signal 9'; echo $?)"
run freshness
check "freshness fails after a killed restic" "$([ "$RC" != 0 ]; echo $?)"

# 2: the script SIGTERMed while restic runs.
setup term term; seed_ok; run backup
check "a stopped unit marks the attempt failed" "$([ "$(field last_attempt_result)" = failed ]; echo $?)"
check "the cause says interrupted" "$(field last_attempt_cause | grep -q 'interrupted'; echo $?)"
run freshness
check "freshness fails after a stopped unit" "$([ "$RC" != 0 ]; echo $?)"

# 3: bash SIGKILLed: running is left, and read as not fresh.
setup killbash killbash; seed_ok; run backup
check "the attempt is marked running at its start" "$([ "$(field last_attempt_result)" = running ]; echo $?)"
run freshness
check "freshness fails on a running attempt whose process is gone" "$([ "$RC" != 0 ] && printf '%s' "$OUT" | grep -q interrupted; echo $?)"

# 4: a run in progress is not flagged.
setup live ok; seed_ok
bash -c 'exec -a off-host-backup.sh bash -c "sleep 30; :"' & LIVE=$!
sleep 0.2
jq -c --argjson p "$LIVE" '. + {last_attempt_result:"running", last_attempt_pid:$p}' "$ST" > "$ST.t" && mv "$ST.t" "$ST"
run freshness
check "a running attempt with a live process is not flagged" "$([ "$RC" = 0 ] && printf '%s' "$OUT" | grep -q 'in progress'; echo $?)"
kill "$LIVE" 2>/dev/null; wait "$LIVE" 2>/dev/null; LIVE=""

# 5: the weekly verify, killed.
setup vkill9 kill9; seed_ok; run verify
check "a killed verify is marked failed" "$([ "$(field last_verify_result)" = failed ] && field last_verify_cause | grep -q 'signal 9'; echo $?)"
run freshness
check "freshness fails at once on a failed verify" "$([ "$RC" != 0 ]; echo $?)"
setup vterm term; seed_ok; run verify
check "a stopped verify is marked failed, interrupted" "$([ "$(field last_verify_result)" = failed ] && field last_verify_cause | grep -q interrupted; echo $?)"
setup vkillbash killbash; seed_ok; run verify; run freshness
check "freshness fails on a verify whose process is gone" "$([ "$RC" != 0 ]; echo $?)"

# 6: clean.
setup clean ok; seed_ok; run backup
check "a clean run is ok" "$([ "$RC" = 0 ] && [ "$(field last_attempt_result)" = ok ] && [ -z "$(field last_attempt_cause)" ]; echo $?)"
run verify
check "a clean verify is ok" "$([ "$(field last_verify_result)" = ok ]; echo $?)"
run freshness
check "clean runs are fresh" "$([ "$RC" = 0 ]; echo $?)"

[ "$fails" = 0 ] && echo "all attempt status checks passed" || { echo "$fails failed"; exit 1; }
