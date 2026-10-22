#!/usr/bin/env bash
# The Dolt stores captured as btrfs snapshots of their data directories.
# No network, no secrets, no real btrfs or systemd: stub tools record calls.
#
# Design: docs/problems/no-off-host-backup-2026-10-20.md section 13 item 16.
# Failure modes (written before the code):
#
#  1. A store whose data directory is a subvolume is synced server-side
#     (DOLT_BACKUP, the memory raise): never; it is snapshotted.
#  2. A store whose data directory is not a subvolume is snapshotted or
#     copied file by file: never; it falls back to the sync.
#  3. A snapshot outlives the run, on success, failure or a kill: never;
#     the exit trap deletes it.
#  4. A snapshot left by a killed run blocks the next run: never; it is
#     deleted before the new one is taken.
#  5. A failed snapshot is silent: never; the run is partial.
#  6. The corpus snapshot's nested .choir-backup is uploaded again: never;
#     restic is told to exclude it.
#  7. The daily run walks every tree with `restic check` (item 18: 9.2
#     million files in three flat directories put it past the memory line
#     for hours): never; the structural check is the weekly verify's.
#  8. The weekly verify downloads every tree pack into a throwaway cache:
#     never; it checks with the run's cache.
# 10. The daily run walks every snapshot's trees to size the repository
#     (`restic stats --mode raw-data`, 10 GB and 7 minutes on run 5):
#     never; the size is the sum of the pack sizes the rest-server lists.
# 11. With Store A's snapshot commits off (PLATFORM_DOLT_SNAPSHOTS=off) the
#     newest dolt_log hash is frozen, so a manifest holding only heads lets a
#     restore of any age pass the drill: never; the manifest also records
#     Store A's event receipt count, read before the capture, on both paths.
#     (docs/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md
#     review round 1 R2.)
# 12. The count cannot be read at backup time and the manifest silently
#     lacks it, so the drill takes the run for one from before the marker
#     and passes on the frozen head alone: never; the backup records
#     platform=unread and the drill fails on it, and on a manifest that has
#     the dolt_rows field but no Store A entry. A manifest with no dolt_rows
#     field at all (written before the marker) still passes on the head.
#     (Same record, review round 2 R9.)
#  9. A lock left by a killed run (its tunnel dies with it, so restic cannot
#     delete the lock) blocks the next run: never; the run removes stale
#     locks before it opens the repository.
#
# Run: bash scripts/off-host-backup-dolt-snapshot-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0

setup() { # name, subvolumes (space separated: platform corpus), snapshot mode (ok|fail|die)
  R="$WORK/$1"; local root="$R/root"
  mkdir -p "$R/bin" "$root/platform-artifacts" "$root/secrets" "$root/vm-state" \
    "$root/platform-dolt/platform/.dolt" "$root/corpus-dolt/.dolt" "$root/corpus-dolt/.choir-backup/corpus" \
    "$root/platform-dolt/.choir-backup/platform" "$R/sec"
  : > "$R/calls"
  local subs=""
  case " $2 " in *" platform "*) subs="$subs $root/platform-dolt/platform" ;; esac
  case " $2 " in *" corpus "*) subs="$subs $root/corpus-dolt" ;; esac
  for t in ssh sqlite3; do printf '#!/bin/sh\nexit 0\n' > "$R/bin/$t"; done
  cat > "$R/bin/curl" <<EOF
#!/bin/sh
cat >/dev/null
echo "curl \$*" | grep -o 'data/' >> "$R/calls"
echo '[{"name":"a","size":1000},{"name":"b","size":2345}]'
EOF
  cat > "$R/bin/restic" <<EOF
#!/bin/sh
case " \$* " in *" stats "*) echo stats >> "$R/calls" ;; esac
case " \$* " in *" unlock "*) echo unlock >> "$R/calls" ;; *" cat config "*) echo "cat config" >> "$R/calls" ;; esac
case " \$* " in *" check "*) echo "restic check \$*" | grep -o -- 'check.*' >> "$R/calls" ;; esac
case "\$*" in *" backup "*) echo "restic backup \$*" | grep -o -- '--exclude [^ ]*choir-backup[^ ]*' | sed 's#.*/staging/#--exclude staging/#' >> "$R/calls" ;; esac
exit 0
EOF
  cat > "$R/bin/systemctl" <<EOF
#!/bin/sh
echo "systemctl \$1" >> "$R/calls"
[ "\$1" = show ] && echo 3221225472
exit 0
EOF
  cat > "$R/bin/dolt" <<EOF
#!/bin/sh
case "\$*" in *DOLT_BACKUP*) echo "sync \$*" | grep -o 'port [0-9]*' >> "$R/calls" ;; *commit_hash*) echo commit_hash; echo h-\$\$ ;; *computer_event_append_receipts*) echo "count(*)"; echo 41 ;; esac
exit 0
EOF
  cat > "$R/bin/btrfs" <<EOF
#!/bin/sh
case "\$1 \$2" in
  "subvolume show") for s in $subs; do [ "\$3" = "\$s" ] && exit 0; done; exit 1 ;;
  "subvolume snapshot")
    dst="\$5"; echo "snapshot \$(basename "\$dst")" >> "$R/calls"
    [ -e "\$dst" ] && { echo "snapshot over existing \$(basename "\$dst")" >> "$R/calls"; exit 1; }
    case "$3" in fail) exit 1 ;; die) mkdir -p "\$dst"; kill -TERM \$PPID; sleep 1; exit 1 ;; esac
    mkdir -p "\$dst"; exit 0 ;;
  "subvolume delete") echo "delete \$(basename "\$3")" >> "$R/calls"; rm -rf "\$3"; exit 0 ;;
esac
exit 1
EOF
  chmod +x "$R/bin/"*
  printf x > "$R/sec/restic-password"; printf x > "$R/sec/rest-password"
}
run() { # optional subcommand, default backup
  OUT="$(PATH="$R/bin:$PATH" OFFHOST_DATA_ROOT="$R/root" OFFHOST_SECRETS_DIR="$R/sec" \
    OFFHOST_DOLT_ROOT_PASSWORD_FILE="$R/sec/none" OFFHOST_NO_TUNNEL=1 OFFHOST_MIN_FREE_GIB=0 \
    bash "$SCRIPT" "${1:-backup}" 2>&1)"; RC=$?
}
calls() { tr '\n' '|' < "$R/calls"; }
has() { grep -qx -- "$1" "$R/calls"; }
left() { ls "$R/root/offhost-backup/staging/dolt" 2>/dev/null | tr '\n' ' '; }
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC calls=$(calls) left=$(left) out=$(printf '%s' "$OUT" | tail -4)"; fails=$((fails + 1)); fi; }

# 1, 3, 6: both subvolumes: snapshots, no sync, no raise, deleted, nested backup excluded.
setup both "platform corpus" ok; run
check "subvolume stores are snapshotted" "$(has 'snapshot platform' && has 'snapshot corpus'; echo $?)"
check "no server-side sync and no memory raise" "$(! grep -q 'port 1330\|systemctl set-property' "$R/calls"; echo $?)"
check "snapshots are deleted at exit" "$([ -z "$(left)" ] && has 'delete platform' && has 'delete corpus'; echo $?)"
check "the corpus snapshot's .choir-backup is excluded" "$(has '--exclude staging/dolt/corpus/.choir-backup'; echo $?)"
check "a clean run is ok" "$([ "$RC" = 0 ]; echo $?)"
check "stale locks are removed before the repository is opened" "$(calls | grep -q '^unlock|cat config|'; echo $?)"
check "the repository is sized from the pack list, not stats" "$(has 'data/' && ! has stats && [ "$(jq -r .repo_raw_bytes "$R/root/offhost-backup/status.json")" = 3345 ]; echo $?)"
check "a snapshot run records Store A's receipt count" "$(jq -r .dolt_rows "$R/root/offhost-backup/staging/manifest.json" 2>/dev/null | grep -qx 'platform=41'; echo $?)"
check "the daily run does not walk the trees" "$(! grep -q '^check' "$R/calls"; echo $?)"

# 8: the weekly verify checks with the run's cache.
: > "$R/calls"; run verify
check "the weekly verify checks a data subset with the cache" "$(grep -q -- '^check.*--with-cache.*--read-data-subset' "$R/calls"; echo $?)"

# 2: neither is a subvolume: the sync, as before.
setup neither "" ok; run
check "non-subvolume stores fall back to the sync" "$(has 'port 13306' && has 'port 13307' && ! grep -q '^snapshot' "$R/calls"; echo $?)"
check "a sync run records Store A's receipt count" "$(jq -r .dolt_rows "$R/root/offhost-backup/staging/manifest.json" 2>/dev/null | grep -qx 'platform=41'; echo $?)"

# 2: mid-migration, Store A moved and Store B not.
setup mixed "platform" ok; run
check "a mixed host snapshots one and syncs the other" "$(has 'snapshot platform' && has 'port 13307' && ! has 'port 13306'; echo $?)"

# 4: a snapshot left by a killed run is deleted first.
setup stale "platform corpus" ok
mkdir -p "$R/root/offhost-backup/staging/dolt/platform"; run
check "a stale snapshot is deleted before a new one" "$(! grep -q 'snapshot over existing' "$R/calls" && [ -z "$(left)" ]; echo $?)"

# 5: a failed snapshot is partial and says so.
setup fail "platform corpus" fail; run
check "a failed snapshot ends partial" "$([ "$RC" = 3 ]; echo $?)"
check "a failed snapshot is named" "$(printf '%s' "$OUT" | grep -q 'snapshot of platform'; echo $?)"

# 3: killed while snapshotting: nothing left behind.
setup die "platform corpus" die; run
check "a killed run leaves no snapshot" "$([ -z "$(left)" ]; echo $?)"

# 12: an unread receipt count is recorded as such.
setup unread "platform corpus" ok
sed -i.bak 's/echo 41/echo ERROR/' "$R/bin/dolt"; rm -f "$R/bin/dolt.bak"; run
check "an unread receipt count is recorded as platform=unread" "$(jq -r .dolt_rows "$R/root/offhost-backup/staging/manifest.json" 2>/dev/null | grep -qx 'platform=unread'; echo $?)"

# 11, 12: the restore drill's snap piece against stub restic and dolt.
drill() { # name, manifest json, restored receipt count
  R="$WORK/drill-$1"; mkdir -p "$R/bin" "$R/sec" "$R/d"; : > "$R/calls"
  printf x > "$R/sec/rest-password"; printf '%s\n' "$2" > "$R/manifest.json"
  printf '#!/bin/sh\nexit 0\n' > "$R/bin/ssh"
  cat > "$R/bin/restic" <<EOF
#!/bin/sh
t=""; inc=""; prev=""
for a; do case "\$prev" in --target) t="\$a" ;; --include) inc="\$a" ;; esac; prev="\$a"; done
if [ -n "\$inc" ]; then mkdir -p "\$t\$(dirname "\$inc")"; cp "$R/manifest.json" "\$t\$inc"; else mkdir -p "\$t/.dolt"; fi
exit 0
EOF
  cat > "$R/bin/dolt" <<EOF
#!/bin/sh
case "\$*" in
  *"commit_hash from dolt_log"*) echo commit_hash; echo h1 ;;
  *"where commit_hash"*) echo 'count(*)'; echo 1 ;;
  *"show tables"*) echo Tables; echo t1 ;;
  *computer_event_append_receipts*) echo 'count(*)'; echo $3 ;;
esac
exit 0
EOF
  chmod +x "$R/bin/"*
  OUT="$(PATH="$R/bin:$PATH" OFFHOST_SECRETS_DIR="$R/sec" OFFHOST_DRILL_DIR="$R/d" \
    bash "$HERE/off-host-restore-drill.sh" s1 snap 2>&1)"; RC=$?
}
drill fresh '{"dolt_heads":"corpus=h1 platform=h1","dolt_rows":"platform=41"}' 41
check "the drill passes a restore holding the recorded receipts" "$([ "$RC" = 0 ]; echo $?)"
drill stale '{"dolt_heads":"corpus=h1 platform=h1","dolt_rows":"platform=41"}' 40
check "the drill fails a restore with fewer receipts than recorded" "$([ "$RC" = 1 ]; echo $?)"
drill unread '{"dolt_heads":"corpus=h1 platform=h1","dolt_rows":"platform=unread"}' 41
check "the drill fails a manifest whose receipt count was unread" "$([ "$RC" = 1 ] && printf '%s' "$OUT" | grep -q 'could not read'; echo $?)"
drill empty '{"dolt_heads":"corpus=h1 platform=h1","dolt_rows":""}' 41
check "the drill fails a marker manifest with no Store A count" "$([ "$RC" = 1 ]; echo $?)"
drill premarker '{"dolt_heads":"corpus=h1 platform=h1"}' 41
check "a manifest from before the marker still passes on the head" "$([ "$RC" = 0 ] && printf '%s' "$OUT" | grep -q 'before the marker'; echo $?)"

[ "$fails" = 0 ] && echo "all dolt snapshot checks passed" || { echo "$fails failed"; exit 1; }
