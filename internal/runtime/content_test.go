//go:build comprehensive

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func allowPrivateSourceFetchForTest(t *testing.T) {
	t.Helper()
	previous := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() {
		sourcefetch.SetAllowPrivateNetworkForTests(previous)
	})
}

func TestContentImportURLCreatesProvenanceRecord(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
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
		Metadata    map[string]any `json:"metadata"`
		Provenance  map[string]any `json:"provenance"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.ContentID == "" {
		t.Fatal("content_id is empty")
	}
	if item.MediaType != "text/markdown" {
		t.Fatalf("media_type = %q, want text/markdown", item.MediaType)
	}
	if item.AppHint != "content" {
		t.Fatalf("app_hint = %q, want content", item.AppHint)
	}
	if item.Title != "Extraction Proof" {
		t.Fatalf("title = %q", item.Title)
	}
	if !strings.Contains(item.TextContent, "readable text and provenance") {
		t.Fatalf("text_content missing extracted text: %q", item.TextContent)
	}
	if item.Metadata["original_media_type"] != "text/html" || item.Metadata["reader_artifact_kind"] != "cleaned_reader_markdown" {
		t.Fatalf("metadata missing reader identity: %#v", item.Metadata)
	}
	if item.Metadata["raw_content_hash"] == "" {
		t.Fatalf("metadata missing raw_content_hash: %#v", item.Metadata)
	}
	if item.Metadata["extraction_adapter"] != "html_readability_lite" {
		t.Fatalf("metadata extraction_adapter = %#v", item.Metadata["extraction_adapter"])
	}
	if item.Metadata["extracted_text_hash"] == "" {
		t.Fatalf("metadata missing extracted_text_hash: %#v", item.Metadata)
	}
	if count, _ := item.Metadata["selector_count"].(float64); count < 1 {
		t.Fatalf("metadata missing selectors: %#v", item.Metadata)
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

func TestContentImportURLCreatesPlainTextSelectors(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	_, handler := testAPISetup(t)
	text := strings.Repeat("RFC-style public text evidence for selector chunking and recall pressure.\n", 600)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(text))
	}))
	defer source.Close()

	body := `{"url":` + strconvQuote(source.URL) + `}`
	req := authenticatedRequest(http.MethodPost, "/api/content/import-url", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var item struct {
		MediaType   string         `json:"media_type"`
		AppHint     string         `json:"app_hint"`
		TextContent string         `json:"text_content"`
		ContentHash string         `json:"content_hash"`
		Metadata    map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.MediaType != "text/plain" {
		t.Fatalf("media_type = %q, want text/plain", item.MediaType)
	}
	if !strings.Contains(item.TextContent, "selector chunking") {
		t.Fatalf("text_content missing imported text")
	}
	if item.Metadata["extraction_adapter"] != "plain_text_decode" {
		t.Fatalf("extraction_adapter = %#v", item.Metadata["extraction_adapter"])
	}
	if item.Metadata["raw_content_hash"] == "" || item.Metadata["extracted_text_hash"] == "" {
		t.Fatalf("hash metadata missing: %#v", item.Metadata)
	}
	if item.ContentHash != strings.TrimPrefix(stringMapValue(item.Metadata, "raw_content_hash"), "sha256:") {
		t.Fatalf("content_hash = %q, raw metadata = %#v", item.ContentHash, item.Metadata["raw_content_hash"])
	}
	if count, _ := item.Metadata["selector_count"].(float64); count < 2 {
		t.Fatalf("selector_count = %#v, want chunk selectors", item.Metadata["selector_count"])
	}
	selectorsRaw, ok := item.Metadata["selectors"].([]any)
	if !ok || len(selectorsRaw) < 2 {
		t.Fatalf("selectors missing: %#v", item.Metadata["selectors"])
	}
}

func TestContentImportURLCleansReaderChrome(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	_, handler := testAPISetup(t)
	article := strings.Repeat("This source paragraph is durable article evidence about private cloud infrastructure, jurisdiction, hosting controls, and operational reliability. ", 8)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Noisy Reader Source</title></head><body>
			<div id="cookie-banner"><h2>This website uses cookies</h2><button>Allow all cookies</button><button>Allow necessary cookies</button></div>
			<header><nav>Community Jobs About us Login Console</nav></header>
			<div class="location-selector"><p>Choose your location settings</p><label>Language</label><select><option>English</option></select><button>Save settings</button></div>
			<main>
				<h1>Noisy Reader Source</h1>
				<form role="search"><label>Search for:</label><input value="query"></form>
				<article><p>` + article + `</p></article>
			</main>
			<footer>Privacy policy and footer links</footer>
		</body></html>`))
	}))
	defer source.Close()

	body := `{"url":` + strconvQuote(source.URL) + `,"query":"noisy reader source"}`
	req := authenticatedRequest(http.MethodPost, "/api/content/import-url", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var item struct {
		MediaType   string `json:"media_type"`
		AppHint     string `json:"app_hint"`
		TextContent string `json:"text_content"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, noisy := range []string{"This website uses cookies", "Allow all cookies", "Choose your location settings", "Community Jobs", "Search for:", "Privacy policy and footer"} {
		if strings.Contains(item.TextContent, noisy) {
			t.Fatalf("text_content leaked reader chrome %q: %q", noisy, item.TextContent)
		}
	}
	if !strings.Contains(item.TextContent, "durable article evidence about private cloud infrastructure") {
		t.Fatalf("text_content missing article evidence: %q", item.TextContent)
	}
	if item.MediaType != "text/markdown" || item.AppHint != "content" {
		t.Fatalf("reader artifact identity = %s/%s, want text/markdown/content", item.MediaType, item.AppHint)
	}
}

func TestContentImportURLUsesSearXNGAlternateWhenPrimaryLowContent(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
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
		MediaType    string         `json:"media_type"`
		AppHint      string         `json:"app_hint"`
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
	if item.MediaType != "text/markdown" || item.AppHint != "content" {
		t.Fatalf("alternate reader identity = %s/%s, want text/markdown/content", item.MediaType, item.AppHint)
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

func TestContentImportURLRefreshesEmptyExistingReadableItem(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	rt, handler := testAPISetup(t)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Refreshed Source</title></head><body><main><h1>Refreshed Source</h1><p>This refreshed source has readable body text after an older empty import.</p></main></body></html>`))
	}))
	defer source.Close()

	now := time.Now().UTC()
	if err := rt.Store().CreateContentItem(context.Background(), types.ContentItem{
		ContentID:    "empty-existing",
		OwnerID:      "user-content",
		SourceType:   "extracted_url",
		MediaType:    "text/html",
		AppHint:      "browser",
		Title:        "Empty Existing",
		SourceURL:    source.URL,
		CanonicalURL: source.URL,
		TextContent:  "",
		ContentHash:  contentHash(""),
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("seed empty content item: %v", err)
	}

	body := `{"url":` + strconvQuote(source.URL) + `,"query":"refreshed source"}`
	req := authenticatedRequest(http.MethodPost, "/api/content/import-url", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var item struct {
		ContentID   string `json:"content_id"`
		MediaType   string `json:"media_type"`
		AppHint     string `json:"app_hint"`
		Title       string `json:"title"`
		TextContent string `json:"text_content"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.ContentID == "empty-existing" {
		t.Fatalf("empty existing content item was reused")
	}
	if item.MediaType != "text/markdown" || item.AppHint != "content" {
		t.Fatalf("refreshed reader identity = %s/%s, want text/markdown/content", item.MediaType, item.AppHint)
	}
	if item.Title != "Refreshed Source" || !strings.Contains(item.TextContent, "readable body text") {
		t.Fatalf("refreshed import = %#v", item)
	}
}

func TestContentImportURLRefreshesLegacyBrowserIdentityReaderItem(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	rt, handler := testAPISetup(t)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Reader Identity Source</title></head><body><main><p>` + strings.Repeat("This source now has durable reader text for citation source windows. ", 10) + `</p></main></body></html>`))
	}))
	defer source.Close()

	now := time.Now().UTC()
	if err := rt.Store().CreateContentItem(context.Background(), types.ContentItem{
		ContentID:    "legacy-browser-readable",
		OwnerID:      "user-content",
		SourceType:   "extracted_url",
		MediaType:    "text/html",
		AppHint:      "browser",
		Title:        "Legacy Browser Readable",
		SourceURL:    source.URL,
		CanonicalURL: source.URL,
		TextContent:  "Old readable text stored with browser identity.",
		ContentHash:  contentHash("Old readable text stored with browser identity."),
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("seed legacy content item: %v", err)
	}

	body := `{"url":` + strconvQuote(source.URL) + `,"query":"reader identity source"}`
	req := authenticatedRequest(http.MethodPost, "/api/content/import-url", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var item struct {
		ContentID string         `json:"content_id"`
		MediaType string         `json:"media_type"`
		AppHint   string         `json:"app_hint"`
		Metadata  map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.ContentID == "legacy-browser-readable" {
		t.Fatalf("legacy browser-identity source item was reused")
	}
	if item.MediaType != "text/markdown" || item.AppHint != "content" {
		t.Fatalf("reader identity = %s/%s, want text/markdown/content", item.MediaType, item.AppHint)
	}
	if item.Metadata["original_media_type"] != "text/html" || item.Metadata["reader_artifact_kind"] != "cleaned_reader_markdown" {
		t.Fatalf("metadata missing original reader identity: %#v", item.Metadata)
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

	readerBody := `{"source_type":"text","media_type":"text/markdown","app_hint":"content","title":"Reader Source","text_content":"Durable source reader text."}`
	readerReq := authenticatedRequest(http.MethodPost, "/api/content/items", readerBody, "user-content")
	readerW := httptest.NewRecorder()
	handler.HandleContentCreate(readerW, readerReq)
	if readerW.Code != http.StatusCreated {
		t.Fatalf("reader status = %d body=%s", readerW.Code, readerW.Body.String())
	}
	var reader struct {
		AppHint string `json:"app_hint"`
	}
	if err := json.Unmarshal(readerW.Body.Bytes(), &reader); err != nil {
		t.Fatalf("decode reader response: %v", err)
	}
	if reader.AppHint != "content" {
		t.Fatalf("manual reader app_hint = %q, want content", reader.AppHint)
	}
}

func TestContentImportFileCreatesExtractedPPTXContentItem(t *testing.T) {
	rt, handler := testAPISetup(t)
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)
	importsDir := filepath.Join(filesRoot, "imports")
	if err := os.MkdirAll(importsDir, 0o755); err != nil {
		t.Fatalf("create imports dir: %v", err)
	}
	pptxBytes := buildMinimalPPTX(t, []string{
		"Frozen corpus slide one",
		"Second slide contains exact recall marker ALPHA-42",
	})
	if err := os.WriteFile(filepath.Join(importsDir, "deck.pptx"), pptxBytes, 0o644); err != nil {
		t.Fatalf("write pptx: %v", err)
	}

	req := authenticatedRequest(http.MethodPost, "/api/content/import-file", `{"file_path":"imports/deck.pptx"}`, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportFile(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var item types.ContentItem
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if item.SourceType != "file" || item.FilePath != "imports/deck.pptx" {
		t.Fatalf("file identity = %s %s", item.SourceType, item.FilePath)
	}
	if item.MediaType != "application/vnd.openxmlformats-officedocument.presentationml.presentation" || item.AppHint != "slides" {
		t.Fatalf("presentation identity = %s/%s", item.MediaType, item.AppHint)
	}
	if item.ContentHash != contentHashBytes(pptxBytes) {
		t.Fatalf("content_hash = %q, want raw hash", item.ContentHash)
	}
	if !strings.Contains(item.TextContent, "Frozen corpus slide one") || !strings.Contains(item.TextContent, "ALPHA-42") {
		t.Fatalf("text_content missing slide text: %q", item.TextContent)
	}
	metadata := map[string]any{}
	if err := json.Unmarshal(item.Metadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["extraction_adapter"] != "pptx_ooxml_slide_text_projection" {
		t.Fatalf("extraction_adapter = %#v", metadata["extraction_adapter"])
	}
	if metadata["raw_content_hash"] != "sha256:"+contentHashBytes(pptxBytes) {
		t.Fatalf("raw_content_hash = %#v", metadata["raw_content_hash"])
	}
	if metadata["extracted_text_hash"] == "" {
		t.Fatalf("extracted_text_hash missing: %#v", metadata)
	}
	selectors := selectorsFromContentMetadata(item.Metadata)
	if len(selectors) != 2 || selectors[0].ID != "slide-1" || selectors[1].ID != "slide-2" {
		t.Fatalf("selectors = %#v", selectors)
	}
	stored, err := rt.Store().GetContentItem(context.Background(), "user-content", item.ContentID)
	if err != nil {
		t.Fatalf("load stored content item: %v", err)
	}
	if stored.ContentID != item.ContentID || !strings.Contains(stored.TextContent, "ALPHA-42") {
		t.Fatalf("stored item = %#v", stored)
	}
}

func TestContentImportURLDedupesYouTubeSourcePackets(t *testing.T) {
	t.Setenv("CHOIR_DISABLE_YOUTUBE_TRANSCRIPT_FETCH", "1")
	_, handler := testAPISetup(t)
	body := `{"url":"https://youtu.be/dQw4w9WgXcQ?si=test"}`
	req := authenticatedRequest(http.MethodPost, "/api/content/import-url", body, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first status = %d body=%s", w.Code, w.Body.String())
	}
	var first struct {
		ContentID    string         `json:"content_id"`
		MediaType    string         `json:"media_type"`
		AppHint      string         `json:"app_hint"`
		CanonicalURL string         `json:"canonical_url"`
		Metadata     map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if first.MediaType != "video/youtube" || first.AppHint != "video" {
		t.Fatalf("media/app = %q/%q, want video/youtube/video", first.MediaType, first.AppHint)
	}
	if first.CanonicalURL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Fatalf("canonical_url = %q", first.CanonicalURL)
	}
	if first.Metadata["transcript_availability"] != "unavailable" {
		t.Fatalf("transcript availability = %#v", first.Metadata["transcript_availability"])
	}

	req = authenticatedRequest(http.MethodPost, "/api/content/import-url", `{"url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}`, "user-content")
	w = httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("second status = %d body=%s", w.Code, w.Body.String())
	}
	var second struct {
		ContentID string `json:"content_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if second.ContentID != first.ContentID {
		t.Fatalf("duplicate import content_id = %q, want existing %q", second.ContentID, first.ContentID)
	}
}

func TestYouTubeJSON3CaptionURLForcesFormat(t *testing.T) {
	got := youtubeJSON3CaptionURL("https://www.youtube.com/api/timedtext?v=abc&lang=en&fmt=srv3")
	if !strings.Contains(got, "fmt=json3") {
		t.Fatalf("caption URL = %q, want fmt=json3", got)
	}
	if strings.Contains(got, "fmt=srv3") {
		t.Fatalf("caption URL retained stale fmt: %q", got)
	}
}

func TestFetchYouTubeTranscriptUsesConfiguredProvider(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	t.Setenv("CHOIR_YOUTUBE_TRANSCRIPT_PROVIDER", "gettranscript")
	t.Setenv("CHOIR_YOUTUBE_TRANSCRIPT_API_KEY", "secret")
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("videoId") != "jNQXAC9IVRw" {
			t.Fatalf("videoId query = %q", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"language": "en",
			"kind": "manual",
			"segments": [
				{"start": 0.0, "duration": 1.2, "text": "First line."},
				{"start": 1.2, "duration": 2.4, "text": "Second line."}
			]
		}`))
	}))
	defer source.Close()
	t.Setenv("CHOIR_YOUTUBE_TRANSCRIPT_API_URL", source.URL)

	got := fetchYouTubeTranscript(context.Background(), "jNQXAC9IVRw")
	if got.Availability != "available" || got.Provider != "gettranscript" {
		t.Fatalf("availability/provider = %q/%q error=%q", got.Availability, got.Provider, got.Error)
	}
	if got.Language != "en" || got.Kind != "manual" {
		t.Fatalf("language/kind = %q/%q", got.Language, got.Kind)
	}
	if len(got.Segments) != 2 || got.Segments[1].Start != 1.2 {
		t.Fatalf("segments = %#v", got.Segments)
	}
	if got.Text != "First line.\nSecond line." {
		t.Fatalf("text = %q", got.Text)
	}
}

func TestContentImportURLStoresConfiguredTranscriptItem(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	t.Setenv("CHOIR_YOUTUBE_TRANSCRIPT_PROVIDER", "gettranscript")
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("videoId") != "jNQXAC9IVRw" {
			t.Fatalf("videoId query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"language": "en",
			"segments": [{"start": 0, "duration": 2, "text": "Stored transcript line."}]
		}`))
	}))
	defer source.Close()
	t.Setenv("CHOIR_YOUTUBE_TRANSCRIPT_API_URL", source.URL)

	_, handler := testAPISetup(t)
	req := authenticatedRequest(http.MethodPost, "/api/content/import-url", `{"url":"https://youtu.be/jNQXAC9IVRw"}`, "user-content")
	w := httptest.NewRecorder()
	handler.HandleContentImportURL(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var video struct {
		MediaType string         `json:"media_type"`
		Metadata  map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &video); err != nil {
		t.Fatalf("decode video response: %v", err)
	}
	if video.MediaType != "video/youtube" || video.Metadata["transcript_availability"] != "available" {
		t.Fatalf("video response = %#v", video)
	}
	transcriptID, _ := video.Metadata["transcript_content_id"].(string)
	if transcriptID == "" {
		t.Fatalf("missing transcript_content_id: %#v", video.Metadata)
	}

	req = authenticatedRequest(http.MethodGet, "/api/content/items/"+transcriptID, "", "user-content")
	w = httptest.NewRecorder()
	handler.HandleContentItem(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get transcript status = %d body=%s", w.Code, w.Body.String())
	}
	var transcript struct {
		SourceType  string         `json:"source_type"`
		MediaType   string         `json:"media_type"`
		TextContent string         `json:"text_content"`
		Metadata    map[string]any `json:"metadata"`
		Provenance  map[string]any `json:"provenance"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &transcript); err != nil {
		t.Fatalf("decode transcript: %v", err)
	}
	if transcript.SourceType != "derived_transcript" || transcript.MediaType != "text/x-youtube-transcript" {
		t.Fatalf("transcript item type = %q/%q", transcript.SourceType, transcript.MediaType)
	}
	if transcript.TextContent != "Stored transcript line." {
		t.Fatalf("transcript text = %q", transcript.TextContent)
	}
	if transcript.Metadata["availability"] != "available" || transcript.Metadata["provider"] != "gettranscript" {
		t.Fatalf("transcript metadata = %#v", transcript.Metadata)
	}
	if transcript.Provenance["rights_scope"] != "private_user_source" || transcript.Provenance["untrusted_source_text"] != true {
		t.Fatalf("transcript provenance = %#v", transcript.Provenance)
	}
}

func TestFetchYouTubeTranscriptUsesInnerTubeAndroidFallback(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	var source *httptest.Server
	source = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/player":
			if r.Method != http.MethodPost {
				t.Fatalf("player method = %s", r.Method)
			}
			if got := r.Header.Get("X-YouTube-Client-Name"); got != "3" {
				t.Fatalf("client name header = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"playabilityStatus": {"status": "OK"},
				"captions": {"playerCaptionsTracklistRenderer": {"captionTracks": [
					{"baseUrl": "` + source.URL + `/caption?fmt=srv3&lang=en", "languageCode": "en", "kind": ""}
				]}}
			}`))
		case "/caption":
			if r.URL.Query().Get("fmt") != "json3" {
				t.Fatalf("caption fmt = %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"events":[{"tStartMs":1000,"dDurationMs":2500,"segs":[{"utf8":"InnerTube line."}]}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer source.Close()
	t.Setenv("CHOIR_YOUTUBE_INNERTUBE_PLAYER_URL", source.URL+"/player")

	got := fetchYouTubeTranscript(context.Background(), "dQw4w9WgXcQ")
	if got.Availability != "available" || got.Provider != "youtube_innertube_android" {
		t.Fatalf("availability/provider = %q/%q error=%q", got.Availability, got.Provider, got.Error)
	}
	if got.Language != "en" || got.Kind != "caption" {
		t.Fatalf("language/kind = %q/%q", got.Language, got.Kind)
	}
	if len(got.Segments) != 1 || got.Segments[0].Start != 1 || got.Segments[0].Duration != 2.5 {
		t.Fatalf("segments = %#v", got.Segments)
	}
	if got.Text != "InnerTube line." {
		t.Fatalf("text = %q", got.Text)
	}
}

func TestFetchConfiguredYouTubeTranscriptRejectsForbiddenProviderURL(t *testing.T) {
	got := fetchConfiguredYouTubeTranscript(context.Background(), "jNQXAC9IVRw", youtubeTranscriptProviderConfig{
		Name:    "generic",
		BaseURL: "http://127.0.0.1:1/transcript",
	})
	if got.Availability != "unavailable" {
		t.Fatalf("availability = %q, want unavailable", got.Availability)
	}
	if !strings.Contains(got.Error, "source URL host is not allowed") {
		t.Fatalf("error = %q, want source fetch policy rejection", got.Error)
	}
}

func TestFetchYouTubeTranscriptFromInnerTubeRejectsForbiddenPlayerURL(t *testing.T) {
	t.Setenv("CHOIR_YOUTUBE_INNERTUBE_PLAYER_URL", "http://127.0.0.1:1/player")
	got := fetchYouTubeTranscriptFromInnerTube(context.Background(), "dQw4w9WgXcQ")
	if got.Availability != "unavailable" {
		t.Fatalf("availability = %q, want unavailable", got.Availability)
	}
	if !strings.Contains(got.Error, "source URL host is not allowed") {
		t.Fatalf("error = %q, want source fetch policy rejection", got.Error)
	}
}

func TestFetchYouTubeTranscriptFromInnerTubeRejectsForbiddenCaptionURL(t *testing.T) {
	previous := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() {
		sourcefetch.SetAllowPrivateNetworkForTests(previous)
	})
	var source *httptest.Server
	source = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/player":
			sourcefetch.SetAllowPrivateNetworkForTests(false)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"playabilityStatus": {"status": "OK"},
				"captions": {"playerCaptionsTracklistRenderer": {"captionTracks": [
					{"baseUrl": "` + source.URL + `/caption?fmt=srv3&lang=en", "languageCode": "en", "kind": ""}
				]}}
			}`))
		case "/caption":
			t.Fatal("forbidden caption URL should not be requested")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer source.Close()
	t.Setenv("CHOIR_YOUTUBE_INNERTUBE_PLAYER_URL", source.URL+"/player")

	got := fetchYouTubeTranscriptFromInnerTube(context.Background(), "dQw4w9WgXcQ")
	if got.Availability != "unavailable" {
		t.Fatalf("availability = %q, want unavailable", got.Availability)
	}
	if !strings.Contains(got.Error, "source URL host is not allowed") {
		t.Fatalf("error = %q, want source fetch policy rejection", got.Error)
	}
}

func TestChooseYouTubeCaptionTrackPrefersHumanEnglish(t *testing.T) {
	got, ok := chooseYouTubeCaptionTrack([]youtubeCaptionTrack{
		{BaseURL: "first", LanguageCode: "es"},
		{BaseURL: "auto", LanguageCode: "en", Kind: "asr"},
		{BaseURL: "manual", LanguageCode: "en"},
	})
	if !ok || got.BaseURL != "manual" {
		t.Fatalf("track = %#v ok=%v, want manual English", got, ok)
	}
}

func TestParseYouTubeTranscriptProviderPayloadHandlesNestedTranscript(t *testing.T) {
	raw := []byte(`{
		"data": [{
			"video_id": "abc123",
			"lang": "en",
			"transcript": [
				{"offset": "3.5", "duration": "1.0", "text": "Nested line one."},
				{"offset": "4.5", "duration": "1.5", "text": "Nested line two."}
			]
		}]
	}`)
	segments, text, language, _, err := parseYouTubeTranscriptProviderPayload(raw, "abc123")
	if err != nil {
		t.Fatalf("parse provider payload: %v", err)
	}
	if language != "en" {
		t.Fatalf("language = %q", language)
	}
	if len(segments) != 2 || segments[0].Start != 3.5 || segments[1].Duration != 1.5 {
		t.Fatalf("segments = %#v", segments)
	}
	if text != "Nested line one.\nNested line two." {
		t.Fatalf("text = %q", text)
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
