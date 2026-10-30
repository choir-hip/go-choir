package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
)

// The gateway's result cache, end to end through HandleSearch
// (docs/archive/problems/search-is-duplicated-and-discarded-2026-10-14.md). Ways it
// can fail here (numbers from that record):
//  3. Computer B is answered from computer A's search under the default
//     (per-computer) scope.
//  7. A replay changes the NDJSON shape: the guest needs start, one
//     provider line per provider, done, in that order.
//  9. With the cache off (TTL 0) the provider is called every time.
// 11. The log line does not say a search was served from the cache, or
//     loses the "answered M of N providers" text the journal is parsed by.
//  -. The seen-URL count is unbounded, counts a URL twice within one
//     search, or never forgets.

func countingProvider(name string, calls *atomic.Int32, results ...SearchResult) *mockSearchProvider {
	return &mockSearchProvider{name: name, available: true, searchFunc: func(ctx context.Context, q string, max int) ([]SearchResult, error) {
		calls.Add(1)
		return results, nil
	}}
}

func cachedSearchClient(providers []SearchProvider, shared bool) *SearchClient {
	return &SearchClient{
		providers:   providers,
		healthStore: searchplane.NewMemoryHealthStore(),
		planeConfig: searchplane.Config{
			RequestTimeout: 5 * time.Second,
			CacheTTL:       30 * time.Minute,
			CacheFreshTTL:  10 * time.Minute,
			CacheMaxBytes:  1 << 20,
			CacheShared:    shared,
		},
	}
}

// searchAs posts a query as one computer and returns every NDJSON line.
func searchAs(t *testing.T, client *SearchClient, computers ...string) func(computer, query string) []map[string]any {
	t.Helper()
	registry := NewIdentityRegistry(time.Hour)
	h := &Handler{registry: registry, searchClient: client}
	srv := httptest.NewServer(http.HandlerFunc(h.HandleSearch))
	t.Cleanup(srv.Close)
	tokens := map[string]string{}
	for _, c := range computers {
		cred, err := registry.IssueCredential(c)
		if err != nil {
			t.Fatal(err)
		}
		tokens[c] = cred.RawToken
	}
	return func(computer, query string) []map[string]any {
		body, _ := json.Marshal(SearchRequest{Query: query, MaxResults: 5})
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/provider/v1/search", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+tokens[computer])
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		var lines []map[string]any
		dec := json.NewDecoder(resp.Body)
		for dec.More() {
			var line map[string]any
			if err := dec.Decode(&line); err != nil {
				t.Fatal(err)
			}
			lines = append(lines, line)
		}
		return lines
	}
}

func lineTypes(lines []map[string]any) string {
	var out []string
	for _, l := range lines {
		out = append(out, l["type"].(string))
	}
	return strings.Join(out, ",")
}

var logMu sync.Mutex

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	logMu.Lock()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev); logMu.Unlock() })
	return &buf
}

func TestHandleSearch_CacheHitReplaysTheSameStreamShape(t *testing.T) { // 7, 11
	logs := captureLog(t)
	var calls atomic.Int32
	client := cachedSearchClient([]SearchProvider{
		countingProvider("keenable", &calls, SearchResult{Title: "k", URL: "https://k.example/1"}, SearchResult{Title: "k2", URL: "https://k.example/2"}),
	}, false)
	search := searchAs(t, client, "vm-a")
	first := search("vm-a", "parlioSDR README")
	client.cache.WaitIdleForTest()
	second := search("vm-a", "parlioSDR  README")
	if calls.Load() != 1 {
		t.Fatalf("provider called %d times, want 1", calls.Load())
	}
	if lineTypes(first) != "start,provider,done" || lineTypes(second) != "start,provider,done" {
		t.Fatalf("shapes = %s / %s, want start,provider,done", lineTypes(first), lineTypes(second))
	}
	if _, ok := first[0]["cached_at"]; ok {
		t.Fatalf("a live search carries cached_at: %v", first[0])
	}
	if second[0]["cached_at"] == nil || second[0]["cached_at"] == "" {
		t.Fatalf("a hit's start line has no cached_at: %v", second[0])
	}
	if got := len(second[1]["results"].([]any)); got != 2 {
		t.Fatalf("replayed results = %d, want 2", got)
	}
	if second[2]["answered"].(float64) != 1 || second[2]["total"].(float64) != 1 {
		t.Fatalf("hit done line = %v, want 1 of 1", second[2])
	}
	out := logs.String()
	for _, want := range []string{
		"answered 1 of 1 providers (0 cooling, 0 pending) skipped=[] served=miss results=2 unique=2 seen=0",
		"answered 1 of 1 providers (0 cooling, 0 pending) skipped=[] served=hit results=2 unique=2 seen=2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log lacks %q:\n%s", want, out)
		}
	}
}

func TestHandleSearch_CacheIsPerComputerByDefault(t *testing.T) { // 3
	var calls atomic.Int32
	client := cachedSearchClient([]SearchProvider{countingProvider("keenable", &calls, SearchResult{Title: "k", URL: "https://k.example/"})}, false)
	search := searchAs(t, client, "vm-a", "vm-b")
	search("vm-a", "q")
	client.cache.WaitIdleForTest()
	if lines := search("vm-b", "q"); lines[0]["cached_at"] != nil {
		t.Fatalf("vm-b was answered from vm-a's search: %v", lines[0])
	}
	if calls.Load() != 2 {
		t.Fatalf("provider called %d times, want 2", calls.Load())
	}
}

func TestHandleSearch_SharedScopeAnswersAcrossComputers(t *testing.T) {
	var calls atomic.Int32
	client := cachedSearchClient([]SearchProvider{countingProvider("keenable", &calls, SearchResult{Title: "k", URL: "https://k.example/"})}, true)
	search := searchAs(t, client, "vm-a", "vm-b")
	search("vm-a", "q")
	client.cache.WaitIdleForTest()
	search("vm-b", "q")
	if calls.Load() != 1 {
		t.Fatalf("provider called %d times with a shared cache, want 1", calls.Load())
	}
}

func TestHandleSearch_CacheOffCallsProvidersEveryTime(t *testing.T) { // 9
	var calls atomic.Int32
	client := testSearchClient([]SearchProvider{countingProvider("keenable", &calls, SearchResult{Title: "k", URL: "https://k.example/"})})
	search := searchAs(t, client, "vm-a")
	search("vm-a", "q")
	if lines := search("vm-a", "q"); lines[0]["cached_at"] != nil {
		t.Fatalf("cache off but answered from cache: %v", lines[0])
	}
	if calls.Load() != 2 {
		t.Fatalf("provider called %d times, want 2", calls.Load())
	}
}

func TestSearchURLSeen_CountsRepeatsPerComputerAndForgets(t *testing.T) {
	now := time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC)
	s := newSearchURLSeen(24*time.Hour, 3)
	s.now = func() time.Time { return now }
	if seen := s.observe("vm-a", []string{"https://A.example/1/", "https://a.example/1", "https://a.example/2"}); seen != 0 {
		t.Fatalf("first search seen = %d, want 0 (a URL twice in one search is not a repeat)", seen)
	}
	if seen := s.observe("vm-a", []string{"https://a.example/1", "https://a.example/3"}); seen != 1 {
		t.Fatalf("second search seen = %d, want 1", seen)
	}
	if seen := s.observe("vm-b", []string{"https://a.example/1"}); seen != 0 {
		t.Fatalf("another computer's seen = %d, want 0", seen)
	}
	now = now.Add(25 * time.Hour)
	if seen := s.observe("vm-a", []string{"https://a.example/1"}); seen != 0 {
		t.Fatalf("after a day seen = %d, want 0", seen)
	}
	s.observe("vm-a", []string{"https://a.example/4", "https://a.example/5", "https://a.example/6", "https://a.example/7"})
	if n := s.size("vm-a"); n > 3 {
		t.Fatalf("vm-a holds %d URLs, cap 3", n)
	}
}
