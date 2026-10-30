#!/usr/bin/env bash
# Store A's memory line during the off-host backup's platform Dolt sync.
# No network, no secrets, no real systemd: stub tools record their calls.
#
# Incident (docs/problems/no-off-host-backup-2026-10-20.md section 13 item
# 15): the sync pushed Store A to its 3G MemoryHigh, the server stalled,
# and the liveness probe aborted it. Failure modes of the fix (written
# before the code):
#
#  1. The platform sync runs at the unit's own line: never; the line is
#     raised first.
#  2. The raised line outlives the sync (success, a failed sync, or the
#     script dying): never; it is restored, by the exit trap at the latest.
#  3. The restore writes a different value than the one read before: never.
#  4. The raise is refused and the sync runs anyway: never; the platform
#     sync is skipped and the run is partial.
#  5. A unit other than Store A's is changed, or the corpus sync raises
#     anything: never.
#
# Run: bash scripts/off-host-backup-dolt-memory-test.sh
set -u
export OFFHOST_DOLT=on # these tests cover the Dolt sections, off by default since the cutover
export OFFHOST_DOLT=on  # these tests cover the Dolt sections, off by default since the cutover

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0

setup() { # name, systemctl mode (ok|refuse), dolt mode (ok|fail|die)
  R="$WORK/$1"; mkdir -p "$R/bin" "$R/root/platform-artifacts" "$R/root/secrets" "$R/root/vm-state" \
    "$R/root/platform-dolt/.choir-backup/platform" "$R/root/corpus-dolt/.choir-backup/corpus" "$R/sec"
  : > "$R/calls"
  for t in restic ssh sqlite3; do printf '#!/bin/sh\nexit 0\n' > "$R/bin/$t"; done
  cat > "$R/bin/systemctl" <<EOF
#!/bin/sh
echo "systemctl \$*" >> "$R/calls"
case "\$1" in
  show) echo 3221225472 ;;
  set-property) [ "$2" = ok ] || exit 1 ;;
esac
EOF
  cat > "$R/bin/dolt" <<EOF
#!/bin/sh
case "\$*" in *DOLT_BACKUP*) echo "dolt sync \$*" | grep -o 'port [0-9]*' >> "$R/calls"
  case "$3" in fail) exit 1 ;; die) kill -TERM \$PPID; sleep 1; exit 1 ;; esac ;; esac
exit 0
EOF
  chmod +x "$R/bin/"*
  printf x > "$R/sec/restic-password"; printf x > "$R/sec/rest-password"
}
run() {
  OUT="$(PATH="$R/bin:$PATH" OFFHOST_DATA_ROOT="$R/root" OFFHOST_SECRETS_DIR="$R/sec" \
    OFFHOST_DOLT_ROOT_PASSWORD_FILE="$R/sec/none" OFFHOST_NO_TUNNEL=1 OFFHOST_MIN_FREE_GIB=0 \
    bash "$SCRIPT" backup 2>&1)"; RC=$?
}
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC calls=$(tr '\n' '|' < "$R/calls") out=$(printf '%s' "$OUT" | tail -5)"; fails=$((fails + 1)); fi; }
calls() { tr '\n' '|' < "$R/calls"; }

# 1, 2, 3, 5: raise, platform sync, restore the value read; corpus untouched.
setup ok ok ok; run
# The two KEK source reads (finding B3 of FP9) come after the syncs, when the
# backup resolves its paths; they change no unit.
want="systemctl show -p MemoryHigh --value go-choir-platform-dolt.service|systemctl set-property --runtime go-choir-platform-dolt.service MemoryHigh=8G|port 13306|systemctl set-property --runtime go-choir-platform-dolt.service MemoryHigh=3221225472|port 13307|systemctl show -p LoadCredential -p LoadCredentialEncrypted -p Environment go-choir-corpusd.service|systemctl show -p LoadCredential -p LoadCredentialEncrypted -p Environment go-choir-checkpointd.service|"
check "raise, platform sync, restore, then corpus" "$([ "$(calls)" = "$want" ]; echo $?)"

# 2: a failed platform sync still restores.
setup fail ok fail; run
check "a failed sync still restores the line" "$(calls | grep -q 'MemoryHigh=3221225472'; echo $?)"

# 2: the script killed during the sync still restores (exit trap).
setup die ok die; run
check "a killed script still restores the line" "$(calls | grep -q 'MemoryHigh=3221225472'; echo $?)"

# 4: the raise refused: no platform sync, partial, corpus still synced.
setup refuse refuse ok; run
check "a refused raise skips the platform sync" "$(! calls | grep -q 'port 13306'; echo $?)"
check "a refused raise still syncs the corpus" "$(calls | grep -q 'port 13307'; echo $?)"
check "a refused raise says why" "$(printf '%s' "$OUT" | grep -q 'platform Dolt sync skipped'; echo $?)"
check "a refused raise ends partial" "$([ "$RC" = 3 ]; echo $?)"

[ "$fails" = 0 ] && echo "all dolt memory checks passed" || { echo "$fails failed"; exit 1; }
