#!/bin/sh
# The operator command refuses what it must before it reaches corpusd, sends
# the per-computer fork-lineage acknowledgement (never the global flag), and
# exits non-zero, printing the apply counts, when the sweep failed.
set -eu
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin"
# A fake curl: a POST records its body and answers a job id; a GET answers the
# canned job in $tmp/job.json (default: a failed apply that skipped a computer).
cat >"$tmp/bin/curl" <<FAKE
#!/bin/sh
for a in "\$@"; do
	case \$prev in -d) printf '%s' "\$a" >"$tmp/body" ;; esac
	prev=\$a
done
case "\$*" in
*"-X POST"*) echo sent >>"$tmp/sent"; printf '{"job_id":"ts-20261124T000000Z-aaaaaaaa","state":"running"}' ;;
*) cat "$tmp/job.json" ;;
esac
FAKE
chmod +x "$tmp/bin/curl"
cat >"$tmp/job.json" <<'JOB'
{"state":"failed","error":"tape sweep: named computers not deleted: [computer-a] (see the receipt /x)","report":{"run_id":"ts-20261124T000000Z-aaaaaaaa","apply":{"deleted":3,"computers_deleted":1,"computers_skipped":1,"deferred_by_cap":5,"errors":0},"report_path":"/x"}}
JOB
fail=0
POLL_SECONDS=0
export POLL_SECONDS
run() { PATH="$tmp/bin:$PATH" CHOIR_INTERNAL_TOKEN_HEADER=/nonexistent "$here/removed-computers-sweep" "$@"; }
refuses() {
	name=$1
	shift
	rm -f "$tmp/sent"
	if run "$@" >/dev/null 2>&1; then
		echo "FAIL: $name was accepted" >&2
		fail=1
	fi
	if [ -e "$tmp/sent" ]; then
		echo "FAIL: $name reached curl" >&2
		fail=1
	fi
}
ACK=ts-20261124T000000Z-aaaaaaaa
refuses "no action"
refuses "an unknown action" delete --ids computer-a
refuses "a dry run without ids" dry-run
refuses "a dry run without the fork-lineage acknowledgement" dry-run --ids computer-a
refuses "an acknowledgement for another computer" dry-run --ids computer-a --accept-unrecorded-fork-lineage-for computer-b
refuses "an acknowledgement for more computers" dry-run --ids computer-a --accept-unrecorded-fork-lineage-for computer-a,computer-b
refuses "a flag taken as the ids" dry-run --ids --no-archive --accept-unrecorded-fork-lineage-for computer-a
refuses "an apply without a run id" apply --ids computer-a --accept-unrecorded-fork-lineage-for computer-a --manifest-sha256 abc
refuses "an apply without a manifest hash" apply --ids computer-a --accept-unrecorded-fork-lineage-for computer-a --run-id $ACK
refuses "a dry run with an apply flag" dry-run --ids computer-a --accept-unrecorded-fork-lineage-for computer-a --run-id $ACK
refuses "an unknown flag" dry-run --ids computer-a --accept-unrecorded-fork-lineage-for computer-a --everything
# A valid apply sends the per-computer acknowledgement and no global flag, and
# a failed job exits non-zero with the counts printed.
out=$(run apply --ids computer-a,computer-b --accept-unrecorded-fork-lineage-for computer-b,computer-a --run-id $ACK --manifest-sha256 abc --no-archive 2>&1) && rc=0 || rc=$?
[ "$rc" != 0 ] || { echo "FAIL: a failed sweep exited 0" >&2; fail=1; }
case $out in *computers_skipped*1*deferred_by_cap*5*) ;; *) echo "FAIL: the apply counts were not printed: $out" >&2; fail=1 ;; esac
body=$(cat "$tmp/body")
case $body in *'"accept_unrecorded_fork_lineage": true'*|*'"accept_unrecorded_fork_lineage":true'*) echo "FAIL: the global acknowledgement was sent: $body" >&2; fail=1 ;; esac
case $body in *'accept_unrecorded_fork_lineage_for'*computer-a*computer-b*) ;; *) echo "FAIL: the per-computer acknowledgement was not sent: $body" >&2; fail=1 ;; esac
[ "$fail" = 0 ] && echo "ok: removed-computers-sweep"
exit "$fail"
