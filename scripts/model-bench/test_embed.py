import os
import tempfile
import unittest

import embed


class EmbedTest(unittest.TestCase):
    def test_parses_the_public_c_api_result(self):
        text = "dim=768 tokens=15 load_seconds=1.361657 prefill_seconds=0.091066 raw_norm=1092.753281 first=39.5812149\n"
        self.assertEqual(embed.parse_result(text), {
            "dim": 768, "tokens": 15, "load_seconds": 1.361657,
            "prefill_seconds": 0.091066, "raw_norm": 1092.753281,
            "first": 39.5812149,
        })

    def test_refuses_a_wrong_dimension_or_empty_output(self):
        with self.assertRaises(ValueError):
            embed.parse_result("dim=256 tokens=15 load_seconds=1 prefill_seconds=0.1 raw_norm=2 first=1\n")
        with self.assertRaises(ValueError):
            embed.parse_result("")

    def test_command_pins_cpu_and_four_threads(self):
        cmd = embed.command(4)
        self.assertEqual(cmd[0], "./embed-bench")
        self.assertIn("embeddinggemma-2-Q8_0.gguf", cmd)
        self.assertEqual(cmd[-1], "4")

    def test_model_hash_mismatch_is_refused(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "model.gguf")
            with open(p, "wb") as f:
                f.write(b"bad")
            with self.assertRaises(ValueError):
                embed.native.verify_artifact(p, "0" * 64, 3)


if __name__ == "__main__":
    unittest.main()
