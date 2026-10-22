#!/usr/bin/env bash
# NET-01 (station FP10: the host fetch service) deployed proof on a TEST
# computer P. Steps are "FP0 proof steps" in
# docs/definitions/choir-launch-fp10-net01-host-fetch-2026-10-21.md on
# lane/fp10-net01-host-fetch, numbered the same. The station's own tool is
# scripts/host-fetch-proof.py: its `api` mode drives imports, a transcript,
# podcast search and subscribe and an RSS schedule through the PUBLIC API on P
# (so it changes P's state: the steps that run it are mutating), and its
# `journal` mode summarizes the gateway's per-hop lines on Node B (this script
# streams the file to `python3 -` over ssh; nothing is copied to the host).
#
# P is a TEST computer. The paper, the owner's computer and the delegate are
# refused for every step.
#
#   CHOIR_API_KEY=... scripts/proofs/net01-host-fetch.sh --computer computer-<P>
#       [--tool scripts/host-fetch-proof.py] [--since "2026-10-22 10:00:00 UTC"]
#       [--out-dir docs/evidence] [--duration 120]
#       [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
#
# Order: 1 (--mutate), 2; grant P by nix + deploy + cold boot; 3 (read), 4
# (--mutate) with the research Reads, 5, 6 MANUAL then 5 again, 7 during the
# fetches, 8; the rollback drill (empty the list, deploy, cold boot) then 1 again.
set -uo pipefail
PROOF_NAME=net01-hostfetch
. "$(dirname "$0")/lib.sh"

step_def 1 mut "ungranted baseline: host-fetch-proof.py api on P (every check PASS)"
step_def 2 ro-after "ungranted baseline journal: no 'gateway: fetch' lines for P"
step_def 3 ro-after "granted: vmctl logs 'host fetch route for computer P ... granted'"
step_def 4 mut "granted: host-fetch-proof.py api on P again (capabilities 1 and 3 through the host)"
step_def 5 ro-after "the journal shows the hops (ok hosts, three refusals)"
step_def 6 ro-after "adversarial reads (FP0's): refusals in the journal"
step_def 7 ro-after "no direct egress from P: conntrack sampled during the fetches"
step_def 8 ro-after "budget headroom: P's hop count in the last minute"

: "${OPT_tool:=scripts/host-fetch-proof.py}" "${OPT_since:=2 hours ago}" "${OPT_out_dir:=docs/evidence}" "${OPT_duration:=120}"

nf_target() { resolve_target "${OPT_computer:-}" 1; }

nf_tool() { [ -f "$OPT_tool" ] || die 2 "$OPT_tool is not in this tree (it lands with the NET-01 batch); pass --tool PATH"; }

# nf_api LABEL: the station tool's api mode; the key reaches it only through
# the environment.
nf_api() {
	local out
	out="$OPT_out_dir/net01-api-$1-$(date -u +%Y%m%dT%H%M%SZ).json"
	will "run $OPT_tool api --computer ${OPT_computer:-P} --out $out (drives imports, a transcript, podcast search/subscribe and an RSS schedule on P)"
	if [ "$PROOF_DRY" = 1 ]; then
		info "\$ CHOIR_API_KEY=<env> python3 -I $OPT_tool api --computer ${OPT_computer:-P} --host $PROOF_API_HOST --out $out"
		return 0
	fi
	nf_tool
	[ -n "${CHOIR_API_KEY:-}" ] || die 2 "CHOIR_API_KEY is not set in the environment"
	python3 -I "$OPT_tool" api --computer "$OPT_computer" --host "$PROOF_API_HOST" --out "$out" && pass "host-fetch-proof api: every check PASS ($out)" || fail "host-fetch-proof api: a check failed ($out)"
}

# nf_journal [IDENTITY]: the station tool's journal mode, run on Node B from stdin.
nf_journal() {
	local ident=${1:-} out
	out="$OPT_out_dir/net01-journal-$(date -u +%Y%m%dT%H%M%SZ).json"
	if [ "$PROOF_DRY" = 1 ]; then
		info "\$ ssh $PROOF_SSH_HOST python3 - journal --since '$OPT_since'${ident:+ --identity $ident} < $OPT_tool"
		return 0
	fi
	nf_tool
	"$PROOF_SSH_BIN" -o BatchMode=yes -o ConnectTimeout=15 "$PROOF_SSH_HOST" python3 - journal --since "'$OPT_since'" ${ident:+--identity "$ident"} <"$OPT_tool" | tee "$out" | jq -c '.' 2>/dev/null | head -c 4000 | while IFS= read -r l; do info "journal $l"; done
	info "journal summary kept at $out"
	NF_LAST_JOURNAL=$out
}

step_1() {
	nf_target
	[ "$PROOF_DRY" = 1 ] || info "P=$T_COMPUTER vm=$T_VM; CHOIR_HOST_FETCH_COMPUTERS must be empty for this baseline"
	nf_api ungranted
	manual "the paper's schedules keep polling (the paper liveness probe stays ok)"
}

step_2() {
	nf_target
	nf_journal "$T_VM"
	if [ "$PROOF_DRY" != 1 ]; then
		node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "gateway: fetch.*computer=$T_VM" 3 | grep -q . && fail "gateway: fetch lines exist for P while ungranted" || pass "no 'gateway: fetch' lines for P"
	fi
}

step_3() {
	nf_target
	manual "grant: set \"CHOIR_HOST_FETCH_COMPUTERS=$T_COMPUTER\" in nix/node-b.nix, land, deploy, then COLD boot P (stop and start, not a snapshot resume)"
	node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "host fetch route for computer $T_COMPUTER" 5
	if [ "$PROOF_DRY" != 1 ]; then
		node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "host fetch route for computer $T_COMPUTER.*granted \\(computer\\)" 3 | grep -q . && pass "vmctl: host fetch route granted (computer)" || fail "no 'granted (computer)' line for $T_COMPUTER"
		guest_get /api/runtime/observability | jq -r '.recent_log[]? | select(test("host fetch route"))' | tail -n 3 | while IFS= read -r l; do info "guest log $l"; done
	fi
}

step_4() {
	nf_target
	nf_api granted
	manual "then ask P's research desk to Read https://en.wikipedia.org/wiki/Special:Random and http://portquiz.net:8080/ (a non-default port): both return page text"
}

step_5() {
	nf_target
	nf_journal "$T_VM"
	info "expect ok hops for en.wikipedia.org, www.youtube.com, itunes.apple.com, feeds.npr.org, feeds.bbci.co.uk and the article hosts, and refused lines for 169.254.169.254, localtest.me and 10.0.0.1"
	if [ "$PROOF_DRY" != 1 ]; then
		local h
		for h in en.wikipedia.org www.youtube.com itunes.apple.com feeds.npr.org feeds.bbci.co.uk; do
			node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "gateway: fetch.*computer=$T_VM.*host=$h" 1 | grep -q . && pass "ok hop for $h" || fail "no hop for $h"
		done
		for h in 169.254.169.254 localtest.me 10.0.0.1; do
			node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "gateway: fetch refused.*computer=$T_VM.*host=$h" 1 | grep -q . && pass "refused line for $h" || fail "no refused line for $h"
		done
	fi
}

step_6() {
	nf_target
	manual "from P, Read Node B's own public address on :22 and :443 and a name that resolves to Node B's address; then a page that redirects to http://169.254.169.254/ (any public redirector)"
	info "expect: fetch refused ... reason=\"address: ...\" for the first, and for the redirect the guest refuses the second hop with NO second gateway line"
	node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "gateway: fetch refused.*computer=$T_VM.*address:" 10
	[ "$PROOF_DRY" = 1 ] || { node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "gateway: fetch refused.*computer=$T_VM.*address:" 1 | grep -q . && pass "an address refusal is logged for P" || fail "no address refusal line for P"; }
}

step_7() {
	local peer out
	nf_target
	peer="$(printf '%s' "$T_IP" | sed -E 's/\.[0-9]+$/.1/')"
	manual "run the step-4 fetches NOW (the sampler below watches the next ${OPT_duration}s)"
	will "(read-only) sample conntrack on Node B for source $T_IP every 5s for ${OPT_duration}s; any destination other than its host peer $peer (assumed the .1 of the guest's /30) is a direct egress"
	out=$(node_b_q "command -v conntrack >/dev/null 2>&1 || { echo 'NO-CONNTRACK'; exit 0; }
end=\$((\$(date +%s) + $OPT_duration)); while [ \$(date +%s) -lt \$end ]; do conntrack -L -s $T_IP 2>/dev/null; sleep 5; done | grep -o 'dst=[0-9.]* dport=[0-9]*' | sort -u")
	[ "$PROOF_DRY" = 1 ] && return 0
	if [ "$out" = NO-CONNTRACK ]; then
		fail "conntrack is not installed on Node B; use tcpdump on P's tap instead (traffic from $T_IP to anything but $peer:80/443)"
		return 1
	fi
	printf '%s\n' "$out" | while IFS= read -r l; do info "flow $l"; done
	if printf '%s\n' "$out" | grep -v "^dst=$peer " | grep -q .; then fail "destinations other than $peer seen from P"; else pass "only the host peer $peer seen from P"; fi
}

step_8() {
	local n
	nf_target
	n=$(node_b_q "journalctl -u '$PROOF_UNIT_GATEWAY' --since '1 minute ago' --no-pager -o cat 2>/dev/null | grep -c 'gateway: fetch.*computer=$T_VM' || true")
	[ "$PROOF_DRY" = 1 ] && return 0
	info "P's hops in the last minute: ${n:-0} (CHOIR_HOST_FETCH_PER_MINUTE budget default is a guess; stay well under 600)"
	[ "${n:-0}" -lt 300 ] && pass "hop count is well under 600" || fail "hop count ${n} is not well under 600: raise CHOIR_HOST_FETCH_PER_MINUTE before granting the paper"
	manual "before granting the paper's computer: count the paper's peak (feeds per cycle plus article imports); granting the paper is a separate step. Rollback drill: empty the list, deploy, cold-boot P: the guest log says 'host fetch route off' and step 1 passes again"
}

proof_main "$@"
