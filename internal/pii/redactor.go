// Package pii implements a PII retraction pipeline that redacts personally
// identifiable information from runtime trace events before persistence.
//
// Design
//
// The pipeline runs at ingestion: every trace event that may carry user
// content passes through a Redactor before it is written to the durable
// event store. Raw PII never reaches persistence.
//
// Two redactor implementations exist:
//
//   - RegexRedactor: deterministic, dependency-free pattern matching. This
//     is the v0 production path. It covers the high-signal structural PII
//     classes (emails, phone numbers, SSNs, credit cards, API keys, IPv4
//     addresses) and is the fallback when the SLM actor is unavailable.
//
//   - SLMRedactor: a local small-language-model actor (7B or smaller, served
//     by Ollama) that redacts novel PII patterns regex misses while
//     preserving semantic structure. The interface is fully specified here;
//     the implementation calls Ollama's /api/chat endpoint. When Ollama is
//     unreachable the SLM redactor degrades to the regex fallback so the
//     ingestion invariant (never store raw PII) holds even during model
//     outages.
//
// The SLM is preferred over regex in the long run because regex misses
// multilingual free-form PII (names, addresses, prose-embedded identifiers),
// over-redacts destroying learning context, and cannot preserve semantic
// structure. Regex remains the safety net.
//
// Redaction tokens preserve the PII class so downstream consumers (supervision
// hierarchy, self-learning layer) retain the semantic fact that PII was
// present without retaining the PII itself: [REDACTED:email], [REDACTED:phone],
// [REDACTED:ssn], [REDACTED:credit_card], [REDACTED:api_key], [REDACTED:ip],
// [REDACTED:name], [REDACTED:address], [REDACTED:credential].
package pii

// PIIClass labels the kind of personally identifiable information that was
// detected and retracted. It is carried in the redaction token so downstream
// consumers retain the semantic fact without the raw value.
type PIIClass string

const (
	ClassEmail       PIIClass = "email"
	ClassPhone       PIIClass = "phone"
	ClassSSN         PIIClass = "ssn"
	ClassCreditCard  PIIClass = "credit_card"
	ClassAPIKey      PIIClass = "api_key"
	ClassIP          PIIClass = "ip"
	ClassName        PIIClass = "name"
	ClassAddress     PIIClass = "address"
	ClassCredential  PIIClass = "credential"
	ClassUnknown     PIIClass = "unknown"
)

// RedactionToken returns the placeholder token used to replace retracted PII
// of the given class, e.g. ClassEmail -> "[REDACTED:email]".
func RedactionToken(c PIIClass) string {
	return "[REDACTED:" + string(c) + "]"
}

// Finding records a single PII detection within a text span.
type Finding struct {
	// Class is the detected PII class.
	Class PIIClass `json:"class"`

	// Start is the byte offset where the PII begins in the source text.
	Start int `json:"start"`

	// End is the byte offset (exclusive) where the PII ends.
	End int `json:"end"`

	// Match is the raw PII substring. This is only populated in-process for
	// redaction; it MUST NOT be persisted or logged. Callers that persist
	// findings should drop this field.
	Match string `json:"-"`
}

// Redactor is the contract every redaction strategy implements.
//
// Implementations must be safe for concurrent use: the ingestion pipeline
// may dispatch many events in parallel.
type Redactor interface {
	// RedactText scans text for PII and returns a copy with every detection
	// replaced by its redaction token, plus the list of findings (without
	// persisting the raw match). Implementations must not mutate the input.
	RedactText(text string) (redacted string, findings []Finding, err error)

	// Name identifies the redaction strategy for observability and fallback
	// routing (e.g. "regex", "slm-ollama").
	Name() string
}

// redactFindings applies a sorted, non-overlapping set of findings to text by
// replacing each span with its redaction token. Findings must be sorted by
// Start ascending and must not overlap; helpers should enforce this before
// calling.
func redactFindings(text string, findings []Finding) string {
	if len(findings) == 0 {
		return text
	}
	var b []byte
	cursor := 0
	for _, f := range findings {
		if f.Start < cursor {
			// Overlap: skip this finding to avoid corrupting output. A
			// well-formed detector should not produce overlaps; this is a
			// defensive guard.
			continue
		}
		if f.Start > len(text) {
			break
		}
		end := f.End
		if end > len(text) {
			end = len(text)
		}
		b = append(b, text[cursor:f.Start]...)
		b = append(b, RedactionToken(f.Class)...)
		cursor = end
	}
	if cursor < len(text) {
		b = append(b, text[cursor:]...)
	}
	return string(b)
}

// sortFindings orders findings by Start ascending and drops spans that overlap
// a previously kept span. The returned slice is safe to feed to redactFindings.
func sortFindings(findings []Finding) []Finding {
	if len(findings) == 0 {
		return findings
	}
	// Insertion sort: finding counts are small per event and we avoid an
	// alloc-heavy generic sort. Order by Start, then by End descending so the
	// longest span at a given offset wins.
	for i := 1; i < len(findings); i++ {
		for j := i; j > 0; j-- {
			a, b := findings[j-1], findings[j]
			if a.Start < b.Start || (a.Start == b.Start && a.End >= b.End) {
				break
			}
			findings[j-1], findings[j] = findings[j], findings[j-1]
		}
	}
	out := findings[:0:0]
	var lastEnd int = -1
	for _, f := range findings {
		if f.Start < lastEnd {
			continue // overlaps prior kept span
		}
		if f.End <= f.Start {
			continue // empty/negative span
		}
		out = append(out, f)
		lastEnd = f.End
	}
	return out
}
