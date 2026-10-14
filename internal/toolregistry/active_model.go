package toolregistry

import (
	"context"
	"strings"
)

// The answering model (docs/problems/desk-records-lack-model-id-2026-10-14.md).
// The tool loop puts the provider and model of the call whose tool calls it
// is executing on the tools' context. After a failover that is the fallback
// model, not the run's selection, so a record a tool writes names the model
// that actually produced the act.

type activeModelKey struct{}

// ActiveModel is the provider and model that answered the current call.
type ActiveModel struct {
	Provider string
	Model    string
}

// ID renders the model as "<provider>/<model>", or the model alone when
// the provider is unknown. Empty when the model is unknown.
func (m ActiveModel) ID() string {
	provider, model := strings.TrimSpace(m.Provider), strings.TrimSpace(m.Model)
	switch {
	case model == "":
		return ""
	case provider == "":
		return model
	}
	return provider + "/" + model
}

// WithActiveModel installs the answering model on ctx. An empty model
// leaves ctx unchanged.
func WithActiveModel(ctx context.Context, provider, model string) context.Context {
	if strings.TrimSpace(model) == "" {
		return ctx
	}
	return context.WithValue(ctx, activeModelKey{}, ActiveModel{Provider: strings.TrimSpace(provider), Model: strings.TrimSpace(model)})
}

// ActiveModelFrom returns the answering model installed on ctx.
func ActiveModelFrom(ctx context.Context) (ActiveModel, bool) {
	if ctx == nil {
		return ActiveModel{}, false
	}
	m, ok := ctx.Value(activeModelKey{}).(ActiveModel)
	return m, ok && m.Model != ""
}
