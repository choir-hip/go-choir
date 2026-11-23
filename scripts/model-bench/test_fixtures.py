import hashlib
import os
import tempfile
import unittest
import wave

import fixtures


class OrderTest(unittest.TestCase):
    def test_shuffled_is_deterministic_and_input_order_free(self):
        ids = ["c-%d" % i for i in range(50)]
        a = fixtures.shuffled(ids, 7)
        self.assertEqual(a, fixtures.shuffled(list(reversed(ids)), 7))
        self.assertNotEqual(a, fixtures.shuffled(ids, 8))
        self.assertEqual(sorted(a), sorted(ids))

    def test_interleave_alternates_clean_and_other(self):
        self.assertEqual(fixtures.interleave(["c1", "c2"], ["o1", "o2"]), ["c1", "o1", "c2", "o2"])

    def test_smoke_set_is_prefix_of_full_set(self):
        clean = ["c%d" % i for i in range(300)]
        other = ["o%d" % i for i in range(300)]
        full = fixtures.choose(clean, other, seed=1, n=100)
        smoke = fixtures.choose(clean, other, seed=1, n=10)
        self.assertEqual(len(full), 100)
        self.assertEqual(full[:10], smoke)
        self.assertEqual(sum(1 for i in full if i.startswith("c")), 50)


class ManifestTest(unittest.TestCase):
    def test_roundtrip_and_sha(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "a.wav")
            with wave.open(p, "wb") as w:
                w.setnchannels(1)
                w.setsampwidth(2)
                w.setframerate(16000)
                w.writeframes(b"\0\0" * 32000)
            self.assertAlmostEqual(fixtures.wav_seconds(p), 2.0)
            row = fixtures.manifest_row("a", "clean", "HELLO\tWORLD", p)
            self.assertEqual(row["sha256"], hashlib.sha256(open(p, "rb").read()).hexdigest())
            m = os.path.join(d, "manifest.tsv")
            fixtures.write_manifest(m, [row])
            self.assertEqual(fixtures.read_manifest(m), [row])
            # Same input, same bytes.
            m2 = os.path.join(d, "m2.tsv")
            fixtures.write_manifest(m2, [row])
            self.assertEqual(open(m, "rb").read(), open(m2, "rb").read())


if __name__ == "__main__":
    unittest.main()
