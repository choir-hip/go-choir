#!/usr/bin/env bash
# Store A (platform Dolt) weekly history squash, offline, with a backup first.
# Owner notes 2026-10-14 sections 23 and 27 item 7;
# docs/problems/store-a-weekly-squash-2026-10-20.md.
#
# Sequence: preflight (nothing touched) -> stop the Dolt unit's timers and
# dependents, then the unit -> reflink backup of the data directory ->
# platform-dolt-squash -> start everything again -> prune older backups.
#
#   platform-dolt-squash.sh            one run
#   platform-dolt-squash.sh --finish   recover after an interrupted run: restore
#                                      the backup if a squash was in flight,
#                                      start what the run stopped, clear state.
#                                      Idempotent; the unit runs it as
#                                      ExecStopPost so an OOM kill or a stop
#                                      cannot leave Store A down.
#
# Exit status: 0 squashed; 1 failed (a squash that may have changed the store
# was restored from the backup); 2 refused at preflight (nothing touched);
# 3 the command refused before changing anything (services restarted).
#
# Rollback after a bad outcome the checks missed: stop the Dolt unit, move the
# data directory aside, `cp -a --reflink=always <newest backup> <data dir>`,
# start the unit. The squash cannot be undone without the backup.
#
# Environment (defaults are Node B's; the tests override them):
#   SQUASH_BIN      platform-dolt-squash binary
#   DOLT_UNIT       go-choir-platform-dolt.service
#   DATA_DIR        /var/lib/go-choir/platform-dolt
#   DATABASE        platform
#   BACKUP_ROOT     /var/lib/go-choir/platform-dolt-squash-backups
#   CP_REFLINK      --reflink=always  (never silently falls back to a full copy)
#   MIN_FREE_FLOOR_KIB  10485760      free space floor; also at least half the store
#   SQL_CHECK_CMD   optional command that exits 0 when Dolt answers a query
#   SQL_CHECK_SECONDS 90
#   STOP_WAIT_SECONDS 60
#   SQUASH_NOTE     text for the squash commit message
set -u

SQUASH_BIN="${SQUASH_BIN:-platform-dolt-squash}"
DOLT_UNIT="${DOLT_UNIT:-go-choir-platform-dolt.service}"
DATA_DIR="${DATA_DIR:-/var/lib/go-choir/platform-dolt}"
DATABASE="${DATABASE:-platform}"
BACKUP_ROOT="${BACKUP_ROOT:-/var/lib/go-choir/platform-dolt-squash-backups}"
CP_REFLINK="${CP_REFLINK---reflink=always}"
MIN_FREE_FLOOR_KIB="${MIN_FREE_FLOOR_KIB:-10485760}"
SQL_CHECK_CMD="${SQL_CHECK_CMD:-}"
SQL_CHECK_SECONDS="${SQL_CHECK_SECONDS:-90}"
STOP_WAIT_SECONDS="${STOP_WAIT_SECONDS:-60}"
SQUASH_NOTE="${SQUASH_NOTE:-weekly}"

STATE_FILE="$BACKUP_ROOT/state"
LAST_RUN_JSON="$BACKUP_ROOT/last-run.json"

log() { printf 'platform-dolt-squash: %s\n' "$*"; }

# ---- state ---------------------------------------------------------------
# key=value lines; values are single words except the unit lists. Rewritten
# atomically at every phase, so an interrupted run can be recovered.
state_phase=""
state_backup=""
state_units=""
state_timers=""
state_failed_dir=""

write_state() { # phase
  state_phase="$1"
  {
    printf 'state_phase=%s\n' "$state_phase"
    printf 'state_backup=%s\n' "$state_backup"
    printf 'state_units="%s"\n' "$state_units"
    printf 'state_timers="%s"\n' "$state_timers"
    printf 'state_failed_dir=%s\n' "$state_failed_dir"
  } >"$STATE_FILE.tmp" && mv -f "$STATE_FILE.tmp" "$STATE_FILE"
}

unit_active() { [ "$(systemctl is-active "$1" 2>/dev/null)" = "active" ]; }

wait_inactive() { # unit
  local i=0
  while unit_active "$1"; do
    i=$((i + 1))
    if [ "$i" -gt "$STOP_WAIT_SECONDS" ]; then return 1; fi
    sleep 1
  done
  return 0
}

wait_active() { # unit
  local i=0
  until unit_active "$1"; do
    i=$((i + 1))
    if [ "$i" -gt "$STOP_WAIT_SECONDS" ]; then return 1; fi
    sleep 1
  done
  return 0
}

# ---- recovery (also the ExecStopPost) -------------------------------------
restore_backup() {
  if [ -z "$state_backup" ] || [ ! -d "$state_backup" ]; then
    log "cannot restore: no complete backup recorded; leaving services down"
    return 1
  fi
  local failed
  if [ -d "$DATA_DIR" ]; then
    failed="$DATA_DIR.squash-failed-$(date -u +%Y%m%dT%H%M%SZ)"
    log "restoring $state_backup to $DATA_DIR (the failed store is kept at $failed)"
    if ! mv "$DATA_DIR" "$failed"; then
      log "cannot move the failed store aside; leaving services down"
      return 1
    fi
    state_failed_dir="$failed"
  else
    log "restoring $state_backup to $DATA_DIR (an earlier restore moved the failed store aside: $state_failed_dir)"
  fi
  write_state restoring
  # shellcheck disable=SC2086
  if ! cp -a $CP_REFLINK "$state_backup" "$DATA_DIR"; then
    rm -rf "$DATA_DIR"
    log "restore copy failed; leaving services down. The failed store is at $failed and the backup at $state_backup"
    return 1
  fi
  return 0
}

start_services() {
  local u rc=0
  if ! systemctl start "$DOLT_UNIT"; then rc=1; fi
  if ! wait_active "$DOLT_UNIT"; then log "$DOLT_UNIT did not become active"; rc=1; fi
  for u in $state_units; do
    systemctl start "$u" || { log "failed to start $u"; rc=1; }
  done
  for u in $state_timers; do
    systemctl start "$u" || { log "failed to start $u"; rc=1; }
  done
  return $rc
}

sql_check() {
  [ -n "$SQL_CHECK_CMD" ] || return 0
  local i=0
  until $SQL_CHECK_CMD >/dev/null 2>&1; do
    i=$((i + 1))
    if [ "$i" -gt "$SQL_CHECK_SECONDS" ]; then return 1; fi
    sleep 1
  done
  return 0
}

# finish: bring the system to rest from whatever phase the state file records.
# Returns 0 when services are up and the state is cleared.
finish() {
  [ -f "$STATE_FILE" ] || return 0
  # shellcheck disable=SC1090
  . "$STATE_FILE"
  state_phase="${state_phase:-}"
  rm -rf "$BACKUP_ROOT"/*.partial
  case "$state_phase" in
    preflight)
      rm -f "$STATE_FILE"
      return 0
      ;;
    squashing | restoring | restore-pending)
      log "finishing an interrupted or failed squash: restoring the backup"
      if ! restore_backup; then
        write_state restore-failed
        return 1
      fi
      ;;
    restore-failed)
      log "an earlier restore failed; not touching anything. Inspect $BACKUP_ROOT and $DATA_DIR*"
      return 1
      ;;
  esac
  if ! start_services; then
    write_state start-failed
    log "services did not all start; state kept in $STATE_FILE"
    return 1
  fi
  rm -f "$STATE_FILE"
  return 0
}

if [ "${1:-}" = "--finish" ]; then
  finish
  exit $?
fi

# ---- run -----------------------------------------------------------------
mkdir -p "$BACKUP_ROOT" || { log "cannot create $BACKUP_ROOT"; exit 2; }
if ! ( set -C; : >"$STATE_FILE" ) 2>/dev/null; then
  log "refused: $STATE_FILE exists: an earlier run did not finish (or one is running). Run $0 --finish after reading the journal"
  exit 2
fi
write_state preflight

run_exit=1
on_exit() {
  trap - EXIT
  local rc=$run_exit
  if ! finish; then rc=1; fi
  exit $rc
}
trap on_exit EXIT
trap 'exit 1' INT TERM

if ! command -v "$SQUASH_BIN" >/dev/null 2>&1 && [ ! -x "$SQUASH_BIN" ]; then
  log "refused: squash command $SQUASH_BIN not found"
  run_exit=2; exit 2
fi
failed_leftovers="$(ls -d "$DATA_DIR".squash-failed-* 2>/dev/null | head -1)"
if [ -n "$failed_leftovers" ]; then
  log "refused: $failed_leftovers is a failed run's evidence; remove it by hand after reading it"
  run_exit=2; exit 2
fi
if [ ! -d "$DATA_DIR/$DATABASE/.dolt" ]; then
  log "refused: no Dolt repository at $DATA_DIR/$DATABASE"
  run_exit=2; exit 2
fi
size_kib="$(du -sk "$DATA_DIR" | cut -f1)"
free_kib="$(df -Pk "$BACKUP_ROOT" | awk 'NR==2 {print $4}')"
need_kib=$((size_kib / 2))
[ "$need_kib" -lt "$MIN_FREE_FLOOR_KIB" ] && need_kib="$MIN_FREE_FLOOR_KIB"
if [ "$free_kib" -lt "$need_kib" ]; then
  log "refused: ${free_kib} KiB free is under the ${need_kib} KiB the GC output needs (store is ${size_kib} KiB)"
  run_exit=2; exit 2
fi
log "preflight ok: store ${size_kib} KiB, free ${free_kib} KiB, need ${need_kib} KiB"

# Dependents (Requires=) of the Dolt unit and the timers that start them. A
# timer firing during the window would start Dolt again under the squash.
dependents="$(systemctl show -p RequiredBy --value "$DOLT_UNIT" 2>/dev/null)"
for u in $dependents; do
  case "$u" in *.service) ;; *) continue ;; esac
  if unit_active "$u"; then state_units="$state_units $u"; fi
  for t in $(systemctl show -p TriggeredBy --value "$u" 2>/dev/null); do
    case "$t" in *.timer) ;; *) continue ;; esac
    if unit_active "$t"; then
      case " $state_timers " in *" $t "*) ;; *) state_timers="$state_timers $t" ;; esac
    fi
  done
done
state_units="${state_units# }"
state_timers="${state_timers# }"

# From here the services are the run's to restore. A crash restart of Dolt is
# harmless; what must never happen is Dolt coming back under a squash.
write_state stopping
log "stopping timers [$state_timers], dependents [$state_units], then $DOLT_UNIT"
for t in $state_timers; do systemctl stop "$t" || true; done
for u in $state_units; do
  systemctl stop "$u" || true
  wait_inactive "$u" || { log "$u still active"; run_exit=1; exit 1; }
done
systemctl stop "$DOLT_UNIT" || true
if ! wait_inactive "$DOLT_UNIT"; then
  log "$DOLT_UNIT still active after ${STOP_WAIT_SECONDS}s; not touching the store"
  run_exit=1; exit 1
fi

ts="$(date -u +%Y%m%dT%H%M%SZ)"
backup="$BACKUP_ROOT/platform-dolt.$ts"
n=0
while [ -e "$backup" ] || [ -e "$backup.partial" ]; do n=$((n + 1)); backup="$BACKUP_ROOT/platform-dolt.$ts.$n"; done
log "reflink backup: $DATA_DIR -> $backup"
# shellcheck disable=SC2086
if ! cp -a $CP_REFLINK "$DATA_DIR" "$backup.partial"; then
  rm -rf "$backup.partial"
  log "backup failed; the store was not touched"
  run_exit=1; exit 1
fi
mv "$backup.partial" "$backup"
state_backup="$backup"
write_state backed-up

write_state squashing
log "squashing $DATA_DIR/$DATABASE"
"$SQUASH_BIN" -dir "$DATA_DIR" -database "$DATABASE" -note "$SQUASH_NOTE" >"$LAST_RUN_JSON"
rc=$?
cat "$LAST_RUN_JSON"
case $rc in
  0)
    write_state squashed
    log "squash ok; starting services"
    ;;
  3)
    write_state refused-clean
    log "the command refused before changing anything; the store is as it was"
    run_exit=3
    exit 3
    ;;
  *)
    log "the squash failed (exit $rc); the backup will be restored"
    run_exit=1
    exit 1
    ;;
esac

# Success path: start, check a query, then prune.
trap - EXIT
if ! finish; then
  log "FAILED: services did not come back after a good squash (store root verified); see the journal"
  exit 1
fi
if ! sql_check; then
  log "FAILED: Dolt started but the query check did not pass within ${SQL_CHECK_SECONDS}s; the older backups are kept"
  exit 1
fi
for old in "$BACKUP_ROOT"/platform-dolt.*; do
  [ -e "$old" ] || continue
  [ "$old" = "$backup" ] && continue
  log "pruning older backup $old"
  rm -rf "$old"
done
log "done; newest backup kept at $backup"
exit 0
