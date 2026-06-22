package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterCoagentUpdateTools(registry *ToolRegistry, rt *Runtime) error {
	return registry.Register(newUpdateCoagentTool(rt))
}

type submitCoagentUpdateArgs struct {
	AgentID   string `json:"agent_id,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	types.CoagentSourcePacketPayload
}

func newUpdateCoagentTool(rt *Runtime) Tool {
	return Tool{
		Name:        "update_coagent",
		Description: "Append one addressed coagent source packet and wake the target actor. The canonical packet shape is schema_version, kind, summary, claims, sources, actions, questions, and notes. Texture may cite/embed only packet.sources; Super may execute only kind=execution_request packets with actions. Runtime derives update_id; do not send update_id or legacy findings/evidence_ids/evidence/artifacts/refs/tests/proposals/capability_requests fields.",
		Parameters: jsonSchemaObject(map[string]any{
			"schema_version": map[string]any{"type": "string", "enum": []string{types.CoagentSourcePacketSchemaV1}},
			"kind":           map[string]any{"type": "string", "enum": []string{"evidence_update", "execution_request", "execution_result", "blocker", "question", "proposal", "decision_request"}},
			"summary":        map[string]any{"type": "string"},
			"agent_id":       map[string]any{"type": "string", "description": "Required for researcher deliveries: the addressed Texture coagent id (texture:<doc_id>). Other roles should set the addressed owning coagent id when not implicit."},
			"channel_id":     map[string]any{"type": "string"},
			"claims": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"claim_id":            map[string]any{"type": "string"},
						"text":                map[string]any{"type": "string"},
						"source_ids":          stringArraySchema(),
						"stance":              map[string]any{"type": "string", "enum": []string{"supports", "qualifies", "contradicts", "background"}},
						"recommended_surface": map[string]any{"type": "string", "enum": []string{"inline_ref", "block_embed", "source_panel", "decision_log"}},
					},
					"required":             []string{"text"},
					"additionalProperties": false,
				},
			},
			"sources": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"source_id": map[string]any{"type": "string"},
						"kind":      map[string]any{"type": "string"},
						"target": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"uri":        map[string]any{"type": "string"},
								"title":      map[string]any{"type": "string"},
								"media_type": map[string]any{"type": "string"},
							},
							"additionalProperties": false,
						},
						"selectors": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"kind":   map[string]any{"type": "string"},
									"quote":  map[string]any{"type": "string"},
									"start":  map[string]any{"type": "integer"},
									"end":    map[string]any{"type": "integer"},
									"x":      map[string]any{"type": "number"},
									"y":      map[string]any{"type": "number"},
									"width":  map[string]any{"type": "number"},
									"height": map[string]any{"type": "number"},
								},
								"required":             []string{"kind"},
								"additionalProperties": false,
							},
						},
						"evidence": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"state":        map[string]any{"type": "string", "enum": []string{"available", "pending", "blocked", "unavailable"}},
								"confidence":   map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
								"rights_scope": map[string]any{"type": "string"},
							},
							"additionalProperties": false,
						},
					},
					"required":             []string{"kind", "target"},
					"additionalProperties": false,
				},
			},
			"actions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action_id": map[string]any{"type": "string"},
						"type":      map[string]any{"type": "string", "enum": []string{"run_command", "inspect_file", "produce_diff", "run_tests", "open_browser", "request_worker", "import_source", "revise_texture"}},
						"objective": map[string]any{"type": "string"},
						"inputs":    map[string]any{"type": "object"},
						"expected_sources": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"kind":     map[string]any{"type": "string"},
									"required": map[string]any{"type": "boolean"},
								},
								"required":             []string{"kind"},
								"additionalProperties": false,
							},
						},
						"safety": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"mutation_class": map[string]any{"type": "string", "enum": []string{"green", "yellow", "orange", "red", "black"}},
								"network":        map[string]any{"type": "string", "enum": []string{"forbidden", "allowed", "required"}},
								"file_mutation":  map[string]any{"type": "string", "enum": []string{"forbidden", "allowed", "required"}},
							},
							"additionalProperties": false,
						},
					},
					"required":             []string{"type", "objective"},
					"additionalProperties": false,
				},
			},
			"questions": stringArraySchema(),
			"notes":     stringArraySchema(),
		}, []string{"schema_version", "kind", "summary"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if err := rejectLegacyUpdateCoagentFields(raw); err != nil {
				return "", err
			}
			var in submitCoagentUpdateArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode update_coagent args: %w", err)
			}
			packet := normalizeCoagentSourcePacketPayload(in.CoagentSourcePacketPayload)
			if err := validateCoagentSourcePacketPayload(packet); err != nil {
				return "", err
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			agentID := stringFromToolContext(ctx, toolCtxAgentID)
			runID := stringFromToolContext(ctx, toolCtxRunID)
			role := stringFromToolContext(ctx, toolCtxRole)
			if ownerID == "" || agentID == "" || runID == "" {
				return "", fmt.Errorf("update_coagent missing coagent context")
			}

			update := types.CoagentSourcePacket{
				OwnerID:   ownerID,
				AgentID:   agentID,
				Role:      nonEmpty(role, configuredAgentProfileForRun(ctxRunRecord(ctx))),
				Packet:    packet,
				CreatedAt: time.Now().UTC(),
			}
			targetAgentID, targetChannelID, err := resolveFindingsTarget(ctx, rt, strings.TrimSpace(in.AgentID))
			if err != nil {
				return "", err
			}
			if canonicalAgentProfile(stringFromToolContext(ctx, toolCtxProfile)) == AgentProfileResearcher {
				if explicitChannel := strings.TrimSpace(in.ChannelID); explicitChannel != "" && explicitChannel != targetChannelID {
					return "", fmt.Errorf("update_coagent channel_id %q does not match Texture coagent %q channel %q", explicitChannel, targetAgentID, targetChannelID)
				}
			}
			if target, err := rt.store.GetAgent(ctx, targetAgentID); err == nil {
				targetProfile := canonicalAgentProfile(target.Profile)
				if targetProfile == AgentProfileEmail {
					return "", fmt.Errorf("%s cannot send arbitrary coagent updates to Email appagent %s; use a Texture-owned request_email_draft artifact handoff", canonicalAgentProfile(stringFromToolContext(ctx, toolCtxProfile)), target.AgentID)
				}
				if err := enforceCoagentUpdateAuthority(ctx, rt, target, targetProfile); err != nil {
					return "", err
				}
			}
			channelID := authoritativeDeliveryChannelID(targetChannelID, in.ChannelID, stringFromToolContext(ctx, toolCtxChannelID))
			if channelID == "" {
				return "", fmt.Errorf("update_coagent could not resolve channel_id")
			}

			trajectoryID := ""
			if runRec := ctxRunRecord(ctx); runRec != nil && runRec.Metadata != nil {
				if id, _ := runRec.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(id) != "" {
					trajectoryID = strings.TrimSpace(id)
				}
			}

			update.TargetAgentID = targetAgentID
			update.ChannelID = channelID
			update.TrajectoryID = trajectoryID
			update.UpdateID = deriveWorkerUpdateID(update)
			update.Content = buildWorkerUpdateMessage(update)

			message := &types.ChannelMessage{
				ChannelID:    channelID,
				From:         runID,
				FromAgentID:  agentID,
				FromRunID:    runID,
				ToAgentID:    targetAgentID,
				TrajectoryID: trajectoryID,
				Role:         update.Role,
				Content:      update.Content,
				Timestamp:    update.CreatedAt,
			}
			stored, created, err := rt.store.DispatchWorkerUpdate(ctx, update, message)
			if err != nil {
				return "", err
			}
			if !created {
				if err := validateExistingWorkerUpdate(stored, update); err != nil {
					return "", err
				}
			} else {
				rt.emitChannelMessageEvent(ctx, *message, ownerID)
				rt.wakeUpdatedCoagent(ctx, stored)
			}

			return toolResultJSON(map[string]any{
				"update_id":     stored.UpdateID,
				"agent_id":      stored.TargetAgentID,
				"channel_id":    stored.ChannelID,
				"cursor":        stored.MessageSeq,
				"trajectory_id": stored.TrajectoryID,
				"status":        map[bool]string{true: "submitted", false: "existing"}[created],
			})
		},
	}
}

func enforceCoagentUpdateAuthority(ctx context.Context, rt *Runtime, target types.AgentRecord, targetProfile string) error {
	if rt == nil || rt.store == nil {
		return nil
	}
	if canonicalAgentProfile(stringFromToolContext(ctx, toolCtxProfile)) != AgentProfileSuper || targetProfile != AgentProfileCoSuper {
		return nil
	}
	ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
	if ownerID == "" {
		return nil
	}
	slot, found, err := rt.store.CoSuperSlotByAgent(ctx, ownerID, target.AgentID)
	if err != nil {
		return fmt.Errorf("lookup co-super slot: %w", err)
	}
	if !found {
		return nil
	}
	owningVSuper := strings.TrimSpace(slot.TrajectoryID)
	if owningVSuper == "" {
		owningVSuper = strings.TrimSpace(slot.Slot)
	}
	return fmt.Errorf("skip-level directive blocked: super must address co-super %s through owning vsuper trajectory %s with update_coagent", target.AgentID, owningVSuper)
}

func stringArraySchema() map[string]any {
	return map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
}

func ctxRunRecord(ctx context.Context) *types.RunRecord {
	runRec, _ := ctx.Value(toolCtxRunRecord).(*types.RunRecord)
	return runRec
}

func workerUpdateEmpty(update types.CoagentSourcePacket) bool {
	return coagentPacketPayloadEmpty(update.Packet)
}

func buildWorkerUpdateMessage(update types.CoagentSourcePacket) string {
	packet := update.Packet
	var b strings.Builder
	b.WriteString("Coagent source packet ready.")
	if strings.TrimSpace(update.Role) != "" {
		b.WriteString("\nRole: ")
		b.WriteString(strings.TrimSpace(update.Role))
		b.WriteString(".")
	}
	b.WriteString("\nSchema: ")
	b.WriteString(packet.SchemaVersion)
	b.WriteString("\nKind: ")
	b.WriteString(packet.Kind)
	if strings.TrimSpace(packet.Summary) != "" {
		b.WriteString("\nSummary: ")
		b.WriteString(strings.TrimSpace(packet.Summary))
	}
	appendCoagentClaimSection(&b, packet.Claims)
	appendCoagentSourceSection(&b, packet.Sources)
	appendCoagentActionSection(&b, packet.Actions)
	appendWorkerUpdateSection(&b, "Questions", packet.Questions)
	appendWorkerUpdateSection(&b, "Notes", packet.Notes)
	return b.String()
}

func appendWorkerUpdateSection(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString("\n\n")
	b.WriteString(title)
	b.WriteString(":\n")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
}

func appendCoagentClaimSection(b *strings.Builder, claims []types.CoagentPacketClaim) {
	if len(claims) == 0 {
		return
	}
	b.WriteString("\n\nClaims:\n")
	for _, claim := range claims {
		text := strings.TrimSpace(claim.Text)
		if text == "" {
			continue
		}
		b.WriteString("- ")
		if claim.ClaimID != "" {
			b.WriteString(claim.ClaimID)
			b.WriteString(": ")
		}
		b.WriteString(text)
		if len(claim.SourceIDs) > 0 {
			b.WriteString(" [sources: ")
			b.WriteString(strings.Join(claim.SourceIDs, ", "))
			b.WriteString("]")
		}
		if claim.Stance != "" {
			b.WriteString(" stance=")
			b.WriteString(claim.Stance)
		}
		b.WriteString("\n")
	}
}

func appendCoagentSourceSection(b *strings.Builder, sources []types.CoagentPacketSource) {
	if len(sources) == 0 {
		return
	}
	b.WriteString("\n\nSources:\n")
	for _, source := range sources {
		kind := strings.TrimSpace(source.Kind)
		uri := strings.TrimSpace(source.Target.URI)
		title := strings.TrimSpace(source.Target.Title)
		if kind == "" && uri == "" && title == "" {
			continue
		}
		b.WriteString("- ")
		if source.SourceID != "" {
			b.WriteString(source.SourceID)
			b.WriteString(": ")
		}
		b.WriteString(kind)
		if title != "" {
			b.WriteString(" ")
			b.WriteString(strconvQuote(title))
		}
		if uri != "" {
			b.WriteString(" <")
			b.WriteString(uri)
			b.WriteString(">")
		}
		b.WriteString("\n")
	}
}

func appendCoagentActionSection(b *strings.Builder, actions []types.CoagentPacketAction) {
	if len(actions) == 0 {
		return
	}
	b.WriteString("\n\nActions:\n")
	for _, action := range actions {
		if strings.TrimSpace(action.Type) == "" && strings.TrimSpace(action.Objective) == "" {
			continue
		}
		b.WriteString("- ")
		if action.ActionID != "" {
			b.WriteString(action.ActionID)
			b.WriteString(": ")
		}
		b.WriteString(action.Type)
		if action.Objective != "" {
			b.WriteString(" - ")
			b.WriteString(action.Objective)
		}
		b.WriteString("\n")
	}
}

func deriveWorkerUpdateID(update types.CoagentSourcePacket) string {
	payload := struct {
		OwnerID       string                           `json:"owner_id"`
		AgentID       string                           `json:"agent_id"`
		TargetAgentID string                           `json:"target_agent_id"`
		ChannelID     string                           `json:"channel_id"`
		TrajectoryID  string                           `json:"trajectory_id,omitempty"`
		Role          string                           `json:"role,omitempty"`
		Packet        types.CoagentSourcePacketPayload `json:"packet"`
	}{
		OwnerID:       strings.TrimSpace(update.OwnerID),
		AgentID:       strings.TrimSpace(update.AgentID),
		TargetAgentID: strings.TrimSpace(update.TargetAgentID),
		ChannelID:     strings.TrimSpace(update.ChannelID),
		TrajectoryID:  strings.TrimSpace(update.TrajectoryID),
		Role:          strings.TrimSpace(update.Role),
		Packet:        normalizeCoagentSourcePacketPayload(update.Packet),
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return "upd-" + hex.EncodeToString(sum[:])[:32]
}

func validateExistingWorkerUpdate(existing, want types.CoagentSourcePacket) error {
	if existing.AgentID != want.AgentID ||
		existing.TargetAgentID != want.TargetAgentID ||
		existing.ChannelID != want.ChannelID ||
		existing.Role != want.Role ||
		existing.Content != want.Content ||
		!reflect.DeepEqual(normalizeCoagentSourcePacketPayload(existing.Packet), normalizeCoagentSourcePacketPayload(want.Packet)) {
		return fmt.Errorf("update_id %s already exists with different payload", want.UpdateID)
	}
	return nil
}

func normalizeCoagentSourcePacketPayload(packet types.CoagentSourcePacketPayload) types.CoagentSourcePacketPayload {
	packet.SchemaVersion = strings.TrimSpace(packet.SchemaVersion)
	packet.Kind = strings.TrimSpace(packet.Kind)
	packet.Summary = strings.TrimSpace(packet.Summary)
	packet.Questions = trimNonEmpty(packet.Questions)
	packet.Notes = trimNonEmpty(packet.Notes)

	claims := make([]types.CoagentPacketClaim, 0, len(packet.Claims))
	for _, claim := range packet.Claims {
		normalized := types.CoagentPacketClaim{
			ClaimID:            strings.TrimSpace(claim.ClaimID),
			Text:               strings.TrimSpace(claim.Text),
			SourceIDs:          trimNonEmpty(claim.SourceIDs),
			Stance:             strings.TrimSpace(claim.Stance),
			RecommendedSurface: strings.TrimSpace(claim.RecommendedSurface),
		}
		if normalized.Text != "" {
			claims = append(claims, normalized)
		}
	}
	packet.Claims = claims

	sources := make([]types.CoagentPacketSource, 0, len(packet.Sources))
	for _, source := range packet.Sources {
		normalized := types.CoagentPacketSource{
			SourceID: strings.TrimSpace(source.SourceID),
			Kind:     strings.TrimSpace(source.Kind),
			Target: types.CoagentPacketSourceTarget{
				URI:       strings.TrimSpace(source.Target.URI),
				Title:     strings.TrimSpace(source.Target.Title),
				MediaType: strings.TrimSpace(source.Target.MediaType),
			},
			Evidence: types.CoagentPacketSourceEvidence{
				State:       strings.TrimSpace(source.Evidence.State),
				Confidence:  strings.TrimSpace(source.Evidence.Confidence),
				RightsScope: strings.TrimSpace(source.Evidence.RightsScope),
			},
		}
		for _, selector := range source.Selectors {
			sel := types.CoagentPacketSourceSelector{
				Kind:   strings.TrimSpace(selector.Kind),
				Quote:  strings.TrimSpace(selector.Quote),
				Start:  selector.Start,
				End:    selector.End,
				X:      selector.X,
				Y:      selector.Y,
				Width:  selector.Width,
				Height: selector.Height,
			}
			if sel.Kind != "" {
				normalized.Selectors = append(normalized.Selectors, sel)
			}
		}
		if normalized.Kind != "" || normalized.Target.URI != "" || normalized.Target.Title != "" {
			sources = append(sources, normalized)
		}
	}
	packet.Sources = sources

	actions := make([]types.CoagentPacketAction, 0, len(packet.Actions))
	for _, action := range packet.Actions {
		normalized := types.CoagentPacketAction{
			ActionID:  strings.TrimSpace(action.ActionID),
			Type:      strings.TrimSpace(action.Type),
			Objective: strings.TrimSpace(action.Objective),
			Inputs:    action.Inputs,
			Safety: types.CoagentPacketActionSafety{
				MutationClass: strings.TrimSpace(action.Safety.MutationClass),
				Network:       strings.TrimSpace(action.Safety.Network),
				FileMutation:  strings.TrimSpace(action.Safety.FileMutation),
			},
		}
		for _, expected := range action.ExpectedSources {
			kind := strings.TrimSpace(expected.Kind)
			if kind == "" {
				continue
			}
			normalized.ExpectedSources = append(normalized.ExpectedSources, types.CoagentPacketExpectedSource{Kind: kind, Required: expected.Required})
		}
		if normalized.Type != "" || normalized.Objective != "" {
			actions = append(actions, normalized)
		}
	}
	packet.Actions = actions
	return packet
}

func validateCoagentSourcePacketPayload(packet types.CoagentSourcePacketPayload) error {
	if packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 {
		return fmt.Errorf("update_coagent schema_version must be %q", types.CoagentSourcePacketSchemaV1)
	}
	if !validCoagentPacketKind(packet.Kind) {
		return fmt.Errorf("update_coagent kind %q is not supported", packet.Kind)
	}
	if packet.Summary == "" {
		return fmt.Errorf("update_coagent summary is required")
	}
	if coagentPacketPayloadEmpty(packet) {
		return fmt.Errorf("update_coagent requires at least one of claims, sources, actions, questions, or notes")
	}
	if packet.Kind == "execution_request" && len(packet.Actions) == 0 {
		return fmt.Errorf("update_coagent kind=execution_request requires actions")
	}
	sourceIDs := make(map[string]bool, len(packet.Sources))
	for i, source := range packet.Sources {
		if err := validateCoagentPacketSource(source); err != nil {
			return fmt.Errorf("update_coagent sources[%d]: %w", i, err)
		}
		sourceID := strings.TrimSpace(source.SourceID)
		if sourceID == "" {
			return fmt.Errorf("update_coagent sources[%d].source_id is required", i)
		}
		if sourceIDs[sourceID] {
			return fmt.Errorf("update_coagent sources[%d].source_id %q is duplicated", i, sourceID)
		}
		sourceIDs[sourceID] = true
	}
	for i, claim := range packet.Claims {
		if err := validateCoagentPacketClaim(claim, sourceIDs); err != nil {
			return fmt.Errorf("update_coagent claims[%d]: %w", i, err)
		}
	}
	for i, action := range packet.Actions {
		if err := validateCoagentPacketAction(action, packet.Kind == "execution_request"); err != nil {
			return fmt.Errorf("update_coagent actions[%d]: %w", i, err)
		}
	}
	return nil
}

func validCoagentPacketKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "evidence_update", "execution_request", "execution_result", "blocker", "question", "proposal", "decision_request":
		return true
	default:
		return false
	}
}

func validateCoagentPacketClaim(claim types.CoagentPacketClaim, sourceIDs map[string]bool) error {
	if strings.TrimSpace(claim.Text) == "" {
		return fmt.Errorf("text is required")
	}
	if stance := strings.TrimSpace(claim.Stance); stance != "" && !validCoagentClaimStance(stance) {
		return fmt.Errorf("stance %q is not supported", stance)
	}
	if surface := strings.TrimSpace(claim.RecommendedSurface); surface != "" && !validCoagentClaimRecommendedSurface(surface) {
		return fmt.Errorf("recommended_surface %q is not supported", surface)
	}
	seen := map[string]bool{}
	for _, sourceID := range claim.SourceIDs {
		sourceID = strings.TrimSpace(sourceID)
		if sourceID == "" {
			return fmt.Errorf("source_ids must not contain empty values")
		}
		if seen[sourceID] {
			return fmt.Errorf("source_id %q is duplicated", sourceID)
		}
		seen[sourceID] = true
		if !sourceIDs[sourceID] {
			return fmt.Errorf("source_id %q does not match packet.sources", sourceID)
		}
	}
	return nil
}

func validateCoagentPacketSource(source types.CoagentPacketSource) error {
	if strings.TrimSpace(source.Kind) == "" {
		return fmt.Errorf("kind is required")
	}
	if strings.TrimSpace(source.Target.URI) == "" {
		return fmt.Errorf("target.uri is required")
	}
	for i, selector := range source.Selectors {
		if strings.TrimSpace(selector.Kind) == "" {
			return fmt.Errorf("selectors[%d].kind is required", i)
		}
	}
	if state := strings.TrimSpace(source.Evidence.State); state != "" && !validCoagentSourceEvidenceState(state) {
		return fmt.Errorf("evidence.state %q is not supported", state)
	}
	if confidence := strings.TrimSpace(source.Evidence.Confidence); confidence != "" && !validCoagentSourceEvidenceConfidence(confidence) {
		return fmt.Errorf("evidence.confidence %q is not supported", confidence)
	}
	return nil
}

func validateCoagentPacketAction(action types.CoagentPacketAction, requireSafety bool) error {
	if strings.TrimSpace(action.Type) == "" {
		return fmt.Errorf("type is required")
	}
	if !validCoagentActionType(action.Type) {
		return fmt.Errorf("type %q is not supported", action.Type)
	}
	if strings.TrimSpace(action.Objective) == "" {
		return fmt.Errorf("objective is required")
	}
	for i, expected := range action.ExpectedSources {
		if strings.TrimSpace(expected.Kind) == "" {
			return fmt.Errorf("expected_sources[%d].kind is required", i)
		}
	}
	safety := action.Safety
	if requireSafety {
		if strings.TrimSpace(safety.MutationClass) == "" || strings.TrimSpace(safety.Network) == "" || strings.TrimSpace(safety.FileMutation) == "" {
			return fmt.Errorf("safety.mutation_class, safety.network, and safety.file_mutation are required for execution_request actions")
		}
	}
	if mutationClass := strings.TrimSpace(safety.MutationClass); mutationClass != "" && !validMutationClass(mutationClass) {
		return fmt.Errorf("safety.mutation_class %q is not supported", mutationClass)
	}
	if network := strings.TrimSpace(safety.Network); network != "" && !validCoagentActionSafetyMode(network) {
		return fmt.Errorf("safety.network %q is not supported", network)
	}
	if fileMutation := strings.TrimSpace(safety.FileMutation); fileMutation != "" && !validCoagentActionSafetyMode(fileMutation) {
		return fmt.Errorf("safety.file_mutation %q is not supported", fileMutation)
	}
	return nil
}

func validCoagentClaimStance(stance string) bool {
	switch strings.TrimSpace(stance) {
	case "supports", "qualifies", "contradicts", "background":
		return true
	default:
		return false
	}
}

func validCoagentClaimRecommendedSurface(surface string) bool {
	switch strings.TrimSpace(surface) {
	case "inline_ref", "block_embed", "source_panel", "decision_log":
		return true
	default:
		return false
	}
}

func validCoagentSourceEvidenceState(state string) bool {
	switch strings.TrimSpace(state) {
	case "available", "pending", "blocked", "unavailable":
		return true
	default:
		return false
	}
}

func validCoagentSourceEvidenceConfidence(confidence string) bool {
	switch strings.TrimSpace(confidence) {
	case "low", "medium", "high":
		return true
	default:
		return false
	}
}

func validCoagentActionType(actionType string) bool {
	switch strings.TrimSpace(actionType) {
	case "run_command", "inspect_file", "produce_diff", "run_tests", "open_browser", "request_worker", "import_source", "revise_texture":
		return true
	default:
		return false
	}
}

func validMutationClass(mutationClass string) bool {
	switch strings.TrimSpace(mutationClass) {
	case "green", "yellow", "orange", "red", "black":
		return true
	default:
		return false
	}
}

func validCoagentActionSafetyMode(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "forbidden", "allowed", "required":
		return true
	default:
		return false
	}
}

func coagentPacketPayloadEmpty(packet types.CoagentSourcePacketPayload) bool {
	return len(packet.Claims) == 0 &&
		len(packet.Sources) == 0 &&
		len(packet.Actions) == 0 &&
		len(packet.Questions) == 0 &&
		len(packet.Notes) == 0
}

func rejectLegacyUpdateCoagentFields(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("decode update_coagent args: %w", err)
	}
	allowed := map[string]bool{
		"agent_id":       true,
		"channel_id":     true,
		"schema_version": true,
		"kind":           true,
		"summary":        true,
		"claims":         true,
		"sources":        true,
		"actions":        true,
		"questions":      true,
		"notes":          true,
	}
	legacy := map[string]bool{
		"update_id":           true,
		"findings":            true,
		"evidence_ids":        true,
		"evidence":            true,
		"artifacts":           true,
		"refs":                true,
		"tests":               true,
		"proposals":           true,
		"capability_requests": true,
	}
	for key := range fields {
		key = strings.TrimSpace(key)
		if legacy[key] {
			return fmt.Errorf("update_coagent legacy field %q is not accepted; use claims, sources, actions, questions, or notes", key)
		}
		if !allowed[key] {
			return fmt.Errorf("update_coagent unknown field %q", key)
		}
	}
	return nil
}

func newCoagentPacket(kind, summary string, claims []types.CoagentPacketClaim, sources []types.CoagentPacketSource, actions []types.CoagentPacketAction, questions, notes []string) types.CoagentSourcePacketPayload {
	return normalizeCoagentSourcePacketPayload(types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          strings.TrimSpace(kind),
		Summary:       strings.TrimSpace(summary),
		Claims:        claims,
		Sources:       sources,
		Actions:       actions,
		Questions:     questions,
		Notes:         notes,
	})
}

func coagentSourceFromURI(sourceID, kind, uri, title string) types.CoagentPacketSource {
	return types.CoagentPacketSource{
		SourceID: strings.TrimSpace(sourceID),
		Kind:     strings.TrimSpace(kind),
		Target: types.CoagentPacketSourceTarget{
			URI:   strings.TrimSpace(uri),
			Title: strings.TrimSpace(title),
		},
		Selectors: []types.CoagentPacketSourceSelector{{Kind: "whole_resource"}},
		Evidence: types.CoagentPacketSourceEvidence{
			State:       "available",
			Confidence:  "medium",
			RightsScope: "private_user_source",
		},
	}
}

func coagentSourcesFromRefs(refs []string) []types.CoagentPacketSource {
	out := make([]types.CoagentPacketSource, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		key, _ := splitTypedWorkerUpdateRef(ref)
		kind := key
		if kind == "" && looksLikeArtifactPath(ref) {
			kind = "file_artifact"
			ref = "file_artifact:" + ref
		}
		if kind == "" {
			continue
		}
		if kind == "evidence" {
			kind = "content_item"
		}
		sourceID := "src-" + sanitizeExportPart(ref)
		if sourceID == "src-" || seen[sourceID] {
			continue
		}
		seen[sourceID] = true
		out = append(out, coagentSourceFromURI(sourceID, kind, ref, ref))
	}
	return out
}

func coagentClaimsFromTexts(texts []string, sources []types.CoagentPacketSource) []types.CoagentPacketClaim {
	sourceIDs := make([]string, 0, len(sources))
	for _, source := range sources {
		if id := strings.TrimSpace(source.SourceID); id != "" {
			sourceIDs = append(sourceIDs, id)
		}
	}
	claims := make([]types.CoagentPacketClaim, 0, len(texts))
	for _, text := range trimNonEmpty(texts) {
		claims = append(claims, coagentClaim(text, sourceIDs...))
	}
	return claims
}

func coagentClaim(text string, sourceIDs ...string) types.CoagentPacketClaim {
	return types.CoagentPacketClaim{
		Text:               strings.TrimSpace(text),
		SourceIDs:          trimNonEmpty(sourceIDs),
		Stance:             "supports",
		RecommendedSurface: "decision_log",
	}
}

func coagentAction(actionType, objective string, inputs map[string]any, expected []types.CoagentPacketExpectedSource, safety types.CoagentPacketActionSafety) types.CoagentPacketAction {
	return types.CoagentPacketAction{
		Type:            strings.TrimSpace(actionType),
		Objective:       strings.TrimSpace(objective),
		Inputs:          inputs,
		ExpectedSources: expected,
		Safety:          safety,
	}
}

func coagentActionsFromTexts(texts []string) []types.CoagentPacketAction {
	actions := make([]types.CoagentPacketAction, 0, len(texts))
	for _, text := range trimNonEmpty(texts) {
		actions = append(actions, coagentAction("revise_texture", text, nil, nil, types.CoagentPacketActionSafety{}))
	}
	return actions
}

func coagentSourcesFromResearchEvidence(items []researchFindingEvidenceInput) []types.CoagentPacketSource {
	sources := make([]types.CoagentPacketSource, 0, len(items))
	for i, item := range items {
		kind := strings.TrimSpace(item.Kind)
		uri := strings.TrimSpace(item.SourceURI)
		if kind == "" && uri == "" {
			continue
		}
		sourceID := fmt.Sprintf("src-evidence-%d", i+1)
		sources = append(sources, coagentSourceFromURI(sourceID, firstNonEmpty(kind, "content_item"), uri, item.Title))
	}
	return sources
}

func coagentPacketSummary(packet types.CoagentSourcePacketPayload) string {
	return strings.TrimSpace(packet.Summary)
}

func coagentPacketKind(packet types.CoagentSourcePacketPayload) string {
	return strings.TrimSpace(packet.Kind)
}

func coagentPacketQuestions(packet types.CoagentSourcePacketPayload) []string {
	return trimNonEmpty(packet.Questions)
}

func coagentPacketNotes(packet types.CoagentSourcePacketPayload) []string {
	return trimNonEmpty(packet.Notes)
}

func coagentPacketSourceURIs(packet types.CoagentSourcePacketPayload, kinds ...string) []string {
	want := map[string]bool{}
	for _, kind := range kinds {
		if kind = strings.TrimSpace(kind); kind != "" {
			want[kind] = true
		}
	}
	out := []string{}
	for _, source := range packet.Sources {
		if len(want) > 0 && !want[strings.TrimSpace(source.Kind)] {
			continue
		}
		if uri := strings.TrimSpace(source.Target.URI); uri != "" {
			out = append(out, uri)
		}
	}
	return out
}
