#!/usr/bin/env bash
# Self-test of the proof scripts with stubs: no network, no ssh, no staging.
# `ssh node-b` is replaced by a stub that runs the script it receives locally
# under fake curl, journalctl, iptables, systemd-run and systemctl; the public
# API goes through the same fake curl. It proves the guards and the shape of
# the evidence, not staging behavior.
#
#   scripts/proofs/selftest.sh        (exit 0 when every case passes)
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/proofs-selftest.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
FAKES=$WORK/fakes
mkdir -p "$FAKES"
export ST=$WORK
: >"$ST/seq.log"

DELEGATE=computer-ad1b46d4c20129e84b9e239cf4720fb9
PAPER=computer-1355b83bced1613eb048ee28d66688a8
OWNER=computer-03335285269bdba4f94377e56879f9e6
TEST=computer-0123456789abcdef0123456789abcdef

# ---- fakes ---------------------------------------------------------------
cat >"$FAKES/curl" <<'EOF'
#!/bin/sh
# fake curl: log argv, answer by URL
echo "curl $*" >>"$ST/curl.log"
url=; method=GET
for a in "$@"; do case "$a" in http*) url=$a ;; esac; done
prev=; for a in "$@"; do [ "$prev" = -X ] && method=$a; prev=$a; done
n=$(cat "$ST/list.n" 2>/dev/null || echo 0)
case "$url" in
*/internal/vmctl/list)
	n=$((n + 1)); echo $n >"$ST/list.n"
	state=active; by=
	if [ -f "$ST/wedged" ] && [ "$n" -ge 3 ]; then state=stopped; by=wedged; fi
	cat <<J
{"ownerships":[
{"vm_id":"vm-delegate","computer_id":"computer-ad1b46d4c20129e84b9e239cf4720fb9","user_id":"u0","desktop_id":"d0","state":"active","computer_url":"http://10.9.9.2:8080"},
{"vm_id":"vm-paper","computer_id":"computer-1355b83bced1613eb048ee28d66688a8","user_id":"u1","desktop_id":"d1","state":"active","warmness_class":"critical_protected","computer_url":"http://10.9.8.2:8080"},
{"vm_id":"vm-test1","computer_id":"computer-0123456789abcdef0123456789abcdef","user_id":"u2","desktop_id":"d2","state":"$state","stopped_by":"$by","warmness_class":"critical_protected","computer_url":"http://10.9.7.2:8080"}]}
J
	;;
*/internal/vmctl/deploy-refresh)
	if [ "$method" = POST ]; then
		printf '{"outcome":"skipped","reason":"the automatic retry for this deploy target is spent"}\nhttp_status=200\n'
	else
		echo '{"candidates":[{"vm_id":"vm-test1","rescue":true,"warmness_class":"critical_protected","computer_url":"http://10.9.7.2:8080"}],"skipped":[]}'
	fi
	;;
*:8083/health) echo '{"status":"ok","build":{"commit":"t1sha"}}' ;;
*:8084/health) echo '{"status":"ok","bearer_binding":{"mode":"observe","pinned":3,"unpinned":0,"would_refuse":0,"refused":0},"anthropic_relay":"off","model_scheduler":{"mode":"on","queued":0,"in_flight":0,"granted":5},"credentials":{"active":3}}' ;;
*/api/runtime/observability|*/api/runtime/observability\?*)
	c=$(cat "$ST/obs.n" 2>/dev/null || echo 0); c=$((c + 1)); echo $c >"$ST/obs.n"
	hold=$((c * 60000000000))
	cat <<J
{"service":"autoputer","computer_id":"x","build":{"commit":"t1sha"},"engine_mutex":{"GetObject choir.run":{"calls":$((c*100)),"wait_ns":$hold,"hold_ns":$hold,"max_hold_ns":1000000},"ListObjectsByMetadataPage choir.run \$.agent_id keyindex":{"calls":$((c*10)),"wait_ns":0,"hold_ns":1000000,"max_hold_ns":2000000}},
"recent_log":["store: open phase=runtime-store-open status=start","store: open phase=runtime-store-open status=complete duration=1m5.2s","derived keys run.agent_id: validated 100 rows in 1s","derived keys a: validated 5 rows in 1s","derived keys b: validated 7 rows in 1s","engineering request B waits behind A","desk cgroup /sys/fs/cgroup/choir-desks memory.max 768M"]}
J
	;;
*/api/runtime/obligations|*/api/runtime/obligations\?*) echo '{"restart":{"kind":"crash"},"runs":{},"open_work_items":0}' ;;
*/health) echo '{"status":"ok","build":{"commit":"t1sha"}}' ;;
*) echo '{}' ;;
esac
EOF
cat >"$FAKES/journalctl" <<'EOF'
#!/bin/sh
cat <<L
2026-10-22T10:00:01+0000 vmctl: wedge detector stopped vm-test1
2026-10-22T10:07:00+0000 vmctl: warmness policy spent the automatic retry for always-on desktop vm=vm-test1
2026-10-22T10:30:00+0000 vmctl: deploy target t2sha re-arms the automatic retry
2026-10-22T10:31:00+0000 vmctl: rescued VM vm-test1 (crash boot)
L
EOF
cat >"$FAKES/iptables" <<'EOF'
#!/bin/sh
echo "iptables $*" >>"$ST/seq.log"
case "$1" in
-I) touch "$ST/rule"; exit 0 ;;
-D) rm -f "$ST/rule"; exit 0 ;;
-C) [ -f "$ST/rule" ]; exit $? ;;
-S) [ -f "$ST/rule" ] && echo "-A OUTPUT -d 10.9.7.2/32 -p tcp -m tcp --dport 8080 -j REJECT" ; exit 0 ;;
esac
EOF
cat >"$FAKES/systemd-run" <<'EOF'
#!/bin/sh
echo "systemd-run $*" >>"$ST/seq.log"
EOF
cat >"$FAKES/systemctl" <<'EOF'
#!/bin/sh
case "$1" in is-active) echo active ;; *) echo "timers: none" ;; esac
EOF
cat >"$WORK/ssh" <<'EOF'
#!/bin/sh
# stub ssh: args are options, host, then the remote command; the script is on stdin
echo "ssh $*" >>"$ST/ssh.log"
PATH="$ST/fakes:$PATH" sh -s
EOF
chmod +x "$FAKES"/* "$WORK/ssh"

export PROOF_SSH_BIN=$WORK/ssh PROOF_CURL_BIN=$FAKES/curl PROOF_NAP_SCALE=0
export PATH="$FAKES:$PATH"

PASSN=0
FAILN=0
t() { # name command...  (expects exit status 0)
	local name=$1
	shift
	if "$@"; then PASSN=$((PASSN + 1)); printf 'ok   %s\n' "$name"; else FAILN=$((FAILN + 1)); printf 'FAIL %s\n' "$name"; fi
}
rc_is() { # want name command...
	local want=$1 got
	shift
	"$@" >"$WORK/out" 2>&1
	got=$?
	[ "$got" = "$want" ] || { printf '   (wanted rc %s, got %s)\n' "$want" "$got"; sed 's/^/   | /' "$WORK/out" | head -5; return 1; }
}
reset_state() { rm -f "$ST/rule" "$ST/list.n" "$ST/obs.n" "$ST/wedged" "$ST/curl.log" "$ST/ssh.log"; : >"$ST/seq.log"; }

SCRIPTS="fp1-store fp2-desk-boundary fp3-group1-wedge fp4-selfdev-admission fp7-gateway fp8-image-jailer net01-host-fetch net02-mirrors"

# ---- syntax and shellcheck -----------------------------------------------
for s in lib $SCRIPTS; do
	t "bash -n $s" bash -n "$HERE/$s.sh"
done
if command -v shellcheck >/dev/null 2>&1; then
	t "shellcheck" shellcheck -x -S warning "$HERE"/*.sh
else
	echo "skip shellcheck (not installed)"
fi

# ---- guards --------------------------------------------------------------
for s in $SCRIPTS; do
	f=$HERE/$s.sh
	t "$s refuses the delegate (read-only)" rc_is 3 "$f" --computer $DELEGATE --control $DELEGATE --t1 $DELEGATE --list
	t "$s refuses a delegate vm/hex tail" rc_is 3 "$f" --computer vm-ad1b46d4c20129e84b9e239cf4720fb9 --list
	for n in $("$f" --list | awk -F'\t' '$2 == "mut" {print $1}'); do
		t "$s step $n refuses the paper (mutating)" rc_is 3 "$f" --step "$n" --mutate --computer $PAPER --dry-run
		t "$s step $n refuses the owner computer (mutating)" rc_is 3 "$f" --step "$n" --mutate --computer $OWNER --dry-run
		t "$s step $n refuses the delegate (mutating)" rc_is 3 "$f" --step "$n" --mutate --computer $DELEGATE
		t "$s step $n needs --mutate" rc_is 2 "$f" --step "$n" --computer $TEST --target t2sha --t2 t2sha
	done
	t "$s --mutate alone is refused" rc_is 2 "$f" --mutate --computer $TEST
done

# ---- dry-run runs nothing ------------------------------------------------
reset_state
"$HERE/fp3-group1-wedge.sh" --dry-run --step 2 --mutate --computer $TEST >"$WORK/out" 2>&1
t "dry-run mutating step makes no ssh call" test ! -s "$ST/ssh.log"
t "dry-run prints the iptables command and its scheduled removal" grep -q 'systemd-run --on-active=.*iptables -I OUTPUT' <(tr '\n' ' ' <"$WORK/out" | sed 's/ | / /g')

# ---- FP3 wedge: the removal is scheduled before the insert -----------------
reset_state
touch "$ST/wedged"
"$HERE/fp3-group1-wedge.sh" --step 2 --mutate --computer $TEST --wait 60 >"$WORK/out" 2>&1
rc=$?
t "fp3 step 2 completes" test "$rc" = 0
t "fp3 step 2 schedules removal with systemd-run --on-active" grep -q 'systemd-run --on-active=1500s' "$ST/seq.log"
t "fp3 step 2 schedules the removal BEFORE the insert" bash -c "awk '/^systemd-run/ {s=NR} /^iptables -I/ {i=NR} END {exit !(s && i && s < i)}' '$ST/seq.log'"
t "fp3 step 2 removes the rule after the wedge stop" grep -q '^iptables -D' "$ST/seq.log"
t "fp3 step 2 left no rule" test ! -f "$ST/rule"
t "fp3 step 2 prints a WILL block first" bash -c "grep -n ' WILL ' '$WORK/out' | head -1 | grep -q ."
t "fp3 step 2 saw the wedge stop" grep -q 'PASS.*stopped by wedged' "$WORK/out"
t "fp3 step 2 never touched the delegate/paper rows" bash -c "! grep -q 'vm-delegate\|vm-paper' '$WORK/out'"
reset_state
"$HERE/fp3-group1-wedge.sh" --step 3 --computer $TEST >"$WORK/out" 2>&1
t "fp3 step 3 reads the sweep's retry" grep -q 'PASS.*spent the automatic retry' "$WORK/out"
reset_state
"$HERE/fp3-group1-wedge.sh" --step 1 --computer $TEST --t1 t1sha >"$WORK/out" 2>&1
t "fp3 step 1 baseline passes" bash -c "grep -q 'PASS.*deploy-refresh candidate' '$WORK/out' && ! grep -q ' FAIL ' '$WORK/out'"

# ---- FP6 deploy-refresh POST is a mutating step with a printed body ---------
reset_state
touch "$ST/wedged"
"$HERE/fp3-group1-wedge.sh" --step 6 --mutate --computer $TEST --t2 t2sha --wait 60 >"$WORK/out" 2>&1
t "fp3 step 6: repeated target answers skipped" grep -q 'PASS.*outcome is skipped' "$WORK/out"
t "fp3 step 6 prints the POST it makes" grep -q 'POST http://127.0.0.1:8083/internal/vmctl/deploy-refresh body=' "$WORK/out"

# ---- the secrets stay out of the output and argv -----------------------------
reset_state
CHOIR_API_KEY=SECRETKEY-abc123 "$HERE/fp2-desk-boundary.sh" --step 1 --computer $TEST --commit t1sha >"$WORK/out" 2>&1
t "fp2 step 1 passes against the stub" grep -q 'PASS.*the guest runs t1sha' "$WORK/out"
t "the API key is not in the output" bash -c "! grep -q SECRETKEY '$WORK/out'"
t "the API key is not in curl's argv" bash -c "! grep -q SECRETKEY '$ST/curl.log'"
t "the internal-service header file's contents are never read here" bash -c "! grep -q 'internal-service-token.header' '$WORK/out' || grep -q '@/var/lib/go-choir/secrets/internal-service-token.header' '$WORK/out'"

# ---- FP1 reading --------------------------------------------------------------
reset_state
"$HERE/fp1-store.sh" --step 1 --computer $TEST --window 0 >"$WORK/out" 2>&1
t "fp1 step 1 computes a busy share" grep -q 'engine mutex held' "$WORK/out"
reset_state
"$HERE/fp1-store.sh" --step 3 --computer $TEST >"$WORK/out" 2>&1
t "fp1 step 3 counts three validated families" grep -q 'PASS.*3 .validated N rows in T. lines' "$WORK/out"
reset_state
"$HERE/fp1-store.sh" --step 2 --computer $TEST >"$WORK/out" 2>&1
t "fp1 step 2 parses a Go duration (1m5.2s < 2m)" grep -q 'PASS.*runtime-store-open under two minutes' "$WORK/out"
t "fp1 step 6 refuses the paper by default" rc_is 3 "$HERE/fp1-store.sh" --step 6 --mutate

# ---- FP1 F-c deploy 1: step 6's messages are text, not shell (review bunch 99) -
EVT=$HERE/fp1-event-table-deploy1.sh
t "bash -n fp1-event-table-deploy1" bash -n "$EVT"
for phase in old recovered; do
	reset_state
	t "fp1 event-table step 6 --phase $phase runs (dry run)" rc_is 0 "$EVT" --step 6 --phase "$phase" --mutate --computer $TEST --dry-run
done
t "fp1 event-table step 6 recovered prints the rollback sequence literally" grep -qF 'T=$(mktemp -d); git show TIP:scripts/rollback/fc-deploy1-rollback.sh >$T/r.sh' "$WORK/out"
t "fp1 event-table step 6 --phase is old or recovered" rc_is 2 "$EVT" --step 6 --phase bogus --mutate --computer $TEST --dry-run

# ---- FP7 -------------------------------------------------------------------------
reset_state
TMPDIR=$WORK "$HERE/fp7-gateway.sh" --step 1 >"$WORK/out" 2>&1
t "fp7 step 1 reads observe mode" grep -q 'PASS.*bearer_binding.mode is observe' "$WORK/out"
TMPDIR=$WORK "$HERE/fp7-gateway.sh" --step 2 >"$WORK/out" 2>&1
t "fp7 step 2 would_refuse unchanged" grep -q 'PASS.*would_refuse did not rise' "$WORK/out"

# ---- FP8 step 2's probe: an empty or malformed mint fails loudly -------------
for mode in empty garbage nonzero; do
	mkdir -p "$WORK/mintfake-$mode"
	case $mode in
	empty) body='exit 0' ;;
	garbage) body='echo "not json at all"; exit 0' ;;
	nonzero) body='echo "boom" >&2; exit 3' ;;
	esac
	printf '#!/bin/sh\n%s\n' "$body" >"$WORK/mintfake-$mode/node"
	chmod +x "$WORK/mintfake-$mode/node"
	PATH="$WORK/mintfake-$mode:$PATH" python3 -I "$HERE/../memory_hotplug_probe.py" --out "$WORK/hp-$mode.json" >"$WORK/out" 2>&1
	rc=$?
	t "memory_hotplug_probe with $mode mint output exits nonzero" test "$rc" -ne 0
	t "memory_hotplug_probe with $mode mint output names the mint, no traceback" bash -c "grep -qi 'mint' '$WORK/out' && ! grep -q Traceback '$WORK/out'"
	t "memory_hotplug_probe with $mode mint output writes no passing receipt" bash -c "! grep -q '\"pass\": true' '$WORK/hp-$mode.json' 2>/dev/null"
done

printf '\n%d passed, %d failed\n' "$PASSN" "$FAILN"
[ "$FAILN" -eq 0 ]
