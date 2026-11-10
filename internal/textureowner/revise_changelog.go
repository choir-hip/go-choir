package textureowner

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The local changelog facts Texture's commit points emit (contract 3,
// docs/design/autopaper-step0-contracts-2026-11-10.md 5.2 and 5.5). Chosen
// emit points, none on the owner's body-save path (a keystroke-level save adds
// no store read):
//   - a desk turn's commit (applyTextureLifecycleTurn) derives the document's
//     update_started and update_finished from the records the computer already
//     has (H1's EmitUpdateEvents; a round still open yields only started);
//   - a take, a merge's request revision and a fork's first revision each
//     emit one revision_committed, because no desk turn follows a take.
//
// update_interrupted is never written here: it is derived on read from the
// closure Report the crash boot writes. Every record is a silent system
// record, so none wakes a reader.

// emitUpdateEvents derives a document's update events at a desk turn's commit.
// Best effort: the changelog is derived, so a failure is logged and the turn
// stands.
func (h *Handler) emitUpdateEvents(ctx context.Context, ownerID, computerID, docID string) {
	if h == nil || h.Core == nil || strings.TrimSpace(computerID) == "" {
		return
	}
	if err := h.Core.EmitUpdateEvents(context.WithoutCancel(ctx), ownerID, computerID, docID); err != nil {
		log.Printf("texture api: changelog update events for %s: %v", docID, err)
	}
}

// A real owner revise opens no Cast round (the wake is a revision occurrence),
// so the round derivation cannot see it. The update is bracketed by two direct
// events with deterministic ids (kind, doc, "revise", the request revision):
// update_started when the owner's request revision commits, and update_finished
// when the desk's answering revision commits. A desk that answers without a
// revision, or a restart, yields no finished event here; the interruption is
// derived on read from the closure Report, not written.
func (h *Handler) emitUpdateStarted(ctx context.Context, ownerID string, doc types.Document, requestRevisionID string) {
	h.emitReviseUpdate(ctx, ownerID, doc, types.ChangelogUpdateStarted, "An update started.", requestRevisionID, requestRevisionID)
}

// emitUpdateFinished is called at the desk's revision commit with the base it
// answered; only an owner request revision (one that carries the owner's
// direction) is an update the owner started.
func (h *Handler) emitUpdateFinished(ctx context.Context, ownerID string, doc types.Document, base types.Revision, deskRevisionID string) {
	if base.AuthorKind != types.AuthorUser || metadataString(decodeRevisionMetadata(base.Metadata), "owner_prompt") == "" {
		return
	}
	h.emitReviseUpdate(ctx, ownerID, doc, types.ChangelogUpdateFinished, "An update finished.", base.RevisionID, deskRevisionID)
}

func (h *Handler) emitReviseUpdate(ctx context.Context, ownerID string, doc types.Document, kind types.ChangelogKind, summary, requestRevisionID, tapeRevisionID string) {
	if h == nil || h.Core == nil || strings.TrimSpace(doc.ComputerID) == "" || strings.TrimSpace(requestRevisionID) == "" {
		return
	}
	body := types.CommitmentChangelog{
		EventID:    types.ChangelogEventID(kind, doc.DocID, "revise", requestRevisionID),
		Kind:       kind,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Subject:    types.ChangelogSubject{DocID: doc.DocID},
		Summary:    summary, Origin: types.ChangelogOriginTape,
		TapeRef: &types.ChangelogTapeRef{DocID: doc.DocID, RevisionID: tapeRevisionID},
	}
	if err := h.Core.EmitChangelog(context.WithoutCancel(ctx), ownerID, doc.ComputerID, body); err != nil {
		log.Printf("texture api: changelog %s for %s: %v", kind, requestRevisionID, err)
	}
}

// emitRevisionCommitted records that a take, merge or fork committed a
// revision. The event id is (kind, doc, revision), so a replay mints nothing.
func (h *Handler) emitRevisionCommitted(ctx context.Context, ownerID string, doc types.Document, revisionID string, kase ReviseCase, forked bool) {
	if h == nil || h.Core == nil || strings.TrimSpace(doc.ComputerID) == "" || strings.TrimSpace(revisionID) == "" {
		return
	}
	summary := "A published version was taken into the document."
	switch {
	case forked:
		summary = "A document was forked from a published version."
	case kase == ReviseMerge:
		summary = "A published version was brought into the document for a merge."
	}
	body := types.CommitmentChangelog{
		EventID:    types.ChangelogEventID(types.ChangelogRevisionCommitted, doc.DocID, revisionID),
		Kind:       types.ChangelogRevisionCommitted,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Subject:    types.ChangelogSubject{DocID: doc.DocID},
		Summary:    summary, Origin: types.ChangelogOriginTape,
		TapeRef: &types.ChangelogTapeRef{DocID: doc.DocID, RevisionID: revisionID},
	}
	if err := h.Core.EmitChangelog(context.WithoutCancel(ctx), ownerID, doc.ComputerID, body); err != nil {
		log.Printf("texture api: changelog revision_committed for %s: %v", revisionID, err)
	}
}
