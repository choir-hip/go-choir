package textureowner

import (
	"context"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// textureTurnModelID names the model that produced a Texture turn
// (docs/problems/desk-records-lack-model-id-2026-10-14.md): the model that
// answered the call (on the tool context), else the run's selection.
func textureTurnModelID(ctx context.Context, rec *types.RunRecord) string {
	if active, ok := toolregistry.ActiveModelFrom(ctx); ok {
		return active.ID()
	}
	if rec == nil {
		return ""
	}
	return toolregistry.ActiveModel{
		Provider: firstNonEmpty(metadataStringValue(rec.Metadata, "llm_provider"), metadataStringValue(rec.Metadata, "provider")),
		Model:    firstNonEmpty(metadataStringValue(rec.Metadata, "llm_model"), metadataStringValue(rec.Metadata, "model")),
	}.ID()
}

// stampRecordModel sets the model on the records a Texture turn dispatches.
// The fold leaves it empty so the turn's identity does not depend on it;
// the store's replay digest clears it again.
func stampRecordModel(records []types.CommitmentRecord, modelID string) []types.CommitmentRecord {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" || len(records) == 0 {
		return records
	}
	out := make([]types.CommitmentRecord, len(records))
	for i, rec := range records {
		rec.Provenance.ModelID = modelID
		out[i] = rec
	}
	return out
}
