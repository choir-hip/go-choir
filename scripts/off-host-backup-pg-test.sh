#!/usr/bin/env bash
# The PostgreSQL section of scripts/off-host-backup.sh (data system cutover,
# phase 2 item I7). No network, no secrets, no PostgreSQL: pg_dump, pg_restore,
# restic and the rest are fakes on PATH that record what they were given.
#
# Failure modes (written before the section):
#
#  1. A database's dump is not in what restic is given: never; the dumps sit
#     under $STAGING/pg, which is in the path list, and the manifest records
#     one entry count per database.
#  2. pg_dump fails and the run still ends ok (a night with no database in
#     the snapshot reads as fresh): never; the run exits 3 (partial), status
#     says partial and names the database, and freshness fails.
#  3. A failed or unreadable dump is left in staging and uploaded as if it
#     were good: never; a dump whose pg_dump failed, whose pg_restore --list
#     failed, or whose listing is empty is removed, and a database the other
#     dumps succeeded for is still dumped.
#  4. The password appears on any command line (pg_dump's, pg_restore's, or
#     any other fake's): never; it reaches pg_dump only through a PGPASSFILE
#     of mode 0600, which holds it and is gone when the run is over.
#  5. The dumps stay in staging after the run (3 GB a night on a disk that
#     also holds the guest images): never; removed at the end, success or not.
#  6. A missing pg-app-password credential is skipped silently: never; every
#     database counts as failed and the run is partial.
#  7. `plan` does not name the dumps, so the owner cannot see what a run
#     would back up: never.
#  8. The credential in $CREDENTIALS_DIRECTORY is ignored in favour of the
#     owner-shell path: never; the credential wins.
#
# Run: bash scripts/off-host-backup-pg-test.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$HERE/off-host-backup.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0
PW='s3cr3t-PW:with\odd'  # colon and backslash exercise the PGPASSFILE escaping

setup() { # name
  R="$WORK/$1"; local root="$R/root"
  mkdir -p "$R/bin" "$R/log" "$R/sec" "$root/platform-artifacts" "$root/secrets" "$root/vm-state"
  for t in ssh sqlite3; do printf '#!/bin/sh\nexit 0\n' > "$R/bin/$t"; done
  printf '#!/bin/sh\ncat >/dev/null\necho "[]"\n' > "$R/bin/curl"
  printf '#!/bin/sh\nexit 0\n' > "$R/bin/systemctl"
  # Every fake appends its argv, one line per call, to $LOG/argv.
  cat > "$R/bin/pg_dump" <<EOF
#!/usr/bin/env bash
printf 'pg_dump %s\n' "\$*" >> "$R/log/argv"
db=""; out=""
while [ "\$#" -gt 0 ]; do
  case "\$1" in -d) db="\$2"; shift ;; -f) out="\$2"; shift ;; esac
  shift
done
case " \${FAKE_PG_FAIL:-} " in *" \$db "*) echo "pg_dump: error: connection to server failed" >&2; exit 1 ;; esac
# the password file must exist, be 0600 and hold the password (escaped)
if [ -r "\${PGPASSFILE:-/nonexistent}" ]; then
  printf '%s\n' "\$PGPASSFILE" > "$R/log/pgpassfile"
  stat -c %a "\$PGPASSFILE" > "$R/log/pgpass-mode" 2>/dev/null || stat -f %Lp "\$PGPASSFILE" > "$R/log/pgpass-mode"
  cat "\$PGPASSFILE" > "$R/log/pgpass-content"
else
  echo "pg_dump: no PGPASSFILE" >&2; exit 1
fi
printf 'PGDMP-%s\n' "\$db" > "\$out"
EOF
  cat > "$R/bin/pg_restore" <<EOF
#!/usr/bin/env bash
printf 'pg_restore %s\n' "\$*" >> "$R/log/argv"
case "\$*" in *"\${FAKE_RESTORE_FAIL:-@none@}"*) echo "pg_restore: error: bad archive" >&2; exit 1 ;; esac
case "\$*" in *"\${FAKE_RESTORE_EMPTY:-@none@}"*) echo "; Archive created"; exit 0 ;; esac
echo "; Archive created"
echo "1; 0 0 SCHEMA - public choir"
echo "2; 0 0 TABLE public events choir"
echo "3; 0 0 TABLE public refs choir"
EOF
  # restic: record argv and the staged tree at the time of the call.
  cat > "$R/bin/restic" <<EOF
#!/usr/bin/env bash
printf 'restic %s\n' "\$*" >> "$R/log/argv"
case " \$* " in *" backup "*) ;; *) exit 0 ;; esac
ls -R "$root/offhost-backup/staging/pg" > "$R/log/staged-at-backup" 2>&1
cat "$root/offhost-backup/staging/manifest.json" > "$R/log/manifest-at-backup" 2>&1
echo '{"message_type":"summary","snapshot_id":"s1","data_added":5}'
exit 0
EOF
  chmod +x "$R/bin/"*
  printf x > "$R/sec/restic-password"; printf x > "$R/sec/rest-password"
  printf '%s' "$PW" > "$R/sec/pg-app-password"
  ST="$root/offhost-backup/status.json"
  STAGE="$root/offhost-backup/staging"
}
run() { # subcommand; extra env as VAR=value arguments after
  local sub="$1"; shift
  OUT="$(env PATH="$R/bin:$PATH" OFFHOST_DATA_ROOT="$R/root" OFFHOST_SECRETS_DIR="$R/sec" \
    OFFHOST_PG_PASSWORD_FILE="$R/sec/pg-app-password" OFFHOST_NO_TUNNEL=1 OFFHOST_MIN_FREE_GIB=0 \
    "$@" bash "$SCRIPT" "$sub" 2>&1)"; RC=$?
}
field() { jq -r ".$1 // \"\"" "$ST"; }
check() { if [ "$2" = 0 ]; then echo "ok   $1"; else echo "FAIL $1"; echo "     rc=$RC status=$(cat "$ST" 2>/dev/null) out=$(printf '%s' "$OUT" | tail -4)"; fails=$((fails + 1)); fi; }

# 1, 4, 5: both dumps succeed.
setup ok; run backup
check "a clean run exits 0" "$([ "$RC" = 0 ]; echo $?)"
check "pg_dump ran once per database, custom format, level 6, over loopback as choir" \
  "$([ "$(grep -c '^pg_dump -w ' "$R/log/argv")" = 2 ] && grep -q '^pg_dump -w -h 127.0.0.1 -p 5432 -U choir -d control -Fc -Z 0 -f .*/staging/pg/control.dump$' "$R/log/argv" && grep -q -- '-d graph -Fc' "$R/log/argv"; echo $?)"
check "pg_restore --list ran on each dump" "$([ "$(grep -c '^pg_restore --list ' "$R/log/argv")" = 2 ]; echo $?)"
check "both dumps were in staging when restic ran" "$(grep -q control.dump "$R/log/staged-at-backup" && grep -q graph.dump "$R/log/staged-at-backup"; echo $?)"
check "restic was given the staging directory (which holds the dumps)" "$(grep '^restic .* backup ' "$R/log/argv" | grep -q "$STAGE"; echo $?)"
check "the manifest records an entry count per database" "$([ "$(jq -r .pg_entries "$R/log/manifest-at-backup")" = "control=3 graph=3" ]; echo $?)"
check "the manifest records no failure" "$([ -z "$(jq -r .pg_failed "$R/log/manifest-at-backup")" ]; echo $?)"
check "status records the entry counts" "$([ "$(field last_pg_entries)" = "control=3 graph=3" ]; echo $?)"
check "status says ok with no cause" "$([ "$(field last_attempt_result)" = ok ] && [ -z "$(field last_attempt_cause)" ]; echo $?)"
check "the password is in no argv" "$(! grep -qF -e 's3cr3t' "$R/log/argv"; echo $?)"
check "the password file was mode 600" "$([ "$(cat "$R/log/pgpass-mode")" = 600 ]; echo $?)"
check "the password file holds the escaped password for the host, port and user" \
  "$([ "$(cat "$R/log/pgpass-content")" = '127.0.0.1:5432:*:choir:s3cr3t-PW\:with\\odd' ]; echo $?)"
check "the password file is gone after the run" "$([ ! -e "$(cat "$R/log/pgpassfile")" ] && [ -z "$(ls "$R/root/offhost-backup/run" | grep '^pgpass')" ]; echo $?)"
check "the password is in no log line" "$(! printf '%s' "$OUT" | grep -qF 's3cr3t'; echo $?)"
check "the dumps are removed from staging after the run" "$([ ! -e "$STAGE/pg" ]; echo $?)"
run freshness
check "a clean run is fresh" "$([ "$RC" = 0 ]; echo $?)"

# 2, 3, 4, 5: pg_dump fails for one database.
setup dumpfail; run backup FAKE_PG_FAIL=graph
check "a failed pg_dump makes the run exit 3 (partial)" "$([ "$RC" = 3 ]; echo $?)"
check "status says partial and names the database" "$([ "$(field last_attempt_result)" = partial ] && field last_attempt_cause | grep -q graph; echo $?)"
check "the error is logged with the database name" "$(printf '%s' "$OUT" | grep -q 'pg_dump of graph failed'; echo $?)"
check "the manifest names the failed database" "$([ "$(jq -r .pg_failed "$R/log/manifest-at-backup")" = graph ]; echo $?)"
check "the other database was still dumped and listed" "$(grep -q control.dump "$R/log/staged-at-backup" && ! grep -q graph.dump "$R/log/staged-at-backup"; echo $?)"
check "no listing was attempted for the failed dump" "$([ "$(grep -c '^pg_restore --list ' "$R/log/argv")" = 1 ]; echo $?)"
check "the password is still in no argv" "$(! grep -qF 's3cr3t' "$R/log/argv"; echo $?)"
check "the dump of the other database is removed after the run" "$([ ! -e "$STAGE/pg" ]; echo $?)"
run freshness
check "freshness fails after a failed dump" "$([ "$RC" != 0 ]; echo $?)"

# 3: pg_restore --list fails: the dump is not trusted and is not uploaded.
setup listfail; run backup FAKE_RESTORE_FAIL=control.dump
check "an unlistable dump makes the run partial" "$([ "$RC" = 3 ] && [ "$(field last_attempt_result)" = partial ]; echo $?)"
check "an unlistable dump is not in staging when restic runs" "$(! grep -q control.dump "$R/log/staged-at-backup"; echo $?)"

# 3: a dump that lists no entries is not trusted.
setup listempty; run backup FAKE_RESTORE_EMPTY=graph.dump
check "an empty listing makes the run partial" "$([ "$RC" = 3 ] && field last_attempt_cause | grep -q graph; echo $?)"
check "an empty-listing dump is not in staging when restic runs" "$(! grep -q graph.dump "$R/log/staged-at-backup"; echo $?)"

# 6: no credential.
setup nocred; rm -f "$R/sec/pg-app-password"; run backup
check "a missing pg-app-password makes the run partial, naming the cause" "$([ "$RC" = 3 ] && field last_attempt_cause | grep -q control; echo $?)"
check "no pg_dump ran without a password" "$(! grep -q '^pg_dump -w ' "$R/log/argv"; echo $?)"

# 8: the systemd credential wins over the owner-shell path.
setup credwins; mkdir -p "$R/cred"; printf '%s' 'from-credential' > "$R/cred/pg-app-password"
run backup CREDENTIALS_DIRECTORY="$R/cred"
check "the credential directory's password is the one used" "$([ "$(cat "$R/log/pgpass-content")" = '127.0.0.1:5432:*:choir:from-credential' ]; echo $?)"

# configurable list
setup onedb; run backup OFFHOST_PG_DATABASES=graph
check "OFFHOST_PG_DATABASES narrows the dumps" "$([ "$RC" = 0 ] && [ "$(grep -c '^pg_dump -w ' "$R/log/argv")" = 1 ] && grep -q -- '-d graph ' "$R/log/argv"; echo $?)"

# 7: plan lists the dumps.
setup plan; run plan
check "plan lists the control dump" "$(printf '%s' "$OUT" | grep -q 'control -> .*/staging/pg/control.dump'; echo $?)"
check "plan lists the graph dump" "$(printf '%s' "$OUT" | grep -q 'graph -> .*/staging/pg/graph.dump'; echo $?)"
check "plan names no Dolt path" "$(! printf '%s' "$OUT" | grep -qi 'dolt'; echo $?)"

[ "$fails" = 0 ] && echo "all PostgreSQL backup checks passed" || { echo "$fails failed"; exit 1; }
