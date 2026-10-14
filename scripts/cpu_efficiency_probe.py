#!/usr/bin/env python3
"""CPU per unit of agent work on one computer (station LR / SA benchmark seed).

Over one window it joins three reads:

  1. Host split (read-only, over SSH): scripts/cpu_host_sample.py run on the
     host for the window: the computer's vCPU guest/sys time, KVM halt-poll
     time, the Firecracker VMM thread, and the host services.
  2. Guest split (optional, --profile): a Go CPU profile of the guest's main
     autoputer process over the same window, through the existing
     host-sourced route /internal/debug/pprof/profile (the guest serves it to
     the host gateway address only; the public edge refuses /internal/*).
     Desk-session workers are separate processes and are not in it: guest
     time minus the main process's profiled time is workers + guest kernel
     threads + other guest processes.
  3. Units of work over the window from the product route GET /api/costs
     (model calls and tokens), with the computer's QA credential.

It prints one JSON document: cores by part, model calls, and CPU-seconds per
model call. With --profile it also writes the profile and `go tool pprof
-top` text under --out.

Safety: read-only. It never changes settings, schedules or model policy and
never boots anything. A CPU profile costs the guest a few percent of CPU for
its window: take it outside owner demo windows. The credential file is read
for api_key and computer_id; the key is never printed.

usage: cpu_efficiency_probe.py --cred FILE --vm-id VM [--ssh node-b]
       [--seconds 120] [--profile] [--out DIR]
"""
import argparse
import datetime as dt
import json
import os
import re
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))


def ssh(host, cmd, stdin_path=None, timeout=None, out_path=None):
    stdin = open(stdin_path, "rb") if stdin_path else subprocess.DEVNULL
    stdout = open(out_path, "wb") if out_path else subprocess.PIPE
    try:
        p = subprocess.run(["ssh", "-o", "ConnectTimeout=15", host, cmd], stdin=stdin,
                           stdout=stdout, stderr=subprocess.PIPE, timeout=timeout)
    finally:
        if stdin_path:
            stdin.close()
        if out_path:
            stdout.close()
    if p.returncode != 0:
        raise SystemExit(f"ssh {host}: exit {p.returncode}: {p.stderr.decode()[:200]}")
    return p.stdout


def guest_addr(host, vm_id):
    """The guest IP from the VM's boot args (ip=<guest>::<gw>:...). Prints only the IP."""
    py = ("import json,re,sys;c=json.load(open('/var/lib/go-choir/vm-state/%s/fc-config.json'));"
          "m=re.search(r'\\bip=([0-9.]+)::',c['boot-source']['boot_args']);print(m.group(1) if m else '')") % vm_id
    return ssh(host, "python3 -c \"%s\"" % py).decode().strip()


def costs(cred, start, end, limit):
    key = cred.get("api_key") or cred.get("key")
    base = (cred.get("base_url") or "https://choir.news").rstrip("/")
    q = urllib.parse.urlencode({"from": start, "to": end, "limit": limit, "detail": "1"})
    req = urllib.request.Request(base + "/api/costs?" + q, headers={
        "Authorization": "Bearer " + key, "X-Choir-Computer": cred["computer_id"]})
    try:
        d = json.load(urllib.request.urlopen(req, timeout=180))
    except urllib.error.HTTPError as err:
        raise SystemExit(f"GET /api/costs: HTTP {err.code}") from None
    s = d.get("summary") or {}
    entries = d.get("entries") or []
    # /api/costs scans the newest `limit` events of the owner and then filters
    # by time, so a window older than those events reads as zero calls (the
    # paper wrote more than 10,000 events an hour on 2026-10-14). Count recent
    # windows only.
    stamps = sorted(str(e.get("Timestamp") or e.get("timestamp") or "") for e in entries)
    models = {}
    for e in entries:
        m = e.get("Model") or e.get("model") or "?"
        models[m] = models.get(m, 0) + 1
    return {"call_count": s.get("call_count"), "input_tokens": s.get("total_input_tokens"),
            "output_tokens": s.get("total_output_tokens"),
            "cached_input_tokens": s.get("total_cached_input_tokens"),
            "first_call": stamps[0] if stamps else None, "last_call": stamps[-1] if stamps else None,
            "calls_by_model": models}


def pprof_top(path, cum=False):
    args = ["go", "tool", "pprof", "-top", "-nodecount=60"] + (["-cum"] if cum else []) + [path]
    p = subprocess.run(args, capture_output=True, text=True)
    return p.stdout + p.stderr


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--cred", required=True)
    ap.add_argument("--vm-id", required=True)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--seconds", type=int, default=120)
    ap.add_argument("--profile", action="store_true", help="also take a guest CPU profile over the window")
    ap.add_argument("--costs-limit", type=int, default=5000)
    ap.add_argument("--out", default=".")
    args = ap.parse_args()
    cred = json.load(open(args.cred))
    os.makedirs(args.out, exist_ok=True)
    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")

    prof_path = os.path.join(args.out, f"guest-cpu-{stamp}.pb.gz")
    threads, errs = [], []
    if args.profile:
        ip = guest_addr(args.ssh, args.vm_id)
        if not re.match(r"^[0-9.]+$", ip):
            raise SystemExit("could not read the guest address")

        def take():
            try:
                url = f"http://{ip}:8085/internal/debug/pprof/profile?seconds={args.seconds}"
                ssh(args.ssh, f"curl -s -f -m {args.seconds + 60} '{url}'", timeout=args.seconds + 90, out_path=prof_path)
            except BaseException as e:  # noqa: BLE001
                errs.append(f"profile: {e}")
        t = threading.Thread(target=take)
        t.start()
        threads.append(t)

    start = dt.datetime.now(dt.timezone.utc)
    host = json.loads(ssh(args.ssh, f"python3 - {args.seconds}", stdin_path=os.path.join(HERE, "cpu_host_sample.py"),
                          timeout=args.seconds + 120))
    end = dt.datetime.now(dt.timezone.utc)
    for t in threads:
        t.join()

    vm = host["vms"].get(args.vm_id)
    if not vm:
        raise SystemExit(f"{args.vm_id} is not running")
    th = vm["threads"]
    vcpu = [v for k, v in th.items() if "vcpu" in k]
    vmm = [v for k, v in th.items() if k.startswith("firecracker#")]
    kvm = vm["kvm_per_s"]
    split = {
        "vcpu_guest_cores": round(sum(v["guest"] for v in vcpu), 3),
        "vcpu_sys_cores": round(sum(v["sys"] for v in vcpu), 3),
        "kvm_halt_poll_cores": round(kvm.get("halt_poll_success_cores", 0) + kvm.get("halt_poll_fail_cores", 0), 3),
        "vmm_thread_cores": round(sum(v["user"] + v["sys"] for v in vmm), 4),
        "vm_total_cores": round(sum(v["user"] + v["sys"] for v in vcpu + vmm), 3),
        "kvm_exits_per_s": kvm.get("exits"), "halt_exits_per_s": kvm.get("halt_exits"),
        "disk_write_bytes_per_s": vm["io_per_s"].get("write_bytes"),
        "host_services_cores": host["services_cores"],
        "host_busy_cores": host["host_busy_cores"],
    }
    work = costs(cred, start.strftime("%Y-%m-%dT%H:%M:%SZ"), end.strftime("%Y-%m-%dT%H:%M:%SZ"), args.costs_limit)
    out = {"vm_id": args.vm_id, "window": [start.isoformat(), end.isoformat()], "seconds": host["window_s"],
           "host_split": split, "work": work, "errors": errs}
    calls = work.get("call_count") or 0
    if calls:
        out["cpu_seconds_per_model_call"] = round(split["vm_total_cores"] * host["window_s"] / calls, 2)
    if args.profile and os.path.exists(prof_path) and os.path.getsize(prof_path) > 0:
        top = pprof_top(prof_path)
        open(prof_path.replace(".pb.gz", ".top.txt"), "w").write(top)
        open(prof_path.replace(".pb.gz", ".cum.txt"), "w").write(pprof_top(prof_path, cum=True))
        m = re.search(r"Duration: ([0-9.]+)s, Total samples = ([0-9.]+)(ms|s)", top)
        if m:
            dur = float(m.group(1))
            samples = float(m.group(2)) / (1000.0 if m.group(3) == "ms" else 1.0)
            out["guest_main_process_cores"] = round(samples / dur, 3)
            out["guest_other_cores"] = round(split["vcpu_guest_cores"] - samples / dur, 3)
        out["profile"] = prof_path
    json.dump(out, sys.stdout, indent=1)
    print()


if __name__ == "__main__":
    main()
