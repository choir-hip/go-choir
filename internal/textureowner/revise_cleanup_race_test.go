package textureowner

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func settledObservationDoc(t *testing.T) (*Handler, *store.Store, types.StartLifecycleRequest) {
	t.Helper()
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s, func(req *types.StartLifecycleRequest) {
		req.InitialRevision.Metadata = json.RawMessage(`{"trajectory_id":"` + req.TrajectoryID + `"}`)
	})
	countWakes(core)
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, start.InitialDocument.DocID, start.TrajectoryID)
	return handler, s, start
}

func workStatus(t *testing.T, s *store.Store, start types.StartLifecycleRequest, workID string) types.WorkItemStatus {
	t.Helper()
	item, err := s.GetLifecycleWorkItem(t.Context(), start.OwnerID, start.ComputerID, workID)
	if err != nil {
		t.Fatal(err)
	}
	return item.Status
}

// Two revises race on a settled document. The continue's work item is shared
// (it is deterministic per settled trajectory), so only the call that created
// it may claim it, and no cleanup may refuse it after a lost race: the
// winner's committed revision keeps its desk turn.
func TestOnlyTheCallThatCreatedTheContinueOwnsItsWork(t *testing.T) {
	handler, s, start := settledObservationDoc(t)
	docID := start.InitialDocument.DocID
	settled, err := s.GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	winnerDoc, winnerSnap, created, err := handler.continueSettledDocumentCreating(t.Context(), start.OwnerID, doc, settled)
	if err != nil || !created {
		t.Fatalf("first continue created=%v err=%v", created, err)
	}
	_, joinedSnap, joinedCreated, err := handler.continueSettledDocumentCreating(t.Context(), start.OwnerID, doc, settled)
	if err != nil || joinedCreated {
		t.Fatalf("the joining continue created=%v err=%v, want it to own nothing", joinedCreated, err)
	}
	_, workID, _ := ownerRevisionTarget(winnerSnap, docID)
	if _, joinedID, _ := ownerRevisionTarget(joinedSnap, docID); joinedID != workID || workID == "" {
		t.Fatalf("the continue's item differs between callers: %q %q", workID, joinedID)
	}

	// The loser's cleanup after a lost race, and after the head moved, leaves
	// the winner's open item alone.
	loser := openedWork{Doc: winnerDoc, WorkItemID: workID, HeadRevisionID: winnerSnap.Document.CurrentRevisionID}
	handler.closeOpenedWork(t.Context(), start.OwnerID, loser, refuseOwnerRevision(http.StatusConflict, "document head changed"))
	handler.closeOpenedWork(t.Context(), start.OwnerID, loser, store.ErrConcurrentStateChange)
	if got := workStatus(t, s, start, workID); got != types.WorkItemOpen {
		t.Fatalf("a lost race closed the winner's work item: %s", got)
	}
	moved := loser
	moved.HeadRevisionID = "a-head-this-call-saw-before-the-winner-committed"
	handler.closeOpenedWork(t.Context(), start.OwnerID, moved, errors.New("failed to commit owner revision"))
	if got := workStatus(t, s, start, workID); got != types.WorkItemOpen {
		t.Fatalf("a cleanup against a moved head closed the item: %s", got)
	}
	// A real failure with the head unchanged does close what this call made.
	handler.closeOpenedWork(t.Context(), start.OwnerID, loser, errors.New("failed to commit owner revision"))
	if got := workStatus(t, s, start, workID); got != types.WorkItemRefused {
		t.Fatalf("a real failure left the item %s", got)
	}
}

// The winner and a later revise that joins its continue both commit; neither
// closes the other's desk work.
func TestSequentialRevisesOnASettledDocumentKeepTheDesksWorkOpen(t *testing.T) {
	handler, s, start := settledObservationDoc(t)
	docID := start.InitialDocument.DocID
	w := postRevise(t, handler, docID, start.OwnerID, map[string]any{"client_request_id": "race-a", "text": "first"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", w.Code, w.Body.String())
	}
	receipt := decodeReceipt(t, w)
	if b := postRevise(t, handler, docID, start.OwnerID, map[string]any{"client_request_id": "race-b", "text": "second"}); b.Code != http.StatusAccepted && b.Code != http.StatusConflict {
		t.Fatalf("second status=%d body=%s", b.Code, b.Body.String())
	}
	snapshot, err := s.GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, receipt.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if _, id, _ := ownerRevisionTarget(snapshot, docID); id == "" {
		t.Fatalf("the desk has no open work after two revises: %+v", snapshot.WorkItems)
	}
}
