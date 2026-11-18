// Package searchquery decides when two search queries inside one run ask the
// same thing. It imports only the standard library. Only the desk's search
// store (internal/yaegikernel) uses Key: the gateway's result cache keeps its
// own key, because a merge there is silent to the caller.
//
// Key folds only what a search engine answers the same way: word order, the
// case of ordinary words, extra whitespace and punctuation between words, and
// small English words. It keeps as terms what changes the answer: a year or a
// date, a quoted phrase (one marked term), a symbol that is part of a word
// (C++, C#), an acronym written in capitals that is a small word in lower case
// (IT, US, WHO, AT), and operators.
package searchquery

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// stopWords are dropped from a key when written in lower or mixed case. "or"
// and "not" are not here: they change what a provider returns.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "in": true, "on": true, "at": true,
	"to": true, "for": true, "and": true, "with": true, "by": true, "from": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "about": true,
	"as": true, "it": true, "its": true, "this": true, "that": true, "us": true, "who": true,
}

var (
	phraseRE = regexp.MustCompile(`["“”]([^"“”]*)["“”]`)
	dateRE   = regexp.MustCompile(`^\d{1,4}[-/.]\d{1,2}[-/.]\d{1,4}$`)
	symbolRE = regexp.MustCompile(`^[\p{L}\p{N}]+[+#]+$`)
)

// Key is the normalised term set of a query, joined by spaces. Two queries
// with equal keys ask the same question. A query made only of dropped words
// keeps them, so it does not collide with every other one.
func Key(q string) string { return strings.Join(Terms(q), " ") }

// Terms is the sorted, de-duplicated term set Key joins.
func Terms(q string) []string {
	var kept, all []string
	// Quoted phrases are one marked term each, whatever is inside them.
	rest := phraseRE.ReplaceAllStringFunc(q, func(m string) string {
		inner := phraseRE.FindStringSubmatch(m)[1]
		if ws := words(strings.ToLower(inner)); len(ws) > 0 {
			t := `"` + strings.Join(ws, " ") + `"`
			kept, all = append(kept, t), append(all, t)
		}
		return " "
	})
	for _, tok := range strings.Fields(rest) {
		clean := strings.Trim(tok, `"'()[],;!?`)
		if op, ok := operatorTerm(strings.ToLower(tok)); ok {
			kept, all = append(kept, op), append(all, op)
			continue
		}
		if dateRE.MatchString(strings.Trim(clean, ".")) {
			t := strings.Trim(clean, ".")
			kept, all = append(kept, t), append(all, t)
			continue
		}
		if symbolRE.MatchString(strings.TrimRight(clean, ".")) {
			t := strings.ToLower(strings.TrimRight(clean, "."))
			kept, all = append(kept, t), append(all, t)
			continue
		}
		neg := strings.HasPrefix(clean, "-") && len(clean) > 1
		for _, w := range words(tok) {
			term := strings.ToLower(w)
			if isAcronym(w) && stopWords[term] {
				term = strings.ToUpper(w) // IT is not "it"
				kept = append(kept, term)
				all = append(all, term)
				continue
			}
			if neg {
				term = "-" + term
				neg = false
			}
			all = append(all, term)
			if stopWords[term] {
				continue
			}
			kept = append(kept, term)
		}
	}
	if len(kept) == 0 {
		kept = all
	}
	return uniqueSorted(kept)
}

// isAcronym reports a word of two or more letters, all capitals.
func isAcronym(w string) bool {
	n := 0
	for _, r := range w {
		if !unicode.IsLetter(r) || !unicode.IsUpper(r) {
			return false
		}
		n++
	}
	return n >= 2
}

// HasRealWord reports whether q has a run of two or more letters or digits.
// A query without one ("x") is a slip, not a question.
func HasRealWord(q string) bool {
	for _, w := range words(q) {
		if len([]rune(w)) >= 2 {
			return true
		}
	}
	return false
}

var timeSensitive = regexp.MustCompile(`(?i)\b(today|tonight|yesterday|this (week|morning|evening|month)|latest|breaking|live|now|just|hours? ago|minutes? ago|january|february|march|april|may|june|july|august|september|october|november|december)\b`)

// IsTimeSensitive reports whether a query asks about the news of the moment.
// The desk's reuse window (10 minutes instead of 30) and the gateway cache's
// fresh TTL both ask this one function.
func IsTimeSensitive(q string) bool { return timeSensitive.MatchString(q) }

// operatorTerm recognises word:value (site:example.com, filetype:pdf) and
// returns it with quotes and trailing punctuation stripped.
func operatorTerm(tok string) (string, bool) {
	tok = strings.Trim(tok, `"'()[],;!?`)
	neg := strings.HasPrefix(tok, "-")
	tok = strings.TrimPrefix(tok, "-")
	name, val, ok := strings.Cut(tok, ":")
	if !ok || name == "" || val == "" || strings.ContainsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) }) || strings.HasPrefix(val, "//") {
		return "", false
	}
	val = strings.Trim(val, `"'.,;:`)
	if val == "" {
		return "", false
	}
	out := name + ":" + val
	if neg {
		out = "-" + out
	}
	return out, true
}

func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func uniqueSorted(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for i, w := range in {
		if i == 0 || w != in[i-1] {
			out = append(out, w)
		}
	}
	return out
}
