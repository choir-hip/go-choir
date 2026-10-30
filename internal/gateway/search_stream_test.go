package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
)

// POST /provider/v1/search streams NDJSON (Cut 2 unit 2a,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md).
// Ways it can fail:
//  1. The handler buffers, so a slow provider delays the lines of the
//     providers that already answered.
//  2. Lines come in registration order rather than completion order.
//  3. "M of N" counts a cooling provider among N, or as a failure, or
//     leaves it out of the final line.
//  4. A provider that never returns holds the stream open past the
//     provider timeout plus the margin, or the stream ends with no final
//     line naming what was still pending.
//  5. A provider error returned on a line carries the provider key.

func testSearchClient(providers []SearchProvider) *SearchClient {
	return &SearchClient{
		providers:   providers,
		healthStore: searchplane.NewMemoryHealthStore(),
		planeConfig: searchplane.Config{RequestTimeout: 5 * time.Second},
	}
}

func answering(name string, delay time.Duration, results ...SearchResult) *mockSearchProvider {
	return &mockSearchProvider{name: name, available: true, searchFunc: func(ctx context.Context, q string, max int) ([]SearchResult, error) {
		select {
		case <-time.After(delay):
			return results, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
}

// searchStreamServer serves the real handler and returns a function that
// posts a query and returns a line reader over the response body.
func searchStreamServer(t *testing.T, client *SearchClient) func(query string) (*bufio.Scanner, func()) {
	t.Helper()
	registry := NewIdentityRegistry(time.Hour)
	h := &Handler{registry: registry, searchClient: client}
	srv := httptest.NewServer(http.HandlerFunc(h.HandleSearch))
	t.Cleanup(srv.Close)
	cred, err := registry.IssueCredential("test-autoputer")
	if err != nil {
		t.Fatal(err)
	}
	return func(query string) (*bufio.Scanner, func()) {
		body, _ := json.Marshal(SearchRequest{Query: query, MaxResults: 5})
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/provider/v1/search", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/x-ndjson" {
			t.Fatalf("content type = %q, want application/x-ndjson", ct)
		}
		return bufio.NewScanner(resp.Body), func() { _ = resp.Body.Close() }
	}
}

type streamLine struct {
	Type      string                  `json:"type"`
	Query     string                  `json:"query"`
	Providers []string                `json:"providers"`
	Cooling   []SearchCoolingProvider `json:"cooling"`
	Provider  string                  `json:"provider"`
	Status    string                  `json:"status"`
	Results   []SearchResult          `json:"results"`
	Error     string                  `json:"error"`
	Answered  int                     `json:"answered"`
	Total     int                     `json:"total"`
	Pending   []string                `json:"pending"`
}

func nextLine(t *testing.T, sc *bufio.Scanner, within time.Duration) streamLine {
	t.Helper()
	got := make(chan streamLine, 1)
	go func() {
		var line streamLine
		if sc.Scan() {
			if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
				line.Type = "undecodable: " + sc.Text()
			}
		} else {
			line.Type = "eof"
		}
		got <- line
	}()
	select {
	case line := <-got:
		return line
	case <-time.After(within):
		t.Fatalf("no line within %s", within)
		return streamLine{}
	}
}

func TestHandleSearch_StreamsProviderLinesInCompletionOrder(t *testing.T) {
	release := make(chan struct{})
	slow := &mockSearchProvider{name: "slow", available: true, searchFunc: func(ctx context.Context, q string, max int) ([]SearchResult, error) {
		select {
		case <-release:
			return []SearchResult{{Title: "slow", URL: "https://slow.example/1"}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	client := testSearchClient([]SearchProvider{
		slow,
		answering("middle", 100*time.Millisecond, SearchResult{Title: "m", URL: "https://m.example/1"}),
		answering("fast", 0, SearchResult{Title: "f", URL: "https://f.example/1"}, SearchResult{Title: "f2", URL: "https://f.example/2"}),
	})
	post := searchStreamServer(t, client)
	sc, done := post("streamed query")
	defer done()

	start := nextLine(t, sc, 2*time.Second)
	if start.Type != "start" || start.Query != "streamed query" || strings.Join(start.Providers, ",") != "slow,middle,fast" {
		t.Fatalf("start line = %+v, want every eligible provider", start)
	}
	first := nextLine(t, sc, time.Second)
	second := nextLine(t, sc, time.Second)
	if first.Type != "provider" || first.Provider != "fast" || len(first.Results) != 2 || first.Results[0].Provider != "fast" {
		t.Fatalf("first provider line = %+v, want fast with its results while slow is still out", first)
	}
	if second.Provider != "middle" || second.Status != "success" {
		t.Fatalf("second provider line = %+v, want middle", second)
	}
	close(release)
	third := nextLine(t, sc, 2*time.Second)
	if third.Provider != "slow" {
		t.Fatalf("third provider line = %+v, want slow", third)
	}
	final := nextLine(t, sc, time.Second)
	if final.Type != "done" || final.Answered != 3 || final.Total != 3 || len(final.Pending) != 0 {
		t.Fatalf("final line = %+v, want 3 of 3 answered", final)
	}
	if end := nextLine(t, sc, time.Second); end.Type != "eof" {
		t.Fatalf("line after done = %+v, want end of stream", end)
	}
}

func TestHandleSearch_FinalLineCountsEligibleAndListsCoolingSeparately(t *testing.T) {
	quota := &mockSearchProvider{name: "quota", available: true, searchFunc: func(ctx context.Context, q string, max int) ([]SearchResult, error) {
		return nil, errors.New("status 402 Payment Required: NO_MORE_CREDITS")
	}}
	broken := &mockSearchProvider{name: "broken", available: true, searchFunc: func(ctx context.Context, q string, max int) ([]SearchResult, error) {
		return nil, errors.New(`status 500: Get "https://serpapi.com/search.json?api_key=sk-secret-0123456789&q=x"`)
	}}
	client := testSearchClient([]SearchProvider{quota, broken, answering("ok", 0, SearchResult{Title: "ok", URL: "https://ok.example/1"})})
	post := searchStreamServer(t, client)

	// The first search benches the quota-limited provider.
	sc, done := post("first")
	var lines []streamLine
	for line := nextLine(t, sc, 2*time.Second); line.Type != "eof"; line = nextLine(t, sc, 2*time.Second) {
		lines = append(lines, line)
	}
	done()
	for _, line := range lines {
		if strings.Contains(line.Error, "sk-secret") {
			t.Fatalf("provider line returns the provider key: %+v", line)
		}
	}
	if final := lines[len(lines)-1]; final.Type != "done" || final.Answered != 1 || final.Total != 3 {
		t.Fatalf("first final line = %+v, want 1 of 3 answered", final)
	}

	// The second starts without the two that failed: N is the set eligible
	// at start, and the cooling providers are listed on their own, never as
	// failures.
	sc, done = post("second")
	defer done()
	start := nextLine(t, sc, 2*time.Second)
	if strings.Join(start.Providers, ",") != "ok" {
		t.Fatalf("second start providers = %v, want only ok (the cooling ones left out)", start.Providers)
	}
	cooling := coolingNames(start.Cooling)
	if cooling != "quota,broken" || start.Cooling[0].Until == "" {
		t.Fatalf("second start cooling = %+v, want quota and broken with their until", start.Cooling)
	}
	// Each skipped provider says why, by class, never by summary
	// (docs/archive/problems/search-providers-cooling-most-of-the-time-2026-10-14.md).
	if start.Cooling[0].Reason != "quota_limited" || start.Cooling[1].Reason != "server_error" {
		t.Fatalf("second start cooling reasons = %+v, want quota_limited and server_error", start.Cooling)
	}
	var final streamLine
	for line := nextLine(t, sc, 2*time.Second); line.Type != "eof"; line = nextLine(t, sc, 2*time.Second) {
		if line.Type == "provider" && line.Provider != "ok" {
			t.Fatalf("cooling provider got a provider line: %+v", line)
		}
		final = line
	}
	if final.Type != "done" || final.Answered != 1 || final.Total != 1 || coolingNames(final.Cooling) != cooling {
		t.Fatalf("second final line = %+v, want 1 of 1 answered with the cooling providers listed", final)
	}
	if quota.searchCount != 1 {
		t.Fatalf("quota provider called %d times, want once (cooling after the first)", quota.searchCount)
	}
}

func TestHandleSearch_StreamEndsAtTheBound(t *testing.T) {
	stuck := make(chan struct{})
	defer close(stuck)
	// A provider that ignores its context: only the stream bound ends it.
	deaf := &mockSearchProvider{name: "deaf", available: true, searchFunc: func(ctx context.Context, q string, max int) ([]SearchResult, error) {
		<-stuck
		return nil, nil
	}}
	client := testSearchClient([]SearchProvider{deaf, answering("ok", 0, SearchResult{Title: "ok", URL: "https://ok.example/1"})})
	client.planeConfig.RequestTimeout = 100 * time.Millisecond
	client.streamMargin = 100 * time.Millisecond
	post := searchStreamServer(t, client)
	started := time.Now()
	sc, done := post("bounded")
	defer done()
	var final streamLine
	for line := nextLine(t, sc, 3*time.Second); line.Type != "eof"; line = nextLine(t, sc, 3*time.Second) {
		final = line
	}
	if took := time.Since(started); took > 2*time.Second {
		t.Fatalf("stream took %s, want it to end at the provider timeout plus the margin", took)
	}
	if final.Type != "done" || final.Answered != 1 || final.Total != 2 || strings.Join(final.Pending, ",") != "deaf" {
		t.Fatalf("final line = %+v, want 1 of 2 answered with deaf pending", final)
	}
}

func TestSearchClient_NoProviders(t *testing.T) {
	if _, err := testSearchClient(nil).Stream(context.Background(), SearchRequest{Query: "test"}); err == nil {
		t.Fatal("expected an error with no providers")
	}
}

func TestSearchClient_EmptyQuery(t *testing.T) {
	client := testSearchClient([]SearchProvider{answering("a", 0)})
	if _, err := client.Stream(context.Background(), SearchRequest{Query: ""}); err == nil {
		t.Fatal("expected an error for an empty query")
	}
}

func TestSearchClient_MaxResultsClamping(t *testing.T) {
	var got []int
	mock := &mockSearchProvider{name: "mock", available: true, searchFunc: func(ctx context.Context, q string, maxResults int) ([]SearchResult, error) {
		got = append(got, maxResults)
		return nil, nil
	}}
	client := testSearchClient([]SearchProvider{mock})
	for _, n := range []int{0, 100} {
		stream, err := client.Stream(context.Background(), SearchRequest{Query: "test", MaxResults: n})
		if err != nil {
			t.Fatal(err)
		}
		for range stream.Arrivals {
		}
	}
	if len(got) != 2 || got[0] != 40 || got[1] != 50 {
		t.Fatalf("provider max_results = %v, want the 40 default and the 50 cap", got)
	}
}

func coolingNames(cooling []SearchCoolingProvider) string {
	names := make([]string, len(cooling))
	for i, c := range cooling {
		names[i] = c.Provider
	}
	return strings.Join(names, ",")
}
