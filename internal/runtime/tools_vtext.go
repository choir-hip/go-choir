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
			return toolResultJSON(map[string]any{
				"doc_id":           rev.DocID,
				"revision_id":      rev.RevisionID,
				"base_revision_id": rev.ParentRevisionID,
				"status":           "stored",
			})
		},
	}
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
			return toolResultJSON(map[string]any{
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
			})
		},
	}
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

	content = strings.TrimSpace(content)
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
