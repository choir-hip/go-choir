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

type textureTextEdit struct {
	Op         string `json:"op"`
	Find       string `json:"find,omitempty"`
	Replace    string `json:"replace,omitempty"`
	Text       string `json:"text,omitempty"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type editTextureArgs struct {
	DocID          string            `json:"doc_id"`
	BaseRevisionID string            `json:"base_revision_id"`
	Operation      string            `json:"operation"`
	Content        string            `json:"content,omitempty"`
	Edits          []textureTextEdit `json:"edits,omitempty"`
	Rationale      string            `json:"rationale,omitempty"`
	SourceTool     string            `json:"-"`
}

type materializedTextureEdit struct {
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
		Description: "Exceptionally replace the entire current Texture document and store the next complete canonical version. Use only for explicit whole-document recovery rewrites or owner-requested full transformations after auditing hard constraints. Rationale is required.",
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
			in.Edits = nil
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
	update := types.WorkerUpdateRecord{
		UpdateID:      uuid.NewString(),
		OwnerID:       ownerID,
		AgentID:       requesterAgentID,
		TargetAgentID: superAgent.AgentID,
		ChannelID:     channelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileTexture,
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
		// One canonical write per run is mechanically enforced here: once this run
		// has stored its revision the mutation is no longer pending. A second write
		// attempt is rejected so the run keeps exactly one canonical revision. The
		// run is still live and should now delegate (spawn_agent), request super
		// execution, record a Texture decision, request an email handoff, or end.
		return types.Revision{}, fmt.Errorf("texture mutation is %s, not pending: this run already stored its one canonical revision; do not write again, instead delegate, request super, record a decision, request an email handoff, or end the run", mutation.State)
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
		case len(in.Edits) > 0:
			in.Operation = "apply_edits"
		case strings.TrimSpace(in.Content) != "":
			in.Operation = "replace_all"
		}
	}
	materialized, err := materializeTextureToolEdit(in, currentRevision)
	if err != nil {
		return types.Revision{}, err
	}

	canonicalPath, err := rt.ensureCanonicalTextureProjectionPath(ctx, rec.OwnerID, doc)
	if err != nil {
		return types.Revision{}, fmt.Errorf("ensure canonical texture projection path: %w", err)
	}
	revMeta := addTextureEditRevisionMetadata(rt.buildAppagentRevisionMetadata(ctx, rec, doc, rec.OwnerID, mutation), materialized, rec)
	if normalizedContent, normalizedCount := normalizeWireArticleBareSourceRefs(materialized.Content, revMeta, rec); normalizedCount > 0 {
		materialized.Content = normalizedContent
		revMeta = mergeTextureRevisionMetadata(revMeta, map[string]any{
			"source_ref_normalization": map[string]any{
				"normalized_bare_source_refs": normalizedCount,
				"syntax":                      "[label](source:ENTITY_ID)",
			},
		})
	}
	if normalizedContent, normalizedCount, entityPatch := normalizeWireArticleSourceServiceProse(materialized.Content, revMeta, rec); normalizedCount > 0 {
		materialized.Content = normalizedContent
		if len(entityPatch) > 0 {
			revMeta = mergeTextureRevisionMetadata(revMeta, map[string]any{"source_entities": entityPatch})
		}
		revMeta = mergeTextureRevisionMetadata(revMeta, map[string]any{
			"source_ref_normalization": map[string]any{
				"normalized_source_service_prose": normalizedCount,
				"syntax":                          "[label](source:ENTITY_ID)",
			},
		})
	}
	if canonicalPath != "" {
		revMeta = mergeTextureRevisionMetadata(revMeta, map[string]any{
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
		return types.Revision{}, fmt.Errorf("create Texture revision: %w", err)
	}
	storedRev, err := rt.store.GetRevision(ctx, rev.RevisionID, rec.OwnerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("load created Texture revision: %w", err)
	}
	if err := rt.store.CompleteAgentMutation(ctx, rec.RunID, rev.RevisionID); err != nil {
		if err != store.ErrMutationAlreadyCompleted {
			return types.Revision{}, fmt.Errorf("complete Texture mutation: %w", err)
		}
	}
	if mutation.ScheduledMessageSeq > 0 {
		if err := rt.store.UpsertTextureControllerCheckpoint(ctx, store.TextureControllerCheckpoint{
			DocID:                docID,
			OwnerID:              rec.OwnerID,
			IntegratedMessageSeq: mutation.ScheduledMessageSeq,
			UpdatedAt:            time.Now().UTC(),
		}); err != nil {
			return types.Revision{}, fmt.Errorf("update texture controller checkpoint: %w", err)
		}
		if err := rt.markTextureWorkerUpdatesDelivered(ctx, rec, docID, mutation.ScheduledMessageSeq); err != nil {
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

var bareTextureSourceRefRE = regexp.MustCompile(`\[source:([A-Za-z0-9_.:-]{1,160})\]`)

func normalizeWireArticleBareSourceRefs(content string, metadata json.RawMessage, rec *types.RunRecord) (string, int) {
	if !wirepublish.IsWireArticleRevisionRun(rec) {
		return content, 0
	}
	meta := decodeRevisionMetadata(metadata)
	entities := decodeTextureSourceEntities(meta["source_entities"])
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
	normalized := bareTextureSourceRefRE.ReplaceAllStringFunc(content, func(match string) string {
		parts := bareTextureSourceRefRE.FindStringSubmatch(match)
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

func normalizeWireArticleSourceServiceProse(content string, metadata json.RawMessage, rec *types.RunRecord) (string, int, []textureSourceEntity) {
	if !wirepublish.IsWireArticleRevisionRun(rec) {
		return content, 0, nil
	}
	if !wireArticleSourceServiceProseRE.MatchString(content) && !textureRawSourceServiceItemIDRE.MatchString(content) {
		return content, 0, nil
	}
	meta := decodeRevisionMetadata(metadata)
	entities := decodeTextureSourceEntities(meta["source_entities"])
	labels := map[string]string{}
	entityByItem := map[string]textureSourceEntity{}
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
			entities, _ = mergeTextureSourceEntities(entities, []textureSourceEntity{entity})
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
	var content string
	var editCount int
	switch operation {
	case "replace_all":
		if len(current.Content) >= 12000 && strings.TrimSpace(edit.Rationale) == "" {
			return materializedTextureEdit{}, fmt.Errorf("replace_all on long Texture documents requires rationale; use apply_edits for ordinary section or line changes")
		}
		content = edit.Content
		editCount = 1
	case "apply_edits":
		content = current.Content
		if len(edit.Edits) == 0 {
			return materializedTextureEdit{}, fmt.Errorf("apply_edits requires at least one edit")
		}
		for i, textEdit := range edit.Edits {
			var err error
			content, err = applyTextureTextEdit(content, textEdit)
			if err != nil {
				return materializedTextureEdit{}, fmt.Errorf("edit %d: %w", i, err)
			}
		}
		editCount = len(edit.Edits)
	default:
		return materializedTextureEdit{}, fmt.Errorf("operation = %q, want replace_all or apply_edits", edit.Operation)
	}

	content = cleanTextureToolContent(content)
	if content == "" {
		return materializedTextureEdit{}, fmt.Errorf("materialized document content must not be empty")
	}
	return materializedTextureEdit{
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

func applyTextureTextEdit(content string, edit textureTextEdit) (string, error) {
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
