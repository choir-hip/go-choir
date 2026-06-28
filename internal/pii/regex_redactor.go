package pii

import (
	"regexp"
	"strings"
)

// RegexRedactor is the deterministic, dependency-free v0 redaction strategy.
// It scans text for the high-signal structural PII classes and replaces each
// detection with its redaction token.
//
// It is safe for concurrent use: regexp.Regexp values are concurrency-safe
// and the redactor holds no mutable state.
//
// It is the fallback for SLMRedactor when the local model is unavailable, and
// the default production path until the SLM actor is wired into the runtime.
type RegexRedactor struct {
	patterns []regexPattern
}

type regexPattern struct {
	class   PIIClass
	pattern *regexp.Regexp
}

// NewRegexRedactor returns a RegexRedactor with the standard PII pattern set:
// emails, phone numbers, SSNs, credit cards, API keys/secrets, and IPv4
// addresses.
func NewRegexRedactor() *RegexRedactor {
	return &RegexRedactor{patterns: defaultRegexPatterns()}
}

// Name implements Redactor.
func (r *RegexRedactor) Name() string { return "regex" }

// RedactText implements Redactor. Every pattern is evaluated against the
// input; overlapping findings are resolved by longest-span-wins, then
// non-overlapping spans are replaced left-to-right.
func (r *RegexRedactor) RedactText(text string) (string, []Finding, error) {
	if text == "" {
		return text, nil, nil
	}
	var findings []Finding
	for _, p := range r.patterns {
		matches := p.pattern.FindAllStringIndex(text, -1)
		for _, m := range matches {
			findings = append(findings, Finding{
				Class:  p.class,
				Start:  m[0],
				End:    m[1],
				Match:  text[m[0]:m[1]],
			})
		}
	}
	findings = sortFindings(findings)
	return redactFindings(text, findings), findings, nil
}

// defaultRegexPatterns returns the compiled pattern set. Patterns are
// deliberately conservative: they favor precision over recall to avoid
// over-redacting structured identifiers (e.g. UUIDs, run IDs) that are not
// PII. Novel/free-form PII (names, addresses) is left to the SLM path.
func defaultRegexPatterns() []regexPattern {
	return []regexPattern{
		// Email — RFC 5322-ish practical subset. Excludes common
		// non-PII sentinels like "example@example.com" only when the
		// caller strips them; the redactor treats all matches as PII.
		{ClassEmail, regexp.MustCompile(
			`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)},

		// Credit card — 13-19 digit groups separated by spaces or
		// dashes, with a Luhn check applied in RedactText to suppress
		// false positives on unrelated long digit runs.
		{ClassCreditCard, regexp.MustCompile(
			`\b(?:\d[ -]*?){13,19}\b`)},

		// SSN — 9 digits in the canonical 3-2-4 form. We do not match
		// 9 consecutive digits to avoid catching credit-card fragments.
		{ClassSSN, regexp.MustCompile(
			`\b\d{3}-\d{2}-\d{4}\b`)},

		// Phone — international and NANP forms. Requires a leading +
		// or a separator-containing grouping (space, dash, or
		// parentheses) to reduce false positives on bare numeric IDs
		// and on dotted IPv4 addresses. Pure-dot-separated digit runs
		// (IP-shaped) are excluded; IP detection runs first and
		// overlapping phone spans are dropped in RedactTextWithLuhn.
		{ClassPhone, regexp.MustCompile(
			`(?:\+\d[\d .()\-]{7,}\d)|(?:\b\d{3}[ -]\d{3}[ -]\d{4}\b)|(?:\b\d{3}\.\d{3}\.\d{4}\b)|(?:\(\d{3}\)\s*\d{3}[ -]?\d{4})`)},

		// API keys / bearer secrets — common sentinel-prefixed tokens.
		// Matches sk-..., "Bearer <token>", "api_key=<val>",
		// "authorization: <val>", and AWS-style AKIA... access key ids.
		// The separator after the sentinel may be ':', '=', or
		// whitespace. The bearer form uses a mandatory space so the
		// word "Bearer" itself is not consumed as the token.
		{ClassAPIKey, regexp.MustCompile(
			`(?i)(?:sk-[A-Za-z0-9]{20,}|` +
				`bearer\s+[A-Za-z0-9_\-./+]{16,}|` +
				`(?:api[_-]?key|secret|token|authorization)\s*[:= ]\s*[A-Za-z0-9_\-./+]{16,}|` +
				`AKIA[0-9A-Z]{16})`)},

		// Credential — generic password assignment. Captures the
		// sentinel so the secret value is removed; the label remains
		// as context.
		{ClassCredential, regexp.MustCompile(
			`(?i)(?:password|passwd|pwd)\s*[:=]\s*\S+`)},

		// IPv4 — four octets. Excludes version-like strings (e.g.
		// "1.2.3.4" is still redacted; callers that log version
		// strings should redact after formatting).
		{ClassIP, regexp.MustCompile(
			`\b(?:25[0-5]|2[0-4]\d|1?\d?\d)(?:\.(?:25[0-5]|2[0-4]\d|1?\d?\d)){3}\b`)},
	}
}

// luhnValid reports whether the digit-only string s satisfies the Luhn
// checksum. Used to distinguish credit-card numbers from arbitrary digit runs.
func luhnValid(s string) bool {
	sum := 0
	double := false
	digits := 0
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c == ' ' || c == '-' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		digits++
		d := int(c - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	if digits < 13 {
		return false
	}
	return sum%10 == 0
}

// RedactTextWithLuhn is a variant of RedactText that applies a Luhn checksum
// filter to credit-card candidates, suppressing false positives on long
// non-card digit runs. This is the production entrypoint used by the pipeline;
// RedactText remains available for callers that want raw pattern matching.
func (r *RegexRedactor) RedactTextWithLuhn(text string) (string, []Finding, error) {
	if text == "" {
		return text, nil, nil
	}
	var findings []Finding
	for _, p := range r.patterns {
		matches := p.pattern.FindAllStringIndex(text, -1)
		for _, m := range matches {
			start, end := m[0], m[1]
			span := text[start:end]
			if p.class == ClassCreditCard && !luhnValid(span) {
				continue
			}
			// Trim trailing punctuation that the phone pattern can
			// over-capture (e.g. "call me at +1 555 123 4567.").
			if p.class == ClassPhone {
				trimmed := strings.TrimRight(span, ".,;:!?")
				if trimmed != span {
					span = trimmed
					end = start + len(trimmed)
				}
			}
			findings = append(findings, Finding{
				Class: p.class,
				Start: start,
				End:   end,
				Match: span,
			})
		}
	}
	findings = sortFindings(findings)
	return redactFindings(text, findings), findings, nil
}
