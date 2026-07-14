package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// RegisterEvidenceTools installs the researcher-owned evidence-gathering tools.
// These belong to roles that do ordinary evidence collection (researcher and the
// execution roles), not to Texture's authoring/control-plane affordance.
func RegisterEvidenceTools(registry *toolregistry.ToolRegistry, rt *Runtime) error {
	for _, tool := range []toolregistry.Tool{
		newSaveEvidenceTool(rt),
		newReadEvidenceTool(rt),
		newListEvidenceTool(rt),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

// RegisterRunMemoryTools installs run-memory retrieval. get_run_memory_entry is
// exact durable run-memory retrieval after compaction, not ordinary evidence
// gathering, so any tool-looping role (including Texture) may keep it without
// receiving researcher-owned evidence tools.
func RegisterRunMemoryTools(registry *toolregistry.ToolRegistry, rt *Runtime) error {
	return registry.Register(newGetRunMemoryEntryTool(rt))
}

// RegisterModelDiagnosticTools installs provider/model diagnostic verifiers.
// verify_model_capability is a model diagnostic and does not belong in Texture's
// default authoring affordance.
func RegisterModelDiagnosticTools(registry *toolregistry.ToolRegistry, rt *Runtime) error {
	return registry.Register(newVerifyModelCapabilityTool(rt))
}

func newGetRunMemoryEntryTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		EntryID string `json:"entry_id"`
	}
	return toolregistry.Tool{Name: "get_run_memory_entry",
		Description: "Retrieve an exact durable run-memory message or compaction by entry_id when a checkpoint summary is insufficient.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"entry_id": map[string]any{"type": "string"},
		}, []string{"entry_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode get_run_memory_entry args: %w", err)
			}
			ownerID := toolregistry.ExecutionContextFrom(ctx).OwnerID
			if ownerID == "" {
				return "", fmt.Errorf("get_run_memory_entry missing owner context")
			}
			entryID := strings.TrimSpace(in.EntryID)
			if entryID == "" {
				return "", fmt.Errorf("entry_id must not be empty")
			}
			entry, err := rt.store.GetRunMemoryEntry(ctx, ownerID, entryID)
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{
				"entry_id":            entry.EntryID,
				"loop_id":             entry.RunID,
				"seq":                 entry.Seq,
				"kind":                entry.Kind,
				"role":                entry.Role,
				"message":             string(entry.Message),
				"summary":             entry.Summary,
				"first_kept_entry_id": entry.FirstKeptEntryID,
				"tokens_before":       entry.TokensBefore,
				"reason":              entry.Reason,
				"details":             entry.Details,
				"created_at":          entry.CreatedAt.Format(time.RFC3339Nano),
			})
		}}
}

func newSaveEvidenceTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		Kind      string          `json:"kind"`
		SourceURI string          `json:"source_uri,omitempty"`
		Title     string          `json:"title,omitempty"`
		Content   string          `json:"content"`
		Metadata  json.RawMessage `json:"metadata,omitempty"`
	}
	return toolregistry.Tool{Name: "save_evidence",
		Description: "Persist retrieved or evidentiary material into the user's embedded Dolt workspace.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"kind":       map[string]any{"type": "string"},
			"source_uri": map[string]any{"type": "string"},
			"title":      map[string]any{"type": "string"},
			"content":    map[string]any{"type": "string"},
			"metadata":   map[string]any{"type": "object"},
		}, []string{"kind", "content"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode save_evidence args: %w", err)
			}
			ownerID := toolregistry.ExecutionContextFrom(ctx).OwnerID
			agentID := toolregistry.ExecutionContextFrom(ctx).AgentID
			if ownerID == "" || agentID == "" {
				return "", fmt.Errorf("save_evidence missing owner or agent context")
			}
			kind := strings.TrimSpace(in.Kind)
			if kind == "" {
				return "", fmt.Errorf("kind must not be empty")
			}
			content := strings.TrimSpace(in.Content)
			if content == "" {
				return "", fmt.Errorf("content must not be empty")
			}
			rec := types.EvidenceRecord{
				EvidenceID: uuid.NewString(),
				OwnerID:    ownerID,
				AgentID:    agentID,
				Kind:       kind,
				SourceURI:  strings.TrimSpace(in.SourceURI),
				Title:      strings.TrimSpace(in.Title),
				Content:    in.Content,
				Metadata:   in.Metadata,
				CreatedAt:  time.Now().UTC(),
			}
			if err := rt.store.CreateEvidence(ctx, rec); err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{
				"evidence_id": rec.EvidenceID,
				"owner_id":    rec.OwnerID,
				"agent_id":    rec.AgentID,
				"kind":        rec.Kind,
				"source_uri":  rec.SourceURI,
				"title":       rec.Title,
				"created_at":  rec.CreatedAt.Format(time.RFC3339Nano),
			})
		}}
}

func newReadEvidenceTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		EvidenceID string `json:"evidence_id"`
	}
	return toolregistry.Tool{Name: "read_evidence",
		Description: "Read a saved evidence record from embedded Dolt.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"evidence_id": map[string]any{"type": "string"},
		}, []string{"evidence_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode read_evidence args: %w", err)
			}
			ownerID := toolregistry.ExecutionContextFrom(ctx).OwnerID
			if ownerID == "" {
				return "", fmt.Errorf("read_evidence missing owner context")
			}
			rec, err := rt.store.GetEvidence(ctx, strings.TrimSpace(in.EvidenceID), ownerID)
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{
				"evidence_id": rec.EvidenceID,
				"owner_id":    rec.OwnerID,
				"agent_id":    rec.AgentID,
				"kind":        rec.Kind,
				"source_uri":  rec.SourceURI,
				"title":       rec.Title,
				"content":     rec.Content,
				"metadata":    rec.Metadata,
				"created_at":  rec.CreatedAt.Format(time.RFC3339Nano),
			})
		}}
}

func newListEvidenceTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		AgentID string `json:"agent_id,omitempty"`
		Limit   int    `json:"limit,omitempty"`
	}
	return toolregistry.Tool{Name: "list_evidence",
		Description: "List recent saved evidence records for an agent or owner scope.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"agent_id": map[string]any{"type": "string"},
			"limit":    map[string]any{"type": "integer", "minimum": 1},
		}, nil, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode list_evidence args: %w", err)
			}
			ownerID := toolregistry.ExecutionContextFrom(ctx).OwnerID
			if ownerID == "" {
				return "", fmt.Errorf("list_evidence missing owner context")
			}
			agentID := strings.TrimSpace(in.AgentID)
			if agentID == "" {
				agentID = toolregistry.ExecutionContextFrom(ctx).AgentID
			}
			recs, err := rt.store.ListEvidenceByAgent(ctx, ownerID, agentID, in.Limit)
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(recs))
			for _, rec := range recs {
				items = append(items, map[string]any{
					"evidence_id": rec.EvidenceID,
					"agent_id":    rec.AgentID,
					"kind":        rec.Kind,
					"source_uri":  rec.SourceURI,
					"title":       rec.Title,
					"created_at":  rec.CreatedAt.Format(time.RFC3339Nano),
				})
			}
			return toolregistry.ResultJSON(map[string]any{
				"agent_id": agentID,
				"items":    items,
			})
		}}
}
