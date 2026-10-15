#!/usr/bin/env bash
# Behavioural test for scripts/dolt-liveness-probe.sh. It drives the real
# script with fake `systemctl`, a fake SQL command and a fake /proc tree, so it
# needs no Dolt, no root and no credentials, and cannot signal anything.
#
#   bash scripts/test-dolt-liveness-probe.sh
#
# docs/problems/dolt-clustering-assessment-2026-10-15.md section 7.2 lists the
# failure modes each case below is written against.
set -u
here="$(cd "$(dirname "$0")" && pwd)"
probe="$here/dolt-liveness-probe.sh"
fail=0
pass=0

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

# ---- fakes ---------------------------------------------------------------
mkdir -p "$root/bin" "$root/fake" "$root/state" "$root/proc"

cat >"$root/bin/systemctl" <<'EOF'
#!/usr/bin/env bash
# fake systemctl: show reads $FAKE_DIR/<unit>.show; kill/restart are recorded.
echo "systemctl $*" >>"$FAKE_DIR/systemctl.log"
case "$1" in
  show)
    unit="$2"
    cat "$FAKE_DIR/$unit.show"
    ;;
  kill)
    unit="${!#}"
    if [ -n "${FAKE_KILL_CHANGES_PID:-}" ]; then
      sed -i.bak 's/^MainPID=.*/MainPID=99999/' "$FAKE_DIR/$unit.show"
    fi
    ;;
  restart) ;;
esac
exit 0
EOF
chmod +x "$root/bin/systemctl"

cat >"$root/bin/fake-sql" <<'EOF'
#!/usr/bin/env bash
# fake SQL command: args are <port> <db>; behaviour from $FAKE_DIR/sql.<port>.
echo "sql $1 $2" >>"$FAKE_DIR/sql.log"
mode="$(cat "$FAKE_DIR/sql.$1" 2>/dev/null || echo ok)"
case "$mode" in
  ok) exit 0 ;;
  timeout) exit 124 ;;
  hang) sleep 5; exit 0 ;;
  refused) echo "dial tcp 127.0.0.1:$1: connect: connection refused" >&2; exit 1 ;;
  denied) echo "Access denied for user 'choir' (using password: YES) SECRET-CANARY-VALUE" >&2; exit 1 ;;
  other) echo "something unexpected SECRET-CANARY-VALUE" >&2; exit 1 ;;
esac
EOF
chmod +x "$root/bin/fake-sql"

set_unit() { # unit state pid active_enter_monotonic_us
  printf 'ActiveState=%s\nSubState=running\nMainPID=%s\nActiveEnterTimestampMonotonic=%s\n' "$2" "$3" "$4" >"$root/fake/$1.show"
}
set_sql() { printf '%s' "$2" >"$root/fake/sql.$1"; }

# A fake /proc/<pid> for a process whose comm is dolt.
make_proc() { # pid comm
  mkdir -p "$root/proc/$1/task/$1"
  printf '%s\n' "$2" >"$root/proc/$1/comm"
  printf 'Name:\t%s\nState:\tS (sleeping)\nVmRSS:\t3500000 kB\nThreads:\t40\n' "$2" >"$root/proc/$1/status"
  printf 'futex_wait_queue' >"$root/proc/$1/wchan"
  printf 'rchar: 1\nwchar: 2\n' >"$root/proc/$1/io"
  printf 'futex_wait_queue' >"$root/proc/$1/task/$1/wchan"
  echo 'proc-uptime' >"$root/proc/$1/stack"
}

NOW=100000
MONO_NOW_US=500000000000 # 500000 s of uptime
OLD_ENTER=$((MONO_NOW_US - 3600 * 1000000)) # active for an hour

reset_world() {
  rm -rf "$root/state" "$root/fake" "$root/proc"
  mkdir -p "$root/state" "$root/fake" "$root/proc"
  : >"$root/fake/systemctl.log"
  : >"$root/fake/sql.log"
  set_unit go-choir-platform-dolt active 4242 "$OLD_ENTER"
  set_unit go-choir-corpus-dolt active 4343 "$OLD_ENTER"
  make_proc 4242 dolt
  make_proc 4343 dolt
  set_sql 13306 ok
  set_sql 13307 ok
  NOW=100000
  unset FAKE_KILL_CHANGES_PID
}

# tick [VAR=value ...]: one probe run; output to $root/out, status to $rc.
rc=0
tick() {
  env FAKE_DIR="$root/fake" PATH="$root/bin:$PATH" \
    PROBE_STATE_DIR="$root/state" PROBE_PROC_ROOT="$root/proc" \
    PROBE_CGROUP_ROOT="$root/cgroup" \
    PROBE_SQL_CMD="$root/bin/fake-sql" PROBE_NOW="$NOW" \
    PROBE_MONO_NOW_US="$MONO_NOW_US" PROBE_POST_SIGNAL_WAIT=0 \
    "$@" bash "$probe" >"$root/out" 2>&1
  rc=$?
  cat "$root/out" >>"$root/all-out"
  NOW=$((NOW + 30))
}

ok() { pass=$((pass + 1)); echo "ok   - $1"; }
bad() { fail=$((fail + 1)); echo "FAIL - $1"; echo "----- last output"; cat "$root/out"; echo "----- systemctl.log"; cat "$root/fake/systemctl.log"; echo "-----"; }
expect() { # description, condition-command...
  local d="$1"; shift
  if "$@"; then ok "$d"; else bad "$d"; fi
}
kills() { grep -c '^systemctl kill' "$root/fake/systemctl.log"; }
restarts() { grep -c '^systemctl restart' "$root/fake/systemctl.log"; }
out_has() { grep -q -- "$1" "$root/out"; }
all_has() { grep -q -- "$1" "$root/all-out"; }

# ---- cases ---------------------------------------------------------------
: >"$root/all-out"

reset_world
tick
expect "healthy servers: exit 0, nothing signalled" test "$rc" -eq 0 -a "$(kills)" -eq 0
expect "healthy servers: both ports probed" test "$(grep -c '^sql ' "$root/fake/sql.log")" -eq 2

reset_world
set_sql 13306 timeout
tick; tick
set_sql 13306 ok
tick
set_sql 13306 timeout
tick; tick
expect "two strikes, a success, two strikes: counter reset, no signal" test "$(kills)" -eq 0

reset_world
set_sql 13306 timeout
tick; tick; tick
expect "three consecutive timeouts: one SIGABRT to the main process of the platform unit" \
  grep -q '^systemctl kill --kill-whom=main --signal=ABRT go-choir-platform-dolt$' "$root/fake/systemctl.log"
expect "three consecutive timeouts: exactly one signal" test "$(kills)" -eq 1
expect "evidence logged before the signal (wchan, status, io)" \
  bash -c "grep -q 'wchan' '$root/out' && grep -q 'VmRSS' '$root/out' && grep -q 'rchar' '$root/out'"
expect "corpus Dolt untouched while only platform stalls" \
  bash -c "! grep -q '^systemctl kill.*corpus' '$root/fake/systemctl.log'"

# keep failing right after the signal: cooldown must prevent a second ABRT
tick; tick; tick; tick
expect "cooldown: no second signal within 10 minutes" test "$(kills)" -eq 1
expect "cooldown: the log says why no signal was sent" all_has 'cooldown'
NOW=$((NOW + 700))
tick; tick; tick
expect "after the cooldown a persistent stall is signalled again" test "$(kills)" -eq 2

reset_world
set_unit go-choir-platform-dolt activating 0 0
set_sql 13306 timeout
tick; tick; tick; tick
expect "unit activating: probe skipped, never signalled" test "$(kills)" -eq 0
expect "unit activating: no SQL attempted against it" test "$(grep -c '^sql 13306' "$root/fake/sql.log")" -eq 0

reset_world
set_unit go-choir-platform-dolt active 4242 $((MONO_NOW_US - 30 * 1000000))
set_sql 13306 timeout
tick; tick; tick; tick
expect "unit active for 30 s (inside the 120 s grace): skipped" test "$(kills)" -eq 0

reset_world
set_unit go-choir-platform-dolt active 4242 "$OLD_ENTER"
set_sql 13306 timeout
tick; tick
set_unit go-choir-platform-dolt activating 0 0
tick
set_unit go-choir-platform-dolt active 4242 "$OLD_ENTER"
tick; tick
expect "a skip (deploy switch) resets the strikes: 2+skip+2 does not signal" test "$(kills)" -eq 0

reset_world
set_sql 13306 refused
tick; tick; tick
expect "connection refused on an active, past-grace unit is a strike" test "$(kills)" -eq 1

reset_world
set_sql 13306 denied
tick; tick; tick; tick; tick
expect "access denied is a probe error, not a strike: never signalled" test "$(kills)" -eq 0
expect "access denied: probe exits non-zero so the failure is visible" test "$rc" -ne 0
expect "access denied: logged as probe-error" out_has 'probe-error'
expect "credential-shaped stderr is never echoed" bash -c "! grep -q 'SECRET-CANARY-VALUE' '$root/all-out'"

reset_world
set_sql 13306 other
tick; tick; tick; tick
expect "unrecognised client error is a probe error, not a strike" test "$(kills)" -eq 0
expect "unrecognised error text is never echoed" bash -c "! grep -q 'SECRET-CANARY-VALUE' '$root/all-out'"

reset_world
set_sql 13306 timeout
tick PROBE_DRY_RUN=1; tick PROBE_DRY_RUN=1; tick PROBE_DRY_RUN=1
expect "dry run: nothing signalled" test "$(kills)" -eq 0 -a "$(restarts)" -eq 0
expect "dry run: says what it would have done" out_has 'DRY RUN'

reset_world
make_proc 4242 notdolt
set_sql 13306 timeout
tick; tick; tick
expect "main pid is not a dolt process: refuses to signal" test "$(kills)" -eq 0
expect "main pid is not a dolt process: says so" out_has 'not dolt'

reset_world
set_sql 13306 timeout
tick; tick; tick
expect "pid unchanged after the signal: escalates to a restart" test "$(restarts)" -eq 1

reset_world
export FAKE_KILL_CHANGES_PID=1
set_sql 13306 timeout
tick FAKE_KILL_CHANGES_PID=1; tick FAKE_KILL_CHANGES_PID=1; tick FAKE_KILL_CHANGES_PID=1
unset FAKE_KILL_CHANGES_PID
expect "pid changed after the signal: no restart escalation" test "$(kills)" -eq 1 -a "$(restarts)" -eq 0

reset_world
set_sql 13306 hang
tick PROBE_DEADLINE=1; tick PROBE_DEADLINE=1; tick PROBE_DEADLINE=1
expect "a SQL command that hangs is cut at the deadline and counts as a timeout" test "$(kills)" -eq 1

reset_world
set_sql 13306 timeout
set_sql 13307 timeout
tick; tick; tick
expect "both servers stalled: each is signalled once" test "$(kills)" -eq 2

reset_world
set_sql 13307 timeout
tick; tick; tick
expect "only corpus stalled: corpus unit signalled" \
  grep -q '^systemctl kill --kill-whom=main --signal=ABRT go-choir-corpus-dolt$' "$root/fake/systemctl.log"

echo
echo "dolt-liveness-probe tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
