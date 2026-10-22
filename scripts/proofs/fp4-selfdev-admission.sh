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
# they are MANUAL lines with the exact instruction. Steps 2, 4, 5 and 7 are
# judged on the ARTIFACT, never on a log line (standing question 6;
# docs/problems/fp4-proof-steps-5-and-7-read-the-wrong-evidence-2026-10-22.md):
# the assignment rows (GET /api/trajectories/<id>, co_super_assignments[]) and
# the lifecycle events (GET /api/trajectories/<id>/events), read through the
# product API with CHOIR_API_KEY of the TEST computer, the way lib.sh's api_get
# calls the product. The guest's recent_log is still read through Node B, as
# INFO corroboration only; its absence or presence never decides a step.
# Steps 3 and 6 run only with --mutate on a computer that is not the paper,
# the owner's or the delegate (lib.sh's guard; the row-reading steps refuse the
# paper and the owner too).
#
# Events are matched to rows by command_id == "engineering-bind:<assignment id>"
# (work_item_id is null on real events). Steps 2, 4 and 7 first assert the join
# found a bound event for A; "B was never bound" is never read off an empty join.
#
# A and B are named by flag, not discovered: the trajectory list
# (GET /api/trajectories) carries no assignment kind or order, so "the two
# newest" is a guess, and a guess here would judge the wrong row.
#   --trajectory-a ID   trajectory whose snapshot holds A's assignment row
#   --trajectory-b ID   trajectory holding B's row (default: the same as A)
#   --assignment-a ID / --assignment-b ID
#                       assignment_id of each row. Required when a trajectory
#                       holds more than one row; with exactly one row the id
#                       may be omitted. Ambiguity is a FAIL, never a pick.
#   --nudge-seconds N   step 5 bound on B's bind after A's terminal_at (30;
#                       the minute retry is 60s, so a pass is the nudge)
#
#   BASH_ENV= CHOIR_API_KEY=<test key> scripts/proofs/fp4-selfdev-admission.sh \
#       --computer computer-<test> [--target SHA] --trajectory-a T [--trajectory-b T] \
#       [--assignment-a ID --assignment-b ID] [--nudge-seconds 30]
#       [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
set -uo pipefail
PROOF_NAME=fp4-admission
. "$(dirname "$0")/lib.sh"

: "${OPT_since:=}"
: "${OPT_nudge_seconds:=30}"

fp4_target() { resolve_target "${OPT_computer:-}" "${1:-0}"; }

# fp4_info_log REGEX: the matching guest log lines as INFO. Corroboration only.
fp4_info_log() {
	local lines
	fp4_target 1
	lines=$(guest_get /api/runtime/observability | jq -r '.recent_log[]?' | grep -E "$1" || true)
	if [ "$PROOF_DRY" = 1 ]; then return 0; fi
	if [ -z "$lines" ]; then info "log (corroboration only): no line matches /$1/"; return 0; fi
	printf '%s\n' "$lines" | tail -n 6 | while IFS= read -r l; do info "log (corroboration only) $l"; done
}

# ---- the product-API reads. A snapshot is GET /api/trajectories/<id>; events
# are GET /api/trajectories/<id>/events. JSON on stdout; empty in dry-run.
fp4_traj() { # a|b: the trajectory id for A or B
	if [ "$1" = a ]; then
		[ -n "${OPT_trajectory_a:-}" ] || { [ "$PROOF_DRY" = 1 ] && { printf '<trajectory-a>'; return; }; die 2 "this step needs --trajectory-a"; }
		printf '%s' "$OPT_trajectory_a"
	else
		[ -n "${OPT_trajectory_b:-${OPT_trajectory_a:-}}" ] || { [ "$PROOF_DRY" = 1 ] && { printf '<trajectory-b>'; return; }; die 2 "this step needs --trajectory-b (or --trajectory-a if both rows are in one trajectory)"; }
		printf '%s' "${OPT_trajectory_b:-$OPT_trajectory_a}"
	fi
}
fp4_snapshot() { api_get "/api/trajectories/$1" "${OPT_computer:-}"; }
fp4_events() { api_get "/api/trajectories/$1/events?limit=1000" "${OPT_computer:-}"; }

# fp4_row SNAPSHOT_JSON ASSIGNMENT_ID_OR_EMPTY: print the one assignment row.
# Returns 1 (prints nothing) when absent or when several rows remain and no id
# narrows them: the script never picks.
fp4_row() {
	printf '%s' "$1" | jq -ce --arg id "$2" '
		[(.co_super_assignments // [])[] | select($id == "" or .assignment_id == $id)]
		| if length == 1 then .[0] else empty end' 2>/dev/null
}

# fp4_bound_events EVENTS_JSON ROW_JSON: the row's co_super_assignment_bound
# events, as a JSON array. On the real tape (2026-10-22) work_item_id is null
# on every event and empty on every row, and the join that matches is
# command_id == "engineering-bind:" + assignment_id. A non-empty work_item_id
# equal to the row's assigned work item also matches; an empty one never does.
fp4_bound_events() {
	printf '%s' "$1" | jq -c --argjson row "$2" '
		($row.binding.assigned_work_item_id // "") as $wi
		| [(.events // [])[] | select(.kind == "co_super_assignment_bound"
			and (.command_id == ("engineering-bind:" + $row.assignment_id)
				or ($wi != "" and .work_item_id == $wi)))]' 2>/dev/null
}

# fp4_unmatched_bound EVENTS_JSON ROW_JSON: the command_ids of bound events
# that are not this row's (they belong to another assignment).
fp4_unmatched_bound() {
	printf '%s' "$1" | jq -r --argjson row "$2" '
		($row.binding.assigned_work_item_id // "") as $wi
		| [(.events // [])[] | select(.kind == "co_super_assignment_bound"
			and ((.command_id == ("engineering-bind:" + $row.assignment_id)
				or ($wi != "" and .work_item_id == $wi)) | not)) | .command_id // "<none>"] | join(",")' 2>/dev/null
}

# fp4_load a|b: read the trajectory snapshot and events for A or B, set
# FP4_ROW (the row) and FP4_BOUND (the bound events of that row). Returns 1
# after a FAIL when the row cannot be named.
fp4_load() {
	local which=$1 traj id snap evs up
	up=$(printf '%s' "$which" | tr '[:lower:]' '[:upper:]')
	traj=$(fp4_traj "$which")
	if [ "$which" = a ]; then id=${OPT_assignment_a:-}; else id=${OPT_assignment_b:-}; fi
	if [ "$PROOF_DRY" = 1 ]; then
		fp4_snapshot "$traj"
		fp4_events "$traj"
		FP4_ROW='{}'
		return 0
	fi
	snap=$(fp4_snapshot "$traj")
	evs=$(fp4_events "$traj")
	FP4_ROW=""
	FP4_BOUND="[]"
	if ! FP4_ROW=$(fp4_row "$snap" "$id"); then
		fail "$up: cannot name exactly one assignment row in trajectory $traj (assignment ${id:-<unset>}); pass --assignment-$which"
		return 1
	fi
	FP4_BOUND=$(fp4_bound_events "$evs" "$FP4_ROW") || FP4_BOUND="[]"
	FP4_FOREIGN=$(fp4_unmatched_bound "$evs" "$FP4_ROW")
	info "$up row $(printf '%s' "$FP4_ROW" | jq -c '{assignment_id, disposition, disposition_reason, capsule_disposition, terminal_at, work_item: .binding.assigned_work_item_id}') bound_events=$(printf '%s' "$FP4_BOUND" | jq -c 'map({created_at, command_id})')"
	if [ -n "$FP4_FOREIGN" ]; then info "$up: bound events that are NOT this row's (command_id matches another assignment): $FP4_FOREIGN"; fi
}

# fp4_load_a_sane: load A and assert the bound-event join found at least one
# bound event for A. Without that, "B has no bound event" would be what a join
# that matches nothing also says, so every "never bound" claim stops here.
fp4_load_a_sane() {
	fp4_load a || return 1
	if [ "$(printf '%s' "$FP4_BOUND" | jq 'length')" = 0 ]; then
		fail "the bound-event join found no co_super_assignment_bound event for A (command_id engineering-bind:<assignment id>): a 'B was never bound' result would be vacuous, so it is not asserted"
		return 1
	fi
	pass "the bound-event join found $(printf '%s' "$FP4_BOUND" | jq 'length') bound event(s) for A"
}

fp4_rowf() { printf '%s' "$FP4_ROW" | jq -r "$1"; }
fp4_unbound_events() { [ "$(printf '%s' "$FP4_BOUND" | jq 'length')" = 0 ]; }

# fp4_ts_jq: seconds since the epoch, fraction kept, of an RFC 3339 UTC time.
FP4_TS_JQ='def ts: (sub("\\.[0-9]+";"") | fromdateiso8601) + (([capture("\\.(?<f>[0-9]+)").f] | first // "0") | ("0." + .) | tonumber);'

fp4_a_is_live() { # A: bound, capsule active, not terminal
	[ "$(fp4_rowf '.disposition')" = bound ] || return 1
	case "$(fp4_rowf '.capsule_disposition')" in unbound | revoke_requested | revoked) return 1 ;; esac
	[ "$(fp4_rowf '.terminal_at // ""')" = "" ]
}
fp4_b_is_waiting() { # B: open, capsule unbound, never bound
	[ "$(fp4_rowf '.disposition')" = open ] && [ "$(fp4_rowf '.capsule_disposition')" = unbound ] && fp4_unbound_events
}

step_def 1 ro "ARCH-04: observability objectgraph_writer_gate reports mode observe and counts"
step_def 2 ro-after "ARCH-10: Cast A then Cast B; rows: A bound and live, B open and never bound"
step_def 3 mut "ARCH-10: a planned refresh of the test computer while A is live"
step_def 4 ro-after "ARCH-10: after boot B's row is still open and never bound"
step_def 5 ro-after "ARCH-10: B's bound event follows A's terminal_at within the nudge bound"
step_def 6 mut "ARCH-10 crash half: operator refresh (crash boot) of a THROWAWAY computer with B queued"
step_def 7 ro-after "ARCH-10 crash half: B's row is cancelled interrupted_by_restart and was never bound"

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
	fp4_target 1
	manual "(1) ask management on ${OPT_computer:-the test computer} to Cast an engineering request A; wait until A's capsule is bound (the assignment row shows a bound capsule)"
	manual "(2) while A is working, Cast a second engineering request B; then rerun this step with --trajectory-a/--trajectory-b (and --assignment-a/-b)"
	if [ "$PROOF_DRY" = 1 ]; then fp4_load a; fp4_load b; fp4_info_log 'waits behind'; return 0; fi
	if fp4_load_a_sane; then
		check "A's row is bound with its capsule live (not revoked) and not terminal" fp4_a_is_live
		if fp4_load b; then
			check "B's row is open, capsule unbound, with no co_super_assignment_bound event" fp4_b_is_waiting
		fi
	fi
	fp4_info_log 'waits behind'
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
	fp4_target 1
	proof_poll 300 30 "ownership is active after the refresh" fp4_is_active
	if [ "$PROOF_DRY" = 1 ]; then fp4_load a; fp4_load b; fp4_info_log 'planned boot: queued engineering request .* kept and resumed'; return 0; fi
	if fp4_load_a_sane && fp4_load b; then
		check "after the planned boot B's row is still open, capsule unbound, never bound" fp4_b_is_waiting
	fi
	fp4_info_log 'planned boot: queued engineering request .* kept and resumed'
}
fp4_is_active() { [ "$(proof_row_json "$T_COMPUTER" | jq -r '.state // ""')" = active ]; }

fp4_b_bound_in_time() { # A's terminal_at, then B's first bound event within --nudge-seconds
	local a_term b_bound delta
	a_term=$(printf '%s' "$FP4_A_ROW" | jq -r '.terminal_at // ""')
	if [ -z "$a_term" ]; then info "A has no terminal_at yet: let A finish or Cancel it, then rerun"; return 1; fi
	b_bound=$(printf '%s' "$FP4_BOUND" | jq -r 'map(.created_at) | sort | first // ""')
	if [ -z "$b_bound" ]; then info "B has no co_super_assignment_bound event"; return 1; fi
	delta=$(jq -nr --arg a "$a_term" --arg b "$b_bound" "$FP4_TS_JQ ((\$b | ts) - (\$a | ts))") || { info "cannot parse A terminal_at=$a_term or B bound created_at=$b_bound"; return 1; }
	info "B bound ${delta}s after A's terminal_at (a_terminal_at=$a_term b_bound_at=$b_bound bound=${OPT_nudge_seconds}s)"
	jq -ne --argjson d "$delta" --argjson n "$OPT_nudge_seconds" '$d >= 0 and $d <= $n' >/dev/null
}

step_5() {
	fp4_target 1
	manual "let A finish, or Cancel it, then rerun this step: B binds within seconds (the nudge), not at the next minute retry"
	if [ "$PROOF_DRY" = 1 ]; then fp4_load a; fp4_load b; fp4_info_log 'engineering.*(admitted|bound)|(admitted|bound).*engineering'; return 0; fi
	FP4_A_ROW=""
	if fp4_load a; then FP4_A_ROW=$FP4_ROW; else return 0; fi
	fp4_load b || return 0
	check "B's co_super_assignment_bound event follows A's terminal_at by 0..${OPT_nudge_seconds}s" fp4_b_bound_in_time
	fp4_info_log 'engineering.*(admitted|bound)|(admitted|bound).*engineering'
}

step_6() {
	local body
	fp4_target 1
	body=$(jq -nc --arg u "$T_USER" --arg d "$T_DESKTOP" '{user_id:$u,desktop_id:$d}')
	will "operator refresh (a CRASH boot: closes open work) of the THROWAWAY computer $T_COMPUTER ($T_VM): POST /internal/vmctl/refresh $body; B must already be queued behind a live A"
	vmctl_post /internal/vmctl/refresh "$body" proof.fp4-crash
}

fp4_b_cancelled_by_restart() {
	[ "$(fp4_rowf '.disposition')" = cancelled ] && [ "$(fp4_rowf '.disposition_reason // ""')" = interrupted_by_restart ] && fp4_unbound_events
}

step_7() {
	fp4_target 1
	proof_poll 300 30 "ownership is active after the crash boot" fp4_is_active
	if [ "$PROOF_DRY" = 1 ]; then fp4_load a; fp4_load b; fp4_info_log 'interrupted_by_restart|interrupted by a restart'; return 0; fi
	if fp4_load_a_sane && fp4_load b; then
		check "B's row is cancelled with reason interrupted_by_restart and has no co_super_assignment_bound event" fp4_b_cancelled_by_restart
	fi
	fp4_info_log 'interrupted_by_restart|interrupted by a restart'
}

# Sourced by the fixture test (fp4-selfdev-admission-test.sh): define, don't run.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then proof_main "$@"; fi
