#!/usr/bin/env python3
"""RES-07 guest-side reading, read-only: what fills a guest's memory.

For each named vm, the host reads the guest daemon's host-sourced route
GET :8085/internal/debug/pprof/procmem (internal/server/debug_procmem.go; the
host sends the internal token header, never printed). It returns the guest's
MemTotal/MemFree/MemAvailable/Buffers/Cached/Shmem/Slab/AnonPages and every
process's RSS with its anon/file split, grouped by name (main daemon, desk
workers, dolt).

THE GAP (RES-07's `fincore` read): no existing host-to-guest route reads page
cache residency per file. procmem gives Cached in total and RssFile per
process, not "nix store pages versus Dolt store pages". The guest has no ssh
and the lane rules forbid guest block-device access. What this script gives
instead is the correlational reading: guest `Cached` against the host's
`fincore` of the store image and of the VM's data.img (host_snapshot.py
--fincore). Closing the gap is an orange change to the guest daemon (a
procmem field that calls mincore over the store mount and the Dolt directory
and reports resident bytes per tree), shipped in a guest image. Without it,
RES-07 can be answered only to within "Cached, of which the host also holds
this much of the store image".

usage: guest_memory.py --vm VM_ID [--vm ...] [--ssh node-b | --local] [--fincore] [--out FILE]
"""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import Host  # noqa: E402


def mib(kib):
    return round((kib or 0) / 1024, 1)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--vm", action="append", required=True)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--local", action="store_true")
    ap.add_argument("--fincore", action="store_true", help="also the host fincore of the store image and data.img files")
    ap.add_argument("--out")
    a = ap.parse_args()
    flags = ["--procmem", ",".join(a.vm)]
    if a.fincore:
        flags.append("--fincore")
    snap = Host(None if a.local else a.ssh).collect(*flags)
    guests = {}
    for vm, pm in (snap.get("guest_procmem") or {}).items():
        mi = pm.get("meminfo") or {}
        guests[vm] = {
            "error": pm.get("error"),
            "meminfo_mib": {k[:-4]: mib(v) for k, v in mi.items()},
            "cached_mib": mib(mi.get("Cached_kib")),
            "anon_mib": mib(mi.get("AnonPages_kib")),
            "slab_mib": mib(mi.get("Slab_kib")),
            "by_name_rss_mib": {n: {"count": g["count"], "rss_mib": mib(g["rss_kib"])} for n, g in (pm.get("by_name") or {}).items()},
            "fc_rss_mib": mib(((snap.get("firecracker") or {}).get(vm) or {}).get("rss_kib")),
            "host_fincore_data_image": ((snap.get("host_fincore") or {}).get("data_images") or {}).get(vm),
        }
    receipt = {"kind": "density_guest_memory", "gap": "no guest fincore route; Cached is total, not store versus Dolt (see docstring)",
               "host_fincore_store_image": (snap.get("host_fincore") or {}).get("store_image"),
               "guests": guests, "host_meminfo_kib": snap.get("meminfo_kib")}
    text = json.dumps(receipt, indent=1)
    if a.out:
        with open(a.out, "w") as f:
            f.write(text + "\n")
    print(text)


if __name__ == "__main__":
    main()
