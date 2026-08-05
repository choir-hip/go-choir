package textureowner

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestEnsureTextureHandoffCorpusWakeRequiresChannelID(t *testing.T) {
	h := &Handler{}
	reconcilerRun := &types.RunRecord{
		OwnerID:      "user-alice",
		AgentProfile: agentprofile.Reconciler,
		AgentRole:    agentprofile.Reconciler,
	}

	_, err := h.EnsureTextureHandoff(context.Background(), reconcilerRun, HandoffRequest{
		Kind:          HandoffKindCorpusWake,
		CallerProfile: agentprofile.Reconciler,
		Objective:     "draft a corpus-wide correction without a target doc",
	})
	if err == nil {
		t.Fatal("corpus_wake handoff without channel_id should fail")
	}
}
