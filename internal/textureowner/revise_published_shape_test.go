package textureowner

import (
	"errors"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// A published story is a structured body (source_ref nodes), its source
// entities and a text with numbered footnotes; the test versions elsewhere are
// plain text. These tests take and fork a version of the paper's shape, with
// synthetic words, on a store whose vocabulary cutover is on (staging's state).
// Failure modes listed before the fix:
//  1. a fork with text starts the document from the version's text, whose
//     "[1]" footnote the strict authoring rules refuse when the merge revision
//     carries that text's body_doc forward (500, "failed to commit owner
//     revision");
//  2. a take or a verbatim fork of the structured body is refused or not under
//     the cutover guard (500 "failed to commit lifecycle revision");
//  3. the 500 said nothing of why.
const publishedShapeBodyDoc = `{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"doc-pub"},"content":[
{"type":"heading","attrs":{"id":"h-1","level":1},"content":[{"type":"text","text":"Harbor opens at dawn"}]},
{"type":"paragraph","attrs":{"id":"p-1"},"content":[{"type":"text","text":"The harbor opens at dawn, the board said."},
{"type":"source_ref","attrs":{"id":"ref-1","display_mode":"numbered_ref","quote_tier":"exact","source_entity_id":"src_1","ref":{"cid":"c1","doc":"obj:choir.web_capture:owner:key-1","hash":"sha256:aa","quote":"opens at dawn","span":{"start":0,"end":13},"version":"sha256:bb","source_entity_id":"src_1"}}}]}]}}`

const publishedShapeEntities = `[{"source_entity_id":"src_1","target":{"kind":"web_url","uri":"https://example.test/harbor","id":"obj:choir.web_capture:owner:key-1","metadata":{"capture_doc":"obj:choir.web_capture:owner:key-1","capture_version":"sha256:bb"}},"selectors":[{"kind":"whole_resource"}],"display":{"mode":"numbered_ref","title":"Harbor board notice"},"evidence":{"state":"available","open_surface":"source"},"provenance":{"created_by":"texture_write"}}]`

func publishedShapeVersion(id string) PublicationVersion {
	v := testVersion(id, "")
	v.BodyDoc = []byte(publishedShapeBodyDoc)
	v.SourceEntities = []byte(publishedShapeEntities)
	return v
}

func TestTakeAndVerbatimForkOfAStructuredVersionAfterCutover(t *testing.T) {
	core, handler := testAPISetup(t)
	if _, err := core.Store().MigrateVocabularyToV2(t.Context()); err != nil {
		t.Fatalf("cutover: %v", err)
	}
	start := startObservationLifecycle(t, core.Store())
	v := publishedShapeVersion("pub-shape-v3")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}

	w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{"client_request_id": "take-shape", "sources": []any{sourceOf(v)}})
	if w.Code != http.StatusAccepted {
		t.Fatalf("take status=%d body=%s", w.Code, w.Body.String())
	}
	// The published text is the body's projection; read it back from the take.
	taken, err := core.Store().GetLifecycleRevision(t.Context(), start.OwnerID, start.ComputerID, decodeReceipt(t, w).RevisionID)
	if err != nil || taken.Content == "" {
		t.Fatalf("taken revision=%+v err=%v", taken, err)
	}
	v.Content = taken.Content
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	if w := postFork(t, handler, "reader-1", map[string]any{"client_request_id": "fork-shape", "source": sourceOf(v)}); w.Code != http.StatusAccepted {
		t.Fatalf("verbatim fork status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestForkWithTextOfAVersionWhoseTextHasNumberedFootnotes(t *testing.T) {
	core, handler := testAPISetup(t)
	if _, err := core.Store().MigrateVocabularyToV2(t.Context()); err != nil {
		t.Fatalf("cutover: %v", err)
	}
	v := testVersion("pub-shape-fn-v1", "The harbor opens at dawn [1], the board said [2].\n\n[1] Harbor board notice\n[2] Port authority")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	w := postFork(t, handler, "reader-1", map[string]any{"client_request_id": "fork-fn", "text": "answer it from the harbor's side", "source": sourceOf(v)})
	if w.Code != http.StatusAccepted {
		t.Fatalf("fork with text status=%d body=%s", w.Code, w.Body.String())
	}
	// A comment on the forked document carries the same text forward again.
	receipt := decodeReceipt(t, w)
	rev, err := core.Store().GetLifecycleRevision(t.Context(), "reader-1", "autoputer-test", receipt.RevisionID)
	if err != nil || rev.Content != v.Content {
		t.Fatalf("revision=%+v err=%v, want the source text carried verbatim", rev, err)
	}
}

func TestACommitRefusedAsInvalidSaysWhy(t *testing.T) {
	err := refuseCommit(errors.Join(store.ErrInvalidTextureRevision, errors.New("doc is empty")), "failed to commit lifecycle revision")
	var refusal *ownerRevisionRefusal
	if !errors.As(err, &refusal) || refusal.status != http.StatusUnprocessableEntity {
		t.Fatalf("err=%v, want a 422 refusal", err)
	}
	if refusal.message == "failed to commit lifecycle revision" {
		t.Fatalf("message=%q names no cause", refusal.message)
	}
	other := refuseCommit(errors.New("disk full"), "failed to commit lifecycle revision")
	if !errors.As(other, &refusal) || refusal.status != http.StatusInternalServerError || refusal.message == "failed to commit lifecycle revision" {
		t.Fatalf("other=%v, want a 500 that names its cause", other)
	}
}
