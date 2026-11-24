#!/usr/bin/env bash
# Deployed proof, UNRUN as of 2026-11-24: a new account's computer (4 GiB, the
# size the platform's computers already run; owner, 2026-11-24) boots, serves,
# hibernates, wakes from its snapshot and survives a deploy refresh. Every
# stranger's first computer is this path; if a step fails, sign-up is broken
# whatever the room says.
#
# Account: the drill account only (a numbered test address), never the paper's
# or the owner's. It registers through the real sign-up like a stranger: no
# account is exempt from the room bound, so it takes one place while it exists;
# delete it afterwards (the owner's act).
#
# Read-only steps run here (1, 2); the rest print the exact commands.
#   scripts/proofs/new-account-computer-deployed.sh [--dry-run]
# CHOIR_HOST defaults to https://choir.news. Node B calls go through ssh node-b.
set -euo pipefail
HOST=${CHOIR_HOST:-https://choir.news}
SSH=${PROOF_SSH_HOST:-node-b}
if [ "${1:-}" = "--dry-run" ]; then echo "dry run ok: would read $HOST/api/pulse/summary and print the manual steps"; exit 0; fi
fails=0
echo "step 1 (read-only): the public summary says whether sign-up is open, and only that"
body=$(curl -sS -m 15 "$HOST/api/pulse/summary") || { echo "FAIL pulse summary unreachable"; exit 1; }
printf '%s' "$body" | python3 -c '
import json, sys
c = json.load(sys.stdin).get("capacity", {})
print("public capacity:", c)
if set(c.keys()) != {"room"} or set(c["room"].keys()) != {"available", "open"}:
    print("FAIL the public capacity carries more than whether sign-up is open"); sys.exit(1)
if not c["room"]["available"] or not c["room"]["open"]:
    print("FAIL sign-up is not open with a trusted reading; the proof cannot register its account"); sys.exit(1)
' || fails=$((fails + 1))
echo "MANUAL step 1a: the owner reads the figures: signed in as the platform operator (passkey), GET $HOST/api/platform/operator/capacity or open Pulse; note room.places, room.promised, room.held_bytes, disk.available_bytes. Anyone else must get 401 or 403."
echo "step 1b (read-only, on Node B): the owners file auth reads is whole, fresh and says what each computer holds"
owners=$(ssh -o BatchMode=yes "$SSH" 'cat /var/lib/go-choir/host-room/computer-owners.json') || { echo "FAIL the owners file cannot be read on Node B"; fails=$((fails + 1)); owners=""; }
if [ -n "$owners" ]; then
  printf '%s' "$owners" | python3 -c '
import json, sys, datetime as d
j = json.load(sys.stdin)
written = d.datetime.fromisoformat(j["written_at"].replace("Z", "+00:00"))
age = (d.datetime.now(d.timezone.utc) - written).total_seconds()
owners = j["owners"]
print("owners file: written", int(age), "s ago;", len(owners), "accounts with a computer; largest disk GiB", round(max([o["disk_bytes"] for o in owners] or [0]) / 2**30, 1))
if age > 180:
    print("FAIL the owners file is older than the 3 minutes auth accepts; sign-up is closed"); sys.exit(1)
' || fails=$((fails + 1))
fi
echo "step 1c (read-only, on Node B): auth has not refused for want of a reading in the last 10 minutes"
refusals=$(ssh -o BatchMode=yes "$SSH" 'journalctl -u go-choir-auth --since "-10min" --no-pager | grep -c "step=room" || true') || { echo "FAIL could not read the auth journal"; fails=$((fails + 1)); refusals=0; }
[ "${refusals:-0}" = "0" ] || { echo "FAIL auth logged $refusals step=room refusals (a missing or stale reading, or no plan)"; fails=$((fails + 1)); }
echo "step 1d (read-only, on Node B): every snapshot's disk is the exact btrfs reading, and vmctl has not fallen back"
metas=$(ssh -o BatchMode=yes "$SSH" 'for f in /var/lib/go-choir/vm-state/*/snapshot/meta.json; do [ -f "$f" ] && printf "%s %s\n" "$f" "$(python3 -c "import json,sys; j=json.load(open(sys.argv[1])); print(j.get(\"disk_bytes_method\",\"\"), j.get(\"disk_bytes\",0))" "$f")"; done') || { echo "FAIL could not read the snapshot metas on Node B"; fails=$((fails + 1)); metas=""; }
bad=$(printf '%s\n' "$metas" | awk 'NF && $2 != "btrfs_extents"' | wc -l | tr -d ' ')
[ "$bad" = "0" ] || { echo "FAIL $bad snapshot(s) were not measured by btrfs_extents: the room counts them as 0 and holds the whole footprint back"; printf '%s\n' "$metas" | awk 'NF && $2 != "btrfs_extents"'; fails=$((fails + 1)); }
falls=$(ssh -o BatchMode=yes "$SSH" 'journalctl -u go-choir-vmctl --since "-1day" --no-pager | grep -c "snapshot disk measure btrfs_extents unavailable"; true') || { echo "FAIL could not read the vmctl journal on Node B"; fails=$((fails + 1)); falls=""; }
case "$falls" in
  ''|*[!0-9]*) echo "FAIL the vmctl journal read answered nothing usable ($falls)"; fails=$((fails + 1)) ;;
  0) ;;
  *) echo "FAIL vmctl fell back from the btrfs extent measure ($falls journal lines)"; fails=$((fails + 1)) ;;
esac
echo "step 1e (read-only, on Node B): no file of any computer, data images included, was counted as 0 for want of an exact measure"
inexact=$(printf '%s' "$owners" | python3 -c '
import json, sys
print(sum(o.get("inexact_files", 0) for o in json.load(sys.stdin)["owners"]))' 2>/dev/null) || inexact=""
case "$inexact" in
  ''|*[!0-9]*) echo "FAIL the owners file could not be read for inexact_files"; fails=$((fails + 1)) ;;
  0) ;;
  *) echo "FAIL $inexact file(s) were not measured exactly (counted 0; the room holds their footprint back)"; fails=$((fails + 1)) ;;
esac
cat <<'MANUAL'
MANUAL step 3: register the drill account in the browser at the site (mailed code, then passkey). Note its user id from the sign-in session; call it $UID_. The owner reading: room.promised rises by one and places fall by one until its computer exists (step 4); after it exists, places must not rise (the gap to 15 GiB is held until it has grown).
MANUAL step 4: sign in, open the computer, wait for it to answer. Then on Node B:
  jq '.[] | select(.user_id=="'$UID_'") | {vm_id, computer_id, mem_size_mib, state}' /var/lib/go-choir/vm-state/ownerships.json
  PASS only if mem_size_mib is 4096 and state is active. Record vm_id and computer_id.
MANUAL step 5: the boot receipt (cold boot at 4 GiB, no out-of-memory in replay):
  curl -s -H @<internal token header> 'localhost:8083/internal/vmctl/boot-timeline?computer_id=$CID'
  journalctl -u go-choir-vmctl --since '-15min' | grep -i "$VM"   # no oom, no kill
MANUAL step 6: hibernate it (the idle sweep, or the drill verb on this one computer), then
  curl -s ... 'localhost:8083/internal/vmctl/boot-timeline?computer_id=$CID&kind=hibernate'
  Before the hibernate record the owner reading's room.places (call it P0). PASS only if a snapshot was taken, its meta.json says disk_bytes_method btrfs_extents, and room.places read right after the hibernate is still P0 (a hibernate must not open a place); record its size in GiB (disk use per computer is the room bound's footprint: it assumes 15 GiB, nix/node-b.nix; a smaller measured figure is the evidence to lower it).
MANUAL step 7: sign in again; the wake must be snapshot_resume (boot-timeline kind boot) and the desktop must answer. FAIL if room.places on the owner reading after the wake differs from P0 (a wake must not open a place either; allow the 30 s tick to settle).
MANUAL step 8: land any commit that deploys; after the planned refresh the computer must be running again at 4096 (ownerships.json mem_size_mib unchanged; boot-timeline shows the new boot).
MANUAL step 9: ask the computer for an engineering capsule (a self-development change) from the drill account: it must admit (the capsule budget is 3/4 of 4 GiB, a capsule asks 2 GiB). If it is refused for memory, the 4 GiB shape cannot hold a capsule and the owner decides.
MANUAL step 9b: with host memory full (a drill on d@e only), a wake of the drill computer must answer 503 with Retry-After and the sentence "Choir is busy right now. Please try again in a minute.", with no host figures, in under 5 seconds.
MANUAL step 10: delete the drill account (owner's act) and confirm the owner reading room.accounts drops by one.
MANUAL step 11 (a full host, d@e only): with room.places 0 on the owner reading (or the owners file stopped for over 3 minutes, or AUTH_ROOM_* unset), a new sign-up at the mail step must answer 403 registration_waitlist (or 503 try again later when the reading is missing), create no account, and an existing account must still sign in.
MANUAL
[ "$fails" = 0 ] && echo "read-only steps passed; steps 3 to 10 (and 9b) are UNRUN" || { echo "FAIL $fails"; exit 1; }
