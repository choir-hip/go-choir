#!/usr/bin/env bash
# FP1 follow-up F-c, deploy 1 (inert) deployed proof: run telemetry in its own
# table, docs/design/choir-event-table-f-c-2026-10-22.md section 4.1 and 6.3.
#
# Deploy 1 adds choir_events, the projector route, the union readers, the
# witness rule and manifest class, and store schema epoch 2. No writer stamps
# an op, so the proof is a no-effect proof: the table stays empty and is not
# observed, the readers return what they did, the boot adds one DDL, and the
# replay report shows manifest version 4 with choir_events neither unknown nor
# missing. Deploy 1 ships alone (owner Q5) and stays at least one staging day
# and one paper boot before deploy 2 exists anywhere.
#
# Everything but steps 5 and 6 is read-only. The computer defaults to the paper (the
# read-only steps may read it); step 5 refuses the paper and the owner's
# computer and needs a test computer.
#
#   scripts/proofs/fp1-event-table-deploy1.sh [--computer computer-<id>] [--run RUN_ID]
#       [--label before|after] [--out FILE] [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
#
# Order: step 2 --label before --run <a finished run> --out before.json
# BEFORE the deploy; deploy 1 as its own deploy; then steps 0, 1, 2 --label
# after (same run), 3, 4; step 5 on a test computer. Step 6 is the rollback
# rehearsal on a test computer (round 1b review): run it with --phase old
# while the computer runs the previous build over its deploy-1 store, and
# with --phase recovered after the recovery.
set -uo pipefail
PROOF_NAME=fp1-event-table-deploy1
. "$(dirname "$0")/lib.sh"

step_def 0 ro "boot log: every store phase closed, runtime-schema/apply-ddl closed, runtime-store-open under two minutes"
step_def 1 ro "replay completeness: manifest version 4; choir_events neither unknown nor missing; eligibility as before"
step_def 2 ro "event reads unchanged: one finished run's events (count and last seq) before and after"
step_def 3 ro "the union reads are live: store.event_table.* engine-mutex ops appear with short holds"
step_def 4 ro "the computer is active; schedules keep polling (manual look)"
step_def 5 mut "restart a TEST computer: it boots, its replay report stays eligible and equivalent, the table stays unobserved"
step_def 6 mut "rollback rehearsal on a TEST computer: the previous build over a deploy-1 store runs but cannot publish; what restores publishing"

: "${OPT_computer:=$PROOF_PAPER}" "${OPT_label:=after}" "${OPT_out:=}" "${OPT_run:=}" "${OPT_phase:=old}"

ft_target() { resolve_target "$OPT_computer" "${1:-0}"; }
ft_observability() { guest_get /api/runtime/observability; }
ft_log_lines() { ft_observability | jq -r '.recent_log[]?' | grep -E "$1" || true; }

step_0() {
	local lines bad
	ft_target
	lines=$(ft_log_lines 'phase=')
	printf '%s\n' "$lines" | sed '/^$/d' | grep -E 'runtime-schema|store-schema-epoch|runtime-store-open' | while IFS= read -r l; do info "log $l"; done
	[ "$PROOF_DRY" = 1 ] && return 0
	bad=$(printf '%s\n' "$lines" | awk '
		/phase=/ { match($0, /phase=[^ ]+/); p = substr($0, RSTART + 6, RLENGTH - 6)
			if ($0 ~ /status=start/) open[p] = 1
			if ($0 ~ /status=complete/) delete open[p] }
		END { for (p in open) print p }')
	if [ -z "$bad" ]; then pass "every logged phase that started has completed"; else fail "phases started and not completed: $(printf '%s' "$bad" | tr '\n' ' ')"; fi
	printf '%s\n' "$lines" | grep -q 'phase=runtime-schema/apply-ddl status=complete' && pass "runtime-schema/apply-ddl completed (the DDL that creates choir_events)" || fail "no completed runtime-schema/apply-ddl line in the recent log"
	printf '%s\n' "$lines" | grep -q 'phase=store-schema-epoch status=complete' && pass "store-schema-epoch completed (the receipt now says 2)" || fail "no completed store-schema-epoch line"
	manual "compare runtime-store-open's duration with the last boot before the deploy: the new DDL creates an empty table and must not move it measurably"
}

step_1() {
	local report
	ft_target
	info "GET /api/computers/$T_COMPUTER/self-development/replay-completeness on the guest"
	report=$(guest_get "/api/computers/$T_COMPUTER/self-development/replay-completeness")
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$report" | jq -c '{manifest_version: .eligibility.manifest_version, eligible: .eligibility.eligible, reason: .eligibility.reason, unknown: .eligibility.unknown_tables, missing: .eligibility.missing_tables, drift: .eligibility.schema_drift, status: .result.status}' | while IFS= read -r l; do info "replay $l"; done
	printf '%s' "$report" | jq -e '.eligibility.manifest_version == 4' >/dev/null && pass "manifest version 4" || fail "manifest version is not 4: the deployed build is not deploy 1"
	printf '%s' "$report" | jq -e '[(.eligibility.unknown_tables // [])[], (.eligibility.missing_tables // [])[], (.eligibility.schema_drift // [])[]] | index("choir_events") | not' >/dev/null && pass "choir_events is not unknown, missing or drifting" || fail "choir_events appears in unknown, missing or schema drift"
	printf '%s' "$report" | jq -e '[.. | strings | select(test("choir_events"))] | length == 0' >/dev/null && pass "no observation names choir_events: the empty table is not observed (pre-deploy-1 checkpoints stay restorable)" || fail "the report names choir_events: the table has a row or the omission rule did not apply"
	manual "eligible must equal its value before the deploy (record both); deploy 1 changes no eligibility by itself"
}

step_2() {
	local events out
	ft_target
	[ -n "$OPT_run" ] || die 2 "step 2 needs --run RUN_ID (a run that finished before the deploy)"
	info "GET /api/runs/$OPT_run/events?limit=1000 on the guest (label=$OPT_label)"
	events=$(guest_get "/api/runs/$OPT_run/events?limit=1000")
	[ "$PROOF_DRY" = 1 ] && return 0
	out=$(printf '%s' "$events" | jq -c --arg label "$OPT_label" --arg run "$OPT_run" '(.events // .) as $e | {label: $label, run: $run, count: ($e | length), first_seq: ($e[0].seq // null), last_seq: ($e[-1].seq // null), ids_sha: ($e | map(.event_id) | join(",") | @base64)}' 2>/dev/null)
	if [ -z "$out" ]; then fail "could not read the run's events"; return 1; fi
	info "events $out"
	[ -z "$OPT_out" ] || printf '%s\n' "$out" >"$OPT_out"
	manual "with --label after: count, first_seq, last_seq and ids_sha equal the --label before reading (the union read returns what og_objects alone returned)"
}

step_3() {
	local ops
	ft_target
	ops=$(ft_observability | jq -c '[.engine_mutex // {} | to_entries[] | select(.key | test("store.event_table")) | {op: .key, calls: .value.calls, max_hold_ms: (.value.max_hold_ns / 1e6 | floor)}]')
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$ops" | jq -r '.[] | "  op=\(.op) calls=\(.calls) max_hold_ms=\(.max_hold_ms)"' | while IFS= read -r l; do info "$l"; done
	printf '%s' "$ops" | jq -e 'length > 0' >/dev/null && pass "store.event_table.* ops are live" || fail "no store.event_table.* op in the engine-mutex counters (no event read or append since boot, or not deploy 1)"
	printf '%s' "$ops" | jq -e 'all(.max_hold_ms < 1000)' >/dev/null && pass "every choir_events read held the engine under a second" || fail "a choir_events read held the engine a second or more"
}

step_4() {
	ft_target
	[ "$PROOF_DRY" = 1 ] || { [ "$T_STATE" = active ] && pass "$T_COMPUTER is active" || fail "$T_COMPUTER state is $T_STATE"; }
	ft_log_lines 'poll|schedule' | tail -n 12 | while IFS= read -r l; do info "log $l"; done
	manual "the paper's liveness probe still reads ok and its source schedules stay active; /api/costs and the WebSocket catch-up behave as before"
}

step_5() {
	local body
	ft_target 1
	body=$(jq -nc --arg u "$T_USER" --arg d "$T_DESKTOP" '{user_id:$u,desktop_id:$d}')
	will "operator refresh (a crash boot, closes open work) of the TEST computer $T_COMPUTER ($T_VM) through POST /internal/vmctl/refresh $body, then read its replay report"
	vmctl_post /internal/vmctl/refresh "$body" proof.fp1-event-table-restart
	proof_poll 600 30 "$T_COMPUTER is active after the restart" ft_is_active
	step_1
	manual "on the test computer, restore a checkpoint taken BEFORE deploy 1 through the owner restore route: it must be accepted (the empty table is not observed, design 3.3 F4)"
}
# step_6: what a rollback below deploy 1 does, rehearsed (design status
# section). The epoch fences only the updater's Apply of an epoch-declaring
# release; a CI revert (deploy-refresh), a platform-follow offer that
# declares no epoch and the
# updater's restorePrior reach the older binary unfenced. Over a deploy-1
# store that binary runs (Open rewrites the receipt to 1) but its manifest
# calls the empty choir_events unknown, so the computer is replay-ineligible
# and checkpoint publication (self-development apply, platform-follow's
# post-apply checkpoint, owner recovery checkpoints) is refused. Nothing is
# corrupted. Publication comes back by any of: a forward deploy to deploy 1
# or later (the empty table is again not observed); the rollback build (the
# script extracted from the deploy-1 tip with git show, since the revert
# deletes it, then the revert, then the script's apply --from <tip>, whose
# boot step drops an empty choir_events; fc-deploy1-rollback.sh rehearse
# runs that sequence in a throwaway clone, and TestEventTableRollback* keep
# the step tested); the restore of a pre-deploy-1 checkpoint, whose
# workspace swap removes the table.
step_6() {
	local report
	ft_target 1
	case "$OPT_phase" in
	old)
		manual "put the previous build (pre-deploy-1, epoch 1) on the TEST computer $T_COMPUTER, which has booted deploy 1 at least once. No vmctl route does this per computer (deploy-refresh's target is a label); use the path FP0 has, and record which: the updater's Apply of a layered epoch-1 release is the fenced path and must be REFUSED with 'newer than release' (record that refusal as evidence too); the unfenced paths are restorePrior after a failed health probe, a platform-follow offer, or the platform image itself"
		will "read the TEST computer's health and replay report under the previous build"
		[ "$PROOF_DRY" = 1 ] && return 0
		[ "$T_STATE" = active ] && pass "$T_COMPUTER is active under the previous build (runtime-safe)" || fail "$T_COMPUTER state is $T_STATE under the previous build"
		report=$(guest_get "/api/computers/$T_COMPUTER/self-development/replay-completeness")
		printf '%s' "$report" | jq -c '{manifest_version: .eligibility.manifest_version, eligible: .eligibility.eligible, reason: .eligibility.reason, unknown: .eligibility.unknown_tables}' | while IFS= read -r l; do info "replay $l"; done
		printf '%s' "$report" | jq -e '.eligibility.manifest_version == 3' >/dev/null && pass "manifest version 3: the previous build is serving" || fail "manifest version is not 3: the previous build is not serving"
		printf '%s' "$report" | jq -e '(.eligibility.eligible | not) and ((.eligibility.unknown_tables // []) | index("choir_events"))' >/dev/null && pass "ineligible with choir_events unknown: checkpoint publication is refused (fail-closed, as the design says)" || fail "the previous build did not report choir_events unknown and ineligible: the rehearsal disagrees with the design"
		manual "attempt one checkpoint publication on the TEST computer (an owner recovery checkpoint): it must be refused as replay-ineligible; nothing else changes"
		;;
	recovered)
		manual "recover publication on the TEST computer by ONE of: a forward deploy to deploy 1 or later; the rollback build (T=$(mktemp -d); git show TIP:scripts/rollback/fc-deploy1-rollback.sh >$T/r.sh, because the revert deletes the script; git revert deploy 1's commits; bash $T/r.sh apply --from TIP, which installs the tested boot step that drops an empty choir_events; rehearse the sequence first with bash $T/r.sh rehearse TIP BASE); the restore of a pre-deploy-1 checkpoint. Record which. For the rollback build, the boot log must show phase=runtime-schema/drop-empty-choir-events status=complete"
		will "read the TEST computer's replay report after the recovery"
		[ "$PROOF_DRY" = 1 ] && return 0
		report=$(guest_get "/api/computers/$T_COMPUTER/self-development/replay-completeness")
		printf '%s' "$report" | jq -c '{manifest_version: .eligibility.manifest_version, eligible: .eligibility.eligible, reason: .eligibility.reason, unknown: .eligibility.unknown_tables}' | while IFS= read -r l; do info "replay $l"; done
		printf '%s' "$report" | jq -e '(.eligibility.unknown_tables // []) | index("choir_events") | not' >/dev/null && pass "choir_events is no longer unknown" || fail "choir_events is still unknown after the recovery"
		printf '%s' "$report" | jq -e '.eligibility.eligible' >/dev/null && pass "replay-eligible again: checkpoint publication can resume" || fail "still replay-ineligible after the recovery"
		;;
	*) die 2 "--phase is old or recovered" ;;
	esac
}

ft_is_active() { [ "$(proof_row_json "$T_COMPUTER" | jq -r '.state // ""')" = active ]; }

proof_main "$@"
