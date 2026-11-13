package computerevent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The guest counts its corpusd calls by endpoint, so a tape write's calls per
// event can be read beside the event count (storage solutions 2, S5).
func TestHTTPClientCountsCallsByEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) { return "t", nil }, true)
	if err != nil {
		t.Fatal(err)
	}
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
