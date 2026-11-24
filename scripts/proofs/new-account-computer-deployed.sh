#!/usr/bin/env bash
# Deployed proof, UNRUN as of 2026-11-24: a new account's computer (4 GiB, the
# size the platform's computers already run; owner, 2026-11-24) boots, serves,
# hibernates, wakes from its snapshot and survives a deploy refresh. Every
# stranger's first computer is this path; if a step fails, sign-up is broken
# whatever the cap says.
#
# Account: the drill account only (a numbered test address), never
# the paper's or the owner's. It must be registered WITHOUT the operator
# secret: an operator-marked account is not counted against the cap, and this
# proof must exercise the stranger's path. It uses one slot of the sign-up cap while it exists; delete it
# afterwards (the owner's act).
#
# Read-only steps run here (1, 2); the rest print the exact commands.
#   scripts/proofs/new-account-computer-deployed.sh [--dry-run]
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
  PASS only if mem_size_mib is 4096 and state is active. Record vm_id and computer_id.
MANUAL step 5: the boot receipt (cold boot at 4 GiB, no out-of-memory in replay):
  curl -s -H @<internal token header> 'localhost:8083/internal/vmctl/boot-timeline?computer_id=$CID'
  journalctl -u go-choir-vmctl --since '-15min' | grep -i "$VM"   # no oom, no kill
MANUAL step 6: hibernate it (the idle sweep, or the drill verb on this one computer), then
  curl -s ... 'localhost:8083/internal/vmctl/boot-timeline?computer_id=$CID&kind=hibernate'
  PASS only if a snapshot was taken; record its size in GiB (disk use per computer is the cap's input: the cap assumes 3 GiB, nix/node-b.nix).
MANUAL step 7: sign in again; the wake must be snapshot_resume (boot-timeline kind boot) and the desktop must answer.
MANUAL step 8: land any commit that deploys; after the planned refresh the computer must be running again at 4096 (ownerships.json mem_size_mib unchanged; boot-timeline shows the new boot).
MANUAL step 9: ask the computer for an engineering capsule (a self-development change) from the drill account: it must admit (the capsule budget is 3/4 of 4 GiB, a capsule asks 2 GiB). If it is refused for memory, the 4 GiB shape cannot hold a capsule and the owner decides.
MANUAL step 9b: with host memory full (a drill on d@e only), a wake of the drill computer must answer 503 with Retry-After and the sentence "Choir is busy right now. Please try again in a minute.", with no host figures, in under 5 seconds.
MANUAL step 10: delete the drill account (owner's act) and confirm /api/pulse/summary counted drops by one.
MANUAL
[ "$fails" = 0 ] && echo "read-only steps passed; steps 3 to 10 (and 9b) are UNRUN" || { echo "FAIL $fails"; exit 1; }
