#!/usr/bin/env bash
# The daily log age job (scripts/node-b-log-age), on a scratch folder with a
# stub journalctl: nothing real is touched.
#
# Failure modes:
#  A1 it runs with no limit, or a limit under 7 or over 28 days.
#  A2 the journal is not closed (rotated) before it is trimmed, so the open
#     file's entries wait for traffic; or the trim does not use the limit.
#  A3 a left-over access log is judged by when it was last written, not by
#     its OLDEST entry: a file whose first entry is past the limit stays.
#  A4 a file whose first entry cannot be read stays (it must go: deleting
#     early never breaks the promise, keeping late does).
#  A5 a file whose first entry is inside the limit is deleted, or something
#     that is not a regular file in that folder is touched.
#  A6 the last-login records stay.
#  A7 a named dead file stays, or a file not named is removed.
#  A8 a failed delete is not reported (exit 0).
#  A9 the first entry's time is taken from a later "ts": in the line (a
#     request header can carry one), so a young file is deleted or an old
#     one kept; or a first entry dated in the future keeps a file for ever.
#  A10 a file in the access-log folder that is not one of our access logs is
#     deleted, or is left there without the job failing (nothing ages it).
#  A11 the access-log folder exists and cannot be read, and the job says all
#     is well.
#  A12 a second journal folder (another machine id, a namespace) is on disk,
#     which this trim does not reach, and the job says all is well.
#  A13 a closed journal file whose first entry is past the limit is still
#     there after the trim, and the job says all is well.
#  A15 a rotated login-record file past the limit stays (logrotate is another
#     unit: this job's success must mean the whole bound), or a young one goes.
#  A16 a live login-record file that logrotate has not rotated for over two
#     days (or has never rotated, and is not empty) passes.
#  A17 a folder, a link or a hidden file in the access-log folder is skipped
#     in silence (it must fail the job, and never be deleted).
#  A18 a journal file set aside as damaged (.journal~) past the limit stays,
#     or a young one is deleted.
#  A19 an open per-user journal passes unseen.
#  A20 the dry run deletes something or calls journalctl, or does not list
#     what a real run would delete.
#  A21 with a second journal folder the closed-file check is skipped.
#  A22 run with no settings at all, the job does not use the production
#     paths and limit (the dry run the owner approves would list nothing).
#  A23 LOG_AGE_DRY_RUN set to anything but 1 is taken as a dry run, or the
#     dry run's count line reads like a real run's.
#  A24 before logrotate's first run the dry run fails on the stale rotation
#     record instead of reporting it; or it does not list what the journal
#     trim would delete.
#  A25 a file under a login record's name that is not a rotation (a lock, a
#     backup) is deleted as one; or it, a folder or a link passes unseen.
#  A26 another login store (wtmpdb, faillog, tallylog) passes unseen.
#  A27 an access-log file a process still holds open is unlinked.
#  A14 any failure is not printed at error priority (the "<3>" prefix the
#     journal reads), so `journalctl -p err` does not show it.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
job="$HERE/node-b-log-age"
pass=0; fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); echo "FAIL: $1"; }
root="$(mktemp -d)"; trap 'chmod -R u+w "$root" 2>/dev/null; rm -rf "$root"' EXIT
mkdir -p "$root/bin" "$root/caddy" "$root/var" "$root/dead" "$root/journal/0123456789abcdef0123456789abcdef"
jdir="$root/journal/0123456789abcdef0123456789abcdef"
jname() { printf 'system@%s-%016x-%016x.journal' 4b6e7d5a52a146ff8e6bc012dcebf5e1 "$2" "$(( $1 * 1000000 ))"; } # first-entry time, sequence
printf '#!/usr/bin/env bash\necho "$*" >>"%s/journalctl.calls"\n' "$root" >"$root/bin/journalctl"; chmod +x "$root/bin/journalctl"
now=$(date +%s)
line() { printf '{"level":"info","ts":%s.123,"logger":"http.log.access","msg":"handled request"}\n' "$1"; }
run() { env PATH="$root/bin:$PATH" LOG_MAX_DAYS="${DAYS-21}" CADDY_LOG_DIR="$root/caddy" JOURNAL_DIR="$root/journal" \
  LOGIN_LAST_FILES="$root/var/lastlog $root/var/lastlog2.db $root/var/absent" DEAD_LOG_FILES="$root/dead/a.log $root/dead/gone.log" \
  LOGIN_RECORD_FILES="$root/var/btmp $root/var/wtmp" LOGROTATE_STATUS="$root/var/logrotate.status" LOG_AGE_DRY_RUN="${DRY-}" \
  LOGIN_OTHER_STORES="$root/var/wtmpdb $root/var/faillog" PROC_DIR="$root/proc" \
  bash "$job" >"$root/out" 2>&1; echo $? >"$root/rc"; }
ymd() { date -u -d "@$1" +%Y-%-m-%-d 2>/dev/null || date -u -r "$1" +%Y-%-m-%-d; }
status() { printf 'logrotate state -- version 2\n"%s" %s-5:0:1\n"%s" %s-5:0:1\n' "$root/var/btmp" "$(ymd "$1")" "$root/var/wtmp" "$(ymd "$2")" >"$root/var/logrotate.status"; }
aged() { touch -t "$(date -u -d "@$(( now - $2 * 86400 ))" +%Y%m%d%H%M 2>/dev/null || date -u -r "$(( now - $2 * 86400 ))" +%Y%m%d%H%M)" "$1"; }

# A1
for d in "" 6 29 x; do DAYS="$d" run; if [ "$(cat "$root/rc")" != 0 ] && [ ! -s "$root/journalctl.calls" ]; then ok; else bad "A1 it ran with LOG_MAX_DAYS='$d'"; fi; : >"$root/journalctl.calls"; done
unset DAYS

# Fixtures. Old first entry, written to yesterday (a quiet host's live file).
{ line $((now - 60 * 86400)); line $((now - 86400)); } >"$root/caddy/access-quiet.example.log"
# Old first entry, gzip.
{ line $((now - 22 * 86400)); line $((now - 21 * 86400)); } | gzip >"$root/caddy/access-a-2026-01-01T00-00-00.000-size.log.gz"
# First entry inside the limit, but the file itself looks old.
{ line $((now - 20 * 86400)); } >"$root/caddy/access-young.log"; touch -t 202001010000 "$root/caddy/access-young.log"
{ line $((now - 3600)); } | gzip >"$root/caddy/access-b-new.log.gz"
# No readable first entry.
echo "not a log line" >"$root/caddy/access-garbled.log"; : >"$root/caddy/access-empty.log"
echo t >"$root/var/target"
echo d >"$root/dead/a.log"; echo d >"$root/dead/b.log"
# Login records: a live file each, rotated today; rotated files young and old.
echo r >"$root/var/btmp"; echo r >"$root/var/wtmp"; status "$now" "$now"
echo r >"$root/var/btmp.1"; aged "$root/var/btmp.1" 2; echo r >"$root/var/btmp.25"; aged "$root/var/btmp.25" 25
echo r >"$root/var/wtmp-20260101.gz"; aged "$root/var/wtmp-20260101.gz" 40; echo r >"$root/var/btmpx"; aged "$root/var/btmpx" 90
echo l >"$root/var/lastlog"; echo l >"$root/var/lastlog2.db"
# A9: the first "ts": is the entry's; a later one is a header's.
printf '{"level":"info","ts":%s.5,"request":{"headers":{"X":["\\"ts\\":%s"]}}}\n' $((now - 3600)) $((now - 90 * 86400)) >"$root/caddy/access-header-old.log"
printf '{"level":"info","ts":%s.5,"request":{"headers":{"X":["\\"ts\\":%s"]}}}\n' $((now - 90 * 86400)) $((now + 3600)) >"$root/caddy/access-header-new.log"
{ line $((now + 400 * 86400)); } >"$root/caddy/access-future.log"
: >"$jdir/system.journal"; : >"$jdir/$(jname $((now - 5 * 86400)) 10)"
jbad() { printf 'system@%016x-%016x.journal~' "$(( $1 * 1000000 ))" 12345; } # set aside as damaged at that time
: >"$jdir/$(jbad $((now - 40 * 86400)))"; : >"$jdir/$(jbad $((now - 3 * 86400)))"
# A20: the dry run first, on the full fixture.
DRY=1 run; unset DRY
[ ! -s "$root/journalctl.calls" ] && ok || bad "A20 the dry run called journalctl"
[ -e "$root/caddy/access-quiet.example.log" ] && [ -e "$root/var/lastlog" ] && [ -e "$root/dead/a.log" ] && [ -e "$root/var/btmp.25" ] && [ -e "$jdir/$(jbad $((now - 40 * 86400)))" ] && ok || bad "A20 the dry run deleted something"
[ "$(grep -c "^would delete " "$root/out")" = 12 ] && grep -q "^would delete $root/var/btmp.25$" "$root/out" && grep -q "would run: journalctl --rotate" "$root/out" && grep -q "DRY RUN" "$root/out" && ok || bad "A20 the dry run did not list the 12 files a real run deletes: $(cat "$root/out")"
run
[ "$(cat "$root/rc")" = 0 ] && ok || bad "the job failed on a plain run: $(cat "$root/out")"
[ "$(sed -n 1p "$root/journalctl.calls")" = "--rotate" ] && [ "$(sed -n 2p "$root/journalctl.calls")" = "--vacuum-time=21d" ] && ok || bad "A2 journalctl calls: $(tr '\n' ';' <"$root/journalctl.calls")"
[ ! -e "$root/caddy/access-quiet.example.log" ] && ok || bad "A3 a file whose first entry is 60 days old stayed because it was written yesterday"
[ ! -e "$root/caddy/access-a-2026-01-01T00-00-00.000-size.log.gz" ] && ok || bad "A3 a compressed file whose first entry is 22 days old stayed"
[ ! -e "$root/caddy/access-garbled.log" ] && [ ! -e "$root/caddy/access-empty.log" ] && ok || bad "A4 a file with no readable first entry stayed"
[ -e "$root/caddy/access-young.log" ] && [ -e "$root/caddy/access-b-new.log.gz" ] && ok || bad "A5 a file whose first entry is inside the limit was deleted"
[ ! -e "$root/var/btmp.25" ] && [ ! -e "$root/var/wtmp-20260101.gz" ] && ok || bad "A15 a rotated login-record file past the limit stayed"
[ -e "$root/var/btmp.1" ] && [ -e "$root/var/btmp" ] && [ -e "$root/var/wtmp" ] && [ -e "$root/var/btmpx" ] && ok || bad "A15 a young rotated file, a live file or a file that only shares the prefix was deleted"
[ ! -e "$jdir/$(jbad $((now - 40 * 86400)))" ] && [ -e "$jdir/$(jbad $((now - 3 * 86400)))" ] && ok || bad "A18 the damaged journal files: the old one stayed or the young one went"
[ ! -e "$root/var/lastlog" ] && [ ! -e "$root/var/lastlog2.db" ] && ok || bad "A6 a last-login record stayed"
[ -e "$root/caddy/access-header-old.log" ] && [ ! -e "$root/caddy/access-header-new.log" ] && ok || bad "A9 the first entry's time was read from a header's ts"
[ ! -e "$root/caddy/access-future.log" ] && ok || bad "A9 a file whose first entry is dated in the future stayed"
[ ! -e "$root/dead/a.log" ] && [ -e "$root/dead/b.log" ] && ok || bad "A7 the named dead file stayed, or one not named was removed"
grep -q "log age: max_days=21 journal_folders=1 access_deleted=6 access_kept=3 access_other=0 login_rotated_deleted=2 login_last_removed=2 dead_removed=1 journal_damaged_deleted=1 failed=0" "$root/out" && ok || bad "the summary line is wrong: $(cat "$root/out")"
# The folder may be gone once it is empty of our files: not an error.
rm -rf "$root/caddy"; run
[ "$(cat "$root/rc")" = 0 ] && ok || bad "a missing access-log folder is an error"
# A8
mkdir -p "$root/caddy"; { line $((now - 60 * 86400)); } >"$root/caddy/access-stuck.log"; chmod a-w "$root/caddy"
if [ "$(id -u)" != 0 ]; then run; [ "$(cat "$root/rc")" != 0 ] && grep -q "failed=1" "$root/out" && ok || bad "A8 a failed delete was not reported: $(cat "$root/out")"; fi
chmod u+w "$root/caddy"
# A10
rm -rf "$root/caddy"; mkdir -p "$root/caddy"; echo "someone else's" >"$root/caddy/notes.txt"; run
[ -e "$root/caddy/notes.txt" ] && ok || bad "A10 a file that is not an access log was deleted"
[ "$(cat "$root/rc")" != 0 ] && grep -q "access_other=1" "$root/out" && grep -q "^<3>log age: .*notes.txt" "$root/out" && ok || bad "A10, A14 a stray file in the access-log folder did not fail the job at error priority: $(cat "$root/out")"
rm -f "$root/caddy/notes.txt"; run; [ "$(cat "$root/rc")" = 0 ] && ok || bad "the job fails on an empty access-log folder: $(cat "$root/out")"
# A11
if [ "$(id -u)" != 0 ]; then chmod a-rx "$root/caddy"; run; chmod u+rwx "$root/caddy"
  [ "$(cat "$root/rc")" != 0 ] && grep -q "^<3>log age: .*cannot be read" "$root/out" && ok || bad "A11 an unreadable access-log folder passed: $(cat "$root/out")"; fi
# A12
mkdir "$root/journal/fedcba9876543210fedcba9876543210.audit"; run
[ "$(cat "$root/rc")" != 0 ] && grep -q "^<3>log age: .*journal folders" "$root/out" && ok || bad "A12 a second journal folder passed: $(cat "$root/out")"
rmdir "$root/journal/fedcba9876543210fedcba9876543210.audit"
# A13 (the stub journalctl deletes nothing, as a trim that did not work)
old="$jdir/$(jname $((now - 40 * 86400)) 3)"; : >"$old"; run
[ "$(cat "$root/rc")" != 0 ] && grep -q "^<3>log age: .*first entry is past" "$root/out" && ok || bad "A13 a closed journal file 40 days old passed: $(cat "$root/out")"
rm -f "$old"; run; [ "$(cat "$root/rc")" = 0 ] && ok || bad "the job fails with only young journal files: $(cat "$root/out")"
rm -rf "$root/journal"; run
[ "$(cat "$root/rc")" != 0 ] && ok || bad "A12 a missing journal folder passed (the job must be told where the journal is)"
mkdir -p "$jdir"; : >"$jdir/system.journal"; run; [ "$(cat "$root/rc")" = 0 ] && ok || bad "the baseline before A16 fails: $(cat "$root/out")"
# A16
status $((now - 4 * 86400)) "$now"; run
[ "$(cat "$root/rc")" != 0 ] && grep -q "^<3>log age: .*btmp was last rotated" "$root/out" && ok || bad "A16 a login-record file not rotated for four days passed: $(cat "$root/out")"
printf 'logrotate state -- version 2\n"%s" %s-5:0:1\n' "$root/var/btmp" "$(ymd "$now")" >"$root/var/logrotate.status"; run
[ "$(cat "$root/rc")" != 0 ] && grep -q "^<3>log age: .*wtmp has never been rotated" "$root/out" && ok || bad "A16 a login-record file with no rotation on record passed: $(cat "$root/out")"
: >"$root/var/wtmp"; run; [ "$(cat "$root/rc")" = 0 ] && ok || bad "A16 an empty, never-rotated file fails the job: $(cat "$root/out")"
status "$now" "$now"
# A17
mkdir "$root/caddy/sub"; echo x >"$root/caddy/sub/access-keep.log"; ln -s "$root/var/target" "$root/caddy/access-link.log"; echo h >"$root/caddy/.hidden"; run
[ -e "$root/caddy/sub/access-keep.log" ] && [ -L "$root/caddy/access-link.log" ] && [ -e "$root/var/target" ] && [ -e "$root/caddy/.hidden" ] && ok || bad "A17 a folder, a link or a hidden file was touched"
[ "$(cat "$root/rc")" != 0 ] && grep -q "access_other=3" "$root/out" && ok || bad "A17 a folder, a link and a hidden file in the access-log folder did not fail the job: $(cat "$root/out")"
rm -rf "$root/caddy/sub" "$root/caddy/access-link.log" "$root/caddy/.hidden"
# A19
: >"$jdir/user-1000.journal"; run
[ "$(cat "$root/rc")" != 0 ] && grep -q "^<3>log age: .*user-1000.journal" "$root/out" && ok || bad "A19 an open per-user journal passed: $(cat "$root/out")"
rm -f "$jdir/user-1000.journal"
# A21
mkdir "$root/journal/fedcba9876543210fedcba9876543210.audit"; old="$jdir/$(jname $((now - 40 * 86400)) 3)"; : >"$old"; run
grep -q "^<3>log age: .*journal folders" "$root/out" && grep -q "^<3>log age: .*first entry is past" "$root/out" && ok || bad "A21 with a second journal folder the closed-file check did not run: $(cat "$root/out")"
# The slack is one day (a file's span): a closed file whose first entry is 21.5 days old is not a failure.
rm -f "$old"; rmdir "$root/journal/fedcba9876543210fedcba9876543210.audit"; : >"$jdir/$(jname $((now - 21 * 86400 - 43200)) 4)"; run
[ "$(cat "$root/rc")" = 0 ] && ok || bad "a closed journal file half a day past the limit fails the job (the slack is a day): $(cat "$root/out")"
rm -f "$jdir"/*.journal "$jdir"/*.journal~; : >"$jdir/system.journal"; run; [ "$(cat "$root/rc")" = 0 ] && ok || bad "the baseline before A22 fails: $(cat "$root/out")"
# A22
env -i PATH="$root/bin:$PATH" LOG_AGE_DRY_RUN=1 bash "$job" >"$root/out" 2>&1; rc=$?
[ "$rc" != 2 ] && grep -q "max_days=21 " "$root/out" && grep -q "journal folders under '/var/log/journal'" "$root/out" && ok || bad "A22 with no settings the job does not use the production limit and paths (rc=$rc): $(cat "$root/out")"
for v in LOGIN_RECORD_FILES LOGROTATE_STATUS LOGIN_LAST_FILES DEAD_LOG_FILES CADDY_LOG_DIR LOGIN_OTHER_STORES; do
  grep -q "^$v=\"\${$v-/" "$job" && ok || bad "A22 the job has no production default for $v"
done
# A23
DRY=0 run; [ "$(cat "$root/rc")" = 2 ] && grep -q "^<3>log age: LOG_AGE_DRY_RUN" "$root/out" && ok || bad "A23 LOG_AGE_DRY_RUN=0 was accepted: $(cat "$root/out")"
DRY=yes run; [ "$(cat "$root/rc")" = 2 ] && ok || bad "A23 LOG_AGE_DRY_RUN=yes was accepted"
echo '{"ts":1.0}' >"$root/caddy/access-old.log"
DRY=1 run; grep -q "^log age: dry_run=1 .*would_access_deleted=1 " "$root/out" && ! grep -q " access_deleted=" "$root/out" && [ -e "$root/caddy/access-old.log" ] && ok || bad "A23 the dry run's count line reads like a real run's: $(cat "$root/out")"
run; grep -q "^log age: max_days=21 .* access_deleted=1 " "$root/out" && ! grep -q "dry_run" "$root/out" && ok || bad "A23 a real run's count line: $(cat "$root/out")"
# A24
status $((now - 20 * 86400)) $((now - 20 * 86400)); old="$jdir/$(jname $((now - 30 * 86400)) 7)"; : >"$old"
DRY=1 run
[ "$(cat "$root/rc")" = 0 ] && grep -q "^note: .*btmp was last rotated" "$root/out" && ! grep -q "^<3>.*last rotated" "$root/out" && ok || bad "A24 the dry run fails on a rotation record logrotate would fix first: $(cat "$root/out")"
grep -q "^the journal trim would delete $old$" "$root/out" && grep -q "^logrotate would rotate $root/var/btmp$" "$root/out" && ok || bad "A24 the dry run does not list what the journal trim and logrotate would do: $(cat "$root/out")"
rm -f "$old"; status "$now" "$now"
# A25
for n in btmp.lock btmp.1.bak wtmp.db; do echo r >"$root/var/$n"; aged "$root/var/$n" 60; done
echo r >"$root/var/btmp.7.gz"; aged "$root/var/btmp.7.gz" 60; echo r >"$root/var/wtmp-20260101"; aged "$root/var/wtmp-20260101" 60
mkdir "$root/var/btmp.d"; ln -s "$root/var/target" "$root/var/wtmp.2"; run
[ -e "$root/var/btmp.lock" ] && [ -e "$root/var/btmp.1.bak" ] && [ -e "$root/var/wtmp.db" ] && [ -d "$root/var/btmp.d" ] && [ -L "$root/var/wtmp.2" ] && [ -e "$root/var/target" ] && ok || bad "A25 a file that is not a rotation, a folder or a link was deleted"
[ ! -e "$root/var/btmp.7.gz" ] && [ ! -e "$root/var/wtmp-20260101" ] && ok || bad "A25 a real old rotation stayed"
[ "$(cat "$root/rc")" != 0 ] && [ "$(grep -c "^<3>log age: .* is under a login record's name and is not a rotation" "$root/out")" = 5 ] && ok || bad "A25 five strays under the login records' names did not each fail the job: $(cat "$root/out")"
rm -rf "$root/var/btmp.lock" "$root/var/btmp.1.bak" "$root/var/wtmp.db" "$root/var/btmp.d" "$root/var/wtmp.2"
# A26
: >"$root/var/wtmpdb"; run
[ "$(cat "$root/rc")" != 0 ] && grep -q "^<3>log age: .*wtmpdb is a login store this job does not age" "$root/out" && ok || bad "A26 another login store passed unseen: $(cat "$root/out")"
rm -f "$root/var/wtmpdb"
# A27
echo '{"ts":1.0}' >"$root/caddy/access-held.log"; mkdir -p "$root/proc/4242/fd"; ln -s "$root/caddy/access-held.log" "$root/proc/4242/fd/7"; run
[ -e "$root/caddy/access-held.log" ] && [ "$(cat "$root/rc")" != 0 ] && grep -q "^<3>log age: .*access-held.log is still open in process 4242" "$root/out" && ok || bad "A27 an access-log file a process holds open was deleted or passed: $(cat "$root/out")"
rm -rf "$root/proc" "$root/caddy/access-held.log"; run; [ "$(cat "$root/rc")" = 0 ] && ok || bad "the final baseline fails: $(cat "$root/out")"
echo "pass=$pass fail=$fail"
[ "$fail" = 0 ]
