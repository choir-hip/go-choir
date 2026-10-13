package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Cut 1 L3a: Texture's conversation with research is on records. A Texture
// turn carries its Cast to research as an addressed record and advances its
// own cursor in the turn's transaction; the first record to research:<doc>
// mints the research desk; research answers with a final Report that closes
// the round and advances research's cursor in the same commit.
//
// Ways this could fail, written first: the turn commits but the record does
// not (or the reverse); the research desk is not minted, so its run has no
// subject; the Report lands in another conversation, so the round never
// closes; the cursor moves without the turn or past the head; a replayed
// turn re-mints the record or a second desk.
func TestTextureTurnCastsResearchAndReadsItsReportByCursor(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	owner, computer, doc := start.OwnerID, start.ComputerID, start.InitialDocument.DocID
	textureInbox, researchInbox := start.Agent.AgentID, "research:"+doc

	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID = "texture-turn-records-cast"
	req.InboxRecords = []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-1:cast:tray-1", Kind: types.CommitmentKindDirective,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover chips", Body: `{"kind":"question","summary":"chips"}`},
		Cast:        &types.CommitmentCast{Hypotheses: []types.CastHypothesis{{Statement: "a new chip shipped", Probability: 0.6}}},
		Inbox:       researchInbox, SenderInbox: textureInbox, Addressee: researchInbox,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: textureInbox},
	}}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil {
		t.Fatalf("turn with a research Cast: %v", err)
	}
	if got := result.TextureTurn.InboxRecords; len(got) != 1 || got[0].InboxSeq != 1 || got[0].Round != 1 || !got[0].OpenedRound || len(got[0].HypothesisIDs) != 1 {
		t.Fatalf("turn's record receipt = %+v", got)
	}
	desk, err := s.GetAgentByScope(ctx, owner, computer, researchInbox)
	if err != nil || desk.Profile != "research" || desk.ChannelID != doc || desk.LifecycleVersion != 1 {
		t.Fatalf("research desk minted with the first record = %+v, %v", desk, err)
	}
	if replay, err := s.ApplyTextureTurn(ctx, req); err != nil || !replay.Replay {
		t.Fatalf("replayed turn = %+v, %v", replay, err)
	}
	if head, _ := s.InboxHead(ctx, owner, computer, researchInbox); head != 1 {
		t.Fatalf("research inbox head after replay = %d, want 1", head)
	}

	// Research reads (0, 1], answers with a final Report, and advances its
	// cursor in the same commit.
	recs, err := s.ListInboxRecords(ctx, owner, computer, researchInbox, 0, 1)
	if err != nil || len(recs) != 1 || recs[0].RecordID != "cell-1:cast:tray-1" {
		t.Fatalf("research reads %+v, %v", recs, err)
	}
	report := types.CommitInboxRecordsRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "cell-records:research-cell-1",
		Records: []types.CommitmentRecord{{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: "research-cell-1:report:tray-1", Kind: types.CommitmentKindReport,
			Prediction: types.CommitmentPrediction{Hypothesis: `{"kind":"evidence_update","summary":"it shipped","work_disposition":"completed"}`},
			Report:     &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "it shipped"}}, StopReason: "completed"},
			Final:      true, Inbox: textureInbox, SenderInbox: researchInbox, Addressee: textureInbox,
			EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
			Provenance:  types.CommitmentProvenance{AgentID: researchInbox},
		}},
		Cursor: &types.InboxCursorAdvance{ReaderAgentID: researchInbox, Inbox: researchInbox, ExpectedSeq: 0, Seq: 1},
	}
	report.CommandDigest, _ = ComputeCommitInboxRecordsDigest(report)
	committed, err := s.CommitInboxRecords(ctx, report)
	if err != nil || len(committed.Records) != 1 || !committed.Records[0].ClosedRound || committed.Records[0].ConversationID != result.TextureTurn.InboxRecords[0].ConversationID {
		t.Fatalf("research final Report = %+v, %v", committed, err)
	}
	if cursor, _ := s.ReaderCursor(ctx, owner, computer, researchInbox, researchInbox); cursor != 1 {
		t.Fatalf("research cursor = %d, want 1", cursor)
	}
	if rounds, _ := s.OpenRounds(ctx, owner, computer, researchInbox); len(rounds) != 0 {
		t.Fatalf("round still open after the final Report: %+v", rounds)
	}

	// Texture's next turn reads (0, 1] of its inbox and advances its cursor
	// with the turn; a stale expected cursor is refused with the turn.
	next := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	next.CommandID = "texture-turn-records-read"
	next.InboxCursor = &types.InboxCursorAdvance{ReaderAgentID: textureInbox, Inbox: textureInbox, ExpectedSeq: 0, Seq: 1}
	setTextureTurnDigest(t, &next, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, next); err != nil {
		t.Fatalf("turn reading the Report: %v", err)
	}
	if cursor, _ := s.ReaderCursor(ctx, owner, computer, textureInbox, textureInbox); cursor != 1 {
		t.Fatalf("Texture cursor = %d, want 1", cursor)
	}
	stale := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	stale.CommandID = "texture-turn-records-stale"
	stale.InboxCursor = &types.InboxCursorAdvance{ReaderAgentID: textureInbox, Inbox: textureInbox, ExpectedSeq: 0, Seq: 1}
	setTextureTurnDigest(t, &stale, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, stale); err == nil {
		t.Fatalf("a turn with a stale cursor committed")
	}
}

// A Texture turn on records needs no open work item (Cut 1 core): its
// agent, run and document authorize it, and its cursor advance commits with
// it. A turn without work cannot claim to complete work.
func TestTextureTurnWithoutWorkItemCommitsOnRecords(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	textureInbox := start.Agent.AgentID
	seed := types.CommitInboxRecordsRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "seed-no-work", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "mgmt:note:no-work", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: "fyi"},
		Inbox: textureInbox, SenderInbox: "management:" + start.OwnerID, Addressee: textureInbox,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: "management:" + start.OwnerID},
	}}}
	seed.CommandDigest, _ = ComputeCommitInboxRecordsDigest(seed)
	if _, err := s.CommitInboxRecords(ctx, seed); err != nil {
		t.Fatalf("seed note: %v", err)
	}
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID = "texture-turn-no-work"
	req.CallerWorkItemID = ""
	req.InboxCursor = &types.InboxCursorAdvance{ReaderAgentID: textureInbox, Inbox: textureInbox, ExpectedSeq: 0, Seq: 1}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil {
		t.Fatalf("work-free turn: %v", err)
	}
	if result.WorkItem != nil || result.TextureTurn.InboxCursor == nil || result.TextureTurn.InboxCursor.Seq != 1 {
		t.Fatalf("work-free turn result = work %+v, cursor %+v", result.WorkItem, result.TextureTurn.InboxCursor)
	}
	done := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	done.CommandID = "texture-turn-no-work-completed"
	done.CallerWorkItemID = ""
	done.CallerWorkDisposition = types.WorkItemCompleted
	setTextureTurnDigest(t, &done, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, done); err == nil {
		t.Fatalf("a work-free turn completed work it does not have")
	}
}
