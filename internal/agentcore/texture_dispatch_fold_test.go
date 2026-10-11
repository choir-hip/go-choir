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
// Texture cell's staged Cast, Ask and Note to research or management fold
// into the cell's one apply or decide ApplyTexture body as controls, so the
// dispatch commits with the turn through textureTurnControls and the P1
// accumulator. Everything else about the cell is unchanged.

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
	out, folded, err := foldTextureDispatch(intents)
	if err != nil {
		t.Fatalf("fold: %v", err)
	}
	if len(folded) != 3 {
		t.Fatalf("folded = %d intents, want 3 (cast, ask, note)", len(folded))
	}
	if len(out) != 2 || out[0].Kind != yaegikernel.IntentTextureApply || out[1].Kind != yaegikernel.IntentEscalate {
		t.Fatalf("remaining intents = %+v, want the apply and the escalate", out)
	}
	controls := foldedControls(t, out[0].Body)
	if len(controls) != 3 {
		t.Fatalf("controls = %d, want 3: %s", len(controls), out[0].Body)
	}
	for i, want := range []string{"cast", "ask", "note"} {
		if controls[i]["open_researcher"] != true || controls[i]["staged_act"] != want {
			t.Fatalf("control %d = %v, want a research opener with staged_act %q", i, controls[i], want)
		}
	}
	if controls[0]["objective"] != "cover chips" {
		t.Fatalf("cast objective = %v", controls[0]["objective"])
	}
	if pkt, _ := controls[1]["packet"].(map[string]any); pkt["kind"] != "question" || pkt["summary"] != "who funded it?" {
		t.Fatalf("ask packet = %v", controls[1]["packet"])
	}
	if !strings.Contains(out[0].Body, `"content":"v1"`) || !strings.Contains(out[0].Body, `"base_revision_id":"rev-1"`) {
		t.Fatalf("the authored edit was not preserved: %s", out[0].Body)
	}

	// A replayed cell folds to the same bytes, so the revision identity
	// derived from the body is stable.
	again, _, err := foldTextureDispatch(intents)
	if err != nil || again[0].Body != out[0].Body {
		t.Fatalf("fold is not deterministic: %v\n%s\n%s", err, out[0].Body, again[0].Body)
	}
}

func TestFoldTextureDispatchManagementCast(t *testing.T) {
	out, folded, err := foldTextureDispatch([]yaegikernel.StagedIntent{
		foldApply("tray-1", `{"op":"decide","decision_kind":"wait_for_evidence","reason":"r"}`),
		{Kind: yaegikernel.IntentCast, LocalID: "tray-2", ToDesk: agentprofile.Management, Objective: "build it",
			Statement: `{"kind":"execution_request","summary":"build","actions":[{"type":"run_tests","objective":"verify the page","safety":{"mutation_class":"yellow","network":"forbidden","file_mutation":"allowed"}}]}`},
	})
	if err != nil || len(folded) != 1 {
		t.Fatalf("fold: %v (folded %d)", err, len(folded))
	}
	controls := foldedControls(t, out[0].Body)
	if len(controls) != 1 || controls[0]["open_persistent_super"] != true || controls[0]["staged_act"] != "cast" {
		t.Fatalf("management control = %v", controls)
	}
}

func TestFoldTextureDispatchLeavesPlainCellsByteIdentical(t *testing.T) {
	body := `{"content":"v1","op":"apply","base_revision_id":"rev-1"}`
	intents := []yaegikernel.StagedIntent{foldApply("tray-1", body)}
	out, folded, err := foldTextureDispatch(intents)
	if err != nil || len(folded) != 0 || len(out) != 1 || out[0].Body != body {
		t.Fatalf("plain cell changed: err=%v folded=%d body=%q", err, len(folded), out[0].Body)
	}
}

// An empty controls array (older prompt examples carried one) is not a
// dispatch and is not refused.
func TestFoldTextureDispatchAcceptsEmptyControls(t *testing.T) {
	body := `{"op":"apply","content":"v1","controls":[]}`
	out, _, err := foldTextureDispatch([]yaegikernel.StagedIntent{foldApply("tray-1", body)})
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
		{"dispatch without a turn", []yaegikernel.StagedIntent{cast(agentprofile.Research)}, "with an apply or decide"},
		{"two applies", []yaegikernel.StagedIntent{apply, foldApply("tray-2", `{"op":"apply","content":"v2"}`), cast(agentprofile.Research)}, "one ApplyTexture"},
		{"email turn", []yaegikernel.StagedIntent{foldApply("tray-1", `{"op":"email","email":{}}`), cast(agentprofile.Research)}, "with an apply or decide"},
		{"other desk", []yaegikernel.StagedIntent{apply, cast(agentprofile.Engineering)}, "research or management"},
		{"ask management", []yaegikernel.StagedIntent{apply, {Kind: yaegikernel.IntentAsk, LocalID: "tray-3", ToDesk: agentprofile.Management, Question: "q"}}, "choir.Cast"},
		{"cast spec instead of a packet", []yaegikernel.StagedIntent{apply, {Kind: yaegikernel.IntentCast, LocalID: "tray-4",
			ToDesk: agentprofile.Research, Objective: "o", Statement: `{"statement":"look into it"}`}}, "not a cast spec"},
		{"two management casts", []yaegikernel.StagedIntent{apply, cast(agentprofile.Management), cast(agentprofile.Management)}, "one management"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := foldTextureDispatch(tc.intents)
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
		RunID: "run-texture-cast", AgentID: "agent-texture", OwnerID: "owner-1", ComputerID: "autoputer-test",
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
	controls := foldedControls(t, auth.seen[0])
	if len(controls) != 1 || controls[0]["open_researcher"] != true || controls[0]["staged_act"] != "cast" || controls[0]["objective"] != "cover chips" {
		t.Fatalf("authorizer body controls = %v\nbody=%s", controls, auth.seen[0])
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
