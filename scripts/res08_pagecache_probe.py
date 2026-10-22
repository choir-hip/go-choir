#!/usr/bin/env python3
"""Read-only measurement for SDC RES-08 / density lever L10 (shared base OS
via virtio-pmem + DAX), before its security panel. Design and thresholds:
docs/reports/res-08-shared-base-os-pmem-dax-design-2026-10-22.md §5.

For each --vm-id it reads, from Node B over ssh:
  guest  GET /internal/debug/pprof/procmem?pagecache=1: meminfo and
         per-process RSS, joined with the page-cache residency of /nix/store
         and /mnt/persistent (mincore; never reads or faults a page; owner
         notes §40). The guest serves it on :8085 to host-sourced callers
         holding the internal service token; the public edge refuses
         /internal/*, so the read goes through Node B, with no guest shell;
and once for the host:
  host   /proc/meminfo; each firecracker process's VmRSS/RssAnon/RssFile;
         every distinct store-image inode those processes hold open
         (so jail copies show as separate inodes) and its residency in the
         host page cache (fincore, as root); KSM counters; SMT control,
         CPU vulnerability strings and EDAC (ECC) presence for the panel.

Safety: read-only. It boots, stops, writes and evicts nothing. Command lines
are never printed (only the vm-state/<id> segment is extracted). The internal
service token header file is passed to curl by path and never read here. The
guest pagecache walk costs the guest about 1 to 2 s of one core; take the
paper's read outside owner demo windows.

usage: res08_pagecache_probe.py --vm-id VM [--vm-id VM ...] [--ssh node-b]
       [--label idle-30m] [--out DIR]
"""
import argparse
import datetime as dt
import json
import os
import re
import subprocess

TOKEN_HEADER_FILE = "/var/lib/go-choir/secrets/internal-service-token.header"

HOST_READ = r'''
import glob, json, os, re, subprocess
def rd(p):
    try:
        return open(p).read()
    except Exception:
        return None
out = {}
mi = {}
for l in open("/proc/meminfo"):
    k, v = l.split(":", 1)
    mi[k] = int(v.split()[0])
out["meminfo_kib"] = {k: mi.get(k) for k in ["MemTotal", "MemAvailable", "Cached", "Shmem", "Mapped", "Active(file)", "Inactive(file)", "AnonPages"]}
out["ksm"] = {n: (rd("/sys/kernel/mm/ksm/" + n) or "").strip() for n in ["run", "pages_shared", "pages_sharing"]}
out["smt_control"] = (rd("/sys/devices/system/cpu/smt/control") or "").strip()
out["cpu_vulnerabilities"] = {os.path.basename(p): (rd(p) or "").strip() for p in sorted(glob.glob("/sys/devices/system/cpu/vulnerabilities/*"))}
out["edac_memory_controllers"] = len(glob.glob("/sys/devices/system/edac/mc/mc*"))
out["configured_store_image"] = os.path.realpath("/var/lib/go-choir/guest/storedisk.erofs")
fcs, images = [], {}
for pid in filter(str.isdigit, os.listdir("/proc")):
    if not (rd("/proc/%s/comm" % pid) or "").startswith("firecracker"):
        continue
    st = {}
    for l in (rd("/proc/%s/status" % pid) or "").splitlines():
        k = l.split(":", 1)[0]
        if k in ("VmRSS", "RssAnon", "RssFile", "RssShmem"):
            st[k + "_kib"] = int(l.split()[1])
    m = re.search(r"vm-state/([A-Za-z0-9._-]+)", (rd("/proc/%s/cmdline" % pid) or "").replace("\0", " "))
    keys = []
    for fd in glob.glob("/proc/%s/fd/*" % pid):
        try:
            t = os.readlink(fd)
            if not (t.endswith(".erofs") or "storedisk" in t):
                continue
            s = os.stat(fd)
        except OSError:
            continue
        key = "%d:%d" % (s.st_dev, s.st_ino)
        img = images.setdefault(key, {"path": t, "size": s.st_size, "fd": fd, "pids": []})
        img["pids"].append(int(pid))
        keys.append(key)
    fcs.append({"pid": int(pid), "vm": m.group(1) if m else None, "status": st, "store_inodes": sorted(set(keys))})
for key, img in images.items():
    p = subprocess.run(["fincore", "--bytes", "--noheadings", "--raw", "--output", "RES,SIZE", img.pop("fd")], capture_output=True, text=True)
    f = p.stdout.split()
    img["resident_bytes"] = int(f[0]) if len(f) >= 2 and f[0].isdigit() else None
    img["fincore_error"] = p.stderr.strip()[:200] or None
out["firecracker"] = fcs
out["store_image_inodes"] = images
print(json.dumps(out))
'''


def ssh(host, cmd, stdin=None, timeout=120):
    p = subprocess.run(["ssh", host, cmd], input=stdin, capture_output=True, text=True, timeout=timeout)
    if p.returncode != 0:
        raise RuntimeError(f"ssh {cmd[:40]}...: exit {p.returncode}: {p.stderr.strip()[:300]}")
    return p.stdout


def guest_addr(host, vm_id):
    """The guest IP from the VM's boot args (ip=<guest>::<gw>:...). Prints only the IP."""
    if not re.match(r"^[A-Za-z0-9._-]+$", vm_id):
        raise SystemExit("bad vm id")
    py = ("import json,re;c=json.load(open('/var/lib/go-choir/vm-state/%s/fc-config.json'));"
          "m=re.search(r'\\bip=([0-9.]+)::',c['boot-source']['boot_args']);print(m.group(1) if m else '')") % vm_id
    ip = ssh(host, "python3 -c \"%s\"" % py).strip()
    if not re.match(r"^[0-9.]+$", ip):
        raise SystemExit(f"could not read the guest address of {vm_id}")
    return ip


def guest_read(host, ip, name):
    auth = f"$(test -r {TOKEN_HEADER_FILE} && echo -H @{TOKEN_HEADER_FILE})"
    body = ssh(host, f"curl -s -f -m 60 -H 'X-Internal-Caller: true' {auth} 'http://{ip}:8085/internal/debug/pprof/{name}'", timeout=90)
    return json.loads(body)


def summarize(guest):
    """The numbers the design report's thresholds use, in MiB, and whether
    the store number may be quoted at all (report §5)."""
    mib = lambda b: round(b / (1 << 20), 1) if b is not None else None
    pm = guest.get("procmem", {})
    pc = pm.get("page_cache") or {}
    roots = {r["name"]: r for r in pc.get("roots", [])}
    store, upper, pers = roots.get("store", {}), roots.get("store_upper", {}), roots.get("persistent", {})
    meminfo = pc.get("meminfo") or {}
    layering = pc.get("store_layering")
    out = {
        "cached_mib": mib((meminfo.get("Cached_kib") or 0) * 1024),
        "mapped_mib": mib((meminfo.get("Mapped_kib") or 0) * 1024),
        "store_layering": layering,
        "store_resident_mib": mib(store.get("resident_bytes")),
        "store_upper_resident_mib": mib(upper.get("resident_bytes")) if upper else None,
        "persistent_resident_mib": mib(pers.get("resident_bytes")),
        "store_apparent_mib": mib(store.get("bytes")),
        "anon_mib": mib((pm.get("meminfo", {}).get("AnonPages_kib") or 0) * 1024),
    }
    floors = []
    for name, r in (("store", store), ("store_upper", upper), ("persistent", pers)):
        if not r:
            continue
        for k in ("examined_files", "skipped", "skipped_mounts", "unmeasured_files", "partial"):
            out[f"{name}_{k}"] = r.get(k, False if k == "partial" else 0)
        if r.get("partial") or r.get("skipped") or r.get("skipped_mounts") or r.get("unmeasured_files"):
            floors.append(name)
    out["floors"] = floors  # totals that are lower bounds, not measurements
    refusals = []
    unpartitioned = layering is None or not layering.get("partitioned")
    if layering is None:
        refusals.append("no store_layering: the read predates the overlay split, so the release delta may be inside the store and persistent totals")
    elif not layering.get("partitioned"):
        reason = layering.get("reason") or "no reason given"
        refusals.append(f"store_layering.partitioned is false ({reason}): the overlay upper may be counted in store and persistent")
    if store.get("unmeasured_files"):
        refusals.append(f"{store['unmeasured_files']} store files unmeasured: the daemon lacks ownership or CAP_FOWNER (Linux 5.2+ mincore)")
    if store.get("partial"):
        refusals.append("the store walk was cut short by a bound (partial)")
    if store.get("skipped") or store.get("skipped_mounts"):
        # Skipped files add nothing to bytes: the memmap cost would be
        # understated, and the saving overstated if the skipped data is cold.
        refusals.append(f"store skipped {store.get('skipped', 0)} entries and {store.get('skipped_mounts', 0)} mounts: its total is a floor in both directions")
    if not store.get("present"):
        refusals.append("no store root")
    elif not store.get("files"):
        refusals.append(f"the store walk measured no files ({store.get('examined_files', 0)} examined): a broken walk, not a store of 0 MiB")
    out["refusals"] = refusals
    out["store_quotable"] = not refusals
    if refusals:
        # Never print a store number that may not be quoted.
        for k in ("store_resident_mib", "store_upper_resident_mib", "store_apparent_mib"):
            out[k] = None
    if unpartitioned:
        # The overlay upper may be inside persistent too.
        out["persistent_resident_mib"] = None
    # The guest struct-page array for a DAX pmem region: 64 bytes per 4 KiB
    # page of the uncompressed base image (the base store's apparent bytes;
    # the release delta in store_upper is not on the image).
    if out["store_quotable"] and store.get("bytes"):
        out["dax_memmap_cost_mib"] = mib(store["bytes"] // 4096 * 64)
        out["net_saving_mib"] = round(out["store_resident_mib"] - out["dax_memmap_cost_mib"], 1)
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--vm-id", action="append", default=[])
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--label", default="", help="the scenario, e.g. idle-30m, cold-boot, busy-run")
    ap.add_argument("--out", default=".")
    ap.add_argument("--no-host", action="store_true")
    args = ap.parse_args()
    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    receipt = {"probe": "res08_pagecache", "at": stamp, "label": args.label, "guests": {}}
    for vm in args.vm_id:
        g = {}
        try:
            ip = guest_addr(args.ssh, vm)
            g["procmem"] = guest_read(args.ssh, ip, "procmem?pagecache=1")
            if g["procmem"].get("page_cache_error"):
                raise RuntimeError("page cache: " + g["procmem"]["page_cache_error"])
            g["summary"] = summarize(g)
        except Exception as e:  # noqa: BLE001
            g["error"] = str(e)[:400]
        receipt["guests"][vm] = g
    if not args.no_host:
        try:
            receipt["host"] = json.loads(ssh(args.ssh, "python3 -", stdin=HOST_READ))
        except Exception as e:  # noqa: BLE001
            receipt["host_error"] = str(e)[:400]
    os.makedirs(args.out, exist_ok=True)
    path = os.path.join(args.out, f"res08-pagecache-{args.label or 'read'}-{stamp}.json")
    with open(path, "w") as f:
        json.dump(receipt, f, indent=1)
    print(json.dumps({"receipt": path, "summaries": {k: v.get("summary") or v.get("error") for k, v in receipt["guests"].items()},
                      "host_store_inodes": {k: {kk: v[kk] for kk in ("size", "resident_bytes", "pids")} for k, v in receipt.get("host", {}).get("store_image_inodes", {}).items()}}, indent=1))


if __name__ == "__main__":
    main()
