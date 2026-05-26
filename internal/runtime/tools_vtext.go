package runtime

import (
	"context"
	"encoding/json"
	"fmt"
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
}

type materializedVTextEdit struct {
	Content        string
	Operation      string
	BaseRevisionID string
	EditCount      int
}

func newEditVTextTool(rt *Runtime) Tool {
	return Tool{
		Name:        "edit_vtext",
		Description: "Apply an explicit edit to the current vtext document and store the next complete canonical version.",
		Parameters: jsonSchemaObject(map[string]any{
			"doc_id":           map[string]any{"type": "string"},
			"base_revision_id": map[string]any{"type": "string"},
			"operation":        map[string]any{"type": "string", "enum": []string{"replace_all", "apply_edits"}},
			"content":          map[string]any{"type": "string"},
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
			if continuation, ok := rt.requiredContinuationAfterInitialVTextEdit(context.Background(), rec, in); ok {
				result["next_required_tool"] = continuation.Tool
				result["next_required_args"] = continuation.Args
				result["next_instruction"] = continuation.Instruction
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

func (rt *Runtime) requiredContinuationAfterInitialVTextEdit(ctx context.Context, rec *types.RunRecord, in editVTextArgs) (vtextRequiredContinuation, bool) {
	if rt == nil || rt.store == nil || rec == nil || metadataStringValue(rec.Metadata, "type") != "vtext_agent_revision" {
		return vtextRequiredContinuation{}, false
	}
	if metadataBoolValue(rec.Metadata, "requires_worker_grounding") {
		return vtextRequiredContinuation{}, false
	}
	docID := strings.TrimSpace(in.DocID)
	baseRevisionID := strings.TrimSpace(in.BaseRevisionID)
	if docID == "" || baseRevisionID == "" {
		return vtextRequiredContinuation{}, false
	}
	baseRevision, err := rt.store.GetRevision(ctx, baseRevisionID, rec.OwnerID)
	if err != nil || baseRevision.AuthorKind != types.AuthorUser {
		return vtextRequiredContinuation{}, false
	}
	grounded, err := rt.channelHasGroundedHistory(ctx, rec.OwnerID, docID, time.Time{})
	if err != nil || grounded {
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
	if prompt == "" || vtextPromptAllowsUngroundedCreativeDraft(prompt) {
		return vtextRequiredContinuation{}, false
	}
	needsResearch := vtextPromptNeedsResearchContinuation(prompt)
	needsSuper := vtextPromptNeedsSuperExecution(prompt)
	if needsResearch && (!needsSuper || vtextPromptExplicitlyAsksResearchFirst(prompt)) {
		return vtextRequiredContinuation{
			Tool: "spawn_agent",
			Args: map[string]any{
				"role":       AgentProfileResearcher,
				"channel_id": docID,
				"objective":  buildVTextResearchContinuationObjective(prompt),
			},
			Instruction: "The first VText revision is now stored, but this VText run is not complete yet. Call spawn_agent next with the provided researcher arguments before ending this run; stopping now leaves the required continuation unsatisfied and fails the run. Do not call edit_vtext again in this revision run. Do not say a researcher was dispatched unless this tool call succeeds.",
		}, true
	}
	if needsSuper {
		return vtextRequiredContinuation{
			Tool: "request_super_execution",
			Args: map[string]any{
				"channel_id": docID,
				"objective":  buildVTextSuperContinuationObjective(prompt),
			},
			Instruction: "The first VText revision is now stored, but this VText run is not complete yet. Call request_super_execution next with the provided arguments before ending this run; stopping now leaves the required continuation unsatisfied and fails the run. Do not call edit_vtext again in this revision run. Do not claim work is underway unless this tool call succeeds.",
		}, true
	}
	if needsResearch {
		return vtextRequiredContinuation{
			Tool: "spawn_agent",
			Args: map[string]any{
				"role":       AgentProfileResearcher,
				"channel_id": docID,
				"objective":  buildVTextResearchContinuationObjective(prompt),
			},
			Instruction: "The first VText revision is now stored, but this VText run is not complete yet. Call spawn_agent next with the provided researcher arguments before ending this run; stopping now leaves the required continuation unsatisfied and fails the run. Do not call edit_vtext again in this revision run. Do not say a researcher was dispatched unless this tool call succeeds.",
		}, true
	}
	return vtextRequiredContinuation{}, false
}

func vtextPromptExplicitlyAsksResearchFirst(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	for _, marker := range []string{"research", "look up", "search", "cite", "citation", "sources"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func vtextPromptNeedsResearchContinuation(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	researchMarkers := []string{
		"latest",
		"current",
		"now",
		"today",
		"yesterday",
		"news",
		"update",
		"what's up",
		"whats up",
		"what is going on",
		"what's going on",
		"last night",
		"recap",
		"research",
		"look up",
		"search",
		"cite",
		"citation",
		"source",
		"sources",
		"weather",
		"score",
		"scores",
		"baseball",
		"nba",
		"nfl",
		"mlb",
		"nhl",
	}
	for _, marker := range researchMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func buildVTextResearchContinuationObjective(prompt string) string {
	return strings.TrimSpace(fmt.Sprintf(`Research the user's request for VText.

Temporal grounding:
- Current UTC date/time at delegation: %s.
- For relative-date requests such as today, tonight, yesterday, last night, latest, current, or now, anchor search queries and findings to this date/time.
- If the user's locale or sport timezone is uncertain, state that uncertainty instead of silently choosing a stale date range.

First checkpoint protocol:
- Run exactly one web_search call before the first submit_coagent_update call. If the target URL is already known, you may run that one web_search plus one targeted fetch_url; otherwise do not issue parallel search calls before the first update.
- As soon as you have 2-4 grounded facts or a precise blocker, call submit_coagent_update with kind="findings".
- The first packet is a usable checkpoint, not a final report. Keep it concise and evidence-backed.
- If you do not yet have durable evidence excerpts, omit the evidence array rather than sending malformed evidence; findings and notes are enough for an early checkpoint.
- If research discovers that another role is needed, include a typed capability_requests entry instead of trying to exercise that capability yourself.
- For live scores, schedules, current rankings, weather, or similar time-sensitive lookups, make one authoritative date-specific pass first: use the current date above, name the target date/timezone uncertainty, prefer official league/event/source pages or established scoreboards, and report whether you found final results, only matchups, or a precise blocker.
- For sports/current-score work, do not treat blocked HTML scoreboard pages as terminal by themselves. If official scoreboard fetches block, look for accessible structured league endpoints, boxscore APIs, static JSON, reputable recap pages, or established scoreboard snippets. Clearly label whether each score is verified final, live/pending, scheduled, or snippet-only.
- After the first packet, continue only if the next pass is likely to materially change the document. Before each additional search/fetch batch, know the missing question it answers; after that batch, call submit_coagent_update again with the new material cluster or blocker before continuing.

User request: %s`, time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(prompt)))
}

func buildVTextSuperContinuationObjective(prompt string) string {
	return strings.TrimSpace(fmt.Sprintf(`Execute or coordinate the user's request, send significant progress back to VText, and preserve concrete evidence for the next document revision.

Reporting contract:
- For bounded command/API/scratch work, run the requested work directly when safe.
- Run each side-effectful command or tool payload at most once per model response; do not emit duplicate same-turn bash calls in parallel. Wait for the first result, then report it.
- After any command result, call submit_coagent_update with kind="evidence" or kind="findings" before ending the run.
- If the command fails, still call submit_coagent_update with the command, exit/error summary, stdout/stderr if available, and the precise blocker.
- Do not leave VText to infer evidence from your local bash result; VText only consumes addressed coagent updates.

User request: %s`, strings.TrimSpace(prompt)))
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
			superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
			if err != nil {
				return "", err
			}
			if channelID == "" {
				channelID = superAgent.ChannelID
			}
			if model := strings.TrimSpace(in.Model); model != "" {
				objective += "\n\nRequested model: " + model
			}
			rt.superRequestMu.Lock()
			defer rt.superRequestMu.Unlock()
			if existing, ok, err := rt.findExistingSuperExecutionRequest(ctx, ownerID, channelID, superAgent.AgentID, requesterRunID, requesterAgentID); err != nil {
				return "", err
			} else if ok {
				superRun, err := rt.reconcilePersistentSuperActor(context.Background(), ownerID, superAgent.AgentID)
				if err != nil {
					return "", err
				}
				loopID := ""
				state := ""
				if superRun != nil {
					loopID = superRun.RunID
					state = string(superRun.State)
				}
				result := map[string]any{
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
				}
				return toolResultJSON(result)
			}
			cursor, err := rt.ChannelCast(ctx, channelID, superAgent.AgentID, "", requesterAgentID, AgentProfileVText, objective)
			if err != nil {
				return "", err
			}
			superRun, err := rt.reconcilePersistentSuperActor(context.Background(), ownerID, superAgent.AgentID)
			if err != nil {
				return "", err
			}
			loopID := ""
			state := ""
			if superRun != nil {
				loopID = superRun.RunID
				state = string(superRun.State)
			}
			result := map[string]any{
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
			}
			return toolResultJSON(result)
		},
	}
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
	if metadataBoolValue(rec.Metadata, "requires_worker_grounding") {
		grounded, err := rt.channelHasGroundedHistory(ctx, rec.OwnerID, docID, time.Time{})
		if err != nil {
			return types.Revision{}, fmt.Errorf("check worker grounding: %w", err)
		}
		if !grounded {
			return types.Revision{}, fmt.Errorf("edit_vtext requires worker grounding for this document seed; request researcher or super work first")
		}
	}

	materialized, err := materializeVTextToolEdit(in, currentRevision)
	if err != nil {
		return types.Revision{}, err
	}
	revMeta := addVTextEditRevisionMetadata(rt.buildAppagentRevisionMetadata(ctx, rec, doc, rec.OwnerID, mutation), materialized)
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

	rt.emitVTextDocumentRevisionEventForRun(ctx, rec, rev)
	completedPayload, _ := json.Marshal(map[string]string{
		"doc_id":      docID,
		"revision_id": rev.RevisionID,
		"loop_id":     rec.RunID,
	})
	rt.emitVTextAgentEvent(ctx, rec, types.EventVTextAgentRevisionCompleted,
		events.CauseToolExecution, completedPayload)
	return rev, nil
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

func addVTextEditRevisionMetadata(raw json.RawMessage, edit materializedVTextEdit) json.RawMessage {
	meta := decodeRevisionMetadata(raw)
	meta["source"] = "edit_vtext"
	meta["vtext_edit_kind"] = "vtext_edit"
	meta["vtext_edit_operation"] = edit.Operation
	meta["vtext_edit_base_revision_id"] = edit.BaseRevisionID
	meta["vtext_edit_count"] = edit.EditCount
	data, err := json.Marshal(meta)
	if err != nil {
		return raw
	}
	return data
}
