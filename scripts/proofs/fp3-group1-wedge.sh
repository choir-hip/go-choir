#!/usr/bin/env bash
# FP3 group 1 deployed proof: a failure-stopped always-on computer is revived
# by a corrective deploy, and the bound holds (one automatic boot per 24 h from
# the sweep, one more per distinct deploy target).
# Steps are docs/definitions/choir-launch-fp3-host-floor-2026-10-21.md,
# "Group 1: deployed proof steps (FP0)", numbered the same.
#
# Needs a staging TEST computer made critical_protected by the reviewed tier
# commit (step 0). Never the paper, the owner's computer or the delegate: all
# three are refused for every step here.
#
#   scripts/proofs/fp3-group1-wedge.sh --computer computer-<test> [--t1 SHA] [--t2 SHA]
#       [--since "2026-10-22 10:00:00 UTC"] [--rule-ttl 1500] [--wait 600] [--hold 240]
#       [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
#
# Order of a full proof (FP0 runs each, reading the output between):
#   0, 1                       read-only, before anything
#   --step 2 --mutate          wedge 1 (inserts the REJECT rule, waits for the wedge stop, removes it)
#   --step 3                   the sweep's one retry (read-only watch)
#   --step 4 --mutate          wedge 2, then 240 s confirming it stays stopped
#   push a commit; CI deploys T2
#   --step 5                   the rescue (read-only watch of the journal)
#   --step 6 --mutate          wedge 3, then POST deploy-refresh with T2: expect skipped
#   --step 7, --step 8         crash semantics and cleanup (read-only)
set -uo pipefail
PROOF_NAME=fp3-group1
. "$(dirname "$0")/lib.sh"

step_def 0 ro "tier precheck: the test computer is critical_protected"
step_def 1 ro "baseline: vmctl identity and the deploy-refresh candidate list"
step_def 2 mut "wedge 1: refuse the host's connections to the guest port until the wedge stop"
step_def 3 ro-after "the sweep's one retry"
step_def 4 mut "wedge 2: stopped again, and it stays stopped (24 h bound)"
step_def 5 ro-after "the rescue: the next deploy target re-arms one boot"
step_def 6 mut "wedge 3, then re-post the same target: skipped"
step_def 7 ro-after "crash semantics: no planned-restart marker, open work interrupted"
step_def 8 ro-after "cleanup: no REJECT rule or proof timer remains"

: "${OPT_rule_ttl:=1500}" "${OPT_wait:=600}" "${OPT_hold:=240}" "${OPT_since:=}"

fp3_target() { resolve_target "${OPT_computer:-}" 1; }

fp3_since() {
	if [ -n "$OPT_since" ]; then printf '%s' "$OPT_since"; else printf '2 hours ago'; fi
}

fp3_state_is() { # state [stopped_by]
	local row
	row=$(proof_row_json "$T_COMPUTER")
	[ -n "$row" ] || return 1
	[ "$(printf '%s' "$row" | jq -r '.state // ""')" = "$1" ] || return 1
	[ -z "${2:-}" ] || [ "$(printf '%s' "$row" | jq -r '.stopped_by // ""')" = "$2" ]
}

fp3_show_row() {
	proof_row_json "$T_COMPUTER" | jq -c '{vm_id, computer_id, state, stopped_by, warmness_class, held, epoch, computer_url, failure_warmed_at, last_active_at}' 2>/dev/null |
		while IFS= read -r l; do info "ownership $l"; done
}

# fp3_wedge LABEL: one wedge cycle. The removal is scheduled before the rule
# is inserted, and the rule is deleted right after the stop (or the timeout).
fp3_wedge() {
	local label=$1 t0
	fp3_target
	if [ "$PROOF_DRY" != 1 ]; then
		fp3_state_is active || die 2 "$T_COMPUTER is not active (state=$T_STATE); a wedge needs a live guest"
		[ "$T_CLASS" = critical_protected ] || die 2 "$T_COMPUTER is class '$T_CLASS', not critical_protected: step 0 first"
	fi
	t0=$(date -u '+%Y-%m-%d %H:%M:%S UTC')
	info "$label starts $t0"
	will "REJECT tcp-reset for the host's connections to $T_IP:$T_PORT (the guest service port of $T_VM); the guest process stays alive; removal scheduled first (ttl ${OPT_rule_ttl}s)"
	proof_wedge_insert "$T_IP" "$T_PORT" "$OPT_rule_ttl"
	proof_poll "$OPT_wait" 30 "$label: ownership reads stopped by wedged" fp3_state_is stopped wedged
	proof_wedge_remove "$T_IP" "$T_PORT"
	fp3_show_row
	info "$label journal (wedge lines):"
	node_journal "$PROOF_UNIT_VMCTL" "$t0" "wedge|$T_VM" 30
	FP3_LAST_WEDGE_START=$t0
}

step_0() {
	fp3_target
	manual "the tier is embedded in vmctl: a reviewed commit adds {\"computer_id\": \"$T_COMPUTER\", \"note\": \"FP3 group 1 deployed proof; remove after\"} to platform_computers in internal/computertiers/computer-tiers.json; the deploy carrying it and group 1 is target T1"
	[ "$PROOF_DRY" = 1 ] || { [ "$T_CLASS" = critical_protected ] && pass "warmness_class is critical_protected" || fail "warmness_class is '$T_CLASS', not critical_protected"; }
}

step_1() {
	local plan
	fp3_target
	local commit service status
	# vmctl's identity is its X-Choir-Build-Commit header, not a body field
	# (R5-3), on a 2xx answer (R6-4).
	read -r commit service status < <(vmctl_build_identity)
	commit=${commit:--} service=${service:--} status=${status:--}
	info "vmctl identity: X-Choir-Build-Commit=$commit X-Choir-Build-Service=$service status=$status"
	if [ -n "${OPT_t1:-}" ] && [ "$PROOF_DRY" != 1 ]; then
		case "$service:$status:$commit" in
		vmctl:2[0-9][0-9]:"$OPT_t1"*) pass "vmctl's build header names T1 ${OPT_t1} (status=$status)" ;;
		*) fail "vmctl's build header does not name T1 ${OPT_t1} (commit=$commit service=$service status=$status)" ;;
		esac
	fi
	plan=$(vmctl_get /internal/vmctl/deploy-refresh)
	if [ "$PROOF_DRY" != 1 ]; then
		printf '%s' "$plan" | jq -e --arg vm "$T_VM" '[.candidates[]? | select(.vm_id == $vm and .warmness_class == "critical_protected")] | length == 1' >/dev/null &&
			pass "$T_VM is a deploy-refresh candidate with warmness_class critical_protected" ||
			fail "$T_VM is not listed as a critical_protected candidate"
		printf '%s' "$plan" | jq -c --arg vm "$T_VM" '.candidates[]? | select(.vm_id == $vm) | {vm_id, rescue, warmness_class, computer_url}' | while IFS= read -r l; do info "candidate $l"; done
	fi
	fp3_show_row
}

step_2() { fp3_wedge "wedge 1"; }

step_3() {
	local since vm
	fp3_target
	since=$(fp3_since)
	vm=$T_VM
	fp3_has_spent() { node_journal "$PROOF_UNIT_VMCTL" "$since" "spent the automatic retry for always-on desktop vm=$vm" 5 | grep -q .; }
	proof_poll "$OPT_wait" 30 "journal: warmness policy spent the automatic retry for always-on desktop vm=$vm" fp3_has_spent
	proof_poll "$OPT_wait" 30 "ownership is active again" fp3_state_is active
	fp3_show_row
}

step_4() {
	local t0 waited=0 spent
	fp3_wedge "wedge 2"
	t0=${FP3_LAST_WEDGE_START:-}
	info "confirming it stays stopped for ${OPT_hold}s (no second automatic retry, no boot)"
	if [ "$PROOF_DRY" = 1 ]; then return 0; fi
	while [ "$waited" -lt "$OPT_hold" ]; do
		fp3_state_is stopped || { fail "the computer left the stopped state within ${waited}s: the 24 h bound is broken"; break; }
		proof_nap 30
		waited=$((waited + 30))
	done
	spent=$(node_journal "$PROOF_UNIT_VMCTL" "$t0" "spent the automatic retry for always-on desktop vm=$T_VM" 5 | grep -c . || true)
	[ "${spent:-0}" -eq 0 ] && pass "no further 'spent the automatic retry' line since wedge 2 started" || fail "$spent 'spent the automatic retry' line(s) since wedge 2 started"
	fp3_state_is stopped && pass "still stopped after ${OPT_hold}s" || fail "not stopped at the end of the hold"
	fp3_show_row
}

step_5() {
	local since vm
	fp3_target
	since=$(fp3_since)
	vm=$T_VM
	[ -n "${OPT_t2:-}" ] || die 2 "step 5 needs --t2 (the deploy target pushed after step 4)"
	info "before the deploy the candidate list should show rescue=true; after it, CI's refresh step prints 'Verified rescued $vm serves autoputer commit'"
	vmctl_get /internal/vmctl/deploy-refresh | jq -c --arg vm "$vm" '.candidates[]? | select(.vm_id == $vm) | {vm_id, rescue, warmness_class}' 2>/dev/null | while IFS= read -r l; do info "candidate $l"; done
	fp3_has_rearm() { node_journal "$PROOF_UNIT_VMCTL" "$since" "deploy target ${OPT_t2}.* re-arms the automatic retry" 5 | grep -q .; }
	fp3_has_rescued() { node_journal "$PROOF_UNIT_VMCTL" "$since" "rescued VM $vm.*crash boot" 5 | grep -q .; }
	proof_poll "$OPT_wait" 30 "journal: deploy target ${OPT_t2} re-arms the automatic retry" fp3_has_rearm
	proof_poll "$OPT_wait" 30 "journal: rescued VM $vm (crash boot)" fp3_has_rescued
	proof_poll "$OPT_wait" 30 "ownership is active" fp3_state_is active
	manual "record the CI run id for T2 and the refresh step lines ('stopped by wedged; booted onto the current deploy as a crash boot (no planned-restart marker)', 'Verified rescued $vm serves autoputer commit ...'); confirm no owner act, resolve or operator refresh was made between steps 4 and 5"
	fp3_show_row
}

step_6() {
	local body out
	[ -n "${OPT_t2:-}" ] || die 2 "step 6 needs --t2"
	fp3_wedge "wedge 3"
	body=$(jq -nc --arg u "$T_USER" --arg d "$T_DESKTOP" --arg t "$OPT_t2" '{user_id:$u,desktop_id:$d,target:$t}')
	will "POST /internal/vmctl/deploy-refresh with the SAME target $OPT_t2 for $T_VM (expects outcome skipped, 'the automatic retry for this deploy target is spent')"
	out=$(vmctl_post /internal/vmctl/deploy-refresh "$body" ci.deploy-refresh)
	info "response: $(printf '%s' "$out" | tr '\n' ' ' | cut -c1-600)"
	if [ "$PROOF_DRY" != 1 ]; then
		printf '%s' "$out" | grep -q '"outcome" *: *"skipped"' && pass "outcome is skipped" || fail "outcome is not skipped"
		printf '%s' "$out" | grep -q 'automatic retry for this deploy target is spent' && pass "reason says the retry for this deploy target is spent" || fail "reason does not say the retry is spent"
		fp3_state_is stopped && pass "nothing booted: still stopped" || fail "the computer is not stopped after the repeat target"
	fi
	fp3_show_row
}

step_7() {
	fp3_target
	info "guest obligations (restart kind and open work), read through the computer's own address:"
	node_b_q "curl -fsS --max-time 15 -H 'X-Internal-Caller: true' -H 'X-Authenticated-User: $T_USER' '$T_URL/api/runtime/obligations'" | jq -c '{restart, runs, open_work_items}' 2>/dev/null | while IFS= read -r l; do info "obligations $l"; done
	info "planned-restart marker lines for $T_VM in vmctl's journal (expect none consumed for the rescue):"
	node_journal "$PROOF_UNIT_VMCTL" "$(fp3_since)" "planned.restart.*$T_VM|$T_VM.*planned.restart" 20
	manual "in the guest's boot log after the rescue: no planned-restart marker consumed; open work from before the wedge closed as 'interrupted by a restart' (no shell channel into the guest here; use the product's run list for the test computer)"
}

step_8() {
	fp3_target
	local n
	n=$(proof_wedge_absent "$T_IP")
	[ "$PROOF_DRY" = 1 ] || { [ "${n:-0}" = 0 ] && pass "no REJECT rule for $T_IP remains" || fail "$n REJECT rule(s) for $T_IP remain"; }
	node_b_sh "systemctl list-timers 'choir-proof-unwedge-*' --no-pager | head -8"
	manual "revert the tier commit from step 0 (or leave it while groups 2 and 3 are proved, and say so)"
	manual "evidence to attach: journal lines of steps 2 to 6, CI run ids for T1 and T2, vmctl health identity, the ownership JSON after steps 2, 4, 5 and 6"
}

proof_main "$@"
