package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Search attribution in the gateway's log (search-reuse commit 1,
// docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md). Failure
// list, written before the code:
//  1. The request line does not carry the run, desk and conversation the
//     guest sent, so a search cannot be tied to the run that spent it.
//  2. A caller that sends none (an old guest) is refused, or logs the
//     literal text of an empty header instead of "-".
//  3. A header value with a space forges a second field in the log line.
//  4. The served line carries URLs, not the hashes search_seen.go makes, or
//     carries more than the first ten, or none.
//  5. The existing "answered M of N providers ... seen=N" text the journal
//     is parsed by changes.

func TestHandleSearch_LogsAttributionAndTheFirstTenResultHashes(t *testing.T) {
	logs := captureLog(t)
	var calls atomic.Int32
	var results []SearchResult
	var urls []string
	for i := 0; i < 12; i++ {
		u := fmt.Sprintf("https://k.example/%d", i)
		urls = append(urls, u)
		results = append(results, SearchResult{Title: "t", URL: u})
	}
	client := cachedSearchClient([]SearchProvider{countingProvider("keenable", &calls, results...)}, false)
	registry := NewIdentityRegistry(time.Hour)
	h := &Handler{registry: registry, searchClient: client}
	srv := httptest.NewServer(http.HandlerFunc(h.HandleSearch))
	defer srv.Close()
	cred, err := registry.IssueCredential("vm-a")
	if err != nil {
		t.Fatal(err)
	}
	post := func(query string, hdr map[string]string) {
		body, _ := json.Marshal(SearchRequest{Query: query, MaxResults: 40})
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/provider/v1/search", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		for k, v := range hdr {
			req.Header[k] = []string{v}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		// Read to the end: the handler writes its served line before it
		// returns, and the body ends only after it returns. Reading one token
		// and closing left the handler logging while the test read the log.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		client.cache.WaitIdleForTest()
	}
	post("first query", map[string]string{"X-Choir-Run": "run-7", "X-Choir-Desk": "research", "X-Choir-Conversation": "conv 1 forged line"})
	post("second query", nil)
	out := logs.String()

	if !strings.Contains(out, `(query="first query" max_results=40 run=run-7 desk=research conv=conv1forgedline)`) {
		t.Fatalf("request line lacks the attribution:\n%s", out)
	}
	if !strings.Contains(out, `(query="second query" max_results=40 run=- desk=- conv=-)`) {
		t.Fatalf("an unattributed request line must say run=- desk=- conv=-:\n%s", out)
	}
	m := regexp.MustCompile(`served=miss results=12 unique=12 seen=0 .* urls=\[([0-9a-f,]*)\]`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("served line lacks urls=[...]:\n%s", out)
	}
	got := strings.Split(m[1], ",")
	if len(got) != 10 {
		t.Fatalf("urls= has %d hashes, want the first 10", len(got))
	}
	for i, u := range urls[:10] {
		if want := fmt.Sprintf("%x", searchURLKey(u)); got[i] != want {
			t.Fatalf("hash %d = %s, want %s", i, got[i], want)
		}
	}
	if strings.Contains(out, "k.example/3") && strings.Contains(m[0], "k.example") {
		t.Fatalf("the served line carries a URL, not a hash")
	}
}
