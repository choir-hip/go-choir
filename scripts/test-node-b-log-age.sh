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
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
job="$HERE/node-b-log-age"
pass=0; fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); echo "FAIL: $1"; }
root="$(mktemp -d)"; trap 'chmod -R u+w "$root" 2>/dev/null; rm -rf "$root"' EXIT
mkdir -p "$root/bin" "$root/caddy/sub" "$root/var" "$root/dead"
printf '#!/usr/bin/env bash\necho "$*" >>"%s/journalctl.calls"\n' "$root" >"$root/bin/journalctl"; chmod +x "$root/bin/journalctl"
now=$(date +%s)
line() { printf '{"level":"info","ts":%s.123,"logger":"http.log.access","msg":"handled request"}\n' "$1"; }
run() { env PATH="$root/bin:$PATH" LOG_MAX_DAYS="${DAYS-21}" CADDY_LOG_DIR="$root/caddy" \
  LOGIN_LAST_FILES="$root/var/lastlog $root/var/lastlog2.db $root/var/absent" DEAD_LOG_FILES="$root/dead/a.log $root/dead/gone.log" \
  bash "$job" >"$root/out" 2>&1; echo $? >"$root/rc"; }

# A1
for d in "" 6 29 x; do DAYS="$d" run; if [ "$(cat "$root/rc")" != 0 ] && [ ! -s "$root/journalctl.calls" ]; then ok; else bad "A1 it ran with LOG_MAX_DAYS='$d'"; fi; : >"$root/journalctl.calls"; done
unset DAYS

# Fixtures. Old first entry, written to yesterday (a quiet host's live file).
{ line $((now - 60 * 86400)); line $((now - 86400)); } >"$root/caddy/access-quiet.log"
# Old first entry, gzip.
{ line $((now - 22 * 86400)); line $((now - 21 * 86400)); } | gzip >"$root/caddy/access-a-2026-01-01T00-00-00.000-size.log.gz"
# First entry inside the limit, but the file itself looks old.
{ line $((now - 20 * 86400)); } >"$root/caddy/access-young.log"; touch -t 202001010000 "$root/caddy/access-young.log"
{ line $((now - 3600)); } | gzip >"$root/caddy/access-b-new.log.gz"
# No readable first entry.
echo "not a log line" >"$root/caddy/access-garbled.log"; : >"$root/caddy/access-empty.log"
echo "x" >"$root/caddy/sub/keep.log"; ln -s "$root/var/target" "$root/caddy/link.log"; echo t >"$root/var/target"
echo l >"$root/var/lastlog"; echo l >"$root/var/lastlog2.db"
echo d >"$root/dead/a.log"; echo d >"$root/dead/b.log"
run
[ "$(cat "$root/rc")" = 0 ] && ok || bad "the job failed on a plain run: $(cat "$root/out")"
[ "$(sed -n 1p "$root/journalctl.calls")" = "--rotate" ] && [ "$(sed -n 2p "$root/journalctl.calls")" = "--vacuum-time=21d" ] && ok || bad "A2 journalctl calls: $(tr '\n' ';' <"$root/journalctl.calls")"
[ ! -e "$root/caddy/access-quiet.log" ] && ok || bad "A3 a file whose first entry is 60 days old stayed because it was written yesterday"
[ ! -e "$root/caddy/access-a-2026-01-01T00-00-00.000-size.log.gz" ] && ok || bad "A3 a compressed file whose first entry is 22 days old stayed"
[ ! -e "$root/caddy/access-garbled.log" ] && [ ! -e "$root/caddy/access-empty.log" ] && ok || bad "A4 a file with no readable first entry stayed"
[ -e "$root/caddy/access-young.log" ] && [ -e "$root/caddy/access-b-new.log.gz" ] && ok || bad "A5 a file whose first entry is inside the limit was deleted"
[ -e "$root/caddy/sub/keep.log" ] && [ -L "$root/caddy/link.log" ] && [ -e "$root/var/target" ] && ok || bad "A5 a folder or a link was touched"
[ ! -e "$root/var/lastlog" ] && [ ! -e "$root/var/lastlog2.db" ] && ok || bad "A6 a last-login record stayed"
[ ! -e "$root/dead/a.log" ] && [ -e "$root/dead/b.log" ] && ok || bad "A7 the named dead file stayed, or one not named was removed"
grep -q "log age: max_days=21 access_deleted=4 access_kept=2 login_last_removed=2 dead_removed=1 failed=0" "$root/out" && ok || bad "the summary line is wrong: $(cat "$root/out")"
# The folder may be gone once it is empty of our files: not an error.
rm -rf "$root/caddy"; run
[ "$(cat "$root/rc")" = 0 ] && ok || bad "a missing access-log folder is an error"
# A8
mkdir -p "$root/caddy"; { line $((now - 60 * 86400)); } >"$root/caddy/access-stuck.log"; chmod a-w "$root/caddy"
if [ "$(id -u)" != 0 ]; then run; [ "$(cat "$root/rc")" != 0 ] && grep -q "failed=1" "$root/out" && ok || bad "A8 a failed delete was not reported: $(cat "$root/out")"; fi
chmod u+w "$root/caddy"
echo "pass=$pass fail=$fail"
[ "$fail" = 0 ]
