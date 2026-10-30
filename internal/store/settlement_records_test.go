package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Cut 2 breakage B10 (docs/archive/problems/reeng-cut2-breakage-2026-10-13.md): on
// the records route the records are the work, so a trajectory is not ready
// to settle while its Texture has unread records or an open round.
//
// Ways this could fail, written first: settlement reads only work items, so
// a Report that arrived after the work closed is stranded on a settled
// trajectory; an open round (research still working) lets the trajectory
// settle under it; a Texture that has read everything stays unsettled
// forever.
func TestSettlementWaitsForTextureRecords(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer, docID = "owner-settle", "computer-settle", "doc-settle"
	texture, research := "texture:"+docID, "research:"+docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "start:" + docID, TrajectoryID: "trajectory:" + docID,
		Kind:            types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work:" + docID, Objective: "answer", AssignedAgentID: texture},
		InitialDocument: types.Document{DocID: docID, Title: "Settle"},
		InitialRevision: types.Revision{RevisionID: "revision:" + docID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "texture", Content: "answer",
			BodyDoc: json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"` + docID + `"},"content":[{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"answer"}]}]}}`)},
		Agent: types.AgentRecord{AgentID: texture, OwnerID: owner, ComputerID: computer, Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	refuse := types.RefuseLifecycleWorkRequest{OwnerID: owner, ComputerID: computer, CommandID: "close-work", TrajectoryID: start.TrajectoryID,
		WorkItemID: "work:" + docID, ActingAgentID: texture, RefusalRef: "refusal://test", Reason: "answered"}
	refuse.CommandDigest, _ = ComputeRefuseLifecycleWorkDigest(refuse)
	if _, err := s.RefuseLifecycleWork(ctx, refuse); err != nil {
		t.Fatalf("close work: %v", err)
	}
	ready := func() bool {
		t.Helper()
		snap, err := s.GetLifecycleSnapshot(ctx, owner, computer, start.TrajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		traj := snap.Trajectory
		traj.TerminalArtifactHeadRef = snap.Document.CurrentRevisionID
		ok, err := s.lifecycleSettlementReady(ctx, traj, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	commit := func(id string, rec types.CommitmentRecord, cursor *types.InboxCursorAdvance) {
		t.Helper()
		rec.SchemaID, rec.RecordID, rec.Discrepancy = types.CommitmentRecordSchemaV1, id, types.DiscrepancyUnresolved
		rec.EmitterKind = types.CommitmentEmitterAgent
		rec.Provenance = types.CommitmentProvenance{AgentID: rec.SenderInbox, CommittedAt: now.Format(time.RFC3339Nano)}
		req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cmd-" + id, Records: []types.CommitmentRecord{rec}, Cursor: cursor}
		req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatalf("commit %s: %v", id, err)
		}
	}
	if !ready() {
		t.Fatal("a trajectory with closed work and no records is not ready to settle")
	}
	// Texture casts research: an open round.
	commit("cast-1", types.CommitmentRecord{Kind: types.CommitmentKindDirective, Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Body: "find"}, Cast: &types.CommitmentCast{},
		Inbox: research, SenderInbox: texture, Addressee: research}, nil)
	if ready() {
		t.Fatal("the trajectory is ready to settle under an open round with research")
	}
	// Research's final Report closes the round and is unread by Texture.
	commit("report-1", types.CommitmentRecord{Kind: types.CommitmentKindReport, Report: &types.CommitmentReport{StopReason: "completed"}, Final: true,
		Inbox: texture, SenderInbox: research, Addressee: texture}, nil)
	if ready() {
		t.Fatal("the trajectory is ready to settle while Texture has an unread Report")
	}
	// Texture reads it.
	commit("note-1", types.CommitmentRecord{Kind: types.CommitmentKindDirective, Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: "thanks"},
		Inbox: research, SenderInbox: texture, Addressee: research}, &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: 0, Seq: 1})
	if !ready() {
		t.Fatal("a trajectory whose Texture has read every record and has no open round is not ready to settle")
	}
}
