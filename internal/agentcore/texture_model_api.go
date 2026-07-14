package agentcore

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TextureModelPolicy returns the concrete policy manager used by Texture-owned
// model operations.
func (rt *Runtime) TextureModelPolicy() *modelpolicy.Manager {
	if rt == nil {
		return nil
	}
	return rt.modelPolicy
}

// TextureProvider returns the concrete provider used by Texture-owned model
// operations.
func (rt *Runtime) TextureProvider() provideriface.Provider {
	if rt == nil {
		return nil
	}
	return rt.provider
}

// PromptBarDecisionSpec is the product-router decision persisted on a
// conductor submission before an app owner materializes its state.
type PromptBarDecisionSpec struct {
	Action    string
	App       string
	Title     string
	SourceURL string
	MediaType string
	AppHint   string
}

// CompletePromptBarDecision records a deterministic conductor decision without
// taking ownership of the selected app's state.
func (rt *Runtime) CompletePromptBarDecision(ctx context.Context, text, ownerID string, metadata map[string]any, spec PromptBarDecisionSpec) (*types.RunRecord, error) {
	return rt.completePromptBarDecisionRun(ctx, text, ownerID, metadata, conductorDecision{
		Action:    spec.Action,
		App:       spec.App,
		Title:     spec.Title,
		SourceURL: spec.SourceURL,
		MediaType: spec.MediaType,
		AppHint:   spec.AppHint,
	})
}
