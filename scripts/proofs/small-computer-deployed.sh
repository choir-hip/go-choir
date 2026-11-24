#!/usr/bin/env bash
# Deployed proof, UNRUN as of 2026-11-24: a real 2 GiB computer boots, serves,
# hibernates, wakes from its snapshot and survives a deploy refresh. No
# 2 GiB computer has ever booted (Node B has run 8 GiB since 2026-09-03
# because a 2 GiB guest ran out of memory at boot replay), so every stranger's
# first computer is this path. If any step fails, sign-up is broken whatever
# the cap says.
#
# Account: the drill account only (d@e's numbered test address pattern), never
# the paper's or the owner's. It must be registered WITHOUT the operator
# secret: an operator-marked account is made at the interactive size on
# purpose (scripts/test-computer-create needs a computer that can run a
# capsule). It uses one slot of the sign-up cap while it exists; delete it
# afterwards (the owner's act).
#
# Read-only steps run here (1, 2); the rest print the exact commands.
#   scripts/proofs/small-computer-deployed.sh [--dry-run]
# CHOIR_HOST defaults to https://choir.news. Node B calls go through ssh node-b.
set -euo pipefail
HOST=${CHOIR_HOST:-https://choir.news}
SSH=${PROOF_SSH_HOST:-node-b}
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok: would read $HOST/api/pulse/summary and print the manual steps"; exit 0; fi
fails=0
echo "step 1 (read-only): the capacity reading is served and says whether sign-up is open"
body=$(curl -sS -m 15 "$HOST/api/pulse/summary") || { echo "FAIL pulse summary unreachable"; exit 1; }
printf '%s' "$body" | python3 -c '
import json, sys
c = json.load(sys.stdin).get("capacity", {})
r = c.get("registration", {})
print("registration:", {k: r.get(k) for k in ("available", "cap", "counted", "remaining", "open")})
print("disk free GiB:", round(c.get("disk", {}).get("available_bytes", 0) / 2**30, 1))
if not r.get("available") or not r.get("open"):
    print("FAIL sign-up is not open with a trusted reading; the proof cannot register its account"); sys.exit(1)
' || fails=$((fails + 1))
echo "step 2 (read-only): the deployed commit"
ssh -o BatchMode=yes "$SSH" 'cat /etc/go-choir-build-commit 2>/dev/null || systemctl show go-choir-vmctl -p ExecStart --value | cut -c1-120' || echo "MANUAL read the deployed commit"
cat <<'MANUAL'
MANUAL step 3: register the drill account in the browser at the site with a passkey (no operator secret). Note its user id from the sign-in session; call it $UID_.
MANUAL step 4: sign in, open the computer, wait for it to answer. Then on Node B:
  jq '.[] | select(.user_id=="'$UID_'") | {vm_id, computer_id, mem_size_mib, state}' /var/lib/go-choir/vm-state/ownerships.json
  PASS only if mem_size_mib is 2048 and state is active. Record vm_id and computer_id.
MANUAL step 5: the boot receipt (cold boot at 2 GiB, no out-of-memory in replay):
  curl -s -H @<internal token header> 'localhost:8083/internal/vmctl/boot-timeline?computer_id=$CID'
  journalctl -u go-choir-vmctl --since '-15min' | grep -i "$VM"   # no oom, no kill
MANUAL step 6: hibernate it (the idle sweep, or the drill verb on this one computer), then
  curl -s ... 'localhost:8083/internal/vmctl/boot-timeline?computer_id=$CID&kind=hibernate'
  PASS only if a snapshot was taken; record its size in GiB (disk use per computer is the cap's input: the cap assumes about 3 GiB).
MANUAL step 7: sign in again; the wake must be snapshot_resume (boot-timeline kind boot) and the desktop must answer.
MANUAL step 8: land any commit that deploys; after the planned refresh the computer must be running again at 2048 (ownerships.json mem_size_mib unchanged; boot-timeline shows the new boot).
MANUAL step 9: ask a self-development change from the drill account: it must be refused plainly ("self-development is not available on a computer this size"), not fail in a capsule.
MANUAL step 10: delete the drill account (owner's act) and confirm /api/pulse/summary counted drops by one.
MANUAL
[ "$fails" = 0 ] && echo "read-only steps passed; steps 3 to 10 are UNRUN" || { echo "FAIL $fails"; exit 1; }
