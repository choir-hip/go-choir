package runtime

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func runtimeTestTextureBodyDoc(t *testing.T, docID, revisionID, content string) json.RawMessage {
	t.Helper()
	doc := plainStructuredTextureToolDoc(docID, revisionID, content)
	bodyDoc, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal test body_doc: %v", err)
	}
	return bodyDoc
}

func TestRuntimeTextureReadsRemainOwnerScopedAfterWireCutover(t *testing.T) {
	rt, handler := testAPISetup(t)
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     "platform-published-doc",
		OwnerID:   universalWirePlatformOwnerID(),
		Title:     "Published elsewhere",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := rt.Store().CreateDocument(t.Context(), doc); err != nil {
		t.Fatalf("create platform document: %v", err)
	}
	rev := types.Revision{
		RevisionID: "platform-published-revision",
		DocID:      doc.DocID,
		OwnerID:    doc.OwnerID,
		AuthorKind: types.AuthorUser,
		Content:    "Canonical public content lives in corpusd.",
		CreatedAt:  now,
	}
	if err := rt.Store().CreateRevision(t.Context(), rev); err != nil {
		t.Fatalf("create platform revision: %v", err)
	}
	doc.CurrentRevisionID = rev.RevisionID
	if err := rt.Store().UpdateDocument(t.Context(), doc); err != nil {
		t.Fatalf("advance platform document head: %v", err)
	}

	for _, path := range []string{
		"/api/texture/documents/" + doc.DocID,
		"/api/texture/documents/" + doc.DocID + "/revisions",
		"/api/texture/revisions/" + rev.RevisionID,
		"/api/texture/documents/" + doc.DocID + "/history",
		"/api/texture/documents/" + doc.DocID + "/stream",
	} {
		w := runtimeHandlerRequest(t, handler.HandleTextureRouter, http.MethodGet, path, "", "reader-1")
		if w.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d body=%s, want owner-scoped not found", path, w.Code, w.Body.String())
		}
	}
}
