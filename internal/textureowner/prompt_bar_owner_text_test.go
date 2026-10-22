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

// The owner-text rule is for the owner. The management desk reaches
// /api/prompt-bar through choir.ProductAPI, which serves the call in-process
// as the owner with an empty RemoteAddr (server.GuestHostCaller's in-process
// marker). Text a desk composes stays under the Texture citation rules: it
// is refused with a 400 and the validator's reason, so the desk can correct
// it, and never stored as an owner-labelled revision (round 2 review F1,
// bunch 81; the problem record's Round 2 section).
func TestHandlePromptBarInProcessCallerStaysUnderCitationRules(t *testing.T) {
	texts := map[string]string{
		"numbered citation": "Findings so far.[1] The rest follows.",
		"source line":       "Summary of captures.\nSource: https://example.com/fake",
		"source token":      "See {{source: capture-7}} for the claim.",
		"bracket source":    "A claim [source: wiki] without a source_ref.",
		"all four together": "Findings so far.[1] See {{source: capture-7}}.\nSource: https://example.com/fake\n[source: wiki]",
	}
	for name, text := range texts {
		t.Run(name, func(t *testing.T) {
			_, handler := testAPISetup(t)
			body, _ := json.Marshal(map[string]string{"text": text})
			req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", string(body), "user-desk-owner")
			req.RemoteAddr = "" // choir.ProductAPI's in-process call
			w := httptest.NewRecorder()
			handler.HandlePromptBar(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("desk-shaped call status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
			}
			var resp apiError
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if !strings.HasPrefix(resp.Error, "prompt refused: ") || !strings.Contains(resp.Error, "legacy") ||
				strings.Contains(resp.Error, "failed to submit prompt") || strings.Contains(resp.Error, "prepare initial revision") {
				t.Fatalf("error = %q, want the validator's reason after \"prompt refused: \"", resp.Error)
			}
		})
	}
}

// A desk still starts a Texture from ordinary text: the marker only keeps the
// authoring rules on its text.
func TestHandlePromptBarInProcessCallerStillStartsATexture(t *testing.T) {
	_, handler := testAPISetup(t)
	body, _ := json.Marshal(map[string]string{"text": "Draft a short note on retry-safe command handling."})
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", string(body), "user-desk-owner")
	req.RemoteAddr = ""
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("desk-shaped call with ordinary text status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
}

// The owner-create route takes the same rule as the prompt bar: literal for
// the owner, strict for an in-process caller (a desk cannot reach this route
// today, validateProductAPIRoute refuses a non-GET under /api/texture/; the
// marker keeps the two owner-start handlers alike).
func TestTextureLifecycleCreateOwnerTextLiteralInProcessStrict(t *testing.T) {
	_, handler := testAPISetup(t)
	post := func(clientRequestID, content string, inProcess bool) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"client_request_id": clientRequestID, "title": "Pasted script", "initial_content": content})
		req := httptest.NewRequest(http.MethodPost, "/api/texture/lifecycle-documents", strings.NewReader(string(body)))
		req.Header.Set("X-Authenticated-User", "user-create-text")
		if inProcess {
			req.RemoteAddr = ""
		}
		w := httptest.NewRecorder()
		handler.HandleTextureRouter(w, req)
		return w
	}
	const text = "print(sys.argv[1])\nSource: my notes"
	if got := post("create-owner-1", text, false); got.Code != http.StatusCreated {
		t.Fatalf("owner create with bracketed text = %d, want %d; body=%s", got.Code, http.StatusCreated, got.Body.String())
	}
	if got := post("create-desk-1", text, true); got.Code != http.StatusBadRequest {
		t.Fatalf("in-process create with bracketed text = %d, want %d; body=%s", got.Code, http.StatusBadRequest, got.Body.String())
	}
}
