package textureowner

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// What a record that reaches a Texture whose trajectory has settled does. This
// pins the ACTUAL behaviour (read-only: nothing here changes it), and the
// behaviour departs from the owner's rule "Texture should be woken by new
// messages": a settled trajectory is never woken by a record. The code path:
// the actor's inbox_record update (actorruntime/handler.go handleInboxRecord)
// calls Handler.ReconcileAgentWake, which returns no run when
// textureLifecycleActivationEligible is false, and
// classifyTextureLifecycleActivationSnapshot says false for a settled or
// cancelled trajectory (texture_controller.go). The owner's own next revise is
// different: it continues the document on a successor trajectory
// (store.ContinueLifecycleDocument; texture_settled_continue_test.go, C5).
//
// How often a record can arrive after settlement: settlement requires the
// Texture inbox to be fully read and no conversation of it to have an open
// round (store lifecycleSettlementReady), so a reply to a round Texture cast
// cannot be late. What can arrive late is unsolicited mail from any desk: a
// management notice, an engineering relay, an owner-act record, or a new
// report on an unrelated conversation.
func TestAMessageToASettledTextureWakesNothing(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	s := core.Store()
	ctx := context.Background()
	computer := core.TextureComputerID()
	rec := seedCellAuthorLifecycleOn(t, s, computer, "owner-settled-wake", "settled-wake", "# Merger\n\nOpening line.\n\nClosing line.", promptSeed)
	h := handler
	h.createAgentMutationForRun(ctx, &rec)
	base := headOf(t, s, rec)
	v1 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Done: "answered from what I know",
		Write: &yaegikernel.TextureWrite{Edits: "@replace b2\nFrom memory, not yet sourced: a court ruled on a merger.", Why: "v1"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, v1, "settled-v1"); err != nil {
		t.Fatalf("v1 with Done: %v", err)
	}
	parkRun(t, s, rec.OwnerID, computer, rec.RunID, "settled-park")
	settleDocumentTrajectory(t, s, rec.OwnerID, computer, rec.ChannelID, rec.TrajectoryID)

	seedTextureReport(t, s, rec, "research-cell:report:after-settlement")
	seedTextureReportFrom(t, s, rec, "management-cell:report:after-settlement", agentprofile.Management+":"+rec.OwnerID)
	woken, err := h.ReconcileAgentWake(ctx, rec.OwnerID, rec.ChannelID)
	if err != nil || woken != nil {
		t.Fatalf("a record to a settled Texture started a run: run=%+v err=%v (the behaviour this pins is that nothing wakes)", woken, err)
	}
	if unread, err := s.TextureWorkUnread(ctx, rec.OwnerID, computer, rec.AgentID); err != nil || !unread {
		t.Fatalf("the records are not left unread: unread=%v err=%v", unread, err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, computer, rec.TrajectoryID)
	if err != nil || snapshot.Trajectory.Status != types.TrajectorySettled {
		t.Fatalf("trajectory = %v, %v; want it still settled", snapshot.Trajectory.Status, err)
	}
	if !strings.Contains(snapshot.HeadRevision.Content, "not yet sourced") {
		t.Fatalf("head moved: %q", snapshot.HeadRevision.Content)
	}
}
