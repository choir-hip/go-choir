package textureowner

// What the Texture root has to read, and what it has read. One invariant:
// a change is either in the ledger of a committed root revision, or still
// pending; never neither.
//
//   - The cut-off is taken BEFORE the first read of the run (textureRootScan,
//     called from the request section that builds the ledger), and it is the
//     run's watermark: carried in the run's metadata and written into the
//     revision the run commits, never the run's creation time, which comes
//     after the reads.
//   - A change is stamped before it is visible, so the next digest looks
//     textureRootOverlap behind the watermark; what it finds there that the
//     last root revision recorded as read at the same version mark (the
//     document's head revision and its trajectory's version and reducer
//     sequence) is not pending, and the rest is.
//   - The watermark never passes a document the run did not read: when more
//     than the bound changed, the oldest changes are read first and the
//     watermark stops at the first one left; a document that could not be
//     read holds it too, and is named.
//   - A run that commits no revision records nothing.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// textureRootMark is one document as a root revision read it.
type textureRootMark struct {
	At   string `json:"at"` // the change time that put it in the scan
	Mark string `json:"m"`  // head revision and trajectory version
}

// textureRootState is what the master's committed root revision recorded.
type textureRootState struct {
	Through    time.Time
	Docs       map[string]textureRootMark
	RevisionAt time.Time // when the revision that carries it was committed
}

// textureRootMarkOf is the version a root revision records for a document: a
// change after it is a different mark, whatever the clock says.
func textureRootMarkOf(entry store.ChangedTextureDocument) string {
	return fmt.Sprintf("%s|%d.%d", entry.Doc.CurrentRevisionID, entry.Trajectory.LifecycleVersion, entry.Trajectory.ReducerSeq)
}

// textureRootStateOf reads the watermark out of revision metadata. ok is
// false when the metadata carries none; an error when it carries a malformed
// one (the caller treats that as none and says so).
func textureRootStateOf(metadata map[string]any) (state textureRootState, ok bool, err error) {
	through := metadataString(metadata, textureRootDigestedKey)
	if through == "" {
		return textureRootState{}, false, nil
	}
	at, err := time.Parse(time.RFC3339Nano, through)
	if err != nil {
		return textureRootState{}, false, fmt.Errorf("watermark %q: %w", through, err)
	}
	state = textureRootState{Through: at, Docs: map[string]textureRootMark{}}
	if raw, present := metadata[textureRootDigestedDocsKey]; present {
		encoded, err := json.Marshal(raw)
		if err == nil {
			err = json.Unmarshal(encoded, &state.Docs)
		}
		if err != nil {
			return textureRootState{}, false, fmt.Errorf("watermark documents: %w", err)
		}
	}
	return state, true, nil
}

// walkTextureRootWatermark finds the newest watermark from headID back,
// reading at most limit revisions. A malformed watermark is none, logged.
func walkTextureRootWatermark(headID string, limit int, load func(id string) (types.Revision, error)) (textureRootState, error) {
	for id, reads := strings.TrimSpace(headID), 0; id != "" && reads < limit; reads++ {
		revision, err := load(id)
		if err != nil {
			return textureRootState{}, err
		}
		state, ok, err := textureRootStateOf(decodeRevisionMetadata(revision.Metadata))
		if err != nil {
			log.Printf("texture root: revision %s carries a malformed watermark, read as none: %v", id, err)
			return textureRootState{}, nil
		}
		if ok {
			state.RevisionAt = revision.CreatedAt
			return state, nil
		}
		id = strings.TrimSpace(revision.ParentRevisionID)
	}
	return textureRootState{}, nil
}

// textureRootWatermark is the master's recorded watermark: its head revision's
// or the nearest one behind it (textureRootWatermarkWalk). Zero is none.
func (h *Handler) textureRootWatermark(ctx context.Context, ownerID, computerID string, root store.TextureRoot) (textureRootState, error) {
	ctx = asTextureReader(ctx)
	doc, err := h.Store.GetLifecycleDocument(ctx, ownerID, computerID, root.DocumentID)
	if err != nil {
		return textureRootState{}, err
	}
	return walkTextureRootWatermark(doc.CurrentRevisionID, textureRootWatermarkWalk, func(id string) (types.Revision, error) {
		return h.Store.GetLifecycleRevision(ctx, ownerID, computerID, id)
	})
}

// textureRootScan is what a digest, or any wake of the master, finds to do.
type textureRootScan struct {
	// Cutoff was taken before the scan began.
	Cutoff time.Time
	State  textureRootState
	// Read are the documents to read, oldest change first, at most the bound.
	Read []store.ChangedTextureDocument
	// Unread are documents that could not be read, named.
	Unread []store.ChangedTextureDocument
	// Left counts the changed documents past the bound (at least).
	Left int
	// firstLeftAt is the change time of the first document left.
	firstLeftAt time.Time
}

// textureRootSelect picks, from the scan's entries, the documents that are
// pending against the recorded state: unreadable ones always, the others when
// their mark differs from the one recorded. The first bound of them are read.
func textureRootSelect(entries []store.ChangedTextureDocument, state textureRootState, bound int) (read, unread []store.ChangedTextureDocument, left int, firstLeftAt time.Time) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].ChangedAt.Before(entries[j].ChangedAt) })
	for _, entry := range entries {
		if entry.Err != nil {
			unread = append(unread, entry)
			continue
		}
		if recorded, ok := state.Docs[entry.Doc.DocID]; ok && recorded.Mark == textureRootMarkOf(entry) {
			continue
		}
		if len(read) >= bound {
			if left == 0 {
				firstLeftAt = entry.ChangedAt
			}
			left++
			continue
		}
		read = append(read, entry)
	}
	return read, unread, left, firstLeftAt
}

// textureRootScan takes the cut-off, then lists the documents changed since
// the watermark less the overlap, never further back than the lookback, and
// selects. Every read error is returned: nothing is swallowed.
func (h *Handler) textureRootScan(ctx context.Context, ownerID, computerID string, root store.TextureRoot) (textureRootScan, error) {
	// The cut-off is compared with the store's own stamps, taken from the
	// wall clock, not with the digest schedule's clock (textureRootNow).
	scan := textureRootScan{Cutoff: time.Now().UTC()}
	var err error
	if scan.State, err = h.textureRootWatermark(ctx, ownerID, computerID, root); err != nil {
		return scan, fmt.Errorf("read the watermark: %w", err)
	}
	since := scan.Cutoff.Add(-textureRootLookback)
	if !scan.State.Through.IsZero() && scan.State.Through.Add(-textureRootOverlap).After(since) {
		since = scan.State.Through.Add(-textureRootOverlap)
	}
	entries, more, err := h.Store.ListTextureDocumentsChangedSince(asTextureReader(ctx), ownerID, computerID, since)
	if err != nil {
		return scan, fmt.Errorf("list the changed documents: %w", err)
	}
	masters := textureRootMasterIDs(root)
	kept := entries[:0]
	for _, entry := range entries {
		if entry.Err == nil && masters[entry.Doc.DocID] {
			continue
		}
		kept = append(kept, entry)
	}
	scan.Read, scan.Unread, scan.Left, scan.firstLeftAt = textureRootSelect(kept, scan.State, textureRootMaxDocuments)
	if more && scan.Left == 0 {
		scan.Left = 1 // the range held more than one listing: at least one is left
	}
	return scan, nil
}

// Finish computes what a committed revision of this run records: the
// watermark and the documents read. unread are the scan's and any the ledger
// reads failed on; read are the ones read.
func (s textureRootScan) Finish(read []store.ChangedTextureDocument, unread []store.ChangedTextureDocument) (through time.Time, docs map[string]textureRootMark) {
	through = s.Cutoff
	if s.Left > 0 && !s.firstLeftAt.IsZero() && s.firstLeftAt.Before(through) {
		through = s.firstLeftAt
	}
	for _, entry := range unread {
		if entry.ChangedAt.Before(through) {
			through = entry.ChangedAt
		}
	}
	docs = map[string]textureRootMark{}
	floor := through.Add(-textureRootOverlap)
	for id, mark := range s.State.Docs {
		if at, err := time.Parse(time.RFC3339Nano, mark.At); err == nil && !at.Before(floor) {
			docs[id] = mark
		}
	}
	for _, entry := range read {
		docs[entry.Doc.DocID] = textureRootMark{At: entry.ChangedAt.UTC().Format(time.RFC3339Nano), Mark: textureRootMarkOf(entry)}
	}
	if len(docs) > textureRootMaxMarks {
		ids := make([]string, 0, len(docs))
		for id := range docs {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return docs[ids[i]].At > docs[ids[j]].At })
		for _, id := range ids[textureRootMaxMarks:] {
			delete(docs, id)
		}
	}
	return through, docs
}

// withTextureRootWatermark returns revision metadata carrying the run's
// watermark keys, copied from the run's metadata (nothing is written for a
// run that carries none).
func withTextureRootWatermark(raw json.RawMessage, run map[string]any) json.RawMessage {
	through, ok := run[textureRootDigestedKey]
	if !ok {
		return raw
	}
	metadata := decodeRevisionMetadata(raw)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata[textureRootDigestedKey] = through
	if docs, ok := run[textureRootDigestedDocsKey]; ok {
		metadata[textureRootDigestedDocsKey] = docs
	}
	out, err := json.Marshal(metadata)
	if err != nil {
		return raw
	}
	return out
}
