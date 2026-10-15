#!/usr/bin/env bash
# Query-level liveness probe for the platform (13306) and corpus (13307) Dolt
# servers on Node B. Run every 30 s by go-choir-dolt-liveness-probe.timer.
#
# docs/problems/dolt-clustering-assessment-2026-10-15.md sections 4 and 7: on
# 2026-10-15 platform Dolt accepted connections but answered nothing for 40
# minutes and nothing noticed. A TCP check or handshake passes in that state;
# only a query shows it. After STRIKES consecutive failures the probe records
# the process state, sends SIGABRT so the Go runtime dumps every goroutine to
# the journal (the missing stack), and lets systemd restart the unit
# (Restart=on-failure).
#
# Usage:  dolt-liveness-probe.sh            one probe round over both servers
#         dolt-liveness-probe.sh --sql PORT DB   (internal) run SELECT 1
#
# Environment (all optional; the PROBE_* knobs exist for the test harness
# scripts/test-dolt-liveness-probe.sh and for dry runs):
#   PROBE_DRY_RUN=1          log what would be done; send no signal, no restart
#   PROBE_STRIKES=3          consecutive failures before acting
#   PROBE_DEADLINE=5         seconds allowed for one SELECT 1
#   PROBE_COOLDOWN=600       minimum seconds between signals to one unit
#   PROBE_GRACE=120          seconds a unit must have been active before probing
#   PROBE_POST_SIGNAL_WAIT=20  seconds to wait for the process to go before
#                            escalating to systemctl restart
#
# Credentials: --sql mode reads the Dolt app user's password from
# $CREDENTIALS_DIRECTORY/dolt-app-password (systemd LoadCredential=, the same
# source as the go-choir-*-dolt-users units) and hands it to the dolt client
# only as DOLT_CLI_PASSWORD. It is never placed in argv, never logged, and
# client stderr is classified, never echoed. There is no `set -x`.
set -u

if [ "${1:-}" = "--sql" ]; then
  port="$2"
  db="$3"
  cred="${CREDENTIALS_DIRECTORY:-}/dolt-app-password"
  if [ ! -r "$cred" ]; then
    echo "credential unreadable" >&2
    exit 78
  fi
  pw="$(cat "$cred")"
  case "$pw" in
    "" | *[!A-Za-z0-9]*)
      echo "credential invalid" >&2
      exit 78
      ;;
  esac
  # dolt wants a writable HOME; the main run removes stale ones.
  HOME="$(mktemp -d "${PROBE_STATE_DIR:-${RUNTIME_DIRECTORY:-/run/go-choir-dolt-liveness}}/home.XXXXXX")"
  export HOME
  cd "$HOME" || exit 78
  printf 'select 1;\n' | DOLT_CLI_PASSWORD="$pw" dolt --host 127.0.0.1 --port "$port" --no-tls \
    --user choir --use-db "$db" sql >/dev/null
  code=$?
  rm -rf "$HOME"
  exit "$code"
fi

STRIKES="${PROBE_STRIKES:-3}"
DEADLINE="${PROBE_DEADLINE:-5}"
COOLDOWN="${PROBE_COOLDOWN:-600}"
GRACE="${PROBE_GRACE:-120}"
POST_WAIT="${PROBE_POST_SIGNAL_WAIT:-20}"
DRY="${PROBE_DRY_RUN:-0}"
STATE="${PROBE_STATE_DIR:-${RUNTIME_DIRECTORY:-/run/go-choir-dolt-liveness}}"
PROC="${PROBE_PROC_ROOT:-/proc}"
CGROUP="${PROBE_CGROUP_ROOT:-/sys/fs/cgroup}"
NOW="${PROBE_NOW:-$(date +%s)}"
if [ -n "${PROBE_MONO_NOW_US:-}" ]; then
  MONO_NOW_US="$PROBE_MONO_NOW_US"
else
  MONO_NOW_US="$(awk '{printf "%d", $1 * 1000000}' /proc/uptime)"
fi
export PROBE_STATE_DIR="$STATE"

log() { echo "dolt-liveness-probe: $*"; }
indent() { sed 's/^/dolt-liveness-probe:   /'; }

mkdir -p "$STATE" || { log "probe-error cannot create state dir"; exit 1; }
rm -rf "$STATE"/home.* "$STATE"/err.* 2>/dev/null

# One run at a time (flock is util-linux; absent on a dev laptop, where the
# harness runs single-threaded anyway).
if command -v flock >/dev/null 2>&1; then
  exec 9>"$STATE/lock"
  if ! flock -n 9; then
    log "previous run still active; skipping"
    exit 0
  fi
fi

run_sql() {
  if [ -n "${PROBE_SQL_CMD:-}" ]; then
    timeout -k 1 "$DEADLINE" "$PROBE_SQL_CMD" "$@"
  else
    timeout -k 1 "$DEADLINE" "$0" --sql "$@"
  fi
}

read_num() { # file default
  local v
  v="$(cat "$1" 2>/dev/null || true)"
  case "$v" in "" | *[!0-9]*) echo "$2" ;; *) echo "$v" ;; esac
}

show_field() { # key (from $show)
  sed -n "s/^$1=//p" <<<"$show" | head -1
}

capture_evidence() { # unit pid
  local unit="$1" pid="$2" f t
  log "evidence unit=$unit pid=$pid"
  {
    echo "--- $PROC/$pid/status"
    grep -E '^(Name|State|Threads|VmRSS|VmSwap|voluntary_ctxt_switches|nonvoluntary_ctxt_switches):' "$PROC/$pid/status" 2>&1
    echo "--- $PROC/$pid/wchan"
    cat "$PROC/$pid/wchan" 2>&1
    echo
    echo "--- $PROC/$pid/io"
    cat "$PROC/$pid/io" 2>&1
    echo "--- $PROC/$pid/task/*/wchan (threads per wait channel)"
    for t in "$PROC/$pid"/task/*/wchan; do
      { cat "$t" 2>/dev/null || true; echo; }
    done | sort | uniq -c | sort -rn | head -10
    echo "--- $PROC/$pid/stack"
    head -20 "$PROC/$pid/stack" 2>&1
    for f in memory.events memory.pressure; do
      echo "--- cgroup $unit.service/$f"
      cat "$CGROUP/system.slice/$unit.service/$f" 2>&1
    done
  } 2>&1 | indent
}

overall=0

act() { # unit pid
  local unit="$1" pid="$2" last since comm newpid i
  last="$(read_num "$STATE/$unit.last_signal" 0)"
  since=$((NOW - last))
  if [ "$last" -gt 0 ] && [ "$since" -lt "$COOLDOWN" ]; then
    log "unit=$unit still failing; cooldown ${since}s of ${COOLDOWN}s since the last signal, no signal sent"
    return
  fi
  capture_evidence "$unit" "$pid"
  comm="$(cat "$PROC/$pid/comm" 2>/dev/null || true)"
  if [ -z "$pid" ] || [ "$pid" = 0 ] || [ "$comm" != dolt ]; then
    log "probe-error unit=$unit main pid '$pid' is not dolt (comm='$comm'); refusing to signal"
    overall=1
    return
  fi
  if [ "$DRY" = 1 ]; then
    log "DRY RUN: would send SIGABRT to $unit main pid $pid and let systemd restart it"
    echo 0 >"$STATE/$unit.strikes"
    return
  fi
  log "unit=$unit unresponsive for $STRIKES consecutive probes; sending SIGABRT to main pid $pid (goroutine dump follows in this unit's journal)"
  echo "$NOW" >"$STATE/$unit.last_signal"
  echo 0 >"$STATE/$unit.strikes"
  systemctl kill --kill-whom=main --signal=ABRT "$unit"
  i=0
  while [ "$i" -lt "$POST_WAIT" ]; do
    newpid="$(systemctl show "$unit" -p MainPID | sed -n 's/^MainPID=//p')"
    [ "$newpid" != "$pid" ] && break
    sleep 1
    i=$((i + 1))
  done
  newpid="$(systemctl show "$unit" -p MainPID | sed -n 's/^MainPID=//p')"
  if [ "$newpid" = "$pid" ]; then
    log "unit=$unit main pid $pid still present ${POST_WAIT}s after SIGABRT; escalating to systemctl restart"
    systemctl restart "$unit"
  else
    log "unit=$unit main pid $pid is gone (now '$newpid'); systemd restarts it"
  fi
}

probe_target() { # unit port db
  local unit="$1" port="$2" db="$3" show state pid enter age strikes errf code class
  show="$(systemctl show "$unit" -p ActiveState -p MainPID -p ActiveEnterTimestampMonotonic 2>/dev/null)" || {
    log "probe-error cannot read the state of $unit"
    overall=1
    return
  }
  state="$(show_field ActiveState)"
  pid="$(show_field MainPID)"
  enter="$(show_field ActiveEnterTimestampMonotonic)"
  if [ "$state" != active ]; then
    echo 0 >"$STATE/$unit.strikes"
    log "unit=$unit is '$state'; skipping (deploy switch, boot or restart in progress)"
    return
  fi
  case "$enter" in "" | *[!0-9]*) enter=0 ;; esac
  age=$(((MONO_NOW_US - enter) / 1000000))
  if [ "$enter" -eq 0 ] || [ "$age" -lt "$GRACE" ]; then
    echo 0 >"$STATE/$unit.strikes"
    log "unit=$unit active for only ${age}s (grace ${GRACE}s); skipping"
    return
  fi

  errf="$(mktemp "$STATE/err.XXXXXX")"
  run_sql "$port" "$db" >/dev/null 2>"$errf"
  code=$?
  if [ "$code" -eq 0 ]; then
    class=ok
  elif [ "$code" -eq 124 ] || [ "$code" -eq 137 ]; then
    class=timeout
  elif [ "$code" -eq 78 ] || grep -qiE 'access denied|authentication|unknown user' "$errf"; then
    class=cred
  elif grep -qi 'connection refused' "$errf"; then
    class=refused
  else
    class=other
  fi
  rm -f "$errf"

  strikes="$(read_num "$STATE/$unit.strikes" 0)"
  case "$class" in
    ok)
      [ "$strikes" -gt 0 ] && log "unit=$unit port=$port recovered after $strikes failed probe(s)"
      echo 0 >"$STATE/$unit.strikes"
      ;;
    timeout | refused)
      strikes=$((strikes + 1))
      echo "$strikes" >"$STATE/$unit.strikes"
      log "unit=$unit port=$port strike $strikes/$STRIKES ($class, deadline ${DEADLINE}s)"
      if [ "$strikes" -ge "$STRIKES" ]; then
        act "$unit" "$pid"
      fi
      ;;
    *)
      # Our own fault (credential, client binary, parsing): never a reason to
      # kill a server. Visible as a failed probe unit.
      log "probe-error unit=$unit port=$port class=$class exit=$code (not a strike)"
      overall=1
      ;;
  esac
}

probe_target go-choir-platform-dolt 13306 platform
probe_target go-choir-corpus-dolt 13307 corpus
exit "$overall"
