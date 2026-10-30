#!/usr/bin/env bash
# Off-host backup restore drill (docs/problems/no-off-host-backup-2026-10-20.md
# section 9), run on Node B as root inside a memory-capped unit, for example:
#   systemd-run --wait --collect -p MemoryHigh=10G -p MemoryMax=12G \
#     -p MemorySwapMax=0 -p OOMScoreAdjust=1000 -p Environment=GOMEMLIMIT=8GiB \
#     -p Environment=PATH=/run/current-system/sw/bin -p Nice=19 -p IOSchedulingClass=idle \
#     bash scripts/off-host-restore-drill.sh <snapshot> tape|dolt|image|snap|pg [vm]
# restic needs about 8 GB to walk a 9.6M-file snapshot (the snap piece
# peaked at 7 GB with no swap, no-off-host-backup record section 13 item
# 21), so the Go limit is 8 GiB under the backup unit's own 10G and 12G
# lines; never run it uncapped beside the guests. Everything lands under a scratch directory,
# never a live path, and each piece deletes its restore before it exits.
#   tape:  200 random tape files compared byte for byte with live, and the
#          two sqlite stores' integrity checks.
#   dolt:  the corpus Dolt backup (restore, `dolt backup restore`, query,
#          head compared with the run's), and the platform Dolt backup
#          directory compared byte for byte with live (no second copy: disk).
#   pg:    the PostgreSQL dumps of a run made after the cutover (phase 2
#          item I7): each restored from staging/pg, `pg_restore --list`
#          read, its entry count compared with the run's manifest. The dolt
#          and snap pieces apply to snapshots from before the cutover.
#   image: one stopped test computer's data.img, compared with live and
#          checked read-only.
#   snap:  the Dolt stores captured as btrfs snapshots (section 13 item 16):
#          each restored data directory opened with `dolt sql` (journal
#          recovery as after a power cut), the head the run recorded found
#          in its log, the table count read, and for Store A the restored
#          event receipt count at least the one the run recorded (its head
#          stops moving with PLATFORM_DOLT_SNAPSHOTS=off; a count the run
#          could not read fails). One store at a time.
# Secrets are read from files and never printed. Each piece deletes its
# restore before the next.
set -euo pipefail
SNAP="${1:?snapshot}"; PIECE="${2:?tape|dolt|image|snap|pg}"
A=/var/lib/go-choir/platform-artifacts/sha256
S="${OFFHOST_SECRETS_DIR:-/var/lib/go-choir/secrets/offhost-backup}"
D="${OFFHOST_DRILL_DIR:-/var/lib/go-choir-restore-drill}"
# The drill chmods $D and deletes under it: an absolute path below / only
# (review round 3 F3 of
# docs/archive/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md).
case "$D" in /*) ;; *) echo "OFFHOST_DRILL_DIR must be an absolute path, not '$D'" >&2; exit 2 ;; esac
case "$D/" in */../*|*/./*) echo "OFFHOST_DRILL_DIR must not contain . or .. components: '$D'" >&2; exit 2 ;; esac
[ -n "$(printf '%s' "$D" | tr -d /)" ] || { echo "OFFHOST_DRILL_DIR must not be /" >&2; exit 2; }
install -d -m 0700 "$D"
CTL="$D/ssh2.ctl"
ssh -i "$S/ssh-key" -o IdentitiesOnly=yes -o BatchMode=yes -o UserKnownHostsFile="$S/known-hosts" \
  -o StrictHostKeyChecking=yes -o ExitOnForwardFailure=yes -o ControlMaster=yes -o ControlPath="$CTL" \
  -f -N -L 127.0.0.1:18002:127.0.0.1:8000 choir-backup@51.81.93.94
trap 'ssh -O exit -o ControlPath="$CTL" choir-backup@51.81.93.94 >/dev/null 2>&1 || true; rm -rf "$D/r" "$D/r2" "$D/dolt-restored"' EXIT
export RESTIC_PASSWORD_FILE="$S/restic-password" RESTIC_REST_USERNAME=nodeb
RESTIC_REST_PASSWORD="$(cat "$S/rest-password")"; export RESTIC_REST_PASSWORD
export RESTIC_REPOSITORY=rest:http://127.0.0.1:18002/nodeb/daily RESTIC_CACHE_DIR="$D/cache"
free_gib() { df -Pk /var/lib | awk 'NR==2{print int($4/1048576)}'; }
restore() { # path in snapshot
  local t0; t0=$(date +%s)
  restic --no-lock restore "$SNAP:$1" --target "$D/r2" --sparse >"$D/restore2.log" 2>&1 \
    || { echo "restore of $1 failed: $(tail -c 400 "$D/restore2.log")"; exit 1; }
  echo "restored $1: $(du -sh "$D/r2" | cut -f1) on disk in $(( $(date +%s) - t0 ))s; free $(free_gib) GiB"
}

case "$PIECE" in
tape)
  t0=$(date +%s)
  find "$A/computer-event" "$A/computer-event-payload" "$A/pin-receipts" "$A/file-cas-roots" "$A/file-cas-chunks" \
    -type f ! -newermt "2026-10-20 20:20" 2>/dev/null | shuf -n 200 > "$D/sample.txt"
  { cat "$D/sample.txt"
    echo /var/lib/go-choir/offhost-backup/staging/sqlite/auth.db
    echo /var/lib/go-choir/offhost-backup/staging/sqlite/mail.db; } > "$D/include.txt"
  t1=$(date +%s)
  restic --no-lock restore "$SNAP" --target "$D/r" --include-file "$D/include.txt" >"$D/restore.log" 2>&1 \
    || { echo "restore failed: $(tail -c 400 "$D/restore.log")"; exit 1; }
  t2=$(date +%s)

  same=0; diff=0; missing=0
  while read -r f; do
    if [ ! -f "$D/r$f" ]; then missing=$((missing + 1))
    elif cmp -s "$D/r$f" "$f"; then same=$((same + 1))
    else diff=$((diff + 1)); fi
  done < "$D/sample.txt"
  echo "sample: $(wc -l < "$D/sample.txt") files; identical=$same different=$diff missing=$missing"
  echo "sample by kind: $(sed -E 's#.*/sha256/([^/]+)/.*#\1#' "$D/sample.txt" | sort | uniq -c | tr '\n' ' ')"
  for db in auth mail; do
    f="$D/r/var/lib/go-choir/offhost-backup/staging/sqlite/$db.db"
    printf '%s.db: integrity=%s tables=%s bytes=%s\n' "$db" \
      "$(sqlite3 "$f" 'PRAGMA integrity_check' 2>&1 | head -1)" \
      "$(sqlite3 "$f" "select count(*) from sqlite_master where type='table'" 2>&1)" "$(stat -c %s "$f")"
  done
  echo "timing: pick $((t1 - t0))s restore $((t2 - t1))s"
  ;;
dolt)
  rm -rf "$D/r2"; restore /var/lib/go-choir/corpus-dolt/.choir-backup/corpus
  rm -rf "$D/dolt-restored"; install -d "$D/dolt-restored"
  ( cd "$D/dolt-restored" && export HOME="$D/dolt-restored" && \
    dolt config --global --add user.name drill >/dev/null && dolt config --global --add user.email drill@localhost >/dev/null && \
    dolt backup restore "file://$D/r2" corpus >/dev/null && cd corpus && \
    echo "corpus restored: head $(dolt sql -r csv -q 'select commit_hash from dolt_log limit 1' | tail -1), tables $(dolt sql -r csv -q 'show tables' | tail -n +2 | wc -l), branches $(dolt sql -r csv -q 'select count(*) from dolt_branches' | tail -1)" )
  rm -rf "$D/r2" "$D/dolt-restored"
  restore /var/lib/go-choir/platform-dolt/.choir-backup/platform
  if diff -rq "$D/r2" /var/lib/go-choir/platform-dolt/.choir-backup/platform >"$D/platform-diff.txt" 2>&1; then
    echo "platform backup directory: identical to live ($(find "$D/r2" -type f | wc -l) files)"
  else
    echo "platform backup directory: differs from live in $(wc -l < "$D/platform-diff.txt") entries (a sync after the snapshot would explain it)"; head -5 "$D/platform-diff.txt"
  fi
  ;;
pg)
  M=/var/lib/go-choir/offhost-backup/staging/manifest.json
  rm -rf "$D/r"; restic --no-lock restore "$SNAP" --target "$D/r" --include "$M" >"$D/restore.log" 2>&1 \
    || { echo "manifest restore failed: $(tail -c 400 "$D/restore.log")"; exit 1; }
  ents="$(jq -r '.pg_entries // ""' "$D/r$M")"; echo "manifest pg entries: ${ents:-none}"
  [ -n "$ents" ] || { echo "the manifest records no PostgreSQL dump"; exit 1; }
  rm -rf "$D/r2"; restore /var/lib/go-choir/offhost-backup/staging/pg
  for kv in $ents; do
    db="${kv%%=*}"; want="${kv#*=}"
    got="$(pg_restore --list "$D/r2/$db.dump" 2>/dev/null | grep -vc '^;')"
    if [ "$got" = "$want" ]; then echo "$db: restored dump lists $got entries, as the run recorded"
    else echo "$db: restored dump lists ${got:-0} entries, the run recorded $want"; exit 1; fi
  done
  rm -rf "$D/r2"
  ;;
image)
  VM="${3:?vm id}"
  rm -rf "$D/r2"; restore "/var/lib/go-choir/offhost-backup/staging/vm-images/$VM"
  img="$D/r2/data.img"
  if cmp -s "$img" "/var/lib/go-choir/vm-state/$VM/data.img"; then echo "image: identical to live (stopped computer)"; else echo "image: differs from live"; fi
  echo "image: $(blkid -o value -s TYPE "$img" 2>/dev/null || echo unknown) filesystem; e2fsck -n: $(e2fsck -fn "$img" >"$D/fsck.txt" 2>&1; echo "exit $?"); $(tail -1 "$D/fsck.txt")"
  ;;
snap)
  M=/var/lib/go-choir/offhost-backup/staging/manifest.json
  rm -rf "$D/r"; restic --no-lock restore "$SNAP" --target "$D/r" --include "$M" >"$D/restore.log" 2>&1 \
    || { echo "manifest restore failed: $(tail -c 400 "$D/restore.log")"; exit 1; }
  heads="$(jq -r '.dolt_heads' "$D/r$M")"; echo "manifest dolt heads: $heads"
  rows="$(jq -r '.dolt_rows // ""' "$D/r$M")"; echo "manifest dolt rows: ${rows:-none}"
  has_rows="$(jq -r 'has("dolt_rows")' "$D/r$M")"
  for store in corpus platform; do
    want="$(printf '%s\n' $heads | sed -n "s/^$store=//p")"
    rm -rf "$D/r2" "$D/dolt-restored"; restore "/var/lib/go-choir/offhost-backup/staging/dolt/$store"
    repo="$D/r2"; [ -d "$D/r2/$store/.dolt" ] && repo="$D/r2/$store"
    [ -d "$repo/.dolt" ] || { echo "$store: no .dolt in the restored snapshot"; exit 1; }
    install -d "$D/dolt-restored"
    out="$(cd "$repo" && export HOME="$D/dolt-restored" && \
      dolt config --global --add user.name drill >/dev/null && dolt config --global --add user.email drill@localhost >/dev/null && \
      printf 'head=%s found=%s tables=%s' \
        "$(dolt sql -r csv -q 'select commit_hash from dolt_log limit 1' | tail -1)" \
        "$(dolt sql -r csv -q "select count(*) from dolt_log where commit_hash = '${want:-none}'" | tail -1)" \
        "$(dolt sql -r csv -q 'show tables' | tail -n +2 | wc -l)")" \
      || { echo "$store: dolt could not open the restored snapshot"; exit 1; }
    echo "$store restored and opened: $out (recorded head ${want:-unread})"
    case "$out" in *found=1*) ;; *) echo "$store: the recorded head is not in the restored log"; exit 1 ;; esac
    # Freshness when Store A makes no snapshot commits (PLATFORM_DOLT_SNAPSHOTS=off):
    # the head no longer moves, so the receipt count read before the capture
    # must be in the restore (review round 1 R2 of
    # docs/archive/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md).
    # A manifest from this change on always carries the field and Store A's
    # entry, a number or "unread" (review round 2 R9); only one with no
    # field at all predates the marker.
    want_rows="$(printf '%s\n' $rows | sed -n "s/^$store=//p")"
    n_rows="$(printf '%s\n' $rows | grep -c "^$store=" || true)"
    if [ "$n_rows" -gt 1 ]; then
      echo "$store: the manifest records the receipt count more than once ($(printf '%s' "$want_rows" | tr '\n' ' ')); the restore's freshness is unproven"; exit 1
    elif [ "$want_rows" = unread ]; then
      echo "$store: the run could not read the receipt count; the restore's freshness is unproven"; exit 1
    elif [ -n "$want_rows" ]; then
      # Round 3 F1, F2: the recorded count must be 1 to 15 decimal digits
      # before it is compared, or a negative passes and a malformed one
      # fails as a stale restore.
      case "$want_rows" in *[!0-9]*) bad_rows=1 ;; *) bad_rows=0 ;; esac
      if [ "$bad_rows" = 1 ] || [ "${#want_rows}" -gt 15 ]; then
        echo "$store: the manifest's recorded receipt count '${want_rows:0:40}' is not 1 to 15 decimal digits; the restore's freshness is unproven"; exit 1
      fi
      got_rows="$(cd "$repo" && HOME="$D/dolt-restored" dolt sql -r csv -q 'select count(*) from computer_event_append_receipts' | tail -1)"
      echo "$store receipts: restored ${got_rows:-unread}, recorded $want_rows"
      case "$got_rows" in ''|*[!0-9]*) echo "$store: the receipt count could not be read in the restore"; exit 1 ;; esac
      [ "$got_rows" -ge "$want_rows" ] || { echo "$store: the restore holds fewer receipts than the run recorded"; exit 1; }
    elif [ "$store" = platform ] && [ "$has_rows" = true ]; then
      echo "$store: the manifest has dolt_rows but no Store A count; the restore's freshness is unproven"; exit 1
    elif [ "$store" = platform ]; then
      echo "$store: no receipt count in the manifest (a run before the marker); freshness rests on the head only"
    fi
    rm -rf "$D/r2" "$D/dolt-restored"
  done
  ;;
  *) echo "piece is tape, dolt, image, snap or pg" >&2; exit 2 ;;
esac
