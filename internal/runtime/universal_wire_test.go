package runtime

import (
	"encoding/json"
	"net/http"
	"testing"

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

func TestRuntimeDoesNotRegisterUniversalWireStories(t *testing.T) {
	_, handler := testAPISetup(t)
	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "reader-1")
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s, want runtime route absent", w.Code, w.Body.String())
	}
}
