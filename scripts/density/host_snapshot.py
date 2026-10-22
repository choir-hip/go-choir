#!/usr/bin/env python3
"""RES-19 / RES-07 host reading, read-only (docs/security-density-architecture-checklist-2026-10-21.md).

One JSON receipt: host meminfo (page cache, slab), memory PSI, KSM counters,
per-VM host RSS (vmctl's list: memory_rss_bytes, state, warmness class, plus
each Firecracker process's anon/file/shmem split from /proc/<pid>/status),
and with --fincore the host page-cache residency of the shared store image
and of each VM's data.img. It creates, stops and changes nothing, and reads
any computer (reads are not guarded; the creating step in density_bench.py is).

usage: host_snapshot.py [--ssh node-b | --local] [--fincore] [--out FILE]
  --ssh HOST   run the collector on HOST over ssh (default node-b)
  --local      this machine is the host (run it on Node B itself)

Reading it: the report's KSM verdict is pages_sharing=0 (vm-density-limits
finding 5); `ksm.run` 0 means KSM is off. Per-VM `memory_rss_bytes` is the
vmctl sample, `firecracker[vm].rss_anon_kib` the live split. Slab and
SReclaimable are in meminfo_kib (L19).
"""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import GIB, Host, ownerships  # noqa: E402


def summarize(snap):
    mi = snap.get("meminfo_kib", {})
    rows = []
    fc = snap.get("firecracker", {})
    for o in ownerships(snap.get("vmctl_list")):
        p = fc.get(o.get("vm_id"), {})
        rows.append({
            "computer_id": o.get("computer_id"), "vm_id": o.get("vm_id"), "state": o.get("state"),
            "warmness_class": o.get("warmness_class"), "kind": o.get("kind"),
            "vmctl_rss_mib": round((o.get("memory_rss_bytes") or 0) / 1048576),
            "fc_rss_anon_mib": round((p.get("rss_anon_kib") or 0) / 1024),
            "fc_rss_file_mib": round((p.get("rss_file_kib") or 0) / 1024),
        })
    rows.sort(key=lambda r: -r["vmctl_rss_mib"])
    return {
        "mem_available_gib": round(mi.get("MemAvailable", 0) * 1024 / GIB, 2),
        "page_cache_gib": round(mi.get("Cached", 0) * 1024 / GIB, 2),
        "slab_gib": round(mi.get("Slab", 0) * 1024 / GIB, 2),
        "slab_reclaimable_gib": round(mi.get("SReclaimable", 0) * 1024 / GIB, 2),
        "psi_some_avg10": ((snap.get("psi_memory") or {}).get("some") or {}).get("avg10"),
        "ksm_run": (snap.get("ksm") or {}).get("run"),
        "ksm_pages_sharing": (snap.get("ksm") or {}).get("pages_sharing"),
        "vms": rows,
        "vms_total_vmctl_rss_gib": round(sum(r["vmctl_rss_mib"] for r in rows) / 1024, 2),
    }


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--local", action="store_true")
    ap.add_argument("--fincore", action="store_true")
    ap.add_argument("--out")
    a = ap.parse_args()
    snap = Host(None if a.local else a.ssh).collect(*(["--fincore"] if a.fincore else []))
    receipt = {"kind": "density_host_snapshot", "summary": summarize(snap) if "error" not in snap else None, "raw": snap}
    text = json.dumps(receipt, indent=1)
    if a.out:
        with open(a.out, "w") as f:
            f.write(text + "\n")
    print(text)


if __name__ == "__main__":
    main()
