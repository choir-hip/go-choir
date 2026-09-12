package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestSettleLifecycleProducerReports(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	startReq := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, startReq); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	now := time.Now().UTC()

	superAgentID := agentprofile.Super + ":" + startReq.OwnerID
	producerAgentID := "co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2"

	// Seed three pending cancel producer reports from the CoSuper assignment
	reportIDs := []string{
		"assignment-report:cancel-report:sha256:1111111111111111111111111111111111111111111111111111111111111111",
		"assignment-report:cancel-report:sha256:2222222222222222222222222222222222222222222222222222222222222222",
		"assignment-report:cancel-report:sha256:3333333333333333333333333333333333333333333333333333333333333333",
	}

	for i, id := range reportIDs {
		update := types.CoagentSourcePacket{
			UpdateID:         id,
			ProducerUpdateID: strings.TrimPrefix(id, "assignment-report:"),
			OwnerID:          startReq.OwnerID,
			ComputerID:       startReq.ComputerID,
			AgentID:          producerAgentID,
			TargetAgentID:    superAgentID,
			ChannelID:        superAgentID,
			TrajectoryID:     startReq.TrajectoryID,
			Role:             "co-super",
			Direction:        types.LifecyclePacketDirectionProducerReport,
			LifecycleVersion: 1,
			ReducerSeq:       int64(10 + i),
			Disposition:      types.UpdatePending,
			Packet: types.CoagentSourcePacketPayload{
				SchemaVersion: types.CoagentSourcePacketSchemaV1,
				Kind:          "evidence_update",
				Summary:       "cancel report",
			},
			Content:   "cancel report body",
			CreatedAt: now,
		}
		key := update.TrajectoryID + "\x00" + update.TargetAgentID + "\x00" + update.AgentID + "\x00" + update.ProducerUpdateID
		meta := lifecycleMetadata("update_id", update.UpdateID, update.ComputerID, update.TrajectoryID, update.ReducerSeq)
		meta["producer_update_id"] = update.ProducerUpdateID
		meta["target_agent_id"] = update.TargetAgentID
		obj, err := lifecycleObject(ogKindWorkerUpdate, startReq.OwnerID, startReq.ComputerID, key, update, meta, now, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ogStore.PutObject(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}

	// Also seed an unrelated report from another producer
	otherReportID := "assignment-report:other:sha256:4444"
	otherUpdate := types.CoagentSourcePacket{
		UpdateID:         otherReportID,
		ProducerUpdateID: "other",
		OwnerID:          startReq.OwnerID,
		ComputerID:       startReq.ComputerID,
		AgentID:          "co-super:other-assignment",
		TargetAgentID:    superAgentID,
		ChannelID:        superAgentID,
		TrajectoryID:     startReq.TrajectoryID,
		Role:             "co-super",
		Direction:        types.LifecyclePacketDirectionProducerReport,
		LifecycleVersion: 1,
		ReducerSeq:       20,
		Disposition:      types.UpdatePending,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "evidence_update",
			Summary:       "other report",
		},
		Content:   "other report",
		CreatedAt: now,
	}
	otherKey := otherUpdate.TrajectoryID + "\x00" + otherUpdate.TargetAgentID + "\x00" + otherUpdate.AgentID + "\x00" + otherUpdate.ProducerUpdateID
	otherMeta := lifecycleMetadata("update_id", otherUpdate.UpdateID, otherUpdate.ComputerID, otherUpdate.TrajectoryID, otherUpdate.ReducerSeq)
	otherMeta["producer_update_id"] = otherUpdate.ProducerUpdateID
	otherMeta["target_agent_id"] = otherUpdate.TargetAgentID
	otherObj, err := lifecycleObject(ogKindWorkerUpdate, startReq.OwnerID, startReq.ComputerID, otherKey, otherUpdate, otherMeta, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutObject(ctx, otherObj); err != nil {
		t.Fatal(err)
	}

	// 1. Verify ListPendingProducerReports enumerates the 3 cancel producer reports when filtered by producerAgentID
	pending, err := s.ListPendingProducerReports(ctx, startReq.OwnerID, startReq.ComputerID, producerAgentID)
	if err != nil {
		t.Fatalf("list pending producer reports: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending cancel producer reports, got %d", len(pending))
	}
	for i, u := range pending {
		if u.UpdateID != reportIDs[i] {
			t.Errorf("pending[%d].UpdateID = %s, want %s", i, u.UpdateID, reportIDs[i])
		}
	}

	// 2. Settle the reports via SettleLifecycleProducerReports
	settleReq := types.SettleLifecycleProducerReportsRequest{
		OwnerID:      startReq.OwnerID,
		ComputerID:   startReq.ComputerID,
		CommandID:    "cmd-settle-0819-cancel-reports",
		TrajectoryID: startReq.TrajectoryID,
		ReportIDs:    reportIDs,
		Reason:       "stale residue settled at store layer as late evidence",
	}
	settleReq.CommandDigest, err = ComputeSettleLifecycleProducerReportsDigest(settleReq)
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.SettleLifecycleProducerReports(ctx, settleReq)
	if err != nil {
		t.Fatalf("settle producer reports: %v", err)
	}
	if result.Receipt.Kind != types.LifecycleSettleProducerReports {
		t.Errorf("receipt kind = %s, want %s", result.Receipt.Kind, types.LifecycleSettleProducerReports)
	}
	if len(result.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(result.Events))
	}
	for _, ev := range result.Events {
		if ev.Kind != types.LifecycleUpdateLate {
			t.Errorf("event kind = %s, want %s", ev.Kind, types.LifecycleUpdateLate)
		}
	}

	// 3. Assert all pending store selectors exclude settled IDs
	afterPending, err := s.ListPendingProducerReports(ctx, startReq.OwnerID, startReq.ComputerID, producerAgentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterPending) != 0 {
		t.Fatalf("expected 0 pending cancel producer reports after settlement, got %d", len(afterPending))
	}

	allPending, err := s.ListAllPendingLifecycleUpdates(ctx, startReq.OwnerID, startReq.ComputerID, superAgentID)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range allPending {
		for _, settledID := range reportIDs {
			if u.UpdateID == settledID {
				t.Fatalf("ListAllPendingLifecycleUpdates still returned settled report %s", settledID)
			}
		}
	}
	// The other report should still be pending
	foundOther := false
	for _, u := range allPending {
		if u.UpdateID == otherReportID {
			foundOther = true
			break
		}
	}
	if !foundOther {
		t.Errorf("unrelated report was unexpectedly filtered out")
	}

	// 4. Test idempotency replay
	replay, err := s.SettleLifecycleProducerReports(ctx, settleReq)
	if err != nil {
		t.Fatalf("idempotent replay failed: %v", err)
	}
	if replay.Receipt.CommandID != settleReq.CommandID {
		t.Errorf("replay command ID = %s, want %s", replay.Receipt.CommandID, settleReq.CommandID)
	}
}
