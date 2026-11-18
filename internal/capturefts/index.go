// Package capturefts is the full-text index of captured pages: BM25 over the
// passages of every page's clean text, one SQLite FTS5 file per scope (an
// owner's computer), owned by corpusd. See docs/reference/CAPTURE-FULLTEXT.md
// for what it is, why SQLite FTS5 and passages, and what the vector index
// takes from it.
//
// It is derived state: nothing here is the only copy of anything. The
// authority is the capture object and its text blob; a lost or stale file
// is rebuilt by the catch-up in internal/platform. The index stores no page
// text: a passage is a byte range of the text blob a page names by hash, so
// the same range is what an embedding model reads and what a snippet shows.
package capturefts

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite" // the cgo-free driver already in go.mod; FTS5 is compiled in
)

const (
	// TitleWeight is how much a title word counts against a body word in the
	// BM25 score (the published-pieces index counts a title word three times).
	TitleWeight = 3.0
	// candidatePassages is how many best passages one search ranks pages from.
	candidatePassages = 300
	// DefaultLimit and MaxLimit bound the pages a search returns.
	DefaultLimit = 10
	MaxLimit     = 100
	// maxOpenScopes bounds the open files; each holds at most openConns
	// connections of cacheKiB page cache, so the index's memory is at most
	// maxOpenScopes * openConns * cacheKiB KiB (96 MiB), whatever the corpus.
	maxOpenScopes = 4
	openConns     = 3
	cacheKiB      = 8192
)

// Scope names whose pages an index holds: one owner's computer.
type Scope struct{ Owner, Computer string }

func (s Scope) valid() bool {
	return strings.TrimSpace(s.Owner) != "" && strings.TrimSpace(s.Computer) != ""
}

func (s Scope) key() string {
	sum := sha256.Sum256([]byte(s.Owner + "\x00" + s.Computer))
	return hex.EncodeToString(sum[:16])
}

// Page is one captured page to index.
type Page struct {
	CaptureID string
	Title     string
	// TextHash names the text blob the passages are ranges of; a page
	// whose hash, title and times are unchanged is not rewritten.
	TextHash    string
	Text        string
	FetchedAt   time.Time
	PublishedAt time.Time // zero when unknown
}

// PutResult says what Put did.
type PutResult struct {
	Passages  int
	Truncated bool // the text was longer than MaxIndexedBytes
	Unchanged bool // the page was already indexed as given
}

// Order is how a search orders the pages it returns.
type Order string

const (
	// ByRelevance orders by BM25 score, then newest, then capture id.
	ByRelevance Order = "relevance"
	// ByRecency orders by time, newest first, then score, then capture id.
	ByRecency Order = "recent"
)

// Query is one full-text search of a scope.
type Query struct {
	Text string
	// Since and Until bound a page's time, Since inclusive and Until
	// exclusive; a zero value is no bound. A page's time is its published
	// time when known, else the time it was fetched.
	Since, Until time.Time
	Order        Order
	Limit        int
}

// Hit is one page found and the best passage that found it.
type Hit struct {
	CaptureID string
	Title     string
	// Score is the BM25 score of the best passage (higher is better; title
	// words count TitleWeight times).
	Score float64
	// PassageID is the passage's key: unique within the scope, never reused.
	PassageID int64
	Ordinal   int
	// Passages is how many passages the page holds.
	Passages int
	// Start and End are the best passage's byte range in the text blob
	// named by TextHash.
	Start, End int
	TextHash   string
	// Time is the page's time (published when known, else fetched).
	Time        time.Time
	FetchedAt   time.Time
	PublishedAt time.Time
	// TitleMatch is true when a query term occurs in the title.
	TitleMatch bool
}

// Result is a search's answer.
type Result struct {
	Hits []Hit
	// Terms are the index terms the query looked up; Pruned are terms of the
	// query left out of the search because they occur in so many passages
	// that they carry almost no weight and cost a scan (see pruneTerms).
	Terms  []string
	Pruned []string
	// Pages is how many pages the ranked passages came from, before Limit.
	Pages int
	// Capped is true when more passages matched than were ranked.
	Capped bool
	// Indexed is how many pages the scope's index holds.
	Indexed int64
}

// Stats counts what a scope's index holds.
type Stats struct{ Pages, Passages int64 }

// Index is a directory of per-scope FTS5 files.
type Index struct {
	dir    string
	mu     sync.Mutex
	scopes map[string]*scopeDB
	tick   atomic.Int64
	closed bool
	// scanBudget is the postings budget of a search; 0 means the default.
	scanBudget int
}

type scopeDB struct {
	ix      *Index
	key     string
	scope   Scope
	db      *sql.DB
	writeMu sync.Mutex
	pages   atomic.Int64
	// passages counts the scope's passages (the document count BM25 and the
	// pruning of common terms measure against).
	passages atomic.Int64
	refs     int
	used     int64
	// df caches how many passages hold a term; it is cleared when the
	// scope's size has changed by a tenth, since only its magnitude is used.
	dfMu     sync.Mutex
	df       map[string]int
	dfAtSize int64
}

// Open opens (creating dir if needed) the index at dir. Files are created
// per scope on first write.
func Open(dir string) (*Index, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("capturefts: directory is required")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("capturefts: %w", err)
	}
	return &Index{dir: dir, scopes: map[string]*scopeDB{}}, nil
}

// Close closes every open file.
func (ix *Index) Close() error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.closed = true
	var errs []error
	for k, s := range ix.scopes {
		errs = append(errs, s.db.Close())
		delete(ix.scopes, k)
	}
	return errors.Join(errs...)
}

const schema = `
CREATE TABLE IF NOT EXISTS meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS pages (
	capture_id   TEXT PRIMARY KEY,
	title        TEXT NOT NULL,
	text_hash    TEXT NOT NULL,
	fetched_at   INTEGER NOT NULL,
	published_at INTEGER,
	eff          INTEGER NOT NULL,
	passages     INTEGER NOT NULL,
	text_bytes   INTEGER NOT NULL,
	truncated    INTEGER NOT NULL,
	indexed_at   INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS passages (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	capture_id TEXT NOT NULL,
	ordinal    INTEGER NOT NULL,
	start_byte INTEGER NOT NULL,
	end_byte   INTEGER NOT NULL,
	eff        INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS passages_page ON passages(capture_id, ordinal);
CREATE INDEX IF NOT EXISTS passages_eff ON passages(eff);
CREATE VIRTUAL TABLE IF NOT EXISTS passages_fts USING fts5(title, body, content='', contentless_delete=1, tokenize='unicode61 remove_diacritics 2', detail=full);
`

func (ix *Index) path(s Scope) string { return filepath.Join(ix.dir, s.key()+".db") }

// acquire returns the scope's open file. With create false and no file on
// disk it returns nil and no error.
func (ix *Index) acquire(scope Scope, create bool) (*scopeDB, func(), error) {
	if !scope.valid() {
		return nil, nil, errors.New("capturefts: owner and computer are required")
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.closed {
		return nil, nil, errors.New("capturefts: index is closed")
	}
	key := scope.key()
	s := ix.scopes[key]
	if s == nil {
		path := ix.path(scope)
		if !create {
			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					return nil, func() {}, nil
				}
				return nil, nil, err
			}
		}
		db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-"+fmt.Sprint(cacheKiB)+")&_pragma=journal_size_limit(67108864)")
		if err != nil {
			return nil, nil, fmt.Errorf("capturefts: open: %w", err)
		}
		db.SetMaxOpenConns(openConns)
		db.SetMaxIdleConns(openConns)
		if _, err := db.Exec(schema); err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("capturefts: schema: %w", err)
		}
		var stored string
		err = db.QueryRow(`SELECT v FROM meta WHERE k = 'scope'`).Scan(&stored)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			_, err = db.Exec(`INSERT INTO meta(k, v) VALUES ('scope', ?)`, scope.Owner+"\x00"+scope.Computer)
		case err == nil && stored != scope.Owner+"\x00"+scope.Computer:
			err = errors.New("file belongs to another scope")
		}
		if err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("capturefts: %w", err)
		}
		s = &scopeDB{ix: ix, key: key, scope: scope, db: db}
		var n int64
		if err := db.QueryRow(`SELECT count(*) FROM pages`).Scan(&n); err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("capturefts: %w", err)
		}
		s.pages.Store(n)
		var np int64
		if err := db.QueryRow(`SELECT coalesce(sum(passages), 0) FROM pages`).Scan(&np); err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("capturefts: %w", err)
		}
		s.passages.Store(np)
		// Held and fresh before eviction looks for an idle, old file.
		s.refs, s.used = 1, ix.tick.Add(1)
		ix.scopes[key] = s
		ix.evictLocked()
		return s, ix.releaser(s), nil
	}
	s.refs++
	s.used = ix.tick.Add(1)
	return s, ix.releaser(s), nil
}

func (ix *Index) releaser(s *scopeDB) func() {
	return func() {
		ix.mu.Lock()
		s.refs--
		ix.mu.Unlock()
	}
}

// evictLocked closes idle files beyond maxOpenScopes, least recently used first.
func (ix *Index) evictLocked() {
	for len(ix.scopes) > maxOpenScopes {
		var victim *scopeDB
		for _, s := range ix.scopes {
			if s.refs == 0 && (victim == nil || s.used < victim.used) {
				victim = s
			}
		}
		if victim == nil {
			return
		}
		_ = victim.db.Close()
		delete(ix.scopes, victim.key)
	}
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// Put indexes a page, replacing its passages if it was indexed before. An
// identical page (same text hash, title and times) is left alone.
func (ix *Index) Put(ctx context.Context, scope Scope, p Page) (PutResult, error) {
	if strings.TrimSpace(p.CaptureID) == "" {
		return PutResult{}, errors.New("capturefts: capture id is required")
	}
	s, release, err := ix.acquire(scope, true)
	if err != nil {
		return PutResult{}, err
	}
	defer release()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	fetched := unix(p.FetchedAt)
	var published sql.NullInt64
	if !p.PublishedAt.IsZero() {
		published = sql.NullInt64{Int64: p.PublishedAt.Unix(), Valid: true}
	}
	eff := fetched
	if published.Valid {
		eff = published.Int64
	}
	var haveHash, haveTitle string
	var haveFetched int64
	var havePublished sql.NullInt64
	var havePassages int
	var haveTruncated bool
	err = s.db.QueryRowContext(ctx, `SELECT text_hash, title, fetched_at, published_at, passages, truncated FROM pages WHERE capture_id = ?`, p.CaptureID).
		Scan(&haveHash, &haveTitle, &haveFetched, &havePublished, &havePassages, &haveTruncated)
	existing := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PutResult{}, err
	}
	if existing && haveHash == p.TextHash && haveTitle == p.Title && haveFetched == fetched && havePublished == published {
		return PutResult{Passages: havePassages, Truncated: haveTruncated, Unchanged: true}, nil
	}

	text := p.Text
	truncated := false
	if len(text) > MaxIndexedBytes {
		cut := MaxIndexedBytes
		for cut > 0 && !isRuneStart(text[cut]) {
			cut--
		}
		text, truncated = text[:cut], true
	}
	spans := Passages(text)
	if len(spans) == 0 {
		spans = []Span{{0, 0}} // a page with no text is still found by its title
	}
	title := strings.Join(Terms(p.Title), " ")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PutResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing {
		if err := removePage(ctx, tx, p.CaptureID); err != nil {
			return PutResult{}, err
		}
	}
	insP, err := tx.PrepareContext(ctx, `INSERT INTO passages(capture_id, ordinal, start_byte, end_byte, eff) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return PutResult{}, err
	}
	defer insP.Close()
	insF, err := tx.PrepareContext(ctx, `INSERT INTO passages_fts(rowid, title, body) VALUES (?, ?, ?)`)
	if err != nil {
		return PutResult{}, err
	}
	defer insF.Close()
	for i, sp := range spans {
		res, err := insP.ExecContext(ctx, p.CaptureID, i, sp.Start, sp.End, eff)
		if err != nil {
			return PutResult{}, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return PutResult{}, err
		}
		if _, err := insF.ExecContext(ctx, id, title, strings.Join(Terms(text[sp.Start:sp.End]), " ")); err != nil {
			return PutResult{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO pages(capture_id, title, text_hash, fetched_at, published_at, eff, passages, text_bytes, truncated, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, p.CaptureID, p.Title, p.TextHash, fetched, published, eff, len(spans), len(p.Text), truncated, time.Now().Unix()); err != nil {
		return PutResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return PutResult{}, err
	}
	if !existing {
		s.pages.Add(1)
	}
	s.passages.Add(int64(len(spans) - havePassages))
	return PutResult{Passages: len(spans), Truncated: truncated}, nil
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// removePage deletes a page's passages and row inside tx.
func removePage(ctx context.Context, tx execer, id string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM passages_fts WHERE rowid IN (SELECT id FROM passages WHERE capture_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM passages WHERE capture_id = ?`, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM pages WHERE capture_id = ?`, id)
	return err
}

// Remove deletes a page's passages. A page that is not indexed is not an
// error.
func (ix *Index) Remove(ctx context.Context, scope Scope, captureID string) error {
	s, release, err := ix.acquire(scope, false)
	if err != nil || s == nil {
		return err
	}
	defer release()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var n, np int
	_ = tx.QueryRowContext(ctx, `SELECT count(*), coalesce(sum(passages), 0) FROM pages WHERE capture_id = ?`, captureID).Scan(&n, &np)
	if err := removePage(ctx, tx, captureID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.pages.Add(-int64(n))
	s.passages.Add(-int64(np))
	return nil
}

// Ref names a page and the text it was indexed from.
type Ref struct{ CaptureID, TextHash string }

// Missing returns the refs not indexed with that text hash. The catch-up uses
// it to skip what is already there.
func (ix *Index) Missing(ctx context.Context, scope Scope, refs []Ref) ([]Ref, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	s, release, err := ix.acquire(scope, false)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return refs, nil
	}
	defer release()
	have := map[string]string{}
	for start := 0; start < len(refs); start += 400 {
		chunk := refs[start:min(start+400, len(refs))]
		args := make([]any, len(chunk))
		for i, r := range chunk {
			args[i] = r.CaptureID
		}
		rows, err := s.db.QueryContext(ctx, `SELECT capture_id, text_hash FROM pages WHERE capture_id IN (?`+strings.Repeat(",?", len(chunk)-1)+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, h string
			if err := rows.Scan(&id, &h); err != nil {
				_ = rows.Close()
				return nil, err
			}
			have[id] = h
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	var out []Ref
	for _, r := range refs {
		if h, ok := have[r.CaptureID]; !ok || h != r.TextHash {
			out = append(out, r)
		}
	}
	return out, nil
}

// Indexed reports whether the page is indexed from that text.
func (ix *Index) Indexed(ctx context.Context, scope Scope, captureID, textHash string) (bool, error) {
	miss, err := ix.Missing(ctx, scope, []Ref{{captureID, textHash}})
	return len(miss) == 0, err
}

// Stats counts a scope's pages and passages.
func (ix *Index) Stats(ctx context.Context, scope Scope) (Stats, error) {
	s, release, err := ix.acquire(scope, false)
	if err != nil || s == nil {
		return Stats{}, err
	}
	defer release()
	var st Stats
	err = s.db.QueryRowContext(ctx, `SELECT count(*), coalesce(sum(passages), 0) FROM pages`).Scan(&st.Pages, &st.Passages)
	return st, err
}

// Pages is the number of pages a scope's index holds, without a scan.
func (ix *Index) Pages(scope Scope) int64 {
	s, release, err := ix.acquire(scope, false)
	if err != nil || s == nil {
		return 0
	}
	defer release()
	return s.pages.Load()
}

// Search ranks the scope's pages for the query: BM25 over passages, the
// best passage of a page standing for the page.
func (ix *Index) Search(ctx context.Context, scope Scope, q Query) (Result, error) {
	if !scope.valid() {
		return Result{}, errors.New("capturefts: owner and computer are required")
	}
	res := Result{Terms: QueryTerms(q.Text)}
	if len(res.Terms) == 0 {
		return res, nil
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	s, release, err := ix.acquire(scope, false)
	if err != nil {
		return res, err
	}
	if s == nil {
		return res, nil
	}
	defer release()
	res.Indexed = s.pages.Load()
	searchTerms, pruned, err := s.pruneTerms(ctx, res.Terms)
	if err != nil {
		return res, fmt.Errorf("capturefts: search: %w", err)
	}
	res.Pruned = pruned
	if len(searchTerms) == 0 {
		return res, nil
	}
	quoted := make([]string, len(searchTerms))
	for i, t := range searchTerms {
		quoted[i] = `"` + t + `"`
	}
	sqlText := `SELECT p.id, p.capture_id, p.ordinal, p.start_byte, p.end_byte, p.eff, -bm25(passages_fts, ?, 1.0) AS score,
			pg.title, pg.text_hash, pg.passages, pg.fetched_at, pg.published_at
		FROM passages_fts JOIN passages p ON p.id = passages_fts.rowid JOIN pages pg ON pg.capture_id = p.capture_id
		WHERE passages_fts MATCH ?`
	args := []any{TitleWeight, strings.Join(quoted, " OR ")}
	if !q.Since.IsZero() {
		sqlText += ` AND p.eff >= ?`
		args = append(args, q.Since.Unix())
	}
	if !q.Until.IsZero() {
		sqlText += ` AND p.eff < ?`
		args = append(args, q.Until.Unix())
	}
	if q.Order == ByRecency {
		sqlText += ` ORDER BY p.eff DESC, score DESC, p.id`
	} else {
		sqlText += ` ORDER BY score DESC, p.eff DESC, p.id`
	}
	sqlText += ` LIMIT ?`
	args = append(args, candidatePassages)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return res, fmt.Errorf("capturefts: search: %w", err)
	}
	defer rows.Close()
	titleTerms := map[string]bool{}
	best := map[string]*Hit{}
	var order []string
	n := 0
	for rows.Next() {
		n++
		var h Hit
		var eff, fetched int64
		var published sql.NullInt64
		if err := rows.Scan(&h.PassageID, &h.CaptureID, &h.Ordinal, &h.Start, &h.End, &eff, &h.Score, &h.Title, &h.TextHash, &h.Passages, &fetched, &published); err != nil {
			return res, err
		}
		h.Time = time.Unix(eff, 0).UTC()
		h.FetchedAt = time.Unix(fetched, 0).UTC()
		if published.Valid {
			h.PublishedAt = time.Unix(published.Int64, 0).UTC()
		}
		if prev, ok := best[h.CaptureID]; ok {
			if h.Score > prev.Score {
				*prev = h
			}
			continue
		}
		hc := h
		best[h.CaptureID] = &hc
		order = append(order, h.CaptureID)
	}
	if err := rows.Err(); err != nil {
		return res, err
	}
	for _, t := range res.Terms {
		titleTerms[t] = true
	}
	res.Capped = n >= candidatePassages
	res.Pages = len(order)
	for _, id := range order {
		h := best[id]
		for _, t := range Terms(h.Title) {
			if titleTerms[t] {
				h.TitleMatch = true
				break
			}
		}
		res.Hits = append(res.Hits, *h)
	}
	sort.SliceStable(res.Hits, func(i, j int) bool {
		a, b := res.Hits[i], res.Hits[j]
		if q.Order == ByRecency {
			if !a.Time.Equal(b.Time) {
				return a.Time.After(b.Time)
			}
			if a.Score != b.Score {
				return a.Score > b.Score
			}
			return a.CaptureID < b.CaptureID
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if !a.Time.Equal(b.Time) {
			return a.Time.After(b.Time)
		}
		return a.CaptureID < b.CaptureID
	})
	if len(res.Hits) > limit {
		res.Hits = res.Hits[:limit]
	}
	return res, nil
}

// Scopes lists the scopes that have an index file.
func (ix *Index) Scopes(ctx context.Context) ([]Scope, error) {
	files, err := filepath.Glob(filepath.Join(ix.dir, "*.db"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []Scope
	for _, f := range files {
		db, err := sql.Open("sqlite", "file:"+f+"?mode=ro&_pragma=busy_timeout(30000)")
		if err != nil {
			return nil, err
		}
		var stored string
		err = db.QueryRowContext(ctx, `SELECT v FROM meta WHERE k = 'scope'`).Scan(&stored)
		_ = db.Close()
		if err != nil {
			continue // not an index file we wrote, or not yet initialised
		}
		owner, computer, ok := strings.Cut(stored, "\x00")
		if ok {
			out = append(out, Scope{Owner: owner, Computer: computer})
		}
	}
	return out, nil
}

// PageIDs lists up to n capture ids of the scope's index after the given id,
// in id order.
func (ix *Index) PageIDs(ctx context.Context, scope Scope, after string, n int) ([]string, error) {
	s, release, err := ix.acquire(scope, false)
	if err != nil || s == nil {
		return nil, err
	}
	defer release()
	rows, err := s.db.QueryContext(ctx, `SELECT capture_id FROM pages WHERE capture_id > ? ORDER BY capture_id LIMIT ?`, after, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

const (
	// minKeptTerms is how many of a query's rarest terms are always kept.
	minKeptTerms = 3
)

// DefaultScanBudget is how many postings (passages holding a query term) one
// search may read, summed over the terms it keeps. FTS5 scores every row
// that matches (there is no early exit for ORDER BY bm25), at about 2 to 8
// microseconds a row on the build VM, so this bounds a search to well under a
// second whatever the index's size.
const DefaultScanBudget = 100000

// SetScanBudget changes the scan budget (tests and the benchmark use a small
// index).
func (ix *Index) SetScanBudget(n int) {
	ix.mu.Lock()
	ix.scanBudget = n
	ix.mu.Unlock()
}

func (ix *Index) budget() int {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.scanBudget <= 0 {
		return DefaultScanBudget
	}
	return ix.scanBudget
}

// termDF is how many passages hold the term, cached.
func (s *scopeDB) termDF(ctx context.Context, term string) (int, error) {
	size := s.passages.Load()
	s.dfMu.Lock()
	if s.df == nil || size > s.dfAtSize+s.dfAtSize/10 || size < s.dfAtSize-s.dfAtSize/10 || len(s.df) > 50000 {
		s.df, s.dfAtSize = map[string]int{}, size
	}
	if n, ok := s.df[term]; ok {
		s.dfMu.Unlock()
		return n, nil
	}
	s.dfMu.Unlock()
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM passages_fts WHERE passages_fts MATCH ?`, `"`+term+`"`).Scan(&n); err != nil {
		return 0, err
	}
	s.dfMu.Lock()
	s.df[term] = n
	s.dfMu.Unlock()
	return n, nil
}

// pruneTerms drops the terms that are in no passage (they cannot match)
// and keeps the rarest of the rest while their postings fit scanBudget,
// always keeping the minKeptTerms rarest. A question keeps the words that
// name its subject and loses "week" and "decided" on a large index: BM25
// gave those words almost no weight anyway, and each costs a scan of every
// passage that holds it. A small index prunes nothing.
func (s *scopeDB) pruneTerms(ctx context.Context, terms []string) (kept, pruned []string, err error) {
	type td struct {
		term string
		n    int
		at   int
	}
	var present []td
	for i, t := range terms {
		n, err := s.termDF(ctx, t)
		if err != nil {
			return nil, nil, err
		}
		if n == 0 {
			pruned = append(pruned, t)
			continue
		}
		present = append(present, td{t, n, i})
	}
	if len(present) <= minKeptTerms {
		for _, x := range present {
			kept = append(kept, x.term)
		}
		return kept, pruned, nil
	}
	byRarity := append([]td(nil), present...)
	sort.SliceStable(byRarity, func(i, j int) bool { return byRarity[i].n < byRarity[j].n })
	drop := map[string]bool{}
	spent := 0
	budget := s.ix.budget()
	for i, x := range byRarity {
		if i < minKeptTerms || spent+x.n <= budget {
			spent += x.n
			continue
		}
		drop[x.term] = true
		pruned = append(pruned, x.term)
	}
	for _, x := range present {
		if !drop[x.term] {
			kept = append(kept, x.term)
		}
	}
	return kept, pruned, nil
}

// Dir is the directory the index keeps its files in.
func (ix *Index) Dir() string { return ix.dir }
