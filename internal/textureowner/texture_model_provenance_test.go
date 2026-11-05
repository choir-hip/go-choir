package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// GET /api/texture/documents/{id}/models serves the models that answered the
// document's runs, from their model.served lines, and 404s for a document
// that does not exist (docs/problems/model-provenance-unwired-2026-11-05.md).
func TestTextureModelProvenanceRoute(t *testing.T) {
	ctx := context.Background()
	_, h := testAPISetup(t)
	s := h.Store
	docID := seedTextureDecisionDocument(t, s)
	run := seedTextureDecisionRun(t, s, docID)
	served, _ := json.Marshal(types.ModelServedRecord{Schema: types.ModelServedSchema, Purpose: types.ModelServedPurposeToolLoop,
		SelectedProvider: "opencode-go", SelectedModel: "deepseek-v4.1-flash",
		Models: []types.ModelServedUsage{{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Calls: 2}}})
	if err := s.AppendEvent(ctx, &types.EventRecord{EventID: "ev-route-served", RunID: run.RunID, OwnerID: run.OwnerID,
		Timestamp: time.Now().UTC(), Kind: types.EventModelServed, Payload: served}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.HandleTextureRouter(w, textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/models", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var got types.ModelProvenance
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Schema != types.ModelProvenanceSchema || len(got.Roles) != 1 || len(got.Roles[0].Models) != 1 ||
		got.Roles[0].Models[0].Model != "deepseek-v4.1-flash" || got.Roles[0].Models[0].Calls != 2 {
		t.Fatalf("provenance = %+v", got)
	}

	missing := httptest.NewRecorder()
	h.HandleTextureRouter(missing, textureRequest(t, http.MethodGet, "/api/texture/documents/doc-missing/models", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing document status = %d: %s", missing.Code, missing.Body.String())
	}
}
