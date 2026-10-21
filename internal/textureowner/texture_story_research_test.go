package textureowner

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/design/research-more-goes-to-research-2026-10-21.md: the paper's
// route for a reader's request. Failure modes: the ask wakes Texture (2);
// a replay answers differently or writes again (4, 10); another owner
// reaches the story (5); an open round or a moved head is admitted (9);
// the request cannot be read back for the fate line (section 8).

func researchRequest(t *testing.T, h *Handler, method, path, owner string, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if owner != "" {
		req.Header.Set("X-Authenticated-User", owner)
	}
	rec := httptest.NewRecorder()
	h.HandleTextureRouter(rec, req)
	return rec
}

func TestStoryResearchRouteCastsResearchWithoutWakingTexture(t *testing.T) {
	core, handler := testAPISetup(t)
	var dispatches []storyReviseDispatch
	core.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		dispatches = append(dispatches, storyReviseDispatch{to: to, kind: kind})
		return nil
	})
	owner, doc := openOwnerStory(t, handler, "research")
	path := "/api/texture/documents/" + doc.DocID + "/research"
	body := map[string]string{"client_request_id": "expand:pub-r:rev-1", "expected_head_revision_id": doc.CurrentRevisionID}

	first := researchRequest(t, handler, http.MethodPost, path, owner, body)
	if first.Code != http.StatusAccepted || !strings.Contains(first.Body.String(), store.StoryResearchRecordID("expand:pub-r:rev-1")) {
		t.Fatalf("POST = %d %s", first.Code, first.Body.String())
	}
	for _, d := range dispatches {
		if d.to == agentprofile.Texture+":"+doc.DocID {
			t.Fatalf("the ask woke Texture: %+v", dispatches)
		}
	}
	again := researchRequest(t, handler, http.MethodPost, path, owner, body)
	if again.Code != http.StatusAccepted || !strings.Contains(again.Body.String(), `"replay":true`) {
		t.Fatalf("replay = %d %s", again.Code, again.Body.String())
	}
	if other := researchRequest(t, handler, http.MethodPost, path, "someone-else", body); other.Code != http.StatusNotFound {
		t.Fatalf("another owner = %d", other.Code)
	}

	read := researchRequest(t, handler, http.MethodGet, path+"/expand:pub-r:rev-1", owner, nil)
	var state store.StoryResearchState
	if read.Code != http.StatusOK || json.Unmarshal(read.Body.Bytes(), &state) != nil || !state.Open || state.Round != 1 {
		t.Fatalf("GET = %d %s", read.Code, read.Body.String())
	}
	if missing := researchRequest(t, handler, http.MethodGet, path+"/expand:none", owner, nil); missing.Code != http.StatusNotFound {
		t.Fatalf("GET unknown = %d", missing.Code)
	}

	// The round is open now: a second, different request is busy.
	busy := researchRequest(t, handler, http.MethodPost, path, owner, map[string]string{"client_request_id": "expand:pub-r:rev-2", "expected_head_revision_id": doc.CurrentRevisionID})
	if busy.Code != http.StatusConflict || !strings.Contains(busy.Body.String(), `"reason":"busy"`) {
		t.Fatalf("open round = %d %s", busy.Code, busy.Body.String())
	}
}

func TestStoryResearchRouteRefusesAMovedHead(t *testing.T) {
	_, handler := testAPISetup(t)
	owner, doc := openOwnerStory(t, handler, "research-head")
	path := "/api/texture/documents/" + doc.DocID + "/research"
	res := researchRequest(t, handler, http.MethodPost, path, owner, map[string]string{"client_request_id": "expand:x", "expected_head_revision_id": "old-head"})
	if res.Code != http.StatusConflict || !strings.Contains(res.Body.String(), `"reason":"changed"`) {
		t.Fatalf("moved head = %d %s", res.Code, res.Body.String())
	}
	_ = types.TrajectoryLive
}
