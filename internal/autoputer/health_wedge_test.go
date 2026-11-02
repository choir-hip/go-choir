package autoputer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// A persistent appender wedge degrades the /health body but never the status
// code: vmctl and the probes key on 200, and a failing health would restart
// the computer (closing its work). Ways it can fail: the wedge is invisible;
// a transient wedge degrades; the code leaves 200.
func TestReplayHealthGateReportsPersistentWedgeKeeps200(t *testing.T) {
	base := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "autoputer"})
	}
	wedge := computerevent.WedgeStatus{}
	gate := &replayHealthGate{base: base, wedge: func() computerevent.WedgeStatus { return wedge }}
	get := func() (int, map[string]any) {
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body %q: %v", rec.Body.String(), err)
		}
		return rec.Code, body
	}

	if code, body := get(); code != 200 || body["status"] != "ok" || body["projection_repair_required"] != nil {
		t.Fatalf("no wedge: code=%d body=%v", code, body)
	}
	wedge = computerevent.WedgeStatus{Active: true, Consecutive: 2}
	if code, body := get(); code != 200 || body["status"] != "ok" || body["projection_repair_required"] != nil {
		t.Fatalf("transient wedge degraded health: code=%d body=%v", code, body)
	}
	wedge = computerevent.WedgeStatus{Active: true, Persistent: true, Consecutive: 9, LocalSeq: 4, PlatformSeq: 5, FirstError: "finalize: projection poison", LastError: "finalize: projection poison"}
	code, body := get()
	if code != http.StatusOK {
		t.Fatalf("persistent wedge changed the status code to %d; it must stay 200", code)
	}
	if body["status"] != "degraded" || body["service"] != "autoputer" {
		t.Fatalf("body = %v, want degraded with the base fields kept", body)
	}
	pr, _ := body["projection_repair_required"].(map[string]any)
	if pr == nil || pr["consecutive"] != float64(9) || pr["local_seq"] != float64(4) || pr["platform_seq"] != float64(5) || pr["first_error"] != "finalize: projection poison" {
		t.Fatalf("projection_repair_required = %v", pr)
	}
	wedge = computerevent.WedgeStatus{}
	if _, body := get(); body["status"] != "ok" || body["projection_repair_required"] != nil {
		t.Fatalf("cleared wedge still degraded: %v", body)
	}
}
