#!/usr/bin/env bash
# Tests for scripts/res10_balloon_stats_probe.sh against a fake ssh host.
# The fake answers like curl would for the flags the probe passes: with
# -w '%{http_code}' it appends the status line; with -f a non-2xx prints
# nothing and exits 22; plain -s prints the error body and exits 0.
#
# Pins (review round 2, bunch 40):
#   - no guest URL (:8085) is fetched with the internal-service credential
#     or the internal-caller marker (hostauth: never sent to a guest);
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
WORK=$(mktemp -d /tmp/res10probe.XXXXXX)
trap 'rm -rf "$WORK"' EXIT

cat >"$WORK/ssh" <<'FAKE'
#!/usr/bin/env bash
cmd="${@: -1}"
printf '%s\n' "$cmd" >>"$FAKE_LOG"
answer() { # status body
	local status=$1 body=$2
	case "$cmd" in
	*"%{http_code}"*) printf '%s\n%s' "$body" "$status" ;;
	*" -f"* | *"-fsS"* | *"-sf"*)
		if [ "$status" -ge 400 ]; then
			echo "curl: (22) The requested URL returned error: $status" >&2
			exit 22
		fi
		printf '%s' "$body"
		;;
	*) printf '%s' "$body" ;;
	esac
	exit 0
}
LIST='{"ownerships":[{"vm_id":"vm-t1","computer_id":"computer-t1","warmness_class":"test","state":"active","computer_url":"http://10.200.5.2:8085"},{"vm_id":"vm-p","computer_id":"computer-p","warmness_class":"public_platform","computer_url":"http://10.200.6.2:8085"}]}'
READING='{"vms":[{"vm_id":"vm-t1","computer_id":"computer-t1","reading":{"vm_id":"vm-t1","stats_polling_interval_s":5,"observed_at":"2026-10-22T10:00:00Z","stats":{"target_pages":0,"actual_pages":0,"target_mib":0,"actual_mib":0,"available_memory":6012954214,"total_memory":8589934592,"free_memory":1073741824,"disk_caches":4831838208}}}],"units":"bytes"}'
REASON='{"vms":[{"vm_id":"vm-t1","computer_id":"computer-t1","reason":"stats_disabled","detail":"balloon booted with stats_polling_interval_s 0"}],"units":"bytes"}'
DENIED='{"error":"vmctl control endpoints are not publicly accessible"}'
PROCMEM='{"meminfo":{"MemTotal":8167000,"MemFree":1048000,"MemAvailable":5870000,"Cached":4600000,"Buffers":10000}}'
case "$cmd" in
*vmctl/list*)
	case "$FAKE_CASE" in list403) answer 403 "$DENIED" ;; *) answer 200 "$LIST" ;; esac ;;
*memory/balloon*)
	case "$FAKE_CASE" in
	balloon403) answer 403 "$DENIED" ;;
	empty) answer 200 '{"vms":[],"units":"bytes"}' ;;
	reason) answer 200 "$REASON" ;;
	*) answer 200 "$READING" ;;
	esac ;;
*pgrep*) echo 1618000 ;;
*procmem*)
	case "$FAKE_CASE" in procmem403) answer 403 'profiles are served to host-sourced callers only' ;; *) answer 200 "$PROCMEM" ;; esac ;;
*) echo "fake ssh: unexpected command" >&2; exit 99 ;;
esac
FAKE
chmod +x "$WORK/ssh"

fails=0
run() { # case computer -> sets rc, out, err
	FAKE_CASE=$1 FAKE_LOG="$WORK/log.$1" PATH="$WORK:$PATH" \
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
guest_cmds=$(grep -F ':8085' "$WORK/log.happy" || true)
check "guest procmem is fetched" "$(is '[ -n "$guest_cmds" ]')"
check "no guest fetch carries the credential or marker" "$(is '! printf "%s" "$guest_cmds" | grep -Eq "internal-service-token|X-Internal-Caller"')"
check "vmctl calls still carry the credential" "$(is 'grep -F "localhost:8083" "$WORK/log.happy" | grep -q internal-service-token')"

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
