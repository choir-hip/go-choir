package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// P2a (docs/archive/definitions/choir-protocol-p2-one-surface-2026-10-11.md) on the
// Cut 2 turn: a Texture cell's staged Cast, Ask and Note fold into the cell's
// one turn (Write, Done, Wait, Reject) as addressed records (inbox_records,
// Cut 1 L3a/L3b) with the cursor advance over the range the cell read
// (inbox_cursor) and the frame's source handles, so they commit together.
//
// Ways it can fail: a dispatch commits without a turn, or is refused and
// lost; the cursor advance is dropped; the fold is not deterministic, so a
// replayed cell mints a second revision; the Write is altered; two turns
// commit from one cell; Texture reaches a desk other than research or
// management.

var foldScope = ReductionScope{DeskAgentID: "texture:doc-fold", FromAgentID: "texture:doc-fold", CellID: "cell-fold", ComputerID: "autoputer-test"}

func foldedTurn(t *testing.T, body string) yaegikernel.TextureTurn {
	t.Helper()
	var turn yaegikernel.TextureTurn
	if err := json.Unmarshal([]byte(body), &turn); err != nil {
		t.Fatalf("decode folded turn: %v\n%s", err, body)
	}
	return turn
}

func foldTurn(local string, turn yaegikernel.TextureTurn) yaegikernel.StagedIntent {
	raw, _ := json.Marshal(turn)
	return yaegikernel.StagedIntent{Kind: yaegikernel.IntentTextureTurn, LocalID: local, Body: string(raw)}
}

func TestFoldTextureDispatchCastIntoTurn(t *testing.T) {
	write := yaegikernel.TextureTurn{BaseRevisionID: "rev-1", Write: &yaegikernel.TextureWrite{Edits: "@insert end\nv1", Why: "first"}}
	intents := []yaegikernel.StagedIntent{
		foldTurn("tray-1", write),
		{Kind: yaegikernel.IntentCast, LocalID: "tray-2", ToDesk: agentprofile.Research, Objective: "cover chips",
			Statement: `{"kind":"question","summary":"chips this week","questions":["what shipped?"]}`},
		{Kind: yaegikernel.IntentAsk, LocalID: "tray-3", ToDesk: agentprofile.Research, Question: "who funded it?"},
		{Kind: yaegikernel.IntentNote, LocalID: "tray-4", ToDesk: agentprofile.Research, Body: "prefer primary sources"},
		{Kind: yaegikernel.IntentEscalate, LocalID: "tray-5", ToDesk: agentprofile.Management, Body: "stuck"},
	}
	packet := `{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","target":{"uri":"https://example.org/a"}}]}`
	read := cellRecords{inbox: "texture:doc-fold", after: 2, through: 5, view: []yaegikernel.InboxRecord{{Seq: 5, RecordID: "rec-5", Kind: "report", From: "research:doc-fold", Body: packet}}}
	out, folded, err := foldTextureDispatch(intents, foldScope, read)
	if err != nil {
		t.Fatalf("fold: %v", err)
	}
	if len(folded) != 3 {
		t.Fatalf("folded = %d intents, want 3 (cast, ask, note)", len(folded))
	}
	if len(out) != 2 || out[0].Kind != yaegikernel.IntentTextureTurn || out[1].Kind != yaegikernel.IntentEscalate {
		t.Fatalf("remaining intents = %+v, want the turn and the escalate", out)
	}
	turn := foldedTurn(t, out[0].Body)
	if len(turn.InboxRecords) != 3 {
		t.Fatalf("records = %d, want 3: %s", len(turn.InboxRecords), out[0].Body)
	}
	for i, rec := range turn.InboxRecords {
		if rec.Inbox != "research:doc-fold" || rec.SenderInbox != "texture:doc-fold" || rec.Provenance.CommittedAt != "" {
			t.Fatalf("record %d = %+v, want texture:doc-fold -> research:doc-fold with the time left to the commit", i, rec)
		}
	}
	if d := turn.InboxRecords[0].Directive; d == nil || d.Subtype != types.CommitmentDirectiveCast || d.Objective != "cover chips" || turn.InboxRecords[0].Cast == nil {
		t.Fatalf("cast record = %+v", turn.InboxRecords[0])
	}
	if turn.InboxRecords[1].Precommit == nil || turn.InboxRecords[1].Precommit.Question != "who funded it?" {
		t.Fatalf("ask record = %+v", turn.InboxRecords[1])
	}
	if c := turn.InboxCursor; c == nil || c.Inbox != "texture:doc-fold" || c.ExpectedSeq != 2 || c.Seq != 5 {
		t.Fatalf("cursor advance = %+v, want texture:doc-fold 2 -> 5", c)
	}
	if turn.Write == nil || turn.Write.Edits != write.Write.Edits || turn.BaseRevisionID != "rev-1" {
		t.Fatalf("the Write was altered: %+v", turn)
	}
	if src, ok := turn.Handles["r1.c1"]; !ok || src.URL != "https://example.org/a" {
		t.Fatalf("handles = %+v, want r1.c1 from the record the cell read", turn.Handles)
	}
	again, _, err := foldTextureDispatch(intents, foldScope, read)
	if err != nil || again[0].Body != out[0].Body {
		t.Fatalf("fold is not deterministic: %v\n%s\n%s", err, out[0].Body, again[0].Body)
	}
}

func TestFoldTextureDispatchManagementCast(t *testing.T) {
	scope := foldScope
	scope.OwnerID = "owner-fold"
	out, folded, err := foldTextureDispatch([]yaegikernel.StagedIntent{
		foldTurn("tray-1", yaegikernel.TextureTurn{BaseRevisionID: "rev-1", Wait: "waiting on the build"}),
		{Kind: yaegikernel.IntentCast, LocalID: "tray-2", ToDesk: agentprofile.Management, Objective: "build it",
			Statement: `{"kind":"execution_request","summary":"build","actions":[{"type":"run_tests","objective":"verify the page"}]}`},
		{Kind: yaegikernel.IntentAsk, LocalID: "tray-3", ToDesk: agentprofile.Management, Question: "is a build possible?"},
	}, scope, cellRecords{})
	if err != nil || len(folded) != 2 {
		t.Fatalf("fold: %v (folded %d)", err, len(folded))
	}
	turn := foldedTurn(t, out[0].Body)
	if len(turn.InboxRecords) != 2 || turn.InboxRecords[0].Inbox != "management:owner-fold" || turn.InboxRecords[0].Directive.Objective != "build it" || turn.InboxRecords[1].Precommit == nil {
		t.Fatalf("management records = %+v", turn.InboxRecords)
	}
}

// A cell that dispatched with no turn verb commits as a Wait: the dispatch
// is never lost to a refusal, and it still commits with a turn.
func TestFoldTextureDispatchWithoutATurnCommitsAsWait(t *testing.T) {
	out, folded, err := foldTextureDispatch([]yaegikernel.StagedIntent{
		{Kind: yaegikernel.IntentCast, LocalID: "tray-1", ToDesk: agentprofile.Research, Objective: "cover chips", Statement: `{"kind":"question","summary":"s"}`},
	}, foldScope, cellRecords{inbox: "texture:doc-fold", after: 1, through: 3})
	if err != nil || len(folded) != 1 || len(out) != 1 || out[0].Kind != yaegikernel.IntentTextureTurn {
		t.Fatalf("fold = %+v, %v", out, err)
	}
	turn := foldedTurn(t, out[0].Body)
	if !strings.Contains(turn.Wait, "cover chips") || turn.Write != nil || len(turn.InboxRecords) != 1 || turn.InboxCursor == nil || turn.InboxCursor.Seq != 3 {
		t.Fatalf("turn = %+v", turn)
	}
}

// A cell with no turn and no dispatch commits nothing: its records stay
// unread for the turn that answers them.
func TestFoldTextureDispatchLeavesCellsWithoutATurnAlone(t *testing.T) {
	intents := []yaegikernel.StagedIntent{{Kind: yaegikernel.IntentEscalate, LocalID: "tray-1", Body: "x"}}
	out, folded, err := foldTextureDispatch(intents, foldScope, cellRecords{inbox: "texture:doc-fold", after: 3, through: 4})
	if err != nil || len(folded) != 0 || len(out) != 1 || out[0].Kind != yaegikernel.IntentEscalate {
		t.Fatalf("cell changed: err=%v out=%+v", err, out)
	}
	// A turn with nothing to dispatch still marks the records it read.
	out, _, err = foldTextureDispatch([]yaegikernel.StagedIntent{foldTurn("tray-1", yaegikernel.TextureTurn{BaseRevisionID: "rev-1", Done: "answered"})}, foldScope, cellRecords{inbox: "texture:doc-fold", after: 3, through: 4})
	if c := foldedTurn(t, out[0].Body).InboxCursor; err != nil || c == nil || c.ExpectedSeq != 3 || c.Seq != 4 {
		t.Fatalf("plain turn over unread records: err=%v body=%s", err, out[0].Body)
	}
}

func TestFoldTextureDispatchRefusals(t *testing.T) {
	turn := foldTurn("tray-1", yaegikernel.TextureTurn{BaseRevisionID: "rev-1", Wait: "w"})
	cast := func(desk string) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, LocalID: "tray-9", ToDesk: desk, Objective: "o",
			Statement: `{"kind":"execution_request","summary":"s","actions":[{"type":"run_tests","objective":"a"}]}`}
	}
	for _, tc := range []struct {
		name    string
		intents []yaegikernel.StagedIntent
		want    string
	}{
		{"two turns", []yaegikernel.StagedIntent{turn, foldTurn("tray-2", yaegikernel.TextureTurn{Wait: "x"})}, "one turn"},
		{"other desk", []yaegikernel.StagedIntent{turn, cast(agentprofile.Engineering)}, "research or management"},
		{"management without an owner", []yaegikernel.StagedIntent{turn, cast(agentprofile.Management)}, "no management inbox"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := foldTextureDispatch(tc.intents, foldScope, cellRecords{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// End to end through a real cell: a Texture cell that Writes and Casts to
// research hands the authorizer one turn carrying the Write and the Cast's
// record; a bad alias is refused in the cell with its line and commits
// nothing; a Cast alone commits as a Wait.
func TestDeskGoEvalTextureWriteAndCastCommitAsOneTurn(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	auth := &stubCellAuthorizer{}
	rt.SetTextureCellAuthorizer(auth)
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{DocID: "doc-cast", OwnerID: "owner-1", Title: "Chips", CreatedAt: now, UpdatedAt: now}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s.CreateRevision(ctx, types.Revision{RevisionID: "rev-1", DocID: doc.DocID, OwnerID: doc.OwnerID, AuthorKind: types.AuthorUser,
		Content: "# Chips\n\nWhat shipped this week?", Citations: json.RawMessage("[]"), CreatedAt: now}); err != nil {
		t.Fatalf("create revision: %v", err)
	}
	rec := types.RunRecord{
		RunID: "run-texture-cast", AgentID: "texture:doc-cast", OwnerID: "owner-1", ComputerID: "autoputer-test",
		ChannelID: doc.DocID, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, State: types.RunRunning,
		Metadata: map[string]any{"doc_id": doc.DocID},
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{
		RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: rec.OwnerID, ChannelID: rec.ChannelID, ComputerID: rec.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture, RunRecord: &rec,
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Texture)
	cellCtx := toolregistry.WithExecutionContext(ctx, execCtx)
	run := func(src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(cellCtx, payload)
		if err != nil {
			return out + err.Error()
		}
		return out
	}

	out := run(`import "choir"
_, err := choir.Write("@replace b2\nWe are finding out what shipped.\n\n@delete b7", "acknowledge")
println(err)`)
	if len(auth.seen) != 0 || !strings.Contains(out, "line 4") || !strings.Contains(out, "b7") {
		t.Fatalf("a bad alias was not refused in the cell with its line: calls=%d out=%s", len(auth.seen), out)
	}

	out = run(`choir.Write("@replace b2\nWe are finding out what shipped.", "acknowledge")
choir.Cast("research", "cover chips", map[string]interface{}{"kind":"question","summary":"chips","questions":[]string{"what shipped?"}})`)
	if len(auth.seen) != 1 {
		t.Fatalf("authorizer calls = %d, want 1; out=%s", len(auth.seen), out)
	}
	turn := foldedTurn(t, auth.seen[0])
	if turn.Write == nil || turn.BaseRevisionID != "rev-1" || len(turn.InboxRecords) != 1 || turn.InboxRecords[0].Inbox != "research:doc-cast" || turn.InboxRecords[0].Directive.Objective != "cover chips" {
		t.Fatalf("authorizer turn = %+v\ncell: %s", turn, out)
	}
	if !strings.Contains(out, "rlm:texture_turn:") {
		t.Fatalf("the cell receipt does not name the committed turn: %s", out)
	}

	out = run(`choir.Cast("research", "alone", map[string]interface{}{"kind":"question","summary":"s","questions":[]string{"q?"}})`)
	if len(auth.seen) != 2 || !strings.Contains(foldedTurn(t, auth.seen[1]).Wait, "alone") {
		t.Fatalf("a Cast alone did not commit as a Wait: calls=%d out=%s", len(auth.seen), out)
	}
}
