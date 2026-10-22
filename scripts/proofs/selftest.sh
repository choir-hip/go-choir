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
*:8083/health)
	# Real vmctl's /health body has no build field; its identity is the
	# X-Choir-Build-Commit header (server.WithBuildIdentity). With -D the
	# headers go to stdout; $ST/vmctlhdr: none (no build headers), other
	# (another service's headers), 401 (vmctl's headers on a 401; the build
	# headers are set on every response). -w's %{http_code} is honored.
	dump=; quiet=; wout=; prev=
	for a in "$@"; do [ "$prev" = -D ] && [ "$a" = - ] && dump=1; [ "$prev" = -o ] && [ "$a" = /dev/null ] && quiet=1; [ "$prev" = -w ] && wout=$a; prev=$a; done
	code=200; [ "$(cat "$ST/vmctlhdr" 2>/dev/null)" = 401 ] && code=401
	if [ -n "$dump" ]; then
		printf 'HTTP/1.1 %s X\r\nContent-Type: application/json\r\n' "$code"
		case "$(cat "$ST/vmctlhdr" 2>/dev/null)" in
		none) ;;
		other) printf 'X-Choir-Build-Commit: t1sha\r\nX-Choir-Build-Service: proxy\r\n' ;;
		*) printf 'X-Choir-Build-Commit: t1sha\r\nX-Choir-Build-Service: vmctl\r\n' ;;
		esac
		printf '\r\n'
	fi
	[ -n "$quiet" ] || echo '{"status":"ok","service":"vmctl","active_vms":2}'
	[ -z "$wout" ] || printf '%b' "$(printf '%s' "$wout" | sed "s/%{http_code}/$code/")"
	;;
*/internal/vmctl/retention-shadow-plan)
	cat <<J
{"mode":"dry-run","decision":"would_prune","config":{"mode":"dry-run","test_idle_mode":"dry-run","test_idle_min_age_seconds":604800,"quarantine_mode":"dry-run","quarantine_min_age_seconds":604800},
"inventory":{"test_class_computers":2,"test_class_bytes":3,"test_class_would_remove":1,"test_class_would_remove_bytes":1,"quarantines":1,"quarantine_bytes":2,"quarantine_would_remove":0},
"test_class":[{"vm_id":"vm-test1","computer_id":"computer-0123456789abcdef0123456789abcdef","state":"hibernated","idle_seconds":900000,"size_bytes":1,"verdict":"would_remove"},
{"vm_id":"vm-test2","computer_id":"computer-1111111111111111111111111111111f","state":"active","idle_seconds":60,"size_bytes":2,"verdict":"keep","reasons":["state_active","idle_below_min_age"]}],
"quarantines":[{"vm_id":"candidate-fleet","path":"/var/lib/go-choir/vm-state/candidate-fleet/data.img.quarantine-1-ab","quarantined_at":"2026-10-07T00:00:00Z","age_seconds":1300000,"size_bytes":2,"verdict":"keep","reasons":["journal_missing"]}]}
J
	;;
*/internal/vmctl/retention-plan)
	# $ST/planmode: fail (curl -f error, no body), garbage (not JSON),
	# shape (JSON, not a plan), empty (a plan with no candidates)
	case "$(cat "$ST/planmode" 2>/dev/null)" in
	fail) exit 22 ;;
	garbage) echo 'upstream connect error'; exit 0 ;;
	shape) echo '{"error":"not found"}'; exit 0 ;;
	empty) echo '{"mode":"active","decision":"observe","config":{"mode":"active","test_idle_mode":"off","quarantine_mode":"off"},"inventory":{"state_dirs":2,"kept_state_dirs":1},"kept_state_dirs":[{"vm_id":"candidate-fleet-x","rule":"orphan_state_dir","size_bytes":1,"reasons":["holds_quarantine"],"release":"an owner act"}]}'; exit 0 ;;
	esac
	if [ -f "$ST/badplan" ]; then
		echo '{"mode":"active","decision":"will_prune","config":{"mode":"active","test_idle_mode":"off","quarantine_mode":"off"},"inventory":{"state_dirs":2},"candidates":[{"vm_id":"candidate-fleet-x","reason":"orphan_state_dir"}]}'
	else
		echo '{"mode":"active","decision":"observe","config":{"mode":"active","test_idle_mode":"off","quarantine_mode":"off"},"inventory":{"state_dirs":2,"kept_state_dirs":1},"candidates":[{"vm_id":"vm-plain","reason":"orphan_state_dir"}],"kept_state_dirs":[{"vm_id":"candidate-fleet-x","rule":"orphan_state_dir","size_bytes":1,"reasons":["holds_quarantine"],"release":"an owner act"}]}'
	fi
	;;
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
reset_state() { rm -f "$ST/rule" "$ST/list.n" "$ST/obs.n" "$ST/wedged" "$ST/curl.log" "$ST/ssh.log" "$ST/vmctlhdr"; : >"$ST/seq.log"; }

SCRIPTS="fp0-vm-retention fp1-store fp2-desk-boundary fp3-group1-wedge fp4-selfdev-admission fp7-gateway fp8-image-jailer net01-host-fetch net02-mirrors"

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
# R6-5: fp3 step 1 fails when port 8083 answers as another service, and
# (R6-4) when vmctl's /health answers non-2xx.
reset_state
echo other >"$ST/vmctlhdr"
"$HERE/fp3-group1-wedge.sh" --step 1 --computer $TEST --t1 t1sha >"$WORK/out" 2>&1
t "fp3 step 1 fails when port 8083 answers as another service" grep -q 'FAIL.*does not name T1 t1sha.*service=proxy' "$WORK/out"
reset_state
echo 401 >"$ST/vmctlhdr"
"$HERE/fp3-group1-wedge.sh" --step 1 --computer $TEST --t1 t1sha >"$WORK/out" 2>&1
t "fp3 step 1 fails when vmctl's /health answers non-2xx" grep -q 'FAIL.*does not name T1 t1sha.*status=401' "$WORK/out"

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

# ---- FP0 VM retention: the shadow report, read-only -----------------------
mkdir -p "$WORK/vmstate/candidate-fleet-x" "$WORK/vmstate/vm-plain"
touch "$WORK/vmstate/candidate-fleet-x/data.img.quarantine-1-a"
reset_state
"$HERE/fp0-vm-retention.sh" --commit t1sha --state-dir "$WORK/vmstate" >"$WORK/out" 2>&1
rc=$?
t "fp0-vm-retention read-only pass completes" test "$rc" = 0
t "fp0-vm-retention reads both shadow rules in dry-run" grep -q 'PASS.*shadow quarantine_mode is dry-run' "$WORK/out"
t "fp0-vm-retention lists a kept test computer with its reasons" grep -q 'test-class computer-1111111111111111111111111111111f.*verdict=keep reasons=state_active,idle_below_min_age' "$WORK/out"
t "fp0-vm-retention lists the quarantine and why it is kept" grep -q 'quarantine .*data.img.quarantine-1-ab.*verdict=keep reasons=journal_missing' "$WORK/out"
t "fp0-vm-retention checks the active plan is unchanged" grep -q 'PASS.*no test_class_idle candidate in the active plan' "$WORK/out"
# R5-1: vmctl's identity is its X-Choir-Build-Commit header, not a body field.
t "fp0-vm-retention step 0 proves vmctl runs the commit from its build header" grep -q 'PASS.*vmctl runs t1sha.*observed commit=t1sha service=vmctl status=200' "$WORK/out"
# R6-3: each negative case asserts what was observed, so it fails on a
# script that always fails. R6-4: a non-2xx answer fails.
reset_state
"$HERE/fp0-vm-retention.sh" --step 0 --commit othersha >"$WORK/out" 2>&1
t "fp0-vm-retention step 0 fails when vmctl's header names another commit" grep -q 'FAIL.*vmctl runs othersha.*observed commit=t1sha service=vmctl status=200' "$WORK/out"
reset_state
echo none >"$ST/vmctlhdr"
"$HERE/fp0-vm-retention.sh" --step 0 --commit t1sha >"$WORK/out" 2>&1
t "fp0-vm-retention step 0 fails when vmctl sends no build header" grep -q 'FAIL.*vmctl runs t1sha.*observed commit=- service=- status=200' "$WORK/out"
reset_state
echo other >"$ST/vmctlhdr"
"$HERE/fp0-vm-retention.sh" --step 0 --commit t1sha >"$WORK/out" 2>&1
t "fp0-vm-retention step 0 fails when port 8083 answers as another service" grep -q 'FAIL.*vmctl runs t1sha.*observed commit=t1sha service=proxy status=200' "$WORK/out"
reset_state
echo 401 >"$ST/vmctlhdr"
"$HERE/fp0-vm-retention.sh" --step 0 --commit t1sha >"$WORK/out" 2>&1
t "fp0-vm-retention step 0 fails when vmctl's /health answers non-2xx" grep -q 'FAIL.*vmctl runs t1sha.*observed commit=t1sha service=vmctl status=401' "$WORK/out"
reset_state
"$HERE/fp0-vm-retention.sh" --step 4 --state-dir "$WORK/vmstate" >"$WORK/out" 2>&1
t "fp0-vm-retention step 4 lists the recovery dir" grep -q 'recovery-dir candidate-fleet-x artifacts=1' "$WORK/out"
t "fp0-vm-retention step 4 shows the kept-dir report" grep -q 'kept candidate-fleet-x rule=orphan_state_dir.*reasons=holds_quarantine' "$WORK/out"
t "fp0-vm-retention step 4 passes when no recovery dir is a candidate" grep -q 'PASS.*no dir holding recovery state is a deletion candidate' "$WORK/out"
# R5-2: the before-deploy run is a precondition, said plainly.
t "fp0-vm-retention step 4 says a run before the deploy is required" grep -q 'MANUAL.*a run of this step BEFORE the deploy is required' "$WORK/out"
reset_state
touch "$ST/badplan"
"$HERE/fp0-vm-retention.sh" --step 4 --state-dir "$WORK/vmstate" >"$WORK/out" 2>&1
t "fp0-vm-retention step 4 fails when a recovery dir is a candidate" grep -q 'FAIL.*candidates: candidate-fleet-x' "$WORK/out"
rm -f "$ST/badplan"
# R4-1: step 4 never passes without having compared something.
step4_run() { # planmode state-dir
	reset_state
	if [ -n "$1" ]; then echo "$1" >"$ST/planmode"; else rm -f "$ST/planmode"; fi
	"$HERE/fp0-vm-retention.sh" --step 4 --state-dir "$2" >"$WORK/out" 2>&1
	echo $? >"$WORK/rc"
	rm -f "$ST/planmode"
}
no_step4_pass() { ! grep -q 'PASS.*no dir holding recovery state is a deletion candidate' "$WORK/out"; }
mkdir -p "$WORK/vmstate-empty" "$WORK/vmstate-clean/vm-plain"
for mode in fail garbage shape; do
	step4_run "$mode" "$WORK/vmstate"
	t "fp0-vm-retention step 4 fails on a $mode plan read" grep -q 'FAIL.*the active plan' "$WORK/out"
	t "fp0-vm-retention step 4 exits 1 on a $mode plan read" test "$(cat "$WORK/rc")" = 1
	t "fp0-vm-retention step 4 does not pass on a $mode plan read" no_step4_pass
done
step4_run "" "$WORK/no-such-dir"
t "fp0-vm-retention step 4 fails on an unreadable state dir" grep -q 'FAIL.*the state dir' "$WORK/out"
t "fp0-vm-retention step 4 exits 1 on an unreadable state dir" test "$(cat "$WORK/rc")" = 1
t "fp0-vm-retention step 4 does not pass on an unreadable state dir" no_step4_pass
step4_run "" "$WORK/vmstate-empty"
t "fp0-vm-retention step 4 fails on a state dir with none of the plan's dirs" grep -q 'FAIL.*the state dir' "$WORK/out"
t "fp0-vm-retention step 4 does not pass on a state dir with none of the plan's dirs" no_step4_pass
step4_run empty "$WORK/vmstate"
t "fp0-vm-retention step 4 says nothing to check when the plan has no candidates" grep -q 'nothing to check.*no candidates' "$WORK/out"
t "fp0-vm-retention step 4 does not pass when the plan has no candidates" no_step4_pass
step4_run "" "$WORK/vmstate-clean"
t "fp0-vm-retention step 4 says nothing to check when no dir holds recovery state" grep -q 'nothing to check.*no dir holds recovery state' "$WORK/out"
t "fp0-vm-retention step 4 does not pass when no dir holds recovery state" no_step4_pass
t "fp0-vm-retention has no mutating step" bash -c "! '$HERE/fp0-vm-retention.sh' --list | awk -F'\t' '\$2 == \"mut\"' | grep -q ."

printf '\n%d passed, %d failed\n' "$PASSN" "$FAILN"
[ "$FAILN" -eq 0 ]
