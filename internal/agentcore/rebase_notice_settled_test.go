package agentcore

// Problem 21 (docs/problems/selfdev-round-blockers-cluster-2026-11-03.md):
// with no live root, a stack on a moved base got no notice once its
// operation's document Texture had settled its work item (pool: minted at
// 10:45Z inside the round, rebase_needs_root from 11:33Z on). The notice is
// the wake: a Texture turn for unread records needs no work item (the records
// route), so the fallback no longer asks for one and opens none. Ways it can
// fail, written before the code:
//   1. a settled document Texture still gets nothing -> ...ASettledTextureGetsTheNotice
//   2. work is opened for a notice: an item reused stale by a later attempt,
//      or left open with no notice when the commit fails (review p21-r1,
//      DeepSeek F1 and F2) -> ...OpensNoWork (attempt 2 with attempt 1's
//      item open, a failed commit, an owner retry refused)
//   3. a Texture that cannot act (trajectory cancelled, no document) gets the
//      notice, or the reason is lost -> ...SaysWhyNot
//   4. a live root is passed over -> TestRebaseNoticeFallbackPrefersTheRoot
//   5. a Cast's refusal still says rebase_needs_root for a settled Texture
//      -> ...ASettledTextureGetsTheNotice
//   6. the document moved to a newer trajectory (an owner revision on its
//      settled trajectory continues it) and the fallback still reads the
//      operation's old one (pool 13:58Z) -> ...FollowsTheDocumentToItsCurrentTrajectory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const armTrajectory = "trajectory-owner-act-doc-arm"

// settleArmWork settles every open work item of the arm's Texture, as the
// end of a round does.
func settleArmWork(t *testing.T, f *rebaseFixture) {
	t.Helper()
	snapshot, err := f.s.GetLifecycleSnapshot(context.Background(), f.owner, f.computer, armTrajectory)
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range snapshot.WorkItems {
		if work.Status != types.WorkItemOpen {
			continue
		}
		req := types.SettleLifecycleWorkRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "settle:" + work.WorkItemID, TrajectoryID: armTrajectory, WorkItemID: work.WorkItemID, ActingAgentID: work.AssignedAgentID, ResultRef: "evidence://round-done"}
		req.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(req)
		if _, err := f.s.SettleLifecycleWork(context.Background(), req); err != nil {
			t.Fatalf("settle %s: %v", work.WorkItemID, err)
		}
	}
}

// armTextureWork is the arm Texture's work items, open ones first counted.
func armTextureWork(t *testing.T, f *rebaseFixture) (open []types.WorkItemRecord, all int) {
	t.Helper()
	snapshot, err := f.s.GetLifecycleSnapshot(context.Background(), f.owner, f.computer, armTrajectory)
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range snapshot.WorkItems {
		if work.AssignedAgentID != "texture:doc-arm" {
			continue
		}
		all++
		if work.Status == types.WorkItemOpen {
			open = append(open, work)
		}
	}
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("the arm trajectory is %s, not live", snapshot.Trajectory.Status)
	}
	return open, all
}

func TestRebaseNoticeASettledTextureGetsTheNotice(t *testing.T) {
	f := newRebaseFallbackFixture(t, true)
	settleArmWork(t, f)
	// The Cast refusal names the Texture the notice goes to.
	if err := f.rt.sourceProjectionCastGate(context.Background()); err == nil || strings.Contains(err.Error(), "rebase_needs_root") || !strings.Contains(err.Error(), "texture:doc-arm") {
		t.Fatalf("a settled document Texture is where the notice goes: %v", err)
	}
	if out := f.fire(t); out.Minted != f.noticeID(1) {
		t.Fatalf("a settled document Texture got no notice: %+v", out)
	}
	rec := f.notice(t, 1)
	if rec == nil || rec.Inbox != "texture:doc-arm" || inboxRecordKind(*rec) != "rebase_notice" {
		t.Fatalf("notice %+v", rec)
	}
	// The notice is the wake: it is an unread record for that Texture.
	unread, err := f.rt.InboxUnread(context.Background(), f.owner, f.computer, "texture:doc-arm")
	if err != nil || !unread {
		t.Fatalf("the notice must be unread pressure on texture:doc-arm: %v %v", unread, err)
	}
}

func TestRebaseNoticeOpensNoWork(t *testing.T) {
	t.Run("attempt 2 with attempt 1's work still open", func(t *testing.T) {
		f := newRebaseFallbackFixture(t, true) // the owner's item stays open
		if out := f.fire(t); out.Minted != f.noticeID(1) {
			t.Fatalf("%+v", out)
		}
		f.now = f.now.Add(31 * time.Minute) // unanswered: attempt 1 closes
		if out := f.fire(t); out.Minted != f.noticeID(2) {
			t.Fatalf("attempt 2: %+v", out)
		}
		if rec := f.notice(t, 2); rec == nil || rec.Inbox != "texture:doc-arm" {
			t.Fatalf("attempt 2 %+v", rec)
		}
		if open, all := armTextureWork(t, f); len(open) != 1 || all != 1 {
			t.Fatalf("a notice opened work: open %+v all %d", open, all)
		}
	})
	t.Run("failed commit", func(t *testing.T) {
		f := newRebaseFallbackFixture(t, true)
		settleArmWork(t, f)
		f.rt.rebaseCommit = func(context.Context, types.CommitInboxRecordsRequest) error { return errors.New("store busy") }
		if out := f.fire(t); out.Minted != "" || out.Rearm <= 0 {
			t.Fatalf("a failed commit: %+v", out)
		}
		if open, _ := armTextureWork(t, f); len(open) != 0 {
			t.Fatalf("a failed commit left open work: %+v", open)
		}
		f.rt.rebaseCommit = nil
		if out := f.fire(t); out.Minted != f.noticeID(1) {
			t.Fatalf("after the failure: %+v", out)
		}
		if open, _ := armTextureWork(t, f); len(open) != 0 {
			t.Fatalf("the mint opened work: %+v", open)
		}
	})
	t.Run("owner retry refused", func(t *testing.T) {
		f := newRebaseFallbackFixture(t, true)
		settleArmWork(t, f)
		f.fire(t)
		if _, err := f.rt.retryRebaseNotice(context.Background(), f.owner); err == nil {
			t.Fatal("a retry while attempt 1 is live must be refused")
		}
		if open, _ := armTextureWork(t, f); len(open) != 0 {
			t.Fatalf("a refused retry left open work: %+v", open)
		}
	})
}

func TestRebaseNoticeSaysWhyNot(t *testing.T) {
	t.Run("unbound operation", func(t *testing.T) {
		f := newRebaseFallbackFixture(t, false)
		out := f.fire(t)
		if out.Minted != "" || !strings.Contains(out.Reason, "rebase_needs_root") || !strings.Contains(out.Reason, "names no document") {
			t.Fatalf("%+v", out)
		}
	})
	t.Run("cancelled trajectory", func(t *testing.T) {
		f := newRebaseFallbackFixture(t, true)
		settleArmWork(t, f)
		snapshot, err := f.s.GetLifecycleSnapshot(context.Background(), f.owner, f.computer, armTrajectory)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := f.s.GetLifecycleDocument(context.Background(), f.owner, f.computer, "doc-arm")
		if err != nil {
			t.Fatal(err)
		}
		intent := types.CancelLifecycleRequest{
			OwnerID: f.owner, ComputerID: f.computer, CommandID: "cancel:arm", TrajectoryID: armTrajectory, Reason: "owner cancellation",
			ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, RequestedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
			ExpectedHeadRevisionID: doc.CurrentRevisionID,
		}
		intent.CommandDigest, _ = store.ComputeCancelLifecycleDigest(intent)
		if _, err := f.s.PrepareLifecycleCancellation(context.Background(), intent); err != nil {
			t.Fatalf("prepare cancellation: %v", err)
		}
		out := f.fire(t)
		if out.Minted != "" || !strings.Contains(out.Reason, "cancellation intent") {
			t.Fatalf("%+v", out)
		}
		if open, _ := armTextureWork(t, f); len(open) != 0 {
			t.Fatalf("a cancelled trajectory was given work: %+v", open)
		}
	})
}

// continueArmDocument settles the arm trajectory and continues doc-arm onto
// next, as an owner revision on a settled document does.
func continueArmDocument(t *testing.T, f *rebaseFixture, next string) {
	t.Helper()
	moveDocumentToNewTrajectory(t, f.s, f.owner, f.computer, "doc-arm", armTrajectory, next)
}

// moveDocumentToNewTrajectory settles every open work item and the trajectory
// from, then continues docID onto next, as an owner revision on a settled
// document does (textureowner continueSettledDocumentForOwner).
func moveDocumentToNewTrajectory(t *testing.T, s *store.Store, owner, computer, docID, from, next string) {
	t.Helper()
	ctx := context.Background()
	snapshot, err := s.GetLifecycleSnapshot(ctx, owner, computer, from)
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range snapshot.WorkItems {
		if work.Status != types.WorkItemOpen {
			continue
		}
		req := types.SettleLifecycleWorkRequest{OwnerID: owner, ComputerID: computer, CommandID: "settle:" + work.WorkItemID, TrajectoryID: from, WorkItemID: work.WorkItemID, ActingAgentID: work.AssignedAgentID, ResultRef: "evidence://round-done"}
		req.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(req)
		if _, err := s.SettleLifecycleWork(ctx, req); err != nil {
			t.Fatalf("settle %s: %v", work.WorkItemID, err)
		}
	}
	if snapshot, err = s.GetLifecycleSnapshot(ctx, owner, computer, from); err != nil {
		t.Fatal(err)
	}
	settle := types.SettleLifecycleTrajectoryRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "settle:" + from, TrajectoryID: from,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snapshot.Document.CurrentRevisionID,
	}
	settle.CommandDigest, _ = store.ComputeSettleLifecycleTrajectoryDigest(settle)
	if _, err := s.SettleLifecycleTrajectory(ctx, settle); err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
	req := types.ContinueLifecycleDocumentRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "continue:" + next, TrajectoryID: next,
		PredecessorTrajectoryID: from, DocID: docID, ExpectedHeadRevisionID: snapshot.Document.CurrentRevisionID,
		InitialWork: types.WorkItemRecord{WorkItemID: "work:" + next, Objective: "Revise the document for the owner's new request.", AssignedAgentID: "texture:" + docID, AuthorityProfile: "texture"},
	}
	req.CommandDigest, _ = store.ComputeContinueLifecycleDocumentDigest(req)
	if _, err := s.ContinueLifecycleDocument(ctx, req); err != nil {
		t.Fatalf("continue: %v", err)
	}
	doc, err := s.GetLifecycleDocument(ctx, owner, computer, docID)
	if err != nil || doc.TrajectoryID != next {
		t.Fatalf("%s is on %q (%v), want %s", docID, doc.TrajectoryID, err, next)
	}
}

func TestRebaseNoticeFollowsTheDocumentToItsCurrentTrajectory(t *testing.T) {
	f := newRebaseFallbackFixture(t, true)
	continueArmDocument(t, f, "trajectory-arm-2")
	if err := f.rt.sourceProjectionCastGate(context.Background()); err == nil || strings.Contains(err.Error(), "rebase_needs_root") || !strings.Contains(err.Error(), "texture:doc-arm") {
		t.Fatalf("the refusal must name the document's Texture: %v", err)
	}
	if out := f.fire(t); out.Minted != f.noticeID(1) {
		t.Fatalf("a document on a newer trajectory got no notice: %+v", out)
	}
	if rec := f.notice(t, 1); rec == nil || rec.Inbox != "texture:doc-arm" {
		t.Fatalf("notice %+v", rec)
	}
	// Settled on the newer trajectory too: still the addressee.
	f2 := newRebaseFallbackFixture(t, true)
	continueArmDocument(t, f2, "trajectory-arm-2")
	snapshot, err := f2.s.GetLifecycleSnapshot(context.Background(), f2.owner, f2.computer, "trajectory-arm-2")
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen {
			req := types.SettleLifecycleWorkRequest{OwnerID: f2.owner, ComputerID: f2.computer, CommandID: "settle:" + work.WorkItemID, TrajectoryID: "trajectory-arm-2", WorkItemID: work.WorkItemID, ActingAgentID: work.AssignedAgentID, ResultRef: "evidence://done"}
			req.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(req)
			if _, err := f2.s.SettleLifecycleWork(context.Background(), req); err != nil {
				t.Fatal(err)
			}
		}
	}
	if out := f2.fire(t); out.Minted != f2.noticeID(1) {
		t.Fatalf("a settled Texture on the newer trajectory: %+v", out)
	}
}

// settleArmTrajectory settles the arm's work and its trajectory, leaving
// doc-arm idle on a settled trajectory: every arm's state between rounds
// (claude, 14:23Z: ffb9e093's trajectory settled at 11:27Z).
func settleArmTrajectory(t *testing.T, f *rebaseFixture) {
	t.Helper()
	ctx := context.Background()
	settleArmWork(t, f)
	snapshot, err := f.s.GetLifecycleSnapshot(ctx, f.owner, f.computer, armTrajectory)
	if err != nil {
		t.Fatal(err)
	}
	settle := types.SettleLifecycleTrajectoryRequest{
		OwnerID: f.owner, ComputerID: f.computer, CommandID: "settle:" + armTrajectory, TrajectoryID: armTrajectory,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snapshot.Document.CurrentRevisionID,
	}
	settle.CommandDigest, _ = store.ComputeSettleLifecycleTrajectoryDigest(settle)
	if _, err := f.s.SettleLifecycleTrajectory(ctx, settle); err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
}

// checkContinuedForNotice: doc-arm is on the next trajectory the owner's
// revise would start, live, with one open Texture work item, and the notice is
// unread pressure on texture:doc-arm.
func checkContinuedForNotice(t *testing.T, f *rebaseFixture, noticeID string) {
	t.Helper()
	ctx := context.Background()
	next, _, _ := store.TextureContinueIdentity(f.owner, f.computer, "doc-arm", armTrajectory)
	doc, err := f.s.GetLifecycleDocument(ctx, f.owner, f.computer, "doc-arm")
	if err != nil || doc.TrajectoryID != next {
		t.Fatalf("doc-arm is on %q (%v), want the continued trajectory %s", doc.TrajectoryID, err, next)
	}
	snapshot, err := f.s.GetLifecycleSnapshot(ctx, f.owner, f.computer, next)
	if err != nil || snapshot.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("the continued trajectory: %+v %v", snapshot.Trajectory, err)
	}
	open := 0
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == "texture:doc-arm" {
			open++
		}
	}
	if open != 1 {
		t.Fatalf("the continued trajectory has %d open Texture items, want 1", open)
	}
	if rec := f.notice(t, 1); rec == nil || rec.RecordID != noticeID || rec.Inbox != "texture:doc-arm" {
		t.Fatalf("notice %+v", rec)
	}
	unread, err := f.rt.InboxUnread(ctx, f.owner, f.computer, "texture:doc-arm")
	if err != nil || !unread {
		t.Fatalf("the notice must be unread on texture:doc-arm: %v %v", unread, err)
	}
}

func TestRebaseNoticeContinuesAnIdleSettledDocument(t *testing.T) {
	f := newRebaseFallbackFixture(t, true)
	settleArmTrajectory(t, f) // no resident run of any desk: the fixture starts none
	if err := f.rt.sourceProjectionCastGate(context.Background()); err == nil || strings.Contains(err.Error(), "rebase_needs_root") || !strings.Contains(err.Error(), "texture:doc-arm") {
		t.Fatalf("a settled document's Texture is where the notice goes: %v", err)
	}
	out := f.fire(t)
	if out.Minted != f.noticeID(1) {
		t.Fatalf("an idle settled document got no notice: %+v", out)
	}
	checkContinuedForNotice(t, f, f.noticeID(1))
	// A repeat fire starts nothing new: attempt 1 is live, one trajectory.
	f.now = f.now.Add(6 * time.Minute)
	if again := f.fire(t); again.Minted != "" {
		t.Fatalf("a repeat fire minted %+v", again)
	}
	checkContinuedForNotice(t, f, f.noticeID(1))
}

// A notice committed on a settled trajectory whose continue then failed (the
// mint commits first): the next fire continues the document, so the record is
// read. (A live trajectory with the notice unread cannot settle.)
func TestRebaseNoticeAContinueThatFailedIsRetried(t *testing.T) {
	f := newRebaseFallbackFixture(t, true)
	settleArmTrajectory(t, f)
	notice, ok := types.NewStackRebaseNotice(types.StackRebaseNotice{
		TextureInbox: "texture:doc-arm", ComputerID: f.computer, OperationID: f.opID, BootedBase: f.moved, StackBase: f.base, Attempt: 1, DocID: "doc-arm",
	})
	if !ok {
		t.Fatal("notice does not bind")
	}
	f.commit(t, notice.RecordID, notice)
	f.now = f.now.Add(6 * time.Minute)
	if out := f.fire(t); out.Minted != "" || !strings.Contains(out.Reason, "still live") {
		t.Fatalf("attempt 1 must stay the live attempt: %+v", out)
	}
	checkContinuedForNotice(t, f, f.noticeID(1))
}

// The owner's revise continued the settled document first: the notice joins
// that trajectory and starts no other.
func TestRebaseNoticeJoinsTheOwnersContinuation(t *testing.T) {
	f := newRebaseFallbackFixture(t, true)
	settleArmTrajectory(t, f)
	next, commandID, workID := store.TextureContinueIdentity(f.owner, f.computer, "doc-arm", armTrajectory)
	doc, err := f.s.GetLifecycleDocument(context.Background(), f.owner, f.computer, "doc-arm")
	if err != nil {
		t.Fatal(err)
	}
	req := types.ContinueLifecycleDocumentRequest{
		OwnerID: f.owner, ComputerID: f.computer, CommandID: commandID, TrajectoryID: next, PredecessorTrajectoryID: armTrajectory,
		DocID: "doc-arm", ExpectedHeadRevisionID: doc.CurrentRevisionID,
		InitialWork: types.WorkItemRecord{WorkItemID: workID, Objective: "Revise the document for the owner's new request.", AssignedAgentID: "texture:doc-arm", AuthorityProfile: "texture"},
	}
	req.CommandDigest, _ = store.ComputeContinueLifecycleDocumentDigest(req)
	if _, err := f.s.ContinueLifecycleDocument(context.Background(), req); err != nil {
		t.Fatalf("the owner's continue: %v", err)
	}
	if out := f.fire(t); out.Minted != f.noticeID(1) {
		t.Fatalf("%+v", out)
	}
	checkContinuedForNotice(t, f, f.noticeID(1))
	trajectories, err := f.s.ListLifecycleTrajectoriesByOwner(context.Background(), f.owner, f.computer, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(trajectories) != 2 {
		t.Fatalf("want the settled and the one continued trajectory, got %d", len(trajectories))
	}
}
