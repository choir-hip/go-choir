package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Regression for the SA1 slice: a record-native directive to persistent
// Management carries no trajectory (computer-scoped), and
// ReconcileUpdateDelivery used to reject empty trajectory_ids — so every
// directive bind attempt errored and the packet stayed pending forever.
// The reconcile must bind it by its trajectory-less worker_update key.
func TestReconcileUpdateDeliveryBindsComputerScopedDirective(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ownerID, computerID := "owner-dir", "computer-dir"
	target := "management:" + ownerID

	directive := types.CoagentSourcePacket{
		UpdateID: "dir-1:packet", ProducerUpdateID: "dir-1:packet",
		OwnerID: ownerID, ComputerID: computerID,
		AgentID: "engineering:doc-1", TargetAgentID: target,
		ChannelID: "", TrajectoryID: "", // computer-scoped
		Role: "engineering", LifecycleVersion: 1, ReducerSeq: 1,
		Direction:      types.LifecyclePacketDirectionDirective,
		SourceRecordID: "cell:directive:1",
		Disposition:    types.UpdatePending,
		Packet:         types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "directive", Summary: "cast"},
		Content:        "cast this", CreatedAt: time.Now().UTC(),
	}
	key := "\x00" + target + "\x00" + directive.AgentID + "\x00" + directive.ProducerUpdateID
	obj, err := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, key, directive,
		lifecycleMetadata("update_id", directive.UpdateID, computerID, "", 1), directive.CreatedAt, directive.CreatedAt)
	if err != nil {
		t.Fatalf("build directive object: %v", err)
	}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{
		{CanonicalID: obj.CanonicalID, Exists: false},
	}, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("write directive: %v", err)
	}

	req := types.ReconcileUpdateDeliveryRequest{
		OwnerID: ownerID, ComputerID: computerID,
		CommandID:    "bind-directive-delivery:run-dir:dir-1:packet",
		TrajectoryID: "", TargetAgentID: target, TargetRunID: "run-dir",
		MaxAttempts: 3,
		Items: []types.ReconcileUpdateDeliveryItem{{
			UpdateID: directive.UpdateID, ProducerAgentID: directive.AgentID,
			ProducerUpdateID: directive.ProducerUpdateID, ExpectedLifecycleVersion: 1,
		}},
	}
	digest, err := ComputeReconcileUpdateDeliveryDigest(req)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	req.CommandDigest = digest
	res, err := s.ReconcileUpdateDelivery(ctx, req)
	if err != nil {
		t.Fatalf("reconcile computer-scoped directive: %v", err)
	}
	if len(res.Events) != 1 || res.Events[0].Kind != types.LifecycleControlDelivered {
		t.Fatalf("expected control_delivered event, got %+v", res.Events)
	}

	gotObj, err := s.lifecycleGetObject(ctx, ogKindWorkerUpdate, ownerID, computerID, key)
	if err != nil {
		t.Fatalf("reload directive: %v", err)
	}
	got, err := decodeLifecycleObject[types.CoagentSourcePacket](gotObj)
	if err != nil {
		t.Fatalf("decode directive: %v", err)
	}
	if got.DeliveredToRunID != "run-dir" || got.DeliveredAt == nil {
		t.Fatalf("directive not bound: %+v", got)
	}
	if got.DeliveryAttempts != 1 {
		t.Fatalf("directive attempts=%d want 1", got.DeliveryAttempts)
	}
}
