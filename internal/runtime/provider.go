package runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Re-exported from internal/provideriface for backward compatibility.
// New code should import internal/provideriface directly.
type (
	EventEmitFunc   = provideriface.EventEmitFunc
	ProviderPolicy  = provideriface.ProviderPolicy
	Provider        = provideriface.Provider
)

// StubProvider simulates task execution with a configurable delay and optional
// failure. It emits progress events during the simulated work and returns a
// canned result or error.
type StubProvider struct {
	// Delay is the simulated execution duration.
	Delay time.Duration

	// FailErr, if set, causes Execute to return this error instead of
	// succeeding. This simulates provider failures for VAL-RUNTIME-008.
	FailErr error

	// Result is the text returned as the task result on success.
	Result string
}

// NewStubProvider creates a StubProvider with the given delay and default
// result text.
func NewStubProvider(delay time.Duration) *StubProvider {
	return &StubProvider{
		Delay:  delay,
		Result: "Task completed successfully (stub provider).",
	}
}

// ProviderName returns "stub" for the stub provider.
func (p *StubProvider) ProviderName() string { return "stub" }

// RuntimeProviderPolicy returns the effective provider/model policy for the
// stub provider.
func (p *StubProvider) RuntimeProviderPolicy() ProviderPolicy {
	return ProviderPolicy{
		ActiveProvider:              "stub",
		ModelSelection:              "No real upstream model is configured. The sandbox is returning stub responses.",
		SupportsPerRunModelOverride: false,
		Notes: []string{
			"Use a real provider or gateway-backed sandbox to exercise actual model/tool behavior.",
		},
	}
}

// Execute simulates task execution by sleeping for the configured delay,
// emitting progress events, and returning the configured result or error.
func (p *StubProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	// Emit a progress event at the start.
	emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"started","provider":"stub"}`))

	if p.Delay > 0 {
		// Simulate work in increments.
		deadline := time.After(p.Delay)
		tick := time.NewTicker(p.Delay / 4)
		defer tick.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-deadline:
				goto done
			case <-tick.C:
				emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"working","provider":"stub"}`))
			}
		}
	}

done:
	if p.FailErr != nil {
		return p.FailErr
	}
	result := p.Result
	if agentProfileForRun(task) == AgentProfileConductor &&
		isTextureDecisionApp(metadataStringValue(task.Metadata, "requested_app")) &&
		result == "Task completed successfully (stub provider)." {
		seedPrompt := conductorSeedPrompt(task)
		title := buildInitialTextureTitle(seedPrompt, "")
		decision, _ := json.Marshal(map[string]any{
			"action":                 "open_app",
			"app":                    AgentProfileTexture,
			"title":                  title,
			"seed_prompt":            seedPrompt,
			"create_initial_version": false,
		})
		result = string(decision)
	}

	payload, _ := json.Marshal(map[string]string{
		"text":     result,
		"provider": "stub",
	})
	emit(types.EventRunDelta, "execution", payload)
	task.Result = result

	return nil
}

func providerPolicyForRuntime(provider Provider) ProviderPolicy {
	if provider == nil {
		return ProviderPolicy{
			ActiveProvider:              "none",
			ModelSelection:              "No provider is configured.",
			SupportsPerRunModelOverride: false,
		}
	}
	if reporter, ok := provider.(interface{ RuntimeProviderPolicy() ProviderPolicy }); ok {
		policy := reporter.RuntimeProviderPolicy()
		if policy.ActiveProvider == "" {
			policy.ActiveProvider = provider.ProviderName()
		}
		if policy.ModelSelection == "" {
			policy.ModelSelection = "Provider chooses its default model unless a run explicitly requests a model override."
		}
		return policy
	}
	return ProviderPolicy{
		ActiveProvider:              provider.ProviderName(),
		ModelSelection:              "Provider chooses its default model unless a run explicitly requests a model override.",
		SupportsPerRunModelOverride: true,
	}
}
