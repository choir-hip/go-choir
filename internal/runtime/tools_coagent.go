package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterCoAgentTools(registry *ToolRegistry, rt *Runtime, spec AgentRoleSpec) error {
	tools := []Tool{
		newCastAgentTool(rt),
		newCastAgentUpdateTool(rt),
		newWaitAgentTool(rt),
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
		Slot           string `json:"slot,omitempty"`
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
			"slot":       map[string]any{"type": "string", "enum": []string{"implementation", "verifier"}, "description": "For vsuper spawning co-super children: use implementation for the candidate writer first; use verifier only after implementation commit/package/blocker evidence exists. Reusing a live slot returns the existing child instead of launching a duplicate."},
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
			slot := normalizeVSuperCoSuperSlot(in.Slot)
			if strings.TrimSpace(in.Slot) != "" && slot == "" {
				return "", fmt.Errorf("spawn_agent slot must be implementation or verifier")
			}
			if callerProfile == AgentProfileVSuper && profile == AgentProfileCoSuper && slot == "" {
				return "", fmt.Errorf("vsuper spawn_agent role=co-super requires slot=\"implementation\" or slot=\"verifier\" to avoid duplicate child runs")
			}
			constraints := map[string]any{
				runMetadataAgentRole:    role,
				runMetadataAgentProfile: profile,
			}
			if slot != "" {
				constraints[runMetadataCoSuperSlot] = slot
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
			result := map[string]any{
				"agent_id":   child.AgentID,
				"loop_id":    child.RunID,
				"channel_id": child.ChannelID,
				"role":       role,
				"profile":    profile,
				"state":      child.State,
			}
			if slot := metadataStringValue(child.Metadata, runMetadataCoSuperSlot); slot != "" {
				result["slot"] = slot
			}
			if metadataBoolValue(child.Metadata, runMetadataSpawnReused) {
				result["reused_existing_child"] = true
			}
			if callerProfile == AgentProfileVText {
				result["next_required_tool"] = "edit_vtext"
				result["next_instruction"] = "Write a brief interim VText revision now. Name the active worker, the evidence being gathered, and what the next revision should contain. Do not include ungrounded factual claims."
			}
			return toolResultJSON(result)
		},
	}
}

func normalizeVSuperCoSuperSlot(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "implementation", "implementer", "worker", "writer", "builder":
		return "implementation"
	case "verifier", "verification", "reviewer", "review", "checker", "tester":
		return "verifier"
	default:
		return ""
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
			if err := enforceSkipLevelCastRule(ctx, rt, targetAgentID, nil); err != nil {
				return "", err
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

func newCastAgentUpdateTool(rt *Runtime) Tool {
	type recipient struct {
		AgentID string `json:"agent_id"`
		RunID   string `json:"loop_id,omitempty"`
	}
	type args struct {
		MessageClass string      `json:"message_class,omitempty"`
		ChannelID    string      `json:"channel_id,omitempty"`
		From         string      `json:"from,omitempty"`
		Role         string      `json:"role,omitempty"`
		Content      string      `json:"content"`
		Recipients   []recipient `json:"recipients"`
	}
	return Tool{
		Name:        "cast_agent_update",
		Description: "Send one typed update to multiple agents with copy-aware delivery. Super-to-co-super directives must include the supervising vsuper in the same recipients list.",
		Parameters: jsonSchemaObject(map[string]any{
			"message_class": map[string]any{"type": "string", "enum": []string{"phase_checkpoint", "evidence_ready", "blocker", "clarification_request", "directive", "cancel", "narrative_revision"}},
			"channel_id":    map[string]any{"type": "string"},
			"from":          map[string]any{"type": "string"},
			"role":          map[string]any{"type": "string"},
			"content":       map[string]any{"type": "string"},
			"recipients": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"agent_id": map[string]any{"type": "string"},
						"loop_id":  map[string]any{"type": "string"},
					},
					"required": []string{"agent_id"},
				},
			},
		}, []string{"content", "recipients"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode cast_agent_update args: %w", err)
			}
			if len(in.Recipients) == 0 {
				return "", fmt.Errorf("recipients must not be empty")
			}
			copyGroupID := uuid.NewString()
			recipientIDs := make([]string, 0, len(in.Recipients))
			for _, rec := range in.Recipients {
				agentID := strings.TrimSpace(rec.AgentID)
				if agentID == "" {
					return "", fmt.Errorf("recipient agent_id must not be empty")
				}
				recipientIDs = append(recipientIDs, agentID)
			}
			for _, agentID := range recipientIDs {
				if err := enforceSkipLevelCastRule(ctx, rt, agentID, recipientIDs); err != nil {
					return "", err
				}
			}
			from := strings.TrimSpace(in.From)
			if from == "" {
				from = stringFromToolContext(ctx, toolCtxRunID)
			}
			role := strings.TrimSpace(in.Role)
			if role == "" {
				role = stringFromToolContext(ctx, toolCtxRole)
			}
			messageClass := strings.TrimSpace(in.MessageClass)
			if messageClass == "" {
				messageClass = "phase_checkpoint"
			}
			content := fmt.Sprintf("[message_class=%s copy_group_id=%s]\n%s", messageClass, copyGroupID, strings.TrimSpace(in.Content))
			cursors := make(map[string]uint64, len(in.Recipients))
			for _, rec := range in.Recipients {
				targetAgentID := strings.TrimSpace(rec.AgentID)
				target, err := rt.store.GetAgent(ctx, targetAgentID)
				if err != nil {
					return "", fmt.Errorf("cast_agent_update target lookup %s: %w", targetAgentID, err)
				}
				channelID := strings.TrimSpace(in.ChannelID)
				if channelID == "" {
					channelID = strings.TrimSpace(target.ChannelID)
				}
				if channelID == "" {
					return "", fmt.Errorf("cast_agent_update target %s has no channel_id", targetAgentID)
				}
				cursor, err := rt.ChannelCast(ctx, channelID, targetAgentID, strings.TrimSpace(rec.RunID), from, role, content)
				if err != nil {
					return "", err
				}
				cursors[targetAgentID] = cursor
			}
			return toolResultJSON(map[string]any{
				"status":        "cast",
				"copy_group_id": copyGroupID,
				"message_class": messageClass,
				"recipients":    recipientIDs,
				"cursors":       cursors,
			})
		},
	}
}

func enforceSkipLevelCastRule(ctx context.Context, rt *Runtime, targetAgentID string, copiedAgentIDs []string) error {
	if rt == nil || rt.store == nil {
		return nil
	}
	if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileSuper {
		return nil
	}
	ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
	if ownerID == "" {
		return nil
	}
	target, err := rt.store.GetAgent(ctx, targetAgentID)
	if err != nil {
		return fmt.Errorf("cast target lookup: %w", err)
	}
	if canonicalAgentProfile(target.Profile) != AgentProfileCoSuper {
		return nil
	}
	run, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, targetAgentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("lookup co-super active run: %w", err)
	}
	parentRunID := strings.TrimSpace(run.ParentRunID)
	if parentRunID == "" {
		return nil
	}
	parent, err := rt.store.GetRun(ctx, parentRunID)
	if err != nil {
		return fmt.Errorf("lookup co-super supervisor run: %w", err)
	}
	if agentProfileForRun(&parent) != AgentProfileVSuper {
		return nil
	}
	supervisorAgentID := agentIDForRun(&parent)
	for _, copied := range copiedAgentIDs {
		if strings.TrimSpace(copied) == supervisorAgentID {
			return nil
		}
	}
	return fmt.Errorf("private skip-level directive rejected: super -> co-super messages must copy supervising vsuper %s in the same cast_agent_update recipients", supervisorAgentID)
}

func newWaitAgentTool(rt *Runtime) Tool {
	type args struct {
		AgentID     string   `json:"agent_id"`
		ChannelID   string   `json:"channel_id,omitempty"`
		Cursor      uint64   `json:"cursor,omitempty"`
		Roles       []string `json:"roles,omitempty"`
		TimeoutMS   int      `json:"timeout_ms,omitempty"`
		MaxMessages int      `json:"max_messages,omitempty"`
	}
	return Tool{
		Name:        "wait_agent",
		Description: "Block briefly for channel messages from an existing agent. Use after spawn_agent or cast_agent when coordination depends on the child result; pass the cast_agent cursor when waiting for a reply to that cast.",
		Parameters: jsonSchemaObject(map[string]any{
			"agent_id":     map[string]any{"type": "string"},
			"channel_id":   map[string]any{"type": "string"},
			"cursor":       map[string]any{"type": "integer", "minimum": 0, "description": "Channel cursor returned by cast_agent or a previous wait_agent call. Omit or use 0 to inspect existing messages."},
			"roles":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional message roles to return, for example result, error, status, or verifier."},
			"timeout_ms":   map[string]any{"type": "integer", "minimum": 1, "description": "Bounded wait duration. Defaults to 30000ms and is capped at 120000ms."},
			"max_messages": map[string]any{"type": "integer", "minimum": 1, "description": "Maximum matching messages to return. Defaults to 10 and is capped at 25."},
		}, []string{"agent_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode wait_agent args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("wait_agent missing owner context")
			}
			targetAgentID := strings.TrimSpace(in.AgentID)
			if targetAgentID == "" {
				return "", fmt.Errorf("agent_id must not be empty")
			}
			target, err := rt.store.GetAgent(ctx, targetAgentID)
			if err != nil {
				return "", fmt.Errorf("wait_agent target lookup: %w", err)
			}
			if strings.TrimSpace(target.OwnerID) != "" && strings.TrimSpace(target.OwnerID) != ownerID {
				return "", fmt.Errorf("wait_agent target %s is not owned by caller", targetAgentID)
			}
			channelID := strings.TrimSpace(in.ChannelID)
			if channelID == "" {
				channelID = strings.TrimSpace(target.ChannelID)
			}
			if channelID == "" {
				channelID = stringFromToolContext(ctx, toolCtxChannelID)
			}
			if channelID == "" {
				return "", fmt.Errorf("wait_agent target %s has no channel_id", targetAgentID)
			}
			timeout := waitAgentTimeout(in.TimeoutMS)
			maxMessages := waitAgentMaxMessages(in.MaxMessages)
			roles := waitAgentRoleSet(in.Roles)

			targetRuns, latestRun := waitAgentTargetRuns(ctx, rt, ownerID, channelID, targetAgentID)
			cursor := in.Cursor
			if matched, nextCursor, err := waitAgentReadMatching(rt, channelID, cursor, targetAgentID, targetRuns, roles, maxMessages); err != nil {
				return "", err
			} else if len(matched) > 0 {
				return waitAgentResultJSON("messages", targetAgentID, channelID, nextCursor, matched, latestRun, targetRuns, maxMessages)
			} else {
				cursor = nextCursor
			}
			if latestRun != nil && latestRun.State.Terminal() {
				return waitAgentResultJSON("target_terminal_without_new_message", targetAgentID, channelID, cursor, nil, latestRun, targetRuns, maxMessages)
			}

			waitCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			for {
				msgs, nextCursor, err := rt.ChannelWait(waitCtx, channelID, cursor)
				if err != nil {
					targetRuns, latestRun = waitAgentTargetRuns(ctx, rt, ownerID, channelID, targetAgentID)
					if waitCtx.Err() != nil {
						status := "timeout"
						if latestRun != nil && latestRun.State.Terminal() {
							status = "target_terminal_without_matching_message"
						}
						return waitAgentResultJSON(status, targetAgentID, channelID, cursor, nil, latestRun, targetRuns, maxMessages)
					}
					return "", err
				}
				cursor = nextCursor
				targetRuns, latestRun = waitAgentTargetRuns(ctx, rt, ownerID, channelID, targetAgentID)
				matched := waitAgentFilterMessages(msgs, targetAgentID, targetRuns, roles, maxMessages)
				if len(matched) > 0 {
					return waitAgentResultJSON("messages", targetAgentID, channelID, cursor, matched, latestRun, targetRuns, maxMessages)
				}
				if latestRun != nil && latestRun.State.Terminal() {
					return waitAgentResultJSON("target_terminal_without_matching_message", targetAgentID, channelID, cursor, nil, latestRun, targetRuns, maxMessages)
				}
			}
		},
	}
}

func waitAgentTimeout(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		return 30 * time.Second
	}
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout > 120*time.Second {
		return 120 * time.Second
	}
	return timeout
}

func waitAgentMaxMessages(maxMessages int) int {
	switch {
	case maxMessages <= 0:
		return 10
	case maxMessages > 25:
		return 25
	default:
		return maxMessages
	}
}

func waitAgentRoleSet(roles []string) map[string]bool {
	if len(roles) == 0 {
		return nil
	}
	out := make(map[string]bool, len(roles))
	for _, role := range roles {
		if role = strings.TrimSpace(role); role != "" {
			out[role] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func waitAgentTargetRuns(ctx context.Context, rt *Runtime, ownerID, channelID, targetAgentID string) ([]types.RunRecord, *types.RunRecord) {
	runs, err := rt.store.ListRunsByChannel(ctx, ownerID, channelID, 200)
	if err != nil {
		return nil, nil
	}
	matches := make([]types.RunRecord, 0, 4)
	for _, run := range runs {
		if strings.TrimSpace(run.AgentID) != targetAgentID {
			continue
		}
		matches = append(matches, run)
	}
	if len(matches) == 0 {
		return matches, nil
	}
	latest := matches[0]
	return matches, &latest
}

func waitAgentReadMatching(rt *Runtime, channelID string, cursor uint64, targetAgentID string, targetRuns []types.RunRecord, roles map[string]bool, maxMessages int) ([]ChannelMessage, uint64, error) {
	msgs, nextCursor, err := rt.ChannelRead(channelID, cursor)
	if err != nil {
		return nil, cursor, err
	}
	return waitAgentFilterMessages(msgs, targetAgentID, targetRuns, roles, maxMessages), nextCursor, nil
}

func waitAgentFilterMessages(msgs []ChannelMessage, targetAgentID string, targetRuns []types.RunRecord, roles map[string]bool, maxMessages int) []ChannelMessage {
	runIDs := make(map[string]bool, len(targetRuns))
	for _, run := range targetRuns {
		if runID := strings.TrimSpace(run.RunID); runID != "" {
			runIDs[runID] = true
		}
	}
	out := make([]ChannelMessage, 0, len(msgs))
	for _, msg := range msgs {
		if len(roles) > 0 && !roles[strings.TrimSpace(msg.Role)] {
			continue
		}
		fromAgentID := strings.TrimSpace(msg.FromAgentID)
		fromRunID := strings.TrimSpace(msg.FromRunID)
		from := strings.TrimSpace(msg.From)
		if fromAgentID != targetAgentID && from != targetAgentID && !runIDs[fromRunID] && !runIDs[from] {
			continue
		}
		out = append(out, msg)
		if len(out) >= maxMessages {
			break
		}
	}
	return out
}

func waitAgentResultJSON(status, targetAgentID, channelID string, cursor uint64, messages []ChannelMessage, latestRun *types.RunRecord, targetRuns []types.RunRecord, maxMessages int) (string, error) {
	result := map[string]any{
		"status":     status,
		"agent_id":   targetAgentID,
		"channel_id": channelID,
		"cursor":     cursor,
		"messages":   waitAgentMessageSummaries(messages),
	}
	if latestRun != nil {
		result["latest_target_run"] = waitAgentRunSummary(*latestRun)
	}
	if len(targetRuns) > 0 {
		limit := maxMessages
		if limit > len(targetRuns) {
			limit = len(targetRuns)
		}
		summaries := make([]map[string]any, 0, limit)
		for _, run := range targetRuns[:limit] {
			summaries = append(summaries, waitAgentRunSummary(run))
		}
		result["target_runs"] = summaries
	}
	return toolResultJSON(result)
}

func waitAgentMessageSummaries(messages []ChannelMessage) []map[string]any {
	out := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		out = append(out, map[string]any{
			"seq":           msg.Seq,
			"from":          msg.From,
			"from_agent_id": msg.FromAgentID,
			"from_loop_id":  msg.FromRunID,
			"to_agent_id":   msg.ToAgentID,
			"role":          msg.Role,
			"content":       truncateWaitAgentText(msg.Content, 8000),
			"timestamp":     msg.Timestamp.UTC().Format(time.RFC3339Nano),
		})
	}
	return out
}

func waitAgentRunSummary(run types.RunRecord) map[string]any {
	summary := map[string]any{
		"loop_id":    run.RunID,
		"agent_id":   run.AgentID,
		"profile":    run.AgentProfile,
		"role":       run.AgentRole,
		"state":      run.State,
		"channel_id": run.ChannelID,
	}
	if run.Result != "" {
		summary["result"] = truncateWaitAgentText(run.Result, 4000)
	}
	if run.Error != "" {
		summary["error"] = truncateWaitAgentText(run.Error, 2000)
	}
	return summary
}

func truncateWaitAgentText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + fmt.Sprintf("\n\n[truncated %d bytes, showing first %d bytes]", len(value), limit)
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
			target, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, strings.TrimSpace(in.AgentID))
			if err != nil {
				if err == store.ErrNotFound {
					return "", fmt.Errorf("agent not found: %s", in.AgentID)
				}
				return "", fmt.Errorf("lookup active agent run: %w", err)
			}
			if stringFromToolContext(ctx, toolCtxProfile) == AgentProfileVSuper &&
				strings.TrimSpace(target.ParentRunID) == stringFromToolContext(ctx, toolCtxRunID) {
				eventsForRun, err := rt.store.ListEvents(ctx, target.RunID, 1000)
				if err != nil {
					return "", fmt.Errorf("check child export evidence before cancel: %w", err)
				}
				if hasSuccessfulToolResult(eventsForRun, "publish_app_change_package") {
					return toolResultJSON(map[string]any{
						"agent_id": in.AgentID,
						"loop_id":  target.RunID,
						"status":   "not_cancelled",
						"reason":   "child already produced publish_app_change_package evidence; incorporate the child package instead of cancelling it",
					})
				}
			}
			if err := rt.CancelRun(ctx, target.RunID, ownerID); err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"agent_id": in.AgentID,
				"loop_id":  target.RunID,
				"status":   "cancelled",
			})
		},
	}
}
