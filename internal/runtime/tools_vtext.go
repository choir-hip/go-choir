package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

func RegisterVTextTools(registry *ToolRegistry, rt *Runtime) error {
	for _, tool := range []Tool{
		newPatchTextureTool(rt),
		newRewriteTextureTool(rt),
		newRecordVTextDecisionTool(rt),
		newRequestSuperExecutionTool(rt),
		newRequestEmailDraftTool(rt),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

type vtextTextEdit struct {
	Op         string `json:"op"`
	Find       string `json:"find,omitempty"`
	Replace    string `json:"replace,omitempty"`
	Text       string `json:"text,omitempty"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type editVTextArgs struct {
	DocID          string          `json:"doc_id"`
	BaseRevisionID string          `json:"base_revision_id"`
	Operation      string          `json:"operation"`
	Content        string          `json:"content,omitempty"`
	Edits          []vtextTextEdit `json:"edits,omitempty"`
	Rationale      string          `json:"rationale,omitempty"`
	SourceTool     string          `json:"-"`
}

type materializedVTextEdit struct {
	Content        string
	Operation      string
	SourceTool     string
	BaseRevisionID string
	EditCount      int
	Rationale      string
	BaseChars      int
	ResultChars    int
	DeltaChars     int
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
		Description: "Apply ordinary structured edits to the current Texture document and store the next complete canonical version. Use this for normal line, paragraph, section, citation, metadata, append, or first-draft changes. Do not use it for whole-document replacement; use rewrite_texture only when a full recovery rewrite is explicitly required.",
		Parameters: jsonSchemaObject(map[string]any{
			"doc_id":           map[string]any{"type": "string"},
			"base_revision_id": map[string]any{"type": "string"},
			"rationale":        map[string]any{"type": "string"},
			"edits": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"op":          map[string]any{"type": "string", "enum": []string{"replace", "append"}},
						"find":        map[string]any{"type": "string"},
						"replace":     map[string]any{"type": "string"},
						"text":        map[string]any{"type": "string"},
						"replace_all": map[string]any{"type": "boolean"},
					},
					"required":             []string{"op"},
					"additionalProperties": false,
				},
			},
		}, []string{"doc_id", "base_revision_id", "edits"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in editVTextArgs
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
		Description: "Exceptionally replace the entire current Texture document and store the next complete canonical version. Use only for explicit whole-document recovery rewrites or owner-requested full transformations after auditing hard constraints. Rationale is required.",
		Parameters: jsonSchemaObject(map[string]any{
			"doc_id":           map[string]any{"type": "string"},
			"base_revision_id": map[string]any{"type": "string"},
			"content":          map[string]any{"type": "string"},
			"rationale":        map[string]any{"type": "string"},
		}, []string{"doc_id", "base_revision_id", "content", "rationale"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in editVTextArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode rewrite_texture args: %w", err)
			}
			if strings.TrimSpace(in.Rationale) == "" {
				return "", fmt.Errorf("rewrite_texture requires rationale")
			}
			in.Operation = "replace_all"
			in.Edits = nil
			in.SourceTool = "rewrite_texture"
			return rt.executeTextureEditTool(ctx, "rewrite_texture", in)
		},
	}
}

func (rt *Runtime) executeTextureEditTool(ctx context.Context, toolName string, in editVTextArgs) (string, error) {
	if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileVText {
		return "", fmt.Errorf("%s is only available to Texture agents", toolName)
	}
	rec := ctxRunRecord(ctx)
	if rec == nil || metadataStringValue(rec.Metadata, "type") != "vtext_agent_revision" {
		return "", fmt.Errorf("%s requires a Texture agent revision run", toolName)
	}
	rev, err := rt.commitVTextToolEdit(context.Background(), rec, in)
	if err != nil {
		return "", err
	}
	result := map[string]any{
		"doc_id":           rev.DocID,
		"revision_id":      rev.RevisionID,
		"base_revision_id": rev.ParentRevisionID,
		"status":           "stored",
	}
	if continuation, ok := rt.requiredContinuationAfterVTextEdit(context.Background(), rec, in, rev); ok && continuation.Tool == "request_email_draft" {
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

type recordVTextDecisionArgs struct {
	DocID        string   `json:"doc_id,omitempty"`
	DecisionKind string   `json:"decision_kind"`
	Reason       string   `json:"reason"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	NextAction   string   `json:"next_action,omitempty"`
}

func newRecordVTextDecisionTool(rt *Runtime) Tool {
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
			if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileVText {
				return "", fmt.Errorf("record_texture_decision is only available to Texture agents")
			}
			rec := ctxRunRecord(ctx)
			if rec == nil {
				return "", fmt.Errorf("record_texture_decision missing run context")
			}
			var in recordVTextDecisionArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode record_texture_decision args: %w", err)
			}
			decisionKind := strings.TrimSpace(in.DecisionKind)
			if !validVTextDecisionKind(decisionKind) {
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
				return "", fmt.Errorf("get vtext document for decision: %w", err)
			}
			now := time.Now().UTC()
			decision := types.VTextDecisionRecord{
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
			if err := rt.store.CreateVTextDecision(ctx, decision); err != nil {
				return "", err
			}
			rt.emitVTextDecisionRecordedEvent(ctx, rec, decision)
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

func validVTextDecisionKind(kind string) bool {
	switch kind {
	case "delegation_opened", "delegation_skipped", "delegation_deferred", "wait_for_evidence", "blocker", "no_worker_needed":
		return true
	default:
		return false
	}
}

type vtextRequiredContinuation struct {
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

func (rt *Runtime) requiredContinuationAfterVTextEdit(ctx context.Context, rec *types.RunRecord, in editVTextArgs, rev types.Revision) (vtextRequiredContinuation, bool) {
	if rt == nil || rt.store == nil || rec == nil || metadataStringValue(rec.Metadata, "type") != "vtext_agent_revision" {
		return vtextRequiredContinuation{}, false
	}
	docID := strings.TrimSpace(in.DocID)
	baseRevisionID := strings.TrimSpace(in.BaseRevisionID)
	if docID == "" || baseRevisionID == "" {
		return vtextRequiredContinuation{}, false
	}
	baseRevision, err := rt.store.GetRevision(ctx, baseRevisionID, rec.OwnerID)
	if err != nil {
		return vtextRequiredContinuation{}, false
	}
	grounded, err := rt.channelHasGroundedHistory(ctx, rec.OwnerID, docID, time.Time{})
	if err != nil {
		return vtextRequiredContinuation{}, false
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
				return vtextRequiredContinuation{
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
	return vtextRequiredContinuation{}, false
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
			if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileVText {
				return "", fmt.Errorf("request_super_execution is only available to vtext agents")
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
			"dedupe_reason":       "vtext_run_already_requested_super",
		}, nil
	}
	now := time.Now().UTC()
	trajectoryID := ""
	if runRec := ctxRunRecord(ctx); runRec != nil {
		trajectoryID = trajectoryIDForRun(runRec)
	}
	update := types.WorkerUpdateRecord{
		UpdateID:      uuid.NewString(),
		OwnerID:       ownerID,
		AgentID:       requesterAgentID,
		TargetAgentID: superAgent.AgentID,
		ChannelID:     channelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileVText,
		Kind:          "assignment",
		Summary:       objective,
		CreatedAt:     now,
	}
	update.Content = buildWorkerUpdateMessage(update)
	message := &types.ChannelMessage{
		ChannelID:    channelID,
		From:         requesterRunID,
		FromAgentID:  requesterAgentID,
		FromRunID:    requesterRunID,
		ToAgentID:    superAgent.AgentID,
		TrajectoryID: trajectoryID,
		Role:         AgentProfileVText,
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
			msg.Role == AgentProfileVText {
			return msg, true, nil
		}
	}
	return types.ChannelMessage{}, false, nil
}

func (rt *Runtime) commitVTextToolEdit(ctx context.Context, rec *types.RunRecord, in editVTextArgs) (types.Revision, error) {
	if rt == nil || rt.store == nil {
		return types.Revision{}, fmt.Errorf("runtime store unavailable")
	}
	rt.vtextEditMu.Lock()
	defer rt.vtextEditMu.Unlock()

	docID := strings.TrimSpace(in.DocID)
	baseRevisionID := strings.TrimSpace(in.BaseRevisionID)

	mutation, err := rt.store.GetAgentMutationByRun(ctx, rec.RunID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("get vtext mutation: %w", err)
	}
	if mutation == nil {
		return types.Revision{}, fmt.Errorf("vtext mutation not found for run %s", rec.RunID)
	}
	if mutation.State != "pending" {
		return types.Revision{}, fmt.Errorf("vtext mutation is %s, not pending", mutation.State)
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
		return types.Revision{}, fmt.Errorf("doc_id %q does not match vtext channel %q", docID, rec.ChannelID)
	}
	if mutation.DocID != docID || mutation.OwnerID != rec.OwnerID {
		return types.Revision{}, fmt.Errorf("vtext mutation does not match edit target")
	}

	doc, err := rt.store.GetDocument(ctx, docID, rec.OwnerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("get vtext document: %w", err)
	}
	if err := rt.canonicalizeAliasedVTextDocumentTitle(ctx, rec.OwnerID, &doc, time.Now().UTC()); err != nil {
		return types.Revision{}, fmt.Errorf("canonicalize vtext document title: %w", err)
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
		case len(in.Edits) > 0:
			in.Operation = "apply_edits"
		case strings.TrimSpace(in.Content) != "":
			in.Operation = "replace_all"
		}
	}
	materialized, err := materializeVTextToolEdit(in, currentRevision)
	if err != nil {
		return types.Revision{}, err
	}

	canonicalPath, err := rt.ensureCanonicalVTextProjectionPath(ctx, rec.OwnerID, doc)
	if err != nil {
		return types.Revision{}, fmt.Errorf("ensure canonical vtext projection path: %w", err)
	}
	revMeta := addVTextEditRevisionMetadata(rt.buildAppagentRevisionMetadata(ctx, rec, doc, rec.OwnerID, mutation), materialized, rec)
	if normalizedContent, normalizedCount := normalizeWireArticleBareSourceRefs(materialized.Content, revMeta, rec); normalizedCount > 0 {
		materialized.Content = normalizedContent
		revMeta = mergeVTextRevisionMetadata(revMeta, map[string]any{
			"source_ref_normalization": map[string]any{
				"normalized_bare_source_refs": normalizedCount,
				"syntax":                      "[label](source:ENTITY_ID)",
			},
		})
	}
	if normalizedContent, normalizedCount, entityPatch := normalizeWireArticleSourceServiceProse(materialized.Content, revMeta, rec); normalizedCount > 0 {
		materialized.Content = normalizedContent
		if len(entityPatch) > 0 {
			revMeta = mergeVTextRevisionMetadata(revMeta, map[string]any{"source_entities": entityPatch})
		}
		revMeta = mergeVTextRevisionMetadata(revMeta, map[string]any{
			"source_ref_normalization": map[string]any{
				"normalized_source_service_prose": normalizedCount,
				"syntax":                          "[label](source:ENTITY_ID)",
			},
		})
	}
	if canonicalPath != "" {
		revMeta = mergeVTextRevisionMetadata(revMeta, map[string]any{
			canonicalTextureSourcePathMetadataKey: canonicalPath,
		})
	}
	now := time.Now().UTC()
	rev := types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            docID,
		OwnerID:          rec.OwnerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		Content:          materialized.Content,
		Citations:        json.RawMessage("[]"),
		Metadata:         revMeta,
		ParentRevisionID: baseRevisionID,
		CreatedAt:        now,
	}
	if err := rt.store.CreateRevision(ctx, rev); err != nil {
		_ = rt.store.FailAgentMutation(ctx, rec.RunID)
		return types.Revision{}, fmt.Errorf("create vtext revision: %w", err)
	}
	storedRev, err := rt.store.GetRevision(ctx, rev.RevisionID, rec.OwnerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("load created vtext revision: %w", err)
	}
	if err := rt.store.CompleteAgentMutation(ctx, rec.RunID, rev.RevisionID); err != nil {
		if err != store.ErrMutationAlreadyCompleted {
			return types.Revision{}, fmt.Errorf("complete vtext mutation: %w", err)
		}
	}
	if mutation.ScheduledMessageSeq > 0 {
		if err := rt.store.UpsertVTextControllerCheckpoint(ctx, store.VTextControllerCheckpoint{
			DocID:                docID,
			OwnerID:              rec.OwnerID,
			IntegratedMessageSeq: mutation.ScheduledMessageSeq,
			UpdatedAt:            time.Now().UTC(),
		}); err != nil {
			return types.Revision{}, fmt.Errorf("update vtext controller checkpoint: %w", err)
		}
		if err := rt.markVTextWorkerUpdatesDelivered(ctx, rec, docID, mutation.ScheduledMessageSeq); err != nil {
			return types.Revision{}, fmt.Errorf("mark vtext worker updates delivered: %w", err)
		}
	}

	rt.emitVTextDocumentRevisionEventForRun(ctx, rec, storedRev)
	completedPayload, _ := json.Marshal(map[string]string{
		"doc_id":      docID,
		"revision_id": storedRev.RevisionID,
		"loop_id":     rec.RunID,
	})
	rt.emitVTextAgentEvent(ctx, rec, types.EventVTextAgentRevisionCompleted,
		events.CauseToolExecution, completedPayload)
	rt.maybeAutonomousPublishWireArticle(ctx, doc, storedRev, rec)
	return storedRev, nil
}

var bareVTextSourceRefRE = regexp.MustCompile(`\[source:([A-Za-z0-9_.:-]{1,160})\]`)

func normalizeWireArticleBareSourceRefs(content string, metadata json.RawMessage, rec *types.RunRecord) (string, int) {
	if !wirepublish.IsWireArticleRevisionRun(rec) {
		return content, 0
	}
	meta := decodeRevisionMetadata(metadata)
	entities := decodeVTextSourceEntities(meta["source_entities"])
	if len(entities) == 0 || !strings.Contains(content, "[source:") {
		return content, 0
	}
	labels := map[string]string{}
	for _, entity := range entities {
		id := strings.TrimSpace(entity.EntityID)
		if id == "" {
			continue
		}
		label := strings.TrimSpace(firstNonEmpty(entity.Label, entity.Kind, "source"))
		if label == "" {
			label = "source"
		}
		labels[id] = label
	}
	if len(labels) == 0 {
		return content, 0
	}
	count := 0
	normalized := bareVTextSourceRefRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := bareVTextSourceRefRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		id := strings.TrimSpace(parts[1])
		label := labels[id]
		if label == "" {
			return match
		}
		count++
		return "[" + label + "](source:" + id + ")"
	})
	return normalized, count
}

var wireArticleSourceServiceProseRE = regexp.MustCompile(`Source Service item (srcitem_[A-Za-z0-9_-]+)`)

func normalizeWireArticleSourceServiceProse(content string, metadata json.RawMessage, rec *types.RunRecord) (string, int, []vtextSourceEntity) {
	if !wirepublish.IsWireArticleRevisionRun(rec) {
		return content, 0, nil
	}
	if !wireArticleSourceServiceProseRE.MatchString(content) && !vtextRawSourceServiceItemIDRE.MatchString(content) {
		return content, 0, nil
	}
	meta := decodeRevisionMetadata(metadata)
	entities := decodeVTextSourceEntities(meta["source_entities"])
	labels := map[string]string{}
	entityByItem := map[string]vtextSourceEntity{}
	for _, entity := range entities {
		id := strings.TrimSpace(entity.EntityID)
		itemID := strings.TrimSpace(entity.Target.ItemID)
		if id == "" {
			continue
		}
		label := strings.TrimSpace(firstNonEmpty(entity.Label, entity.Kind, "source"))
		if label == "" {
			label = "source"
		}
		labels[id] = label
		if itemID != "" {
			entityByItem[itemID] = entity
		}
	}
	count := 0
	normalized := wireArticleSourceServiceProseRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := wireArticleSourceServiceProseRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		itemID := strings.TrimSpace(parts[1])
		entity, ok := entityByItem[itemID]
		if !ok {
			entity = sourceServiceItemRefToSourceEntity(itemID, content)
			entities, _ = mergeVTextSourceEntities(entities, []vtextSourceEntity{entity})
			entityByItem[itemID] = entity
		}
		entityID := strings.TrimSpace(entity.EntityID)
		label := strings.TrimSpace(firstNonEmpty(entity.Label, labels[entityID], "source"))
		if entityID == "" || label == "" {
			return match
		}
		labels[entityID] = label
		count++
		return "[" + label + "](source:" + entityID + ")"
	})
	return normalized, count, entities
}

func materializeVTextToolEdit(edit editVTextArgs, current types.Revision) (materializedVTextEdit, error) {
	baseRevisionID := strings.TrimSpace(edit.BaseRevisionID)
	if baseRevisionID == "" {
		return materializedVTextEdit{}, fmt.Errorf("base_revision_id is required")
	}
	if current.RevisionID == "" {
		return materializedVTextEdit{}, fmt.Errorf("current revision is required")
	}
	if baseRevisionID != current.RevisionID {
		return materializedVTextEdit{}, fmt.Errorf("base_revision_id %q does not match current revision %q", baseRevisionID, current.RevisionID)
	}

	operation := strings.TrimSpace(edit.Operation)
	sourceTool := strings.TrimSpace(edit.SourceTool)
	if sourceTool == "" {
		sourceTool = "patch_texture"
	}
	var content string
	var editCount int
	switch operation {
	case "replace_all":
		if len(current.Content) >= 12000 && strings.TrimSpace(edit.Rationale) == "" {
			return materializedVTextEdit{}, fmt.Errorf("replace_all on long VText documents requires rationale; use apply_edits for ordinary section or line changes")
		}
		content = edit.Content
		editCount = 1
	case "apply_edits":
		content = current.Content
		if len(edit.Edits) == 0 {
			return materializedVTextEdit{}, fmt.Errorf("apply_edits requires at least one edit")
		}
		for i, textEdit := range edit.Edits {
			var err error
			content, err = applyVTextTextEdit(content, textEdit)
			if err != nil {
				return materializedVTextEdit{}, fmt.Errorf("edit %d: %w", i, err)
			}
		}
		editCount = len(edit.Edits)
	default:
		return materializedVTextEdit{}, fmt.Errorf("operation = %q, want replace_all or apply_edits", edit.Operation)
	}

	content = cleanVTextToolContent(content)
	if content == "" {
		return materializedVTextEdit{}, fmt.Errorf("materialized document content must not be empty")
	}
	return materializedVTextEdit{
		Content:        content,
		Operation:      operation,
		SourceTool:     sourceTool,
		BaseRevisionID: baseRevisionID,
		EditCount:      editCount,
		Rationale:      strings.TrimSpace(edit.Rationale),
		BaseChars:      len(current.Content),
		ResultChars:    len(content),
		DeltaChars:     len(content) - len(current.Content),
	}, nil
}

func cleanVTextToolContent(content string) string {
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

func applyVTextTextEdit(content string, edit vtextTextEdit) (string, error) {
	switch strings.TrimSpace(edit.Op) {
	case "replace":
		find := edit.Find
		if find == "" {
			return "", fmt.Errorf("replace edit requires find")
		}
		matches := strings.Count(content, find)
		if matches == 0 {
			return "", fmt.Errorf("find text not present")
		}
		if !edit.ReplaceAll && matches != 1 {
			return "", fmt.Errorf("find text matched %d times; set replace_all true to replace every match", matches)
		}
		if edit.ReplaceAll {
			return strings.ReplaceAll(content, find, edit.Replace), nil
		}
		return strings.Replace(content, find, edit.Replace, 1), nil
	case "append":
		text := strings.TrimSpace(edit.Text)
		if text == "" {
			return "", fmt.Errorf("append edit requires text")
		}
		if strings.TrimSpace(content) == "" {
			return text, nil
		}
		if strings.HasSuffix(content, "\n") || strings.HasPrefix(edit.Text, "\n") {
			return content + edit.Text, nil
		}
		return content + "\n" + edit.Text, nil
	default:
		return "", fmt.Errorf("op = %q, want replace or append", edit.Op)
	}
}

func addVTextEditRevisionMetadata(raw json.RawMessage, edit materializedVTextEdit, rec *types.RunRecord) json.RawMessage {
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
	meta["vtext_edit_kind"] = "vtext_edit"
	meta["vtext_edit_operation"] = edit.Operation
	meta["vtext_edit_base_revision_id"] = edit.BaseRevisionID
	meta["vtext_edit_count"] = edit.EditCount
	meta["vtext_edit_base_chars"] = edit.BaseChars
	meta["vtext_edit_result_chars"] = edit.ResultChars
	meta["vtext_edit_delta_chars"] = edit.DeltaChars
	if edit.Rationale != "" {
		meta["vtext_edit_rationale"] = edit.Rationale
	}
	if rec != nil {
		meta["vtext_run_prompt_chars"] = len(rec.Prompt)
		if contextMode := metadataStringValue(rec.Metadata, "vtext_context_mode"); contextMode != "" {
			meta["vtext_context_mode"] = contextMode
		}
		if rec.CreatedAt.IsZero() || rec.UpdatedAt.IsZero() {
			meta["vtext_run_latency_ms"] = 0
		} else {
			meta["vtext_run_latency_ms"] = rec.UpdatedAt.Sub(rec.CreatedAt).Milliseconds()
		}
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return raw
	}
	return data
}
