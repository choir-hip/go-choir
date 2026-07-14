package coagentowner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// RegisterSpawnTool composes generic coagent lifecycle with the concrete owner
// selected by the requested profile. No app owner is imported by agentcore.
func RegisterSpawnTool(registry *toolregistry.ToolRegistry, core *agentcore.Runtime, texture *textureowner.Handler, policy agentprofile.Policy) error {
	if registry == nil || len(policy.AllowedDelegateTargets) == 0 {
		return nil
	}
	return registry.Register(newSpawnAgentTool(core, texture, policy))
}

func newSpawnAgentTool(core *agentcore.Runtime, texture *textureowner.Handler, policy agentprofile.Policy) toolregistry.Tool {
	type args struct {
		Objective            string   `json:"objective"`
		Role                 string   `json:"role"`
		Profile              string   `json:"profile,omitempty"`
		ChannelID            string   `json:"channel_id,omitempty"`
		Slot                 string   `json:"slot,omitempty"`
		Model                string   `json:"model,omitempty"`
		ModelPolicyOverlayID string   `json:"model_policy_overlay_id,omitempty"`
		Title                string   `json:"title,omitempty"`
		InitialContent       string   `json:"initial_content,omitempty"`
		SourceItemIDs        []string `json:"source_item_ids,omitempty"`
	}
	allowed := canonicalTargets(policy.AllowedDelegateTargets)
	roleDescription := "Canonical role/profile name. Allowed target roles for this caller: " + strings.Join(allowed, ", ") + "."
	description := "Spawn an allowed child agent run for the current " + policy.Profile + " profile."
	if policy.Profile == agentprofile.Conductor {
		description = "Open a Texture document from a top-level conductor route. Conductor does not spawn researcher, super, or co-super workers."
	}
	return toolregistry.Tool{
		Name:        "spawn_agent",
		Description: description,
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"objective":               map[string]any{"type": "string"},
			"role":                    map[string]any{"type": "string", "enum": allowed, "description": roleDescription},
			"profile":                 map[string]any{"type": "string", "enum": allowed, "description": "Optional canonical profile override. Usually omit; if set, it must be one of the allowed target roles for this caller."},
			"channel_id":              map[string]any{"type": "string"},
			"slot":                    map[string]any{"type": "string", "enum": []string{"implementation", "verifier"}, "description": "For vsuper spawning co-super children: use implementation for the candidate writer first; use verifier only after implementation commit/package/blocker evidence exists. Reusing a live slot returns the existing child instead of launching a duplicate."},
			"model":                   map[string]any{"type": "string"},
			"model_policy_overlay_id": map[string]any{"type": "string", "description": "Optional owner-visible model policy overlay id from System/model-policy-overlays/<id>.toml. Use this for eval/model arms instead of passing provider metadata directly."},
			"title":                   map[string]any{"type": "string", "description": "For role=texture from processor or reconciler: optional Texture document title for a new article."},
			"initial_content":         map[string]any{"type": "string", "description": "For role=texture from processor or reconciler: optional source/brief seed revision for the Texture before the Texture agent writes the article."},
			"source_item_ids":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "For role=texture from processor: the exact source item ids this story handoff covers. Required when the processor request contains multiple source items."},
		}, []string{"objective", "role"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode spawn_agent args: %w", err)
			}
			exec := toolregistry.ExecutionContextFrom(ctx)
			if exec.RunID == "" || exec.OwnerID == "" {
				return "", fmt.Errorf("spawn_agent missing run context")
			}
			role := normalizeTarget(in.Role, allowed)
			if role == "" {
				return "", fmt.Errorf("role must be one of %s", strings.Join(allowed, ", "))
			}
			profile := normalizeTarget(in.Profile, allowed)
			if strings.TrimSpace(in.Profile) != "" && profile == "" {
				return "", fmt.Errorf("profile must be one of %s", strings.Join(allowed, ", "))
			}
			if profile == "" {
				profile = role
			}
			callerProfile := agentprofile.Canonical(exec.Profile)
			if !agentprofile.CanDelegate(callerProfile, profile) {
				return "", fmt.Errorf("%s cannot delegate to %s", callerProfile, profile)
			}
			slot := normalizeSlot(in.Slot)
			if strings.TrimSpace(in.Slot) != "" && slot == "" {
				return "", fmt.Errorf("spawn_agent slot must be implementation or verifier")
			}
			if callerProfile == agentprofile.VSuper && profile == agentprofile.CoSuper && slot == "" {
				return "", fmt.Errorf("vsuper spawn_agent role=co-super requires slot=\"implementation\" or slot=\"verifier\"")
			}
			if profile == agentprofile.Texture {
				parent := exec.RunRecord
				if parent == nil {
					parent = &types.RunRecord{RunID: exec.RunID, OwnerID: exec.OwnerID, AgentID: exec.AgentID, AgentProfile: callerProfile, AgentRole: exec.Role, ChannelID: exec.ChannelID}
				}
				decision, err := texture.EnsureTextureHandoff(ctx, parent, textureowner.HandoffRequest{
					Kind: textureowner.HandoffKindForCaller(callerProfile), CallerProfile: callerProfile,
					Objective: in.Objective, Title: in.Title, ChannelID: in.ChannelID,
					InitialContent: in.InitialContent, SourceItemIDs: trimNonEmpty(in.SourceItemIDs),
				})
				if err != nil {
					return "", err
				}
				if decision.Kind == textureowner.HandoffKindUserPrompt {
					return toolregistry.ResultJSON(map[string]any{
						"action": decision.Conductor.Action, "app": decision.Conductor.App,
						"title": decision.Conductor.Title, "seed_prompt": decision.Conductor.SeedPrompt,
						"initial_content":        decision.Conductor.InitialContent,
						"create_initial_version": decision.Conductor.CreateInitialVersion != nil && *decision.Conductor.CreateInitialVersion,
						"agent_id":               textureAgentID(decision.DocID), "doc_id": decision.DocID,
						"user_revision_id": decision.UserRevisionID, "initial_revision_id": decision.Conductor.InitialRevisionID,
						"initial_loop_id": decision.InitialLoopID, "loop_id": decision.RevisionRunID,
						"channel_id": decision.DocID, "role": agentprofile.Texture, "profile": agentprofile.Texture,
						"state": "open", "handoff_kind": string(decision.Kind),
					})
				}
				return toolregistry.ResultJSON(map[string]any{
					"agent_id": textureAgentID(decision.DocID), "doc_id": decision.DocID,
					"seed_revision_id": decision.SeedRevisionID, "loop_id": decision.RevisionRunID,
					"revision_loop_id": decision.RevisionRunID, "channel_id": decision.DocID,
					"role": agentprofile.Texture, "profile": agentprofile.Texture, "state": decision.State,
					"title": decision.Title, "created_document": decision.CreatedDocument,
					"revised_existing_doc": !decision.CreatedDocument, "handoff_kind": string(decision.Kind),
				})
			}
			constraints := map[string]any{"agent_role": role, "agent_profile": profile}
			if slot != "" {
				constraints["co_super_slot"] = slot
			}
			if value := strings.TrimSpace(in.ChannelID); value != "" {
				constraints["channel_id"] = value
			}
			if value := strings.TrimSpace(in.Model); value != "" {
				constraints["model"] = value
			}
			if value := strings.TrimSpace(in.ModelPolicyOverlayID); value != "" {
				constraints[modelpolicy.MetadataPolicyOverlayID] = value
			}
			child, err := core.StartCoagentRun(ctx, exec.RunID, in.Objective, exec.OwnerID, constraints)
			if err != nil {
				return "", err
			}
			result := map[string]any{"agent_id": child.AgentID, "loop_id": child.RunID, "channel_id": child.ChannelID, "role": role, "profile": profile, "state": child.State}
			if value := metadataString(child.Metadata, "co_super_slot"); value != "" {
				result["slot"] = value
			}
			if metadataBool(child.Metadata, "spawn_reused") {
				result["reused_existing_child"] = true
			}
			return toolregistry.ResultJSON(result)
		},
	}
}

func canonicalTargets(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = agentprofile.Canonical(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func normalizeTarget(raw string, allowed []string) string {
	direct := agentprofile.Canonical(raw)
	for _, value := range allowed {
		if direct == value {
			return direct
		}
	}
	var match string
	for _, token := range strings.FieldsFunc(raw, func(r rune) bool { return !unicode.IsLetter(r) && r != '-' && r != '_' }) {
		candidate := agentprofile.Canonical(token)
		for _, value := range allowed {
			if candidate == value {
				if match != "" && match != value {
					return direct
				}
				match = value
			}
		}
	}
	return match
}

func normalizeSlot(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "implementation", "verifier":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}
func textureAgentID(docID string) string { return "texture:" + strings.TrimSpace(docID) }
func trimNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}
func metadataBool(metadata map[string]any, key string) bool {
	value, _ := metadata[key].(bool)
	return value
}
