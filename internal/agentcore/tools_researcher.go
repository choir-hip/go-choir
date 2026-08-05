package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type researchFindingEvidenceInput struct {
	Kind      string          `json:"kind"`
	SourceURI string          `json:"source_uri,omitempty"`
	Title     string          `json:"title,omitempty"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

func resolveFindingsTarget(ctx context.Context, rt *Runtime, explicitAgentID string, packetKinds ...string) (string, string, error) {
	packetKind := ""
	if len(packetKinds) > 0 {
		packetKind = packetKinds[0]
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	profile := agentprofile.Canonical(execution.Profile)
	if profile == "" && execution.RunRecord != nil {
		profile = agentprofile.Canonical(agentProfileForRun(execution.RunRecord))
	}
	if profile == agentprofile.Researcher {
		return resolveResearcherFindingsTarget(ctx, rt, explicitAgentID)
	}
	if profile == agentprofile.Texture && strings.TrimSpace(packetKind) == "execution_request" {
		runRec := execution.RunRecord
		if runRec == nil {
			return "", "", fmt.Errorf("Texture execution_request requires scoped run context")
		}
		expected := persistentSuperAgentID(runRec.OwnerID)
		if explicitAgentID = strings.TrimSpace(explicitAgentID); explicitAgentID != "" && explicitAgentID != expected {
			return "", "", fmt.Errorf("Texture execution_request agent_id must name persistent Super %q", expected)
		}
		target, err := rt.ensurePersistentSuperAgent(ctx, runRec.OwnerID)
		if err != nil {
			return "", "", err
		}
		channelID := strings.TrimSpace(metadataStringValue(runRec.Metadata, runMetadataChannelID))
		if channelID == "" {
			channelID = strings.TrimSpace(runRec.ChannelID)
		}
		if channelID == "" {
			channelID = strings.TrimSpace(target.ChannelID)
		}
		return target.AgentID, channelID, nil
	}
	return resolveCoagentFindingsTarget(ctx, rt, explicitAgentID)
}

func resolveResearcherFindingsTarget(ctx context.Context, rt *Runtime, explicitAgentID string) (string, string, error) {
	explicitAgentID = strings.TrimSpace(explicitAgentID)
	if explicitAgentID == "" {
		return "", "", fmt.Errorf("researcher update_coagent requires agent_id naming the addressed Texture coagent (texture:<doc_id>)")
	}
	if !isTextureAgentID(explicitAgentID) {
		return "", "", fmt.Errorf("researcher update_coagent agent_id must name a Texture coagent (texture:<doc_id>), got %q", explicitAgentID)
	}
	docID := docIDFromTextureAgentID(explicitAgentID)
	if docID == "" {
		return "", "", fmt.Errorf("researcher update_coagent agent_id %q is not a valid Texture coagent id", explicitAgentID)
	}
	channelID := docID
	if rt != nil && rt.store != nil {
		if runRec := toolregistry.ExecutionContextFrom(ctx).RunRecord; runRec != nil && strings.TrimSpace(runRec.OwnerID) != "" && strings.TrimSpace(runRec.SandboxID) != "" {
			target, err := rt.store.GetAgentByScope(ctx, runRec.OwnerID, runRec.SandboxID, explicitAgentID)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return "", "", fmt.Errorf("resolve texture delivery target: %w", err)
			}
			if err == nil {
				if ch := strings.TrimSpace(target.ChannelID); ch != "" {
					channelID = ch
				}
			}
		}
	}
	return explicitAgentID, channelID, nil
}

func resolveCoagentFindingsTarget(ctx context.Context, rt *Runtime, explicitAgentID string) (string, string, error) {
	runRec := toolregistry.ExecutionContextFrom(ctx).RunRecord
	resolveExplicit := func(targetAgentID string) (string, string, error) {
		targetAgentID = strings.TrimSpace(targetAgentID)
		if targetAgentID == "" {
			return "", "", fmt.Errorf("resolve delivery target: agent_id is required")
		}
		if runRec == nil {
			return "", "", fmt.Errorf("resolve delivery target lookup: missing scoped run context")
		}
		if targetAgentID == persistentSuperAgentID(runRec.OwnerID) {
			target, err := rt.ensurePersistentSuperAgent(ctx, runRec.OwnerID)
			if err != nil {
				return "", "", err
			}
			return target.AgentID, strings.TrimSpace(target.ChannelID), nil
		}
		target, err := rt.store.GetAgentByScope(ctx, runRec.OwnerID, runRec.SandboxID, targetAgentID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) && isTextureAgentID(targetAgentID) {
				if fallbackAgentID, fallbackChannelID := textureDeliveryFallbackFromContext(runRec, targetAgentID); fallbackAgentID != "" && fallbackChannelID != "" {
					return fallbackAgentID, fallbackChannelID, nil
				}
			}
			return "", "", fmt.Errorf("resolve delivery target lookup: %w", err)
		}
		return targetAgentID, strings.TrimSpace(target.ChannelID), nil
	}

	if explicitAgentID = strings.TrimSpace(explicitAgentID); explicitAgentID != "" {
		return resolveExplicit(explicitAgentID)
	}
	if runRec != nil {
		if requesterAgentID := strings.TrimSpace(metadataStringValue(runRec.Metadata, "requested_by_agent_id")); requesterAgentID != "" {
			return resolveExplicit(requesterAgentID)
		}
		if channelID := strings.TrimSpace(metadataStringValue(runRec.Metadata, runMetadataChannelID)); channelID != "" {
			return currentTextureAgentID(channelID), channelID, nil
		}
	}
	return "", "", fmt.Errorf("structured delivery requires agent_id, requested_by_agent_id, or a texture channel context")
}

func textureDeliveryFallbackFromContext(runRec *types.RunRecord, explicitAgentID string) (string, string) {
	if runRec == nil {
		return "", ""
	}
	channelID := strings.TrimSpace(metadataStringValue(runRec.Metadata, runMetadataChannelID))
	if channelID == "" {
		channelID = strings.TrimSpace(runRec.ChannelID)
	}
	explicitAgentID = strings.TrimSpace(explicitAgentID)
	if isTextureAgentID(explicitAgentID) {
		explicitDocID := docIDFromTextureAgentID(explicitAgentID)
		if explicitDocID != "" {
			if channelID == "" {
				channelID = explicitDocID
			}
			if explicitDocID == channelID {
				return explicitAgentID, channelID
			}
		}
	}
	if channelID == "" {
		return "", ""
	}
	return currentTextureAgentID(channelID), channelID
}

func authoritativeDeliveryChannelID(targetChannelID, explicitChannelID, contextChannelID string) string {
	targetChannelID = strings.TrimSpace(targetChannelID)
	if targetChannelID != "" {
		return targetChannelID
	}
	explicitChannelID = strings.TrimSpace(explicitChannelID)
	if explicitChannelID != "" {
		return explicitChannelID
	}
	return strings.TrimSpace(contextChannelID)
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func trimNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func (rt *Runtime) emitChannelMessageEvent(ctx context.Context, message types.ChannelMessage, ownerID string) {
	payload, err := json.Marshal(map[string]any{
		"channel_id":    message.ChannelID,
		"cursor":        message.Seq,
		"from":          message.From,
		"from_agent_id": message.FromAgentID,
		"from_loop_id":  message.FromRunID,
		"to_agent_id":   message.ToAgentID,
		"to_loop_id":    message.ToRunID,
		"trajectory_id": message.TrajectoryID,
		"role":          message.Role,
		"content":       message.Content,
	})
	if err != nil {
		log.Printf("runtime: marshal channel event payload: %v", err)
		return
	}
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        message.FromRunID,
		AgentID:      message.FromAgentID,
		ChannelID:    message.ChannelID,
		OwnerID:      ownerID,
		TrajectoryID: message.TrajectoryID,
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventChannelMessage,
		Payload:      payload,
	}
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist channel event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorChannel,
		Cause:  events.CauseChannelMessage,
	})
}
