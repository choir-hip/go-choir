package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The story library (L2a A2,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md): every living
// story of one computer, newest first, with no cap. The ProductAPI document
// list keeps its cap for the UI; intake labels and management's
// choir.Stories read this.

// StorySummary is one story as the library lists it.
type StorySummary struct {
	DocID        string    `json:"doc_id"`
	TrajectoryID string    `json:"trajectory_id,omitempty"`
	Headline     string    `json:"headline"`
	Lede         string    `json:"lede,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	// Version is the head revision's number, counted from 1; the seed a
	// story opens with is 1.
	Version    int    `json:"version"`
	RevisionID string `json:"revision_id,omitempty"`
}

// storyLedeMaxRunes bounds a lede.
const storyLedeMaxRunes = 400

// ListStoryDocuments returns the computer's story documents (trajectory
// kind story), newest first (ties by doc id), after the cursor (before,
// beforeDoc): those updated strictly before before, or at before with a
// doc id after beforeDoc. A zero before is no bound. At most limit (0:
// all).
func (s *Store) ListStoryDocuments(ctx context.Context, ownerID, computerID string, before time.Time, beforeDoc string, limit int) ([]types.Document, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("stories: owner_id and computer_id are required")
	}
	docs, err := s.ListTextureDocumentsByScopeOG(ctx, ownerID, computerID, 1<<30)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(docs, func(i, j int) bool {
		if !docs[i].UpdatedAt.Equal(docs[j].UpdatedAt) {
			return docs[i].UpdatedAt.After(docs[j].UpdatedAt)
		}
		return docs[i].DocID < docs[j].DocID
	})
	out := make([]types.Document, 0)
	for _, doc := range docs {
		if strings.TrimSpace(doc.TrajectoryID) == "" || doc.ArchivedAt != nil {
			continue
		}
		if !before.IsZero() && (doc.UpdatedAt.After(before) || (doc.UpdatedAt.Equal(before) && doc.DocID <= beforeDoc)) {
			continue
		}
		kind, err := s.storyTrajectoryKind(ctx, ownerID, computerID, doc.TrajectoryID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if kind != types.TrajectoryKindStory {
			continue
		}
		out = append(out, doc)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// StorySummaryOf is one story's summary (StorySummariesOf).
func (s *Store) StorySummaryOf(ctx context.Context, doc types.Document) StorySummary {
	return s.StorySummariesOf(ctx, []types.Document{doc})[0]
}

// StorySummariesOf gives each story its head's lede and version, from the
// head entries the corpus search keeps by revision id (search_index.go), so a
// head is read once however often the library is listed, and a few at a time
// when it is (docs/problems/tool-call-failures-dual-model-2026-10-21.md T1).
// A head that cannot be read leaves both empty; the story is still listed.
func (s *Store) StorySummariesOf(ctx context.Context, docs []types.Document) []StorySummary {
	entries := s.headEntries(ctx, docs)
	out := make([]StorySummary, len(docs))
	for i, doc := range docs {
		out[i] = StorySummary{DocID: doc.DocID, TrajectoryID: doc.TrajectoryID, Headline: doc.Title, UpdatedAt: doc.UpdatedAt, RevisionID: doc.CurrentRevisionID}
		if e := entries[i]; e != nil {
			out[i].Version = types.RevisionNumber(e.version)
			out[i].Lede = StoryLede(e.content)
		}
	}
	return out
}

// storyKindCache keeps each trajectory's kind, which never changes after the
// trajectory is created, so the library reads a trajectory once.
type storyKindCache struct {
	mu    sync.Mutex
	kinds map[string]types.TrajectoryKind
	// reads counts trajectories read to fill the cache.
	reads atomic.Uint64
}

func (s *Store) storyTrajectoryKind(ctx context.Context, ownerID, computerID, trajectoryID string) (types.TrajectoryKind, error) {
	key := ownerID + "\x00" + computerID + "\x00" + trajectoryID
	s.storyKinds.mu.Lock()
	kind, ok := s.storyKinds.kinds[key]
	s.storyKinds.mu.Unlock()
	if ok {
		return kind, nil
	}
	s.storyKinds.reads.Add(1)
	trajectory, err := s.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return "", err
	}
	s.storyKinds.mu.Lock()
	if s.storyKinds.kinds == nil {
		s.storyKinds.kinds = map[string]types.TrajectoryKind{}
	}
	s.storyKinds.kinds[key] = trajectory.Kind
	s.storyKinds.mu.Unlock()
	return trajectory.Kind, nil
}

// warmStoryKinds reads the kinds of the listed documents' trajectories a few
// at a time, so the first library page after a boot does not read them one
// by one.
func (s *Store) warmStoryKinds(ctx context.Context, docs []types.Document) {
	work := make(chan types.Document)
	var wg sync.WaitGroup
	for w := 0; w < searchHeadReadPool; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for doc := range work {
				_, _ = s.storyTrajectoryKind(ctx, doc.OwnerID, doc.ComputerID, doc.TrajectoryID)
			}
		}()
	}
	for _, doc := range docs {
		if ctx.Err() != nil {
			break
		}
		if strings.TrimSpace(doc.TrajectoryID) == "" || strings.TrimSpace(doc.ComputerID) == "" || doc.ArchivedAt != nil {
			continue
		}
		work <- doc
	}
	close(work)
	wg.Wait()
}

// StoryLede is the first paragraph of a story's text that is not a
// heading, bounded.
func StoryLede(content string) string {
	for _, para := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" || strings.HasPrefix(para, "#") {
			continue
		}
		para = strings.Join(strings.Fields(para), " ")
		if runes := []rune(para); len(runes) > storyLedeMaxRunes {
			para = strings.TrimSpace(string(runes[:storyLedeMaxRunes])) + "…"
		}
		return para
	}
	return ""
}
