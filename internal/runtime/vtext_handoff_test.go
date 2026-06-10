package runtime

import (
	"context"
	"testing"
)

func TestEnsureVTextHandoffCorpusWakeRequiresChannelID(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()

	reconcilerRun, err := handler.rt.StartRunWithMetadata(ctx, "reconcile story corpus", "user-alice", map[string]any{
		runMetadataAgentProfile:    "corpus-reconciler",
		runMetadataAgentRole:       "corpus-reconciler",
		runMetadataReconcilerScope: "story-corpus",
	})
	if err != nil {
		t.Fatalf("start reconciler run: %v", err)
	}

	_, err = handler.rt.ensureVTextHandoff(ctx, reconcilerRun, vtextHandoffRequest{
		Kind:          vtextHandoffKindCorpusWake,
		CallerProfile: AgentProfileReconciler,
		Objective:     "draft a corpus-wide correction without a target doc",
	})
	if err == nil {
		t.Fatal("corpus_wake handoff without channel_id should fail")
	}
}
