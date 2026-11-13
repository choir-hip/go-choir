package computerevent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func countingClient(t *testing.T) *HTTPClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	client, err := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) { return "t", nil }, true)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// The guest counts its corpusd calls by endpoint, so a tape write's calls per
// event can be read beside the event count (storage solutions 2, S5).
func TestHTTPClientCountsCallsByEndpoint(t *testing.T) {
	client := countingClient(t)
	if got := client.CallCounts(); len(got) != 0 {
		t.Fatalf("fresh client counts = %v", got)
	}
	ctx := context.Background()
	_, _ = client.Head(ctx, "c1")
	_, _ = client.Head(ctx, "c1")
	_, _ = client.PinEvent(ctx, "c1", []byte("e"), "rc")
	_, _ = client.CompareAndSwap(ctx, CASRequest{})
	got := client.CallCounts()
	if got["head"] != 2 || got["pin"] != 1 || got["append"] != 1 {
		t.Fatalf("counts = %v, want head 2, pin 1, append 1", got)
	}
	var nilClient *HTTPClient
	if nilClient.CallCounts() != nil {
		t.Fatal("nil client counts must be nil")
	}
}

// No string derived from data may become a key: a host-build status poll puts
// a job id in the last path segment, and the counts are served on the
// observability endpoint.
func TestHTTPClientCallCountsNeverKeyOnData(t *testing.T) {
	client := countingClient(t)
	ctx := context.Background()
	_, _ = client.DoJSON(ctx, http.MethodGet, "/internal/computers/selfdev-builds/job-secret-1234?computer_id=c1", nil, nil)
	for i := 0; i < 100; i++ {
		_, _ = client.DoJSON(ctx, http.MethodGet, fmt.Sprintf("/internal/computers/selfdev-builds/job-%d", i), nil, nil)
	}
	_, _ = client.Head(ctx, "c1")
	got := client.CallCounts()
	if len(got) != 2 || got["other"] != 101 || got["head"] != 1 {
		t.Fatalf("counts = %v, want other 101 and head 1 only", got)
	}
	for k := range got {
		if strings.Contains(k, "job") {
			t.Fatalf("a job id became a key: %q", k)
		}
	}
}
