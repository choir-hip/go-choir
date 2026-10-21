package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md E1: corpus
// search read every story's head revision on every query (2,752 serial
// reads on the paper, 32 s p50, 194 s max) and kept a story only when it
// held every query term, so 61% of searches found no story. Failure modes
// of the ranked index that replaces the scan:
//
//	F1 an unchanged corpus is read again on the next search (the scan returns);
//	F2 a story whose head changed is searched by its old text;
//	F3 another owner's story is returned;
//	F4 a story that shares no query term is returned;
//	F5 a story holding most query terms is dropped because one is missing;
//	F6 ranking ignores the title or term rarity;
//	F7 a cached story is scored without being in the current list (by
//	   construction: scoring walks the list, never the cache);
//	F8 the limit is ignored, or ties order differently between calls;
//	F9 concurrent searches race on the cache.

func searchFixtureDoc(t *testing.T, s *Store, owner, id, title, body string, at time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := s.CreateDocument(ctx, types.Document{DocID: id, OwnerID: owner, Title: title, CreatedAt: at, UpdatedAt: at}); err != nil {
		t.Fatalf("CreateDocument %s: %v", id, err)
	}
	searchFixtureRevise(t, s, owner, id, "", "rev-"+id+"-1", body, at)
}

func searchFixtureRevise(t *testing.T, s *Store, owner, id, parent, revID, body string, at time.Time) {
	t.Helper()
	if err := s.CreateRevision(context.Background(), types.Revision{
		RevisionID: revID, DocID: id, OwnerID: owner, AuthorKind: types.AuthorUser,
		AuthorLabel: "writer", Content: body, CreatedAt: at, ParentRevisionID: parent,
	}); err != nil {
		t.Fatalf("CreateRevision %s: %v", revID, err)
	}
}

func searchIDs(results []SearchResult) []string {
	ids := make([]string, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.DocID)
	}
	return ids
}

func TestSearchDocumentsRanksPartialMatchesAndReadsEachHeadOnce(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	searchFixtureDoc(t, s, "owner", "paris", "Protest violence strains talks in Paris",
		"Unions met the prime minister after a second night of curfew in Paris. Shopkeepers counted losses.", base)
	searchFixtureDoc(t, s, "owner", "curfew", "Cities weigh curfews",
		"Several cities considered a curfew after unrest. Paris was not among them.", base.Add(time.Minute))
	searchFixtureDoc(t, s, "owner", "apu", "Apu returns to The Simpsons",
		"A South Asian actor now voices Apu.", base.Add(2*time.Minute))
	searchFixtureDoc(t, s, "someone-else", "other", "Paris protest curfew unions",
		"Paris protest curfew unions shopkeepers.", base.Add(3*time.Minute))

	// F5: no story holds all six terms; the best partial match still comes back.
	// F6: the story with the terms in its title and body ranks first.
	// F4: the Apu story shares no term and never appears. F3: no other owner.
	results, err := s.SearchDocuments(ctx, "Paris protest unions curfew shopkeepers Lecornu", "owner", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := searchIDs(results); len(got) != 2 || got[0] != "paris" || got[1] != "curfew" {
		t.Fatalf("ranked = %v, want [paris curfew]", got)
	}
	if results[0].MatchSource != "title" || results[1].MatchSource != "title" {
		t.Fatalf("match sources = %q, %q; want title for a title term", results[0].MatchSource, results[1].MatchSource)
	}

	// F1: the corpus did not change, so the next search reads no head.
	before := s.searchIdx.headReads.Load()
	if _, err := s.SearchDocuments(ctx, "Apu actor", "owner", 10); err != nil {
		t.Fatal(err)
	}
	if reads := s.searchIdx.headReads.Load() - before; reads != 0 {
		t.Fatalf("second search read %d heads; an unchanged corpus is read once", reads)
	}

	// F2: a new head is searched by its new text, and only it is read.
	searchFixtureRevise(t, s, "owner", "apu", "rev-apu-1", "rev-apu-2", "A South Asian actor now voices Apu; the episode addresses the controversy.", base.Add(4*time.Minute))
	before = s.searchIdx.headReads.Load()
	results, err = s.SearchDocuments(ctx, "controversy", "owner", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := searchIDs(results); len(got) != 1 || got[0] != "apu" || results[0].MatchSource != "content" {
		t.Fatalf("after revise = %+v, want the Apu story by content", results)
	}
	if reads := s.searchIdx.headReads.Load() - before; reads != 1 {
		t.Fatalf("after one revise the search read %d heads, want 1", reads)
	}
	if results[0].Snippet == "" {
		t.Fatal("a content match carries a snippet")
	}
}

// F8: the limit holds and equal scores order newest first, the same every time.
// F9: concurrent searches over a cold cache agree (run with -race).
func TestSearchDocumentsLimitTiesAndConcurrency(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		searchFixtureDoc(t, s, "owner", fmt.Sprintf("d%d", i), "Market report", "Shares rose.", base.Add(time.Duration(i)*time.Minute))
	}
	var wg sync.WaitGroup
	outs := make([][]string, 4)
	for i := range outs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := s.SearchDocuments(ctx, "market shares", "owner", 3)
			if err != nil {
				t.Error(err)
			}
			outs[i] = searchIDs(r)
		}(i)
	}
	wg.Wait()
	want := []string{"d5", "d4", "d3"}
	for _, got := range outs {
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("limited, tied = %v, want %v", got, want)
		}
	}
}
