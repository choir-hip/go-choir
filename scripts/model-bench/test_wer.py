import unittest

import wer


class NormalizeTest(unittest.TestCase):
    def test_lowercase_and_punctuation(self):
        self.assertEqual(wer.normalize("Don't STOP, now!"), "don't stop now")

    def test_hyphen_and_brackets_become_spaces(self):
        self.assertEqual(wer.normalize("well-known; (yes)"), "well known yes")

    def test_whitespace_collapsed(self):
        self.assertEqual(wer.normalize("  a \n b\t c "), "a b c")

    def test_digits_kept_not_expanded(self):
        self.assertEqual(wer.normalize("It was 1,000."), "it was 1 000")

    def test_stray_apostrophes_dropped(self):
        self.assertEqual(wer.normalize("'quoted' word"), "quoted word")


class WerTest(unittest.TestCase):
    def test_identical(self):
        self.assertEqual(wer.errors("the cat sat", "the cat sat").total, 0)

    def test_sub_and_deletion(self):
        # ref: the cat sat on the mat / hyp: the cat sit on mat
        # sat->sit is one substitution, one "the" deleted: 2 errors over 6 words.
        e = wer.errors("the cat sat on the mat", "the cat sit on mat")
        self.assertEqual((e.subs, e.dels, e.ins, e.ref_words), (1, 1, 0, 6))
        self.assertEqual(e.total, 2)

    def test_insertions(self):
        e = wer.errors("a b", "a x b y")
        self.assertEqual((e.subs, e.dels, e.ins, e.ref_words), (0, 0, 2, 2))

    def test_empty_hypothesis_is_all_deletions(self):
        e = wer.errors("one two three", "")
        self.assertEqual((e.subs, e.dels, e.ins), (0, 3, 0))

    def test_normalisation_applies_to_both_sides(self):
        e = wer.errors("DON'T STOP", "Don't stop.")
        self.assertEqual(e.total, 0)

    def test_corpus_wer_is_total_errors_over_total_words(self):
        pairs = [("the cat sat on the mat", "the cat sit on mat"), ("a b c", "a b c")]
        # 2 errors / 9 words, not the mean of 2/6 and 0/3.
        self.assertAlmostEqual(wer.corpus_wer(pairs), 2 / 9)

    def test_corpus_wer_empty_reference_raises(self):
        with self.assertRaises(ValueError):
            wer.corpus_wer([("", "x")])


if __name__ == "__main__":
    unittest.main()
