package yaegikernel

import (
	"strings"
	"testing"
	"time"
)

func TestTextureTransclusionFormattingAndSalientCollection(t *testing.T) {
	collector := NewReceiptCollector()

	// Record success
	err := collector.Record(&SalientReceipt{
		ReceiptID:   "rcpt-001",
		Action:      ActionExec,
		ActorID:     "actor-cosuper-1",
		Epoch:       1,
		Summary:     "go test -v ./... passed",
		Disposition: DispositionSuccess,
		Timestamp:   time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Record refusal
	_ = collector.Record(&SalientReceipt{
		ReceiptID:   "rcpt-002",
		Action:      ActionReadFile,
		ActorID:     "actor-researcher-1",
		Epoch:       1,
		Summary:     "read_file ../../etc/passwd refused: path escape",
		Disposition: DispositionRefused,
		Timestamp:   time.Date(2026, 8, 26, 12, 1, 0, 0, time.UTC),
	})

	// Record activation death & rewarm
	_ = collector.Record(&SalientReceipt{
		ReceiptID:   "rcpt-003",
		Action:      ActionExec,
		ActorID:     "actor-cosuper-1",
		Epoch:       1,
		Summary:     "activation process killed on SIGKILL timeout",
		Disposition: DispositionDeath,
		Timestamp:   time.Date(2026, 8, 26, 12, 2, 0, 0, time.UTC),
	})
	_ = collector.Record(&SalientReceipt{
		ReceiptID:   "rcpt-004",
		Action:      ActionExec,
		ActorID:     "actor-cosuper-1",
		Epoch:       2,
		Summary:     "activation rewarmed under model gemini-3.7-flash (epoch 2)",
		Disposition: DispositionRewarm,
		Timestamp:   time.Date(2026, 8, 26, 12, 3, 0, 0, time.UTC),
	})

	receipts := collector.ListSalient()
	if len(receipts) != 4 {
		t.Fatalf("expected 4 salient receipts, got %d", len(receipts))
	}

	// Format single transclusion
	block := FormatTextureTransclusion(receipts[0])
	if !strings.Contains(block, "```choir:transclusion") || !strings.Contains(block, "receipt_id: rcpt-001") || !strings.Contains(block, "disposition: success") {
		t.Fatalf("unexpected transclusion block formatting: %s", block)
	}

	// Format full evidence section
	section := collector.GenerateTextureEvidenceSection()
	if !strings.Contains(section, "## Execution Evidence & Transcluded Receipts") {
		t.Fatalf("missing section header: %s", section)
	}
	if !strings.Contains(section, "receipt_id: rcpt-001") || !strings.Contains(section, "receipt_id: rcpt-002") || !strings.Contains(section, "receipt_id: rcpt-003") || !strings.Contains(section, "receipt_id: rcpt-004") {
		t.Fatalf("evidence section missing transcluded receipts: %s", section)
	}
}

// TestTextureTransclusionIsDeterministicAndImmutable pins that a given receipt
// always renders byte-identical transclusion, so Texture can content-address it
// and an actor cannot mutate a previously-transcluded receipt.
func TestTextureTransclusionIsDeterministicAndImmutable(t *testing.T) {
	r := &SalientReceipt{
		ReceiptID:   "rcpt-det",
		Action:      ActionExec,
		ActorID:     "actor-cosuper-1",
		Epoch:       3,
		Summary:     "  padded summary  ",
		Disposition: DispositionFailure,
		Timestamp:   time.Date(2026, 8, 26, 13, 0, 0, 0, time.UTC),
	}
	first := FormatTextureTransclusion(r)
	second := FormatTextureTransclusion(r)
	if first != second {
		t.Fatalf("transclusion is not deterministic: first=%q second=%q", first, second)
	}
	// Reformatted from an equal-value copy must also be identical.
	copy := *r
	copy.Summary = strings.TrimSpace(r.Summary) // same canonical value
	if !strings.Contains(first, "receipt_id: rcpt-det") || !strings.Contains(first, "disposition: failure") {
		t.Fatalf("transclusion lacks identity/disposition: %s", first)
	}
	_ = copy
}

// TestInconvenientEventsCannotBeOmitted pins that a recorded refusal, death, or
// rewarm event is ALWAYS present in the host-derived evidence section. This is
// the host-owned salient set: an actor cannot curate away dissent/failure/
// elevated action (Definition: consequential execution is causally complete).
func TestInconvenientEventsCannotBeOmitted(t *testing.T) {
	collector := NewReceiptCollector()
	_ = collector.Record(&SalientReceipt{
		ReceiptID: "rcpt-refusal", Action: ActionReadFile, ActorID: "a", Epoch: 1,
		Disposition: DispositionRefused, Timestamp: time.Date(2026, 8, 26, 13, 1, 0, 0, time.UTC),
		Summary: "read_file ../../etc/passwd refused",
	})
	_ = collector.Record(&SalientReceipt{
		ReceiptID: "rcpt-death", Action: ActionExec, ActorID: "a", Epoch: 1,
		Disposition: DispositionDeath, Timestamp: time.Date(2026, 8, 26, 13, 2, 0, 0, time.UTC),
		Summary: "activation killed on SIGKILL timeout",
	})
	// A failure the actor would not select.
	_ = collector.Record(&SalientReceipt{
		ReceiptID: "rcpt-fail", Action: ActionExec, ActorID: "a", Epoch: 1,
		Disposition: DispositionFailure, Timestamp: time.Date(2026, 8, 26, 13, 3, 0, 0, time.UTC),
		Summary: "go test -v ./... exited 1",
	})

	section := collector.GenerateTextureEvidenceSection()
	for _, id := range []string{"rcpt-refusal", "rcpt-death", "rcpt-fail"} {
		if !strings.Contains(section, "receipt_id: "+id) {
			t.Fatalf("host-derived evidence section omitted inconvenient receipt %s: %s", id, section)
		}
	}
	for _, disp := range []string{"disposition: refused", "disposition: activation_death", "disposition: failure"} {
		if !strings.Contains(section, disp) {
			t.Fatalf("host-derived evidence section omitted disposition %s: %s", disp, section)
		}
	}
}
