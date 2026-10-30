package proxy

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

func TestPlatformPublicationResolveIsPublicAndInternalOnly(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var gotInternal, gotView string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInternal = r.Header.Get("X-Internal-Caller")
		switch r.URL.Path {
		case "/internal/platform/publications/resolve":
			if r.URL.Query().Get("route") != "/pub/texture/test" {
				t.Fatalf("platform route query: %s", r.URL.RawQuery)
			}
			gotView = r.URL.Query().Get("view")
			_ = json.NewEncoder(w).Encode(platform.PublicationBundle{
				Route:       platform.PublicationRoute{Path: "/pub/texture/test", State: "active"},
				Publication: platform.PublicationSummary{ID: "pub-1", Title: "Public"},
				Version:     platform.PublicationVersionSummary{ID: "pubver-1", ContentHash: "hash", SourceRevisionHash: "source-hash"},
				Artifact:    platform.PublicationArtifact{Content: "public content"},
			})
		case "/internal/platform/publications/export":
			if r.URL.Query().Get("route") != "/pub/texture/test" || r.URL.Query().Get("format") != "md" {
				t.Fatalf("platform export query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(platform.PublicationExport{
				RoutePath:            "/pub/texture/test",
				PublicationID:        "pub-1",
				PublicationVersionID: "pubver-1",
				Format:               "md",
				MediaType:            "text/markdown; charset=utf-8",
				Filename:             "test.md",
				Content:              "public content",
				ContentHash:          "hash",
			})
		default:
			t.Fatalf("corpusd path: got %s", r.URL.Path)
		}
	}))
	defer corpusd.Close()

	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/pub/texture/test", nil)
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("resolve status: got %d body %s", w.Code, w.Body.String())
	}
	if gotInternal != "true" {
		t.Fatalf("missing internal caller header")
	}
	if gotView != "" {
		t.Fatalf("plain resolve forwarded view=%q", gotView)
	}
	// The reader's poll asks for the nav alone; any other view is dropped.
	for _, c := range []struct{ query, want string }{{"&view=nav", "nav"}, {"&view=everything", ""}} {
		gotView = "unset"
		navW := httptest.NewRecorder()
		h.HandleAPI(navW, httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/pub/texture/test"+c.query, nil))
		if navW.Code != http.StatusOK || gotView != c.want {
			t.Fatalf("resolve%s: status %d, forwarded view=%q, want %q", c.query, navW.Code, gotView, c.want)
		}
	}
	exportReq := httptest.NewRequest(http.MethodGet, "/api/platform/publications/export?route=/pub/texture/test&format=md", nil)
	exportW := httptest.NewRecorder()
	h.HandleAPI(exportW, exportReq)
	if exportW.Code != http.StatusOK {
		t.Fatalf("export status: got %d body %s", exportW.Code, exportW.Body.String())
	}
	var exportResp platform.PublicationExport
	if err := json.NewDecoder(exportW.Body).Decode(&exportResp); err != nil {
		t.Fatalf("decode export response: %v", err)
	}
	if exportResp.Content != "public content" || exportResp.Format != "md" {
		t.Fatalf("export response = %#v", exportResp)
	}
}

func TestPlatformPublicationResolveAndExportPropagateNotFound(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("corpusd missing internal caller header")
		}
		switch r.URL.Path {
		case "/internal/platform/publications/resolve", "/internal/platform/publications/export":
			http.NotFound(w, r)
		default:
			t.Fatalf("corpusd path: got %s", r.URL.Path)
		}
	}))
	defer corpusd.Close()

	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	resolveReq := httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/pub/texture/private", nil)
	resolveW := httptest.NewRecorder()
	h.HandleAPI(resolveW, resolveReq)
	if resolveW.Code != http.StatusNotFound {
		t.Fatalf("resolve status: got %d body %s, want 404", resolveW.Code, resolveW.Body.String())
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/platform/publications/export?route=/pub/texture/private&format=md", nil)
	exportW := httptest.NewRecorder()
	h.HandleAPI(exportW, exportReq)
	if exportW.Code != http.StatusNotFound {
		t.Fatalf("export status: got %d body %s, want 404", exportW.Code, exportW.Body.String())
	}
}

// The Autopaper front list is public (no session) and reads corpusd alone:
// the proxy never resolves or calls a computer for it, so the front page
// stays up while the paper computer is stopped (Cut 2 unit 2b).
func TestPlatformFrontStoriesIsPublicAndNeverCallsAComputer(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("front list reached a computer: %s %s", r.Method, r.URL.Path)
		http.Error(w, "computer stopped", http.StatusServiceUnavailable)
	}))
	defer guest.Close()
	var gotInternal, gotQuery string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/publications/front" {
			t.Fatalf("corpusd path: got %s", r.URL.Path)
		}
		gotInternal = r.Header.Get("X-Internal-Caller")
		gotQuery = r.URL.RawQuery
		if r.URL.Query().Get("cursor") == "bad" || r.URL.Query().Get("section") == "sports" {
			http.Error(w, `{"error":"invalid cursor"}`, http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(platform.FrontPage{
			Stories: []platform.FrontStory{{
				PublicationID:  "pub-1",
				Title:          "Harbor reopens",
				Dek:            "The harbor reopened at dawn.",
				VersionCount:   3,
				LatestRevision: 3,
				Route:          platform.FrontRoute{Bare: "/pub/texture/harbor-abc", Permalink: "/pub/texture/harbor-abc/v3"},
			}},
			NextCursor: "next",
		})
	}))
	defer corpusd.Close()

	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       guest.URL,
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/publications/front?limit=5&cursor=abc&section=us_politics", nil)
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("front status: got %d body %s", w.Code, w.Body.String())
	}
	if gotInternal != "true" {
		t.Fatalf("missing internal caller header")
	}
	if gotQuery != "cursor=abc&limit=5&section=us_politics" {
		t.Fatalf("corpusd query = %q", gotQuery)
	}
	var page platform.FrontPage
	if err := json.NewDecoder(w.Body).Decode(&page); err != nil {
		t.Fatalf("decode front: %v", err)
	}
	if len(page.Stories) != 1 || page.Stories[0].VersionCount != 3 || page.NextCursor != "next" {
		t.Fatalf("front page = %+v", page)
	}

	badW := httptest.NewRecorder()
	h.HandleAPI(badW, httptest.NewRequest(http.MethodGet, "/api/platform/publications/front?cursor=bad", nil))
	if badW.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor status: got %d", badW.Code)
	}
	badSectionW := httptest.NewRecorder()
	h.HandleAPI(badSectionW, httptest.NewRequest(http.MethodGet, "/api/platform/publications/front?section=sports", nil))
	if badSectionW.Code != http.StatusBadRequest {
		t.Fatalf("bad section status: got %d", badSectionW.Code)
	}
	postW := httptest.NewRecorder()
	h.HandleAPI(postW, httptest.NewRequest(http.MethodPost, "/api/platform/publications/front", nil))
	if postW.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status: got %d", postW.Code)
	}
}

// The proposal is recorded for its author by corpusd and nowhere else. R3d
// deleted the author guest's /internal/texture/proposals route; the proxy
// used to POST there anyway (after resolving, and so starting, the author's
// computer) and always fell back (docs/archive/problems/surface-broken-calls-2026-10-14.md,
// item 2). Failure modes pinned: a request reaches the author computer, the
// proxy calls corpusd's delivery-state route, or the client sees a delivery
// state corpusd did not record.
func TestHandlePublicationProposalReadsPrivateDerivativeAndPostsProjection(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.SubmitPublicationProposalRequest
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("corpusd missing internal caller header")
		}
		switch r.URL.Path {
		case "/internal/platform/publications/pub-1/proposals":
			if err := json.NewDecoder(r.Body).Decode(&gotPlatformReq); err != nil {
				t.Fatalf("decode platform proposal request: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(platform.SubmitPublicationProposalResponse{
				ProposalID:           "readerprop-1",
				PublicationID:        "pub-1",
				PublicationVersionID: "pubver-1",
				SourceOwnerID:        "author-1",
				SubmitterID:          "reader-1",
				ContentHash:          "hash",
				DeliveryID:           "delivery-1",
				DeliveryState:        "recorded_for_author",
				State:                "proposed",
			})
		default:
			t.Fatalf("corpusd path: got %s", r.URL.Path)
		}
	}))
	defer corpusd.Close()

	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/texture/documents/doc-1":
			if r.Header.Get("X-Authenticated-User") != "reader-1" {
				t.Fatalf("autoputer trusted user header: got %q", r.Header.Get("X-Authenticated-User"))
			}
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{
				DocID:             "doc-1",
				OwnerID:           "reader-1",
				Title:             "My derivative",
				CurrentRevisionID: "rev-1",
			})
		case "/api/texture/revisions/rev-1":
			if r.Header.Get("X-Authenticated-User") != "reader-1" {
				t.Fatalf("autoputer trusted user header: got %q", r.Header.Get("X-Authenticated-User"))
			}
			_ = json.NewEncoder(w).Encode(autoputerTextureRevision{
				RevisionID: "rev-1",
				DocID:      "doc-1",
				OwnerID:    "reader-1",
				Content:    "reader proposal content",
			})
		default:
			t.Fatalf("autoputer path: got %s", r.URL.Path)
		}
	}))
	defer autoputer.Close()

	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputer.URL,
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	body := `{"doc_id":"doc-1","revision_id":"rev-1","publication_version_id":"pubver-1","transclusions":[{"source_kind":"published_texture_span","publication_id":"pub-1","publication_version_id":"pubver-1","span_id":"span-1","content_hash":"hash","snapshot_text":"source"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/platform/publications/pub-1/proposals", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "reader-1")})
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("proposal status: got %d body %s", w.Code, w.Body.String())
	}
	if gotPlatformReq.SubmitterID != "reader-1" || gotPlatformReq.SubmitterDocID != "doc-1" || gotPlatformReq.SubmitterRevisionID != "rev-1" {
		t.Fatalf("platform proposal private source mismatch: %#v", gotPlatformReq)
	}
	if gotPlatformReq.Content != "reader proposal content" {
		t.Fatalf("platform proposal content: %q", gotPlatformReq.Content)
	}
	if len(gotPlatformReq.Transclusions) != 1 || gotPlatformReq.Transclusions[0].SpanID != "span-1" {
		t.Fatalf("platform proposal transclusions: %#v", gotPlatformReq.Transclusions)
	}
	if strings.Contains(w.Body.String(), "source_owner_id") {
		t.Fatalf("client proposal response leaked source owner: %s", w.Body.String())
	}
	var clientResp publicationProposalClientResponse
	if err := json.NewDecoder(w.Body).Decode(&clientResp); err != nil {
		t.Fatalf("decode client proposal response: %v", err)
	}
	if clientResp.DeliveryState != "recorded_for_author" {
		t.Fatalf("client delivery_state = %q, want corpusd's recorded_for_author", clientResp.DeliveryState)
	}
}
