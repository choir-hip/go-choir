package computerevent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestHTTPClientFetchPayloadReadsOversizedPayloadEnvelope(t *testing.T) {
	payload := []byte(strings.Repeat("p", 900_000))
	digest := DigestBytes(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("computer_id") != "computer-payload" || r.URL.Query().Get("artifact_digest") != digest {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"payload_base64": base64.RawStdEncoding.EncodeToString(payload),
		})
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) {
		return "test-token", nil
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.FetchPayload(context.Background(), "computer-payload", digest)
	if err != nil {
		t.Fatalf("FetchPayload() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload length = %d, want %d", len(got), len(payload))
	}
}

func TestHTTPClientEventsReadsOversizedPagesAndProgresses(t *testing.T) {
	records := make([]DurableEvent, EventReplayPageSize+1)
	for index := range records {
		records[index].Request.Event.Sequence = uint64(index + 1)
	}
	records[0].Request.Event.AuthorityRef = strings.Repeat("x", 1<<20)

	var mu sync.Mutex
	var afters []uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		after, err := strconv.ParseUint(r.URL.Query().Get("after_sequence"), 10, 64)
		if err != nil {
			t.Errorf("after_sequence: %v", err)
			return
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit != EventReplayPageSize {
			t.Errorf("limit = %q, want %d", r.URL.Query().Get("limit"), EventReplayPageSize)
			return
		}
		mu.Lock()
		afters = append(afters, after)
		mu.Unlock()
		start := int(after)
		if start > len(records) {
			start = len(records)
		}
		end := start + limit
		if end > len(records) {
			end = len(records)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(records[start:end])
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) {
		return "test-token", nil
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Events(context.Background(), "computer-replay", 0)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if len(got) != len(records) {
		t.Fatalf("replayed %d records, want %d", len(got), len(records))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(afters) != 2 || afters[0] != 0 || afters[1] != EventReplayPageSize {
		t.Fatalf("page after_sequence values = %v, want [0 %d]", afters, EventReplayPageSize)
	}
}

func TestHTTPClientEventsRejectsNonProgressingPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]DurableEvent{{Request: CASRequest{Event: Event{Sequence: 2}}}})
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) {
		return "test-token", nil
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Events(context.Background(), "computer-replay", 0)
	if err == nil || !strings.Contains(err.Error(), "replay sequence did not progress") {
		t.Fatalf("non-progressing replay error = %v", err)
	}
}

func TestHTTPClientEventsRejectsOversizedPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]DurableEvent{{Request: CASRequest{Event: Event{AuthorityRef: strings.Repeat("x", EventReplayMaxResponseBytes)}}}})
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) {
		return "test-token", nil
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Events(context.Background(), "computer-replay", 0)
	if err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("oversized replay error = %v", err)
	}
}

func TestHTTPClientEventsSendsComputerAndSequenceQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("computer_id") != "computer-replay" || r.URL.Query().Get("after_sequence") != "7" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]DurableEvent{})
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) {
		return "test-token", nil
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Events(context.Background(), "computer-replay", 7); err != nil {
		t.Fatal(err)
	}
}
