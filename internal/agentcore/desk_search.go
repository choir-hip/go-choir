package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Async search, host side (Cut 2 unit 2a,
// docs/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md).
// choir.SearchStart reaches startSearch through the cell's egress frame. The
// host opens the gateway's NDJSON stream, pushes its start to the desk
// worker, answers the cell at once, and forwards each provider's answer to
// the worker as it lands, as its own frame (yaegikernel.StreamSearchArrival).
// A host call cannot outlive its cell, so the arrivals do not ride one: the
// worker's reader loop writes them into the session's search store.
//
// The search table is the authority for what is still streaming; the
// worker's store is a copy. Every stream belongs to one desk session and is
// cancelled when that session ends (release, reclaim, replacement, or
// closeAll). Nothing here wakes a desk: an arrival after the desk ended its
// turn waits in the store for the next cell's count line.

// How much search text a desk's context carries. A search-printing cell
// holds up to 2k tokens (the 8,000-character cell cap) that every later call
// of the run resends: 29% of a research run's input
// (docs/reports/autopaper-compute-halving-2026-10-15.md). A hit is about
// 200 characters of title and URL plus its snippet, so 12 hits at 300
// characters fit in about 6k characters where 40 hits at full-page snippets
// hit the cap.
//
// Owner ruling (2026-10-15): do not economize on search; keep the
// append-only prompt pattern so the provider prompt cache stays above 90%.
// The defaults are therefore the earlier behaviour (40 results, snippets
// whole). Opt in to trimming with CHOIR_DESK_SEARCH_MAX_RESULTS=12 and
// CHOIR_DESK_SEARCH_SNIPPET_CHARS=300 (every computer), or with the owner's
// research.search_caps setting (one computer, live), which uses the capped
// profile below when the env names no value.
const (
	defaultDeskSearchMaxResults   = 40
	defaultDeskSearchSnippetChars = 0
	cappedDeskSearchMaxResults    = 12
	cappedDeskSearchSnippetChars  = 300
	searchSnippetCutMarker        = "…"
)

// deskSearchMaxResults is how many results each provider is asked for. The
// gateway honours it before it marks hits as seen, so a hit dropped here
// can appear in a later search. An env value that is a positive integer
// wins; otherwise capsOn (the computer's research.search_caps setting) gives
// 12 and off gives 40.
func deskSearchMaxResults(capsOn bool) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("CHOIR_DESK_SEARCH_MAX_RESULTS"))); err == nil && n > 0 {
		return n
	}
	if capsOn {
		return cappedDeskSearchMaxResults
	}
	return defaultDeskSearchMaxResults
}

// deskSearchSnippetChars is how many characters of a hit's snippet the desk
// keeps: 0 keeps it whole. An env value of zero or more wins; a negative or
// non-numeric one is ignored; otherwise capsOn gives 300 and off gives
// the whole snippet.
func deskSearchSnippetChars(capsOn bool) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("CHOIR_DESK_SEARCH_SNIPPET_CHARS"))); err == nil && n >= 0 {
		return n
	}
	if capsOn {
		return cappedDeskSearchSnippetChars
	}
	return defaultDeskSearchSnippetChars
}

// trimSearchSnippet keeps the first limit runes of a snippet and ends a cut
// one with a marker. limit 0 keeps it whole. Titles and URLs are never cut.
func trimSearchSnippet(snippet string, limit int) string {
	snippet = strings.TrimSpace(snippet)
	if limit <= 0 || utf8.RuneCountInString(snippet) <= limit {
		return snippet
	}
	runes := []rune(snippet)
	return strings.TrimSpace(string(runes[:limit])) + searchSnippetCutMarker
}

// searchClient is the computer's gateway search client, nil when none.
func (rt *Runtime) searchClient() search.Client {
	if rt == nil || rt.researchDeps == nil {
		return nil
	}
	return rt.researchDeps.Search
}

// deskSearches is one desk session's search table.
type deskSearches struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	next   int
	live   map[string]context.CancelFunc
}

func newDeskSearches() *deskSearches {
	ctx, cancel := context.WithCancel(context.Background())
	return &deskSearches{ctx: ctx, cancel: cancel, live: map[string]context.CancelFunc{}}
}

// begin numbers a new search and gives it a context that ends with the
// session. ok is false once the session has ended.
func (d *deskSearches) begin() (id string, ctx context.Context, cancel context.CancelFunc, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ctx.Err() != nil {
		return "", nil, nil, false
	}
	d.next++
	id = fmt.Sprintf("s%d", d.next)
	ctx, cancel = context.WithCancel(d.ctx)
	d.live[id] = cancel
	return id, ctx, cancel, true
}

// end forgets a search whose stream has finished.
func (d *deskSearches) end(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if cancel := d.live[id]; cancel != nil {
		cancel()
		delete(d.live, id)
	}
}

// close cancels every stream of the session.
func (d *deskSearches) close() {
	if d != nil {
		d.cancel()
	}
}

// searchDone is what the host records when a search's stream ends.
type searchDone struct {
	id       string
	query    string
	started  time.Time
	answered int
	arrived  int
	total    int
	pending  []string
	err      string
}

// startSearch serves choir.SearchStart for the session key's worker. It
// returns once the gateway has named the providers it started; record is
// called when the stream ends, before the worker hears of it.
func (m *deskSessionWorkers) startSearch(ctx context.Context, key string, worker *yaegikernel.DeskSessionWorker, client search.Client, payload json.RawMessage, capsOn bool, record func(searchDone)) (json.RawMessage, error) {
	var req yaegikernel.SearchStartPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("search_start: decode: %w", err)
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("search_start: query must not be empty")
	}
	if client == nil {
		return nil, fmt.Errorf("search_start host unavailable: no search gateway on this computer")
	}
	m.mu.Lock()
	s := m.sessions[key]
	var searches *deskSearches
	if s != nil && worker != nil && s.worker == worker {
		searches = s.searches
	}
	m.mu.Unlock()
	if searches == nil {
		return nil, fmt.Errorf("search_start: this desk session has ended")
	}
	id, streamCtx, cancel, ok := searches.begin()
	if !ok {
		return nil, fmt.Errorf("search_start: this desk session has ended")
	}
	// The stream lives with the session, but waiting for its start line is
	// part of this cell's call: the cell's end abandons it.
	stopWithCell := context.AfterFunc(ctx, cancel)
	stream, err := client.Stream(streamCtx, query, deskSearchMaxResults(capsOn))
	cellEnded := !stopWithCell()
	if err == nil && cellEnded {
		_ = stream.Close()
		err = ctx.Err()
	}
	if err != nil {
		searches.end(id)
		return nil, err
	}
	cooling := make([]string, 0, len(stream.Start.Cooling))
	for _, c := range stream.Start.Cooling {
		cooling = append(cooling, c.Provider)
	}
	start := yaegikernel.SearchFrame{ID: id, Kind: yaegikernel.SearchFrameStart, Query: query, Providers: stream.Start.Providers, Cooling: cooling}
	// The start frame goes out before the cell's answer, on the same
	// socket, so the worker's store has the search when SearchStart returns.
	if err := worker.PushSearch(start); err != nil {
		_ = stream.Close()
		searches.end(id)
		return nil, fmt.Errorf("search_start: %w", err)
	}
	go forwardSearch(streamCtx, worker, stream, id, query, deskSearchSnippetChars(capsOn), func(done searchDone) {
		searches.end(id)
		if record != nil {
			record(done)
		}
	})
	return json.Marshal(yaegikernel.SearchStartResult{ID: id, Query: query, Providers: stream.Start.Providers, Cooling: cooling})
}

// forwardSearch pushes each provider's answer to the worker as it lands,
// then the search's end. A stream cancelled with its session ends quietly:
// the worker is gone.
func forwardSearch(ctx context.Context, worker *yaegikernel.DeskSessionWorker, stream *search.Stream, id, query string, snippetChars int, finish func(searchDone)) {
	defer func() { _ = stream.Close() }()
	done := searchDone{id: id, query: query, started: time.Now(), total: len(stream.Start.Providers)}
	for {
		a, err := stream.Next()
		if errors.Is(err, io.EOF) {
			if final, ok := stream.Done(); ok {
				done.pending = final.Pending
			}
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				done.err = "the desk session ended"
				finish(done)
				return
			}
			done.err = err.Error()
			break
		}
		arrival := yaegikernel.SearchArrival{Provider: a.Provider, Status: a.Status, LatencyMs: a.LatencyMs, Error: a.Error}
		for _, r := range a.Results {
			arrival.Results = append(arrival.Results, yaegikernel.SearchHit{Title: r.Title, URL: r.URL, Snippet: trimSearchSnippet(r.Snippet, snippetChars), PublishedAt: r.PublishedAt, Score: r.Score, Provider: r.Provider})
		}
		done.arrived++
		if arrival.Answered() {
			done.answered++
		}
		if err := worker.PushSearch(yaegikernel.SearchFrame{ID: id, Kind: yaegikernel.SearchFrameProvider, Arrival: &arrival}); err != nil {
			done.err = "worker gone: " + err.Error()
			finish(done)
			return
		}
	}
	finish(done)
	_ = worker.PushSearch(yaegikernel.SearchFrame{ID: id, Kind: yaegikernel.SearchFrameDone, Pending: done.pending, Error: done.err})
}
