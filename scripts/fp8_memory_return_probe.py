#!/usr/bin/env python3
"""Burst-then-idle memory-return probe for FP8 P4 / RES-06 with RES-10
(docs/problems/guest-damon-reclaim-state-unreported-2026-10-22.md; what
each outcome means: docs/reports/guest-memory-return-outcomes-2026-10-22.md).

FP0 runs it on ONE test computer (vmctl warmness_class "test"), never the
paper or an owner's computer: it refuses any other class. Every --interval
seconds (30) it samples, through Node B over ssh:

  host   VmRSS, RssAnon, RssFile of the computer's firecracker process, and
         host MemAvailable;
  host   GET 127.0.0.1:8083/internal/vmctl/memory/balloon?computer_id=ID
         (RES-10 reader; a reason such as stats_disabled when the computer
         is not in VM_BALLOON_STATS_ONLY);
  guest  GET <guest>:8085/internal/debug/pprof/procmem: meminfo and
         memory_return (DAMON reclaim enabled, kdamond_pid, kdamond CPU,
         bytes_reclaimed_regions, page_reporting_order and whether a
         reporting device registered, the balloon's negotiated reporting
         bit, reclaim and refault vmstat, free memory in reportable blocks).
         A guest that predates memory_return gives empty DAMON columns.

and writes one CSV row per sample to --out, then a JSON summary (deltas
per phase and a first reading against the outcomes table) to stdout and
--out with .summary.json.

Phases: baseline (--baseline-min, 1), burst (--burst-window-min, 4, from
the moment the burst is sent), idle (--idle-min, 20).

Burst (--burst):
  none     FP0 makes its own burst (or none); the probe only samples and
           labels the phases by the clock.
  capsule  with --cred-file (the 0600 credential file
           scripts/qa_test_computer.mjs wrote for this test computer), one
           prompt-bar request asks engineering to start one capsule job
           that touches --anon-mib (768) of anonymous memory, reads
           --cache-mib (1024) of /nix/store files into the page cache,
           holds 60 s and exits. At its end the guest frees the anonymous
           memory and capsule teardown writes memory.reclaim for the
           capsule's cache (internal/capsule/teardown_reclaim.go), so the
           burst's end tests the RETURN step (free page reporting) without
           DAMON. The idle tail tests DAMON on the cache outside capsules.
           The key is read from the file and never printed.

Safety: sampling is read-only (nothing is booted, stopped, selected or
written; no balloon PATCH; no DAMON parameter written). The burst is sent
only when host MemAvailable is at least --host-avail-floor-mib (4096). The
host-internal token goes only to 127.0.0.1:8083 through
scripts/choir_internal.py; the guest read carries the marker header only.
Command lines are read on the host to find the firecracker pid and never
printed.

usage: fp8_memory_return_probe.py --computer-id ID --out FILE.csv
       [--burst none|capsule --cred-file F] [--ssh node-b]
       [--interval 30] [--baseline-min 1] [--burst-window-min 4]
       [--idle-min 20] [--anon-mib 768] [--cache-mib 1024]
       [--host-avail-floor-mib 4096]

Exit: 0 a complete run (whatever the reading); 2 usage or refusal; 3 the
computer stopped answering for 3 consecutive samples (rows so far kept).
"""
import argparse
import csv
import datetime as dt
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

HOST = "https://choir.news"

# Remote side: `ssh HOST python3 - VM_ID COMPUTER_ID`, read-only. The helper
# goes first (as scripts/node_b_host_reboot.py does): internal_headers(url)
# attaches the token only for a loopback internal port, never a guest.
with open(Path(__file__).resolve().parent / "choir_internal.py") as _f:
    CHOIR_INTERNAL = _f.read()

# Host side: the firecracker process of one VM, matched by a cmdline
# argument equal to the VM id or ending in /<VM id>. A miss returns an error
# naming the VM, the pattern and how many firecracker processes were seen,
# so an empty fc_rss_mib column has a cause. Command lines are never
# returned.
FC_RSS = r'''
import os

def firecracker_rss(vm, proc="/proc"):
    def rd(p):
        try:
            with open(p) as f:
                return f.read()
        except OSError:
            return ""
    seen = 0
    for pid in filter(str.isdigit, os.listdir(proc)):
        if rd("%s/%s/comm" % (proc, pid)).strip() != "firecracker":
            continue
        seen += 1
        args = rd("%s/%s/cmdline" % (proc, pid)).split("\0")
        if not any(a == vm or a.endswith("/" + vm) for a in args):
            continue
        out = {}
        for l in rd("%s/%s/status" % (proc, pid)).splitlines():
            k = l.split(":", 1)[0]
            if k in ("VmRSS", "RssAnon", "RssFile"):
                out[k + "_kib"] = int(l.split()[1])
        if "VmRSS_kib" not in out:
            return out, "firecracker pid %s for vm %s has no VmRSS in its status (exited?)" % (pid, vm)
        return out, None
    return {}, ("no firecracker pid for vm %s: none of %d firecracker processes has a cmdline "
                "argument equal to %r or ending in %r" % (vm, seen, vm, "/" + vm))
'''

REMOTE = CHOIR_INTERNAL + FC_RSS + r'''
import json, os, re, sys, urllib.request, urllib.error

VM, CID = sys.argv[1], sys.argv[2]

def get(url, timeout=20):
    req = urllib.request.Request(url, headers=internal_headers(url))
    try:
        with internal_urlopen(req, timeout=timeout) as r:
            return r.status, r.read(4 << 20).decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read(1 << 14).decode("utf-8", "replace")
    except Exception as e:
        return 0, str(e)

def rd(p):
    try:
        with open(p) as f:
            return f.read()
    except OSError:
        return ""

out = {"errors": []}
for l in rd("/proc/meminfo").splitlines():
    if l.startswith("MemAvailable:"):
        out["host_mem_available_kib"] = int(l.split()[1])
st, body = get("http://127.0.0.1:8083/internal/vmctl/list")
own = None
if st == 200:
    for o in json.loads(body).get("ownerships") or []:
        if o.get("computer_id") == CID:
            own = {k: o.get(k) for k in ("vm_id", "warmness_class", "state", "computer_url")}
else:
    out["errors"].append("vmctl list %s" % st)
out["ownership"] = own
if VM != "no-vm-yet":  # the first call only reads the ownership
    rss, miss = firecracker_rss(VM)
    out.update(rss)
    if miss:
        out["errors"].append(miss)
st, body = get("http://127.0.0.1:8083/internal/vmctl/memory/balloon?computer_id=" + CID)
out["balloon_status"] = st
try:
    vms = json.loads(body).get("vms") or []
    out["balloon"] = vms[0] if vms else None
except Exception:
    out["balloon"] = None
m = re.match(r"^http://([0-9.]+):8085", (own or {}).get("computer_url") or "")
if m:
    url = "http://%s:8085/internal/debug/pprof/procmem" % m.group(1)
    st, body = get(url, timeout=30)
    out["procmem_status"] = st
    if st == 200:
        pm = json.loads(body)
        out["meminfo"] = pm.get("meminfo") or {}
        out["memory_return"] = pm.get("memory_return")
    else:
        out["errors"].append("procmem %s %s" % (st, body[:200]))
else:
    out["errors"].append("no guest address in computer_url")
print(json.dumps(out))
'''

COLUMNS = [
    "t_utc", "elapsed_s", "phase",
    "host_mem_available_mib", "fc_rss_mib", "fc_rss_anon_mib", "fc_rss_file_mib",
    "balloon_reason", "balloon_actual_mib", "balloon_free_mib", "balloon_available_mib", "balloon_disk_caches_mib",
    "guest_memfree_mib", "guest_memavailable_mib", "guest_cached_mib", "guest_anon_mib",
    "damon_present", "damon_enabled", "kdamond_pid", "kdamond_cpu_ticks",
    "damon_bytes_reclaim_tried", "damon_bytes_reclaimed", "damon_nr_reclaimed", "damon_nr_quota_exceeds",
    "page_reporting_order", "page_reporting_registered", "balloon_bound", "balloon_reporting",
    "guest_reportable_free_mib",
    "pgsteal_kswapd", "pgsteal_direct", "pgsteal_proactive", "workingset_refault_file", "pgmajfault",
    "errors",
]


def log(event, **kv):
    print(json.dumps({"t": time.strftime("%H:%M:%S", time.gmtime()), "event": event, **kv}), file=sys.stderr, flush=True)


def remote(ssh_host, vm, cid):
    p = subprocess.run(["ssh", "-o", "BatchMode=yes", ssh_host, "python3", "-", vm, cid],
                       input=REMOTE, capture_output=True, text=True, timeout=120)
    if p.returncode != 0:
        return {"errors": ["ssh exit %d: %s" % (p.returncode, p.stderr.strip()[:200])]}
    try:
        return json.loads(p.stdout)
    except json.JSONDecodeError:
        return {"errors": ["remote output not JSON: %s" % p.stdout[:200]]}


def mib_k(v):
    return None if v is None else round(v / 1024, 1)


def mib_b(v):
    return None if v is None else round(v / (1 << 20), 1)


def row(raw, t0, phase):
    now = dt.datetime.now(dt.timezone.utc)
    mi = raw.get("meminfo") or {}
    mr = raw.get("memory_return") or {}
    d = mr.get("damon_reclaim") or {}
    ds = d.get("stats") or {}
    pr = mr.get("page_reporting") or {}
    bd = mr.get("balloon") or {}
    vs = mr.get("vmstat") or {}
    fb = mr.get("free_blocks") or {}
    be = raw.get("balloon") or {}
    bs = (be.get("reading") or {}).get("stats") or {}
    return {
        "t_utc": now.isoformat(timespec="seconds"),
        "elapsed_s": int(time.time() - t0),
        "phase": phase,
        "host_mem_available_mib": mib_k(raw.get("host_mem_available_kib")),
        "fc_rss_mib": mib_k(raw.get("VmRSS_kib")),
        "fc_rss_anon_mib": mib_k(raw.get("RssAnon_kib")),
        "fc_rss_file_mib": mib_k(raw.get("RssFile_kib")),
        "balloon_reason": be.get("reason") or ("" if be else "no entry"),
        "balloon_actual_mib": bs.get("actual_mib"),
        "balloon_free_mib": mib_b(bs.get("free_memory")),
        "balloon_available_mib": mib_b(bs.get("available_memory")),
        "balloon_disk_caches_mib": mib_b(bs.get("disk_caches")),
        "guest_memfree_mib": mib_k(mi.get("MemFree_kib")),
        "guest_memavailable_mib": mib_k(mi.get("MemAvailable_kib")),
        "guest_cached_mib": mib_k(mi.get("Cached_kib")),
        "guest_anon_mib": mib_k(mi.get("AnonPages_kib")),
        "damon_present": d.get("present"),
        "damon_enabled": d.get("enabled"),
        "kdamond_pid": d.get("kdamond_pid"),
        "kdamond_cpu_ticks": d.get("kdamond_cpu_ticks"),
        "damon_bytes_reclaim_tried": ds.get("bytes_reclaim_tried_regions"),
        "damon_bytes_reclaimed": ds.get("bytes_reclaimed_regions"),
        "damon_nr_reclaimed": ds.get("nr_reclaimed_regions"),
        "damon_nr_quota_exceeds": ds.get("nr_quota_exceeds"),
        "page_reporting_order": pr.get("order"),
        "page_reporting_registered": pr.get("registered"),
        "balloon_bound": bd.get("bound"),
        "balloon_reporting": bd.get("reporting"),
        "guest_reportable_free_mib": mib_k(fb.get("reportable_kib")),
        "pgsteal_kswapd": vs.get("pgsteal_kswapd"),
        "pgsteal_direct": vs.get("pgsteal_direct"),
        "pgsteal_proactive": vs.get("pgsteal_proactive"),
        "workingset_refault_file": vs.get("workingset_refault_file"),
        "pgmajfault": vs.get("pgmajfault"),
        "errors": "; ".join((raw.get("errors") or []) + (mr.get("errors") or []))[:500],
    }


def burst_brief(stamp, anon_mib, cache_mib):
    script = (
        # Reads go through one 1 MiB buffer, so the cache grows and the
        # anonymous memory stays the --anon-mib block.
        f"import os, time; b = b'x' * ({anon_mib} << 20); buf = bytearray(1 << 20); n = 0\\n"
        f"for r, _, fs in os.walk('/nix/store'):\\n"
        f"    for f in fs:\\n"
        f"        if n >= ({cache_mib} << 20): break\\n"
        f"        try: h = open(os.path.join(r, f), 'rb')\\n"
        f"        except Exception: continue\\n"
        f"        with h:\\n"
        f"            while (k := h.readinto(buf)): n += k\\n"
        f"    if n >= ({cache_mib} << 20): break\\n"
        f"print('anon', len(b) >> 20, 'cache', n >> 20); time.sleep(60)"
    )
    need = anon_mib + 512
    return (
        f"Engineering probe {stamp} for guest memory return. Engineering: in one cell start one capsule job and nothing else: "
        f'id, err := choir.Start([]string{{"python3", "-c", "{script}"}}, map[string]any{{"memory_mib": {need}, "minutes": 5}}); '
        f"println(id, err). If it is refused, Complete with result failed quoting the refusal. Otherwise end your turn; "
        f"when you are woken with its end, read choir.Poll(id) and Complete with result completed naming the job id and its state. "
        f"Do not Freeze: this is a probe, not a change to land."
    )


def send_burst(cred_file, anon_mib, cache_mib):
    cred = json.load(open(cred_file))
    key = cred.get("api_key") or cred.get("key")
    cid = cred["computer_id"]
    stamp = time.strftime("%Y%m%dT%H%M%S", time.gmtime())
    body = json.dumps({"text": burst_brief(stamp, anon_mib, cache_mib),
                       "command_id": f"memory-return-probe-{stamp}"}).encode()
    req = urllib.request.Request(HOST + "/api/prompt-bar", data=body, method="POST", headers={
        "Authorization": "Bearer " + key, "X-Choir-Computer": cid, "Content-Type": "application/json"})
    try:
        got = json.load(urllib.request.urlopen(req, timeout=300))
    except urllib.error.HTTPError as e:
        return cid, {"error": e.code}
    except urllib.error.URLError as e:
        return cid, {"error": str(e.reason)[:200]}
    return cid, {k: got.get(k) for k in ("trajectory_id", "submission_id", "error") if isinstance(got, dict)}


def num(r, k):
    v = r.get(k)
    return v if isinstance(v, (int, float)) and not isinstance(v, bool) else None


def delta(rows, k):
    vals = [num(r, k) for r in rows if num(r, k) is not None]
    return None if len(vals) < 2 else round(vals[-1] - vals[0], 1)


def first_reading(rows):
    """A first reading against the outcomes table; FP0's read of the CSV wins."""
    idle = [r for r in rows if r["phase"] == "idle"]
    burst = [r for r in rows if r["phase"] == "burst"]
    last = rows[-1] if rows else {}
    out = {"deltas_idle": {k: delta(idle, k) for k in (
        "fc_rss_mib", "guest_memfree_mib", "guest_cached_mib", "damon_bytes_reclaimed",
        "guest_reportable_free_mib", "kdamond_cpu_ticks", "workingset_refault_file", "pgmajfault")}}
    peak = max((num(r, "fc_rss_mib") or 0 for r in burst), default=None)
    out["burst_peak_fc_rss_mib"] = peak or None
    out["idle_end_fc_rss_mib"] = num(last, "fc_rss_mib")
    if last.get("damon_present") is None:
        out["damon"] = "unknown (guest predates memory_return)"
    elif not last.get("damon_present"):
        out["damon"] = "absent (kernel without DAMON_RECLAIM)"
    elif not last.get("damon_enabled") or (num(last, "kdamond_pid") or -1) <= 0:
        out["damon"] = "off (outcome A)"
    elif (out["deltas_idle"]["damon_bytes_reclaimed"] or 0) <= 0:
        out["damon"] = "on, reclaimed nothing in the idle window (outcome A': watermarks, min_age or quota)"
    else:
        out["damon"] = "on, reclaimed %s MiB in the idle window" % mib_b(out["deltas_idle"]["damon_bytes_reclaimed"])
    if last.get("page_reporting_registered") is False or last.get("balloon_reporting") is False:
        out["reporting"] = "not active (outcome B: no reporting device, or the balloon did not negotiate reporting)"
        return out
    if last.get("page_reporting_registered") is None:
        out["reporting"] = "unknown (guest predates memory_return)"
        return out
    # The burst's end (anon freed at exit, capsule cache dropped by teardown
    # reclaim) tests return without DAMON: from the burst's RSS peak to the
    # first idle sample. The idle window tests return of what DAMON frees.
    windows = []
    peak_row = max(burst, key=lambda r: num(r, "fc_rss_mib") or 0, default=None)
    if peak_row and idle:
        windows.append(("burst_end", [peak_row, idle[0]]))
    windows.append(("idle", idle))
    out["reporting_windows"] = {}
    for name, rs in windows:
        # Both ends must be measured: a missing reading is not a zero.
        ends = (("start", rs[0]), ("end", rs[-1])) if rs else ()
        missing = ["%s at the window's %s" % (k, side) for side, e in ends
                   for k in ("fc_rss_mib", "guest_memfree_mib") if num(e, k) is None]
        if not rs or missing:
            v = "undeterminable (%s)" % ("no samples" if not rs else "no " + ", no ".join(missing))
            out["reporting_windows"][name] = v
            continue
        free_up = round(num(rs[-1], "guest_memfree_mib") - num(rs[0], "guest_memfree_mib"), 1)
        rss_down = round(num(rs[0], "fc_rss_mib") - num(rs[-1], "fc_rss_mib"), 1)
        if free_up < 128:
            v = "untested (guest MemFree rose %s MiB)" % free_up
        elif rss_down >= 0.5 * free_up:
            v = "returned (outcome C): host RSS fell %s MiB for %s MiB freed" % (rss_down, free_up)
        else:
            v = "freed but not returned (outcome B): host RSS fell %s MiB for %s MiB freed" % (rss_down, free_up)
        out["reporting_windows"][name] = v
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--computer-id", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--burst", choices=("none", "capsule"), default="none")
    ap.add_argument("--cred-file")
    ap.add_argument("--interval", type=int, default=30)
    ap.add_argument("--baseline-min", type=float, default=1)
    ap.add_argument("--burst-window-min", type=float, default=4)
    ap.add_argument("--idle-min", type=float, default=20)
    ap.add_argument("--anon-mib", type=int, default=768)
    ap.add_argument("--cache-mib", type=int, default=1024)
    ap.add_argument("--host-avail-floor-mib", type=int, default=4096)
    args = ap.parse_args()

    def refuse(msg):
        log("refused", reason=msg)
        sys.exit(2)

    if not re.match(r"^[A-Za-z0-9._-]+$", args.computer_id):
        refuse("bad computer id")
    if not re.match(r"^[A-Za-z0-9._@-]+$", args.ssh):
        refuse("bad ssh host")
    if args.interval < 5 or args.anon_mib < 0 or args.cache_mib < 0:
        refuse("bad interval or sizes")
    if args.burst == "capsule":
        if not args.cred_file:
            refuse("--burst capsule needs --cred-file (the test computer's 0600 credential file)")
        cred_cid = json.load(open(args.cred_file)).get("computer_id")
        if cred_cid != args.computer_id:
            refuse("--cred-file names another computer")

    first = remote(args.ssh, "no-vm-yet", args.computer_id)
    own = first.get("ownership")
    if not own:
        refuse("computer not in vmctl list: %s" % "; ".join(first.get("errors") or []))
    if own.get("warmness_class") != "test":
        refuse("computer is class %r, not test; this probe runs on test computers only" % own.get("warmness_class"))
    vm = own.get("vm_id") or ""
    if not re.match(r"^[A-Za-z0-9._-]+$", vm):
        refuse("bad vm id from vmctl")
    log("start", computer=args.computer_id, vm=vm, state=own.get("state"), burst=args.burst)

    t0 = time.time()
    burst_at = t0 + args.baseline_min * 60
    end = burst_at + (args.burst_window_min + args.idle_min) * 60
    sent = None
    rows, misses = [], 0
    summary_path = re.sub(r"\.csv$", "", args.out) + ".summary.json"
    with open(args.out, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=COLUMNS)
        w.writeheader()
        while time.time() < end:
            tick = time.time()
            if sent is None and tick >= burst_at:
                sent = {"at": dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds"), "mode": args.burst}
                if args.burst == "capsule":
                    avail = (rows[-1].get("host_mem_available_mib") if rows else None) or 0
                    if avail < args.host_avail_floor_mib:
                        sent["skipped"] = "host MemAvailable %s MiB < floor %s" % (avail, args.host_avail_floor_mib)
                    else:
                        _, sent["prompt_bar"] = send_burst(args.cred_file, args.anon_mib, args.cache_mib)
                log("burst", **sent)
            phase = "baseline" if sent is None else ("burst" if tick < burst_at + args.burst_window_min * 60 else "idle")
            raw = remote(args.ssh, vm, args.computer_id)
            r = row(raw, t0, phase)
            if r["guest_memfree_mib"] is None and r["fc_rss_mib"] is None:
                misses += 1
            else:
                misses = 0
            rows.append(r)
            w.writerow(r)
            f.flush()
            log("sample", phase=phase, fc_rss_mib=r["fc_rss_mib"], guest_memfree_mib=r["guest_memfree_mib"],
                guest_cached_mib=r["guest_cached_mib"], damon_bytes_reclaimed=r["damon_bytes_reclaimed"],
                errors=r["errors"] or None)
            if misses >= 3:
                log("stopped", reason="the computer did not answer for 3 samples (hibernated or stopped?)")
                break
            time.sleep(max(0, args.interval - (time.time() - tick)))
    summary = {"probe": "fp8_memory_return", "computer_id": args.computer_id, "vm_id": vm,
               "csv": args.out, "samples": len(rows), "burst": sent, "reading": first_reading(rows)}
    with open(summary_path, "w") as f:
        json.dump(summary, f, indent=2)
    print(json.dumps(summary, indent=2))
    sys.exit(3 if misses >= 3 else 0)


if __name__ == "__main__":
    main()
