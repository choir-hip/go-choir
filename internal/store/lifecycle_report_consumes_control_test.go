package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// A research carrier that consumes a bound lifecycle control and reports the
// work must mark the control incorporated atomically with the producer_report.
// Regression for s0m-bound-control-not-marked-incorporated: the rebound carrier
// did the work and reported, but the bound control stayed pending and a later
// terminalize released it, so no system:reducer resolve could fire.
func TestCommitLifecycleActProducerReportMarksBoundControlIncorporated(t *testing.T) {
	s, start, caller, researchWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()

	// Mint a control for the research work and bind it to the carrier run,
	// matching the production dispatch path (texture turn -> bind -> carrier).
	turnReq := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	turnReq.CommandID = "turn-for-consume"
	turnReq.Controls = []types.TextureTurnControl{textureTurnControl(t, "control-consume", researchWork.AssignedAgentID, researchWork.WorkItemID)}
	setTextureTurnDigest(t, &turnReq, TextureSourceGraphWriteSet{})
	turn, err := s.ApplyTextureTurn(ctx, turnReq)
	if err != nil {
		t.Fatal(err)
	}
	carrier, err := s.GetLifecycleRun(ctx, start.OwnerID, start.ComputerID, "run-researcher-target")
	if err != nil {
		t.Fatal(err)
	}
	bind := bindControlRequestForTest(t, s, start, carrier, turn.Controls)
	if _, err := s.BindLifecycleControlDelivery(ctx, bind); err != nil {
		t.Fatalf("bind control: %v", err)
	}
	control := turn.Controls[0]

	// Carrier consumes the work and commits its producer_report for that work
	// item. The bound control is the thing it consumed; it must flip to
	// incorporated so the record can mechanically resolve.
	rec := commitActReportFixture("cell-report:report:consume", carrier.AgentID, start.Agent.AgentID)
	report := commitActProducerReportRequest(t, s, start, carrier, researchWork, rec)
	report.PacketSpec.WorkDisposition = types.WorkItemCompleted
	report.CommandDigest, _ = ComputeCommitLifecycleActDigest(report)
	if _, err := s.CommitLifecycleAct(ctx, report); err != nil {
		t.Fatalf("commit producer report: %v", err)
	}

	consumed, err := s.GetLifecycleUpdate(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, control.TargetAgentID, control.AgentID, control.ProducerUpdateID)
	if err != nil {
		t.Fatalf("read bound control: %v", err)
	}
	if consumed.Disposition != types.UpdateIncorporated {
		t.Fatalf("bound control disposition=%q want %q (carrier consumed the work but the control stayed %q)", consumed.Disposition, types.UpdateIncorporated, consumed.Disposition)
	}
	// The incorporated control must not be re-bound on a later reconcile: it is
	// terminal, so it drops out of the pending scan.
	pending, err := s.ListAllPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, carrier.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range pending {
		if u.UpdateID == control.UpdateID {
			t.Fatalf("consumed control still pending for rebind: %+v", u)
		}
	}
}
