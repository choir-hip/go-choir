package textureowner

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// postRevise posts a JSON body to /revise as owner.
func postRevise(t *testing.T, handler *Handler, docID, owner string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/texture/documents/"+docID+"/revise", bytes.NewReader(raw))
	if owner != "" {
		req.Header.Set("X-Authenticated-User", owner)
	}
	w := httptest.NewRecorder()
	handler.HandleTextureRouter(w, req)
	return w
}

// countWakes records the desk wakes the revise commits dispatch.
func countWakes(core *agentcore.Runtime) func() int {
	var mu sync.Mutex
	n := 0
	core.SetDispatchActor(func(_ context.Context, _, _, _, kind, _, _, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		if kind == "coagent_result" {
			n++
		}
		return nil
	})
	return func() int { mu.Lock(); defer mu.Unlock(); return n }
}

func decodeReceipt(t *testing.T, w *httptest.ResponseRecorder) textureOwnerRevisionResponse {
	t.Helper()
	var receipt textureOwnerRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&receipt); err != nil {
		t.Fatalf("decode receipt: %v body=%s", err, w.Body.String())
	}
	return receipt
}

func TestReviseReplayIsOneRevisionOneWake(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	wakes := countWakes(core)
	body := map[string]any{"client_request_id": "replay-1", "text": "tighten the lede"}
	first := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, body)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, body)
	if second.Code != http.StatusAccepted {
		t.Fatalf("replay status=%d body=%s", second.Code, second.Body.String())
	}
	a, b := decodeReceipt(t, first), decodeReceipt(t, second)
	if a.Replay || !b.Replay || a.RevisionID != b.RevisionID {
		t.Fatalf("receipts first=%+v replay=%+v", a, b)
	}
	if got := wakes(); got != 1 {
		t.Fatalf("wakes=%d, want 1", got)
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.HeadRevision.RevisionID != a.RevisionID {
		t.Fatalf("head=%+v err=%v", snapshot.HeadRevision, err)
	}
	// A plain instruction on the head carries no parent links: its provenance
	// is what it always was (empty).
	if len(snapshot.HeadRevision.Provenance) != 0 && string(snapshot.HeadRevision.Provenance) != "{}" {
		t.Fatalf("plain comment provenance=%s, want none", snapshot.HeadRevision.Provenance)
	}
}

func TestReviseRequestIdWithOtherTextConflicts(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	wakes := countWakes(core)
	doc := start.InitialDocument.DocID
	if w := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "same-id", "text": "one"}); w.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", w.Code, w.Body.String())
	}
	if w := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "same-id", "text": "another"}); w.Code != http.StatusConflict {
		t.Fatalf("other text status=%d body=%s, want 409", w.Code, w.Body.String())
	}
	if got := wakes(); got != 1 {
		t.Fatalf("wakes=%d, want 1", got)
	}
}

func TestReviseOlderBaseCommentRecordsBaseLink(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	countWakes(core)
	doc := start.InitialDocument.DocID
	first := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "c1", "text": "first"})
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	head := decodeReceipt(t, first).RevisionID
	older := start.InitialRevision.RevisionID
	second := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "c2", "text": "second, answering the original", "base_revision_id": older})
	if second.Code != http.StatusAccepted {
		t.Fatalf("older-base status=%d body=%s", second.Code, second.Body.String())
	}
	rev, err := core.Store().GetLifecycleRevision(t.Context(), start.OwnerID, start.ComputerID, decodeReceipt(t, second).RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if rev.ParentRevisionID != head {
		t.Fatalf("chain parent=%s, want the head %s", rev.ParentRevisionID, head)
	}
	var prov types.Provenance
	if err := json.Unmarshal(rev.Provenance, &prov); err != nil || len(prov.ParentLinks) != 1 || prov.ParentLinks[0].Kind != types.ParentLinkBase || prov.ParentLinks[0].RevisionID != older {
		t.Fatalf("provenance=%s err=%v, want one base link to %s", rev.Provenance, err, older)
	}
	// The legacy strict form still refuses a head that moved.
	third := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "c3", "text": "third", "expected_head_revision_id": older})
	if third.Code != http.StatusConflict {
		t.Fatalf("stale expected head status=%d body=%s, want 409", third.Code, third.Body.String())
	}
	// A base that is not a revision of the document is refused.
	fourth := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "c4", "text": "fourth", "base_revision_id": "not-a-revision"})
	if fourth.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown base status=%d body=%s, want 422", fourth.Code, fourth.Body.String())
	}
}

func TestReviseOtherOwnersDocumentIs404(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	wakes := countWakes(core)
	other := postRevise(t, handler, start.InitialDocument.DocID, "someone-else", map[string]any{"client_request_id": "x", "text": "hello"})
	missing := postRevise(t, handler, "no-such-doc", "someone-else", map[string]any{"client_request_id": "x", "text": "hello"})
	if other.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || other.Body.String() != missing.Body.String() {
		t.Fatalf("other owner=%d %s, missing=%d %s, want the same 404", other.Code, other.Body.String(), missing.Code, missing.Body.String())
	}
	if wakes() != 0 {
		t.Fatal("a refused revise woke the desk")
	}
}

func TestReviseRefusesShapesTheTableRefuses(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	doc := start.InitialDocument.DocID
	src := map[string]string{"publication_version_id": "pv", "manifest_hash": "h"}
	many := make([]map[string]string, reviseMaxSources+1)
	for i := range many {
		many[i] = map[string]string{"publication_version_id": "pv" + string(rune('a'+i)), "manifest_hash": "h"}
	}
	for name, c := range map[string]struct {
		body map[string]any
		want int
	}{
		"nothing":                    {map[string]any{"client_request_id": "n"}, http.StatusBadRequest},
		"body with sources (R15)":    {map[string]any{"client_request_id": "n", "body": map[string]any{"content": "x"}, "sources": []any{src}}, http.StatusBadRequest},
		"text with body":             {map[string]any{"client_request_id": "n", "text": "t", "body": map[string]any{"content": "x"}}, http.StatusBadRequest},
		"too many sources":           {map[string]any{"client_request_id": "n", "text": "t", "sources": many}, http.StatusBadRequest},
		"two sources, no text":       {map[string]any{"client_request_id": "n", "sources": []any{src, map[string]string{"publication_version_id": "pv2", "manifest_hash": "h"}}}, http.StatusBadRequest},
		"a source named twice":       {map[string]any{"client_request_id": "n", "text": "t", "sources": []any{src, src}}, http.StatusBadRequest},
		"source without a hash":      {map[string]any{"client_request_id": "n", "text": "t", "sources": []any{map[string]string{"publication_version_id": "pv"}}}, http.StatusBadRequest},
		"sources without an id":      {map[string]any{"text": "t", "sources": []any{src}}, http.StatusBadRequest},
		"text over the cap":          {map[string]any{"client_request_id": "n", "text": strings.Repeat("x", reviseMaxTextBytes+1)}, http.StatusRequestEntityTooLarge},
		"identity in the body (R13)": {map[string]any{"client_request_id": "n", "text": "t", "owner_id": "someone-else"}, http.StatusBadRequest},
		"computer in the body (R13)": {map[string]any{"client_request_id": "n", "text": "t", "computer_id": "other-computer"}, http.StatusBadRequest},
	} {
		if w := postRevise(t, handler, doc, start.OwnerID, c.body); w.Code != c.want {
			t.Errorf("%s: status=%d body=%s, want %d", name, w.Code, w.Body.String(), c.want)
		}
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.HeadRevision.RevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("a refused request moved the head: %+v err=%v", snapshot.HeadRevision, err)
	}
}

func TestReviseStaleBaseOnEditIs409(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	countWakes(core)
	doc := start.InitialDocument.DocID
	if w := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "move-head", "text": "move the head"}); w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	stale := postRevise(t, handler, doc, start.OwnerID, map[string]any{
		"client_request_id": "stale-edit", "base_revision_id": start.InitialRevision.RevisionID,
		"body": map[string]any{"content": "edited on an old base", "expected_lifecycle_version": snapshot.Trajectory.LifecycleVersion},
	})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale edit status=%d body=%s, want 409", stale.Code, stale.Body.String())
	}
	after, _ := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if after.HeadRevision.RevisionID != snapshot.HeadRevision.RevisionID {
		t.Fatalf("a stale edit moved the head to %s", after.HeadRevision.RevisionID)
	}
}
