package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// model_id on desk records (docs/problems/desk-records-lack-model-id-2026-10-14.md).
// Ways it can fail, written before the fix: M3 the run's selection wins
// over the model that answered (failover); M4 a desk act misses it; M2 the
// Texture fold carries it and a failover changes the turn's identity.

func TestRecordModelIDPrefersTheAnsweringModel(t *testing.T) {
	rec := &types.RunRecord{Metadata: map[string]any{"llm_provider": "opencode-go", "llm_model": "deepseek-v4.1-flash"}}
	if got := recordModelID(context.Background(), rec); got != "opencode-go/deepseek-v4.1-flash" {
		t.Fatalf("from metadata = %q", got)
	}
	failedOver := toolregistry.WithActiveModel(context.Background(), "chatgpt", "gpt-6-luna")
	if got := recordModelID(failedOver, rec); got != "chatgpt/gpt-6-luna" {
		t.Fatalf("after failover = %q, want the answering model", got)
	}
	legacy := &types.RunRecord{Metadata: map[string]any{"provider": "chatgpt", "model": "gpt-6-luna"}}
	if got := recordModelID(context.Background(), legacy); got != "chatgpt/gpt-6-luna" {
		t.Fatalf("from the mirrored keys = %q", got)
	}
	if got := recordModelID(context.Background(), nil); got != "" {
		t.Fatalf("no run, no context = %q, want empty", got)
	}
}

func TestDeskActsCarryTheScopeModel(t *testing.T) {
	scope := ReductionScope{FromAgentID: "management:o", DeskAgentID: "management:o", CellID: "cell-m", OwnerID: "o", ComputerID: "c", ModelID: "opencode-go/deepseek-v4.1-flash"}
	intents := []yaegikernel.StagedIntent{
		{Kind: yaegikernel.IntentCast, LocalID: "t1", ToDesk: "texture:new", Objective: "open", Statement: `{"hypotheses":[{"statement":"x","probability":0.7}]}`},
		{Kind: yaegikernel.IntentReport, LocalID: "t2", ToDesk: "texture", Claim: "done"},
		{Kind: yaegikernel.IntentAsk, LocalID: "t3", ToDesk: "research", Question: "who?"},
		{Kind: yaegikernel.IntentPrecommit, LocalID: "t4", Precommit: `{"question":"q","distribution":{"a":0.6,"b":0.4},"resolver":"research"}`},
	}
	for _, in := range intents {
		if got := commitmentRecordForIntent(scope, in).Provenance.ModelID; got != scope.ModelID {
			t.Errorf("%s record model_id = %q, want %q", in.Kind, got, scope.ModelID)
		}
	}
	rec, err := inboxRecordForIntent(scope, intents[0], "texture:doc-1", false)
	if err != nil || rec.Provenance.ModelID != scope.ModelID {
		t.Fatalf("records-route Cast model_id = %q (%v)", rec.Provenance.ModelID, err)
	}
}

func TestTextureFoldLeavesTheModelToTheCommit(t *testing.T) {
	scope := foldScope
	scope.ModelID = "chatgpt/gpt-6-luna"
	intents := []yaegikernel.StagedIntent{{Kind: yaegikernel.IntentCast, LocalID: "tray-2", ToDesk: agentprofile.Research, Objective: "o",
		Statement: `{"kind":"question","summary":"s","questions":["q?"]}`}}
	out, _, err := foldTextureDispatch(intents, scope, cellRecords{})
	if err != nil || len(out) != 1 {
		t.Fatalf("fold: %v %+v", err, out)
	}
	other := scope
	other.ModelID = "opencode-go/deepseek-v4.1-flash"
	again, _, _ := foldTextureDispatch(intents, other, cellRecords{})
	if out[0].Body != again[0].Body {
		t.Fatalf("the folded turn depends on the model:\n%s\n%s", out[0].Body, again[0].Body)
	}
	var turn yaegikernel.TextureTurn
	_ = json.Unmarshal([]byte(out[0].Body), &turn)
	if len(turn.InboxRecords) != 1 || turn.InboxRecords[0].Provenance.ModelID != "" {
		t.Fatalf("fold records = %+v, want the model left to the commit", turn.InboxRecords)
	}
}
