package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
)

// CeramicProvider (docs/reference/CERAMIC.md; mutation red: gateway provider
// calls). Failure list, written before the code:
//  1. The request is not what Ceramic accepts: wrong method, path or
//     Authorization, or a body with a field other than query, maxResults and
//     maxDescriptionLength (the body admits no unknown fields), or
//     maxResults outside 1..20, or maxDescriptionLength not 1000.
//  2. Results are read from the wrong place (result.results), or a result
//     with an empty URL is returned.
//  3. Query preparation sends what Ceramic rejects: a URL, a slash, a
//     number with commas, more than 50 words; or it damages what Ceramic
//     reads (quotes, OR, parentheses, site: domains, word order).
//  4. A query Ceramic cannot take (non-Latin letters, or nothing left) is
//     sent, or the provider declines it as an error that strikes it.
//  5. An error class is wrong: 402 must be quota_limited, 429 rate_limited
//     (and honour retry_after_seconds), 401 and 403 auth_error, 408 and 5xx
//     transient. A body phrase or digit must not change the class.
//  6. The API key appears in an error string, however the server echoes it.
//  7. NewSearchClient does not register ceramic when its key is set, or the
//     endpoint switch does not name it.

func ceramicServer(t *testing.T, handler http.HandlerFunc) (*CeramicProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("CERAMIC_API_KEY", "ck-test-SECRET-key-1234")
	return &CeramicProvider{endpoint: srv.URL + "/search"}, srv
}

func TestCeramicRequestShapeAndParsing(t *testing.T) { // 1, 2
	var gotMethod, gotPath, gotAuth, gotType string
	var raw map[string]any
	var rawBody string
	p, _ := ceramicServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth, gotType = r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		rawBody = string(b)
		_ = json.Unmarshal(b, &raw)
		_, _ = w.Write([]byte(`{"requestId":"r","result":{"results":[
			{"title":"One","url":"https://a.example/1","description":"first"},
			{"title":"No URL","url":"","description":"skipped"},
			{"title":"Two","url":"https://b.example/2","description":"second"}],
			"searchMetadata":{"executionTime":0.1},"totalResults":3}}`))
	})
	res, err := p.Search(context.Background(), "mars rover", 40)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotPath != "/search" || gotAuth != "Bearer ck-test-SECRET-key-1234" || gotType != "application/json" {
		t.Fatalf("request = %s %s auth=%q type=%q", gotMethod, gotPath, gotAuth, gotType)
	}
	if len(raw) != 3 || raw["query"] != "mars rover" || raw["maxResults"] != float64(20) || raw["maxDescriptionLength"] != float64(1000) {
		t.Fatalf("body = %s, want exactly query, maxResults 20 (clamped from 40) and maxDescriptionLength 1000", rawBody)
	}
	if len(res) != 2 || res[0].Title != "One" || res[0].URL != "https://a.example/1" || res[0].Snippet != "first" || res[1].URL != "https://b.example/2" {
		t.Fatalf("results = %+v, want two (the empty URL skipped)", res)
	}
}

func TestCeramicMaxResultsClamp(t *testing.T) { // 1
	for in, want := range map[int]float64{-3: 10, 0: 10, 1: 1, 7: 7, 20: 20, 21: 20, 50: 20} {
		var raw map[string]any
		p, _ := ceramicServer(t, func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&raw)
			_, _ = w.Write([]byte(`{"result":{"results":[]}}`))
		})
		if _, err := p.Search(context.Background(), "q", in); err != nil {
			t.Fatal(err)
		}
		if raw["maxResults"] != want {
			t.Errorf("maxResults %d sent as %v, want %v", in, raw["maxResults"], want)
		}
	}
}

func TestPrepareCeramicQuery(t *testing.T) { // 3, 4
	long := strings.TrimSpace(strings.Repeat("w ", 60))
	fifty := strings.TrimSpace(strings.Repeat("w ", 50))
	cases := []struct {
		name, in, want string
		ok             bool
	}{
		{"plain", "mars rover landing", "mars rover landing", true},
		{"whitespace", "  mars \t rover\n landing  ", "mars rover landing", true},
		{"scheme and path", "https://www.example.com/news/2026/story", "www.example.com news 2026 story", true},
		{"scheme in a sentence", "what does http://example.org/a say", "what does example.org a say", true},
		{"site operator keeps its domain", "inflation site:bls.gov", "inflation site:bls.gov", true},
		{"site operator with a path", "cpi site:www.bls.gov/news.release", "cpi site:www.bls.gov news.release", true},
		{"slashes", "and/or input/output", "and or input output", true},
		{"comma in a number", "revenue 101,342 dollars", "revenue 101342 dollars", true},
		{"commas in a long number", "population 1,234,567,890", "population 1234567890", true},
		{"comma in a list is kept", "apples, pears", "apples, pears", true},
		{"quotes, OR, parentheses", `"interest rate" (fed OR ecb) -crypto`, `"interest rate" (fed OR ecb) -crypto`, true},
		{"curly quotes", "“interest rate” decision", `"interest rate" decision`, true},
		{"accented Latin is Latin", "café über straße", "café über straße", true},
		{"exactly 50 words", fifty, fifty, true},
		{"cut to 50 words", long, fifty, true},
		{"cut inside a quote closes it", `"` + strings.Repeat("w ", 60), `"` + strings.Repeat("w ", 49) + `w"`, true},
		{"Cyrillic", "новости mars", "", false},
		{"Han among Latin", "mars 火星", "", false},
		{"Arabic", "مرحبا", "", false},
		{"empty", "", "", false},
		{"only spaces", "   \t ", "", false},
		{"only slashes and a scheme", "https:// / //", "", false},
	}
	for _, c := range cases {
		got, ok := prepareCeramicQuery(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: prepare(%q) = %q, %v; want %q, %v", c.name, c.in, got, ok, c.want, c.ok)
		}
		if p := (&CeramicProvider{}); p.DeclinesQuery(c.in) == c.ok {
			t.Errorf("%s: DeclinesQuery = %v, want %v", c.name, !c.ok, !c.ok)
		}
	}
}

func TestCeramicDoesNotSendADeclinedQuery(t *testing.T) { // 4
	called := false
	p, _ := ceramicServer(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	if _, err := p.Search(context.Background(), "новости", 10); err == nil {
		t.Fatal("a non-Latin query was searched")
	}
	if called {
		t.Fatal("a non-Latin query was sent")
	}
}

func TestCeramicSendsThePreparedQuery(t *testing.T) { // 3
	var raw map[string]any
	p, _ := ceramicServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		_, _ = w.Write([]byte(`{"result":{"results":[]}}`))
	})
	if _, err := p.Search(context.Background(), "https://x.example/a revenue 1,200", 5); err != nil {
		t.Fatal(err)
	}
	if raw["query"] != "x.example a revenue 1200" {
		t.Fatalf("query sent = %v", raw["query"])
	}
}

func TestCeramicErrorClasses(t *testing.T) { // 5, 6
	// Bodies echo the key and misleading phrases; none may change the class
	// or survive into the error.
	body := func(title, code string, extra string) string {
		return `{"title":"` + title + `","status":0,"detail":"your key ck-test-SECRET-key-1234 has no credits left, quota exceeded, 500 429 402","requestId":"req-402-429-500","code":"` + code + `"` + extra + `}`
	}
	cases := []struct {
		name      string
		status    int
		body      string
		header    string
		class     searchplane.OutcomeClass
		wantRetry time.Duration
	}{
		{"402 credits used up", 402, body("Payment Required", "payment_required", ""), "", searchplane.OutcomeQuotaLimited, 0},
		{"402 with an empty body", 402, ``, "", searchplane.OutcomeQuotaLimited, 0},
		{"429 plain", 429, body("Too Many Requests", "rate_limited", ""), "", searchplane.OutcomeRateLimited, 0},
		{"429 with retry_after_seconds", 429, body("Too Many Requests", "rate_limited", `,"retry_after_seconds":12`), "", searchplane.OutcomeRateLimited, 12 * time.Second},
		{"429 with a fractional retry", 429, body("Too Many Requests", "rate_limited", `,"retry_after_seconds":2.5`), "", searchplane.OutcomeRateLimited, 2500 * time.Millisecond},
		{"429 with a Retry-After header", 429, ``, "30", searchplane.OutcomeRateLimited, 30 * time.Second},
		{"401", 401, body("Unauthorized", "invalid_api_key", ""), "", searchplane.OutcomeAuthError, 0},
		{"403", 403, body("Forbidden", "forbidden", ""), "", searchplane.OutcomeAuthError, 0},
		{"408", 408, body("Request Timeout", "timeout", ""), "", "", 0}, // transient: timeout or error; see below
		{"500", 500, body("Internal Server Error", "internal", ""), "", searchplane.OutcomeServerError, 0},
		{"502", 502, ``, "", searchplane.OutcomeServerError, 0},
		{"503", 503, body("Service Unavailable", "unavailable", ""), "", searchplane.OutcomeServerError, 0},
		{"504", 504, ``, "", "", 0}, // "Gateway Timeout": read as timeout; transient either way
		{"400 is our fault, not the vendor's quota", 400, body("Bad Request", "invalid_request", ""), "", searchplane.OutcomeError, 0},
		{"422", 422, body("Unprocessable Entity", "unsupported_parameter", ""), "", searchplane.OutcomeError, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, _ := ceramicServer(t, func(w http.ResponseWriter, r *http.Request) {
				if c.header != "" {
					w.Header().Set("Retry-After", c.header)
				}
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			})
			_, err := p.Search(context.Background(), "q", 10)
			if err == nil {
				t.Fatal("no error")
			}
			class := searchplane.ClassifyCall(err, 0)
			if c.class == "" {
				if class != searchplane.OutcomeTimeout && class != searchplane.OutcomeError && class != searchplane.OutcomeServerError {
					t.Fatalf("a 408/504 classified %s, want a transient class (%v)", class, err)
				}
			} else if class != c.class {
				t.Fatalf("class = %s, want %s (%v)", class, c.class, err)
			}
			if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "ck-test") {
				t.Fatalf("error carries the key: %v", err)
			}
			status, stage := searchplane.FailureFields(err)
			if status == "-" || stage != "http_status" {
				t.Fatalf("FailureFields = %s %s for %v", status, stage, err)
			}
			var ra interface{ RetryAfter() time.Duration }
			got := time.Duration(0)
			if asRetry(err, &ra) {
				got = ra.RetryAfter()
			}
			if got != c.wantRetry {
				t.Fatalf("retry after = %v, want %v", got, c.wantRetry)
			}
		})
	}
}

func asRetry(err error, target *interface{ RetryAfter() time.Duration }) bool {
	e, ok := err.(interface{ RetryAfter() time.Duration })
	if ok {
		*target = e
	}
	return ok
}

func TestCeramicKeyNeverInTransportErrors(t *testing.T) { // 6
	p, srv := ceramicServer(t, func(w http.ResponseWriter, r *http.Request) {})
	srv.Close()
	_, err := p.Search(context.Background(), "q", 10)
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("err = %v", err)
	}
	p2, _ := ceramicServer(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{not json`)) })
	if _, err := p2.Search(context.Background(), "q", 10); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("parse err = %v", err)
	}
}

func TestCeramicIsRegisteredWithItsKey(t *testing.T) { // 7
	t.Setenv("CERAMIC_API_KEY", "")
	for _, n := range NewSearchClient().AvailableProviders() {
		if n == "ceramic" {
			t.Fatal("ceramic registered without a key")
		}
	}
	t.Setenv("CERAMIC_API_KEY", "k")
	found := false
	for _, n := range NewSearchClient().AvailableProviders() {
		found = found || n == "ceramic"
	}
	if !found {
		t.Fatal("ceramic not registered with CERAMIC_API_KEY set")
	}
	if got := searchProviderEndpoint("ceramic"); got != "https://api.ceramic.ai/search" {
		t.Fatalf("endpoint = %q", got)
	}
	var _ searchplane.QueryDecliner = searchProviderAdapter{inner: &CeramicProvider{}}
}
