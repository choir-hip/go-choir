package proxy

// Station TE (docs/design/texture-evidence-embeds-te-2026-10-25.md E6): the
// owner's publish route refuses a revision that holds capsule evidence
// before corpusd sees it, with the reason, and never strips it.

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleTexturePublicationRefusesCapsuleEvidence(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("corpusd was called for a document holding capsule evidence")
	}))
	defer corpusd.Close()
	body := json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"doc-ev"},"content":[` +
		`{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"Evidence follows."}]},` +
		`{"type":"embed","attrs":{"id":"embed-1","kind":"capsule_evidence","target":{"digest":"` + strings.Repeat("a", 64) + `","operation_id":"selfdev-op-1","computer_id":"c1","route":"/reader","width":1280,"height":800},"caption":"Screenshot of /reader","alt":"Screenshot of /reader"}}]}}`)
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/texture/documents/doc-ev":
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{DocID: "doc-ev", OwnerID: "user-1", Title: "Evidence", CurrentRevisionID: "rev-ev"})
		case "/api/texture/revisions/rev-ev":
			_ = json.NewEncoder(w).Encode(autoputerTextureRevision{RevisionID: "rev-ev", DocID: "doc-ev", OwnerID: "user-1",
				Content: "Evidence follows.\n\n[evidence: Screenshot of /reader]", BodyDoc: body, Metadata: json.RawMessage(`{}`)})
		default:
			t.Errorf("autoputer path: got %s", r.URL.Path)
		}
	}))
	defer autoputer.Close()
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: autoputer.URL,
		AuthPublicKeyPath: "/unused/in/test", CorpusdURL: corpusd.URL}, pub)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/publications", strings.NewReader(`{"doc_id":"doc-ev","revision_id":"rev-ev"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()
	h.HandleTexturePublication(w, req)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "capsule_evidence") || !strings.Contains(w.Body.String(), "never published") {
		t.Fatalf("status %d body %s; want 409 naming the evidence", w.Code, w.Body.String())
	}
}
