#!/usr/bin/env python3
"""Behavioural test for scripts/paper-liveness-probe.py.

It runs the real script with a fake `journalctl` first on PATH and temp tiers,
ownerships and uptime files. Run: python3 scripts/test_paper_liveness_probe.py
"""
import json
import os
import stat
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
PROBE = os.path.join(HERE, "paper-liveness-probe.py")
VM = "vm-1669020adcece51c2f5ff9b6df399ef5"
COMPUTER = "computer-1355b83bced1613eb048ee28d66688a8"

FAKE_JOURNALCTL = """#!/bin/sh
# Fake journalctl: prints $FAKE_JOURNAL_FILE lines containing the --grep pattern,
# or fails with $FAKE_JOURNAL_RC when set.
if [ -n "$FAKE_JOURNAL_RC" ]; then echo "boom" >&2; exit "$FAKE_JOURNAL_RC"; fi
pattern=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--grep" ]; then pattern="$2"; fi
  shift
done
grep -F -- "$pattern" "$FAKE_JOURNAL_FILE" || exit 1
"""


class PaperLivenessProbe(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        d = self.dir.name
        self.write("tiers.json", json.dumps({"platform_computers": [{"computer_id": COMPUTER, "note": "paper"}]}))
        self.write("ownerships.json", json.dumps({"ownerships": {
            "1": {"vm_id": "vm-other", "computer_id": "computer-other"},
            "2": {"vm_id": VM, "computer_id": COMPUTER}}}))
        self.write("uptime", "36000.00 1.0\n")
        self.write("journal", "")
        os.makedirs(os.path.join(d, "bin"))
        fake = os.path.join(d, "bin", "journalctl")
        with open(fake, "w") as f:
            f.write(FAKE_JOURNALCTL)
        os.chmod(fake, os.stat(fake).st_mode | stat.S_IEXEC)

    def tearDown(self):
        self.dir.cleanup()

    def write(self, name, text):
        with open(os.path.join(self.dir.name, name), "w") as f:
            f.write(text)

    def run_probe(self, **extra):
        d = self.dir.name
        env = {
            "PATH": os.path.join(d, "bin") + os.pathsep + os.environ["PATH"],
            "PAPER_TIERS_FILE": os.path.join(d, "tiers.json"),
            "OWNERSHIPS_FILE": os.path.join(d, "ownerships.json"),
            "PROC_UPTIME": os.path.join(d, "uptime"),
            "FAKE_JOURNAL_FILE": os.path.join(d, "journal"),
        }
        env.update(extra)
        proc = subprocess.run([sys.executable, "-I", PROBE], env=env, capture_output=True, text=True)
        return proc.returncode, proc.stdout + proc.stderr

    def test_recent_inference_is_ok(self):
        self.write("journal", f"2026-10-15T13:00:03 gateway: inference succeeded for autoputer {VM} (provider=chatgpt)\n")
        rc, out = self.run_probe()
        self.assertEqual(rc, 0, out)
        self.assertIn("ok ", out)
        self.assertIn(VM, out)
        self.assertNotIn("ALERT", out)

    def test_no_paper_inference_alerts_at_error_priority_and_names_the_vm(self):
        # Other computers' calls do not count as the paper's.
        self.write("journal", "2026-10-15T13:00:03 gateway: inference succeeded for autoputer vm-other (provider=x)\n")
        rc, out = self.run_probe()
        self.assertEqual(rc, 0, out)
        self.assertIn("<3>paper-liveness-probe: ALERT", out)
        self.assertIn(VM, out)

    def test_vm_is_looked_up_from_the_platform_computer_not_hardcoded(self):
        self.write("ownerships.json", json.dumps({"ownerships": [{"vm_id": "vm-reminted", "computer_id": COMPUTER}]}))
        self.write("journal", "2026-10-15T13:00:03 gateway: inference succeeded for autoputer vm-reminted (p)\n")
        rc, out = self.run_probe()
        self.assertEqual(rc, 0, out)
        self.assertIn("vm=vm-reminted", out)
        self.assertNotIn("ALERT", out)

    def test_fresh_boot_is_skipped_not_alerted(self):
        self.write("uptime", "600.00 1.0\n")
        rc, out = self.run_probe()
        self.assertEqual(rc, 0, out)
        self.assertIn("skipped", out)
        self.assertNotIn("ALERT", out)

    def test_unresolvable_vm_is_a_probe_error_that_fails_the_unit(self):
        self.write("ownerships.json", json.dumps({"ownerships": {}}))
        rc, out = self.run_probe()
        self.assertEqual(rc, 1, out)
        self.assertIn("probe-error", out)

    def test_unreadable_journal_is_a_probe_error_not_an_alert(self):
        rc, out = self.run_probe(FAKE_JOURNAL_RC="2")
        self.assertEqual(rc, 1, out)
        self.assertIn("probe-error", out)
        self.assertNotIn("ALERT", out)

    def test_override_vm_id(self):
        self.write("journal", "2026-10-15T13:00:03 gateway: inference succeeded for autoputer vm-x (p)\n")
        rc, out = self.run_probe(PAPER_VM_ID="vm-x")
        self.assertEqual(rc, 0, out)
        self.assertIn("vm=vm-x", out)


if __name__ == "__main__":
    unittest.main()
