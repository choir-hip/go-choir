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

func resolveFindingsTarget(ctx context.Context, rt *Runtime, explicitAgentID string) (string, string, error) {
	profile := toolregistry.ExecutionContextFrom(ctx).Profile
	if profile == "" {
		if runRec := toolregistry.ExecutionContextFrom(ctx).RunRecord; runRec != nil {
			profile = agentProfileForRun(runRec)
		}
	}
	if profile == agentprofile.Researcher {
		return resolveResearcherFindingsTarget(ctx, rt, explicitAgentID)
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

	if runRec != nil && agentprofile.IsTexture(metadataStringValue(runRec.Metadata, "requested_by_profile")) {
		requesterAgentID := metadataStringValue(runRec.Metadata, "requested_by_agent_id")
		if requesterAgentID != "" {
			target, err := rt.store.GetAgentByScope(ctx, runRec.OwnerID, runRec.SandboxID, requesterAgentID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) && isTextureAgentID(requesterAgentID) {
					channelID := metadataStringValue(runRec.Metadata, runMetadataChannelID)
					if channelID == "" {
						channelID = docIDFromTextureAgentID(requesterAgentID)
					}
					if channelID != "" {
						return requesterAgentID, channelID, nil
					}
				}
				return "", "", fmt.Errorf("resolve delivery target requester lookup: %w", err)
			}
			return requesterAgentID, strings.TrimSpace(target.ChannelID), nil
		}
	}

	if runRec != nil {
		if channelID := strings.TrimSpace(metadataStringValue(runRec.Metadata, runMetadataChannelID)); channelID != "" {
			return currentTextureAgentID(channelID), channelID, nil
		}
	}

	explicitAgentID = strings.TrimSpace(explicitAgentID)
	if explicitAgentID != "" {
		if runRec == nil {
			return "", "", fmt.Errorf("resolve delivery target lookup: missing scoped run context")
		}
		target, err := rt.store.GetAgentByScope(ctx, runRec.OwnerID, runRec.SandboxID, explicitAgentID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				if fallbackAgentID, fallbackChannelID := textureDeliveryFallbackFromContext(runRec, explicitAgentID); fallbackAgentID != "" && fallbackChannelID != "" {
					return fallbackAgentID, fallbackChannelID, nil
				}
			}
			return "", "", fmt.Errorf("resolve delivery target lookup: %w", err)
		}
		return explicitAgentID, strings.TrimSpace(target.ChannelID), nil
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
