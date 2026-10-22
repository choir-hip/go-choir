#!/usr/bin/env python3
"""Behavioural test for scripts/fp8_memory_return_probe.py: the first
reading must not report an unmeasured host RSS as a measured one, and the
host-side firecracker pid match must say why it found no process.

Ways the probe could mislead FP0 (written before the code; review bunch 87):
  1. No firecracker RSS at a window's start or end prints "host RSS fell
     0.0 MiB" and reads as outcome B (freed but not returned): it must be
     undeterminable, naming the missing side.
  2. No guest MemFree at a window's start or end prints "MemFree rose 0
     MiB" (untested): undeterminable too.
  3. An empty idle window reads as untested with a 0 MiB rise.
  4. The pid matcher finds no firecracker for the VM and records nothing,
     so an empty fc_rss_mib column has no cause: the miss must name the VM
     and the pattern matched.
  5. A measured window still reads returned (C) or freed but not returned
     (B) as before.
Run: python3 -I scripts/test_fp8_memory_return_probe.py
"""
import importlib.util
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("probe", os.path.join(HERE, "fp8_memory_return_probe.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


def r(phase, rss, free):
    return {"phase": phase, "fc_rss_mib": rss, "guest_memfree_mib": free,
            "damon_present": True, "damon_enabled": True, "kdamond_pid": 77,
            "page_reporting_registered": True, "balloon_reporting": True}


class Reading(unittest.TestCase):
    def windows(self, rows):
        return probe.first_reading(rows)["reporting_windows"]

    def test_missing_rss_on_both_sides_is_undeterminable(self):
        w = self.windows([r("burst", None, 100), r("idle", None, 900), r("idle", None, 1500)])
        for name in ("burst_end", "idle"):
            self.assertTrue(w[name].startswith("undeterminable"), w)
            self.assertNotIn("fell 0.0", w[name])

    def test_missing_rss_at_the_end_is_undeterminable(self):
        w = self.windows([r("burst", 3000, 100), r("idle", 2900, 900), r("idle", None, 1500)])
        self.assertTrue(w["idle"].startswith("undeterminable"), w)
        self.assertIn("end", w["idle"])
        self.assertIn("fc_rss_mib", w["idle"])

    def test_missing_memfree_is_undeterminable(self):
        w = self.windows([r("burst", 3000, 100), r("idle", 2900, None), r("idle", 2800, 1500)])
        self.assertTrue(w["idle"].startswith("undeterminable"), w)
        self.assertIn("guest_memfree_mib", w["idle"])

    def test_empty_idle_window_is_undeterminable(self):
        w = self.windows([r("burst", 3000, 100)])
        self.assertTrue(w["idle"].startswith("undeterminable"), w)

    def test_measured_windows_still_read(self):
        w = self.windows([r("burst", 3000, 100), r("idle", 2400, 800), r("idle", 2390, 1500)])
        self.assertTrue(w["burst_end"].startswith("returned (outcome C)"), w)
        self.assertTrue(w["idle"].startswith("freed but not returned (outcome B)"), w)


class PidMatch(unittest.TestCase):
    def proc(self, procs):
        d = tempfile.mkdtemp()
        for pid, (comm, args, status) in procs.items():
            os.makedirs(os.path.join(d, pid))
            for name, body in (("comm", comm + "\n"), ("cmdline", "\0".join(args) + "\0"), ("status", status)):
                with open(os.path.join(d, pid, name), "w") as f:
                    f.write(body)
        return d

    def matcher(self):
        ns = {}
        exec(probe.FC_RSS, ns)
        return ns["firecracker_rss"]

    def test_miss_names_the_vm_and_the_pattern(self):
        d = self.proc({"10": ("firecracker", ["firecracker", "--id", "vm-other"], "VmRSS:\t100 kB\n"),
                       "11": ("bash", ["bash", "vm-a"], "VmRSS:\t5 kB\n")})
        got, err = self.matcher()("vm-a", d)
        self.assertEqual(got, {})
        self.assertIsNotNone(err)
        self.assertIn("vm-a", err)
        self.assertIn("/vm-a", err)
        self.assertIn("1 firecracker", err)

    def test_hit_reads_rss(self):
        d = self.proc({"12": ("firecracker", ["firecracker", "--api-sock", "/run/vms/vm-a"],
                              "Name:\tfirecracker\nVmRSS:\t2048 kB\nRssAnon:\t1024 kB\nRssFile:\t512 kB\n")})
        got, err = self.matcher()("vm-a", d)
        self.assertIsNone(err)
        self.assertEqual(got, {"VmRSS_kib": 2048, "RssAnon_kib": 1024, "RssFile_kib": 512})


if __name__ == "__main__":
    unittest.main()
