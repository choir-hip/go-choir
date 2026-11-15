// Package jevbudget sizes a Jev request against the vendor's token limits.
//
// The vendor limits a request to 64k tokens and the state plus any one
// question to 32k tokens (docs/reference/system1/jev-vendor.md section 4,
// docs/reference/SYSTEM1.md section 3.3). The per-question limit is the one
// that binds when a state is large and the questions are few; the request
// total binds when there are many questions. Callers used to keep a request
// under 30,000 bytes (OpenRouter's "about 32,000 bytes" long-context line);
// the owner ruled on 2026-11-12 that Jev is sized by the vendor's limits, not
// by an arbitrary byte guard of ours. Whether OpenRouter switches to the
// vendor's slower, less accurate long-context mode above 32,000 bytes is
// unpublished (SYSTEM1.md section 10, item 2); it is watched, not guarded.
package jevbudget

import (
	"encoding/json"
	"math"
	"unicode"
	"unicode/utf8"
)

const (
	// VendorRequestTokens is the vendor's context limit for a whole request:
	// the state and every question ("64k tokens").
	VendorRequestTokens = 64000
	// VendorQuestionTokens is the vendor's limit for the state plus any one
	// question ("32k tokens"); with a large state it is the one that binds.
	VendorQuestionTokens = 32000
	// VendorMaxChoices is the most options one choice question takes (1 to
	// 255; jev-vendor.md section 4). It is a limit of the question, not of the
	// state: a state can list more candidates than one question may offer.
	VendorMaxChoices = 255

	// RequestBudgetTokens and QuestionBudgetTokens are what a caller may use:
	// the vendor's limits themselves, so a split's logged limit is the
	// vendor's. The estimate below is weighted above the measured tokenizer
	// (it is never under the densest ratio we observed), and a request the
	// vendor refuses for size is halved and asked again (the callers' refusal
	// fallback, logged forced_by=vendor_refused), so no margin of ours decides
	// a call count.
	RequestBudgetTokens  = VendorRequestTokens
	QuestionBudgetTokens = VendorQuestionTokens

	// baseTokens is the intercept of the independent fit, input tokens =
	// 412 + 0.224 x state characters + 0.210 x question characters
	// (R2 0.990, 23,411 calls).
	baseTokens = 412
)

// Tokens estimates the input tokens of text from its characters, by script:
// the vendor tokenizer reads Latin prose at about 0.26 tokens a character,
// digits at 0.94, Cyrillic at 0.94, CJK and Hangul at 0.92 and emoji at 1.71,
// and rare scripts at up to one token per UTF-8 byte (SYSTEM1.md section 3.3).
// The weights are the vendor figures rounded up; JSON punctuation counts 0.7
// because a quote, colon or comma is rarely free; Latin letters count 0.30.
// An intake request (hundreds of handle keys: digits, quotes, colons) comes
// out above the 0.58 tokens a byte observed at most.
func Tokens(text []byte) int {
	var sum float64
	for len(text) > 0 {
		r, size := utf8.DecodeRune(text)
		text = text[size:]
		sum += runeWeight(r, size)
	}
	return int(math.Ceil(sum))
}

func runeWeight(r rune, size int) float64 {
	switch {
	case r < utf8.RuneSelf:
		switch {
		case r >= '0' && r <= '9':
			return 1.0
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			return 0.30
		case r == ' ' || r == '\n' || r == '\t' || r == '\r':
			return 0.25
		default:
			return 0.7
		}
	case unicode.Is(unicode.Latin, r):
		return 0.6
	case r >= 0x1F000, unicode.In(r, unicode.So, unicode.Sk):
		return 2.0
	case unicode.In(r, unicode.Cyrillic, unicode.Greek, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Arabic, unicode.Hebrew):
		return 1.0
	default:
		// A rare script, or punctuation outside ASCII: up to one token per
		// UTF-8 byte.
		return float64(size)
	}
}

// Estimate is a request's size in estimated input tokens.
type Estimate struct {
	// Request is the whole request: the intercept, the state and every
	// question. The vendor limits it to VendorRequestTokens.
	Request int
	// WorstQuestion is the state plus the largest single question. The vendor
	// limits it to VendorQuestionTokens.
	WorstQuestion int
}

// Measure estimates a request's input tokens.
func Measure(state json.RawMessage, questions map[string]json.RawMessage) Estimate {
	s := Tokens(state)
	total, worst := baseTokens+s, 0
	for _, q := range questions {
		n := Tokens(q)
		total += n
		worst = max(worst, n)
	}
	return Estimate{Request: total, WorstQuestion: baseTokens + s + worst}
}

// Fits reports whether the request is inside every vendor limit.
func Fits(state json.RawMessage, questions map[string]json.RawMessage) bool {
	return Limit(state, questions) == ""
}

// The names of the limits a split can be forced by, as logged.
const (
	LimitRequest  = "request_tokens_64k"
	LimitQuestion = "state_plus_question_tokens_32k"
	LimitChoices  = "choices_255"
)

// Limit names the limit a request crosses, or "" when it fits: a choice
// question with more than 255 options, the request total (64k tokens), then the
// state plus one question (32k).
func Limit(state json.RawMessage, questions map[string]json.RawMessage) string {
	for _, q := range questions {
		if MaxOptions(q) > VendorMaxChoices {
			return LimitChoices
		}
	}
	e := Measure(state, questions)
	switch {
	case e.Request > RequestBudgetTokens:
		return LimitRequest
	case e.WorstQuestion > QuestionBudgetTokens:
		return LimitQuestion
	}
	return ""
}

// MaxOptions is the number of options a choice question offers: the keys of
// its criteria (the vendor reads the option set from them) or, for a question
// that lists choices, those. Other questions offer none.
func MaxOptions(question json.RawMessage) int {
	var q struct {
		Criteria map[string]json.RawMessage `json:"criteria"`
		Choices  []json.RawMessage          `json:"choices"`
	}
	if json.Unmarshal(question, &q) != nil {
		return 0
	}
	return max(len(q.Criteria), len(q.Choices))
}
