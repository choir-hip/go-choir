#!/usr/bin/env python3
"""autopaper_source_audit.py parsing against a fixture of invented text.

  python3 -I scripts/autopaper_source_audit_test.py

Ways the parser can fail, written first: a www/co.uk host counts as a
different outlet; a doubled cite marker is missed; a French story is read as
English; a two-anchor source is counted as two sources; the 24h window
includes a story outside it; a diff shows nothing when a number moved.
"""
import datetime as dt
import importlib.util
import json
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("aud", os.path.join(HERE, "autopaper_source_audit.py"))
aud = importlib.util.module_from_spec(spec)
spec.loader.exec_module(aud)
FX = json.load(open(os.path.join(HERE, "fixtures", "autopaper-audit", "fixture.json")))


class T(unittest.TestCase):
    def setUp(self):
        it = FX["front"]["stories"]
        self.a = aud.parse_story(it[0], FX["resolve_a"])
        self.b = aud.parse_story(it[1], FX["resolve_b"])

    def test_domains(self):
        self.assertEqual(aud.domain_of("https://www.example-one.com/x"), "example-one.com")
        self.assertEqual(aud.domain_of("https://news.example-two.co.uk/x"), "example-two.co.uk")
        self.assertEqual(self.a["domains"], ["example-one.com", "example-two.co.uk"])

    def test_sources_and_cites(self):
        self.assertEqual(self.a["n_sources"], 2)
        self.assertEqual(self.a["n_outlets"], 2)
        self.assertEqual(self.a["n_anchors"], 3)
        self.assertEqual(sorted(s["cites"] for s in self.a["sources"]), [1, 2])
        self.assertEqual(self.b["n_outlets"], 1)

    def test_adjacent_marker_and_checks(self):
        self.assertEqual(self.a["adjacent_dup_cites"], 1)
        self.assertEqual(self.a["checks"], {"supported": 2, "failed": 1})
        self.assertEqual(self.a["checks_by_domain"]["example-one.com"]["failed"], 1)

    def test_language(self):
        self.assertEqual(self.a["lang"], "en")
        self.assertEqual(self.b["lang"], "fr")

    def test_url_date(self):
        self.assertEqual(aud.url_date("https://x.com/2026/11/09/a"), dt.date(2026, 11, 9))
        self.assertEqual(aud.url_date("https://x.com/a-20261109"), dt.date(2026, 11, 9))
        self.assertIsNone(aud.url_date("https://x.com/a"))

    def test_windows_and_new_only(self):
        S = [self.a, self.b]
        now = dt.datetime(2026, 11, 9, 11, 30, tzinfo=dt.timezone.utc)
        w24 = aud.window_summary(S, "24h", now - dt.timedelta(hours=1), now, set())
        self.assertEqual(w24["stories"], 1)
        w7 = aud.window_summary(S, "7d", now - dt.timedelta(days=7), now, set())
        self.assertEqual(w7["stories"], 2)
        self.assertEqual(w7["single_outlet_stories"], 1)
        self.assertEqual(w7["language"], {"en": 1, "fr": 1})
        h = aud.headline(S, {"aaaaaaaaa"})
        self.assertEqual(h["new_stories"], 1)
        self.assertEqual(h["non_english"], 1)

    def test_dup_groups(self):
        t0 = dt.datetime(2026, 11, 9, tzinfo=dt.timezone.utc)
        items = [{"id": "1", "title": "Harbour opens new ferry line today", "ts": t0},
                 {"id": "2", "title": "Harbour opens new ferry line", "ts": t0 + dt.timedelta(hours=5)},
                 {"id": "3", "title": "Harbour opens new ferry line", "ts": t0 + dt.timedelta(days=5)}]
        g, n = aud.dup_groups(items)
        self.assertEqual((len(g), n), (1, 2))

    def test_diff_shows_movement(self):
        d = aud.diff_summaries({"window_7d": {"stories": 5}}, {"window_7d": {"stories": 9}})
        self.assertIn("window_7d.stories", d)


if __name__ == "__main__":
    unittest.main()
