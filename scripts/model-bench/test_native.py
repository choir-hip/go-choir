import hashlib
import io
import json
import os
import tempfile
import unittest

import native


class ParseTest(unittest.TestCase):
    def test_llama_bench_json_after_backend_logs(self):
        text = 'load_backend: loaded CPU backend\n[{"n_prompt":512,"n_gen":0,"avg_ts":205.0}]\n'
        self.assertEqual(native.parse_llama_bench(text)[0]["avg_ts"], 205.0)

    def test_llama_bench_refuses_missing_or_malformed_json(self):
        for text in ["load_backend only\n", "not json [nope]"]:
            with self.assertRaises(ValueError):
                native.parse_llama_bench(text)

    def test_tts_timings(self):
        text = "I timings: prompt eval 0.03s + generation 0.00s + vocoder 0.15s = total 0.18s\nI output audio = 2.10s (audio time = 11.99x process time)\n"
        got = native.parse_tts(text)
        self.assertEqual(got["process_seconds"], 0.18)
        self.assertEqual(got["audio_seconds"], 2.10)


class CommandTest(unittest.TestCase):
    def test_job_command_substitutes_numbers(self):
        job = {"executable": "llama-bench", "args": ["-m", "m.gguf", "-t", "{threads}", "-r", "{repeats}"]}
        self.assertEqual(native.job_command(job, 4, 2), ["./llama-bench", "-m", "m.gguf", "-t", "4", "-r", "2"])

    def test_job_command_refuses_absolute_or_parent_executable(self):
        for exe in ["/bin/sh", "../llama-bench", "bin/llama-bench"]:
            with self.assertRaises(ValueError):
                native.job_command({"executable": exe, "args": []}, 4, 1)


class ArtifactTest(unittest.TestCase):
    def test_hash_mismatch_is_refused_and_part_removed(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "model.gguf")
            with open(path, "wb") as f:
                f.write(b"good")
            digest = hashlib.sha256(b"good").hexdigest()
            native.verify_artifact(path, digest, 4)
            with self.assertRaises(ValueError):
                native.verify_artifact(path, "0" * 64, 4)

    def test_fetch_refuses_wrong_hash_and_leaves_no_final_file(self):
        with tempfile.TemporaryDirectory() as d:
            src = os.path.join(d, "src")
            dst = os.path.join(d, "dst")
            with open(src, "wb") as f:
                f.write(b"bad")
            with self.assertRaises(ValueError):
                native.fetch_artifact("file://" + src, dst, "0" * 64, 3)
            self.assertFalse(os.path.exists(dst))


class RowTest(unittest.TestCase):
    def test_native_row_has_stable_columns_and_blank_unknowns(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "rows.tsv")
            native.append_row(path, {"model_id": "qwen-q4", "decode_tps": 36.4})
            with open(path) as f:
                text = f.read()
            self.assertTrue(text.startswith("phone\tsoc\tandroid\t"))
            self.assertIn("qwen-q4\t", text)
            self.assertIn("36.4\t", text)


if __name__ == "__main__":
    unittest.main()
