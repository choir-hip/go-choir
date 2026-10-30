package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/steptrace"
)

// docs/problems/engineering-completion-path-cluster-2026-10-20.md failure 3:
// a Complete that never returns must be locatable from the product API. A
// step that is running shows on /api/runtime/observability with its scope,
// name and start time while it runs, and moves to the recent list when it
// ends.
func TestObservabilityShowsTheStepAHungCompleteIsIn(t *testing.T) {
	rt := telemetryRuntime(t, "computer-step-trace-obs")
	read := func() (steps steptrace.Snapshot) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
		req.Header.Set("X-Authenticated-User", "owner-telemetry")
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
		var obs struct {
			Steps *steptrace.Snapshot `json:"steps"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || obs.Steps == nil {
			t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
		}
		return *obs.Steps
	}

	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = traceCall(rt, "run:assignment-hung", "complete.freeze_capsule", func() (int, error) {
			close(entered)
			<-release
			return 0, nil
		})
	}()
	<-entered

	steps := read()
	if len(steps.InFlight) != 1 || steps.InFlight[0].Scope != "run:assignment-hung" || steps.InFlight[0].Step != "complete.freeze_capsule" ||
		steps.InFlight[0].StartedAt == "" {
		t.Fatalf("in-flight steps while the step runs = %+v", steps)
	}

	close(release)
	<-done
	steps = read()
	if len(steps.InFlight) != 0 || len(steps.Recent) != 1 || steps.Recent[0].Step != "complete.freeze_capsule" || steps.Recent[0].Error != "" {
		t.Fatalf("steps after the step ends = %+v", steps)
	}
}
