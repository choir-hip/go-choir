#!/usr/bin/env python3
"""Tests for the host-free parts of scripts/node_b_host_reboot.py.

No host, no ssh: the marker phase runs against a fake `remote`. Run:
python3 scripts/test_node_b_host_reboot.py

Problem: docs/problems/paper-crash-boot-holds-work-six-minutes-2026-10-22.md
(findings R1 to R3).
"""
import argparse
import importlib.util
import json
import os
import re
import unittest
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
spec = importlib.util.spec_from_file_location("node_b_host_reboot", os.path.join(HERE, "node_b_host_reboot.py"))
reboot = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reboot)

PAPER = "vm-1669020adcece51c2f5ff9b6df399ef5"
PLATFORM = "vm-platform"
PRIMARY = "vm-48bc09812f3fce06d715f6c7f8e2db9a"

# FORWARD as Node B lists it after the 2026-10-22 reboot (iptables -S FORWARD).
LIVE_FORWARD = [
    "-P FORWARD ACCEPT",
    "-A FORWARD -o vm-+ -m comment --comment go-choir-guest -j CHOIR-GUEST-RET",
    "-A FORWARD -i vm-+ -m comment --comment go-choir-guest -j CHOIR-GUEST-FWD",
]


def busy_body(running=8, selfdev=0, capsules=0, allow_busy=False):
    why = "guest is busy; restart deferred to its next idle moment"
    if selfdev or capsules:
        why = ("a self-development operation or a live capsule would not survive a restart; "
               "restart deferred until it settles")
    return json.dumps({"error": why, "outcome": "busy", "allow_busy": allow_busy, "running_runs": running,
                       "selfdev_active_operations": selfdev, "live_capsules": capsules})


class TierRule(unittest.TestCase):
    def test_busy_allowed_tiers(self):
        for cls in ("critical_protected", "public_platform"):
            self.assertTrue(reboot.marker_allows_busy(cls), cls)
        for cls in ("premium_always_on", "primary", "test", "", None):
            self.assertFalse(reboot.marker_allows_busy(cls), cls)

    def test_no_allow_busy_keeps_the_old_rule(self):
        for cls in ("critical_protected", "public_platform", "primary"):
            self.assertFalse(reboot.marker_allows_busy(cls, no_allow_busy=True), cls)

    def test_same_tiers_as_vmctl_deploy_refresh(self):
        # The script must not grow a second rule: read deployRefreshAllowsBusy.
        with open(os.path.join(REPO, "internal", "vmctl", "deploy_refresh.go")) as f:
            src = f.read()
        body = re.search(r"func deployRefreshAllowsBusy\(class WarmnessClass\) bool \{(.*?)\n\}", src, re.S).group(1)
        case = re.search(r"case ([^:]+):\s*return true", body).group(1)
        names = [n.strip() for n in case.split(",")]
        with open(os.path.join(REPO, "internal", "vmctl", "warmness_policy.go")) as f:
            consts = dict(re.findall(r"(WarmnessClass\w+)\s+WarmnessClass = \"([^\"]+)\"", f.read()))
        self.assertEqual({consts[n] for n in names}, set(reboot.BUSY_ALLOWED_CLASSES))


class MarkerAnswer(unittest.TestCase):
    def test_written(self):
        a = reboot.classify_marker_answer(200, json.dumps({"interrupted_running_runs": 6}), True)
        self.assertEqual(a["outcome"], "written")
        self.assertEqual(a["interrupted_running_runs"], 6)

    def test_busy_with_runs(self):
        a = reboot.classify_marker_answer(409, busy_body(running=8), False)
        self.assertEqual(a["outcome"], "busy")
        self.assertEqual(a["running_runs"], 8)

    def test_apply_or_capsule_is_unsafe_even_with_allow_busy(self):
        for body in (busy_body(running=0, selfdev=1, allow_busy=True), busy_body(running=3, capsules=1, allow_busy=True)):
            self.assertEqual(reboot.classify_marker_answer(409, body, True)["outcome"], "unsafe", body)

    def test_old_guest_ignores_allow_busy(self):
        a = reboot.classify_marker_answer(409, busy_body(running=4, allow_busy=False), True)
        self.assertEqual(a["outcome"], "busy")
        self.assertIn("ignored allow_busy", a["detail"])

    def test_unreadable_409_fails_closed(self):
        self.assertEqual(reboot.classify_marker_answer(409, "<html>", True)["outcome"], "unsafe")
        self.assertEqual(reboot.classify_marker_answer(409, busy_body()[:120], False)["outcome"], "unsafe")

    def test_other_answers(self):
        self.assertEqual(reboot.classify_marker_answer(404, "", False)["outcome"], "unsupported")
        self.assertEqual(reboot.classify_marker_answer(405, "", False)["outcome"], "unsupported")
        self.assertEqual(reboot.classify_marker_answer(0, "connection refused", False)["outcome"], "unavailable")
        self.assertEqual(reboot.classify_marker_answer(503, "", False)["outcome"], "unavailable")


class ForwardOrder(unittest.TestCase):
    def test_live_order_passes(self):
        ok, detail = reboot.forward_guest_jumps(LIVE_FORWARD)
        self.assertTrue(ok, detail)

    def test_later_rules_allowed(self):
        ok, _ = reboot.forward_guest_jumps(LIVE_FORWARD + ["-A FORWARD -i docker0 -j ACCEPT"])
        self.assertTrue(ok)

    def test_reversed_fails(self):
        ok, _ = reboot.forward_guest_jumps([LIVE_FORWARD[0], LIVE_FORWARD[2], LIVE_FORWARD[1]])
        self.assertFalse(ok)

    def test_missing_either_fails(self):
        self.assertFalse(reboot.forward_guest_jumps([LIVE_FORWARD[0], LIVE_FORWARD[1]])[0])
        self.assertFalse(reboot.forward_guest_jumps([LIVE_FORWARD[0], LIVE_FORWARD[2]])[0])

    def test_rule_ahead_of_the_jumps_fails(self):
        self.assertFalse(reboot.forward_guest_jumps([LIVE_FORWARD[0], "-A FORWARD -j ACCEPT"] + LIVE_FORWARD[1:])[0])

    def test_unreadable_fails(self):
        self.assertFalse(reboot.forward_guest_jumps(None)[0])
        self.assertFalse(reboot.forward_guest_jumps([])[0])
        self.assertFalse(reboot.forward_guest_jumps(["-P FORWARD ACCEPT"])[0])


class FakeHost:
    """remote() for phase_mark: per-VM guest answers, given what was sent."""

    def __init__(self, rows, answer):
        self.rows, self.answer = rows, answer
        self.sent, self.unmarked = [], []

    def remote(self, host, mode, *args, timeout=180):
        if mode == "computers":
            return {"counts": {}, "computers": self.rows}
        req = json.loads(args[0])
        if mode == "unmark":
            self.unmarked += [it["vm_id"] for it in req["items"]]
            return [{"vm_id": it["vm_id"], "status": 200, "body": ""} for it in req["items"]]
        self.sent += req["items"]
        out = []
        for it in req["items"]:
            st, body = self.answer(it["vm_id"], bool(it.get("allow_busy")))
            out.append({"vm_id": it["vm_id"], "status": st, "body": body})
        return out


def row(vm, cls):
    return {"vm_id": vm, "state": "active", "warmness_class": cls, "computer_url": "http://10.200.1.2:8085", "held": False}


def run_mark(fake, **kw):
    args = argparse.Namespace(host="node-b", no_markers=False, busy_wait=0, allow_crash_close=False, no_allow_busy=False)
    for k, v in kw.items():
        setattr(args, k, v)
    receipt = {"pre": {"facts": {"current_kernel": "6.18.57"}}}
    with mock.patch.object(reboot, "remote", fake.remote), mock.patch.object(reboot.time, "sleep", lambda s: None):
        ok = reboot.phase_mark(args, receipt)
    return ok, receipt


def guest(busy_runs=None, unsafe=()):
    """A guest with running runs (vm -> count) that honours allow_busy, and
    guests mid-apply that always refuse."""
    busy_runs = busy_runs or {}

    def answer(vm, allow_busy):
        if vm in unsafe:
            return 409, busy_body(running=2, selfdev=1, allow_busy=allow_busy)
        n = busy_runs.get(vm, 0)
        if n and not allow_busy:
            return 409, busy_body(running=n)
        return 200, json.dumps({"reason": "platform update", "interrupted_running_runs": n})
    return answer


class MarkPhase(unittest.TestCase):
    def test_busy_paper_is_marked_with_allow_busy(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PLATFORM, "public_platform"), row(PRIMARY, "primary")],
                        guest({PAPER: 8}))
        ok, receipt = run_mark(fake)
        self.assertTrue(ok)
        sent = {it["vm_id"]: bool(it.get("allow_busy")) for it in fake.sent}
        self.assertEqual(sent, {PAPER: True, PLATFORM: True, PRIMARY: False})
        self.assertIn(PAPER, receipt["marks"]["marked"])
        out = receipt["marks"]["outcomes"][PAPER]
        self.assertEqual((out["outcome"], out["allow_busy"], out["interrupted_running_runs"]), ("written", True, 8))

    def test_no_allow_busy_aborts_on_the_busy_paper(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], guest({PAPER: 8}))
        ok, receipt = run_mark(fake, no_allow_busy=True)
        self.assertFalse(ok)
        self.assertFalse(any(it.get("allow_busy") for it in fake.sent))
        self.assertEqual(fake.unmarked, [PRIMARY])
        self.assertEqual(receipt["marks"]["outcomes"][PAPER]["outcome"], "busy")

    def test_apply_in_flight_blocks_the_reboot_even_with_allow_crash_close(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], guest(unsafe={PAPER}))
        ok, receipt = run_mark(fake, allow_crash_close=True)
        self.assertFalse(ok)
        self.assertEqual(fake.unmarked, [PRIMARY])
        self.assertEqual(receipt["marks"]["outcomes"][PAPER]["outcome"], "unsafe")
        self.assertTrue(receipt["marks"]["cleared"])

    def test_allow_crash_close_still_covers_runs_on_other_tiers(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], guest({PRIMARY: 2}))
        ok, receipt = run_mark(fake, allow_crash_close=True)
        self.assertTrue(ok)
        self.assertEqual(receipt["marks"]["unmarked"], [PRIMARY])
        self.assertEqual(receipt["marks"]["outcomes"][PRIMARY]["outcome"], "busy")


if __name__ == "__main__":
    unittest.main()
