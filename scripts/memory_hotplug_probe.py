#!/usr/bin/env python3
"""Staging proof for virtio-mem memory leases
(docs/reports/memory-hotplug-controller-design-2026-10-14.md, section 6).

Runs ONLY on a test computer: one it mints (scripts/qa_test_computer.mjs;
selected by VM_MEMORY_HOTPLUG_ONLY=class:test) or one named with
--computer-id that vmctl reports as selected (memory_hotplug_selected).

1. Baseline: firecracker host RSS, guest MemTotal (owner API
   /api/runtime/observability), vmctl's memory view.
2. Grow by --grow-mib (default 1024) through POST /internal/vmctl/memory
   (host route over ssh, an operator act) and wait for plugged == requested.
3. Check the guest sees it: MemTotal rose by at least 90% of the grow.
4. Shrink to base and check the firecracker RSS returns within
   --rss-tolerance-mib (128) of baseline within --shrink-timeout-s (120),
   and that the guest's MemTotal is back.
5. Stop at host memory PSI some avg10 > --psi-stop (1.0) at every step; on
   any stop or failure, release the lease before exiting.

The owner's key is read from the 0600 credential file the mint wrote and is
never printed. Output: one JSON receipt on stdout and in --out.

usage: memory_hotplug_probe.py --out FILE [--computer-id ID] [--grow-mib 1024]
       [--keep-computer] [--ssh node-b]

A freshly minted computer may have booted before its test classification
reached vmctl; if vmctl reports it selected but without a device, the probe
stops and starts it once through the owner lifecycle API (a cold boot picks
up the device).
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
TOKEN_HEADER_FILE = "/var/lib/go-choir/secrets/internal-service-token.header"
VMCTL_AUTH = f"-H 'X-Internal-Caller: true' $(test -r {TOKEN_HEADER_FILE} && echo -H @{TOKEN_HEADER_FILE})"
MIB = 1 << 20


def log(event, **kv):
    print(json.dumps({"t": time.strftime("%H:%M:%S", time.gmtime()), "event": event, **kv}), flush=True)


class Stop(Exception):
    pass


class Computer:
    def __init__(self, key, computer_id):
        self.key = key
        self.id = computer_id

    @classmethod
    def from_cred(cls, cred_file):
        cred = json.load(open(cred_file))
        return cls(cred.get("api_key") or cred.get("key"), cred["computer_id"])

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

    def guest_memtotal(self):
        obs = self.call("GET", "/api/runtime/observability")
        mem = obs.get("memory") or {}
        return mem.get("mem_total_bytes")


def ssh(host, cmd):
    return subprocess.run(["ssh", host, cmd], capture_output=True, text=True, timeout=600).stdout


def vmctl_json(host, method, path, body=None):
    data = ""
    if body is not None:
        data = f"-H 'Content-Type: application/json' -d '{json.dumps(body)}'"
    out = ssh(host, f"curl -s -X{method} {VMCTL_AUTH} {data} 'localhost:8083{path}'")
    try:
        return json.loads(out)
    except json.JSONDecodeError:
        return {"error": "non-json", "body": out[:400]}


def ownership(host, computer_id):
    listing = vmctl_json(host, "GET", "/internal/vmctl/list")
    for o in listing.get("ownerships") or []:
        if o.get("computer_id") == computer_id:
            return o
    return None


def fc_rss_kib(host, vm):
    # pgrep -f over ssh also matches the remote shell; keep the real process.
    out = ssh(host, f"for p in $(pgrep -f 'firecracker.*--id {vm}'); do [ \"$(cat /proc/$p/comm)\" = firecracker ] && grep VmRSS /proc/$p/status | awk '{{print $2}}'; done | head -1")
    return int(out.strip()) if out.strip().isdigit() else None


def psi_some_avg10(host):
    out = ssh(host, "cat /proc/pressure/memory")
    for line in out.splitlines():
        if line.startswith("some "):
            for field in line.split():
                if field.startswith("avg10="):
                    return float(field.split("=", 1)[1])
    return None


def check_psi(host, limit, receipt, step):
    psi = psi_some_avg10(host)
    receipt.setdefault("psi", []).append({"step": step, "some_avg10": psi})
    if psi is not None and psi > limit:
        raise Stop(f"host memory PSI some avg10 {psi} > {limit} at {step}")


def memory_view(host, computer_id):
    return vmctl_json(host, "GET", f"/internal/vmctl/memory?computer_id={computer_id}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", required=True)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--computer-id", help="an already selected test computer (owner key from --cred-file)")
    ap.add_argument("--cred-file", help="0600 credential file for --computer-id")
    ap.add_argument("--grow-mib", type=int, default=1024)
    ap.add_argument("--rss-tolerance-mib", type=int, default=128)
    ap.add_argument("--shrink-timeout-s", type=int, default=120)
    ap.add_argument("--psi-stop", type=float, default=1.0)
    ap.add_argument("--keep-computer", action="store_true")
    args = ap.parse_args()
    label = "memhp-proof-" + time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
    receipt = {"label": label, "checks": {}, "grow_mib": args.grow_mib}
    minted = False

    if args.computer_id:
        if not args.cred_file:
            raise SystemExit("--computer-id needs --cred-file (the owner key; the probe acts as the owner)")
        c = Computer.from_cred(args.cred_file)
        if c.id != args.computer_id:
            raise SystemExit("credential file is for another computer")
    else:
        mint = subprocess.run(["node", os.path.join(REPO, "scripts", "qa_test_computer.mjs"), "--label", label],
                              cwd=REPO, capture_output=True, text=True, timeout=1800)
        m = json.loads([ln for ln in mint.stdout.splitlines() if ln.strip()][-1])
        c = Computer.from_cred(m["credential_file"])
        minted = True
        receipt["build"] = m.get("guest_build_commit")
    receipt["computer_id"] = c.id
    log("computer", computer_id=c.id, minted=minted)

    own = ownership(args.ssh, c.id)
    if not own or not own.get("memory_hotplug_selected"):
        receipt["refused"] = "computer is not selected for memory hotplug (VM_MEMORY_HOTPLUG / VM_MEMORY_HOTPLUG_ONLY)"
        json.dump(receipt, open(args.out, "w"), indent=1)
        log("refused", reason=receipt["refused"], ownership=own)
        raise SystemExit(2)
    vm = own["vm_id"]
    receipt["vm_id"] = vm

    view = memory_view(args.ssh, c.id)
    if view.get("error") and minted:
        # Booted before its test classification reached vmctl: one cold boot.
        log("no_device_yet", view=view)
        c.call("POST", f"/api/computers/{c.id}/lifecycle/stop", {"idempotency_key": label + "-stop-for-device"})
        c.call("POST", f"/api/computers/{c.id}/lifecycle/start", {"idempotency_key": label + "-start-for-device"})
        c.call("GET", "/api/computer-settings")
        own = ownership(args.ssh, c.id) or {}
        vm = own.get("vm_id", vm)
        receipt["vm_id"] = vm
        view = memory_view(args.ssh, c.id)
    if view.get("error"):
        receipt["refused"] = f"no hotplug device: {view}"
        json.dump(receipt, open(args.out, "w"), indent=1)
        log("refused", reason=receipt["refused"])
        raise SystemExit(2)

    base = view["base_mib"]
    leased = False
    try:
        check_psi(args.ssh, args.psi_stop, receipt, "baseline")
        receipt["baseline"] = {
            "rss_kib": fc_rss_kib(args.ssh, vm),
            "guest_mem_total_bytes": c.guest_memtotal(),
            "view": view,
        }
        log("baseline", **{k: v for k, v in receipt["baseline"].items() if k != "view"}, base_mib=base)

        t0 = time.time()
        grow = vmctl_json(args.ssh, "POST", "/internal/vmctl/memory",
                          {"computer_id": c.id, "target_mib": base + args.grow_mib, "ttl_seconds": 600, "wait_seconds": 60})
        leased = not grow.get("error")
        receipt["grow"] = grow
        receipt["grow_ms"] = int((time.time() - t0) * 1000)
        log("grow", ms=receipt["grow_ms"], view=grow)
        if not leased:
            raise Stop(f"grow refused: {grow.get('error')}")
        check_psi(args.ssh, args.psi_stop, receipt, "grown")
        after_grow_total = c.guest_memtotal()
        receipt["grown"] = {"rss_kib": fc_rss_kib(args.ssh, vm), "guest_mem_total_bytes": after_grow_total,
                            "view": memory_view(args.ssh, c.id)}
        log("grown", rss_kib=receipt["grown"]["rss_kib"], guest_mem_total_bytes=after_grow_total)

        base_total = receipt["baseline"]["guest_mem_total_bytes"] or 0
        receipt["checks"]["plugged_equals_requested"] = grow.get("plugged_mib") == args.grow_mib
        receipt["checks"]["guest_sees_growth"] = bool(after_grow_total and base_total and
                                                      after_grow_total - base_total >= 0.9 * args.grow_mib * MIB)

        t1 = time.time()
        shrink = vmctl_json(args.ssh, "POST", "/internal/vmctl/memory",
                            {"computer_id": c.id, "target_mib": base, "wait_seconds": 60})
        leased = bool(shrink.get("error"))
        receipt["shrink"] = shrink
        log("shrink_requested", view=shrink)
        baseline_rss = receipt["baseline"]["rss_kib"] or 0
        samples = []
        returned = False
        while time.time() - t1 < args.shrink_timeout_s:
            check_psi(args.ssh, args.psi_stop, receipt, "shrinking")
            rss = fc_rss_kib(args.ssh, vm)
            v = memory_view(args.ssh, c.id)
            samples.append({"s": round(time.time() - t1, 1), "rss_kib": rss, "plugged_mib": v.get("plugged_mib"), "phase": v.get("phase")})
            if v.get("plugged_mib") == 0 and rss is not None and rss <= baseline_rss + args.rss_tolerance_mib * 1024:
                returned = True
                break
            time.sleep(5)
        receipt["shrink_samples"] = samples
        receipt["shrink_s"] = round(time.time() - t1, 1)
        final_total = c.guest_memtotal()
        receipt["final"] = {"rss_kib": fc_rss_kib(args.ssh, vm), "guest_mem_total_bytes": final_total,
                            "view": memory_view(args.ssh, c.id)}
        receipt["checks"]["unplugged_to_base"] = receipt["final"]["view"].get("plugged_mib") == 0
        receipt["checks"]["rss_within_tolerance_in_time"] = returned
        receipt["checks"]["guest_total_back"] = bool(final_total and base_total and abs(final_total - base_total) < 64 * MIB)
        log("shrunk", returned=returned, seconds=receipt["shrink_s"], final_rss_kib=receipt["final"]["rss_kib"])
    except Stop as stop:
        receipt["stopped"] = str(stop)
        log("stop", reason=str(stop))
    finally:
        if leased:
            rel = vmctl_json(args.ssh, "POST", "/internal/vmctl/memory", {"computer_id": c.id, "target_mib": 0})
            receipt["cleanup_release"] = rel
            log("released", view=rel)
        receipt["counters"] = (vmctl_json(args.ssh, "GET", "/internal/vmctl/memory") or {}).get("counters")
        receipt["pass"] = "stopped" not in receipt and bool(receipt["checks"]) and all(receipt["checks"].values())
        if minted and not args.keep_computer:
            res = c.call("POST", f"/api/computers/{c.id}/lifecycle/stop", {"idempotency_key": label + "-stop"})
            receipt["computer_stopped"] = res.get("resulting_lifecycle_state")
        json.dump(receipt, open(args.out, "w"), indent=1)
        log("receipt", checks=receipt["checks"], pass_=receipt["pass"], stopped=receipt.get("stopped"))


if __name__ == "__main__":
    main()
