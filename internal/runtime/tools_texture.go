package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterTextureTools(registry *ToolRegistry, rt *Runtime) error {
	for _, tool := range []Tool{
		newPatchTextureTool(rt),
		newRewriteTextureTool(rt),
		newRecordTextureDecisionTool(rt),
		newRequestSuperExecutionTool(rt),
		newRequestEmailDraftTool(rt),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

type textureStructuredEdit struct {
	Op             string                   `json:"op"`
	BlockID        string                   `json:"block_id,omitempty"`
	NodeID         string                   `json:"node_id,omitempty"`
	AfterBlockID   string                   `json:"after_block_id,omitempty"`
	Text           string                   `json:"text,omitempty"`
	BlockType      string                   `json:"block_type,omitempty"`
	HeadingLevel   int                      `json:"heading_level,omitempty"`
	SourceEntityID string                   `json:"source_entity_id,omitempty"`
	DisplayMode    string                   `json:"display_mode,omitempty"`
	Offset         *int                     `json:"offset,omitempty"`
	Rationale      string                   `json:"rationale,omitempty"`
	SourceEntity   *texturedoc.SourceEntity `json:"source_entity,omitempty"`
}

type editTextureArgs struct {
	DocID                 string                    `json:"doc_id"`
	BaseRevisionID        string                    `json:"base_revision_id"`
	Operation             string                    `json:"operation"`
	Content               string                    `json:"content,omitempty"`
	StructuredEdits       []textureStructuredEdit   `json:"edits,omitempty"`
	AvailableSources      []texturedoc.SourceEntity `json:"-"`
	Rationale             string                    `json:"rationale,omitempty"`
	SourceTool            string                    `json:"-"`
	UnusedSourceEntityIDs []string                  `json:"-"`
}

type materializedTextureEdit struct {
	Content               string
	BodyDoc               json.RawMessage
	SourceEntities        json.RawMessage
	Operation             string
	SourceTool            string
	BaseRevisionID        string
	EditCount             int
	Rationale             string
	BaseChars             int
	ResultChars           int
	DeltaChars            int
	UnusedSourceEntityIDs []string
}

func isTextureWriteToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case "patch_texture", "rewrite_texture":
		return true
	default:
		return false
	}
}

func newPatchTextureTool(rt *Runtime) Tool {
	return Tool{
		Name:        "patch_texture",
		Description: "Apply validated structured operations to the current Texture document BodyDoc and store the next canonical version. Use update_block_text, insert_block, append_block, delete_node, insert_source_ref, and mark_source_unused. insert_source_ref with display_mode numbered_ref is the default inline citation; use display_mode expanded_ref only when a visible block excerpt is editorially required. Do not send raw document JSON, markdown source links, find/replace patches, or metadata source sidecars.",
		Parameters: jsonSchemaObject(map[string]any{
			"doc_id":           map[string]any{"type": "string"},
			"base_revision_id": map[string]any{"type": "string"},
			"rationale":        map[string]any{"type": "string"},
			"edits": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"op":               map[string]any{"type": "string", "enum": []string{"update_block_text", "insert_block", "append_block", "delete_node", "insert_source_ref", "mark_source_unused"}},
						"block_id":         map[string]any{"type": "string"},
						"node_id":          map[string]any{"type": "string"},
						"after_block_id":   map[string]any{"type": "string"},
						"text":             map[string]any{"type": "string"},
						"block_type":       map[string]any{"type": "string", "enum": []string{"paragraph", "heading"}},
						"heading_level":    map[string]any{"type": "integer"},
						"source_entity_id": map[string]any{"type": "string"},
						"display_mode":     map[string]any{"type": "string", "enum": []string{"numbered_ref", "expanded_ref"}},
						"offset":           map[string]any{"type": "integer"},
						"rationale":        map[string]any{"type": "string", "description": "Required for mark_source_unused: short owner-readable reason the source is not cited in the body."},
						"source_entity": map[string]any{
							"type":        "object",
							"description": "Optional SourceEntity target/selectors/display/evidence payload for a new runtime-minted source. Omit source_entity_id or leave it blank; runtime assigns the canonical source_entity_id.",
						},
					},
					"required":             []string{"op"},
					"additionalProperties": false,
				},
			},
		}, []string{"doc_id", "base_revision_id", "edits"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in editTextureArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode patch_texture args: %w", err)
			}
			in.Operation = "apply_edits"
			in.Content = ""
			in.SourceTool = "patch_texture"
			return rt.executeTextureEditTool(ctx, "patch_texture", in)
		},
	}
}

func newRewriteTextureTool(rt *Runtime) Tool {
	return Tool{
		Name:        "rewrite_texture",
		Description: "Exceptionally rewrite the whole Texture document from plain prose through server-owned StructuredTextureDoc conversion and validation. Use only for explicit recovery rewrites or owner-requested full transformations after auditing source/ref loss. Rationale is required.",
		Parameters: jsonSchemaObject(map[string]any{
			"doc_id":           map[string]any{"type": "string"},
			"base_revision_id": map[string]any{"type": "string"},
			"content":          map[string]any{"type": "string"},
			"rationale":        map[string]any{"type": "string"},
		}, []string{"doc_id", "base_revision_id", "content", "rationale"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in editTextureArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode rewrite_texture args: %w", err)
			}
			if strings.TrimSpace(in.Rationale) == "" {
				return "", fmt.Errorf("rewrite_texture requires rationale")
			}
			in.Operation = "replace_all"
			in.SourceTool = "rewrite_texture"
			return rt.executeTextureEditTool(ctx, "rewrite_texture", in)
		},
	}
}

func (rt *Runtime) executeTextureEditTool(ctx context.Context, toolName string, in editTextureArgs) (string, error) {
	if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileTexture {
		return "", fmt.Errorf("%s is only available to Texture agents", toolName)
	}
	rec := ctxRunRecord(ctx)
	if rec == nil || !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		return "", fmt.Errorf("%s requires a Texture agent revision run", toolName)
	}
	rev, err := rt.commitTextureToolEdit(context.Background(), rec, in)
	if err != nil {
		return "", err
	}
	result := map[string]any{
		"doc_id":           rev.DocID,
		"revision_id":      rev.RevisionID,
		"base_revision_id": rev.ParentRevisionID,
		"status":           "stored",
	}
	if continuation, ok := rt.requiredContinuationAfterTextureEdit(context.Background(), rec, in, rev); ok && continuation.Tool == "request_email_draft" {
		emailResult, err := rt.executeRequiredEmailDraftContinuation(ctx, rec, continuation.Args)
		if err != nil {
			return "", err
		}
		result["email_draft_request"] = emailResult
		result["email_draft_request_status"] = emailResult["status"]
		result["next_instruction"] = "Email appagent draft handoff completed from the stored Texture revision. Do not send mail directly; owner approval remains required."
	}
	return toolResultJSON(result)
}

type recordTextureDecisionArgs struct {
	DocID        string   `json:"doc_id,omitempty"`
	DecisionKind string   `json:"decision_kind"`
	Reason       string   `json:"reason"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	NextAction   string   `json:"next_action,omitempty"`
}

func newRecordTextureDecisionTool(rt *Runtime) Tool {
	allowedKinds := []string{
		"delegation_opened",
		"delegation_skipped",
		"delegation_deferred",
		"wait_for_evidence",
		"blocker",
		"no_worker_needed",
	}
	return Tool{
		Name:        "record_texture_decision",
		Description: "Record an audit-worthy Texture decision outside the canonical document. Use this for reasoned delegation choices, waits, blockers, or no-worker decisions that reviewers may need later. If the owner explicitly asks Texture to record an off-document decision note and the requested record is truthful and within Texture authority, call this tool. Do not use it for ordinary sentence-level edits, and do not put agent process rationale into document text.",
		Parameters: jsonSchemaObject(map[string]any{
			"doc_id": map[string]any{
				"type":        "string",
				"description": "The Texture document id. Omit only when the current Texture run is already scoped to the document.",
			},
			"decision_kind": map[string]any{
				"type":        "string",
				"enum":        allowedKinds,
				"description": "Typed decision category.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Short owner-readable reason. Keep it about the coordination decision, not document prose.",
			},
			"evidence_refs": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional evidence, run, finding, source, or revision refs that support this decision.",
			},
			"next_action": map[string]any{
				"type":        "string",
				"description": "Optional concise next action or blocker discriminator.",
			},
		}, []string{"decision_kind", "reason"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileTexture {
				return "", fmt.Errorf("record_texture_decision is only available to Texture agents")
			}
			rec := ctxRunRecord(ctx)
			if rec == nil {
				return "", fmt.Errorf("record_texture_decision missing run context")
			}
			var in recordTextureDecisionArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode record_texture_decision args: %w", err)
			}
			decisionKind := strings.TrimSpace(in.DecisionKind)
			if !validTextureDecisionKind(decisionKind) {
				return "", fmt.Errorf("decision_kind must be one of delegation_opened, delegation_skipped, delegation_deferred, wait_for_evidence, blocker, no_worker_needed")
			}
			reason := strings.TrimSpace(in.Reason)
			if reason == "" {
				return "", fmt.Errorf("reason must not be empty")
			}
			docID := strings.TrimSpace(in.DocID)
			if docID == "" {
				docID = strings.TrimSpace(firstNonEmpty(
					metadataStringValue(rec.Metadata, "doc_id"),
					rec.ChannelID,
				))
			}
			if docID == "" {
				return "", fmt.Errorf("doc_id is required when the Texture run is not document-scoped")
			}
			if _, err := rt.store.GetDocument(ctx, docID, rec.OwnerID); err != nil {
				return "", fmt.Errorf("get texture document for decision: %w", err)
			}
			// One decision record per (run, kind, reason). The deterministic initial
			// decision recorder may already have stored an equivalent note before the
			// loop, and the model may also call this tool; recording the identical
			// decision twice in one run is never useful, so dedupe idempotently.
			if existing, err := rt.store.ListTextureDecisionsByDocument(ctx, rec.OwnerID, docID, 100); err == nil {
				for _, prior := range existing {
					if prior.RunID == rec.RunID && prior.DecisionKind == decisionKind && prior.Reason == reason {
						return toolResultJSON(map[string]any{
							"decision_id":   prior.DecisionID,
							"doc_id":        prior.DocID,
							"decision_kind": prior.DecisionKind,
							"status":        "recorded",
							"created_at":    prior.CreatedAt.Format(time.RFC3339Nano),
						})
					}
				}
			}
			now := time.Now().UTC()
			decision := types.TextureDecisionRecord{
				DecisionID:   uuid.NewString(),
				OwnerID:      rec.OwnerID,
				DocID:        docID,
				RunID:        rec.RunID,
				TrajectoryID: trajectoryIDForRun(rec),
				ActorID:      rec.AgentID,
				DecisionKind: decisionKind,
				Reason:       reason,
				EvidenceRefs: trimNonEmpty(in.EvidenceRefs),
				NextAction:   strings.TrimSpace(in.NextAction),
				CreatedAt:    now,
			}
			if err := rt.store.CreateTextureDecision(ctx, decision); err != nil {
				return "", err
			}
			rt.emitTextureDecisionRecordedEvent(ctx, rec, decision)
			return toolResultJSON(map[string]any{
				"decision_id":   decision.DecisionID,
				"doc_id":        decision.DocID,
				"decision_kind": decision.DecisionKind,
				"status":        "recorded",
				"created_at":    decision.CreatedAt.Format(time.RFC3339Nano),
			})
		},
	}
}

func validTextureDecisionKind(kind string) bool {
	switch kind {
	case "delegation_opened", "delegation_skipped", "delegation_deferred", "wait_for_evidence", "blocker", "no_worker_needed":
		return true
	default:
		return false
	}
}

type textureRequiredContinuation struct {
	Tool        string
	Args        map[string]any
	Instruction string
}

func (rt *Runtime) executeRequiredEmailDraftContinuation(ctx context.Context, rec *types.RunRecord, args map[string]any) (map[string]any, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal email draft continuation args: %w", err)
	}
	var in requestEmailDraftArgs
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("decode email draft continuation args: %w", err)
	}
	result, err := rt.recordEmailDraftRequest(ctx, rec, in)
	if err != nil {
		return nil, fmt.Errorf("execute email draft continuation: %w", err)
	}
	return result, nil
}

func (rt *Runtime) requiredContinuationAfterTextureEdit(ctx context.Context, rec *types.RunRecord, in editTextureArgs, rev types.Revision) (textureRequiredContinuation, bool) {
	if rt == nil || rt.store == nil || rec == nil || !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		return textureRequiredContinuation{}, false
	}
	docID := strings.TrimSpace(in.DocID)
	baseRevisionID := strings.TrimSpace(in.BaseRevisionID)
	if docID == "" || baseRevisionID == "" {
		return textureRequiredContinuation{}, false
	}
	baseRevision, err := rt.store.GetRevision(ctx, baseRevisionID, rec.OwnerID)
	if err != nil {
		return textureRequiredContinuation{}, false
	}
	grounded, err := rt.channelHasGroundedHistory(ctx, rec.OwnerID, docID, time.Time{})
	if err != nil {
		return textureRequiredContinuation{}, false
	}
	prompt := strings.TrimSpace(firstNonEmpty(
		metadataStringValue(rec.Metadata, "original_prompt"),
		metadataStringValue(rec.Metadata, "request_intent"),
		metadataStringValue(rec.Metadata, "seed_prompt"),
	))
	if prompt == "" {
		prompt = strings.TrimSpace(baseRevision.Content)
	}
	if prompt != "" {
		if intent, ok := extractEmailDraftIntent(prompt, rev.Content); ok {
			if baseRevision.AuthorKind == types.AuthorUser || grounded {
				return textureRequiredContinuation{
					Tool: "request_email_draft",
					Args: map[string]any{
						"doc_id":              rev.DocID,
						"revision_id":         rev.RevisionID,
						"source_content_hash": emailSourceContentHash(rev.DocID, rev.RevisionID, rev.Content),
						"to_addresses":        intent.ToAddresses,
						"subject":             intent.Subject,
						"body_text":           intent.BodyText,
						"approval_mode":       "owner_click_or_email_reply",
					},
					Instruction: "The Texture email artifact is now stored. Call request_email_draft next with the provided arguments before ending this run; stopping now leaves the Email appagent handoff incomplete. Do not call request_super_execution for this simple email draft handoff, and do not send mail directly.",
				}, true
			}
		}
	}
	return textureRequiredContinuation{}, false
}

func newRequestSuperExecutionTool(rt *Runtime) Tool {
	type args struct {
		Objective string `json:"objective"`
		ChannelID string `json:"channel_id,omitempty"`
		Model     string `json:"model,omitempty"`
	}
	return Tool{
		Name:        "request_super_execution",
		Description: "Request privileged execution from the persistent super agent without spawning a new super.",
		Parameters: jsonSchemaObject(map[string]any{
			"objective":  map[string]any{"type": "string"},
			"channel_id": map[string]any{"type": "string"},
			"model":      map[string]any{"type": "string"},
		}, []string{"objective"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileTexture {
				return "", fmt.Errorf("request_super_execution is only available to texture agents")
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode request_super_execution args: %w", err)
			}
			objective := strings.TrimSpace(in.Objective)
			if objective == "" {
				return "", fmt.Errorf("objective must not be empty")
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("request_super_execution missing owner context")
			}
			requesterRunID := stringFromToolContext(ctx, toolCtxRunID)
			requesterAgentID := stringFromToolContext(ctx, toolCtxAgentID)
			channelID := strings.TrimSpace(in.ChannelID)
			if channelID == "" {
				channelID = stringFromToolContext(ctx, toolCtxChannelID)
			}
			result, err := rt.requestPersistentSuperExecution(ctx, ownerID, channelID, requesterRunID, requesterAgentID, objective, in.Model)
			if err != nil {
				return "", err
			}
			return toolResultJSON(result)
		},
	}
}

func (rt *Runtime) requestPersistentSuperExecution(ctx context.Context, ownerID, channelID, requesterRunID, requesterAgentID, objective, model string) (map[string]any, error) {
	objective = strings.TrimSpace(objective)
	if objective == "" {
		return nil, fmt.Errorf("objective must not be empty")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("request_super_execution missing owner context")
	}
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		channelID = superAgent.ChannelID
	}
	if model := strings.TrimSpace(model); model != "" {
		objective += "\n\nRequested model: " + model
	}
	requesterAgentID = strings.TrimSpace(requesterAgentID)
	requesterRunID = strings.TrimSpace(requesterRunID)

	rt.superRequestMu.Lock()
	defer rt.superRequestMu.Unlock()
	if existing, ok, err := rt.findExistingSuperExecutionRequest(ctx, ownerID, channelID, superAgent.AgentID, requesterRunID, requesterAgentID); err != nil {
		return nil, err
	} else if ok {
		superRun, err := rt.reconcilePersistentSuperActor(context.Background(), ownerID, superAgent.AgentID)
		if err != nil {
			return nil, err
		}
		loopID := ""
		state := ""
		if superRun != nil {
			loopID = superRun.RunID
			state = string(superRun.State)
		}
		return map[string]any{
			"agent_id":            superAgent.AgentID,
			"loop_id":             loopID,
			"channel_id":          channelID,
			"cursor":              existing.Seq,
			"profile":             superAgent.Profile,
			"role":                superAgent.Role,
			"requested_by":        requesterAgentID,
			"requested_by_run_id": requesterRunID,
			"persistent":          true,
			"state":               state,
			"request_source":      "update_coagent",
			"deduped":             true,
			"dedupe_reason":       "texture_run_already_requested_super",
		}, nil
	}
	now := time.Now().UTC()
	trajectoryID := ""
	if runRec := ctxRunRecord(ctx); runRec != nil {
		trajectoryID = trajectoryIDForRun(runRec)
	}
	update := types.CoagentSourcePacket{
		UpdateID:      uuid.NewString(),
		OwnerID:       ownerID,
		AgentID:       requesterAgentID,
		TargetAgentID: superAgent.AgentID,
		ChannelID:     channelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileTexture,
		Packet: newCoagentPacket("execution_request", objective, nil, nil, []types.CoagentPacketAction{
			coagentAction("request_worker", objective, map[string]any{"requested_by_run_id": requesterRunID}, nil, types.CoagentPacketActionSafety{
				MutationClass: "red",
				Network:       "allowed",
				FileMutation:  "allowed",
			}),
		}, nil, nil),
		CreatedAt: now,
	}
	update.Content = buildWorkerUpdateMessage(update)
	message := &types.ChannelMessage{
		ChannelID:    channelID,
		From:         requesterRunID,
		FromAgentID:  requesterAgentID,
		FromRunID:    requesterRunID,
		ToAgentID:    superAgent.AgentID,
		TrajectoryID: trajectoryID,
		Role:         AgentProfileTexture,
		Content:      update.Content,
		Timestamp:    now,
	}
	stored, created, err := rt.store.DispatchWorkerUpdate(ctx, update, message)
	if err != nil {
		return nil, err
	}
	if created {
		message.Seq = stored.MessageSeq
		rt.emitChannelMessageEvent(ctx, *message, ownerID)
		rt.wakeUpdatedCoagent(ctx, stored)
	}
	superRun, err := rt.reconcilePersistentSuperActor(context.Background(), ownerID, superAgent.AgentID)
	if err != nil {
		return nil, err
	}
	loopID := ""
	state := ""
	if superRun != nil {
		loopID = superRun.RunID
		state = string(superRun.State)
	}
	return map[string]any{
		"agent_id":            superAgent.AgentID,
		"loop_id":             loopID,
		"channel_id":          channelID,
		"cursor":              stored.MessageSeq,
		"update_id":           stored.UpdateID,
		"profile":             superAgent.Profile,
		"role":                superAgent.Role,
		"requested_by":        requesterAgentID,
		"requested_by_run_id": requesterRunID,
		"persistent":          true,
		"state":               state,
		"request_source":      "update_coagent",
	}, nil
}

func (rt *Runtime) findExistingSuperExecutionRequest(ctx context.Context, ownerID, channelID, superAgentID, requesterRunID, requesterAgentID string) (types.ChannelMessage, bool, error) {
	if rt == nil || rt.store == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(channelID) == "" || strings.TrimSpace(superAgentID) == "" || strings.TrimSpace(requesterRunID) == "" {
		return types.ChannelMessage{}, false, nil
	}
	messages, err := rt.store.ListChannelMessages(ctx, ownerID, channelID, 0, 1000)
	if err != nil {
		return types.ChannelMessage{}, false, fmt.Errorf("request_super_execution dedupe scan: %w", err)
	}
	for _, msg := range messages {
		if msg.ToAgentID == superAgentID &&
			msg.FromRunID == requesterRunID &&
			msg.FromAgentID == requesterAgentID &&
			isTextureProfileValue(msg.Role) {
			return msg, true, nil
		}
	}
	return types.ChannelMessage{}, false, nil
}

func buildStructuredAppagentRevisionProvenance(rec *types.RunRecord, sourceEntitiesRaw json.RawMessage, now time.Time) json.RawMessage {
	var structured []texturedoc.SourceEntity
	if len(strings.TrimSpace(string(sourceEntitiesRaw))) > 0 {
		_ = json.Unmarshal(sourceEntitiesRaw, &structured)
	}
	sources := make([]types.SourceEntity, 0, len(structured))
	for _, entity := range structured {
		sources = append(sources, provenanceSourceEntityFromStructured(entity))
	}
	prov := types.Provenance{
		SchemaVersion: types.ProvenanceSchemaVersion,
		AuthoredAt:    now.UTC(),
		Sources:       sources,
	}
	if rec != nil {
		prov.AuthoringModel = types.ProvenanceModel{
			Provider: strings.TrimSpace(metadataStringValue(rec.Metadata, "provider")),
			Model:    strings.TrimSpace(metadataStringValue(rec.Metadata, "model")),
		}
	}
	canonical, err := prov.CanonicalJSON()
	if err != nil {
		return nil
	}
	return json.RawMessage(canonical)
}

func provenanceSourceEntityFromStructured(entity texturedoc.SourceEntity) types.SourceEntity {
	targetKind := strings.TrimSpace(entity.Target.Kind)
	return types.SourceEntity{
		EntityID: strings.TrimSpace(entity.SourceEntityID),
		Kind:     targetKind,
		Label:    strings.TrimSpace(firstNonEmpty(entity.Display.Title, entity.Display.Label, entity.Target.ID, entity.Target.URI)),
		Target: types.SourceEntityTarget{
			TargetKind:   targetKind,
			ItemID:       strings.TrimSpace(entity.Target.ID),
			ContentID:    strings.TrimSpace(entity.Target.ID),
			URL:          strings.TrimSpace(entity.Target.URI),
			CanonicalURL: strings.TrimSpace(entity.Target.URI),
		},
		Selectors:  provenanceSourceSelectorsFromStructured(entity.Selectors),
		Display:    provenanceSourceDisplayFromStructured(entity.Display, entity.Evidence),
		Evidence:   types.SourceEntityEvidence{State: strings.TrimSpace(entity.Evidence.State)},
		Provenance: types.SourceEntityProvenance{CreatedBy: strings.TrimSpace(entity.Provenance.CreatedBy)},
	}
}

func provenanceSourceSelectorsFromStructured(selectors []texturedoc.SourceSelector) []types.SourceEntitySelector {
	out := make([]types.SourceEntitySelector, 0, len(selectors))
	for _, selector := range selectors {
		out = append(out, types.SourceEntitySelector{
			SelectorKind: strings.TrimSpace(selector.Kind),
			TextQuote:    metadataString(selector.Data, "exact"),
		})
	}
	return out
}

func provenanceSourceDisplayFromStructured(display texturedoc.SourceDisplay, evidence texturedoc.SourceEvidence) types.SourceEntityDisplay {
	return types.SourceEntityDisplay{
		InlineMode:   strings.TrimSpace(display.Mode),
		ExpandedMode: strings.TrimSpace(display.Mode),
		OpenSurface:  strings.TrimSpace(evidence.OpenSurface),
	}
}

func (rt *Runtime) commitTextureToolEdit(ctx context.Context, rec *types.RunRecord, in editTextureArgs) (types.Revision, error) {
	if rt == nil || rt.store == nil {
		return types.Revision{}, fmt.Errorf("runtime store unavailable")
	}
	rt.textureEditMu.Lock()
	defer rt.textureEditMu.Unlock()

	docID := strings.TrimSpace(in.DocID)
	baseRevisionID := strings.TrimSpace(in.BaseRevisionID)

	mutation, err := rt.store.GetAgentMutationByRun(ctx, rec.RunID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("get texture mutation: %w", err)
	}
	if mutation == nil {
		return types.Revision{}, fmt.Errorf("texture mutation not found for run %s", rec.RunID)
	}
	if mutation.State != "pending" {
		return types.Revision{}, fmt.Errorf("texture mutation is %s, not pending: this Texture actor run is no longer writable", mutation.State)
	}
	if docID == "" {
		docID = strings.TrimSpace(metadataStringValue(rec.Metadata, "doc_id"))
	}
	if docID == "" {
		docID = strings.TrimSpace(rec.ChannelID)
	}
	if docID == "" {
		docID = strings.TrimSpace(mutation.DocID)
	}
	if docID == "" {
		return types.Revision{}, fmt.Errorf("doc_id must not be empty")
	}
	if metaDocID := metadataStringValue(rec.Metadata, "doc_id"); metaDocID != "" && metaDocID != docID {
		return types.Revision{}, fmt.Errorf("doc_id %q does not match run document %q", docID, metaDocID)
	}
	if rec.ChannelID != "" && rec.ChannelID != docID {
		return types.Revision{}, fmt.Errorf("doc_id %q does not match texture channel %q", docID, rec.ChannelID)
	}
	if mutation.DocID != docID || mutation.OwnerID != rec.OwnerID {
		return types.Revision{}, fmt.Errorf("texture mutation does not match edit target")
	}

	doc, err := rt.store.GetDocument(ctx, docID, rec.OwnerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("get texture document: %w", err)
	}
	if err := rt.canonicalizeAliasedTextureDocumentTitle(ctx, rec.OwnerID, &doc, time.Now().UTC()); err != nil {
		return types.Revision{}, fmt.Errorf("canonicalize texture document title: %w", err)
	}
	if strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return types.Revision{}, fmt.Errorf("document has no current revision")
	}
	if baseRevisionID == "" {
		baseRevisionID = strings.TrimSpace(doc.CurrentRevisionID)
	}
	if baseRevisionID == "" {
		return types.Revision{}, fmt.Errorf("base_revision_id is required")
	}
	if doc.CurrentRevisionID != baseRevisionID {
		return types.Revision{}, fmt.Errorf("base_revision_id %q is stale; current revision is %q", baseRevisionID, doc.CurrentRevisionID)
	}
	currentRevision, err := rt.store.GetRevision(ctx, baseRevisionID, rec.OwnerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("get base revision: %w", err)
	}

	in.DocID = docID
	in.BaseRevisionID = baseRevisionID
	if strings.TrimSpace(in.Operation) == "" {
		switch {
		case len(in.StructuredEdits) > 0:
			in.Operation = "apply_edits"
		case strings.TrimSpace(in.Content) != "":
			in.Operation = "replace_all"
		}
	}
	in.AvailableSources = structuredSourceEntitiesFromRuntimeSources(rec.Metadata[textureAvailableSourceEntitiesKey])
	materialized, err := materializeTextureToolEdit(in, currentRevision)
	if err != nil {
		return types.Revision{}, err
	}

	canonicalPath, err := rt.ensureCanonicalTextureProjectionPath(ctx, rec.OwnerID, doc)
	if err != nil {
		return types.Revision{}, fmt.Errorf("ensure canonical texture projection path: %w", err)
	}
	consumedThroughSeq := rt.textureWorkerUpdateCommitSeq(ctx, rec, doc.DocID, mutation)
	revMeta := addTextureEditRevisionMetadata(rt.buildAppagentRevisionMetadata(ctx, rec, doc, rec.OwnerID, mutation, consumedThroughSeq), materialized, rec)
	if materialized.Content == currentRevision.Content {
		meta := decodeRevisionMetadata(revMeta)
		if consumedThroughSeq > 0 {
			return types.Revision{}, fmt.Errorf("worker update revision must change Texture content before consumed updates are marked delivered")
		}
		if metadataBoolValue(meta, "model_prior_interim") || metadataStringValue(meta, "revision_grounding") == "model_prior" {
			return types.Revision{}, fmt.Errorf("initial model-prior Texture revision must change prompt content before first paint is stored")
		}
	}
	if canonicalPath != "" {
		revMeta = mergeTextureRevisionMetadata(revMeta, map[string]any{
			canonicalTextureSourcePathMetadataKey: canonicalPath,
		})
	}
	revMeta = sanitizeTextureToolRevisionMetadata(revMeta)
	now := time.Now().UTC()
	rev := types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            docID,
		OwnerID:          rec.OwnerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		BodyDoc:          materialized.BodyDoc,
		SourceEntities:   materialized.SourceEntities,
		Citations:        json.RawMessage("[]"),
		Metadata:         revMeta,
		Provenance:       buildStructuredAppagentRevisionProvenance(rec, materialized.SourceEntities, now),
		ParentRevisionID: baseRevisionID,
		CreatedAt:        now,
	}
	graph, err := textureToolSourceGraphWriteSet(rev, materialized, rec)
	if err != nil {
		_ = rt.store.FailAgentMutation(ctx, rec.RunID)
		return types.Revision{}, fmt.Errorf("build Texture source graph shadow write: %w", err)
	}
	if err := rt.store.CreateRevisionWithSourceGraph(ctx, rev, graph); err != nil {
		_ = rt.store.FailAgentMutation(ctx, rec.RunID)
		return types.Revision{}, fmt.Errorf("create Texture revision: %w", err)
	}
	storedRev, err := rt.store.GetRevision(ctx, rev.RevisionID, rec.OwnerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("load created Texture revision: %w", err)
	}
	if err := rt.store.RecordAgentMutationRevision(ctx, rec.RunID, rev.RevisionID); err != nil {
		if err != store.ErrMutationAlreadyCompleted {
			return types.Revision{}, fmt.Errorf("record Texture mutation revision: %w", err)
		}
	}
	if consumedThroughSeq > 0 {
		if err := rt.store.UpsertTextureControllerCheckpoint(ctx, store.TextureControllerCheckpoint{
			DocID:                docID,
			OwnerID:              rec.OwnerID,
			IntegratedMessageSeq: consumedThroughSeq,
			UpdatedAt:            time.Now().UTC(),
		}); err != nil {
			return types.Revision{}, fmt.Errorf("update texture controller checkpoint: %w", err)
		}
		if err := rt.markTextureWorkerUpdatesDelivered(ctx, rec, docID, consumedThroughSeq); err != nil {
			return types.Revision{}, fmt.Errorf("mark texture worker updates delivered: %w", err)
		}
	}

	rt.emitTextureDocumentRevisionEventForRun(ctx, rec, storedRev)
	completedPayload, _ := json.Marshal(map[string]string{
		"doc_id":      docID,
		"revision_id": storedRev.RevisionID,
		"loop_id":     rec.RunID,
	})
	rt.emitTextureAgentEvent(ctx, rec, types.EventTextureAgentRevisionCompleted,
		events.CauseToolExecution, completedPayload)
	rt.maybeAutonomousPublishWireArticle(ctx, doc, storedRev, rec)
	return storedRev, nil
}

func textureToolSourceGraphWriteSet(rev types.Revision, materialized materializedTextureEdit, rec *types.RunRecord) (store.TextureSourceGraphWriteSet, error) {
	var entities []texturedoc.SourceEntity
	rawEntities := strings.TrimSpace(string(materialized.SourceEntities))
	if rawEntities != "" && rawEntities != "null" {
		if err := json.Unmarshal(materialized.SourceEntities, &entities); err != nil {
			return store.TextureSourceGraphWriteSet{}, fmt.Errorf("decode structured source entities: %w", err)
		}
	}
	records := make([]store.TextureSourceEntityGraphRecord, 0, len(entities))
	recordsByLegacyID := make(map[string]store.TextureSourceEntityGraphRecord, len(entities))
	recordsByGraphKey := map[string]store.TextureSourceEntityGraphRecord{}
	for i, entity := range entities {
		record, err := textureToolSourceEntityGraphRecord(rev, entity, rec)
		if err != nil {
			return store.TextureSourceGraphWriteSet{}, fmt.Errorf("source_entities[%d]: %w", i, err)
		}
		key := record.CanonicalID + "\x00" + record.VersionID
		if existing, ok := recordsByGraphKey[key]; ok {
			if legacyID := strings.TrimSpace(record.LegacySourceEntityID); legacyID != "" {
				recordsByLegacyID[legacyID] = existing
			}
			continue
		}
		recordsByGraphKey[key] = record
		records = append(records, record)
		if legacyID := strings.TrimSpace(record.LegacySourceEntityID); legacyID != "" {
			recordsByLegacyID[legacyID] = record
		}
	}
	graph := store.TextureSourceGraphWriteSet{SourceEntities: records}
	rawBodyDoc := strings.TrimSpace(string(materialized.BodyDoc))
	if rawBodyDoc == "" || rawBodyDoc == "null" {
		return graph, nil
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(materialized.BodyDoc, &doc); err != nil {
		return store.TextureSourceGraphWriteSet{}, fmt.Errorf("decode structured body_doc: %w", err)
	}
	refs, err := textureToolSourceRefGraphRecords(rev, doc, recordsByLegacyID, rec)
	if err != nil {
		return store.TextureSourceGraphWriteSet{}, err
	}
	graph.SourceRefs = refs
	return graph, nil
}

func textureToolSourceRefGraphRecords(rev types.Revision, doc texturedoc.StructuredTextureDoc, entitiesByLegacyID map[string]store.TextureSourceEntityGraphRecord, rec *types.RunRecord) ([]store.TextureSourceRefGraphRecord, error) {
	var refs []store.TextureSourceRefGraphRecord
	var walk func(node texturedoc.Node, path string) error
	walk = func(node texturedoc.Node, path string) error {
		if node.Type == "source_ref" {
			ref, err := textureToolSourceRefGraphRecord(rev, node, path, entitiesByLegacyID, rec)
			if err != nil {
				return err
			}
			refs = append(refs, ref)
		}
		for i, child := range node.Content {
			if err := walk(child, fmt.Sprintf("%s.content[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc.Doc, "doc"); err != nil {
		return nil, err
	}
	return refs, nil
}

func textureToolSourceRefGraphRecord(rev types.Revision, node texturedoc.Node, path string, entitiesByLegacyID map[string]store.TextureSourceEntityGraphRecord, rec *types.RunRecord) (store.TextureSourceRefGraphRecord, error) {
	nodeID := textureNodeStringAttr(node, "id")
	sourceEntityID := textureNodeStringAttr(node, "source_entity_id")
	if sourceEntityID == "" {
		return store.TextureSourceRefGraphRecord{}, fmt.Errorf("source_ref at %s has no source_entity_id", path)
	}
	entity, ok := entitiesByLegacyID[sourceEntityID]
	if !ok {
		return store.TextureSourceRefGraphRecord{}, fmt.Errorf("source_ref at %s source_entity_id %q does not resolve to a graph source entity version", path, sourceEntityID)
	}
	displayMode := textureNodeStringAttr(node, "display_mode")
	if displayMode == "" {
		displayMode = store.TextureSourceRefDisplayNumbered
	}
	occurrenceKey := path + "\x00" + nodeID + "\x00" + sourceEntityID
	canonicalID, err := store.BuildTextureSourceRefCanonicalID(rev.OwnerID, rev.RevisionID, occurrenceKey)
	if err != nil {
		return store.TextureSourceRefGraphRecord{}, err
	}
	pathHash := objectgraph.SHA256([]byte(path))
	metadata := map[string]any{
		"schema_version":             "choir.source_ref.v1",
		"doc_id":                     rev.DocID,
		"texture_revision_id":        rev.RevisionID,
		"body_node_id":               nodeID,
		"body_node_path_hash":        pathHash,
		"legacy_source_entity_id":    sourceEntityID,
		"source_entity_canonical_id": entity.CanonicalID,
		"source_entity_version_id":   entity.VersionID,
		"display_mode":               displayMode,
		"citation_state":             "cited",
		"texture_parent_revision_id": rev.ParentRevisionID,
	}
	if rec != nil {
		if runID := strings.TrimSpace(rec.RunID); runID != "" {
			metadata["created_run_id"] = runID
		}
	}
	normalized, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		return store.TextureSourceRefGraphRecord{}, err
	}
	return store.TextureSourceRefGraphRecord{
		CanonicalID:             canonicalID,
		OwnerID:                 rev.OwnerID,
		DocID:                   rev.DocID,
		TextureRevisionID:       rev.RevisionID,
		BodyNodeID:              nodeID,
		BodyNodePathHash:        pathHash,
		LegacySourceEntityID:    sourceEntityID,
		SourceEntityCanonicalID: entity.CanonicalID,
		SourceEntityVersionID:   entity.VersionID,
		DisplayMode:             displayMode,
		CitationState:           "cited",
		Metadata:                normalized,
		CreatedAt:               rev.CreatedAt,
	}, nil
}

func textureToolSourceEntityGraphRecord(rev types.Revision, entity texturedoc.SourceEntity, rec *types.RunRecord) (store.TextureSourceEntityGraphRecord, error) {
	sourceKind := strings.TrimSpace(entity.Target.Kind)
	if sourceKind == "" {
		sourceKind = "source_entity"
	}
	targetIdentity := textureToolSourceEntityTargetIdentity(entity)
	if targetIdentity == "" {
		return store.TextureSourceEntityGraphRecord{}, fmt.Errorf("target identity is required")
	}
	canonicalID, err := store.BuildTextureSourceEntityCanonicalID(rev.OwnerID, rev.OwnerID, sourceKind, targetIdentity)
	if err != nil {
		return store.TextureSourceEntityGraphRecord{}, err
	}
	body := textureToolSourceEntityBody(entity)
	metadata, err := textureToolSourceEntityGraphMetadata(rev, entity, sourceKind, targetIdentity, rec)
	if err != nil {
		return store.TextureSourceEntityGraphRecord{}, err
	}
	versionID, contentHash, normalized, err := store.TextureSourceGraphVersionID(store.TextureSourceEntityObjectKind, body, metadata)
	if err != nil {
		return store.TextureSourceEntityGraphRecord{}, err
	}
	return store.TextureSourceEntityGraphRecord{
		CanonicalID:          canonicalID,
		OwnerID:              rev.OwnerID,
		VersionID:            versionID,
		ContentHash:          contentHash,
		Body:                 body,
		Metadata:             normalized,
		LegacySourceEntityID: strings.TrimSpace(entity.SourceEntityID),
		CreatedAt:            rev.CreatedAt,
	}, nil
}

func textureToolSourceEntityTargetIdentity(entity texturedoc.SourceEntity) string {
	if uri := strings.TrimSpace(entity.Target.URI); uri != "" {
		return uri
	}
	if id := strings.TrimSpace(entity.Target.ID); id != "" {
		return id
	}
	return strings.TrimSpace(entity.SourceEntityID)
}

func textureToolSourceEntityBody(entity texturedoc.SourceEntity) []byte {
	for _, key := range []string{"text", "summary", "content"} {
		if value, ok := entity.ReaderSnapshot[key].(string); ok && strings.TrimSpace(value) != "" {
			return []byte(strings.TrimSpace(value))
		}
	}
	return nil
}

func textureToolSourceEntityGraphMetadata(rev types.Revision, entity texturedoc.SourceEntity, sourceKind, targetIdentity string, rec *types.RunRecord) (json.RawMessage, error) {
	target := map[string]any{
		"kind":     sourceKind,
		"identity": targetIdentity,
	}
	if uri := strings.TrimSpace(entity.Target.URI); uri != "" {
		target["uri"] = uri
	}
	if id := strings.TrimSpace(entity.Target.ID); id != "" {
		target["id"] = id
	}
	if len(entity.Target.Metadata) > 0 {
		target["metadata"] = entity.Target.Metadata
	}
	display := map[string]any{}
	if title := strings.TrimSpace(entity.Display.Title); title != "" {
		display["title"] = title
	}
	if label := strings.TrimSpace(entity.Display.Label); label != "" {
		display["label"] = label
	}
	if description := strings.TrimSpace(entity.Display.Description); description != "" {
		display["description"] = description
	}
	if mode := strings.TrimSpace(entity.Display.Mode); mode != "" {
		display["display_mode"] = mode
	}
	evidence := map[string]any{}
	if state := strings.TrimSpace(entity.Evidence.State); state != "" {
		evidence["state"] = state
	}
	if openSurface := strings.TrimSpace(entity.Evidence.OpenSurface); openSurface != "" {
		evidence["open_surface"] = openSurface
	}
	if relation := strings.TrimSpace(entity.Evidence.Relation); relation != "" {
		evidence["relation"] = relation
	}
	if researchState := strings.TrimSpace(entity.Evidence.ResearchState); researchState != "" {
		evidence["research_state"] = researchState
	}
	if uncertainty := strings.TrimSpace(entity.Evidence.Uncertainty); uncertainty != "" {
		evidence["uncertainty"] = uncertainty
	}
	if readerArtifactState := strings.TrimSpace(entity.Evidence.ReaderArtifactState); readerArtifactState != "" {
		evidence["reader_artifact_state"] = readerArtifactState
	}
	if len(entity.Evidence.EvidenceRefs) > 0 {
		evidence["evidence_refs"] = entity.Evidence.EvidenceRefs
	}
	provenance := map[string]any{}
	if createdBy := strings.TrimSpace(entity.Provenance.CreatedBy); createdBy != "" {
		provenance["created_by"] = createdBy
	}
	if createdAt := strings.TrimSpace(entity.Provenance.CreatedAt); createdAt != "" {
		provenance["created_at"] = createdAt
	}
	if sourceSystem := strings.TrimSpace(entity.Provenance.SourceSystem); sourceSystem != "" {
		provenance["source_system"] = sourceSystem
	}
	if importArtifact := strings.TrimSpace(entity.Provenance.ImportArtifact); importArtifact != "" {
		provenance["import_artifact"] = importArtifact
	}
	if rightsScope := strings.TrimSpace(entity.Provenance.RightsScope); rightsScope != "" {
		provenance["rights_scope"] = rightsScope
	}
	if entity.Provenance.UntrustedSourceText {
		provenance["untrusted_source_text"] = true
	}
	metadata := map[string]any{
		"schema_version":          "choir.source_entity.v1",
		"legacy_entity_id":        strings.TrimSpace(entity.SourceEntityID),
		"source_kind":             sourceKind,
		"target":                  target,
		"display":                 display,
		"evidence":                evidence,
		"provenance":              provenance,
		"texture_doc_id":          rev.DocID,
		"texture_revision_id":     rev.RevisionID,
		"texture_parent_revision": rev.ParentRevisionID,
	}
	if rec != nil {
		if runID := strings.TrimSpace(rec.RunID); runID != "" {
			metadata["created_run_id"] = runID
		}
	}
	if len(entity.Selectors) > 0 {
		metadata["selectors"] = entity.Selectors
	}
	if len(entity.ReaderSnapshotStatus) > 0 {
		metadata["reader_snapshot_status"] = entity.ReaderSnapshotStatus
	}
	return objectgraph.NormalizeMetadata(metadata)
}

func sanitizeTextureToolRevisionMetadata(raw json.RawMessage) json.RawMessage {
	meta := decodeRevisionMetadata(raw)
	if meta == nil {
		return raw
	}
	for _, key := range []string{
		"source_entities",
		"media_source_refs",
		"source_gaps",
		"source_repair_resolutions",
		"source_attachment_manifest",
		"source_ref_normalization",
		"citations_json",
	} {
		delete(meta, key)
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return raw
	}
	return data
}

func materializeTextureToolEdit(edit editTextureArgs, current types.Revision) (materializedTextureEdit, error) {
	baseRevisionID := strings.TrimSpace(edit.BaseRevisionID)
	if baseRevisionID == "" {
		return materializedTextureEdit{}, fmt.Errorf("base_revision_id is required")
	}
	if current.RevisionID == "" {
		return materializedTextureEdit{}, fmt.Errorf("current revision is required")
	}
	if baseRevisionID != current.RevisionID {
		return materializedTextureEdit{}, fmt.Errorf("base_revision_id %q does not match current revision %q", baseRevisionID, current.RevisionID)
	}

	operation := strings.TrimSpace(edit.Operation)
	sourceTool := strings.TrimSpace(edit.SourceTool)
	if sourceTool == "" {
		sourceTool = "patch_texture"
	}
	doc, entities, err := structuredRevisionForTextureToolEdit(edit, current)
	if err != nil {
		return materializedTextureEdit{}, err
	}
	entities = mergeStructuredSourceEntityPool(entities, edit.AvailableSources)
	var unusedSourceEntityIDs []string
	var editCount int
	switch operation {
	case "replace_all":
		if len(current.Content) >= 12000 && strings.TrimSpace(edit.Rationale) == "" {
			return materializedTextureEdit{}, fmt.Errorf("replace_all on long Texture documents requires rationale; use apply_edits for ordinary section or line changes")
		}
		doc, err = structuredTextureToolDocFromMarkdown(edit.DocID, uuid.NewString(), cleanTextureToolContent(edit.Content))
		if err != nil {
			return materializedTextureEdit{}, err
		}
		entities = nil
		editCount = 1
	case "apply_edits":
		if len(edit.StructuredEdits) == 0 {
			return materializedTextureEdit{}, fmt.Errorf("apply_edits requires at least one edit")
		}
		if err := validateStructuredTextureEditBatch(edit.StructuredEdits); err != nil {
			return materializedTextureEdit{}, err
		}
		for i, structuredEdit := range edit.StructuredEdits {
			if err := applyStructuredTextureEdit(&doc, &entities, structuredEdit); err != nil {
				return materializedTextureEdit{}, fmt.Errorf("edit %d: %w", i, err)
			}
			if structuredEdit.Op == "mark_source_unused" {
				unusedSourceEntityIDs = append(unusedSourceEntityIDs, strings.TrimSpace(structuredEdit.SourceEntityID))
			}
		}
		// Carry forward unused declarations from the prior revision so the
		// tri-state source invariant round-trips across edits.
		unusedSourceEntityIDs = append(unusedSourceEntityIDs, revisionUnusedSourceEntityIDs(current)...)
		editCount = len(edit.StructuredEdits)
	default:
		return materializedTextureEdit{}, fmt.Errorf("operation = %q, want replace_all or apply_edits", edit.Operation)
	}

	entities = filterDetachedStructuredSourceEntities(doc, entities, unusedSourceEntityIDs...)
	unusedSourceEntityIDs = dedupeTextureUnusedSourceIDs(unusedSourceEntityIDs)
	projection, err := texturedoc.Project(doc, entities, unusedSourceEntityIDs...)
	if err != nil {
		return materializedTextureEdit{}, fmt.Errorf("structured Texture document validation failed: %w", err)
	}
	bodyDocJSON, err := json.Marshal(doc)
	if err != nil {
		return materializedTextureEdit{}, fmt.Errorf("marshal structured Texture document: %w", err)
	}
	sourceEntitiesJSON, err := json.Marshal(entities)
	if err != nil {
		return materializedTextureEdit{}, fmt.Errorf("marshal structured Texture source entities: %w", err)
	}
	content := strings.TrimSpace(projection.Text)
	if content == "" {
		return materializedTextureEdit{}, fmt.Errorf("materialized document content must not be empty")
	}
	return materializedTextureEdit{
		Content:               content,
		BodyDoc:               json.RawMessage(bodyDocJSON),
		SourceEntities:        json.RawMessage(sourceEntitiesJSON),
		Operation:             operation,
		SourceTool:            sourceTool,
		BaseRevisionID:        baseRevisionID,
		EditCount:             editCount,
		Rationale:             strings.TrimSpace(edit.Rationale),
		BaseChars:             len(current.Content),
		ResultChars:           len(content),
		DeltaChars:            len(content) - len(current.Content),
		UnusedSourceEntityIDs: unusedSourceEntityIDs,
	}, nil
}

func structuredRevisionForTextureToolEdit(edit editTextureArgs, current types.Revision) (texturedoc.StructuredTextureDoc, []texturedoc.SourceEntity, error) {
	if len(strings.TrimSpace(string(current.BodyDoc))) == 0 {
		return plainStructuredTextureToolDoc(edit.DocID, current.RevisionID, current.Content), nil, nil
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(current.BodyDoc, &doc); err != nil {
		return texturedoc.StructuredTextureDoc{}, nil, fmt.Errorf("current body_doc is invalid JSON: %w", err)
	}
	var entities []texturedoc.SourceEntity
	sourceEntitiesRaw := strings.TrimSpace(string(current.SourceEntities))
	if sourceEntitiesRaw != "" && sourceEntitiesRaw != "null" {
		if err := json.Unmarshal(current.SourceEntities, &entities); err != nil {
			return texturedoc.StructuredTextureDoc{}, nil, fmt.Errorf("current source_entities are invalid JSON: %w", err)
		}
	}
	if err := texturedoc.Validate(doc, entities, revisionUnusedSourceEntityIDs(current)...); err != nil {
		return texturedoc.StructuredTextureDoc{}, nil, fmt.Errorf("current structured Texture revision is invalid: %w", err)
	}
	return doc, entities, nil
}

// revisionUnusedSourceEntityIDs reads the unused_source_entity_ids list from a
// revision's metadata so the tri-state source invariant (cited, toolbar-only,
// marked-unused) round-trips across revision edits.
func revisionUnusedSourceEntityIDs(rev types.Revision) []string {
	meta := decodeRevisionMetadata(rev.Metadata)
	if meta == nil {
		return nil
	}
	switch value := meta["unused_source_entity_ids"].(type) {
	case []string:
		return value
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func dedupeTextureUnusedSourceIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func plainStructuredTextureToolDoc(docID, revisionID, content string) texturedoc.StructuredTextureDoc {
	content = cleanTextureToolContent(content)
	return texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": textureToolNodeID("doc", docID, revisionID, "root")},
			Content: []texturedoc.Node{{
				Type:    "paragraph",
				Attrs:   map[string]any{"id": textureToolNodeID("p", docID, revisionID, "0")},
				Content: plainTextureToolInlineNodes(content),
			}},
		},
	}
}

func structuredTextureToolDocFromMarkdown(docID, revisionID, content string) (texturedoc.StructuredTextureDoc, error) {
	parseInline := func(text string) ([]texturedoc.Node, error) {
		if markdownLineageSourceLinkOrMarkerRE.MatchString(text) {
			return nil, fmt.Errorf("rewrite_texture content must not contain markdown source links or numeric citation markers; use patch_texture insert_source_ref for native citations")
		}
		return plainTextureToolInlineNodes(text), nil
	}
	nextSeq := 0
	nextBlockID := func(prefix string) string {
		nextSeq++
		return textureToolNodeID(prefix, docID, revisionID, fmt.Sprintf("%d", nextSeq))
	}
	paragraphNode := func(text string) (texturedoc.Node, bool, error) {
		nodes, err := parseInline(strings.TrimSpace(text))
		if err != nil {
			return texturedoc.Node{}, false, err
		}
		if len(nodes) == 0 {
			return texturedoc.Node{}, false, nil
		}
		return texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": nextBlockID("p")}, Content: nodes}, true, nil
	}
	blocks, err := markdownLineageBodyDocBlocks(content, parseInline, nextBlockID, paragraphNode)
	if err != nil {
		return texturedoc.StructuredTextureDoc{}, err
	}
	if len(blocks) == 0 {
		return texturedoc.StructuredTextureDoc{}, fmt.Errorf("rewrite_texture content must not be empty")
	}
	return texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:    "doc",
			Attrs:   map[string]any{"id": textureToolNodeID("doc", docID, revisionID, "root")},
			Content: blocks,
		},
	}, nil
}

func plainTextureToolInlineNodes(content string) []texturedoc.Node {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if content == "" {
		return nil
	}
	parts := strings.Split(content, "\n")
	nodes := make([]texturedoc.Node, 0, len(parts)*2-1)
	for i, part := range parts {
		if i > 0 {
			nodes = append(nodes, texturedoc.Node{Type: "hard_break"})
		}
		if part != "" {
			nodes = append(nodes, texturedoc.Node{Type: "text", Text: part})
		}
	}
	return nodes
}

func textureToolNodeID(prefix, docID, revisionID, suffix string) string {
	parts := []string{strings.TrimSpace(prefix), strings.TrimSpace(docID), strings.TrimSpace(revisionID), strings.TrimSpace(suffix)}
	for i, part := range parts {
		if part == "" {
			parts[i] = "unknown"
		}
	}
	return strings.Join(parts, "-")
}

func validateStructuredTextureEditBatch(edits []textureStructuredEdit) error {
	seenSourceOffsets := map[string]int{}
	for i, edit := range edits {
		if strings.TrimSpace(edit.Op) != "insert_source_ref" || edit.Offset == nil {
			continue
		}
		key := strings.TrimSpace(edit.BlockID) + "\x00" + fmt.Sprintf("%d", *edit.Offset)
		if first, ok := seenSourceOffsets[key]; ok {
			return fmt.Errorf("insert_source_ref edits %d and %d target the same block_id and offset; source refs must be distributed next to the claims they support", first, i)
		}
		seenSourceOffsets[key] = i
	}
	return nil
}

func applyStructuredTextureEdit(doc *texturedoc.StructuredTextureDoc, entities *[]texturedoc.SourceEntity, edit textureStructuredEdit) error {
	switch strings.TrimSpace(edit.Op) {
	case "update_block_text":
		blockID := strings.TrimSpace(edit.BlockID)
		if blockID == "" {
			return fmt.Errorf("update_block_text requires block_id")
		}
		block := findStructuredNodeByID(&doc.Doc, blockID)
		if block == nil {
			return fmt.Errorf("block_id %q not found", blockID)
		}
		if block.Type != "paragraph" && block.Type != "heading" {
			return fmt.Errorf("update_block_text supports paragraph or heading blocks, got %q", block.Type)
		}
		if textureToolTextLooksLikeMarkdownDocument(edit.Text) {
			return fmt.Errorf("update_block_text is a single-block operation and cannot accept whole-document markdown; use insert_block/append_block for structured sections or rewrite_texture for an audited full-document rewrite")
		}
		preservedRefs := collectDirectSourceRefNodes(block.Content)
		block.Content = append(plainTextureToolInlineNodes(cleanTextureToolContent(edit.Text)), preservedRefs...)
		return nil
	case "insert_block", "append_block":
		block, err := structuredBlockFromEdit(edit)
		if err != nil {
			return err
		}
		if strings.TrimSpace(edit.Op) == "append_block" || strings.TrimSpace(edit.AfterBlockID) == "" {
			doc.Doc.Content = append(doc.Doc.Content, block)
			return nil
		}
		return insertStructuredBlockAfter(&doc.Doc.Content, strings.TrimSpace(edit.AfterBlockID), block)
	case "delete_node":
		nodeID := strings.TrimSpace(edit.NodeID)
		if nodeID == "" {
			return fmt.Errorf("delete_node requires node_id")
		}
		if nodeID == textureNodeStringAttr(doc.Doc, "id") {
			return fmt.Errorf("delete_node cannot delete the document root")
		}
		if !deleteStructuredNodeByID(&doc.Doc.Content, nodeID) {
			return fmt.Errorf("node_id %q not found", nodeID)
		}
		return nil
	case "insert_source_ref":
		blockID := strings.TrimSpace(edit.BlockID)
		if blockID == "" {
			return fmt.Errorf("insert_source_ref requires block_id")
		}
		block := findStructuredNodeByID(&doc.Doc, blockID)
		if block == nil {
			return fmt.Errorf("block_id %q not found", blockID)
		}
		if block.Type != "paragraph" && block.Type != "heading" {
			return fmt.Errorf("insert_source_ref supports paragraph or heading blocks, got %q", block.Type)
		}
		sourceEntityID, err := resolveStructuredEditSourceEntity(entities, edit, "numbered_ref")
		if err != nil {
			return err
		}
		displayMode := strings.TrimSpace(edit.DisplayMode)
		if displayMode == "" {
			displayMode = "numbered_ref"
		}
		if displayMode != "numbered_ref" && displayMode != "expanded_ref" {
			return fmt.Errorf("insert_source_ref display_mode %q is not supported; use numbered_ref or expanded_ref", displayMode)
		}
		ref := texturedoc.Node{
			Type: "source_ref",
			Attrs: map[string]any{
				"id":               uuid.NewString(),
				"source_entity_id": sourceEntityID,
				"display_mode":     displayMode,
			},
		}
		if edit.Offset != nil && *edit.Offset == 0 && structuredInlineTextLen(block.Content) > 0 {
			return fmt.Errorf("insert_source_ref offset 0 would place the citation before existing text; omit offset to append to the block or set offset after the supported clause")
		}
		block.Content = insertInlineNodeAtOffset(block.Content, ref, edit.Offset)
		return nil
	case "mark_source_unused":
		sourceEntityID := strings.TrimSpace(edit.SourceEntityID)
		if sourceEntityID == "" {
			return fmt.Errorf("mark_source_unused requires source_entity_id")
		}
		if strings.TrimSpace(edit.Rationale) == "" {
			return fmt.Errorf("mark_source_unused requires a rationale explaining why the source is not cited in the body")
		}
		if !structuredSourceEntityExists(*entities, sourceEntityID) {
			return fmt.Errorf("mark_source_unused source_entity_id %q is not present in the current structured source_entities", sourceEntityID)
		}
		// The unused declaration is recorded in revision metadata by the caller;
		// the source entity remains in the list and the schema validator accepts
		// it without a body source_ref.
		return nil
	default:
		return fmt.Errorf("op = %q, want update_block_text, insert_block, append_block, delete_node, insert_source_ref, or mark_source_unused", edit.Op)
	}
}

func textureToolTextLooksLikeMarkdownDocument(text string) bool {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	nonEmpty := 0
	blankSeen := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if nonEmpty > 0 {
				blankSeen = true
			}
			continue
		}
		nonEmpty++
		if markdownLineageHeadingRE.MatchString(trimmed) ||
			markdownLineageBulletRE.MatchString(trimmed) ||
			markdownLineageOrderedRE.MatchString(trimmed) {
			return true
		}
	}
	return blankSeen && nonEmpty > 1
}

func structuredInlineTextLen(nodes []texturedoc.Node) int {
	total := 0
	for _, node := range nodes {
		switch node.Type {
		case "text":
			total += len([]rune(node.Text))
		case "hard_break":
			total++
		default:
			total += structuredInlineTextLen(node.Content)
		}
	}
	return total
}

func structuredBlockFromEdit(edit textureStructuredEdit) (texturedoc.Node, error) {
	blockType := strings.TrimSpace(edit.BlockType)
	if blockType == "" {
		blockType = "paragraph"
	}
	switch blockType {
	case "paragraph":
		return texturedoc.Node{
			Type:    "paragraph",
			Attrs:   map[string]any{"id": uuid.NewString()},
			Content: plainTextureToolInlineNodes(cleanTextureToolContent(edit.Text)),
		}, nil
	case "heading":
		level := edit.HeadingLevel
		if level == 0 {
			level = 2
		}
		if level < 1 || level > 6 {
			return texturedoc.Node{}, fmt.Errorf("heading_level must be 1..6")
		}
		return texturedoc.Node{
			Type:    "heading",
			Attrs:   map[string]any{"id": uuid.NewString(), "level": level},
			Content: plainTextureToolInlineNodes(cleanTextureToolContent(edit.Text)),
		}, nil
	default:
		return texturedoc.Node{}, fmt.Errorf("block_type = %q, want paragraph or heading", blockType)
	}
}

func resolveStructuredEditSourceEntity(entities *[]texturedoc.SourceEntity, edit textureStructuredEdit, defaultDisplayMode string) (string, error) {
	sourceEntityID := strings.TrimSpace(edit.SourceEntityID)
	if edit.SourceEntity != nil {
		if sourceEntityID != "" {
			return "", fmt.Errorf("source_entity_id must be omitted when source_entity is provided; runtime mints the id")
		}
		entity := *edit.SourceEntity
		entity.SourceEntityID = "src_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if strings.TrimSpace(entity.Display.Mode) == "" {
			entity.Display.Mode = defaultDisplayMode
		}
		if strings.TrimSpace(entity.Evidence.State) == "" {
			entity.Evidence.State = sourcecontract.EvidenceStateAvailable
		}
		if strings.TrimSpace(entity.Evidence.OpenSurface) == "" {
			entity.Evidence.OpenSurface = sourcecontract.OpenSurfaceSource
		}
		if strings.TrimSpace(entity.Provenance.CreatedBy) == "" {
			entity.Provenance.CreatedBy = "runtime"
		}
		if len(entity.Selectors) == 0 {
			entity.Selectors = []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindWholeResource}}
		}
		next := append([]texturedoc.SourceEntity{}, (*entities)...)
		next = append(next, entity)
		*entities = next
		return entity.SourceEntityID, nil
	}
	if sourceEntityID == "" {
		return "", fmt.Errorf("source_entity_id or source_entity is required")
	}
	for _, entity := range *entities {
		if strings.TrimSpace(entity.SourceEntityID) == sourceEntityID {
			return sourceEntityID, nil
		}
	}
	return "", fmt.Errorf("source_entity_id %q is not present in the current structured source_entities", sourceEntityID)
}

func mergeStructuredSourceEntityPool(current, incoming []texturedoc.SourceEntity) []texturedoc.SourceEntity {
	if len(incoming) == 0 {
		return current
	}
	seen := map[string]bool{}
	out := make([]texturedoc.SourceEntity, 0, len(current)+len(incoming))
	for _, entity := range current {
		id := strings.TrimSpace(entity.SourceEntityID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, entity)
	}
	for _, entity := range incoming {
		id := strings.TrimSpace(entity.SourceEntityID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, entity)
	}
	return out
}

func structuredSourceEntitiesFromRuntimeSources(value any) []texturedoc.SourceEntity {
	runtimeEntities := decodeTextureSourceEntities(value)
	out := make([]texturedoc.SourceEntity, 0, len(runtimeEntities))
	for _, entity := range runtimeEntities {
		structured := structuredSourceEntityFromRuntimeSource(entity)
		if strings.TrimSpace(structured.SourceEntityID) != "" {
			out = append(out, structured)
		}
	}
	return out
}

func structuredSourceEntityFromRuntimeSource(entity textureSourceEntity) texturedoc.SourceEntity {
	targetKind := structuredSourceTargetKind(entity)
	displayMode := structuredSourceDisplayMode(entity)
	return texturedoc.SourceEntity{
		SourceEntityID: strings.TrimSpace(entity.EntityID),
		Target: texturedoc.SourceTarget{
			Kind: targetKind,
			URI:  strings.TrimSpace(firstNonEmpty(entity.Target.CanonicalURL, entity.Target.URL)),
			ID: strings.TrimSpace(firstNonEmpty(
				entity.Target.ItemID,
				entity.Target.ContentID,
				entity.Target.FilePath,
				entity.Target.PublicRecordID,
				entity.Target.DocID,
				entity.Target.RevisionID,
				entity.EntityID,
			)),
		},
		Selectors:            structuredSourceSelectorsFromRuntime(entity.Selectors),
		Display:              texturedoc.SourceDisplay{Mode: displayMode, Title: strings.TrimSpace(entity.Label)},
		Evidence:             structuredSourceEvidenceFromRuntime(entity),
		Provenance:           texturedoc.SourceEntityProvenance{CreatedBy: strings.TrimSpace(firstNonEmpty(entity.Provenance.CreatedBy, "runtime"))},
		ReaderSnapshot:       copyStringAnyMap(entity.ReaderSnapshot),
		ReaderSnapshotStatus: copyStringAnyMap(entity.ReaderSnapshotStatus),
	}
}

func structuredSourceTargetKind(entity textureSourceEntity) string {
	switch strings.TrimSpace(entity.Target.TargetKind) {
	case "source_service_item":
		return "source_service_item"
	case "content_item":
		return "content_item"
	case "image", "video", "audio", "pdf", "transcript", "texture_span", "publication_span", "source_viewer_artifact", "reader_artifact", "file_artifact",
		"command_output", "shell_session", "diff_hunk", "patch", "test_run", "app_change_package", "screenshot", "video_artifact", "benchmark_log":
		return strings.TrimSpace(entity.Target.TargetKind)
	case "url", "web_url":
		return "web_url"
	default:
		switch strings.TrimSpace(entity.Kind) {
		case "image":
			return "image"
		case "youtube_video", "video":
			return "video"
		default:
			return "content_item"
		}
	}
}

func structuredSourceDisplayMode(entity textureSourceEntity) string {
	// source_embed display modes (block_embed, excerpt, player, image_preview,
	// pdf_pages, transcript, source_window, inline_chip) collapsed into the
	// source_ref display_mode enum after the hard cutover. Legacy expanded
	// block modes map to expanded_ref; everything else is the default
	// numbered_ref inline citation.
	expandedModes := map[string]bool{
		"block_embed":   true,
		"excerpt":       true,
		"player":        true,
		"image_preview": true,
		"pdf_pages":     true,
		"transcript":    true,
		"source_window": true,
		"expanded_ref":  true,
	}
	mode := strings.TrimSpace(firstNonEmpty(entity.Display.ExpandedMode, entity.Display.InlineMode))
	if expandedModes[mode] {
		return "expanded_ref"
	}
	return "numbered_ref"
}

func structuredSourceSelectorsFromRuntime(selectors []textureSourceEntitySelector) []texturedoc.SourceSelector {
	if len(selectors) == 0 {
		return []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindWholeResource}}
	}
	out := make([]texturedoc.SourceSelector, 0, len(selectors))
	for _, selector := range selectors {
		kind := sourcecontract.NormalizeSelectorKind(selector.SelectorKind)
		if kind == "" {
			kind = sourcecontract.SelectorKindWholeResource
		}
		data := map[string]any{}
		if selector.TextQuote != "" {
			data["exact"] = selector.TextQuote
		}
		if selector.ContentHash != "" {
			data["content_hash"] = selector.ContentHash
		}
		out = append(out, texturedoc.SourceSelector{Kind: kind, Data: data})
	}
	return out
}

func structuredSourceEvidenceFromRuntime(entity textureSourceEntity) texturedoc.SourceEvidence {
	state := sourcecontract.NormalizeEvidenceState(entity.Evidence.State)
	if state == "" {
		state = sourcecontract.EvidenceStateAvailable
	}
	openSurface := sourcecontract.NormalizeOpenSurface(entity.Display.OpenSurface)
	if openSurface == "" {
		openSurface = sourcecontract.OpenSurfaceSource
	}
	return texturedoc.SourceEvidence{
		State:               state,
		OpenSurface:         openSurface,
		ReaderArtifactState: sourcecontract.NormalizeReaderArtifactState(entity.Evidence.SourceRepresentationID),
	}
}

func findStructuredNodeByID(node *texturedoc.Node, nodeID string) *texturedoc.Node {
	if node == nil {
		return nil
	}
	if textureNodeStringAttr(*node, "id") == nodeID {
		return node
	}
	for i := range node.Content {
		if found := findStructuredNodeByID(&node.Content[i], nodeID); found != nil {
			return found
		}
	}
	return nil
}

func insertStructuredBlockAfter(nodes *[]texturedoc.Node, afterNodeID string, block texturedoc.Node) error {
	for i := range *nodes {
		if textureNodeStringAttr((*nodes)[i], "id") == afterNodeID {
			next := append((*nodes)[:i+1], append([]texturedoc.Node{block}, (*nodes)[i+1:]...)...)
			*nodes = next
			return nil
		}
		if err := insertStructuredBlockAfter(&(*nodes)[i].Content, afterNodeID, block); err == nil {
			return nil
		}
	}
	return fmt.Errorf("after_block_id %q not found", afterNodeID)
}

func deleteStructuredNodeByID(nodes *[]texturedoc.Node, nodeID string) bool {
	for i := 0; i < len(*nodes); i++ {
		if textureNodeStringAttr((*nodes)[i], "id") == nodeID {
			*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)
			return true
		}
		if deleteStructuredNodeByID(&(*nodes)[i].Content, nodeID) {
			return true
		}
	}
	return false
}

func collectDirectSourceRefNodes(nodes []texturedoc.Node) []texturedoc.Node {
	var refs []texturedoc.Node
	for _, node := range nodes {
		if node.Type == "source_ref" {
			refs = append(refs, node)
		}
	}
	return refs
}

func insertInlineNodeAtOffset(nodes []texturedoc.Node, insert texturedoc.Node, offset *int) []texturedoc.Node {
	if offset == nil || *offset < 0 {
		return append(nodes, insert)
	}
	remaining := normalizeInlineInsertionOffset(nodes, *offset)
	out := make([]texturedoc.Node, 0, len(nodes)+1)
	inserted := false
	for _, node := range nodes {
		if inserted || node.Type != "text" {
			out = append(out, node)
			continue
		}
		runes := []rune(node.Text)
		if remaining > len(runes) {
			remaining -= len(runes)
			out = append(out, node)
			continue
		}
		if remaining > 0 {
			left := node
			left.Text = string(runes[:remaining])
			out = append(out, left)
		}
		out = append(out, insert)
		if remaining < len(runes) {
			right := node
			right.Text = string(runes[remaining:])
			out = append(out, right)
		}
		inserted = true
	}
	if !inserted {
		out = append(out, insert)
	}
	return out
}

func normalizeInlineInsertionOffset(nodes []texturedoc.Node, offset int) int {
	if offset <= 0 {
		return 0
	}
	runes := []rune{}
	for _, node := range nodes {
		if node.Type != "text" {
			continue
		}
		runes = append(runes, []rune(node.Text)...)
	}
	if offset >= len(runes) {
		return offset
	}
	if !isTextureWordRune(runes[offset-1]) || !isTextureWordRune(runes[offset]) {
		return offset
	}
	for offset < len(runes) && isTextureWordRune(runes[offset]) {
		offset++
	}
	return offset
}

func isTextureWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '\''
}

func filterDetachedStructuredSourceEntities(doc texturedoc.StructuredTextureDoc, entities []texturedoc.SourceEntity, unusedSourceEntityIDs ...string) []texturedoc.SourceEntity {
	referenced := map[string]bool{}
	collectStructuredSourceEntityRefs(doc.Doc, referenced)
	unused := make(map[string]bool, len(unusedSourceEntityIDs))
	for _, id := range unusedSourceEntityIDs {
		unused[strings.TrimSpace(id)] = true
	}
	out := make([]texturedoc.SourceEntity, 0, len(entities))
	for _, entity := range entities {
		id := strings.TrimSpace(entity.SourceEntityID)
		if referenced[id] || unused[id] {
			out = append(out, entity)
		}
	}
	return out
}

func collectStructuredSourceEntityRefs(node texturedoc.Node, refs map[string]bool) {
	if node.Type == "source_ref" {
		if id := textureNodeStringAttr(node, "source_entity_id"); id != "" {
			refs[id] = true
		}
	}
	for _, child := range node.Content {
		collectStructuredSourceEntityRefs(child, refs)
	}
}

func structuredSourceEntityExists(entities []texturedoc.SourceEntity, sourceEntityID string) bool {
	sourceEntityID = strings.TrimSpace(sourceEntityID)
	for _, entity := range entities {
		if strings.TrimSpace(entity.SourceEntityID) == sourceEntityID {
			return true
		}
	}
	return false
}

func textureNodeStringAttr(node texturedoc.Node, key string) string {
	if node.Attrs == nil {
		return ""
	}
	if value, ok := node.Attrs[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func cleanTextureToolContent(content string) string {
	cleaned := strings.TrimSpace(content)
	for {
		next := strings.TrimSpace(cleaned)
		next = strings.TrimPrefix(next, "<payload>")
		next = strings.TrimPrefix(next, "<content>")
		next = strings.TrimSuffix(next, "</payload>")
		next = strings.TrimSuffix(next, "</content>")
		next = trimTrailingClosingMarkupFragment(next)
		next = strings.TrimSpace(next)
		if next == cleaned {
			return cleaned
		}
		cleaned = next
	}
}

func trimTrailingClosingMarkupFragment(content string) string {
	cleaned := strings.TrimSpace(content)
	idx := strings.LastIndex(cleaned, "</")
	if idx < 0 {
		return cleaned
	}
	suffix := cleaned[idx:]
	fragment := strings.TrimPrefix(suffix, "</")
	if len([]rune(suffix)) > 32 || strings.ContainsAny(fragment, " \t\r\n<") {
		return cleaned
	}
	return strings.TrimSpace(cleaned[:idx])
}

func addTextureEditRevisionMetadata(raw json.RawMessage, edit materializedTextureEdit, rec *types.RunRecord) json.RawMessage {
	meta := decodeRevisionMetadata(raw)
	if meta == nil {
		meta = map[string]any{}
	}
	sourceTool := strings.TrimSpace(edit.SourceTool)
	if sourceTool == "" {
		sourceTool = "patch_texture"
	}
	meta["source"] = sourceTool
	meta["texture_edit_tool"] = sourceTool
	meta["texture_edit_kind"] = "texture_edit"
	meta["texture_edit_operation"] = edit.Operation
	meta["texture_edit_base_revision_id"] = edit.BaseRevisionID
	meta["texture_edit_count"] = edit.EditCount
	meta["texture_edit_base_chars"] = edit.BaseChars
	meta["texture_edit_result_chars"] = edit.ResultChars
	meta["texture_edit_delta_chars"] = edit.DeltaChars
	if edit.Rationale != "" {
		meta["texture_edit_rationale"] = edit.Rationale
	}
	if len(edit.UnusedSourceEntityIDs) > 0 {
		meta["unused_source_entity_ids"] = edit.UnusedSourceEntityIDs
	}
	if rec != nil {
		meta["texture_run_prompt_chars"] = len(rec.Prompt)
		if contextMode := metadataStringValue(rec.Metadata, "texture_context_mode"); contextMode != "" {
			meta["texture_context_mode"] = contextMode
		}
		if rec.CreatedAt.IsZero() || rec.UpdatedAt.IsZero() {
			meta["texture_run_latency_ms"] = 0
		} else {
			meta["texture_run_latency_ms"] = rec.UpdatedAt.Sub(rec.CreatedAt).Milliseconds()
		}
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return raw
	}
	return data
}
