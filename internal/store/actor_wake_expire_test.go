package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// writePendingWorkerUpdate inserts a worker_update row directly, simulating a
// pre-outbox-fold obligation row.
func writePendingWorkerUpdate(t *testing.T, s *Store, update types.CoagentSourcePacket) objectgraph.Object {
	t.Helper()
	ctx := context.Background()
	key := update.TrajectoryID + "\x00" + update.TargetAgentID + "\x00" + update.AgentID + "\x00" + update.ProducerUpdateID
	obj, err := lifecycleObject(ogKindWorkerUpdate, update.OwnerID, update.ComputerID, key, update,
		lifecycleMetadata("update_id", update.UpdateID, update.ComputerID, update.TrajectoryID, update.ReducerSeq), update.CreatedAt, update.CreatedAt)
	if err != nil {
		t.Fatalf("build update object: %v", err)
	}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{
		{CanonicalID: obj.CanonicalID, Exists: false},
	}, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("write pending update: %v", err)
	}
	return obj
}

// The one-shot marker: a second boot migration must be a no-op even while
// open obligations still have unprojected-equivalent sources. Re-arming on
// every boot was the SA1 storm mechanism; the marker retires it.
func TestMigrateActorWakeOutboxRunsOncePerMarkerVersion(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	update := types.CoagentSourcePacket{
		UpdateID: "upd-marker-1", ProducerUpdateID: "p-marker-1",
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		AgentID: "texture:t", TargetAgentID: start.Agent.AgentID,
		ChannelID: start.Agent.ChannelID, TrajectoryID: start.TrajectoryID,
		Role: "texture", LifecycleVersion: 1, ReducerSeq: 1, Disposition: types.UpdatePending,
		Direction: types.LifecyclePacketDirectionControl,
		Packet:    types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "q"},
		Content:   "wake me", CreatedAt: time.Now().UTC(),
	}
	writePendingWorkerUpdate(t, s, update)

	minted, err := s.MigrateActorWakeOutbox(ctx)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if minted == 0 {
		t.Fatal("first migration minted nothing")
	}
	// Second call at the same version: no scan, no mint, no discharge — boot
	// cost is O(1) marker read.
	again, err := s.MigrateActorWakeOutbox(ctx)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if again != 0 {
		t.Fatalf("marker-gated migration re-ran: minted %d", again)
	}
}

// A pending control whose work item is no longer open can never deliver: the
// migration must expire it (cancelled + recorded event) instead of minting a
// wake that re-arms the stale obligation forever.
func TestMigrateActorWakeOutboxExpiresStalePendingPacket(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	// Open then cancel the target work item so its control packet is stale.
	work := types.WorkItemRecord{
		WorkItemID: "work-stale-1", OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		TrajectoryID: start.TrajectoryID, AssignedAgentID: start.Agent.AgentID,
		Status: types.WorkItemCancelled, LifecycleVersion: 2,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	workObj, err := lifecycleObject(ogKindWorkItem, start.OwnerID, start.ComputerID, work.WorkItemID, work,
		lifecycleMetadata("work_item_id", work.WorkItemID, start.ComputerID, start.TrajectoryID, 2), work.CreatedAt, work.UpdatedAt)
	if err != nil {
		t.Fatalf("build work item: %v", err)
	}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{
		{CanonicalID: workObj.CanonicalID, Exists: false},
	}, objectgraph.Batch{Objects: []objectgraph.Object{workObj}}); err != nil {
		t.Fatalf("write work item: %v", err)
	}

	update := types.CoagentSourcePacket{
		UpdateID: "upd-stale-1", ProducerUpdateID: "p-stale-1",
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		AgentID: "texture:t", TargetAgentID: start.Agent.AgentID,
		ChannelID: start.Agent.ChannelID, TrajectoryID: start.TrajectoryID,
		Role: "texture", LifecycleVersion: 1, ReducerSeq: 1, Disposition: types.UpdatePending,
		Direction: types.LifecyclePacketDirectionControl, TargetWorkItemID: "work-stale-1",
		Packet:  types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "q"},
		Content: "stale", CreatedAt: time.Now().UTC(),
	}
	writePendingWorkerUpdate(t, s, update)

	if _, err := s.MigrateActorWakeOutbox(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// The packet must be terminal (cancelled) with the recorded reason.
	key := update.TrajectoryID + "\x00" + update.TargetAgentID + "\x00" + update.AgentID + "\x00" + update.ProducerUpdateID
	obj, err := s.lifecycleGetObject(ctx, ogKindWorkerUpdate, start.OwnerID, start.ComputerID, key)
	if err != nil {
		t.Fatalf("reload packet: %v", err)
	}
	pkt, err := decodeLifecycleObject[types.CoagentSourcePacket](obj)
	if err != nil {
		t.Fatalf("decode packet: %v", err)
	}
	if pkt.Disposition != types.UpdateCancelled || pkt.DispositionReason != "stale_obligation_expired" {
		t.Fatalf("stale packet not discharged: disposition=%s reason=%s", pkt.Disposition, pkt.DispositionReason)
	}
	// And no wake row may be unprojected for it.
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatalf("list wakes: %v", err)
	}
	for _, w := range wakes {
		if w.SourceUpdateID == update.UpdateID {
			t.Fatalf("wake minted for discharged packet: %+v", w)
		}
	}
}

// ExpireStaleLifecyclePacket is a CAS command: replaying the same command is
// idempotent, and a packet that moved past the expected version conflicts.
func TestExpireStaleLifecyclePacketReplayAndConflict(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	update := types.CoagentSourcePacket{
		UpdateID: "upd-expire-1", ProducerUpdateID: "p-expire-1",
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		AgentID: "texture:t", TargetAgentID: start.Agent.AgentID,
		ChannelID: start.Agent.ChannelID, TrajectoryID: start.TrajectoryID,
		Role: "texture", LifecycleVersion: 1, ReducerSeq: 1, Disposition: types.UpdatePending,
		Direction: types.LifecyclePacketDirectionControl,
		Packet:    types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "q"},
		Content:   "expire me", CreatedAt: time.Now().UTC(),
	}
	writePendingWorkerUpdate(t, s, update)

	req := types.ExpireStaleLifecyclePacketRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "expire-stale-packet:upd-expire-1:1",
		UpdateID:  update.UpdateID, ProducerAgentID: update.AgentID,
		ProducerUpdateID: update.ProducerUpdateID, TrajectoryID: update.TrajectoryID,
		TargetAgentID: update.TargetAgentID, ExpectedLifecycleVersion: 1,
		StaleReason: "work_item_not_open",
	}
	digest, err := ComputeExpireStaleLifecyclePacketDigest(req)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	req.CommandDigest = digest
	res, err := s.ExpireStaleLifecyclePacket(ctx, req)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if len(res.Events) != 1 || res.Events[0].Kind != types.LifecycleUpdateExpired {
		t.Fatalf("expire emitted %+v", res.Events)
	}
	// Replay: same command id + digest returns the stored receipt, no error.
	replay, err := s.ExpireStaleLifecyclePacket(ctx, req)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replay.Receipt.CommandID != req.CommandID {
		t.Fatalf("replay receipt mismatch: %+v", replay.Receipt)
	}
	// Conflict: a stale version number on a fresh command id must reject.
	req2 := req
	req2.CommandID = "expire-stale-packet:upd-expire-1:99"
	req2.ExpectedLifecycleVersion = 1 // packet is now version 2 + cancelled
	d2, _ := ComputeExpireStaleLifecyclePacketDigest(req2)
	req2.CommandDigest = d2
	if _, err := s.ExpireStaleLifecyclePacket(ctx, req2); err == nil {
		t.Fatal("stale-version expire must conflict")
	}
}
