package pii

import (
	"encoding/json"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Pipeline redacts PII from runtime trace events before persistence.
//
// Usage at ingestion:
//
//	pipe := pii.NewPipeline(pii.NewRegexRedactor())
//	clean, report, err := pipe.RedactEvent(rawEvent)
//	if err != nil { return err }
//	store.AppendEvent(ctx, clean) // raw PII never reaches the durable log
//
// The pipeline walks the EventRecord's Payload (a JSON blob) and redacts every
// string-valued field it contains, recursively. Non-string fields (numbers,
// bools, ids, timestamps) are passed through unchanged. The EventID, RunID,
// AgentID, ChannelID, OwnerID, TrajectoryID, Kind, Phase, and Timestamp
// envelope fields are structural identifiers, not user content, and are NOT
// redacted — only the Payload is scanned, because that is where LLM I/O,
// tool arguments, and message content live.
//
// The pipeline is safe for concurrent use.
type Pipeline struct {
	redactor Redactor
}

// NewPipeline returns a Pipeline backed by the given redactor.
func NewPipeline(r Redactor) *Pipeline {
	return &Pipeline{redactor: r}
}

// Redactor returns the underlying redactor.
func (p *Pipeline) Redactor() Redactor { return p.redactor }

// Report summarizes a single event redaction pass. It contains no raw PII:
// Finding.Match is stripped before findings are exposed here.
type Report struct {
	// Redactor is the strategy that performed the redaction.
	Redactor string `json:"redactor"`

	// EventID is the id of the event that was redacted.
	EventID string `json:"event_id"`

	// FindingsCount is the total number of PII detections across all
	// payload fields.
	FindingsCount int `json:"findings_count"`

	// Classes is a per-class count of detections.
	Classes map[PIIClass]int `json:"classes,omitempty"`

	// FieldsScanned is the number of string fields in the payload that
	// were scanned.
	FieldsScanned int `json:"fields_scanned"`

	// FieldsRedacted is the number of string fields that had at least one
	// detection.
	FieldsRedacted int `json:"fields_redacted"`

	// Changed reports whether the redacted payload differs from the
	// original.
	Changed bool `json:"changed"`
}

// RedactEvent redacts PII from an EventRecord's Payload and returns a copy
// with the cleaned payload. The input is not mutated. The returned report
// contains no raw PII (Finding.Match is dropped).
func (p *Pipeline) RedactEvent(ev types.EventRecord) (types.EventRecord, Report, error) {
	report := Report{
		Redactor: p.redactor.Name(),
		EventID:  ev.EventID,
		Classes:  map[PIIClass]int{},
	}

	if len(ev.Payload) == 0 {
		return ev, report, nil
	}

	var root any
	if err := json.Unmarshal(ev.Payload, &root); err != nil {
		// Payload is not valid JSON. Redact the raw bytes as a string
		// so we never persist raw PII even in malformed payloads.
		raw := string(ev.Payload)
		redacted, findings, rerr := p.redactText(raw)
		if rerr != nil {
			return ev, report, fmt.Errorf("pii pipeline: redact raw payload: %w", rerr)
		}
		out := ev
		out.Payload = json.RawMessage(redacted)
		p.accumulate(&report, findings, 1, 0, redacted != raw)
		return out, report, nil
	}

	scanned, redactedCount, changed := p.walkRoot(&root, &report)
	out := ev
	if changed {
		cleaned, err := json.Marshal(root)
		if err != nil {
			return ev, report, fmt.Errorf("pii pipeline: re-marshal payload: %w", err)
		}
		out.Payload = cleaned
	}
	p.accumulateCounts(&report, scanned, redactedCount, changed)
	return out, report, nil
}

// walkRoot redacts the top-level payload value in place. A bare string
// payload is replaced via the pointer; maps/slices are mutated in place.
func (p *Pipeline) walkRoot(root *any, report *Report) (int, int, bool) {
	switch v := (*root).(type) {
	case string:
		red, findings, err := p.redactText(v)
		if err != nil || len(findings) == 0 {
			return 1, 0, false
		}
		*root = red
		for _, f := range findings {
			report.Classes[f.Class]++
			report.FindingsCount++
		}
		return 1, 1, true
	case map[string]any:
		return p.walkMap(v, report)
	case []any:
		return p.walkSlice(v, report)
	default:
		return 0, 0, false
	}
}

// walkValue redacts nested (non-top-level) values. Strings inside maps and
// slices are handled directly by walkMap/walkSlice; this function recurses
// into nested containers. Returns (fieldsScanned, fieldsRedacted, changed).
func (p *Pipeline) walkValue(v any, report *Report) (int, int, bool) {
	switch val := v.(type) {
	case map[string]any:
		return p.walkMap(val, report)
	case []any:
		return p.walkSlice(val, report)
	default:
		// Non-container scalar: counted by the caller, not here.
		return 0, 0, false
	}
}

// walkMap redacts string values in a map and recurses into container values.
func (p *Pipeline) walkMap(m map[string]any, report *Report) (int, int, bool) {
	scanned, redacted, changed := 0, 0, false
	for k, child := range m {
		if s, ok := child.(string); ok {
			red, findings, err := p.redactText(s)
			scanned++
			if err == nil && len(findings) > 0 {
				m[k] = red
				redacted++
				changed = true
				for _, f := range findings {
					report.Classes[f.Class]++
					report.FindingsCount++
				}
			}
			continue
		}
		cs, cr, cc := p.walkValue(child, report)
		scanned += cs
		redacted += cr
		if cc {
			changed = true
		}
	}
	return scanned, redacted, changed
}

// walkSlice redacts string elements in a slice and recurses into container
// elements.
func (p *Pipeline) walkSlice(s []any, report *Report) (int, int, bool) {
	scanned, redacted, changed := 0, 0, false
	for i, child := range s {
		if str, ok := child.(string); ok {
			red, findings, err := p.redactText(str)
			scanned++
			if err == nil && len(findings) > 0 {
				s[i] = red
				redacted++
				changed = true
				for _, f := range findings {
					report.Classes[f.Class]++
					report.FindingsCount++
				}
			}
			continue
		}
		cs, cr, cc := p.walkValue(child, report)
		scanned += cs
		redacted += cr
		if cc {
			changed = true
		}
	}
	return scanned, redacted, changed
}

// redactText dispatches to the redactor, preferring the Luhn-filtering regex
// path when the redactor is a *RegexRedactor so credit-card false positives
// are suppressed.
func (p *Pipeline) redactText(s string) (string, []Finding, error) {
	if rr, ok := p.redactor.(*RegexRedactor); ok {
		return rr.RedactTextWithLuhn(s)
	}
	return p.redactor.RedactText(s)
}

// accumulate is used by the raw-payload fallback path.
func (p *Pipeline) accumulate(r *Report, findings []Finding, scanned, redacted int, changed bool) {
	for _, f := range findings {
		r.Classes[f.Class]++
		r.FindingsCount++
	}
	r.FieldsScanned = scanned
	if changed {
		r.FieldsRedacted = redacted
	}
	r.Changed = changed
}

// accumulateCounts finalizes the structured-payload path counts.
func (p *Pipeline) accumulateCounts(r *Report, scanned, redacted int, changed bool) {
	r.FieldsScanned = scanned
	r.FieldsRedacted = redacted
	r.Changed = changed
}
