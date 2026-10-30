#!/usr/bin/env python3
"""Paper liveness probe for Node B.

Run every 5 minutes by go-choir-paper-liveness-probe.timer. It alerts in the
journal when the paper computer made no inference call for PAPER_QUIET_MINUTES
(default 30), judged from the gateway journal line

    gateway: inference succeeded for autoputer vm-<id> (provider=...)

The paper is "healthy" only when its /health is ok AND its source schedules are
not overdue AND it made an inference call in the last 30 minutes. This probe
covers the third term. A healthy-but-idle paper (all schedules active, none
polled, nothing queued) passed the old health and disk check for 1h45m on
2026-10-15 (docs/archive/problems/paper-silent-since-10-56-after-host-reboot-2026-10-15.md).

The paper's VM id is looked up, not hardcoded: internal/computertiers/
computer-tiers.json lists the platform (critical_protected) computer ids, and
vmctl's ownerships.json maps a computer id to its vm_id. PAPER_VM_ID overrides.

Output (all to the journal):
  paper-liveness-probe: ok ...               healthy
  <3>paper-liveness-probe: ALERT ...         no paper inference call in the window
  paper-liveness-probe: skipped ...          host booted less than the window ago
  <3>paper-liveness-probe: probe-error ...   exit 1: lookup or journal read failed

An ALERT exits 0 (the timer keeps running and the unit is not "failed");
a probe-error exits 1 so `systemctl --failed` and verify-host-hardening show
that the probe itself is blind. The probe reads only; it restarts nothing.

Environment (tests and dry runs): PAPER_TIERS_FILE, OWNERSHIPS_FILE,
PAPER_VM_ID, PAPER_QUIET_MINUTES, GATEWAY_UNIT, PROC_UPTIME.
"""
import json
import os
import subprocess
import sys

NAME = "paper-liveness-probe"


def say(msg, level=None):
    prefix = f"<{level}>" if level is not None else ""
    print(f"{prefix}{NAME}: {msg}", flush=True)


def probe_error(msg):
    say(f"probe-error {msg}", level=3)
    return 1


def resolve_vm_ids(tiers_file, ownerships_file):
    """vm ids of the platform computers (the critical_protected class)."""
    with open(tiers_file) as f:
        tiers = json.load(f)
    computers = {c["computer_id"] for c in tiers.get("platform_computers", []) if c.get("computer_id")}
    if not computers:
        raise ValueError(f"{tiers_file} lists no platform_computers")
    with open(ownerships_file) as f:
        owned = json.load(f).get("ownerships", {})
    rows = owned.values() if isinstance(owned, dict) else owned
    vms = sorted({r["vm_id"] for r in rows if r.get("computer_id") in computers and r.get("vm_id")})
    if not vms:
        raise ValueError(f"no ownership in {ownerships_file} for platform computers {sorted(computers)}")
    return vms


def host_uptime_minutes():
    path = os.environ.get("PROC_UPTIME", "/proc/uptime")
    with open(path) as f:
        return float(f.read().split()[0]) / 60.0


def gateway_inference_lines(unit, vm_id, minutes):
    pattern = f"inference succeeded for autoputer {vm_id}"
    proc = subprocess.run(
        ["journalctl", "-u", unit, "--since", f"-{minutes}min", "--no-pager", "-o", "short-iso", "--grep", pattern],
        capture_output=True, text=True, check=False)
    # journalctl --grep exits 1 with no match; any other failure is a blind probe.
    if proc.returncode not in (0, 1):
        raise RuntimeError(f"journalctl exited {proc.returncode}: {proc.stderr.strip()[:200]}")
    return [line for line in proc.stdout.splitlines() if pattern in line]


def last_call_any_time(unit, vm_id):
    pattern = f"inference succeeded for autoputer {vm_id}"
    proc = subprocess.run(
        ["journalctl", "-u", unit, "--since", "-24h", "--no-pager", "-o", "short-iso", "--grep", pattern, "-n", "1", "-r"],
        capture_output=True, text=True, check=False)
    lines = [line for line in proc.stdout.splitlines() if pattern in line]
    return lines[0].split(" ")[0] if lines else "none in the last 24h"


def main():
    minutes = int(os.environ.get("PAPER_QUIET_MINUTES", "30"))
    unit = os.environ.get("GATEWAY_UNIT", "go-choir-gateway")
    try:
        vm_override = os.environ.get("PAPER_VM_ID", "").strip()
        vms = [vm_override] if vm_override else resolve_vm_ids(
            os.environ.get("PAPER_TIERS_FILE", "/etc/go-choir/computer-tiers.json"),
            os.environ.get("OWNERSHIPS_FILE", "/var/lib/go-choir/vm-state/ownerships.json"))
    except (OSError, ValueError, KeyError) as exc:
        return probe_error(f"cannot resolve the paper vm: {exc}")
    try:
        up = host_uptime_minutes()
    except (OSError, ValueError) as exc:
        return probe_error(f"cannot read host uptime: {exc}")
    if up < minutes:
        say(f"skipped: host up {up:.0f} min, less than the {minutes} min window")
        return 0
    try:
        counts = {vm: len(gateway_inference_lines(unit, vm, minutes)) for vm in vms}
    except (OSError, RuntimeError) as exc:
        return probe_error(f"cannot read the {unit} journal: {exc}")
    if any(counts.values()):
        say("ok " + " ".join(f"vm={vm} inference_calls_{minutes}m={n}" for vm, n in counts.items()))
        return 0
    for vm in vms:
        say(f"ALERT paper made no inference call for {minutes} min (vm={vm}, last call "
            f"{last_call_any_time(unit, vm)}); check /api/autopaper/sources/schedules for overdue "
            f"schedules and restart them with POST (owner act)", level=3)
    return 0


if __name__ == "__main__":
    sys.exit(main())
