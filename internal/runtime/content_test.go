//go:build comprehensive

package runtime

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestContentImportURLCreatesProvenanceRecord(t *testing.T) {
	_, handler := testAPISetup(t)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Extraction Proof</title><script>noise()</script></head><body><main><h1>Extraction Proof</h1><p>This page proves URL content import captures readable text and provenance.</p></main></body></html>`))
	}))
	defer source.Close()

	body := `{"url":` + strconvQuote(source.URL) + `,"query":"extraction proof"}`
	req := authenticatedRequest(http.MethodPost, "/api/content/import-url", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var item struct {
		ContentID   string         `json:"content_id"`
		MediaType   string         `json:"media_type"`
		AppHint     string         `json:"app_hint"`
		Title       string         `json:"title"`
		TextContent string         `json:"text_content"`
		Provenance  map[string]any `json:"provenance"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.ContentID == "" {
		t.Fatal("content_id is empty")
	}
	if item.MediaType != "text/html" {
		t.Fatalf("media_type = %q, want text/html", item.MediaType)
	}
	if item.AppHint != "browser" {
		t.Fatalf("app_hint = %q, want browser", item.AppHint)
	}
	if item.Title != "Extraction Proof" {
		t.Fatalf("title = %q", item.Title)
	}
	if !strings.Contains(item.TextContent, "readable text and provenance") {
		t.Fatalf("text_content missing extracted text: %q", item.TextContent)
	}
	if item.Provenance["hash_algorithm"] != "sha256" {
		t.Fatalf("provenance hash_algorithm = %#v", item.Provenance["hash_algorithm"])
	}
	if !provenanceHasRung(item.Provenance, "direct_http") || !provenanceHasRung(item.Provenance, "readability_lite") {
		t.Fatalf("provenance missing direct extraction rungs: %#v", item.Provenance["rungs"])
	}

	getReq := authenticatedRequest(http.MethodGet, "/api/content/items/"+item.ContentID, "", "user-content")
	getW := httptest.NewRecorder()
	handler.HandleContentItem(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getW.Code, getW.Body.String())
	}
}

func TestContentImportURLUsesSearXNGAlternateWhenPrimaryLowContent(t *testing.T) {
	_, handler := testAPISetup(t)
	alternate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Alternate Source</title></head><body><article><h1>Alternate Source</h1><p>` + strings.Repeat("This alternate source has enough article text to beat the blocked low-content original. ", 12) + `</p></article></body></html>`))
	}))
	defer alternate.Close()
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Blocked</title></head><body>blocked</body></html>`))
	}))
	defer primary.Close()
	searxng := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.URL.Query().Get("format") != "json" {
			t.Fatalf("unexpected searxng request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Alternate Source","url":` + strconvQuote(alternate.URL) + `,"content":"alternate article","engine":"test"}]}`))
	}))
	defer searxng.Close()
	t.Setenv("SEARXNG_URL", searxng.URL)

	body := `{"url":` + strconvQuote(primary.URL) + `,"query":"blocked article alternate source"}`
	req := authenticatedRequest(http.MethodPost, "/api/content/import-url", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var item struct {
		CanonicalURL string         `json:"canonical_url"`
		Title        string         `json:"title"`
		TextContent  string         `json:"text_content"`
		Metadata     map[string]any `json:"metadata"`
		Provenance   map[string]any `json:"provenance"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.CanonicalURL != alternate.URL {
		t.Fatalf("canonical_url = %q, want alternate %q", item.CanonicalURL, alternate.URL)
	}
	if item.Title != "Alternate Source" {
		t.Fatalf("title = %q", item.Title)
	}
	if !strings.Contains(item.TextContent, "beat the blocked low-content original") {
		t.Fatalf("text_content did not use alternate: %q", item.TextContent)
	}
	if item.Metadata["retrieval_strategy"] != "direct_http_readability_with_searxng_discovery" {
		t.Fatalf("retrieval_strategy = %#v", item.Metadata["retrieval_strategy"])
	}
	for _, rung := range []string{"direct_http", "readability_lite", "searxng_discovery", "searxng_alt_http", "searxng_alt_readability_lite"} {
		if !provenanceHasRung(item.Provenance, rung) {
			t.Fatalf("provenance missing rung %q: %#v", rung, item.Provenance["rungs"])
		}
	}
}

func TestContentCreateSupportsDurableMediaReferences(t *testing.T) {
	_, handler := testAPISetup(t)
	body := `{"source_type":"file","file_path":"uploads/book.epub","media_type":"application/epub+zip","title":"Book"}`
	req := authenticatedRequest(http.MethodPost, "/api/content/items", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentCreate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var item struct {
		ContentID string `json:"content_id"`
		AppHint   string `json:"app_hint"`
		FilePath  string `json:"file_path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.AppHint != "epub" {
		t.Fatalf("app_hint = %q, want epub", item.AppHint)
	}
	if item.FilePath != "uploads/book.epub" {
		t.Fatalf("file_path = %q", item.FilePath)
	}

	listReq := authenticatedRequest(http.MethodGet, "/api/content/items", "", "user-content")
	listW := httptest.NewRecorder()
	handler.HandleContentList(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listW.Code, listW.Body.String())
	}
	if !strings.Contains(listW.Body.String(), item.ContentID) {
		t.Fatalf("list response missing content id %s: %s", item.ContentID, listW.Body.String())
	}
}

func TestPromptBarBareURLRoutesToDisplayApp(t *testing.T) {
	_, handler := testAPISetup(t)
	body := `{"text":"https://example.com/report.pdf"}`
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var submitted promptBarSubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode submission: %v", err)
	}

	var status promptBarSubmissionStatusResponse
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusReq := authenticatedRequest(http.MethodGet, submitted.StatusURL, "", "user-content")
		statusW := httptest.NewRecorder()
		handler.HandlePromptBarSubmission(statusW, statusReq)
		if statusW.Code != http.StatusOK {
			t.Fatalf("status lookup = %d body=%s", statusW.Code, statusW.Body.String())
		}
		if err := json.Unmarshal(statusW.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		if status.Decision != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status.Decision == nil {
		t.Fatalf("timed out waiting for conductor decision: %#v", status)
	}
	if status.Decision.App != "pdf" {
		t.Fatalf("decision app = %q, want pdf", status.Decision.App)
	}
	if status.Decision.SourceURL != "https://example.com/report.pdf" {
		t.Fatalf("source_url = %q", status.Decision.SourceURL)
	}
}

func TestPromptBarBareURLDoesNotRequireProvider(t *testing.T) {
	rt, handler := testAPISetup(t)
	rt.provider = &StubProvider{Delay: 10 * time.Millisecond, FailErr: errors.New("provider unavailable")}
	body := `{"text":"https://example.com/report.pdf"}`
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var submitted promptBarSubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode submission: %v", err)
	}

	statusReq := authenticatedRequest(http.MethodGet, submitted.StatusURL, "", "user-content")
	statusW := httptest.NewRecorder()
	handler.HandlePromptBarSubmission(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("status lookup = %d body=%s", statusW.Code, statusW.Body.String())
	}
	var status promptBarSubmissionStatusResponse
	if err := json.Unmarshal(statusW.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.State != types.RunCompleted {
		t.Fatalf("state = %q, want completed", status.State)
	}
	if status.Decision == nil || status.Decision.App != "pdf" {
		t.Fatalf("decision = %#v, want pdf decision", status.Decision)
	}
	if strings.Contains(status.Error, "provider unavailable") {
		t.Fatalf("bare URL routing leaked provider error: %q", status.Error)
	}
}

func TestPromptBarContextualURLRoutesToVText(t *testing.T) {
	_, handler := testAPISetup(t)
	body := `{"text":"Summarize https://example.com/report.pdf for a research note"}`
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var submitted promptBarSubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode submission: %v", err)
	}

	var status promptBarSubmissionStatusResponse
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusReq := authenticatedRequest(http.MethodGet, submitted.StatusURL, "", "user-content")
		statusW := httptest.NewRecorder()
		handler.HandlePromptBarSubmission(statusW, statusReq)
		if statusW.Code != http.StatusOK {
			t.Fatalf("status lookup = %d body=%s", statusW.Code, statusW.Body.String())
		}
		if err := json.Unmarshal(statusW.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		if status.Decision != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status.Decision == nil {
		t.Fatalf("timed out waiting for conductor decision: %#v", status)
	}
	if status.Decision.App != "vtext" {
		t.Fatalf("decision app = %q, want vtext", status.Decision.App)
	}
	if status.Decision.SourceURL != "" {
		t.Fatalf("contextual URL should not be routed as bare source_url, got %q", status.Decision.SourceURL)
	}
}

func provenanceHasRung(provenance map[string]any, name string) bool {
	rungs, ok := provenance["rungs"].([]any)
	if !ok {
		return false
	}
	for _, raw := range rungs {
		rung, ok := raw.(map[string]any)
		if ok && rung["name"] == name {
			return true
		}
	}
	return false
}
