#!/usr/bin/env python3
"""Staging proof for virtio-mem memory leases
(docs/archive/reports/memory-hotplug-controller-design-2026-10-14.md, sections 6
and 9.5).

Runs ONLY on a test computer: one it mints (scripts/qa_test_computer.mjs;
selected by VM_MEMORY_HOTPLUG_ONLY=class:test) or one named with
--computer-id that vmctl reports as selected (memory_hotplug_selected).

0. Guest driver: GET /api/runtime/observability must report
   memory.virtio_mem_loaded and memory.virtio_mem_bound.
1. Device check (host route over ssh, an operator act): grow by --grow-mib
   (1024), wait for plugged == requested, check the guest's MemTotal rose by
   at least 90% of it, shrink to base, check plugged returns to 0 and
   MemTotal comes back. Plugged memory the guest never touches never enters
   the firecracker RSS, so this phase claims nothing about RSS.
2. Job check (the real one): a prompt-bar run asks engineering to start a
   capsule job with memory_mib --job-memory-mib (1024) that allocates and
   touches --touch-mib (768) and holds it for two minutes. The guest takes
   the lease itself. Pass only if vmctl shows a guest lease with plugged
   memory >= --touch-mib, the firecracker RSS rose by at least
   --min-rss-rise-mib (600) over the last sample before the lease (the guest
   used the memory), and after the job plugged returns to 0 and the RSS
   returns within --rss-tolerance-mib (128) of that pre-lease sample within
   --shrink-timeout-s (120) of the lease ending.
3. Stop at host memory PSI some avg10 > --psi-stop (1.0) at every sample;
   on any stop or failure, release the lease before exiting.

The owner's key is read from the 0600 credential file the mint wrote and is
never printed. Output: one JSON receipt on stdout and in --out.

usage: memory_hotplug_probe.py --out FILE [--computer-id ID --cred-file F]
       [--grow-mib 1024] [--skip-device-check] [--skip-job-check]
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
import urllib.parse
import urllib.request

HOST = "https://choir.news"
REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TOKEN_HEADER_FILE = "/var/lib/go-choir/secrets/internal-service-token.header"
VMCTL_AUTH = f"-H 'X-Internal-Caller: true' $(test -r {TOKEN_HEADER_FILE} && echo -H @{TOKEN_HEADER_FILE})"
MIB = 1 << 20
TERMINAL = {"completed", "failed", "cancelled", "blocked"}


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
                return {"error": err.code, "path": path.split("?")[0]}
            except urllib.error.URLError:
                time.sleep(5)
        return {"error": "unavailable"}

    def guest_memory(self):
        obs = self.call("GET", "/api/runtime/observability")
        return (obs.get("memory") or {}) if isinstance(obs, dict) else {}

    def guest_memtotal(self):
        return self.guest_memory().get("mem_total_bytes")


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
    return psi


def memory_view(host, computer_id):
    return vmctl_json(host, "GET", f"/internal/vmctl/memory?computer_id={computer_id}")


def release(host, computer_id):
    return vmctl_json(host, "POST", "/internal/vmctl/memory", {"computer_id": computer_id, "target_mib": 0})


def device_check(args, c, vm, base, receipt, state):
    """Phase 1: the host grows and shrinks; the guest sees it."""
    check_psi(args.ssh, args.psi_stop, receipt, "device_baseline")
    base_total = c.guest_memtotal() or 0
    d = receipt["device"] = {"baseline": {"rss_kib": fc_rss_kib(args.ssh, vm), "guest_mem_total_bytes": base_total}}
    t0 = time.time()
    grow = vmctl_json(args.ssh, "POST", "/internal/vmctl/memory",
                      {"computer_id": c.id, "target_mib": base + args.grow_mib, "ttl_seconds": 600, "wait_seconds": 60})
    state["leased"] = not grow.get("error")
    d["grow"], d["grow_ms"] = grow, int((time.time() - t0) * 1000)
    log("device_grow", ms=d["grow_ms"], view=grow)
    if not state["leased"]:
        raise Stop(f"grow refused: {grow.get('error')}")
    check_psi(args.ssh, args.psi_stop, receipt, "device_grown")
    grown_total = c.guest_memtotal()
    d["grown"] = {"rss_kib": fc_rss_kib(args.ssh, vm), "guest_mem_total_bytes": grown_total}
    receipt["checks"]["device_plugged_equals_requested"] = grow.get("plugged_mib") == args.grow_mib
    receipt["checks"]["device_guest_sees_growth"] = bool(grown_total and base_total and
                                                         grown_total - base_total >= 0.9 * args.grow_mib * MIB)
    t1 = time.time()
    shrink = vmctl_json(args.ssh, "POST", "/internal/vmctl/memory", {"computer_id": c.id, "target_mib": base, "wait_seconds": 60})
    state["leased"] = bool(shrink.get("error"))
    d["shrink"] = shrink
    v = shrink
    while time.time() - t1 < args.shrink_timeout_s and v.get("plugged_mib") != 0:
        check_psi(args.ssh, args.psi_stop, receipt, "device_shrinking")
        time.sleep(5)
        v = memory_view(args.ssh, c.id)
    d["shrink_s"] = round(time.time() - t1, 1)
    final_total = c.guest_memtotal()
    d["final"] = {"rss_kib": fc_rss_kib(args.ssh, vm), "guest_mem_total_bytes": final_total, "view": v}
    receipt["checks"]["device_unplugged_to_base"] = v.get("plugged_mib") == 0
    receipt["checks"]["device_guest_total_back"] = bool(final_total and base_total and abs(final_total - base_total) < 64 * MIB)
    log("device_done", **{k: receipt["checks"][k] for k in receipt["checks"] if k.startswith("device_")})


def job_brief(stamp, touch_mib, need_mib):
    script = f"import time; b = b'x' * ({touch_mib} << 20); time.sleep(120)"
    return (
        f"Engineering probe {stamp} for memory leases. Engineering: in one cell start one capsule job and nothing else: "
        f'id, err := choir.Start([]string{{"python3", "-c", "{script}"}}, map[string]any{{"memory_mib": {need_mib}, "minutes": 5}}); '
        f"println(id, err). If it is refused, Complete with result failed quoting the refusal. Otherwise end your turn; "
        f"when you are woken with its end, read choir.Poll(id) and Complete with result completed naming the job id, its state "
        f"and its receipt_ref. Do not Freeze: this is a probe, not a change to land."
    )


def engineering_run(c, tid):
    """The newest engineering run serving the prompt-bar trajectory (best effort, for the receipt)."""
    q = urllib.parse.quote(tid, safe="")
    runs = (c.call("GET", f"/api/runs?trajectory_id={q}&limit=200").get("runs") or []) + \
           (c.call("GET", "/api/runs?limit=200").get("runs") or [])
    best = None
    for r in runs:
        meta = r.get("metadata") or {}
        traj = r.get("trajectory_id") or meta.get("trajectory_id") or meta.get("assignment_trajectory_id")
        eng = r.get("agent_profile") == "engineering" or str(r.get("agent_id") or "").startswith("engineering:")
        if eng and traj == tid and (best is None or str(r.get("created_at") or "") > str(best.get("created_at") or "")):
            best = r
    return best


def job_check(args, c, vm, base, receipt, state):
    """Phase 2: a capsule job takes the lease and touches the memory."""
    stamp = time.strftime("%Y%m%dT%H%M%S", time.gmtime())
    j = receipt["job"] = {"touch_mib": args.touch_mib, "memory_mib": args.job_memory_mib}
    started = c.call("POST", "/api/prompt-bar", {"text": job_brief(stamp, args.touch_mib, args.job_memory_mib),
                                                 "command_id": f"memory-hotplug-probe-{stamp}"})
    tid = started.get("trajectory_id") if isinstance(started, dict) else None
    j["prompt_bar"] = {k: started.get(k) for k in ("trajectory_id", "doc_id", "submission_id", "error") if isinstance(started, dict)}
    log("job_prompt", **j["prompt_bar"])
    if not tid:
        raise Stop("the prompt bar did not return a trajectory")

    deadline = time.time() + args.job_timeout_min * 60
    pre_lease_rss = None
    lease_seen_at = lease_ended_at = None
    peak_rss = max_plugged = 0
    samples = []
    returned = False
    while time.time() < deadline:
        check_psi(args.ssh, args.psi_stop, receipt, "job")
        rss = fc_rss_kib(args.ssh, vm)
        v = memory_view(args.ssh, c.id)
        lease = v.get("lease") or {}
        plugged = v.get("plugged_mib") or 0
        now = round(time.time(), 1)
        samples.append({"t": now, "rss_kib": rss, "plugged_mib": plugged, "phase": v.get("phase"), "holder": lease.get("holder")})
        if lease_seen_at is None:
            if lease.get("holder") == "guest":
                lease_seen_at = now
                state["leased"] = True
                log("job_lease", plugged_mib=plugged, pre_lease_rss_kib=pre_lease_rss)
            elif not lease and plugged == 0 and rss is not None:
                pre_lease_rss = rss
        if lease_seen_at is not None:
            max_plugged = max(max_plugged, plugged)
            peak_rss = max(peak_rss, rss or 0)
            if lease_ended_at is None and not lease:
                lease_ended_at = now
                state["leased"] = plugged > 0
                log("job_lease_ended", plugged_mib=plugged, rss_kib=rss)
            if lease_ended_at is not None:
                if plugged == 0:
                    state["leased"] = False
                if plugged == 0 and rss is not None and pre_lease_rss is not None and \
                        rss <= pre_lease_rss + args.rss_tolerance_mib * 1024:
                    returned = True
                    break
                if now - lease_ended_at > args.shrink_timeout_s:
                    break
        elif len(samples) % 10 == 0:
            run = engineering_run(c, tid)
            if run and run.get("state") in TERMINAL:
                j["engineering_run"] = {"run_id": run.get("run_id"), "state": run.get("state")}
                raise Stop(f"the engineering run ended {run.get('state')} before any guest lease appeared")
        time.sleep(3)
    j["samples"] = samples
    j["pre_lease_rss_kib"], j["peak_rss_kib"], j["max_plugged_mib"] = pre_lease_rss, peak_rss, max_plugged
    j["lease_seen_at"], j["lease_ended_at"] = lease_seen_at, lease_ended_at
    if lease_ended_at is not None:
        j["shrink_s"] = round(samples[-1]["t"] - lease_ended_at, 1)
    run = engineering_run(c, tid)
    if run:
        j["engineering_run"] = {"run_id": run.get("run_id"), "state": run.get("state"),
                                "assignment_id": (run.get("metadata") or {}).get("assignment_id")}
    checks = receipt["checks"]
    checks["job_guest_lease"] = lease_seen_at is not None
    checks["job_plugged_covers_touch"] = max_plugged >= args.touch_mib
    checks["job_guest_used_memory"] = bool(pre_lease_rss and peak_rss - pre_lease_rss >= args.min_rss_rise_mib * 1024)
    checks["job_lease_released"] = lease_ended_at is not None
    checks["job_unplugged_to_base"] = bool(samples) and samples[-1]["plugged_mib"] == 0
    checks["job_rss_returned_in_time"] = returned
    log("job_done", **{k: v for k, v in checks.items() if k.startswith("job_")})


def parse_mint(mint):
    """The minted computer's JSON (last non-empty stdout line), or a loud stop."""
    lines = [ln for ln in mint.stdout.splitlines() if ln.strip()]
    tail = (mint.stderr or "").strip()[-300:]
    if mint.returncode != 0:
        raise SystemExit(f"mint failed: qa_test_computer.mjs exited {mint.returncode}; stderr: {tail}")
    if not lines:
        raise SystemExit(f"mint failed: qa_test_computer.mjs printed nothing; stderr: {tail}")
    try:
        m = json.loads(lines[-1])
    except ValueError:
        raise SystemExit(f"mint failed: last output line is not JSON: {lines[-1][:200]!r}")
    if not isinstance(m, dict) or not m.get("credential_file"):
        raise SystemExit(f"mint failed: output has no credential_file: {lines[-1][:200]!r}")
    return m


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", required=True)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--computer-id", help="an already selected test computer (owner key from --cred-file)")
    ap.add_argument("--cred-file", help="0600 credential file for --computer-id")
    ap.add_argument("--grow-mib", type=int, default=1024)
    ap.add_argument("--job-memory-mib", type=int, default=1024)
    ap.add_argument("--touch-mib", type=int, default=768)
    ap.add_argument("--min-rss-rise-mib", type=int, default=600)
    ap.add_argument("--rss-tolerance-mib", type=int, default=128)
    ap.add_argument("--shrink-timeout-s", type=int, default=120)
    ap.add_argument("--job-timeout-min", type=int, default=30)
    ap.add_argument("--psi-stop", type=float, default=1.0)
    ap.add_argument("--skip-device-check", action="store_true")
    ap.add_argument("--skip-job-check", action="store_true")
    ap.add_argument("--keep-computer", action="store_true")
    args = ap.parse_args()
    label = "memhp-proof-" + time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
    receipt = {"label": label, "checks": {}}
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
        m = parse_mint(mint)
        c = Computer.from_cred(m["credential_file"])
        minted = True
        receipt["build"] = m.get("guest_build_commit")
    receipt["computer_id"] = c.id
    log("computer", computer_id=c.id, minted=minted)

    def refuse(reason, **kv):
        receipt["refused"] = reason
        json.dump(receipt, open(args.out, "w"), indent=1)
        log("refused", reason=reason, **kv)
        raise SystemExit(2)

    own = ownership(args.ssh, c.id)
    if not own or not own.get("memory_hotplug_selected"):
        refuse("computer is not selected for memory hotplug (VM_MEMORY_HOTPLUG / VM_MEMORY_HOTPLUG_ONLY)", ownership=own)
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
        refuse(f"no hotplug device: {view}")
    base = view["base_mib"]
    receipt["base_mib"] = base

    mem = c.guest_memory()
    receipt["guest_driver"] = {k: mem.get(k) for k in ("virtio_mem_loaded", "virtio_mem_bound", "mem_total_bytes")}
    receipt["checks"]["virtio_mem_loaded"] = bool(mem.get("virtio_mem_loaded"))
    receipt["checks"]["virtio_mem_bound"] = bool(mem.get("virtio_mem_bound"))
    log("guest_driver", **receipt["guest_driver"])

    state = {"leased": False}
    try:
        if not (receipt["checks"]["virtio_mem_loaded"] and receipt["checks"]["virtio_mem_bound"]):
            raise Stop("the guest's virtio_mem driver is not loaded and bound")
        if not args.skip_device_check:
            device_check(args, c, vm, base, receipt, state)
        if not args.skip_job_check:
            job_check(args, c, vm, base, receipt, state)
    except Stop as stop:
        receipt["stopped"] = str(stop)
        log("stop", reason=str(stop))
    finally:
        if state["leased"] or (memory_view(args.ssh, c.id).get("plugged_mib") or 0) > 0:
            receipt["cleanup_release"] = release(args.ssh, c.id)
            log("released", view=receipt["cleanup_release"])
        receipt["counters"] = (vmctl_json(args.ssh, "GET", "/internal/vmctl/memory") or {}).get("counters")
        receipt["pass"] = "stopped" not in receipt and bool(receipt["checks"]) and all(receipt["checks"].values())
        if minted and not args.keep_computer:
            res = c.call("POST", f"/api/computers/{c.id}/lifecycle/stop", {"idempotency_key": label + "-stop"})
            receipt["computer_stopped"] = res.get("resulting_lifecycle_state")
        json.dump(receipt, open(args.out, "w"), indent=1)
        log("receipt", checks=receipt["checks"], pass_=receipt["pass"], stopped=receipt.get("stopped"))


if __name__ == "__main__":
    main()
