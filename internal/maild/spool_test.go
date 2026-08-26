package maild

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSpoolQueueEnqueueFetchDeliveredPurge(t *testing.T) {
	dir := t.TempDir()
	queue, err := NewSpoolQueue(dir)
	if err != nil {
		t.Fatalf("NewSpoolQueue failed: %v", err)
	}
	defer queue.Close()

	ctx := context.Background()
	raw := []byte("From: alice@example.com\nTo: bob@example.com\nSubject: Hello\n\nBody test.")
	msgID := "<test-1@example.com>"

	spoolID, err := queue.Enqueue(ctx, "computer-test-1", "bob@example.com", "alice@example.com", "Hello", msgID, raw)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if spoolID == "" {
		t.Fatalf("expected non-empty spool ID")
	}

	// Fetch pending
	pending, err := queue.FetchPending(ctx, 10)
	if err != nil {
		t.Fatalf("FetchPending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending message, got %d", len(pending))
	}
	if pending[0].ID != spoolID || pending[0].ComputerID != "computer-test-1" {
		t.Fatalf("pending message mismatch: %+v", pending[0])
	}

	// Verify raw file exists
	if _, err := os.Stat(pending[0].RawPath); err != nil {
		t.Fatalf("raw file missing: %v", err)
	}

	// Mark delivered
	if err := queue.MarkDelivered(ctx, spoolID); err != nil {
		t.Fatalf("MarkDelivered failed: %v", err)
	}

	// Pending should now be 0
	pendingAfter, err := queue.FetchPending(ctx, 10)
	if err != nil {
		t.Fatalf("FetchPending after deliver failed: %v", err)
	}
	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending messages after delivery, got %d", len(pendingAfter))
	}

	// Purge delivered
	if err := queue.PurgeDelivered(ctx, spoolID); err != nil {
		t.Fatalf("PurgeDelivered failed: %v", err)
	}
	if _, err := os.Stat(pending[0].RawPath); !os.IsNotExist(err) {
		t.Fatalf("raw file should be removed after purge: %v", err)
	}
}

func TestSpoolQueueBackoff(t *testing.T) {
	dir := t.TempDir()
	queue, err := NewSpoolQueue(dir)
	if err != nil {
		t.Fatalf("NewSpoolQueue failed: %v", err)
	}
	defer queue.Close()

	ctx := context.Background()
	spoolID, err := queue.Enqueue(ctx, "computer-test-1", "bob@example.com", "alice@example.com", "Test", "<msg-2>", []byte("raw"))
	if err != nil {
		t.Fatal(err)
	}

	// Record attempt failure with 1 hour delay
	if err := queue.RecordAttemptFailure(ctx, spoolID, 1*time.Hour); err != nil {
		t.Fatal(err)
	}

	// Should not be in pending list immediately
	pending, err := queue.FetchPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending messages due to backoff, got %d", len(pending))
	}
}
