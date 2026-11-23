import unittest

import run_android


class FakeAdb:
    """Records every call; can fail on a chosen step."""

    def __init__(self, fail_on=None):
        self.calls = []
        self.fail_on = fail_on

    def shell(self, cmd, timeout=None):
        self.calls.append(("shell", cmd))
        if self.fail_on and self.fail_on in cmd:
            raise RuntimeError("boom: " + cmd)
        return ""

    def push(self, src, dst):
        self.calls.append(("push", src, dst))
        if self.fail_on == "push":
            raise RuntimeError("push failed")

    def pull(self, src, dst):
        self.calls.append(("pull", src, dst))


class CleanupTest(unittest.TestCase):
    def test_remove_refuses_paths_outside_mbench(self):
        for bad in ["/data/local/tmp/exact", "/data/local/tmp", "/sdcard/x", "/data/local/tmp/mbench-", "/data/local/tmp/mbench-a/../exact"]:
            with self.assertRaises(ValueError, msg=bad):
                run_android.remove_dir(FakeAdb(), bad)

    def test_cleanup_runs_when_the_run_fails(self):
        adb = FakeAdb(fail_on="RUNSTEP")
        d = "/data/local/tmp/mbench-t1"
        with self.assertRaises(RuntimeError):
            run_android.with_workdir(adb, d, lambda: adb.shell("RUNSTEP"))
        self.assertIn(("shell", "rm -rf %s" % d), adb.calls)
        self.assertEqual(adb.calls[0], ("shell", "mkdir -p %s" % d))

    def test_cleanup_runs_when_the_run_succeeds_and_verifies(self):
        adb = FakeAdb()
        d = "/data/local/tmp/mbench-t2"
        run_android.with_workdir(adb, d, lambda: None)
        self.assertEqual(adb.calls[-1][0], "shell")
        self.assertIn("rm -rf %s" % d, [c[1] for c in adb.calls if c[0] == "shell"])

    def test_cleanup_runs_on_keyboard_interrupt(self):
        adb = FakeAdb()
        d = "/data/local/tmp/mbench-t3"

        def boom():
            raise KeyboardInterrupt

        with self.assertRaises(KeyboardInterrupt):
            run_android.with_workdir(adb, d, boom)
        self.assertIn(("shell", "rm -rf %s" % d), adb.calls)

    def test_no_forbidden_adb_verbs(self):
        adb = FakeAdb()
        run_android.with_workdir(adb, "/data/local/tmp/mbench-t4", lambda: None)
        text = " ".join(str(c) for c in adb.calls)
        for bad in ["install", "setprop", "settings", "reboot", "pkill", "killall", " su "]:
            self.assertNotIn(bad, text)


class PreflightTest(unittest.TestCase):
    def test_refuses_hot_or_low_battery(self):
        ok = dict(thermal_status=0, battery_temp_c=25.0, battery_level=90)
        run_android.check_preflight(**ok)
        for k, v in [("thermal_status", 2), ("battery_temp_c", 41.0), ("battery_level", 10)]:
            bad = dict(ok)
            bad[k] = v
            with self.assertRaises(run_android.PhoneNotReady):
                run_android.check_preflight(**bad)


if __name__ == "__main__":
    unittest.main()
