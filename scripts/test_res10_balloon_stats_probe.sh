#!/usr/bin/env bash
# Tests for scripts/res10_balloon_stats_probe.sh against a fake ssh host.
# The fake ssh takes the probe's `ssh HOST sh -s` and runs what arrives on
# stdin (scripts/lib/choir_internal.sh, then the command) under a local
# sh, with a fake curl on PATH that records its argv and answers as curl
# would for those flags: with -w '%{http_code}' it appends the status line;
# with -f a non-2xx prints nothing and exits 22. A readable fake token
# header file stands in for Node B's, so the helper's real decision (token
# or not) is what the test sees.
#
# Pins (review round 2, bunch 40; deploy L token-probe fix):
#   - the helper's text is sent ahead of every remote command;
#   - no guest request (:8085) carries the token or the internal-caller
#     marker; vmctl requests go to http://127.0.0.1:8083 and carry both;
#   - the probe holds no token variable and no header-file path;
#   - a non-2xx from vmctl (list or balloon route) exits non-zero and
#     prints the status and the error body, never a dash table;
#   - a 2xx balloon answer with no entry for the computer is an error;
#   - a failed guest procmem read exits non-zero;
#   - a reason (stats_disabled) is a valid answer: exit 0, no numbers;
#   - a computer not classed test is refused.
#
# usage: scripts/test_res10_balloon_stats_probe.sh
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
PROBE="$HERE/res10_balloon_stats_probe.sh"
LIB="$HERE/lib/choir_internal.sh"
WORK=$(mktemp -d /tmp/res10probe.XXXXXX)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/sshbin"
printf 'X-Choir-Internal-Token: fake-test-token\n' >"$WORK/token.header"
export FAKE_TOKEN_FILE="$WORK/token.header" FAKE_LIB="$LIB" FAKE_BIN="$WORK/bin"

cat >"$WORK/sshbin/ssh" <<'FAKE'
#!/usr/bin/env bash
# Only the probe's shape: ssh [-o ...] HOST sh -s, script on stdin.
n=$#
if [ "${@:n-1:1}" != sh ] || [ "${@:n:1}" != -s ]; then
	echo "fake ssh: expected 'HOST sh -s', got: $*" >&2
	exit 98
fi
script=$(cat)
lib=$(cat "$FAKE_LIB")
case "$script" in
"$lib"*) ;;
*) echo "fake ssh: the helper was not sent ahead of the command" >&2; echo "LIB-MISSING" >>"$FAKE_LOG"; exit 97 ;;
esac
printf '%s\n' "$script" | PATH="$FAKE_BIN:$PATH" CHOIR_INTERNAL_TOKEN_HEADER="$FAKE_TOKEN_FILE" sh -s
FAKE

cat >"$WORK/bin/curl" <<'FAKE'
#!/usr/bin/env bash
printf 'CURL' >>"$FAKE_LOG"
printf ' [%s]' "$@" >>"$FAKE_LOG"
printf '\n' >>"$FAKE_LOG"
url="" w=0 f=0
for a in "$@"; do
	case "$a" in
	http://* | https://*) url=$a ;;
	*%{http_code}*) w=1 ;;
	-f | -fsS | -sf | -fs | --fail) f=1 ;;
	esac
done
answer() { # status body
	if [ $f = 1 ] && [ "$1" -ge 400 ]; then
		echo "curl: (22) The requested URL returned error: $1" >&2
		exit 22
	fi
	printf '%s' "$2"
	[ $w = 1 ] && printf '\n%s' "$1"
	exit 0
}
LIST='{"ownerships":[{"vm_id":"vm-t1","computer_id":"computer-t1","warmness_class":"test","state":"active","computer_url":"http://10.200.5.2:8085"},{"vm_id":"vm-p","computer_id":"computer-p","warmness_class":"public_platform","computer_url":"http://10.200.6.2:8085"}]}'
READING='{"vms":[{"vm_id":"vm-t1","computer_id":"computer-t1","reading":{"vm_id":"vm-t1","stats_polling_interval_s":5,"observed_at":"2026-10-22T10:00:00Z","stats":{"target_pages":0,"actual_pages":0,"target_mib":0,"actual_mib":0,"available_memory":6012954214,"total_memory":8589934592,"free_memory":1073741824,"disk_caches":4831838208}}}],"units":"bytes"}'
REASON='{"vms":[{"vm_id":"vm-t1","computer_id":"computer-t1","reason":"stats_disabled","detail":"balloon booted with stats_polling_interval_s 0"}],"units":"bytes"}'
DENIED='{"error":"vmctl control endpoints are not publicly accessible"}'
# The guest's shape (internal/server/debug_procmem.go: <key>_kib), read from
# test computer 71afbf14 on 2026-10-22; PROCMEM_LEGACY is an older guest's
# bare names.
PROCMEM='{"meminfo":{"AnonPages_kib":143732,"Buffers_kib":6804,"Cached_kib":640484,"MemAvailable_kib":7608712,"MemFree_kib":7187332,"MemTotal_kib":8133468,"Shmem_kib":11884,"Slab_kib":52220}}'
PROCMEM_LEGACY='{"meminfo":{"MemTotal":8167000,"MemFree":1048000,"MemAvailable":5870000,"Cached":4600000,"Buffers":10000}}'
case "$url" in
http://127.0.0.1:8083/internal/vmctl/list)
	case "$FAKE_CASE" in list403) answer 403 "$DENIED" ;; *) answer 200 "$LIST" ;; esac ;;
http://127.0.0.1:8083/internal/vmctl/memory/balloon*)
	case "$FAKE_CASE" in
	balloon403) answer 403 "$DENIED" ;;
	empty) answer 200 '{"vms":[],"units":"bytes"}' ;;
	reason) answer 200 "$REASON" ;;
	*) answer 200 "$READING" ;;
	esac ;;
http://10.200.*:8085/internal/debug/pprof/procmem)
	case "$FAKE_CASE" in procmem403) answer 403 'profiles are served to host-sourced callers only' ;; legacy) answer 200 "$PROCMEM_LEGACY" ;; *) answer 200 "$PROCMEM" ;; esac ;;
*) echo "fake curl: unexpected URL $url" >&2; exit 99 ;;
esac
FAKE
# The rss read: no firecracker process in the fake.
printf '#!/bin/sh\nexit 1\n' >"$WORK/bin/pgrep"
chmod +x "$WORK/sshbin/ssh" "$WORK/bin/curl" "$WORK/bin/pgrep"

fails=0
run() { # case computer -> sets rc, out, err
	FAKE_CASE=$1 FAKE_LOG="$WORK/log.$1" PATH="$WORK/sshbin:$PATH" \
		"$PROBE" --computer-id "$2" >"$WORK/out" 2>"$WORK/err"
	rc=$?
	out=$(cat "$WORK/out")
	err=$(cat "$WORK/err")
}
check() { # name condition-result
	if [ "$2" = ok ]; then echo "ok   $1"; else echo "FAIL $1"; echo "  rc=$rc"; sed 's/^/  err: /' "$WORK/err"; fails=$((fails + 1)); fi
}
is() { if eval "$1"; then echo ok; else echo no; fi; }

run happy computer-t1
check "happy path exits 0 with numbers" "$(is '[ $rc -eq 0 ] && printf "%s" "$out" | grep -q "\"available_memory\": 6012954214"')"
guest_calls=$(grep '^CURL' "$WORK/log.happy" | grep -F ':8085' || true)
vmctl_calls=$(grep '^CURL' "$WORK/log.happy" | grep -F '127.0.0.1:8083' || true)
check "the helper is sent ahead of every remote command" "$(is '! grep -q LIB-MISSING "$WORK/log.happy"')"
check "guest procmem is fetched" "$(is '[ -n "$guest_calls" ]')"
check "no guest request carries the token or the marker" "$(is '! printf "%s" "$guest_calls" | grep -Eq "token\.header|X-Internal-Caller|X-Choir-Internal-Token"')"
check "vmctl requests go to 127.0.0.1:8083 with the marker and the token" "$(is '[ $(printf "%s\n" "$vmctl_calls" | wc -l) -eq 2 ] && ! printf "%s\n" "$vmctl_calls" | grep -vq "\[@$WORK/token.header\]" && ! printf "%s\n" "$vmctl_calls" | grep -vq "\[X-Internal-Caller: true\]"')"
check "the probe holds no token variable or header-file path" "$(is '! grep -Eq "internal-service-token|AUTH=|\\\$AUTH" "$PROBE"')"

# Guest columns, MiB: _kib keys (Cached+Buffers 647288 KiB = 632.1), and an
# older guest's bare names (4610000 KiB = 4502.0).
guest_col() { printf '%s' "$out" | python3 -c 'import json,sys; print({r["what"]: r["guest_meminfo"] for r in json.load(sys.stdin)["compare_mib"]})'; }
check "guest meminfo read from the _kib keys" "$(is '[ "$(guest_col)" = "{'"'"'available'"'"': 7430.4, '"'"'disk caches'"'"': 632.1, '"'"'total'"'"': 7942.8, '"'"'free'"'"': 7018.9}" ]')"
run legacy computer-t1
check "an older guest's bare meminfo names still read" "$(is '[ $rc -eq 0 ] && [ "$(guest_col)" = "{'"'"'available'"'"': 5732.4, '"'"'disk caches'"'"': 4502.0, '"'"'total'"'"': 7975.6, '"'"'free'"'"': 1023.4}" ]')"

run balloon403 computer-t1
check "balloon route 403 exits non-zero" "$(is '[ $rc -ne 0 ]')"
check "balloon route 403 prints status and body" "$(is 'printf "%s" "$err" | grep -q 403 && printf "%s" "$err" | grep -q "not publicly accessible"')"
check "balloon route 403 prints no table" "$(is '! printf "%s" "$err" | grep -q "^available"')"

run list403 computer-t1
check "list 403 exits non-zero with status and body" "$(is '[ $rc -ne 0 ] && printf "%s" "$err" | grep -q 403 && printf "%s" "$err" | grep -q "not publicly accessible"')"

run empty computer-t1
check "missing vms entry exits non-zero" "$(is '[ $rc -ne 0 ] && ! printf "%s" "$err" | grep -q "^available"')"

run procmem403 computer-t1
check "guest procmem 403 exits non-zero with status" "$(is '[ $rc -ne 0 ] && printf "%s" "$err" | grep -q 403')"

run reason computer-t1
check "a reason exits 0 and shows no balloon numbers" "$(is '[ $rc -eq 0 ] && printf "%s" "$err" | grep -q "UNAVAILABLE: stats_disabled" && printf "%s" "$out" | grep -q "\"balloon_stats_bytes\": null"')"

run happy computer-p
check "non-test computer refused" "$(is '[ $rc -ne 0 ] && printf "%s" "$err" | grep -q "not test"')"

if [ $fails -ne 0 ]; then
	echo "$fails failed"
	exit 1
fi
echo "all passed"
