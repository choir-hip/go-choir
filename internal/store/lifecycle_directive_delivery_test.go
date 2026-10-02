package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestDirectiveDeliveryBindsToDeskRunAndListsDeliveredPacket(t *testing.T) {
	s, start, caller, researchWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	record := commitActFixtureRecord("directive-delivery:1", caller.AgentID, researchWork.AssignedAgentID, "read the commitment record")
	committed, err := s.CommitLifecycleAct(ctx, commitActRequestForTest(t, s, start, caller, record, researchWork.AssignedAgentID))
	if err != nil || committed.Update == nil {
		t.Fatalf("commit directive: update=%+v err=%v", committed.Update, err)
	}
	update := *committed.Update

	req := types.ReconcileUpdateDeliveryRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "bind-directive-to-desk", TrajectoryID: start.TrajectoryID,
		TargetAgentID: researchWork.AssignedAgentID, TargetRunID: "run-researcher-target",
		MaxAttempts: 3,
		Items: []types.ReconcileUpdateDeliveryItem{{
			UpdateID: update.UpdateID, ProducerAgentID: update.AgentID,
			ProducerUpdateID:         update.ProducerUpdateID,
			ExpectedLifecycleVersion: update.LifecycleVersion,
		}},
	}
	var digestErr error
	req.CommandDigest, digestErr = ComputeReconcileUpdateDeliveryDigest(req)
	if digestErr != nil {
		t.Fatalf("digest directive bind: %v", digestErr)
	}
	if _, err := s.ReconcileUpdateDelivery(ctx, req); err != nil {
		t.Fatalf("bind directive: %v", err)
	}

	delivered, err := s.ListLifecycleControlsDeliveredToRun(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, researchWork.AssignedAgentID, "run-researcher-target", 100)
	if err != nil || len(delivered) != 1 || delivered[0].UpdateID != update.UpdateID || delivered[0].SourceRecordID != record.RecordID {
		t.Fatalf("delivered directive = %+v, err=%v", delivered, err)
	}
}
