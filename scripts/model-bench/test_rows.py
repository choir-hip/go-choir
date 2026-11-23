import os
import tempfile
import unittest

import rows


class RowsTest(unittest.TestCase):
    def test_header_once_and_column_order(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "r.tsv")
            rows.append_row(p, {"phone": "a", "wer": "0.1"})
            rows.append_row(p, {"wer": "0.2", "phone": "b"})
            lines = open(p).read().splitlines()
            self.assertEqual(lines[0].split("\t"), rows.COLUMNS)
            self.assertEqual(len(lines), 3)
            first = lines[1].split("\t")
            self.assertEqual(len(first), len(rows.COLUMNS))
            self.assertEqual(first[rows.COLUMNS.index("phone")], "a")
            self.assertEqual(first[rows.COLUMNS.index("wer")], "0.1")
            self.assertEqual(first[rows.COLUMNS.index("rtf")], "")  # missing is empty, not zero

    def test_unknown_column_rejected(self):
        with tempfile.TemporaryDirectory() as d:
            with self.assertRaises(KeyError):
                rows.append_row(os.path.join(d, "r.tsv"), {"nope": 1})

    def test_tabs_and_newlines_cannot_break_a_row(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "r.tsv")
            rows.append_row(p, {"notes": "a\tb\nc"})
            self.assertEqual(len(open(p).read().splitlines()), 2)

    def test_required_columns_present(self):
        for c in ["phone", "soc", "android", "runtime", "provider", "threads", "model_id",
                  "quant", "model_bytes", "set", "utterances", "audio_seconds", "wer",
                  "load_seconds", "decode_seconds", "rtf", "peak_rss_mib", "thermal_before",
                  "thermal_after", "battery_temp_before", "battery_temp_after", "plugged",
                  "date", "harness_commit"]:
            self.assertIn(c, rows.COLUMNS)


if __name__ == "__main__":
    unittest.main()
