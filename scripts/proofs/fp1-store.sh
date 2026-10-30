#!/usr/bin/env bash
# FP1 (batch B2, the store) deployed proof: the three hot reads on the
# rollback-safe side table og_key_index.
# Steps are "Deployed proof" in docs/archive/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md
# on lane/b2-store-b1fix, plus the A1 pre-check in
# docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md.
#
# Everything but step 6 is read-only. The computer defaults to the paper
# (read-only steps may read it); step 6 (a restart) refuses the paper and the
# owner's computer and needs a test computer.
#
#   scripts/proofs/fp1-store.sh [--computer computer-<id>] [--label before|after] [--window 120]
#       [--out FILE] [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
#
# Order: step 0 and step 1 --label before BEFORE the deploy; deploy FP1 as its
# own deploy; then steps 2, 3, 4 --label after, 5, 7; step 6 on a test computer.
set -uo pipefail
PROOF_NAME=fp1-store
. "$(dirname "$0")/lib.sh"

step_def 0 ro "A1 pre-check: the boot log shows no ALTER of og_objects"
step_def 1 ro "engine-mutex reading over a window (use --label before ahead of the deploy)"
step_def 2 ro "boot log: every store/startup phase closed; runtime-store-open under two minutes"
step_def 3 ro "derived-key validation: three families validated, no withdrawal, short holds"
step_def 4 ro "engine-mutex reading after: the three scans are keyindex reads and the busy share fell"
step_def 5 ro "the computer is active; schedules keep polling (manual look)"
step_def 6 mut "restart a TEST computer: the second validation pass finishes in seconds"
step_def 7 ro "replay completeness lists no unknown table"

: "${OPT_computer:=$PROOF_PAPER}" "${OPT_window:=120}" "${OPT_label:=after}" "${OPT_out:=}"

fp1_target() { resolve_target "$OPT_computer" "${1:-0}"; }

# fp1_observability: one GET of the guest's /api/runtime/observability.
fp1_observability() { guest_get /api/runtime/observability; }

# fp1_log_lines REGEX: the boot log's recent lines matching REGEX.
fp1_log_lines() { fp1_observability | jq -r '.recent_log[]?' | grep -E "$1" || true; }

# fp1_duration_seconds "1m30.5s": Go duration to seconds (awk).
fp1_duration_awk='
function secs(d,   h, m, s, ms) {
	h = 0; m = 0; s = 0; ms = 0
	if (match(d, /[0-9.]+h/)) h = substr(d, RSTART, RLENGTH - 1) + 0
	if (match(d, /[0-9.]+m[^s]|[0-9.]+m$/)) m = substr(d, RSTART, RLENGTH - 1) + 0
	if (match(d, /[0-9.]+ms/)) ms = substr(d, RSTART, RLENGTH - 2) + 0
	else if (match(d, /[0-9.]+s/)) s = substr(d, RSTART, RLENGTH - 1) + 0
	return h * 3600 + m * 60 + s + ms / 1000
}'

step_0() {
	local lines
	fp1_target
	lines=$(fp1_log_lines 'added [0-9]+ generated columns|add-generated-columns|record indexes ensured in')
	info "boot-log lines about generated columns and record indexes:"
	printf '%s\n' "$lines" | sed '/^$/d' | while IFS= read -r l; do info "log $l"; done
	if [ "$PROOF_DRY" != 1 ]; then
		if printf '%s\n' "$lines" | grep -Eq 'added [0-9]+ generated columns'; then
			fail "the boot added generated columns to og_objects (the A1 rewrite): the next boot of this store rewrites it"
		else
			pass "no 'added N generated columns' line in the recent boot log"
		fi
	fi
	manual "if the log is silent, run the A1 query inside the guest's store: SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'og_objects' AND COLUMN_NAME IN ('og_meta_run_id','og_meta_trajectory_id','og_meta_channel_id','og_rec_inbox','og_rec_sender_inbox','og_rec_conversation_id','og_rec_inbox_seq','og_rec_claim'); fewer than eight rows on a store with rows means the next boot rewrites og_objects"
}

step_1() {
	local raw out busy
	fp1_target
	info "sampling the engine-mutex counters twice, ${OPT_window}s apart, in one Node B session (label=$OPT_label)"
	raw=$(node_b_q "H1='X-Internal-Caller: true'
t1=\$(date +%s.%N); s1=\$(curl -fsS --max-time 60 -H \"\$H1\" -H 'X-Authenticated-User: $T_USER' -H 'X-Authenticated-Computer: $T_COMPUTER' '$T_URL/api/runtime/observability')
sleep $OPT_window
t2=\$(date +%s.%N); s2=\$(curl -fsS --max-time 60 -H \"\$H1\" -H 'X-Authenticated-User: $T_USER' -H 'X-Authenticated-Computer: $T_COMPUTER' '$T_URL/api/runtime/observability')
printf '{\"t1\":%s,\"s1\":%s,\"t2\":%s,\"s2\":%s}\\n' \"\$t1\" \"\$s1\" \"\$t2\" \"\$s2\"")
	[ "$PROOF_DRY" = 1 ] && return 0
	out=$(printf '%s' "$raw" | jq -c --arg lbl "$OPT_label" '
		(.s1.engine_mutex // {}) as $x | (.s2.engine_mutex // {}) as $y |
		($y | to_entries | map({op: .key,
			calls: (.value.calls - ($x[.key].calls // 0)),
			hold_ms: ((.value.hold_ns - ($x[.key].hold_ns // 0)) / 1e6),
			wait_ms: ((.value.wait_ns - ($x[.key].wait_ns // 0)) / 1e6),
			max_hold_ms: (.value.max_hold_ns / 1e6)})
			| map(select(.calls > 0))) as $ops |
		{"label": $lbl, window_s: ((.t2 | tonumber) - (.t1 | tonumber)), ops: ($ops | sort_by(-.hold_ms)),
		 total_hold_ms: ($ops | map(.hold_ms) | add // 0)}' 2>/dev/null)
	if [ -z "$out" ]; then
		fail "could not read two engine_mutex samples from $T_URL (is the guest answering /api/runtime/observability?)"
		return 1
	fi
	busy=$(printf '%s' "$out" | jq -r '(.total_hold_ms / 1000) / .window_s')
	info "window $(printf '%s' "$out" | jq -r '.window_s | floor')s: engine mutex held $(printf '%s' "$busy" | awk '{printf "%.0f%%", $1*100}') of the window (baseline before the deploy: 70% on the paper at 17:58 UTC)"
	printf '%s' "$out" | jq -r '.ops[0:10][] | "  op=\(.op) calls=\(.calls) held_ms=\(.hold_ms|floor) waited_ms=\(.wait_ms|floor) per_call_ms=\((.hold_ms/.calls*100|floor)/100)"' | while IFS= read -r l; do info "$l"; done
	[ -z "$OPT_out" ] || printf '%s\n' "$out" >"$OPT_out"
	FP1_LAST_READING=$out
	FP1_LAST_BUSY=$busy
}

step_2() {
	local lines bad
	fp1_target
	lines=$(fp1_log_lines 'phase=')
	printf '%s\n' "$lines" | sed '/^$/d' | while IFS= read -r l; do info "log $l"; done
	[ "$PROOF_DRY" = 1 ] && return 0
	# A phase that logged status=start and never status=complete is open.
	bad=$(printf '%s\n' "$lines" | awk '
		/phase=/ { match($0, /phase=[^ ]+/); p = substr($0, RSTART + 6, RLENGTH - 6)
			if ($0 ~ /status=start/) open[p] = 1
			if ($0 ~ /status=complete/) delete open[p] }
		END { for (p in open) print p }')
	if [ -z "$bad" ]; then pass "every logged phase that started has completed"; else fail "phases started and not completed: $(printf '%s' "$bad" | tr '\n' ' ')"; fi
	while IFS= read -r l; do
		case "$l" in OK) pass "runtime-store-open under two minutes" ;; SLOW) fail "runtime-store-open took two minutes or more" ;; *) info "$l" ;; esac
	done < <(printf '%s\n' "$lines" | awk "$fp1_duration_awk"'
		/runtime-store-open/ && /status=complete/ && match($0, /duration=[^ ]+/) { d = substr($0, RSTART + 9, RLENGTH - 9); t = secs(d); printf "runtime-store-open duration=%s (%.1fs)\n", d, t; if (t < 120) print "OK"; else print "SLOW" }')
}

step_3() {
	local lines validated withdrawn maxhold
	fp1_target
	lines=$(fp1_log_lines 'derived keys ')
	printf '%s\n' "$lines" | sed '/^$/d' | while IFS= read -r l; do info "log $l"; done
	[ "$PROOF_DRY" = 1 ] && return 0
	validated=$(printf '%s\n' "$lines" | grep -Ec 'validated [0-9]+ rows in' || true)
	withdrawn=$(printf '%s\n' "$lines" | grep -c 'withdrawn' || true)
	[ "$validated" -ge 3 ] && pass "$validated 'validated N rows in T' lines (three families: run.agent_id, commitment addressee, source_ref doc_id)" || fail "only $validated validated line(s); expected one per family (3)"
	[ "$withdrawn" -eq 0 ] && pass "no 'withdrawn' line" || fail "$withdrawn 'withdrawn' line(s)"
	maxhold=$(fp1_observability | jq -r '[.engine_mutex // {} | to_entries[] | select(.key | test("ValidateDerivedKeys")) | .value.max_hold_ns] | max // 0')
	info "max engine hold of the validator's page op: $(printf '%s' "$maxhold" | awk '{printf "%.0f ms", $1/1e6}')"
	if [ "$(printf '%s' "$maxhold" | awk '{print ($1 < 1e9) ? 1 : 0}')" = 1 ]; then pass "validator page max hold is under a second"; else fail "validator page max hold is a second or more"; fi
}

step_4() {
	local keyindex json
	step_1
	[ "$PROOF_DRY" = 1 ] && return 0
	[ -n "${FP1_LAST_READING:-}" ] || return 1
	info "the three scans' rows (agent_id, addressee, doc_id), by backend:"
	printf '%s' "$FP1_LAST_READING" | jq -r '.ops[] | select(.op | test("agent_id|addressee|doc_id")) | "  op=\(.op) calls=\(.calls) per_call_ms=\((.hold_ms/.calls*100|floor)/100) max_hold_ms=\(.max_hold_ms|floor)"' | while IFS= read -r l; do info "$l"; done
	keyindex=$(printf '%s' "$FP1_LAST_READING" | jq '[.ops[] | select((.op | test("agent_id|addressee|doc_id")) and (.op | test("keyindex")))] | length')
	json=$(printf '%s' "$FP1_LAST_READING" | jq '[.ops[] | select((.op | test("agent_id|addressee|doc_id")) and (.op | test("json$")))] | length')
	[ "$keyindex" -ge 1 ] && pass "keyindex rows present for the scans ($keyindex)" || fail "no keyindex row among the three scans in this window"
	[ "$json" -eq 0 ] && pass "no json-path call among the three scans" || fail "$json json-path row(s) among the scans (expected only lookups of 255+ characters)"
	printf '%s' "$FP1_LAST_READING" | jq -r '.ops[] | select(.op | test("^\\(?\\*?DoltStore\\)?\\.?GetObject|^GetObject|ListObjectsPage")) | .op' | head -4 | while IFS= read -r l; do info "kind-labelled op: $l"; done
	[ "$(printf '%s' "$FP1_LAST_BUSY" | awk '{print ($1 < 0.70) ? 1 : 0}')" = 1 ] && pass "busy share is below the 70% baseline" || fail "busy share is not below the 70% baseline"
}

step_5() {
	fp1_target
	[ "$PROOF_DRY" = 1 ] || { [ "$T_STATE" = active ] && pass "$T_COMPUTER is active" || fail "$T_COMPUTER state is $T_STATE"; }
	info "recent source/schedule polling lines:"
	fp1_log_lines 'poll|schedule' | tail -n 12 | while IFS= read -r l; do info "log $l"; done
	manual "the paper's liveness probe still reads ok and its source schedules stay active (the paper's own Autopaper surface); this script cannot read the schedule list"
}

step_6() {
	local body
	fp1_target 1
	body=$(jq -nc --arg u "$T_USER" --arg d "$T_DESKTOP" '{user_id:$u,desktop_id:$d}')
	will "operator refresh (a crash boot, closes open work) of the TEST computer $T_COMPUTER ($T_VM) through POST /internal/vmctl/refresh $body, then read the second validation pass"
	vmctl_post /internal/vmctl/refresh "$body" proof.fp1-restart
	proof_poll 600 30 "$T_COMPUTER is active after the restart" fp1_is_active
	fp1_target 1
	fp1_log_lines 'derived keys .*(validation pass|validated)' | while IFS= read -r l; do info "log $l"; done
	manual "the second pass should read 'validated N rows in T' with T in seconds (the first pass compared and derived; the repeat only compares narrow columns)"
}
fp1_is_active() { [ "$(proof_row_json "$T_COMPUTER" | jq -r '.state // ""')" = active ]; }

step_7() {
	local report
	fp1_target
	info "GET /api/computers/$T_COMPUTER/self-development/replay-completeness on the guest"
	report=$(guest_get "/api/computers/$T_COMPUTER/self-development/replay-completeness")
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$report" | jq -c '{eligible: .eligibility.eligible, unknown_tables: .eligibility.unknown_tables, status: .result.status}' | while IFS= read -r l; do info "replay $l"; done
	printf '%s' "$report" | jq -e '(.eligibility.unknown_tables // []) | length == 0' >/dev/null && pass "no unknown table (og_key_index is known to the extractor)" || fail "replay lists unknown table(s)"
}

proof_main "$@"
