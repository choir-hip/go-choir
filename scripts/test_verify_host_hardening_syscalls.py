#!/usr/bin/env python3
"""Test of the unit syscall filter check in scripts/verify-host-hardening.py.

    python3 -I scripts/test_verify_host_hardening_syscalls.py

Feeds check_units fake systemctl answers; needs no root. go-choir-vmctl
allows @privileged on purpose (the jailer, 10e9f18e1, nix/node-b.nix); every
other unit must still exclude it.

Failure modes pinned here:
  - vmctl's intended @privileged reads as a FAIL (the stale check);
  - the vmctl exemption hides why (no reason, no commit ref);
  - the exemption leaks to another unit, or a deny-list filter is misread.
"""

import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("vhh", os.path.join(HERE, "verify-host-hardening.py"))
vhh = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vhh)

# An allow list as systemctl show prints it: @privileged expanded in.
ALLOW_PRIVILEGED = "bpf chroot acct setuid capset read write openat"
# The common hardening: ~@privileged, a deny list.
DENY_PRIVILEGED = "~bpf chroot acct setuid capset"
ALLOW_CLEAN = "read write openat"


def run_check(filters):
    vhh.results.clear()
    show = {u: {"ActiveState": "active", "MainPID": "0", "SystemCallFilter": f} for u, f in filters.items()}
    vhh.systemctl_show = lambda unit, props: show.get(unit, {})
    vhh.check_units()
    return {r["check"]: r for r in vhh.results if "syscall filter" in r["check"]}


class SyscallFilterCheck(unittest.TestCase):
    def test_vmctl_privileged_is_allowed_with_reason(self):
        got = run_check({"go-choir-vmctl": ALLOW_PRIVILEGED})
        self.assertEqual(len(got), 1, got)
        (name, r), = got.items()
        self.assertTrue(r["ok"], r)
        self.assertIn("allows @privileged", name)
        self.assertIn("jailer", r["detail"])
        self.assertIn("10e9f18e1", r["detail"])

    def test_other_unit_privileged_fails(self):
        got = run_check({"go-choir-proxy": ALLOW_PRIVILEGED})
        r = got["go-choir-proxy syscall filter excludes @privileged"]
        self.assertFalse(r["ok"], r)

    def test_other_unit_deny_list_passes(self):
        got = run_check({"go-choir-gateway": DENY_PRIVILEGED, "go-choir-auth": ALLOW_CLEAN})
        self.assertTrue(got["go-choir-gateway syscall filter excludes @privileged"]["ok"])
        self.assertTrue(got["go-choir-auth syscall filter excludes @privileged"]["ok"])


if __name__ == "__main__":
    unittest.main()
