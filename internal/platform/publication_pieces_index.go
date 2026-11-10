package platform

// The paper's own archive, searched by BM25 (docs/problems/published-search-recall-2026-11-10.md,
// docs/design/autopaper-learning-materiality-2026-11-04.md section 8.6).
//
// The index is derived state: an in-memory inverted index over every
// published story and opinion piece, built from Store B and never written
// back. It stays current without a publish hook: each search first reads the
// publication rows newer than the newest one it has seen (one windowed query
// when nothing changed) and re-indexes those. A restart starts empty and
// rebuilds from the store, newest first, in the background
// (WarmPublishedPieceIndex); a search that arrives before the build is whole
// answers from what is indexed and says capped. No embeddings; no new durable
// store.

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/captureterms"
)

const (
	bm25K1 = 1.2
	bm25B  = 0.75
	// pieceTitleWeight counts a title word as this many body words.
	pieceTitleWeight = 3
	// pieceQueryMaxTerms bounds a query's distinct index terms (a CJK query
	// is unigrams and bigrams, so this is larger than the word count).
	pieceQueryMaxTerms = 32
)

// Variables so tests can shrink them. The slice budget bounds how long one
// search spends catching the index up; the warm budget bounds the boot build.
var (
	publishedPieceSliceBudget = 20 * time.Second
	publishedPieceWarmBudget  = 10 * time.Minute
)

type piecePosting struct {
	doc int32
	tf  uint16
}

type pieceDoc struct {
	id     string
	terms  []string
	length int32
	live   bool
}

// pieceIndex is the derived search state. refreshMu admits one refresher;
// mu guards the data.
type pieceIndex struct {
	refreshMu sync.Mutex
	mu        sync.RWMutex
	docs      []pieceDoc
	slot      map[string]int32
	postings  map[string][]piecePosting
	totalLen  int64
	live      int
	// watermark is the newest row update seen; cursor is where the backfill
	// stopped; complete is true once the walk reached the oldest row.
	watermark time.Time
	cursor    *frontCursor
	started   bool
	complete  bool
}

// pieceTokens is a text's index terms, repeats kept, with light plural
// folding on plain ASCII words so "harbors" finds "harbor". Other scripts
// are the capture term table's tokens (CJK unigrams and bigrams); accents
// are kept.
func pieceTokens(text string) []string {
	toks := captureterms.Tokens(text)
	for i, t := range toks {
		toks[i] = foldPlural(t)
	}
	return toks
}

func foldPlural(t string) string {
	n := len(t)
	if n < 4 {
		return t
	}
	for i := 0; i < n; i++ {
		if t[i] < 'a' || t[i] > 'z' {
			return t
		}
	}
	switch {
	case strings.HasSuffix(t, "ies") && n > 4:
		return t[:n-3] + "y"
	case strings.HasSuffix(t, "sses"), strings.HasSuffix(t, "xes"), strings.HasSuffix(t, "ches"), strings.HasSuffix(t, "shes"):
		return t[:n-2]
	case strings.HasSuffix(t, "ss"), strings.HasSuffix(t, "us"), strings.HasSuffix(t, "is"):
		return t
	case strings.HasSuffix(t, "s"):
		return t[:n-1]
	}
	return t
}

func distinctQueryTerms(query string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range pieceTokens(query) {
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == pieceQueryMaxTerms {
			break
		}
	}
	return out
}

// pieceLoad is one publication read for indexing or for a hit.
type pieceLoad struct {
	meta    frontPublicationMeta
	kind    string
	title   string
	content string
	route   string
}

// loadPiece reads a publication's latest published version. ok is false for
// anything research does not search or cannot read.
func (s *Service) loadPiece(ctx context.Context, og *ObjectGraphStore, row frontPublicationRow) (pieceLoad, bool) {
	var meta frontPublicationMeta
	if err := json.Unmarshal(row.Metadata, &meta); err != nil {
		return pieceLoad{}, false
	}
	if meta.State != "published" || strings.TrimSpace(meta.RoutePath) == "" {
		return pieceLoad{}, false
	}
	kind := s.pieceKind(meta)
	if kind == "" {
		return pieceLoad{}, false
	}
	versions, err := publishedVersions(ctx, og, row.CanonicalID)
	if err != nil || len(versions) == 0 {
		return pieceLoad{}, false
	}
	versionObj, err := og.GetObject(ctx, versions[len(versions)-1].CanonicalID)
	if err != nil {
		return pieceLoad{}, false
	}
	manifest, err := s.frontManifest(ctx, og, versionObj.CanonicalID)
	if err != nil || !publicationAccessPolicyAllowsPublicRoute(manifest.AccessPolicy) || manifest.StorageRef == "" {
		return pieceLoad{}, false
	}
	contentBytes, err := s.readBlob(manifest.StorageRef)
	if err != nil {
		return pieceLoad{}, false
	}
	return pieceLoad{
		meta:    meta,
		kind:    kind,
		title:   strings.TrimSpace(firstNonEmpty(meta.Title, manifest.Title)),
		content: string(contentBytes),
		route:   normalizePublicationRoutePath(meta.RoutePath),
	}, true
}

// removeLocked drops a document's postings. The caller holds mu.
func (ix *pieceIndex) removeLocked(id string) {
	slot, ok := ix.slot[id]
	if !ok {
		return
	}
	d := &ix.docs[slot]
	if !d.live {
		return
	}
	for _, term := range d.terms {
		list := ix.postings[term]
		for i := range list {
			if list[i].doc == slot {
				list[i] = list[len(list)-1]
				list = list[:len(list)-1]
				break
			}
		}
		if len(list) == 0 {
			delete(ix.postings, term)
		} else {
			ix.postings[term] = list
		}
	}
	ix.totalLen -= int64(d.length)
	ix.live--
	d.terms, d.live, d.length = nil, false, 0
}

// putLocked indexes a document, replacing any earlier version of it.
func (ix *pieceIndex) putLocked(id, title, content string) {
	ix.removeLocked(id)
	counts := map[string]int{}
	length := 0
	for _, t := range pieceTokens(content) {
		counts[t]++
		length++
	}
	for _, t := range pieceTokens(title) {
		counts[t] += pieceTitleWeight
		length += pieceTitleWeight
	}
	slot, ok := ix.slot[id]
	if !ok {
		slot = int32(len(ix.docs))
		ix.docs = append(ix.docs, pieceDoc{id: id})
		ix.slot[id] = slot
	}
	terms := make([]string, 0, len(counts))
	for term, n := range counts {
		if n > 65535 {
			n = 65535
		}
		ix.postings[term] = append(ix.postings[term], piecePosting{doc: slot, tf: uint16(n)})
		terms = append(terms, term)
	}
	ix.docs[slot] = pieceDoc{id: id, terms: terms, length: int32(length), live: true}
	ix.totalLen += int64(length)
	ix.live++
}

// refreshPieceIndex catches the index up: the rows newer than the watermark,
// then, until the walk is complete, more of the backlog, within budget.
// With wait false, a refresh already running (the boot build) is not waited
// for: the caller answers from what is indexed.
func (s *Service) refreshPieceIndex(ctx context.Context, budget time.Duration, wait bool) (complete bool, err error) {
	ix := &s.pieces
	if wait {
		ix.refreshMu.Lock()
	} else if !ix.refreshMu.TryLock() {
		ix.mu.RLock()
		defer ix.mu.RUnlock()
		return ix.complete, nil
	}
	defer ix.refreshMu.Unlock()
	og := s.ogStore()
	if og == nil {
		return false, nil
	}
	ix.mu.Lock()
	if ix.slot == nil {
		ix.slot = map[string]int32{}
		ix.postings = map[string][]piecePosting{}
	}
	started, watermark, cursor, done := ix.started, ix.watermark, ix.cursor, ix.complete
	ix.mu.Unlock()
	deadline := time.Now().Add(budget)

	// walk reads windows after the cursor and indexes each row, until the
	// archive ends, a row is older than stopBefore, or the budget is spent.
	walk := func(after *frontCursor, stopBefore time.Time, record bool) (*frontCursor, bool, time.Time, error) {
		batch := publishedPieceBatch
		if batch < 1 {
			batch = 1
		}
		var newest time.Time
		for {
			if ctx.Err() != nil {
				return after, false, newest, ctx.Err()
			}
			rows, err := s.frontPublicationBatch(ctx, after, batch)
			if err != nil {
				return after, false, newest, err
			}
			reached := false
			for _, row := range rows {
				if !stopBefore.IsZero() && row.UpdatedAt.Before(stopBefore) {
					reached = true
					break
				}
				if newest.IsZero() {
					newest = row.UpdatedAt
				}
				s.indexPieceRow(ctx, og, row)
			}
			if reached || len(rows) < batch {
				return after, true, newest, nil
			}
			last := rows[len(rows)-1]
			after = &frontCursor{UpdatedAt: last.UpdatedAt.Unix(), CanonicalID: last.CanonicalID}
			if record {
				ix.mu.Lock()
				ix.cursor = after
				ix.mu.Unlock()
			}
			if time.Now().After(deadline) {
				return after, false, newest, nil
			}
		}
	}

	if !started {
		ix.mu.Lock()
		ix.started = true
		ix.mu.Unlock()
		next, finished, newest, err := walk(nil, time.Time{}, true)
		ix.mu.Lock()
		ix.watermark, ix.cursor, ix.complete = newest, next, finished && err == nil
		complete = ix.complete
		ix.mu.Unlock()
		return complete, err
	}
	if _, finished, newest, err := walk(nil, watermark, false); err != nil {
		return done, err
	} else if finished && newest.After(watermark) {
		ix.mu.Lock()
		ix.watermark = newest
		ix.mu.Unlock()
	}
	if done {
		return true, nil
	}
	next, finished, _, err := walk(cursor, time.Time{}, true)
	ix.mu.Lock()
	ix.cursor, ix.complete = next, finished && err == nil
	complete = ix.complete
	ix.mu.Unlock()
	return complete, err
}

func (s *Service) indexPieceRow(ctx context.Context, og *ObjectGraphStore, row frontPublicationRow) {
	load, ok := s.loadPiece(ctx, og, row)
	ix := &s.pieces
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if !ok {
		// Not a searchable piece now (withdrawn, or never one). A transient
		// read failure drops the piece until its row moves again; the search
		// verifies every hit, so this only costs recall.
		ix.removeLocked(row.CanonicalID)
		return
	}
	ix.putLocked(row.CanonicalID, load.title, load.content)
}

// WarmPublishedPieceIndex builds the index from the store, in the
// background at boot, within publishedPieceWarmBudget.
func (s *Service) WarmPublishedPieceIndex(ctx context.Context) {
	if s == nil || s.store == nil {
		return
	}
	start := time.Now()
	complete, err := s.refreshPieceIndex(ctx, publishedPieceWarmBudget, true)
	s.pieces.mu.RLock()
	n := s.pieces.live
	s.pieces.mu.RUnlock()
	log.Printf("corpusd: published piece index warm: pieces=%d complete=%v ms=%d err=%v", n, complete, time.Since(start).Milliseconds(), err)
}

type pieceCandidate struct {
	id    string
	score float64
	slot  int32
}

// rank scores every live document that holds at least one query term and
// returns the best n, strongest first; ties go to the later slot (a newer
// indexing). Partial matches count: a document missing a word loses only that
// word's score.
func (ix *pieceIndex) rank(terms []string, n int) (best []pieceCandidate, idf map[string]float64) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	idf = map[string]float64{}
	if ix.live == 0 {
		return nil, idf
	}
	avg := float64(ix.totalLen) / float64(ix.live)
	if avg < 1 {
		avg = 1
	}
	scores := map[int32]float64{}
	for _, term := range terms {
		list := ix.postings[term]
		if len(list) == 0 {
			continue
		}
		df := float64(len(list))
		w := math.Log(1 + (float64(ix.live)-df+0.5)/(df+0.5))
		idf[term] = w
		for _, p := range list {
			tf := float64(p.tf)
			dl := float64(ix.docs[p.doc].length)
			scores[p.doc] += w * tf * (bm25K1 + 1) / (tf + bm25K1*(1-bm25B+bm25B*dl/avg))
		}
	}
	best = make([]pieceCandidate, 0, len(scores))
	for slot, sc := range scores {
		best = append(best, pieceCandidate{id: ix.docs[slot].id, score: sc, slot: slot})
	}
	sort.Slice(best, func(i, j int) bool {
		if best[i].score != best[j].score {
			return best[i].score > best[j].score
		}
		return best[i].slot > best[j].slot
	})
	if len(best) > n {
		best = best[:n]
	}
	return best, idf
}

func (ix *pieceIndex) recent(n int) []pieceCandidate {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	var out []pieceCandidate
	for slot := len(ix.docs) - 1; slot >= 0 && len(out) < n; slot-- {
		if ix.docs[slot].live {
			out = append(out, pieceCandidate{id: ix.docs[slot].id, score: 1, slot: int32(slot)})
		}
	}
	return out
}
