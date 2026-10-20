package gateway

// Failure modes the source-api route must hold against (written before the
// code; docs/problems/keyed-source-apis-2026-10-20.md section 2):
//
//  1. a caller without a valid gateway token gets a key-bearing call: 401.
//  2. an unknown provider, endpoint or parameter reaches upstream: 400, no
//     upstream call.
//  3. a parameter carries a path segment, host, scheme or query separator
//     into the URL: refused, never a different path.
//  4. the key appears in a response body, log line, error string or
//     guest-visible field: never.
//  5. a provider without a key loaded is called: 503, no upstream call.
//  6. a provider over its daily budget is called: 429, no upstream call.
//  7. an upstream body larger than the cap is read whole: cut, reported as
//     truncated.
//  8. an upstream 4xx/5xx body (which may echo the request) reaches the
//     guest: only the status.

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testSourceKey = "k3y-0123456789abcdef0123456789abcdef"

type upstreamSeen struct {
	calls  atomic.Int32
	path   atomic.Value
	query  atomic.Value
	header atomic.Value
	body   atomic.Value
}

func newSourceUpstream(t *testing.T, status int, body string) (*httptest.Server, *upstreamSeen) {
	t.Helper()
	seen := &upstreamSeen{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.calls.Add(1)
		seen.path.Store(r.URL.EscapedPath())
		seen.query.Store(r.URL.RawQuery)
		seen.header.Store(r.Header.Clone())
		b, _ := io.ReadAll(r.Body)
		seen.body.Store(string(b))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

// sourceTestHandler builds a handler whose source-api client points every
// provider at base, with the test key loaded for each provider in keyed.
func sourceTestHandler(t *testing.T, base string, keyed []string, daily map[string]int) (*Handler, string) {
	t.Helper()
	registry := NewIdentityRegistry(time.Hour)
	h := NewHandlerWithRateLimit(registry, nil, nil)
	keys := map[string]string{}
	for _, p := range keyed {
		keys[p] = testSourceKey
	}
	h.sourceAPI = newSourceAPIClientForTest(base, keys, daily)
	cred, err := registry.IssueCredential("computer-source-api")
	if err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	return h, cred.RawToken
}

func callSourceAPI(h *Handler, token string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/source-api", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.HandleSourceAPI(w, req)
	return w
}

func captureGatewayLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func allProviders() []string {
	return []string{"fred", "congress", "courtlistener", "youtube", "newsapi_ai"}
}

func TestSourceAPIRequiresGatewayToken(t *testing.T) {
	srv, seen := newSourceUpstream(t, 200, `{}`)
	h, _ := sourceTestHandler(t, srv.URL, allProviders(), nil)
	for _, token := range []string{"", "not-a-token"} {
		w := callSourceAPI(h, token, `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE"}}`)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("token %q: status %d, want 401", token, w.Code)
		}
	}
	if seen.calls.Load() != 0 {
		t.Fatalf("upstream called without a valid gateway token")
	}
}

func TestSourceAPIRefusesUnknownOrUnsafeRequests(t *testing.T) {
	srv, seen := newSourceUpstream(t, 200, `{}`)
	h, token := sourceTestHandler(t, srv.URL, allProviders(), nil)
	cases := map[string]string{
		"bad json":            `{`,
		"unknown provider":    `{"provider":"evil","endpoint":"x","params":{}}`,
		"unknown endpoint":    `{"provider":"fred","endpoint":"anything","params":{}}`,
		"unknown parameter":   `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE","api_key":"mine"}}`,
		"missing required":    `{"provider":"fred","endpoint":"series","params":{}}`,
		"traversal in id":     `{"provider":"courtlistener","endpoint":"docket","params":{"id":"../../admin"}}`,
		"slash in path param": `{"provider":"congress","endpoint":"bill","params":{"congress":119,"type":"hr/../../x","number":1}}`,
		"query in id":         `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE&api_key=x"}}`,
		"url as id":           `{"provider":"fred","endpoint":"series","params":{"series_id":"https://evil.example/"}}`,
		"int out of range":    `{"provider":"fred","endpoint":"series_observations","params":{"series_id":"UNRATE","limit":100000}}`,
		"int as text":         `{"provider":"congress","endpoint":"bill","params":{"congress":"119/x","type":"hr","number":1}}`,
		"bad enum":            `{"provider":"congress","endpoint":"bill","params":{"congress":119,"type":"zz","number":1}}`,
		"bad date":            `{"provider":"fred","endpoint":"series_observations","params":{"series_id":"UNRATE","observation_start":"yesterday"}}`,
		"control char text":   `{"provider":"courtlistener","endpoint":"search","params":{"q":"a\u0000b"}}`,
		"too many video ids":  `{"provider":"youtube","endpoint":"videos","params":{"id":"` + strings.TrimSuffix(strings.Repeat("jNQXAC9IVRw,", 51), ",") + `"}}`,
		"bad video id":        `{"provider":"youtube","endpoint":"videos","params":{"id":"jNQXAC9IVRw,../x"}}`,
	}
	for name, body := range cases {
		w := callSourceAPI(h, token, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d, want 400 (%s)", name, w.Code, w.Body.String())
		}
	}
	if n := seen.calls.Load(); n != 0 {
		t.Fatalf("upstream called %d times for refused requests", n)
	}
}

func TestSourceAPIPlacesKeyOnHostAndNeverReturnsIt(t *testing.T) {
	cases := []struct {
		name, body, wantPath string
		check                func(t *testing.T, s *upstreamSeen)
	}{
		{"fred query key", `{"provider":"fred","endpoint":"series_observations","params":{"series_id":"UNRATE","limit":3,"sort_order":"desc"}}`,
			"/fred/series/observations", func(t *testing.T, s *upstreamSeen) {
				q := s.query.Load().(string)
				for _, want := range []string{"api_key=" + testSourceKey, "file_type=json", "series_id=UNRATE", "limit=3", "sort_order=desc"} {
					if !strings.Contains(q, want) {
						t.Fatalf("query %q lacks %q", strings.ReplaceAll(q, testSourceKey, "[key]"), strings.ReplaceAll(want, testSourceKey, "[key]"))
					}
				}
			}},
		{"congress header key", `{"provider":"congress","endpoint":"bill_actions","params":{"congress":119,"type":"hr","number":1,"limit":5}}`,
			"/v3/bill/119/hr/1/actions", func(t *testing.T, s *upstreamSeen) {
				if s.header.Load().(http.Header).Get("X-Api-Key") != testSourceKey {
					t.Fatalf("congress key not in X-Api-Key")
				}
				if strings.Contains(s.query.Load().(string), testSourceKey) {
					t.Fatalf("congress key in the query")
				}
			}},
		{"courtlistener token header", `{"provider":"courtlistener","endpoint":"docket","params":{"id":4214664}}`,
			"/api/rest/v4/dockets/4214664/", func(t *testing.T, s *upstreamSeen) {
				if s.header.Load().(http.Header).Get("Authorization") != "Token "+testSourceKey {
					t.Fatalf("courtlistener key not in Authorization: Token")
				}
			}},
		{"youtube header key", `{"provider":"youtube","endpoint":"videos","params":{"id":"jNQXAC9IVRw,dQw4w9WgXcQ"}}`,
			"/youtube/v3/videos", func(t *testing.T, s *upstreamSeen) {
				if s.header.Load().(http.Header).Get("X-Goog-Api-Key") != testSourceKey {
					t.Fatalf("youtube key not in X-Goog-Api-Key")
				}
				if q := s.query.Load().(string); !strings.Contains(q, "id=jNQXAC9IVRw%2CdQw4w9WgXcQ") || !strings.Contains(q, "part=") {
					t.Fatalf("youtube query %q", q)
				}
			}},
		{"newsapi body key", `{"provider":"newsapi_ai","endpoint":"articles","params":{"keyword":"Taiwan","lang":"zho","count":5}}`,
			"/api/v1/article/getArticles", func(t *testing.T, s *upstreamSeen) {
				var b map[string]any
				if err := json.Unmarshal([]byte(s.body.Load().(string)), &b); err != nil {
					t.Fatalf("newsapi body not JSON: %v", err)
				}
				if b["apiKey"] != testSourceKey || b["keyword"] != "Taiwan" || b["lang"] != "zho" || b["articlesCount"] != float64(5) {
					t.Fatalf("newsapi body fields wrong")
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureGatewayLog(t)
			// The upstream echoes the key, as a careless API might.
			srv, seen := newSourceUpstream(t, 200, `{"echo":"`+testSourceKey+`","ok":true}`)
			h, token := sourceTestHandler(t, srv.URL, allProviders(), nil)
			w := callSourceAPI(h, token, tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if seen.calls.Load() != 1 {
				t.Fatalf("upstream calls = %d, want 1", seen.calls.Load())
			}
			if got := seen.path.Load().(string); got != tc.wantPath {
				t.Fatalf("upstream path %q, want %q", got, tc.wantPath)
			}
			tc.check(t, seen)
			if strings.Contains(w.Body.String(), testSourceKey) {
				t.Fatalf("key reached the guest response")
			}
			if strings.Contains(logs.String(), testSourceKey) {
				t.Fatalf("key reached the log")
			}
			var env SourceAPIResponse
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("response not JSON: %v", err)
			}
			if env.UpstreamStatus != 200 || env.Truncated || !strings.Contains(string(env.Body), `"ok":true`) {
				t.Fatalf("envelope %+v", env)
			}
		})
	}
}

func TestSourceAPIProviderWithoutKeyIsUnavailable(t *testing.T) {
	srv, seen := newSourceUpstream(t, 200, `{}`)
	h, token := sourceTestHandler(t, srv.URL, []string{"congress"}, nil)
	w := callSourceAPI(h, token, `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE"}}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", w.Code)
	}
	if seen.calls.Load() != 0 {
		t.Fatalf("upstream called for a provider with no key")
	}
}

func TestSourceAPIDailyBudget(t *testing.T) {
	srv, seen := newSourceUpstream(t, 200, `{}`)
	h, token := sourceTestHandler(t, srv.URL, allProviders(), map[string]int{"newsapi_ai": 2})
	body := `{"provider":"newsapi_ai","endpoint":"articles","params":{"keyword":"x"}}`
	for i := 0; i < 2; i++ {
		if w := callSourceAPI(h, token, body); w.Code != http.StatusOK {
			t.Fatalf("call %d: status %d", i, w.Code)
		}
	}
	w := callSourceAPI(h, token, body)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over budget: status %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatalf("429 without Retry-After")
	}
	if seen.calls.Load() != 2 {
		t.Fatalf("upstream calls = %d, want 2", seen.calls.Load())
	}
	// The usage endpoint costs nothing and is not counted.
	if w := callSourceAPI(h, token, `{"provider":"newsapi_ai","endpoint":"usage","params":{}}`); w.Code != http.StatusOK {
		t.Fatalf("usage over budget: status %d, want 200", w.Code)
	}
	// Another provider has its own budget.
	if w := callSourceAPI(h, token, `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE"}}`); w.Code != http.StatusOK {
		t.Fatalf("fred: status %d", w.Code)
	}
}

func TestSourceAPITruncatesLargeBody(t *testing.T) {
	big := `{"x":"` + strings.Repeat("a", sourceAPIMaxBody+1024) + `"}`
	srv, _ := newSourceUpstream(t, 200, big)
	h, token := sourceTestHandler(t, srv.URL, allProviders(), nil)
	w := callSourceAPI(h, token, `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE"}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var env SourceAPIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if !env.Truncated || len(env.Text) > sourceAPIMaxBody || len(env.Body) != 0 {
		t.Fatalf("truncated=%v text=%d body=%d", env.Truncated, len(env.Text), len(env.Body))
	}
}

func TestSourceAPIUpstreamErrorPassesOnlyStatus(t *testing.T) {
	logs := captureGatewayLog(t)
	srv, _ := newSourceUpstream(t, 400, `{"error":"bad request for api_key=`+testSourceKey+` details"}`)
	h, token := sourceTestHandler(t, srv.URL, allProviders(), nil)
	w := callSourceAPI(h, token, `{"provider":"fred","endpoint":"series","params":{"series_id":"NOPE"}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 envelope", w.Code)
	}
	var env SourceAPIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if env.UpstreamStatus != 400 || len(env.Body) != 0 || env.Text != "" || env.Error == "" {
		t.Fatalf("envelope %+v", env)
	}
	if strings.Contains(w.Body.String(), "details") || strings.Contains(w.Body.String(), testSourceKey) {
		t.Fatalf("upstream error body reached the guest: %s", w.Body.String())
	}
	if strings.Contains(logs.String(), testSourceKey) {
		t.Fatalf("key reached the log")
	}
}

func TestSourceAPIUnreachableUpstreamHidesURL(t *testing.T) {
	logs := captureGatewayLog(t)
	srv, _ := newSourceUpstream(t, 200, `{}`)
	base := srv.URL
	srv.Close()
	h, token := sourceTestHandler(t, base, allProviders(), nil)
	w := callSourceAPI(h, token, `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE"}}`)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status %d, want 502", w.Code)
	}
	if strings.Contains(w.Body.String(), testSourceKey) || strings.Contains(logs.String(), testSourceKey) {
		t.Fatalf("key leaked through a transport error")
	}
}

func TestParseSourceAPIDaily(t *testing.T) {
	got := parseSourceAPIDaily("fred=10, newsapi_ai=0 ,bogus, youtube=-3,congress=x")
	if got["fred"] != 10 || got["newsapi_ai"] != 0 {
		t.Fatalf("parsed %v", got)
	}
	if _, ok := got["youtube"]; ok {
		t.Fatalf("negative budget accepted")
	}
	if _, ok := got["congress"]; ok {
		t.Fatalf("non-numeric budget accepted")
	}
}

func TestSourceAPIStatusNamesProvidersNotKeys(t *testing.T) {
	c := newSourceAPIClientForTest("http://127.0.0.1:1", map[string]string{"fred": testSourceKey}, nil)
	s := c.Status()
	if strings.Contains(s, testSourceKey) || !strings.Contains(s, "fred: key loaded, 2000 a day") || !strings.Contains(s, "youtube: no key") {
		t.Fatalf("Status = %q", strings.ReplaceAll(s, testSourceKey, "[key]"))
	}
}
