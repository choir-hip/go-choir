#!/usr/bin/env bash
# FP0 VM-state retention: the deployed proof of the test-class idle rule and
# the quarantine age-out
# (docs/problems/vm-state-retention-skips-test-computers-and-quarantines-2026-10-22.md;
# station docs/definitions/choir-launch-fp0-vm-retention-2026-10-22.md).
#
# Every step here is read-only. Enforce is not a step: it is an env change
# on vmctl in nix/node-b.nix (a reviewed commit and a deploy), and step 3
# reads its result.
#
#   scripts/proofs/fp0-vm-retention.sh [--commit SHA] [--since "..."]
#       [--dry-run | --list | --step N] [--evidence FILE]
#
#   0, 1, 2          the inert deploy: identity, the shadow report, the active plan unchanged
#   --step 3         after an enforce deploy: manifest lines and journal lines
set -uo pipefail
PROOF_NAME=fp0-vm-retention
. "$(dirname "$0")/lib.sh"

step_def 0 ro "identity: vmctl runs the commit, and the shadow plan has both rules in dry-run"
step_def 1 ro "the shadow report: every test-class computer and quarantined image, with bytes, age, verdict and reasons"
step_def 2 ro "the active plan is today's: no new candidate, no manifest"
step_def 3 ro-after "after enforce: manifest lines and vmctl's journal lines"

: "${OPT_commit:=}" "${OPT_since:=2 hours ago}"

# commit_matches A B: one is a prefix of the other (short or full SHAs).
commit_matches() {
	[ -n "$1" ] && [ -n "$2" ] || return 1
	case "$1" in "$2"*) return 0 ;; esac
	case "$2" in "$1"*) return 0 ;; esac
	return 1
}

SHADOW_JSON=""
shadow_plan() {
	[ -n "$SHADOW_JSON" ] || SHADOW_JSON=$(vmctl_get /internal/vmctl/retention-shadow-plan)
	printf '%s' "$SHADOW_JSON"
}

step_0() {
	local health commit cfg
	health=$(vmctl_get /health)
	commit=$(printf '%s' "$health" | jq -r '.build.commit // ""' 2>/dev/null)
	info "vmctl build.commit=$commit"
	if [ -n "$OPT_commit" ]; then
		check "vmctl runs $OPT_commit" commit_matches "$commit" "$OPT_commit"
	fi
	cfg=$(shadow_plan | jq -c '.config | {mode, test_idle_mode, test_idle_min_age_seconds, quarantine_mode, quarantine_min_age_seconds, selector_named}' 2>/dev/null)
	info "shadow config $cfg"
	check "shadow test_idle_mode is dry-run" test "$(printf '%s' "$cfg" | jq -r '.test_idle_mode // ""')" = dry-run
	check "shadow quarantine_mode is dry-run" test "$(printf '%s' "$cfg" | jq -r '.quarantine_mode // ""')" = dry-run
}

step_1() {
	local plan bad
	plan=$(shadow_plan)
	info "inventory $(printf '%s' "$plan" | jq -c '.inventory | {test_class_computers, test_class_bytes, test_class_would_remove, test_class_would_remove_bytes, quarantines, quarantine_bytes, quarantine_would_remove, quarantine_would_remove_bytes}' 2>/dev/null)"
	printf '%s' "$plan" | jq -r '.test_class[]? | "test-class \(.computer_id) \(.vm_id) state=\(.state) idle_days=\((.idle_seconds // 0) / 86400 | floor) gib=\((.size_bytes // 0) / 1073741824 * 10 | floor / 10) verdict=\(.verdict) reasons=\((.reasons // []) | join(","))"' 2>/dev/null |
		while IFS= read -r l; do info "$l"; done
	printf '%s' "$plan" | jq -r '.quarantines[]? | "quarantine \(.path) computer=\(.computer_id // "-") at=\(.quarantined_at) age_days=\((.age_seconds // 0) / 86400 | floor) gib=\((.size_bytes // 0) / 1073741824 * 10 | floor / 10) recovery=\(.recovery_id // "-") phase=\(.journal_phase // "-") status=\(.journal_status // "-") verdict=\(.verdict) reasons=\((.reasons // []) | join(","))"' 2>/dev/null |
		while IFS= read -r l; do info "$l"; done
	bad=$(printf '%s' "$plan" | jq -r --arg p "$PROOF_PAPER_HEX" --arg o "$PROOF_OWNER_HEX" --arg d "$PROOF_DELEGATE_HEX" \
		'[(.test_class[]?, .quarantines[]?) | select(.verdict != "keep") | select(((.computer_id // "") + (.vm_id // "") + (.path // "")) | ascii_downcase | (contains($p) or contains($o) or contains($d)))] | length' 2>/dev/null)
	check "the paper, the owner's computer and the delegate are never would_remove" test "${bad:-x}" = 0
	bad=$(printf '%s' "$plan" | jq -r '[(.test_class[]?, .quarantines[]?) | select(.verdict == "keep" and ((.reasons // []) | length) == 0)] | length' 2>/dev/null)
	check "every kept entry names its reasons" test "${bad:-x}" = 0
	bad=$(printf '%s' "$plan" | jq -r '[(.test_class[]?, .quarantines[]?) | select(.verdict == "will_remove")] | length' 2>/dev/null)
	check "the shadow plan plans no removal (no will_remove)" test "${bad:-x}" = 0
	manual "read the kept reasons of the five test computers (computer-2bd23366, -1f86c719, -7dcdf5e2 and FP0's two) and of the candidate fleet's two 2026-10-07 quarantines; record them in the problem record before any enforce"
}

step_2() {
	local plan n cfg
	plan=$(vmctl_get /internal/vmctl/retention-plan)
	cfg=$(printf '%s' "$plan" | jq -c '.config | {mode, test_idle_mode, quarantine_mode}' 2>/dev/null)
	info "active plan config $cfg decision=$(printf '%s' "$plan" | jq -r '.decision' 2>/dev/null)"
	check "active test_idle_mode is off" test "$(printf '%s' "$cfg" | jq -r '.test_idle_mode // ""')" = off
	check "active quarantine_mode is off" test "$(printf '%s' "$cfg" | jq -r '.quarantine_mode // ""')" = off
	n=$(printf '%s' "$plan" | jq -r '[.candidates[]? | select(.reason == "test_class_idle")] | length' 2>/dev/null)
	check "no test_class_idle candidate in the active plan" test "${n:-x}" = 0
	n=$(node_b_q "test -e /var/lib/go-choir/vm-state/retention-manifest.jsonl && echo present || echo absent")
	info "manifest: $n"
	[ "$PROOF_DRY" = 1 ] || check "no retention manifest yet (nothing removed)" test "$n" = absent
}

step_3() {
	info "manifest lines (last 40):"
	node_b_q "tail -n 40 /var/lib/go-choir/vm-state/retention-manifest.jsonl 2>/dev/null" |
		jq -c '{kind, event, vm_id, computer_id, path, size_bytes, quarantined_at, last_active_at, removed_at, why, error}' 2>/dev/null |
		while IFS= read -r l; do info "manifest $l"; done
	node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "retention (test_class_idle|quarantine_age_out)" 60
	manual "pair each removing line with a removed line (a lone removing line is a removal interrupted by a crash); compare df/du of /var/lib/go-choir/vm-state before and after; btrfs may free less than size_bytes when extents are shared"
}

proof_main "$@"
