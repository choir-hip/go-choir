package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ChangedTextureDocument is one document that changed, found through the
// header time of its object or of its trajectory (a work item or a report
// changes the trajectory and leaves the document's head alone).
type ChangedTextureDocument struct {
	// ChangedAt is the header time that put the document in the list: its
	// earliest change in the range.
	ChangedAt time.Time
	// ObjectID names the object whose row it came from.
	ObjectID string
	Doc      types.Document
	// Trajectory is the document's trajectory, zero when it has none.
	Trajectory types.TrajectoryRecord
	// Err is set when the document could not be read: it is named in the
	// list, not skipped, so the caller can hold its watermark behind it.
	Err error
}

const (
	changedTexturePage = 256
	// ChangedTextureScanCap bounds one listing: more is reported as more.
	ChangedTextureScanCap = 2000
)

// ListTextureDocumentsChangedSince lists the non-story, non-archived
// documents of one computer whose document or trajectory object changed at
// or after since, oldest change first. Stories are left out of the query,
// before any bound, so a flood of them never crowds the others out. Work is
// bounded by the rows changed in the range, through
// idx_og_objects_kind_owner_updated, and by ChangedTextureScanCap (more
// reports that the range held more).
func (s *Store) ListTextureDocumentsChangedSince(ctx context.Context, ownerID, computerID string, since time.Time) (changed []ChangedTextureDocument, more bool, err error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, false, fmt.Errorf("list changed texture documents: owner_id and computer_id are required")
	}
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return nil, false, fmt.Errorf("store: object graph not initialized")
	}
	seen := map[string]bool{}
	var cursor objectgraph.ChangeCursor
	for {
		docs, err := store.ListObjectsChangedSince(ctx, string(ogKindTexDoc), ownerID, computerID, since, cursor, changedTexturePage)
		if err != nil {
			return nil, false, err
		}
		trajs, err := store.ListObjectsChangedSince(ctx, string(ogKindTrajectory), ownerID, computerID, since, cursor, changedTexturePage)
		if err != nil {
			return nil, false, err
		}
		// Process the merged rows up to the frontier: the last row of a list
		// that came back full (it may have more past it); the rest is read
		// again from the cursor.
		var frontier *objectgraph.ChangeCursor
		for _, list := range [][]objectgraph.Object{docs, trajs} {
			if len(list) == changedTexturePage {
				last := list[len(list)-1]
				c := objectgraph.ChangeCursor{At: last.UpdatedAt, ID: last.CanonicalID}
				if frontier == nil || changeBefore(c.At, c.ID, frontier.At, frontier.ID) {
					frontier = &c
				}
			}
		}
		rows := mergeByChange(docs, trajs)
		consumed := cursor
		for _, obj := range rows {
			if frontier != nil && changeBefore(frontier.At, frontier.ID, obj.UpdatedAt, obj.CanonicalID) {
				break
			}
			consumed = objectgraph.ChangeCursor{At: obj.UpdatedAt, ID: obj.CanonicalID}
			entry, ok := s.changedTextureEntry(ctx, ownerID, computerID, obj)
			if !ok {
				continue
			}
			key := entry.Doc.DocID
			if key == "" {
				key = entry.ObjectID
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			if len(changed) >= ChangedTextureScanCap {
				return changed, true, nil
			}
			changed = append(changed, entry)
		}
		if frontier == nil {
			return changed, false, nil
		}
		cursor = consumed
		if cursor.At.IsZero() || changeBefore(cursor.At, cursor.ID, frontier.At, frontier.ID) {
			cursor = *frontier
		}
	}
}

func changeBefore(aAt time.Time, aID string, bAt time.Time, bID string) bool {
	if !aAt.Equal(bAt) {
		return aAt.Before(bAt)
	}
	return aID < bID
}

func mergeByChange(a, b []objectgraph.Object) []objectgraph.Object {
	out := make([]objectgraph.Object, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case j >= len(b) || (i < len(a) && changeBefore(a[i].UpdatedAt, a[i].CanonicalID, b[j].UpdatedAt, b[j].CanonicalID)):
			out = append(out, a[i])
			i++
		default:
			out = append(out, b[j])
			j++
		}
	}
	return out
}

// changedTextureEntry turns one changed object into an entry, or reports
// false for a row that is not an ordinary document (a story, an archived or
// missing document, a trajectory that names none).
func (s *Store) changedTextureEntry(ctx context.Context, ownerID, computerID string, obj objectgraph.Object) (ChangedTextureDocument, bool) {
	entry := ChangedTextureDocument{ChangedAt: obj.UpdatedAt, ObjectID: obj.CanonicalID}
	fail := func(err error) (ChangedTextureDocument, bool) {
		entry.Err = fmt.Errorf("%s: %w", obj.CanonicalID, err)
		return entry, true
	}
	if obj.ObjectKind == ogKindTrajectory {
		trajectory, err := decodeLifecycleObject[types.TrajectoryRecord](obj)
		if err != nil {
			return fail(err)
		}
		docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
		if docID == "" || trajectory.Kind == types.TrajectoryKindStory {
			return ChangedTextureDocument{}, false
		}
		doc, err := s.GetLifecycleDocument(ctx, ownerID, computerID, docID)
		if errors.Is(err, ErrNotFound) {
			return ChangedTextureDocument{}, false
		}
		if err != nil {
			return fail(err)
		}
		entry.Doc, entry.Trajectory = doc, trajectory
	} else {
		var doc types.Document
		if err := ogDecode(obj, &doc); err != nil {
			return fail(err)
		}
		entry.Doc = doc
		if strings.TrimSpace(doc.TrajectoryID) != "" {
			trajectory, err := s.GetLifecycleTrajectory(ctx, ownerID, computerID, doc.TrajectoryID)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return fail(err)
			}
			entry.Trajectory = trajectory
		}
	}
	if entry.Doc.ArchivedAt != nil || entry.Trajectory.Kind == types.TrajectoryKindStory {
		return ChangedTextureDocument{}, false
	}
	return entry, true
}
