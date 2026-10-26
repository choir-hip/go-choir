#!/usr/bin/env python3
"""D7 density acceptance: five test computers started in a row on a host that
cannot hold them all resident, with no hand stop (docs/definitions/
choir-density-2026-10-26.md acceptance; choir-consolidate-and-scale-metamission
finish.acceptance). Creates test computers; run it on staging only, in a quiet
window, from a checkout that has frontend/node_modules.

Without --run it prints the plan and the preflight (host MemAvailable, vmctl
health counts, admission settings, the protected set) and changes nothing.

With --run it:
  1. mints --n (default 5) test computers in a row with
     scripts/qa_test_computer.mjs (labels d7-accept-<run>-<i>), waiting for
     each to report active before the next. It never stops anything by hand:
     admission is meant to make room by hibernating idle computers (D1).
  2. for each start records the vmctl health admission.recent decisions newer
     than the run start (decision, the hibernated VM ids and their warmness
     class), MemAvailable before and after, the wall time to active, and the
     snapshot bytes of every computer hibernated to make room (vmctl's
     hibernate timeline, and du on that one VM's snapshot dir; never the
     artifacts root).
  3. (--wake, default on) after an idle pause (--idle-s, default 60) wakes
     each computer that was hibernated to make room, through the product API
     with that computer's own key (`choir computer start`), recording the wall
     time and vmctl's boot timeline (snapshot restore versus cold boot). Only
     computers this run minted are woken: a victim must be test class and
     have a key from this run.
  4. writes DIR/accept-five-<run>.json and a short summary: none refused,
     the number of make-room hibernations, every wake under 1 s (expected to
     fail until D2 lands; the times are reported either way) and the
     MemAvailable series.
  5. leaves the minted computers to idle hibernation (the product behavior
     under test) and records their ids.

Guards: lib.refusal (vmctl warmness class "test" only) plus the protected set:
the delegate and the owner computer (computer-03335285...), the paper by class,
and any --protect-id. Stops: memory PSI some avg10 > 5, a protected or
non-test computer that was active leaving active (a bug worth a receipt), a
minted computer that does not classify as safe, or a make-room victim that is
not test class. There is no MemAvailable floor stop: the floor is vmctl's.
The internal token header is read on the host and sent only to localhost
vmctl (lib.Host); the API key of a minted computer is read from its 0600
credential file, handed to the choir CLI through its environment, and never
printed or written to the receipt.

usage: accept_five.py [--ssh node-b | --local] [--run] [--n 5] [--idle-s 60]
         [--no-wake] [--protect-id COMPUTER_ID ...] [--out receipts]
         [--run-id ID] [--repo-root DIR] [--choir-bin /tmp/choir]
         [--canned FILE]   (offline plan and preflight from canned JSON)
"""
import argparse
import calendar
import json
import math
import os
import re
import shlex
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402
from lib import GIB, Host, ownerships, refusal  # noqa: E402

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
# The owner's computer (docs/mission-graph.yaml); with lib.DELEGATE_COMPUTER_ID
# these two are refused even if a misconfiguration ever classified them as test.
OWNER_COMPUTER_ID = "computer-03335285269bdba4f94377e56879f9e6"
STATE_DIR = "/var/lib/go-choir/vm-state"  # host_collect.STATE_DIR; snapshots live in <STATE_DIR>/<vm_id>/snapshot
PSI_STOP = 5.0
WAKE_BUDGET_MS = 1000
MEMORY_GROW_MODE = "memory_grow"  # hotplug grow decisions share admission.recent; they are not starts
SAFE_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*$")


class Stop(Exception):
    """A stop condition: ends the run, and the receipt records why."""


def log(event, **kv):
    print(json.dumps({"t": time.strftime("%H:%M:%S", time.gmtime()), "event": event, **kv}), file=sys.stderr, flush=True)


# ---- pure pieces (tested offline) ------------------------------------------

def protected_ids(extra=()):
    return [lib.DELEGATE_COMPUTER_ID, OWNER_COMPUTER_ID, *extra]


ISO = re.compile(r"^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(\.\d+)?(Z|[+-]\d\d:\d\d)?$")


def parse_ts(s):
    """RFC3339 (with or without fractions and an offset) to epoch seconds; None if it is not one."""
    m = ISO.match(s or "")
    if not m:
        return None
    t = calendar.timegm(time.strptime(m.group(1), "%Y-%m-%dT%H:%M:%S")) + (float(m.group(2)) if m.group(2) else 0)
    off = m.group(3)
    if off and off != "Z":
        t -= (1 if off[0] == "+" else -1) * (int(off[1:3]) * 3600 + int(off[4:6]) * 60)
    return t


def fmt_ts(t):
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t))


def psi_stop(snap, limit=PSI_STOP):
    psi = ((snap.get("psi_memory") or {}).get("some") or {}).get("avg10", 0)
    return f"memory PSI some avg10 {psi} > {limit}" if psi > limit else ""


def protected_active(listing, pids):
    return lib.non_test_active(listing, pids)


def left_active(before, listing):
    now = {o.get("computer_id") for o in ownerships(listing) if o.get("state") == "active"}
    return set(before) - now


def require_minted_safe(own, pids):
    why = refusal(own, pids)
    if why:
        raise Stop(f"minted computer {(own or {}).get('computer_id', '?')} is not safe to start: {why}")


def fresh_decisions(health, since_ts, seen):
    """admission.recent entries at or after since_ts not already in seen (which is updated)."""
    floor = math.floor(since_ts or 0)
    out = []
    for d in ((health or {}).get("admission") or {}).get("recent") or []:
        t = parse_ts(d.get("at"))
        key = json.dumps(d, sort_keys=True)
        if t is None or t < floor or key in seen:
            continue
        seen.add(key)
        out.append(d)
    return out


def start_decisions(decisions, vm_id):
    return [d for d in decisions if d.get("vm_id") == vm_id and d.get("mode") != MEMORY_GROW_MODE]


def refusals(decisions, vm_id):
    return [d for d in start_decisions(decisions, vm_id) if str(d.get("decision", "")).startswith("refuse")]


def made_room_ids(decisions):
    out = []
    for d in decisions:
        for v in d.get("made_room_vm_ids") or []:
            if v not in out:
                out.append(v)
    return out


def find_by_vm(listing, vm_id):
    return next((o for o in ownerships(listing) if o.get("vm_id") == vm_id), None)


def victim_view(vm_id, listing, pids, minted_computers):
    own = find_by_vm(listing, vm_id)
    if not own:
        return {"vm_id": vm_id, "computer_id": "", "warmness_class": "", "state": "", "minted": False,
                "safe": False, "refusal": "not in vmctl's list"}
    why = refusal(own, pids)
    return {"vm_id": vm_id, "computer_id": own.get("computer_id", ""), "warmness_class": own.get("warmness_class", ""),
            "state": own.get("state", ""), "minted": own.get("computer_id") in set(minted_computers),
            "safe": not why, "refusal": why}


def wake_candidates(victims, listing, pids):
    """(candidates, skipped): a victim is woken only if it is test class (not protected),
    was minted by this run (so a key exists), and is hibernated now."""
    cands, skipped, seen = [], [], set()
    for v in victims:
        if v["vm_id"] in seen:
            continue
        seen.add(v["vm_id"])
        own = find_by_vm(listing, v["vm_id"])
        why = ""
        if not v.get("safe") or (own and refusal(own, pids)):
            why = "refused: " + (v.get("refusal") or (refusal(own, pids) if own else "unknown"))
        elif not v.get("minted"):
            why = "not minted by this run (no credential)"
        elif not own or own.get("state") != "hibernated":
            why = f"not hibernated now (state {(own or {}).get('state', 'unknown')!r})"
        if why:
            skipped.append({"vm_id": v["vm_id"], "computer_id": v.get("computer_id", ""), "why": why})
        else:
            cands.append(v)
    return cands, skipped


def safe_vm_id(vm_id):
    return bool(vm_id) and bool(SAFE_ID.match(vm_id)) and vm_id not in (".", "..")


def snapshot_dir(vm_id):
    if not safe_vm_id(vm_id):
        raise ValueError(f"unsafe vm id {vm_id!r}")
    return f"{STATE_DIR}/{vm_id}/snapshot"


def parse_du(out):
    m = re.match(r"^\s*(\d+)\s", out or "")
    return int(m.group(1)) if m else None


def timeline_facts(tl):
    if not tl or tl.get("error"):
        return {}
    snap = tl.get("snapshot") or {}
    keep = {"boot_kind": tl.get("boot_kind"), "outcome": tl.get("outcome"), "started_at": tl.get("started_at"),
            "total_ms": tl.get("total_ms"), "first_healthy_offset_ms": tl.get("first_healthy_offset_ms"),
            "snapshot_outcome": snap.get("outcome"), "snapshot_reason": snap.get("reason"),
            "load_ms": snap.get("load_ms"), "wake_to_healthy_ms": snap.get("wake_to_healthy_ms"),
            "snapshot_bytes": snap.get("bytes"), "snapshot_disk_bytes": snap.get("disk_bytes"),
            "snapshot_type": snap.get("snapshot_type"), "mode": snap.get("mode"), "created_at": snap.get("created_at")}
    return {k: v for k, v in keep.items() if v not in (None, "")}


def restore_kind(tl, since_ts):
    """resume (snapshot restore), cold, fallback_cold, or unknown (no timeline, or one older than the call)."""
    if not tl or tl.get("error"):
        return "unknown"
    t = parse_ts(tl.get("started_at"))
    if t is None or t < since_ts - 5:
        return "unknown"
    if (tl.get("snapshot") or {}).get("outcome") == "fallback_cold":
        return "fallback_cold"
    kind = str(tl.get("boot_kind") or "").lower()
    return "resume" if kind == "resume" else (kind or "unknown")


def wake_argv(choir_bin, computer_id, idem_key):
    # Flags before positionals; `computer start` takes no positional.
    return [choir_bin, "computer", "start", "--computer", computer_id, "--idempotency-key", idem_key, "--timeout", "120s"]


def wake_env(cred, base_env):
    key, base = (cred or {}).get("api_key", ""), (cred or {}).get("base_url", "")
    if not key:
        raise Stop("credential file has no api_key")
    if not base.startswith("https://"):
        raise Stop("credential base_url is not https; the key goes only to the product host")
    return {**base_env, "CHOIR_API_KEY": key, "CHOIR_HOST": base}


def redact(text, key=""):
    text = re.sub(r"choir_sk_[A-Za-z0-9_-]+", "<redacted>", text or "")
    return text.replace(key, "<redacted>") if key else text


def wake_under_1s(w):
    if w.get("error") or w.get("wake_ms") is None:
        return False
    if w.get("restore_kind") not in ("resume", "unknown", None):
        return False  # a cold boot is not a wake (goodharting: counting cold boots as wakes)
    return w["wake_ms"] < WAKE_BUDGET_MS


def preflight(snap, health, extra_protect, n, floor_gib=4.67):
    pids = protected_ids(extra_protect)
    blockers, soft, warnings = [], [], []
    mem_kib = (snap.get("meminfo_kib") or {}).get("MemAvailable")
    if "error" in snap or mem_kib is None:
        blockers.append(f"host collect failed: {snap.get('error', 'no MemAvailable')}")
    if psi_stop(snap):
        blockers.append(psi_stop(snap))
    adm = (health or {}).get("admission")
    if "error" in (health or {}):
        blockers.append(f"vmctl health unreadable: {health.get('error')}")
    elif adm is None:
        warnings.append("admission settings are not exposed by vmctl health")
    else:
        if adm.get("enabled") is False:
            soft.append("admission is not enabled (pressure reclaim is not active): the run would prove nothing")
        if adm.get("make_room") is None:
            warnings.append("admission.make_room is not exposed")
        elif adm["make_room"] != "active":
            soft.append(f"admission make_room is {adm['make_room']!r}, not 'active' (D1 is off)")
    blockers += soft
    avail_gib = (mem_kib or 0) * 1024 / GIB
    need = ((adm or {}).get("default_need_bytes") or 2 << 30) / GIB
    headroom = ((adm or {}).get("headroom_bytes") or 1 << 30) / GIB
    if mem_kib is not None and avail_gib - floor_gib - headroom >= n * need:
        warnings.append(f"the host looks able to hold all {n} resident ({avail_gib:.1f} GiB available): "
                        "no make-room may be needed and the run would be vacuous")
    listing = snap.get("vmctl_list")
    return {
        "mem_available_gib": round(avail_gib, 2),
        "psi_some_avg10": ((snap.get("psi_memory") or {}).get("some") or {}).get("avg10"),
        "active_vms": (health or {}).get("active_vms"),
        "by_state": (health or {}).get("by_state"),
        "admission": {k: adm.get(k) for k in ("mode", "enabled", "make_room", "need_mode", "default_need_bytes",
                                              "min_need_bytes", "headroom_bytes", "settle_seconds", "reserved_bytes")
                      if k in adm} if adm else None,
        "protected_ids": pids,
        "protected_not_touched": [{"computer_id": o.get("computer_id"), "class": o.get("warmness_class"),
                                   "state": o.get("state"), "why": refusal(o, pids)}
                                  for o in ownerships(listing) if refusal(o, pids)],
        "blockers": blockers, "soft_blockers": soft, "warnings": warnings,
    }


def build_plan(a):
    labels = [f"d7-accept-{a.run_id}-{i}" for i in range(1, a.n + 1)]
    steps = [f"preflight: host MemAvailable, vmctl health, admission settings, protected set",
             f"mint {a.n} test computers in a row with scripts/qa_test_computer.mjs, waiting for each to be active (no stop by hand)",
             "per start: admission.recent decisions, victims and their classes, MemAvailable before/after, wall time, snapshot bytes of each victim",
             f"idle {a.idle_s}s sampling MemAvailable and PSI"]
    steps.append("wake each make-room victim this run minted through the product API with its own key, record wall time and boot timeline"
                 if a.wake else "wake phase skipped (--no-wake)")
    steps += [f"write {os.path.join(a.out, f'accept-five-{a.run_id}.json')} and the summary",
              "leave the minted computers to idle hibernation; record their ids"]
    return {"will_change_anything": bool(a.run), "run_id": a.run_id, "n": a.n, "labels": labels, "wake": a.wake,
            "idle_s": a.idle_s, "stops": [f"memory PSI some avg10 > {PSI_STOP:g}",
                                          "a protected or non-test computer that was active leaves active",
                                          "a minted computer or a make-room victim that is not safe test class"],
            "never": ["stop a computer", "touch the paper, the owner computers or the delegate", "du or find the artifacts root"],
            "steps": steps}


def evaluate(r):
    minted = r.get("minted") or []
    started = (r.get("stop") is None and len(minted) == r.get("n") and all(m.get("active") for m in minted))
    refused = sum(len(m.get("refusals") or []) for m in minted)
    victims = []
    for m in minted:
        for v in m.get("made_room") or []:
            if v.get("vm_id") not in victims:
                victims.append(v.get("vm_id"))
    wakes = r.get("wakes") or []
    series = [s["mem_available_kib"] for s in r.get("mem_series") or [] if s.get("mem_available_kib") is not None]
    return {
        "all_started": "pass" if started else "fail",
        "none_refused": "pass" if started and refused == 0 else "fail",
        "refused_starts": refused,
        "make_room_hibernations": len(victims),
        "vacuous": len(victims) == 0,
        "every_wake_under_1s": "not_run" if not wakes else ("pass" if all(wake_under_1s(w) for w in wakes) else "fail"),
        "wake_ms": [w.get("wake_ms") for w in wakes],
        "min_mem_available_gib": round(min(series) * 1024 / GIB, 2) if series else None,
    }


def render_summary(r):
    c = evaluate(r)
    L = [f"D7 acceptance run {r.get('run_id')}: n={r.get('n')} host={r.get('host', '?')}",
         f"- all started and active: {c['all_started']}" + (f" (stop: {r['stop']})" if r.get("stop") else ""),
         f"- none refused: {c['none_refused']} ({c['refused_starts']} refused starts)",
         f"- make-room hibernations: {c['make_room_hibernations']}" + (" (VACUOUS: nothing was hibernated to make room)" if c["vacuous"] else ""),
         f"- every wake under 1 s: {c['every_wake_under_1s']} (expected to fail until D2 lands)"]
    for w in r.get("wakes") or []:
        L.append(f"    wake {w.get('computer_id')}: {w.get('wake_ms')} ms (cli {w.get('cli_ms')} ms), {w.get('restore_kind')}"
                 + (f", error {w.get('error')}" if w.get("error") else ""))
    for m in r.get("minted") or []:
        L.append(f"  start {m.get('index')} {m.get('computer_id')}: {m.get('mint_wall_s', '?')} s to active; decisions "
                 + ",".join(d.get("decision", "?") for d in m.get("decisions") or []) + "; made room: "
                 + (", ".join(f"{v.get('vm_id')}({v.get('warmness_class') or '?'}, snapshot {v.get('snapshot', {}).get('du_bytes', '?')} B)"
                              for v in m.get("made_room") or []) or "none"))
    series = " ".join(f"{s.get('phase')}{s.get('i', '')}={round(s['mem_available_kib'] * 1024 / GIB, 2)}"
                      for s in r.get("mem_series") or [] if s.get("mem_available_kib") is not None)
    L.append(f"- MemAvailable (GiB) series: {series or 'none'}")
    L.append("- minted computers left to idle hibernation: " + ", ".join(m.get("computer_id", "?") for m in r.get("minted") or []))
    return "\n".join(L)


def parse_args(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--ssh", default="node-b")
    ap.add_argument("--local", action="store_true")
    ap.add_argument("--run", action="store_true", help="actually mint test computers (default: plan and preflight only)")
    ap.add_argument("--n", type=int, default=5)
    ap.add_argument("--idle-s", type=int, default=60)
    ap.add_argument("--wake", action=argparse.BooleanOptionalAction, default=True)
    ap.add_argument("--poll-s", type=int, default=10, help="seconds between polls while a mint runs")
    ap.add_argument("--mint-timeout-s", type=int, default=1500)
    ap.add_argument("--floor-gib", type=float, default=4.67, help="vmctl's memory floor, for the can-it-hold-all warning only")
    ap.add_argument("--protect-id", action="append", default=[])
    ap.add_argument("--run-id", default=time.strftime("%Y%m%dt%H%M%Sz", time.gmtime()))
    ap.add_argument("--out", default="receipts")
    ap.add_argument("--repo-root", default=REPO, help="checkout with scripts/qa_test_computer.mjs and frontend/node_modules")
    ap.add_argument("--choir-bin", default="/tmp/choir")
    ap.add_argument("--allow-weak-preflight", action="store_true", help="run despite preflight blockers (a vacuous run)")
    ap.add_argument("--canned", help="offline: read the host collect and vmctl health from this JSON; plan and preflight only")
    a = ap.parse_args(argv)
    a.run_id = a.run_id.lower()
    return a


# ---- host reads -----------------------------------------------------------

class CannedHost:
    def __init__(self, path):
        with open(path) as f:
            self.d = json.load(f)

    def collect(self, *flags, timeout=300):
        return self.d["collect"]

    def vmctl(self, method, path, body=None):
        return self.d["health"] if path.startswith("/health") else {}

    def sh(self, cmd, stdin=None, timeout=300):
        return ""


def light(host):
    return host.collect("--light", "--cpu-window", "1")


def mem_kib(snap):
    return (snap.get("meminfo_kib") or {}).get("MemAvailable")


def hibernate_facts(host, victim):
    """Snapshot bytes of one victim: vmctl's hibernate timeline plus du on that VM's own snapshot dir."""
    out = {"dir": None}
    cid = victim.get("computer_id", "")
    if SAFE_ID.match(cid or ""):
        tl = host.vmctl("GET", f"/internal/vmctl/boot-timeline?computer_id={cid}&kind=hibernate")
        out["timeline"] = timeline_facts(tl) or {"unavailable": (tl or {}).get("error", "empty")}
    if safe_vm_id(victim.get("vm_id", "")):
        d = snapshot_dir(victim["vm_id"])
        out["dir"] = d
        q = shlex.quote(d)
        raw = host.sh(f"test -d {q} && {{ du -s --block-size=1 {q}; du -s --apparent-size --block-size=1 {q}; }}")
        lines = [x for x in raw.splitlines() if x.strip()]
        if len(lines) >= 2:
            out["du_bytes"], out["du_apparent_bytes"] = parse_du(lines[0]), parse_du(lines[1])
        else:
            out["du_note"] = "snapshot dir absent (kill hibernate, or a vault root)"
    return out


# ---- the run --------------------------------------------------------------

class Run:
    def __init__(self, a, host):
        self.a, self.host = a, host
        self.pids = protected_ids(a.protect_id)
        self.t0 = time.time()
        self.seen, self.decisions = set(), []
        self.protected_before = set()
        self.r = {"kind": "density_accept_five", "run_id": a.run_id, "n": a.n, "host": "local" if a.local else a.ssh,
                  "started_at": fmt_ts(self.t0), "protected_ids": self.pids, "minted": [], "wakes": [],
                  "mem_series": [], "stop": None, "wake_skipped": []}

    def poll(self, phase="poll"):
        snap = light(self.host)
        why = psi_stop(snap)
        if why:
            raise Stop(why)
        lost = left_active(self.protected_before, snap.get("vmctl_list"))
        if lost:
            raise Stop(f"protected or non-test computer(s) left the active state: {sorted(lost)}")
        self.decisions += fresh_decisions(self.host.vmctl("GET", "/health"), self.t0, self.seen)
        return snap

    def sample(self, phase, i, snap):
        self.r["mem_series"].append({"phase": phase, "i": i, "at": snap.get("at"), "mem_available_kib": mem_kib(snap),
                                     "psi_some_avg10": ((snap.get("psi_memory") or {}).get("some") or {}).get("avg10")})

    def mint(self, i):
        a = self.a
        label = f"d7-accept-{a.run_id}-{i}"
        rec = {"index": i, "label": label, "active": False, "refusals": [], "made_room": [], "decisions": []}
        self.r["minted"].append(rec)
        before = self.poll()
        self.sample("before", i, before)
        rec["mem_before_kib"] = mem_kib(before)
        t_start = time.time()
        with tempfile.TemporaryFile("w+") as so, tempfile.TemporaryFile("w+") as se:
            proc = subprocess.Popen(["node", os.path.join(a.repo_root, "scripts/qa_test_computer.mjs"), "--label", label],
                                    stdout=so, stderr=se, text=True, cwd=a.repo_root)
            try:
                while proc.poll() is None:
                    if time.time() - t_start > a.mint_timeout_s:
                        proc.kill()
                        raise Stop(f"mint {label} did not finish in {a.mint_timeout_s}s")
                    time.sleep(a.poll_s)
                    self.poll()
            except BaseException:
                if proc.poll() is None:
                    proc.kill()
                raise
            so.seek(0), se.seek(0)
            out, err = so.read(), se.read()
        rec["mint_wall_s"] = round(time.time() - t_start, 1)
        try:
            info = json.loads(out.strip().splitlines()[-1])
            cid = info["computer_id"]
        except (ValueError, IndexError, KeyError):
            raise Stop(f"mint {label} failed: {redact((err or out)[-200:])}")
        rec.update({"computer_id": cid, "user_id": info.get("user_id"), "credential_file": info.get("credential_file"),
                    "guest_build_commit": info.get("guest_build_commit"), "mint_timings_ms": info.get("timings")})
        own = None
        for _ in range(40):  # up to ~2 min for vmctl to list it active
            snap = self.poll()
            own = next((o for o in ownerships(snap.get("vmctl_list")) if o.get("computer_id") == cid), None)
            if own and own.get("state") == "active":
                break
            time.sleep(3)
        if not own:
            raise Stop(f"minted {cid} is not in vmctl's list")
        require_minted_safe(own, self.pids)
        rec.update({"vm_id": own.get("vm_id"), "warmness_class": own.get("warmness_class"), "state_after": own.get("state")})
        if own.get("state") != "active":
            raise Stop(f"minted {cid} did not report active (state {own.get('state')!r})")
        rec["active"] = True
        rec["time_to_active_s"] = round(time.time() - t_start, 1)
        after = self.poll()
        self.sample("after", i, after)
        rec["mem_after_kib"] = mem_kib(after)
        mine = [d for d in self.decisions if d.get("vm_id") == rec["vm_id"]]
        rec["decisions"] = mine
        rec["refusals"] = refusals(self.decisions, rec["vm_id"])
        minted_cids = [m.get("computer_id") for m in self.r["minted"]]
        for vm in made_room_ids(start_decisions(self.decisions, rec["vm_id"])):
            v = victim_view(vm, after.get("vmctl_list"), self.pids, minted_cids)
            v["snapshot"] = hibernate_facts(self.host, v)
            rec["made_room"].append(v)
            if not v["safe"]:
                raise Stop(f"make-room hibernated {vm} ({v.get('computer_id') or 'unknown'}), which is not safe test class: {v['refusal']}")
        log("started", n=i, computer_id=cid, vm_id=rec["vm_id"], decisions=[d.get("decision") for d in mine],
            made_room=[v["vm_id"] for v in rec["made_room"]], s=rec["time_to_active_s"])

    def idle(self):
        end = time.time() + self.a.idle_s
        k = 0
        while time.time() < end:
            time.sleep(min(self.a.poll_s, max(1, end - time.time())))
            k += 1
            self.sample("idle", k, self.poll())

    def wake_all(self):
        a = self.a
        listing = self.poll().get("vmctl_list")
        minted_cids = [m.get("computer_id") for m in self.r["minted"]]
        victims = []
        for m in self.r["minted"]:
            for v in m.get("made_room") or []:
                victims.append(victim_view(v["vm_id"], listing, self.pids, minted_cids))
        cands, self.r["wake_skipped"] = wake_candidates(victims, listing, self.pids)
        creds = {m["computer_id"]: m.get("credential_file") for m in self.r["minted"] if m.get("computer_id")}
        for k, v in enumerate(cands, 1):
            w = {"computer_id": v["computer_id"], "vm_id": v["vm_id"], "order": k}
            self.r["wakes"].append(w)
            key = ""
            try:
                with open(creds[v["computer_id"]]) as f:
                    cred = json.load(f)
                key = cred.get("api_key", "")
                env = wake_env(cred, os.environ)
                t0 = time.time()
                p = subprocess.run(wake_argv(a.choir_bin, v["computer_id"], f"d7-wake-{a.run_id}-{k}"), env=env,
                                   capture_output=True, text=True, timeout=180)
                w["cli_ms"] = round((time.time() - t0) * 1000)
                w["cli_exit"] = p.returncode
                w["response_head"] = redact((p.stdout or p.stderr)[:300], key)
                if p.returncode != 0:
                    w["error"] = f"choir computer start exited {p.returncode}"
                    continue
                active_ms, state, first = None, "", True
                for _ in range(200):
                    tc = time.time()
                    snap = light(self.host)
                    own = next((o for o in ownerships(snap.get("vmctl_list")) if o.get("computer_id") == v["computer_id"]), None)
                    state = (own or {}).get("state", "")
                    if first:
                        w["first_check_ms"] = round((time.time() - tc) * 1000)
                    if state == "active":
                        active_ms = round((time.time() - t0) * 1000)
                        break
                    first = False
                    time.sleep(0.3)
                w["state_after"] = state
                # Active at the first check after the call returned: the call blocked until the computer was
                # active (or it was active within one check), so the call's own wall time is the wake time.
                # Otherwise the time until a later check saw it active (an upper bound).
                w["wake_ms"] = w["cli_ms"] if (first and active_ms is not None) else active_ms
                tl = self.host.vmctl("GET", f"/internal/vmctl/boot-timeline?computer_id={v['computer_id']}&kind=boot")
                w["restore_kind"] = restore_kind(tl, t0)
                w["timeline"] = timeline_facts(tl)
                w["under_1s"] = wake_under_1s(w)
                self.decisions += fresh_decisions(self.host.vmctl("GET", "/health"), self.t0, self.seen)
                self.sample("wake", k, self.poll())
            except Stop:
                raise
            except Exception as e:  # a failed wake is data, not a reason to lose the receipt
                w["error"] = redact(f"{type(e).__name__}: {e}", key)
            finally:
                w["under_1s"] = wake_under_1s(w)
                log("wake", **{x: w.get(x) for x in ("computer_id", "wake_ms", "cli_ms", "restore_kind", "error")})

    def go(self, build_cli):
        pre = light(self.host)
        self.protected_before = protected_active(pre.get("vmctl_list"), self.pids)
        self.r["protected_active_before"] = sorted(self.protected_before)
        self.sample("start", 0, pre)
        try:
            for i in range(1, self.a.n + 1):
                self.mint(i)
            self.idle()
            if self.a.wake:
                build_cli()
                self.wake_all()
        except Stop as e:
            self.r["stop"] = str(e)
            log("stop", reason=str(e))
        except Exception as e:
            self.r["stop"] = f"error: {type(e).__name__}: {redact(str(e))}"
            log("error", reason=self.r["stop"])
        finally:
            try:
                final = light(self.host)
                self.r["final_states"] = {m["computer_id"]: next((o.get("state") for o in ownerships(final.get("vmctl_list"))
                                                                   if o.get("computer_id") == m["computer_id"]), None)
                                          for m in self.r["minted"] if m.get("computer_id")}
                self.sample("end", 0, final)
            except Exception as e:
                self.r["final_states_error"] = str(e)
            self.decisions += fresh_decisions(self.host.vmctl("GET", "/health"), self.t0, self.seen)
            self.r["all_decisions"] = self.decisions
            self.r["left_to_idle_hibernation"] = [m.get("computer_id") for m in self.r["minted"] if m.get("computer_id")]
            self.r["ended_at"] = fmt_ts(time.time())
            self.r["checks"] = evaluate(self.r)
        return self.r


def build_cli(a):
    if os.path.isfile(a.choir_bin) and os.access(a.choir_bin, os.X_OK) and os.path.getmtime(a.choir_bin) > time.time() - 6 * 3600:
        return
    log("build_cli", out=a.choir_bin)
    p = subprocess.run(["go", "build", "-o", a.choir_bin, "./cmd/choir"], cwd=a.repo_root, capture_output=True, text=True)
    if p.returncode != 0:
        raise Stop("go build ./cmd/choir failed: " + (p.stderr or p.stdout)[-300:])


def main(argv=None):
    a = parse_args(sys.argv[1:] if argv is None else argv)
    if a.canned and a.run:
        raise SystemExit("--canned is an offline dry run; it cannot be combined with --run")
    host = CannedHost(a.canned) if a.canned else Host(None if a.local else a.ssh)
    pf = preflight(light(host), host.vmctl("GET", "/health"), a.protect_id, a.n, a.floor_gib)
    plan = build_plan(a)
    if not a.run:
        print(json.dumps({"plan": plan, "preflight": pf}, indent=1))
        return 0
    hard = [b for b in pf["blockers"] if b not in pf["soft_blockers"]]
    if not os.path.isdir(os.path.join(a.repo_root, "frontend", "node_modules")):
        hard.append(f"{a.repo_root}/frontend/node_modules is missing; qa_test_computer.mjs needs it (use --repo-root of a checkout that has it)")
    soft = [] if a.allow_weak_preflight else pf["soft_blockers"]
    if hard or soft:
        print(json.dumps({"plan": plan, "preflight": pf, "refused_to_start": hard + soft}, indent=1))
        return 2
    os.makedirs(a.out, exist_ok=True)
    runner = Run(a, host)
    runner.r["preflight"] = pf
    r = runner.go(lambda: build_cli(a))
    path = os.path.join(a.out, f"accept-five-{a.run_id}.json")
    with open(path, "w") as f:
        json.dump(r, f, indent=1)
    text = render_summary(r)
    with open(os.path.join(a.out, f"accept-five-{a.run_id}.summary.txt"), "w") as f:
        f.write(text + "\n")
    print(text)
    print(f"receipt: {path}")
    c = r["checks"]
    return 0 if c["all_started"] == "pass" and c["none_refused"] == "pass" and c["every_wake_under_1s"] != "fail" else 1


if __name__ == "__main__":
    sys.exit(main())
