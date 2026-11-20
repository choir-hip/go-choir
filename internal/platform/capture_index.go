package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/captureterms"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The typed capture projection and its term table (rungs 2-3 of
// docs/archive/reports/object-graph-retrieval-research-2026-10-14.md). og_objects
// stays the authority: both tables are rebuilt from it, are written after
// each capture (a failed projection write never fails the capture). Corpus search and URL citation lookup read them over all
// history instead of the newest 1,000 captures.

const (
	captureIndexTable = "proj_capture_index"
	captureTermsTable = "proj_capture_terms"
	// captureTermsCoverIdx lets the term search read its postings from the
	// index alone (see ensureCaptureTermsCoverIndex).
	captureTermsCoverIdx = "idx_pct_cover"
	// captureScopeChars bounds owner and computer ids in the projection's
	// keys; a longer id is not projected and is searched by the window
	// path only.
	captureScopeChars = 128
	// captureIDPrefix and captureIDEnd bound every capture's canonical id
	// ("obj:choir.web_capture:<owner>:<suffix>"): the backfill pages
	// through captures by primary key.
	captureIDPrefix = "obj:" + string(objectgraph.WebCaptureObjectKind) + ":"
	captureIDEnd    = "obj:" + string(objectgraph.WebCaptureObjectKind) + ";"
	// captureBackfillPage is one backfill read.
	captureBackfillPage = 250
)

// captureRecountDelay separates the two counts of a reconcile check.
var captureRecountDelay = 2 * time.Second

// captureBackfillPause separates two written backfill pages.
var captureBackfillPause = 250 * time.Millisecond

// kickReconcile wakes the periodic reconciler (non-blocking).
func (s *Store) kickReconcile() {
	select {
	case s.retrievalState.reconcileKick <- struct{}{}:
	default:
	}
}

const captureIndexDDL = `
CREATE TABLE IF NOT EXISTS proj_capture_index (
	capture_id    TEXT NOT NULL PRIMARY KEY,
	owner_id      TEXT NOT NULL,
	computer_id   TEXT NOT NULL DEFAULT '',
	url_key       TEXT NOT NULL,
	canonical_key TEXT NOT NULL,
	url           TEXT NOT NULL,
	canonical_url TEXT NOT NULL,
	host          TEXT NOT NULL DEFAULT '',
	source_id     TEXT NOT NULL DEFAULT '',
	item_id       TEXT NOT NULL DEFAULT '',
	language      TEXT NOT NULL DEFAULT '',
	title         TEXT NOT NULL,
	has_blocks    BOOLEAN NOT NULL DEFAULT FALSE,
	fetched_at    TIMESTAMPTZ NULL,
	published_at  TIMESTAMPTZ NULL,
	updated_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pci_url ON proj_capture_index(owner_id, computer_id, url_key, fetched_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_pci_canonical ON proj_capture_index(owner_id, computer_id, canonical_key, fetched_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_pci_fetched ON proj_capture_index(owner_id, computer_id, fetched_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_pci_host ON proj_capture_index(owner_id, computer_id, host, fetched_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_pci_source ON proj_capture_index(owner_id, computer_id, source_id, fetched_at DESC NULLS LAST);

CREATE TABLE IF NOT EXISTS proj_capture_terms (
	owner_id    TEXT NOT NULL,
	computer_id TEXT NOT NULL,
	term        TEXT COLLATE "C" NOT NULL,
	capture_id  TEXT NOT NULL,
	fetched_at  TIMESTAMPTZ NULL,
	PRIMARY KEY (owner_id, computer_id, term, capture_id)
);
CREATE INDEX IF NOT EXISTS idx_pct_capture ON proj_capture_terms(capture_id);
`

// ensureCaptureIndexTables creates both projection tables (idempotent) and
// turns on projection writes.
func (s *Store) ensureCaptureIndexTables(ctx context.Context) error {
	if _, err := s.corpus().ExecContext(ctx, captureIndexDDL); err != nil {
		return fmt.Errorf("create capture projection: %w", err)
	}
	s.retrievalState.captureIndexReady.Store(true)
	return s.ensureCaptureResolutionTable(ctx)
}

// captureURLKey is the lookup key of a capture URL: the sha256 of the URL
// as Texture's citation lookup compares it (trimmed, trailing "/" removed).
func captureURLKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimRight(strings.TrimSpace(raw), "/")))
	return hex.EncodeToString(sum[:])
}

func captureHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

func parseCaptureTime(raw string) sql.NullTime {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sql.NullTime{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return sql.NullTime{Time: t.UTC(), Valid: true}
		}
	}
	return sql.NullTime{}
}

// captureProjection is one capture's projection rows.
type captureProjection struct {
	id, owner, computer             string
	urlKey, canonicalKey            string
	url, canonicalURL, host, source string
	itemID, language, title         string
	hasBlocks                       bool
	fetched, published              sql.NullTime
	updated                         time.Time
	terms                           []string
}

// projectCapture derives a capture's projection. ok is false for an object
// that is not a projectable capture (another kind, unreadable metadata, a
// tombstone, an over-long scope id).
func projectCapture(obj objectgraph.Object) (captureProjection, bool) {
	if obj.ObjectKind != objectgraph.WebCaptureObjectKind || obj.Tombstone {
		return captureProjection{}, false
	}
	if utf8.RuneCountInString(obj.OwnerID) > captureScopeChars || utf8.RuneCountInString(obj.ComputerID) > captureScopeChars {
		return captureProjection{}, false
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		return captureProjection{}, false
	}
	host := captureHost(meta.URL)
	p := captureProjection{
		id: obj.CanonicalID, owner: obj.OwnerID, computer: obj.ComputerID,
		urlKey: captureURLKey(meta.URL), canonicalKey: captureURLKey(meta.CanonicalURL),
		url: meta.URL, canonicalURL: meta.CanonicalURL, host: clipRunes(host, 255),
		source: clipRunes(meta.SourceID, 255), itemID: clipRunes(meta.ItemID, 255),
		language: clipRunes(meta.Language, 64), title: meta.Title,
		hasBlocks: meta.BlocksBlobID != "", fetched: parseCaptureTime(meta.FetchedAt),
		published: parseCaptureTime(meta.PublishedAt), updated: obj.UpdatedAt.UTC(),
	}
	// The fields the corpus search's Go matcher reads from the capture
	// itself (its labels are the guest's and are sent by id).
	p.terms = captureterms.Terms(strings.Join([]string{meta.Title, host, meta.SourceID, meta.Language}, "\n"))
	return p, true
}

// indexCaptures writes the projection of the captures among objs, in one
// transaction. fresh says the captures were just inserted (no projection
// rows can exist), which saves the deletes. It never fails the caller: an
// error is counted, logged, and left to the reconciler, woken now.
func (s *Store) indexCaptures(ctx context.Context, fresh bool, objs ...objectgraph.Object) {
	if s == nil {
		return
	}
	var captures []objectgraph.Object
	for _, obj := range objs {
		if obj.ObjectKind == objectgraph.WebCaptureObjectKind {
			captures = append(captures, obj)
		}
	}
	if len(captures) == 0 {
		return
	}
	// Every capture write path ends here (a put, a create, a batch: the
	// capture route, a source poll and a research read all arrive through
	// them), so this is where the embed worker is told. The hand-off is
	// deferred, so it runs whether or not the projection below is written:
	// the worker's eligibility check reads the object graph, not the
	// projection. A wake is not a call: the ids join a set, one wake is held,
	// and a worker that is idle embeds exactly these captures that lack a
	// vector, without a full pass. While a pass is doing work the step does
	// not interleave: the next pass covers them
	// (docs/design/entity-layer-2026-11-20.md, 4.1; in production a capture
	// waited a median 115 s for its vector, 2026-11-20).
	ids := make([]string, len(captures))
	for i, c := range captures {
		ids[i] = c.CanonicalID
	}
	defer s.NoteCapturesWritten(ids...)
	if !s.retrievalState.captureIndexReady.Load() {
		return
	}
	// The pages' text is indexed beside the projection; its failure is its
	// own (logged with ids, repaired by the catch-up).
	defer s.indexCaptureText(ctx, captures)
	// A lead for the page's URL learns it was captured (search_leads.go).
	defer s.markSearchLeadsCaptured(ctx, captures)
	done := s.retrievalTimer(retrievalCaptureIndexWrite)
	err := s.writeCaptureProjections(ctx, fresh, captures)
	if err != nil {
		// The projection misses these captures until the reconciler
		// repairs it; search stays on (the guest's URL lookup still checks
		// the window on a miss).
		s.retrievalState.captureIndexDirty.Store(true)
		s.kickReconcile()
	}
	done("inline", len(captures), err)
}

// Multi-row statement bounds (placeholders per statement stay far below
// the protocol's 65,535).
const (
	captureIndexRowsPerInsert = 200
	captureTermRowsPerInsert  = 1000
)

// writeCaptureProjections replaces the projection rows of captures in one
// transaction with multi-row statements: a capture that is not projectable
// has its rows removed. Every statement costs a round trip, so a backfill
// page is a handful of statements, not three per capture.
func (s *Store) writeCaptureProjections(ctx context.Context, fresh bool, objs []objectgraph.Object) error {
	if len(objs) == 0 {
		return nil
	}
	var rows []captureProjection
	var clear []any
	rowAt := map[string]int{}
	for _, obj := range objs {
		p, ok := projectCapture(obj)
		if ok {
			// One row per capture id: an upsert statement may not touch
			// a row twice.
			if at, dup := rowAt[p.id]; dup {
				rows[at] = p
			} else {
				rowAt[p.id] = len(rows)
				rows = append(rows, p)
			}
		}
		if !fresh || !ok {
			clear = append(clear, obj.CanonicalID)
		}
	}
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if len(clear) > 0 {
		in := strings.TrimSuffix(strings.Repeat("?, ", len(clear)), ", ")
		if _, err := tx.ExecContext(ctx, `DELETE FROM proj_capture_terms WHERE capture_id IN (`+in+`)`, clear...); err != nil {
			return fmt.Errorf("clear capture terms: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM proj_capture_index WHERE capture_id IN (`+in+`)`, clear...); err != nil {
			return fmt.Errorf("clear capture index: %w", err)
		}
	}
	for start := 0; start < len(rows); start += captureIndexRowsPerInsert {
		chunk := rows[start:min(start+captureIndexRowsPerInsert, len(rows))]
		args := make([]any, 0, 16*len(chunk))
		for _, p := range chunk {
			args = append(args, p.id, p.owner, p.computer, p.urlKey, p.canonicalKey, p.url, p.canonicalURL, p.host, p.source, p.itemID,
				p.language, p.title, p.hasBlocks, p.fetched, p.published, p.updated)
		}
		values := strings.TrimSuffix(strings.Repeat("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?), ", len(chunk)), ", ")
		if _, err := tx.ExecContext(ctx, `INSERT INTO proj_capture_index
			(capture_id, owner_id, computer_id, url_key, canonical_key, url, canonical_url, host, source_id, item_id, language, title, has_blocks, fetched_at, published_at, updated_at)
			VALUES `+values+` ON CONFLICT (capture_id) DO UPDATE SET
			owner_id = excluded.owner_id, computer_id = excluded.computer_id, url_key = excluded.url_key,
			canonical_key = excluded.canonical_key, url = excluded.url, canonical_url = excluded.canonical_url,
			host = excluded.host, source_id = excluded.source_id, item_id = excluded.item_id,
			language = excluded.language, title = excluded.title, has_blocks = excluded.has_blocks,
			fetched_at = excluded.fetched_at, published_at = excluded.published_at, updated_at = excluded.updated_at`, args...); err != nil {
			return fmt.Errorf("write capture index: %w", err)
		}
	}
	var termArgs []any
	n := 0
	flushTerms := func() error {
		if n == 0 {
			return nil
		}
		values := strings.TrimSuffix(strings.Repeat("(?, ?, ?, ?, ?), ", n), ", ")
		_, err := tx.ExecContext(ctx, `INSERT INTO proj_capture_terms (owner_id, computer_id, term, capture_id, fetched_at) VALUES `+values+` ON CONFLICT DO NOTHING`, termArgs...)
		termArgs, n = termArgs[:0], 0
		if err != nil {
			return fmt.Errorf("write capture terms: %w", err)
		}
		return nil
	}
	for _, p := range rows {
		for _, term := range p.terms {
			termArgs = append(termArgs, p.owner, p.computer, term, p.id, p.fetched)
			n++
			if n == captureTermRowsPerInsert {
				if err := flushTerms(); err != nil {
					return err
				}
			}
		}
	}
	if err := flushTerms(); err != nil {
		return err
	}
	return tx.Commit()
}

// unindexCapture removes a deleted capture's projection rows.
func (s *Store) unindexCapture(ctx context.Context, id string) {
	if s == nil || !s.retrievalState.captureIndexReady.Load() || !strings.HasPrefix(id, captureIDPrefix) {
		return
	}
	s.removeCaptureText(ctx, id) // before the projection row, which holds the scope
	_, err1 := s.corpus().ExecContext(ctx, `DELETE FROM proj_capture_terms WHERE capture_id = ?`, id)
	_, err2 := s.corpus().ExecContext(ctx, `DELETE FROM proj_capture_index WHERE capture_id = ?`, id)
	if err1 != nil || err2 != nil {
		s.retrievalState.captureIndexDirty.Store(true)
		log.Printf("corpusd retrieval unindex capture=%s err=%v %v", id, err1, err2)
	}
}

// ReconcileCaptureIndex makes the projection match og_objects: when the
// counts differ (or an inline write failed), it pages through every
// capture by primary key and writes the projection of each one missing
// from it, then marks the projection complete. It is idempotent and safe
// to interrupt; the next pass resumes from the start and skips what is
// there.
func (s *Store) ReconcileCaptureIndex(ctx context.Context) error {
	if s == nil || !s.retrievalState.captureIndexReady.Load() {
		return fmt.Errorf("capture projection not ready")
	}
	s.retrievalState.reconcileMu.Lock()
	defer s.retrievalState.reconcileMu.Unlock()
	done := s.retrievalTimer(retrievalCaptureIndexSync)
	written, err := s.reconcileCaptureIndex(ctx)
	shape := "checked"
	if written > 0 {
		shape = "backfilled"
	}
	done(shape, written, err)
	return err
}

func (s *Store) reconcileCaptureIndex(ctx context.Context) (int, error) {
	dirty := s.retrievalState.captureIndexDirty.Swap(false)
	if !dirty {
		var captures, projected int
		if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE canonical_id >= ? AND canonical_id < ? AND tombstone = FALSE`, captureIDPrefix, captureIDEnd).Scan(&captures); err != nil {
			return 0, err
		}
		if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index`).Scan(&projected); err != nil {
			return 0, err
		}
		// The gap is the captures the last full pass found unprojectable
		// (unreadable metadata, an over-long scope id): they never get a
		// row, and must not force a pass on every check.
		if captures-projected != int(s.retrievalState.captureIndexGap.Load()) {
			// A capture committed a moment before its projection row
			// makes the counts differ briefly; count again before a
			// full pass.
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(captureRecountDelay):
			}
			if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE canonical_id >= ? AND canonical_id < ? AND tombstone = FALSE`, captureIDPrefix, captureIDEnd).Scan(&captures); err != nil {
				return 0, err
			}
			if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index`).Scan(&projected); err != nil {
				return 0, err
			}
		}
		if captures-projected == int(s.retrievalState.captureIndexGap.Load()) {
			s.retrievalState.captureIndexComplete.Store(true)
			return 0, nil
		}
	}
	written := 0
	cursor := captureIDPrefix
	for {
		if err := ctx.Err(); err != nil {
			s.retrievalState.captureIndexDirty.Store(true)
			return written, err
		}
		rows, err := s.corpus().QueryContext(ctx, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
			FROM og_objects WHERE canonical_id > ? AND canonical_id < ? ORDER BY canonical_id LIMIT ?`, cursor, captureIDEnd, captureBackfillPage)
		if err != nil {
			s.retrievalState.captureIndexDirty.Store(true)
			return written, err
		}
		var objs []objectgraph.Object
		for rows.Next() {
			obj, _, err := scanObjectGraphObject(rows)
			if err != nil {
				_ = rows.Close()
				s.retrievalState.captureIndexDirty.Store(true)
				return written, err
			}
			obj.Body = nil // the projection reads metadata only
			objs = append(objs, obj)
		}
		_ = rows.Close()
		if len(objs) == 0 {
			break
		}
		cursor = objs[len(objs)-1].CanonicalID
		have, err := s.projectedCaptureIDs(ctx, objs)
		if err != nil {
			s.retrievalState.captureIndexDirty.Store(true)
			return written, err
		}
		// Missing rows of projectable captures are written fresh; rows of
		// captures that can no longer be projected are removed.
		var missing, stale []objectgraph.Object
		for _, obj := range objs {
			_, projectable := projectCapture(obj)
			switch {
			case projectable && !have[obj.CanonicalID]:
				missing = append(missing, obj)
			case !projectable && have[obj.CanonicalID]:
				stale = append(stale, obj)
			}
		}
		if err := s.writeCaptureProjections(ctx, true, missing); err != nil {
			s.retrievalState.captureIndexDirty.Store(true)
			return written, err
		}
		if err := s.writeCaptureProjections(ctx, false, stale); err != nil {
			s.retrievalState.captureIndexDirty.Store(true)
			return written, err
		}
		written += len(missing) + len(stale)
		if len(objs) < captureBackfillPage {
			break
		}
		if len(missing)+len(stale) > 0 && captureBackfillPause > 0 {
			// Yield to the capture route between written pages: on a
			// sql-server every commit serializes with the route's.
			select {
			case <-ctx.Done():
				s.retrievalState.captureIndexDirty.Store(true)
				return written, ctx.Err()
			case <-time.After(captureBackfillPause):
			}
		}
	}
	// Projection rows whose capture is gone are not repaired by the
	// forward pass; captures are append-only, and DeleteObject removes
	// them inline.
	var captures, projected int
	if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE canonical_id >= ? AND canonical_id < ? AND tombstone = FALSE`, captureIDPrefix, captureIDEnd).Scan(&captures); err != nil {
		s.retrievalState.captureIndexDirty.Store(true)
		return written, err
	}
	if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index`).Scan(&projected); err != nil {
		s.retrievalState.captureIndexDirty.Store(true)
		return written, err
	}
	s.retrievalState.captureIndexGap.Store(int64(captures - projected))
	s.retrievalState.captureIndexComplete.Store(true)
	return written, nil
}

// projectedCaptureIDs reports which of objs already have a projection row.
func (s *Store) projectedCaptureIDs(ctx context.Context, objs []objectgraph.Object) (map[string]bool, error) {
	have := map[string]bool{}
	if len(objs) == 0 {
		return have, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(objs)), ", ")
	args := make([]any, 0, len(objs))
	for _, obj := range objs {
		args = append(args, obj.CanonicalID)
	}
	rows, err := s.corpus().QueryContext(ctx, `SELECT capture_id FROM proj_capture_index WHERE capture_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		have[id] = true
	}
	return have, rows.Err()
}

// ErrCaptureIndexIncomplete is the search and URL routes' answer while the
// projection is not known to hold every capture: the caller uses the
// window path.
var ErrCaptureIndexIncomplete = fmt.Errorf("capture index is not complete")

// CaptureSearchRequest is one corpus search over a computer's captures.
type CaptureSearchRequest struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	// Words are the query's search words, as the guest's corpus search
	// prepared them (lowercased, punctuation trimmed, stop words dropped).
	Words []string `json:"words"`
	// Need is how many words a capture's own fields must match to be a
	// candidate.
	Need int `json:"need"`
	// Candidates bounds the captures returned (strongest, then newest).
	Candidates int `json:"candidates"`
	// IDs are captures the caller matched by its own hints (labels); they
	// are returned whatever their words, within the owner and computer.
	IDs []string `json:"ids,omitempty"`
	// Query, when set, also searches the pages' text (capture_fulltext.go):
	// BM25 over passages, every query word counting, no word gate. Since and
	// Until bound a page's time (published when known, else fetched; Since
	// inclusive, Until exclusive; RFC 3339 or a date). Order is "relevance"
	// (default) or "recent". Limit is how many pages the text search
	// returns. Words, Need and Candidates keep driving the title and label
	// half as before.
	Query string `json:"query,omitempty"`
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
	Order string `json:"order,omitempty"`
	Limit int    `json:"limit,omitempty"`
	// Size is how many pages the ranked answer holds (capture_rank.go); the
	// default is 10 and the most 25. Only a request with a Query is ranked.
	Size int `json:"size,omitempty"`
}

// CaptureSearchResponse holds the candidate captures, newest first among
// equals, as full objects (no body), for the caller's own ranking.
type CaptureSearchResponse struct {
	Objects []objectgraph.Object `json:"objects"`
	// Matched is how many captures met Need before the Candidates bound.
	Matched int `json:"matched"`
	// Capped is true when Matched exceeded Candidates.
	Capped bool `json:"capped"`
	// Searched is how many captures of this computer the projection
	// holds: the search covered all of them.
	Searched int `json:"searched"`
	// Hits are the pages found by their text, best first, each with the
	// passage that found it; HitObjects are those captures (no body). Both
	// are empty unless the request carried a Query and the index answered.
	Hits       []CaptureFullTextHit `json:"hits,omitempty"`
	HitObjects []objectgraph.Object `json:"hit_objects,omitempty"`
	// FullText says how the text half went (nil when no Query was sent).
	FullText *CaptureFullTextInfo `json:"fulltext,omitempty"`
	// TermsIncomplete is true when the text half answered but the title and
	// label projection is still being built: Objects is empty and the caller
	// uses its own window for that half.
	TermsIncomplete bool `json:"terms_incomplete,omitempty"`
	// Ranked is the one answer for a request with a Query: vector order with
	// BM25's top three pages guaranteed a place, or BM25's pages alone when the
	// vectors could not answer (capture_rank.go). Every capture it names has
	// its object in HitObjects.
	Ranked *CaptureRanking `json:"ranked,omitempty"`
}

const (
	captureSearchMaxWords      = 8
	captureSearchMaxCandidates = 1000
	captureSearchMaxIDs        = 200
	captureSearchIDChunk       = 200
)

// SearchCaptures returns the captures of one computer whose title, host,
// source or language hold at least Need of the words, from the term table
// over all history, plus the captures named by IDs.
func (s *Store) SearchCaptures(ctx context.Context, req CaptureSearchRequest) (resp CaptureSearchResponse, err error) {
	resp, err = s.searchCaptureParts(ctx, req)
	if err == nil && strings.TrimSpace(req.Query) != "" {
		// After the terms timer has ended, so neither journal line contains
		// the other.
		s.rankCaptureAnswer(ctx, req, &resp)
	}
	return resp, err
}

func (s *Store) searchCaptureParts(ctx context.Context, req CaptureSearchRequest) (resp CaptureSearchResponse, err error) {
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if strings.TrimSpace(req.Query) != "" && owner != "" && computer != "" {
		// The text half has its own timer (capture_fulltext_search). The
		// terms timer starts after it, so the journal's two lines add up to
		// the call and neither contains the other.
		if err := s.searchCaptureFullText(ctx, owner, computer, req, &resp); err != nil {
			return resp, err
		}
	}
	done := s.retrievalTimerExtra(retrievalCaptureSearch, "")
	defer func() { done("terms", len(resp.Objects), err, fmt.Sprintf(" matched=%d", resp.Matched)) }()
	if !s.retrievalState.captureIndexComplete.Load() {
		if resp.FullText != nil && resp.FullText.Used {
			// The text half answered; the title and label half is still
			// being built, and the caller covers it with its own window.
			resp.TermsIncomplete = true
			return resp, nil
		}
		return resp, ErrCaptureIndexIncomplete
	}
	if owner == "" || computer == "" {
		return resp, fmt.Errorf("owner and computer are required")
	}
	words := req.Words
	if len(words) > captureSearchMaxWords {
		words = words[:captureSearchMaxWords]
	}
	limit := req.Candidates
	if limit <= 0 || limit > captureSearchMaxCandidates {
		limit = captureSearchMaxCandidates
	}
	need := req.Need
	if need < 1 {
		need = 1
	}
	if err := s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM proj_capture_index WHERE owner_id = ? AND computer_id = ?`, owner, computer).Scan(&resp.Searched); err != nil {
		return resp, err
	}
	since, err := parseSearchTime("since", req.Since)
	if err != nil {
		return resp, err
	}
	until, err := parseSearchTime("until", req.Until)
	if err != nil {
		return resp, err
	}
	ids, matched, err := s.captureTermCandidates(ctx, owner, computer, words, need, limit, since, until)
	if err != nil {
		return resp, err
	}
	resp.Matched = matched
	resp.Capped = matched > len(ids)
	seen := map[string]bool{}
	var order []string
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			order = append(order, id)
		}
	}
	extra := req.IDs
	if len(extra) > captureSearchMaxIDs {
		extra = extra[:captureSearchMaxIDs]
	}
	for _, id := range extra {
		id = strings.TrimSpace(id)
		if strings.HasPrefix(id, captureIDPrefix) && !seen[id] {
			seen[id] = true
			order = append(order, id)
		}
	}
	objs, err := s.captureObjectsByID(ctx, owner, computer, order)
	if err != nil {
		return resp, err
	}
	resp.Objects = objs
	return resp, nil
}

// captureTermQuery is the statement that finds the captures holding at least
// need of words, with its arguments; the empty string when no word has a
// searchable part.
func captureTermQuery(owner, computer string, words []string, need int) (string, []any) {
	var parts []string
	var args []any
	for i, word := range words {
		qp := captureterms.QueryParts(word)
		if len(qp) == 0 {
			continue
		}
		var conds []string
		var condArgs []any
		for _, p := range qp {
			if p.Prefix {
				conds = append(conds, "(term >= ? AND term < ?)")
				condArgs = append(condArgs, p.Term, captureterms.PrefixUpper(p.Term))
			} else {
				conds = append(conds, "term = ?")
				condArgs = append(condArgs, p.Term)
			}
		}
		if len(qp) == 1 {
			parts = append(parts, fmt.Sprintf(`SELECT capture_id, %d AS qt, fetched_at FROM proj_capture_terms WHERE owner_id = ? AND computer_id = ? AND %s`, i, conds[0]))
			args = append(args, owner, computer)
			args = append(args, condArgs...)
			continue
		}
		// Every part of the word: a capture holding all of them is a
		// candidate (a prefix part may count once per matching term, so
		// this is a superset the caller's matcher narrows).
		parts = append(parts, fmt.Sprintf(`SELECT capture_id, %d AS qt, MAX(fetched_at) AS fetched_at FROM proj_capture_terms WHERE owner_id = ? AND computer_id = ? AND (%s) GROUP BY capture_id HAVING COUNT(DISTINCT term) >= %d`,
			i, strings.Join(conds, " OR "), len(qp)))
		args = append(args, owner, computer)
		args = append(args, condArgs...)
	}
	if len(parts) == 0 {
		return "", nil
	}
	query := `WITH hits AS (` + strings.Join(parts, " UNION ALL ") + `)
		SELECT capture_id, COUNT(DISTINCT qt) AS words, MAX(fetched_at) AS fetched
		FROM hits GROUP BY capture_id HAVING COUNT(DISTINCT qt) >= ?`
	args = append(args, need)
	return query, args
}

// captureTermRankedQuery is captureTermQuery ranked and cut in SQL: the
// captures holding at least need of the words, strongest first (words
// matched, then newest fetched, then id, each descending, as the Go sort it
// replaces gave), at most limit of them, each row carrying how many captures
// matched in all. A time window (published time, else fetched; since
// inclusive, until exclusive) is applied before the cut, so a window that holds
// few of the strongest matches still returns up to limit of its own.
func captureTermRankedQuery(owner, computer string, words []string, need, limit int, since, until time.Time) (string, []any) {
	grouped, args := captureTermQuery(owner, computer, words, need)
	if grouped == "" {
		return "", nil
	}
	q := `WITH g AS (` + grouped + `) SELECT g.capture_id, g.words, g.fetched, COUNT(*) OVER () AS matched FROM g`
	if !since.IsZero() || !until.IsZero() {
		q += ` JOIN proj_capture_index p ON p.capture_id = g.capture_id WHERE COALESCE(p.published_at, p.fetched_at) IS NOT NULL`
		if !since.IsZero() {
			q += ` AND COALESCE(p.published_at, p.fetched_at) >= ?`
			args = append(args, since)
		}
		if !until.IsZero() {
			q += ` AND COALESCE(p.published_at, p.fetched_at) < ?`
			args = append(args, until)
		}
	}
	q += ` ORDER BY g.words DESC, g.fetched DESC NULLS LAST, g.capture_id DESC LIMIT ?`
	args = append(args, limit)
	return q, args
}

// captureTermCandidates returns up to limit capture ids holding at least
// need of words (strongest, then newest, inside the window), and how many
// matched. The ranking and the cut are the statement's: the rows read here
// are the rows returned.
func (s *Store) captureTermCandidates(ctx context.Context, owner, computer string, words []string, need, limit int, since, until time.Time) ([]string, int, error) {
	query, args := captureTermRankedQuery(owner, computer, words, need, limit, since, until)
	if query == "" {
		return nil, 0, nil
	}
	rows, err := s.corpus().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("capture term search: %w", err)
	}
	defer rows.Close()
	var ids []string
	matched := 0
	for rows.Next() {
		var id string
		var nwords int
		var fetched sql.NullTime
		if err := rows.Scan(&id, &nwords, &fetched, &matched); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	s.retrievalState.reads.termRows.Add(int64(len(ids)))
	return ids, matched, nil
}

// captureObjectsByID reads captures by id within one owner and computer,
// in the order given, without bodies.
func (s *Store) captureObjectsByID(ctx context.Context, owner, computer string, ids []string) ([]objectgraph.Object, error) {
	byID := map[string]objectgraph.Object{}
	for start := 0; start < len(ids); start += captureSearchIDChunk {
		end := start + captureSearchIDChunk
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(chunk)), ", ")
		args := make([]any, 0, len(chunk)+3)
		for _, id := range chunk {
			args = append(args, id)
		}
		args = append(args, string(objectgraph.WebCaptureObjectKind), owner, computer)
		s.retrievalState.reads.objectStatements.Add(1)
		rows, err := s.corpus().QueryContext(ctx, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
			FROM og_objects WHERE canonical_id IN (`+placeholders+`) AND object_kind = ? AND owner_id = ? AND computer_id = ?`, args...)
		if err != nil {
			return nil, fmt.Errorf("read capture candidates: %w", err)
		}
		for rows.Next() {
			obj, _, err := scanObjectGraphObject(rows)
			if err != nil {
				_ = rows.Close()
				return nil, err
			}
			obj.Body = nil // a capture row carries no body; never hydrate one here
			s.retrievalState.reads.objectRows.Add(1)
			byID[obj.CanonicalID] = obj
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	out := make([]objectgraph.Object, 0, len(byID))
	for _, id := range ids {
		if obj, ok := byID[id]; ok {
			out = append(out, obj)
			delete(byID, id)
		}
	}
	return out, nil
}

// CaptureByURLResult is the newest capture of a URL.
type CaptureByURLResult struct {
	Found     bool   `json:"found"`
	CaptureID string `json:"capture_id,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
}

// CaptureByURL finds the newest capture of a URL (by its URL or its
// canonical URL, compared as Texture's citation lookup compares them) among
// one computer's captures over all history. blocksOnly keeps block
// captures only (a citation needs blocks to quote).
func (s *Store) CaptureByURL(ctx context.Context, owner, computer, rawURL string, blocksOnly bool) (res CaptureByURLResult, err error) {
	done := s.retrievalTimer(retrievalCaptureByURL)
	defer func() {
		n := 0
		if res.Found {
			n = 1
		}
		done("url_key", n, err)
	}()
	if !s.retrievalState.captureIndexComplete.Load() {
		return res, ErrCaptureIndexIncomplete
	}
	key := captureURLKey(rawURL)
	blocks := ""
	if blocksOnly {
		blocks = " AND has_blocks = TRUE"
	}
	type hit struct {
		id      string
		fetched sql.NullTime
		updated time.Time
	}
	var hits []hit
	for _, column := range []string{"url_key", "canonical_key"} {
		rows, err := s.corpus().QueryContext(ctx, `SELECT capture_id, fetched_at, updated_at FROM proj_capture_index
			WHERE owner_id = ? AND computer_id = ? AND `+column+` = ?`+blocks+` ORDER BY fetched_at DESC NULLS LAST LIMIT 20`,
			strings.TrimSpace(owner), strings.TrimSpace(computer), key)
		if err != nil {
			return res, fmt.Errorf("capture by url: %w", err)
		}
		for rows.Next() {
			var h hit
			if err := rows.Scan(&h.id, &h.fetched, &h.updated); err != nil {
				_ = rows.Close()
				return res, err
			}
			hits = append(hits, h)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return res, err
		}
	}
	if len(hits) == 0 {
		return res, nil
	}
	// Newest by fetch time (an unparsed time sorts as the zero time, as in
	// the window path), then by the store's newest-first order.
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i].fetched.Time, hits[j].fetched.Time
		if !a.Equal(b) {
			return a.After(b)
		}
		return hits[i].updated.After(hits[j].updated)
	})
	res.Found = true
	res.CaptureID = hits[0].id
	if hits[0].fetched.Valid {
		res.FetchedAt = hits[0].fetched.Time.Format(time.RFC3339Nano)
	}
	return res, nil
}

// captureTermsCoverDDL is the covering index of the term search. The primary
// key finds a word's postings but does not hold fetched_at, so every posting
// cost a heap fetch (random reads for a common word); with fetched_at
// INCLUDEd the search is an index-only scan. Measured at 214,000 captures
// and 2.76 million postings: 228 to 46 ms, 274 to 74 ms, 731 to 178 ms for
// queries of 124k, 185k and 428k postings. The statement and its ranking are
// unchanged; the primary key stays (it enforces uniqueness and serves the
// writers' ON CONFLICT).
const captureTermsCoverDDL = `CREATE INDEX CONCURRENTLY ` + captureTermsCoverIdx + ` ON ` + captureTermsTable + ` (owner_id, computer_id, term, capture_id) INCLUDE (fetched_at)`

// captureTermsCoverLockKey serializes the build between processes: an index
// left invalid is rebuilt only by the process holding the lock, so one is never
// dropped while another is still building it.
const captureTermsCoverLockKey = 7330212470803

// ensureCaptureTermsCoverIndex builds the covering index of the term table if
// it is missing, and rebuilds it if an earlier build was interrupted and left
// it invalid. It builds CONCURRENTLY, outside a transaction, so capture
// writes continue during the build (CONCURRENTLY holds only a lock that
// excludes other schema changes and vacuum) and the search keeps using the
// primary key, slower, until the index is valid: a planner never uses an
// invalid index. It waits for transactions older than the build to end, so a
// stuck long transaction delays it but blocks nothing. A failed or cancelled
// build leaves an invalid index; the next call drops and rebuilds it. Calls
// come from MigrateRetrieval at start and from every tick of retrievalPeriodic
// (every 10 minutes by default), so an index left invalid by a process that
// died while another skipped for its lock is repaired without a restart; once
// the index is valid a call is a pool checkout, a try-lock, one catalog query
// and an unlock.
func (s *Store) ensureCaptureTermsCoverIndex(ctx context.Context) error {
	if !s.retrievalState.captureIndexReady.Load() {
		return nil
	}
	// One connection, so the session advisory lock and the statements share a
	// session.
	conn, err := s.corpus().Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var got bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(?)`, captureTermsCoverLockKey).Scan(&got); err != nil {
		// The server may have taken the lock before the answer was lost: do not
		// return this connection to the pool holding a session lock. Closing
		// after a bad-connection error discards it (and so ends the session).
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return err
	}
	if !got {
		return nil // another process is building it
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(?)`, captureTermsCoverLockKey)
	}()
	var valid sql.NullBool
	err = conn.QueryRowContext(ctx, `SELECT i.indisvalid FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = ? AND i.indrelid = ?::regclass`, captureTermsCoverIdx, captureTermsTable).Scan(&valid)
	switch {
	case err == nil && valid.Bool:
		return nil
	case err == nil:
		log.Printf("corpusd retrieval migrate index=%s was left invalid by an interrupted build; rebuilding", captureTermsCoverIdx)
		if _, err := conn.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS `+captureTermsCoverIdx); err != nil {
			return fmt.Errorf("drop invalid %s: %w", captureTermsCoverIdx, err)
		}
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	start := time.Now()
	if _, err := conn.ExecContext(ctx, captureTermsCoverDDL); err != nil {
		return fmt.Errorf("build %s: %w", captureTermsCoverIdx, err)
	}
	log.Printf("corpusd retrieval migrate index=%s table=%s ms=%d concurrently", captureTermsCoverIdx, captureTermsTable, time.Since(start).Milliseconds())
	// An index-only scan reads the heap for every page the visibility map does
	// not mark all-visible, which is as slow as no index at all (measured: no
	// gain until the table was vacuumed). Autovacuum keeps the map, but a table
	// that was bulk-loaded may not have been vacuumed; one vacuum after the
	// build makes the index useful at once. It takes a lock that excludes only
	// schema changes and other vacuums, not writers, and failing is harmless.
	vstart := time.Now()
	if _, err := conn.ExecContext(ctx, `VACUUM (ANALYZE) `+captureTermsTable); err != nil {
		log.Printf("corpusd retrieval migrate vacuum table=%s err=%v (autovacuum will do it)", captureTermsTable, err)
		return nil
	}
	log.Printf("corpusd retrieval migrate vacuum table=%s ms=%d", captureTermsTable, time.Since(vstart).Milliseconds())
	return nil
}
