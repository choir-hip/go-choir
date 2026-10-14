package textureowner

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A Texture turn's dispatched records name the model that produced the
// turn (docs/problems/desk-records-lack-model-id-2026-10-14.md M2): the
// fold leaves it empty, the author stamps it, and the caller's slice is
// not mutated.
func TestTextureTurnStampsTheAnsweringModel(t *testing.T) {
	rec := &types.RunRecord{Metadata: map[string]any{"llm_provider": "opencode-go", "llm_model": "deepseek-v4.1-flash"}}
	if got := textureTurnModelID(context.Background(), rec); got != "opencode-go/deepseek-v4.1-flash" {
		t.Fatalf("selection = %q", got)
	}
	ctx := toolregistry.WithActiveModel(context.Background(), "chatgpt", "gpt-6-luna")
	model := textureTurnModelID(ctx, rec)
	if model != "chatgpt/gpt-6-luna" {
		t.Fatalf("answering model = %q", model)
	}
	folded := []types.CommitmentRecord{{RecordID: "r1"}, {RecordID: "r2"}}
	stamped := stampRecordModel(folded, model)
	for i := range stamped {
		if stamped[i].Provenance.ModelID != model || folded[i].Provenance.ModelID != "" {
			t.Fatalf("record %d: stamped %q, original %q", i, stamped[i].Provenance.ModelID, folded[i].Provenance.ModelID)
		}
	}
	if got := stampRecordModel(nil, model); got != nil {
		t.Fatalf("no records = %+v", got)
	}
}
