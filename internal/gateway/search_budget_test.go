package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
	"github.com/yusefmosiah/go-choir/internal/search"
)

// Budget routing end to end through HandleSearch
// (docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md §4.1). Ways it can
// fail here (numbers from that record):
//  1. A sufficient free answer still calls a paid provider.
//  5. A capped provider is not named with reason "capped" on the start
//     line and in the log line's skipped list.
//  7. A deep search is answered from an ordinary search's cached results.
//  8. The stream breaks the guest decoder: a new line type, a total below
//     the arrivals, or an escalated provider's line lost.
//  -. The log line does not say which tiers ran, whether and why the
//     search escalated, the paid calls made, and the day's counts.
//  -. The health route does not show today's counts and caps.

func budgetSearchClient(providers []SearchProvider, caps map[string]int) *SearchClient {
	return &SearchClient{
		providers:   providers,
		healthStore: searchplane.NewMemoryHealthStore(),
		planeConfig: searchplane.Config{
			RequestTimeout: 5 * time.Second,
			Routing:        searchplane.RoutingBudget,
			Tiers:          searchplane.ParseProviderTiers(searchplane.DefaultProviderTiers),
			DailyCaps:      caps,
			MinResults:     10,
			EscalateAfter:  2 * time.Second,
			CacheTTL:       30 * time.Minute,
			CacheFreshTTL:  10 * time.Minute,
			CacheMaxBytes:  1 << 20,
		},
	}
}

func results(prefix string, n int) []SearchResult {
	out := make([]SearchResult, n)
	for i := range out {
		out[i] = SearchResult{Title: fmt.Sprintf("%s %d", prefix, i), URL: fmt.Sprintf("https://%s.example/%d", prefix, i)}
	}
	return out
}

// budgetPoster posts a search as one computer and returns the response body.
func budgetPoster(t *testing.T, client *SearchClient) func(req SearchRequest) io.ReadCloser {
	t.Helper()
	registry := NewIdentityRegistry(time.Hour)
	h := &Handler{registry: registry, searchClient: client}
	srv := httptest.NewServer(http.HandlerFunc(h.HandleSearch))
	t.Cleanup(srv.Close)
	cred, err := registry.IssueCredential("vm-budget")
	if err != nil {
		t.Fatal(err)
	}
	return func(sr SearchRequest) io.ReadCloser {
		body, _ := json.Marshal(sr)
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/provider/v1/search", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		return resp.Body
	}
}

func readLines(t *testing.T, body io.ReadCloser) []map[string]any {
	t.Helper()
	defer func() { _ = body.Close() }()
	var lines []map[string]any
	dec := json.NewDecoder(body)
	for dec.More() {
		var line map[string]any
		if err := dec.Decode(&line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	return lines
}

func TestHandleSearch_BudgetEscalationStreamAndLogLine(t *testing.T) { // 8, log
	logs := captureLog(t)
	var serperCalls, tavilyCalls atomic.Int32
	client := budgetSearchClient([]SearchProvider{
		answering("keenable", 0, results("k", 3)...),
		countingProvider("tavily", &tavilyCalls, results("t", 20)...),
		countingProvider("serper", &serperCalls, results("s", 20)...),
	}, map[string]int{"serper": 5})
	post := budgetPoster(t, client)
	lines := readLines(t, post(SearchRequest{Query: "mauritius riots", MaxResults: 40}))
	if got := lineTypes(lines); got != "start,provider,provider,done" {
		t.Fatalf("line types = %s", got)
	}
	start, done := lines[0], lines[3]
	if fmt.Sprint(start["providers"]) != "[keenable]" || start["routing"] != "budget" || fmt.Sprint(start["reserve"]) != "[serper tavily]" {
		t.Fatalf("start = %v", start)
	}
	if lines[2]["provider"] != "serper" {
		t.Fatalf("escalated line = %v", lines[2])
	}
	if done["total"] != float64(2) || done["answered"] != float64(2) || fmt.Sprint(done["escalated"]) != "[map[provider:serper reason:short]]" {
		t.Fatalf("done = %v", done)
	}
	if serperCalls.Load() != 1 || tavilyCalls.Load() != 0 {
		t.Fatalf("serper=%d tavily=%d", serperCalls.Load(), tavilyCalls.Load())
	}
	want := "answered 2 of 2 providers (0 cooling, 0 pending) skipped=[] served=miss results=23 unique=23 seen=0 routing=budget escalated=short(3):serper paid_calls=1 today=[keenable=1 serper=1/5]"
	if !strings.Contains(logs.String(), want) {
		t.Fatalf("log = %s\nwant %s", logs.String(), want)
	}
}

func TestHandleSearch_BudgetFreeSufficientNoPaidCalls(t *testing.T) { // 1
	logs := captureLog(t)
	var serperCalls atomic.Int32
	client := budgetSearchClient([]SearchProvider{
		answering("keenable", 0, results("k", 12)...),
		countingProvider("serper", &serperCalls, results("s", 20)...),
	}, nil)
	lines := readLines(t, budgetPoster(t, client)(SearchRequest{Query: "q", MaxResults: 40}))
	if got := lineTypes(lines); got != "start,provider,done" || serperCalls.Load() != 0 {
		t.Fatalf("lines=%s serper=%d", got, serperCalls.Load())
	}
	if _, ok := lines[2]["escalated"]; ok {
		t.Fatalf("done = %v", lines[2])
	}
	if !strings.Contains(logs.String(), "routing=budget escalated=no paid_calls=0 today=[keenable=1]") {
		t.Fatalf("log = %s", logs.String())
	}
}

func TestHandleSearch_CappedProviderNamedWithReason(t *testing.T) { // 5
	logs := captureLog(t)
	var serperCalls atomic.Int32
	client := budgetSearchClient([]SearchProvider{
		answering("keenable", 0, results("k", 1)...),
		countingProvider("serper", &serperCalls, results("s", 20)...),
	}, map[string]int{"serper": 0})
	lines := readLines(t, budgetPoster(t, client)(SearchRequest{Query: "q", MaxResults: 40}))
	cooling := lines[0]["cooling"].([]any)
	if len(cooling) != 1 || cooling[0].(map[string]any)["reason"] != "capped" || cooling[0].(map[string]any)["state"] != "capped" {
		t.Fatalf("start cooling = %v", cooling)
	}
	if serperCalls.Load() != 0 {
		t.Fatal("capped provider called")
	}
	if !strings.Contains(logs.String(), "(1 cooling, 0 pending) skipped=[serper=capped]") || !strings.Contains(logs.String(), "today=[keenable=1 serper=0/0]") {
		t.Fatalf("log = %s", logs.String())
	}
}

func TestHandleSearch_DeepIsNotAnsweredFromAnOrdinaryCachedSearch(t *testing.T) { // 7
	var keenableCalls, serperCalls atomic.Int32
	client := budgetSearchClient([]SearchProvider{
		countingProvider("keenable", &keenableCalls, results("k", 40)...),
		countingProvider("serper", &serperCalls, results("s", 5)...),
	}, nil)
	post := budgetPoster(t, client)
	readLines(t, post(SearchRequest{Query: "q", MaxResults: 40}))
	client.cache.WaitIdleForTest()
	readLines(t, post(SearchRequest{Query: "q", MaxResults: 40}))
	if keenableCalls.Load() != 1 || serperCalls.Load() != 0 {
		t.Fatalf("ordinary repeat: keenable=%d serper=%d, want a cache hit", keenableCalls.Load(), serperCalls.Load())
	}
	lines := readLines(t, post(SearchRequest{Query: "q", MaxResults: 40, Depth: "deep"}))
	if keenableCalls.Load() != 2 || serperCalls.Load() != 1 {
		t.Fatalf("deep: keenable=%d serper=%d, want a live deep search", keenableCalls.Load(), serperCalls.Load())
	}
	if fmt.Sprint(lines[0]["providers"]) != "[keenable serper]" {
		t.Fatalf("deep start = %v", lines[0])
	}
}

func TestHandleSearch_GuestDecoderReadsAnEscalatedStream(t *testing.T) { // 8
	client := budgetSearchClient([]SearchProvider{
		&mockSearchProvider{name: "keenable", available: true, searchFunc: func(ctx context.Context, q string, max int) ([]SearchResult, error) {
			return nil, errors.New("status 500")
		}},
		answering("serper", 0, results("s", 20)...),
	}, nil)
	stream, err := search.ReadStream(budgetPoster(t, client)(SearchRequest{Query: "q", MaxResults: 40}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()
	var got []string
	for {
		a, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, a.Provider+"="+a.Status)
	}
	done, ok := stream.Done()
	if !ok || done.Total != 2 || done.Answered != 1 || len(done.Pending) != 0 {
		t.Fatalf("done = %+v", done)
	}
	if fmt.Sprint(stream.Start.Providers) != "[keenable]" || fmt.Sprint(got) != "[keenable=server_error serper=success]" {
		t.Fatalf("start=%v arrivals=%v", stream.Start.Providers, got)
	}
}

func TestHandleSearchHealth_ReportsDailyCalls(t *testing.T) {
	client := budgetSearchClient([]SearchProvider{
		answering("keenable", 0, results("k", 1)...),
		answering("serper", 0, results("s", 20)...),
	}, map[string]int{"serper": 150})
	readLines(t, budgetPoster(t, client)(SearchRequest{Query: "q", MaxResults: 40}))
	h := &Handler{searchClient: client}
	req := httptest.NewRequest(http.MethodGet, "/provider/v1/search/health", nil)
	req.RemoteAddr = "127.0.0.1:41000"
	req.Header.Set(internalCallerHeader, "true")
	rec := httptest.NewRecorder()
	h.HandleSearchHealth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		DailyCalls map[string]SearchDailyCalls `json:"daily_calls"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC().Format("2006-01-02")
	if got := body.DailyCalls["serper"]; got != (SearchDailyCalls{Day: today, Calls: 1, Cap: 150, Tier: "paid"}) {
		t.Fatalf("serper = %+v", got)
	}
	if got := body.DailyCalls["keenable"]; got != (SearchDailyCalls{Day: today, Calls: 1, Cap: -1, Tier: "free"}) {
		t.Fatalf("keenable = %+v", got)
	}
}

func TestSearchClient_StreamBoundGrowsByTheEscalationWindow(t *testing.T) {
	client := budgetSearchClient(nil, nil)
	client.planeConfig.EscalateWindow = 7 * time.Second
	if got := client.StreamBound(); got != 5*time.Second+7*time.Second+searchStreamMargin {
		t.Fatalf("budget bound = %v", got)
	}
	client.planeConfig.Routing = searchplane.RoutingFanout
	if got := client.StreamBound(); got != 5*time.Second+searchStreamMargin {
		t.Fatalf("fan-out bound = %v, want today's", got)
	}
}

// Keenable's monthly pace on the operator route
// (docs/archive/problems/search-throughput-keenable-underused-2026-10-15.md): the
// route shows the period, its use before today, the target and today's
// cap, so the owner can read the pace against the billing page.
func TestHandleSearchHealth_ReportsKeenableMonthlyPace(t *testing.T) {
	client := budgetSearchClient([]SearchProvider{answering("keenable", 0, results("k", 12)...)}, nil)
	client.planeConfig.MonthlyQuotas = map[string]searchplane.MonthlyQuota{"keenable": {Allowance: 100000, RenewDay: 5, TargetPct: 90}}
	readLines(t, budgetPoster(t, client)(SearchRequest{Query: "q", MaxResults: 40}))
	h := &Handler{searchClient: client}
	req := httptest.NewRequest(http.MethodGet, "/provider/v1/search/health", nil)
	req.RemoteAddr = "127.0.0.1:41000"
	req.Header.Set(internalCallerHeader, "true")
	rec := httptest.NewRecorder()
	h.HandleSearchHealth(rec, req)
	var body struct {
		DailyCalls map[string]SearchDailyCalls `json:"daily_calls"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	got := body.DailyCalls["keenable"]
	m := got.Monthly
	if m == nil || got.Calls != 1 || m.Target != 90000 || m.UsedBeforeToday != 0 || m.DaysLeft < 1 || m.DayCap != 90000/m.DaysLeft || got.Cap != m.DayCap {
		t.Fatalf("keenable = %+v monthly = %+v", got, m)
	}
	start, err := time.Parse("2006-01-02", m.PeriodStart)
	if err != nil || start.Day() != 5 || !strings.HasSuffix(m.PeriodEnd, "-05") {
		t.Fatalf("period %s..%s", m.PeriodStart, m.PeriodEnd)
	}
}
