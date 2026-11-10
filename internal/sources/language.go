package sources

import (
	"strings"
	"unicode"
)

// Language guess (lane Q, docs/problems/autopaper-article-quality-2026-11-09.md).
// An item's Language was its source's first language, so a mixed or
// mislabelled feed was wrong item by item (rss.go) and nothing could tell
// that a story was written in French. GuessLanguage reads the text itself,
// as GuessScript does for the writing system: a vote of common function
// words in the languages the paper's sources are written in. It is a pure
// function of text, and it says "" when it is not sure.

// languageWords are common function words, chosen to be rare in the other
// languages listed. A word that two languages share counts for both.
var languageWords = map[string]map[string]bool{
	"en": wordSet("the and of to in is that for with on was are as by at from has have said it its be an will not but their after which were been this more than about"),
	"fr": wordSet("le les des une est dans pour que qui sur avec par pas au aux du ce ses sont mais il elle nous ont être été plus cette leur aussi ou ne se je vous mais très après selon"),
	"de": wordSet("der die das und ist nicht mit von den dem ein eine für auf sich zu im auch wird sind es bei nach aus wie wurde hat einen oder aber nur über"),
	"es": wordSet("el los las una por con para que del se es su al más como pero fue sus está han este entre sobre también ser fue sido muy año"),
	"pt": wordSet("os as uma não com para por que do da dos das em mais foi são seu sua também ao pelo pela como mas ele ela isso"),
	"it": wordSet("il lo gli che di un una per con non sono della del nel alla anche più ha stato come nella dei delle questo ma"),
	"nl": wordSet("de het een van en is dat niet voor met zijn op aan ook bij naar werd om maar nog wordt dit door"),
}

func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(s) {
		out[w] = true
	}
	return out
}

const (
	// languageMinWords is how many letters-words a text needs before a guess
	// is made; a short headline is left to its source's label.
	languageMinWords = 8
	// languageMinShare is the lowest share of a text's words the winning
	// language's function words must reach.
	languageMinShare = 0.12
	// languageMargin is how many times the runner-up's hits the winner needs.
	languageMargin = 1.8
)

// languageHits counts each listed language's function words in words.
func languageHits(words []string) map[string]int {
	out := map[string]int{}
	for lang, set := range languageWords {
		for _, w := range words {
			if set[w] {
				out[lang]++
			}
		}
	}
	return out
}

func textWords(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) })
}

// GuessLanguage names the language of text as an ISO 639-1 code (en, fr, de,
// es, pt, it, nl) when the text is in the Latin script and one of those
// languages' function words clearly leads, and "" otherwise. Text in another
// script is not guessed here; GuessScript names its writing system.
func GuessLanguage(text string) string {
	words := textWords(text)
	if len(words) < languageMinWords || GuessScript(text) != ScriptLatin {
		return ""
	}
	hits := languageHits(words)
	best, second, bestLang := 0, 0, ""
	for _, lang := range []string{"en", "fr", "de", "es", "pt", "it", "nl"} {
		switch n := hits[lang]; {
		case n > best:
			second, best, bestLang = best, n, lang
		case n > second:
			second = n
		}
	}
	if bestLang == "" || float64(best) < languageMinShare*float64(len(words)) || float64(best) < languageMargin*float64(second) {
		return ""
	}
	return bestLang
}

// notEnglishMinWords is how long a text must be before LooksNotEnglish
// judges a Latin-script one.
const notEnglishMinWords = 30

// LooksNotEnglish reports whether text is written in a language other than
// English: mostly in another writing system, or in the Latin script with
// another listed language's function words clearly outnumbering English's.
// Quotes in another language inside English prose do not flip it: the vote
// is over the whole text. Short text and unsure text are not refused.
func LooksNotEnglish(text string) bool {
	switch GuessScript(text) {
	case ScriptHan, ScriptKana, ScriptHangul, ScriptArabic, ScriptCyrillic:
		return true
	case ScriptLatin:
	default:
		return false
	}
	words := textWords(text)
	if len(words) < notEnglishMinWords {
		return false
	}
	hits := languageHits(words)
	other := 0
	for lang, n := range hits {
		if lang != "en" && n > other {
			other = n
		}
	}
	return float64(other) >= languageMinShare*float64(len(words)) && other >= 2*hits["en"]
}
