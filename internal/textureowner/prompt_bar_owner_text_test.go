package textureowner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// An owner's prompt is the owner's literal text. A bracketed index, a Python
// argv[1], a markdown footnote and a "Source:" line are ordinary things to
// paste; none of them is a Texture citation, and none of them may turn the
// prompt into a server error (receipt:
// docs/problems/prompt-bar-refuses-bracketed-index-as-server-error-2026-10-22.md,
// staging 2026-10-22 16:28 UTC, HTTP 500 "failed to submit prompt").
func TestHandlePromptBarKeepsOwnersBracketedTextVerbatim(t *testing.T) {
	longScript := make([]string, 0, 220)
	for i := 0; i < 220; i++ {
		longScript = append(longScript, fmt.Sprintf("echo step %d", i))
	}
	longScript[100] = `args=("$@"); echo "${args[1]}"  # the second argument, args[1]`
	cases := []struct {
		name   string
		prompt string
	}{
		{"bracketed index", "Explain why values[1] and values[23] differ."},
		{"python argv", "import sys\nprint(sys.argv[1])\nprint(sys.argv[2])"},
		{"markdown footnote", "Summarize this note.[1]\n\n[1]: https://example.com/a-note"},
		{"source line", "Source: the quarterly report\nSummarize it in two lines."},
		{"source token", "Use this template as is: {{source: quarterly}} and [source: wiki]."},
		{"long pasted script", strings.Join(longScript, "\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, handler := testAPISetup(t)
			const ownerID = "user-owner-text"
			body, _ := json.Marshal(map[string]string{"text": tc.prompt})
			w := httptest.NewRecorder()
			handler.HandlePromptBar(w, authenticatedRequest(http.MethodPost, "/api/prompt-bar", string(body), ownerID))
			if w.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want %d (the owner's text is not a citation); body=%s", w.Code, http.StatusAccepted, w.Body.String())
			}
			var resp promptBarSubmitResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.DocID == "" || resp.RevisionID == "" || resp.TrajectoryID == "" {
				t.Fatalf("response carries no document: %+v", resp)
			}
			rev, err := rt.Store().GetLifecycleRevision(context.Background(), ownerID, rt.TextureComputerID(), resp.RevisionID)
			if err != nil {
				t.Fatalf("get initial revision: %v", err)
			}
			if rev.Content != tc.prompt {
				t.Fatalf("initial revision content = %q, want the owner's words exactly %q", rev.Content, tc.prompt)
			}
			if len(rev.BodyDoc) == 0 || strings.Contains(string(rev.BodyDoc), "source_ref") {
				t.Fatalf("initial body_doc = %s, want plain text with no source_ref", rev.BodyDoc)
			}
		})
	}
}

// A retry of the same command replays the stored lifecycle; the replay does
// not validate the owner's text again and answers the same document.
func TestHandlePromptBarReplaysBracketedPrompt(t *testing.T) {
	_, handler := testAPISetup(t)
	const ownerID = "user-owner-text-replay"
	body, _ := json.Marshal(map[string]string{
		"text":       "print(sys.argv[1])  # run with a path",
		"command_id": "prompt-command-bracket-replay-1",
	})
	submit := func() promptBarSubmitResponse {
		t.Helper()
		w := httptest.NewRecorder()
		handler.HandlePromptBar(w, authenticatedRequest(http.MethodPost, "/api/prompt-bar", string(body), ownerID))
		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
		}
		var resp promptBarSubmitResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp
	}
	first := submit()
	replay := submit()
	if !reflect.DeepEqual(first, replay) {
		t.Fatalf("replayed result changed: first=%+v replay=%+v", first, replay)
	}
}
