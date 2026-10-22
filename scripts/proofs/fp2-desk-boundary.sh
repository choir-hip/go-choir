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
# Steps 1, 2 and 5 use the PRODUCT API: CHOIR_API_KEY must be the key minted
# for the test computer by scripts/qa_test_computer.mjs. The key is never
# printed, and never passed in argv; for the probe it is written to a 0600
# temp credential file, removed on exit. Steps 4 and 6 (the OOM and seccomp
# legs) are host-internal. They resolve the test computer through vmctl's
# list and POST the guest's /internal/runtime/isolation-selftest from Node B
# over ssh, with no credential: the internal service token is never sent to a
# guest; the guest's source-address check is the gate. No API key is used
# for them.
#
#   CHOIR_API_KEY=... scripts/proofs/fp2-desk-boundary.sh --computer computer-<test> [--commit SHA]
#       [--probe scripts/desk_cell_confinement_probe.py] [--egress yes] [--out docs/evidence/fp2-desk-boundary-<date>.json]
#       [--oom-out docs/evidence/fp2-desk-oom-<date>.json] [--seccomp-out docs/evidence/fp2-capsule-seccomp-<date>.json] [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
set -uo pipefail
PROOF_NAME=fp2-desk
. "$(dirname "$0")/lib.sh"

step_def 1 ro "deployed commit: platform /health and the test computer's observability"
step_def 2 mut "the isolation self-test on the test computer, judged by the confinement probe"
step_def 3 ro-after "trace refusals (read by hand)"
step_def 4 mut "out-of-memory: the self-test's host-internal OOM leg, from Node B (a research worker killed by its cgroup; daemon and sibling live)"
step_def 5 mut "real turns: the Texture acceptance spec on the test computer"
step_def 6 mut "capsule seccomp on the deployed kernel: the self-test's host-internal seccomp leg, from Node B (SEC-13)"
step_def 7 ro-after "rollback"

: "${OPT_probe:=scripts/desk_cell_confinement_probe.py}" "${OPT_egress:=no}" "${OPT_commit:=}" "${OPT_out:=}" "${OPT_oom_out:=}" "${OPT_seccomp_out:=}"

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
	# The key is read from the environment by jq, never passed in argv.
	jq -nc --arg c "$OPT_computer" '{api_key: env.CHOIR_API_KEY, computer_id: $c}' >"$cred"
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

# fp2_internal_leg LEG OUT: run the self-test's host-internal LEG on the test
# computer through the probe, from Node B over ssh. The guest address comes
# from vmctl's list. The guest gets no credential (the internal service token
# never goes to a guest; vmctl's list read is loopback). No API key is read
# here.
fp2_internal_leg() {
	local leg=$1 out=$2 rc
	resolve_target "$OPT_computer" 1
	if [ "$PROOF_DRY" = 1 ]; then
		info "\$ python3 -I $OPT_probe --leg $leg --ssh $PROOF_SSH_HOST --guest-url $T_URL --computer $T_COMPUTER --out $out"
		return 0
	fi
	[ -f "$OPT_probe" ] || die 2 "$OPT_probe is not in this tree; pass --probe PATH"
	python3 -I "$OPT_probe" --leg "$leg" --ssh "$PROOF_SSH_HOST" --guest-url "$T_URL" --computer "$T_COMPUTER" --out "$out" >/dev/null
	rc=$?
	return $rc
}

# Step 4 runs the self-test's OOM leg (POST
# /internal/runtime/isolation-selftest?leg=oom on the guest, from Node B). No
# model is in the loop: the deployed desks refuse a runaway prompt
# (docs/problems/fp2-step4-deployed-oom-not-drivable-desk-refuses-runaway-2026-10-22.md).
# The daemon's identity comes from the daemon process (pid, /proc/self/stat
# start time, boot id), not from observability's steps.recent. A desk
# worker's cgroup reports memory.max and memory.events; memory.peak is logged
# for capsules only.
step_4() {
	local rc out
	fp2_need_computer
	out=${OPT_oom_out:-docs/evidence/fp2-desk-oom-$(date -u +%Y%m%d).json}
	will "POST /internal/runtime/isolation-selftest?leg=oom on $OPT_computer from $PROOF_SSH_HOST through $OPT_probe. If MemAvailable covers 768 MiB + 1 GiB and a sibling answers, a throwaway research worker allocates until its cgroup kills it. One attempt, never retried. Writes $out"
	fp2_internal_leg oom "$out"
	rc=$?
	[ "$PROOF_DRY" = 1 ] && return 0
	case "$rc" in
	0) pass "OOM leg proven (exit 0): the worker was killed by its cgroup, the daemon and the sibling lived; receipt $out" ;;
	1) fail "OOM leg: a check failed or the guest gave no answer (exit 1); receipt $out" ;;
	*) fail "OOM leg: not deployed, not authorized or unreachable (exit $rc); nothing proven" ;;
	esac
	if [ -s "$out" ]; then
		jq -c '.server.oom // {} | {mode, mem_available_bytes, required_available_bytes, cell_started: .worker.cell_started, oom_kill: .worker.oom_kill, worker_memory_max: .worker.cgroup.memory_max, worker_memory_high: .worker.cgroup.memory_high, slice_memory_max: .slice.memory_max, slice_memory_high: .slice.memory_high, exit_cause: .worker.exit_cause, sibling_after: .sibling.after_answered, daemon_before: .daemon_before, daemon_after: .daemon_after}' "$out" 2>/dev/null | while IFS= read -r l; do info "oom $l"; done
		jq -c '.failures // []' "$out" 2>/dev/null | while IFS= read -r l; do info "failures $l"; done
	fi
	manual "the next real research turn on this computer works (step 5)"
}

step_5() {
	fp2_need_computer
	will "run the deployed Texture acceptance spec against https://choir.news (creates documents on the test computer's session): cd frontend && BASE_URL=https://choir.news npx playwright test tests/texture-acceptance-deployed.spec.js"
	if [ "$PROOF_DRY" = 1 ]; then return 0; fi
	(cd "$(dirname "$0")/../../frontend" && BASE_URL=https://choir.news npx playwright test tests/texture-acceptance-deployed.spec.js) && pass "texture-acceptance-deployed passed" || fail "texture-acceptance-deployed failed"
	manual "management, research and Texture each complete a turn on this computer, and one engineering assignment completes (proves the capsule seccomp, the session worker's new limits, and that nothing inside needed clone3)"
}

step_7() {
	manual "rollback is git revert of the lane's commits and the Landing Loop; the guest image carries the unit environment, so a revert is a deploy"
}

# Step 6 runs the self-test's seccomp leg (POST
# /internal/runtime/isolation-selftest?leg=seccomp on the guest, from Node
# B). One child per capsule filter loads the capsule's own filter and tries
# clone3 and a new user namespace. It proves the filters on the deployed
# kernel, not that a live capsule loads them, and nothing about the session
# worker's bounds (RES-03).
step_6() {
	local rc out
	fp2_need_computer
	out=${OPT_seccomp_out:-docs/evidence/fp2-capsule-seccomp-$(date -u +%Y%m%d).json}
	will "POST /internal/runtime/isolation-selftest?leg=seccomp on $OPT_computer from $PROOF_SSH_HOST through $OPT_probe (six throwaway children, one per capsule filter; nothing persists), writing $out"
	fp2_internal_leg seccomp "$out"
	rc=$?
	[ "$PROOF_DRY" = 1 ] && return 0
	case "$rc" in
	0) pass "seccomp leg proven (exit 0): every capsule filter answers clone3 ENOSYS and refuses a new user namespace on the deployed kernel; receipt $out" ;;
	1) fail "seccomp leg: a check failed (exit 1); receipt $out" ;;
	*) fail "seccomp leg: not deployed, not authorized or unreachable (exit $rc); nothing proven" ;;
	esac
	if [ -s "$out" ]; then
		jq -c '.server.seccomp // {} | {kernel, filters: [.filters[]? | {name, bpf_sha256, seccomp, probes}]}' "$out" 2>/dev/null | while IFS= read -r l; do info "seccomp $l"; done
		jq -c '.failures // []' "$out" 2>/dev/null | while IFS= read -r l; do info "failures $l"; done
	fi
}

proof_main "$@"
