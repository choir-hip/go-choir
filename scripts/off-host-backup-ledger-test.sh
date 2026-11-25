#!/usr/bin/env bash
# The account-deletion ledger travels with the daily backup (round 3, B2 of the
# account-deletion review). The ledger (account-deletion-ledger.jsonl, beside
# auth.db) holds the hash of every account whose purge began or finished. A
# restore after a total loss of the host has only what the off-host repository
# holds, so a ledger that stays on the host is lost with it and every purged
# account comes back unnoticed.
#
# Failure modes (written before the change):
#  1. The ledger is not in what restic is given.
#  2. It is copied BEFORE the auth.db backup: a deletion begun between the two
#     is in auth.db's snapshot but not in the ledger copy. The ledger copy comes
#     after auth.db's.
#  3. A ledger that does not exist yet (no deletion ever) fails the run: it must
#     not; the run says so and goes on.
#  4. A ledger that exists but cannot be copied is passed over silently: the run
#     is partial (exit 3) and the status names it.
#
# Run: bash scripts/off-host-backup-ledger-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0

setup() { # name
  R="$WORK/$1"; local root="$R/root"
  mkdir -p "$R/bin" "$R/log" "$R/sec" "$root/platform-artifacts" "$root/secrets" "$root/vm-state" "$root/auth" "$root/mail"
  for t in ssh; do printf '#!/bin/sh\nexit 0\n' > "$R/bin/$t"; done
  printf '#!/bin/sh\ncat >/dev/null\necho "[]"\n' > "$R/bin/curl"
  printf '#!/bin/sh\nexit 0\n' > "$R/bin/systemctl"
  # sqlite3 ".backup 'path'": make the file and note the order in $LOG/order.
  cat > "$R/bin/sqlite3" <<EOF
#!/usr/bin/env bash
dest="\${2#.backup \'}"; dest="\${dest%\'}"
printf 'sqlite3 %s\n' "\$(basename "\$dest")" >> "$R/log/order"
printf 'SQLITE\n' > "\$dest"
EOF
  cat > "$R/bin/pg_dump" <<EOF
#!/usr/bin/env bash
out=""; while [ "\$#" -gt 0 ]; do case "\$1" in -f) out="\$2"; shift ;; esac; shift; done
printf 'PGDMP\n' > "\$out"
EOF
  printf '#!/bin/sh\necho "; a"\necho "1; 0 0 SCHEMA - public choir"\n' > "$R/bin/pg_restore"
  cat > "$R/bin/restic" <<EOF
#!/usr/bin/env bash
case " \$* " in *" backup "*) ;; *) exit 0 ;; esac
ls -R "$root/offhost-backup/staging/sqlite" > "$R/log/staged-at-backup" 2>&1
[ -f "$root/offhost-backup/staging/sqlite/account-deletion-ledger.jsonl" ] \
  && cp "$root/offhost-backup/staging/sqlite/account-deletion-ledger.jsonl" "$R/log/ledger-at-backup"
echo '{"message_type":"summary","snapshot_id":"s1","data_added":5}'
exit 0
EOF
  chmod +x "$R/bin/"*
  printf x > "$R/sec/restic-password"; printf x > "$R/sec/rest-password"; printf pw > "$R/sec/pg-app-password"
  printf 'SQLITE\n' > "$root/auth/auth.db"; printf 'SQLITE\n' > "$root/mail/mail.db"
  ST="$root/offhost-backup/status.json"
  STAGE="$root/offhost-backup/staging"
  LEDGER="$root/auth/account-deletion-ledger.jsonl"
}
run() {
  OUT="$(env PATH="$R/bin:$PATH" OFFHOST_DATA_ROOT="$R/root" OFFHOST_SECRETS_DIR="$R/sec" \
    OFFHOST_PG_PASSWORD_FILE="$R/sec/pg-app-password" OFFHOST_NO_TUNNEL=1 OFFHOST_MIN_FREE_GIB=0 \
    bash "$SCRIPT" backup 2>&1)"; RC=$?
}
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC out=$(printf '%s' "$OUT" | tail -5)"; fails=$((fails + 1)); fi; }

# 1, 2: a ledger is staged for restic, after auth.db.
setup with
printf '{"user_hash":"aaaa","job_id":"job-1"}\n{"user_hash":"bbbb","job_id":"job-2"}\n' > "$LEDGER"
run
check "a clean run with a ledger exits 0" "$([ "$RC" = 0 ]; echo $?)"
check "the ledger is in staging when restic runs" "$(grep -q account-deletion-ledger.jsonl "$R/log/staged-at-backup"; echo $?)"
check "the staged ledger is the live ledger, line for line" "$(cmp -s "$LEDGER" "$R/log/ledger-at-backup"; echo $?)"
check "the ledger is copied after auth.db is backed up" \
  "$([ "$(printf '%s\n' "$OUT" | grep -n 'sqlite backup auth.db ok' | head -1 | cut -d: -f1)" -lt "$(printf '%s\n' "$OUT" | grep -n 'deletion ledger copied' | head -1 | cut -d: -f1)" ]; echo $?)"
check "the ledger line count is logged (hashes only, never a user)" "$(grep -q 'ledger copied: 2 lines' <<<"$OUT"; echo $?)"

# 3: no ledger yet.
setup without; run
check "no ledger yet: the run still exits 0" "$([ "$RC" = 0 ]; echo $?)"
check "no ledger yet: the run says so" "$(grep -q 'no deletion ledger yet' <<<"$OUT"; echo $?)"

# 4: a ledger that cannot be read.
setup unreadable
printf '{"user_hash":"aaaa","job_id":"job-1"}\n' > "$LEDGER"; chmod 000 "$LEDGER"
run
if [ "$(id -u)" = 0 ]; then echo "skip an unreadable ledger (running as root)"; else
  check "an unreadable ledger makes the run partial" "$([ "$RC" = 3 ]; echo $?)"
  check "the error names the ledger" "$(grep -q 'deletion ledger' <<<"$OUT"; echo $?)"
fi
chmod 600 "$LEDGER" 2>/dev/null

[ "$fails" = 0 ] && echo "all ok" || { echo "$fails failed"; exit 1; }
