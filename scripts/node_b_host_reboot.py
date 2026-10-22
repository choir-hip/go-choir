#!/usr/bin/env python3
"""Node B host reboot: pre-checks, planned-restart markers, reboot, post-checks.

Runbook: docs/runbooks/node-b-host-reboot.md. Problem:
docs/problems/host-kernel-216-days-unpatched-2026-10-15.md.

Run it from the workstation. It drives Node B over ssh, as root, the way the
deploy does.

    scripts/node_b_host_reboot.py                     # dry run: pre-checks + plan
    scripts/node_b_host_reboot.py --phase post --state FILE   # read-only post-checks
    scripts/node_b_host_reboot.py --execute --console-verified   # the real reboot

DRY RUN IS THE DEFAULT. Without --execute it changes nothing: it reads
Node B, prints what it would do, and exits. --execute also needs
--console-verified, the operator's statement that the OVH IPMI/KVM console
was opened for this server in this window. A host that doesn't come back
over ssh can only be recovered from that console.

Phases (--phase all runs them in order):
  pre     read-only. Kernel staged (the current system's kernel at or above
          --min-kernel and different from the booted one); nested=0 in the
          staged modprobe config; GRUB's default entry boots the staged
          kernel; md arrays clean; no NixOS switch running; go-choir units
          active; vmctl healthy; staging /health; firewall chains; a
          verify-host-hardening.py baseline; the computers that are running.
  mark    (--execute) asks every running, non-held computer to write its
          planned-restart marker (POST /internal/runtime/planned-restart,
          the same request deploy-refresh makes). A busy guest refuses
          (409). Busy ones are retried until --busy-wait runs out. Then the
          run aborts and clears the markers already written, unless
          --allow-crash-close accepts that their work ends "interrupted by
          a restart". A marker counts only if its computer boots within
          15 minutes (plannedRestartMaxAge), so this runs immediately before
          the reboot, and post reports the marker age. A marker that expires
          turns that boot into a crash boot, the safe fallback. Markers are
          written only to idle guests, so they can resume parked work (wakes,
          schedule generations) but never re-dispatch an in-flight run.
          --no-markers skips this phase: every boot is a crash boot.
  reboot  (--execute) systemctl reboot. Waits for ssh with a new boot_id
          (--boot-timeout).
  post    read-only. Booted kernel = staged kernel; nested=N; booted system
          = current system; units back; md clean; vmctl healthy; firewall
          chains re-created; no new verify-host-hardening failures; staging
          /health ok with the same deployed commit; always-on and critical
          computers active, and each marked computer's boot kind (planned
          or crash) from its obligations surface.

Hibernate is deliberately not used. A snapshot is bound to the host kernel
release, so after a kernel change every snapshot cold-boots as a crash boot
(internal/vmmanager/snapshot.go, host_kernel_mismatch).

Exit status: 0 ok; 1 a pre-check or post-check failed; 2 usage error; 3
aborted before the reboot (markers cleared); 4 the host did not come back
within --boot-timeout (go to the console; see the runbook).
"""

import argparse
import json
import os
import re
import subprocess
import sys
import time
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
HARDENING_SCRIPT = REPO / "scripts" / "verify-host-hardening.py"
STAGING_HEALTH = "https://choir.news/health"
SSH_OPTS = ["-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=10",
            "-o", "ServerAliveCountMax=3"]
# Warmness classes vmctl wakes on its own after vmctl starts (alwaysOnWarmTargets
# and the platform computer); every other computer boots on its next request.
AUTO_WARM_CLASSES = {"premium_always_on", "critical_protected", "public_platform"}

# ---------------------------------------------------------------------------
# Remote side: runs as root on Node B via `ssh host python3 - <mode>`. The
# modes "facts" and "computers" only read. The modes "mark" and "unmark" write
# or clear guest markers, and the caller sends them only under --execute.
# scripts/choir_internal.py goes first: internal_headers(url) attaches the
# host-internal token only for a loopback internal port, never a guest, and
# internal_urlopen sends it with no proxy and follows no redirect.
with open(Path(__file__).resolve().parent / "choir_internal.py") as _f:
    CHOIR_INTERNAL = _f.read()
REMOTE = CHOIR_INTERNAL + r'''
import json, os, re, subprocess, sys, urllib.request, urllib.error

def sh(cmd, timeout=30):
    try:
        p = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=timeout)
        return p.returncode, p.stdout.strip(), p.stderr.strip()
    except subprocess.TimeoutExpired:
        return 124, "", "timeout"

def read(path):
    try:
        with open(path) as f:
            return f.read().strip()
    except OSError as e:
        return None

def kernel_version(path):
    m = re.search(r"linux-(\d+\.\d+\.\d+)", path or "")
    return m.group(1) if m else None

def http(method, url, headers=None, body=None, timeout=20):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers=dict(headers or {}))
    if data is not None:
        req.add_header("Content-Type", "application/json")
    try:
        with internal_urlopen(req, timeout=timeout) as r:
            return r.status, r.read(1 << 20).decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read(1 << 16).decode("utf-8", "replace")
    except Exception as e:
        return 0, str(e)

def vmctl_list():
    url = "http://127.0.0.1:8083/internal/vmctl/list"
    status, body = http("GET", url, internal_headers(url))
    if status != 200:
        return None, "vmctl list: %s %s" % (status, body[:200])
    return json.loads(body).get("ownerships", []), None

def facts():
    out = {}
    out["boot_id"] = read("/proc/sys/kernel/random/boot_id")
    out["uname_r"] = os.uname().release
    out["hostname"] = os.uname().nodename
    out["uptime_s"] = float((read("/proc/uptime") or "0").split()[0])
    booted = os.path.realpath("/run/booted-system")
    current = os.path.realpath("/run/current-system")
    out["booted_system"], out["current_system"] = booted, current
    out["booted_kernel"] = kernel_version(os.path.realpath("/run/booted-system/kernel"))
    out["current_kernel"] = kernel_version(os.path.realpath("/run/current-system/kernel"))
    out["nested"] = read("/sys/module/kvm_intel/parameters/nested")
    out["smt_active"] = read("/sys/devices/system/cpu/smt/active")
    out["cpus_online"] = os.cpu_count()
    modprobe = ""
    d = "/run/current-system/etc/modprobe.d"
    for name in sorted(os.listdir(d)) if os.path.isdir(d) else []:
        modprobe += (read(os.path.join(d, name)) or "") + "\n"
    out["staged_nested_off"] = bool(re.search(r"options\s+kvm_intel\s+.*nested=(0|N|n)\b", modprobe))
    out["staged_kernel_params"] = read("/run/current-system/kernel-params")
    cfg = read("/boot/grub/grub.cfg") or ""
    m = re.search(r'menuentry "NixOS" .*?\n\s*search[^\n]*\n\s*linux\s+(\S+)', cfg, re.S)
    out["grub_default_kernel"] = kernel_version(m.group(1)) if m else None
    out["grub_entries"] = len(re.findall(r"^\s*menuentry ", cfg, re.M))
    out["boot_kernels"] = sorted(os.listdir("/boot/kernels")) if os.path.isdir("/boot/kernels") else []
    mdstat = read("/proc/mdstat") or ""
    out["md_arrays"] = re.findall(r"^(md\d+) : (\S+) .*?\n\s+\d+ blocks.*?\[([U_]+)\]", mdstat, re.M | re.S)
    out["md_clean"] = bool(out["md_arrays"]) and all(state == "active" and "_" not in bits for _, state, bits in out["md_arrays"])
    _, out["system_state"], _ = sh("systemctl is-system-running")
    _, failed, _ = sh("systemctl list-units --failed --plain --no-legend")
    out["failed_units"] = [l.split()[0] for l in failed.splitlines() if l.strip()]
    _, units, _ = sh("systemctl list-units --all --plain --no-legend 'go-choir-*'")
    out["go_choir_units"] = {l.split()[0]: l.split()[2] + "/" + l.split()[3] for l in units.splitlines() if len(l.split()) >= 4}
    rc, _, _ = sh("pgrep -f '[s]witch-to-configuration'")
    switch_unit = sh("systemctl is-active nixos-rebuild-switch-to-configuration.service")[1]
    out["switch_running"] = rc == 0 or switch_unit in ("active", "activating")
    status, _ = http("GET", "http://127.0.0.1:8083/health", timeout=5)
    out["vmctl_health"] = status
    fw = {}
    for chain in ("FORWARD", "CHOIR-GUEST-FWD", "CHOIR-GUEST-TAPS", "nixos-fw"):
        rc, rules, _ = sh("iptables -S %s" % chain)
        fw[chain] = rules.splitlines() if rc == 0 else None
    fwd = [r for r in (fw["FORWARD"] or []) if r.startswith("-A FORWARD")]
    out["fw_guest_jump_first"] = bool(fwd) and "-i vm-+" in fwd[0] and fwd[0].endswith("-j CHOIR-GUEST-FWD")
    gf = [r for r in (fw["CHOIR-GUEST-FWD"] or []) if r.startswith("-A ")]
    out["fw_guest_fwd_fails_closed"] = bool(gf) and gf[-1].endswith("-j DROP") and any("-j CHOIR-GUEST-TAPS" in r for r in gf)
    out["fw_guest_taps_chain"] = fw["CHOIR-GUEST-TAPS"] is not None
    out["fw_nixos_chain"] = fw["nixos-fw"] is not None
    gi = "/var/lib/go-choir/guest"
    out["guest_image"] = os.path.realpath(gi)
    try:
        out["guest_image_installed_at"] = os.lstat(gi).st_ctime
    except OSError:
        out["guest_image_installed_at"] = None
    manifest = read(gi + "/guest-image-manifest") or ""
    m = re.search(r"^kernel=(\S+)", manifest, re.M)
    out["guest_kernel"] = kernel_version(m.group(1)) if m else None
    _, df, _ = sh("df -P / /boot | tail -n +2")
    out["df"] = [l.split() for l in df.splitlines()]
    return out

def computers():
    owns, err = vmctl_list()
    if err:
        return {"error": err}
    rows = []
    for o in owns:
        row = {k: o.get(k) for k in ("vm_id", "computer_id", "user_id", "desktop_id", "state", "warmness_class",
                                     "held", "stopped_by", "computer_url", "snapshot_kind")}
        if o.get("state") in ("active", "degraded", "booting") and o.get("computer_url"):
            # A computer booted before the current guest image was installed
            # still runs the old guest kernel: deploy refresh deferred it
            # (busy), and it is retried only at the next deploy.
            try:
                with open("/var/lib/go-choir/vm-state/%s/boot-timeline.json" % o["vm_id"]) as fh:
                    row["booted_at"] = json.load(fh).get("started_at")
            except (OSError, ValueError):
                row["booted_at"] = None
            url = o["computer_url"].rstrip("/")
            hs, _ = http("GET", url + "/health", timeout=8)
            row["health"] = hs
            st, body = http("GET", url + "/api/runtime/obligations",
                            {"X-Internal-Caller": "true", "X-Authenticated-User": o.get("user_id") or ""}, timeout=15)
            if st == 200:
                ob = json.loads(body)
                row["restart_kind"] = (ob.get("restart") or {}).get("kind")
                row["runs"] = ob.get("runs") or {}
                row["open_work_items"] = ob.get("open_work_items")
            else:
                row["obligations_status"] = st
        rows.append(row)
    counts = {}
    for o in owns:
        counts[o.get("state")] = counts.get(o.get("state"), 0) + 1
    return {"counts": counts, "computers": rows}

def mark(items, target, method):
    results = []
    for it in items:
        url = it["computer_url"].rstrip("/") + "/internal/runtime/planned-restart"
        body = {"target": target} if method == "POST" else None
        st, text = http(method, url, {"X-Internal-Caller": "true"}, body, timeout=25)
        results.append({"vm_id": it["vm_id"], "status": st, "body": text[:300]})
    return results

mode = sys.argv[1]
if mode == "facts":
    print(json.dumps(facts()))
elif mode == "computers":
    print(json.dumps(computers()))
elif mode in ("mark", "unmark"):
    req = json.loads(sys.argv[2])
    print(json.dumps(mark(req["items"], req.get("target", ""), "POST" if mode == "mark" else "DELETE")))
else:
    sys.exit("unknown mode " + mode)
'''


def now():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def log(msg):
    print("[%s] %s" % (now(), msg), flush=True)


def remote(host, mode, *args, timeout=180):
    cmd = ["ssh", *SSH_OPTS, host, "python3", "-", mode, *[_shq(a) for a in args]]
    p = subprocess.run(cmd, input=REMOTE, capture_output=True, text=True, timeout=timeout)
    if p.returncode != 0:
        raise RuntimeError("remote %s failed (%d): %s" % (mode, p.returncode, p.stderr.strip()[-500:]))
    return json.loads(p.stdout)


def _shq(s):
    # ssh joins argv with spaces into a remote shell command; quote each arg.
    return "'" + s.replace("'", "'\"'\"'") + "'"


def ssh_ok(host, timeout=15):
    try:
        p = subprocess.run(["ssh", *SSH_OPTS, host, "cat", "/proc/sys/kernel/random/boot_id"],
                           capture_output=True, text=True, timeout=timeout)
        return p.stdout.strip() if p.returncode == 0 else None
    except subprocess.TimeoutExpired:
        return None


def staging_health():
    try:
        with urllib.request.urlopen(STAGING_HEALTH, timeout=10) as r:
            h = json.loads(r.read())
        return {"status": h.get("status"), "vmctl_status": h.get("vmctl_status"),
                "deployed_commit": (h.get("platform") or {}).get("deployed_commit")}
    except Exception as e:
        return {"error": str(e)}


def hardening(host, extra):
    if not HARDENING_SCRIPT.exists():
        return {"error": "missing " + str(HARDENING_SCRIPT)}
    cmd = ["ssh", *SSH_OPTS, host, "python3", "-", "--json", *extra]
    p = subprocess.run(cmd, stdin=open(HARDENING_SCRIPT), capture_output=True, text=True, timeout=300)
    try:
        results = json.loads(p.stdout)
    except ValueError:
        return {"error": "verify-host-hardening output unreadable (exit %d): %s" % (p.returncode, p.stderr[-300:])}
    failed = sorted("[%s] %s" % (r["section"], r["check"]) for r in results if not r.get("ok"))
    return {"exit": p.returncode, "passed": len(results) - len(failed), "failed": failed}


def iso_ts(v):
    """Seconds since the epoch for an RFC 3339 time (nanoseconds allowed)."""
    if not v:
        return None
    try:
        return datetime.strptime(v[:19], "%Y-%m-%dT%H:%M:%S").replace(tzinfo=timezone.utc).timestamp()
    except ValueError:
        return None


def kernel_security_floor():
    # The flake's kernel floor assertion reads the same file. No fallback:
    # a missing or malformed floor must stop the run, not lower the bar.
    path = REPO / "nix" / "kernel-security-floor"
    try:
        floor = path.read_text().strip()
    except OSError as e:
        raise SystemExit("%s: %s; pass --min-kernel or restore the file" % (path, e))
    if not re.fullmatch(r"\d+\.\d+\.\d+", floor):
        raise SystemExit("nix/kernel-security-floor: %r is not a kernel version" % floor)
    return floor


def vtuple(v):
    try:
        return tuple(int(x) for x in v.split("."))
    except (AttributeError, ValueError):
        return (0,)


class Checks:
    def __init__(self):
        self.items = []

    def add(self, name, ok, detail="", blocking=True):
        self.items.append({"check": name, "ok": bool(ok), "blocking": blocking, "detail": detail})
        print("  %s  %s%s" % ("PASS" if ok else ("FAIL" if blocking else "WARN"), name,
                              (": " + str(detail)) if detail != "" else ""), flush=True)

    def failed(self):
        return [c for c in self.items if c["blocking"] and not c["ok"]]


def running_computers(comp):
    return [c for c in comp.get("computers", [])
            if c.get("state") in ("active", "degraded", "booting") and c.get("computer_url")]


def phase_pre(args, receipt):
    log("PRE-CHECKS (read-only) on %s" % args.host)
    f = remote(args.host, "facts")
    comp = remote(args.host, "computers")
    receipt["pre"] = {"facts": f, "computers": comp, "staging": staging_health()}
    c = Checks()
    c.add("booted kernel", True, "%s (up %.1f days, hostname %s)" % (f["uname_r"], f["uptime_s"] / 86400, f["hostname"]), False)
    c.add("staged kernel >= %s" % args.min_kernel, vtuple(f["current_kernel"]) >= vtuple(args.min_kernel),
          "current-system kernel %s" % f["current_kernel"])
    c.add("staged kernel differs from booted", f["current_kernel"] != f["uname_r"],
          "booted %s, staged %s" % (f["uname_r"], f["current_kernel"]))
    c.add("staged modprobe sets kvm_intel nested=0", f["staged_nested_off"])
    c.add("guest image kernel >= %s" % args.min_kernel, vtuple(f["guest_kernel"]) >= vtuple(args.min_kernel),
          "%s (%s)" % (f["guest_kernel"], f["guest_image"]))
    c.add("GRUB default entry boots the staged kernel", f["grub_default_kernel"] == f["current_kernel"],
          "grub %s, %d entries, /boot/kernels %s" % (f["grub_default_kernel"], f["grub_entries"],
                                                    [k for k in f["boot_kernels"] if "bzImage" in k]))
    c.add("md RAID arrays clean", f["md_clean"], f["md_arrays"])
    c.add("no NixOS switch/deploy running", not f["switch_running"])
    c.add("systemd state", f["system_state"] in ("running", "degraded"), f["system_state"])
    c.add("no failed units", not f["failed_units"], f["failed_units"], blocking=False)
    inactive = {u: s for u, s in f["go_choir_units"].items() if not s.startswith("active")}
    c.add("go-choir units active", not inactive, inactive or "%d units" % len(f["go_choir_units"]), blocking=False)
    c.add("vmctl /health", f["vmctl_health"] == 200, f["vmctl_health"])
    c.add("firewall: FORWARD jumps vm-+ to CHOIR-GUEST-FWD first", f["fw_guest_jump_first"], blocking=False)
    c.add("firewall: CHOIR-GUEST-FWD fails closed", f["fw_guest_fwd_fails_closed"], blocking=False)
    st = receipt["pre"]["staging"]
    c.add("staging /health", st.get("status") == "ok", st, blocking=False)
    c.add("SMT (informational; owner decision)", True, "smt/active=%s, %s logical CPUs" % (f["smt_active"], f["cpus_online"]), False)
    c.add("nested now (informational)", True, f["nested"], False)
    if "error" in comp:
        c.add("vmctl computer list", False, comp["error"])
    else:
        c.add("computers by state", True, comp["counts"], False)
        for r in running_computers(comp):
            busy = (r.get("runs") or {}).get("running", 0)
            c.add("running computer %s" % r["vm_id"], True,
                  "class=%s held=%s health=%s runs=%s auto-warm=%s" % (
                      r.get("warmness_class"), bool(r.get("held")), r.get("health"), r.get("runs"),
                      r.get("warmness_class") in AUTO_WARM_CLASSES), False)
            if busy:
                c.add("computer %s idle" % r["vm_id"], False, "%d running run(s): the marker will be refused until idle" % busy, False)
            booted, installed = iso_ts(r.get("booted_at")), f.get("guest_image_installed_at")
            if booted and installed and booted < installed:
                c.add("computer %s on the current guest image" % r["vm_id"], False,
                      "booted %s, before the guest image was installed: deploy refresh deferred or skipped it (busy, held, divergent or not active); vmctl's sweep "
                      "retries a deferred one at an idle moment and never retries a held one; this reboot moves an unheld one "
                      "to the new guest kernel" % r.get("booted_at"), False)
        crit = [r for r in comp["computers"] if r.get("warmness_class") == "critical_protected"]
        for r in crit:
            c.add("critical computer %s (the paper)" % r["vm_id"], True,
                  "state=%s stopped_by=%s" % (r.get("state"), r.get("stopped_by")), False)
    if args.skip_hardening:
        receipt["pre"]["hardening"] = None
    else:
        h = hardening(args.host, args.hardening_args.split())
        receipt["pre"]["hardening"] = h
        c.add("verify-host-hardening baseline", "error" not in h, h, blocking=False)
    receipt["pre"]["checks"] = c.items
    return c


def phase_mark(args, receipt):
    if args.no_markers:
        log("MARK: skipped (--no-markers): every computer boots as a crash boot; parked work closes interrupted_by_restart")
        receipt["marks"] = {"skipped": True}
        return True
    comp = remote(args.host, "computers")
    targets = [r for r in running_computers(comp) if not r.get("held")]
    target = "host-reboot kernel %s" % receipt["pre"]["facts"]["current_kernel"]
    marked, busy = [], {r["vm_id"]: r for r in targets}
    deadline = time.time() + args.busy_wait
    log("MARK: asking %d running computer(s) for planned-restart markers (target %r)" % (len(targets), target))
    while busy:
        res = remote(args.host, "mark", json.dumps({"items": list(busy.values()), "target": target}))
        for r in res:
            if r["status"] == 200:
                log("  marked %s" % r["vm_id"])
                marked.append(busy.pop(r["vm_id"]))
            elif r["status"] == 409:
                log("  busy %s: %s" % (r["vm_id"], r["body"][:160]))
            else:
                log("  no marker for %s: HTTP %s %s" % (r["vm_id"], r["status"], r["body"][:160]))
        if not busy or time.time() >= deadline:
            break
        time.sleep(15)
    receipt["marks"] = {"target": target, "marked": [m["vm_id"] for m in marked],
                        "unmarked": list(busy), "marked_at": now()}
    if busy and not args.allow_crash_close:
        log("ABORT: %d computer(s) still busy or unmarkable: %s. Clearing markers written so far." % (len(busy), list(busy)))
        if marked:
            remote(args.host, "unmark", json.dumps({"items": marked}))
        receipt["marks"]["cleared"] = True
        return False
    if busy:
        log("--allow-crash-close: rebooting with %d unmarked computer(s); their work ends interrupted_by_restart" % len(busy))
    return True


def phase_reboot(args, receipt):
    before = receipt["pre"]["facts"]["boot_id"]
    log("REBOOT: systemctl reboot on %s (boot_id %s)" % (args.host, before))
    try:
        subprocess.run(["ssh", *SSH_OPTS, args.host, "systemctl", "reboot"], capture_output=True, text=True, timeout=60)
    except subprocess.TimeoutExpired:
        pass  # the connection drops as the host goes down
    receipt["reboot"] = {"requested_at": now(), "boot_id_before": before}
    t0 = time.time()
    deadline = t0 + args.boot_timeout
    time.sleep(30)
    while time.time() < deadline:
        bid = ssh_ok(args.host)
        if bid and bid != before:
            receipt["reboot"].update({"back_at": now(), "seconds": round(time.time() - t0), "boot_id_after": bid})
            log("host back after %ds (boot_id %s)" % (time.time() - t0, bid))
            if time.time() - t0 > 15 * 60:
                log("WARNING: over 15 minutes; planned-restart markers are stale, so computers boot as crash boots")
            return True
        time.sleep(10)
    receipt["reboot"]["timed_out"] = True
    log("HOST DID NOT RETURN within %ds. Open the OVH IPMI/KVM console (runbook: Rollback)." % args.boot_timeout)
    return False


def wait_for(fn, seconds, interval=10):
    """Poll fn until it returns a truthy value or seconds pass. A remote
    error (ssh not up yet, vmctl still starting) counts as not yet."""
    deadline = time.time() + seconds
    while True:
        try:
            v = fn()
        except (RuntimeError, ValueError, subprocess.TimeoutExpired) as e:
            log("  waiting: %s" % str(e)[:160])
            v = None
        if v or time.time() >= deadline:
            return v
        time.sleep(interval)


def settled(host):
    f = remote(host, "facts")
    return f if f["system_state"] not in ("starting", "initializing") and f["vmctl_health"] == 200 else None


def phase_post(args, receipt):
    log("POST-CHECKS (read-only) on %s" % args.host)
    pre = receipt.get("pre") or {}
    pf = pre.get("facts") or {}
    c = Checks()
    f = wait_for(lambda: settled(args.host), args.settle_wait) or remote(args.host, "facts")
    receipt["post"] = {"facts": f}
    if pf:
        c.add("new boot", f["boot_id"] != pf.get("boot_id"), f["boot_id"])
        expect = pf.get("current_kernel")
    else:
        expect = f["current_kernel"]
    c.add("uname -r is the staged kernel", f["uname_r"] == expect, "uname %s, expected %s" % (f["uname_r"], expect))
    c.add("kernel >= %s" % args.min_kernel, vtuple(f["uname_r"]) >= vtuple(args.min_kernel), f["uname_r"])
    c.add("booted system = current system", f["booted_system"] == f["current_system"], f["booted_system"])
    c.add("kvm_intel nested=N", f["nested"] in ("N", "0"), f["nested"])
    c.add("SMT (informational)", True, "smt/active=%s, %s logical CPUs" % (f["smt_active"], f["cpus_online"]), False)
    c.add("hostname (informational)", True, f["hostname"], False)
    c.add("systemd state", f["system_state"] == "running", f["system_state"], blocking=f["system_state"] != "degraded")
    c.add("no failed units", not f["failed_units"], f["failed_units"])
    before = (pf.get("go_choir_units") or {})
    regressed = {u: s for u, s in f["go_choir_units"].items()
                 if not s.startswith("active") and before.get(u, "active").startswith("active")}
    c.add("go-choir units active again", not regressed, regressed or "%d units" % len(f["go_choir_units"]))
    c.add("md RAID arrays clean", f["md_clean"], f["md_arrays"])
    c.add("vmctl /health", f["vmctl_health"] == 200, f["vmctl_health"])
    c.add("firewall: FORWARD jumps vm-+ to CHOIR-GUEST-FWD first", f["fw_guest_jump_first"])
    c.add("firewall: CHOIR-GUEST-FWD fails closed (TAPS then DROP)", f["fw_guest_fwd_fails_closed"])
    c.add("firewall: CHOIR-GUEST-TAPS exists", f["fw_guest_taps_chain"])
    c.add("firewall: nixos-fw exists", f["fw_nixos_chain"])
    st = staging_health()
    receipt["post"]["staging"] = st
    c.add("staging /health ok", st.get("status") == "ok" and st.get("vmctl_status") == "ok", st)
    pst = pre.get("staging") or {}
    if pst.get("deployed_commit"):
        c.add("deploy identity unchanged", st.get("deployed_commit") == pst["deployed_commit"],
              "%s (before %s)" % (st.get("deployed_commit"), pst["deployed_commit"]))
    if not args.skip_hardening:
        h = hardening(args.host, args.hardening_args.split())
        receipt["post"]["hardening"] = h
        base = set(((pre.get("hardening") or {}).get("failed")) or [])
        new = sorted(set(h.get("failed", [])) - base) if "error" not in h else ["error: " + h["error"]]
        c.add("verify-host-hardening: no new failures", not new, new or "%s passed" % h.get("passed"))
    # Computers: auto-warm ones come back on their own; marked ones report
    # their boot kind.
    marked = set(((receipt.get("marks") or {}).get("marked")) or [])
    marked_at = iso_ts((receipt.get("marks") or {}).get("marked_at"))
    pre_running = {r["vm_id"]: r for r in running_computers(pre.get("computers") or {})}

    def warmed():
        comp = remote(args.host, "computers")
        want = [r for r in comp.get("computers", [])
                if r.get("warmness_class") in AUTO_WARM_CLASSES and not r.get("held")
                and (r["vm_id"] in pre_running or r.get("warmness_class") == "critical_protected")]
        return comp if all(r.get("state") == "active" and r.get("health") == 200 for r in want) else None

    comp = wait_for(warmed, args.computer_wait, 20) or remote(args.host, "computers")
    receipt["post"]["computers"] = comp
    if marked and marked_at:
        age = time.time() - marked_at
        c.add("markers were inside their 15-minute TTL when checked", age < 900,
              "%ds since marking; a computer that booted after 900s booted as a crash (safe fallback)" % age, False)
    for r in comp.get("computers", []):
        auto = r.get("warmness_class") in AUTO_WARM_CLASSES and not r.get("held")
        if r["vm_id"] in pre_running or r.get("warmness_class") == "critical_protected":
            kind = r.get("restart_kind")
            detail = "state=%s health=%s boot=%s marked=%s class=%s" % (
                r.get("state"), r.get("health"), kind, r["vm_id"] in marked, r.get("warmness_class"))
            if auto:
                c.add("computer %s back" % r["vm_id"], r.get("state") == "active" and r.get("health") == 200, detail)
                if r["vm_id"] in marked:
                    c.add("computer %s booted planned" % r["vm_id"], kind == "planned", detail, blocking=False)
            else:
                c.add("computer %s (boots on next request)" % r["vm_id"], True, detail, False)
    receipt["post"]["checks"] = c.items
    return c


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0],
                                 formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    ap.add_argument("--host", default="node-b", help="ssh host alias for Node B (root)")
    ap.add_argument("--phase", choices=["all", "pre", "post"], default="all")
    ap.add_argument("--execute", action="store_true", help="actually write markers and reboot (default: dry run)")
    ap.add_argument("--console-verified", action="store_true",
                    help="operator statement: the OVH IPMI/KVM console for this server was opened in this window")
    ap.add_argument("--allow-crash-close", action="store_true",
                    help="reboot even if some computers stay busy (their work ends interrupted_by_restart)")
    ap.add_argument("--no-markers", action="store_true",
                    help="write no planned-restart markers: every boot is a crash boot (the conservative choice)")
    ap.add_argument("--min-kernel", default=None,
                    help="lowest acceptable staged and booted kernel (default: nix/kernel-security-floor, the flake's floor)")
    ap.add_argument("--busy-wait", type=int, default=600, help="seconds to wait for busy computers to go idle")
    ap.add_argument("--boot-timeout", type=int, default=1200, help="seconds to wait for ssh after the reboot")
    ap.add_argument("--settle-wait", type=int, default=300, help="seconds for systemd and vmctl to settle")
    ap.add_argument("--computer-wait", type=int, default=600, help="seconds for always-on computers to come back")
    ap.add_argument("--hardening-args", default="", help="extra args for verify-host-hardening.py (e.g. '--expect-internal enforce')")
    ap.add_argument("--skip-hardening", action="store_true")
    ap.add_argument("--state", help="receipt JSON path (default: $TMPDIR/node-b-host-reboot-<UTC>.json); --phase post reads its pre baseline")
    args = ap.parse_args()
    if args.min_kernel is None:
        # Read after parsing so --help works without the file.
        args.min_kernel = kernel_security_floor()
    elif not re.fullmatch(r"\d+\.\d+\.\d+", args.min_kernel):
        # vtuple reads junk as (0,), which every kernel passes.
        ap.error("--min-kernel %r is not MAJOR.MINOR.PATCH" % args.min_kernel)

    tmp = os.environ.get("TMPDIR", "/tmp")
    state = Path(args.state or os.path.join(tmp, "node-b-host-reboot-%s.json" % datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")))
    receipt = {}
    if args.phase == "post" and state.exists():
        receipt = json.loads(state.read_text())
    receipt.setdefault("runs", []).append({"at": now(), "phase": args.phase, "execute": args.execute})

    def save():
        state.write_text(json.dumps(receipt, indent=2))
        log("receipt: %s" % state)

    try:
        if args.phase == "post":
            c = phase_post(args, receipt)
            save()
            return 1 if c.failed() else 0

        pre = phase_pre(args, receipt)
        if pre.failed():
            log("PRE-CHECKS FAILED: %s" % [x["check"] for x in pre.failed()])
            save()
            return 1
        if args.phase == "pre":
            save()
            return 0
        if not args.execute:
            running = [r["vm_id"] for r in running_computers(receipt["pre"]["computers"]) if not r.get("held")]
            log("DRY RUN. With --execute --console-verified this would:")
            if args.no_markers:
                log("  1. write no markers (--no-markers): every computer boots as a crash boot")
            else:
                log("  1. POST planned-restart markers to %d running computer(s): %s" % (len(running), running))
                log("     (busy ones retried for %ds, then abort unless --allow-crash-close)" % args.busy_wait)
            log("  2. ssh %s systemctl reboot; wait up to %ds for a new boot_id" % (args.host, args.boot_timeout))
            log("  3. run the post-checks (or later: --phase post --state %s)" % state)
            save()
            return 0
        if not args.console_verified:
            log("REFUSED: --execute needs --console-verified (open the OVH IPMI/KVM console first; runbook step 0)")
            save()
            return 2
        if not phase_mark(args, receipt):
            save()
            return 3
        save()
        if not phase_reboot(args, receipt):
            save()
            return 4
        save()
        c = phase_post(args, receipt)
        save()
        return 1 if c.failed() else 0
    except Exception as e:  # keep the receipt for the operator
        receipt["error"] = repr(e)
        save()
        raise


if __name__ == "__main__":
    sys.exit(main())
