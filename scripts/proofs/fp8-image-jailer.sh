#!/usr/bin/env bash
# FP8 (batch B6: the VM image, the jailer and density) deployed proofs.
# Steps are "FP0 proof steps" in
# docs/definitions/choir-launch-fp8-image-jailer-2026-10-21.md on
# lane/fp8-image-jailer. Every behavior change there ships off and is selected
# per computer, so the deploy changes no running computer until a reviewed
# nix/node-b.nix change names a TEST computer. Those "turn on" edits (a nix
# change and a deploy, one red activation each) are MANUAL lines here; this
# script reads what they should have produced.
#
# Targets are TEST computers only: the paper, the owner's computer and the
# delegate are refused for every step that resolves a target (the control
# computer, read-only, may be any non-delegate computer).
#
#   scripts/proofs/fp8-image-jailer.sh [--computer computer-<test1>] [--computer2 computer-<test2>]
#       [--control computer-<non-test>] [--label before|after] [--since "2026-10-22 10:00:00 UTC"]
#       [--dry-run | --list | --step N [--mutate]] [--evidence FILE]
#
# Order: 1 --label before; deploy FP8 as its own red batch; 1 --label after;
# 2 (mutating, mints a test computer) then 3; turn the jailer on (nix), 4, 5, 6;
# NET-08 (nix), 7; RES-06 (nix), 8; 9; 10.
set -uo pipefail
PROOF_NAME=fp8-image
. "$(dirname "$0")/lib.sh"

step_def 1 ro "landing: a control computer's fc-config.json hash is unchanged; vmctl's environment names no VM_FIRECRACKER_NEXT_BIN"
step_def 2 mut "RES-09: scripts/memory_hotplug_probe.py (mints its own test computer, grows and shrinks it)"
step_def 3 ro-after "RES-09 diagnosis: the test computer's list entry and fc-config.json"
step_def 4 ro-after "SEC-04: vmctl says 'jailer ready'; the test computer's confinement is jailer"
step_def 5 ro-after "RES-05: two test computers' jails share one root-owned 0444 store image"
step_def 6 ro-after "SEC-04: vmm_confinement_verify on the test computer (read-only)"
step_def 7 ro-after "NET-08/RES-16: pinned Firecracker 1.17 for every computer; data drive discard"
step_def 8 ro-after "RES-06: damon_reclaim boot args; host RSS of the test computer vs the control"
step_def 9 ro-after "RES-14: hibernate disk-bytes method"
step_def 10 ro "L4-03: users holding a passkey, for the cap number the owner confirms"

: "${OPT_label:=before}" "${OPT_since:=2 hours ago}"
VMSTATE=/var/lib/go-choir/vm-state

fp8_target() { resolve_target "${OPT_computer:-}" 1; }

fp8_state() { printf '%s/fp8-%s.txt' "${TMPDIR:-/tmp}" "$1"; }

# fp8_read_env: FP8_ENV = the running vmctl unit's Environment, read from
# systemd (positive evidence of the host's pin; a journal grep cannot give it,
# because "no line" is also what a failed or windowed journal read prints).
# FAILs and returns 1 when the read itself fails or is not an Environment line;
# returns 1 without a verdict in dry-run (after printing the command).
fp8_read_env() {
	if [ "$PROOF_DRY" = 1 ]; then
		node_b_q "systemctl show '$PROOF_UNIT_VMCTL' -p Environment"
		return 1
	fi
	FP8_ENV=$(node_b_q "systemctl show '$PROOF_UNIT_VMCTL' -p Environment") || {
		fail "could not read vmctl's Environment (ssh or systemctl failed)"
		return 1
	}
	case "$FP8_ENV" in
	Environment=*) FP8_ENV=${FP8_ENV#Environment=} ;;
	*)
		fail "vmctl's Environment read returned no Environment= line"
		return 1
		;;
	esac
}

# fp8_env_val NAME: the value of NAME in FP8_ENV, empty when unset.
fp8_env_val() { printf '%s\n' "$FP8_ENV" | tr ' ' '\n' | sed -n "s/^$1=//p" | head -n 1; }

# fp8_check_no_next_build: vmctl's environment names no VM_FIRECRACKER_NEXT_BIN.
fp8_check_no_next_build() {
	[ -z "$(fp8_env_val VM_FIRECRACKER_NEXT_BIN)" ] && pass "no VM_FIRECRACKER_NEXT_BIN in vmctl's environment" || fail "VM_FIRECRACKER_NEXT_BIN is set in vmctl's environment"
}

# fp8_check_pinned_117: VM_FIRECRACKER_BIN and VM_JAILER_BIN are both under one
# firecracker-1.17.x store path, and no next build is set.
fp8_check_pinned_117() {
	local bin jail re='^/nix/store/[a-z0-9]{32}-firecracker-1\.17\.[0-9]+/bin/'
	fp8_read_env || return 0
	bin=$(fp8_env_val VM_FIRECRACKER_BIN)
	jail=$(fp8_env_val VM_JAILER_BIN)
	if [[ $bin =~ ${re}firecracker$ ]]; then pass "VM_FIRECRACKER_BIN is the pinned 1.17 build ($bin)"; else fail "VM_FIRECRACKER_BIN is '${bin:-<unset>}', not a firecracker-1.17.x store path"; fi
	if [[ $jail =~ ${re}jailer$ ]] && [ "${jail%/bin/jailer}" = "${bin%/bin/firecracker}" ]; then pass "VM_JAILER_BIN is from the same package ($jail)"; else fail "VM_JAILER_BIN is '${jail:-<unset>}', not the jailer of VM_FIRECRACKER_BIN's package"; fi
	fp8_check_no_next_build
}

step_1() {
	local vm hash
	[ -n "${OPT_control:-}" ] || die 2 "step 1 needs --control (a non-test computer; read-only, so the paper is fine)"
	resolve_target "$OPT_control" 0
	vm=$T_VM
	hash=$(node_b_q "sha256sum $VMSTATE/$vm/fc-config.json | cut -d' ' -f1")
	if [ "$PROOF_DRY" != 1 ]; then
		info "control $T_COMPUTER ($vm) fc-config.json sha256=$hash (label=$OPT_label)"
		if [ "$OPT_label" = before ]; then
			printf '%s\n' "$hash" >"$(fp8_state control-fc)"
			info "saved; redeploy, let the control computer relaunch at its NEXT natural launch, then --label after"
		else
			[ -s "$(fp8_state control-fc)" ] || die 2 "no 'before' hash saved"
			[ "$hash" = "$(cat "$(fp8_state control-fc)")" ] && pass "control fc-config.json unchanged across the deploy" || fail "control fc-config.json changed (before $(cat "$(fp8_state control-fc)") after $hash)"
		fi
	fi
	info "vmctl journal 'next Firecracker build' lines since '$OPT_since' (informational only; the check below reads vmctl's environment):"
	node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "next Firecracker build" 10
	if [ "$OPT_label" = after ]; then
		fp8_read_env && fp8_check_no_next_build
	fi
}

step_2() {
	local out
	out="docs/evidence/fp8-res09-$(date -u +%Y%m%d).json"
	will "run scripts/memory_hotplug_probe.py --ssh $PROOF_SSH_HOST --out $out: it mints its own test computer, hot-plugs and unplugs memory, and reports; it may leave the computer unless it cleans up (its own --keep-computer flag decides)"
	if [ "$PROOF_DRY" = 1 ]; then
		info "\$ python3 -I scripts/memory_hotplug_probe.py --ssh $PROOF_SSH_HOST --out $out"
		return 0
	fi
	python3 -I "$(dirname "$0")/../memory_hotplug_probe.py" --ssh "$PROOF_SSH_HOST" --out "$out" && pass "memory_hotplug_probe passed (no 'not selected' refusal); receipt $out" || fail "memory_hotplug_probe failed; read $out and step 3"
}

step_3() {
	local list_row
	fp8_target
	list_row=$(proof_row_json "$T_COMPUTER")
	[ "$PROOF_DRY" = 1 ] || printf '%s' "$list_row" | jq -c '{state, warmness_class, memory_hotplug_selected, memory_hotplug_phase, memory_plugged_mib}' | while IFS= read -r l; do info "list $l"; done
	node_b_sh "f=$VMSTATE/$T_VM/fc-config.json; grep -c 'memory-hotplug' \$f | sed 's/^/memory-hotplug entries: /'; grep -o 'memhp_default_state=[a-z_]*' \$f | head -1"
	if [ "$PROOF_DRY" != 1 ]; then
		[ "$(node_b_q "grep -c 'memory-hotplug' $VMSTATE/$T_VM/fc-config.json || true")" -ge 1 ] && pass "fc-config.json has memory-hotplug" || fail "fc-config.json has no memory-hotplug"
		[ "$(node_b_q "grep -c 'memhp_default_state=online_movable' $VMSTATE/$T_VM/fc-config.json || true")" -ge 1 ] && pass "boot args carry memhp_default_state=online_movable" || fail "boot args lack memhp_default_state=online_movable"
	fi
	info "the P5 hypothesis's verdict: 403 means the probe's token header, a non-active state means timing, class primary means the auth DB read; record which"
}

step_4() {
	fp8_target
	manual "turn on: nix/node-b.nix VMCTL_JAILER=on and VMCTL_JAILER_ONLY=$T_COMPUTER (a reviewed change and a deploy), then stop/start the test computer"
	info "vmctl journal since '$OPT_since':"
	node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "jailer ready|JAILER UNAVAILABLE" 6
	if [ "$PROOF_DRY" != 1 ]; then
		node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "JAILER UNAVAILABLE" 3 | grep -q . && fail "vmctl logged JAILER UNAVAILABLE"
		node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "jailer ready" 3 | grep -q . && pass "vmctl logged 'jailer ready'" || fail "no 'jailer ready' line since '$OPT_since'"
		node_b_q "grep -o '\"store.img\":\"[^\"]*\"\\|store.img=[a-z()]*' $VMSTATE/$T_VM/boot-timeline.json | head -3" | while IFS= read -r l; do info "boot-timeline $l"; done
	fi
}

step_5() {
	local a b
	fp8_target
	a=$T_VM
	[ -n "${OPT_computer2:-}" ] || die 2 "step 5 needs --computer2 (a second test computer, added to VMCTL_JAILER_ONLY and started)"
	resolve_target "$OPT_computer2" 1
	b=$T_VM
	local base=$VMSTATE/.jail/firecracker
	node_b_sh "stat -c '%d:%i %U %a %h %n' $base/$a/root/store.img $base/$b/root/store.img; ls -la $VMSTATE/.jail/.shared/"
	if [ "$PROOF_DRY" != 1 ]; then
		local s
		s=$(node_b_q "stat -c '%d:%i|%U|%a|%h' $base/$a/root/store.img $base/$b/root/store.img")
		[ "$(printf '%s\n' "$s" | awk -F'|' '{print $1}' | sort -u | wc -l | tr -d ' ')" = 1 ] && pass "both jails hold the same dev:inode" || fail "the jails' store.img are different files"
		printf '%s\n' "$s" | awk -F'|' '$2 != "root" || $3 != "444" || $4 < 3 {bad=1} END {exit bad}' && pass "owner root, mode 444, link count >= 3" || fail "owner/mode/link count not as expected"
	fi
}

step_6() {
	fp8_target
	will "(read-only) python3 scripts/vmm_confinement_verify.py --ssh $PROOF_SSH_HOST --vm $T_VM --expect jailer"
	if [ "$PROOF_DRY" = 1 ]; then return 0; fi
	python3 -I "$(dirname "$0")/../vmm_confinement_verify.py" --ssh "$PROOF_SSH_HOST" --vm "$T_VM" --expect jailer --out "docs/evidence/vmm-confinement-$(date -u +%Y%m%d).json" >/dev/null && pass "vmm_confinement_verify --expect jailer passed" || fail "vmm_confinement_verify did not pass"
	manual "lifecycle on the test computer: hibernate by snapshot and wake (scripts/s3_snapshot_probe.py pattern; expect snapshot_resume), stop/start, systemctl restart go-choir-vmctl and confirm reattach, then the assumed-breach probe (problem record step 3); then VMCTL_JAILER_ONLY=class:test (owner decision first: it cold boots every hibernated computer once)"
}

step_7() {
	# Since the pin to 1.17.0 (docs/problems/firecracker-1-17-for-every-computer-2026-10-22.md)
	# there is no "next" build and no per-computer selector: every computer runs the
	# pinned 1.17 VMM and every data drive has discard. The pin is asserted from
	# vmctl's running environment (positive evidence); the journal's "next
	# Firecracker build" line is written once at config load, so a --since
	# window can miss it and an empty read cannot show it was absent. Host-side
	# proof of the pin before the deploy: scripts/check-host-firecracker-pin.sh.
	fp8_target
	manual "host step (done by the 1.17 pin deploy, not here): VM_FIRECRACKER_BIN and VM_JAILER_BIN are under the pinned firecracker-1.17.0 path, VM_FIRECRACKER_NEXT_BIN is unset, VM_DATA_DISCARD=1 with VM_DATA_DISCARD_ONLY=*"
	node_b_sh "grep -o '\"discard\": *[a-z]*\\|\"io_engine\": *\"[A-Za-z]*\"' $VMSTATE/$T_VM/fc-config.json"
	fp8_check_pinned_117
	if [ "$PROOF_DRY" != 1 ]; then
		[ "$(node_b_q "grep -Ec '\"discard\": *true' $VMSTATE/$T_VM/fc-config.json || true")" -ge 1 ] && pass "data drive has discard true" || fail "data drive lacks discard true"
	fi
	manual "in the guest: /sys/block/vdb/queue/discard_max_bytes non-zero and choir-data-fstrim.timer active; hibernate and wake expects snapshot_resume under Firecracker v1.17.0"
	manual "RES-16 measurement: hourly for 6 hours, du -k data.img against the live store: within about 1.5x and no hourly latency regression"
}

step_8() {
	local pid
	fp8_target
	manual "owed first: free page reporting reads (LR report steps 3 to 5) on a test computer without reclaim; then VM_GUEST_RECLAIM_ONLY=<second test computer id> (nix + deploy) and a cold boot of it"
	node_b_sh "grep -o 'damon_reclaim\\.[a-z_]*=[^ \"]*' $VMSTATE/$T_VM/fc-config.json | sort -u"
	if [ "$PROOF_DRY" != 1 ]; then
		[ "$(node_b_q "grep -o 'damon_reclaim\\.[a-z_]*=' $VMSTATE/$T_VM/fc-config.json | sort -u | wc -l | tr -d ' '")" -ge 6 ] && pass "six damon_reclaim.* boot args" || fail "fewer than six damon_reclaim.* boot args"
	fi
	pid=$(node_b_q "for p in \$(pgrep -f 'firecracker.*--id $T_VM'); do [ \"\$(cat /proc/\$p/comm)\" = firecracker ] && echo \$p && break; done")
	[ "$PROOF_DRY" = 1 ] || info "host firecracker VmRSS: $(node_b_q "grep VmRSS /proc/${pid:-0}/status 2>/dev/null || echo unreadable")"
	manual "guest DAMON state is on procmem now (memory_return: enabled, kdamond_pid, bytes_reclaimed_regions, page reporting, balloon reporting bit): run scripts/fp8_memory_return_probe.py --computer-id <this test computer> --burst capsule --cred-file <its cred file> --out docs/evidence/fp8-memory-return-<id>-<date>.csv (about 25 min), then the same on an unselected test computer as the control; read the outcome in docs/reports/guest-memory-return-outcomes-2026-10-22.md. Pass = outcome C at idle with no workingset_refault_file / pgmajfault growth and kdamond CPU < ~1% of a core"
}

step_9() {
	fp8_target
	manual "hibernate the test computer by snapshot first"
	info "vmctl journal 'hibernated VM $T_VM' lines (expect disk_bytes_method=btrfs_extents):"
	node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "hibernated VM $T_VM" 5
	if [ "$PROOF_DRY" != 1 ]; then
		node_journal "$PROOF_UNIT_VMCTL" "$OPT_since" "hibernated VM $T_VM.*disk_bytes_method=btrfs_extents" 2 | grep -q . && pass "disk_bytes_method=btrfs_extents" || fail "no hibernated line with disk_bytes_method=btrfs_extents"
	fi
	manual "run scripts/snapshot_disk_measure_compare.py on that snapshot/mem and compare; then the owner revisits VM_SNAPSHOT_BUDGET_GIB=48 and the 36 h max age"
}

step_10() {
	local n
	n=$(node_b_q "sqlite3 -readonly /var/lib/go-choir/auth/auth.db 'SELECT COUNT(DISTINCT user_id) FROM credentials' 2>&1")
	[ "$PROOF_DRY" = 1 ] && return 0
	case "$n" in
	'' | *[!0-9]*) fail "could not count users holding a passkey on Node B ($n)" ;;
	*) info "users holding a passkey: $n; recommended first cap: $((n + 75)) (the owner confirms the number)" ;;
	esac
	manual "set AUTH_REGISTRATION_CAP=<n> (nix + deploy); probe with a fresh @example.test address at the cap: 403 with code registration_waitlist, no users row, registration_waitlist holds the email; the owner can still log in; host admission stays the second layer (VMCTL_ADMISSION_MODE=measured)"
}

proof_main "$@"
