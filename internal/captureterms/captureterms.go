// Package captureterms is the tokenizer of Store B's capture term table
// (rung 3 of docs/archive/reports/object-graph-retrieval-research-2026-10-14.md).
// It replaces nothing: it decides which terms a capture's title, outlet and
// language are found by, and which terms a query word looks up, so that
// every capture the corpus search's Go matcher (researchtools
// corpusTermIn) accepts is among the candidates the table returns. The Go
// matcher still decides; the table only narrows.
//
// Rules:
//   - text is lowercased (strings.ToLower, as the Go matcher does);
//   - a token is a run of letters and digits, the Go matcher's own word
//     boundary; every length is kept ("ai", "eu", "5g" are terms; Dolt
//     FULLTEXT drops them);
//   - scripts the Go matcher searches anywhere in a word (Han, kana, Thai,
//     Lao, Khmer, Myanmar, Arabic, Hebrew) are indexed by character
//     unigrams and bigrams, the segmenter Dolt lacks;
//   - a term is at most MaxTermRunes runes.
package captureterms

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxTermRunes bounds a stored term (the column is VARCHAR(64)).
const MaxTermRunes = 64

// MinPrefixRunes is the shortest query word looked up as a word prefix;
// shorter words ("ai", "eu", "us") match whole tokens only, since a
// two-letter prefix range covers a large share of the table. This is the
// one deliberate difference from the Go matcher, which lets "ai" match
// "aid": the table finds "AI" exactly, and finds "aid" only through the
// query's other words.
const MinPrefixRunes = 3

// Anywhere reports whether r belongs to a script the Go matcher matches
// anywhere in a word (scripts written without spaces between words, and
// scripts that attach prefixes to a word).
func Anywhere(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar, unicode.Arabic, unicode.Hebrew)
}

// wordRune is the Go matcher's word rune: a term matches where it starts
// after a rune that is neither a letter nor a digit.
func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// span is one run of word runes, split where the script class changes
// between anywhere-scripts and the rest.
type span struct {
	runes    []rune
	anywhere bool
}

func spans(lower string) []span {
	var out []span
	var cur []rune
	curAnywhere := false
	flush := func() {
		if len(cur) > 0 {
			out = append(out, span{runes: cur, anywhere: curAnywhere})
		}
		cur = nil
	}
	for _, r := range lower {
		if !wordRune(r) {
			flush()
			continue
		}
		anywhere := Anywhere(r)
		if len(cur) > 0 && anywhere != curAnywhere {
			flush()
		}
		curAnywhere = anywhere
		cur = append(cur, r)
	}
	flush()
	return out
}

func clip(runes []rune) string {
	if len(runes) > MaxTermRunes {
		runes = runes[:MaxTermRunes]
	}
	return string(runes)
}

// Terms returns the distinct terms text is found by.
func Terms(text string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(t string) {
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, sp := range spans(strings.ToLower(text)) {
		if !sp.anywhere {
			add(clip(sp.runes))
			continue
		}
		for i := range sp.runes {
			add(string(sp.runes[i]))
			if i+1 < len(sp.runes) {
				add(string(sp.runes[i : i+2]))
			}
		}
	}
	return out
}

// Part is one stored term a query word requires.
type Part struct {
	Term string
	// Prefix: any stored term that starts with Term satisfies the part.
	Prefix bool
}

// QueryParts returns the stored terms a query word needs, every one of
// them: a capture holding all parts is a candidate for the word, and the
// Go matcher then checks the word itself. A word of ordinary script is its
// tokens, the last one looked up as a prefix when it has at least
// MinPrefixRunes runes; a word in an anywhere-script is its character
// bigrams (one character: its unigram). A word with no word runes has no
// parts.
func QueryParts(word string) []Part {
	sps := spans(strings.ToLower(word))
	var out []Part
	seen := map[Part]bool{}
	add := func(p Part) {
		if p.Term != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for i, sp := range sps {
		if !sp.anywhere {
			last := i == len(sps)-1
			prefix := last && len(sp.runes) >= MinPrefixRunes
			// A clipped stored term is a prefix of the token; look it up
			// as one.
			if len(sp.runes) > MaxTermRunes {
				prefix = true
			}
			add(Part{Term: clip(sp.runes), Prefix: prefix})
			continue
		}
		if len(sp.runes) == 1 {
			add(Part{Term: string(sp.runes)})
			continue
		}
		for j := 0; j+1 < len(sp.runes); j++ {
			add(Part{Term: string(sp.runes[j : j+2])})
		}
	}
	return out
}

// PrefixUpper is the exclusive upper bound of the range of terms starting
// with prefix under a binary collation.
func PrefixUpper(prefix string) string {
	return prefix + string(utf8.MaxRune)
}
