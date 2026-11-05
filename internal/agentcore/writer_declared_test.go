package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// MA3.2 (DeepSeek B1): the initial Texture decision a run records declares the
// custodian of Texture's decisions, so it is not an undeclared desk-kind write.
func TestInitialTextureDecisionIsWrittenAsTexture(t *testing.T) {
	rt, _ := testRuntime(t)
	objectgraph.ResetWriterGateCounts()
	objectgraph.ResetUndeclaredWrites()
	rec := &types.RunRecord{
		RunID: "run-initial-decision", OwnerID: "owner-decision", AgentProfile: agentprofile.Texture,
		Metadata: map[string]any{
			"doc_id": "doc-initial-decision", "texture_initial_decision_required": true,
			"texture_initial_decision_reason": "nothing to chase", "texture_initial_decision_kind": "no_worker_needed",
		},
	}
	if err := rt.recordExplicitInitialTextureDecisionIfNeeded(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if rec.Metadata["texture_initial_decision_recorded"] != true {
		t.Fatalf("the decision was not recorded: %v", rec.Metadata)
	}
	if got := objectgraph.UndeclaredWrites(); len(got) != 0 {
		t.Fatalf("undeclared writes: %v", got)
	}
	if got := objectgraph.WriterGateCounts(); len(got) != 0 {
		t.Fatalf("decision counted as a violation: %v", got)
	}
}

// N6: a management Cast that opens a story counts as desk:management writing
// Texture's kinds through the reducer's own declaration, not a hand-supplied
// writer.
func TestReducerDeclaresTheRunsDeskForAStoryOpening(t *testing.T) {
	ctx := withDeskWriter(context.Background(), agentprofile.Management)
	w, ok := objectgraph.WriterFrom(ctx)
	if !ok || w != objectgraph.WriterManagement {
		t.Fatalf("withDeskWriter(management) = %q, %v", w, ok)
	}
	if _, ok := objectgraph.WriterFrom(withDeskWriter(context.Background(), "no-such-desk")); ok {
		t.Fatal("an unregistered profile declared a writer")
	}
}
