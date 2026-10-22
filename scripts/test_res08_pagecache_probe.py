#!/usr/bin/env python3
"""Behavioural test for scripts/res08_pagecache_probe.py's summary: when a
store number may be quoted, and what it surfaces.

Ways the summary could mislead FP0 (written before the code; review bunch 13):
  1. It quotes a store number from a read that predates the overlay split
     (no store_layering), or one that could not partition it: the release
     delta would sit inside the base image and inside persistent.
  2. It quotes a store number the kernel could not measure (unmeasured
     files), or one a bound cut short (partial).
  3. It hides skipped subtrees, so a floor reads as a total.
  4. It counts the release delta (store_upper) in the DAX memmap cost or
     the saving; only the base store is on the image.
  5. It quotes a store with skipped files or mounts as a floor. Skipped files
     add nothing to bytes, so the memmap cost is understated and the saving
     can be overstated when the skipped data is cold (review bunch 16).
  6. It prints raw store numbers beside store_quotable false, where they can
     be copied as if measured: they are null until quotable.
Run: python3 -I scripts/test_res08_pagecache_probe.py
"""
import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("probe", os.path.join(HERE, "res08_pagecache_probe.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)

MIB = 1 << 20


def read(layering=True, partitioned=True, store=None, upper=None, pers=None):
    roots = [dict({"name": "store", "present": True, "files": 900, "bytes": 6144 * MIB, "resident_bytes": 450 * MIB, "examined_files": 950}, **(store or {}))]
    if upper is not None:
        roots.append(dict({"name": "store_upper", "present": True, "files": 10, "bytes": 200 * MIB, "resident_bytes": 40 * MIB, "examined_files": 10}, **upper))
    roots.append(dict({"name": "persistent", "present": True, "files": 30, "bytes": 900 * MIB, "resident_bytes": 150 * MIB, "examined_files": 30}, **(pers or {})))
    pc = {"meminfo": {"Cached_kib": 655360, "Mapped_kib": 102400}, "roots": roots}
    if layering:
        pc["store_layering"] = {"fstype": "overlay" if upper is not None else "erofs", "partitioned": partitioned}
    return {"procmem": {"meminfo": {"AnonPages_kib": 100000}, "page_cache": pc}}


class Summary(unittest.TestCase):
    def test_quotes_a_clean_partitioned_read(self):
        s = probe.summarize(read(upper={}))
        self.assertTrue(s["store_quotable"], s)
        self.assertEqual(s["refusals"], [])
        self.assertEqual(s["store_resident_mib"], 450.0)
        self.assertEqual(s["store_upper_resident_mib"], 40.0)
        # memmap from the base image only: 6144 MiB / 4096 * 64 = 96 MiB
        self.assertEqual(s["dax_memmap_cost_mib"], 96.0)
        self.assertEqual(s["net_saving_mib"], 354.0)

    def test_refuses_a_read_without_layering(self):
        s = probe.summarize(read(layering=False))
        self.assertFalse(s["store_quotable"])
        self.assertNotIn("net_saving_mib", s)
        self.assertTrue(any("store_layering" in r for r in s["refusals"]), s["refusals"])

    def test_refuses_an_unpartitioned_read(self):
        s = probe.summarize(read(partitioned=False, upper={}))
        self.assertFalse(s["store_quotable"])
        self.assertNotIn("net_saving_mib", s)

    def test_refuses_unmeasured_partial_or_skipped_store(self):
        for store in ({"unmeasured_files": 3, "unmeasured_bytes": 4096}, {"partial": True}, {"skipped": 2}, {"skipped_mounts": 1}):
            s = probe.summarize(read(store=store))
            self.assertFalse(s["store_quotable"], store)
            self.assertNotIn("net_saving_mib", s)

    def test_store_numbers_are_null_while_unquotable(self):
        s = probe.summarize(read(layering=False, upper={}))
        self.assertFalse(s["store_quotable"])
        for k in ("store_resident_mib", "store_upper_resident_mib", "store_apparent_mib"):
            self.assertIsNone(s[k], k)
        # the diagnostics that explain the refusal stay
        self.assertEqual(s["store_examined_files"], 950)

    def test_surfaces_skips_and_marks_floors(self):
        s = probe.summarize(read(store={"skipped": 2}, pers={"skipped_mounts": 1, "unmeasured_files": 4}))
        self.assertEqual(s["store_skipped"], 2)
        self.assertEqual(s["persistent_skipped_mounts"], 1)
        self.assertEqual(s["persistent_unmeasured_files"], 4)
        self.assertIn("store", s["floors"])
        self.assertIn("persistent", s["floors"])
        self.assertFalse(s["store_quotable"], s)

    def test_a_persistent_floor_alone_does_not_refuse_the_store(self):
        s = probe.summarize(read(pers={"skipped_mounts": 2}))
        self.assertIn("persistent", s["floors"])
        self.assertTrue(s["store_quotable"], s)


if __name__ == "__main__":
    unittest.main()
