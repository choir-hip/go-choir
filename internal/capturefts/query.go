package capturefts

import (
	"strings"
	"unicode"
)

const (
	// MaxQueryTerms bounds the distinct terms one search looks up. A Han,
	// Arabic or similar query is unigrams and bigrams, so this is larger
	// than the number of words a model writes. The bound is applied after
	// the terms are ranked by rarity: the words dropped are the most common
	// ones, and the result names them (Result.TermCapped).
	MaxQueryTerms = 32
	// maxParsedTerms bounds what a query is parsed into before ranking.
	maxParsedTerms = 64
)

// Plan is what a query asks for, read from the text the model wrote:
//
//   - words, and the words of anything else that is not syntax, are
//     searched for, any of them (OR), stop words dropped;
//   - "a quoted phrase" is searched as a phrase (its words together, in
//     order);
//   - -word and -"a phrase" are exclusions: a passage that holds one is not
//     returned, and the word is never searched for;
//   - everything else is punctuation: C++ is the word c, AT&T is at and t,
//     NEAR/3 is near and 3, title: is the word title. No FTS5 operator or
//     column filter is reachable from the text.
type Plan struct {
	// Positive are the distinct terms to look up, in order of appearance.
	Positive []string
	// Phrases are the quoted phrases of two or more terms.
	Phrases [][]string
	// Excluded are the terms and phrases (words joined by a space) that a
	// returned passage must not hold.
	Excluded []string
}

// Empty reports whether the plan has nothing to search for.
func (p Plan) Empty() bool { return len(p.Positive) == 0 && len(p.Phrases) == 0 }

func isQuote(r rune) bool { return r == '"' || r == '“' || r == '”' }

func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// PlanQuery reads a query.
func PlanQuery(text string) Plan {
	var p Plan
	runes := []rune(text)
	n := len(runes)
	var all, kept []string
	seen := map[string]bool{}
	excl := map[string]bool{}
	addPositive := func(t string) {
		if seen[t] || len(all) >= maxParsedTerms {
			return
		}
		seen[t] = true
		all = append(all, t)
		if !queryStopwords[t] {
			kept = append(kept, t)
		}
	}
	for i := 0; i < n; {
		for i < n && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= n {
			break
		}
		neg := false
		if runes[i] == '-' && i+1 < n && (wordRune(runes[i+1]) || isQuote(runes[i+1])) {
			neg = true
			i++
		}
		var item string
		phrase := false
		if isQuote(runes[i]) {
			phrase = true
			i++
			start := i
			for i < n && !isQuote(runes[i]) {
				i++
			}
			item = string(runes[start:i])
			if i < n {
				i++ // the closing quote
			}
		} else {
			start := i
			for i < n && !unicode.IsSpace(runes[i]) {
				i++
			}
			item = string(runes[start:i])
		}
		toks := Terms(item)
		if len(toks) == 0 {
			continue
		}
		switch {
		case neg:
			key := strings.Join(toks, " ")
			if !excl[key] {
				excl[key] = true
				p.Excluded = append(p.Excluded, key)
			}
		case phrase && len(toks) >= 2:
			p.Phrases = append(p.Phrases, toks)
		default:
			for _, t := range toks {
				addPositive(t)
			}
		}
	}
	p.Positive = kept
	if len(kept) == 0 && len(p.Phrases) == 0 {
		p.Positive = all // a query of nothing but stop words still searches them
	}
	// A positive term that is also excluded is not searched for.
	var pos []string
	for _, t := range p.Positive {
		if !excl[t] {
			pos = append(pos, t)
		}
	}
	p.Positive = pos
	return p
}

// Match is the FTS5 MATCH argument for the given terms and phrases, less the
// plan's exclusions. Every item is a quoted string of letters and digits, so
// no query text reaches the engine as syntax. It is empty when there is
// nothing to search for.
func (p Plan) Match(terms []string, phrases [][]string) string {
	var items []string
	for _, ph := range phrases {
		items = append(items, `"`+strings.Join(ph, " ")+`"`)
	}
	for _, t := range terms {
		items = append(items, `"`+t+`"`)
	}
	if len(items) == 0 {
		return ""
	}
	if len(p.Excluded) == 0 {
		return strings.Join(items, " OR ")
	}
	ex := make([]string, len(p.Excluded))
	for i, e := range p.Excluded {
		ex[i] = `"` + e + `"`
	}
	return "(" + strings.Join(items, " OR ") + ") NOT (" + strings.Join(ex, " OR ") + ")"
}
