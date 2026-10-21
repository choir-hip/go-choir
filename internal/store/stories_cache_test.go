package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// choir.Stories timed out management cells on the paper: a page read every
// candidate's trajectory and every listed story's head one at a time
// (docs/problems/tool-call-failures-dual-model-2026-10-21.md T1). Ways the
// fix can fail, written before it:
//   - a second page reads trajectories or heads it already read for the same
//     trajectory and revision;
//   - a summary is cached by document, so a story whose head moved keeps its
//     old lede and version;
//   - the order, the cursor or the limit change;
//   - concurrent pages race on the caches.

func TestStoryLibraryReadsEachTrajectoryAndHeadOnce(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	var ids []string
	for _, open := range []string{"lib-a", "lib-b", "lib-c"} {
		id, _ := storyResearchFixture(t, s, open)
		ids = append(ids, id)
	}

	first, err := s.ListStoryDocuments(ctx, storyOwner, storyComputer, time.Time{}, "", 0)
	if err != nil || len(first) != 3 {
		t.Fatalf("first page: %d stories, %v", len(first), err)
	}
	sums := s.StorySummariesOf(ctx, first)
	kindsAfterFirst, headsAfterFirst := s.storyKinds.reads.Load(), s.searchIdx.headReads.Load()

	second, err := s.ListStoryDocuments(ctx, storyOwner, storyComputer, time.Time{}, "", 0)
	if err != nil || len(second) != 3 {
		t.Fatalf("second page: %d stories, %v", len(second), err)
	}
	again := s.StorySummariesOf(ctx, second)
	if k, h := s.storyKinds.reads.Load()-kindsAfterFirst, s.searchIdx.headReads.Load()-headsAfterFirst; k != 0 || h != 0 {
		t.Fatalf("second page read %d trajectories and %d heads, want 0 and 0", k, h)
	}
	for i := range sums {
		if sums[i] != again[i] || sums[i].Version < 1 {
			t.Fatalf("summary %d = %+v then %+v", i, sums[i], again[i])
		}
		if one := s.StorySummaryOf(ctx, first[i]); one != sums[i] {
			t.Fatalf("StorySummaryOf = %+v, StorySummariesOf = %+v", one, sums[i])
		}
	}

	// Keyed by revision, not by document: the same document at another head
	// shows that head's lede.
	moved := first[0]
	moved.CurrentRevisionID = first[1].CurrentRevisionID
	if got := s.StorySummariesOf(ctx, []types.Document{moved})[0]; got.Lede != sums[1].Lede || got.Version != sums[1].Version || got.RevisionID != first[1].CurrentRevisionID || got.DocID != first[0].DocID {
		t.Fatalf("moved head summary = %+v, want lede %q for doc %s", got, sums[1].Lede, first[0].DocID)
	}

	// Order, cursor and limit as before: newest first, the page after the
	// first story starts at the second.
	page, err := s.ListStoryDocuments(ctx, storyOwner, storyComputer, first[0].UpdatedAt, first[0].DocID, 1)
	if err != nil || len(page) != 1 || page[0].DocID != first[1].DocID {
		t.Fatalf("page after the first = %+v, %v; want %s", page, err, first[1].DocID)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			docs, err := s.ListStoryDocuments(ctx, storyOwner, storyComputer, time.Time{}, "", 0)
			if err != nil || len(docs) != 3 {
				t.Errorf("concurrent page: %d, %v", len(docs), err)
				return
			}
			s.StorySummariesOf(ctx, docs)
		}()
	}
	wg.Wait()
	_ = ids
}
