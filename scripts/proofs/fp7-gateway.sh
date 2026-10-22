#!/usr/bin/env bash
# FP7 (batch B7, gateway identity and fairness) deployed proofs.
# Steps are "Deployed proofs for FP0" in
# docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md on
# lane/b7-gateway-b1fix, numbered the same. The gateway's /health (port 8084 on
# Node B, read through ssh node-b) carries bearer_binding, credentials,
# anthropic_relay and model_scheduler; nothing read here is a secret.
#
# Every step is read-only. The acts that need a shell inside a guest, a model
# turn, a hibernate or a planned refresh are MANUAL lines with the exact
# instruction; the step then checks the counters they should have moved. Test
# computers named by --t1/--t2 must be disposable; the paper, the owner's
# computer and the delegate are refused as targets.
#
#   scripts/proofs/fp7-gateway.sh [--t1 computer-<test1>] [--t2 computer-<test2>]
#       [--since "2026-10-22 10:00:00 UTC"] [--label before|after]
#       [--dry-run | --list | --step N] [--evidence FILE]
#
# Order: 1 after the deploy with observe; 2 and 3 after the model call and the
# hibernate/refresh acts; flip to enforce; 4 and 5; 6; 7 after one timer tick
# (10 minutes); 8; flip the scheduler on; 9 --label before, the burst, 9 --label after.
set -uo pipefail
PROOF_NAME=fp7-gateway
. "$(dirname "$0")/lib.sh"

step_def 1 ro "SEC-05 (observe): bearer_binding on the gateway /health, baseline saved"
step_def 2 ro-after "SEC-05: would_refuse has not risen after a test computer's model call"
step_def 3 ro-after "SEC-05: would_refuse still 0 after hibernate/wake and the paper's planned refresh"
step_def 4 ro-after "SEC-05 (enforce): the test computer's call still passes; no computer refused after its pin moved"
step_def 5 ro-after "SEC-05 (enforce): cross-computer token copy is refused (guest acts by hand)"
step_def 6 ro "SEC-08 gateway half: the relay is dark (anthropic_relay off)"
step_def 7 ro-after "SEC-16: the front-importance tick reaches Jev through the gateway; the unit holds no OPENROUTER key"
step_def 8 ro-after "SEC-16: the host judgment route refuses a caller without the internal credential"
step_def 9 ro-after "RES-11 (scheduler on): counters before and after 40+1 concurrent calls"

: "${OPT_label:=before}" "${OPT_since:=2 hours ago}"

fp7_state() { printf '%s/fp7-gateway-health-%s.json' "${TMPDIR:-/tmp}" "$1"; }

fp7_health() { node_b_q "curl -fsS --max-time 15 http://127.0.0.1:8084/health"; }

fp7_check_targets() {
	local t
	for t in "${OPT_t1:-}" "${OPT_t2:-}"; do
		[ -z "$t" ] || proof_guard_value "$t" 1
	done
}

# fp7_counter NAME: bearer_binding.NAME from the saved baseline / current health.
fp7_num() { printf '%s' "$1" | jq -r "$2 // 0"; }

step_1() {
	local h mode
	h=$(fp7_health)
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$h" | jq -c '{bearer_binding, credentials, anthropic_relay, model_scheduler}' | while IFS= read -r l; do info "health $l"; done
	mode=$(printf '%s' "$h" | jq -r '.bearer_binding.mode // ""')
	case "$mode" in
	observe) pass "bearer_binding.mode is observe" ;;
	enforce | strict) info "bearer_binding.mode is $mode (already past observe)" ;;
	*) fail "bearer_binding.mode is '$mode' (expected observe after the deploy)" ;;
	esac
	printf '%s' "$h" | jq -e '.bearer_binding | has("pinned") and has("unpinned") and has("would_refuse")' >/dev/null && pass "pinned, unpinned and would_refuse are reported" || fail "bearer_binding lacks pinned/unpinned/would_refuse"
	printf '%s\n' "$h" >"$(fp7_state baseline)"
	info "baseline saved to $(fp7_state baseline)"
}

fp7_would_refuse_unchanged() { # what
	local h base cur
	h=$(fp7_health)
	[ "$PROOF_DRY" = 1 ] && return 0
	[ -s "$(fp7_state baseline)" ] || die 2 "no baseline: run --step 1 first"
	base=$(fp7_num "$(cat "$(fp7_state baseline)")" '.bearer_binding.would_refuse')
	cur=$(fp7_num "$h" '.bearer_binding.would_refuse')
	info "would_refuse baseline=$base now=$cur unpinned=$(fp7_num "$h" '.bearer_binding.unpinned') pinned=$(fp7_num "$h" '.bearer_binding.pinned')"
	[ "$cur" -le "$base" ] && pass "$1: would_refuse did not rise ($base -> $cur)" || fail "$1: would_refuse rose ($base -> $cur)"
	info "gateway journal, binding lines since '$OPT_since':"
	node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "bearer binding" 20
}

step_2() {
	fp7_check_targets
	manual "start a test computer${OPT_t1:+ ($OPT_t1)} and make one model call through the product (a Texture prompt), then rerun this step"
	fp7_would_refuse_unchanged "after a test computer's model call"
}

step_3() {
	manual "hibernate and wake the test computer and repeat the model call; then a planned refresh of the paper's computer and repeat; rerun this step after each"
	fp7_would_refuse_unchanged "after hibernate/wake and the paper's refresh"
}

step_4() {
	# A refusal while a computer's pin is stale (after a refresh, before vmctl's
	# ensure moves it) is the design: 429 + Retry-After 2, healed by the ensure.
	# The failure is a computer still refused after its pin moved (FP7
	# definition, "SEC-05 enforce flip": the watch). The counters reset with the
	# gateway, so the journal since --since is the record.
	local h j
	h=$(fp7_health)
	[ "$PROOF_DRY" = 1 ] && return 0
	[ "$(printf '%s' "$h" | jq -r '.bearer_binding.mode // ""')" = enforce ] && pass "mode is enforce" || fail "mode is not enforce: flip it first (nix gatewayBearerBindingMode)"
	info "refused=$(fp7_num "$h" '.bearer_binding.refused') would_refuse=$(fp7_num "$h" '.bearer_binding.would_refuse') first_use_bound=$(fp7_num "$h" '.bearer_binding.first_use_bound') pinned=$(fp7_num "$h" '.bearer_binding.pinned')"
	manual "make one model call from the test computer now (a Texture prompt), then rerun this step: each run reads fresh counters and the journal"
	j=$(node_b_q "journalctl -u go-choir-gateway --since '${OPT_since}' --no-pager -o short-iso | grep -E 'bearer binding enforce: refused|moved to its current guest address' || true")
	printf '%s\n' "$j" | python3 -I -c '
import re, sys
last_ref, last_move = {}, {}
for line in sys.stdin:
    ts = line.split(" ", 1)[0]
    m = re.search(r"enforce: refused (\S+) from", line)
    if m: last_ref[m.group(1)] = ts
    m = re.search(r"autoputer (\S+) moved to its current guest address", line)
    if m: last_move[m.group(1)] = ts
bad = [vm for vm, t in last_ref.items() if t > last_move.get(vm, "")]
for vm, t in sorted(last_ref.items()):
    print("INFO refused %s last at %s; its pin last moved at %s" % (vm, t, last_move.get(vm, "never")))
print("BAD " + " ".join(bad) if bad else "OK")
' > "$(fp7_state step4)"
	grep '^INFO' "$(fp7_state step4)" | while read -r _ rest; do info "$rest"; done
	if grep -q '^OK' "$(fp7_state step4)"; then
		pass "no computer is refused after its pin moved to its current address"
	else
		fail "still refused after the pin moved: $(grep '^BAD' "$(fp7_state step4)" | cut -c5-) (rollback trigger)"
	fi
}

step_5() {
	fp7_check_targets
	[ -n "${OPT_t1:-}" ] && [ -n "${OPT_t2:-}" ] || info "pass --t1 and --t2 (two disposable test computers) to name them in the instructions"
	manual "from ${OPT_t1:-T1}'s guest run one gateway call with T1's own token file (expect it passes)"
	manual "copy that token file into ${OPT_t2:-T2}'s guest and call from there: expect 429 with Retry-After: 2; the provider is never called"
	manual "revoke by stopping and removing both test computers"
	info "gateway journal evidence (the refusal names the computer and both addresses):"
	node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "bearer binding enforce: refused" 10
	if [ "$PROOF_DRY" != 1 ]; then
		node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "bearer binding enforce: refused" 10 | grep -q . && pass "journal has 'bearer binding enforce: refused'" || fail "no 'bearer binding enforce: refused' line since '$OPT_since'"
	fi
}

step_6() {
	local h
	h=$(fp7_health)
	[ "$PROOF_DRY" = 1 ] && return 0
	[ "$(printf '%s' "$h" | jq -r '.anthropic_relay // ""')" = off ] && pass "anthropic_relay is off" || fail "anthropic_relay is '$(printf '%s' "$h" | jq -r '.anthropic_relay // ""')' (expected off with no key and an empty allowlist)"
	manual "from a disposable test computer's guest: POST /provider/anthropic/v1/messages with its own bearer returns 403 (not on the relay allowlist) or 503 (no key); nothing is relayed"
}

step_7() {
	local unit=go-choir-front-importance n
	info "front-importance journal since '$OPT_since' (expect 'Jev reached by gateway'):"
	node_journal "$unit" "$OPT_since" "Jev reached by gateway" 5
	info "gateway journal (expect 'host judgment from host:front-importance served'):"
	node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "host judgment from host:front-importance served" 5
	node_b_sh "f=\$(find /var/lib/go-choir -name scores.json 2>/dev/null | head -1); echo \"scores.json: \${f:-not found}\"; [ -z \"\$f\" ] || stat -c '%y' \"\$f\""
	if [ "$PROOF_DRY" != 1 ]; then
		node_journal "$unit" "$OPT_since" "Jev reached by gateway" 5 | grep -q . && pass "the tick reached Jev by gateway" || fail "no 'Jev reached by gateway' line since '$OPT_since' (wait one 10-minute tick)"
		node_journal "$PROOF_UNIT_GATEWAY" "$OPT_since" "host judgment from host:front-importance served" 5 | grep -q . && pass "the gateway served the host judgment" || fail "no served host judgment line"
		# Names only: the Environment string itself is never printed.
		n=$(node_b_q "systemctl show $unit -p Environment | grep -c OPENROUTER_API_KEY || true")
		info "OPENROUTER_API_KEY present in the unit's Environment: ${n:-0} (0 expected after the second landing; the first half still holds it)"
		node_b_q "systemctl show $unit -p EnvironmentFiles" | sed 's/^/  /' | while IFS= read -r l; do info "$l"; done
	fi
	manual "after FP0 cherry-picks 8c8bd7e54 as its own landing (only after the first half passed), rerun: the next tick still scores and the unit has no OPENROUTER_API_KEY"
}

step_8() {
	local code
	info "POST the host judgment route from Node B WITHOUT the internal credential (a refusal, not a call; empty body)"
	code=$(node_b_q "curl -s -o /dev/null -w '%{http_code}' --max-time 15 -X POST -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:8084/provider/v1/host/judgments")
	[ "$PROOF_DRY" = 1 ] && return 0
	[ "$code" = 403 ] && pass "no internal credential: 403" || fail "no internal credential: HTTP $code (expected 403)"
	manual "from a test computer's shell, POST the same route with the computer's own bearer: expect 403"
}

step_9() {
	local h st
	h=$(fp7_health)
	[ "$PROOF_DRY" = 1 ] && return 0
	printf '%s' "$h" | jq -c '.model_scheduler' | while IFS= read -r l; do info "scheduler $l"; done
	[ "$(printf '%s' "$h" | jq -r '.model_scheduler.mode // ""')" = on ] && pass "scheduler mode is on" || fail "scheduler mode is not on (GATEWAY_MODEL_SCHEDULER=on)"
	st=$(fp7_state "sched-$OPT_label")
	printf '%s\n' "$h" >"$st"
	if [ "$OPT_label" = before ]; then
		manual "from one test computer fire 40 concurrent small model calls through the product path while a second test computer makes one call; time the second call's latency; then rerun with --label after"
	else
		local b g0 g1
		[ -s "$(fp7_state sched-before)" ] || die 2 "run --label before first"
		g0=$(fp7_num "$(cat "$(fp7_state sched-before)")" '.model_scheduler.granted')
		g1=$(fp7_num "$h" '.model_scheduler.granted')
		info "granted $g0 -> $g1 (expect +41), avg_wait_ms=$(fp7_num "$h" '.model_scheduler.avg_wait_ms'), queued=$(fp7_num "$h" '.model_scheduler.queued'), in_flight=$(fp7_num "$h" '.model_scheduler.in_flight')"
		[ $((g1 - g0)) -ge 41 ] && pass "all 41 calls were granted" || fail "granted rose by $((g1 - g0)), expected at least 41"
		[ "$(fp7_num "$h" '.model_scheduler.queued')" = 0 ] && [ "$(fp7_num "$h" '.model_scheduler.in_flight')" = 0 ] && pass "queued and in_flight returned to zero" || fail "queued or in_flight not back to zero"
		b=$(fp7_num "$h" '.model_scheduler.queue_full')
		[ "$b" = 0 ] && pass "queue_full is 0" || info "queue_full=$b"
		manual "the second computer's single call stayed near one call's latency (judge from your timing)"
	fi
}

proof_main "$@"
