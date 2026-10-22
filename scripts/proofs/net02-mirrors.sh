#!/usr/bin/env bash
# NET-02 (station FP10: capsule package mirrors) landing checks and proof on a
# TEST computer. Steps are "Landing and proof (for FP0)" in
# docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md on
# lane/fp10-net02-mirrors, in that order.
#
# The probe itself (scripts/capsule-mirror-probe.sh) must run INSIDE an
# implementation capsule on the granted computer; no channel from here reaches
# it, so the capsule runs are MANUAL lines. Save the probe's stdout and pass it
# with --probe-output FILE: this script judges its summary line and checks.
# Everything else is read-only on Node B. The paper, the owner's computer and
# the delegate are refused as targets.
#
#   scripts/proofs/net02-mirrors.sh [--computer computer-<test>] [--computer2 computer-<test2>]
#       [--mode route|only] [--ecosystems npm,pypi,git] [--probe-output FILE]
#       [--since "2026-10-22 10:00:00 UTC"] [--dry-run | --list | --step N] [--evidence FILE]
#
# Order: 0 after the third landing commit (the deploy); 1; the capsule run, then
# 2 --mode route; 3 --mode only; 4; 5.
set -uo pipefail
PROOF_NAME=net02-mirrors
. "$(dirname "$0")/lib.sh"

step_def 0 ro "landing: go-choir-pkgmirror active, /healthz shows no ecosystems, vmctl lists empty"
step_def 1 ro-after "grant: vmctl 'capsule mirrors for computer ... granted' and the unit's ecosystems"
step_def 2 ro-after "route mode: the probe reports failures 0; pkgmirror 'fetched' records"
step_def 3 ro-after "mirrors only: failures 0; refused CONNECTs to the registries"
step_def 4 ro-after "sharing: a second computer hits the first one's fetches"
step_def 5 ro-after "rollback drill: the probe's *-env-absent checks pass"

: "${OPT_mode:=route}" "${OPT_ecosystems:=npm,pypi,git}" "${OPT_since:=2 hours ago}" "${OPT_probe_output:=}"

nm_targets() {
	local t
	for t in "${OPT_computer:-}" "${OPT_computer2:-}"; do
		[ -z "$t" ] || proof_guard_value "$t" 1
	done
}

# nm_judge_probe MODE: read the saved probe stdout.
nm_judge_probe() {
	[ -n "$OPT_probe_output" ] || { manual "no --probe-output given: run 'bash scripts/capsule-mirror-probe.sh $1 $OPT_ecosystems' as a capsule job on the granted computer, save its stdout, rerun this step with --probe-output FILE"; return 0; }
	[ "$PROOF_DRY" = 1 ] && return 0
	[ -s "$OPT_probe_output" ] || die 2 "$OPT_probe_output is empty or missing"
	while IFS= read -r l; do fail "$l"; done < <(grep -v '^{"summary"' "$OPT_probe_output" | jq -r 'select(.got != .want) | "check \(.check) wanted \(.want) got \(.got) rc=\(.rc)"' 2>/dev/null)
	jq -e --arg m "$1" 'select(.summary == "capsule-mirror-probe") | .mode == $m and .failures == 0' "$OPT_probe_output" >/dev/null 2>&1 && pass "probe summary: mode=$1 failures=0" || fail "probe summary is not mode=$1 failures=0"
}

step_0() {
	local h
	info "go-choir-pkgmirror unit and healthz on Node B:"
	h=$(node_b_q "systemctl is-active go-choir-pkgmirror; curl -fsS --max-time 10 http://127.0.0.1:8089/healthz")
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s\n' "$h" | while IFS= read -r l; do info "$l"; done
	printf '%s\n' "$h" | head -1 | grep -qx active && pass "go-choir-pkgmirror is active" || fail "go-choir-pkgmirror is not active"
	printf '%s\n' "$h" | tail -n +2 | jq -e '(.ecosystems // []) | length == 0' >/dev/null 2>&1 && pass "/healthz shows \"ecosystems\":[]" || fail "/healthz does not show an empty ecosystems list"
	node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "capsule mirrors for computer" 3 | grep -q . && fail "vmctl already lists a computer for capsule mirrors" || pass "no capsule mirror grants yet"
	manual "CI built goServices with cmd/pkgmirror; the paper's schedules and polls read the same before and after (paper liveness probe)"
}

step_1() {
	nm_targets
	manual "grant: in nix/node-b.nix set the unit's -ecosystems $OPT_ecosystems and go-choir-vmctl's CHOIR_CAPSULE_MIRROR_COMPUTERS=${OPT_computer:-<test ComputerID>} and CHOIR_CAPSULE_MIRROR_ECOSYSTEMS=$OPT_ecosystems; land, deploy, then COLD boot the test computer (a snapshot resume keeps the old credential disk)"
	node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "capsule mirrors for computer" 5
	node_b_sh "curl -fsS --max-time 10 http://127.0.0.1:8089/healthz"
	if [ "$PROOF_DRY" != 1 ]; then
		node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "capsule mirrors for computer.*granted \\(computer\\)" 3 | grep -q . && pass "vmctl: capsule mirrors granted (computer)" || fail "no 'capsule mirrors ... granted (computer)' line"
	fi
}

step_2() {
	nm_targets
	manual "from an implementation capsule on ${OPT_computer:-the granted computer} (a capsule job): bash scripts/capsule-mirror-probe.sh route $OPT_ecosystems  (summary \"failures\":0)"
	nm_judge_probe route
	info "go-choir-pkgmirror 'fetched' records:"
	node_journal go-choir-pkgmirror "$OPT_since" "fetched" 10
	[ "$PROOF_DRY" = 1 ] || { node_journal go-choir-pkgmirror "$OPT_since" "fetched" 1 | grep -q . && pass "pkgmirror logged fetched records" || fail "no 'fetched' record in the pkgmirror journal"; }
	manual "the capsule's egress ledger shows the mirror requests as host mirror.choir.internal with outcome ok"
}

step_3() {
	nm_targets
	manual "set CHOIR_CAPSULE_MIRRORS_ONLY=1 (nix), land, deploy, cold-boot the computer, then in a capsule: bash scripts/capsule-mirror-probe.sh only $OPT_ecosystems"
	nm_judge_probe only
	manual "the egress ledger shows refused CONNECTs to registry.npmjs.org, pypi.org and github.com, and huggingface.co still ok"
}

step_4() {
	nm_targets
	manual "add a second test computer${OPT_computer2:+ ($OPT_computer2)} (or class:test), cold-boot it, run the probe there"
	info "pkgmirror 'hit' records (tarballs, files and repository the first computer fetched):"
	node_journal go-choir-pkgmirror "$OPT_since" "hit" 10
	[ "$PROOF_DRY" = 1 ] || { node_journal go-choir-pkgmirror "$OPT_since" "hit" 1 | grep -q . && pass "pkgmirror logged hits" || fail "no 'hit' record: the second computer did not share the first one's fetches"; }
}

step_5() {
	nm_targets
	manual "rollback drill: empty the computer list (nix), deploy, cold-boot; run the probe in a capsule and pass its stdout as --probe-output"
	if [ -n "$OPT_probe_output" ] && [ "$PROOF_DRY" != 1 ]; then
		for c in npm-env-absent pypi-env-absent; do
			jq -e --arg c "$c" 'select(.check == $c) | .got == "pass"' "$OPT_probe_output" >/dev/null 2>&1 && pass "$c passed" || fail "$c did not pass"
		done
	fi
}

proof_main "$@"
