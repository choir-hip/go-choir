import os
import tempfile
import unittest

import native
import vad


class VadTest(unittest.TestCase):
    def test_parses_speech_segments(self):
        text = "VadModelConfig(...)\n0.542 -- 4.000\nSaved to output.wav\n"
        self.assertEqual(vad.parse_segments(text), [(0.542, 4.0)])

    def test_refuses_output_without_a_segment(self):
        with self.assertRaises(ValueError):
            vad.parse_segments("Saved to output.wav\n")

    def test_command_pins_silero_window_and_threads(self):
        cmd = vad.command(4)
        self.assertEqual(cmd[0], "./sherpa-onnx-vad")
        self.assertIn("--silero-vad-window-size=512", cmd)
        self.assertIn("--vad-num-threads=4", cmd)

    def test_model_hash_mismatch_is_refused(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "silero.onnx")
            with open(p, "wb") as f:
                f.write(b"bad")
            with self.assertRaises(ValueError):
                native.verify_artifact(p, "0" * 64, 3)

    def test_wrapper_preloads_the_libcxx_hash_shim(self):
        self.assertIn('LD_PRELOAD="$D/libhashshim.so"', vad.WRAPPER)
        self.assertIn('LD_LIBRARY_PATH="$D"', vad.WRAPPER)


if __name__ == "__main__":
    unittest.main()
