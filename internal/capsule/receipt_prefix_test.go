package capsule

import (
	"strings"
	"testing"
)

func TestOpenExecutionReceiptPrefixValidation(t *testing.T) {
	e := new(Executor)

	// Test 1: rlm:* token is explicitly rejected as internal intent token
	_, err := e.OpenExecutionReceipt("rlm:complete:1")
	if err == nil {
		t.Fatal("expected error for rlm:complete:1, got nil")
	}
	if !strings.Contains(err.Error(), "internal intent token, not an execution receipt") {
		t.Fatalf("expected intent token rejection message, got: %v", err)
	}

	// Test 2: unsupported prefix rejected
	_, err = e.OpenExecutionReceipt("invalid-ref:123")
	if err == nil {
		t.Fatal("expected error for invalid-ref:123, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported prefix") {
		t.Fatalf("expected unsupported prefix rejection message, got: %v", err)
	}

	// Test 3: empty ref rejected
	_, err = e.OpenExecutionReceipt("   ")
	if err == nil {
		t.Fatal("expected error for empty ref, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported prefix") {
		t.Fatalf("expected unsupported prefix rejection message for empty ref, got: %v", err)
	}

	// Test 4: ResolveExecutionReceipts fails fast on rlm:* in a batch
	refs := []string{"rlm:complete:1", "capsule-go-eval:sha256:7fe0432dcb0600ba03ba4bbbe15bc5026902fea196c1675e3a442b1d88f5a166"}
	_, err = e.ResolveExecutionReceipts(refs)
	if err == nil {
		t.Fatal("expected ResolveExecutionReceipts to fail on batch with rlm:complete:1, got nil")
	}
	if !strings.Contains(err.Error(), "internal intent token") {
		t.Fatalf("expected batch to fail with intent token error, got: %v", err)
	}

	// Test 5: capsule-fate:* rejected in execution receipt opener
	_, err = e.OpenExecutionReceipt("capsule-fate:sha256:7fe0432dcb0600ba03ba4bbbe15bc5026902fea196c1675e3a442b1d88f5a166")
	if err == nil {
		t.Fatal("expected error for capsule-fate in execution receipt opener, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported prefix") {
		t.Fatalf("expected unsupported prefix error for capsule-fate, got: %v", err)
	}
}
