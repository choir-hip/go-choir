package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Lane F: ask about a story no one is covering
// (docs/problems/lane-f-asking-failure-list-2026-11-10.md, A1 to A6). The
// tests were written before the code.

func postAsk(t *testing.T, handler *Handler, owner string, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := authenticatedRequest(http.MethodPost, "/api/autopaper/ask", body, owner)
	if owner == "" {
		req.Header.Del("X-Authenticated-User")
	}
	handler.HandleAskStory(w, req)
	return w
}

func TestAskLandsAsANewDocumentThatRecordsTheQuestionA5A6(t *testing.T) {
	core, handler := testAPISetup(t)
	const owner = "asker-1"
	const question = "What happened to the ferry service in the outer harbor this week?"
	body, _ := json.Marshal(map[string]string{"text": question, "command_id": "ask-1"})
	w := postAsk(t, handler, owner, string(body))
	if w.Code != http.StatusAccepted {
		t.Fatalf("ask = %d %s", w.Code, w.Body)
	}
	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.DocID == "" || resp.RevisionID == "" {
		t.Fatalf("the ask made no document: %+v", resp)
	}
	// A6: the document is the asker's own.
	doc, err := core.Store().GetLifecycleDocument(context.Background(), owner, core.TextureComputerID(), resp.DocID)
	if err != nil || doc.OwnerID != owner {
		t.Fatalf("doc=%+v err=%v", doc, err)
	}
	// A5: v0 is the asker's words and says what kind of ask it was.
	rev, err := core.Store().GetLifecycleRevision(context.Background(), owner, core.TextureComputerID(), resp.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if rev.Content != question {
		t.Fatalf("v0 content = %q, want the question verbatim", rev.Content)
	}
	var meta map[string]any
	if err := json.Unmarshal(rev.Metadata, &meta); err != nil || meta["asked"] != "story" {
		t.Fatalf("v0 metadata = %s, want asked=story", rev.Metadata)
	}
	// The desk's first objective is to investigate, in the question's terms.
	snap, err := core.Store().GetLifecycleSnapshot(context.Background(), owner, core.TextureComputerID(), resp.TrajectoryID)
	if err != nil || len(snap.WorkItems) == 0 {
		t.Fatalf("snapshot err=%v", err)
	}
	obj := snap.WorkItems[0].Objective
	if !strings.Contains(obj, question) || !strings.Contains(strings.ToLower(obj), "investigat") {
		t.Fatalf("work objective = %q, want an investigation of the question", obj)
	}
}

func TestAskReplayIsOneDocumentA4(t *testing.T) {
	_, handler := testAPISetup(t)
	body := `{"text":"Who is buying the shuttered mill?","command_id":"ask-replay"}`
	var first promptBarSubmitResponse
	for i := 0; i < 2; i++ {
		w := postAsk(t, handler, "asker-2", body)
		if w.Code != http.StatusAccepted {
			t.Fatalf("ask %d = %d %s", i, w.Code, w.Body)
		}
		var got promptBarSubmitResponse
		_ = json.NewDecoder(w.Body).Decode(&got)
		if i == 0 {
			first = got
		} else if got.DocID != first.DocID || got.RevisionID != first.RevisionID {
			t.Fatalf("replay made another document: %+v vs %+v", got, first)
		}
	}
}

func TestAskRefusals(t *testing.T) {
	_, handler := testAPISetup(t)
	cases := []struct {
		name, owner, body string
		want              int
	}{
		{"empty text A1", "asker-3", `{"text":"   "}`, http.StatusBadRequest},
		{"no text A1", "asker-3", `{}`, http.StatusBadRequest},
		{"too long A2", "asker-3", `{"text":"` + strings.Repeat("q", askStoryMaxBytes+1) + `"}`, http.StatusRequestEntityTooLarge},
		{"no session A3", "", `{"text":"what happened"}`, http.StatusUnauthorized},
		{"unknown field", "asker-3", `{"text":"what happened","owner_id":"someone-else"}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		if w := postAsk(t, handler, c.owner, c.body); w.Code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, w.Code, w.Body, c.want)
		}
	}
}

func TestAskRefusesWrongMethod(t *testing.T) {
	_, handler := testAPISetup(t)
	w := httptest.NewRecorder()
	handler.HandleAskStory(w, authenticatedRequest(http.MethodGet, "/api/autopaper/ask", "", "asker-4"))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d", w.Code)
	}
}
