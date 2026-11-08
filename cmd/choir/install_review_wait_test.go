package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// An install answered 202 install_review_owed is a wait, not an error: the
// owner's CLI prints the body (state pending_review) and exits clean. The
// client treats every 2xx as success, and the body names what is happening.
func TestInstallAnsweringAReviewWaitIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"error":"management is reviewing this release; it installs when management accepts","reason":"install_review_owed","owed_record":"install-review-owed:r:s","state":"pending_review"}`)
	}))
	defer srv.Close()
	c := &client{host: srv.URL, apiKey: "k", http: srv.Client(), stdout: io.Discard, stderr: io.Discard}
	var out json.RawMessage
	if err := c.do(http.MethodPost, installsPath("c1", ""), map[string]any{"release_id": "r", "action": "install"}, &out); err != nil {
		t.Fatalf("a 202 review wait errored: %v", err)
	}
	var body struct{ State, Reason string }
	if err := json.Unmarshal(out, &body); err != nil || body.State != "pending_review" || body.Reason != "install_review_owed" {
		t.Fatalf("body = %s, %v", out, err)
	}
}
