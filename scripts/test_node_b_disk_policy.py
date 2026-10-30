"""Node B's Nix store and host disk policy, checked as text and as a tool.

    python3 -I scripts/test_node_b_disk_policy.py

The host units cannot run here (no Nix, no Node B), so their declarations in
nix/node-b.nix and the deploy preflight's floor are checked as text, the way
internal/maild/digest_nix_test.go checks the digest unit. The dedup estimator
runs for real on a temp tree.

Failure modes (docs/problems/node-b-nix-store-and-host-disk-policy-2026-10-22.md),
written before the change:
  N1  new store paths wait a week for dedup (auto-optimise-store off)
  N2  nix's GC band stays at 80/100 GiB, so the host must idle near 90 GiB
  N3  the deploy floor falls without min-free/max-free falling with it: a
      deploy build starts below min-free and nix's auto-GC deletes its own
      unrooted inputs mid-build (2026-09-17)
  N4  the generation count is written in more than one place and drifts
  N5  no host disk alarm, or one that fires on other thresholds
  N6  the new pruning ships in enforce instead of observe
  E1  the estimator counts a hard link or a symlink as reclaimable
  E2  the estimator walks /nix/store/.links (already deduplicated)
  E3  the estimator writes anything
  N7  the floor leaves no room for a guest store compaction (13.5 GiB peak)
      or the paper's regrowth overlapping a deploy (review round 2, DS-2),
      with no margin beyond the terms (round 2b, DS2b-3)
  E4  the estimator hashes files over its size cap (the guest images,
      through the shared page cache; S-F10)
"""

import importlib.util
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
REPO = HERE.parent
GiB = 1 << 30


def nix_src():
    return (REPO / "nix" / "node-b.nix").read_text()


def block(src, marker):
    i = src.index(marker)
    rest = src[i:]
    end = rest.index("\n  };\n")
    return rest[:end]


def nix_setting(src, name):
    m = re.search(r"^\s*" + re.escape(name) + r"\s*=\s*([^;]+);", src, re.M)
    if not m:
        raise AssertionError("node-b.nix sets no " + name)
    return m.group(1).strip()


def preflight_floor_bytes():
    text = (HERE / "node-b-deploy-disk-preflight").read_text()
    m = re.search(r'DEPLOY_MIN_FREE_KIB:-(\d+)', text)
    if not m:
        raise AssertionError("preflight has no DEPLOY_MIN_FREE_KIB default")
    return int(m.group(1)) * 1024


class NixPolicy(unittest.TestCase):
    def test_n1_auto_optimise(self):
        self.assertEqual(nix_setting(nix_src(), "auto-optimise-store"), "true")

    def test_n2_gc_band(self):
        src = nix_src()
        self.assertEqual(int(nix_setting(src, "min-free")), 24 * GiB)
        self.assertEqual(int(nix_setting(src, "max-free")), 40 * GiB)

    def test_n3_floor_coupled_to_gc_band(self):
        src = nix_src()
        min_free = int(nix_setting(src, "min-free"))
        max_free = int(nix_setting(src, "max-free"))
        floor = preflight_floor_bytes()
        # A deploy that starts at the floor must end above min-free, so
        # nix's auto-GC never runs inside the build, even when the guest
        # store compaction (peak new allocation ~13.5 GiB) and the paper's
        # regrowth (up to ~290 MiB/min over a 30 min deploy) overlap it.
        footprint = 8 * GiB
        compaction = 14 * GiB
        regrowth = 10 * GiB
        self.assertGreater(min_free, 0)
        self.assertGreater(max_free, min_free)
        margin = 4 * GiB
        self.assertGreaterEqual(floor - footprint - compaction - regrowth - margin, min_free)
        self.assertGreaterEqual(floor - footprint, max_free)
        self.assertEqual(floor, 60 * GiB)

    def test_n4_one_generation_count(self):
        src = nix_src()
        pre = (HERE / "node-b-deploy-disk-preflight").read_text()
        for name, text in (("node-b.nix", src), ("preflight", pre)):
            self.assertIsNone(re.search(r"--delete-generations\s+\+\d", text), name + " still prunes generations itself")
        self.assertIn("../scripts/node-b-nix-retention", src)
        self.assertIn("node-b-nix-retention", pre)
        self.assertIn("nixRetention", block(src, "systemd.services.go-choir-nix-gc ="))
        sweep = src[src.index("diskRetentionSweep = "):src.index("# Off-host backup stopgap")]
        self.assertIn("nixRetention", sweep)

    def test_n5_host_disk_alarm(self):
        src = nix_src()
        svc = block(src, "systemd.services.go-choir-host-disk-alarm =")
        timer = block(src, "systemd.timers.go-choir-host-disk-alarm =")
        self.assertIn("../scripts/node-b-host-disk-alarm", svc)
        self.assertIn('"HOST_DISK_WARN_FREE_GIB=60"', svc)
        self.assertIn('"HOST_DISK_WARN_UNALLOC_GIB=20"', svc)
        self.assertIn('"HOST_DISK_ERR_FREE_GIB=24"', svc)
        self.assertIn('"HOST_DISK_ERR_UNALLOC_GIB=10"', svc)
        self.assertRegex(timer, r'OnCalendar\s*=\s*"\*:0/15";')
        self.assertRegex(timer, r"Persistent\s*=\s*false;")
        # The free warning is the deploy floor: a warning means the next
        # deploy is refused.
        self.assertEqual(60 * GiB, preflight_floor_bytes())

    def test_n6_retention_observe(self):
        src = nix_src()
        # Owner, 2026-10-23: retention enforces (was observe through D6).
        self.assertIn('"GO_CHOIR_NIX_RETENTION_MODE=enforce"', block(src, "systemd.services.go-choir-nix-gc ="))
        self.assertIn('"GO_CHOIR_NIX_RETENTION_MODE=enforce"', block(src, "systemd.services.go-choir-disk-gc ="))
        self.assertNotIn("GO_CHOIR_NIX_RETENTION_MODE=observe", src)


class DedupEstimate(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.tmp.name) / "store"
        r = self.root
        for d in ("a", "b", "c", "d", "e", "f", "g", ".links"):
            (r / d).mkdir(parents=True)
        A, B, C = b"A" * 5000, b"B" * 5000, b"C" * 300
        (r / "a" / "x").write_bytes(A)
        (r / "b" / "x").write_bytes(A)            # duplicate of a/x, own inode
        os.link(r / "a" / "x", r / "c" / "x")      # hard link: already shared
        (r / "d" / "y").write_bytes(B)            # same size as A, other bytes
        (r / "e" / "z").write_bytes(C)
        (r / "f" / "z").write_bytes(C)
        os.symlink(r / "a" / "x", r / "g" / "link")
        (r / ".links" / "0abc").write_bytes(A)    # nix's own link farm
        self.size_a, self.size_c = len(A), len(C)

    def tearDown(self):
        self.tmp.cleanup()

    def run_tool(self, *extra):
        out = subprocess.run(
            [sys.executable, "-I", str(HERE / "nix-store-dedup-estimate.py"), "--root", str(self.root), "--json", *extra],
            check=True, capture_output=True, text=True,
        ).stdout
        import json
        return json.loads(out)

    def test_e1_e2_reclaimable(self):
        before = sorted((p, p.lstat().st_mtime_ns) for p in self.root.rglob("*"))
        r = self.run_tool()
        self.assertEqual(r["reclaimable_bytes"], self.size_a + self.size_c)
        self.assertEqual(r["duplicate_groups"], 2)
        # a/x and c/x are one inode with two names
        self.assertEqual(r["already_linked_bytes"], self.size_a)
        # .links is skipped: 6 regular file names outside it
        self.assertEqual(r["files"], 6)
        after = sorted((p, p.lstat().st_mtime_ns) for p in self.root.rglob("*"))
        self.assertEqual(before, after, "E3: the estimator changed the tree")

    def test_e4_size_cap(self):
        r = self.run_tool("--max-file-bytes", "1000")
        # A (5000 bytes) is over the cap: only C's duplicate counts, and the
        # skipped bytes are reported, not hidden.
        self.assertEqual(r["reclaimable_bytes"], self.size_c)
        # a/x (and its hard link c/x), b/x, d/y: three distinct inodes
        self.assertEqual(r["skipped_over_cap_files"], 3)


if __name__ == "__main__":
    unittest.main()
