#!/usr/bin/env bash
# FP2 (batch B3, the guest desk boundary) deployed proof on a DISPOSABLE test
# computer, after the deploy carrying the batch and never first on the paper.
# Steps are "Staging probe steps (for FP0; no secrets here)" in
# docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md on
# lane/b3-desk-boundary-b1fix, numbered the same. The confinement probe itself
# is scripts/desk_cell_confinement_probe.py (on that branch, on main once it
# lands); this script runs it and judges its exit status (0 proven, 1 a check
# failed, 2 not deployed or unreachable).
#
# Uses the PRODUCT API only (no ssh): CHOIR_API_KEY must be the key minted for
# the test computer by scripts/qa_test_computer.mjs. The key is never printed;
# for the probe it is written to a 0600 temp credential file removed on exit.
#
#   CHOIR_API_KEY=... scripts/proofs/fp2-desk-boundary.sh --computer computer-<test> [--commit SHA]
#       [--probe scripts/desk_cell_confinement_probe.py] [--egress yes] [--out docs/evidence/fp2-desk-boundary-<date>.json]
#       [--label before|after] [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
set -uo pipefail
PROOF_NAME=fp2-desk
. "$(dirname "$0")/lib.sh"

step_def 1 ro "deployed commit: platform /health and the test computer's observability"
step_def 2 mut "the isolation self-test on the test computer, judged by the confinement probe"
step_def 3 ro-after "trace refusals (read by hand)"
step_def 4 ro-after "out-of-memory: daemon identity before and after a runaway research cell"
step_def 5 mut "real turns: the Texture acceptance spec on the test computer"
step_def 6 ro-after "rollback"

: "${OPT_probe:=scripts/desk_cell_confinement_probe.py}" "${OPT_egress:=no}" "${OPT_label:=before}" "${OPT_commit:=}" "${OPT_out:=}"

fp2_need_computer() {
	[ -n "${OPT_computer:-}" ] || die 2 "this step needs --computer (the disposable test computer)"
	proof_guard_value "$OPT_computer" 1
}

step_1() {
	local plat guest
	fp2_need_computer
	plat=$(api_public /health)
	guest=$(api_get /api/runtime/observability "$OPT_computer")
	[ "$PROOF_DRY" = 1 ] && return 0
	info "platform /health: $(printf '%s' "$plat" | jq -c '{commit: (.build.commit // .commit // null), status}' 2>/dev/null)"
	info "guest observability: $(printf '%s' "$guest" | jq -c '{service, computer_id, commit: .build.commit}' 2>/dev/null)"
	if [ -n "$OPT_commit" ]; then
		printf '%s' "$plat" | jq -e --arg c "$OPT_commit" '(.build.commit // .commit // "") | startswith($c)' >/dev/null && pass "platform serves $OPT_commit" || fail "platform does not serve $OPT_commit"
		printf '%s' "$guest" | jq -e --arg c "$OPT_commit" '(.build.commit // "") | startswith($c)' >/dev/null && pass "the guest runs $OPT_commit" || fail "the guest does not run $OPT_commit (cold boot it onto the new guest image: an idle computer picks it up on its next refresh)"
	fi
	printf '%s' "$guest" | jq -r '.recent_log[]? | select(test("desk cgroup"))' | while IFS= read -r l; do info "log $l"; done
}

step_2() {
	local cred rc out
	fp2_need_computer
	out=${OPT_out:-docs/evidence/fp2-desk-boundary-$(date -u +%Y%m%d).json}
	will "POST /api/runtime/isolation-selftest on $OPT_computer through $OPT_probe (spawns one worker per desk and acts as an escaped cell; nothing persists), writing $out"
	if [ "$PROOF_DRY" = 1 ]; then
		info "\$ python3 $OPT_probe --cred <0600 temp file from CHOIR_API_KEY and --computer> --out $out$([ "$OPT_egress" = yes ] || printf ' --skip-egress')"
		return 0
	fi
	[ -f "$OPT_probe" ] || die 2 "$OPT_probe is not in this tree (it lands with the FP2 batch); pass --probe PATH"
	[ -n "${CHOIR_API_KEY:-}" ] || die 2 "CHOIR_API_KEY is not set in the environment"
	cred=$(umask 077 && mktemp "${TMPDIR:-/tmp}/fp2-cred.XXXXXX") || die 2 "mktemp failed"
	trap 'rm -f "${cred:-}"' EXIT
	jq -nc --arg k "$CHOIR_API_KEY" --arg c "$OPT_computer" '{api_key:$k, computer_id:$c}' >"$cred"
	if [ "$OPT_egress" = yes ]; then
		python3 -I "$OPT_probe" --cred "$cred" --out "$out" >/dev/null
	else
		python3 -I "$OPT_probe" --cred "$cred" --out "$out" --skip-egress >/dev/null
	fi
	rc=$?
	rm -f "$cred"
	case "$rc" in
	0) pass "confinement probe proven (exit 0); receipt $out" ;;
	1) fail "confinement probe: a check failed (exit 1); receipt $out" ;;
	*) fail "confinement probe: route not deployed or computer unreachable (exit $rc); nothing proven" ;;
	esac
	if [ -s "$out" ]; then
		jq -c '{verdict, desk_cells: (.server.desk_cells.ok // null), mode: (.server.desk_cells.mode // null), frame_boundary: (.server.frame_boundary.ok // null), failures}' "$out" 2>/dev/null | while IFS= read -r l; do info "receipt $l"; done
	fi
	[ "$OPT_egress" = yes ] || info "egress legs skipped (--egress yes after FP3's host egress policy is deployed)"
}

step_3() {
	manual "a run of the test computer after step 2 expects nothing; a real refusal appears as a desk.egress run event with refused: true (read the run's events in the product)"
}

fp2_state_file() { printf '%s/fp2-daemon-%s.json' "${TMPDIR:-/tmp}" "$OPT_computer"; }

step_4() {
	local obs now state
	fp2_need_computer
	obs=$(api_get /api/runtime/observability "$OPT_computer")
	[ "$PROOF_DRY" = 1 ] && return 0
	# The daemon's identity: whatever the observability answer carries about
	# process start, pid or uptime, compared before and after.
	now=$(printf '%s' "$obs" | jq -S -c '[paths(scalars) as $p | {path: ($p | map(tostring) | join(".")), value: getpath($p)}] | map(select(.path | test("(^|\\.)(started_at|start_time|boot_time|pid|process_start|uptime)"; "i")))')
	state=$(fp2_state_file)
	info "daemon identity fields: $now"
	if [ "$OPT_label" = before ]; then
		printf '%s\n' "$now" >"$state"
		info "saved to $state"
		manual "run a research desk turn whose cell allocates without end (a prompt that asks for a cell: for { keep = append(keep, make([]byte, 8<<20)) }); expect the cell result \"cell's worker died ... killed by its memory budget (memory.max 768 MiB ...)\"; then rerun this step with --label after"
	else
		[ -s "$state" ] || die 2 "no saved 'before' reading at $state (run with --label before first)"
		[ "$now" = "$(cat "$state")" ] && pass "the daemon's start identity is unchanged (the worker was killed, not the daemon)" || fail "the daemon identity changed: before $(cat "$state") after $now"
		printf '%s' "$obs" | jq -r '.recent_log[]? | select(test("memory budget|memory.peak|desk cgroup"))' | while IFS= read -r l; do info "log $l"; done
		manual "the next turn of the same desk works; read each worker's memory.peak from the daemon's desk cgroup log lines above"
	fi
}

step_5() {
	fp2_need_computer
	will "run the deployed Texture acceptance spec against https://choir.news (creates documents on the test computer's session): cd frontend && BASE_URL=https://choir.news npx playwright test tests/texture-acceptance-deployed.spec.js"
	if [ "$PROOF_DRY" = 1 ]; then return 0; fi
	(cd "$(dirname "$0")/../../frontend" && BASE_URL=https://choir.news npx playwright test tests/texture-acceptance-deployed.spec.js) && pass "texture-acceptance-deployed passed" || fail "texture-acceptance-deployed failed"
	manual "management, research and Texture each complete a turn on this computer, and one engineering assignment completes (proves the capsule seccomp, the session worker's new limits, and that nothing inside needed clone3)"
}

step_6() {
	manual "rollback is git revert of the lane's commits and the Landing Loop; the guest image carries the unit environment, so a revert is a deploy"
}

proof_main "$@"
