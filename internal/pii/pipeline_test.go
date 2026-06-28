package pii

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Synthetic PII only. No real personal data.

func TestPipeline_RedactEvent_NestedPayload(t *testing.T) {
	pipe := NewPipeline(NewRegexRedactor())
	payload := map[string]any{
		"summary": "user alice@example.com called about order 42",
		"tool":    "email.send",
		"args": map[string]any{
			"to":      "bob@example.com",
			"subject": "hello",
			"body":    "reach me at +1 555 123 4567 or 192.168.1.1",
			"tags":    []any{"billing", "alice@example.com"},
		},
		"count": 3,
	}
	raw, _ := json.Marshal(payload)
	ev := types.EventRecord{EventID: "evt-1", Kind: types.EventToolInvoked, Payload: raw}

	clean, report, err := pipe.RedactEvent(ev)
	if err != nil {
		t.Fatalf("redact event: %v", err)
	}
	if !report.Changed {
		t.Fatalf("expected changed=true")
	}
	if report.FindingsCount == 0 {
		t.Fatalf("expected findings, got %+v", report)
	}
	cleanStr := string(clean.Payload)
	for _, leak := range []string{"alice@example.com", "bob@example.com", "+1 555 123 4567", "192.168.1.1"} {
		if strings.Contains(cleanStr, leak) {
			t.Fatalf("raw PII leaked into payload: %q in %s", leak, cleanStr)
		}
	}
	if !strings.Contains(cleanStr, RedactionToken(ClassEmail)) {
		t.Fatalf("expected email token in payload: %s", cleanStr)
	}
	// Non-string scalar fields must survive intact.
	var parsed map[string]any
	if err := json.Unmarshal(clean.Payload, &parsed); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if parsed["count"].(float64) != 3 {
		t.Fatalf("scalar count corrupted: %v", parsed["count"])
	}
	if parsed["tool"] != "email.send" {
		t.Fatalf("non-PII string corrupted: %v", parsed["tool"])
	}
}

func TestPipeline_RedactEvent_NoPII(t *testing.T) {
	pipe := NewPipeline(NewRegexRedactor())
	payload := map[string]any{"summary": "run abc completed", "n": 1}
	raw, _ := json.Marshal(payload)
	ev := types.EventRecord{EventID: "evt-2", Payload: raw}
	clean, report, err := pipe.RedactEvent(ev)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if report.Changed {
		t.Fatalf("non-PII event reported changed")
	}
	if report.FindingsCount != 0 {
		t.Fatalf("expected 0 findings, got %+v", report)
	}
	if string(clean.Payload) != string(raw) {
		t.Fatalf("non-PII payload mutated")
	}
}

func TestPipeline_RedactEvent_EmptyPayload(t *testing.T) {
	pipe := NewPipeline(NewRegexRedactor())
	ev := types.EventRecord{EventID: "evt-3"}
	clean, report, err := pipe.RedactEvent(ev)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if report.Changed || report.FindingsCount != 0 {
		t.Fatalf("empty payload should be noop: %+v", report)
	}
	if len(clean.Payload) != 0 {
		t.Fatalf("empty payload mutated")
	}
}

func TestPipeline_RedactEvent_MalformedJSON(t *testing.T) {
	pipe := NewPipeline(NewRegexRedactor())
	// Not valid JSON; contains an email so the raw-bytes fallback must fire.
	ev := types.EventRecord{EventID: "evt-4", Payload: json.RawMessage(`not json but alice@example.com here`)}
	clean, report, err := pipe.RedactEvent(ev)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if !report.Changed {
		t.Fatalf("expected changed for malformed payload with PII")
	}
	if strings.Contains(string(clean.Payload), "alice@example.com") {
		t.Fatalf("raw PII leaked in malformed payload: %s", clean.Payload)
	}
}

func TestPipeline_RedactEvent_BareStringPayload(t *testing.T) {
	pipe := NewPipeline(NewRegexRedactor())
	raw, _ := json.Marshal("contact carol@example.com")
	ev := types.EventRecord{EventID: "evt-5", Payload: raw}
	clean, report, err := pipe.RedactEvent(ev)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if !report.Changed {
		t.Fatalf("expected changed for bare-string payload with PII")
	}
	if strings.Contains(string(clean.Payload), "carol@example.com") {
		t.Fatalf("raw PII leaked: %s", clean.Payload)
	}
}

func TestPipeline_RedactEvent_DoesNotMutateInput(t *testing.T) {
	pipe := NewPipeline(NewRegexRedactor())
	raw, _ := json.Marshal(map[string]any{"to": "dave@example.com"})
	original := make([]byte, len(raw))
	copy(original, raw)
	ev := types.EventRecord{EventID: "evt-6", Payload: raw}
	if _, _, err := pipe.RedactEvent(ev); err != nil {
		t.Fatalf("redact: %v", err)
	}
	if string(ev.Payload) != string(original) {
		t.Fatalf("input payload was mutated: %s != %s", ev.Payload, original)
	}
}
