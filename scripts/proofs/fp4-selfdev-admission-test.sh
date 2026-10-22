#!/usr/bin/env bash
# Offline fixture test of scripts/proofs/fp4-selfdev-admission.sh steps 2, 4, 5
# and 7: canned trajectory snapshots and event pages (the JSON shapes of
# GET /api/trajectories/<id> and /events) served by a fake curl, and a fake ssh
# for vmctl's list and the guest log. No network, no Node B, no key. Each step
# is run for a PASS fixture and for FAIL fixtures, including the two defects of
# docs/problems/fp4-proof-steps-5-and-7-read-the-wrong-evidence-2026-10-22.md:
# a matching log line must not rescue a wrong row, and a missing log line must
# not fail a right one.
#
#   scripts/proofs/fp4-selfdev-admission-test.sh   (exit 0 when every case passes)
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT=$HERE/fp4-selfdev-admission.sh
WORK=$(mktemp -d "${TMPDIR:-/tmp}/fp4-admission-test.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
FX=$WORK/fx
FAKES=$WORK/fakes
mkdir -p "$FX" "$FAKES"
export FX
TEST=computer-0123456789abcdef0123456789abcdef
PAPER=computer-1355b83bced1613eb048ee28d66688a8

cat >"$FAKES/curl" <<'J'
#!/bin/sh
url=; for a in "$@"; do case "$a" in http*) url=$a ;; esac; done
echo "$url" >>"$FX/urls"
p=${url#*://*/}; p=/${p%%\?*}
case "$p" in
/api/trajectories/*/events) f=$FX/${p#/api/trajectories/}; f=${f%/events}.events.json ;;
/api/trajectories/*) f=$FX/${p#/api/trajectories/}.snap.json ;;
esac
[ -f "$f" ] && cat "$f" || { echo "fake curl: no fixture for $url" >&2; exit 22; }
J
cat >"$FAKES/ssh" <<'J'
#!/bin/sh
in=$(cat)
case "$in" in
*vmctl/list*) echo '{"ownerships":[{"vm_id":"vm-test1","computer_id":"computer-0123456789abcdef0123456789abcdef","user_id":"u2","desktop_id":"d2","state":"active","warmness_class":"critical_protected","computer_url":"http://10.9.7.2:8080"}]}' ;;
*observability*) cat "$FX/obs.json" ;;
esac
J
chmod +x "$FAKES/curl" "$FAKES/ssh"

# ---- fixture builders (jq) ------------------------------------------------
# row ID WORKITEM DISPOSITION CAPSULE REASON TERMINAL_AT
row() { jq -nc --arg id "$1" --arg wi "$2" --arg d "$3" --arg c "$4" --arg r "$5" --arg t "$6" '
	{assignment_id:$id, binding:{assigned_work_item_id:$wi}, disposition:$d, capsule_disposition:$c}
	+ (if $r != "" then {disposition_reason:$r} else {} end)
	+ (if $t != "" then {terminal_at:$t} else {} end)'; }
# put_snap TRAJ ROW...
put_snap() { local t=$1; shift; printf '%s\n' "$@" | jq -sc '{co_super_assignments:.}' >"$FX/$t.snap.json"; }
# put_events TRAJ KIND:WORKITEM:CREATED_AT...
put_events() {
	local t=$1 e; shift
	for e in "$@"; do k=${e%%:*}; w=${e#*:}; w=${w%%:*}; c=${e#*:*:}; jq -nc --arg k "$k" --arg w "$w" --arg c "$c" '{kind:$k, work_item_id:$w, created_at:$c, reducer_seq:1}'; done | jq -sc '{schema:"x", events:., next_cursor:1}' >"$FX/$t.events.json"
}
obs() { jq -nc --argjson l "$1" '{recent_log:$l}' >"$FX/obs.json"; }

A_OPEN='co_super_assignment_opened'
BOUND='co_super_assignment_bound'
TS_A_TERM=2026-10-22T05:00:00.250Z
PASSN=0; FAILN=0

# run NAME STEP EXPECT(pass|fail) [extra args]: EXPECT pass = a PASS line and
# no FAIL; fail = at least one FAIL line.
run() {
	local name=$1 step=$2 expect=$3 out rc
	shift 3
	out=$(BASH_ENV= PROOF_NAP_SCALE=0 PROOF_SSH_BIN="$FAKES/ssh" PROOF_CURL_BIN="$FAKES/curl" CHOIR_API_KEY=testkey-not-a-secret \
		"$SCRIPT" --computer "$TEST" --step "$step" "$@" 2>&1); rc=$?
	local npass nfail
	npass=$(printf '%s\n' "$out" | grep -c ' PASS step=')
	nfail=$(printf '%s\n' "$out" | grep -c ' FAIL step=')
	if { [ "$expect" = pass ] && [ "$nfail" = 0 ] && [ "$rc" = 0 ] && [ "$npass" -ge 1 ]; } ||
		{ [ "$expect" = fail ] && [ "$nfail" -ge 1 ] && [ "$rc" = 1 ]; }; then
		PASSN=$((PASSN + 1)); echo "ok   $name"
	else
		FAILN=$((FAILN + 1)); echo "NOT OK $name (expected $expect, rc=$rc pass=$npass fail=$nfail)"; printf '%s\n' "$out" | sed 's/^/     | /'
	fi
}

AROW=$(row asg-a wi-a bound active "" "")
BROW_OPEN=$(row asg-b wi-b open unbound "" "")
ARGS=(--trajectory-a ta --trajectory-b tb)
LOGGED='["runtime: engineering admission: assignment asg-b waits behind asg-a (x); retry 1 in 1m0s","actorruntime: boot follows a crash or stop; pre-boot work is interrupted_by_restart","engineering request bound"]'
NOLOG='[]'

# ---------------- step 2
obs "$NOLOG"
put_snap ta "$AROW"; put_events ta "$A_OPEN:wi-a:2026-10-22T04:00:00Z" "$BOUND:wi-a:2026-10-22T04:00:05Z"
put_snap tb "$BROW_OPEN"; put_events tb "$A_OPEN:wi-b:2026-10-22T04:01:00Z"
run "step2 PASS: A bound+active, B open unbound, no log line at all" 2 pass "${ARGS[@]}"
put_snap tb "$(row asg-b wi-b bound active "" "")"; put_events tb "$BOUND:wi-b:2026-10-22T04:02:00Z"
obs "$LOGGED"
run "step2 FAIL: B already bound (log says waits behind)" 2 fail "${ARGS[@]}"
put_snap tb "$BROW_OPEN"; put_events tb "$A_OPEN:wi-b:2026-10-22T04:01:00Z" "$BOUND:wi-b:2026-10-22T04:02:00Z"
run "step2 FAIL: B row open but a bound event exists" 2 fail "${ARGS[@]}"
put_events tb "$A_OPEN:wi-b:2026-10-22T04:01:00Z"
put_snap ta "$(row asg-a wi-a bound revoked "" "")"
run "step2 FAIL: A's capsule revoked" 2 fail "${ARGS[@]}"
put_snap ta "$(row asg-a wi-a cancelled revoked interrupted_by_restart 2026-10-22T04:30:00Z)"
run "step2 FAIL: A cancelled" 2 fail "${ARGS[@]}"
# one trajectory holding both rows
put_snap tab "$AROW" "$BROW_OPEN"; put_events tab "$BOUND:wi-a:2026-10-22T04:00:05Z"
run "step2 FAIL: both rows in one trajectory, no assignment ids (ambiguous, never picked)" 2 fail --trajectory-a tab
run "step2 PASS: both rows in one trajectory, named by assignment id" 2 pass --trajectory-a tab --assignment-a asg-a --assignment-b asg-b
run "step2 FAIL: assignment id absent from the snapshot" 2 fail --trajectory-a tab --assignment-a asg-a --assignment-b asg-zzz

# ---------------- step 4
put_snap tb "$BROW_OPEN"; put_events tb "$A_OPEN:wi-b:2026-10-22T04:01:00Z"
obs "$NOLOG"
run "step4 PASS: B still open after boot (kept-and-resumed line absent)" 4 pass "${ARGS[@]}"
put_snap tb "$(row asg-b wi-b cancelled revoked interrupted_by_restart 2026-10-22T04:40:00Z)"
obs '["runtime: planned boot: queued engineering request asg-b kept and resumed"]'
run "step4 FAIL: B cancelled after boot (kept-and-resumed line present)" 4 fail "${ARGS[@]}"
put_snap tb "$BROW_OPEN"; put_events tb "$BOUND:wi-b:2026-10-22T04:41:00Z"
run "step4 FAIL: B has a bound event" 4 fail "${ARGS[@]}"

# ---------------- step 5
ATERM=$(row asg-a wi-a completed revoked "" "$TS_A_TERM")
put_snap ta "$ATERM"; put_events ta "$BOUND:wi-a:2026-10-22T04:00:05Z"
put_snap tb "$(row asg-b wi-b bound active "" "")"
obs "$NOLOG"
put_events tb "$A_OPEN:wi-b:2026-10-22T04:01:00Z" "$BOUND:wi-b:2026-10-22T05:00:05.100Z"
run "step5 PASS: B bound 4.85s after A's terminal_at (no log line)" 5 pass "${ARGS[@]}"
put_events tb "$BOUND:wi-b:2026-10-22T05:01:00Z"
obs '["engineering request admitted and bound"]'
run "step5 FAIL: B bound 60s after (the minute retry; log says bound)" 5 fail "${ARGS[@]}"
run "step5 PASS: the same 60s with --nudge-seconds 90" 5 pass "${ARGS[@]}" --nudge-seconds 90
put_events tb "$BOUND:wi-b:2026-10-22T04:59:50Z"
run "step5 FAIL: B bound BEFORE A's terminal_at" 5 fail "${ARGS[@]}"
put_events tb "$A_OPEN:wi-b:2026-10-22T04:01:00Z"
run "step5 FAIL: B has no bound event" 5 fail "${ARGS[@]}"
put_snap ta "$AROW"; put_events tb "$BOUND:wi-b:2026-10-22T05:00:05Z"
run "step5 FAIL: A not terminal yet" 5 fail "${ARGS[@]}"
put_snap ta "$ATERM"; put_events tb "$BOUND:wi-other:2026-10-22T05:00:05Z"
run "step5 FAIL: the bound event belongs to another work item" 5 fail "${ARGS[@]}"

# ---------------- step 7
BCAN=$(row asg-b wi-b cancelled revoked interrupted_by_restart 2026-10-22T06:00:00Z)
put_snap tb "$BCAN"; put_events tb "$A_OPEN:wi-b:2026-10-22T04:01:00Z"
obs "$NOLOG"
run "step7 PASS: B cancelled interrupted_by_restart, never bound (no log line)" 7 pass "${ARGS[@]}"
put_snap tb "$BROW_OPEN"
obs "$LOGGED"
run "step7 FAIL: B still open though the generic crash-boot line is in the log" 7 fail "${ARGS[@]}"
put_snap tb "$(row asg-b wi-b cancelled revoked operator_cancel 2026-10-22T06:00:00Z)"
run "step7 FAIL: B cancelled for another reason" 7 fail "${ARGS[@]}"
put_snap tb "$BCAN"; put_events tb "$BOUND:wi-b:2026-10-22T04:02:00Z"
run "step7 FAIL: B cancelled interrupted_by_restart but had been bound" 7 fail "${ARGS[@]}"

# ---------------- guards
out=$(BASH_ENV= PROOF_SSH_BIN="$FAKES/ssh" PROOF_CURL_BIN="$FAKES/curl" CHOIR_API_KEY=k "$SCRIPT" --computer "$PAPER" --step 5 "${ARGS[@]}" 2>&1); rc=$?
if [ "$rc" = 3 ]; then PASSN=$((PASSN + 1)); echo "ok   guard: the paper's computer is refused for a row-reading step"; else FAILN=$((FAILN + 1)); echo "NOT OK guard paper rc=$rc"; fi
out=$(BASH_ENV= "$SCRIPT" --computer computer-ad1b46d4c20129e84b9e239cf4720fb9 --step 2 "${ARGS[@]}" 2>&1); rc=$?
if [ "$rc" = 3 ]; then PASSN=$((PASSN + 1)); echo "ok   guard: the delegate is refused"; else FAILN=$((FAILN + 1)); echo "NOT OK guard delegate rc=$rc"; fi
# the key never appears in evidence
if grep -q testkey-not-a-secret "$FX"/urls 2>/dev/null; then FAILN=$((FAILN + 1)); echo "NOT OK key in url"; fi

echo "$PASSN passed, $FAILN failed"
[ "$FAILN" = 0 ]
