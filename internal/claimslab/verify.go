package claimslab

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/quotematch"
)

// Reasons a quote is not retained.
const (
	ReasonEmpty    = "empty"
	ReasonOverCap  = "over_cap"
	ReasonNotFound = "not_found"
)

// QuoteCheck is the verdict on one claim's quote against its capture.
type QuoteCheck struct {
	Retained bool              `json:"retained"`
	Reason   string            `json:"reason,omitempty"`
	Result   quotematch.Result `json:"result"`
	// HintMismatch: the quote matched a block other than the one the model
	// named (or the named block does not exist).
	HintMismatch bool `json:"hint_mismatch,omitempty"`
}

// CheckQuote verifies a quote against a capture's blocks with quotematch.
// The model's block hint is tried first and is never trusted: the quote is
// then checked against every block, and the matched block decides.
func CheckQuote(quote, hint string, blocks []quotematch.Block) QuoteCheck {
	if !strings.ContainsFunc(quote, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) {
		return QuoteCheck{Reason: ReasonEmpty, Result: quotematch.Result{Outcome: quotematch.NotFound}}
	}
	if utf8.RuneCountInString(strings.TrimSpace(quote)) > QuoteCap {
		return QuoteCheck{Reason: ReasonOverCap, Result: quotematch.Result{Outcome: quotematch.NotFound}}
	}
	if hint != "" {
		for _, b := range blocks {
			if b.ID == hint {
				if r := quotematch.Match(quote, []quotematch.Block{b}); r.Outcome == quotematch.Matched && r.Tier == quotematch.TierExact {
					return QuoteCheck{Retained: true, Result: r}
				}
				break
			}
		}
	}
	r := quotematch.Match(quote, blocks)
	if r.Outcome != quotematch.Matched {
		return QuoteCheck{Reason: ReasonNotFound, Result: r}
	}
	return QuoteCheck{Retained: true, Result: r, HintMismatch: hint != r.BlockID}
}

// Span is a matched quote's place in one block, in bytes of the block's
// original text.
type Span struct {
	Block string `json:"block"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// SpanOf returns the span of a retained check.
func SpanOf(c QuoteCheck) (Span, bool) {
	if !c.Retained {
		return Span{}, false
	}
	return Span{Block: c.Result.BlockID, Start: c.Result.Start, End: c.Result.End}, true
}
