package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The story library (L2a A2,
// docs/reports/l2a-synthesis-intake-design-2026-10-14.md): every living
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
// kind story) updated strictly before before (zero: no bound), newest
// first, at most limit (0: all).
func (s *Store) ListStoryDocuments(ctx context.Context, ownerID, computerID string, before time.Time, limit int) ([]types.Document, error) {
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
		if !before.IsZero() && !doc.UpdatedAt.Before(before) {
			continue
		}
		trajectory, err := s.GetLifecycleTrajectory(ctx, ownerID, computerID, doc.TrajectoryID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if trajectory.Kind != types.TrajectoryKindStory {
			continue
		}
		out = append(out, doc)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// StorySummaryOf reads a story's head revision for its lede and version.
// A head that cannot be read leaves both empty; the story is still listed.
func (s *Store) StorySummaryOf(ctx context.Context, doc types.Document) StorySummary {
	out := StorySummary{DocID: doc.DocID, TrajectoryID: doc.TrajectoryID, Headline: doc.Title, UpdatedAt: doc.UpdatedAt, RevisionID: doc.CurrentRevisionID}
	if strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return out
	}
	rev, err := s.GetLifecycleRevision(ctx, doc.OwnerID, doc.ComputerID, doc.CurrentRevisionID)
	if err != nil {
		return out
	}
	out.Version = types.RevisionNumber(rev.VersionNumber)
	out.Lede = StoryLede(rev.Content)
	return out
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
