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
	var gotInternal string
	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInternal = r.Header.Get("X-Internal-Caller")
		switch r.URL.Path {
		case "/internal/platform/publications/resolve":
			if r.URL.Query().Get("route") != "/pub/vtext/test" {
				t.Fatalf("platform route query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(platform.PublicationBundle{
				Route:       platform.PublicationRoute{Path: "/pub/vtext/test", State: "active"},
				Publication: platform.PublicationSummary{ID: "pub-1", Title: "Public"},
				Version:     platform.PublicationVersionSummary{ID: "pubver-1", ContentHash: "hash", SourceRevisionHash: "source-hash"},
				Artifact:    platform.PublicationArtifact{Content: "public content"},
			})
		case "/internal/platform/publications/export":
			if r.URL.Query().Get("route") != "/pub/vtext/test" || r.URL.Query().Get("format") != "md" {
				t.Fatalf("platform export query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(platform.PublicationExport{
				RoutePath:            "/pub/vtext/test",
				PublicationID:        "pub-1",
				PublicationVersionID: "pubver-1",
				Format:               "md",
				MediaType:            "text/markdown; charset=utf-8",
				Filename:             "test.md",
				Content:              "public content",
				ContentHash:          "hash",
			})
		default:
			t.Fatalf("platformd path: got %s", r.URL.Path)
		}
	}))
	defer platformd.Close()

	h, err := NewHandler(&Config{
		Port:              "0",
		SandboxURL:        "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test",
		PlatformdURL:      platformd.URL,
	}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/pub/vtext/test", nil)
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("resolve status: got %d body %s", w.Code, w.Body.String())
	}
	if gotInternal != "true" {
		t.Fatalf("missing internal caller header")
	}
	exportReq := httptest.NewRequest(http.MethodGet, "/api/platform/publications/export?route=/pub/vtext/test&format=md", nil)
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
	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("platformd missing internal caller header")
		}
		switch r.URL.Path {
		case "/internal/platform/publications/resolve", "/internal/platform/publications/export":
			http.NotFound(w, r)
		default:
			t.Fatalf("platformd path: got %s", r.URL.Path)
		}
	}))
	defer platformd.Close()

	h, err := NewHandler(&Config{
		Port:              "0",
		SandboxURL:        "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test",
		PlatformdURL:      platformd.URL,
	}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	resolveReq := httptest.NewRequest(http.MethodGet, "/api/platform/publications/resolve?route=/pub/vtext/private", nil)
	resolveW := httptest.NewRecorder()
	h.HandleAPI(resolveW, resolveReq)
	if resolveW.Code != http.StatusNotFound {
		t.Fatalf("resolve status: got %d body %s, want 404", resolveW.Code, resolveW.Body.String())
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/platform/publications/export?route=/pub/vtext/private&format=md", nil)
	exportW := httptest.NewRecorder()
	h.HandleAPI(exportW, exportReq)
	if exportW.Code != http.StatusNotFound {
		t.Fatalf("export status: got %d body %s, want 404", exportW.Code, exportW.Body.String())
	}
}

func TestHandlePublicationProposalReadsPrivateDerivativeAndPostsProjection(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.SubmitPublicationProposalRequest
	var gotDeliveryUpdate platform.UpdateProposalDeliveryStateRequest
	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("platformd missing internal caller header")
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
		case "/internal/platform/proposal-deliveries/state":
			if err := json.NewDecoder(r.Body).Decode(&gotDeliveryUpdate); err != nil {
				t.Fatalf("decode platform delivery update: %v", err)
			}
			_ = json.NewEncoder(w).Encode(platform.UpdateProposalDeliveryStateResponse{
				ProposalID:    gotDeliveryUpdate.ProposalID,
				DeliveryID:    gotDeliveryUpdate.DeliveryID,
				DeliveryState: gotDeliveryUpdate.DeliveryState,
			})
		default:
			t.Fatalf("platformd path: got %s", r.URL.Path)
		}
	}))
	defer platformd.Close()

	delivered := false
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/vtext/documents/doc-1":
			if r.Header.Get("X-Authenticated-User") != "reader-1" {
				t.Fatalf("sandbox trusted user header: got %q", r.Header.Get("X-Authenticated-User"))
			}
			_ = json.NewEncoder(w).Encode(sandboxVTextDocument{
				DocID:             "doc-1",
				OwnerID:           "reader-1",
				Title:             "My derivative",
				CurrentRevisionID: "rev-1",
			})
		case "/api/vtext/revisions/rev-1":
			if r.Header.Get("X-Authenticated-User") != "reader-1" {
				t.Fatalf("sandbox trusted user header: got %q", r.Header.Get("X-Authenticated-User"))
			}
			_ = json.NewEncoder(w).Encode(sandboxVTextRevision{
				RevisionID: "rev-1",
				DocID:      "doc-1",
				OwnerID:    "reader-1",
				Content:    "reader proposal content",
			})
		case "/internal/vtext/proposals":
			if r.Header.Get("X-Internal-Caller") != "true" {
				t.Fatalf("author delivery missing internal caller")
			}
			delivered = true
			_ = json.NewEncoder(w).Encode(authorProposalDeliveryResponse{
				DeliveryID:    "delivery-1",
				TargetAgentID: "super:author-1",
				ChannelID:     "super:author-1",
				State:         "delivered",
			})
		default:
			t.Fatalf("sandbox path: got %s", r.URL.Path)
		}
	}))
	defer sandbox.Close()

	h, err := NewHandler(&Config{
		Port:              "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		PlatformdURL:      platformd.URL,
	}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	body := `{"doc_id":"doc-1","revision_id":"rev-1","publication_version_id":"pubver-1","transclusions":[{"source_kind":"published_vtext_span","publication_id":"pub-1","publication_version_id":"pubver-1","span_id":"span-1","content_hash":"hash","snapshot_text":"source"}]}`
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
	if !delivered {
		t.Fatalf("expected author delivery attempt")
	}
	if gotDeliveryUpdate.ProposalID != "readerprop-1" || gotDeliveryUpdate.DeliveryID != "delivery-1" || gotDeliveryUpdate.DeliveryState != "delivered" {
		t.Fatalf("platform delivery update mismatch: %#v", gotDeliveryUpdate)
	}
	if strings.Contains(w.Body.String(), "source_owner_id") {
		t.Fatalf("client proposal response leaked source owner: %s", w.Body.String())
	}
	var clientResp publicationProposalClientResponse
	if err := json.NewDecoder(w.Body).Decode(&clientResp); err != nil {
		t.Fatalf("decode client proposal response: %v", err)
	}
	if clientResp.DeliveryState != "delivered" {
		t.Fatalf("client delivery_state = %q, want delivered", clientResp.DeliveryState)
	}
}
