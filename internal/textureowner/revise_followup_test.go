package textureowner

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The payloads the real clients send, read from the code:
//
//	frontend/src/packages/texture/texture.js submitAgentRevision (App.svelte):
//	    /revise  {intent}
//	cmd/choir texture revise (commands.go):
//	    /revise  {prompt, client_request_id?, expected_head_revision_id?}
//	frontend/tests/texture-owner-revision-deployed.spec.js, ...source-ref-live-agent.spec.js:
//	    /revise  {prompt, intent, client_request_id}
//	frontend/src/packages/texture/texture.js createRevision (the editor save):
//	    /revisions  {content, body_doc?, source_entities?, citations?, metadata?,
//	                 parent_revision_id?, allow_rebase?, idempotency_key?,
//	                 expected_lifecycle_version?}
//	frontend/src/lib/conductor.js (the prompt bar): /api/prompt-bar, whose own
//	    strict struct is unchanged; a focused submit reaches reviseDocument in-process.
func TestReviseRefusesNoRealClientPayload(t *testing.T) {
	reviseShapes := map[string]string{
		"frontend submitAgentRevision": `{"intent":"revise"}`,
		"cmd/choir texture revise":     `{"prompt":"tighten","client_request_id":"r-1","expected_head_revision_id":"h"}`,
		"cmd/choir, prompt only":       `{"prompt":"tighten"}`,
		"deployed specs":               `{"prompt":"expand","intent":"revise","client_request_id":"m1"}`,
	}
	for name, body := range reviseShapes {
		var wire reviseWireRequest
		dec := json.NewDecoder(strings.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&wire); err != nil {
			t.Errorf("/revise refuses the %s payload %s: %v", name, body, err)
		} else if req := wire.request("o", "d", wire.ClientRequestID); strings.TrimSpace(req.Text) == "" {
			t.Errorf("/revise read no text from the %s payload", name)
		}
	}
	editorShapes := map[string]string{
		"editor save, full": `{"content":"c","body_doc":{},"source_entities":[],"citations":[],"metadata":{"a":1},"parent_revision_id":"p","allow_rebase":false,"idempotency_key":"k","expected_lifecycle_version":3}`,
		"initial version":   `{"content":"c","metadata":{"created_from":"conductor"},"idempotency_key":"k","expected_lifecycle_version":1,"parent_revision_id":"p"}`,
		"derivative":        `{"content":"c","citations":[],"metadata":{}}`,
	}
	for name, body := range editorShapes {
		var req textureCreateRevisionRequest
		dec := json.NewDecoder(strings.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			t.Errorf("/revisions refuses the %s payload %s: %v", name, body, err)
		}
	}
}

func TestReviseAcceptsTheClientPayloadsOverHTTP(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	countWakes(core)
	doc := start.InitialDocument.DocID
	head := start.InitialRevision.RevisionID
	if w := postRevise(t, handler, doc, start.OwnerID, json.RawMessage(`{"prompt":"tighten","client_request_id":"cmd-1","expected_head_revision_id":"`+head+`"}`)); w.Code != http.StatusAccepted {
		t.Fatalf("cmd/choir payload status=%d body=%s", w.Code, w.Body.String())
	}
	if w := postRevise(t, handler, doc, start.OwnerID, json.RawMessage(`{"intent":"revise"}`)); w.Code != http.StatusAccepted {
		t.Fatalf("frontend payload status=%d body=%s", w.Code, w.Body.String())
	}
}

// R11: an in-process caller (a desk through choir.ProductAPI has an empty
// RemoteAddr) is refused by every entry point of the call, and nothing is written.
func TestReviseEntryPointsRefuseAnInProcessDesk(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	wakes := countWakes(core)
	v := testVersion("pub-r11-v1", "Body.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	post := func(path, body string, handle func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
		req.RemoteAddr = ""
		req.Header.Set("X-Authenticated-User", start.OwnerID)
		w := httptest.NewRecorder()
		handle(w, req)
		return w
	}
	if w := post("/api/texture/documents/"+start.InitialDocument.DocID+"/revise", `{"client_request_id":"d1","text":"hello"}`, handler.HandleTextureRouter); w.Code != http.StatusForbidden {
		t.Fatalf("/revise by a desk = %d %s, want 403", w.Code, w.Body.String())
	}
	if w := post("/api/texture/fork", `{"client_request_id":"d2","source":{"publication_version_id":"pub-r11-v1","manifest_hash":"manifest-pub-r11-v1"}}`, handler.HandleTextureRouter); w.Code != http.StatusForbidden {
		t.Fatalf("/fork by a desk = %d %s, want 403", w.Code, w.Body.String())
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.HeadRevision.RevisionID != start.InitialRevision.RevisionID || wakes() != 0 {
		t.Fatalf("a refused desk call wrote: head=%s wakes=%d err=%v", snapshot.HeadRevision.RevisionID, wakes(), err)
	}
}

func TestForkClosesItsInitialWorkOnlyWhenNoDeskWillServeIt(t *testing.T) {
	core, handler := testAPISetup(t)
	countWakes(core)
	v := testVersion("pub-w-v1", "Fork me.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	openItems := func(w *httptest.ResponseRecorder) (open, completed int) {
		t.Helper()
		if w.Code != http.StatusAccepted {
			t.Fatalf("fork status=%d body=%s", w.Code, w.Body.String())
		}
		receipt := decodeReceipt(t, w)
		snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), "reader-1", "autoputer-test", receipt.TrajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range snapshot.WorkItems {
			switch item.Status {
			case types.WorkItemOpen:
				open++
			case types.WorkItemCompleted:
				completed++
			}
		}
		return open, completed
	}
	plain := map[string]any{"client_request_id": "fork-w1", "source": sourceOf(v)}
	if open, done := openItems(postFork(t, handler, "reader-1", plain)); open != 0 || done != 1 {
		t.Fatalf("verbatim fork: open=%d completed=%d, want 0 and 1", open, done)
	}
	// A retry settles once and still answers.
	if open, done := openItems(postFork(t, handler, "reader-1", plain)); open != 0 || done != 1 {
		t.Fatalf("verbatim fork retry: open=%d completed=%d", open, done)
	}
	withText := map[string]any{"client_request_id": "fork-w2", "text": "answer it", "source": sourceOf(v)}
	if open, _ := openItems(postFork(t, handler, "reader-1", withText)); open != 1 {
		t.Fatalf("fork with text: open=%d, want the desk's item open", open)
	}
}

// The desk's Write on an owner revision that carries source links stamps the
// same links on its own revision, through the real commit; a second Write on
// the desk's own revision does not pass them on again.
func TestDeskWriteInheritsTheRequestRevisionsLinks(t *testing.T) {
	core, h := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	s := core.Store()
	link := types.ParentLink{Kind: types.ParentLinkSource, PublicationVersionID: "pub-i-v1", ManifestHash: "mh", RoutePath: "/pub/texture/i/v1", AuthorLabel: "Autopaper"}
	prov, err := types.Provenance{SchemaVersion: types.ProvenanceSchemaVersion, ParentLinks: []types.ParentLink{{Kind: types.ParentLinkBase, RevisionID: "revision-cell-author-base-inherit"}, link}}.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	rec := seedCellAuthorLifecycle(t, s, "owner-cell-author", "inherit", "# Merger\n\nOpening line.\n\nClosing line.", func(r *types.Revision) { r.Provenance = prov })
	h.createAgentMutationForRun(context.Background(), &rec)
	base := headOf(t, s, rec)
	turn := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Done: "answers the owner request with the version merged in", Write: &yaegikernel.TextureWrite{Edits: "@replace b2\nOpening line, revised with the version.", Why: "merge"}})
	if _, err := h.CommitCellTextureAuthor(context.Background(), &rec, turn, "inherit-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	desk := headOf(t, s, rec)
	if desk.AuthorKind != types.AuthorAppAgent {
		t.Fatalf("head author=%s", desk.AuthorKind)
	}
	var got types.Provenance
	if err := json.Unmarshal(desk.Provenance, &got); err != nil || len(got.ParentLinks) != 2 {
		t.Fatalf("desk provenance=%s err=%v, want the request's two links", desk.Provenance, err)
	}
	found := false
	for _, l := range got.ParentLinks {
		found = found || l == link
	}
	if !found {
		t.Fatalf("links=%+v, want %+v", got.ParentLinks, link)
	}
	// The desk's own revision is not a trigger: as a base it passes no links
	// on. (The Done above settled the request, so the next desk turn is a new
	// owner revise, not a second Write on this one.)
	if again := inheritParentLinks(desk.Provenance, desk); string(again) != string(desk.Provenance) {
		t.Fatalf("a desk-authored base passed links on: %s", again)
	}
}
