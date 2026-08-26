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
