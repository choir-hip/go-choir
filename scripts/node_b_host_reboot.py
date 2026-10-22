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
          the same request deploy-refresh makes). For critical_protected
          and public_platform computers it asks with allow_busy, vmctl's
          rule (deployRefreshAllowsBusy): their running runs park for the
          planned boot's one resume (CLAUDE.md: always-busy computers take
          a planned refresh while busy). Other busy guests refuse (409).
          Busy ones are retried until --busy-wait runs out. Each
          computer's worst answer across the rounds decides, not its last.
          A busy one aborts the run, unless --allow-crash-close accepts
          that its work ends "interrupted by a restart". A guest that
          cannot say whether an apply or a capsule is live (a 503, a
          timeout, no answer, another 5xx, a 404/405 from a guest without
          the endpoint) aborts the run unless --allow-crash-unverified
          names that risk. A guest that refuses for a self-development
          operation mid-transaction or a live capsule (any tier) aborts
          the run at once, whatever the flags. Then every marker is sent
          again immediately before the reboot: it refreshes the marker's
          15-minute TTL (plannedRestartMaxAge) and re-checks for a new
          apply or capsule; a computer that does not write again aborts
          the run. An abort or any failure clears the marker on every
          computer that may hold the host's (a 200 in any round, a lost
          answer, a 5xx); a computer that refused is not cleared, since
          the marker file is shared with the guest's own apply. A failed
          clear is logged and kept in the receipt. Each computer's outcome is logged and kept in the
          receipt; post reports the marker age. A marker that expires
          turns that boot into a crash boot, the safe fallback.
          --no-allow-busy sends no allow_busy (markers only to idle
          guests, the old rule). --no-markers sends no markers: every
          boot is a crash boot.
          Computers not asked (held ones, and every one under
          --no-markers) get a read-only check from their obligations
          surface: a self-development operation mid-transaction aborts
          the run; an unreadable count needs --allow-crash-unverified.
          The computers are listed again immediately before the final
          pass: unasked ones are checked again, and a computer that
          started running during the wait is asked in the final pass.
          A 404/405 guest is not asked again in later rounds. A
          WARNING line before the reboot names every unasked computer
          whose live capsules were not checked.
          Their live capsules cannot be read without asking and are not
          checked. Computers that are not running (hibernated, stopped)
          are not checked: the kernel change invalidates their
          snapshots, so each cold-boots as a crash boot at its next wake.
  reboot  (--execute) systemctl reboot. Waits for ssh with a new boot_id
          (--boot-timeout). If ssh reports that systemctl refused (an exit
          status other than 0 or ssh's own 255), ssh's stderr shows it
          never connected or authenticated, or the old boot_id still
          answers after --down-wait, the host did not go down: the markers
          are cleared and the run exits 3. Any failure after the markers
          are written (a failed receipt save, an interrupt, SIGHUP or
          SIGTERM) clears them. Exit 4 keeps them: the host may be
          rebooting.
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
aborted before the reboot, or the reboot was refused or the host did not
go down (markers cleared); 4 the host did not come back within
--boot-timeout (go to the console; see the runbook).
"""

import argparse
import json
import os
import re
import signal
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
# Classes asked with allow_busy: vmctl's deployRefreshAllowsBusy
# (internal/vmctl/deploy_refresh.go), the one rule; the test reads it from
# the Go source. vmctl list's warmness_class is the class that rule reads
# (WarmnessClassForOwnership).
BUSY_ALLOWED_CLASSES = {"critical_protected", "public_platform"}

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
    # The FORWARD order is judged on the workstation (forward_guest_jumps).
    out["fw_forward_rules"] = fw["FORWARD"]
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
                # Self-development operations mid-transaction: the same
                # states the marker refusal counts. None when the guest
                # could not read them or predates the section.
                sd = (ob.get("selfdev") or {}).get("busy") if isinstance(ob.get("selfdev"), dict) else None
                if any(str(e).startswith("selfdev:") for e in (ob.get("errors") or [])):
                    sd = None
                row["selfdev_busy"] = sd
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
        if method == "POST" and it.get("allow_busy"):
            # A query parameter, as vmctl sends it (httpGuestMarker.mark):
            # older guests ignore it and keep their busy refusal.
            url += "?allow_busy=1"
        body = {"target": target} if method == "POST" else None
        st, text = http(method, url, {"X-Internal-Caller": "true"}, body, timeout=25)
        results.append({"vm_id": it["vm_id"], "status": st, "body": text[:2000]})
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


def now(ts=None):
    t = datetime.fromtimestamp(ts, timezone.utc) if ts is not None else datetime.now(timezone.utc)
    return t.strftime("%Y-%m-%dT%H:%M:%SZ")


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


def marker_allows_busy(warmness_class, no_allow_busy=False):
    """Whether the marker request carries allow_busy: only the always-busy
    tiers (owner, 2026-10-15), never under --no-allow-busy."""
    return not no_allow_busy and warmness_class in BUSY_ALLOWED_CLASSES


def classify_marker_answer(status, body, sent_allow_busy):
    """One guest answer to POST planned-restart, as an outcome:
    written (200), busy (409 for running runs only: --allow-crash-close may
    cut them), unsafe (409 for a self-development operation mid-transaction
    or a live capsule, or a 409 the script cannot read: never rebooted
    through), unsupported (404/405: the guest predates the endpoint), or
    unavailable (anything else)."""
    out = {"status": status, "allow_busy": bool(sent_allow_busy)}
    try:
        doc = json.loads(body) if body else {}
    except ValueError:
        doc = None
    if not isinstance(doc, dict):
        doc = None
    if status == 200:
        out["outcome"] = "written"
        out["interrupted_running_runs"] = (doc or {}).get("interrupted_running_runs", 0)
        out["detail"] = "marker written%s" % (
            ", %d running run(s) park for the planned boot's one resume" % out["interrupted_running_runs"]
            if out["interrupted_running_runs"] else "")
        return out
    if status == 409:
        keys = ("running_runs", "selfdev_active_operations", "live_capsules")
        if doc is None or not all(isinstance(doc.get(k), int) for k in keys):
            out["outcome"] = "unsafe"
            out["detail"] = "refused, answer unreadable (counted as an apply or a live capsule): %s" % (body or "")[:160]
            return out
        for k in keys:
            out[k] = doc[k]
        if doc["selfdev_active_operations"] or doc["live_capsules"]:
            out["outcome"] = "unsafe"
            out["detail"] = "refused: %d self-development operation(s) mid-transaction, %d live capsule(s), %d running run(s)" % (
                doc["selfdev_active_operations"], doc["live_capsules"], doc["running_runs"])
            return out
        out["outcome"] = "busy"
        out["detail"] = "busy: %d running run(s)" % doc["running_runs"]
        if sent_allow_busy and not doc.get("allow_busy"):
            out["detail"] += " (the guest ignored allow_busy: an older build)"
        return out
    if status in (404, 405):
        out["outcome"] = "unsupported"
        out["detail"] = "guest predates the planned-restart endpoint (HTTP %s)" % status
        return out
    out["outcome"] = "unavailable"
    out["detail"] = "HTTP %s %s" % (status, (body or "")[:160])
    return out


# How bad an outcome is for the abort decision; each computer keeps its worst
# across rounds (F2). An outcome not listed counts as unsafe.
SEVERITY = {"written": 0, "unasked": 0, "busy": 1, "unsupported": 2, "unavailable": 2, "unsafe": 3}
# The flag that accepts crashing through an outcome; unsafe has none.
CRASH_FLAG = {"busy": "--allow-crash-close", "unsupported": "--allow-crash-unverified",
              "unavailable": "--allow-crash-unverified"}


def worse(a, b):
    if a is None:
        return b
    return a if SEVERITY.get(a, 3) >= SEVERITY.get(b, 3) else b


def reboot_blockers(standing, allow_crash_close, allow_crash_unverified):
    """The computers whose outcome stops the reboot: unsafe always; busy
    without --allow-crash-close; unsupported or unavailable (the guest did
    not say whether an apply or a capsule is live) without
    --allow-crash-unverified; anything unknown."""
    allowed = {"written", "unasked"}
    if allow_crash_close:
        allowed.add("busy")
    if allow_crash_unverified:
        allowed |= {"unsupported", "unavailable"}
    return {vm: o for vm, o in standing.items() if o not in allowed}


def classify_unasked(r):
    """A running computer the script does not ask (held, or --no-markers):
    the read-only self-development count from its obligations surface.
    unsafe for an operation mid-transaction, unavailable when the count
    cannot be read, else unasked (boots as a crash; live capsules are not
    readable without asking, so they are not checked)."""
    sd = r.get("selfdev_busy")
    out = {"warmness_class": r.get("warmness_class"), "held": bool(r.get("held")), "selfdev_busy": sd,
           "running_runs": (r.get("runs") or {}).get("running")}
    if not isinstance(sd, int) or isinstance(sd, bool) or sd < 0:
        out["outcome"] = "unavailable"
        out["detail"] = "not asked; its obligations surface does not say whether a self-development operation is mid-transaction (%s)" % (
            "HTTP %s" % r["obligations_status"] if r.get("obligations_status") else "selfdev count %r" % (sd,))
    elif sd > 0:
        out["outcome"] = "unsafe"
        out["detail"] = "not asked; %d self-development operation(s) mid-transaction" % sd
    else:
        out["outcome"] = "unasked"
        out["detail"] = ("not asked; no self-development operation mid-transaction; live capsules not checked "
                         "(not readable without asking); boots as a crash, %s running run(s) end interrupted_by_restart" % out["running_runs"])
    return out


def answer_may_have_written(status, body):
    """Whether a POST answer leaves the host's marker possibly on disk: a
    200 wrote it; a lost answer (0) or a 5xx (a write that failed after the
    rename) may have. Every other answer is a refusal before the write
    (D1), and so are the guest's own two 503s, both sent before
    writeHostPlannedRestartMarker (HandleInternalPlannedRestart):
    {"error":"planned restart surface unavailable"} and
    {"error":"planned restart: <err>"} when plannedRestartBusy cannot
    read its state (R1). A 503 with any other body (a proxy, an older
    guest) may have written."""
    if status == 200 or status == 0:
        return True
    if status == 503:
        try:
            err = (json.loads(body) or {}).get("error") if body else None
        except (ValueError, AttributeError):
            err = None
        if isinstance(err, str) and (err == "planned restart surface unavailable" or err.startswith("planned restart: ")):
            return False
        return True
    return status >= 500


# ssh's stderr when it never connected or authenticated, so the remote
# command never ran (R2). A refused or timed-out connect reads "ssh:
# connect to host <h> port 22: ..."; a bare "Connection timed out" or "No
# route to host" can also follow "Read from remote host" mid-session, and
# "Connection to <h> closed by remote host" is the session dropping as
# the host goes down, so neither is here.
SSH_NOT_SENT = ("connect to host", "Permission denied", "Could not resolve hostname", "Host key verification failed",
                "kex_exchange_identification", "Too many authentication failures")


def ssh_never_connected(stderr):
    return any(p in (stderr or "") for p in SSH_NOT_SENT)


class Interrupted(KeyboardInterrupt):
    """SIGHUP or SIGTERM, raised like Ctrl-C so the clear paths run (R3)."""


def install_signal_handlers():
    def handler(signum, frame):
        raise Interrupted("signal %d" % signum)
    for sig in (signal.SIGHUP, signal.SIGTERM):
        signal.signal(sig, handler)


def mark_timeout(n):
    """ssh timeout for one mark or unmark call: each guest may take 25 s."""
    return 60 + 30 * n


def computers_timeout(n):
    """ssh timeout for the computers call: an 8 s health and a 15 s
    obligations request per running guest, in turn."""
    return max(180, 60 + 25 * n)


def list_computers(host, n):
    return remote(host, "computers", timeout=computers_timeout(n))


def forward_guest_jumps(rules):
    """FORWARD must begin with the two guest jumps in the order vmctl
    installs them at start (ensureBaseLocked, internal/vmmanager/
    hostfirewall.go: the CHOIR-GUEST-FWD jump goes in at position 1, then
    the CHOIR-GUEST-RET jump at position 1): rule 1 `-o vm-+ ... -j
    CHOIR-GUEST-RET`, rule 2 `-i vm-+ ... -j CHOIR-GUEST-FWD`. Anything
    else, an unreadable chain included, fails."""
    if rules is None:
        return False, "iptables -S FORWARD unreadable"
    fwd = [r for r in rules if r.startswith("-A FORWARD")]
    want = [("-o vm-+", "-j CHOIR-GUEST-RET"), ("-i vm-+", "-j CHOIR-GUEST-FWD")]
    got = fwd[:2]
    ok = len(got) == 2 and all((" %s " % iface) in (" %s " % r) and r.endswith(jump)
                               for r, (iface, jump) in zip(got, want))
    return ok, got or "no FORWARD rules"


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
    c.add("firewall: FORWARD begins vm-+ CHOIR-GUEST-RET then CHOIR-GUEST-FWD", *forward_guest_jumps(f.get("fw_forward_rules")),
          blocking=False)
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
                  "class=%s held=%s health=%s runs=%s selfdev_busy=%s auto-warm=%s" % (
                      r.get("warmness_class"), bool(r.get("held")), r.get("health"), r.get("runs"),
                      r.get("selfdev_busy"), r.get("warmness_class") in AUTO_WARM_CLASSES), False)
            if busy and marker_allows_busy(r.get("warmness_class"), args.no_allow_busy):
                c.add("computer %s busy, asked with allow_busy" % r["vm_id"], True,
                      "%d running run(s) park for the planned boot's one resume; a self-development apply "
                      "or a live capsule still refuses" % busy, False)
            elif busy:
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
    n_pre = len(running_computers((receipt.get("pre") or {}).get("computers") or {}))
    comp = list_computers(args.host, n_pre)
    target = "host-reboot kernel %s" % receipt["pre"]["facts"]["current_kernel"]
    m = receipt["marks"] = {"target": target, "skipped": bool(args.no_markers), "sent": [], "sent_items": [],
                            "may_hold_marker": [], "in_flight": [],
                            "marked": [], "unmarked": [], "marked_at": None, "outcomes": {}, "worst": {}, "unasked": {}}
    if "error" in comp:
        log("ABORT: vmctl computer list unreadable: %s" % comp["error"])
        m["error"] = comp["error"]
        return False
    if args.no_markers:
        log("MARK: --no-markers: no markers; every computer boots as a crash boot; parked work closes interrupted_by_restart")
    items = {}
    split_computers(args, m, comp, items)
    blocked = reboot_blockers({vm: a["outcome"] for vm, a in m["unasked"].items()},
                              args.allow_crash_close, args.allow_crash_unverified)
    if blocked:
        abort(args, m, blocked, {})
        return False
    sent = {}
    try:
        return mark_rounds(args, m, items, sent, target)
    except BaseException as e:
        # An ssh drop or timeout may lose answers from guests that wrote:
        # clear every computer that may hold a marker, then fail.
        m["error"] = repr(e)
        # A call that failed in flight may have written on any guest in it.
        m["may_hold_marker"] = sorted(set(m["may_hold_marker"]) | set(m["in_flight"]))
        log("MARK FAILED (%r): clearing the marker on every computer that may hold one" % e)
        clear_markers(args, m, sent)
        raise


def split_computers(args, m, comp, items):
    """Sort a vmctl listing: running computers the script does not ask
    (held, or all under --no-markers) get the read-only check, kept fresh
    in m["unasked"]; every other running computer joins items (asked).
    Returns the vm_ids newly added to items."""
    m["unasked"] = {}
    added = []
    for r in running_computers(comp):
        vm = r["vm_id"]
        if args.no_markers or r.get("held"):
            a = classify_unasked(r)
            m["unasked"][vm] = a
            log("  %s %s (class=%s held=%s): %s" % (a["outcome"], vm, a["warmness_class"], a["held"], a["detail"]))
        elif vm not in items:
            items[vm] = {"vm_id": vm, "computer_url": r["computer_url"], "warmness_class": r.get("warmness_class"),
                         "allow_busy": marker_allows_busy(r.get("warmness_class"), args.no_allow_busy)}
            added.append(vm)
    return added


def mark_rounds(args, m, items, sent, target):
    outcomes, worst = m["outcomes"], m["worst"]

    def ask(batch):
        for it in batch:
            sent[it["vm_id"]] = it
        m["sent"] = sorted(sent)
        m["sent_items"] = [{"vm_id": vm, "computer_url": sent[vm]["computer_url"]} for vm in sorted(sent)]
        sent_at = time.time()
        m["in_flight"] = sorted(it["vm_id"] for it in batch)
        res = remote(args.host, "mark", json.dumps({"items": batch, "target": target}), timeout=mark_timeout(len(batch)))
        m["in_flight"] = []
        got = {r["vm_id"]: r for r in res if r.get("vm_id") in items}
        answers = {}
        for it in batch:
            vm = it["vm_id"]
            r = got.get(vm, {"status": 0, "body": "no answer in the remote result"})
            a = classify_marker_answer(r["status"], r["body"], it["allow_busy"])
            a["warmness_class"] = it["warmness_class"]
            if answer_may_have_written(r["status"], r["body"]):
                m["may_hold_marker"] = sorted(set(m["may_hold_marker"]) | {vm})
            if a["outcome"] == "written":
                # Stamped when the request went out: the guest's RequestedAt
                # is no earlier, so the age post reports is no smaller.
                a["written_at"] = now(sent_at)
            outcomes[vm] = a
            worst[vm] = worse(worst.get(vm), a["outcome"])
            answers[vm] = a
            log("  %s %s (class=%s allow_busy=%s): %s" % (a["outcome"], vm, a["warmness_class"], a["allow_busy"], a["detail"]))
        return answers, sent_at

    pending, written = dict(items), set()
    deadline = time.time() + args.busy_wait
    log("MARK: asking %d running computer(s) for planned-restart markers (target %r; allow_busy for %s)" % (
        len(items), target, "none (--no-allow-busy)" if args.no_allow_busy else sorted(BUSY_ALLOWED_CLASSES)))
    while pending:
        answers, _ = ask(list(pending.values()))
        for vm, a in answers.items():
            if a["outcome"] == "written":
                written.add(vm)
                pending.pop(vm)
            elif a["outcome"] == "unsupported":
                pending.pop(vm)  # the guest lacks the endpoint: asking again cannot change it
        if "unsafe" in worst.values():
            break  # never rebooted through; waiting cannot change the decision
        if not pending or time.time() >= deadline:
            break
        time.sleep(15)

    def standing():
        st = {vm: ("written" if vm in written and worst[vm] != "unsafe" else worst[vm]) for vm in items if vm in worst}
        for vm, a in m["unasked"].items():
            st[vm] = worse(st.get(vm), a["outcome"])
        return st

    blocked = reboot_blockers(standing(), args.allow_crash_close, args.allow_crash_unverified)
    if blocked:
        abort(args, m, blocked, sent)
        return False
    # List the computers again immediately before the final pass: a held
    # (or, under --no-markers, any) computer may have started an apply
    # during the wait, and a computer woken meanwhile must be asked too.
    n = len(items) + len(m["unasked"])
    comp = list_computers(args.host, n)
    if "error" in comp:
        log("vmctl computer list unreadable before the final pass: %s" % comp["error"])
        abort(args, m, {"vmctl list": "unreadable"}, sent)
        return False
    added = split_computers(args, m, comp, items)
    if added:
        log("MARK: %d computer(s) started running during the wait; asking them in the final pass: %s" % (len(added), added))
    blocked = reboot_blockers(standing(), args.allow_crash_close, args.allow_crash_unverified)
    if blocked:
        abort(args, m, blocked, sent)
        return False
    if items:
        # Final pass, immediately before the reboot: every marker is
        # written again (a fresh 15-minute TTL) and the guest re-checks for
        # an apply or a capsule that started since. A computer marked
        # earlier must write again.
        log("MARK: final pass: sending every marker again immediately before the reboot")
        before = set(written)
        answers, sent_at = ask(list(items.values()))
        written = {vm for vm, a in answers.items() if a["outcome"] == "written"}
        lost = {vm: answers[vm]["outcome"] for vm in before - written}
        blocked = reboot_blockers(standing(), args.allow_crash_close, args.allow_crash_unverified)
        if lost or blocked:
            for vm, o in lost.items():
                if blocked.get(vm) != "unsafe":
                    blocked[vm] = "marker not renewed in the final pass (answered %s)" % o
            abort(args, m, blocked, sent)
            return False
        m["marked_at"] = now(sent_at)
    m["marked"] = sorted(written)
    m["unmarked"] = sorted(set(items) - written)
    log("MARK outcomes:")
    for vm, a in outcomes.items():
        log("  %s: %s (worst %s), class=%s, allow_busy=%s, %s" % (
            vm, a["outcome"], worst[vm], a["warmness_class"], a["allow_busy"], a["detail"]))
    for vm in m["unmarked"]:
        o = worst[vm]
        risk = (" Its guest did not say whether a self-development apply or a live capsule is running; the reboot may cut one."
                if o in ("unsupported", "unavailable") else "")
        log("%s: rebooting %s unmarked (worst outcome %s); its work ends interrupted_by_restart.%s" % (
            CRASH_FLAG.get(o, "?"), vm, o, risk))
    for vm, a in m["unasked"].items():
        log("%s: rebooting %s unasked (%s): %s" % (
            "--allow-crash-unverified" if a["outcome"] == "unavailable" else ("--no-markers" if args.no_markers else "held"),
            vm, a["outcome"], a["detail"]))
    if m["unasked"]:
        log("WARNING: %d unasked computer(s), live capsules not checked: the reboot destroys any they hold: %s" % (
            len(m["unasked"]), sorted(m["unasked"])))
    return True


def abort(args, m, blocked, sent):
    for vm, o in sorted(blocked.items()):
        flag = CRASH_FLAG.get(o)
        why = ("%s would accept it" % flag if flag else
               "no flag reboots through a self-development apply or a live capsule" if o == "unsafe" else
               "a computer marked earlier must write again; rerun once it answers" if o.startswith("marker not renewed") else
               "no flag accepts it")
        log("ABORT: %s: %s (%s)" % (vm, o, why))
    log("ABORT: %d computer(s) stop the reboot. Clearing the marker on every computer that may hold one." % len(blocked))
    m["blocked"] = blocked
    clear_markers(args, m, sent)


def clear_receipt_markers(args, receipt):
    """clear_markers for every computer the receipt says was POSTed to."""
    m = receipt.get("marks") or {}
    sent = {it["vm_id"]: it for it in m.get("sent_items") or []}
    if sent:
        log("Clearing the markers: the host did not reboot")
    return clear_markers(args, m, sent)


def clear_markers(args, m, sent):
    """DELETE the marker on every computer POSTed to that may hold the
    host's marker (m["may_hold_marker"]: a 200 in any round, a lost answer,
    a 5xx, a call that failed in flight), and report each failure. A
    computer whose answer was a refusal is not cleared: the host wrote
    nothing there, and the marker file is shared with the guest's own
    self-development apply (markPlannedRestart), which a DELETE would
    clear (D1)."""
    possible = set(m.get("may_hold_marker") or [])
    skipped = sorted(vm for vm in sent if vm not in possible)
    if skipped:
        log("Not clearing %s: each refused before writing, and a DELETE could clear the guest's own apply marker" % skipped)
    sent = {vm: it for vm, it in sent.items() if vm in possible}
    items = list(sent.values())
    m["cleared"], m["unmark_failed"] = True, {}
    if not items:
        return True
    log("Clearing markers on %d computer(s): %s" % (len(items), sorted(sent)))
    try:
        res = remote(args.host, "unmark", json.dumps({"items": items}), timeout=mark_timeout(len(items)))
        answered = {r.get("vm_id"): r for r in res}
        failed = {vm: ("HTTP %s %s" % (answered[vm].get("status"), (answered[vm].get("body") or "")[:160])
                       if vm in answered else "no answer")
                  for vm in sent if vm not in answered or answered[vm].get("status") != 200}
    except Exception as e:
        failed = {vm: "unmark failed: %r" % e for vm in sent}
    m["unmark_failed"], m["cleared"] = failed, not failed
    for vm, why in sorted(failed.items()):
        log("WARNING: marker on %s may remain (%s). For up to 15 minutes a crash of this computer boots as planned "
            "and resumes work: clear it by hand (DELETE <computer_url>/internal/runtime/planned-restart)." % (vm, why))
    return not failed


def reboot_after_mark(args, receipt, save):
    """Everything after the markers are written: save the receipt, then
    reboot. Any failure clears every host marker (a crash in the next 15
    minutes must not boot as planned); clearing a computer that already
    rebooted is harmless."""
    try:
        save()
        return phase_reboot(args, receipt)
    except BaseException as e:
        log("FAILED after marking (%r): clearing the markers" % e)
        clear_receipt_markers(args, receipt)
        raise


def phase_reboot(args, receipt):
    """0 the host is back with a new boot_id; 3 it did not go down (the
    reboot was refused, or the old boot_id still answers after
    --down-wait), markers cleared; 4 it did not come back in time."""
    before = receipt["pre"]["facts"]["boot_id"]
    log("REBOOT: systemctl reboot on %s (boot_id %s)" % (args.host, before))
    try:
        p = subprocess.run(["ssh", *SSH_OPTS, args.host, "systemctl", "reboot"], capture_output=True, text=True, timeout=60)
        rc, err = p.returncode, (p.stderr or "").strip()
    except subprocess.TimeoutExpired as e:
        # Usually the connection hanging as the host goes down.
        err = e.stderr.decode("utf-8", "replace") if isinstance(e.stderr, bytes) else (e.stderr or "")
        rc, err = None, err.strip()
    receipt["reboot"] = {"requested_at": now(), "boot_id_before": before, "ssh_exit": rc, "ssh_stderr": err[-300:]}
    if rc in (255, None) and ssh_never_connected(err):
        log("REBOOT NOT SENT: ssh never reached %s (%s). The host did not go down." % (args.host, err[-300:]))
        receipt["reboot"]["not_sent"] = True
        clear_receipt_markers(args, receipt)
        return 3
    if rc not in (0, 255, None):
        # 255 is ssh's own error (the connection dropping as the host goes
        # down); any other status is systemctl's answer: it refused.
        log("REBOOT REFUSED: ssh systemctl reboot exited %s: %s. The host did not go down." % (rc, err[-300:]))
        receipt["reboot"]["refused"] = True
        clear_receipt_markers(args, receipt)
        return 3
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
            return 0
        if bid == before and time.time() - t0 >= args.down_wait:
            log("HOST DID NOT GO DOWN: boot_id %s still answers %ds after the reboot request. Clearing the markers; "
                "if the host goes down later every computer boots as a crash (safe); run --phase post after it." % (
                    before, time.time() - t0))
            receipt["reboot"]["did_not_go_down"] = True
            clear_receipt_markers(args, receipt)
            return 3
        time.sleep(10)
    receipt["reboot"]["timed_out"] = True
    log("HOST DID NOT RETURN within %ds. Open the OVH IPMI/KVM console (runbook: Rollback)." % args.boot_timeout)
    return 4


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
    c.add("firewall: FORWARD begins vm-+ CHOIR-GUEST-RET then CHOIR-GUEST-FWD", *forward_guest_jumps(f.get("fw_forward_rules")))
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
        comp = list_computers(args.host, len(pre_running))
        want = [r for r in comp.get("computers", [])
                if r.get("warmness_class") in AUTO_WARM_CLASSES and not r.get("held")
                and (r["vm_id"] in pre_running or r.get("warmness_class") == "critical_protected")]
        return comp if all(r.get("state") == "active" and r.get("health") == 200 for r in want) else None

    comp = wait_for(warmed, args.computer_wait, 20) or list_computers(args.host, len(pre_running))
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
                    help="reboot even if some computers stay busy with runs (their work ends interrupted_by_restart)")
    ap.add_argument("--allow-crash-unverified", action="store_true",
                    help="also reboot through computers that could not say whether a self-development apply or a "
                         "live capsule is running (no answer, 503, other 5xx, 404/405, unreadable obligations): "
                         "the reboot may cut an apply or destroy a capsule. It does not cover a computer that wrote "
                         "its marker earlier and does not write again in the final pass: that always aborts")
    ap.add_argument("--no-allow-busy", action="store_true",
                    help="never send allow_busy: busy critical_protected and public_platform computers refuse "
                         "their marker like any other (the behaviour before 2026-10-22)")
    ap.add_argument("--no-markers", action="store_true",
                    help="write no planned-restart markers: every boot is a crash boot (the conservative choice)")
    ap.add_argument("--min-kernel", default=None,
                    help="lowest acceptable staged and booted kernel (default: nix/kernel-security-floor, the flake's floor)")
    ap.add_argument("--busy-wait", type=int, default=600, help="seconds to wait for busy computers to go idle")
    ap.add_argument("--boot-timeout", type=int, default=1200, help="seconds to wait for ssh after the reboot")
    ap.add_argument("--down-wait", type=int, default=300,
                    help="seconds the old boot_id may keep answering after the reboot request before the run "
                         "decides the host did not go down (markers cleared, exit 3)")
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
                log("  1. write no markers (--no-markers): every computer boots as a crash boot; a computer with a "
                    "self-development operation mid-transaction (obligations surface) aborts the run")
            else:
                rows = [r for r in running_computers(receipt["pre"]["computers"]) if not r.get("held")]
                log("  1. POST planned-restart markers to %d running computer(s):" % len(running))
                for r in rows:
                    log("       %s class=%s allow_busy=%s" % (r["vm_id"], r.get("warmness_class"),
                                                            marker_allows_busy(r.get("warmness_class"), args.no_allow_busy)))
                log("     (busy ones retried for %ds, then abort unless --allow-crash-close; no answer, a 503 or a "
                    "guest without the endpoint aborts unless --allow-crash-unverified; a refusal for a "
                    "self-development apply or a live capsule always aborts; then every marker is sent again "
                    "immediately before the reboot)" % args.busy_wait)
                held = [r["vm_id"] for r in running_computers(receipt["pre"]["computers"]) if r.get("held")]
                if held:
                    log("     held, not asked (read-only selfdev check only; live capsules unchecked): %s" % held)
            log("  2. ssh %s systemctl reboot; wait up to %ds for a new boot_id (a refused reboot, or the old boot_id "
                "still answering after %ds, clears the markers and exits 3)" % (args.host, args.boot_timeout, args.down_wait))
            log("  3. run the post-checks (or later: --phase post --state %s)" % state)
            save()
            return 0
        if not args.console_verified:
            log("REFUSED: --execute needs --console-verified (open the OVH IPMI/KVM console first; runbook step 0)")
            save()
            return 2
        # From here markers may be written: a closed terminal or a kill
        # must reach the clear paths too.
        install_signal_handlers()
        if not phase_mark(args, receipt):
            save()
            return 3
        code = reboot_after_mark(args, receipt, save)
        save()
        if code:
            return code
        c = phase_post(args, receipt)
        save()
        return 1 if c.failed() else 0
    except Exception as e:  # keep the receipt for the operator
        receipt["error"] = repr(e)
        save()
        raise


if __name__ == "__main__":
    sys.exit(main())
