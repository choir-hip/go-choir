#!/usr/bin/env python3
"""Canned-JSON checks for the density scripts' guard and safety stops. Run:
python3 -I scripts/density/test_guard.py (no host, no network)."""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lib  # noqa: E402

LISTING = {"ownerships": [
    {"computer_id": "computer-aaaa", "vm_id": "vm-aaaa", "state": "active", "warmness_class": "test"},
    {"computer_id": "computer-paper", "vm_id": "vm-1669020a", "state": "active", "warmness_class": "critical_protected"},
    {"computer_id": "computer-owner", "vm_id": "vm-owner", "state": "active", "warmness_class": "primary"},
    {"computer_id": lib.DELEGATE_COMPUTER_ID, "vm_id": "vm-delegate", "state": "active", "warmness_class": "test"},
    {"computer_id": "computer-held", "vm_id": "vm-held", "state": "active", "warmness_class": "test", "held": True},
    {"computer_id": "computer-new", "vm_id": "vm-new", "state": "active"},
    {"computer_id": "computer-plat", "vm_id": "vm-plat", "state": "active", "warmness_class": "public_platform"},
]}


def own(cid):
    return next(o for o in LISTING["ownerships"] if o["computer_id"] == cid)


class Guard(unittest.TestCase):
    def test_test_computer_passes(self):
        self.assertEqual(lib.refusal(own("computer-aaaa")), "")

    def test_protected_are_refused(self):
        for cid in ("computer-paper", "computer-owner", "computer-plat", "computer-new"):
            self.assertTrue(lib.refusal(own(cid)), cid)

    def test_delegate_refused_even_if_classed_test(self):
        self.assertIn("delegate", lib.refusal(own(lib.DELEGATE_COMPUTER_ID)))

    def test_held_and_named_refused(self):
        self.assertIn("held", lib.refusal(own("computer-held")))
        self.assertIn("protect-id", lib.refusal(own("computer-aaaa"), protect_ids=["computer-aaaa"]))

    def test_require_test_exits(self):
        with self.assertRaises(SystemExit):
            lib.require_test(own("computer-paper"))
        with self.assertRaises(SystemExit):
            lib.require_test({})

    def test_non_test_active_set(self):
        self.assertEqual(lib.non_test_active(LISTING),
                         {"computer-paper", "computer-owner", "computer-held", "computer-new", "computer-plat", lib.DELEGATE_COMPUTER_ID})


class Stops(unittest.TestCase):
    def snap(self, psi=0.0, avail_gib=20):
        return {"psi_memory": {"some": {"avg10": psi}}, "meminfo_kib": {"MemAvailable": int(avail_gib * lib.GIB / 1024)}}

    def test_ok(self):
        self.assertEqual(lib.safety_stop(self.snap(), 4.67), "")

    def test_psi(self):
        self.assertIn("PSI", lib.safety_stop(self.snap(psi=1.5), 4.67))

    def test_floor(self):
        self.assertIn("MemAvailable", lib.safety_stop(self.snap(avail_gib=6.0), 4.67))
        self.assertEqual(lib.safety_stop(self.snap(avail_gib=6.8), 4.67), "")


if __name__ == "__main__":
    unittest.main()
