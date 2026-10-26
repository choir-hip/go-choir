package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Texture's frame (Cut 2 2a): each turn Texture reads its document as
// aliased blocks and the records past its cursor, injected as a runtime
// user turn whenever either changed since the last frame the run saw. A
// cell then works against that frame: its ReadDoc snapshot is the frame's
// revision, a Write applies to it, and the cell's cursor advance stops at
// the last record the frame showed, so no record is marked read unseen.

// textureFrameMark is the last frame a run saw.
type textureFrameMark struct {
	revisionID string
	through    int64
	ok         bool
}

// textureFrameMarkFromMemory finds the last frame turn the runtime injected
// into a run. Only runtime injections count, so a model cannot forge one.
func textureFrameMarkFromMemory(entries []types.RunMemoryEntry) textureFrameMark {
	var mark textureFrameMark
	for _, entry := range entries {
		if entry.Kind != types.RunMemoryEntryMessage || entry.Role != types.RunMemoryRoleRuntimeInjection || len(entry.Message) == 0 {
			continue
		}
		for _, text := range runMemoryUserMessageTexts(entry.Message) {
			if !strings.HasPrefix(text, texturewrite.FrameHeader+"\n") {
				continue
			}
			line, _, _ := strings.Cut(strings.TrimPrefix(text, texturewrite.FrameHeader+"\n"), "\n")
			var doc, rev string
			var through int64
			if _, err := fmt.Sscanf(line, "doc=%s rev=%s through=%d", &doc, &rev, &through); err == nil {
				mark = textureFrameMark{revisionID: rev, through: through, ok: true}
			}
		}
	}
	return mark
}

func (rt *Runtime) textureFrameMarkForRun(ctx context.Context, rec *types.RunRecord) textureFrameMark {
	if rt == nil || rt.store == nil || rec == nil {
		return textureFrameMark{}
	}
	entries, err := rt.store.ListRunMemoryEntries(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		return textureFrameMark{}
	}
	return textureFrameMarkFromMemory(entries)
}

// textureRunRevision loads a Texture run's document and one revision of it
// (the head when revisionID is empty), plus the head's parent.
func (rt *Runtime) textureRunRevision(ctx context.Context, rec *types.RunRecord, revisionID string) (types.Document, types.Revision, *types.Revision, error) {
	docID := strings.TrimSpace(metadataStringValue(rec.Metadata, "doc_id"))
	if docID == "" {
		docID = strings.TrimSpace(rec.ChannelID)
	}
	if docID == "" {
		return types.Document{}, types.Revision{}, nil, fmt.Errorf("texture frame: run %s has no document", rec.RunID)
	}
	ownerID, computerID := strings.TrimSpace(rec.OwnerID), strings.TrimSpace(rec.ComputerID)
	var doc types.Document
	var err error
	if subject, subErr := rt.store.GetAgentByScope(ctx, ownerID, computerID, rec.AgentID); subErr == nil && subject.LifecycleVersion > 0 {
		doc, err = rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	} else {
		doc, err = rt.store.GetDocument(ctx, docID, ownerID)
	}
	if err != nil {
		return types.Document{}, types.Revision{}, nil, err
	}
	if strings.TrimSpace(revisionID) == "" {
		revisionID = doc.CurrentRevisionID
	}
	if strings.TrimSpace(revisionID) == "" {
		return types.Document{}, types.Revision{}, nil, fmt.Errorf("texture frame: document %s has no revision", docID)
	}
	get := func(id string) (types.Revision, error) {
		if strings.TrimSpace(doc.TrajectoryID) != "" {
			return rt.store.GetLifecycleRevision(ctx, ownerID, computerID, id)
		}
		return rt.store.GetRevision(ctx, id, ownerID)
	}
	rev, err := get(revisionID)
	if err != nil {
		return types.Document{}, types.Revision{}, nil, err
	}
	var parent *types.Revision
	if id := strings.TrimSpace(rev.ParentRevisionID); id != "" {
		if p, perr := get(id); perr == nil {
			parent = &p
		}
	}
	return doc, rev, parent, nil
}

// textureRecordInputs converts a cell's record view for the frame.
func textureRecordInputs(view []yaegikernel.InboxRecord) []texturewrite.RecordInput {
	out := make([]texturewrite.RecordInput, 0, len(view))
	for _, r := range view {
		out = append(out, texturewrite.RecordInput{
			Seq: r.Seq, RecordID: r.RecordID, Kind: r.Kind, From: r.From, Round: r.Round, Final: r.Final,
			Objective: r.Objective, Body: r.Body, Cast: r.Cast, Report: r.Report, Support: r.Support,
		})
	}
	return out
}

// currentTextureFrame is a run's frame now: the head and the records past
// the cursor.
func (rt *Runtime) currentTextureFrame(ctx context.Context, rec *types.RunRecord) (texturewrite.Frame, error) {
	doc, rev, parent, err := rt.textureRunRevision(ctx, rec, "")
	if err != nil {
		return texturewrite.Frame{}, err
	}
	base, _, err := texturewrite.Base(rev)
	if err != nil {
		return texturewrite.Frame{}, err
	}
	records, err := rt.loadCellRecords(ctx, rec.OwnerID, rec.ComputerID, strings.TrimSpace(rec.AgentID), "")
	if err != nil {
		return texturewrite.Frame{}, err
	}
	frame := texturewrite.Frame{
		DocID: doc.DocID, RevisionID: rev.RevisionID, Blocks: texturewrite.DocBlocks(base),
		Dropped: texturewrite.DroppedFrom(rev), Records: texturewrite.RecordsFrom(textureRecordInputs(records.view)),
		ThroughSeq: records.through,
	}
	if id, pending, perr := rt.store.PendingStoryResearchRequest(ctx, rec.OwnerID, rec.ComputerID, doc.DocID); perr == nil && pending {
		frame.ReaderRequest = id
	}
	if parent != nil {
		if prior, _, perr := texturewrite.Base(*parent); perr == nil {
			frame.Changed = texturewrite.ChangedBlocks(base, prior)
		}
	}
	if captures := rt.TextureCaptures(); captures != nil {
		frame.Captured = map[string]bool{}
		for handle, src := range texturewrite.Handles(frame.Records) {
			if _, cerr := captures.CaptureBlocks(ctx, rec.OwnerID, src.Key()); cerr == nil {
				frame.Captured[handle] = true
			}
		}
	}
	return frame, nil
}

// textureFrameInjector injects the frame before a Texture run's next model
// call whenever its revision or its records changed since the last frame
// the run saw.
func (rt *Runtime) textureFrameInjector(rec *types.RunRecord) toolregistry.InjectUserTurnsFunc {
	if rt == nil || rt.store == nil || rec == nil || !runHasProfile(rec, agentprofile.Texture) {
		return nil
	}
	return func(bool) ([]json.RawMessage, error) {
		ctx := context.Background()
		frame, err := rt.currentTextureFrame(ctx, rec)
		if err != nil {
			// A run without a readable document has no frame; the turn
			// still runs on its prompt.
			return nil, nil
		}
		mark := rt.textureFrameMarkForRun(ctx, rec)
		if mark.ok && mark.revisionID == frame.RevisionID && mark.through == frame.ThroughSeq {
			return nil, nil
		}
		msg, err := json.Marshal(map[string]any{
			"role":    "user",
			"content": []any{map[string]string{"type": "text", "text": frame.Render()}},
		})
		if err != nil {
			return nil, err
		}
		return []json.RawMessage{msg}, nil
	}
}

// composeInjectors runs each injector in order and joins their turns.
func composeInjectors(injectors ...toolregistry.InjectUserTurnsFunc) toolregistry.InjectUserTurnsFunc {
	var live []toolregistry.InjectUserTurnsFunc
	for _, inject := range injectors {
		if inject != nil {
			live = append(live, inject)
		}
	}
	switch len(live) {
	case 0:
		return nil
	case 1:
		return live[0]
	}
	return func(final bool) ([]json.RawMessage, error) {
		var out []json.RawMessage
		for _, inject := range live {
			turns, err := inject(final)
			if err != nil {
				return nil, err
			}
			out = append(out, turns...)
		}
		return out, nil
	}
}

// capToTextureFrame limits a Texture cell's records to those the run's last
// frame showed.
func capToTextureFrame(records cellRecords, mark textureFrameMark) cellRecords {
	if !mark.ok || mark.through >= records.through {
		return records
	}
	limit := mark.through
	if limit < records.after {
		limit = records.after
	}
	kept := records.view[:0:0]
	for _, r := range records.view {
		if r.Seq <= limit {
			kept = append(kept, r)
		}
	}
	records.view, records.through = kept, limit
	return records
}

// textureDocSnapshotForRun is a Texture cell's document: the revision the
// run's last frame showed (the head when no frame was shown), its blocks by
// alias, and the frame's record sources by handle.
func textureDocSnapshotForRun(ctx context.Context, rt *Runtime, rec *types.RunRecord, records cellRecords) *yaegikernel.DocSnapshot {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	mark := rt.textureFrameMarkForRun(ctx, rec)
	doc, rev, _, err := rt.textureRunRevision(ctx, rec, mark.revisionID)
	if err != nil && mark.ok {
		doc, rev, _, err = rt.textureRunRevision(ctx, rec, "")
	}
	if err != nil {
		return nil
	}
	base, _, err := texturewrite.Base(rev)
	if err != nil {
		base = texturedoc.StructuredTextureDoc{}
	}
	snapshot := &yaegikernel.DocSnapshot{
		DocID: doc.DocID, RevisionID: rev.RevisionID, AuthorKind: string(rev.AuthorKind), Content: rev.Content,
		Blocks: texturewrite.DocBlocks(base),
	}
	snapshot.OwnerRevision = rt.textureRunOwnerRevision(ctx, rec, doc, rev)

	for _, r := range texturewrite.RecordsFrom(textureRecordInputs(records.view)) {
		snapshot.Sources = append(snapshot.Sources, r.Sources...)
	}
	return snapshot
}

// textureRunOwnerRevision is the revision that woke the run when the owner
// wrote it: a Cast names it and the reducer attaches the owner's words
// (texture_owner_directions_cast.go). It trusts the revision, not the run's
// current_author_kind (review 272 F1): the revision the run names is read
// (the frame's own when it is that one) and must be this document's and
// owner-written.
func (rt *Runtime) textureRunOwnerRevision(ctx context.Context, rec *types.RunRecord, doc types.Document, framed types.Revision) string {
	id := strings.TrimSpace(metadataStringValue(rec.Metadata, "current_revision_id"))
	if id == "" {
		return ""
	}
	current := framed
	if strings.TrimSpace(framed.RevisionID) != id {
		var err error
		if current, err = rt.store.GetLifecycleRevision(ctx, strings.TrimSpace(rec.OwnerID), strings.TrimSpace(rec.ComputerID), id); err != nil {
			return ""
		}
	}
	if strings.TrimSpace(current.RevisionID) != id || strings.TrimSpace(current.DocID) != strings.TrimSpace(doc.DocID) || current.AuthorKind != types.AuthorUser {
		return ""
	}
	return id
}
