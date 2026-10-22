#!/usr/bin/env python3
"""Tests for scripts/paper_boot_window_probe.py (the sample classifier).

Ways the probe could mislead FP0, each a test below:
  1. A healthy sample is reported as a stall.
  2. A /health hang while the online Dolt GC runs is not attributed to it.
  3. A /health hang behind another engine holder is attributed to the GC.
  4. A stalled process (observability does not answer either) is read as a
     store hold: H2/H3 hidden behind H1.
  5. A goroutine dump frame count misses a frame or counts a near-miss.
  6. The summary misses where the window starts or ends.

Run: python3 scripts/test_paper_boot_window_probe.py
"""
import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("probe", os.path.join(HERE, "paper_boot_window_probe.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)

GC_DUMP = """goroutine 81 [syscall]:
github.com/yusefmosiah/go-choir/internal/store.(*Store).runOnlineDoltGC(0xc000123)
goroutine 90 [select]:
database/sql.(*DB).conn(0xc0001, {0x1, 0x2}, 0x1)
github.com/yusefmosiah/go-choir/internal/agentcore.(*APIHandler).HandleHealth(0xc0002)
goroutine 91 [sync.Mutex.Lock]:
github.com/yusefmosiah/go-choir/internal/objectgraph.(*engineMutex).lockAs(0xc0003)
"""


def sample(health, obs=None, t=0.0):
    return {"t": t, "health": health, "obs": obs}


class Classify(unittest.TestCase):
    def test_healthy(self):
        self.assertEqual(probe.classify(sample({"status": 200})), "healthy")

    def test_health_hang_during_online_gc(self):
        obs = {"status": 200, "frames": probe.frame_counts(GC_DUMP),
               "engine_mutex_live": {"holder_op": "GetObject choir.run", "held_ms": 90000}}
        self.assertEqual(probe.classify(sample({"timeout": True}, obs)), "h4_online_dolt_gc")

    def test_health_hang_behind_other_holder(self):
        obs = {"status": 200, "frames": probe.frame_counts("goroutine 1 [running]:\nmain.main()\n"),
               "engine_mutex_live": {"holder_op": "ListObjectsByOwnerAndBody choir.commitment_record $.addressee json", "held_ms": 61000}}
        self.assertEqual(probe.classify(sample({"timeout": True}, obs)),
                         "store_held:ListObjectsByOwnerAndBody choir.commitment_record $.addressee json")

    def test_health_hang_with_no_holder_named(self):
        obs = {"status": 200, "frames": {}, "engine_mutex_live": {"holder_op": "", "held_ms": 0}}
        self.assertEqual(probe.classify(sample({"status": 503}, obs)), "health_failing_unattributed")

    def test_process_stalled_when_observability_also_hangs(self):
        self.assertEqual(probe.classify(sample({"timeout": True}, {"timeout": True})), "process_stalled")
        self.assertEqual(probe.classify(sample({"timeout": True}, None)), "process_stalled")

    def test_frame_counts(self):
        counts = probe.frame_counts(GC_DUMP + "internal/store.(*Store).runOnlineDoltGCx\n")
        self.assertEqual(counts["runOnlineDoltGC"], 1)
        self.assertEqual(counts["database/sql.(*DB).conn"], 1)
        self.assertEqual(counts["engineMutex).lockAs"], 1)
        self.assertEqual(counts["HandleHealth"], 1)
        self.assertEqual(counts["finalizeBatch"], 0)


class Summarize(unittest.TestCase):
    def test_window_bounds_and_counts(self):
        obs_gc = {"status": 200, "frames": probe.frame_counts(GC_DUMP), "engine_mutex_live": {}}
        samples = [
            sample({"status": 200}, t=0),
            sample({"timeout": True}, obs_gc, t=10),
            sample({"timeout": True}, obs_gc, t=20),
            sample({"timeout": True}, {"timeout": True}, t=30),
            sample({"status": 200}, t=40),
        ]
        s = probe.summarize(samples)
        self.assertEqual(s["first_failure_t"], 10)
        self.assertEqual(s["recovered_t"], 40)
        self.assertEqual(s["verdicts"], {"healthy": 2, "h4_online_dolt_gc": 2, "process_stalled": 1})

    def test_no_failure(self):
        s = probe.summarize([sample({"status": 200}, t=0)])
        self.assertIsNone(s["first_failure_t"])
        self.assertIsNone(s["recovered_t"])


if __name__ == "__main__":
    unittest.main()
