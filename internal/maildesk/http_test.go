package maildesk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The guest reaches the owner's mailbox on the host over HTTP (maild's tap
// route). Failure modes: the ref is not path-escaped; the owner header is not
// sent (maild binds the tap caller to the asserted owner); a 404 is not
// ErrNoMessage, so a missing ref leases; a non-200 is read as a message.
func TestHTTPMailboxReadsOneMessageAsTheOwner(t *testing.T) {
	var gotOwner, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOwner, gotPath = r.Header.Get("X-Authenticated-User"), r.URL.EscapedPath()
		if r.URL.Path != "/api/email/messages/msg a" || r.Method != http.MethodGet {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"message":{"id":"msg a","from_address":"a@example.com","from_display":"A","subject":"hello","received_at":"2026-11-08T00:00:00Z","trust_status":"trusted"},"text_body":"the body","html_body":"<p>x</p>"}`))
	}))
	defer srv.Close()
	mb := NewHTTPMailbox(srv.URL + "/")

	m, err := mb.Message(context.Background(), "owner-1", "msg a")
	if err != nil {
		t.Fatal(err)
	}
	if gotOwner != "owner-1" || gotPath != "/api/email/messages/msg%20a" {
		t.Fatalf("owner %q path %q", gotOwner, gotPath)
	}
	if m.From != "a@example.com" || m.Subject != "hello" || m.TextBody != "the body" || m.TrustStatus != "trusted" || m.ReceivedAt == "" {
		t.Fatalf("message: %+v", m)
	}
	if _, err := mb.Message(context.Background(), "owner-1", "nope"); !errors.Is(err, ErrNoMessage) {
		t.Fatalf("a 404 is %v, want ErrNoMessage", err)
	}
	srv.Close()
	if _, err := mb.Message(context.Background(), "owner-1", "msg a"); err == nil || errors.Is(err, ErrNoMessage) {
		t.Fatalf("an unreachable maild is %v, want a plain error", err)
	}
	if _, err := NewHTTPMailbox("").Message(context.Background(), "o", "r"); err == nil {
		t.Fatal("an empty maild URL served a message")
	}
}

// A 200 with no message id is not a message: Mail leases no nonexistent ref.
func TestHTTPMailboxRefusesAnAnswerWithNoMessageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":{"subject":"x"},"text_body":"y"}`))
	}))
	defer srv.Close()
	if _, err := NewHTTPMailbox(srv.URL).Message(context.Background(), "o", "r"); !errors.Is(err, ErrNoMessage) {
		t.Fatalf("an answer with no id = %v, want ErrNoMessage", err)
	}
}
