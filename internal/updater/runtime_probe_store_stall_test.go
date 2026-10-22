package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A guest whose store is held answers /health 503 {"status":"store_stalled"}
// in about a second, where it used to time out (F1,
// docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md).
// The probe's stall budget must stay the wall time it had when each stalled
// attempt cost a client timeout: attempts x (client timeout + interval).

func storeStallServer(stallFor time.Duration) *httptest.Server {
	start := time.Now()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if stallFor < 0 || time.Since(start) < stallFor {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "store_stalled", "service": "autoputer"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":                  "ok",
			"self_development_marker": "marker-1",
			"event_schema_version":    1,
			"reducer_version":         1,
			"release_digest":          strings.Repeat("a", 64),
		})
	}))
}

func storeStallProber(url string) HTTPHealthProber {
	// Budget: 3 x (100ms + 1ms), about 303ms.
	return HTTPHealthProber{URL: url, Client: &http.Client{Timeout: 100 * time.Millisecond}, Attempts: 3, Interval: time.Millisecond}
}

func TestHTTPHealthProberWaitsOutAStoreStallAsLongAsTimeoutsDid(t *testing.T) {
	server := storeStallServer(100 * time.Millisecond)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	observations, err := storeStallProber(server.URL).Probe(ctx, strings.Repeat("a", 64), ReleaseManifest{
		Marker: "marker-1", EventSchemaVersion: 1, ReducerVersion: 1,
	})
	if err != nil {
		t.Fatalf("a 100ms store stall inside a ~303ms budget failed the probe: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("observations = %v", observations)
	}
}

func TestHTTPHealthProberStoreStallFailsAtTheTimeoutBudget(t *testing.T) {
	server := storeStallServer(-1)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err := storeStallProber(server.URL).Probe(ctx, strings.Repeat("a", 64), ReleaseManifest{Marker: "marker-1"})
	took := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "health probe failed") || !strings.Contains(err.Error(), "store_stalled") {
		t.Fatalf("an endless store stall should fail the probe naming it, got %v", err)
	}
	if took < 250*time.Millisecond {
		t.Fatalf("store stall failed the probe after %s; the budget is about 303ms", took)
	}
	if took > 2*time.Second {
		t.Fatalf("store stall held the probe %s, far past its ~303ms budget", took)
	}
}
