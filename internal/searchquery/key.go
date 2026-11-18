// Package searchquery decides when two search queries ask the same thing.
// It imports only the standard library, so the desk's search store
// (internal/yaegikernel) and the gateway's result cache
// (internal/gateway/searchplane) can both use it without an import cycle.
package searchquery

import (
	"sort"
	"strings"
	"unicode"
)

// stopWords are dropped from a key. "or" and "not" are not here: they change
// what a provider returns.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "in": true, "on": true, "at": true,
	"to": true, "for": true, "and": true, "with": true, "by": true, "from": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "about": true,
	"as": true, "it": true, "its": true, "this": true, "that": true,
}

// Key is the normalised term set of a query: lower case; quotes, punctuation
// and word order ignored; a bare year (1900-2099) and stop words dropped;
// operators kept as terms (site:host, any other word:value, a -exclusion, an
// OR). Two queries with equal keys ask the same question. A query made only
// of dropped words keeps them, so it does not collide with every other one.
func Key(q string) string { return strings.Join(Terms(q), " ") }

// Terms is the sorted, de-duplicated term set Key joins.
func Terms(q string) []string {
	var kept, all []string
	for _, tok := range strings.Fields(strings.ToLower(q)) {
		if op, ok := operatorTerm(tok); ok {
			kept = append(kept, op)
			all = append(all, op)
			continue
		}
		neg := strings.HasPrefix(strings.TrimLeft(tok, `"'(`), "-") && len(strings.TrimLeft(tok, `"'(`)) > 1
		for _, w := range words(tok) {
			if neg {
				w = "-" + w
				neg = false
			}
			all = append(all, w)
			if stopWords[w] || isYear(w) {
				continue
			}
			kept = append(kept, w)
		}
	}
	if len(kept) == 0 {
		kept = all
	}
	return uniqueSorted(kept)
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

func isYear(w string) bool {
	if len(w) != 4 || (w[:2] != "19" && w[:2] != "20") {
		return false
	}
	for _, r := range w[2:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
