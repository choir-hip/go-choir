package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func exaTestServer(t *testing.T, respond func(w http.ResponseWriter), got *[]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if got != nil {
			*got = b
		}
		if r.Header.Get("x-api-key") != "exa_test" {
			t.Errorf("missing x-api-key")
		}
		respond(w)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func exaBody(n, textLen int) string {
	var sb strings.Builder
	sb.WriteString(`{"results":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"title":"T%d","url":"https://e.example/%d","text":%q,"publishedDate":"2026-10-14"}`, i, i, strings.Repeat("x", textLen))
	}
	sb.WriteString(`]}`)
	return sb.String()
}

// Failure mode 1: 40 results with 12,000 chars each (more than the
// bounded request asks for, and past the old 256 KiB limit) must parse.
func TestExaLargeValidResponseParses(t *testing.T) {
	t.Setenv("EXA_API_KEY", "exa_test")
	payload := exaBody(40, 12000)
	if len(payload) <= 256*1024 {
		t.Fatalf("fixture must exceed the old 256 KiB limit, got %d", len(payload))
	}
	srv := exaTestServer(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, payload) }, nil)
	results, err := (&ExaProvider{endpoint: srv.URL}).Search(t.Context(), "q", 40)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 40 || len(results[39].Snippet) != 12000 {
		t.Fatalf("results = %d", len(results))
	}
}

// Failure mode 2: a body past the read limit is a distinct truncation error,
// never a silent JSON parse error.
func TestExaTruncatedResponseIsDistinctError(t *testing.T) {
	t.Setenv("EXA_API_KEY", "exa_test")
	payload := exaBody(2, exaMaxResponseBytes) // well over the limit
	srv := exaTestServer(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, payload) }, nil)
	_, err := (&ExaProvider{endpoint: srv.URL}).Search(t.Context(), "q", 40)
	if err == nil || !strings.HasPrefix(err.Error(), "response truncated:") {
		t.Fatalf("err = %v, want response truncated", err)
	}
	if strings.Contains(err.Error(), "parse response") {
		t.Fatalf("truncation must not be reported as a parse error: %v", err)
	}
}

// Failure mode 3: a genuinely malformed (short) body still reports parse.
func TestExaMalformedResponseIsParseError(t *testing.T) {
	t.Setenv("EXA_API_KEY", "exa_test")
	srv := exaTestServer(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, `{"results":[`) }, nil)
	_, err := (&ExaProvider{endpoint: srv.URL}).Search(t.Context(), "q", 5)
	if err == nil || !strings.HasPrefix(err.Error(), "parse response:") {
		t.Fatalf("err = %v, want parse response", err)
	}
}

// The request keeps 40 results and bounds the text per result.
func TestExaRequestBoundsContents(t *testing.T) {
	t.Setenv("EXA_API_KEY", "exa_test")
	var got []byte
	srv := exaTestServer(t, func(w http.ResponseWriter) { _, _ = io.WriteString(w, `{"results":[]}`) }, &got)
	if _, err := (&ExaProvider{endpoint: srv.URL}).Search(t.Context(), "q", 40); err != nil {
		t.Fatalf("search: %v", err)
	}
	var body struct {
		NumResults int `json:"numResults"`
		Contents   struct {
			Text struct {
				MaxCharacters int `json:"maxCharacters"`
			} `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(got, &body); err != nil {
		t.Fatalf("body %s: %v", got, err)
	}
	if body.NumResults != 40 || body.Contents.Text.MaxCharacters != exaMaxTextCharacters || exaMaxTextCharacters < 2000 {
		t.Fatalf("body = %s", got)
	}
}
