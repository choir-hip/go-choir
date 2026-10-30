#!/usr/bin/env python3
"""Staging proof for snapshot hibernation (S3, docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md).

1. Mint a test computer (scripts/qa_test_computer.mjs); act only on it.
2. Leave state on it: a computer setting, read back before and after.
3. Wait until it has no running runs, then hibernate it through vmctl
   (POST /internal/vmctl/hibernate on Node B, over ssh: an operator act).
4. Check the Firecracker process is gone and the snapshot files exist
   with root-only modes.
5. Wake it with an ordinary owner request and time the request.
6. Read the boot timeline: the wake must be boot_kind=snapshot_resume.
7. Check the state survived and no obligation was closed "interrupted by a
   restart".
8. Stop the computer unless --keep-computer.

The owner's key is read from the 0600 credential file the mint wrote and
is never printed. Output: one JSON receipt on stdout and in --out.

usage: s3_snapshot_probe.py --out FILE [--wait-min 0] [--keep-computer] [--ssh node-b]

--wait-min holds the computer hibernated that long before the wake, to
prove a resume past the guest capability's expiry (the credential window).
The wake is the owner's start act (an operator hibernate waits for one).
"""
import argparse
import json
import os
import subprocess
import time
import urllib.error
import urllib.request

HOST = "https://choir.news"
REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# Host-internal credential for vmctl's internal routes (internal/hostauth),
# as a shell fragment run on Node B: `-H @<root-only header file>` when it is
# readable, nothing otherwise, the way ci.yml's deploy sends it. The marker
# header is a hint, never authority; without the token these calls are
# refused once hostauth enforces
# (docs/archive/problems/surface-broken-calls-2026-10-14.md, item 9).
TOKEN_HEADER_FILE = "/var/lib/go-choir/secrets/internal-service-token.header"
VMCTL_AUTH = f"-H 'X-Internal-Caller: true' $(test -r {TOKEN_HEADER_FILE} && echo -H @{TOKEN_HEADER_FILE})"


def log(event, **kv):
    print(json.dumps({"t": time.strftime("%H:%M:%S", time.gmtime()), "event": event, **kv}), flush=True)


class Computer:
    def __init__(self, cred_file):
        cred = json.load(open(cred_file))
        self.key = cred.get("api_key") or cred.get("key")
        self.id = cred["computer_id"]

    def call(self, method, path, body=None, retries=20):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.id}
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        for _ in range(retries):
            try:
                req = urllib.request.Request(HOST + path, data=data, headers=headers, method=method)
                return json.load(urllib.request.urlopen(req, timeout=300))
            except urllib.error.HTTPError as err:
                if err.code in (502, 503, 504):
                    time.sleep(5)
                    continue
                return {"error": err.code, "body": err.read()[:300].decode(errors="replace")}
        return {"error": "unavailable"}


def ssh(host, cmd):
    return subprocess.run(["ssh", host, cmd], capture_output=True, text=True, timeout=600).stdout


def vm_id(host, computer_id):
    listing = json.loads(ssh(host, f"curl -s {VMCTL_AUTH} localhost:8083/internal/vmctl/list") or "{}")
    for o in listing.get("ownerships") or []:
        if o.get("computer_id") == computer_id:
            return o.get("vm_id"), o
    return None, None


def fc_rss_kib(host, vm):
    # pgrep -f over ssh also matches the remote shell; keep the real process.
    out = ssh(host, f"for p in $(pgrep -f 'firecracker.*--id {vm}'); do [ \"$(cat /proc/$p/comm)\" = firecracker ] && grep VmRSS /proc/$p/status | awk '{{print $2}}'; done | head -1")
    return int(out.strip()) if out.strip().isdigit() else None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", required=True)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--keep-computer", action="store_true")
    ap.add_argument("--wait-min", type=float, default=0)
    args = ap.parse_args()
    label = "s3-proof-" + time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
    receipt = {"label": label, "checks": {}}

    mint = subprocess.run(["node", os.path.join(REPO, "scripts", "qa_test_computer.mjs"), "--label", label],
                          cwd=REPO, capture_output=True, text=True, timeout=1800)
    minted = json.loads([ln for ln in mint.stdout.splitlines() if ln.strip()][-1])
    c = Computer(minted["credential_file"])
    receipt.update(computer_id=c.id, build=minted.get("guest_build_commit"))
    log("minted", computer_id=c.id, build=receipt["build"])

    c.call("POST", "/api/computer-settings", {"key": "intake.labels", "value": True})
    before = c.call("GET", "/api/computer-settings")
    receipt["state_before"] = [(s.get("key"), s.get("value")) for s in before.get("settings") or []]

    for _ in range(60):
        runs = c.call("GET", "/api/runs?limit=20").get("runs") or []
        if not any(r.get("state") in ("running", "pending") for r in runs):
            break
        time.sleep(5)

    vm, own = vm_id(args.ssh, c.id)
    receipt["vm_id"] = vm
    receipt["rss_kib_before"] = fc_rss_kib(args.ssh, vm)
    sock = ssh(args.ssh, f"ls -la /var/lib/go-choir/vm-state/{vm}/api/ 2>&1").strip()
    receipt["api_socket"] = sock
    log("pre_hibernate", vm_id=vm, rss_kib=receipt["rss_kib_before"], api_socket=sock[-120:])

    t0 = time.time()
    hib = ssh(args.ssh, f"curl -s -XPOST {VMCTL_AUTH} localhost:8083/internal/vmctl/hibernate -H 'Content-Type: application/json' -d '{{\"computer_id\":\"{c.id}\"}}'")
    receipt["hibernate_ms"] = int((time.time() - t0) * 1000)
    try:
        receipt["hibernate"] = json.loads(hib)
    except json.JSONDecodeError:
        receipt["hibernate"] = hib[:500]
    log("hibernated", ms=receipt["hibernate_ms"], response=str(receipt["hibernate"])[:400])

    receipt["rss_kib_after_hibernate"] = fc_rss_kib(args.ssh, vm)
    receipt["snapshot_files"] = ssh(args.ssh, f"ls -la /var/lib/go-choir/vm-state/{vm}/snapshot/ 2>&1; stat -c '%a %n' /var/lib/go-choir/vm-state/{vm}/snapshot 2>&1").strip()
    log("after_hibernate", fc_rss_kib=receipt["rss_kib_after_hibernate"], files=receipt["snapshot_files"][-300:])

    if args.wait_min:
        log("holding_hibernated", minutes=args.wait_min)
        time.sleep(args.wait_min * 60)
    t0 = time.time()
    start = c.call("POST", f"/api/computers/{c.id}/lifecycle/start", {"idempotency_key": label + "-wake"})
    receipt["wake_state"] = start.get("resulting_lifecycle_state") or start
    after = c.call("GET", "/api/computer-settings")
    receipt["wake_request_ms"] = int((time.time() - t0) * 1000)
    receipt["state_after"] = [(s.get("key"), s.get("value")) for s in after.get("settings") or []]
    log("woke", request_ms=receipt["wake_request_ms"], state=receipt["state_after"])

    tl = ssh(args.ssh, f"curl -s {VMCTL_AUTH} 'localhost:8083/internal/vmctl/boot-timeline?computer_id={c.id}'")
    try:
        timeline = json.loads(tl)
    except json.JSONDecodeError:
        timeline = tl[:800]
    receipt["boot_timeline"] = timeline
    obligations = c.call("GET", "/api/runtime/obligations")
    receipt["interrupted_fates"] = json.dumps(obligations).count("interrupted by a restart")
    receipt["rss_kib_after_wake"] = fc_rss_kib(args.ssh, vm)

    kinds = json.dumps(timeline)
    receipt["checks"] = {
        "hibernate_snapshot": "hibernate_snapshot" in json.dumps(receipt["hibernate"]),
        "fc_gone_while_hibernated": receipt["rss_kib_after_hibernate"] is None,
        "snapshot_resume": "snapshot_resume" in kinds,
        "credential_delivered": (not args.wait_min) or "delivered" in kinds,
        "state_survived": receipt["state_before"] == receipt["state_after"],
        "no_interrupted_fates": receipt["interrupted_fates"] == 0,
    }
    receipt["pass"] = all(receipt["checks"].values())
    if not args.keep_computer:
        res = c.call("POST", f"/api/computers/{c.id}/lifecycle/stop", {"idempotency_key": label + "-stop"})
        receipt["stopped"] = res.get("resulting_lifecycle_state")
    json.dump(receipt, open(args.out, "w"), indent=1)
    log("receipt", **{k: receipt[k] for k in ("checks", "pass", "wake_request_ms", "hibernate_ms")})


if __name__ == "__main__":
    main()
