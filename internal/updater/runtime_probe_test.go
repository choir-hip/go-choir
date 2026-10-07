package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestHTTPHealthProberToleratesAdvancingReplay reproduces
// app-layer-push-health-gate-not-commit-bound-2026-10-07: the owner
// computer's post-restart replay serves 503 {"status":"replaying",
// committed_sequence, progress} longer than the probe's fixed 30-attempt
// fence, so the apply rolled back a healthy release. A 503 body whose
// replay markers ADVANCE between polls is liveness, not failure — the
// probe must keep waiting within its wall-time cap instead of exhausting
// attempts.
func TestHTTPHealthProberToleratesAdvancingReplay(t *testing.T) {
	const replayResponses = 40 // one more than the default 30-attempt stall budget
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call <= replayResponses {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":             "replaying",
				"sequence":           546000 + call,
				"committed_sequence": 546000 + call,
				"progress":           call,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":                  "ok",
			"self_development_marker": "marker-1",
			"event_schema_version":    1,
			"reducer_version":         1,
			"release_digest":          strings.Repeat("a", 64),
		})
	}))
	defer server.Close()

	prober := HTTPHealthProber{URL: server.URL, Interval: time.Millisecond}
	observations, err := prober.Probe(context.Background(), strings.Repeat("a", 64), ReleaseManifest{
		Marker:             "marker-1",
		EventSchemaVersion: 1,
		ReducerVersion:     1,
	})
	if err != nil {
		t.Fatalf("probe gave up while replay was still advancing: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("observations = %v", observations)
	}
	if calls.Load() <= replayResponses {
		t.Fatalf("probe returned after %d calls; replay liveness was not consumed", calls.Load())
	}
}

// TestHTTPHealthProberStallFails ensures the liveness tolerance is not a
// free pass: a replaying 503 whose sequence/progress do NOT advance still
// exhausts the attempt budget and fails.
func TestHTTPHealthProberStallFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":             "replaying",
			"sequence":           546148,
			"committed_sequence": 546148,
			"progress":           0,
		})
	}))
	defer server.Close()

	prober := HTTPHealthProber{URL: server.URL, Attempts: 5, Interval: time.Millisecond}
	_, err := prober.Probe(context.Background(), strings.Repeat("a", 64), ReleaseManifest{Marker: "marker-1"})
	if err == nil || !strings.Contains(err.Error(), "health probe failed") {
		t.Fatalf("stalled replay should fail the probe, got %v", err)
	}
}

// TestHTTPHealthProberMaxDurationCaps ensures liveness tolerance is bounded:
// an ever-advancing replay cannot hold the apply open past the prober's
// absolute wall-time cap.
func TestHTTPHealthProberMaxDurationCaps(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":             "replaying",
			"sequence":           call,
			"committed_sequence": call,
			"progress":           call,
		})
	}))
	defer server.Close()

	prober := HTTPHealthProber{URL: server.URL, Interval: time.Millisecond, MaxDuration: 20 * time.Millisecond}
	_, err := prober.Probe(context.Background(), strings.Repeat("a", 64), ReleaseManifest{Marker: "marker-1"})
	if err == nil {
		t.Fatal("advancing replay ran past MaxDuration without failing")
	}
	if calls.Load() > 2000 {
		t.Fatalf("probe ran %d calls past MaxDuration", calls.Load())
	}
}

func ExampleHTTPHealthProber_Probe() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":                  "ok",
			"self_development_marker": "m",
			"event_schema_version":    2,
			"reducer_version":         3,
			"release_digest":          strings.Repeat("b", 64),
		})
	}))
	defer server.Close()
	prober := HTTPHealthProber{URL: server.URL}
	observations, err := prober.Probe(context.Background(), strings.Repeat("b", 64), ReleaseManifest{
		Marker: "m", EventSchemaVersion: 2, ReducerVersion: 3,
	})
	fmt.Println(len(observations), err)
	// Output: 1 <nil>
}
