package textureowner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
