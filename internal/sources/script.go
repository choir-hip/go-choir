package sources

import "unicode"

// Script guesses (L2a A5, defect 5 in
// docs/problems/autopaper-intake-defects-2026-10-14.md). A capture's
// Language is its source's first language; the script guess is read from
// the item's own text, so a mixed-language feed is not mislabelled.
const (
	ScriptHan      = "han"
	ScriptKana     = "kana"
	ScriptHangul   = "hangul"
	ScriptArabic   = "arabic"
	ScriptCyrillic = "cyrillic"
	ScriptLatin    = "latin"
	ScriptOther    = "other"
)

// GuessScript names the writing system most of text's letters are in: han,
// kana, hangul, arabic, cyrillic, latin, or other. Digits, punctuation and
// spaces do not count. Any kana makes the text Japanese, so its han letters
// (kanji) count as kana. Text with no letters, or mostly in another script,
// is other. It is a pure function of text.
func GuessScript(text string) string {
	counts := map[string]int{}
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			counts[ScriptKana]++
		case unicode.Is(unicode.Han, r):
			counts[ScriptHan]++
		case unicode.Is(unicode.Hangul, r):
			counts[ScriptHangul]++
		case unicode.Is(unicode.Arabic, r):
			counts[ScriptArabic]++
		case unicode.Is(unicode.Cyrillic, r):
			counts[ScriptCyrillic]++
		case unicode.Is(unicode.Latin, r):
			counts[ScriptLatin]++
		case unicode.IsLetter(r):
			counts[ScriptOther]++
		}
	}
	if counts[ScriptKana] > 0 {
		counts[ScriptKana] += counts[ScriptHan]
		counts[ScriptHan] = 0
	}
	best, bestCount := ScriptOther, 0
	// A fixed order breaks ties the same way every time.
	for _, script := range []string{ScriptLatin, ScriptHan, ScriptKana, ScriptHangul, ScriptArabic, ScriptCyrillic, ScriptOther} {
		if counts[script] > bestCount {
			best, bestCount = script, counts[script]
		}
	}
	return best
}
