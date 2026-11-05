package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The story-index projection (modular architecture plan section 8.4, X8 and
// X9; MA3.7a). Texture is the custodian of stories, and management used to
// read them straight from Texture's documents and revisions to list the
// library and to give the intake labeler its candidates. Now Texture
// publishes a projection of what management uses, one row per story
// (headline, lede, version and ids) and one library row that mirrors the
// story-library head; management reads only those rows.
//
// Texture writes the rows where it writes the story: the opening writes the
// story's row and advances the mirror in the same batch as the head
// (openStories). A revision, an archive or an owner edit changes the
// document through many store paths, so the custodian also reconciles the
// rows against its documents on demand (PublishStoryIndex), which every
// management read of the index runs first. Both writers declare the Texture
// custodian, taken from the registry; nothing here names a desk.

const ogKindStoryIndex = objectgraph.ObjectKind("choir.story_index")

func storyIndexRowKey(docID string) string { return "story:" + docID }

// textureCustodianCtx declares the custodian of Texture's documents as the
// writer and the reader of ctx: the host-side Texture handler acting as the
// custodian.
func textureCustodianCtx(ctx context.Context) context.Context {
	w, ok := objectgraph.CustodianOf(ogKindTexDoc)
	if !ok {
		objectgraph.FailOnUnknownCustodian(ogKindTexDoc)
		return ctx
	}
	return objectgraph.WithReader(objectgraph.WithWriter(ctx, w), w)
}

func storyIndexObject(ownerID, computerID, key string, entry types.StoryIndexEntry, prior *objectgraph.Object, now time.Time) (objectgraph.Object, objectgraph.ObjectCondition, error) {
	meta := map[string]any{"computer_id": computerID}
	if entry.DocID != "" {
		meta["doc_id"] = entry.DocID
	}
	created := now
	if prior != nil {
		created = prior.CreatedAt
	}
	obj, err := lifecycleObject(ogKindStoryIndex, ownerID, computerID, key, entry, meta, created, now)
	if err != nil {
		return objectgraph.Object{}, objectgraph.ObjectCondition{}, err
	}
	cond := objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID}
	if prior != nil {
		cond.Exists, cond.ExpectedContentHash = true, prior.ContentHash
	}
	return obj, cond, nil
}

// storyIndexRows reads the scope's index rows by key.
func (s *Store) storyIndexRows(ctx context.Context, ownerID, computerID string) (map[string]objectgraph.Object, error) {
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindStoryIndex)
	if err != nil {
		return nil, err
	}
	rows := map[string]objectgraph.Object{}
	for _, obj := range objs {
		if obj.OwnerID != ownerID || strings.TrimSpace(obj.ComputerID) != computerID {
			continue
		}
		var e types.StoryIndexEntry
		if err := json.Unmarshal(obj.Body, &e); err != nil {
			return nil, err
		}
		key := types.StoryIndexLibraryKey
		if e.Library == nil {
			key = storyIndexRowKey(e.DocID)
		}
		rows[key] = obj
	}
	return rows, nil
}

// PublishStoryIndex is the Texture custodian publishing its story index for
// one computer: it reconciles the rows against the stories (headline, lede,
// version, ids, archive) and the library row against the story-library head,
// writing only what differs. It is idempotent and safe to run on any read.
func (s *Store) PublishStoryIndex(ctx context.Context, ownerID, computerID string) error {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return fmt.Errorf("story index: owner_id and computer_id are required")
	}
	ctx = textureCustodianCtx(ctx)
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	return s.reconcileStoryIndex(ctx, ownerID, computerID, true)
}

func sameStoryIndexEntry(a, b types.StoryIndexEntry) bool {
	return a.DocID == b.DocID && a.TrajectoryID == b.TrajectoryID && a.RevisionID == b.RevisionID && a.Headline == b.Headline &&
		a.Lede == b.Lede && a.Version == b.Version && a.UpdatedAt.Equal(b.UpdatedAt) && a.Archived == b.Archived
}

// reconcileStoryIndex does PublishStoryIndex's work with trajectoryMu held and
// the custodian declared. stories false reconciles the library row only (the
// opening path, which writes its own story row).
func (s *Store) reconcileStoryIndex(ctx context.Context, ownerID, computerID string, stories bool) error {
	rows, err := s.storyIndexRows(ctx, ownerID, computerID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	if stories {
		docs, err := s.ListStoryDocuments(ctx, ownerID, computerID, time.Time{}, "", 0)
		if err != nil {
			return err
		}
		live := map[string]bool{}
		for _, sum := range s.StorySummariesOf(ctx, docs) {
			key := storyIndexRowKey(sum.DocID)
			live[key] = true
			want := types.StoryIndexEntry{DocID: sum.DocID, TrajectoryID: sum.TrajectoryID, RevisionID: sum.RevisionID,
				Headline: sum.Headline, Lede: sum.Lede, Version: sum.Version, UpdatedAt: sum.UpdatedAt.UTC()}
			var priorPtr *objectgraph.Object
			if prior, had := rows[key]; had {
				var cur types.StoryIndexEntry
				if json.Unmarshal(prior.Body, &cur) == nil && sameStoryIndexEntry(cur, want) {
					continue
				}
				priorPtr = &prior
			}
			obj, cond, err := storyIndexObject(ownerID, computerID, key, want, priorPtr, now)
			if err != nil {
				return err
			}
			objects, conditions = append(objects, obj), append(conditions, cond)
		}
		for key, prior := range rows {
			if live[key] || key == types.StoryIndexLibraryKey {
				continue
			}
			var cur types.StoryIndexEntry
			if json.Unmarshal(prior.Body, &cur) != nil || cur.Archived {
				continue
			}
			cur.Archived = true
			obj, cond, err := storyIndexObject(ownerID, computerID, key, cur, &prior, now)
			if err != nil {
				return err
			}
			objects, conditions = append(objects, obj), append(conditions, cond)
		}
	}
	// The library row mirrors the head the custodian keeps.
	head, err := loadInboxCached[types.StoryLibraryHead](ctx, s, ogKindStoryLibraryHead, ownerID, computerID, storyLibraryKey)
	if err != nil {
		return err
	}
	var libPrior *objectgraph.Object
	var libCur types.StoryIndexEntry
	if prior, had := rows[types.StoryIndexLibraryKey]; had {
		libPrior = &prior
		_ = json.Unmarshal(prior.Body, &libCur)
	}
	if libPrior == nil || libCur.Library == nil || libCur.Library.Head != head.value.Head {
		value := head.value
		obj, cond, err := storyIndexObject(ownerID, computerID, types.StoryIndexLibraryKey, types.StoryIndexEntry{Library: &value}, libPrior, now)
		if err != nil {
			return err
		}
		objects, conditions = append(objects, obj), append(conditions, cond)
	}
	if len(objects) == 0 {
		return nil
	}
	return s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects})
}

// ListStoryIndex reads the story-index projection as the caller (management's
// reader): the computer's stories, newest first (ties by doc id), after the
// cursor (before, beforeDoc), at most limit (0: all). It reads no document.
func (s *Store) ListStoryIndex(ctx context.Context, ownerID, computerID string, before time.Time, beforeDoc string, limit int) ([]StorySummary, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("stories: owner_id and computer_id are required")
	}
	rows, err := s.storyIndexRows(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	var all []StorySummary
	for key, obj := range rows {
		if key == types.StoryIndexLibraryKey {
			continue
		}
		var e types.StoryIndexEntry
		if err := json.Unmarshal(obj.Body, &e); err != nil {
			return nil, err
		}
		if e.Archived {
			continue
		}
		all = append(all, StorySummary{DocID: e.DocID, TrajectoryID: e.TrajectoryID, Headline: e.Headline, Lede: e.Lede,
			UpdatedAt: e.UpdatedAt, Version: e.Version, RevisionID: e.RevisionID})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].UpdatedAt.Equal(all[j].UpdatedAt) {
			return all[i].UpdatedAt.After(all[j].UpdatedAt)
		}
		return all[i].DocID < all[j].DocID
	})
	out := make([]StorySummary, 0, len(all))
	for _, sum := range all {
		if !before.IsZero() && (sum.UpdatedAt.After(before) || (sum.UpdatedAt.Equal(before) && sum.DocID <= beforeDoc)) {
			continue
		}
		out = append(out, sum)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// storyLibraryMirror reads the library row of the projection (the head and
// its recent opens), the zero value before the Texture custodian published it.
func (s *Store) storyLibraryMirror(ctx context.Context, ownerID, computerID string) (types.StoryLibraryHead, error) {
	cached, err := loadInboxCached[types.StoryIndexEntry](ctx, s, ogKindStoryIndex, ownerID, computerID, types.StoryIndexLibraryKey)
	if err != nil {
		return types.StoryLibraryHead{}, err
	}
	if cached.value.Library == nil {
		return types.StoryLibraryHead{}, nil
	}
	return *cached.value.Library, nil
}
