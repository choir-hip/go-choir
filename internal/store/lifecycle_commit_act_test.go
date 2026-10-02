package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// commitActFixtureRecord mints the deterministic record body a cell reducer
// derives for one staged note intent.
func commitActFixtureRecord(recordID, callerAgentID, addressee, body string) types.CommitmentRecord {
	return types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    recordID,
		Kind:        types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{
			AgentID:     callerAgentID,
			CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
		Addressee: addressee,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: body},
	}
}

func commitActRequestForTest(t *testing.T, s *Store, start types.StartLifecycleRequest, caller types.RunRecord, rec types.CommitmentRecord, targetAgentID string) types.CommitLifecycleActRequest {
	t.Helper()
	ctx := context.Background()
	callerAgent, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, caller.AgentID)
	if err != nil {
		t.Fatalf("load caller agent: %v", err)
	}
	req := types.CommitLifecycleActRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID:    "commit-act:" + rec.RecordID,
		TrajectoryID: start.TrajectoryID, CallerAgentID: caller.AgentID, CallerRunID: caller.RunID,
		ExpectedCallerLifecycleVersion: callerAgent.LifecycleVersion,
		Record:                         rec,
	}
	trajectory, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("load trajectory: %v", err)
	}
	req.ExpectedLifecycleVersion = trajectory.LifecycleVersion
	if targetAgentID != "" {
		target, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, targetAgentID)
		if err != nil {
			t.Fatalf("load target agent: %v", err)
		}
		packet := types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "directive", Summary: "note: directive", Notes: []string{"directive:note", rec.Directive.Body},
		}
		content := rec.Directive.Body
		digest, err := ComputeLifecycleUpdatePayloadDigest(packet, content)
		if err != nil {
			t.Fatalf("packet digest: %v", err)
		}
		req.PacketSpec = &types.LifecycleActPacketSpec{
			TargetAgentID: target.AgentID, ChannelID: strings.TrimSpace(target.ChannelID),
			Packet: packet, Content: content, PayloadDigest: digest,
		}
	}
	req.CommandDigest, err = ComputeCommitLifecycleActDigest(req)
	if err != nil {
		t.Fatalf("command digest: %v", err)
	}
	return req
}

func TestCommitLifecycleActMintsRecordAndPacketAtomically(t *testing.T) {
	s, start, caller, researchWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	rec := commitActFixtureRecord("cell-1:note:1", caller.AgentID, "research:texture-target", "hold rewarm order before next wake")
	req := commitActRequestForTest(t, s, start, caller, rec, researchWork.AssignedAgentID)

	result, err := s.CommitLifecycleAct(ctx, req)
	if err != nil {
		t.Fatalf("commit act: %v", err)
	}
	if result.Update == nil || result.Update.UpdateID != rec.RecordID+":packet" {
		t.Fatalf("derived packet = %+v", result.Update)
	}
	packet := *result.Update
	if packet.Direction != types.LifecyclePacketDirectionDirective ||
		packet.SourceRecordID != rec.RecordID ||
		packet.TargetAgentID != researchWork.AssignedAgentID ||
		packet.TrajectoryID != start.TrajectoryID ||
		packet.Disposition != types.UpdatePending ||
		packet.DeliveredAt != nil || packet.DeliveredToRunID != "" ||
		packet.AgentID != caller.AgentID {
		t.Fatalf("packet projection = %+v", packet)
	}
	stored, err := s.GetCommitmentRecord(ctx, start.OwnerID, start.ComputerID, rec.RecordID)
	if err != nil || stored == nil || stored.RecordID != rec.RecordID || stored.Directive == nil || stored.Directive.Body != rec.Directive.Body {
		t.Fatalf("commitment record = %+v, %v", stored, err)
	}
	pending, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, researchWork.AssignedAgentID, 100)
	if err != nil || len(pending) != 1 || pending[0].UpdateID != packet.UpdateID {
		t.Fatalf("target pending set = %+v, %v", pending, err)
	}
	// Caller version consumed one act (replay re-derives nothing).
	after, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, caller.AgentID)
	if err != nil {
		t.Fatalf("reload caller agent: %v", err)
	}
	if after.LifecycleVersion != req.ExpectedCallerLifecycleVersion+1 {
		t.Fatalf("caller lifecycle version = %d", after.LifecycleVersion)
	}
}

func TestCommitLifecycleActReplayIsByteIdentical(t *testing.T) {
	s, start, caller, researchWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	rec := commitActFixtureRecord("cell-2:note:1", caller.AgentID, "research:texture-target", "replay me")
	req := commitActRequestForTest(t, s, start, caller, rec, researchWork.AssignedAgentID)

	first, err := s.CommitLifecycleAct(ctx, req)
	if err != nil {
		t.Fatalf("first commit: %v", err)
	}
	replay, err := s.CommitLifecycleAct(ctx, req)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !replay.Replay {
		t.Fatalf("replay flag = %+v", replay)
	}
	if first.Update == nil || replay.Update == nil || first.Update.UpdateID != replay.Update.UpdateID {
		t.Fatalf("replay packet identity = %+v vs %+v", first.Update, replay.Update)
	}
	pending, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, researchWork.AssignedAgentID, 100)
	if err != nil || len(pending) != 1 {
		t.Fatalf("replay duplicated packet: %+v %v", pending, err)
	}

	// Changed content under the same CommandID is a digest conflict.
	changed := req
	changed.Record.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: "different body"}
	changed.CommandDigest, _ = ComputeCommitLifecycleActDigest(changed)
	if _, err := s.CommitLifecycleAct(ctx, changed); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("changed replay err=%v", err)
	}
}

func TestCommitLifecycleActRejectsAddresseeMismatchAndBadDigest(t *testing.T) {
	s, start, caller, researchWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()

	rec := commitActFixtureRecord("cell-3:note:1", caller.AgentID, "management", "wrong desk")
	req := commitActRequestForTest(t, s, start, caller, rec, researchWork.AssignedAgentID)
	if _, err := s.CommitLifecycleAct(ctx, req); err == nil || !strings.Contains(err.Error(), "addressee") {
		t.Fatalf("addressee mismatch err=%v", err)
	}

	rec2 := commitActFixtureRecord("cell-3:note:2", caller.AgentID, "research:texture-target", "bad digest")
	req2 := commitActRequestForTest(t, s, start, caller, rec2, researchWork.AssignedAgentID)
	req2.PacketSpec.PayloadDigest = "sha256:deadbeef"
	req2.CommandDigest, _ = ComputeCommitLifecycleActDigest(req2)
	if _, err := s.CommitLifecycleAct(ctx, req2); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("payload digest mismatch err=%v", err)
	}
}

func TestCommitLifecycleActLedgerOnlyForUnaddressedAct(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	rec := commitActFixtureRecord("cell-4:note:1", caller.AgentID, "", "ledger-only note")
	req := commitActRequestForTest(t, s, start, caller, rec, "")

	result, err := s.CommitLifecycleAct(ctx, req)
	if err != nil {
		t.Fatalf("unaddressed commit: %v", err)
	}
	if result.Update != nil {
		t.Fatalf("unaddressed act minted packet: %+v", result.Update)
	}
	stored, err := s.GetCommitmentRecord(ctx, start.OwnerID, start.ComputerID, rec.RecordID)
	if err != nil || stored == nil || stored.RecordID != rec.RecordID {
		t.Fatalf("unaddressed record = %+v, %v", stored, err)
	}
}
