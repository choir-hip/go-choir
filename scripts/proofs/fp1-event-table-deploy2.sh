#!/usr/bin/env bash
# FP1 follow-up F-c, deploy 2 (the writer) deployed proof: run telemetry in its
# own table, docs/design/choir-event-table-f-c-2026-10-22.md sections 4.2 and
# 6.3.
#
# Deploy 2 adds the writer behind two keys, both required, both off as
# deployed:
#   - the host selector CHOIR_EVENT_TABLE_COMPUTERS (nix/node-b.nix,
#     go-choir-vmctl Environment; ComputerIDs only), which puts a grant on the
#     named computer's credential disk at its next cold boot;
#   - the owner's setting store.event_table, POST /api/computer-settings
#     {"key":"store.event_table","value":true} with the computer's own key.
# With the selector empty deploy 2 is inert: step 0 is its no-effect proof on
# the paper (and any computer). Then FP0 names ONE test computer in the
# selector (a nix commit, deployed), cold boots it, and runs steps 1 to 7 on
# it. The paper comes only after the test computer passes and the panel
# agrees, with the owner's conditions (step 8 prints them; it changes
# nothing).
#
# Before any of it, the local journal probe F17 must have passed its go bar
# (docs/evidence/fp1-event-table-journal-probe-2026-10-22.json).
#
#   BASH_ENV= CHOIR_API_KEY=<the test computer's key> \
#   scripts/proofs/fp1-event-table-deploy2.sh [--computer computer-<id>] [--run RUN_ID]
#       [--label LABEL] [--out FILE] [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
#
# Order: step 0 on the paper after the deploy. Mint the test computer
# (node scripts/qa_test_computer.mjs --label fp1-event-table-d2), name it in
# CHOIR_EVENT_TABLE_COMPUTERS, deploy, cold boot it (step 5 is a cold boot),
# then step 1; step 3 --label off-<n> snapshots around the gate's workload
# with the setting off; step 2 (setting on); step 3 --label on-<n> around the
# same workload; step 4; step 5 (crash drill); step 6 (live rollback). The fit
# over the snapshots is the gate's segment method (step 7, manual).
set -uo pipefail
PROOF_NAME=fp1-event-table-deploy2
. "$(dirname "$0")/lib.sh"

step_def 0 ro "no effect where off: no event_table block, no tape_ops_by_table, store.event_table false, no 'event table' boot line"
step_def 1 ro "the named test computer is granted by the host after its cold boot; the setting is off; the writer is off"
step_def 2 mut "the owner sets store.event_table on the TEST computer; the writer is on at once (no restart)"
step_def 3 ro "snapshot for the fit: store bytes, tape head, tape events and ops, ops by table, events by kind (--label, --out appends)"
step_def 4 ro "correctness: replay report eligible and equivalent at manifest version 4 with stamped ops; a run's events read whole"
step_def 5 mut "crash drill: operator refresh (cold boot) of the TEST computer; the setting and the grant survive; the writer is on again"
step_def 6 mut "live rollback: the owner sets store.event_table off; the writer is off at once; stamped events stay readable"
step_def 7 ro "the fit and the decision (manual): b_ce/b_og <= 0.5 from step 3's snapshots"
step_def 8 ro "the paper (manual, never run by this script): the owner's conditions and readings 1 to 6 before and after"

: "${OPT_computer:=$PROOF_PAPER}" "${OPT_label:=}" "${OPT_out:=}" "${OPT_run:=}"

ft_target() { resolve_target "$OPT_computer" "${1:-0}"; }
ft_observability() { guest_get "/api/runtime/observability${1:-}"; }
ft_event_table() { ft_observability | jq -c '.event_table // null'; }
ft_settings() { guest_get /api/computer-settings; }

# ft_set_event_table VALUE: the owner act, through the product API with the
# computer's own key (handed to curl through a descriptor, never printed).
ft_set_event_table() {
	local body
	body=$(jq -nc --argjson v "$1" '{key:"store.event_table",value:$v}')
	will "POST $PROOF_API_HOST$(api_with_computer /api/computer-settings "$T_COMPUTER") $body (the owner's setting on $T_COMPUTER)"
	[ "$PROOF_DRY" = 1 ] && return 0
	[ -n "${CHOIR_API_KEY:-}" ] || die 2 "CHOIR_API_KEY (the test computer's key) is not set in the environment"
	"$PROOF_CURL_BIN" -fsS --max-time 60 -X POST -H 'Content-Type: application/json' \
		-H @<(printf 'Authorization: Bearer %s\n' "$CHOIR_API_KEY") -d "$body" \
		"$PROOF_API_HOST$(api_with_computer /api/computer-settings "$T_COMPUTER")"
}

step_0() {
	local obs settings
	ft_target
	obs=$(ft_observability)
	settings=$(ft_settings)
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$obs" | jq -e '.event_table == null' >/dev/null && pass "no event_table block: $T_COMPUTER is neither granted nor set" || fail "event_table present: $(printf '%s' "$obs" | jq -c .event_table)"
	printf '%s' "$obs" | jq -e '(.telemetry_batching.tape_ops_by_table == null) and (.telemetry_batching.event_table_writes == null)' >/dev/null && pass "telemetry_batching has no tape_ops_by_table and no event_table_writes (byte-identical to deploy 1)" || fail "telemetry_batching shows the writer on $T_COMPUTER"
	printf '%s' "$settings" | jq -e '.settings | map(select(.key == "store.event_table")) | length == 1 and (.[0].value == false)' >/dev/null && pass "store.event_table is listed, false" || fail "store.event_table is not listed false: $(printf '%s' "$settings" | jq -c '.settings')"
	printf '%s' "$obs" | jq -r '.recent_log[]?' | grep -q 'runtime: event table' && fail "the boot log carries an event table line on an unnamed computer" || pass "no event table line in the boot log"
	manual "repeat on the paper and on one primary computer; then deploy 1's step 4 (the paper active and polling)"
}

step_1() {
	local st
	ft_target
	st=$(ft_event_table)
	[ "$PROOF_DRY" = 1 ] && return 0
	info "event_table $st"
	printf '%s' "$st" | jq -e '.host_granted == true' >/dev/null && pass "the host grant reached $T_COMPUTER (CHOIR_EVENT_TABLE_COMPUTERS names it; cold boot done)" || fail "not granted: name the computer in CHOIR_EVENT_TABLE_COMPUTERS, deploy, and cold boot it (a snapshot resume keeps the old credential disk)"
	printf '%s' "$st" | jq -e '.setting == false and .writing == false' >/dev/null && pass "the setting and the writer are off: the grant alone turns nothing on" || fail "the writer or setting is on before the owner's act"
}

step_2() {
	local st head
	ft_target 1
	head=$(guest_get /health | jq -r '.effective_release.event_head.sequence // empty')
	info "tape head before the setting write: ${head:-?}"
	ft_set_event_table true || { fail "the setting write was refused"; return 1; }
	st=$(ft_event_table)
	[ "$PROOF_DRY" = 1 ] && return 0
	info "event_table $st"
	printf '%s' "$st" | jq -e '.host_granted and .setting and .writing' >/dev/null && pass "the writer is on at once (no restart)" || fail "the writer is not on after the owner's act: $st"
	manual "record the tape head after the write ($(guest_get /health | jq -r '.effective_release.event_head.sequence // "?"')): stamped ops start after it"
}

step_3() {
	local obs head line
	ft_target
	[ -n "$OPT_label" ] || die 2 "step 3 needs --label (off-1, on-1, ...)"
	obs=$(ft_observability '?memory=detail')
	head=$(guest_get /health | jq -r '.effective_release.event_head.sequence // empty')
	[ "$PROOF_DRY" = 1 ] && return 0
	line=$(printf '%s' "$obs" | jq -c --arg label "$OPT_label" --arg at "$(proof_now)" --arg head "$head" '{
		label: $label, at: $at, computer: .computer_id, build: .build,
		event_head: ($head | tonumber? // null),
		store_bytes: ([.memory_detail | .. | objects | select(.label? == "store") | .bytes][0] // null),
		event_table: (.event_table // null),
		tape_events: .telemetry_batching.tape_events, tape_ops: .telemetry_batching.tape_ops,
		tape_ops_by_table: (.telemetry_batching.tape_ops_by_table // {}),
		tape_ops_by_kind: .telemetry_batching.tape_ops_by_kind,
		events_appended_by_kind: .telemetry_batching.events_appended_by_kind,
		batching: .telemetry_batching.batching.enabled}')
	info "snapshot $line"
	[ -z "$OPT_out" ] || printf '%s\n' "$line" >>"$OPT_out"
	printf '%s' "$line" | jq -e '.store_bytes != null and .event_head != null' >/dev/null && pass "snapshot has store bytes and the tape head" || fail "snapshot is missing store bytes or the tape head"
}

step_4() {
	local report events
	ft_target
	report=$(guest_get "/api/computers/$T_COMPUTER/self-development/replay-completeness")
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$report" | jq -c '{manifest_version: .eligibility.manifest_version, eligible: .eligibility.eligible, status: .result.status, unknown: .eligibility.unknown_tables, missing: .eligibility.missing_tables}' | while IFS= read -r l; do info "replay $l"; done
	printf '%s' "$report" | jq -e '.eligibility.manifest_version == 4 and .eligibility.eligible == true' >/dev/null && pass "eligible at manifest version 4" || fail "not eligible, or not manifest version 4"
	printf '%s' "$report" | jq -e '.result.status == "equivalent"' >/dev/null && pass "replay is equivalent with stamped ops in the tail" || fail "replay is not equivalent: read the choir_events fingerprints in the report"
	printf '%s' "$report" | jq -e '[.. | strings | select(test("choir_events"))] | length > 0' >/dev/null && pass "choir_events is observed (it has rows)" || fail "choir_events is not observed: no op was stamped"
	if [ -n "$OPT_run" ]; then
		events=$(guest_get "/api/runs/$OPT_run/events?limit=1000")
		printf '%s' "$events" | jq -c '(.events // .) as $e | {count: ($e | length), first_seq: ($e[0].seq // null), last_seq: ($e[-1].seq // null), seqs_unique: (($e | map(.seq) | unique | length) == ($e | length))}' | while IFS= read -r l; do info "run $OPT_run $l"; done
		manual "for a run that spans the setting write: every event the recorded WebSocket published is in this list; no seq repeats"
	else
		manual "pass --run RUN_ID (a run that spans the setting write) to read its events"
	fi
	manual "POST /api/run-acceptances/synthesize on a trajectory with stamped tool.result events cites them; /api/costs and the texture diagnosis return events from both tables; the whole-chain audit finds no duplicate seq or stream_seq"
}

step_5() {
	local body
	ft_target 1
	body=$(jq -nc --arg u "$T_USER" --arg d "$T_DESKTOP" '{user_id:$u,desktop_id:$d}')
	will "operator refresh (a crash boot, closes open work) of the TEST computer $T_COMPUTER ($T_VM) through POST /internal/vmctl/refresh $body"
	vmctl_post /internal/vmctl/refresh "$body" proof.fp1-event-table-d2-restart
	proof_poll 600 30 "$T_COMPUTER is active after the restart" ft_is_active
	[ "$PROOF_DRY" = 1 ] && return 0
	ft_event_table | jq -e '.host_granted and .setting and .writing' >/dev/null && pass "after the crash boot the grant and the setting are read again; the writer is on" || fail "after the crash boot the writer is not on: $(ft_event_table)"
	manual "runs open before the refresh are closed 'interrupted by a restart'; the whole-chain audit finds no duplicate seq or stream_seq across the boot"
}

step_6() {
	local before after st
	ft_target 1
	before=$(ft_observability | jq -c '.telemetry_batching.tape_ops_by_table // {}')
	ft_set_event_table false || { fail "the setting write was refused"; return 1; }
	st=$(ft_event_table)
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$st" | jq -e '.setting == false and .writing == false' >/dev/null && pass "the writer is off at once" || fail "the writer is still on: $st"
	info "tape_ops_by_table at the off write: $before"
	manual "run a turn; tape_ops_by_table.choir_events stays at $before while tape_ops grows (new events go to og_objects); step 4 with --run on a run that spans the on and off writes still reads every event"
}

step_7() {
	manual "fit delta_store_bytes = a*tape_events + b_og*og_ops + b_ce*ce_ops over every segment of both arms (ce_ops = tape_ops_by_table.choir_events deltas, og_ops = tape_ops deltas minus ce_ops). Pass: b_ce/b_og <= 0.5. Report bytes per telemetry record for both arms and the share of stamped rows with an empty trajectory_id (from the tape audit)"
	manual "composition: telemetry.batch on with store.event_table on, the same fit (k about 5)"
	manual "write docs/evidence/fp1-event-table-deploy2-staging-<date>.json in the shape of telemetry-batching-phase1-staging-2026-10-22.json"
}

step_8() {
	manual "the paper only after steps 1 to 7 pass on the test computer and the panel agrees (owner Q7)"
	manual "conditions: telemetry.batch stays OFF on the paper until the planned-restart drain fix lands; any binary rollback goes no lower than deploy 1 (the paper is always busy, so the floor is absolute)"
	manual "baseline: the journal record's readings 1 to 6 BEFORE anything changes: two samples 600 s apart inside one GC interval; reading 4 confirms telemetry.batch is off"
	manual "then a nix commit adds the paper ($PROOF_PAPER) to CHOIR_EVENT_TABLE_COMPUTERS; the paper takes the grant at its next cold boot (a planned, marker-backed refresh); then the owner sets store.event_table on the paper with the paper owner's key"
	manual "after: the same readings 1 to 6. Keep it if journal bytes per tape event (reading 3) fall at least 20% allowing for the kind mix (reading 4); otherwise the owner's setting off (the live rollback), never a binary rollback below deploy 1, and the next design is 'no telemetry in Dolt' (design 6.2)"
}

ft_is_active() { [ "$(proof_row_json "$T_COMPUTER" | jq -r '.state // ""')" = active ]; }

proof_main "$@"
