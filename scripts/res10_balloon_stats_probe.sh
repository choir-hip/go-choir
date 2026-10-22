#!/usr/bin/env bash
# Read-only RES-10 probe (docs/problems/balloon-stats-unread-no-emergency-inflation-2026-10-21.md):
# for ONE test computer, print the guest's balloon statistics as vmctl reads
# them from Firecracker beside the guest's own /proc/meminfo and the VMM's
# host RSS, so FP0 can compare the two views of the same guest.
#
#   host   GET http://127.0.0.1:8083/internal/vmctl/memory/balloon?computer_id=ID
#          (host only; Firecracker GET /balloon + GET /balloon/statistics),
#          through choir_internal_curl (scripts/lib/choir_internal.sh)
#   host   VmRSS of that computer's firecracker process
#   guest  GET <guest>:8085/internal/debug/pprof/procmem (meminfo, KiB),
#          a plain curl with NO credential and no internal-caller marker:
#          the guest gate is the source address (HostSourcedCaller)
#
# Refuses any computer vmctl does not class as "test". Boots, stops,
# selects and writes nothing; never PATCHes a balloon. Each remote command
# runs under `ssh HOST sh -s` with the helper's text sent ahead of it (as
# scripts/proofs/lib.sh does). The helper attaches the internal service
# token only for 127.0.0.1, ::1 or localhost on an internal service port,
# never to a guest; this script never reads or prints the token. Output: a
# table on stderr and one JSON receipt on stdout (and in --out FILE).
#
# Exit: 0 a reading or a reason (stats_disabled, ...); 2 usage, refusal, or
# a vmctl answer that is not 2xx or has no entry for the computer (status
# and body printed, no table); 3 the guest meminfo read failed (receipt
# printed with the error).
#
# usage: scripts/res10_balloon_stats_probe.sh --computer-id ID [--ssh node-b]
#        [--out FILE] [--label cold-boot-5m]
#
# Before it can show numbers, the computer must be selected
# (VM_BALLOON_STATS_ONLY=<ID> in nix/node-b.nix, deployed) and cold booted.
# Unselected or restored-without-stats computers print the reason
# (stats_disabled, not_reported_yet, ...), never zeros.
set -euo pipefail

SSH_HOST=node-b
COMPUTER_ID=""
OUT=""
LABEL=""
VMCTL=http://127.0.0.1:8083
CHOIR_INTERNAL_LIB=$(cat "$(dirname "${BASH_SOURCE[0]}")/lib/choir_internal.sh") || {
	echo "res10_balloon_stats_probe: cannot read scripts/lib/choir_internal.sh" >&2
	exit 2
}

die() { echo "res10_balloon_stats_probe: $*" >&2; exit 2; }

while [ $# -gt 0 ]; do
	case "$1" in
	--computer-id) COMPUTER_ID="${2:-}"; shift 2 ;;
	--ssh) SSH_HOST="${2:-}"; shift 2 ;;
	--out) OUT="${2:-}"; shift 2 ;;
	--label) LABEL="${2:-}"; shift 2 ;;
	-h | --help) sed -n '2,25p' "$0"; exit 0 ;;
	*) die "unknown argument $1" ;;
	esac
done
[ -n "$COMPUTER_ID" ] || die "--computer-id is required (a test computer)"
[[ "$COMPUTER_ID" =~ ^[A-Za-z0-9._-]+$ ]] || die "bad computer id"
[[ "$SSH_HOST" =~ ^[A-Za-z0-9._@-]+$ ]] || die "bad ssh host"
[[ "$LABEL" =~ ^[A-Za-z0-9._:-]*$ ]] || die "bad label"
command -v python3 >/dev/null || die "python3 is required locally"

# remote SCRIPT: run SCRIPT on the host under sh -s, after the helper.
remote() { printf '%s\n%s\n' "$CHOIR_INTERNAL_LIB" "$1" | ssh -o BatchMode=yes "$SSH_HOST" sh -s; }

# fetch CURL URL [curl args...]: CURL is choir_internal_curl (vmctl) or
# curl (the guest). Sets BODY and STATUS (the HTTP status, or
# "ssh-or-curl-failed"). curl prints the status on the last line.
fetch() {
	local via=$1 url=$2 raw
	shift 2
	if ! raw=$(remote "$via '$url' -sS $* -w '\n%{http_code}'"); then
		BODY=$raw STATUS=ssh-or-curl-failed
		return
	fi
	STATUS=${raw##*$'\n'}
	BODY=${raw%$'\n'*}
	[ "$BODY" != "$raw" ] || BODY=""
}
ok2xx() { [[ "$STATUS" =~ ^2[0-9][0-9]$ ]]; }
vmctl_get() { # path -> BODY, or die with status and body
	fetch choir_internal_curl "$VMCTL$1" -m 20
	ok2xx || die "vmctl GET $1: status $STATUS: $(printf '%s' "$BODY" | head -c 400)"
}

vmctl_get /internal/vmctl/list
listing=$BODY
own=$(printf '%s' "$listing" | python3 -c '
import json, sys
cid = sys.argv[1]
try:
    data = json.load(sys.stdin)
except Exception:
    print("ERR vmctl list is not JSON"); sys.exit(0)
for o in data.get("ownerships") or []:
    if o.get("computer_id") == cid:
        print(json.dumps({k: o.get(k) for k in ("vm_id", "computer_id", "warmness_class", "state", "computer_url")}))
        sys.exit(0)
print("ERR computer not found in vmctl list")
' "$COMPUTER_ID")
case "$own" in ERR*) die "${own#ERR }" ;; esac
field() { printf '%s' "$own" | python3 -c 'import json,sys; print(json.load(sys.stdin).get(sys.argv[1]) or "")' "$1"; }
CLASS=$(field warmness_class)
VM_ID=$(field vm_id)
URL=$(field computer_url)
[ "$CLASS" = "test" ] || die "computer $COMPUTER_ID is class '$CLASS', not test; this probe reads test computers only"
[[ "$VM_ID" =~ ^[A-Za-z0-9._-]+$ ]] || die "bad vm id from vmctl"
GUEST=$(printf '%s' "$URL" | sed -n 's#^http://\([0-9.]*\):8085.*#\1#p')

vmctl_get "/internal/vmctl/memory/balloon?computer_id=$COMPUTER_ID"
balloon=$BODY
entry_check=$(printf '%s' "$balloon" | python3 -c '
import json, sys
try:
    vms = json.load(sys.stdin).get("vms")
except Exception:
    print("not JSON"); sys.exit(0)
if not isinstance(vms, list) or len(vms) != 1 or vms[0].get("vm_id") != sys.argv[1]:
    print("no single entry for vm %s" % sys.argv[1]); sys.exit(0)
e = vms[0]
if not e.get("reason") and not e.get("reading"):
    print("entry has neither a reading nor a reason")
' "$VM_ID")
[ -z "$entry_check" ] || die "vmctl balloon answer: $entry_check: $(printf '%s' "$balloon" | head -c 400)"
rss_kib=$(remote "for p in \$(pgrep -f 'firecracker.*$VM_ID'); do [ \"\$(cat /proc/\$p/comm)\" = firecracker ] && awk '/^VmRSS/{print \$2}' /proc/\$p/status; done | head -1" || true)
procmem=""
procmem_error=""
if [[ "$GUEST" =~ ^[0-9.]+$ ]]; then
	# Plain curl: no token, no marker. (choir_internal_curl would never
	# attach the token to a guest either; the guest needs neither.)
	fetch curl "http://$GUEST:8085/internal/debug/pprof/procmem" -m 30
	if ok2xx; then
		procmem=$BODY
	else
		procmem_error="guest procmem: status $STATUS: $(printf '%s' "$BODY" | head -c 300)"
	fi
else
	procmem_error="no guest address in vmctl computer_url"
fi

receipt=$(BALLOON="$balloon" PROCMEM="$procmem" PROCMEM_ERROR="$procmem_error" RSS_KIB="$rss_kib" OWN="$own" LABEL="$LABEL" python3 -c '
import datetime, json, os, sys
MIB = 1 << 20
def load(name):
    try:
        return json.loads(os.environ.get(name) or "null")
    except Exception:
        return {"error": "not JSON", "body": (os.environ.get(name) or "")[:300]}
own, balloon, procmem = load("OWN"), load("BALLOON"), load("PROCMEM")
entry = ((balloon or {}).get("vms") or [{}])[0] if isinstance(balloon, dict) else {}
reading = entry.get("reading") or {}
st = reading.get("stats") or {}
mi = (procmem or {}).get("meminfo") or {} if isinstance(procmem, dict) else {}
def b2m(v):
    return None if v is None else round(v / MIB, 1)
def k2m(v):
    return None if v is None else round(v / 1024, 1)
rows = [
    ("available", b2m(st.get("available_memory")), k2m(mi.get("MemAvailable")), "balloon available_memory vs MemAvailable"),
    ("disk caches", b2m(st.get("disk_caches")), k2m((mi.get("Cached") or 0) + (mi.get("Buffers") or 0)) if mi else None, "balloon disk_caches vs Cached+Buffers"),
    ("total", b2m(st.get("total_memory")), k2m(mi.get("MemTotal")), "balloon total_memory vs MemTotal"),
    ("free", b2m(st.get("free_memory")), k2m(mi.get("MemFree")), "balloon free_memory vs MemFree"),
]
rss = os.environ.get("RSS_KIB", "").strip()
out = {
    "probe": "res10_balloon_stats",
    "label": os.environ.get("LABEL") or None,
    "at": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds"),
    "computer": own,
    "balloon_reason": entry.get("reason") or None,
    "balloon_detail": entry.get("detail") or None,
    "stats_polling_interval_s": reading.get("stats_polling_interval_s"),
    "balloon_observed_at": reading.get("observed_at"),
    "balloon_stats_bytes": st or None,
    "guest_meminfo_kib": mi or None,
    "guest_procmem_error": os.environ.get("PROCMEM_ERROR") or (None if mi else "guest procmem answer has no meminfo"),
    "firecracker_rss_mib": k2m(int(rss)) if rss.isdigit() else None,
    "compare_mib": [{"what": r[0], "balloon": r[1], "guest_meminfo": r[2], "pair": r[3]} for r in rows],
}
w = sys.stderr.write
w("computer %s (%s) vm %s\n" % (own.get("computer_id"), own.get("warmness_class"), own.get("vm_id")))
if out["balloon_reason"]:
    w("balloon stats UNAVAILABLE: %s %s\n" % (out["balloon_reason"], out["balloon_detail"] or ""))
else:
    w("balloon stats interval %ss, observed %s\n" % (out["stats_polling_interval_s"], out["balloon_observed_at"]))
w("%-12s %12s %14s\n" % ("MiB", "balloon", "guest meminfo"))
for r in rows:
    w("%-12s %12s %14s\n" % (r[0], "-" if r[1] is None else r[1], "-" if r[2] is None else r[2]))
w("%-12s %12s\n" % ("vmm rss", "-" if out["firecracker_rss_mib"] is None else out["firecracker_rss_mib"]))
print(json.dumps(out, indent=2))
')
printf '%s\n' "$receipt"
if [ -n "$OUT" ]; then
	printf '%s\n' "$receipt" >"$OUT"
fi
if ! printf '%s' "$receipt" | python3 -c 'import json,sys; sys.exit(1 if json.load(sys.stdin)["guest_procmem_error"] else 0)'; then
	echo "res10_balloon_stats_probe: ${procmem_error:-guest procmem answer has no meminfo}" >&2
	exit 3
fi
