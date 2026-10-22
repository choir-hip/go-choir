#!/usr/bin/env bash
# Shared helpers for the FP0 staging proof scripts (scripts/proofs/*.sh).
# Sourced, never executed. Station: docs/definitions/choir-launch-fp0-proof-scripts-2026-10-21.md.
#
# Runs on the owner's Mac (bash 3.2 is enough) or on Node B. Node B calls go
# through `ssh node-b`; the vmctl internal-service header file is read on
# Node B by curl and never printed. Public API calls read CHOIR_API_KEY from
# the environment and pass it to curl through a file descriptor, never on a
# command line and never to a log.
#
# Conventions (every script):
#   --dry-run        print each command, run none
#   --list           print the steps and whether each changes state
#   --step N         run step N (steps that change state also need --mutate)
#   --mutate         allow the named state-changing step; never runs without --step
#   --evidence FILE  also append evidence lines to FILE
#   --<name> VALUE   script options, read as $OPT_<name> (dashes become _)
# With no --step, only the read-only steps that make sense standing alone run.
#
# Evidence line: <UTC time> <script> <LEVEL> step=<N> <text>
# LEVEL: INFO, PASS, FAIL, MANUAL (an act this script cannot do), WILL (what a
# mutating step is about to do). The exit status is 1 if any FAIL was printed,
# 2 for a usage error, 3 for a refused computer.

PROOF_DELEGATE=computer-ad1b46d4c20129e84b9e239cf4720fb9
PROOF_PAPER=computer-1355b83bced1613eb048ee28d66688a8
PROOF_OWNER=computer-03335285269bdba4f94377e56879f9e6
# The hex tails too, so a bare id, a vm name carrying it, or a URL still matches.
PROOF_DELEGATE_HEX=ad1b46d4c20129e84b9e239cf4720fb9
PROOF_PAPER_HEX=1355b83bced1613eb048ee28d66688a8
PROOF_OWNER_HEX=03335285269bdba4f94377e56879f9e6

PROOF_SSH_BIN=${PROOF_SSH_BIN:-ssh}
PROOF_CURL_BIN=${PROOF_CURL_BIN:-curl}
PROOF_SSH_HOST=${PROOF_SSH_HOST:-node-b}
PROOF_TOKEN_HEADER=/var/lib/go-choir/secrets/internal-service-token.header
PROOF_API_HOST=${CHOIR_HOST:-https://choir.news}
PROOF_VMCTL=http://127.0.0.1:8083
PROOF_UNIT_VMCTL=${PROOF_UNIT_VMCTL:-go-choir-vmctl}
PROOF_UNIT_GATEWAY=${PROOF_UNIT_GATEWAY:-go-choir-gateway}
PROOF_NAP_SCALE=${PROOF_NAP_SCALE:-1}

PROOF_NAME=${PROOF_NAME:-proof}
PROOF_FAILS=0
PROOF_DRY=0
PROOF_MUTATE=0
PROOF_LIST=0
PROOF_STEP=""
PROOF_EVIDENCE=""
PROOF_CUR_STEP="-"
PROOF_ALL_VALUES=""
PROOF_STEPS=""
PROOF_LIST_CACHE=""

# ---------------------------------------------------------------- evidence

proof_now() { date -u +%Y-%m-%dT%H:%M:%SZ; }

proof_emit() { # LEVEL text...
	local level=$1 line
	shift
	line="$*"
	if [ -n "${CHOIR_API_KEY:-}" ]; then
		line=${line//"$CHOIR_API_KEY"/<redacted>}
	fi
	line=$(printf '%s %s %s step=%s %s' "$(proof_now)" "$PROOF_NAME" "$level" "$PROOF_CUR_STEP" "$line")
	printf '%s\n' "$line"
	if [ -n "$PROOF_EVIDENCE" ]; then printf '%s\n' "$line" >>"$PROOF_EVIDENCE"; fi
}
info() { proof_emit INFO "$*"; }
pass() { proof_emit PASS "$*"; }
manual() { proof_emit MANUAL "$*"; }
will() { proof_emit WILL "$*"; }
fail() {
	PROOF_FAILS=$((PROOF_FAILS + 1))
	proof_emit FAIL "$*"
}
die() { # exit-code text...
	local code=$1
	shift
	proof_emit FAIL "$*"
	exit "$code"
}

# check "what" <command...>: PASS when the command succeeds, FAIL otherwise.
check() {
	local what=$1
	shift
	if [ "$PROOF_DRY" = 1 ]; then
		info "(dry-run) would check: $what"
		return 0
	fi
	if "$@"; then pass "$what"; else fail "$what"; fi
}

# ------------------------------------------------------------------ guards

proof_lc() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }

# proof_guard_value VALUE MUTATING: refuse (exit 3) a value naming the
# delegate always, and the paper or the owner computer when MUTATING is 1.
proof_guard_value() {
	local v
	v=$(proof_lc "$1")
	case "$v" in
	*"$PROOF_DELEGATE_HEX"*) die 3 "REFUSED: $1 names the delegate computer; no step of any proof script touches it" ;;
	esac
	if [ "$2" = 1 ]; then
		case "$v" in
		*"$PROOF_PAPER_HEX"*) die 3 "REFUSED: $1 names the paper's computer; a mutating step never touches it" ;;
		*"$PROOF_OWNER_HEX"*) die 3 "REFUSED: $1 names the owner's computer; a mutating step never touches it" ;;
		esac
	fi
}

# proof_guard_args MUTATING: every value on the command line is checked, so a
# misplaced id in any option is refused too.
proof_guard_args() {
	local v
	for v in $PROOF_ALL_VALUES; do proof_guard_value "$v" "$1"; done
}

# ----------------------------------------------------------------- options

proof_need() { # tool...
	local t
	[ "$PROOF_DRY" = 1 ] && return 0
	for t in "$@"; do
		command -v "$t" >/dev/null 2>&1 || die 2 "needs $t on PATH"
	done
}

# step_def N KIND "title": KIND is ro (runs in the default pass), ro-after
# (read-only but only meaningful after a mutating step; runs with --step) or
# mut (changes state; needs --step N --mutate). The step body is step_N.
step_def() {
	printf -v "PROOF_STEP_KIND_$1" '%s' "$2"
	printf -v "PROOF_STEP_TITLE_$1" '%s' "$3"
	PROOF_STEPS="$PROOF_STEPS $1"
}

proof_usage() {
	cat <<EOF
usage: $0 [--dry-run] [--list] [--step N [--mutate]] [--evidence FILE] [--<option> VALUE]...
See the header of $0 for its options. Read-only steps run by default; a step
that changes state runs only with --step N --mutate, after printing what it will do.
EOF
}

proof_parse() {
	local k name
	while [ $# -gt 0 ]; do
		k=$1
		case "$k" in
		--dry-run) PROOF_DRY=1 ;;
		--list) PROOF_LIST=1 ;;
		--mutate) PROOF_MUTATE=1 ;;
		-h | --help)
			proof_usage
			exit 0
			;;
		--step | --evidence)
			[ $# -ge 2 ] || die 2 "$k needs a value"
			if [ "$k" = --step ]; then PROOF_STEP=$2; else PROOF_EVIDENCE=$2; fi
			PROOF_ALL_VALUES="$PROOF_ALL_VALUES $2"
			shift
			;;
		--*)
			[ $# -ge 2 ] || die 2 "$k needs a value"
			name=${k#--}
			name=${name//-/_}
			case "$name" in *[!A-Za-z0-9_]*) die 2 "bad option $k" ;; esac
			printf -v "OPT_$name" '%s' "$2"
			PROOF_ALL_VALUES="$PROOF_ALL_VALUES $2"
			shift
			;;
		*) die 2 "unexpected argument $k" ;;
		esac
		shift
	done
}

# ----------------------------------------------------------- Node B access

proof_nap() { # seconds
	local s=$1
	[ "$PROOF_DRY" = 1 ] && return 0
	if [ "$PROOF_NAP_SCALE" = 0 ]; then return 0; fi
	sleep "$s"
}

# node_b_sh SCRIPT: run a shell script on Node B (stdin to `sh -s`). The
# script text is printed first. Dry-run prints it and runs nothing.
node_b_sh() {
	info "\$ ssh $PROOF_SSH_HOST sh -s <<'EOF'"
	printf '%s\n' "$1" | sed 's/^/    | /'
	if [ "$PROOF_DRY" = 1 ]; then return 0; fi
	printf '%s\n' "$1" | "$PROOF_SSH_BIN" -o BatchMode=yes -o ConnectTimeout=15 "$PROOF_SSH_HOST" sh -s
}

# node_b_q SCRIPT: like node_b_sh but quiet, for reads whose output a step
# parses. It sends PROOF_INTERNAL_LIB ahead of SCRIPT. Dry-run prints both,
# exactly as sent, and returns an empty answer.
node_b_q() {
	if [ "$PROOF_DRY" = 1 ]; then
		{
			info "\$ ssh $PROOF_SSH_HOST sh -s <<'EOF'"
			printf '%s\n%s\n' "$PROOF_INTERNAL_LIB" "$1" | sed 's/^/    | /'
		} >&2
		return 0
	fi
	printf '%s\n%s\n' "$PROOF_INTERNAL_LIB" "$1" | "$PROOF_SSH_BIN" -o BatchMode=yes -o ConnectTimeout=15 "$PROOF_SSH_HOST" sh -s
}

# choir_internal_curl (scripts/lib/choir_internal.sh), sent ahead of every
# node_b_q script: on Node B, curl reads the header file, when readable, and
# only for a loopback internal port, never a guest. Its contents never cross
# the wire to this shell.
PROOF_INTERNAL_LIB=$(cat "$(dirname "${BASH_SOURCE[0]}")/../lib/choir_internal.sh") || exit 2

# vmctl_get PATH: GET a vmctl internal route on Node B; JSON on stdout.
vmctl_get() {
	node_b_q "choir_internal_curl '$PROOF_VMCTL$1' -fsS --max-time 30"
}

# vmctl_build_identity: "COMMIT SERVICE" of the running vmctl, from the
# X-Choir-Build-Commit and X-Choir-Build-Service headers that
# server.WithBuildIdentity (internal/server/server.go) sets on every
# response; the deploy reads the same header for vmctl's activation receipt
# (.github/scripts/node-b-ci-deploy). The /health body has no build field.
# A missing header prints "-" in its place.
vmctl_build_identity() {
	node_b_q "choir_internal_curl '$PROOF_VMCTL/health' -sS --max-time 15 -D - -o /dev/null" |
		awk '{ gsub(/\r/, "") } tolower($1) == "x-choir-build-commit:" { c = $2 } tolower($1) == "x-choir-build-service:" { s = $2 } END { printf "%s %s\n", (c == "" ? "-" : c), (s == "" ? "-" : s) }'
}

# vmctl_post PATH JSON LIFECYCLE_CALLER: a state-changing POST. Prints the
# request first (the header file's contents are not part of it).
vmctl_post() {
	case "$2" in *"'"*) die 2 "refusing a request body with a single quote" ;; esac
	info "POST $PROOF_VMCTL$1 body=$2 caller=$3 (internal-service header from Node B's file)"
	node_b_q "choir_internal_curl '$PROOF_VMCTL$1' -sS --max-time 600 -X POST -H 'X-Choir-Lifecycle-Caller: $3' -H 'Content-Type: application/json' -d '$2' -w '\\nhttp_status=%{http_code}\\n'"
}

# node_journal UNIT SINCE REGEX [LIMIT]: matching journal lines on Node B.
node_journal() {
	node_b_q "journalctl -u '$1' --since '$2' --no-pager -o short-iso 2>/dev/null | grep -E '$3' | tail -n ${4:-40}"
}

# ------------------------------------------------------------- public API

# api_get PATH [COMPUTER]: GET a product route with CHOIR_API_KEY. The key is
# handed to curl through a descriptor (not argv) and is never printed.
# api_with_computer PATH COMPUTER: the product API selects a computer with the
# computer_id query parameter (an X-Choir-Computer header is refused with 403).
# On a machine whose BASH_ENV sources per-developer secrets, every bash
# re-exports the owner-wide CHOIR_API_KEY over the one a caller passes in.
# Run proofs as: BASH_ENV= CHOIR_API_KEY=<test key> scripts/proofs/<station>.sh
api_with_computer() {
	local path=$1 computer=${2:-}
	[ -n "$computer" ] || { printf '%s' "$path"; return; }
	case $path in *\?*) printf '%s&computer_id=%s' "$path" "$computer" ;; *) printf '%s?computer_id=%s' "$path" "$computer" ;; esac
}

api_get() {
	local path=$1 computer=${2:-}
	proof_guard_value "$computer" 0
	if [ "$PROOF_DRY" = 1 ]; then
		info "\$ curl -fsS -H 'Authorization: Bearer <CHOIR_API_KEY>' '$PROOF_API_HOST$(api_with_computer "$path" "$computer")'"
		return 0
	fi
	[ -n "${CHOIR_API_KEY:-}" ] || die 2 "CHOIR_API_KEY is not set in the environment"
	if [ -n "$computer" ]; then
		"$PROOF_CURL_BIN" -fsS --max-time 60 -H @<(printf 'Authorization: Bearer %s\n' "$CHOIR_API_KEY") "$PROOF_API_HOST$(api_with_computer "$path" "$computer")"
	else
		"$PROOF_CURL_BIN" -fsS --max-time 60 -H @<(printf 'Authorization: Bearer %s\n' "$CHOIR_API_KEY") "$PROOF_API_HOST$path"
	fi
}

# api_public PATH: an unauthenticated GET of the public site.
api_public() {
	if [ "$PROOF_DRY" = 1 ]; then
		info "\$ curl -fsS '$PROOF_API_HOST$1'"
		return 0
	fi
	"$PROOF_CURL_BIN" -fsS --max-time 30 "$PROOF_API_HOST$1"
}

# guest_get PATH: GET a route on the resolved target's own address from Node B,
# with the headers vmctl itself uses for its read-only calls to a guest.
guest_get() {
	node_b_q "curl -fsS --max-time 60 -H 'X-Internal-Caller: true' -H 'X-Authenticated-User: $T_USER' -H 'X-Authenticated-Computer: $T_COMPUTER' '$T_URL$1'"
}

# ------------------------------------------------------- computer resolution

# proof_list_json: vmctl's ownership list with the delegate's row removed, so
# no step ever sees or acts on it. Cached for the run.
proof_list_json() {
	if [ -z "$PROOF_LIST_CACHE" ]; then
		PROOF_LIST_CACHE=$(vmctl_get /internal/vmctl/list | jq -c --arg d "$PROOF_DELEGATE_HEX" '.ownerships |= map(select(((.computer_id // "") + (.vm_id // "")) | ascii_downcase | contains($d) | not))' 2>/dev/null)
	fi
	printf '%s' "$PROOF_LIST_CACHE"
}

# resolve_target ID MUTATING: set T_COMPUTER, T_VM, T_USER, T_DESKTOP, T_URL,
# T_STATE, T_CLASS, T_STOPPED_BY, T_IP, T_PORT for the computer (or vm) ID.
resolve_target() {
	local id=$1 mut=$2 row
	[ -n "$id" ] || die 2 "this step needs --computer"
	proof_guard_value "$id" "$mut"
	if [ "$PROOF_DRY" = 1 ]; then
		T_COMPUTER=$id T_VM="<vm-of-$id>" T_USER="<user>" T_DESKTOP="<desktop>" T_URL="http://<guest-ip>:<port>"
		T_STATE="<state>" T_CLASS="<class>" T_STOPPED_BY="" T_IP="<guest-ip>" T_PORT="<port>"
		info "(dry-run) would resolve $id through GET /internal/vmctl/list on Node B"
		return 0
	fi
	row=$(proof_list_json | jq -c --arg id "$id" '[.ownerships[]? | select(.computer_id == $id or .vm_id == $id)] | first // empty')
	[ -n "$row" ] || die 2 "no ownership row for $id in vmctl's list"
	T_COMPUTER=$(printf '%s' "$row" | jq -r '.computer_id // ""')
	T_VM=$(printf '%s' "$row" | jq -r '.vm_id // ""')
	T_USER=$(printf '%s' "$row" | jq -r '.user_id // ""')
	T_DESKTOP=$(printf '%s' "$row" | jq -r '.desktop_id // ""')
	T_URL=$(printf '%s' "$row" | jq -r '.computer_url // ""')
	T_STATE=$(printf '%s' "$row" | jq -r '.state // ""')
	T_CLASS=$(printf '%s' "$row" | jq -r '.warmness_class // ""')
	T_STOPPED_BY=$(printf '%s' "$row" | jq -r '.stopped_by // ""')
	# The row's own ids are checked again: the caller may have passed a vm id.
	proof_guard_value "$T_COMPUTER" "$mut"
	proof_guard_value "$T_VM" "$mut"
	T_IP=$(printf '%s' "$T_URL" | sed -E 's#^[a-z]+://([0-9.]+)(:([0-9]+))?.*#\1#')
	T_PORT=$(printf '%s' "$T_URL" | sed -E 's#^[a-z]+://[0-9.]+:([0-9]+).*#\1#')
	info "target computer=$T_COMPUTER vm=$T_VM state=$T_STATE class=$T_CLASS stopped_by=${T_STOPPED_BY:--} url=$T_URL"
}

# proof_row_json ID: the computer's CURRENT ownership row (fresh read, not the
# cached list); empty when absent.
proof_row_json() {
	vmctl_get /internal/vmctl/list | jq -c --arg id "$1" --arg d "$PROOF_DELEGATE_HEX" '[.ownerships[]? | select(.computer_id == $id or .vm_id == $id) | select(((.computer_id // "") + (.vm_id // "")) | ascii_downcase | contains($d) | not)] | first // empty'
}

# proof_poll TIMEOUT INTERVAL "what" CMD...: run CMD until it succeeds.
# PASS with the wait on success; FAIL (and return 1) at the timeout.
proof_poll() {
	local timeout=$1 interval=$2 what=$3 waited=0
	shift 3
	if [ "$PROOF_DRY" = 1 ]; then
		info "(dry-run) would wait up to ${timeout}s, every ${interval}s, for: $what"
		return 0
	fi
	while :; do
		if "$@"; then
			pass "$what (after ${waited}s)"
			return 0
		fi
		if [ "$waited" -ge "$timeout" ]; then
			fail "$what: not seen within ${timeout}s"
			return 1
		fi
		proof_nap "$interval"
		waited=$((waited + interval))
	done
}

# ---------------------------------------------------------------- iptables

# proof_wedge_insert IP PORT TTL_SECONDS: REJECT (tcp-reset) the host's
# connections to a guest service port, leaving the guest process alive. The
# removal is scheduled FIRST, with systemd-run --on-active, so a lost session
# cannot leave the rule behind. Echoes the unit name on stdout of Node B.
proof_wedge_rule() { printf 'OUTPUT -d %s -p tcp --dport %s -j REJECT --reject-with tcp-reset' "$1" "$2"; }

proof_wedge_insert() {
	local ip=$1 port=$2 ttl=$3 rule unit
	case "$ip" in *[!0-9.]* | "" | 127.* | 0.*) [ "$PROOF_DRY" = 1 ] || die 2 "refusing to wedge address '$ip'" ;; esac
	case "$port" in *[!0-9]* | "") [ "$PROOF_DRY" = 1 ] || die 2 "refusing to wedge port '$port'" ;; esac
	rule=$(proof_wedge_rule "$ip" "$port")
	unit="choir-proof-unwedge-$(date +%s)-$$"
	will "on Node B: systemd-run --on-active=${ttl}s --unit=$unit removes the rule if present; then iptables -I $rule"
	node_b_sh "set -e
IPT=\$(command -v iptables)
systemd-run --on-active=${ttl}s --unit=$unit --description='FP0 proof: remove wedge rule for $ip:$port' /bin/sh -c \"\$IPT -C $rule && \$IPT -D $rule || true\"
\$IPT -I $rule
\$IPT -C $rule && echo 'rule present' || { echo 'RULE NOT PRESENT AFTER INSERT'; exit 1; }
systemctl list-timers '$unit.timer' --no-pager | head -3"
}

# proof_wedge_remove IP PORT: delete the rule now (the timer stays as a no-op).
proof_wedge_remove() {
	local rule
	rule=$(proof_wedge_rule "$1" "$2")
	will "on Node B: iptables -D $rule"
	node_b_sh "IPT=\$(command -v iptables)
if \$IPT -C $rule 2>/dev/null; then \$IPT -D $rule && echo 'rule removed'; else echo 'rule already absent'; fi"
}

proof_wedge_absent() { # IP: no REJECT rule for the address remains
	node_b_q "iptables -S OUTPUT | grep -F -- '-d $1/32' | grep -c REJECT || true"
}

# ------------------------------------------------------------------- driver

# proof_main "$@": parse, guard, then run the selected steps. A script defines
# step_def lines and step_N functions, then ends with proof_main "$@".
proof_main() {
	local n kind title run
	proof_parse "$@"
	proof_guard_args 0
	if [ "$PROOF_LIST" = 1 ]; then
		for n in $PROOF_STEPS; do
			kind=PROOF_STEP_KIND_$n
			title=PROOF_STEP_TITLE_$n
			printf '%s\t%s\t%s\n' "$n" "${!kind}" "${!title}"
		done
		exit 0
	fi
	if [ "$PROOF_MUTATE" = 1 ] && [ -z "$PROOF_STEP" ]; then
		die 2 "--mutate only works with --step N: a mutating step is never run as part of a pass"
	fi
	proof_need jq
	run=""
	if [ -n "$PROOF_STEP" ]; then
		case " $PROOF_STEPS " in *" $PROOF_STEP "*) run=$PROOF_STEP ;; *) die 2 "no step $PROOF_STEP (try --list)" ;; esac
	else
		for n in $PROOF_STEPS; do
			kind=PROOF_STEP_KIND_$n
			[ "${!kind}" = ro ] && run="$run $n"
		done
	fi
	for n in $run; do
		kind=PROOF_STEP_KIND_$n
		title=PROOF_STEP_TITLE_$n
		kind=${!kind}
		title=${!title}
		PROOF_CUR_STEP=$n
		if [ "$kind" = mut ]; then
			proof_guard_args 1
			if [ "$PROOF_MUTATE" != 1 ] && [ "$PROOF_DRY" != 1 ]; then
				die 2 "step $n ($title) changes state: rerun with --step $n --mutate"
			fi
		fi
		if [ "$PROOF_DRY" = 1 ]; then info "begin: $title [$kind] (dry-run)"; else info "begin: $title [$kind]"; fi
		"step_$n"
		info "end"
	done
	PROOF_CUR_STEP="-"
	if [ "$PROOF_FAILS" -gt 0 ]; then
		proof_emit FAIL "$PROOF_FAILS check(s) failed"
		exit 1
	fi
	proof_emit INFO "done, no failed checks"
}
