package textureowner

// FE-02 step 1 (docs/problems/prompt-bar-always-creates-a-new-document-2026-10-26.md):
// a prompt-bar submit with continue_doc_id builds on that document through
// the owner revise instead of creating a new one. The failure modes P1-P10
// are named in the problem doc; each test names the ones it covers.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// postPromptBar posts body to /api/prompt-bar as owner. inProcess clears
// RemoteAddr, the marker of a desk's in-process ProductAPI call.
func postPromptBar(t *testing.T, handler *Handler, owner string, body map[string]string, inProcess bool) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal prompt-bar body: %v", err)
	}
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", string(raw), owner)
	if inProcess {
		req.RemoteAddr = ""
	}
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	return w
}

// conductorRunCount counts the owner's conductor runs: a prompt-bar
// continue records none (P1).
func conductorRunCount(t *testing.T, core *agentcore.Runtime, owner string) int {
	t.Helper()
	runs, err := core.Store().ListRunsByOwner(t.Context(), owner, 1000)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	n := 0
	for _, run := range runs {
		if run.AgentProfile == agentprofile.Conductor {
			n++
		}
	}
	return n
}

func ownerTrajectoryCount(t *testing.T, s *store.Store, owner, computerID string) int {
	t.Helper()
	all, err := s.ListLifecycleTrajectoriesByOwner(t.Context(), owner, computerID, 1000)
	if err != nil {
		t.Fatalf("list trajectories: %v", err)
	}
	return len(all)
}

// continueReceipt is the 202 a prompt-bar continue answers with, as the
// frontend reads it.
type continueReceipt struct {
	Schema       string `json:"schema"`
	Continued    bool   `json:"continued"`
	CommandID    string `json:"command_id"`
	DocID        string `json:"doc_id"`
	TrajectoryID string `json:"trajectory_id"`
	RevisionID   string `json:"revision_id"`
	RequestID    string `json:"request_id"`
	Replay       bool   `json:"replay"`
}

func decodeContinueReceipt(t *testing.T, w *httptest.ResponseRecorder) continueReceipt {
	t.Helper()
	var receipt continueReceipt
	if err := json.NewDecoder(w.Body).Decode(&receipt); err != nil {
		t.Fatalf("decode continue receipt: %v", err)
	}
	if receipt.Schema != "choir.prompt_bar_continue.v1" || !receipt.Continued || receipt.RequestID == "" {
		t.Fatalf("continue receipt=%+v, want schema choir.prompt_bar_continue.v1, continued, a request id", receipt)
	}
	return receipt
}

// P1, P2, P7: a continue on a live document commits one owner revision on
// its head carrying the owner's text unchanged and wakes the desk; no new
// document, trajectory or conductor run.
func TestPromptBarContinueRevisesLiveDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	mu, dispatches := recordContinueDispatches(core)
	trajectoriesBefore := ownerTrajectoryCount(t, s, start.OwnerID, start.ComputerID)

	// Backticks, brackets, a blank line and non-ASCII: the shapes SD-07 saw
	// retyped. Outer whitespace is trimmed as the new-document path trims it.
	typed := "  build on this: add `costs` [1]\n\n- keep «quotes» and café  \n"
	w := postPromptBar(t, handler, start.OwnerID, map[string]string{"text": typed, "command_id": "continue-live", "continue_doc_id": docID}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("P2: continue status=%d body=%s", w.Code, w.Body.String())
	}
	receipt := decodeContinueReceipt(t, w)
	if receipt.DocID != docID || receipt.TrajectoryID != start.TrajectoryID || receipt.RevisionID == "" || receipt.CommandID != "continue-live" || receipt.Replay {
		t.Fatalf("P2: receipt=%+v, want doc %s on %s", receipt, docID, start.TrajectoryID)
	}

	snapshot, err := s.GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	head := snapshot.HeadRevision
	if head.RevisionID != receipt.RevisionID || head.AuthorKind != types.AuthorUser || head.ParentRevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("P2: head=%+v, want owner revision %s on %s", head, receipt.RevisionID, start.InitialRevision.RevisionID)
	}
	var metadata map[string]any
	if err := json.Unmarshal(head.Metadata, &metadata); err != nil {
		t.Fatal(err)
	}
	if got, want := metadata["owner_prompt"], strings.TrimSpace(typed); got != want {
		t.Fatalf("P7: owner_prompt=%q, want %q", got, want)
	}
	if metadata["input_origin"] != textureInputOriginUserPrompt {
		t.Fatalf("P2: input_origin=%v", metadata["input_origin"])
	}
	mu.Lock()
	woke := false
	for _, d := range *dispatches {
		if occurrence, err := agentcore.DecodeTextureActorOccurrence(d.content); err == nil && occurrence.HeadRevisionID == receipt.RevisionID {
			woke = true
		}
	}
	mu.Unlock()
	if !woke {
		t.Fatalf("P2: no owner-revision wake: %+v", *dispatches)
	}

	if got := ownerTrajectoryCount(t, s, start.OwnerID, start.ComputerID); got != trajectoriesBefore {
		t.Fatalf("P1: trajectories %d -> %d, want no new document", trajectoriesBefore, got)
	}
	if got := conductorRunCount(t, core, start.OwnerID); got != 0 {
		t.Fatalf("P1: a continue recorded %d conductor runs", got)
	}
}

// P3: a continue on a settled document moves it to a new live trajectory
// (F1) and the receipt names that trajectory.
func TestPromptBarContinueSettledDocumentStartsNextTrajectory(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	recordContinueDispatches(core)
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)

	w := postPromptBar(t, handler, start.OwnerID, map[string]string{"text": "next episode: add a section on costs", "command_id": "continue-settled", "continue_doc_id": docID}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("P3: continue on a settled document status=%d body=%s", w.Code, w.Body.String())
	}
	receipt := decodeContinueReceipt(t, w)
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.TrajectoryID == start.TrajectoryID || receipt.TrajectoryID != doc.TrajectoryID {
		t.Fatalf("P3: document on %s, receipt on %s, settled %s", doc.TrajectoryID, receipt.TrajectoryID, start.TrajectoryID)
	}
	next, err := s.GetLifecycleTrajectory(t.Context(), start.OwnerID, start.ComputerID, doc.TrajectoryID)
	if err != nil || next.Status != types.TrajectoryLive || next.SubjectRefs[continuesTrajectoryRef] != start.TrajectoryID {
		t.Fatalf("P3: next trajectory=%+v err=%v", next, err)
	}
	if doc.CurrentRevisionID != receipt.RevisionID {
		t.Fatalf("P3: head %s, receipt revision %s", doc.CurrentRevisionID, receipt.RevisionID)
	}
	if got := documentTrajectories(t, s, start.OwnerID, start.ComputerID, docID); len(got) != 2 {
		t.Fatalf("P3: document has %d trajectories, want 2", len(got))
	}
}

// P4: another owner's document answers exactly as a missing one does, and
// nothing moves.
func TestPromptBarContinueRefusesAnotherOwnersDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	mu, dispatches := recordContinueDispatches(core)

	other := postPromptBar(t, handler, "user-mallory", map[string]string{"text": "rewrite it my way", "command_id": "steal", "continue_doc_id": docID}, false)
	missing := postPromptBar(t, handler, "user-mallory", map[string]string{"text": "rewrite it my way", "command_id": "missing", "continue_doc_id": "doc-that-does-not-exist"}, false)
	if other.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || other.Body.String() != missing.Body.String() {
		t.Fatalf("P4: other owner's doc %d %s; missing doc %d %s; want the same 404", other.Code, other.Body.String(), missing.Code, missing.Body.String())
	}
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.CurrentRevisionID != start.InitialRevision.RevisionID || doc.TrajectoryID != start.TrajectoryID {
		t.Fatalf("P4: document moved: head %s trajectory %s", doc.CurrentRevisionID, doc.TrajectoryID)
	}
	if got := ownerTrajectoryCount(t, s, "user-mallory", start.ComputerID); got != 0 {
		t.Fatalf("P4: a refused continue started %d trajectories for the caller", got)
	}
	if got := conductorRunCount(t, core, "user-mallory"); got != 0 {
		t.Fatalf("P4: a refused continue recorded %d conductor runs", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*dispatches) != 0 {
		t.Fatalf("P4: refused continue dispatched %+v", *dispatches)
	}
}

// P5: a cancelled document is not continued.
func TestPromptBarContinueRefusesCancelledDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	mu, dispatches := recordContinueDispatches(core)
	snapshot, err := s.GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "cancel-before-prompt-bar-continue",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, Reason: "owner stopped the work",
	}
	cancel.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancel)
	if _, err := s.CancelLifecycleTrajectory(t.Context(), cancel); err != nil {
		t.Fatal(err)
	}

	w := postPromptBar(t, handler, start.OwnerID, map[string]string{"text": "continue anyway", "command_id": "after-cancel", "continue_doc_id": docID}, false)
	if w.Code != http.StatusConflict {
		t.Fatalf("P5: continue on a cancelled document status=%d body=%s", w.Code, w.Body.String())
	}
	if got := documentTrajectories(t, s, start.OwnerID, start.ComputerID, docID); len(got) != 1 {
		t.Fatalf("P5: cancelled document has %d trajectories", len(got))
	}
	if got := conductorRunCount(t, core, start.OwnerID); got != 0 {
		t.Fatalf("P5: a refused continue recorded %d conductor runs", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*dispatches) != 0 {
		t.Fatalf("P5: cancelled document dispatched %+v", *dispatches)
	}
}

// P6: without continue_doc_id the prompt bar still creates a new document
// through the conductor, and leaves an existing document alone.
func TestPromptBarWithoutContinueCreatesNewDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)

	w := postPromptBar(t, handler, start.OwnerID, map[string]string{"text": "write one short sentence about tides", "command_id": "new-doc"}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("P6: submit status=%d body=%s", w.Code, w.Body.String())
	}
	var raw map[string]any
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["continued"]; ok {
		t.Fatalf("P6: a plain submit answered as a continue: %v", raw)
	}
	if raw["schema"] != types.DurableWorkSchemaV1 || raw["submission_id"] == "" || raw["doc_id"] == "" || raw["doc_id"] == start.InitialDocument.DocID {
		t.Fatalf("P6: plain submit response=%v, want a new conductor document", raw)
	}
	if got := conductorRunCount(t, core, start.OwnerID); got != 1 {
		t.Fatalf("P6: conductor runs=%d, want 1", got)
	}
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil || doc.CurrentRevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("P6: the existing document moved: %+v err=%v", doc, err)
	}
}

// P8: a retried submit with the same command_id commits no second
// revision. The revise it reuses does not replay a commit whose head has
// already moved (its head check answers first, as /revise does today), so
// the retry is a replay or a 409; either way the head stays the first
// revision.
func TestPromptBarContinueRetryCommitsOneRevision(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	recordContinueDispatches(core)
	body := map[string]string{"text": "add a summary", "command_id": "continue-retry", "continue_doc_id": docID}

	first := postPromptBar(t, handler, start.OwnerID, body, false)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	firstReceipt := decodeContinueReceipt(t, first)
	retry := postPromptBar(t, handler, start.OwnerID, body, false)
	switch retry.Code {
	case http.StatusAccepted:
		if receipt := decodeContinueReceipt(t, retry); receipt.RevisionID != firstReceipt.RevisionID || !receipt.Replay {
			t.Fatalf("P8: accepted retry receipt=%+v, want a replay of %s", receipt, firstReceipt.RevisionID)
		}
	case http.StatusConflict:
	default:
		t.Fatalf("P8: retry status=%d body=%s", retry.Code, retry.Body.String())
	}
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil || doc.CurrentRevisionID != firstReceipt.RevisionID {
		t.Fatalf("P8: head=%s err=%v, want %s", doc.CurrentRevisionID, err, firstReceipt.RevisionID)
	}
}

// P9: a desk's in-process call cannot write an owner revision through the
// prompt bar; product_api already refuses a desk any Texture write.
func TestPromptBarContinueRefusesInProcessDesk(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	mu, dispatches := recordContinueDispatches(core)

	w := postPromptBar(t, handler, start.OwnerID, map[string]string{"text": "revise it for the owner", "command_id": "desk-continue", "continue_doc_id": docID}, true)
	if w.Code != http.StatusForbidden {
		t.Fatalf("P9: in-process continue status=%d body=%s", w.Code, w.Body.String())
	}
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil || doc.CurrentRevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("P9: the document moved: %+v err=%v", doc, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*dispatches) != 0 {
		t.Fatalf("P9: refused continue dispatched %+v", *dispatches)
	}
}
