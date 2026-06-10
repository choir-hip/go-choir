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
)

func RegisterVTextTools(registry *ToolRegistry, rt *Runtime) error {
	for _, tool := range []Tool{
		newEditVTextTool(rt),
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
}

type materializedVTextEdit struct {
	Content        string
	Operation      string
	BaseRevisionID string
	EditCount      int
	Rationale      string
	BaseChars      int
	ResultChars    int
	DeltaChars     int
}

func newEditVTextTool(rt *Runtime) Tool {
	return Tool{
		Name:        "edit_vtext",
		Description: "Apply a structured edit to the current VText document and store the next complete canonical version. Use apply_edits for ordinary line, paragraph, section, citation, or metadata changes. Use replace_all only for explicit whole-document rewrites and include rationale, especially for long documents.",
		Parameters: jsonSchemaObject(map[string]any{
			"doc_id":           map[string]any{"type": "string"},
			"base_revision_id": map[string]any{"type": "string"},
			"operation":        map[string]any{"type": "string", "enum": []string{"replace_all", "apply_edits"}},
			"content":          map[string]any{"type": "string"},
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
		}, []string{"doc_id", "base_revision_id", "operation"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if stringFromToolContext(ctx, toolCtxProfile) != AgentProfileVText {
				return "", fmt.Errorf("edit_vtext is only available to vtext agents")
			}
			rec := ctxRunRecord(ctx)
			if rec == nil || metadataStringValue(rec.Metadata, "type") != "vtext_agent_revision" {
				return "", fmt.Errorf("edit_vtext requires a vtext agent revision run")
			}
			var in editVTextArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode edit_vtext args: %w", err)
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
			if continuation, ok := rt.requiredContinuationAfterVTextEdit(context.Background(), rec, in, rev); ok {
				if continuation.Tool == "request_email_draft" {
					emailResult, err := rt.executeRequiredEmailDraftContinuation(ctx, rec, continuation.Args)
					if err != nil {
						return "", err
					}
					result["email_draft_request"] = emailResult
					result["email_draft_request_status"] = emailResult["status"]
					result["next_instruction"] = "Email appagent draft handoff completed from the stored VText revision. Do not send mail directly; owner approval remains required."
				}
			}
			return toolResultJSON(result)
		},
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
	if prompt == "" {
		return vtextRequiredContinuation{}, false
	}
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
				Instruction: "The VText email artifact is now stored. Call request_email_draft next with the provided arguments before ending this run; stopping now leaves the Email appagent handoff incomplete. Do not call request_super_execution for this simple email draft handoff, and do not send mail directly.",
			}, true
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
			"request_source":      "super_inbox",
			"deduped":             true,
			"dedupe_reason":       "vtext_run_already_requested_super",
		}, nil
	}
	cursor, err := rt.ChannelCast(ctx, channelID, superAgent.AgentID, "", requesterAgentID, AgentProfileVText, objective)
	if err != nil {
		return nil, err
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
		"cursor":              cursor,
		"profile":             superAgent.Profile,
		"role":                superAgent.Role,
		"requested_by":        requesterAgentID,
		"requested_by_run_id": requesterRunID,
		"persistent":          true,
		"state":               state,
		"request_source":      "super_inbox",
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
	if docID == "" {
		return types.Revision{}, fmt.Errorf("doc_id must not be empty")
	}
	if baseRevisionID == "" {
		return types.Revision{}, fmt.Errorf("base_revision_id is required")
	}
	if metaDocID := metadataStringValue(rec.Metadata, "doc_id"); metaDocID != "" && metaDocID != docID {
		return types.Revision{}, fmt.Errorf("doc_id %q does not match run document %q", docID, metaDocID)
	}
	if rec.ChannelID != "" && rec.ChannelID != docID {
		return types.Revision{}, fmt.Errorf("doc_id %q does not match vtext channel %q", docID, rec.ChannelID)
	}

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
	if doc.CurrentRevisionID != baseRevisionID {
		return types.Revision{}, fmt.Errorf("base_revision_id %q is stale; current revision is %q", baseRevisionID, doc.CurrentRevisionID)
	}
	currentRevision, err := rt.store.GetRevision(ctx, baseRevisionID, rec.OwnerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("get base revision: %w", err)
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
	if canonicalPath != "" {
		revMeta = mergeVTextRevisionMetadata(revMeta, map[string]any{
			"canonical_vtext_source_path": canonicalPath,
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
	if !isWireArticleRevisionRun(rec) {
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
	meta["source"] = "edit_vtext"
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
