package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// P2a (docs/definitions/choir-protocol-p2-one-surface-2026-10-11.md): a
// Texture cell's staged dispatch folds into the cell's one apply or decide
// ApplyTexture body, so it commits with the turn. Cut 1 L3a: dispatches to
// research fold as addressed records (inbox_records) with the cursor advance
// over the range the cell read (inbox_cursor); a management Cast is still a
// control until L3b. Everything else about the cell is unchanged.

var foldScope = ReductionScope{DeskAgentID: "texture:doc-fold", FromAgentID: "texture:doc-fold", CellID: "cell-fold", ComputerID: "autoputer-test"}

func foldedInbox(t *testing.T, body string) ([]types.CommitmentRecord, *types.InboxCursorAdvance) {
	t.Helper()
	var decoded struct {
		Records []types.CommitmentRecord  `json:"inbox_records"`
		Cursor  *types.InboxCursorAdvance `json:"inbox_cursor"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("decode folded body: %v\n%s", err, body)
	}
	return decoded.Records, decoded.Cursor
}

func foldApply(local, body string) yaegikernel.StagedIntent {
	return yaegikernel.StagedIntent{Kind: yaegikernel.IntentTextureApply, LocalID: local, Body: body}
}

func foldedControls(t *testing.T, body string) []map[string]any {
	t.Helper()
	var decoded struct {
		Controls []map[string]any `json:"controls"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("decode folded body: %v\n%s", err, body)
	}
	return decoded.Controls
}

func TestFoldTextureDispatchCastIntoApply(t *testing.T) {
	intents := []yaegikernel.StagedIntent{
		foldApply("tray-1", `{"op":"apply","base_revision_id":"rev-1","content":"v1"}`),
		{Kind: yaegikernel.IntentCast, LocalID: "tray-2", ToDesk: agentprofile.Research, Objective: "cover chips",
			Statement: `{"kind":"question","summary":"chips this week","questions":["what shipped?"]}`},
		{Kind: yaegikernel.IntentAsk, LocalID: "tray-3", ToDesk: agentprofile.Research, Question: "who funded it?"},
		{Kind: yaegikernel.IntentNote, LocalID: "tray-4", ToDesk: agentprofile.Research, Body: "prefer primary sources"},
		{Kind: yaegikernel.IntentEscalate, LocalID: "tray-5", ToDesk: agentprofile.Management, Body: "stuck"},
	}
	read := cellRecords{inbox: "texture:doc-fold", after: 2, through: 5}
	out, folded, err := foldTextureDispatch(intents, foldScope, read)
	if err != nil {
		t.Fatalf("fold: %v", err)
	}
	if len(folded) != 3 {
		t.Fatalf("folded = %d intents, want 3 (cast, ask, note)", len(folded))
	}
	if len(out) != 2 || out[0].Kind != yaegikernel.IntentTextureApply || out[1].Kind != yaegikernel.IntentEscalate {
		t.Fatalf("remaining intents = %+v, want the apply and the escalate", out)
	}
	if controls := foldedControls(t, out[0].Body); len(controls) != 0 {
		t.Fatalf("research dispatch became controls: %s", out[0].Body)
	}
	records, cursor := foldedInbox(t, out[0].Body)
	if len(records) != 3 {
		t.Fatalf("records = %d, want 3: %s", len(records), out[0].Body)
	}
	for i, rec := range records {
		if rec.Inbox != "research:doc-fold" || rec.SenderInbox != "texture:doc-fold" || rec.Provenance.CommittedAt != "" {
			t.Fatalf("record %d = %+v, want texture:doc-fold -> research:doc-fold with the time left to the commit", i, rec)
		}
	}
	if d := records[0].Directive; d == nil || d.Subtype != types.CommitmentDirectiveCast || d.Objective != "cover chips" || records[0].Cast == nil {
		t.Fatalf("cast record = %+v", records[0])
	}
	if records[1].Precommit == nil || records[1].Precommit.Question != "who funded it?" {
		t.Fatalf("ask record = %+v", records[1])
	}
	if d := records[2].Directive; d == nil || d.Subtype != types.CommitmentDirectiveNote || d.Body != "prefer primary sources" {
		t.Fatalf("note record = %+v", records[2])
	}
	if cursor == nil || cursor.Inbox != "texture:doc-fold" || cursor.ExpectedSeq != 2 || cursor.Seq != 5 {
		t.Fatalf("cursor advance = %+v, want texture:doc-fold 2 -> 5", cursor)
	}
	if !strings.Contains(out[0].Body, `"content":"v1"`) || !strings.Contains(out[0].Body, `"base_revision_id":"rev-1"`) {
		t.Fatalf("the authored edit was not preserved: %s", out[0].Body)
	}

	// A replayed cell folds to the same bytes, so the revision identity
	// derived from the body is stable.
	again, _, err := foldTextureDispatch(intents, foldScope, read)
	if err != nil || again[0].Body != out[0].Body {
		t.Fatalf("fold is not deterministic: %v\n%s\n%s", err, out[0].Body, again[0].Body)
	}
}

func TestFoldTextureDispatchManagementCast(t *testing.T) {
	scope := foldScope
	scope.OwnerID = "owner-fold"
	out, folded, err := foldTextureDispatch([]yaegikernel.StagedIntent{
		foldApply("tray-1", `{"op":"decide","decision_kind":"wait_for_evidence","reason":"r"}`),
		{Kind: yaegikernel.IntentCast, LocalID: "tray-2", ToDesk: agentprofile.Management, Objective: "build it",
			Statement: `{"kind":"execution_request","summary":"build","actions":[{"type":"run_tests","objective":"verify the page"}]}`},
		{Kind: yaegikernel.IntentAsk, LocalID: "tray-3", ToDesk: agentprofile.Management, Question: "is a build possible?"},
	}, scope, cellRecords{})
	if err != nil || len(folded) != 2 {
		t.Fatalf("fold: %v (folded %d)", err, len(folded))
	}
	records, _ := foldedInbox(t, out[0].Body)
	if len(records) != 2 || records[0].Inbox != "management:owner-fold" || records[0].Directive == nil || records[0].Directive.Objective != "build it" || records[1].Precommit == nil {
		t.Fatalf("management records = %+v\nbody=%s", records, out[0].Body)
	}
}

func TestFoldTextureDispatchLeavesPlainCellsByteIdentical(t *testing.T) {
	body := `{"content":"v1","op":"apply","base_revision_id":"rev-1"}`
	intents := []yaegikernel.StagedIntent{foldApply("tray-1", body)}
	out, folded, err := foldTextureDispatch(intents, foldScope, cellRecords{inbox: "texture:doc-fold", after: 3, through: 3})
	if err != nil || len(folded) != 0 || len(out) != 1 || out[0].Body != body {
		t.Fatalf("plain cell changed: err=%v folded=%d body=%q", err, len(folded), out[0].Body)
	}
	// A turn with nothing to dispatch still marks the records it read.
	out, _, err = foldTextureDispatch(intents, foldScope, cellRecords{inbox: "texture:doc-fold", after: 3, through: 4})
	if _, cursor := foldedInbox(t, out[0].Body); err != nil || cursor == nil || cursor.ExpectedSeq != 3 || cursor.Seq != 4 {
		t.Fatalf("plain turn over unread records: err=%v body=%s", err, out[0].Body)
	}
}

// An empty controls array (older prompt examples carried one) is not a
// dispatch and is not refused.
func TestFoldTextureDispatchAcceptsEmptyControls(t *testing.T) {
	body := `{"op":"apply","content":"v1","controls":[]}`
	out, _, err := foldTextureDispatch([]yaegikernel.StagedIntent{foldApply("tray-1", body)}, foldScope, cellRecords{})
	if err != nil || out[0].Body != body {
		t.Fatalf("empty controls refused or changed: err=%v body=%q", err, out[0].Body)
	}
}

func TestFoldTextureDispatchRefusals(t *testing.T) {
	apply := foldApply("tray-1", `{"op":"apply","base_revision_id":"rev-1","content":"v1"}`)
	cast := func(desk string) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, LocalID: "tray-9", ToDesk: desk, Objective: "o",
			Statement: `{"kind":"execution_request","summary":"s","actions":[{"type":"run_tests","objective":"a","safety":{"mutation_class":"yellow","network":"forbidden","file_mutation":"allowed"}}]}`}
	}
	for _, tc := range []struct {
		name    string
		intents []yaegikernel.StagedIntent
		want    string
	}{
		{"authored controls", []yaegikernel.StagedIntent{
			foldApply("tray-1", `{"op":"apply","content":"v1","controls":[{"open_researcher":true}]}`),
		}, "choir.Cast"},
		{"authored inbox records", []yaegikernel.StagedIntent{
			foldApply("tray-1", `{"op":"apply","content":"v1","inbox_records":[]}`),
		}, "inbox_records"},
		{"authored inbox cursor", []yaegikernel.StagedIntent{
			foldApply("tray-1", `{"op":"apply","content":"v1","inbox_cursor":{"seq":9}}`),
		}, "inbox_cursor"},
		{"dispatch without a turn", []yaegikernel.StagedIntent{cast(agentprofile.Research)}, "with an apply or decide"},
		{"two applies", []yaegikernel.StagedIntent{apply, foldApply("tray-2", `{"op":"apply","content":"v2"}`), cast(agentprofile.Research)}, "one ApplyTexture"},
		{"email turn", []yaegikernel.StagedIntent{foldApply("tray-1", `{"op":"email","email":{}}`), cast(agentprofile.Research)}, "with an apply or decide"},
		{"other desk", []yaegikernel.StagedIntent{apply, cast(agentprofile.Engineering)}, "research or management"},
		{"management without an owner", []yaegikernel.StagedIntent{apply, cast(agentprofile.Management)}, "no management inbox"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := foldTextureDispatch(tc.intents, foldScope, cellRecords{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// End to end through a real cell: a Texture cell that stages ApplyTexture and
// a Cast to research hands the authorizer one body whose controls carry the
// Cast; a Cast with no turn in its cell is refused and commits nothing.
func TestDeskGoEvalTextureCastFoldsIntoTheTurn(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	auth := &stubCellAuthorizer{}
	rt.SetTextureCellAuthorizer(auth)
	rec := types.RunRecord{
		RunID: "run-texture-cast", AgentID: "texture:doc-cast", OwnerID: "owner-1", ComputerID: "autoputer-test",
		ChannelID: "channel-texture", AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, State: types.RunRunning,
	}
	if err := s.CreateRun(context.Background(), rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{
		RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: rec.OwnerID, ChannelID: rec.ChannelID, ComputerID: rec.ComputerID,
		Profile: agentprofile.Texture, Role: agentprofile.Texture, RunRecord: &rec,
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Texture)
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	src := `import "choir"
choir.ApplyTexture(map[string]interface{}{"op":"apply","base_revision_id":"rev-1","content":"v1"})
choir.Cast("research", "cover chips", map[string]interface{}{"kind":"question","summary":"chips","questions":[]string{"what shipped?"}})`
	payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
	out, err := tool.Func(ctx, payload)
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	if len(auth.seen) != 1 {
		t.Fatalf("authorizer calls = %d, want 1; out=%s", len(auth.seen), out)
	}
	records, _ := foldedInbox(t, auth.seen[0])
	if len(records) != 1 || records[0].Inbox != "research:doc-cast" || records[0].Directive == nil || records[0].Directive.Objective != "cover chips" {
		t.Fatalf("authorizer body records = %+v\nbody=%s", records, auth.seen[0])
	}

	lone := `import "choir"
choir.Cast("research", "alone", map[string]interface{}{"kind":"question","summary":"s","questions":[]string{"q?"}})`
	payload, _ = json.Marshal(map[string]any{"source": lone, "timeout_ms": 15000})
	out, err = tool.Func(ctx, payload)
	refusal := out
	if err != nil {
		refusal += err.Error()
	}
	if len(auth.seen) != 1 || !strings.Contains(refusal, "apply or decide") {
		t.Fatalf("a Cast with no turn was not refused: calls=%d out=%s err=%v", len(auth.seen), out, err)
	}
}
