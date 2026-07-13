package provider

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const defaultStubResult = "Task completed successfully (stub provider)."

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
		Result: defaultStubResult,
	}
}

// ProviderName returns "stub" for the stub provider.
func (p *StubProvider) ProviderName() string { return "stub" }

// RuntimeProviderPolicy returns the effective provider/model policy for the
// stub provider.
func (p *StubProvider) RuntimeProviderPolicy() provideriface.ProviderPolicy {
	return provideriface.ProviderPolicy{
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
func (p *StubProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"started","provider":"stub"}`))

	if p.Delay > 0 {
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
	if stubAgentProfile(task) == agentprofile.Conductor &&
		stubRequestedApp(task) == agentprofile.Texture &&
		result == defaultStubResult {
		seedPrompt := ConductorSeedPrompt(task)
		decision, _ := json.Marshal(map[string]any{
			"action":                 "open_app",
			"app":                    agentprofile.Texture,
			"title":                  InitialTextureTitle(seedPrompt, ""),
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

func stubAgentProfile(task *types.RunRecord) string {
	if task == nil {
		return agentprofile.Super
	}
	profile := strings.TrimSpace(task.AgentProfile)
	if profile == "" && task.Metadata != nil {
		profile, _ = task.Metadata["agent_profile"].(string)
	}
	profile = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(profile), "_", "-"))
	if profile == "" {
		return agentprofile.Super
	}
	return profile
}

func stubRequestedApp(task *types.RunRecord) string {
	if task == nil || task.Metadata == nil {
		return ""
	}
	app, _ := task.Metadata["requested_app"].(string)
	return strings.ToLower(strings.TrimSpace(app))
}

// ConductorSeedPrompt returns the explicit seed prompt or falls back to the run prompt.
func ConductorSeedPrompt(task *types.RunRecord) string {
	if task == nil {
		return ""
	}
	seedPrompt, _ := task.Metadata["seed_prompt"].(string)
	if strings.TrimSpace(seedPrompt) == "" {
		seedPrompt = task.Prompt
	}
	return strings.TrimSpace(seedPrompt)
}

// InitialTextureTitle derives the bounded initial title used by conductor Texture decisions.
func InitialTextureTitle(seedPrompt, objective string) string {
	source := strings.TrimSpace(seedPrompt)
	if source == "" {
		source = strings.TrimSpace(objective)
	}
	source = strings.Trim(source, " \t\r\n.:;!?")
	if source == "" {
		return "Working Document"
	}
	words := strings.Fields(source)
	if len(words) > 9 {
		words = words[:9]
	}
	title := strings.Trim(strings.Join(words, " "), " \t\r\n.:;!?")
	if title == "" {
		return "Working Document"
	}
	return title
}
