package capturefts

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/yusefmosiah/go-choir/internal/captureterms"
)

// Terms is the one tokenizer of the index and of its queries, so a word is
// found the way it was written: the capture term table's tokens
// (captureterms: lowercased runs of letters and digits; Han, kana, Thai, Lao,
// Khmer, Myanmar, Arabic and Hebrew as character unigrams and bigrams, the
// segmenter these scripts lack), the published-pieces index's light plural
// fold (captureterms.PluralFold), and accents folded on Latin words
// ("Café" is "cafe"). Repeats are kept.
func Terms(text string) []string {
	text = norm.NFC.String(text)
	toks := captureterms.Tokens(text)
	for i, t := range toks {
		toks[i] = foldTerm(t)
	}
	return toks
}

func foldTerm(t string) string {
	return captureterms.PluralFold(foldAccents(t))
}

// foldAccents removes combining marks from a Latin-script word. Other
// scripts keep theirs (a Cyrillic "й" and a Thai vowel mark are letters of
// those scripts, not accents).
func foldAccents(t string) string {
	ascii := true
	for i := 0; i < len(t); i++ {
		if t[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return t
	}
	latin := false
	for _, r := range t {
		if unicode.Is(unicode.Latin, r) {
			latin = true
			break
		}
	}
	if !latin {
		return t
	}
	var b strings.Builder
	for _, r := range norm.NFD.String(t) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return norm.NFC.String(b.String())
}

// spanTerms is captureterms.TokenSpans with the index's folding.
func spanTerms(text string) []captureterms.TokenSpan {
	spans := captureterms.TokenSpans(text)
	for i := range spans {
		spans[i].Term = foldTerm(spans[i].Term)
	}
	return spans
}

// queryStopwords are dropped from a query: they match nearly every passage,
// add nothing to a BM25 score, and make a long question cost as much as a
// scan. They stay in the index (a quoted phrase search may want them).
var queryStopwords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`a an the and or but nor of in on at to for from by with about into over under as is are was were be been being am
		it its this that these those there their they them he she his her we us our you your i me my do does did done will would can could should may might
		not no what whats who whom whose how why when where which while than then so if also just very more most some any each other such only own same too
		has have had having get got latest current recent new says said say`) {
		m[w] = true
	}
	return m
}()

// MatchedTerms are the query terms that occur in text, in the query's order.
func MatchedTerms(text string, terms []string) []string {
	have := map[string]bool{}
	for _, t := range Terms(text) {
		have[t] = true
	}
	var out []string
	for _, t := range terms {
		if have[t] {
			out = append(out, t)
		}
	}
	return out
}

// Snippet is a window of at most maxRunes runes of text around the densest
// cluster of query terms, cut at word boundaries and marked with an ellipsis
// where text continues. A text with no term starts at its beginning.
func Snippet(text string, terms []string, maxRunes int) string {
	text = strings.ToValidUTF8(text, "�")
	runes := []rune(text)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return strings.TrimSpace(text)
	}
	want := map[string]bool{}
	for _, t := range terms {
		want[t] = true
	}
	// byte offset -> rune index
	runeAt := make(map[int]int, len(runes))
	off := 0
	for i, r := range runes {
		runeAt[off] = i
		off += len(string(r))
	}
	runeAt[off] = len(runes)
	type hit struct {
		at   int
		term string
	}
	var hits []hit
	for _, sp := range spanTerms(text) {
		if want[sp.Term] {
			if i, ok := runeAt[sp.Start]; ok {
				hits = append(hits, hit{i, sp.Term})
			}
		}
	}
	start := 0
	if len(hits) > 0 {
		best, bestAt := -1, hits[0].at
		for i := range hits {
			distinct := map[string]bool{}
			for j := i; j < len(hits) && hits[j].at-hits[i].at < maxRunes*2/3; j++ {
				distinct[hits[j].term] = true
			}
			if len(distinct) > best {
				best, bestAt = len(distinct), hits[i].at
			}
		}
		start = max(0, bestAt-maxRunes/6)
	}
	end := min(len(runes), start+maxRunes)
	// Snap to word boundaries (spaces) when the cut falls inside a word of a
	// spaced script.
	if start > 0 {
		for s := start; s < min(start+30, end) && s < len(runes); s++ {
			if unicode.IsSpace(runes[s-1]) {
				start = s
				break
			}
		}
	}
	if end < len(runes) {
		for e := end; e > max(start+1, end-30); e-- {
			if unicode.IsSpace(runes[e]) {
				end = e
				break
			}
		}
	}
	out := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}
