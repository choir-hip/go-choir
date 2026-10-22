#!/usr/bin/env python3
"""Tests for the host-free parts of scripts/node_b_host_reboot.py.

No host, no ssh: the marker phase runs against a fake `remote`. Run:
python3 scripts/test_node_b_host_reboot.py

Problem: docs/problems/paper-crash-boot-holds-work-six-minutes-2026-10-22.md
(findings R1 to R3, review round 2 F1 to F9, round 3 N1 to N6, DeepSeek D1 to D5, round 4 R1 to R3).
"""
import argparse
import importlib.util
import json
import os
import re
import signal
import subprocess
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
        # Every case that returns true, not only the first (F9).
        cases = re.findall(r"case ([^:]+):\s*return true", body)
        self.assertTrue(cases)
        names = [n.strip() for case in cases for n in case.split(",")]
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


class Clock:
    """time.time and time.sleep for phase_mark: sleeping advances the clock."""

    def __init__(self, t=1_800_000_000.0):
        self.t = t

    def time(self):
        return self.t

    def sleep(self, s):
        self.t += s


class FakeHost:
    """remote() for phase_mark: per-VM guest answers, given what was sent and
    which mark call this is (1-based). fail_on_call raises from that mark
    call after the guests answered (an ssh drop that lost the answers)."""

    def __init__(self, rows, answer, clock=None, fail_on_call=None, unmark_status=200, later_rows=None):
        self.rows, self.answer, self.clock = rows, answer, clock
        self.fail_on_call, self.unmark_status = fail_on_call, unmark_status
        # later_rows: what vmctl lists from the second computers call on
        # (a dict with "error" for an unreadable list).
        self.later_rows = later_rows
        self.sent, self.unmarked, self.calls, self.timeouts = [], [], [], []
        self.listings, self.list_timeouts = 0, []

    def remote(self, host, mode, *args, timeout=180):
        if mode == "computers":
            self.listings += 1
            self.list_timeouts.append(timeout)
            rows = self.rows if self.listings == 1 or self.later_rows is None else self.later_rows
            if isinstance(rows, dict):
                return rows
            return {"counts": {}, "computers": rows}
        req = json.loads(args[0])
        if mode == "unmark":
            self.unmarked += [it["vm_id"] for it in req["items"]]
            return [{"vm_id": it["vm_id"], "status": self.unmark_status, "body": ""} for it in req["items"]]
        self.timeouts.append(timeout)
        n = len(self.calls) + 1
        self.calls.append({"at": self.clock.time() if self.clock else None, "vms": [it["vm_id"] for it in req["items"]]})
        self.sent += req["items"]
        out = []
        for it in req["items"]:
            st, body = self.answer(it["vm_id"], bool(it.get("allow_busy")), n)
            out.append({"vm_id": it["vm_id"], "status": st, "body": body})
        if self.clock:
            self.clock.sleep(2)
        if self.fail_on_call == n:
            raise RuntimeError("remote mark failed (255): Connection reset by peer")
        if self.fail_on_call == -n:
            raise subprocess.TimeoutExpired("ssh", timeout)
        return out


def row(vm, cls, held=False, selfdev_busy=0):
    return {"vm_id": vm, "state": "active", "warmness_class": cls, "computer_url": "http://10.200.1.2:8085", "held": held,
            "selfdev_busy": selfdev_busy}


def run_mark(fake, clock=None, **kw):
    args = argparse.Namespace(host="node-b", no_markers=False, busy_wait=0, allow_crash_close=False, no_allow_busy=False,
                              allow_crash_unverified=False)
    for k, v in kw.items():
        setattr(args, k, v)
    receipt = {"pre": {"facts": {"current_kernel": "6.18.57"}}}
    clock = clock or Clock()
    with mock.patch.object(reboot, "remote", fake.remote), mock.patch.object(reboot.time, "sleep", clock.sleep), \
            mock.patch.object(reboot.time, "time", clock.time):
        ok = reboot.phase_mark(args, receipt)
    return ok, receipt


def guest(busy_runs=None, unsafe=()):
    """A guest with running runs (vm -> count) that honours allow_busy, and
    guests mid-apply that always refuse."""
    busy_runs = busy_runs or {}

    def answer(vm, allow_busy, n=1):
        if vm in unsafe:
            return 409, busy_body(running=2, selfdev=1, allow_busy=allow_busy)
        k = busy_runs.get(vm, 0)
        if k and not allow_busy:
            return 409, busy_body(running=k)
        return 200, json.dumps({"reason": "platform update", "interrupted_running_runs": k})
    return answer


WRITTEN = (200, json.dumps({"reason": "platform update", "interrupted_running_runs": 0}))
BUSY = (409, busy_body(running=3))
UNSAFE = (409, busy_body(running=1, selfdev=1))
TIMEOUT = (0, "timed out")
CANNOT_TELL = (503, json.dumps({"error": "planned restart: cannot read self-development operations: database is locked"}))


def scripted(per_vm):
    """vm -> list of answers by mark call; the last one repeats."""
    def answer(vm, allow_busy, n):
        seq = per_vm.get(vm, [WRITTEN])
        return seq[min(n, len(seq)) - 1]
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
        # The paper's readable 409 proves no host marker: it is not cleared (D1).
        self.assertEqual(fake.unmarked, [PRIMARY])
        self.assertEqual(receipt["marks"]["outcomes"][PAPER]["outcome"], "busy")

    def test_apply_in_flight_blocks_the_reboot_even_with_allow_crash_close(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], guest(unsafe={PAPER}))
        ok, receipt = run_mark(fake, allow_crash_close=True)
        self.assertFalse(ok)
        # A DELETE would clear the apply's own marker (D1).
        self.assertEqual(fake.unmarked, [PRIMARY])
        self.assertEqual(receipt["marks"]["outcomes"][PAPER]["outcome"], "unsafe")
        self.assertTrue(receipt["marks"]["cleared"])

    def test_allow_crash_close_still_covers_runs_on_other_tiers(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], guest({PRIMARY: 2}))
        ok, receipt = run_mark(fake, allow_crash_close=True)
        self.assertTrue(ok)
        self.assertEqual(receipt["marks"]["unmarked"], [PRIMARY])
        self.assertEqual(receipt["marks"]["outcomes"][PRIMARY]["outcome"], "busy")


class CannotTell(unittest.TestCase):
    """F1: an answer that does not say whether an apply or a capsule is live
    is never covered by --allow-crash-close."""

    def test_503_or_timeout_is_not_crash_closed(self):
        for ans in (CANNOT_TELL, TIMEOUT, (500, "boom"), (404, "")):
            fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], scripted({PRIMARY: [ans]}))
            ok, receipt = run_mark(fake, allow_crash_close=True)
            self.assertFalse(ok, ans)
            self.assertIn(PAPER, fake.unmarked, ans)

    def test_allow_crash_unverified_names_the_risk(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], scripted({PRIMARY: [TIMEOUT]}))
        ok, receipt = run_mark(fake, allow_crash_unverified=True)
        self.assertTrue(ok)
        self.assertEqual(receipt["marks"]["worst"][PRIMARY], "unavailable")

    def test_allow_crash_unverified_never_covers_unsafe(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({PAPER: [UNSAFE]}))
        ok, _ = run_mark(fake, allow_crash_close=True, allow_crash_unverified=True)
        self.assertFalse(ok)


class StickyOutcomes(unittest.TestCase):
    """F2: the abort reads each computer's worst outcome, not its last."""

    def test_unsafe_then_timeout_still_aborts(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({PAPER: [UNSAFE, TIMEOUT]}), clock=Clock())
        ok, receipt = run_mark(fake, clock=fake.clock, busy_wait=120, allow_crash_close=True, allow_crash_unverified=True)
        self.assertFalse(ok)
        self.assertEqual(receipt["marks"]["worst"][PAPER], "unsafe")

    def test_unsafe_aborts_without_waiting_out_busy_wait(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({PAPER: [UNSAFE]}), clock=Clock())
        ok, _ = run_mark(fake, clock=fake.clock, busy_wait=600)
        self.assertFalse(ok)
        self.assertEqual(len(fake.calls), 1)

    def test_timeout_then_busy_needs_the_unverified_flag(self):
        fake = FakeHost([row(PRIMARY, "primary")], scripted({PRIMARY: [TIMEOUT, BUSY]}), clock=Clock())
        ok, receipt = run_mark(fake, clock=fake.clock, busy_wait=20, allow_crash_close=True)
        self.assertFalse(ok)
        self.assertEqual(receipt["marks"]["outcomes"][PRIMARY]["outcome"], "busy")
        self.assertEqual(receipt["marks"]["worst"][PRIMARY], "unavailable")

    def test_busy_then_written_is_marked(self):
        fake = FakeHost([row(PRIMARY, "primary")], scripted({PRIMARY: [BUSY, WRITTEN]}), clock=Clock())
        ok, receipt = run_mark(fake, clock=fake.clock, busy_wait=60)
        self.assertTrue(ok)
        self.assertEqual(receipt["marks"]["marked"], [PRIMARY])
        self.assertEqual(fake.unmarked, [])


class FinalRefresh(unittest.TestCase):
    """F3: every marker is re-written immediately before the reboot, and the
    receipt's marked_at is that last write."""

    def test_markers_rewritten_after_the_busy_wait(self):
        clock = Clock()
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")],
                        scripted({PRIMARY: [BUSY] * 30 + [WRITTEN]}), clock=clock)
        ok, receipt = run_mark(fake, clock=clock, busy_wait=600)
        self.assertTrue(ok)
        self.assertEqual(sorted(receipt["marks"]["marked"]), sorted([PAPER, PRIMARY]))
        last = fake.calls[-1]
        self.assertEqual(sorted(last["vms"]), sorted([PAPER, PRIMARY]))
        first_paper = next(c for c in fake.calls if PAPER in c["vms"])
        self.assertGreater(last["at"] - first_paper["at"], 300)
        self.assertGreaterEqual(reboot.iso_ts(receipt["marks"]["marked_at"]), int(last["at"]))
        self.assertLess(clock.time() - reboot.iso_ts(receipt["marks"]["marked_at"]), 60)

    def test_apply_started_after_marking_aborts(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")],
                        scripted({PAPER: [WRITTEN, WRITTEN, UNSAFE], PRIMARY: [BUSY, WRITTEN]}), clock=Clock())
        ok, receipt = run_mark(fake, clock=fake.clock, busy_wait=60)
        self.assertFalse(ok)
        self.assertEqual(sorted(fake.unmarked), sorted([PAPER, PRIMARY]))

    def test_marked_computer_unanswered_in_final_pass_aborts(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({PAPER: [WRITTEN, TIMEOUT]}), clock=Clock())
        ok, _ = run_mark(fake, clock=fake.clock, allow_crash_close=True, allow_crash_unverified=True)
        self.assertFalse(ok)
        self.assertEqual(fake.unmarked, [PAPER])

    def test_mark_timeout_covers_every_guest(self):
        rows = [row("vm-%02d" % i, "primary") for i in range(12)]
        fake = FakeHost(rows, scripted({}))
        run_mark(fake)
        self.assertGreater(min(fake.timeouts), 12 * 25)


class ClearOnFailure(unittest.TestCase):
    """F4: any failure clears every marker the script asked for."""

    def test_ssh_drop_clears_everything_sent(self):
        for fail in (1, -1):
            fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], scripted({}), fail_on_call=fail)
            with self.assertRaises((RuntimeError, subprocess.TimeoutExpired)):
                run_mark(fake)
            self.assertEqual(sorted(fake.unmarked), sorted([PAPER, PRIMARY]), fail)

    def test_receipt_lists_what_was_sent_on_a_failure(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({}), fail_on_call=1)
        args = argparse.Namespace(host="node-b", no_markers=False, busy_wait=0, allow_crash_close=False,
                                  no_allow_busy=False, allow_crash_unverified=False)
        receipt = {"pre": {"facts": {"current_kernel": "6.18.57"}}}
        clock = Clock()
        with mock.patch.object(reboot, "remote", fake.remote), mock.patch.object(reboot.time, "sleep", clock.sleep), \
                mock.patch.object(reboot.time, "time", clock.time):
            with self.assertRaises(RuntimeError):
                reboot.phase_mark(args, receipt)
        self.assertEqual(receipt["marks"]["sent"], [PAPER])
        self.assertTrue(receipt["marks"]["cleared"])

    def test_lost_answer_is_cleared_on_abort(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], scripted({PRIMARY: [TIMEOUT]}))
        ok, _ = run_mark(fake)
        self.assertFalse(ok)
        self.assertIn(PRIMARY, fake.unmarked)

    def test_failed_unmark_is_reported(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], scripted({PRIMARY: [BUSY]}),
                        unmark_status=500)
        ok, receipt = run_mark(fake)
        self.assertFalse(ok)
        self.assertFalse(receipt["marks"]["cleared"])
        self.assertEqual(sorted(receipt["marks"]["unmark_failed"]), [PAPER])


class UnaskedComputers(unittest.TestCase):
    """F5: computers the script does not ask (held ones, or all under
    --no-markers) get the read-only self-development check."""

    def test_no_markers_aborts_on_an_apply(self):
        fake = FakeHost([row(PAPER, "critical_protected", selfdev_busy=1)], scripted({}))
        ok, receipt = run_mark(fake, no_markers=True, allow_crash_close=True, allow_crash_unverified=True)
        self.assertFalse(ok)
        self.assertEqual(fake.sent, [])
        self.assertEqual(receipt["marks"]["unasked"][PAPER]["outcome"], "unsafe")

    def test_no_markers_idle_proceeds_and_sends_nothing(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({}))
        ok, receipt = run_mark(fake, no_markers=True)
        self.assertTrue(ok)
        self.assertEqual(fake.sent, [])
        self.assertTrue(receipt["marks"]["skipped"])

    def test_held_computer_with_an_apply_aborts(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary", held=True, selfdev_busy=2)], scripted({}))
        ok, _ = run_mark(fake, allow_crash_close=True)
        self.assertFalse(ok)
        # Checked before any marker is written: nothing to clear.
        self.assertEqual(fake.sent, [])

    def test_unreadable_obligations_need_the_unverified_flag(self):
        for sd in (None, "x"):
            fake = FakeHost([row(PRIMARY, "primary", held=True, selfdev_busy=sd)], scripted({}))
            self.assertFalse(run_mark(fake)[0], sd)
            fake = FakeHost([row(PRIMARY, "primary", held=True, selfdev_busy=sd)], scripted({}))
            self.assertTrue(run_mark(fake, allow_crash_unverified=True)[0], sd)

    def test_held_idle_computer_proceeds(self):
        fake = FakeHost([row(PRIMARY, "primary", held=True)], scripted({}))
        ok, receipt = run_mark(fake)
        self.assertTrue(ok)
        self.assertEqual(receipt["marks"]["unasked"][PRIMARY]["outcome"], "unasked")


class Relisted(unittest.TestCase):
    """N1: the computers are listed again immediately before the final pass;
    held and --no-markers computers are checked again, and a computer that
    started running during the wait is asked."""

    def test_held_computer_that_starts_an_apply_during_the_wait_aborts(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary", held=True)],
                        scripted({}), clock=Clock(),
                        later_rows=[row(PAPER, "critical_protected"), row(PRIMARY, "primary", held=True, selfdev_busy=1)])
        ok, receipt = run_mark(fake, clock=fake.clock)
        self.assertFalse(ok)
        self.assertEqual(receipt["marks"]["unasked"][PRIMARY]["outcome"], "unsafe")
        self.assertIn(PAPER, fake.unmarked)

    def test_no_markers_rechecks_before_the_reboot(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({}),
                        later_rows=[row(PAPER, "critical_protected", selfdev_busy=1)])
        ok, _ = run_mark(fake, no_markers=True)
        self.assertFalse(ok)
        self.assertEqual(fake.sent, [])

    def test_newly_running_computer_is_asked_in_the_final_pass(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({}),
                        later_rows=[row(PAPER, "critical_protected"), row(PRIMARY, "primary")])
        ok, receipt = run_mark(fake)
        self.assertTrue(ok)
        self.assertIn(PRIMARY, fake.calls[-1]["vms"])
        self.assertEqual(sorted(receipt["marks"]["marked"]), sorted([PAPER, PRIMARY]))

    def test_newly_running_computer_mid_apply_aborts(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({PRIMARY: [UNSAFE]}),
                        later_rows=[row(PAPER, "critical_protected"), row(PRIMARY, "primary")])
        ok, _ = run_mark(fake, allow_crash_close=True, allow_crash_unverified=True)
        self.assertFalse(ok)
        self.assertEqual(fake.unmarked, [PAPER])

    def test_newly_running_unreadable_under_no_markers_needs_the_flag(self):
        fake = FakeHost([], scripted({}), later_rows=[row(PRIMARY, "primary", selfdev_busy=None)])
        self.assertFalse(run_mark(fake, no_markers=True)[0])

    def test_unreadable_second_listing_aborts(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({}), later_rows={"error": "vmctl list: 0 timed out"})
        ok, _ = run_mark(fake)
        self.assertFalse(ok)
        self.assertEqual(fake.unmarked, [PAPER])

    def test_computers_timeout_scales_with_the_guests(self):
        rows = [row("vm-%02d" % i, "primary") for i in range(12)]
        fake = FakeHost(rows, scripted({}))
        run_mark(fake)
        self.assertGreater(fake.list_timeouts[-1], 12 * 23)


class CapsuleSummary(unittest.TestCase):
    """N5: one loud line before the reboot names the unasked computers."""

    def test_summary_names_unchecked_capsules(self):
        lines = []
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary", held=True)], scripted({}))
        with mock.patch.object(reboot, "log", lines.append):
            ok, _ = run_mark(fake)
        self.assertTrue(ok)
        summary = [l for l in lines if "live capsules not checked" in l and l.startswith("WARNING")]
        self.assertEqual(len(summary), 1, lines)
        self.assertIn(PRIMARY, summary[0])


class UnsupportedNotRetried(unittest.TestCase):
    """N6: a 404/405 answer cannot change, so it is not asked every round."""

    def test_unsupported_left_out_of_later_rounds(self):
        fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")],
                        scripted({PAPER: [(404, "")], PRIMARY: [BUSY, BUSY, WRITTEN]}), clock=Clock())
        ok, _ = run_mark(fake, clock=fake.clock, busy_wait=600, allow_crash_unverified=True)
        self.assertTrue(ok)
        self.assertIn(PAPER, fake.calls[0]["vms"])
        self.assertNotIn(PAPER, fake.calls[1]["vms"])


def reboot_args(**kw):
    a = argparse.Namespace(host="node-b", boot_timeout=1200, down_wait=300)
    for k, v in kw.items():
        setattr(a, k, v)
    return a


def marked_receipt():
    url = "http://10.200.1.2:8085"
    return {"pre": {"facts": {"boot_id": "old"}},
            "marks": {"marked": [PAPER], "sent": [PAPER, PRIMARY], "may_hold_marker": [PAPER],
                      "sent_items": [{"vm_id": PAPER, "computer_url": url}, {"vm_id": PRIMARY, "computer_url": url}]}}


class RebootNotDone(unittest.TestCase):
    """N2: a reboot that did not happen clears the markers and exits 3."""

    def run_reboot(self, rc, boot_ids, stderr="Failed to reboot: inhibited", hang=False):
        fake, clock = FakeHost([], scripted({})), Clock()
        ids = iter(boot_ids)
        receipt = marked_receipt()
        done = subprocess.CompletedProcess(["ssh"], rc, "", stderr)

        def run(*a, **k):
            if hang:
                raise subprocess.TimeoutExpired("ssh", 60, stderr=stderr)
            return done
        with mock.patch.object(reboot, "remote", fake.remote), mock.patch.object(reboot.time, "sleep", clock.sleep), \
                mock.patch.object(reboot.time, "time", clock.time), \
                mock.patch.object(reboot.subprocess, "run", run), \
                mock.patch.object(reboot, "ssh_ok", lambda host, timeout=15: next(ids, boot_ids[-1])):
            code = reboot.phase_reboot(reboot_args(), receipt)
        return code, fake, receipt

    def test_refused_reboot_clears_and_exits_3(self):
        code, fake, receipt = self.run_reboot(1, ["old"])
        self.assertEqual(code, 3)
        self.assertEqual(fake.unmarked, [PAPER])
        self.assertTrue(receipt["marks"]["cleared"])

    def test_host_still_up_after_down_wait_clears_and_exits_3(self):
        code, fake, _ = self.run_reboot(255, ["old"])
        self.assertEqual(code, 3)
        self.assertEqual(fake.unmarked, [PAPER])

    def test_real_reboot_keeps_the_markers(self):
        code, fake, _ = self.run_reboot(255, [None, None, "new"])
        self.assertEqual(code, 0)
        self.assertEqual(fake.unmarked, [])

    def test_host_never_back_exits_4_and_keeps_the_markers(self):
        for err in ("Connection to node-b closed by remote host.", "Read from remote host node-b: Connection timed out",
                    "Timeout, server node-b not responding.", "client_loop: send disconnect: Broken pipe"):
            code, fake, _ = self.run_reboot(255, [None], stderr=err)
            self.assertEqual(code, 4, err)
            self.assertEqual(fake.unmarked, [], err)

    def test_ssh_that_never_connected_clears_and_exits_3(self):
        """R2: a connect or auth failure means the reboot was never sent."""
        for err in ("ssh: connect to host node-b port 22: Connection refused",
                    "ssh: connect to host node-b port 22: Connection timed out",
                    "root@node-b: Permission denied (publickey).",
                    "kex_exchange_identification: read: Connection reset by peer",
                    "ssh: Could not resolve hostname node-b: nodename nor servname provided",
                    "Host key verification failed."):
            code, fake, receipt = self.run_reboot(255, [None], stderr=err)
            self.assertEqual(code, 3, err)
            self.assertEqual(fake.unmarked, [PAPER], err)
            self.assertTrue(receipt["reboot"]["not_sent"], err)

    def test_ssh_timeout_before_connecting_clears(self):
        code, fake, _ = self.run_reboot(None, [None], stderr="ssh: connect to host node-b port 22: Connection timed out",
                                        hang=True)
        self.assertEqual(code, 3)
        self.assertEqual(fake.unmarked, [PAPER])

    def test_ssh_timeout_after_connecting_waits(self):
        code, fake, _ = self.run_reboot(None, [None, "new"], stderr="", hang=True)
        self.assertEqual(code, 0)
        self.assertEqual(fake.unmarked, [])

    def test_failure_between_mark_and_reboot_clears(self):
        fake = FakeHost([], scripted({}))
        receipt = marked_receipt()

        def save():
            raise OSError("disk full")
        with mock.patch.object(reboot, "remote", fake.remote):
            with self.assertRaises(OSError):
                reboot.reboot_after_mark(reboot_args(), receipt, save)
        self.assertEqual(fake.unmarked, [PAPER])


class ClearOnlyPossibleMarkers(unittest.TestCase):
    """D1: the host clears only computers that may hold its marker (a 200
    in any round, a lost answer, a 5xx, a call that failed in flight); a
    readable refusal proves it wrote nothing, and a DELETE there could
    clear the guest's own apply marker."""

    def test_refusals_are_not_cleared(self):
        for ans in (UNSAFE, BUSY, (404, ""), (405, ""), (400, "{}"), (403, "{}")):
            fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], scripted({PRIMARY: [ans]}))
            ok, receipt = run_mark(fake)
            self.assertFalse(ok, ans)
            self.assertEqual(fake.unmarked, [PAPER], ans)
            self.assertEqual(receipt["marks"]["may_hold_marker"], [PAPER], ans)

    def test_pre_write_503_is_not_cleared(self):
        """R1: the guest's two 503s come before the write."""
        for ans in (CANNOT_TELL, (503, json.dumps({"error": "planned restart surface unavailable"}))):
            fake = FakeHost([row(PAPER, "critical_protected"), row(PRIMARY, "primary")], scripted({PRIMARY: [ans]}))
            ok, receipt = run_mark(fake)
            self.assertFalse(ok, ans)
            self.assertEqual(fake.unmarked, [PAPER], ans)

    def test_other_503_is_cleared(self):
        for ans in ((503, "<html>Service Unavailable</html>"), (503, json.dumps({"error": "upstream down"}))):
            fake = FakeHost([row(PRIMARY, "primary")], scripted({PRIMARY: [ans]}))
            ok, _ = run_mark(fake)
            self.assertFalse(ok, ans)
            self.assertEqual(fake.unmarked, [PRIMARY], ans)

    def test_lost_or_failed_writes_are_cleared(self):
        for ans in (TIMEOUT, (500, "planned restart marker: rename"), (502, "")):
            fake = FakeHost([row(PRIMARY, "primary")], scripted({PRIMARY: [ans]}))
            ok, _ = run_mark(fake)
            self.assertFalse(ok, ans)
            self.assertEqual(fake.unmarked, [PRIMARY], ans)

    def test_written_then_refused_is_still_cleared(self):
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({PAPER: [WRITTEN, UNSAFE]}))
        ok, _ = run_mark(fake)
        self.assertFalse(ok)
        self.assertEqual(fake.unmarked, [PAPER])

    def test_receipt_clear_skips_computers_without_a_host_marker(self):
        fake = FakeHost([], scripted({}))
        receipt = marked_receipt()
        with mock.patch.object(reboot, "remote", fake.remote):
            reboot.clear_receipt_markers(reboot_args(), receipt)
        self.assertEqual(fake.unmarked, [PAPER])


class FinalPassAbortReport(unittest.TestCase):
    def test_not_renewed_marker_says_so(self):
        """D3: the abort names a marker the final pass did not renew."""
        lines = []
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({PAPER: [WRITTEN, TIMEOUT]}))
        with mock.patch.object(reboot, "log", lines.append):
            ok, _ = run_mark(fake, allow_crash_unverified=True)
        self.assertFalse(ok)
        self.assertTrue(any("not renewed" in l and PAPER in l for l in lines), lines)
        self.assertFalse(any("no flag accepts it" in l for l in lines), lines)

    def test_marked_at_only_on_success(self):
        """D5: an aborted final pass leaves no marking time in the receipt."""
        fake = FakeHost([row(PAPER, "critical_protected")], scripted({PAPER: [WRITTEN, UNSAFE]}))
        ok, receipt = run_mark(fake)
        self.assertFalse(ok)
        self.assertIsNone(receipt["marks"]["marked_at"])



class Signals(unittest.TestCase):
    """R3: SIGHUP and SIGTERM raise into the clear paths."""

    def test_hup_and_term_raise_an_interrupt(self):
        got = {}
        with mock.patch.object(reboot.signal, "signal", lambda sig, fn: got.__setitem__(sig, fn)):
            reboot.install_signal_handlers()
        self.assertEqual(set(got), {signal.SIGHUP, signal.SIGTERM})
        for fn in got.values():
            with self.assertRaises(KeyboardInterrupt):
                fn(signal.SIGHUP, None)

    def test_signal_after_marking_clears(self):
        fake = FakeHost([], scripted({}))
        receipt = marked_receipt()

        def save():
            raise reboot.Interrupted("SIGHUP")
        with mock.patch.object(reboot, "remote", fake.remote):
            with self.assertRaises(KeyboardInterrupt):
                reboot.reboot_after_mark(reboot_args(), receipt, save)
        self.assertEqual(fake.unmarked, [PAPER])


if __name__ == "__main__":
    unittest.main()
