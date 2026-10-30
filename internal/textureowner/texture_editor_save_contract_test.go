package textureowner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Texture editor's Save posts raw JSON to the real revision handler. The
// handler decodes strictly (DisallowUnknownFields) and always writes a
// user-authored revision, so the client must not send authorship fields.
//
// Failure modes this pins (docs/archive/problems/surface-broken-calls-2026-10-14.md):
//   - the editor sends author_kind/author_label and every Save gets 400;
//   - the server starts accepting client-chosen authorship (it must not: the
//     public route writes user revisions only);
//   - the server relaxes strict decoding, hiding the next client drift.
func TestTextureEditorSaveBodyContract(t *testing.T) {
	_, handler := testAPISetup(t)

	createDocW := httptest.NewRecorder()
	handler.HandleTextureCreateDocument(createDocW, textureRequest(t, http.MethodPost, "/api/texture/documents", textureCreateDocRequest{Title: "Editor save contract"}))
	if createDocW.Code != http.StatusCreated {
		t.Fatalf("create doc status = %d body=%s", createDocW.Code, createDocW.Body.String())
	}
	var created textureCreateDocResponse
	if err := json.NewDecoder(createDocW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create doc: %v", err)
	}
	path := "/api/texture/documents/" + created.DocID + "/revisions"

	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Authenticated-User", "user-1")
		w := httptest.NewRecorder()
		handler.HandleTextureRevisions(w, req)
		return w
	}

	// The body frontend/src/lib/texture.js createRevision sent before the fix.
	legacy := post(`{"content":"first save","author_kind":"user","author_label":"owner@example.com","metadata":{"surface":"texture_editor"}}`)
	if legacy.Code != http.StatusBadRequest {
		t.Fatalf("editor body with authorship fields: status = %d body=%s, want 400 (strict decoder)", legacy.Code, legacy.Body.String())
	}

	// The body it sends now: content, metadata, and nothing about authorship.
	saved := post(`{"content":"first save","metadata":{"surface":"texture_editor"}}`)
	if saved.Code != http.StatusCreated {
		t.Fatalf("editor body without authorship fields: status = %d body=%s, want 201", saved.Code, saved.Body.String())
	}
	var rev textureRevisionResponse
	if err := json.NewDecoder(saved.Body).Decode(&rev); err != nil {
		t.Fatalf("decode revision: %v", err)
	}
	if rev.AuthorKind != "user" || rev.AuthorLabel != "user-1" {
		t.Fatalf("revision authorship = %q/%q, want user/user-1 from the authenticated owner", rev.AuthorKind, rev.AuthorLabel)
	}
}
