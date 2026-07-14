package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TextureActorToolLoopBudgetSpend is the persisted spend carried across
// activations of one resident Texture actor.
type TextureActorToolLoopBudgetSpend struct {
	SourceRunID        string
	ProviderCalls      int
	InputTokens        int
	OutputTokens       int
	ObservedUsageEvent bool
}

// DispatchActor sends a concrete actor-runtime message through the configured
// actor execution substrate.
func (rt *Runtime) DispatchActor(ctx context.Context, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
	if rt == nil || rt.dispatchActor == nil {
		return fmt.Errorf("runtime: actor dispatch unavailable")
	}
	return rt.dispatchActor(ctx, toAgentID, kind, content, trajectoryID, fromAgentID)
}

// ActivateRun dispatches a persisted run's initial actor activation.
func (rt *Runtime) ActivateRun(rec *types.RunRecord) {
	if rt == nil || rt.dispatchActor == nil {
		panic("runtime: activate called without dispatchActor set — actor runtime is required")
	}
	if rec == nil {
		panic("runtime: activate called with nil run")
	}
	agentID := strings.TrimSpace(rec.AgentID)
	if agentID == "" {
		panic("runtime: activate called with empty AgentID")
	}
	trajectoryID := metadataStringValue(rec.Metadata, runMetadataTrajectoryID)
	if err := rt.dispatchActor(context.Background(), agentID, "initial_dispatch", rec.RunID, trajectoryID, ""); err != nil {
		log.Printf("runtime: activate dispatch for run %s: %v", rec.RunID, err)
	}
}

// TextureActorParkIdle returns the configured resident Texture idle interval.
func (rt *Runtime) TextureActorParkIdle() time.Duration {
	if rt == nil {
		return 0
	}
	return rt.cfg.TextureActorParkIdle
}

// TextureSandboxID returns the runtime sandbox identity used for durable actor records.
func (rt *Runtime) TextureSandboxID() string {
	if rt == nil {
		return ""
	}
	return rt.cfg.SandboxID
}

// TextureActiveRunByAgent returns the latest executing run for an actor.
func (rt *Runtime) TextureActiveRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, bool, error) {
	if rt == nil || rt.store == nil {
		return types.RunRecord{}, false, nil
	}
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return types.RunRecord{}, false, nil
	}
	rec, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, agentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return types.RunRecord{}, false, nil
		}
		return types.RunRecord{}, false, err
	}
	if rec.State == types.RunBlocked {
		return types.RunRecord{}, false, nil
	}
	return rec, true, nil
}

// TextureChannelHasGroundedHistory reports whether grounded worker actors have
// already delivered a channel message before the optional boundary.
func (rt *Runtime) TextureChannelHasGroundedHistory(ctx context.Context, ownerID, channelID string, before time.Time) (bool, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return false, nil
	}
	runs, err := rt.store.ListRunsByChannel(ctx, ownerID, channelID, 500)
	if err != nil {
		return false, err
	}
	groundedRunIDs := make(map[string]struct{})
	for _, run := range runs {
		if !before.IsZero() && !run.CreatedAt.Before(before) {
			continue
		}
		switch agentProfileForRun(&run) {
		case agentprofile.Researcher, agentprofile.Super, agentprofile.CoSuper:
			groundedRunIDs[run.RunID] = struct{}{}
		}
	}
	if len(groundedRunIDs) == 0 {
		return false, nil
	}
	messages, err := rt.store.ListChannelMessages(ctx, ownerID, channelID, 0, 500)
	if err != nil {
		return false, err
	}
	for _, message := range messages {
		if !before.IsZero() && !message.Timestamp.Before(before) {
			continue
		}
		if _, ok := groundedRunIDs[strings.TrimSpace(message.FromRunID)]; ok {
			return true, nil
		}
	}
	return false, nil
}

// LatestTextureActorToolLoopBudgetSpend loads durable provider/tool-loop spend
// from the previous activation of the same actor.
func (rt *Runtime) LatestTextureActorToolLoopBudgetSpend(ctx context.Context, ownerID, agentID string) (TextureActorToolLoopBudgetSpend, bool, error) {
	var spend TextureActorToolLoopBudgetSpend
	if rt == nil || rt.store == nil {
		return spend, false, nil
	}
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return spend, false, nil
	}
	sourceRunID, _, err := rt.store.LatestActorRunMemoryEntries(ctx, ownerID, agentID, "")
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return spend, false, nil
		}
		return spend, false, err
	}
	spend.SourceRunID = sourceRunID
	eventsForRun, err := rt.store.ListEvents(ctx, sourceRunID, 5000)
	if err != nil {
		return spend, false, err
	}
	providerCallsFromPreflight := 0
	for _, event := range eventsForRun {
		if event.Kind != types.EventRunProgress {
			continue
		}
		switch event.Phase {
		case "provider_call":
			providerCallsFromPreflight++
		case "tool_loop_budget_usage", "tool_loop_budget":
			var payload map[string]any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				continue
			}
			spend.ObservedUsageEvent = true
			if value := metadataIntValue(payload, "provider_calls"); value > spend.ProviderCalls {
				spend.ProviderCalls = value
			}
			if value := metadataIntValue(payload, "input_tokens"); value > spend.InputTokens {
				spend.InputTokens = value
			}
			if value := metadataIntValue(payload, "output_tokens"); value > spend.OutputTokens {
				spend.OutputTokens = value
			}
		}
	}
	if spend.ProviderCalls == 0 && providerCallsFromPreflight > 0 {
		spend.ProviderCalls = providerCallsFromPreflight
	}
	return spend, true, nil
}
