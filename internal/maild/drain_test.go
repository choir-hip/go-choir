package maild

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDrainWorkerDeliversToGuest(t *testing.T) {
	dir := t.TempDir()
	queue, err := NewSpoolQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Close()

	ctx := context.Background()
	receivedCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-cap-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/api/mail/inbound" {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if body["message_id"] == "<drain-test-1>" {
				receivedCount++
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "delivered"})
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Enqueue a message
	spoolID, err := queue.Enqueue(ctx, "computer-test-1", "bob@example.com", "alice@example.com", "Subject", "<drain-test-1>", []byte("From: alice@example.com\n\nHi Bob!"))
	if err != nil {
		t.Fatal(err)
	}

	resolver := func(ctx context.Context, computerID string) (string, string, error) {
		if computerID == "computer-test-1" {
			return server.URL, "test-cap-token", nil
		}
		return "", "", http.ErrHandlerTimeout
	}

	worker := NewDrainWorker(queue, resolver)
	delivered, err := worker.DrainOnce(ctx, 10)
	if err != nil {
		t.Fatalf("DrainOnce failed: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("expected 1 delivered message, got %d", delivered)
	}
	if receivedCount != 1 {
		t.Fatalf("expected server to receive 1 message, got %d", receivedCount)
	}

	// Message should now be marked delivered
	pending, err := queue.FetchPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending messages, got %d", len(pending))
	}

	// Purge should succeed
	if err := queue.PurgeDelivered(ctx, spoolID); err != nil {
		t.Fatalf("PurgeDelivered failed: %v", err)
	}
}
