package textureowner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func postFork(t *testing.T, handler *Handler, owner string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/texture/fork", bytes.NewReader(raw))
	req.Header.Set("X-Authenticated-User", owner)
	w := httptest.NewRecorder()
	handler.HandleTextureRouter(w, req)
	return w
}

func TestForkIsAVerbatimCopyWithNoWake(t *testing.T) {
	core, handler := testAPISetup(t)
	wakes := countWakes(core)
	v := testVersion("pub-f-v1", "The op-ed's argument, in full.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	body := map[string]any{"client_request_id": "fork-1", "source": sourceOf(v)}
	w := postFork(t, handler, "reader-1", body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("fork status=%d body=%s", w.Code, w.Body.String())
	}
	receipt := decodeReceipt(t, w)
	doc, err := core.Store().GetLifecycleDocument(t.Context(), "reader-1", "autoputer-test", receipt.DocID)
	if err != nil || doc.Title != v.Title || doc.CurrentRevisionID != receipt.RevisionID {
		t.Fatalf("doc=%+v err=%v", doc, err)
	}
	rev, err := core.Store().GetLifecycleRevision(t.Context(), "reader-1", "autoputer-test", receipt.RevisionID)
	if err != nil || rev.Content != v.Content || rev.AuthorKind != types.AuthorUser {
		t.Fatalf("first revision=%+v err=%v", rev, err)
	}
	links := revisionSourceLinks(rev)
	if len(links) != 1 || links[0].PublicationVersionID != v.PublicationVersionID {
		t.Fatalf("links=%+v", links)
	}
	var meta map[string]any
	if err := json.Unmarshal(rev.Metadata, &meta); err != nil || meta["forked_from"] == nil {
		t.Fatalf("metadata=%s err=%v, want forked_from", rev.Metadata, err)
	}
	if wakes() != 0 {
		t.Fatalf("a verbatim fork woke the desk %d times", wakes())
	}
	// A retry is the same document and revision.
	again := postFork(t, handler, "reader-1", body)
	if again.Code != http.StatusAccepted {
		t.Fatalf("retry status=%d body=%s", again.Code, again.Body.String())
	}
	if r := decodeReceipt(t, again); r.DocID != receipt.DocID || r.RevisionID != receipt.RevisionID || !r.Replay {
		t.Fatalf("retry receipt=%+v, want a replay of %+v", r, receipt)
	}
}

func TestForkWithTextWakesTheDeskOnce(t *testing.T) {
	core, handler := testAPISetup(t)
	wakes := countWakes(core)
	v := testVersion("pub-f2-v1", "An argument to answer.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	w := postFork(t, handler, "reader-1", map[string]any{"client_request_id": "fork-2", "text": "answer it from the harbor's side", "source": sourceOf(v)})
	if w.Code != http.StatusAccepted {
		t.Fatalf("fork status=%d body=%s", w.Code, w.Body.String())
	}
	rev, err := core.Store().GetLifecycleRevision(t.Context(), "reader-1", "autoputer-test", decodeReceipt(t, w).RevisionID)
	if err != nil || len(revisionSourceLinks(rev)) != 1 {
		t.Fatalf("revision=%+v err=%v", rev, err)
	}
	if wakes() != 1 {
		t.Fatalf("wakes=%d, want 1", wakes())
	}
}

func TestForkRefusedSourceCreatesNoDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{}}
	_, _, docID, _, _, _ := textureCreateIdentity("reader-1", "autoputer-test", "fork-bad")
	for name, c := range map[string]struct {
		body map[string]any
		want int
	}{
		"unpublished source": {map[string]any{"client_request_id": "fork-bad", "source": map[string]string{"publication_version_id": "nope", "manifest_hash": "x"}}, http.StatusUnprocessableEntity},
		"no request id":      {map[string]any{"source": map[string]string{"publication_version_id": "nope", "manifest_hash": "x"}}, http.StatusBadRequest},
		"no source":          {map[string]any{"client_request_id": "fork-bad"}, http.StatusBadRequest},
		"identity in body":   {map[string]any{"client_request_id": "fork-bad", "owner_id": "x", "source": map[string]string{"publication_version_id": "nope", "manifest_hash": "x"}}, http.StatusBadRequest},
	} {
		if w := postFork(t, handler, "reader-1", c.body); w.Code != c.want {
			t.Errorf("%s: status=%d body=%s, want %d", name, w.Code, w.Body.String(), c.want)
		}
	}
	if _, err := core.Store().GetLifecycleDocument(t.Context(), "reader-1", "autoputer-test", docID); err == nil {
		t.Fatal("a refused fork created its document")
	}
}
