#!/usr/bin/env bash
# Self-test of the chaos harness's own safety guards, with fakes: no network,
# no ssh, no Node B. `ssh node-b` is a stub that runs the script it receives
# locally under fake curl, systemctl, systemd-run, psql, flock, df, ps and
# kill, against fixture ownerships.json, /proc and console logs. It proves
# the guards (paper, owner and unmapped VMs refused; pre-flight refusals; the
# kill switch; traps undo faults; dry-run injects nothing; the key and the
# internal token stay where they belong) and the artifact shape, not staging
# behavior.
#
#   scripts/chaos/selftest.sh        (exit 0 when every case passes)
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/chaos-selftest.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
SECONDS=0
FAKES=$WORK/fakes
mkdir -p "$FAKES"

T=computer-59c902c5d46caecc0a180a8557164e72
PAPER=computer-1355b83bced1613eb048ee28d66688a8
OWNER=computer-03335285269bdba4f94377e56879f9e6
DELEGATE=computer-ad1b46d4c20129e84b9e239cf4720fb9
KEY=sk-chaos-selftest-SECRET-7731

# ---- fakes ---------------------------------------------------------------
cp "$HERE/testdata/fake_curl.py" "$FAKES/curl"
chmod +x "$FAKES/curl"
cat >"$FAKES/ssh" <<'EOF'
#!/bin/sh
# fake ssh: drop options and the host, run the script from stdin locally
while [ $# -gt 0 ] && [ "$1" != sh ]; do shift; done
PATH="$FAKES:$PATH" exec sh -s
EOF
cat >"$FAKES/kill" <<'EOF'
#!/bin/sh
echo "kill $*" >>"$ST/seq.log"
: >"$ST/killed"
echo "autoputer: restart kind=crash_or_stop" >>"$FAKE_CONSOLE"
[ -e "$ST/switch_on_fault" ] && : >"$FAKE_SWITCH"
exit 0
EOF
cat >"$FAKES/systemctl" <<'EOF'
#!/bin/sh
echo "systemctl $*" >>"$ST/seq.log"
case "$1" in
is-active)
	if [ -e "$ST/corpusd_dead" ]; then [ "$2" = --quiet ] || echo inactive; exit 3; fi
	[ "$2" = --quiet ] || echo active ;;
kill)
	: >"$ST/killed"
	[ -e "$ST/stay_dead" ] && : >"$ST/corpusd_dead"
	[ -e "$ST/switch_on_fault" ] && : >"$FAKE_SWITCH" ;;
start) rm -f "$ST/corpusd_dead" ;;
esac
exit 0
EOF
cat >"$FAKES/systemd-run" <<'EOF'
#!/bin/sh
case " $* " in *" --on-active="*) echo "systemd-run deadman $*" >>"$ST/seq.log"; exit 0 ;; esac
while [ $# -gt 0 ]; do case "$1" in -p) shift 2 ;; -*) shift ;; *) break ;; esac; done
exec "$@"
EOF
cat >"$FAKES/runuser" <<'EOF'
#!/bin/sh
while [ $# -gt 0 ] && [ "$1" != -- ]; do shift; done
shift
exec "$@"
EOF
cat >"$FAKES/psql" <<'EOF'
#!/bin/sh
for a in "$@"; do sql=$a; done
case "$sql" in
*"count(distinct"*) echo "5 5" ;;
*"c1:"*) echo 5 ;;
*"min(sequence)"*) if [ -e "$ST/killed" ]; then echo "15 1 15"; else echo "10 1 10"; fi ;;
esac
EOF
cat >"$FAKES/flock" <<'EOF'
#!/bin/sh
[ -e "$ST/gate_held" ] && exit 1
exit 0
EOF
cat >"$FAKES/df" <<'EOF'
#!/bin/sh
printf 'Avail\n%s\n' "$(cat "$ST/disk" 2>/dev/null || echo 200000000000)"
EOF
cat >"$FAKES/ps" <<'EOF'
#!/bin/sh
cat "$ST/ps.txt" 2>/dev/null
EOF
chmod +x "$FAKES"/*

# ---- fixtures --------------------------------------------------------------
fc_proc() { # PID ARGV0 VM
	mkdir -p "$ST/proc/$1"
	printf '%s\0--api-sock\0/x/fc.sock\0--id\0%s\0--config-file\0/x/fc.json\0' "$2" "$3" >"$ST/proc/$1/cmdline"
}
own_rows() { # JSON array body of ownerships
	printf '{"epoch_counter":1,"ownerships":[%s]}\n' "$1" >"$ST/ownerships.json"
}
ROW_DE='{"vm_id":"vm-de","computer_id":"'$T'","user_id":"u-de","desktop_id":"primary","computer_url":"http://10.201.167.2:8085","state":"active","reported_release":{"build_commit":"b1"}}'
ROW_PAPER='{"vm_id":"vm-1669020adcece51c2f5ff9b6df399ef5","computer_id":"'$PAPER'","user_id":"u-p","computer_url":"http://10.201.169.2:8085","state":"active"}'
ROW_OWNER='{"vm_id":"candidate-fleet-e15cb89f25d963c220319b7b","computer_id":"'$OWNER'","user_id":"u-o","computer_url":"http://10.201.168.2:8085","state":"active"}'

fixture() {
	ST=$(mktemp -d "$WORK/st.XXXXXX")
	export ST FAKES
	mkdir -p "$ST/vmstate/vm-de" "$ST/artifacts/tape-seg/$T" "$ST/results"
	own_rows "$ROW_DE,$ROW_PAPER,$ROW_OWNER"
	fc_proc 111 /nix/store/x-firecracker/bin/firecracker vm-de
	fc_proc 222 /nix/store/x-firecracker/bin/firecracker vm-1669020adcece51c2f5ff9b6df399ef5
	fc_proc 333 /firecracker candidate-fleet-e15cb89f25d963c220319b7b
	echo "MemAvailable:   20000000 kB" >"$ST/meminfo"
	printf 'boot\nautoputer: restart kind=planned reason=old\n' >"$ST/vmstate/vm-de/console.log"
	echo segment >"$ST/artifacts/tape-seg/$T/seg-00000001.seg"
	printf 'OTHER=1\nCHOIR_TEST_D_E_API_KEY=%s\n' "$KEY" >"$ST/env"
	echo 'Authorization-free header for the fake token' >"$ST/token.header"
	: >"$ST/seq.log"
	: >"$ST/curl.log"
	export FAKE_CONSOLE=$ST/vmstate/vm-de/console.log FAKE_SWITCH=$ST/kill-switch FAKE_KEY=$KEY
}

chaos() { # args...: run the harness against the fixture; output in $ST/out
	env CHAOS_SSH_BIN="$FAKES/ssh" CHAOS_CURL_BIN="$FAKES/curl" CHAOS_ENV_FILE="$ST/env" \
		CHAOS_RESULTS_DIR="$ST/results" CHAOS_KILL_SWITCH="$ST/kill-switch" CHAOS_NAP_SCALE=0 \
		CHAOS_LOAD_MAX=4 CHAOS_S1_TIMEOUT=2 CHAOS_S3_TIMEOUT=1 \
		CHAOS_R_OWN="$ST/ownerships.json" CHAOS_R_VMSTATE="$ST/vmstate" CHAOS_R_PROC="$ST/proc" \
		CHAOS_R_MEMINFO="$ST/meminfo" CHAOS_R_STOP="$ST/remote-stop" CHAOS_R_GATE_LOCK="$ST/gate.lock" \
		CHAOS_R_KILL="$FAKES/kill" CHAOS_R_ARTIFACTS="$ST/artifacts" CHOIR_INTERNAL_TOKEN_HEADER="$ST/token.header" \
		BASH_ENV= "$HERE/run" "$@" >"$ST/out" 2>&1
	RC=$?
}
artifact() { find "$ST/results" -name artifact.json | head -1; }

PASSED=0 FAILED=0
ok() { PASSED=$((PASSED + 1)); printf 'ok   %s\n' "$1"; }
bad() { FAILED=$((FAILED + 1)); printf 'FAIL %s\n' "$1"; sed 's/^/     | /' "$ST/out" | tail -25; }
expect_rc() { if [ "$RC" = "$2" ]; then ok "$1 (exit $RC)"; else bad "$1: exit $RC, want $2"; fi; }
expect_out() { if grep -q -- "$2" "$ST/out"; then ok "$1"; else bad "$1: output lacks '$2'"; fi; }
expect_seq() { if grep -q -- "$2" "$ST/seq.log"; then ok "$1"; else bad "$1: seq.log lacks '$2'"; fi; }
expect_noseq() { if grep -q -- "$2" "$ST/seq.log"; then bad "$1: seq.log has '$2'"; else ok "$1"; fi; }
no_fault() { # no kill, no corpusd kill, no deploy-refresh
	if grep -qE '^kill |systemctl kill|deploy-refresh POST' "$ST/seq.log"; then bad "$1: a fault was injected"; else ok "$1: nothing injected"; fi
}

# ---- 1. the paper, the owner, the delegate and any non-d@e id are refused ----
for id in "$PAPER" "$OWNER" "$DELEGATE" candidate-fleet-e15cb89f25d963c220319b7b vm-1669020adcece51c2f5ff9b6df399ef5 computer-0123456789abcdef0123456789abcdef "${T}x"; do
	for s in s1-guest-crash s2-corpusd-crash s3-planned-restart; do
		fixture
		chaos "$s" --execute --computer "$id"
		expect_rc "$s refuses --computer $id" 3
		no_fault "$s with $id"
	done
done
fixture; chaos s1-guest-crash --execute --computer "$PAPER"; expect_out "refusal names the paper" "names the paper"
fixture; chaos s1-guest-crash --execute --computer "$OWNER"; expect_out "refusal names the owner" "names the owner"

# ---- 2. a VM that does not map positively to d@e is refused -------------------
unmapped() { # label, then the fixture is already altered
	chaos s1-guest-crash --execute
	expect_rc "unmapped: $1" 3
	no_fault "unmapped: $1"
}
fixture; own_rows "$ROW_PAPER,$ROW_OWNER"; unmapped "no d@e row"
fixture; own_rows "$ROW_DE,$ROW_DE,$ROW_PAPER"; unmapped "two d@e rows"
fixture; own_rows "$(printf '%s' "$ROW_DE" | sed 's/"vm-de"/"vm-1669020adcece51c2f5ff9b6df399ef5"/'),$(printf '%s' "$ROW_PAPER" | sed 's/"vm-1669020adcece51c2f5ff9b6df399ef5"/"vm-paper-2"/'),$ROW_OWNER"; unmapped "d@e row naming the paper's vm"
fixture; own_rows "$(printf '%s' "$ROW_DE" | sed 's/"vm-de"/"candidate-fleet-e15cb89f25d963c220319b7b"/'),$ROW_PAPER"; unmapped "d@e row naming the owner's vm"
fixture; own_rows "$ROW_DE,$(printf '%s' "$ROW_PAPER" | sed 's/"vm-1669020adcece51c2f5ff9b6df399ef5"/"vm-de"/')"; unmapped "d@e's vm also named by the paper's row"
fixture; rm -rf "$ST/proc/111"; unmapped "no firecracker runs d@e's vm"
fixture; fc_proc 444 /nix/store/x-firecracker/bin/firecracker vm-de; unmapped "two firecrackers run d@e's vm"
fixture; rm -rf "$ST/proc/111"; fc_proc 111 /bin/sleep vm-de; unmapped "a non-firecracker process with --id vm-de"
fixture; rm -rf "$ST/proc/111"; fc_proc 111 /nix/store/x-firecracker/bin/firecracker vm-de-2; unmapped "a firecracker whose --id only starts with vm-de"

# ---- 3. pre-flight refusals inject nothing ---------------------------------------
pre() { chaos s1-guest-crash --execute; expect_rc "pre-flight: $1" 4; no_fault "pre-flight: $1"; }
fixture; echo "MemAvailable:   4000000 kB" >"$ST/meminfo"; pre "MemAvailable below 8 GB"
fixture; echo 50000000000 >"$ST/disk"; pre "disk free below 60 GB"
fixture; : >"$ST/paper_down"; pre "paper unhealthy"
fixture; own_rows "$ROW_DE,$ROW_OWNER"; pre "paper absent"
fixture; : >"$ST/gate.lock"; : >"$ST/gate_held"; pre "deploy holds the CI gate lock"
fixture; echo "bash /nix/store/x/node-b-ci-deploy" >"$ST/ps.txt"; pre "deploy process running"
fixture; : >"$ST/remote-stop"; pre "Node B kill switch"
fixture; : >"$ST/kill-switch"; pre "local kill switch"

# ---- 4. dry-run injects nothing and still writes an artifact -----------------------
for s in s1-guest-crash s2-corpusd-crash s3-planned-restart; do
	fixture
	chaos "$s"
	expect_rc "$s dry-run" 0
	no_fault "$s dry-run"
	if [ "$(jq -r '.mode + " " + (.pass|tostring) + " " + (.checks|length|tostring)' "$(artifact)")" = "dry-run true $(jq '.checks|length' "$(artifact)")" ] &&
		[ "$(jq '[.checks[] | select(.pass == null)] | length' "$(artifact)")" -gt 0 ]; then ok "$s dry-run artifact lists checks unjudged"; else bad "$s dry-run artifact"; fi
	if grep -q 'choir.news' "$ST/curl.log"; then bad "$s dry-run called the d@e API"; else ok "$s dry-run made no d@e API call"; fi
done

# ---- 5. execute paths with fakes ---------------------------------------------------
fixture; chaos s1-guest-crash --execute
expect_rc "s1 execute (fakes)" 0
expect_seq "s1 kills exactly the mapped pid" "kill -KILL 111"
expect_noseq "s1 never signals the paper's or owner's pid" "kill -KILL 2"
if [ "$(jq -r '.pass' "$(artifact)")" = true ] && [ "$(jq '[.checks[] | select(.pass != true)] | length' "$(artifact)")" = 0 ]; then ok "s1 artifact passes, every check judged"; else bad "s1 artifact"; fi
if jq -e '.checks[] | select(.name | test("may be missing")) | .pass' "$(artifact)" >/dev/null; then ok "s1 checks the 'may be missing' note"; else bad "s1 note check"; fi
if grep -q "$KEY" "$ST/out" "$ST/curl.log" "$(artifact)"; then bad "the d@e key appears in output, argv or artifact"; else ok "the d@e key is never printed or on a command line"; fi
if [ "$(jq -Rn '[inputs | fromjson? | select(.url | test("choir.news"))] | length > 0 and all(.[]; .auth == "ok")' "$ST/curl.log")" = true ]; then ok "every d@e API call carried the key by descriptor"; else bad "a d@e API call lacked the key"; fi

fixture; chaos s2-corpusd-crash --execute
expect_rc "s2 execute (fakes)" 0
if awk '/systemd-run deadman/ && !d { d = NR } /systemctl kill/ { k = NR } END { exit !(d && k && d < k) }' "$ST/seq.log"; then ok "s2 schedules the dead-man restart before the kill"; else bad "s2 dead-man order"; fi

fixture; chaos s3-planned-restart --execute
expect_rc "s3 execute (fakes)" 0
if [ "$(jq -Rn '[inputs | fromjson? | select(.url | test(":8083/internal/vmctl/deploy-refresh"))] | length > 0 and all(.[]; .auth == "internal-token")' "$ST/curl.log")" = true ]; then ok "s3 vmctl call carries the internal token (loopback)"; else bad "s3 vmctl token: $(grep 8083 "$ST/curl.log")"; fi

# ---- 6. traps undo faults --------------------------------------------------------------
fixture; : >"$ST/switch_on_fault"; : >"$ST/stay_dead"; chaos s2-corpusd-crash --execute
expect_rc "s2 kill switch during the wait aborts" 4
expect_seq "s2 undo starts corpusd again" "systemctl start go-choir-corpusd"
expect_seq "s2 undo stops the dead-man timer" "systemctl stop choir-chaos-corpusd-deadman-"
if [ "$(jq -r '.aborted != null and .pass == false' "$(artifact)")" = true ] && [ "$(jq '.fault.undone | length' "$(artifact)")" -ge 2 ]; then ok "s2 abort artifact records the abort and the undos"; else bad "s2 abort artifact"; fi

fixture; : >"$ST/switch_on_fault"; chaos s1-guest-crash --execute
expect_rc "s1 kill switch after the kill aborts" 4
if awk '/"url"/ && /api\/compute\/status/ { n++ } END { exit !(n >= 1) }' "$ST/curl.log" && grep -q 'undo: ask for d@e' "$ST/out"; then ok "s1 undo asks for d@e again"; else bad "s1 undo wake"; fi
n1=$(grep -c '/api/files/chaos/.*w[0-9]' "$ST/curl.log"); sleep 1; n2=$(grep -c '/api/files/chaos/.*w[0-9]' "$ST/curl.log")
if [ "$n1" = "$n2" ]; then ok "s1 undo stopped the writer"; else bad "s1 writer still running after abort"; fi

fixture; : >"$ST/refresh_defer"; chaos s3-planned-restart --execute
expect_rc "s3 deferred refresh with no planned boot fails its checks" 1
expect_seq "s3 undo clears the guest marker" "guest-marker DELETE"
if grep -q 'guest-marker DELETE auth=none' "$ST/seq.log"; then ok "s3 guest call carries no internal token"; else bad "s3 guest call token"; fi

printf '\n%s passed, %s failed (%ss)\n' "$PASSED" "$FAILED" "$SECONDS"
[ "$FAILED" = 0 ]
