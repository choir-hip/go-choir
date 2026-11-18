package podcastworker

import (
	"math"
	"strings"
	"unicode"
)

type quoteTok struct {
	s  string
	at float64
}

// quoteTokens lowercases words into letter and digit runs, each carrying its
// word's start time.
func quoteTokens(words []Word) []quoteTok {
	var out []quoteTok
	for _, w := range words {
		for _, f := range strings.FieldsFunc(strings.ToLower(w.W), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			out = append(out, quoteTok{f, w.Start})
		}
	}
	return out
}

// FindQuote finds where a clip's quote is spoken in words: it returns the start
// time of the words that best match, and the share of the quote's tokens
// matched in order (the longest common subsequence over the quote's length).
// Of near-equal matches it prefers the one nearest expect, so a phrase said
// twice is found where the clip was. A score of 0 means nothing matched.
func FindQuote(words []Word, quote string, expect float64) (at, score float64) {
	var qw []Word
	for _, f := range strings.Fields(quote) {
		qw = append(qw, Word{W: f})
	}
	q := quoteTokens(qw)
	if len(q) > 40 {
		q = q[:40]
	}
	w := quoteTokens(words)
	if len(q) == 0 || len(w) == 0 {
		return 0, 0
	}
	type cand struct{ at, score float64 }
	var cands []cand
	best := 0.0
	for p := range w {
		anchor := -1
		for i := 0; i < len(q) && i < 3; i++ {
			if w[p].s == q[i].s {
				anchor = i
				break
			}
		}
		if anchor < 0 || p-anchor < 0 {
			continue
		}
		from := p - anchor
		to := min(len(w), from+len(q)+3)
		sc := float64(lcs(q, w[from:to])) / float64(len(q))
		cands = append(cands, cand{w[from].at, sc})
		best = math.Max(best, sc)
	}
	bestAt, bestDist := 0.0, math.Inf(1)
	for _, c := range cands {
		if c.score >= best-0.03 && math.Abs(c.at-expect) < bestDist {
			bestAt, bestDist = c.at, math.Abs(c.at-expect)
		}
	}
	return bestAt, best
}

func lcs(a, b []quoteTok) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			switch {
			case a[i-1].s == b[j-1].s:
				cur[j] = prev[j-1] + 1
			case prev[j] >= cur[j-1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
