#!/usr/bin/env bash
# Offline fixture test of scripts/proofs/fp8-image-jailer.sh steps 1 (--label
# after) and 7: a fake ssh answers vmctl's list, the target's fc-config.json,
# `systemctl show go-choir-vmctl -p Environment`, and a journalctl that always
# fails (exit 255), so a verdict that rests on the journal is a false pass.
# No network, no Node B, no key (docs/problems/firecracker-1-17-for-every-
# computer-2026-10-22.md, round 2 review finding 7). Cases:
#   - the pinned 1.17 environment passes;
#   - D4's shape (1.16.2 pinned, a next build set) fails, journal unreadable;
#   - an unreadable environment (systemctl fails) fails;
#   - a jailer from another package fails.
#
#   scripts/proofs/fp8-image-jailer-test.sh   (exit 0 when every case passes)
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT=$HERE/fp8-image-jailer.sh
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required" >&2; exit 1; }
WORK=$(mktemp -d "${TMPDIR:-/tmp}/fp8-image-test.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
FX=$WORK/fx
FAKES=$WORK/fakes
mkdir -p "$FX" "$FAKES"
export FX
TEST=computer-0123456789abcdef0123456789abcdef
CTRL=computer-fedcba9876543210fedcba9876543210
H=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
OLD=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
P117=/nix/store/$H-firecracker-1.17.0
P116=/nix/store/$OLD-firecracker-1.16.2

cat >"$FAKES/ssh" <<'J'
#!/bin/sh
in=$(cat)
case "$in" in
*journalctl*) echo "fake ssh: journalctl failed" >&2; exit 255 ;;
*systemctl*) if [ -f "$FX/env" ]; then echo "Environment=$(cat "$FX/env")"; else echo "fake ssh: systemctl failed" >&2; exit 255; fi ;;
*vmctl/list*) echo '{"ownerships":[{"vm_id":"vm-test1","computer_id":"computer-0123456789abcdef0123456789abcdef","user_id":"u2","desktop_id":"d2","state":"active","warmness_class":"test","computer_url":"http://10.9.7.2:8080"},{"vm_id":"vm-ctrl","computer_id":"computer-fedcba9876543210fedcba9876543210","user_id":"u3","desktop_id":"d3","state":"active","warmness_class":"standard","computer_url":"http://10.9.7.3:8080"}]}' ;;
*sha256sum*) echo abc123 ;;
*"grep -Ec"*) echo 1 ;;
*fc-config.json*) echo '"discard": true' ;;
esac
J
chmod +x "$FAKES/ssh"

failures=0
# run_case NAME EXPECT(pass|fail) STEP-ARGS... ; the environment text is in $FX/env (absent = unreadable)
run_case() {
	local name=$1 expect=$2 status
	shift 2
	printf '%s\n' abc123 >"$WORK/fp8-control-fc.txt"
	TMPDIR=$WORK PROOF_SSH_BIN=$FAKES/ssh PROOF_NAP_SCALE=0 PROOF_SSH_HOST=fake \
		bash "$SCRIPT" "$@" >"$WORK/out" 2>&1
	status=$?
	if { [ "$expect" = pass ] && [ "$status" -eq 0 ]; } || { [ "$expect" = fail ] && [ "$status" -eq 1 ]; }; then
		echo "ok: $name"
	else
		echo "FAIL: $name: expected $expect, exit $status: $(grep -E 'FAIL|PASS' "$WORK/out" | tail -n 3 | tr '\n' '|')"
		failures=$((failures + 1))
	fi
}
env_text() { printf '%s' "$1" >"$FX/env"; }

step7=(--step 7 --computer "$TEST")
step1=(--step 1 --label after --control "$CTRL")

env_text "VM_FIRECRACKER_BIN=$P117/bin/firecracker VM_JAILER_BIN=$P117/bin/jailer VM_DATA_DISCARD=1 VM_DATA_DISCARD_ONLY=*"
run_case "step 7: pinned 1.17, journal unreadable"      pass "${step7[@]}"
run_case "step 1 after: pinned 1.17, journal unreadable" pass "${step1[@]}"

env_text "VM_FIRECRACKER_BIN=$P116/bin/firecracker VM_JAILER_BIN=$P116/bin/jailer VM_FIRECRACKER_NEXT_BIN=$P117/bin/firecracker VM_FIRECRACKER_NEXT_ONLY=* VM_DATA_DISCARD=1 VM_DATA_DISCARD_ONLY=*"
run_case "step 7: D4's shape (1.16.2 pinned, next build), journal unreadable"      fail "${step7[@]}"
run_case "step 1 after: D4's shape (next build set), journal unreadable"           fail "${step1[@]}"

env_text "VM_FIRECRACKER_BIN=$P117/bin/firecracker VM_JAILER_BIN=$P117/bin/jailer VM_FIRECRACKER_NEXT_BIN=$P117/bin/firecracker"
run_case "step 7: pinned 1.17 but a next build is still set"                        fail "${step7[@]}"

env_text "VM_FIRECRACKER_BIN=$P117/bin/firecracker VM_JAILER_BIN=$P116/bin/jailer"
run_case "step 7: jailer from another package"                                      fail "${step7[@]}"

rm -f "$FX/env"
run_case "step 7: the environment cannot be read"                                   fail "${step7[@]}"
run_case "step 1 after: the environment cannot be read"                             fail "${step1[@]}"

[ "$failures" -eq 0 ] || { echo "$failures case(s) failed" >&2; exit 1; }
echo "all cases passed"
