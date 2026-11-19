"""Tests of scripts/embed-pass-export.py: `python3 -I scripts/embed_pass_export_test.py`. No server, no network."""
import importlib.util, io, json, os, sys, unittest, contextlib

here = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("e", os.path.join(here, "embed-pass-export.py"))
e = importlib.util.module_from_spec(spec); spec.loader.exec_module(e)

H = "a" * 64

class T(unittest.TestCase):
    def test_private_label_forms(self):
        yes = [{"labels": ["owner_private:capture"]}, {"labels": ["world_untrusted", " Owner_Private:mail "]},
               {"labels": "owner_private:capture"}, {"labels": {"x": 1}}, {"labels": [7]}, {"labels": ["owner_private"]}]
        no = [{}, {"labels": []}, {"labels": ["world_untrusted"]}, {"labels": None}, {"title": "owner_private:capture"}]
        for m in yes: self.assertTrue(e.private_label(m), m)
        for m in no: self.assertFalse(e.private_label(m), m)

    def run_captures(self, rows):
        e.psql_rows = lambda sql: iter(rows)
        e.blob = lambda p, limit=None: b"First para.\n\nSecond para."
        out, c = io.StringIO(), {k: 0 for k in ("seen", "exported", "refused_private", "refused_tombstone_or_superseded",
            "skipped_bad_metadata", "skipped_no_text_blob", "skipped_unreadable_blob", "skipped_no_text")}
        with contextlib.redirect_stdout(out): e.captures("", 0, c)
        return [json.loads(l) for l in out.getvalue().splitlines()], c

    def test_a_private_or_removed_capture_never_leaves(self):
        m = lambda **k: json.dumps({"title": "T", "extracted_text_blob_id": "sha256:" + H, **k})
        rows = [["c-ok", "f", "", m()], ["c-priv", "f", "", m(labels=["owner_private:capture"])],
                ["c-tomb", "t", "", m()], ["c-sup", "f", "c-new", m()], ["c-badlabels", "f", "", m(labels="x")],
                ["c-notext", "f", "", json.dumps({"title": "T"})], ["c-badjson", "f", "", "{"]]
        out, c = self.run_captures(rows)
        self.assertEqual([o["id"] for o in out], ["c-ok"])
        self.assertEqual((c["refused_private"], c["refused_tombstone_or_superseded"], c["skipped_no_text_blob"], c["skipped_bad_metadata"]), (2, 2, 1, 1))
        self.assertNotIn("owner_private", json.dumps(out))
        self.assertEqual(out[0]["text"], "First para.\n\nSecond para."); self.assertEqual(out[0]["ordinal"], 0)

    def test_the_query_names_the_papers_scope_and_is_read_only(self):
        seen = []
        e.psql_rows = lambda sql: (seen.append(sql), iter([]))[1]
        e.captures("", 0, {"seen": 0}); e.stories(0, {"seen": 0})
        self.assertIn(e.PAPER_OWNER, seen[0]); self.assertIn(e.PAPER_COMPUTER, seen[0]); self.assertIn(e.PAPER_OWNER, seen[1])

    def test_passages_cut_like_the_go_index(self):
        t = ("word " * 300).strip().encode()          # one paragraph of 1,499 runes: one passage
        self.assertEqual(len(e.passages(t)), 1)
        self.assertEqual(len(e.passages(b"a\n\n\n b \r\n\r\n c")), 1)   # short blocks pack together
        big = ("x" * 2000 + "\n\n") * 3
        b = big.encode()      # paragraphs of 2,000 runes do not pack (4,000 > 1,800): three passages
        self.assertEqual([e_ - s for s, e_ in e.passages(b)], [2000, 2000, 2000])

if __name__ == "__main__":
    unittest.main()
