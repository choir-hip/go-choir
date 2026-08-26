package autoputer

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestMailInboundHandler(t *testing.T) {
	mailRoot := t.TempDir()
	maildir, err := NewMaildir(mailRoot)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewMailInboundHandler(maildir)

	body := mailInboundRequest{
		MessageID: "<api-msg-1>",
		Recipient: "user@choir.news",
		Sender:    "sender@example.com",
		Subject:   "API Inbound Test",
		RawEML:    "Message-ID: <api-msg-1>\nFrom: sender@example.com\nTo: user@choir.news\nSubject: API Inbound Test\n\nBody content.",
	}
	jsonBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/mail/inbound", bytes.NewReader(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("handler status = %d, want 200: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "delivered" || resp["message_id"] != "<api-msg-1>" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	// Verify file on disk
	fileName, ok := resp["filename"].(string)
	if !ok || fileName == "" {
		t.Fatalf("missing filename in response")
	}
	if _, err := os.Stat(filepath.Join(mailRoot, "new", fileName)); err != nil {
		t.Fatalf("message file missing: %v", err)
	}
}
