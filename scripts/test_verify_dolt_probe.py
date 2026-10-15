#!/usr/bin/env python3
"""Test of the Dolt liveness probe check in scripts/verify-host-hardening.py.

    python3 -I scripts/test_verify_dolt_probe.py

Feeds check_dolt_probe fake systemctl/journal answers; needs no root.
"""

import importlib.util
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("vhh", os.path.join(HERE, "verify-host-hardening.py"))
vhh = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vhh)

TIMER = "go-choir-dolt-liveness-probe.timer"
SERVICE = "go-choir-dolt-liveness-probe.service"


def run_check(show, journal):
    vhh.results.clear()
    vhh.systemctl_show = lambda unit, props: show.get(unit, {})
    vhh.journal_lines = lambda unit, pattern, since="-24h": journal.get((unit, pattern), [])
    vhh.check_dolt_probe()
    return {r["check"]: r["ok"] for r in vhh.results}


def healthy_show():
    return {
        TIMER: {"ActiveState": "active"},
        SERVICE: {"Result": "success"},
        "go-choir-platform-dolt.service": {"TimeoutStopUSec": "20s"},
        "go-choir-corpus-dolt.service": {"TimeoutStopUSec": "20s"},
    }


class DoltProbeCheck(unittest.TestCase):
    def test_healthy(self):
        got = run_check(healthy_show(), {})
        self.assertTrue(got and all(got.values()), got)

    def test_timer_inactive_fails(self):
        show = healthy_show()
        show[TIMER] = {"ActiveState": "inactive"}
        got = run_check(show, {})
        self.assertFalse(got["dolt liveness probe timer is active"], got)

    def test_recent_probe_error_fails(self):
        got = run_check(healthy_show(), {(SERVICE, "probe-error"): ["probe-error unit=x class=cred"]})
        self.assertFalse(got["dolt liveness probe reports no probe-error in the last 15 minutes"], got)

    def test_failed_last_run_fails(self):
        show = healthy_show()
        show[SERVICE] = {"Result": "exit-code"}
        got = run_check(show, {})
        self.assertFalse(got["dolt liveness probe last run succeeded"], got)

    def test_default_stop_timeout_fails(self):
        show = healthy_show()
        show["go-choir-platform-dolt.service"] = {"TimeoutStopUSec": "1min 30s"}
        got = run_check(show, {})
        self.assertFalse(got["go-choir-platform-dolt stop timeout is 20s"], got)


if __name__ == "__main__":
    sys.exit(unittest.main())
