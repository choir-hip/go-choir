#!/usr/bin/env bash
# Shared helpers for the chaos drills (scripts/chaos/run). Sourced, never
# executed. Runs on the owner's Mac (bash 3.2 is enough); Node B is reached
# through `ssh node-b`, as root, with scripts on stdin (scripts/proofs/lib.sh).
#
# Safety, enforced here and proved by scripts/chaos/selftest.sh:
#   - the only target is d@e (CHAOS_TARGET). Any other id is refused (exit 3);
#     the paper, the owner's computer (and its VM candidate-fleet-e15c...) and
#     the delegate are named in the refusal. A value carrying one of their
#     hex ids anywhere is refused too.
#   - a VM is acted on only after it maps to d@e by exactly one row of
#     /var/lib/go-choir/vm-state/ownerships.json, and a process only after its
#     cmdline is firecracker with `--id <that vm>` and it is the only one. The
#     mapping is checked again on Node B in the same script that acts.
#   - the internal-service token header goes only to loopback host services
#     (scripts/lib/choir_internal.sh); guest ports get no token.
#   - the d@e key is read from a file into a shell variable and handed to curl
#     through a descriptor (-H @<(printf ...)); it is never printed or put on a
#     command line.
#   - every fault registers its undo first; the EXIT/INT/TERM trap runs them.
#   - the kill switch (a local file, or /run/choir-chaos-stop on Node B) aborts
#     before injection and inside every wait; the undos still run.
#   - dry-run injects nothing and writes nothing to d@e: it runs the read-only
#     Node B probes (pre-flight, mapping) and prints every other step.
#
# Exit status: 0 every check passed, 1 a check failed, 2 usage, 3 refused
# target, 4 pre-flight refused or kill switch.

CHAOS_TARGET=computer-59c902c5d46caecc0a180a8557164e72
CHAOS_TARGET_HEX=59c902c5d46caecc0a180a8557164e72
CHAOS_PAPER=computer-1355b83bced1613eb048ee28d66688a8
CHAOS_PAPER_HEX=1355b83bced1613eb048ee28d66688a8
CHAOS_OWNER_HEX=03335285269bdba4f94377e56879f9e6
CHAOS_DELEGATE_HEX=ad1b46d4c20129e84b9e239cf4720fb9
CHAOS_OWNER_VM_PREFIX=candidate-fleet-e15c
CHAOS_PAPER_VM=vm-1669020adcece51c2f5ff9b6df399ef5

CHAOS_HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
CHAOS_SSH_BIN=${CHAOS_SSH_BIN:-ssh}
CHAOS_CURL_BIN=${CHAOS_CURL_BIN:-curl}
CHAOS_SSH_HOST=${CHAOS_SSH_HOST:-node-b}
CHAOS_API=${CHAOS_API:-https://choir.news}
CHAOS_ENV_FILE=${CHAOS_ENV_FILE:-$HOME/Dev/go-choir/.env}
CHAOS_RESULTS_DIR=${CHAOS_RESULTS_DIR:-$CHAOS_HERE/results}
CHAOS_KILL_SWITCH=${CHAOS_KILL_SWITCH:-$HOME/.choir-chaos-stop}
CHAOS_NAP_SCALE=${CHAOS_NAP_SCALE:-1}
# Pre-flight floors (bytes).
CHAOS_MIN_MEM=${CHAOS_MIN_MEM:-8589934592}
CHAOS_MIN_DISK=${CHAOS_MIN_DISK:-60000000000}
# Node B paths, overridable only so the selftest can point them at fixtures.
CHAOS_R_OWN=${CHAOS_R_OWN:-/var/lib/go-choir/vm-state/ownerships.json}
CHAOS_R_VMSTATE=${CHAOS_R_VMSTATE:-/var/lib/go-choir/vm-state}
CHAOS_R_PROC=${CHAOS_R_PROC:-/proc}
CHAOS_R_MEMINFO=${CHAOS_R_MEMINFO:-/proc/meminfo}
CHAOS_R_STOP=${CHAOS_R_STOP:-/run/choir-chaos-stop}
CHAOS_R_GATE_LOCK=${CHAOS_R_GATE_LOCK:-/var/lib/go-choir/ci-gate-floor.lock}
CHAOS_R_KILL=${CHAOS_R_KILL:-kill}
CHAOS_R_ARTIFACTS=${CHAOS_R_ARTIFACTS:-/var/lib/go-choir/platform-artifacts}

CHAOS_DRY=1
CHAOS_OFFLINE=0
CHAOS_FAILS=0
CHAOS_RUN_ID=""
CHAOS_RUN_DIR=""
CHAOS_UNDO=""
CHAOS_ABORTED=""
CHAOS_KEY=""
CHAOS_FINISHED=0

# ---------------------------------------------------------------- output

chaos_now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
chaos_epoch() { date -u +%s; }
chaos_say() { # LEVEL text
	local level=$1 line
	shift
	line="$*"
	if [ -n "$CHAOS_KEY" ]; then line=${line//"$CHAOS_KEY"/<redacted>}; fi
	printf '%s chaos/%s %s %s\n' "$(chaos_now)" "${SCN_NAME:--}" "$level" "$line" >&2
}
info() { chaos_say INFO "$*"; }
will() { chaos_say WILL "$*"; }
chaos_die() { # code text
	local code=$1
	shift
	chaos_say FAIL "$*"
	exit "$code"
}

# chaos_event KIND DETAIL: one line of the run's event log (fault, undo, abort).
chaos_event() {
	[ -n "$CHAOS_RUN_DIR" ] || return 0
	jq -nc --arg at "$(chaos_now)" --arg kind "$1" --arg detail "$2" '{at:$at,kind:$kind,detail:$detail}' >>"$CHAOS_RUN_DIR/events.jsonl"
}

# chaos_check NAME OP EXPECTED ACTUAL: record a check. OP: eq ne ge le
# contains. In dry-run the check is listed, not judged (pass null).
chaos_check() {
	local name=$1 op=$2 exp=$3 act=$4 ok=false
	if [ "$CHAOS_DRY" = 1 ]; then
		info "(dry-run) check: $name: $op $exp"
		jq -nc --arg n "$name" --arg op "$op" --arg e "$exp" '{name:$n,op:$op,expected:$e,actual:null,pass:null}' >>"$CHAOS_RUN_DIR/checks.jsonl"
		return 0
	fi
	case "$op" in
	eq) [ "$act" = "$exp" ] && ok=true ;;
	ne) [ "$act" != "$exp" ] && ok=true ;;
	ge) chaos_isnum "$act" && [ "$act" -ge "$exp" ] && ok=true ;;
	le) chaos_isnum "$act" && [ "$act" -le "$exp" ] && ok=true ;;
	contains) case "$act" in *"$exp"*) ok=true ;; esac ;;
	*) chaos_die 2 "bad check op $op" ;;
	esac
	if [ "$ok" = true ]; then chaos_say PASS "$name ($op $exp; actual $act)"; else
		CHAOS_FAILS=$((CHAOS_FAILS + 1))
		chaos_say FAIL "$name ($op $exp; actual $act)"
	fi
	jq -nc --arg n "$name" --arg op "$op" --arg e "$exp" --arg a "$act" --argjson p "$ok" '{name:$n,op:$op,expected:$e,actual:$a,pass:$p}' >>"$CHAOS_RUN_DIR/checks.jsonl"
}
chaos_isnum() { case "$1" in '' | *[!0-9]*) return 1 ;; esac; }

# ------------------------------------------------------------------ guards

chaos_lc() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }

# chaos_guard_value V: refuse a value naming a protected computer or VM.
chaos_guard_value() {
	local v
	v=$(chaos_lc "$1")
	case "$v" in
	*"$CHAOS_PAPER_HEX"* | *"$CHAOS_PAPER_VM"*) chaos_die 3 "REFUSED: '$1' names the paper's computer; chaos never touches it" ;;
	*"$CHAOS_OWNER_HEX"* | *"$CHAOS_OWNER_VM_PREFIX"*) chaos_die 3 "REFUSED: '$1' names the owner's computer; chaos never touches it" ;;
	*"$CHAOS_DELEGATE_HEX"*) chaos_die 3 "REFUSED: '$1' names the delegate computer; chaos never touches it" ;;
	esac
}

# chaos_guard_target ID: the target must be d@e exactly.
chaos_guard_target() {
	chaos_guard_value "$1"
	[ "$1" = "$CHAOS_TARGET" ] || chaos_die 3 "REFUSED: '$1' is not d@e ($CHAOS_TARGET); chaos runs only against d@e"
}

# chaos_guard_vm VM: shape and protected names.
chaos_guard_vm() {
	chaos_guard_value "$1"
	case "$1" in '' | *[!a-z0-9-]*) chaos_die 3 "REFUSED: VM id '$1' is empty or malformed" ;; esac
}

# --------------------------------------------------------------- kill switch

chaos_stopped_local() { [ -e "$CHAOS_KILL_SWITCH" ]; }
chaos_abort_if_stopped() {
	if chaos_stopped_local; then chaos_abort "kill switch $CHAOS_KILL_SWITCH present"; fi
}
chaos_abort() {
	CHAOS_ABORTED=$1
	chaos_event abort "$1"
	chaos_say FAIL "ABORT: $1"
	exit 4
}

# ------------------------------------------------------------------- undo

# chaos_undo_push FN LABEL: FN runs at exit, last pushed first.
chaos_undo_push() {
	CHAOS_UNDO="$1|$2
$CHAOS_UNDO"
	chaos_event undo-registered "$2"
}
chaos_run_undos() {
	local line fn label rc
	while IFS= read -r line; do
		[ -n "$line" ] || continue
		fn=${line%%|*}
		label=${line#*|}
		info "undo: $label"
		rc=0
		"$fn" || rc=$?
		chaos_event undo "$label rc=$rc"
	done <<EOF
$CHAOS_UNDO
EOF
	CHAOS_UNDO=""
}

# ----------------------------------------------------------------- Node B

# The remote prelude: choir_internal_curl and the read/act functions every
# Node B script uses. Values are validated before they are put in a script.
CHAOS_INTERNAL_LIB=$(cat "$CHAOS_HERE/../lib/choir_internal.sh") || exit 2
chaos_remote_prelude() {
	printf '%s\n' "$CHAOS_INTERNAL_LIB"
	cat <<EOF
CR_OWN='$CHAOS_R_OWN'; CR_VMSTATE='$CHAOS_R_VMSTATE'; CR_PROC='$CHAOS_R_PROC'
CR_MEMINFO='$CHAOS_R_MEMINFO'; CR_STOP='$CHAOS_R_STOP'; CR_GATE_LOCK='$CHAOS_R_GATE_LOCK'; CR_KILL='$CHAOS_R_KILL'; CR_ARTIFACTS='$CHAOS_R_ARTIFACTS'
EOF
	cat <<'EOF'
cr_refuse() { echo "REFUSED: $*"; exit 3; }
# cr_row COMPUTER: the one ownership row of COMPUTER, or nothing.
cr_row() { jq -c --arg c "$1" '[.ownerships[] | select(.computer_id == $c)] | if length == 1 then .[0] else empty end' "$CR_OWN"; }
# cr_vm_rows VM: how many rows name VM.
cr_vm_rows() { jq --arg v "$1" '[.ownerships[] | select(.vm_id == $v)] | length' "$CR_OWN"; }
# cr_pids VM: pids whose cmdline is firecracker with --id VM exactly.
cr_pids() {
	for d in "$CR_PROC"/[0-9]*; do
		[ -r "$d/cmdline" ] || continue
		a0=$(tr '\000' '\n' <"$d/cmdline" | sed -n 1p)
		case "${a0##*/}" in firecracker) ;; *) continue ;; esac
		idv=$(tr '\000' '\n' <"$d/cmdline" | awk 'p { print; exit } $0 == "--id" { p = 1 }')
		[ "$idv" = "$1" ] && echo "${d##*/}"
	done
}
# cr_map COMPUTER: "VM PID" when COMPUTER maps to exactly one VM row whose VM
# no other row names, and exactly one firecracker runs it; refuses otherwise.
cr_map() {
	row=$(cr_row "$1"); [ -n "$row" ] || cr_refuse "no single ownership row for $1"
	vm=$(printf '%s' "$row" | jq -r '.vm_id // ""'); [ -n "$vm" ] || cr_refuse "ownership row of $1 has no vm_id"
	[ "$(cr_vm_rows "$vm")" = 1 ] || cr_refuse "vm $vm is named by more than one ownership row"
	pids=$(cr_pids "$vm"); n=$(printf '%s' "$pids" | grep -c . || true)
	[ "$n" = 1 ] || cr_refuse "vm $vm has $n firecracker processes (need exactly 1)"
	echo "$vm $pids"
}
# cr_console_mark VM: "INODE SIZE" of the guest console log.
cr_console_mark() { f="$CR_VMSTATE/$1/console.log"; [ -f "$f" ] && echo "$(ls -i "$f" | awk '{print $1}') $(wc -c <"$f" | tr -d ' ')" || echo "0 0"; }
# cr_console_since VM INODE SIZE: restart lines the guest logged after the mark,
# following a rotation (console.log.N, oldest highest).
cr_console_since() {
	dir="$CR_VMSTATE/$1"; files=""
	for k in 9 8 7 6 5 4 3 2 1; do [ -f "$dir/console.log.$k" ] && files="$files $dir/console.log.$k"; done
	files="$files $dir/console.log"; seen=0
	for f in $files; do
		[ -f "$f" ] || continue
		ino=$(ls -i "$f" | awk '{print $1}')
		if [ "$seen" = 0 ]; then
			[ "$ino" = "$2" ] || continue
			seen=1; tail -c +$(($3 + 1)) "$f"
		else cat "$f"; fi
	done | grep -aE 'autoputer: restart kind=|treating boot as a crash|after a crash restart|host requested a planned restart|telemetry drain failed|passivated run '
}
# cr_capped CMD...: a read under a memory cap.
cr_capped() { systemd-run --quiet --wait --pipe --collect -p MemoryMax=512M -p MemorySwapMax=0 "$@"; }
EOF
}

chaos_ssh() { "$CHAOS_SSH_BIN" -o BatchMode=yes -o ConnectTimeout=15 "$CHAOS_SSH_HOST" sh -s; }

# chaos_probe SCRIPT: a read-only Node B script; runs in dry-run too (not
# offline). chaos_remote SCRIPT: a read that only means something during a
# drill; printed in dry-run. chaos_remote_mut LABEL SCRIPT: a fault or undo;
# printed in dry-run.
chaos_probe() {
	if [ "$CHAOS_OFFLINE" = 1 ]; then
		info "(offline) would run on $CHAOS_SSH_HOST: $(printf '%s' "$1" | head -1)"
		return 0
	fi
	{ chaos_remote_prelude; printf '%s\n' "$1"; } | chaos_ssh
}
chaos_remote() {
	if [ "$CHAOS_DRY" = 1 ]; then
		info "(dry-run) would read on $CHAOS_SSH_HOST:"
		printf '%s\n' "$1" | sed 's/^/    | /' >&2
		return 0
	fi
	{ chaos_remote_prelude; printf '%s\n' "$1"; } | chaos_ssh
}
# chaos_remote_mut LABEL SCRIPT fault|undo: a fault checks both kill switches
# first (on Node B in the same script); an undo always runs.
chaos_remote_mut() {
	local label=$1 kind=${3:-fault} guard=""
	if [ "$CHAOS_DRY" = 1 ]; then
		will "(dry-run, not run) $kind: $label on $CHAOS_SSH_HOST:"
		printf '%s\n' "$2" | sed 's/^/    | /' >&2
		return 0
	fi
	will "$kind: $label"
	if [ "$kind" = fault ]; then
		chaos_abort_if_stopped
		guard='[ -e "$CR_STOP" ] && { echo "ABORT: remote kill switch $CR_STOP"; exit 4; }'
	fi
	{ chaos_remote_prelude; printf '%s\n%s\n' "$guard" "$2"; } | chaos_ssh
}

# ------------------------------------------------------------ target state

# chaos_resolve: T_VM T_PID T_USER T_DESKTOP T_URL T_STATE T_BUILD from
# ownerships.json and /proc on Node B, refusing anything unmapped. With
# NEED_PID=0 a stopped or hibernated target resolves without a pid.
chaos_resolve() {
	local need_pid=${1:-1} out row
	chaos_guard_target "$CHAOS_TARGET"
	if [ "$CHAOS_OFFLINE" = 1 ]; then
		T_VM="<vm-of-d@e>" T_PID="<pid>" T_USER="<user>" T_DESKTOP="primary" T_URL="http://<guest>:8085" T_STATE="<state>" T_BUILD="<build>"
		info "(offline) would map d@e through $CHAOS_R_OWN and $CHAOS_R_PROC on $CHAOS_SSH_HOST"
		return 0
	fi
	row=$(chaos_probe "cr_row '$CHAOS_TARGET'")
	[ -n "$row" ] || chaos_die 3 "REFUSED: d@e has no single ownership row in $CHAOS_R_OWN; cannot map it to a VM"
	T_VM=$(printf '%s' "$row" | jq -r '.vm_id // ""')
	chaos_guard_vm "$T_VM"
	[ "$(printf '%s' "$row" | jq -r '.computer_id')" = "$CHAOS_TARGET" ] || chaos_die 3 "REFUSED: mapped row is not d@e"
	T_USER=$(printf '%s' "$row" | jq -r '.user_id // ""')
	T_DESKTOP=$(printf '%s' "$row" | jq -r '.desktop_id // "primary"')
	T_URL=$(printf '%s' "$row" | jq -r '.computer_url // ""')
	T_STATE=$(printf '%s' "$row" | jq -r '.state // ""')
	T_BUILD=$(printf '%s' "$row" | jq -r '.reported_release.build_commit // ""')
	T_PID=""
	if [ "$need_pid" = 1 ]; then
		out=$(chaos_probe "cr_map '$CHAOS_TARGET'")
		case "$out" in REFUSED:*) chaos_die 3 "$out" ;; esac
		[ "${out%% *}" = "$T_VM" ] || chaos_die 3 "REFUSED: pid mapping named vm '${out%% *}', not $T_VM"
		T_PID=${out#* }
		chaos_isnum "$T_PID" || chaos_die 3 "REFUSED: no single firecracker pid for $T_VM ($out)"
	fi
	info "target d@e computer=$CHAOS_TARGET vm=$T_VM pid=${T_PID:--} state=$T_STATE url=$T_URL build=${T_BUILD:--}"
}

# ---------------------------------------------------------------- pre-flight

# chaos_preflight: refuse (exit 4) unless the paper is active and answers
# /health, MemAvailable and free disk are above the floors, no deploy holds
# the CI gate lock or runs, and no kill switch is set. Read-only; runs in
# dry-run too.
chaos_preflight() {
	local out k v ok=1
	chaos_abort_if_stopped
	if [ "$CHAOS_OFFLINE" = 1 ]; then info "(offline) pre-flight skipped"; return 0; fi
	out=$(chaos_probe "
mem=\$(awk '/^MemAvailable:/ { print \$2 * 1024 }' \"\$CR_MEMINFO\")
disk=\$(df -B1 --output=avail /var/lib | tail -1 | tr -d ' ')
paper=\$(cr_row '$CHAOS_PAPER')
pstate=\$(printf '%s' \"\$paper\" | jq -r '.state // \"absent\"')
purl=\$(printf '%s' \"\$paper\" | jq -r '.computer_url // \"\"')
phealth=000
[ -n \"\$purl\" ] && phealth=\$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 \"\$purl/health\" || true)
if [ -e \"\$CR_GATE_LOCK\" ]; then if flock -s -n \"\$CR_GATE_LOCK\" true; then lock=free; else lock=held; fi; else lock=free; fi
deploys=\$(ps -eo args= | awk '{ for (i = 1; i <= 2 && i <= NF; i++) if (\$i ~ /node-b-ci-(deploy|prebuild)\$/) { n++; break } } END { print n + 0 }')
stop=absent; [ -e \"\$CR_STOP\" ] && stop=present
echo mem_available=\$mem; echo disk_free=\$disk; echo paper_state=\$pstate; echo paper_health=\$phealth
echo deploy_lock=\$lock; echo deploy_procs=\$deploys; echo remote_kill_switch=\$stop")
	pf() { printf '%s\n' "$out" | sed -n "s/^$1=//p" | head -1; }
	pfc() { # name ok? expected actual
		jq -nc --arg n "$1" --arg e "$3" --arg a "$4" --argjson p "$2" '{name:$n,expected:$e,actual:$a,pass:$p}' >>"$CHAOS_RUN_DIR/preflight.jsonl"
		if [ "$2" = true ]; then chaos_say PASS "pre-flight $1 ($4)"; else chaos_say FAIL "pre-flight $1: expected $3, actual ${4:-<none>}"; ok=0; fi
	}
	v=$(pf mem_available); if chaos_isnum "$v" && [ "$v" -ge "$CHAOS_MIN_MEM" ]; then pfc host_mem_available true ">= $CHAOS_MIN_MEM" "$v"; else pfc host_mem_available false ">= $CHAOS_MIN_MEM" "$v"; fi
	v=$(pf disk_free); if chaos_isnum "$v" && [ "$v" -ge "$CHAOS_MIN_DISK" ]; then pfc host_disk_free true ">= $CHAOS_MIN_DISK" "$v"; else pfc host_disk_free false ">= $CHAOS_MIN_DISK" "$v"; fi
	v=$(pf paper_state); if [ "$v" = active ]; then pfc paper_state true active "$v"; else pfc paper_state false active "$v"; fi
	v=$(pf paper_health); if [ "$v" = 200 ]; then pfc paper_health true 200 "$v"; else pfc paper_health false 200 "$v"; fi
	v=$(pf deploy_lock); if [ "$v" = free ]; then pfc deploy_gate_lock true free "$v"; else pfc deploy_gate_lock false free "$v"; fi
	v=$(pf deploy_procs); if [ "$v" = 0 ]; then pfc deploy_processes true 0 "$v"; else pfc deploy_processes false 0 "$v"; fi
	v=$(pf remote_kill_switch); if [ "$v" = absent ]; then pfc remote_kill_switch true absent "$v"; else pfc remote_kill_switch false absent "$v"; fi
	[ "$ok" = 1 ] || chaos_abort "pre-flight refused"
}

# ------------------------------------------------------------- public API

# chaos_load_key: d@e's key from CHAOS_ENV_FILE (CHOIR_TEST_D_E_API_KEY),
# into a shell variable only.
chaos_load_key() {
	[ "$CHAOS_DRY" = 1 ] && return 0
	[ -r "$CHAOS_ENV_FILE" ] || chaos_die 2 "cannot read $CHAOS_ENV_FILE for CHOIR_TEST_D_E_API_KEY"
	CHAOS_KEY=$(sed -n 's/^\(export \)\{0,1\}CHOIR_TEST_D_E_API_KEY=//p' "$CHAOS_ENV_FILE" | tail -1 | sed "s/^[\"']//; s/[\"']\$//")
	[ -n "$CHAOS_KEY" ] || chaos_die 2 "CHOIR_TEST_D_E_API_KEY is not set in $CHAOS_ENV_FILE"
}

chaos_with_computer() { case $1 in *\?*) printf '%s&computer_id=%s' "$1" "$CHAOS_TARGET" ;; *) printf '%s?computer_id=%s' "$1" "$CHAOS_TARGET" ;; esac; }

# chaos_api METHOD PATH [BODY]: d@e's product API with its key. Prints the
# body, then a last line "http=<code>". Dry-run prints the call only.
chaos_api() {
	local m=$1 p=$2 body=${3:-}
	if [ "$CHAOS_DRY" = 1 ]; then
		info "(dry-run) $m $CHAOS_API$(chaos_with_computer "$p")${body:+ body=$body} (Authorization from d@e key; not run)"
		return 0
	fi
	if [ -n "$body" ]; then
		"$CHAOS_CURL_BIN" -sS --max-time 60 -X "$m" -H @<(printf 'Authorization: Bearer %s\n' "$CHAOS_KEY") \
			-H 'Content-Type: application/json' --data-binary "$body" -w '\nhttp=%{http_code}\n' "$CHAOS_API$(chaos_with_computer "$p")" 2>/dev/null || echo "http=000"
	else
		"$CHAOS_CURL_BIN" -sS --max-time 60 -X "$m" -H @<(printf 'Authorization: Bearer %s\n' "$CHAOS_KEY") \
			-w '\nhttp=%{http_code}\n' "$CHAOS_API$(chaos_with_computer "$p")" 2>/dev/null || echo "http=000"
	fi
}
chaos_http() { printf '%s\n' "$1" | sed -n 's/^http=//p' | tail -1; }
chaos_body() { printf '%s\n' "$1" | sed '$d'; }

# chaos_nap SECONDS: sleep, checking the kill switch each second.
chaos_nap() {
	local i=0
	[ "$CHAOS_DRY" = 1 ] && return 0
	[ "$CHAOS_NAP_SCALE" = 0 ] && { chaos_abort_if_stopped; return 0; }
	while [ "$i" -lt "$1" ]; do chaos_abort_if_stopped; sleep 1; i=$((i + 1)); done
}

# chaos_wait TIMEOUT INTERVAL WHAT CMD...: poll CMD until it succeeds; records
# the wait. Returns 1 at the timeout.
chaos_wait() {
	local timeout=$1 interval=$2 what=$3 t0
	shift 3
	if [ "$CHAOS_DRY" = 1 ]; then info "(dry-run) would wait up to ${timeout}s for: $what"; return 0; fi
	t0=$(chaos_epoch)
	while :; do
		chaos_abort_if_stopped
		if "$@"; then chaos_event wait "$what after $(($(chaos_epoch) - t0))s"; info "seen: $what after $(($(chaos_epoch) - t0))s"; return 0; fi
		[ $(($(chaos_epoch) - t0)) -ge "$timeout" ] && { chaos_event wait-timeout "$what"; info "not seen within ${timeout}s: $what"; return 1; }
		chaos_nap "$interval"
	done
}

# ------------------------------------------------------------ d@e activity

# Writes are acknowledged when the API answered 2xx with its receipt. Two
# kinds: a file PUT (no model) whose content is a nonce read back later, and
# a Texture create (a canonical write and its receipt) keyed by
# client_request_id, replayed afterwards to prove it is not duplicated.

chaos_files_dir() { printf 'chaos/%s' "$CHAOS_RUN_ID"; }

chaos_write_file() { # N
	local n=$1 nonce out code
	nonce="chaos-$CHAOS_RUN_ID-w$n-$(chaos_epoch)"
	out=$("$CHAOS_CURL_BIN" -sS --max-time 30 -X PUT -H @<(printf 'Authorization: Bearer %s\n' "$CHAOS_KEY") \
		--data-binary "$nonce" -w '\nhttp=%{http_code}\n' "$CHAOS_API$(chaos_with_computer "/api/files/$(chaos_files_dir)/w$n.txt")" 2>/dev/null || echo "http=000")
	code=$(chaos_http "$out")
	jq -nc --arg at "$(chaos_now)" --argjson n "$n" --arg kind file --arg path "$(chaos_files_dir)/w$n.txt" --arg nonce "$nonce" --arg http "$code" \
		'{at:$at,n:$n,kind:$kind,path:$path,nonce:$nonce,http:$http,ack:($http|test("^2"))}' >>"$CHAOS_RUN_DIR/writes.jsonl"
}

chaos_write_texture() { # N
	local n=$1 rid out code body
	rid="chaos-$CHAOS_RUN_ID-t$n"
	out=$(chaos_api POST /api/texture/lifecycle-documents "$(jq -nc --arg t "chaos drill $CHAOS_RUN_ID $n" --arg r "$rid" \
		'{title:$t,initial_content:"Chaos drill write on the test computer. No action needed; reply done.",client_request_id:$r}')")
	code=$(chaos_http "$out")
	body=$(chaos_body "$out")
	jq -nc --arg at "$(chaos_now)" --argjson n "$n" --arg rid "$rid" --arg http "$code" --arg body "$body" \
		'{at:$at,n:$n,kind:"texture",request_id:$rid,http:$http,doc_id:(($body|fromjson?|.doc_id)//""),revision_id:(($body|fromjson?|.revision_id)//"")} | .ack = ((.http|test("^2")) and .doc_id != "")' \
		>>"$CHAOS_RUN_DIR/writes.jsonl"
}

# chaos_load_start KINDS: background writer until load.stop or CHAOS_LOAD_MAX
# writes. KINDS: "file" or "file texture" (every 4th write a Texture create,
# at most CHAOS_TEXTURE_MAX).
chaos_load_start() {
	local kinds=$1
	if [ "$CHAOS_DRY" = 1 ]; then
		info "(dry-run) would start writes ($kinds) on d@e: PUT /api/files/$(chaos_files_dir)/w<n>.txt every ${CHAOS_LOAD_INTERVAL:-1}s${kinds#file}"
		return 0
	fi
	chaos_api POST "/api/files/chaos" >/dev/null
	chaos_api POST "/api/files/$(chaos_files_dir)" >/dev/null
	# The first write is synchronous, so the path is proved before any fault.
	chaos_write_file 1
	chaos_check "pre: first write to d@e acknowledged" eq 1 "$(chaos_acked file | grep -c . || true)"
	(
		n=1 tex=0
		while [ ! -e "$CHAOS_RUN_DIR/load.stop" ] && [ "$n" -lt "${CHAOS_LOAD_MAX:-200}" ]; do
			n=$((n + 1))
			chaos_write_file "$n"
			case " $kinds " in *" texture "*)
				if [ $((n % 4)) = 1 ] && [ "$tex" -lt "${CHAOS_TEXTURE_MAX:-3}" ]; then tex=$((tex + 1)); chaos_write_texture "$n"; fi ;;
			esac
			[ "$CHAOS_NAP_SCALE" = 0 ] || sleep "${CHAOS_LOAD_INTERVAL:-1}"
		done
	) &
	CHAOS_LOAD_PID=$!
	chaos_undo_push chaos_load_stop "stop the d@e writer"
	chaos_event load-start "$kinds pid=$CHAOS_LOAD_PID"
}
chaos_load_stop() {
	[ -n "${CHAOS_LOAD_PID:-}" ] || return 0
	: >"$CHAOS_RUN_DIR/load.stop"
	wait "$CHAOS_LOAD_PID" 2>/dev/null || true
	CHAOS_LOAD_PID=""
	chaos_event load-stop "writes=$(wc -l <"$CHAOS_RUN_DIR/writes.jsonl" 2>/dev/null | tr -d ' ')"
}

chaos_acked() { # KIND: acknowledged writes of KIND as JSON lines
	[ -f "$CHAOS_RUN_DIR/writes.jsonl" ] || return 0
	jq -c --arg k "$1" 'select(.kind == $k and .ack)' "$CHAOS_RUN_DIR/writes.jsonl"
}

# chaos_check_acked_present: every acknowledged file reads back its nonce,
# every acknowledged Texture create reads back, and a replay of its
# client_request_id answers the same doc_id (no duplicate).
chaos_check_acked_present() {
	local acked=0 present=0 tacked=0 tpresent=0 tsame=0 line out
	if [ "$CHAOS_DRY" != 1 ]; then
		while IFS= read -r line; do
			[ -n "$line" ] || continue
			acked=$((acked + 1))
			out=$(chaos_api GET "/api/files/$(printf '%s' "$line" | jq -r .path)")
			[ "$(chaos_http "$out")" = 200 ] && [ "$(chaos_body "$out")" = "$(printf '%s' "$line" | jq -r .nonce)" ] && present=$((present + 1))
		done <<EOF
$(chaos_acked file)
EOF
		while IFS= read -r line; do
			[ -n "$line" ] || continue
			tacked=$((tacked + 1))
			out=$(chaos_api GET "/api/texture/documents/$(printf '%s' "$line" | jq -r .doc_id)")
			[ "$(chaos_http "$out")" = 200 ] && tpresent=$((tpresent + 1))
			out=$(chaos_api POST /api/texture/lifecycle-documents "$(jq -nc --arg t "chaos drill $CHAOS_RUN_ID $(printf '%s' "$line" | jq -r .n)" --arg r "$(printf '%s' "$line" | jq -r .request_id)" \
				'{title:$t,initial_content:"Chaos drill write on the test computer. No action needed; reply done.",client_request_id:$r}')")
			[ "$(chaos_body "$out" | jq -r '.doc_id // ""' 2>/dev/null)" = "$(printf '%s' "$line" | jq -r .doc_id)" ] && tsame=$((tsame + 1))
		done <<EOF
$(chaos_acked texture)
EOF
	fi
	chaos_check "acknowledged file writes before the fault" ge 1 "$acked"
	chaos_check "acknowledged file writes read back with their nonce" eq "$acked" "$present"
	chaos_check "acknowledged Texture creates read back" eq "$tacked" "$tpresent"
	chaos_check "Texture create replays answer the same doc_id (no duplicate)" eq "$tacked" "$tsame"
}

# chaos_open_runs: run ids not terminal (pending running passivated blocked).
chaos_runs_json() { chaos_body "$(chaos_api GET '/api/runs?limit=500')"; }
chaos_open_runs() {
	[ "$CHAOS_DRY" = 1 ] && { chaos_api GET '/api/runs?limit=500'; return 0; }
	chaos_runs_json | jq -r '.runs[]? | select(.state == "pending" or .state == "running" or .state == "passivated" or .state == "blocked") | .run_id' 2>/dev/null
}

# chaos_start_run: one prompt-bar run on d@e (model activity) and wait until
# a run is running.
chaos_start_run() {
	chaos_api POST /api/prompt-bar "$(jq -nc --arg c "chaos-$CHAOS_RUN_ID-run" \
		'{text:"Chaos drill on the test computer: write the numbers 1 to 40, one per line, each with one sentence about it. Take your time.",command_id:$c}')" >"${CHAOS_RUN_DIR}/prompt-bar.out"
	chaos_wait 90 3 "a run running on d@e" chaos_has_running_run || true
}
chaos_has_running_run() { [ -n "$(chaos_runs_json | jq -r '.runs[]? | select(.state == "running") | .run_id' 2>/dev/null | head -1)" ]; }

# chaos_ready: d@e is active and its runtime answers.
chaos_ready() {
	local st
	st=$(chaos_body "$(chaos_api GET /api/compute/status)" | jq -r --arg c "$CHAOS_TARGET" '[.computers[]? | select(.computer_id == $c) | .state] | first // ""' 2>/dev/null)
	[ "$st" = active ] && [ "$(chaos_http "$(chaos_api GET /api/runtime/observability)")" = 200 ]
}

chaos_setting() { # KEY: d@e's stored or default computer setting value
	chaos_body "$(chaos_api GET /api/computer-settings)" | jq -r --arg k "$1" '[.settings[]? | select(.key == $k) | .value] | first // "unknown"' 2>/dev/null
}

chaos_replay_status() {
	chaos_body "$(chaos_api GET "/api/computers/$CHAOS_TARGET/self-development/replay-completeness")" | jq -r '.result.status // "unavailable"' 2>/dev/null
}

# chaos_run_events RUN: the run's events (JSON).
chaos_run_events() { chaos_body "$(chaos_api GET "/api/runs/$1/events?limit=1000")"; }

# ----------------------------------------------------------------- artifact

chaos_artifact() {
	local f="$CHAOS_RUN_DIR/artifact.json" pass
	for x in checks preflight events writes; do [ -f "$CHAOS_RUN_DIR/$x.jsonl" ] || : >"$CHAOS_RUN_DIR/$x.jsonl"; done
	jq -n --arg scenario "$SCN_NAME" --arg run "$CHAOS_RUN_ID" --arg mode "$([ "$CHAOS_DRY" = 1 ] && echo dry-run || echo execute)" \
		--arg started "$CHAOS_STARTED" --arg finished "$(chaos_now)" --arg fault "$SCN_FAULT" --arg aborted "$CHAOS_ABORTED" \
		--arg computer "$CHAOS_TARGET" --arg vm "${T_VM:-}" --arg pid "${T_PID:-}" \
		--slurpfile checks "$CHAOS_RUN_DIR/checks.jsonl" --slurpfile preflight "$CHAOS_RUN_DIR/preflight.jsonl" \
		--slurpfile events "$CHAOS_RUN_DIR/events.jsonl" --slurpfile writes "$CHAOS_RUN_DIR/writes.jsonl" '
	{schema:"choir.chaos_run.v1",scenario:$scenario,run_id:$run,mode:$mode,started_at:$started,finished_at:$finished,
	 target:{computer_id:$computer,vm_id:$vm,firecracker_pid:$pid},
	 fault:{planned:$fault,injected:[$events[]|select(.kind=="fault")],undone:[$events[]|select(.kind=="undo")]},
	 aborted:(if $aborted=="" then null else $aborted end),
	 preflight:$preflight,checks:$checks,events:$events,
	 writes:{attempted:($writes|length),acknowledged:([$writes[]|select(.ack)]|length)},
	 pass:(if $aborted!="" then false elif $mode=="dry-run" then ([$preflight[]|.pass]|all) else (([$checks[]|.pass]|all) and ($checks|length>0)) end)}' >"$f"
	info "artifact: $f"
}

# chaos_finish: the EXIT trap. Stops the writer, runs every undo, writes the
# artifact. Runs once.
chaos_finish() {
	local rc=$?
	[ "$CHAOS_FINISHED" = 1 ] && return
	CHAOS_FINISHED=1
	trap - INT TERM
	chaos_run_undos
	[ -n "$CHAOS_RUN_DIR" ] && chaos_artifact
	if [ "$rc" = 0 ] && [ "$CHAOS_FAILS" -gt 0 ]; then rc=1; fi
	exit "$rc"
}
