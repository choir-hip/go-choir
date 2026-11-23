import unittest

import parse

OFFLINE = '''Creating recognizer ...
recognizer created in 3.903 s
Started
Done!

wav/a.wav
----
wav/b.wav
----
num threads: 4
decoding method: greedy_search
Elapsed seconds: 3.869 s
Real time factor (RTF): 3.869 / 19.730 = 0.196
{"lang": "", "text": "That will be the safest. No, no, never", "tokens":[" That"], "words": []}
{"lang": "", "text": "Thought you'd be in the chips.", "tokens":[" Th"], "words": []}
'''

ONLINE = '''Start to create recognizer
Recognizer created in 4.64513 s
wav/a.wav
Number of threads: 4, Elapsed seconds: 1, Audio duration (s): 3.5, Real time factor (RTF) = 1/3.5 = 0.29
THAT WILL BE
{ "text": "THAT WILL BE", "tokens": [" THAT"], "words": []}

wav/b.wav
Number of threads: 4, Elapsed seconds: 0.79, Audio duration (s): 2.3, Real time factor (RTF) = 0.79/2.3 = 0.35
THOUGHT YOU'D
{ "text": "THOUGHT YOU'D", "tokens": [" THOUGHT"], "words": []}
'''


class ParseTest(unittest.TestCase):
    def test_offline(self):
        r = parse.parse_sherpa(OFFLINE)
        self.assertEqual(r["load_seconds"], 3.903)
        self.assertEqual(r["decode_seconds"], 3.869)
        self.assertEqual(r["texts"], ["That will be the safest. No, no, never", "Thought you'd be in the chips."])

    def test_online_has_no_precise_decode_time(self):
        r = parse.parse_sherpa(ONLINE)
        self.assertEqual(r["load_seconds"], 4.64513)
        self.assertIsNone(r["decode_seconds"])  # per-file seconds are printed to 2 digits only
        self.assertEqual(r["texts"], ["THAT WILL BE", "THOUGHT YOU'D"])

    def test_meta(self):
        m = parse.parse_meta("exit=0 wall_start=100.5 wall_end=110.75 vmhwm_kb=2048\n")
        self.assertEqual(m, {"exit": 0, "wall_seconds": 10.25, "vmhwm_kb": 2048})

    def test_thermal_and_battery(self):
        self.assertEqual(parse.thermal_status("Thermal Status: 0\nfoo"), 0)
        bat = "  AC powered: true\n  USB powered: false\n  level: 96\n  temperature: 220\n"
        self.assertEqual(parse.battery(bat), {"plugged": "ac", "level": 96, "temp_c": 22.0})
        self.assertEqual(parse.battery(bat.replace("AC powered: true", "AC powered: false")
                                       .replace("USB powered: false", "USB powered: true"))["plugged"], "usb")
        self.assertEqual(parse.battery("  AC powered: false\n  USB powered: false\n  level: 5\n  temperature: 301\n")["plugged"], "no")


if __name__ == "__main__":
    unittest.main()
