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
	req := types.ContinueLifecycleDocumentRequest{
		OwnerID: f.owner, ComputerID: f.computer, CommandID: "continue:" + next, TrajectoryID: next,
		PredecessorTrajectoryID: armTrajectory, DocID: "doc-arm", ExpectedHeadRevisionID: snapshot.Document.CurrentRevisionID,
		InitialWork: types.WorkItemRecord{WorkItemID: "work:" + next, Objective: "Revise the document for the owner's new request.", AssignedAgentID: "texture:doc-arm", AuthorityProfile: "texture"},
	}
	req.CommandDigest, _ = store.ComputeContinueLifecycleDocumentDigest(req)
	if _, err := f.s.ContinueLifecycleDocument(ctx, req); err != nil {
		t.Fatalf("continue: %v", err)
	}
	doc, err := f.s.GetLifecycleDocument(ctx, f.owner, f.computer, "doc-arm")
	if err != nil || doc.TrajectoryID != next {
		t.Fatalf("doc-arm is on %q (%v), want %s", doc.TrajectoryID, err, next)
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
