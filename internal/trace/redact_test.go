package trace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pii"
)

// All PII in this file is synthetic. No real personal data is used.

// newRedactingTestStore opens a fresh in-memory SQLite store wrapped in a
// RedactingStore backed by the regex-only pipeline (SLM off, the default).
func newRedactingTestStore(t *testing.T) *RedactingStore {
	t.Helper()
	inner, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open trace store: %v", err)
	}
	t.Cleanup(func() { _ = inner.Close() })
	return NewRedactingStore(inner, NewPipelineFromConfig(RedactionConfig{}))
}

func TestRedactingStore_RedactsEmailInPayload(t *testing.T) {
	s := newRedactingTestStore(t)
	ctx := context.Background()
	ts := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	ev := sampleEvent("evt-pii-1", "run-1", "tool.invoked", ts)
	ev.Tool = "email.send"
	ev.Payload = json.RawMessage(`{"to":"alice@example.com","subject":"hello","body":"reach me at +1 555 123 4567"}`)

	if err := s.Append(ctx, &ev); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := s.Get(ctx, "evt-pii-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	stored := string(got.Payload)
	for _, leak := range []string{"alice@example.com", "+1 555 123 4567"} {
		if strings.Contains(stored, leak) {
			t.Fatalf("raw PII leaked into persisted payload: %q in %s", leak, stored)
		}
	}
	if !strings.Contains(stored, pii.RedactionToken(pii.ClassEmail)) {
		t.Fatalf("expected email redaction token in payload: %s", stored)
	}
	if !strings.Contains(stored, pii.RedactionToken(pii.ClassPhone)) {
		t.Fatalf("expected phone redaction token in payload: %s", stored)
	}

	// Envelope fields must survive intact.
	if got.ID != "evt-pii-1" || got.RunID != "run-1" || got.EventType != "tool.invoked" {
		t.Fatalf("envelope corrupted: %+v", got)
	}
	if got.Tool != "email.send" {
		t.Fatalf("tool corrupted: %q", got.Tool)
	}
	if !got.CreatedAt.Equal(ts) {
		t.Fatalf("created_at corrupted: %v", got.CreatedAt)
	}

	// Non-PII string fields must survive.
	var parsed map[string]any
	if err := json.Unmarshal(got.Payload, &parsed); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if parsed["subject"] != "hello" {
		t.Fatalf("non-PII string corrupted: %v", parsed["subject"])
	}
}

func TestRedactingStore_DoesNotMutateCallerEvent(t *testing.T) {
	s := newRedactingTestStore(t)
	ctx := context.Background()
	original := json.RawMessage(`{"to":"bob@example.com"}`)
	ev := sampleEvent("evt-pii-2", "run-1", "tool.invoked", time.Now())
	ev.Payload = original

	if err := s.Append(ctx, &ev); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// Caller's event must be unchanged.
	if string(ev.Payload) != string(original) {
		t.Fatalf("caller event payload mutated: %q != %q", ev.Payload, original)
	}
}

func TestRedactingStore_NoPIIPassesThroughUnchanged(t *testing.T) {
	s := newRedactingTestStore(t)
	ctx := context.Background()
	payload := json.RawMessage(`{"summary":"run abc completed","count":3}`)
	ev := sampleEvent("evt-no-pii", "run-1", "loop.completed", time.Now())
	ev.Payload = payload

	if err := s.Append(ctx, &ev); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := s.Get(ctx, "evt-no-pii")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got.Payload) != string(payload) {
		t.Fatalf("non-PII payload mutated: %q != %q", got.Payload, payload)
	}
}

// failingRedactor is a Redactor that always returns an error, used to verify
// graceful degradation: the event must still be stored with a warning.
type failingRedactor struct{}

func (failingRedactor) Name() string { return "failing-test" }

func (failingRedactor) RedactText(text string) (string, []pii.Finding, error) {
	return text, nil, errors.New("synthetic redactor failure")
}

func TestRedactingStore_RedactionFailureStoresOriginalWithWarning(t *testing.T) {
	inner, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open trace store: %v", err)
	}
	t.Cleanup(func() { _ = inner.Close() })

	warnLog := &redactionWarnLogger{}
	s := NewRedactingStore(inner, pii.NewPipeline(failingRedactor{}), WithRedactionLogger(warnLog.logf))

	ctx := context.Background()
	// Use a non-JSON payload: the pipeline only propagates redactor errors on
	// the raw-bytes fallback path (valid-JSON payloads swallow per-field
	// errors). A malformed payload with a failing redactor triggers the
	// error return from RedactEvent, exercising graceful degradation.
	payload := json.RawMessage(`not json but alice@example.com here`)
	ev := sampleEvent("evt-fail", "run-1", "tool.invoked", time.Now())
	ev.Payload = payload

	if err := s.Append(ctx, &ev); err != nil {
		t.Fatalf("Append must not fail on redactor error: %v", err)
	}

	// Event must still be stored (with original payload — graceful degradation).
	got, err := s.Get(ctx, "evt-fail")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "evt-fail" {
		t.Fatalf("event not stored: %+v", got)
	}

	// A warning must have been logged.
	if len(warnLog.warnings) == 0 {
		t.Fatalf("expected a redaction warning, got none")
	}
	if !strings.Contains(warnLog.warnings[0], "evt-fail") {
		t.Fatalf("warning should reference event id: %q", warnLog.warnings[0])
	}
	if !strings.Contains(warnLog.warnings[0], "synthetic redactor failure") {
		t.Fatalf("warning should contain error: %q", warnLog.warnings[0])
	}
}

func TestRedactingStore_NestedPayloadRedacted(t *testing.T) {
	s := newRedactingTestStore(t)
	ctx := context.Background()
	ev := sampleEvent("evt-nested", "run-1", "tool.invoked", time.Now())
	ev.Payload = json.RawMessage(`{
		"summary":"user alice@example.com called about order 42",
		"args":{
			"to":"bob@example.com",
			"body":"reach me at 192.168.1.1",
			"tags":["billing","alice@example.com"]
		},
		"count":3
	}`)

	if err := s.Append(ctx, &ev); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := s.Get(ctx, "evt-nested")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	stored := string(got.Payload)
	for _, leak := range []string{"alice@example.com", "bob@example.com", "192.168.1.1"} {
		if strings.Contains(stored, leak) {
			t.Fatalf("raw PII leaked in nested payload: %q in %s", leak, stored)
		}
	}
	if !strings.Contains(stored, pii.RedactionToken(pii.ClassEmail)) {
		t.Fatalf("expected email token: %s", stored)
	}
	if !strings.Contains(stored, pii.RedactionToken(pii.ClassIP)) {
		t.Fatalf("expected ip token: %s", stored)
	}
	// Non-string scalar must survive.
	var parsed map[string]any
	if err := json.Unmarshal(got.Payload, &parsed); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if parsed["count"].(float64) != 3 {
		t.Fatalf("scalar count corrupted: %v", parsed["count"])
	}
}
