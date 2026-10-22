#!/usr/bin/env python3
"""RES-19 density benchmark, step 1 (the idle ladder) of
docs/reports/vm-density-limits-2026-10-14.md §4. Creates test computers.

Without --create-vms this prints the plan and the preflight (host state, the
protected computers it will not touch) and exits; it creates nothing.

With --create-vms it mints test computers one at a time with
scripts/qa_test_computer.mjs (a fresh staging owner and computer per rung
step), and for each rung N in --ladder samples at --marks minutes after the
last mint: host RSS split into anon and file, guest procmem (Cached,
AnonPages, Slab), and idle CPU per VM from /proc/<pid>/stat deltas between
consecutive samples. Run it only in a quiet window, with no deploy and no
owner activity; at most one boot is in flight.

Guards (lib.refusal): a computer is touched only if vmctl classifies it as
warmness class "test"; the paper, the owner computer and the delegate
computer-ad1b46d4c20129e84b9e239cf4720fb9 are refused, as is any id given
with --protect-id. A minted computer that does not classify as test ends the
run. Safety stops, polled every --poll-s and before every mint: memory PSI
some avg10 > 1.0; MemAvailable below --floor-gib + 2 GiB; a non-test
computer that was active leaving the active state; host CPU above 80% for 60
s. On any stop, or at the end, every minted computer is stopped (never
removed) and its id recorded in the receipt.

Not scripted here (they need a fixed workload or an announced window):
step 2 hibernate and wake waves, step 3 busy ladder, step 4 deploy storm.

usage: density_bench.py [--ssh node-b | --local] [--create-vms]
         [--ladder 1,2,4,8] [--marks 3,10,30] [--max-n 20] [--floor-gib 4.67]
         [--protect-id COMPUTER_ID ...] --out DIR
"""
import argparse
import json
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import Host, non_test_active, ownerships, refusal, require_test, safety_stop  # noqa: E402

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def log(event, **kv):
    print(json.dumps({"t": time.strftime("%H:%M:%S", time.gmtime()), "event": event, **kv}), flush=True)


def find(listing, computer_id):
    return next((o for o in ownerships(listing) if o.get("computer_id") == computer_id), None)


class Stop(Exception):
    pass


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--local", action="store_true")
    ap.add_argument("--create-vms", action="store_true", help="actually mint test computers")
    ap.add_argument("--ladder", default="1,2,4,8")
    ap.add_argument("--marks", default="3,10,30", help="minutes after the last mint of a rung")
    ap.add_argument("--max-n", type=int, default=20)
    ap.add_argument("--floor-gib", type=float, default=4.67, help="vmctl's host memory floor (report: 4.67 on Node B)")
    ap.add_argument("--poll-s", type=int, default=30)
    ap.add_argument("--protect-id", action="append", default=[])
    ap.add_argument("--run-id", default=time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()))
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    host = Host(None if a.local else a.ssh)
    os.makedirs(a.out, exist_ok=True)
    ladder = [n for n in (int(x) for x in a.ladder.split(",")) if n <= a.max_n]
    marks = [int(x) for x in a.marks.split(",")]

    pre = host.collect("--cpu-window", "2")
    protected = [{"computer_id": o.get("computer_id"), "class": o.get("warmness_class"), "why": refusal(o, a.protect_id)}
                 for o in ownerships(pre.get("vmctl_list")) if refusal(o, a.protect_id)]
    log("preflight", ladder=ladder, marks_min=marks, create_vms=a.create_vms,
        stop_now=safety_stop(pre, a.floor_gib), protected_not_touched=len(protected))
    if not a.create_vms:
        log("dry_run", note="pass --create-vms to mint test computers; nothing was created")
        return
    if safety_stop(pre, a.floor_gib):
        raise SystemExit("refused to start: " + safety_stop(pre, a.floor_gib))

    active_before = non_test_active(pre.get("vmctl_list"), a.protect_id)
    minted, samples, hot_polls = [], [], [0]
    receipt = {"kind": "density_idle_ladder", "run_id": a.run_id, "host": a.ssh if not a.local else "local",
               "floor_gib": a.floor_gib, "minted": minted, "samples": samples, "stop": None, "stopped": []}

    def check(snap):
        why = safety_stop(snap, a.floor_gib)
        if not why and snap.get("host_cpu_busy_fraction", 0) > 0.8:
            hot_polls[0] += 1
            if hot_polls[0] * a.poll_s >= 60:
                why = "host CPU above 80% for 60 s"
        elif not why:
            hot_polls[0] = 0
        lost = active_before - {o["computer_id"] for o in ownerships(snap.get("vmctl_list")) if o.get("state") == "active"}
        if not why and lost:
            why = f"non-test computer(s) left the active state: {sorted(lost)}"
        if why:
            raise Stop(why)

    def wait_until(deadline):
        while time.time() < deadline:
            time.sleep(min(a.poll_s, max(1, deadline - time.time())))
            check(host.collect("--light", "--cpu-window", "2"))

    def mint(i):
        label = f"density-{a.run_id}-{i}".lower()
        r = subprocess.run(["node", os.path.join(REPO, "scripts/qa_test_computer.mjs"), "--label", label],
                           capture_output=True, text=True, cwd=REPO, timeout=900)
        try:
            info = json.loads(r.stdout.strip().splitlines()[-1])
        except (ValueError, IndexError):
            raise Stop(f"mint failed: {(r.stderr or r.stdout)[-200:]}")
        own = None
        for _ in range(20):
            own = find(host.vmctl("GET", "/internal/vmctl/list"), info["computer_id"])
            if own:
                break
            time.sleep(3)
        if not own:
            raise Stop(f"minted {info['computer_id']} is not in vmctl's list")
        why = refusal(own, a.protect_id)
        minted.append({"computer_id": info["computer_id"], "label": label, "vm_id": own.get("vm_id"),
                       "user_id": own.get("user_id"), "desktop_id": own.get("desktop_id"), "refusal": why})
        if why:
            raise Stop(f"minted computer is not safe to ladder: {why}")
        log("minted", computer_id=info["computer_id"], vm_id=own.get("vm_id"), n=len(minted))

    prev_ticks = {}
    try:
        for n in ladder:
            while len(minted) < n:
                check(host.collect("--light", "--cpu-window", "2"))
                mint(len(minted) + 1)
            t_mint = time.time()
            for m in marks:
                wait_until(t_mint + m * 60)
                vms = ",".join(x["vm_id"] for x in minted)
                snap = host.collect("--procmem", vms, "--cpu-window", "5")
                check(snap)
                fc = snap.get("firecracker") or {}
                row = {"n": n, "mark_min": m, "at": snap.get("at"), "meminfo_kib": snap.get("meminfo_kib"),
                       "psi_memory": snap.get("psi_memory"), "ksm": snap.get("ksm"), "vms": {}}
                for x in minted:
                    p = fc.get(x["vm_id"], {})
                    pm = (snap.get("guest_procmem") or {}).get(x["vm_id"], {})
                    mi = pm.get("meminfo") or {}
                    row["vms"][x["computer_id"]] = {
                        "rss_kib": p.get("rss_kib"), "rss_anon_kib": p.get("rss_anon_kib"), "rss_file_kib": p.get("rss_file_kib"),
                        "idle_cpu_cores_5s": p.get("cpu_cores_over_window"),
                        "guest_cached_kib": mi.get("Cached_kib"), "guest_anon_kib": mi.get("AnonPages_kib"),
                        "guest_slab_kib": mi.get("Slab_kib"), "guest_procmem_error": pm.get("error")}
                samples.append(row)
                log("sample", n=n, mark_min=m, host_mem_available_kib=(snap.get("meminfo_kib") or {}).get("MemAvailable"))
    except Stop as e:
        receipt["stop"] = str(e)
        log("stop", reason=str(e))
    finally:
        for x in minted:
            own = find(host.vmctl("GET", "/internal/vmctl/list"), x["computer_id"])
            if own and not refusal(own, a.protect_id):
                r = host.vmctl("POST", "/internal/vmctl/stop", {"user_id": own["user_id"], "desktop_id": own.get("desktop_id", "")})
                receipt["stopped"].append({"computer_id": x["computer_id"], "result": r})
        with open(os.path.join(a.out, f"idle-ladder-{a.run_id}.json"), "w") as f:
            json.dump(receipt, f, indent=1)
        log("done", minted=[x["computer_id"] for x in minted], stop=receipt["stop"],
            receipt=os.path.join(a.out, f"idle-ladder-{a.run_id}.json"))


if __name__ == "__main__":
    main()
