package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterCoAgentTools(registry *ToolRegistry, rt *Runtime, spec AgentRoleSpec) error {
	tools := []Tool{
		newCastAgentTool(rt),
		newCancelAgentTool(rt),
	}
	if len(spec.AllowedDelegateTargets) > 0 {
		tools = append([]Tool{newSpawnAgentTool(rt, spec)}, tools...)
	}
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func newSpawnAgentTool(rt *Runtime, spec AgentRoleSpec) Tool {
	type args struct {
		Objective      string `json:"objective"`
		Role           string `json:"role"`
		Profile        string `json:"profile,omitempty"`
		ChannelID      string `json:"channel_id,omitempty"`
		Model          string `json:"model,omitempty"`
		InitialContent string `json:"initial_content,omitempty"`
	}
	allowedTargets := canonicalAllowedDelegateTargets(spec.AllowedDelegateTargets)
	roleDescription := "Canonical role/profile name. Allowed target roles for this caller: " + strings.Join(allowedTargets, ", ") + "."
	description := "Spawn an allowed child agent run for the current " + spec.Profile + " profile."
	if spec.Profile == AgentProfileConductor {
		description = "Open a VText document from a top-level conductor route. Conductor does not spawn researcher, super, or co-super workers."
	}
	return Tool{
		Name:        "spawn_agent",
		Description: description,
		Parameters: jsonSchemaObject(map[string]any{
			"objective":  map[string]any{"type": "string"},
			"role":       map[string]any{"type": "string", "enum": allowedTargets, "description": roleDescription},
			"profile":    map[string]any{"type": "string", "enum": allowedTargets, "description": "Optional canonical profile override. Usually omit; if set, it must be one of the allowed target roles for this caller."},
			"channel_id": map[string]any{"type": "string"},
			"model":      map[string]any{"type": "string"},
			"initial_content": map[string]any{
				"type":        "string",
				"description": "For role=vtext from conductor only: the complete first document revision to store as v1.",
			},
		}, []string{"objective", "role"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode spawn_agent args: %w", err)
			}
			parentID := stringFromToolContext(ctx, toolCtxRunID)
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if parentID == "" || ownerID == "" {
				return "", fmt.Errorf("spawn_agent missing run context")
			}
			role := canonicalAgentProfile(strings.TrimSpace(in.Role))
			if role == "" {
				return "", fmt.Errorf("role must not be empty")
			}
			callerProfile := stringFromToolContext(ctx, toolCtxProfile)
			profile := canonicalAgentProfile(strings.TrimSpace(in.Profile))
			if profile == "" {
				profile = role
			}
			if !canDelegateTo(callerProfile, profile) {
				return "", fmt.Errorf("%s cannot delegate to %s", callerProfile, profile)
			}
			constraints := map[string]any{
				runMetadataAgentRole:    role,
				runMetadataAgentProfile: profile,
			}
			if channelID := strings.TrimSpace(in.ChannelID); channelID != "" {
				constraints[runMetadataChannelID] = channelID
			}
			if model := strings.TrimSpace(in.Model); model != "" {
				constraints[runMetadataModel] = model
			}
			if callerProfile == AgentProfileConductor && profile == AgentProfileVText {
				parentRec, _ := ctx.Value(toolCtxRunRecord).(*types.RunRecord)
				if parentRec == nil {
					parentRec = &types.RunRecord{
						RunID:        parentID,
						OwnerID:      ownerID,
						AgentProfile: callerProfile,
					}
				}
				if strings.TrimSpace(in.InitialContent) == "" {
					return "", fmt.Errorf("conductor spawn_agent role=vtext requires initial_content containing the first document revision")
				}
				decision, err := rt.ensureConductorVTextRoute(ctx, parentRec, in.Objective, in.InitialContent)
				if err != nil {
					return "", err
				}
				return toolResultJSON(map[string]any{
					"action":                 decision.Action,
					"app":                    decision.App,
					"title":                  decision.Title,
					"seed_prompt":            decision.SeedPrompt,
					"initial_content":        decision.InitialContent,
					"create_initial_version": decision.CreateInitialVersion != nil && *decision.CreateInitialVersion,
					"agent_id":               "vtext:" + decision.DocID,
					"doc_id":                 decision.DocID,
					"user_revision_id":       decision.UserRevisionID,
					"framing_revision_id":    decision.FramingRevisionID,
					"initial_revision_id":    decision.InitialRevisionID,
					"initial_loop_id":        decision.InitialLoopID,
					"loop_id":                decision.InitialLoopID,
					"channel_id":             decision.DocID,
					"role":                   role,
					"profile":                profile,
					"state":                  "open",
				})
			}
			child, err := rt.StartChildRun(ctx, parentID, in.Objective, ownerID, constraints)
			if err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"agent_id":   child.AgentID,
				"loop_id":    child.RunID,
				"channel_id": child.ChannelID,
				"role":       role,
				"profile":    profile,
				"state":      child.State,
			})
		},
	}
}

func canonicalAllowedDelegateTargets(targets []string) []string {
	out := make([]string, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		target = canonicalAgentProfile(target)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

func newCastAgentTool(rt *Runtime) Tool {
	type args struct {
		AgentID   string `json:"agent_id"`
		ChannelID string `json:"channel_id,omitempty"`
		From      string `json:"from,omitempty"`
		Role      string `json:"role,omitempty"`
		Content   string `json:"content"`
	}
	return Tool{
		Name:        "cast_agent",
		Description: "Send an addressed asynchronous message to an existing agent without blocking.",
		Parameters: jsonSchemaObject(map[string]any{
			"agent_id":   map[string]any{"type": "string"},
			"channel_id": map[string]any{"type": "string"},
			"from":       map[string]any{"type": "string"},
			"role":       map[string]any{"type": "string"},
			"content":    map[string]any{"type": "string"},
		}, []string{"agent_id", "content"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode cast_agent args: %w", err)
			}
			targetAgentID := strings.TrimSpace(in.AgentID)
			if targetAgentID == "" {
				return "", fmt.Errorf("agent_id must not be empty")
			}
			target, err := rt.store.GetAgent(ctx, targetAgentID)
			if err != nil {
				return "", fmt.Errorf("cast_agent target lookup: %w", err)
			}
			channelID := strings.TrimSpace(in.ChannelID)
			if channelID == "" {
				channelID = strings.TrimSpace(target.ChannelID)
			}
			if channelID == "" {
				return "", fmt.Errorf("cast_agent target %s has no channel_id", targetAgentID)
			}
			from := strings.TrimSpace(in.From)
			if from == "" {
				from = stringFromToolContext(ctx, toolCtxRunID)
			}
			role := strings.TrimSpace(in.Role)
			if role == "" {
				role = stringFromToolContext(ctx, toolCtxRole)
			}
			cursor, err := rt.ChannelCast(ctx, channelID, targetAgentID, "", from, role, in.Content)
			if err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"agent_id":   targetAgentID,
				"channel_id": channelID,
				"cursor":     cursor,
				"status":     "cast",
			})
		},
	}
}

func newCancelAgentTool(rt *Runtime) Tool {
	type args struct {
		AgentID string `json:"agent_id"`
	}
	return Tool{
		Name:        "cancel_agent",
		Description: "Cancel the latest active loop for an existing agent by agent id.",
		Parameters: jsonSchemaObject(map[string]any{
			"agent_id": map[string]any{"type": "string"},
		}, []string{"agent_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode cancel_agent args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("cancel_agent missing owner context")
			}
			if err := rt.CancelAgent(ctx, in.AgentID, ownerID); err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"agent_id": in.AgentID,
				"status":   "cancelled",
			})
		},
	}
}
