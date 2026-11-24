#!/usr/bin/env bash
# Behavioural test for scripts/node-b-caddy-log-age, the daily job that
# bounds the age of Caddy's access logs where Caddy itself does not: Caddy
# deletes a rolled file only when it next rolls, and rolls only a file it has
# written to since it started, so a quiet or retired host name's files would
# stay for ever. Runs against a temporary directory.
#
# Failure list, written before the code:
#  A1 a rolled file last written 28 days ago or more is not deleted
#  A2 a file last written under 28 days ago is deleted (logs are lost)
#  A3 a live file (no timestamp in its name) that nobody has written to for
#     28 days is not deleted (a retired host name's log stays for ever)
#  A4 a live file not written for over a day is left under its live name:
#     Caddy would later append to it and stamp the whole file with the new
#     roll time, so its old lines would outlive the bound. It must be set
#     aside under a rolled name (and keep its contents).
#  A5 a live file written within the last day is touched (Caddy is using it)
#  A6 something that is not a regular file directly in the directory, or a
#     file in a subdirectory, is deleted or renamed
#  A7 a missing or unset directory, or a limit that is not a number of at
#     least 2, deletes anything or exits 0
#  A8 a file whose name starts with a dash or holds a space breaks the job
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
S="$HERE/node-b-caddy-log-age"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
pass=0; fail=0
check() { if eval "$2"; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "FAIL: $1"; fi; }
stamp() { python3 -c "import time,sys; print(time.strftime('%Y%m%d%H%M.%S', time.localtime(time.time()-float(sys.argv[1])*86400)))" "$1"; }
mk() { printf '%s\n' "line of $1" >"$D/$1"; touch -t "$(stamp "$2")" "$D/$1"; }

D="$T/caddy"; mkdir -p "$D/sub"
mk "access-a.example-2026-10-01T00-00-00.000-time.log.gz" 29
mk "access-a.example-2026-10-02T00-00-00.000-size.log.gz" 28.2
mk "access-a.example-2026-10-20T00-00-00.000-time.log.gz" 27.5
mk "access-a.example-2026-11-20T00-00-00.000-time.log" 3
mk "access-a.example.log" 0.2
mk "access-retired.example.log" 40
mk "access-quiet.example.log" 5
mk "access with space.log" 5
mk "-dash.log" 30
printf 'keep\n' >"$D/sub/old.log"; touch -t "$(stamp 60)" "$D/sub/old.log"
ln -s "$D/access-a.example.log" "$D/link.log"

CADDY_LOG_DIR="$D" CADDY_LOG_MAX_DAYS=28 bash "$S" >"$T/out" 2>&1; rc=$?
check "exit 0" '[ "$rc" = 0 ]'
check "A1 29 days: deleted" '[ ! -e "$D/access-a.example-2026-10-01T00-00-00.000-time.log.gz" ]'
check "A1 28.2 days: deleted" '[ ! -e "$D/access-a.example-2026-10-02T00-00-00.000-size.log.gz" ]'
check "A2 27.5 days: kept" '[ -e "$D/access-a.example-2026-10-20T00-00-00.000-time.log.gz" ]'
check "A2 3 days: kept" '[ -e "$D/access-a.example-2026-11-20T00-00-00.000-time.log" ]'
check "A3 retired live file: deleted" '[ ! -e "$D/access-retired.example.log" ] && ! ls "$D" | grep -q "^access-retired"'
check "A4 quiet live file: set aside under a rolled name" '[ ! -e "$D/access-quiet.example.log" ] && [ "$(ls "$D" | grep -c "^access-quiet.example-20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9-]*\.[0-9]*-idle\.log$")" = 1 ]'
check "A4 its contents are kept" 'grep -q "line of access-quiet.example.log" "$D"/access-quiet.example-*-idle.log'
check "A5 live file in use: untouched" '[ -e "$D/access-a.example.log" ] && grep -q "line of" "$D/access-a.example.log"'
check "A6 subdirectory file untouched" '[ -e "$D/sub/old.log" ]'
check "A6 symlink untouched" '[ -L "$D/link.log" ]'
check "A8 name with a space: set aside" '[ ! -e "$D/access with space.log" ] && ls "$D" | grep -q "^access with space-.*-idle\.log$"'
check "A8 name with a dash, 30 days: deleted" '[ ! -e "$D/-dash.log" ]'
check "the job says what it did, by count" 'grep -q "deleted=4 set_aside=2" "$T/out"'

# A second run a moment later changes nothing more.
before="$(ls "$D" | sort)"
CADDY_LOG_DIR="$D" CADDY_LOG_MAX_DAYS=28 bash "$S" >"$T/out2" 2>&1
check "a second run is a no-op" '[ "$(ls "$D" | sort)" = "$before" ] && grep -q "deleted=0 set_aside=0" "$T/out2"'

mk "x-2026-01-01T00-00-00.000-time.log" 100
for bad in "" "x" "1" "0" "-5"; do
  CADDY_LOG_DIR="$D" CADDY_LOG_MAX_DAYS="$bad" bash "$S" >/dev/null 2>&1; rc=$?
  check "A7 limit \"$bad\" refused" '[ "$rc" != 0 ] && [ -e "$D/x-2026-01-01T00-00-00.000-time.log" ]'
done
CADDY_LOG_DIR="$T/none" CADDY_LOG_MAX_DAYS=28 bash "$S" >/dev/null 2>&1; check "A7 missing directory refused" '[ "$?" != 0 ]'
(cd "$D" && env -u CADDY_LOG_DIR CADDY_LOG_MAX_DAYS=28 bash "$S" >/dev/null 2>&1); rc=$?
check "A7 unset directory refused, nothing deleted in the current directory" '[ "$rc" != 0 ] && [ -e "$D/x-2026-01-01T00-00-00.000-time.log" ]'

echo "node-b-caddy-log-age: $pass passed, $fail failed"
[ "$fail" = 0 ]
