package store

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The corpus story search's index (docs/problems/research-more-slow-and-hidden-2026-10-21.md
// E1). Each document's head text is tokenized once per head revision and
// kept here; a search walks the current document list, reads only heads it
// has not seen, and ranks by BM25 with title terms weighted up. Revision
// ids name immutable content, so an entry keyed by its revision id is never
// stale: a new head is a cache miss. The cache is derived, rebuilt from the
// store after a restart, and holds no authority.

const (
	searchBM25K1        = 1.2
	searchBM25B         = 0.75
	searchTitleWeight   = 3
	searchHeadReadPool  = 4
	searchSnippetRadius = 100
)

type searchIndex struct {
	mu      sync.Mutex
	entries map[string]*searchEntry // by doc id
	// headReads counts head revisions read to fill the cache.
	headReads atomic.Uint64
}

type searchEntry struct {
	revisionID string
	// version is the revision's VersionNumber (the story library's
	// summaries read it here too).
	version int
	terms   map[string]int
	length  int
	content string
}

func (x *searchIndex) get(docID string) *searchEntry {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.entries[docID]
}

func (x *searchIndex) put(docID string, e *searchEntry) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.entries == nil {
		x.entries = map[string]*searchEntry{}
	}
	x.entries[docID] = e
}

// prune drops entries for documents the store no longer lists.
func (x *searchIndex) prune(listed map[string]bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	for id := range x.entries {
		if !listed[id] {
			delete(x.entries, id)
		}
	}
}

// searchTokens lowercases and splits on anything that is not a letter or a
// digit, and folds a plural "s" so "protests" finds "protest".
func searchTokens(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := fields[:0]
	for _, f := range fields {
		if len(f) > 3 && strings.HasSuffix(f, "s") && !strings.HasSuffix(f, "ss") {
			f = f[:len(f)-1]
		}
		out = append(out, f)
	}
	return out
}

func searchTermCounts(text string) (map[string]int, int) {
	tokens := searchTokens(text)
	counts := make(map[string]int, len(tokens))
	for _, t := range tokens {
		counts[t]++
	}
	return counts, len(tokens)
}

// searchEntries returns each listed document's entry, reading the heads the
// cache has not seen (a few at a time), and drops entries for documents no
// longer listed: docs must be the whole corpus. A head that cannot be read
// leaves the document searchable by title only and is retried next time.
func (s *Store) searchEntries(ctx context.Context, docs []types.Document) []*searchEntry {
	entries := s.headEntries(ctx, docs)
	listed := make(map[string]bool, len(docs))
	for _, doc := range docs {
		listed[doc.DocID] = true
	}
	s.searchIdx.prune(listed)
	return entries
}

// headEntries returns each document's head entry, reading the heads the cache
// has not seen a few at a time; nil where a head cannot be read. It drops
// nothing, so it serves any subset of the corpus.
func (s *Store) headEntries(ctx context.Context, docs []types.Document) []*searchEntry {
	entries := make([]*searchEntry, len(docs))
	var missing []int
	for i, doc := range docs {
		if doc.CurrentRevisionID == "" {
			continue
		}
		if e := s.searchIdx.get(doc.DocID); e != nil && e.revisionID == doc.CurrentRevisionID {
			entries[i] = e
			continue
		}
		missing = append(missing, i)
	}
	if len(missing) > 0 {
		work := make(chan int)
		var wg sync.WaitGroup
		for w := 0; w < searchHeadReadPool && w < len(missing); w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range work {
					doc := docs[i]
					rev, err := s.searchHeadRevision(ctx, doc)
					s.searchIdx.headReads.Add(1)
					if err != nil {
						continue
					}
					terms, length := searchTermCounts(rev.Content)
					e := &searchEntry{revisionID: doc.CurrentRevisionID, version: rev.VersionNumber, terms: terms, length: length, content: rev.Content}
					s.searchIdx.put(doc.DocID, e)
					entries[i] = e
				}
			}()
		}
		for _, i := range missing {
			if ctx.Err() != nil {
				break
			}
			work <- i
		}
		close(work)
		wg.Wait()
	}
	return entries
}

// rankDocuments scores docs against the query with BM25 over title and head
// text (title terms count searchTitleWeight times) and returns the best limit,
// ties newest first. A document sharing no query term is never returned.
func rankDocuments(query string, docs []types.Document, entries []*searchEntry, limit int) []SearchResult {
	seen := map[string]bool{}
	var terms []string
	for _, t := range searchTokens(query) {
		if !seen[t] {
			seen[t] = true
			terms = append(terms, t)
		}
	}
	if len(terms) == 0 || len(docs) == 0 {
		return nil
	}
	type scored struct {
		i       int
		score   float64
		inTitle bool
	}
	titles := make([]map[string]int, len(docs))
	lengths := make([]float64, len(docs))
	total := 0.0
	df := make(map[string]int, len(terms))
	for i, doc := range docs {
		titles[i], _ = searchTermCounts(doc.Title)
		n := 0
		for _, c := range titles[i] {
			n += c * searchTitleWeight
		}
		if e := entries[i]; e != nil {
			n += e.length
		}
		lengths[i] = float64(n)
		total += float64(n)
		for _, t := range terms {
			if titles[i][t] > 0 || (entries[i] != nil && entries[i].terms[t] > 0) {
				df[t]++
			}
		}
	}
	avg := total / float64(len(docs))
	if avg == 0 {
		avg = 1
	}
	n := float64(len(docs))
	var hits []scored
	for i := range docs {
		score, inTitle := 0.0, false
		for _, t := range terms {
			tf := float64(titles[i][t] * searchTitleWeight)
			if titles[i][t] > 0 {
				inTitle = true
			}
			if entries[i] != nil {
				tf += float64(entries[i].terms[t])
			}
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(df[t])+0.5)/(float64(df[t])+0.5))
			score += idf * tf * (searchBM25K1 + 1) / (tf + searchBM25K1*(1-searchBM25B+searchBM25B*lengths[i]/avg))
		}
		if score > 0 {
			hits = append(hits, scored{i: i, score: score, inTitle: inTitle})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score > hits[b].score
		}
		da, db := docs[hits[a].i], docs[hits[b].i]
		if !da.UpdatedAt.Equal(db.UpdatedAt) {
			return da.UpdatedAt.After(db.UpdatedAt)
		}
		return da.DocID < db.DocID
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	results := make([]SearchResult, 0, len(hits))
	for _, h := range hits {
		doc := docs[h.i]
		r := SearchResult{DocID: doc.DocID, Title: doc.Title, OwnerID: doc.OwnerID, UpdatedAt: doc.UpdatedAt, MatchSource: "content"}
		if h.inTitle {
			r.MatchSource = "title"
		} else if e := entries[h.i]; e != nil {
			r.Snippet = searchSnippet(e.content, terms)
		}
		results = append(results, r)
	}
	return results
}

// searchSnippet is about 200 bytes of content around the first query term.
func searchSnippet(content string, terms []string) string {
	lower := strings.ToLower(content)
	at := -1
	for _, t := range terms {
		if i := strings.Index(lower, t); i >= 0 && (at < 0 || i < at) {
			at = i
		}
	}
	start := 0
	if at > searchSnippetRadius {
		start = at - searchSnippetRadius
	}
	end := start + 2*searchSnippetRadius
	if end > len(content) {
		end = len(content)
	}
	return strings.ToValidUTF8(content[start:end], "")
}

// WarmSearchIndex reads every listed document's head into the search cache,
// so the first corpus search after a boot does not pay for the whole corpus.
// It returns how many documents it saw and how many heads it read.
func (s *Store) WarmSearchIndex(ctx context.Context) (docs int, reads uint64, err error) {
	listed, err := s.searchableTextureDocuments(ctx, "")
	if err != nil {
		return 0, 0, err
	}
	before := s.searchIdx.headReads.Load()
	s.searchEntries(ctx, listed)
	s.warmStoryKinds(ctx, listed)
	return len(listed), s.searchIdx.headReads.Load() - before, ctx.Err()
}

// StoryLibraryIDF returns the inverse document frequency of a word over the
// owner's story library, from the search index's head term counts, for the
// entity extraction's rarity (docs/problems/related-stories-no-entity-layer-
// 2026-10-26.md, slice c0 §1): ln((N+1)/(df+1)) + 1, so a word in every
// story weighs about 1 and a word in none weighs most. A word is looked up
// as the index tokenizes it. An empty library gives nil (rarity unknown).
// The function memoizes and is for one goroutine.
func (s *Store) StoryLibraryIDF(ctx context.Context, ownerID string) (func(string) float64, error) {
	docs, err := s.searchableTextureDocuments(ctx, strings.TrimSpace(ownerID))
	if err != nil {
		return nil, err
	}
	var live []*searchEntry
	for _, e := range s.headEntries(ctx, docs) {
		if e != nil {
			live = append(live, e)
		}
	}
	if len(live) == 0 {
		return nil, nil
	}
	n := float64(len(live))
	memo := map[string]float64{}
	return func(word string) float64 {
		toks := searchTokens(word)
		if len(toks) == 0 {
			return 1
		}
		term := toks[0]
		if v, ok := memo[term]; ok {
			return v
		}
		df := 0
		for _, e := range live {
			if e.terms[term] > 0 {
				df++
			}
		}
		v := math.Log((n+1)/(float64(df)+1)) + 1
		memo[term] = v
		return v
	}, nil
}
