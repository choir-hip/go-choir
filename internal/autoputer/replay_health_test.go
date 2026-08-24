package autoputer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestReplayHealthGateReportsProgressThenReadiness(t *testing.T) {
	base := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}
	gate := &replayHealthGate{base: base}

	// While pending with no appender, /health must be 503 replaying.
	gate.setPending(true)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("pending gate status=%d, want 503", rec.Code)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Status != "replaying" {
		t.Fatalf("pending gate body=%q status=%q", rec.Body.String(), body.Status)
	}

	// After replay completes, /health must pass through to the base 200.
	gate.setPending(false)
	rec = httptest.NewRecorder()
	gate.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ready gate status=%d, want 200", rec.Code)
	}

	// ReplaySnapshot integration: an in-progress appender reports its sequence.
	app := &computerevent.ComputerEventAppender{}
	app.SetReplayMode(true)
	// ReplaySnapshot reads fields that are only updated by the reconstruct loop;
	// a fresh snapshot reports no progress and must still render as replaying.
	gate.appender = app
	gate.setPending(true)
	rec = httptest.NewRecorder()
	gate.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("appender-backed pending gate status=%d, want 503", rec.Code)
	}
}
