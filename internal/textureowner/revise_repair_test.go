package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// A take is the pinned published body exactly. The editor's markdown
// stabilizer repairs text an owner typed over a table; it would splice the
// head's table into a taken body that lacks one, so a take bypasses it.
func TestTakeWritesTheSourceBodyByteForByteOverATableHead(t *testing.T) {
	core, handler := testAPISetup(t)
	const headText = "| Name | Qty |\n| --- | --- |\n| Ferries | 4 |\n\n## Notes\n\nOld."
	start := startObservationLifecycle(t, core.Store())
	countWakes(core)
	// The owner's own edit puts a table in the head.
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{
		"client_request_id": "table-head", "base_revision_id": snapshot.HeadRevision.RevisionID,
		"body": map[string]any{"content": headText, "expected_lifecycle_version": snapshot.Trajectory.LifecycleVersion},
	}); w.Code != http.StatusCreated && w.Code != http.StatusAccepted {
		t.Fatalf("table head status=%d body=%s", w.Code, w.Body.String())
	}
	const body = "Harbor opens at dawn.\n\n## Notes\n\nNew."
	if out, changed := stabilizeTextureUserMarkdownStructures(headText, body); !changed || out == body {
		t.Fatalf("the test needs a body the stabilizer would change; it did not (%q)", out)
	}
	v := testVersion("pub-t-v1", body)
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{"client_request_id": "take-table", "sources": []any{sourceOf(v)}})
	if w.Code != http.StatusAccepted {
		t.Fatalf("take status=%d body=%s", w.Code, w.Body.String())
	}
	rev, err := core.Store().GetLifecycleRevision(t.Context(), start.OwnerID, start.ComputerID, decodeReceipt(t, w).RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if rev.Content != body {
		t.Fatalf("take wrote %q, want the source body %q exactly", rev.Content, body)
	}
	var meta map[string]any
	if err := json.Unmarshal(rev.Metadata, &meta); err != nil || meta["texture_structure_stabilized"] != nil {
		t.Fatalf("a take recorded a stabilization: %s", rev.Metadata)
	}
}

func TestTakeOnSettledDocumentClosesTheWorkItsContinueOpened(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s, func(req *types.StartLifecycleRequest) {
		req.InitialRevision.Metadata = json.RawMessage(`{"trajectory_id":"` + req.TrajectoryID + `"}`)
	})
	docID := start.InitialDocument.DocID
	countWakes(core)
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	v := testVersion("pub-sw-v1", "Settled take.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	body := map[string]any{"client_request_id": "take-sw", "sources": []any{sourceOf(v)}}
	w := postRevise(t, handler, docID, start.OwnerID, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	receipt := decodeReceipt(t, w)
	snapshot, err := s.GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, receipt.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range snapshot.WorkItems {
		if item.Status == types.WorkItemOpen {
			t.Fatalf("work item %s left open by a take no desk will serve", item.WorkItemID)
		}
	}
	// A retry still answers.
	if again := postRevise(t, handler, docID, start.OwnerID, body); again.Code != http.StatusAccepted {
		t.Fatalf("retry status=%d body=%s", again.Code, again.Body.String())
	}
}

func TestForkWithTextRecordsForkedFromAndAnyRetryFindsItsRevision(t *testing.T) {
	core, handler := testAPISetup(t)
	countWakes(core)
	v := testVersion("pub-ff-v1", "Answer me.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	w := postFork(t, handler, "reader-1", map[string]any{"client_request_id": "fork-text", "text": "answer it", "source": sourceOf(v)})
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	rev, err := core.Store().GetLifecycleRevision(t.Context(), "reader-1", "autoputer-test", decodeReceipt(t, w).RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(rev.Metadata, &meta); err != nil || meta["forked_from"] == nil || meta[reviseClientIDMetadataKey] != "fork-text" {
		t.Fatalf("metadata=%s err=%v, want forked_from and the client request id", rev.Metadata, err)
	}
}

// A real owner revise opens no Cast round. The update is bracketed by its own
// events: started when the request revision commits, finished when the desk's
// answering revision commits, through the real commit and turn hook.
func TestOwnerReviseAnsweredByTheDeskYieldsStartedThenFinished(t *testing.T) {
	core, h := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	s := core.Store()
	wakes := countWakes(core)
	const suffix = "update"
	rec := seedCellAuthorLifecycleOn(t, s, "autoputer-test", "owner-cell-author", suffix, "# Merger\n\nOpening line.\n\nClosing line.")
	h.createAgentMutationForRun(context.Background(), &rec)
	doc := rec.ChannelID
	base := headOf(t, s, rec)

	w := postRevise(t, h, doc, rec.OwnerID, map[string]any{"client_request_id": "upd-1", "text": "tighten the opening"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("revise status=%d body=%s", w.Code, w.Body.String())
	}
	request := decodeReceipt(t, w)
	if kinds := changelogKinds(t, s, rec.OwnerID, rec.ComputerID); len(kinds) != 1 || kinds[0] != types.ChangelogUpdateStarted {
		t.Fatalf("after the revise: %v, want update_started", kinds)
	}
	wakesAfterRevise := wakes()

	// The desk answers the request revision with a Write.
	head := headOf(t, s, rec)
	if head.RevisionID != request.RevisionID || head.RevisionID == base.RevisionID {
		t.Fatalf("head=%s, want the request revision %s", head.RevisionID, request.RevisionID)
	}
	turn := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: head.RevisionID, Done: "answers the owner request", Write: &yaegikernel.TextureWrite{Edits: "@replace b2\nOpening line, tightened.", Why: "tighten"}})
	if _, err := h.CommitCellTextureAuthor(context.Background(), &rec, turn, "upd-turn-1"); err != nil {
		t.Fatalf("desk Write: %v", err)
	}
	desk := headOf(t, s, rec)
	events, err := s.ListChangelogEvents(t.Context(), rec.OwnerID, rec.ComputerID, types.ChangelogQuery{IncludeMinor: true})
	if err != nil || len(events) != 2 || events[0].Changelog.Kind != types.ChangelogUpdateStarted || events[1].Changelog.Kind != types.ChangelogUpdateFinished {
		t.Fatalf("changelog = %+v err=%v, want started then finished", events, err)
	}
	if events[1].Changelog.TapeRef == nil || events[1].Changelog.TapeRef.RevisionID != desk.RevisionID {
		t.Fatalf("finished names %+v, want the desk's revision %s", events[1].Changelog.TapeRef, desk.RevisionID)
	}
	// A desk revision that answers no owner request is not another update.
	// (The Done above settled the request; a second Write needs a new owner revise.)
	h.emitUpdateFinished(context.Background(), rec.OwnerID, types.Document{DocID: doc, ComputerID: rec.ComputerID}, desk, "some-later-revision")
	if kinds := changelogKinds(t, s, rec.OwnerID, rec.ComputerID); len(kinds) != 2 {
		t.Fatalf("a desk-authored base emitted %v", kinds)
	}
	if wakes() != wakesAfterRevise {
		t.Fatalf("changelog records woke %d more readers", wakes()-wakesAfterRevise)
	}
}
