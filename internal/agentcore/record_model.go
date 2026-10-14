package agentcore

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// recordModelID names the model that produced a desk's act, for the
// record's provenance (docs/problems/desk-records-lack-model-id-2026-10-14.md):
// the model that answered the current call (the tool loop installs it on
// the tool context, so a failover is followed), else the run's selected
// model from its metadata. "<provider>/<model>", or "" when unknown.
// It is observation metadata, not act identity: replay digests clear it.
func recordModelID(ctx context.Context, rec *types.RunRecord) string {
	if active, ok := toolregistry.ActiveModelFrom(ctx); ok {
		return active.ID()
	}
	if rec == nil {
		return ""
	}
	selected := toolregistry.ActiveModel{
		Provider: firstNonEmptyString(metadataStringValue(rec.Metadata, modelpolicy.MetadataProvider), metadataStringValue(rec.Metadata, "provider")),
		Model:    firstNonEmptyString(metadataStringValue(rec.Metadata, modelpolicy.MetadataModel), metadataStringValue(rec.Metadata, runMetadataModel)),
	}
	return selected.ID()
}
