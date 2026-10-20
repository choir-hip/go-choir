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
  cat > "$R/bin/restic" <<EOF
#!/bin/sh
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
case "\$*" in *DOLT_BACKUP*) echo "sync \$*" | grep -o 'port [0-9]*' >> "$R/calls" ;; *commit_hash*) echo commit_hash; echo h-\$\$ ;; esac
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
run() {
  OUT="$(PATH="$R/bin:$PATH" OFFHOST_DATA_ROOT="$R/root" OFFHOST_SECRETS_DIR="$R/sec" \
    OFFHOST_DOLT_ROOT_PASSWORD_FILE="$R/sec/none" OFFHOST_NO_TUNNEL=1 OFFHOST_MIN_FREE_GIB=0 \
    bash "$SCRIPT" backup 2>&1)"; RC=$?
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

# 2: neither is a subvolume: the sync, as before.
setup neither "" ok; run
check "non-subvolume stores fall back to the sync" "$(has 'port 13306' && has 'port 13307' && ! grep -q '^snapshot' "$R/calls"; echo $?)"

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

[ "$fails" = 0 ] && echo "all dolt snapshot checks passed" || { echo "$fails failed"; exit 1; }
