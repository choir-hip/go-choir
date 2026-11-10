package textureowner

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// A fork whose first revision does not commit leaves no empty document and no
// open work item: the commit failure here is real (the store refuses the
// source's body), not injected.
func TestFailedForkCommitLeavesNoOpenWorkAndNoLiveDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	countWakes(core)
	bad := testVersion("pub-bad-v1", "A body the store refuses.")
	bad.BodyDoc = json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[{"type":"bogus"}]}}`)
	good := testVersion("pub-good-v1", "A good body.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{bad.PublicationVersionID: bad, good.PublicationVersionID: good}}
	_, _, docID, _, workID, trajectoryID := textureCreateIdentity("reader-1", "autoputer-test", "fork-fail")

	w := postFork(t, handler, "reader-1", map[string]any{"client_request_id": "fork-fail", "source": sourceOf(bad)})
	if w.Code < 400 {
		t.Fatalf("a fork whose commit the store refuses answered %d", w.Code)
	}
	doc, err := core.Store().GetLifecycleDocument(t.Context(), "reader-1", "autoputer-test", docID)
	if err != nil || doc.ArchivedAt == nil {
		t.Fatalf("the failed fork's document = %+v err=%v, want it archived", doc, err)
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), "reader-1", "autoputer-test", trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Trajectory.Status == types.TrajectoryLive {
		t.Fatalf("the failed fork's trajectory is still live")
	}
	found := false
	for _, item := range snapshot.WorkItems {
		if item.Status == types.WorkItemOpen {
			t.Fatalf("work item %s left open", item.WorkItemID)
		}
		if item.WorkItemID == workID {
			found = true
			if item.Status != types.WorkItemRefused || !strings.HasPrefix(item.Reason, UnattendedRequestReason) || !strings.Contains(item.Reason, "fork failed") {
				t.Fatalf("work fate = %s %q, want a visible refusal naming the failed fork", item.Status, item.Reason)
			}
		}
	}
	if !found {
		t.Fatal("the fork's work item is missing")
	}
	// The spent id answers 409; a new id forks the good source.
	if again := postFork(t, handler, "reader-1", map[string]any{"client_request_id": "fork-fail", "source": sourceOf(good)}); again.Code != http.StatusConflict {
		t.Fatalf("spent id status=%d body=%s, want 409", again.Code, again.Body.String())
	}
	if ok := postFork(t, handler, "reader-1", map[string]any{"client_request_id": "fork-fail-2", "source": sourceOf(good)}); ok.Code != http.StatusAccepted {
		t.Fatalf("new id status=%d body=%s", ok.Code, ok.Body.String())
	}
}

// A retry of a fork that already committed is never undone by a later failure.
func TestAbandonForkLeavesACommittedForkAlone(t *testing.T) {
	core, handler := testAPISetup(t)
	countWakes(core)
	v := testVersion("pub-keep-v1", "Keep me.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	w := postFork(t, handler, "reader-1", map[string]any{"client_request_id": "fork-keep", "source": sourceOf(v)})
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	receipt := decodeReceipt(t, w)
	_, _, _, _, workID, _ := textureCreateIdentity("reader-1", "autoputer-test", "fork-keep")
	handler.abandonFork(t.Context(), "reader-1", textureLifecycleCreateResponse{DocID: receipt.DocID, RevisionID: "the-initial-revision", TargetWorkItemID: workID}, errors.New("later transient failure"))
	doc, err := core.Store().GetLifecycleDocument(t.Context(), "reader-1", "autoputer-test", receipt.DocID)
	if err != nil || doc.ArchivedAt != nil {
		t.Fatalf("a committed fork was archived: %+v err=%v", doc, err)
	}
}

// Work a failed revise opened is closed with a visible fate, and the owner's
// next revise reopens work.
func TestWorkOpenedByAFailedReviseIsClosedAndReopenable(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	countWakes(core)
	doc := start.InitialDocument
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	_, workID, _ := ownerRevisionTarget(snapshot, doc.DocID)
	if workID == "" {
		t.Fatal("the fixture has no open desk work item")
	}
	live, err := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, doc.DocID)
	if err != nil {
		t.Fatal(err)
	}
	handler.closeOpenedWork(t.Context(), start.OwnerID, openedWork{Doc: live, WorkItemID: workID}, errors.New("failed to commit owner revision"))
	after, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range after.WorkItems {
		if item.WorkItemID == workID && (item.Status != types.WorkItemRefused || !strings.HasPrefix(item.Reason, UnattendedRequestReason)) {
			t.Fatalf("work fate = %s %q, want a refusal the next revise reopens", item.Status, item.Reason)
		}
	}
	if w := postRevise(t, handler, doc.DocID, start.OwnerID, map[string]any{"client_request_id": "after-failure", "text": "try again"}); w.Code != http.StatusAccepted {
		t.Fatalf("the revise after a closed item status=%d body=%s", w.Code, w.Body.String())
	}
}
