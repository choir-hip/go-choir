package textureowner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// texture_cell_author.go — the full-RLM texture authoring commit path (R3d).
// The texture desk's cell stages a texture intent via choir.ApplyTexture /
// the texture op verbs; the desk-cell reducer calls CommitCellTextureAuthor
// (the agentcore.TextureCellAuthorizer seam) with the run record and the
// staged body. This dispatches on op and commits through the SAME atomic
// ApplyTextureTurn / lifecycle commit the retired typed tools used — the
// genuine authoring turn, not a projection and not a parallel write path.

// cellTextureAuthorArgs is the staged texture act body the cell authors. op
// selects the turn: "apply" authors a revision (the patch_texture/rewrite
// successor), "decide" records a non-revision decision turn
// (record_texture_decision successor), "email" requests an Email appagent
// draft handoff (request_email_draft successor).
type cellTextureAuthorArgs struct {
	Op             string `json:"op"`
	DocID          string `json:"doc_id,omitempty"`
	BaseRevisionID string `json:"base_revision_id,omitempty"`
	// apply fields — one of content (full replace) or edits (structured).
	Operation        string                  `json:"operation,omitempty"`
	Content          string                  `json:"content,omitempty"`
	StructuredEdits  []textureStructuredEdit `json:"edits,omitempty"`
	Rationale        string                  `json:"rationale,omitempty"`
	WorkDisposition  string                  `json:"work_disposition,omitempty"`
	UpdateDispositions []textureUpdateDisposition `json:"update_dispositions,omitempty"`
	Controls         []textureControlArgs    `json:"controls,omitempty"`
	// decide fields.
	DecisionKind string   `json:"decision_kind,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	NextAction   string   `json:"next_action,omitempty"`
	// email fields — forwarded verbatim to the Email appagent handoff.
	Email json.RawMessage `json:"email,omitempty"`
}

// CommitCellTextureAuthor implements agentcore.TextureCellAuthorizer. bodyJSON
// is the staged texture act; revisionIdentity is the deterministic per-cell
// idempotency identity the reducer derives (it substitutes for the retired
// provider tool_call_id so a replayed cell replays the same commit).
// Returns a JSON receipt describing the commit.
func (h *Handler) CommitCellTextureAuthor(ctx context.Context, rec *types.RunRecord, bodyJSON string, revisionIdentity string) (string, error) {
	if h == nil || h.Store == nil {
		return "", fmt.Errorf("texture cell author: runtime store unavailable")
	}
	if rec == nil {
		return "", fmt.Errorf("texture cell author: missing run record")
	}
	// A replayed cell re-derives the same identity: return the receipt the
	// committed turn already recorded on the durable run tape instead of
	// re-committing — ApplyTextureTurn is head-pinned, so a second dispatch
	// would CAS-conflict on the head the first commit advanced.
	if receipt, ok := h.cellAuthorReceipt(ctx, rec, revisionIdentity); ok {
		return receipt, nil
	}
	var in cellTextureAuthorArgs
	dec := json.NewDecoder(strings.NewReader(bodyJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return "", fmt.Errorf("texture cell author: decode staged body: %w", err)
	}
	var receipt string
	var err error
	switch strings.TrimSpace(in.Op) {
	case "apply":
		receipt, err = h.commitCellTextureApply(ctx, rec, in, revisionIdentity)
	case "decide":
		receipt, err = h.commitCellTextureDecide(ctx, rec, in, revisionIdentity)
	case "email":
		receipt, err = h.commitCellTextureEmail(ctx, rec, in, revisionIdentity)
	default:
		return "", fmt.Errorf("texture cell author: unknown op %q (want apply|decide|email)", in.Op)
	}
	if err != nil {
		return "", err
	}
	h.recordCellAuthorReceipt(ctx, rec, revisionIdentity, receipt)
	return receipt, nil
}

// cellAuthorReceiptKind marks the durable receipt a committed cell-author turn
// leaves on the run tape. It is the replay identity for a re-reduced cell and
// the restart-durable record that the authoring episode completed.
const cellAuthorReceiptKind = types.RunMemoryEntryKind("texture_cell_author_receipt")

func (h *Handler) cellAuthorReceipt(ctx context.Context, rec *types.RunRecord, revisionIdentity string) (string, bool) {
	if strings.TrimSpace(revisionIdentity) == "" {
		return "", false
	}
	entries, err := h.Store.ListRunMemoryEntries(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.Kind != cellAuthorReceiptKind {
			continue
		}
		if metadataStringValue(e.Details, "intent_identity") != revisionIdentity {
			continue
		}
		if receipt := metadataStringValue(e.Details, "receipt"); receipt != "" {
			return receipt, true
		}
	}
	return "", false
}

func (h *Handler) recordCellAuthorReceipt(ctx context.Context, rec *types.RunRecord, revisionIdentity, receipt string) {
	if strings.TrimSpace(revisionIdentity) == "" || strings.TrimSpace(receipt) == "" {
		return
	}
	_, _ = h.Store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:   rec.RunID,
		OwnerID: rec.OwnerID,
		Kind:    cellAuthorReceiptKind,
		Summary: "texture cell authored turn committed",
		Details: map[string]any{"intent_identity": revisionIdentity, "receipt": receipt},
	})
}

// commitCellTextureApply authors a doc revision: structured edits or a full
// content replace committed as an AuthorAppAgent revision through
// applyTextureLifecycleTurn — the genuine authoring turn.
func (h *Handler) commitCellTextureApply(ctx context.Context, rec *types.RunRecord, in cellTextureAuthorArgs, revisionIdentity string) (string, error) {
	edit := editTextureArgs{
		DocID:              strings.TrimSpace(in.DocID),
		BaseRevisionID:     strings.TrimSpace(in.BaseRevisionID),
		Operation:          strings.TrimSpace(in.Operation),
		Content:            in.Content,
		StructuredEdits:    in.StructuredEdits,
		UpdateDispositions: in.UpdateDispositions,
		Controls:           in.Controls,
		Rationale:          in.Rationale,
		WorkDisposition:    strings.TrimSpace(in.WorkDisposition),
		ToolCallID:         strings.TrimSpace(revisionIdentity),
	}
	rev, err := h.commitTextureToolEdit(ctx, rec, edit)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]any{
		"op": "apply", "doc_id": rev.DocID, "revision_id": rev.RevisionID,
		"base_revision_id": rev.ParentRevisionID, "trajectory_id": rev.TrajectoryID,
	})
	return string(out), nil
}

// commitCellTextureDecide records a non-revision decision turn
// (record_texture_decision successor) through the same atomic lifecycle turn.
func (h *Handler) commitCellTextureDecide(ctx context.Context, rec *types.RunRecord, in cellTextureAuthorArgs, revisionIdentity string) (string, error) {
	dec := recordTextureDecisionArgs{
		DocID:              strings.TrimSpace(in.DocID),
		BaseRevisionID:     strings.TrimSpace(in.BaseRevisionID),
		DecisionKind:       strings.TrimSpace(in.DecisionKind),
		Reason:             strings.TrimSpace(in.Reason),
		EvidenceRefs:       in.EvidenceRefs,
		NextAction:         strings.TrimSpace(in.NextAction),
		UpdateDispositions: in.UpdateDispositions,
		Controls:           in.Controls,
	}
	result, err := h.commitTextureNonRevisionTurn(ctx, rec, dec, revisionIdentity)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]any{
		"op": "decide", "doc_id": strings.TrimSpace(in.DocID), "decision_kind": dec.DecisionKind,
		"outcome": result.TextureTurn.Outcome, "head_revision_id": result.TextureTurn.HeadRevisionID,
		"command_id": result.Receipt.CommandID, "replay": result.Replay,
	})
	return string(out), nil
}

// commitCellTextureEmail forwards an Email appagent draft handoff
// (request_email_draft successor).
func (h *Handler) commitCellTextureEmail(ctx context.Context, rec *types.RunRecord, in cellTextureAuthorArgs, revisionIdentity string) (string, error) {
	if h.Core == nil {
		return "", fmt.Errorf("texture cell author: core runtime unavailable")
	}
	var draft agentcore.TextureEmailDraftRequest
	if len(in.Email) == 0 {
		return "", fmt.Errorf("texture cell author: email op requires an email body")
	}
	if err := json.Unmarshal(in.Email, &draft); err != nil {
		return "", fmt.Errorf("texture cell author: decode email body: %w", err)
	}
	result, err := h.Core.RecordTextureEmailDraftRequest(ctx, rec, draft)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]any{"op": "email", "result": result})
	return string(out), nil
}
