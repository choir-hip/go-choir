package platform

// The full-text index of captured pages (docs/reference/CAPTURE-FULLTEXT.md):
// every capture's clean text, in passages, ranked by BM25 in a SQLite FTS5
// file per computer (internal/capturefts). corpusd owns it, as it owns the
// capture projection beside it (capture_index.go).
//
// It is derived state. The authority is the capture object in og_objects and
// the text blob it names (CaptureBodyStore); the index holds no page text, so
// a lost or stale file costs a rebuild and nothing else. It is written after
// each capture, and a failed write never fails the capture: it is logged
// with the capture's id, flagged, and repaired by the catch-up.
//
// The catch-up is not a resumed work obligation (CLAUDE.md, Restarts). It
// re-derives a projection from its source of truth, as ReconcileCaptureIndex
// does at every boot: it opens no run, closes no obligation, reads no
// marker, and doing it twice, or never, changes no fact, only what a search
// can find. Until a page is indexed it is still found the old way, by title
// and labels (the guest merges both).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capturefts"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	// captureFullTextDirName is the index's directory under the platform
	// artifacts root. The artifact collector sweeps named namespaces only, so
	// this directory is outside it.
	captureFullTextDirName = "capture-fts"
	// captureSnippetRunes bounds the passage excerpt a hit carries.
	captureSnippetRunes = 420
	// captureFullTextMaxLimit bounds the pages one search returns.
	captureFullTextMaxLimit = 50
)

// captureFullTextSearchTimeout is the deadline of one text search. A query
// that scans more than this (a single very common word on a large index)
// answers "timed out" and the guest uses the title and label half alone.
var captureFullTextSearchTimeout = 8 * time.Second

// Variables so tests can shrink them.
var (
	// captureFullTextBatch is how many captures one catch-up batch reads.
	captureFullTextBatch = 100
	// captureFullTextDutyFactor is how long the catch-up rests after a batch,
	// as a multiple of the time the batch took: 2 keeps it to a third of one
	// core's time and leaves the rest to captures and searches.
	captureFullTextDutyFactor = 2.0
	// captureFullTextMaxWaitIdle bounds how long a batch waits for running
	// searches to finish before it goes on.
	captureFullTextMaxWaitIdle = 2 * time.Second
	// captureFullTextMaxConsecutiveFailures stops a walk: 20 writes in a row
	// failing is a full or failing disk, not 20 bad pages, and going on would
	// only count failures; the walk ends with an error and the next attempt
	// waits longer (doubling from 5 minutes to an hour).
	captureFullTextMaxConsecutiveFailures = 20
	// captureFullTextProgressEvery is how often the catch-up logs progress.
	captureFullTextProgressEvery = 30 * time.Second
	// captureFullTextYield is called between batches (tests count it).
	captureFullTextYield = func(context.Context) {}
)

type captureFullText struct {
	index  *capturefts.Index
	bodies *CaptureBodyStore
	// searching counts searches in flight; the catch-up lets them finish.
	searching atomic.Int32
	// dirty: an inline write failed or a removal could not be done; the next
	// catch-up pass repairs it.
	dirty    atomic.Bool
	kick     chan struct{}
	complete atomic.Bool
	catchMu  sync.Mutex
	started  sync.Once
	// failures counts inline write failures; kicks counts the catch-up
	// requests they made. failMu guards the log window and the kick time.
	failures              atomic.Int64
	kicks                 atomic.Int64
	failMu                sync.Mutex
	failLogAt, lastKickAt time.Time
	failSuppressed        int64
	// gap is captures minus indexed pages after the last full walk: the
	// captures that cannot be indexed (no text blob, owner-private, no
	// scope). -1 until a walk has measured it.
	gap atomic.Int64
	// writeHook, when set, replaces the index write (tests inject failures).
	writeHook func(objectgraph.Object) error
	// counts caches the number of captures per scope for captureCountTTL.
	countMu sync.Mutex
	counts  map[capturefts.Scope]cachedCount
}

type cachedCount struct {
	n  int
	at time.Time
}

// captureCountTTL is how long a computer's capture count, shown beside a
// search, is reused rather than counted again.
var captureCountTTL = time.Minute

// failureLogWindow is how often a run of write failures logs a line.
var failureLogWindow = 30 * time.Second

// kickBackoff is the least time between two catch-up requests made by write
// failures.
var kickBackoff = 5 * time.Minute

func (ft *captureFullText) resetCountCache() {
	ft.countMu.Lock()
	ft.counts = nil
	ft.countMu.Unlock()
}

// noteFailure records a failed inline write: counted, logged once per window
// with the count and the first id, and the catch-up requested at most once per
// backoff. The index is no longer known complete.
func (ft *captureFullText) noteFailure(id string, err error) {
	n := ft.failures.Add(1)
	ft.dirty.Store(true)
	ft.complete.Store(false)
	ft.failMu.Lock()
	defer ft.failMu.Unlock()
	now := time.Now()
	if now.Sub(ft.failLogAt) >= failureLogWindow {
		log.Printf("corpusd capture fulltext write failed count=%d suppressed=%d capture=%s err=%v (the catch-up will repair it)", n, ft.failSuppressed, id, err)
		ft.failLogAt, ft.failSuppressed = now, 0
	} else {
		ft.failSuppressed++
	}
	if now.Sub(ft.lastKickAt) >= kickBackoff {
		ft.lastKickAt = now
		ft.kicks.Add(1)
		select {
		case ft.kick <- struct{}{}:
		default:
		}
	}
}

// ConfigureCaptureFullText opens the full-text index in dir and turns on
// index writes and the search path. bodies is where capture text blobs are
// read.
func (s *Store) ConfigureCaptureFullText(dir string, bodies *CaptureBodyStore) error {
	if s == nil || bodies == nil {
		return errors.New("capture full text: store and body store are required")
	}
	ix, err := capturefts.Open(dir)
	if err != nil {
		return err
	}
	ft := &captureFullText{index: ix, bodies: bodies, kick: make(chan struct{}, 1)}
	ft.gap.Store(-1)
	s.fullText.Store(ft)
	return nil
}

// CaptureFullTextDir is the index's directory under the artifacts root.
func CaptureFullTextDir(artifactsRoot string) string {
	return strings.TrimRight(artifactsRoot, "/") + "/" + captureFullTextDirName
}

type ftAction int

const (
	ftSkip ftAction = iota
	ftIndex
	ftRemove
)

// captureFullTextPage reads what the index needs of a capture object. A
// capture that is deleted-in-place (tombstone) or superseded is to be
// removed; one with no text blob or no scope is skipped.
func captureFullTextPage(obj objectgraph.Object) (capturefts.Scope, capturefts.Page, string, ftAction) {
	if obj.ObjectKind != objectgraph.WebCaptureObjectKind {
		return capturefts.Scope{}, capturefts.Page{}, "", ftSkip
	}
	scope := capturefts.Scope{Owner: strings.TrimSpace(obj.OwnerID), Computer: strings.TrimSpace(obj.ComputerID)}
	if scope.Owner == "" || scope.Computer == "" {
		return scope, capturefts.Page{}, "", ftSkip
	}
	if obj.Tombstone || strings.TrimSpace(obj.SupersededBy) != "" || types.LabelsFromMetadata(obj.Metadata).OwnerPrivate() {
		// An owner-private capture is not served to a non-custodian
		// (agentcore capture_read_gate.go); a passage of it must not leave
		// through the search either, so it is not indexed.
		return scope, capturefts.Page{CaptureID: obj.CanonicalID}, "", ftRemove
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		return scope, capturefts.Page{}, "", ftSkip
	}
	hash := strings.TrimPrefix(strings.TrimSpace(meta.ExtractedTextBlobID), "sha256:")
	if len(hash) != 64 {
		return scope, capturefts.Page{}, "", ftSkip
	}
	return scope, capturefts.Page{
		CaptureID: obj.CanonicalID, Title: meta.Title, TextHash: meta.ExtractedTextBlobID,
		FetchedAt: parseCaptureTime(meta.FetchedAt).Time, PublishedAt: parseCaptureTime(meta.PublishedAt).Time,
	}, hash, ftIndex
}

// writeCaptureText indexes one capture's text (or removes a removed
// capture). It reads at most capturefts.MaxIndexedBytes of the blob.
func (ft *captureFullText) writeCaptureText(ctx context.Context, obj objectgraph.Object) error {
	if ft.writeHook != nil {
		return ft.writeHook(obj)
	}
	scope, page, hash, action := captureFullTextPage(obj)
	switch action {
	case ftRemove:
		return ft.index.Remove(ctx, scope, obj.CanonicalID)
	case ftIndex:
		text, err := ft.bodies.Prefix(hash, capturefts.MaxIndexedBytes+8)
		if err != nil {
			return fmt.Errorf("read text %s: %w", page.TextHash, err)
		}
		page.Text = string(text)
		_, err = ft.index.Put(ctx, scope, page)
		return err
	}
	return nil
}

// indexCaptureText is the inline write after captures are stored. It never
// fails the caller.
func (s *Store) indexCaptureText(ctx context.Context, captures []objectgraph.Object) {
	if s == nil {
		return
	}
	ft := s.fullText.Load()
	if ft == nil {
		return
	}
	for _, obj := range captures {
		done := s.retrievalTimer(retrievalFullTextWrite)
		err := ft.writeCaptureText(ctx, obj)
		done("inline", 1, err)
		if err != nil {
			ft.noteFailure(obj.CanonicalID, err)
		}
	}
}

// removeCaptureText drops a deleted capture's passages. The scope is read
// from the projection row, which still exists.
func (s *Store) removeCaptureText(ctx context.Context, id string) {
	if s == nil {
		return
	}
	ft := s.fullText.Load()
	if ft == nil {
		return
	}
	var owner, computer string
	err := s.corpus().QueryRowContext(ctx, `SELECT owner_id, computer_id FROM proj_capture_index WHERE capture_id = ?`, id).Scan(&owner, &computer)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err == nil {
		err = ft.index.Remove(ctx, capturefts.Scope{Owner: owner, Computer: computer}, id)
	}
	if err != nil {
		ft.noteFailure(id, fmt.Errorf("remove: %w", err))
	}
}

// CaptureFullTextHit is one page found by its text.
type CaptureFullTextHit struct {
	CaptureID string  `json:"capture_id"`
	Title     string  `json:"title,omitempty"`
	Score     float64 `json:"score"`
	PassageID int64   `json:"passage_id"`
	Ordinal   int     `json:"ordinal"`
	Passages  int     `json:"passages"`
	// Start and End are the best passage's byte range in the page's text.
	Start    int    `json:"start"`
	End      int    `json:"end"`
	TextHash string `json:"text_hash,omitempty"`
	// Snippet is the passage's excerpt around the query words.
	Snippet     string `json:"snippet,omitempty"`
	FetchedAt   string `json:"fetched_at,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	TitleMatch  bool   `json:"title_match"`
	BodyMatch   bool   `json:"body_match"`
}

// CaptureFullTextInfo says how the full-text half of a search went.
type CaptureFullTextInfo struct {
	// Used is true when the index answered (even with no hits).
	Used bool `json:"used"`
	// Reason is why it did not.
	Reason string `json:"reason,omitempty"`
	// Indexed is how many of the computer's pages the index holds, and
	// Captures how many captures the computer has: the rest are found by
	// title and labels only until the catch-up reaches them.
	Indexed  int64    `json:"indexed"`
	Captures int      `json:"captures"`
	Complete bool     `json:"complete"`
	Terms    []string `json:"terms,omitempty"`
	// Pruned are query words not searched in page text because no page holds
	// them or they are so common they carry no weight; TermCapped are the
	// most common words dropped to keep the query to 32 terms; Excluded are
	// the words and phrases excluded (-word); PhraseFallbacks are quoted
	// phrases found nowhere, searched as their words.
	Pruned          []string `json:"pruned,omitempty"`
	TermCapped      []string `json:"term_capped,omitempty"`
	Excluded        []string `json:"excluded,omitempty"`
	PhraseFallbacks []string `json:"phrase_fallbacks,omitempty"`
	// Pages is how many pages the ranked passages came from, before the limit.
	Pages  int  `json:"pages"`
	Capped bool `json:"capped,omitempty"`
}

// errBadCaptureSearch marks a request the caller got wrong (a 400).
var errBadCaptureSearch = errors.New("invalid capture search")

func parseSearchTime(field, raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: %s %q is not a date (use 2026-11-18 or RFC 3339)", errBadCaptureSearch, field, raw)
}

// searchCaptureFullText ranks the scope's pages by their text for req.Query
// and fills resp.Hits, resp.HitObjects and resp.FullText. A failure is
// reported in resp.FullText, never as an error: the old half still answers.
func (s *Store) searchCaptureFullText(ctx context.Context, owner, computer string, req CaptureSearchRequest, resp *CaptureSearchResponse) error {
	info := &CaptureFullTextInfo{}
	resp.FullText = info
	since, err := parseSearchTime("since", req.Since)
	if err != nil {
		return err
	}
	until, err := parseSearchTime("until", req.Until)
	if err != nil {
		return err
	}
	ft := s.fullText.Load()
	if ft == nil {
		info.Reason = "the full-text index is not configured on this host"
		return nil
	}
	done := s.retrievalTimer(retrievalFullTextSearch)
	var searchErr error
	defer func() { done("passages", len(resp.Hits), searchErr) }()
	ft.searching.Add(1)
	defer ft.searching.Add(-1)

	order := capturefts.ByRelevance
	if strings.EqualFold(strings.TrimSpace(req.Order), string(capturefts.ByRecency)) {
		order = capturefts.ByRecency
	}
	limit := req.Limit
	if limit <= 0 {
		limit = capturefts.DefaultLimit
	}
	limit = min(limit, captureFullTextMaxLimit)
	searchCtx, cancel := context.WithTimeout(ctx, captureFullTextSearchTimeout)
	defer cancel()
	res, err := ft.index.Search(searchCtx, capturefts.Scope{Owner: owner, Computer: computer}, capturefts.Query{Text: req.Query, Since: since, Until: until, Order: order, Limit: limit})
	if err != nil {
		searchErr = err
		log.Printf("corpusd capture fulltext search failed computer=%s err=%v", computer, err)
		info.Reason = "the full-text index failed: " + err.Error()
		return nil
	}
	info.Pruned, info.TermCapped, info.Excluded, info.PhraseFallbacks = res.Pruned, res.TermCapped, res.Excluded, res.PhraseFallbacks
	if res.Note != "" {
		// Nothing was searched: say so, do not answer as a search with no hits.
		info.Reason = res.Note
		return nil
	}
	info.Used = true
	info.Captures = ft.captureCount(ctx, s, owner, computer)
	info.Indexed = res.Indexed
	info.Complete = ft.complete.Load()
	info.Terms = res.Terms
	info.Pages = res.Pages
	info.Capped = res.Capped
	if len(res.Hits) == 0 {
		return nil
	}
	ids := make([]string, len(res.Hits))
	for i, h := range res.Hits {
		ids[i] = h.CaptureID
	}
	// Objects are read under the request's owner and computer in SQL: a page
	// the object graph does not hold for this scope is not returned, whatever
	// the index says.
	objs, err := s.captureObjectsByID(ctx, owner, computer, ids)
	if err != nil {
		searchErr = err
		info.Used = false
		info.Reason = "the full-text hits could not be read: " + err.Error()
		return nil
	}
	have := map[string]bool{}
	for _, o := range objs {
		// A capture that has become owner-private since it was indexed is
		// not served to a non-custodian; its passage does not leave here.
		if _, _, _, action := captureFullTextPage(o); action == ftIndex {
			have[o.CanonicalID] = true
		}
	}
	for _, h := range res.Hits {
		if !have[h.CaptureID] {
			ft.dirty.Store(true) // a page whose capture is gone: the catch-up removes it
			continue
		}
		out := CaptureFullTextHit{
			CaptureID: h.CaptureID, Title: h.Title, Score: h.Score, PassageID: h.PassageID, Ordinal: h.Ordinal, Passages: h.Passages,
			Start: h.Start, End: h.End, TextHash: h.TextHash, TitleMatch: h.TitleMatch,
		}
		if !h.FetchedAt.IsZero() {
			out.FetchedAt = h.FetchedAt.Format(time.RFC3339)
		}
		if !h.PublishedAt.IsZero() {
			out.PublishedAt = h.PublishedAt.Format(time.RFC3339)
		}
		if raw, rerr := ft.bodies.Range(strings.TrimPrefix(h.TextHash, "sha256:"), h.Start, h.End); rerr == nil {
			passage := string(raw)
			out.Snippet = capturefts.Snippet(passage, res.Terms, captureSnippetRunes)
			out.BodyMatch = len(capturefts.MatchedTerms(passage, res.Terms)) > 0
		}
		resp.Hits = append(resp.Hits, out)
	}
	resp.HitObjects = objs
	return nil
}

// captureCount is the number of captures the projection holds for a computer,
// cached for captureCountTTL: a scan of the projection per search is not
// worth the coverage note it feeds.
func (ft *captureFullText) captureCount(ctx context.Context, s *Store, owner, computer string) int {
	key := capturefts.Scope{Owner: owner, Computer: computer}
	ft.countMu.Lock()
	if c, ok := ft.counts[key]; ok && time.Since(c.at) < captureCountTTL {
		ft.countMu.Unlock()
		return c.n
	}
	ft.countMu.Unlock()
	var n int
	if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index WHERE owner_id = ? AND computer_id = ?`, owner, computer).Scan(&n); err != nil {
		return 0
	}
	ft.countMu.Lock()
	if ft.counts == nil {
		ft.counts = map[capturefts.Scope]cachedCount{}
	}
	ft.counts[key] = cachedCount{n, time.Now()}
	ft.countMu.Unlock()
	return n
}

// ---- catch-up ----

// CaptureFullTextProgress is the state of a catch-up pass.
type CaptureFullTextProgress struct {
	Scanned, Indexed, Skipped, Failed, Removed int
	Total                                      int
}

// StartCaptureFullTextCatchUp runs the catch-up once, a little after boot,
// and again when an inline write failed or once a day. It returns at once.
func (s *Store) StartCaptureFullTextCatchUp(ctx context.Context) {
	if s == nil {
		return
	}
	ft := s.fullText.Load()
	if ft == nil {
		return
	}
	ft.started.Do(func() {
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(20 * time.Second):
			}
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			backoff := 5 * time.Minute
			for {
				if _, err := s.CatchUpCaptureFullText(ctx); err != nil && ctx.Err() == nil {
					log.Printf("corpusd capture fulltext catch-up err=%v (next attempt in %s)", err, backoff)
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
					}
					backoff = min(2*backoff, time.Hour)
					continue
				}
				backoff = 5 * time.Minute
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				case <-ft.kick:
				}
			}
		}()
	})
}

// waitIdle lets searches in flight finish, up to captureFullTextMaxWaitIdle.
func (ft *captureFullText) waitIdle(ctx context.Context) {
	deadline := time.Now().Add(captureFullTextMaxWaitIdle)
	for ft.searching.Load() > 0 && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// CatchUpCaptureFullText indexes every capture the index is missing, in
// bounded batches, newest work first being impossible (captures page by
// primary key) but unneeded: captures made after the index was turned on are
// indexed as they are written. It is idempotent and safe to interrupt: a
// page already indexed from its text is skipped by a lookup, not rewritten.
// After the forward pass it removes index pages whose capture is gone.
func (s *Store) CatchUpCaptureFullText(ctx context.Context) (CaptureFullTextProgress, error) {
	var prog CaptureFullTextProgress
	if s == nil {
		return prog, errors.New("capture full text: nil store")
	}
	ft := s.fullText.Load()
	if ft == nil {
		return prog, errors.New("capture full text is not configured")
	}
	ft.catchMu.Lock()
	defer ft.catchMu.Unlock()
	dirty := ft.dirty.Swap(false)
	fail := func(err error) (CaptureFullTextProgress, error) {
		ft.dirty.Store(true)
		ft.complete.Store(false)
		return prog, err
	}
	consecutive := 0
	if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE canonical_id >= ? AND canonical_id < ? AND tombstone = FALSE`, captureIDPrefix, captureIDEnd).Scan(&prog.Total); err != nil {
		return fail(err)
	}
	// The count shortcut ReconcileCaptureIndex has: when nothing failed since
	// the last full walk and the captures minus the indexed pages are still
	// the gap that walk measured (the captures that cannot be indexed), there
	// is nothing to do.
	if !dirty {
		if gap := ft.gap.Load(); gap >= 0 {
			if pages, err := s.indexedPageCount(ctx, ft); err == nil && int64(prog.Total)-pages == gap {
				ft.complete.Store(true)
				return prog, nil
			}
		}
	}
	started := time.Now()
	lastLog := started
	expected := map[capturefts.Scope]int64{}
	cursor := captureIDPrefix
	for {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		batchStart := time.Now()
		rows, err := s.corpus().QueryContext(ctx, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
			FROM og_objects WHERE canonical_id > ? AND canonical_id < ? ORDER BY canonical_id LIMIT ?`, cursor, captureIDEnd, captureFullTextBatch)
		if err != nil {
			return fail(err)
		}
		var objs []objectgraph.Object
		for rows.Next() {
			obj, _, err := scanObjectGraphObject(rows)
			if err != nil {
				_ = rows.Close()
				return fail(err)
			}
			obj.Body = nil
			objs = append(objs, obj)
		}
		_ = rows.Close()
		if len(objs) == 0 {
			break
		}
		cursor = objs[len(objs)-1].CanonicalID
		done := s.retrievalTimer(retrievalFullTextCatchUp)
		bp, err := s.catchUpBatch(ctx, ft, objs, true, expected, &consecutive)
		done("batch", bp.Indexed, err)
		prog.Scanned += len(objs)
		prog.Indexed += bp.Indexed
		prog.Skipped += bp.Skipped
		prog.Failed += bp.Failed
		prog.Removed += bp.Removed
		if err != nil {
			return fail(err)
		}
		if bp.Failed > 0 {
			ft.dirty.Store(true)
		}
		if time.Since(lastLog) >= captureFullTextProgressEvery {
			lastLog = time.Now()
			logCaptureFullTextProgress(prog, started)
		}
		if len(objs) < captureFullTextBatch {
			break
		}
		// Rest in proportion to the work, and let searches finish first.
		captureFullTextYield(ctx)
		ft.waitIdle(ctx)
		if rest := time.Duration(float64(time.Since(batchStart)) * captureFullTextDutyFactor); rest > 0 && bp.Indexed > 0 {
			select {
			case <-ctx.Done():
				return fail(ctx.Err())
			case <-time.After(rest):
			}
		}
	}
	// Pages whose capture is gone (a removal that failed once).
	removed, err := s.sweepCaptureFullTextOrphans(ctx, ft, expected)
	prog.Removed += removed
	if err != nil {
		return fail(err)
	}
	if prog.Failed == 0 && !ft.dirty.Load() {
		if pages, err := s.indexedPageCount(ctx, ft); err == nil {
			ft.gap.Store(int64(prog.Total) - pages)
		}
		ft.complete.Store(true)
	}
	logCaptureFullTextProgress(prog, started)
	return prog, nil
}

// indexedPageCount is the pages held across every scope's file.
func (s *Store) indexedPageCount(ctx context.Context, ft *captureFullText) (int64, error) {
	scopes, err := ft.index.Scopes(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, scope := range scopes {
		n += ft.index.Pages(scope)
	}
	return n, nil
}

func logCaptureFullTextProgress(p CaptureFullTextProgress, started time.Time) {
	elapsed := time.Since(started).Seconds()
	rate := 0.0
	if elapsed > 0 {
		rate = float64(p.Scanned) / elapsed
	}
	remaining := max(p.Total-p.Scanned, 0)
	log.Printf("corpusd capture fulltext catch-up scanned=%d indexed=%d skipped=%d failed=%d removed=%d total=%d remaining=%d rate=%.0f/s elapsed=%.0fs",
		p.Scanned, p.Indexed, p.Skipped, p.Failed, p.Removed, p.Total, remaining, rate, elapsed)
}

// catchUpBatch indexes the captures among objs the index does not hold.
func (s *Store) catchUpBatch(ctx context.Context, ft *captureFullText, objs []objectgraph.Object, repairRemovals bool, expected map[capturefts.Scope]int64, consecutive *int) (CaptureFullTextProgress, error) {
	var bp CaptureFullTextProgress
	type item struct {
		obj  objectgraph.Object
		page capturefts.Page
		hash string
	}
	byScope := map[capturefts.Scope][]item{}
	for _, obj := range objs {
		scope, page, hash, action := captureFullTextPage(obj)
		switch action {
		case ftIndex:
			expected[scope]++
			byScope[scope] = append(byScope[scope], item{obj, page, hash})
		case ftRemove:
			if repairRemovals {
				if err := ft.index.Remove(ctx, scope, obj.CanonicalID); err != nil {
					bp.Failed++
					log.Printf("corpusd capture fulltext catch-up remove failed capture=%s err=%v", obj.CanonicalID, err)
					continue
				}
				bp.Removed++
			}
		default:
			bp.Skipped++
		}
	}
	for scope, items := range byScope {
		refs := make([]capturefts.Ref, len(items))
		for i, it := range items {
			refs[i] = capturefts.Ref{CaptureID: it.page.CaptureID, TextHash: it.page.TextHash}
		}
		missing, err := ft.index.Missing(ctx, scope, refs)
		if err != nil {
			return bp, err
		}
		need := map[string]bool{}
		for _, m := range missing {
			need[m.CaptureID] = true
		}
		for _, it := range items {
			if !need[it.page.CaptureID] {
				bp.Skipped++
				continue
			}
			if err := ctx.Err(); err != nil {
				return bp, err
			}
			if err := ft.writeCaptureText(ctx, it.obj); err != nil {
				bp.Failed++
				ft.noteFailure(it.obj.CanonicalID, err) // counted and logged in a window, as an inline failure is
				*consecutive++
				if *consecutive >= captureFullTextMaxConsecutiveFailures {
					return bp, fmt.Errorf("catch-up stopped after %d consecutive write failures: %w", *consecutive, err)
				}
				continue
			}
			*consecutive = 0
			bp.Indexed++
		}
	}
	return bp, nil
}

// sweepCaptureFullTextOrphans removes index pages whose capture no longer
// exists or is removed, in scopes whose index holds more pages than the
// forward pass found indexable captures.
func (s *Store) sweepCaptureFullTextOrphans(ctx context.Context, ft *captureFullText, expected map[capturefts.Scope]int64) (int, error) {
	scopes, err := ft.index.Scopes(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, scope := range scopes {
		if ft.index.Pages(scope) <= expected[scope] {
			continue
		}
		after := ""
		for {
			ids, err := ft.index.PageIDs(ctx, scope, after, 400)
			if err != nil {
				return removed, err
			}
			if len(ids) == 0 {
				break
			}
			after = ids[len(ids)-1]
			args := make([]any, 0, len(ids)+2)
			for _, id := range ids {
				args = append(args, id)
			}
			args = append(args, scope.Owner, scope.Computer)
			rows, err := s.corpus().QueryContext(ctx, `SELECT canonical_id, metadata, tombstone, superseded_by FROM og_objects WHERE canonical_id IN (?`+strings.Repeat(",?", len(ids)-1)+`)
				AND owner_id = ? AND computer_id = ?`, args...)
			if err != nil {
				return removed, err
			}
			live := map[string]bool{}
			for rows.Next() {
				obj := objectgraph.Object{ObjectKind: objectgraph.WebCaptureObjectKind, OwnerID: scope.Owner, ComputerID: scope.Computer}
				var meta string
				if err := rows.Scan(&obj.CanonicalID, &meta, &obj.Tombstone, &obj.SupersededBy); err != nil {
					_ = rows.Close()
					return removed, err
				}
				obj.Metadata = json.RawMessage(meta)
				// Gone, tombstoned, superseded or owner-private: not live.
				if _, _, _, action := captureFullTextPage(obj); action == ftIndex {
					live[obj.CanonicalID] = true
				}
			}
			err = rows.Err()
			_ = rows.Close()
			if err != nil {
				return removed, err
			}
			for _, id := range ids {
				if live[id] {
					continue
				}
				if err := ft.index.Remove(ctx, scope, id); err != nil {
					return removed, err
				}
				removed++
			}
			if len(ids) < 400 {
				break
			}
		}
	}
	return removed, nil
}
