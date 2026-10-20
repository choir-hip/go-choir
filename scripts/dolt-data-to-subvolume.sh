#!/usr/bin/env bash
# One-time move of a Dolt server's data directory into its own btrfs
# subvolume, so the off-host backup can capture it as an atomic read-only
# snapshot instead of a server-side DOLT_BACKUP (design:
# docs/problems/no-off-host-backup-2026-10-20.md section 13 item 16).
#
# Usage (Node B, root, a planned stop of the store):
#   dolt-data-to-subvolume.sh platform   # Store A, platform-dolt/platform, :13306
#   dolt-data-to-subvolume.sh corpus     # Store B, corpus-dolt, :13307
#
# What it does: reads the head, pauses the Dolt liveness probe, stops the
# server (systemd also stops the units that Require it; they are started
# again), creates <dir>.subvolume-new, copies the directory into it with
# reflinks (metadata only), swaps the two, starts the server and its
# dependents, and compares the head. The old directory stays as
# <dir>.pre-subvolume-<time> until a restore drill has passed; delete it
# by hand then. Any failure after the stop puts the old directory back and
# starts everything again.
#
# Rollback after success: stop the server, move <dir> aside, move
# <dir>.pre-subvolume-<time> back to <dir>, start the server and its
# dependents. The backup falls back to the server-side sync on its own.
set -euo pipefail

case "${1:-}" in
  platform) UNIT=go-choir-platform-dolt.service; DIR=/var/lib/go-choir/platform-dolt/platform; PORT=13306; DB=platform
            DEPENDENTS=(go-choir-corpusd.service go-choir-checkpointd.service) ;;
  corpus)   UNIT=go-choir-corpus-dolt.service; DIR=/var/lib/go-choir/corpus-dolt; PORT=13307; DB=corpus
            DEPENDENTS=(go-choir-corpusd.service) ;;
  *) sed -n '2,22p' "$0" >&2; exit 2 ;;
esac
PROBE=go-choir-dolt-liveness-probe.timer
NEW="$DIR.subvolume-new"
OLD="$DIR.pre-subvolume-$(date -u +%Y%m%dT%H%M%SZ)"
log() { printf '%s dolt-subvolume: %s\n' "$(date -u +%FT%TZ)" "$*"; }

[ "$(id -u)" = 0 ] || { log "run as root"; exit 2; }
[ -d "$DIR" ] || { log "$DIR is absent"; exit 2; }
if btrfs subvolume show "$DIR" >/dev/null 2>&1; then log "$DIR is already a subvolume; nothing to do"; exit 0; fi
[ "$(stat -f -c %T "$DIR")" = btrfs ] || { log "$DIR is not on btrfs"; exit 2; }
[ ! -e "$NEW" ] || { log "$NEW exists (an earlier attempt?); inspect and remove it first"; exit 2; }
for u in go-choir-offhost-backup.service go-choir-offhost-backup-verify.service; do
  ! systemctl is-active -q "$u" || { log "$u is running; wait for it"; exit 2; }
done

head_of() { # newest commit, as root with its password or passwordless
  local pw home out
  home="$(mktemp -d)"
  pw="$(cat /var/lib/go-choir/secrets/dolt-root-password 2>/dev/null || true)"
  case "$pw" in *[!A-Za-z0-9]*) pw="" ;; esac
  if [ -n "$pw" ] && printf 'select 1;\n' | HOME="$home" DOLT_CLI_PASSWORD="$pw" dolt --host 127.0.0.1 --port "$PORT" --no-tls --user root sql >/dev/null 2>&1; then
    out="$(HOME="$home" DOLT_CLI_PASSWORD="$pw" dolt --host 127.0.0.1 --port "$PORT" --no-tls --user root --use-db "$DB" sql -r csv -q 'select commit_hash from dolt_log limit 1' 2>/dev/null | tail -1)"
  else
    out="$(HOME="$home" dolt --host 127.0.0.1 --port "$PORT" --no-tls --user root --password "" --use-db "$DB" sql -r csv -q 'select commit_hash from dolt_log limit 1' 2>/dev/null | tail -1)"
  fi
  rm -rf "$home"; printf '%s' "$out"
}
start_all() {
  systemctl start "$UNIT" || true
  local u; for u in "${RESTART[@]}"; do systemctl start "$u" || log "could not start $u"; done
  systemctl start "$PROBE" || true
}

BEFORE="$(head_of)"
[ -n "$BEFORE" ] || { log "could not read the head on :$PORT; is the server up?"; exit 1; }
RESTART=()
for u in "${DEPENDENTS[@]}"; do if systemctl is-active -q "$u"; then RESTART+=("$u"); fi; done
log "head before: $BEFORE; dependents to start again: ${RESTART[*]:-none}"

btrfs subvolume create "$NEW" >/dev/null
chmod --reference="$DIR" "$NEW"; chown --reference="$DIR" "$NEW"

STOPPED=0
rollback() {
  log "FAILED; rolling back"
  if [ -d "$OLD" ]; then # the original was moved aside: put it back
    systemctl stop "$UNIT" || true
    if [ -e "$DIR" ]; then mv "$DIR" "$NEW.failed"; fi
    mv "$OLD" "$DIR"
  fi
  [ "$STOPPED" = 1 ] && start_all
  log "rolled back; $DIR is the original directory. Inspect and remove $NEW or $NEW.failed by hand."
}
trap 'rollback' ERR

t0=$(date +%s)
systemctl stop "$PROBE"
systemctl stop "$UNIT"; STOPPED=1
cp -a --reflink=always "$DIR"/. "$NEW"/
[ "$(find "$DIR" | wc -l)" = "$(find "$NEW" | wc -l)" ] || { log "entry counts differ after the copy"; false; }
[ "$(du -s --apparent-size -B1 "$DIR" | cut -f1)" = "$(du -s --apparent-size -B1 "$NEW" | cut -f1)" ] || { log "sizes differ after the copy"; false; }
mv "$DIR" "$OLD"; mv "$NEW" "$DIR"
btrfs subvolume show "$DIR" >/dev/null
start_all
t1=$(date +%s)
trap - ERR

AFTER=""
for _ in $(seq 1 60); do AFTER="$(head_of)"; [ -n "$AFTER" ] && break; sleep 2; done
log "store stopped for $((t1 - t0)) s; head after: ${AFTER:-unread}"
if [ "$AFTER" != "$BEFORE" ]; then
  log "WARNING: the head changed or could not be read; check the store before deleting $OLD"; exit 1
fi
log "done: $DIR is a subvolume, the head is unchanged, the old directory is $OLD"
