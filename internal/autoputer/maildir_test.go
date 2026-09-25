package autoputer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMaildirDeliverMessageAndDeduplicate(t *testing.T) {
	mailRoot := t.TempDir()
	maildir, err := NewMaildir(mailRoot)
	if err != nil {
		t.Fatalf("NewMaildir failed: %v", err)
	}

	ctx := context.Background()
	msgID := "<msg-100@example.com>"
	raw := []byte("Message-ID: <msg-100@example.com>\nFrom: alice@example.com\nTo: bob@example.com\nSubject: Test\n\nHello Maildir.")

	file1, err := maildir.DeliverMessage(ctx, msgID, raw)
	if err != nil {
		t.Fatalf("DeliverMessage failed: %v", err)
	}
	if file1 == "" {
		t.Fatalf("expected non-empty filename")
	}

	// Verify file is in new/
	newPath := filepath.Join(mailRoot, "new", file1)
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read new message failed: %v", err)
	}
	if string(data) != string(raw) {
		t.Fatalf("message content mismatch: got %q, want %q", string(data), string(raw))
	}

	// Duplicate delivery with same messageID should be idempotent
	file2, err := maildir.DeliverMessage(ctx, msgID, raw)
	if err != nil {
		t.Fatalf("second DeliverMessage failed: %v", err)
	}
	if file2 != file1 {
		t.Fatalf("expected duplicate to return existing filename %q, got %q", file1, file2)
	}

	// Ensure only 1 file is in new/
	entries, err := os.ReadDir(filepath.Join(mailRoot, "new"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in new/, got %d", len(entries))
	}
}
