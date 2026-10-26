#!/usr/bin/env bash
# FP0 VM-state retention: the deployed proof of the test-class idle rule and
# the quarantine age-out
# (docs/problems/vm-state-retention-skips-test-computers-and-quarantines-2026-10-22.md;
# station docs/definitions/choir-launch-fp0-vm-retention-2026-10-22.md).
#
# NOTE 2026-10-26: GET /internal/vmctl/retention-shadow-plan and the
# VMCTL_RETENTION_SHADOW_* env were deleted (the active plan, test-idle and
# quarantine modes active since 10-24, replaces them). Steps 0, 1 and 2 are
# the inert-deploy checks of that phase and read the retired endpoint; they
# are kept for the record and fail against a current vmctl. Step 3 onward
# reads the active plan's result.
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
#   4                the live orphan and ephemeral rules (R2-8, R3-3): no dir
#                    holding recovery state is a candidate; a run BEFORE the
#                    deploy is required (--evidence before.txt), then one AFTER
#                    it (--evidence after.txt); the recovery-dir lists must match
#
#   [--state-dir DIR]  vm-state on Node B (default /var/lib/go-choir/vm-state)
set -uo pipefail
PROOF_NAME=fp0-vm-retention
. "$(dirname "$0")/lib.sh"

step_def 0 ro "identity: vmctl runs the commit, and the shadow plan has both rules in dry-run"
step_def 1 ro "the shadow report: every test-class computer and quarantined image, with bytes, age, verdict and reasons"
step_def 2 ro "the active plan is today's: no new candidate, no manifest"
step_def 3 ro-after "after enforce: manifest lines and vmctl's journal lines"
step_def 4 ro "live rules: no dir holding recovery state is a deletion candidate; the kept-dir report"

: "${OPT_commit:=}" "${OPT_since:=2 hours ago}" "${OPT_state_dir:=/var/lib/go-choir/vm-state}"

# commit_matches A B: one is a prefix of the other (short or full SHAs).
commit_matches() {
	[ -n "$1" ] && [ -n "$2" ] || return 1
	case "$1" in "$2"*) return 0 ;; esac
	case "$2" in "$1"*) return 0 ;; esac
	return 1
}

# vmctl_runs COMMIT SERVICE STATUS WANT: /health answered 2xx and the build
# headers name vmctl and a commit matching WANT.
vmctl_runs() {
	case "$3" in 2[0-9][0-9]) ;; *) return 1 ;; esac
	[ "$2" = vmctl ] && [ "$1" != - ] && commit_matches "$1" "$4"
}

SHADOW_JSON=""
shadow_plan() {
	[ -n "$SHADOW_JSON" ] || SHADOW_JSON=$(vmctl_get /internal/vmctl/retention-shadow-plan)
	printf '%s' "$SHADOW_JSON"
}

step_0() {
	local commit service status cfg
	# vmctl's identity is its X-Choir-Build-Commit response header (the
	# commit compiled into the process), not a /health body field (R5-1),
	# on a 2xx answer (R6-4). The check text names what was observed (R6-3).
	read -r commit service status < <(vmctl_build_identity)
	commit=${commit:--} service=${service:--} status=${status:--}
	info "vmctl X-Choir-Build-Commit=$commit X-Choir-Build-Service=$service status=$status"
	if [ -n "$OPT_commit" ]; then
		check "vmctl runs $OPT_commit (2xx :8083/health, X-Choir-Build-Commit, X-Choir-Build-Service vmctl): observed commit=$commit service=$service status=$status" vmctl_runs "$commit" "$service" "$status" "$OPT_commit"
	else
		manual "no --commit given: vmctl's build identity was read, not compared"
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

step_4() {
	local out rc dirs plan planrc cands bad d n kib seen planned
	case "$OPT_state_dir" in *"'"*) die 2 "refusing a state dir with a single quote" ;; esac
	# Every vm-* or candidate-* dir holding a quarantine, a staging image or
	# a recovery journal, with its size. Read-only (ls and du). The first
	# line counts every vm-* and candidate-* dir; an unreadable state dir
	# exits 3, so a wrong --state-dir fails instead of reading as "none".
	out=$(node_b_q "cd '$OPT_state_dir' 2>/dev/null || exit 3
seen=0
for d in vm-* candidate-*; do [ -d \"\$d\" ] && seen=\$((seen + 1)); done
echo \"state-dir dirs=\$seen\"
for d in vm-* candidate-*; do
	[ -d \"\$d\" ] || continue
	n=\$(ls -1 \"\$d\" 2>/dev/null | grep -cE '^(data\\.img\\.quarantine-|data\\.img\\.staging-|rec-.*\\.journal\$)')
	[ \"\$n\" -gt 0 ] || continue
	printf '%s %s %s\\n' \"\$d\" \"\$n\" \"\$(du -sk \"\$d\" 2>/dev/null | cut -f1)\"
done")
	rc=$?
	plan=$(vmctl_get /internal/vmctl/retention-plan)
	planrc=$?
	if [ "$PROOF_DRY" = 1 ]; then
		info "(dry-run) would check: the state dir and the active plan were read, the plan's shape, and that no dir holding recovery state is a candidate"
		return 0
	fi
	seen=$(printf '%s\n' "$out" | sed -n '1s/^state-dir dirs=\([0-9][0-9]*\)$/\1/p')
	if [ "$rc" != 0 ] || [ -z "$seen" ]; then
		fail "the state dir $OPT_state_dir was not read on Node B (exit $rc); nothing compared"
		return 0
	fi
	dirs=$(printf '%s\n' "$out" | sed 1d)
	printf '%s\n' "$dirs" | while read -r d n kib; do
		[ -n "$d" ] && info "recovery-dir $d artifacts=$n kib=$kib"
	done
	if [ "$planrc" != 0 ] || [ -z "$plan" ]; then
		fail "the active plan was not read (exit $planrc, ${#plan} bytes); nothing compared"
		return 0
	fi
	if ! printf '%s' "$plan" | jq -e 'type == "object" and (.decision | type) == "string" and .decision != "" and (.inventory | type) == "object" and (.config | type) == "object" and ((.candidates // []) | type) == "array" and ((.kept_state_dirs // []) | type) == "array"' >/dev/null 2>&1; then
		fail "the active plan does not parse as a retention plan (${#plan} bytes: $(printf '%s' "$plan" | head -c 120 | tr -d '\n')); nothing compared"
		return 0
	fi
	info "active plan decision=$(printf '%s' "$plan" | jq -r '.decision') candidates=$(printf '%s' "$plan" | jq -r '[.candidates[]?] | length') state_dirs=$(printf '%s' "$plan" | jq -r '.inventory.state_dirs // 0') kept_state_dirs=$(printf '%s' "$plan" | jq -r '.inventory.kept_state_dirs // 0') state_dir_seen=$seen"
	# Sorted by vm_id so the before and after lines diff line for line.
	printf '%s' "$plan" | jq -r '.kept_state_dirs // [] | sort_by(.vm_id)[] | "kept \(.vm_id) rule=\(.rule) gib=\((.size_bytes // 0) / 1073741824 * 10 | floor / 10) reasons=\((.reasons // []) | join(",")) release=\(.release)"' |
		while IFS= read -r l; do info "$l"; done
	planned=$(printf '%s' "$plan" | jq -r '.inventory.state_dirs // 0')
	if [ "$seen" = 0 ] && [ "${planned:-0}" != 0 ]; then
		fail "the state dir $OPT_state_dir holds no vm-* or candidate-* dir, but the plan counts $planned; wrong --state-dir? nothing compared"
		return 0
	fi
	cands=$(printf '%s' "$plan" | jq -r '.candidates // [] | .[].vm_id // empty')
	if [ -z "$dirs" ]; then
		manual "nothing to check: no dir holds recovery state in $OPT_state_dir ($seen dirs read); this is not a pass of the no-candidate check"
		return 0
	fi
	if [ -z "$cands" ]; then
		manual "nothing to check: the active plan has no candidates; this is not a pass of the no-candidate check"
	else
		bad=""
		for d in $(printf '%s\n' "$dirs" | awk '{print $1}'); do
			if printf '%s\n' "$cands" | grep -qx "$d"; then bad="$bad $d"; fi
		done
		check "no dir holding recovery state is a deletion candidate${bad:+ (candidates:$bad)}" test -z "$bad"
	fi
	manual "a run of this step BEFORE the deploy is required, not optional: without its --evidence before.txt this run has nothing to be compared with (D2 missed it). Run it before the deploy and again after it (--evidence after.txt); every recovery-dir line before must still be there after (the 10-07 candidate-fleet quarantines included), and after the deploy each ownerless one must appear as a kept line with rule orphan_state_dir"
}

proof_main "$@"
