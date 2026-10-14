package textureowner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// texture_cell_author.go — a Texture cell's turn (Cut 2 2a). The cell builds
// one yaegikernel.TextureTurn with choir.Write, Done, Wait and Reject; the
// reducer folds its dispatch in and calls CommitCellTextureAuthor (the
// agentcore.TextureCellAuthorizer seam), which commits it through the atomic
// ApplyTextureTurn transaction:
//
//   - Write: the script materializes against the revision the cell's frame
//     showed (internal/texturewrite) and commits as the next revision; with
//     Done in the same cell it completes the caller's work.
//   - Done alone completes the work at the current revision.
//   - Wait commits a wait turn; Reject and the runtime's Idle commit a turn
//     with no semantic change. Rejects name records (the reason is the
//     turn's) or leftover update packets (rejected dispositions).
//
// Texture stays the sole writer of canonical revisions: nothing else here
// mints one.

// CommitCellTextureAuthor implements agentcore.TextureCellAuthorizer. bodyJSON
// is the folded turn; revisionIdentity is the deterministic per-cell
// idempotency identity the reducer derives, so a replayed cell replays the
// same commit. Returns a JSON receipt describing the commit.
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
	var turn yaegikernel.TextureTurn
	dec := json.NewDecoder(strings.NewReader(bodyJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&turn); err != nil {
		return "", fmt.Errorf("texture cell author: decode turn: %w", err)
	}
	receipt, err := h.commitTextureTurn(ctx, rec, turn, strings.TrimSpace(revisionIdentity))
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

// textureTurnReason is the committed reason of a turn: why it was written,
// done or waited, and every rejection.
func textureTurnReason(turn yaegikernel.TextureTurn) string {
	var parts []string
	if turn.Write != nil {
		parts = append(parts, strings.TrimSpace(turn.Write.Why))
	}
	if turn.Done != "" {
		parts = append(parts, "done: "+strings.TrimSpace(turn.Done))
	}
	if turn.Wait != "" {
		parts = append(parts, "waiting: "+strings.TrimSpace(turn.Wait))
	}
	for _, r := range turn.Rejects {
		parts = append(parts, "rejected "+firstNonEmpty(r.RecordID, r.UpdateID)+": "+strings.TrimSpace(r.Why))
	}
	if turn.Idle != "" {
		parts = append(parts, strings.TrimSpace(turn.Idle))
	}
	return strings.Join(parts, "; ")
}

// textureCaptures is the capture store a Write cites from.
func (h *Handler) textureCaptures() texturewrite.CaptureSource {
	if h == nil || h.Core == nil {
		return nil
	}
	return h.Core.TextureCaptures()
}

func (h *Handler) commitTextureTurn(ctx context.Context, rec *types.RunRecord, turn yaegikernel.TextureTurn, identity string) (string, error) {
	h.textureEditMu.Lock()
	defer h.textureEditMu.Unlock()
	if turn.Write == nil && turn.Done == "" && turn.Wait == "" && len(turn.Rejects) == 0 && turn.Idle == "" {
		return "", fmt.Errorf("texture cell author: the turn has no Write, Done, Wait or Reject")
	}
	if turn.Write != nil && turn.Wait != "" {
		return "", fmt.Errorf("texture cell author: a turn either Writes or Waits")
	}
	computerID := strings.TrimSpace(rec.ComputerID)
	mutation, err := h.Store.GetAgentMutationByRun(ctx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID)
	if err != nil {
		return "", fmt.Errorf("get texture mutation: %w", err)
	}
	if mutation == nil {
		return "", fmt.Errorf("texture mutation not found for run %s", rec.RunID)
	}
	if mutation.State != "pending" {
		return "", fmt.Errorf("texture mutation is %s, not pending: this Texture actor run is no longer writable", mutation.State)
	}
	docID := strings.TrimSpace(firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID, mutation.DocID))
	if docID == "" {
		return "", fmt.Errorf("doc_id must not be empty")
	}
	if rec.ChannelID != "" && rec.ChannelID != docID {
		return "", fmt.Errorf("doc_id %q does not match texture channel %q", docID, rec.ChannelID)
	}
	if mutation.DocID != docID || mutation.OwnerID != rec.OwnerID {
		return "", fmt.Errorf("texture mutation does not match the turn's document")
	}
	subject, subjectErr := h.Store.GetAgentByScope(ctx, rec.OwnerID, computerID, rec.AgentID)
	durableLifecycle := subjectErr == nil && subject.LifecycleVersion > 0
	if metadataStringValue(rec.Metadata, "lifecycle_work_item_id") != "" && !durableLifecycle {
		return "", fmt.Errorf("get scoped lifecycle Texture subject: %w", subjectErr)
	}
	var doc types.Document
	if durableLifecycle {
		doc, err = h.Store.GetLifecycleDocument(ctx, rec.OwnerID, computerID, docID)
	} else {
		doc, err = h.Store.GetDocument(ctx, docID, rec.OwnerID)
	}
	if err != nil {
		return "", fmt.Errorf("get texture document: %w", err)
	}
	head := strings.TrimSpace(doc.CurrentRevisionID)
	if head == "" {
		return "", fmt.Errorf("document has no current revision")
	}
	base := strings.TrimSpace(turn.BaseRevisionID)
	if base == "" {
		base = head
	}
	if base != head {
		return "", fmt.Errorf("the document changed since your frame (your frame showed revision %s; the head is now %s). Nothing was committed; your next frame shows the new head, so write against it", base, head)
	}
	args := textureTurnArgs{InboxRecords: stampRecordModel(turn.InboxRecords, textureTurnModelID(ctx, rec)), InboxCursor: turn.InboxCursor, WorkDisposition: types.WorkItemOpen, ToolCallID: identity}
	if turn.Done != "" {
		args.WorkDisposition = types.WorkItemCompleted
	}
	for _, r := range turn.Rejects {
		if r.UpdateID != "" {
			args.UpdateDispositions = append(args.UpdateDispositions, textureUpdateDisposition{UpdateID: r.UpdateID, Disposition: string(types.UpdateRejected), Reason: r.Why})
		}
	}
	reason := textureTurnReason(turn)
	if !durableLifecycle && (turn.Write == nil || len(args.InboxRecords) > 0 || args.InboxCursor != nil || len(args.UpdateDispositions) > 0) {
		return "", fmt.Errorf("Texture turns without a revision, dispatch and rejections need a lifecycle Texture document")
	}
	if turn.Write == nil {
		outcome := types.TextureTurnNoSemanticChange
		if turn.Wait != "" {
			outcome = types.TextureTurnWait
		}
		if doc.TrajectoryID == "" || rec.AgentID != currentTextureAgentID(docID) {
			return "", fmt.Errorf("texture turn requires the exact lifecycle Texture caller")
		}
		result, err := h.applyTextureLifecycleTurn(ctx, rec, doc, args, outcome, types.Revision{}, store.TextureSourceGraphWriteSet{}, reason)
		if err != nil {
			return "", fmt.Errorf("apply atomic Texture turn: %w", err)
		}
		out, _ := json.Marshal(map[string]any{
			"turn": string(result.TextureTurn.Outcome), "doc_id": docID, "head_revision_id": result.TextureTurn.HeadRevisionID,
			"command_id": result.Receipt.CommandID, "replay": result.Replay, "done": turn.Done != "",
		})
		return string(out), nil
	}
	return h.commitTextureWrite(ctx, rec, doc, mutation, turn, args, reason, durableLifecycle)
}

// commitTextureWrite materializes a Write against the head and commits it.
func (h *Handler) commitTextureWrite(ctx context.Context, rec *types.RunRecord, doc types.Document, mutation *store.AgentMutation, turn yaegikernel.TextureTurn, args textureTurnArgs, reason string, durableLifecycle bool) (string, error) {
	computerID := strings.TrimSpace(rec.ComputerID)
	var baseRev types.Revision
	var err error
	if durableLifecycle {
		baseRev, err = h.Store.GetLifecycleRevision(ctx, rec.OwnerID, computerID, doc.CurrentRevisionID)
	} else {
		baseRev, err = h.Store.GetRevision(ctx, doc.CurrentRevisionID, rec.OwnerID)
	}
	if err != nil {
		return "", fmt.Errorf("get base revision: %w", err)
	}
	baseDoc, baseEntities, err := texturewrite.Base(baseRev)
	if err != nil {
		return "", fmt.Errorf("read base revision: %w", err)
	}
	result, err := texturewrite.Apply(ctx, texturewrite.Input{
		OwnerID: rec.OwnerID, DocID: doc.DocID, Seed: baseRev.RevisionID,
		Base: baseDoc, Entities: baseEntities, Script: turn.Write.Edits,
		Handles: turn.Handles, Resolver: h.textureCaptures(),
	})
	if err != nil {
		return "", fmt.Errorf("choir.Write refused, nothing committed: %w", err)
	}
	projection, err := texturedoc.Project(result.Doc, result.Entities)
	if err != nil {
		return "", fmt.Errorf("choir.Write refused, nothing committed: the result is not a valid document: %w", err)
	}
	bodyDoc, err := json.Marshal(result.Doc)
	if err != nil {
		return "", fmt.Errorf("encode Texture body: %w", err)
	}
	entities, err := json.Marshal(result.Entities)
	if err != nil {
		return "", fmt.Errorf("encode Texture source entities: %w", err)
	}
	if prior, _ := json.Marshal(baseDoc); bytes.Equal(prior, bodyDoc) {
		return "", fmt.Errorf("choir.Write refused, nothing committed: the Write changes nothing; use choir.Wait or choir.Done to close a turn without a revision")
	}
	content := strings.TrimSpace(projection.Text)
	if content == "" {
		return "", fmt.Errorf("choir.Write refused, nothing committed: the document would have no text")
	}
	if strings.TrimSpace(doc.TrajectoryID) == "" {
		if err := h.canonicalizeAliasedTextureDocumentTitle(ctx, rec.OwnerID, &doc, time.Now().UTC()); err != nil {
			return "", fmt.Errorf("canonicalize texture document title: %w", err)
		}
	}
	meta := decodeRevisionMetadata(h.buildAppagentRevisionMetadata(ctx, rec, doc, rec.OwnerID, mutation, 0))
	if meta == nil {
		meta = map[string]any{}
	}
	meta["texture_write_why"] = strings.TrimSpace(turn.Write.Why)
	if p := turn.Write.PSupported; p != nil {
		// The Write's self-score; the turn's transaction copies it onto
		// each new citation's check memo (icrl-calibration-experiment).
		meta[store.RevisionMetaPSupported] = *p
	}
	meta["texture_write_directives"] = result.Ops
	meta["texture_write_base_revision_id"] = baseRev.RevisionID
	if len(result.Dropped) > 0 {
		meta[texturewrite.DroppedMetadataKey] = result.Dropped
	}
	// Each citation's quote check rides the revision, where the reader and
	// the version overlay read it (Cut 2 2a (v)); the support pairs are
	// listed to management by the turn's transaction below.
	checks, _ := texturewrite.Checks(result.Doc, doc.DocID, "")
	if len(checks) > 0 {
		meta[texturewrite.ChecksMetadataKey] = checks
	}
	if strings.TrimSpace(doc.TrajectoryID) == "" {
		canonicalPath, err := h.ensureCanonicalTextureProjectionPath(ctx, rec.OwnerID, doc)
		if err != nil {
			return "", fmt.Errorf("ensure canonical texture projection path: %w", err)
		}
		if canonicalPath != "" {
			meta[canonicalTextureSourcePathMetadataKey] = canonicalPath
		}
	}
	rawMeta, _ := json.Marshal(meta)
	now := time.Now().UTC()
	rev := types.Revision{
		RevisionID:       uuid.NewString(),
		DocID:            doc.DocID,
		OwnerID:          rec.OwnerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		Content:          content,
		BodyDoc:          bodyDoc,
		SourceEntities:   entities,
		Citations:        json.RawMessage("[]"),
		Metadata:         sanitizeTextureToolRevisionMetadata(rawMeta),
		Provenance:       buildStructuredAppagentRevisionProvenance(rec, entities, now),
		ParentRevisionID: baseRev.RevisionID,
		CreatedAt:        now,
	}
	graph, err := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{BodyDoc: bodyDoc, SourceEntities: entities}, rec)
	if err != nil {
		return "", fmt.Errorf("build Texture source graph: %w", err)
	}
	if strings.TrimSpace(doc.TrajectoryID) != "" {
		// A turn on records has no work item (Cut 1); the caller agent and
		// run authorize it, and the store checks a named work item.
		if agentID := strings.TrimSpace(rec.AgentID); agentID == "" || agentID != currentTextureAgentID(doc.DocID) {
			return "", fmt.Errorf("create Texture revision: run agent %q does not own lifecycle subject %q", agentID, currentTextureAgentID(doc.DocID))
		}
		rev.RevisionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{
			"choir:texture:lifecycle-revision", doc.TrajectoryID, baseRev.RevisionID, objectgraph.SHA256(bodyDoc),
		}, ":"))).String()
		rev.ComputerID = strings.TrimSpace(doc.ComputerID)
		if graph, err = textureToolSourceGraphWriteSet(rev, materializedTextureEdit{BodyDoc: bodyDoc, SourceEntities: entities}, rec); err != nil {
			return "", fmt.Errorf("build Texture source graph: %w", err)
		}
		_, args.Citations = texturewrite.Checks(result.Doc, doc.DocID, rev.RevisionID)
		applied, applyErr := h.applyTextureLifecycleTurn(ctx, rec, doc, args, types.TextureTurnRevision, rev, graph, reason)
		if applyErr != nil {
			if errors.Is(applyErr, store.ErrConcurrentStateChange) || errors.Is(applyErr, store.ErrStaleDocumentHead) {
				// A late owner edit invalidates this attempt only: the run and
				// its mutation stay alive and the next frame shows the head.
				return "", fmt.Errorf("the document or its owner instructions changed while this turn committed; nothing was committed, and your next frame shows the new head: %w", applyErr)
			}
			return "", fmt.Errorf("apply atomic Texture turn: %w", applyErr)
		}
		h.recordTextureAudit(ctx, "revision_committed", rec.OwnerID, doc.ComputerID, doc.TrajectoryID, doc.DocID, rev.RevisionID, applied.Receipt.CommandID, applied.Receipt.CommandDigest, applied.Trajectory.LifecycleVersion)
		// The commit listed its new citation pairs; Jev judges them off
		// this path (support-judgement-to-jev-design-2026-10-14). The
		// hand-off returns at once; a failure reaches this Texture's inbox.
		if h.Core != nil {
			h.Core.JudgeRevisionSupport(rec.OwnerID, doc.ComputerID, rec.RunID, rev.RevisionID, args.Citations)
		}
	} else if err := h.Store.CreateRevisionWithSourceGraph(ctx, rev, graph); err != nil {
		return "", fmt.Errorf("create Texture revision: %w", err)
	}
	var stored types.Revision
	if durableLifecycle {
		stored, err = h.Store.GetLifecycleRevision(ctx, rec.OwnerID, computerID, rev.RevisionID)
	} else {
		stored, err = h.Store.GetRevision(ctx, rev.RevisionID, rec.OwnerID)
	}
	if err != nil {
		return "", fmt.Errorf("load created Texture revision: %w", err)
	}
	if !durableLifecycle {
		auditDigest := textureAuditDigest("revision_committed", rec.OwnerID, computerID, doc.DocID, stored.RevisionID, stored.RevisionHash)
		h.recordTextureAudit(ctx, "revision_committed", rec.OwnerID, computerID, stored.TrajectoryID, doc.DocID, stored.RevisionID, "texture-revision:"+doc.DocID+":"+stored.RevisionID, auditDigest, 0)
	}
	h.emitTextureDocumentRevisionEventForRun(ctx, rec, stored)
	completed, _ := json.Marshal(map[string]string{"doc_id": doc.DocID, "revision_id": stored.RevisionID, "loop_id": rec.RunID})
	h.emitTextureAgentEvent(ctx, rec, types.EventTextureAgentRevisionCompleted, events.CauseToolExecution, completed)
	// A story opened with publish auto publishes this revision as its next
	// version, after the commit and off its path (Cut 2 2b). A failed
	// publish is a gap; nothing here waits for it. A revision that cites
	// no source is held, and the turn's result says why.
	publishHeld := ""
	switch {
	case !durableLifecycle:
		log.Printf("story publish: %s revision %s not considered: the document is not a lifecycle document", doc.DocID, stored.RevisionID)
	case h.Core == nil:
		log.Printf("story publish: %s revision %s not considered: no runtime to publish through", doc.DocID, stored.RevisionID)
	default:
		publishHeld = h.Core.AutoPublishRevision(ctx, rec.OwnerID, doc, stored)
	}
	committed := map[string]any{
		"turn": string(types.TextureTurnRevision), "doc_id": stored.DocID, "revision_id": stored.RevisionID,
		"base_revision_id": stored.ParentRevisionID, "trajectory_id": stored.TrajectoryID,
		"dropped_citations": len(result.Dropped), "done": turn.Done != "",
	}
	if publishHeld != "" {
		committed["publish_held"] = "not published: " + publishHeld
	}
	out, _ := json.Marshal(committed)
	return string(out), nil
}
