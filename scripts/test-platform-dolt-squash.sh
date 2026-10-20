#!/usr/bin/env bash
# Behavioural test for scripts/platform-dolt-squash.sh. It drives the real
# script with a fake `systemctl` and a fake squash command over a fake data
# directory, so it needs no Dolt, no root and no Node B. The squash command
# itself is tested against real Dolt in internal/projectionbase/squash_test.go.
#
#   bash scripts/test-platform-dolt-squash.sh
#
# Failure modes are FM7, FM8, FM10, FM11 and FM12 of
# docs/problems/store-a-weekly-squash-2026-10-20.md.
set -u
here="$(cd "$(dirname "$0")" && pwd)"
script="$here/platform-dolt-squash.sh"
fail=0
pass=0

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

mkdir -p "$root/bin" "$root/fake"

# Fake systemctl: unit state lives in $FAKE_DIR/state.<unit>; RequiredBy and
# TriggeredBy come from $FAKE_DIR/requiredby / triggeredby.<unit>; every
# call is logged in order.
cat >"$root/bin/systemctl" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_DIR/systemctl.log"
case "$1" in
  is-active)
    s="$(cat "$FAKE_DIR/state.$2" 2>/dev/null || echo inactive)"
    echo "$s"
    [ "$s" = active ]
    ;;
  stop) echo inactive >"$FAKE_DIR/state.$2" ;;
  start)
    if [ -n "${FAKE_START_FAILS:-}" ] && [ "$2" = "$FAKE_START_FAILS" ]; then exit 1; fi
    echo active >"$FAKE_DIR/state.$2"
    ;;
  show)
    # systemctl show -p RequiredBy --value <unit>
    case "$3" in
      RequiredBy) cat "$FAKE_DIR/requiredby" 2>/dev/null ;;
      TriggeredBy) cat "$FAKE_DIR/triggeredby.$5" 2>/dev/null ;;
    esac
    ;;
esac
exit 0
EOF
chmod +x "$root/bin/systemctl"

# Fake squash command. FAKE_SQUASH_MODE: ok | refuse | fail | kill.
# "Squashing" rewrites the store's marker file; a failure leaves it half-done.
cat >"$root/bin/platform-dolt-squash" <<'EOF'
#!/usr/bin/env bash
dir=""; db=""
while [ $# -gt 0 ]; do
  case "$1" in -dir) dir="$2"; shift 2 ;; -database) db="$2"; shift 2 ;; *) shift ;; esac
done
echo "squash $dir $db $(cat "$FAKE_DIR/state.go-choir-platform-dolt.service" 2>/dev/null)" >>"$FAKE_DIR/squash.log"
case "${FAKE_SQUASH_MODE:-ok}" in
  ok)
    echo SQUASHED >"$dir/$db/.dolt/marker"
    echo '{"squashed":true,"root_before":"r","root_after":"r"}'
    ;;
  refuse)
    echo '{"error":"refused: 1 tags","refused_before_mutation":true}'
    exit 3
    ;;
  fail)
    echo TORN >"$dir/$db/.dolt/marker"
    echo '{"error":"root changed"}'
    exit 1
    ;;
  kill)
    echo TORN >"$dir/$db/.dolt/marker"
    kill -9 "$PPID"
    sleep 1
    exit 137
    ;;
esac
EOF
chmod +x "$root/bin/platform-dolt-squash"

DOLT=go-choir-platform-dolt.service

new_case() { # name -> sets case dir, data, backups
  case_dir="$root/$1"
  rm -rf "$case_dir"
  mkdir -p "$case_dir/fake" "$case_dir/data/platform/.dolt" "$case_dir/backups"
  echo ORIGINAL >"$case_dir/data/platform/.dolt/marker"
  echo data >"$case_dir/data/platform/.dolt/chunk"
  : >"$case_dir/fake/systemctl.log"
  for u in "$DOLT" go-choir-corpusd.service go-choir-checkpointd.timer go-choir-front-importance.timer; do
    echo active >"$case_dir/fake/state.$u"
  done
  printf 'go-choir-corpusd.service go-choir-checkpointd.service go-choir-platform-dolt-history-audit.service\n' >"$case_dir/fake/requiredby"
  echo go-choir-checkpointd.timer >"$case_dir/fake/triggeredby.go-choir-checkpointd.service"
  echo go-choir-platform-dolt-history-audit.timer >"$case_dir/fake/triggeredby.go-choir-platform-dolt-history-audit.service"
  echo active >"$case_dir/fake/state.go-choir-checkpointd.timer"
  echo active >"$case_dir/fake/state.go-choir-platform-dolt-history-audit.timer"
}

run_script() { # args...; env FAKE_* set by caller
  FAKE_DIR="$case_dir/fake" PATH="$root/bin:$PATH" \
    SQUASH_BIN="$root/bin/platform-dolt-squash" DATA_DIR="$case_dir/data" BACKUP_ROOT="$case_dir/backups" \
    CP_REFLINK="" MIN_FREE_FLOOR_KIB="${MIN_FREE_FLOOR_KIB:-1}" STOP_WAIT_SECONDS=2 SQL_CHECK_SECONDS=2 \
    SQL_CHECK_CMD="${SQL_CHECK_CMD:-}" FAKE_SQUASH_MODE="${FAKE_SQUASH_MODE:-ok}" \
    bash "$script" "$@" >"$case_dir/out.log" 2>&1
}

not() { ! "$@"; }
check() { # description, command...
  local desc="$1"; shift
  if "$@"; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "FAIL: $desc"; sed 's/^/    /' "$case_dir/out.log" | tail -15; fi
}
marker() { cat "$case_dir/data/platform/.dolt/marker"; }
is_up() { [ "$(cat "$case_dir/fake/state.$1" 2>/dev/null)" = active ]; }
no_state() { [ ! -f "$case_dir/backups/state" ]; }
line_of() { grep -n "^$1\$" "$case_dir/fake/systemctl.log" | head -1 | cut -d: -f1; }
backups() { ls -d "$case_dir"/backups/platform-dolt.* 2>/dev/null | wc -l | tr -d ' '; }
failed_dirs() { ls -d "$case_dir"/data.squash-failed-* 2>/dev/null | wc -l | tr -d ' '; }

# 1. Success: stopped before the backup, backup is the pre-squash store, the
# store is squashed, everything is started again (Dolt first, timers last).
new_case success
run_script; rc=$?
check "success exits 0" [ $rc -eq 0 ]
check "success squashed the store" [ "$(marker)" = SQUASHED ]
check "success: backup holds the pre-squash store" [ "$(cat "$case_dir"/backups/platform-dolt.*/platform/.dolt/marker)" = ORIGINAL ]
check "success: the squash ran with Dolt stopped" grep -q "squash .* platform inactive" "$case_dir/fake/squash.log"
check "success: dolt up" is_up "$DOLT"
check "success: corpusd up" is_up go-choir-corpusd.service
check "success: timers up" is_up go-choir-checkpointd.timer
check "success: state cleared" no_state
check "success: dependent stopped before dolt" [ "$(line_of 'stop go-choir-corpusd.service')" -lt "$(line_of "stop $DOLT")" ]
check "success: timer stopped before its dependents" [ "$(line_of 'stop go-choir-checkpointd.timer')" -lt "$(line_of 'stop go-choir-corpusd.service')" ]
check "success: dolt started before the dependents" [ "$(line_of "start $DOLT")" -lt "$(line_of 'start go-choir-corpusd.service')" ]
check "success: oneshot dependents that were not running are not started" not grep -q '^start go-choir-checkpointd.service$' "$case_dir/fake/systemctl.log"

# 2. Only the newest backup is kept across runs.
sleep 1
run_script; rc=$?
check "second run exits 0" [ $rc -eq 0 ]
check "second run keeps exactly the newest backup" [ "$(backups)" = 1 ]
check "second run's backup is of the store as the first run left it" [ "$(cat "$case_dir"/backups/platform-dolt.*/platform/.dolt/marker)" = SQUASHED ]

# 3. The command refuses before changing anything: no restore, services back, exit 3.
new_case refused
FAKE_SQUASH_MODE=refuse run_script; rc=$?
check "refusal exits 3" [ $rc -eq 3 ]
check "refusal leaves the store untouched" [ "$(marker)" = ORIGINAL ]
check "refusal does not restore (no failed dir)" [ "$(failed_dirs)" = 0 ]
check "refusal restarts dolt" is_up "$DOLT"
check "refusal restarts the timers" is_up go-choir-checkpointd.timer
check "refusal clears the state" no_state

# 4. The command fails after changing the store: the backup is restored, the
# failed store is kept as evidence, services are back, exit 1.
new_case failed
FAKE_SQUASH_MODE=fail run_script; rc=$?
check "failure exits 1" [ $rc -eq 1 ]
check "failure restores the original store" [ "$(marker)" = ORIGINAL ]
check "failure keeps the torn store as evidence" [ "$(failed_dirs)" = 1 ]
check "the kept store holds the torn state" [ "$(cat "$case_dir"/data.squash-failed-*/platform/.dolt/marker)" = TORN ]
check "failure restarts dolt" is_up "$DOLT"
check "failure restarts corpusd" is_up go-choir-corpusd.service
check "failure clears the state" no_state
# ... and the next run refuses while the evidence exists, touching nothing.
: >"$case_dir/fake/systemctl.log"
run_script; rc=$?
check "a run refuses while a failed store is kept" [ $rc -eq 2 ]
check "that refusal stops nothing" not grep -q '^stop' "$case_dir/fake/systemctl.log"

# 5. The squash process kills the script (an OOM kill takes the unit's main
# process): ExecStopPost runs --finish, which restores and restarts.
new_case killed
FAKE_SQUASH_MODE=kill run_script; rc=$?
check "a killed run dies" [ $rc -ne 0 ]
check "a killed run leaves the state file" [ -f "$case_dir/backups/state" ]
check "a killed run leaves dolt down" not is_up "$DOLT"
run_script --finish; rc=$?
check "--finish exits 0" [ $rc -eq 0 ]
check "--finish restores the original store" [ "$(marker)" = ORIGINAL ]
check "--finish restarts dolt" is_up "$DOLT"
check "--finish restarts corpusd and timers" is_up go-choir-corpusd.service
check "--finish clears the state" no_state
run_script --finish; rc=$?
check "--finish is idempotent" [ $rc -eq 0 ]

# 6. The backup fails (no reflink support): the squash never runs, the store
# is untouched, services are back, exit 1.
new_case nobackup
FAKE_DIR="$case_dir/fake" PATH="$root/bin:$PATH" SQUASH_BIN="$root/bin/platform-dolt-squash" DATA_DIR="$case_dir/data" \
  BACKUP_ROOT="$case_dir/backups" CP_REFLINK="--no-such-cp-flag" MIN_FREE_FLOOR_KIB=1 STOP_WAIT_SECONDS=2 \
  bash "$script" >"$case_dir/out.log" 2>&1; rc=$?
check "a failed backup exits 1" [ $rc -eq 1 ]
check "a failed backup never runs the squash" [ ! -f "$case_dir/fake/squash.log" ]
check "a failed backup leaves the store as it was" [ "$(marker)" = ORIGINAL ]
check "a failed backup leaves no partial backup" [ "$(ls "$case_dir/backups" | grep -c partial)" = 0 ]
check "a failed backup restarts dolt" is_up "$DOLT"

# 7. Not enough free space: refused at preflight, nothing stopped.
new_case nospace
MIN_FREE_FLOOR_KIB=999999999999 run_script; rc=$?
check "low free space exits 2" [ $rc -eq 2 ]
check "low free space stops nothing" not grep -q '^stop' "$case_dir/fake/systemctl.log"
check "low free space clears the state" no_state

# 8. An unfinished earlier run blocks a new one (and touches nothing).
new_case unfinished
printf 'state_phase=squashing\n' >"$case_dir/backups/state"
run_script; rc=$?
check "an unfinished run blocks a new run" [ $rc -eq 2 ]
check "an unfinished run: nothing stopped" not grep -q '^stop' "$case_dir/fake/systemctl.log"
check "an unfinished run: state kept" [ -f "$case_dir/backups/state" ]

# 9. Dolt will not stop: the store is never touched.
new_case stuck
mkdir -p "$root/stuckbin"
cp "$root/bin/systemctl" "$root/stuckbin/real-systemctl"
cat >"$root/stuckbin/systemctl" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = stop ] && [ "$2" = go-choir-platform-dolt.service ]; then echo "$*" >>"$FAKE_DIR/systemctl.log"; exit 0; fi
exec "$(dirname "$0")/real-systemctl" "$@"
EOF
chmod +x "$root/stuckbin/systemctl"
FAKE_DIR="$case_dir/fake" PATH="$root/stuckbin:$root/bin:$PATH" SQUASH_BIN="$root/bin/platform-dolt-squash" DATA_DIR="$case_dir/data" \
  BACKUP_ROOT="$case_dir/backups" CP_REFLINK="" MIN_FREE_FLOOR_KIB=1 STOP_WAIT_SECONDS=2 \
  bash "$script" >"$case_dir/out.log" 2>&1; rc=$?
check "a unit that will not stop exits 1" [ $rc -eq 1 ]
check "a unit that will not stop: no backup, no squash" [ ! -f "$case_dir/fake/squash.log" ]
check "a unit that will not stop: store untouched" [ "$(marker)" = ORIGINAL ]

# 10. The query check after the restart fails: exit 1 and the older backup is kept.
new_case sqlfail
run_script; rc=$?   # first run makes backup A
sleep 1
SQL_CHECK_CMD=false run_script; rc=$?
check "a failing query check exits 1" [ $rc -eq 1 ]
check "a failing query check keeps both backups" [ "$(backups)" = 2 ]

echo "platform-dolt-squash tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
