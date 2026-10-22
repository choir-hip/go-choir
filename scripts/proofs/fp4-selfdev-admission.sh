#!/usr/bin/env bash
# FP4 (ARCH-10 engineering admission, ARCH-04 writer gate) staging drill on a
# TEST computer. Steps are the drill in `next_action` and `acceptance` of
# docs/definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md on
# lane/b10-selfdev-b1fix (frozen diff b0f872bd6..059924c7e):
#   (1) Cast A, wait for its capsule bound; (2) Cast B; read rows: A bound,
#   B open and unbound, the log "waits behind A"; (3) a planned refresh of the
#   test computer while A is live; read after boot: B still open, log "planned
#   boot: queued engineering request B kept and resumed"; (4) A finishes or is
#   cancelled; B binds within seconds; (5) observability objectgraph_writer_gate
#   {mode: observe, counts}. Crash half: on a throwaway computer, kill the guest
#   with B queued; after boot B is cancelled interrupted_by_restart.
#
# The Casts are model turns on the test computer (management casts engineering):
# they are MANUAL lines with the exact instruction. The script reads the
# guest's boot/runtime log (observability recent_log) through Node B for the
# lines each step expects, and runs the two lifecycle acts (steps 3 and 6) only
# with --mutate on a computer that is not the paper, the owner's or the
# delegate. Treat the log phrases below as the station file wrote them; a
# phrase that is absent is a FAIL to read against the real log, not proof.
#
#   scripts/proofs/fp4-selfdev-admission.sh --computer computer-<test> [--target SHA]
#       [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
set -uo pipefail
PROOF_NAME=fp4-admission
. "$(dirname "$0")/lib.sh"

step_def 1 ro "ARCH-04: observability objectgraph_writer_gate reports mode observe and counts"
step_def 2 ro-after "ARCH-10: Cast A then Cast B; B waits behind A, A is untouched"
step_def 3 mut "ARCH-10: a planned refresh of the test computer while A is live"
step_def 4 ro-after "ARCH-10: after boot B is kept and resumed"
step_def 5 ro-after "ARCH-10: when A goes terminal B binds within seconds"
step_def 6 mut "ARCH-10 crash half: operator refresh (crash boot) of a THROWAWAY computer with B queued"
step_def 7 ro-after "ARCH-10 crash half: B is cancelled interrupted_by_restart"

: "${OPT_since:=}"

fp4_target() { resolve_target "${OPT_computer:-}" "${1:-0}"; }
fp4_log() { fp4_target; guest_get /api/runtime/observability | jq -r '.recent_log[]?' | grep -E "$1" || true; }

fp4_expect_log() { # regex "what"
	local lines
	lines=$(fp4_log "$1")
	if [ "$PROOF_DRY" = 1 ]; then return 0; fi
	printf '%s\n' "$lines" | sed '/^$/d' | tail -n 12 | while IFS= read -r l; do info "log $l"; done
	printf '%s\n' "$lines" | grep -Eq "$1" && pass "$2" || fail "$2: no matching line in the guest log"
}

step_1() {
	local obs
	fp4_target
	obs=$(guest_get /api/runtime/observability)
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$obs" | jq -c '.objectgraph_writer_gate // null' | while IFS= read -r l; do info "writer gate $l"; done
	printf '%s' "$obs" | jq -e '.objectgraph_writer_gate.mode == "observe"' >/dev/null && pass "objectgraph_writer_gate.mode is observe" || fail "objectgraph_writer_gate missing or not observe"
	printf '%s' "$obs" | jq -e '.objectgraph_writer_gate | has("counts")' >/dev/null 2>&1 && pass "counts are reported" || fail "no counts reported"
	printf '%s' "$obs" | jq -r '.recent_log[]? | select(test("writer gate|writer_gate"))' | tail -n 5 | while IFS= read -r l; do info "log $l"; done
}

step_2() {
	fp4_target
	manual "(1) ask management on ${OPT_computer:-the test computer} to Cast an engineering request A; wait until A's capsule is bound (the assignment row shows a bound capsule)"
	manual "(2) while A is working, Cast a second engineering request B"
	manual "(read the rows) A stays bound and unrevoked; B stays open with no capsule and retries; the first assignment is not cancelled"
	fp4_expect_log 'waits behind' "the log names the assignment B waits behind"
}

step_3() {
	local body
	fp4_target 1
	[ -n "${OPT_target:-}" ] || die 2 "step 3 needs --target SHA (the deploy target the planned refresh names)"
	body=$(jq -nc --arg u "$T_USER" --arg d "$T_DESKTOP" --arg t "$OPT_target" '{user_id:$u,desktop_id:$d,target:$t}')
	[ "$T_CLASS" = critical_protected ] || info "class is '$T_CLASS': a busy computer that is not critical_protected defers the refresh (the expected answer 'deferred' is not a failure of the drill, it means pick a protected test computer)"
	will "planned refresh of the TEST computer $T_COMPUTER ($T_VM) while A is live: POST /internal/vmctl/deploy-refresh $body"
	vmctl_post /internal/vmctl/deploy-refresh "$body" proof.fp4-planned-refresh
	manual "record the answer above (outcome refreshed, planned_restart true) and the CI-style reason"
}

step_4() {
	fp4_target
	proof_poll 300 30 "ownership is active after the refresh" fp4_is_active
	fp4_expect_log 'planned boot: queued engineering request .* kept and resumed' "after the planned boot B is kept and resumed"
	manual "read the rows after boot: B is still open"
}
fp4_is_active() { [ "$(proof_row_json "$T_COMPUTER" | jq -r '.state // ""')" = active ]; }

step_5() {
	fp4_target
	manual "let A finish, or Cancel it, then rerun this step: B binds within seconds (the nudge), not at the next minute retry"
	fp4_expect_log 'engineering.*(admitted|bound)|(admitted|bound).*engineering' "an engineering admission/bind line follows A going terminal"
}

step_6() {
	local body
	fp4_target 1
	body=$(jq -nc --arg u "$T_USER" --arg d "$T_DESKTOP" '{user_id:$u,desktop_id:$d}')
	will "operator refresh (a CRASH boot: closes open work) of the THROWAWAY computer $T_COMPUTER ($T_VM): POST /internal/vmctl/refresh $body; B must already be queued behind a live A"
	vmctl_post /internal/vmctl/refresh "$body" proof.fp4-crash
}

step_7() {
	fp4_target
	proof_poll 300 30 "ownership is active after the crash boot" fp4_is_active
	fp4_expect_log 'interrupted_by_restart|interrupted by a restart' "B is cancelled interrupted_by_restart after the crash boot"
}

proof_main "$@"
