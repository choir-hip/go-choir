package textureowner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Lane D: POST /api/prompt-bar/classify. Ways it could fail:
//  D1 a refusal or a failed judge blocks the send: the answer is always a
//     class, with the default's rule;
//  D2 an unknown doc_state or empty text is accepted;
//  an in-process desk caller spends a Jev call (refused, as a focused submit is).

func postClassify(t *testing.T, handler *Handler, owner, method string, body map[string]any, inProcess bool) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := authenticatedRequest(method, "/api/prompt-bar/classify", string(raw), owner)
	if inProcess {
		req.RemoteAddr = ""
	}
	w := httptest.NewRecorder()
	handler.HandlePromptBarClassify(w, req)
	return w
}

func TestPromptBarClassifyAnswersADefaultClassAndRefusesBadInput(t *testing.T) {
	_, handler := testAPISetup(t)
	owner := "owner-classify"
	w := postClassify(t, handler, owner, http.MethodPost, map[string]any{"text": "tighten the lede", "doc_state": "owned", "command_id": "c1"}, false)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Class string `json:"class"`
		Rule  string `json:"rule"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Class != "edit" || got.Rule == "" {
		t.Fatalf("D1: %+v", got)
	}
	w = postClassify(t, handler, owner, http.MethodPost, map[string]any{"text": "hello", "doc_state": "foreign"}, false)
	if w.Code != http.StatusOK {
		t.Fatalf("foreign: %d %s", w.Code, w.Body.String())
	}
	var f struct{ Class string }
	_ = json.Unmarshal(w.Body.Bytes(), &f)
	if f.Class != "comment" {
		t.Fatalf("D2: foreign default class = %q", f.Class)
	}
	for name, body := range map[string]map[string]any{
		"empty text":    {"text": "  ", "doc_state": "owned"},
		"unknown state": {"text": "x", "doc_state": "admin"},
	} {
		if w := postClassify(t, handler, owner, http.MethodPost, body, false); w.Code != http.StatusBadRequest {
			t.Fatalf("D2 %s: status=%d", name, w.Code)
		}
	}
	if w := postClassify(t, handler, owner, http.MethodGet, map[string]any{}, false); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d", w.Code)
	}
	if w := postClassify(t, handler, owner, http.MethodPost, map[string]any{"text": "x", "doc_state": "owned"}, true); w.Code != http.StatusForbidden {
		t.Fatalf("in-process status=%d", w.Code)
	}
}

// Lane D, "show the revision being responded to": a focused submit carries the
// revision the window reads as base_revision_id, and the revise call records
// the base link (D7: the prompt bar must not answer the head when the window
// reads v1 of v2).
func TestPromptBarFocusedSubmitCarriesTheBaseRevision(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	countWakes(core)
	doc := start.InitialDocument.DocID
	first := postRevise(t, handler, doc, start.OwnerID, map[string]any{"client_request_id": "pb-c1", "text": "first"})
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	older := start.InitialRevision.RevisionID
	w := postPromptBar(t, handler, start.OwnerID, map[string]any{
		"text": "answering the original", "command_id": "pb-c2", "focus": textureFocus(doc), "base_revision_id": older,
	}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	rev, err := core.Store().GetLifecycleRevision(t.Context(), start.OwnerID, start.ComputerID, decodeContinueReceipt(t, w).RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	var prov types.Provenance
	if err := json.Unmarshal(rev.Provenance, &prov); err != nil || len(prov.ParentLinks) != 1 || prov.ParentLinks[0].RevisionID != older {
		t.Fatalf("D7: provenance=%s err=%v, want one base link to %s", rev.Provenance, err, older)
	}
}
