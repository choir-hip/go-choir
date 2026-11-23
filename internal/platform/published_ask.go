package platform

// A listener's ask, answered from the paper: the one read the Autoradio desk's
// turn makes of the paper's published stories (the defect, 2026-11-22: the
// radio was given 40 front-page titles and said it had no stories on Taiwan
// while the paper held 14 to 18; owner: "bad context engineering; it should do
// keyword and vector search over the corpus and rank by new/important/
// citations"). One request carries both halves: BM25 over the published
// stories (publication_pieces_index.go) and the nearest story passages by
// vector (vector_search.go, the query embedded by the platform's own query
// embedder). The rows carry what the caller ranks on (update time, importance,
// distinct sources) and the rank each half gave, and no more: the ranking
// formula is the caller's (internal/agentcore/api_autoradio_paper.go).
//
// Published stories only, as published/search: the route reads what any reader
// can already open and names no owner. Bounds on a stranger: the query's bytes,
// the result count and a deadline.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	// askQueryMaxBytes bounds the words one call may carry. A spoken ask is
	// a sentence; the caller cuts its words to this.
	askQueryMaxBytes = 300
	// askDefaultLimit and askMaxLimit are how many each half is asked for; the
	// answer holds at most twice the limit (the halves' union).
	askDefaultLimit = 15
	askMaxLimit     = 20
	// askMaxPassages is the most story passages one ask's vector query reads.
	// Measured on staging 2026-11-23 on the production shape (72,565 story
	// passages, an exact read): LIMIT 40, 45, 50, 60 and 80 took 67 to 97 ms;
	// LIMIT 100, 120 and 135 took 2.3 to 4.3 s, because Postgres changes plan
	// between 80 and 100. The first version asked for 135 and ran out its whole
	// budget on both proof turns, so the answer was the keyword half alone. 80 is
	// the last size measured fast: do not raise it without measuring again.
	askMaxPassages = 80
	// askOverfetch is how many candidates each half is asked for per story it
	// may return: an opinion piece, an owner-mode essay or a withdrawn story
	// can match too, and is dropped when its row is read, so a half asks for
	// more than the limit and keeps the first limit stories among them.
	askOverfetch = 3
)

// askDeadline bounds the whole read, the vector half and the row reads
// included. It is shorter than the Autoradio desk's own wait for the answer
// (1.5 s, internal/agentcore/api_autoradio_paper.go), so a slow index is
// answered as capped, and a slow vector half as partial, before the desk gives
// up and tells the model the search did not run. A variable so a test can
// shrink it.
var askDeadline = 1000 * time.Millisecond

// askRowBudget is the fresh time the row reads get after the index walk and
// the vector half have used (or run out of) askDeadline, so keyword rows that
// are already indexed are still read when the walk took the whole deadline.
// askDeadline plus askRowBudget stays under the desk's 1.5 s wait.
var askRowBudget = 250 * time.Millisecond

// ErrAskQuery is a query that cannot be answered: empty, or over the bound.
var ErrAskQuery = errors.New("the query must be 1 to 300 bytes")

// AskedStory is one published story with the ranks the halves gave it. A rank
// is 1 for the best and 0 when that half did not find the story.
type AskedStory struct {
	FrontStory
	KeywordRank int     `json:"keyword_rank,omitempty"`
	VectorRank  int     `json:"vector_rank,omitempty"`
	VectorScore float64 `json:"vector_score,omitempty"`
}

// PublishedAsk is the answer. Vector is "answered", or the reason the vector
// half did not run (the keyword rows are still answered). Capped says the
// keyword index is still being built. Partial says rows may be missing because
// some could not be read (a count, never a definite nothing).
type PublishedAsk struct {
	Query   string       `json:"query"`
	Stories []AskedStory `json:"stories"`
	Vector  string       `json:"vector"`
	Capped  bool         `json:"capped,omitempty"`
	Partial string       `json:"partial,omitempty"`
}

// askVectorHit is one story the vector half found, best first.
type askVectorHit struct {
	PublicationID string
	Score         float64
}

// askVectorPassageLimit is how many passages to read for n stories: up to twice
// the stories (a story has several passages), never past askMaxPassages. The
// query is by passage, so this is not n stories: the yield in distinct stories
// is logged with every ask (askVectors) and a shortfall is marked "short".
func askVectorPassageLimit(n int) int { return min(n*2, askMaxPassages) }

// askVectors runs the vector half: the nearest story passages for the query,
// one hit per publication, best first. why is empty when it answered.
func (s *Service) askVectors(ctx context.Context, query string, n int) ([]askVectorHit, string) {
	if s.askVectorHook != nil {
		return s.askVectorHook(ctx, query, n)
	}
	if s.store == nil {
		return nil, "no store"
	}
	passages := askVectorPassageLimit(n)
	started := time.Now()
	vr, why := s.store.searchVectorsBounded(ctx, VectorSearchRequest{Query: query, StoryLimit: passages})
	ms := time.Since(started).Milliseconds()
	if why != "" {
		log.Printf("corpusd: published ask vector: ms=%d asked=%d passages=0 stories=0 failed=%q", ms, passages, why)
		return nil, why
	}
	note := ""
	if vr.Partial != "" {
		// the list is what the index gave and may lack nearer rows: say so
		note = "partial: " + vr.Partial
	}
	var out []askVectorHit
	seen := map[string]bool{}
	for _, h := range vr.Stories {
		if seen[h.PublicationID] {
			continue
		}
		seen[h.PublicationID] = true
		out = append(out, askVectorHit{PublicationID: h.PublicationID, Score: h.Score})
		if len(out) == n {
			break
		}
	}
	// One line per ask: the vector query's time and yield, so the planner cliff
	// moving (askMaxPassages) is seen. A shortfall is when the passages gave
	// fewer distinct stories than the limit asked for.
	wanted := (n + askOverfetch - 1) / askOverfetch
	short := ""
	if len(out) < wanted {
		short = " short"
	}
	log.Printf("corpusd: published ask vector: ms=%d asked=%d passages=%d stories=%d wanted=%d%s", ms, passages, len(vr.Stories), len(out), wanted, short)
	return out, note
}

// AskPublished answers one ask. limit is how many stories each half is asked
// for (default 15, at most 20).
func (s *Service) AskPublished(ctx context.Context, query string, limit int) (*PublishedAsk, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	query = strings.TrimSpace(query)
	if query == "" || len(query) > askQueryMaxBytes {
		return nil, ErrAskQuery
	}
	if limit <= 0 {
		limit = askDefaultLimit
	}
	if limit > askMaxLimit {
		limit = askMaxLimit
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, askDeadline)
	defer cancel()

	// The vector half starts first and runs beside the keyword half: it is the
	// slower one (the query embedding, then the passage query), and it must not
	// wait for the index walk.
	type vectorAnswer struct {
		hits []askVectorHit
		why  string
	}
	vecDone := make(chan vectorAnswer, 1)
	go func() {
		// A panic here would end the process (no handler goroutine above it):
		// it becomes "the vector half did not answer", logged with its stack.
		defer func() {
			if r := recover(); r != nil {
				log.Printf("corpusd: published ask vector half panicked: %v\n%s", r, debug.Stack())
				vecDone <- vectorAnswer{why: "the vector half did not answer: it failed"}
			}
		}()
		h, w := s.askVectors(ctx, query, limit*askOverfetch)
		vecDone <- vectorAnswer{h, w}
	}()

	// An index that cannot catch up inside the deadline is answered from what
	// is indexed and says so (Capped); only a real store error fails the ask.
	if s.askRefreshHook != nil {
		s.askRefreshHook()
	}
	complete, err := s.refreshPieceIndex(ctx, publishedPieceSliceBudget, false)
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			cancel() // the goroutine ends on its own: vecDone is buffered
			return nil, fmt.Errorf("platform ask: %w", err)
		}
		complete = false
	}
	out := &PublishedAsk{Query: query, Stories: []AskedStory{}, Capped: !complete}

	var kwOrder []string
	if terms := distinctQueryTerms(query); len(terms) > 0 {
		cands, _ := s.pieces.rank(terms, limit*askOverfetch)
		for _, c := range cands {
			kwOrder = append(kwOrder, c.id)
		}
	}
	vector := map[string]askVectorHit{}
	var vecOrder []string
	// The join waits no longer than the ask's own deadline, whether or not the
	// vector half honours its context; a value already there is preferred.
	var va vectorAnswer
	select {
	case va = <-vecDone:
	case <-ctx.Done():
		select {
		case va = <-vecDone:
		default:
			va = vectorAnswer{why: "the vector half did not answer before the deadline"}
		}
	}
	hits, why := va.hits, va.why
	out.Vector = "answered"
	if why != "" {
		out.Vector = why
	}
	for _, h := range hits {
		if _, dup := vector[h.PublicationID]; dup {
			continue
		}
		vector[h.PublicationID] = h
		vecOrder = append(vecOrder, h.PublicationID)
	}

	// Each half's list is cut to the stories among its candidates (a candidate
	// that is not a published Autopaper story is dropped when its row is read),
	// and a story's rank is its place among stories. Rows are read once.
	// The rows are read under their own short budget, counted from here: the
	// walk and the vector half may have used all of askDeadline, and the
	// keyword rows already indexed are still worth reading.
	rctx, rcancel := context.WithTimeout(parent, askRowBudget)
	defer rcancel()
	rows := map[string]*FrontStory{}
	failed := 0
	read := func(id string) *FrontStory {
		if r, done := rows[id]; done {
			return r
		}
		rows[id] = nil
		if rctx.Err() != nil {
			failed++
			return nil
		}
		if s.askRowFault != nil {
			if err := s.askRowFault(id); err != nil {
				failed++
				return nil
			}
		}
		obj, err := og.GetObject(rctx, id)
		if err != nil {
			if !errors.Is(err, objectgraph.ErrNotFound) {
				failed++
			}
			return nil
		}
		if obj.Tombstone {
			return nil
		}
		story, ok, err := s.frontStory(rctx, og, frontPublicationRow{CanonicalID: id, Metadata: obj.Metadata, UpdatedAt: obj.UpdatedAt})
		if err != nil {
			failed++
			return nil
		}
		if !ok {
			return nil
		}
		rows[id] = &story
		return &story
	}
	storiesOf := func(order []string) []string {
		var kept []string
		for _, id := range order {
			if len(kept) == limit {
				break
			}
			if read(id) != nil {
				kept = append(kept, id)
			}
		}
		return kept
	}
	kwStories, vecStories := storiesOf(kwOrder), storiesOf(vecOrder)
	keyword, vecRank := map[string]int{}, map[string]int{}
	for i, id := range kwStories {
		keyword[id] = i + 1
	}
	for i, id := range vecStories {
		vecRank[id] = i + 1
	}
	// The union, the two halves interleaved best first.
	seen := map[string]bool{}
	var order []string
	for i := 0; i < len(kwStories) || i < len(vecStories); i++ {
		for _, list := range [][]string{kwStories, vecStories} {
			if i < len(list) && !seen[list[i]] {
				seen[list[i]] = true
				order = append(order, list[i])
			}
		}
	}
	stories := make([]FrontStory, len(order))
	for i, id := range order {
		stories[i] = *rows[id]
	}
	s.attachFrontImportance(stories)
	for i, id := range order {
		out.Stories = append(out.Stories, AskedStory{FrontStory: stories[i], KeywordRank: keyword[id], VectorRank: vecRank[id], VectorScore: vector[id].Score})
	}
	if failed > 0 {
		out.Partial = fmt.Sprintf("%d %s failed", failed, map[bool]string{true: "row read", false: "row reads"}[failed == 1])
		log.Printf("corpusd: published ask: %s (q=%d bytes, %d stories answered)", out.Partial, len(query), len(out.Stories))
	}
	return out, nil
}
