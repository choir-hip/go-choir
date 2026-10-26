package textureowner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Slice d (docs/problems/related-stories-no-entity-layer-2026-10-26.md): the
// owner's act that asks older stories' research desks to record what each
// story is about. Failure modes: anyone but the owner runs it; it runs with
// the setting off or past its bound; the answer does not name what it asked.

func relatedReviewRequest(t *testing.T, h *Handler, method, owner string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, relatedReviewPath, bytes.NewReader(raw))
	if owner != "" {
		req.Header.Set("X-Authenticated-User", owner)
	}
	rec := httptest.NewRecorder()
	h.HandleTextureRouter(rec, req)
	return rec
}

func TestRelatedReviewRouteIsTheOwnersBoundedAct(t *testing.T) {
	_, handler := testAPISetup(t)
	owner, doc := openOwnerStory(t, handler, "related-review")
	body := map[string]any{"batch_id": "batch-1", "limit": 5}

	if got := relatedReviewRequest(t, handler, http.MethodPost, "", body); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", got.Code)
	}
	if got := relatedReviewRequest(t, handler, http.MethodGet, owner, nil); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d", got.Code)
	}
	if got := relatedReviewRequest(t, handler, http.MethodPost, owner, map[string]any{"limit": 5}); got.Code != http.StatusBadRequest {
		t.Fatalf("no batch id = %d", got.Code)
	}
	if got := relatedReviewRequest(t, handler, http.MethodPost, owner, body); got.Code != http.StatusConflict {
		t.Fatalf("setting off = %d %s, want 409", got.Code, got.Body.String())
	}
	if err := handler.Store.PutComputerSetting(t.Context(), store.ComputerSetting{OwnerID: owner, ComputerID: doc.ComputerID, Key: store.SettingResearchRelatedStory, Value: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if got := relatedReviewRequest(t, handler, http.MethodPost, owner, map[string]any{"batch_id": "batch-big", "limit": agentcore.StoryEntitiesBackfillMax + 1}); got.Code != http.StatusBadRequest {
		t.Fatalf("past the bound = %d", got.Code)
	}
	got := relatedReviewRequest(t, handler, http.MethodPost, owner, body)
	var receipt struct {
		Schema string `json:"schema"`
		agentcore.StoryEntitiesBackfillResult
	}
	if got.Code != http.StatusAccepted || json.Unmarshal(got.Body.Bytes(), &receipt) != nil || receipt.Scanned != 1 || receipt.Empty != 1 || len(receipt.Requested) != 0 {
		// The story has only its opening seed, so it is read and skipped
		// (review 325 F3), which shows the act ran for the owner's computer.
		t.Fatalf("POST = %d %s, want the seed-only story read and skipped", got.Code, got.Body.String())
	}
}
