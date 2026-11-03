package agentcore

// Fix 10 (docs/problems/selfdev-round-blockers-cluster-2026-11-03.md): with
// no live root Texture RLM, the rebase notice goes to the Texture of the
// document the stack operation's own trajectory binds. Ways it can fail,
// written before the code (that doc, "Failure modes", 10):
//
//   1. the notice goes to a Texture that cannot answer (archived document,
//      trajectory not live or cancelled) -> ...NeedsALiveDocument
//   2. root and document both live -> ...PrefersTheRoot
//   4. an operation with no document -> ...NeedsALiveDocument (unbound op)
//   5. the store's admission refuses the answering Cast -> ...OpensWithTheClaim
//   the refusal text a Cast gets names the Texture -> ...RefusalNamesTheTexture

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// newRebaseFallbackFixture is a computer with no root marker: the applied
// stack op-a sits on B, its trajectory binds document doc-arm (a live
// Texture), and the booted base moved to B'.
func newRebaseFallbackFixture(t *testing.T, bindDocument bool) *rebaseFixture {
	t.Helper()
	const computerID = "computer-rebase-arm"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	f := &rebaseFixture{rt: rt, s: rt.store, owner: "owner-rebase-arm", computer: computerID, opID: "op-a", repo: repo, base: base}
	rt.selfdevRouteOwnerID = f.owner
	f.management = persistentManagementAgentID(f.owner)
	trajectoryID, agentID := seedOwnerActDocument(t, f.s, f.owner, computerID, "doc-arm")
	if !bindDocument {
		trajectoryID = "trajectory-unbound"
	}
	f.root = agentID // the addressee the notice must name
	patch, _ := releasePatch(t, repo, stackChange())
	bundle := releaseFrozenBundleOn(t, updaterRoot, computerID, f.opID, trajectoryID, base, patch)
	release := writeCurrentRelease(t, updaterRoot, computerID, "selfdev-"+f.opID, nil)
	insertReleaseOperation(t, rt.store, releaseOpRow{operationID: f.opID, computerID: computerID, state: "applied", bundleDigest: bundle, releaseDigest: release, trajectoryID: trajectoryID})
	f.moved = commitOnTop(t, repo, base, movedBaseChange())
	setBootedBase(t, rt, f.moved)

	f.now = time.Now().UTC()
	rt.bootedAt = f.now.Add(-time.Hour)
	rt.rebaseStayUp, rt.rebaseAnswerWindow = 5*time.Minute, 30*time.Minute
	rt.rebaseNow = func() time.Time { return f.now }
	rt.rebaseAfterFunc = func(time.Duration, func()) rebaseTimerHandle { return noopRebaseTimer{} }
	t.Cleanup(rt.stopRebaseNotice)
	return f
}

func TestRebaseNoticeWithNoRootGoesToTheOperationsDocumentTexture(t *testing.T) {
	f := newRebaseFallbackFixture(t, true)
	out := f.fire(t)
	if out.Minted != f.noticeID(1) {
		t.Fatalf("no root and a live document Texture: want attempt 1 minted, got %+v", out)
	}
	rec := f.notice(t, 1)
	if rec == nil || rec.Inbox != "texture:doc-arm" || rec.SubjectRefs["doc_id"] != "doc-arm" {
		t.Fatalf("the notice must go to texture:doc-arm: %+v", rec)
	}
	if kind := inboxRecordKind(*rec); kind != "rebase_notice" {
		t.Fatalf("the document Texture must see a rebase notice, not %q", kind)
	}
}

func TestRebaseNoticeFallbackRefusalNamesTheTexture(t *testing.T) {
	f := newRebaseFallbackFixture(t, true)
	err := f.rt.sourceProjectionCastGate(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "stack_base_moved") {
		t.Fatalf("an ordinary Cast on a moved stack must still refuse: %v", err)
	}
	if strings.Contains(err.Error(), "rebase_needs_root") || !strings.Contains(err.Error(), "texture:doc-arm") {
		t.Fatalf("the refusal must name the Texture the notice goes to: %v", err)
	}
}

func TestRebaseNoticeFallbackNeedsALiveDocument(t *testing.T) {
	t.Run("unbound operation", func(t *testing.T) {
		f := newRebaseFallbackFixture(t, false)
		if out := f.fire(t); out.Minted != "" || !strings.Contains(out.Reason, "rebase_needs_root") {
			t.Fatalf("an operation bound to no document minted %+v", out)
		}
		if err := f.rt.sourceProjectionCastGate(context.Background()); err == nil || !strings.HasSuffix(err.Error(), rebaseNeedsRoot) {
			t.Fatalf("the refusal must keep rebase_needs_root: %v", err)
		}
	})
	t.Run("cancelled trajectory", func(t *testing.T) {
		f := newRebaseFallbackFixture(t, true)
		trajectoryID := "trajectory-owner-act-doc-arm"
		snapshot, err := f.s.GetLifecycleSnapshot(context.Background(), f.owner, f.computer, trajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := f.s.GetLifecycleDocument(context.Background(), f.owner, f.computer, "doc-arm")
		if err != nil {
			t.Fatal(err)
		}
		intent := types.CancelLifecycleRequest{
			OwnerID: f.owner, ComputerID: f.computer, CommandID: "cancel:arm", TrajectoryID: trajectoryID, Reason: "owner cancellation",
			ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, RequestedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
			ExpectedHeadRevisionID: doc.CurrentRevisionID,
		}
		intent.CommandDigest, _ = store.ComputeCancelLifecycleDigest(intent)
		if _, err := f.s.PrepareLifecycleCancellation(context.Background(), intent); err != nil {
			t.Fatalf("prepare cancellation: %v", err)
		}
		if out := f.fire(t); out.Minted != "" {
			t.Fatalf("a cancelled trajectory's Texture got a notice: %+v", out)
		}
	})
}

func TestRebaseNoticeFallbackPrefersTheRoot(t *testing.T) {
	f := newRebaseFallbackFixture(t, true)
	seedOwnerActDocument(t, f.s, f.owner, f.computer, "doc-master")
	if _, err := f.s.ClaimTextureRoot(context.Background(), nil, store.TextureRoot{OwnerID: f.owner, ComputerID: f.computer, DocumentID: "doc-master", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if out := f.fire(t); out.Minted != f.noticeID(1) {
		t.Fatalf("mint: %+v", out)
	}
	if rec := f.notice(t, 1); rec == nil || rec.Inbox != "texture:doc-master" {
		t.Fatalf("a live root must take the notice: %+v", rec)
	}
}

func TestRebaseNoticeFallbackCastOpensWithTheClaim(t *testing.T) {
	base := newRebaseFallbackFixture(t, true)
	f := &rebaseCastFixture{rebaseFixture: base, castID: "cast-doc-1"}
	f.preflight = &fakeSourcePreflight{head: gitHead(t, f.repo)}
	f.rt.sourcePreflight = f.preflight
	f.rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)
	if out := f.fire(t); out.Minted != f.noticeID(1) {
		t.Fatalf("attempt 1 not minted: %+v", out)
	}
	// The document Texture's own Cast carries the notice (rootCast sends from f.root).
	f.rootCast(t, f.castID, f.noticeID(1))
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatalf("a rebase Cast answering the document Texture's notice must plan: %v", err)
	}
	if !plan.Rebase || len(f.preflight.commits) != 1 || f.preflight.commits[0] != f.moved {
		t.Fatalf("the capsule must start from the booted base: plan=%+v commits=%v", plan.Rebase, f.preflight.commits)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(context.Background(), plan, records); err != nil {
		t.Fatalf("the store's admission must accept the document Texture's rebase: %v", err)
	}
	claim, err := f.s.GetRebaseNoticeClaim(context.Background(), f.owner, f.computer, types.StackRebasePairKey(f.opID, f.moved))
	if err != nil || claim.AssignmentID != plan.AssignmentID {
		t.Fatalf("claim %+v, %v", claim, err)
	}
}
