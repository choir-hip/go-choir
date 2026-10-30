package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// P1 (record-first controls): every control mints one cast record in the
// same commit as its packet, the packet names the record, a replay mints
// nothing, and an owner-issued control records the owner as its emitter
// (docs/archive/definitions/choir-protocol-p1-record-first-controls-2026-10-11.md).

func controlRecordFor(t *testing.T, s *Store, packet types.CoagentSourcePacket) *types.CommitmentRecord {
	t.Helper()
	if packet.SourceRecordID == "" {
		t.Fatalf("control %s names no source record", packet.UpdateID)
	}
	rec, err := s.GetCommitmentRecord(context.Background(), packet.OwnerID, packet.ComputerID, packet.SourceRecordID)
	if err != nil || rec == nil {
		t.Fatalf("control %s record %s: %+v, %v", packet.UpdateID, packet.SourceRecordID, rec, err)
	}
	return rec
}

func TestTextureTurnControlsMintOneCastRecordEachInTheirCommit(t *testing.T) {
	s, start, caller, researchWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnNoSemanticChange)
	req.CommandID = "texture-turn-records"
	req.Controls = []types.TextureTurnControl{
		textureTurnControl(t, "control-a", researchWork.AssignedAgentID, researchWork.WorkItemID),
		textureTurnControl(t, "control-b", researchWork.AssignedAgentID, researchWork.WorkItemID),
	}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil || len(result.Controls) != 2 {
		t.Fatalf("apply Texture turn = %+v, %v", result, err)
	}
	pending, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, researchWork.AssignedAgentID, 10)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending controls = %+v, %v", pending, err)
	}
	seen := map[string]bool{}
	for _, packet := range pending {
		if packet.Direction != types.LifecyclePacketDirectionControl || packet.UpdateID != packet.ProducerUpdateID {
			t.Fatalf("control packet changed shape: %+v", packet)
		}
		rec := controlRecordFor(t, s, packet)
		if seen[rec.RecordID] {
			t.Fatalf("two controls share record %s", rec.RecordID)
		}
		seen[rec.RecordID] = true
		if rec.RecordKind() != types.CommitmentKindDirective || rec.Directive == nil || rec.Directive.Subtype != types.CommitmentDirectiveCast {
			t.Fatalf("control record is not a cast: %+v", rec)
		}
		if rec.Addressee != researchWork.AssignedAgentID || rec.Provenance.AgentID != caller.AgentID || rec.Provenance.CommittedAt == "" {
			t.Fatalf("control record addressing/provenance = %+v", rec)
		}
		if rec.EmitterKind != types.CommitmentEmitterAgent || rec.ComputerID != start.ComputerID ||
			rec.SubjectRefs["doc_id"] != start.InitialDocument.DocID || rec.SubjectRefs["trajectory_id"] != start.TrajectoryID ||
			rec.SubjectRefs["control_id"] != packet.UpdateID || rec.SubjectRefs["work_item_id"] != researchWork.WorkItemID {
			t.Fatalf("control record reserved fields = %+v", rec)
		}
	}
	before, err := s.ListCommitmentRecords(ctx, start.OwnerID, start.ComputerID, "", 100)
	if err != nil || len(before) != 2 {
		t.Fatalf("records after the turn = %d, %v", len(before), err)
	}
	peer := &Store{ogStore: s.ogStore, ogReadStore: s.ogReadStore}
	if replay, err := peer.ApplyTextureTurn(ctx, req); err != nil || !replay.Replay {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	after, err := s.ListCommitmentRecords(ctx, start.OwnerID, start.ComputerID, "", 100)
	if err != nil || len(after) != 2 {
		t.Fatalf("replay minted records: %d, %v", len(after), err)
	}
}

func TestRefusedControlCommandMintsNoRecord(t *testing.T) {
	s, start, caller, researchWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnNoSemanticChange)
	req.CommandID = "texture-turn-refused"
	req.Controls = []types.TextureTurnControl{
		textureTurnControl(t, "control-ok", researchWork.AssignedAgentID, researchWork.WorkItemID),
		textureTurnControl(t, "control-bad", researchWork.AssignedAgentID, "work-that-does-not-exist"),
	}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, req); err == nil {
		t.Fatal("a turn with an invalid control committed")
	}
	records, err := s.ListCommitmentRecords(ctx, start.OwnerID, start.ComputerID, "", 100)
	if err != nil || len(records) != 0 {
		t.Fatalf("a refused command minted records: %+v, %v", records, err)
	}
}

func TestOwnerIssuedControlRecordsTheOwnerAsEmitter(t *testing.T) {
	s, start, caller, researchWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	trajectory, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	callerAgent, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, caller.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	req := types.IssueLifecycleControlRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "issue-control-owner",
		TrajectoryID: start.TrajectoryID, DocumentID: start.InitialDocument.DocID,
		CallerAgentID: caller.AgentID, CallerRunID: caller.RunID,
		ExpectedLifecycleVersion: trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: callerAgent.LifecycleVersion,
		Controls: []types.TextureTurnControl{textureTurnControl(t, "owner-control", researchWork.AssignedAgentID, researchWork.WorkItemID)},
		Reason:   "owner_management_open", Emitter: types.CommitmentEmitterOwner,
	}
	if req.CommandDigest, err = ComputeIssueLifecycleControlDigest(req); err != nil {
		t.Fatal(err)
	}
	result, err := s.IssueLifecycleControl(ctx, req)
	if err != nil || len(result.Controls) != 1 {
		t.Fatalf("issue = %+v, %v", result, err)
	}
	rec := controlRecordFor(t, s, result.Controls[0])
	if rec.EmitterKind != types.CommitmentEmitterOwner || rec.Provenance.AgentID != "owner:"+start.OwnerID {
		t.Fatalf("owner control recorded as %q / %q", rec.EmitterKind, rec.Provenance.AgentID)
	}
	// The packet's transport fields still name the Texture run that carried it.
	if result.Controls[0].AgentID != caller.AgentID {
		t.Fatalf("owner control changed its transport caller: %+v", result.Controls[0])
	}
}

// An empty emitter is absent from the request's JSON, so every pre-P1
// command keeps its digest and a replay of an old command does not conflict.
func TestControlEmitterLeavesExistingDigestsUnchanged(t *testing.T) {
	req := types.IssueLifecycleControlRequest{OwnerID: "o", ComputerID: "c", CommandID: "cmd", TrajectoryID: "t", DocumentID: "d",
		CallerAgentID: "texture:d", CallerRunID: "r", ExpectedLifecycleVersion: 1, ExpectedCallerLifecycleVersion: 1, Reason: "x"}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "emitter") {
		t.Fatalf("an empty emitter is serialized: %s", raw)
	}
	plain, err := ComputeIssueLifecycleControlDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Emitter = types.CommitmentEmitterOwner
	owner, err := ComputeIssueLifecycleControlDigest(req)
	if err != nil || owner == plain {
		t.Fatalf("a set emitter does not enter the digest: %q %v", owner, err)
	}
}

// The desk pack reads the newest records. Cast records must not push the
// desk's own acts out of that window: the pack query excludes directives
// before its limit, so the pack over a ledger plus many casts equals the
// old pipeline's pack over the ledger without them.
func TestCastRecordsDoNotCrowdTheDeskPackWindow(t *testing.T) {
	s, start, _, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	research := "research:" + start.InitialDocument.DocID
	base := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		rec := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: fmt.Sprintf("report-%02d", i),
			Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved, Addressee: start.Agent.AgentID,
			Prediction: types.CommitmentPrediction{Hypothesis: fmt.Sprintf("finding %d", i)},
			Provenance: types.CommitmentProvenance{AgentID: research, CommittedAt: base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339Nano)}}
		if _, err := s.AppendCommitmentRecord(ctx, start.OwnerID, start.ComputerID, rec); err != nil {
			t.Fatal(err)
		}
	}
	oldPack := func(limit int) types.ActingPack {
		records, err := s.ListCommitmentRecords(ctx, start.OwnerID, start.ComputerID, "", limit)
		if err != nil {
			t.Fatal(err)
		}
		return types.BuildActingPack(records, research, 50)
	}
	// The old pipeline over the ledger without casts, with a window that
	// holds every report, is the baseline.
	want := oldPack(20)
	if len(want.Items) != 20 {
		t.Fatalf("baseline pack has %d items, want 20", len(want.Items))
	}
	// Casts are strictly newer than the reports even at one-second
	// timestamp resolution, as a live control is newer than older acts.
	time.Sleep(1100 * time.Millisecond)
	for i := 0; i < 30; i++ {
		rec := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: fmt.Sprintf("cast-%02d", i),
			Kind: types.CommitmentKindDirective, Discrepancy: types.DiscrepancyUnresolved, Addressee: research,
			Directive:  &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover a branch"},
			Provenance: types.CommitmentProvenance{AgentID: start.Agent.AgentID, CommittedAt: base.Add(time.Hour + time.Duration(i)*time.Minute).Format(time.RFC3339Nano)}}
		if _, err := s.AppendCommitmentRecord(ctx, start.OwnerID, start.ComputerID, rec); err != nil {
			t.Fatal(err)
		}
	}
	// With 30 casts added, the old pipeline's 20-record window must spend
	// slots on casts (50 records compete for 20), so it loses reports.
	if crowded := oldPack(20); len(crowded.Items) >= 20 {
		t.Fatalf("old pipeline kept all %d reports beside 30 casts; the window did not fill", len(crowded.Items))
	}
	claims, err := s.ListCommitmentClaimRecords(ctx, start.OwnerID, start.ComputerID, 20)
	if err != nil {
		t.Fatal(err)
	}
	got := types.BuildActingPack(claims, research, 50)
	if len(got.Items) != len(want.Items) {
		t.Fatalf("pack with casts has %d items, want %d", len(got.Items), len(want.Items))
	}
	for i := range want.Items {
		if got.Items[i].RecordID != want.Items[i].RecordID {
			t.Fatalf("pack item %d = %s, want %s", i, got.Items[i].RecordID, want.Items[i].RecordID)
		}
	}
}
