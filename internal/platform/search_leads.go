package platform

// Search leads (landing L1 of
// docs/definitions/corpus-first-search-one-verb-2026-11-19.md; problem
// docs/problems/web-searches-are-sent-without-asking-what-is-held-2026-11-19.md).
//
// A search lead is one web-search result a computer was handed: one row per
// (owner, computer, URL key) in corpusd's graph database, with the title and
// snippet, the provider's date, when it was first and last returned, how many
// times, the latest queries and conversations that returned it, when a run
// tried to read it, and the capture that read made. A lead is never embedded
// (the embedder makes about 37,000 passages a day and a day's searches return
// far more results than that); L2 finds a held lead by its URL key or by a
// full-text match on its title and snippet (SearchLeadsByText). What a search returned and nobody read survives the run here.
//
// A lead is never evidence: it is in no capture table, no search, no story,
// no event and no brief. corpusd is its custodian; the computer's host writes
// it (one call per search) and a read of the URL marks it. Visibility
// is the captures' rule, owner and computer, in the SQL of every statement.
// A lead not returned for searchLeadRetention is deleted (PruneSearchLeads).
// In L1 nothing returns a lead to a guest: ReadSearchLeads
// is for tests.
//
// The table has its own creation and readiness (ensureSearchLeads, run by
// StartSearchLeads, never on a request's path) and needs no pgvector; until it
// is made every function here answers errSearchLeadsNotReady, and a failed
// creation affects nothing else. The URL key is the
// capture side's (captureURLKey), so a lead and the capture of its page meet on
// one key.

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

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const searchLeadTable = "search_leads"

// searchLeadKeepRecent bounds the query and conversation lists of a lead.
const searchLeadKeepRecent = 8

// Bounds on what one result may store (a provider is a stranger).
const (
	searchLeadURLRunes     = 2048
	searchLeadTitleRunes   = 500
	searchLeadSnippetRunes = 4000
	searchLeadQueryRunes   = 200
	searchLeadConvRunes    = 255
)

var errSearchLeadsNotReady = errors.New("search leads: the table is not ready on this database")

// searchLeadRetention is how long a lead not returned again is kept; a lead
// returned again has its last_returned_at moved, so a live lead is not
// deleted. searchLeadPruneBatch bounds the one statement a pass runs: 50,000
// an hour is 1.2 million a day against about 190,000 rows a day written.
const (
	searchLeadRetention  = 30 * 24 * time.Hour
	searchLeadPruneBatch = 50000
	// Bounds on what one guest-reachable call may carry (a provider returns at
	// most 40 results per search; 500 is a dozen providers).
	searchLeadMaxResults = 500
	searchLeadMaxURLs    = 500
	// A capture id is a short generated id; longer is refused at the route.
	searchLeadCaptureIDRunes = 255
)

// tsv is the full-text vector of the title and snippet ('simple': no stemming,
// so the match is on the words a page used, as a URL-or-words lookup wants).
const searchLeadsDDL = `
CREATE TABLE IF NOT EXISTS search_leads (
	owner_id          TEXT NOT NULL,
	computer_id       TEXT NOT NULL,
	url_key           TEXT NOT NULL,
	url               TEXT NOT NULL,
	title             TEXT NOT NULL DEFAULT '',
	snippet           TEXT NOT NULL DEFAULT '',
	published_at      TIMESTAMPTZ NULL,
	first_returned_at TIMESTAMPTZ NOT NULL,
	last_returned_at  TIMESTAMPTZ NOT NULL,
	times_returned    INTEGER NOT NULL DEFAULT 1,
	query_keys        TEXT[] NOT NULL DEFAULT '{}',
	conversation_ids  TEXT[] NOT NULL DEFAULT '{}',
	tried_at          TIMESTAMPTZ NULL,
	capture_id        TEXT NULL,
	tsv               tsvector GENERATED ALWAYS AS (to_tsvector('simple', title || ' ' || snippet)) STORED,
	PRIMARY KEY (owner_id, computer_id, url_key)
);
`

// search_query_times is when each query was last searched on a computer: the
// time of the search itself, which a lead's last_returned_at is not (a later,
// different query that returns the same lead moves that). One row per (owner,
// computer, query key), written in the transaction that writes the search's
// leads, pruned with them. Id and key columns are COLLATE "C" so equality and
// the primary key do not depend on the database's locale.
const searchQueryTimesDDL = `
CREATE TABLE IF NOT EXISTS search_query_times (
	owner_id         TEXT COLLATE "C" NOT NULL,
	computer_id      TEXT COLLATE "C" NOT NULL,
	query_key        TEXT COLLATE "C" NOT NULL,
	last_searched_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (owner_id, computer_id, query_key)
);
`

// The retention delete reads this index; the table is small (one row per query
// a computer searched in 30 days), so it is made plainly.
const searchQueryTimesIndexDDL = `CREATE INDEX IF NOT EXISTS idx_sqt_searched ON search_query_times(last_searched_at)`

// searchLeadConvIndex serves the conversation read (conversation_ids @> ...). It is built
// CONCURRENTLY, after the table is ready, so a deploy onto a table that holds
// a day of leads never blocks the lead writes; until it is built the
// conversation read is not made (its counts are unknown, not zero).
const searchLeadConvIndex = "idx_sl_conv"

// searchLeadIndexes are created only when pg_class lacks them. The GIN index
// serves SearchLeadsByText; the last_returned_at index serves the retention
// delete; the scope index serves a computer's newest leads.
var searchLeadIndexes = []struct{ name, ddl string }{
	{"idx_sl_tsv", `CREATE INDEX idx_sl_tsv ON search_leads USING GIN (tsv)`},
	{"idx_sl_returned", `CREATE INDEX idx_sl_returned ON search_leads(last_returned_at)`},
	{"idx_sl_scope_recent", `CREATE INDEX idx_sl_scope_recent ON search_leads(owner_id, computer_id, last_returned_at DESC)`},
}

// SearchLeadHit is one result of a provider's arrival.
type SearchLeadHit struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Snippet     string `json:"snippet"`
	PublishedAt string `json:"published_at,omitempty"`
}

// SearchLeadsWrite is one arrival: its results, the query that asked and the
// conversation that asked.
type SearchLeadsWrite struct {
	OwnerID        string          `json:"owner_id"`
	ComputerID     string          `json:"computer_id"`
	Query          string          `json:"query,omitempty"`
	ConversationID string          `json:"conversation_id,omitempty"`
	Results        []SearchLeadHit `json:"results"`
}

// SearchLead is a row as a test reads it.
type SearchLead struct {
	URLKey, URL, Title, Snippet     string
	PublishedAt                     *time.Time
	FirstReturnedAt, LastReturnedAt time.Time
	TimesReturned                   int
	QueryKeys, ConversationIDs      []string
	TriedAt                         *time.Time
	CaptureID                       string
}

// leadQueryKey is a query as the lead lists keep it: lower case, spaces
// collapsed, bounded.
func leadQueryKey(q string) string {
	return clipRunes(strings.Join(strings.Fields(strings.ToLower(q)), " "), searchLeadQueryRunes)
}

// searchLeadState is the table's own readiness: made by ensureSearchLeads,
// independent of the vector tables.
type searchLeadState struct {
	ready     atomic.Bool
	convIndex atomic.Bool
	started sync.Once
}

func (s *Store) searchLeadsReady() bool { return s != nil && s.leads.ready.Load() }

// ensureSearchLeads makes the table and its indexes (idempotent) and marks the
// table ready.
func (s *Store) ensureSearchLeads(ctx context.Context) error {
	db := s.corpus()
	if _, err := db.ExecContext(ctx, searchLeadsDDL); err != nil {
		return fmt.Errorf("search leads: create table: %w", err)
	}
	for _, ix := range searchLeadIndexes {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass(?) IS NOT NULL`, ix.name).Scan(&exists); err != nil {
			return fmt.Errorf("search leads: read index %s: %w", ix.name, err)
		}
		if exists {
			continue
		}
		if _, err := db.ExecContext(ctx, ix.ddl); err != nil {
			return fmt.Errorf("search leads: create index %s: %w", ix.name, err)
		}
	}
	if _, err := db.ExecContext(ctx, searchQueryTimesDDL); err != nil {
		return fmt.Errorf("search leads: create query times: %w", err)
	}
	if _, err := db.ExecContext(ctx, searchQueryTimesIndexDDL); err != nil {
		return fmt.Errorf("search leads: create query times index: %w", err)
	}
	// Lead writes and the other reads are on from here; the conversation
	// index is built after, without a lock that blocks writes. A failed build
	// does not undo that: the maintenance loop retries it (retryConvIndex).
	s.leads.ready.Store(true)
	if err := s.ensureConvIndex(ctx); err != nil {
		log.Printf("corpusd search_leads op=ensure_conv_index err=%v (lead writes are on; conversation counts are unknown until the index is built; retrying)", err)
	}
	return nil
}

// retryConvIndex builds the conversation index when it is not built. It
// returns (0, true) once built, else the next wait (doubled, at most five
// minutes) and false.
func (s *Store) retryConvIndex(ctx context.Context, wait time.Duration) (time.Duration, bool) {
	if s.leads.convIndex.Load() {
		return 0, true
	}
	if err := s.ensureConvIndex(ctx); err != nil {
		next := min(wait*2, 5*time.Minute)
		log.Printf("corpusd search_leads op=ensure_conv_index err=%v (lead writes are on; conversation counts are unknown; retry in %s)", err, next)
		return next, false
	}
	return 0, true
}

// ensureConvIndex builds the conversation index CONCURRENTLY (a failed build
// leaves an invalid index, which is dropped and built again) and marks it.
func (s *Store) ensureConvIndex(ctx context.Context) error {
	db := s.corpus()
	var valid sql.NullBool
	err := db.QueryRowContext(ctx, `SELECT indisvalid FROM pg_index WHERE indexrelid = to_regclass(?)`, searchLeadConvIndex).Scan(&valid)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("search leads: read index %s: %w", searchLeadConvIndex, err)
	}
	if err == nil && !valid.Bool {
		if _, err := db.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS `+searchLeadConvIndex); err != nil {
			return fmt.Errorf("search leads: drop invalid index %s: %w", searchLeadConvIndex, err)
		}
		err = sql.ErrNoRows
	}
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := db.ExecContext(ctx, `CREATE INDEX CONCURRENTLY IF NOT EXISTS `+searchLeadConvIndex+` ON search_leads USING GIN (conversation_ids)`); err != nil {
			return fmt.Errorf("search leads: create index %s: %w", searchLeadConvIndex, err)
		}
	}
	s.leads.convIndex.Store(true)
	return nil
}

// PruneSearchLeads deletes at most limit leads not returned within keep, in one
// statement, oldest first, and returns how many.
func (s *Store) PruneSearchLeads(ctx context.Context, keep time.Duration, limit int) (int, error) {
	if !s.searchLeadsReady() {
		return 0, errSearchLeadsNotReady
	}
	res, err := s.corpus().ExecContext(ctx, `DELETE FROM search_leads WHERE ctid IN (
		SELECT ctid FROM search_leads WHERE last_returned_at < now() - make_interval(secs => ?) ORDER BY last_returned_at LIMIT ?)`,
		keep.Seconds(), limit)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if _, err := s.corpus().ExecContext(ctx, `DELETE FROM search_query_times WHERE ctid IN (
		SELECT ctid FROM search_query_times WHERE last_searched_at < now() - make_interval(secs => ?) ORDER BY last_searched_at LIMIT ?)`,
		keep.Seconds(), limit); err != nil {
		return int(n), err
	}
	return int(n), nil
}

// StartSearchLeads makes the table a little after boot (retrying with a
// doubling wait; a failure is logged and affects only lead writes) and then
// prunes leads past their retention once an hour.
func (s *Store) StartSearchLeads(ctx context.Context) {
	if s == nil {
		return
	}
	s.leads.started.Do(func() {
		go func() {
			wait := 5 * time.Second
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}
				if s.searchLeadsReady() {
					if next, built := s.retryConvIndex(ctx, wait); !built {
						wait = next
						continue
					}
					n, err := s.PruneSearchLeads(ctx, searchLeadRetention, searchLeadPruneBatch)
					log.Printf("corpusd search_leads op=prune deleted=%d err=%v", n, err)
					wait = time.Hour
					continue
				}
				if err := s.ensureSearchLeads(ctx); err != nil {
					log.Printf("corpusd search_leads op=ensure err=%v (lead writes are off; retry in %s)", err, wait*2)
					wait = min(wait*2, 5*time.Minute)
					continue
				}
				wait = time.Minute
			}
		}()
	})
}

// SearchLeadsByText returns a computer's leads whose title or snippet matches
// every word of the query, newest return first. L2 reads by words through
// SearchLeadsByWords (the guest's half-of-the-words rule); this exact-match
// read is for tests and lookups. A query of no words matches nothing.
func (s *Store) SearchLeadsByText(ctx context.Context, ownerID, computerID, query string, limit int) ([]SearchLead, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	return s.readLeads(ctx, `AND tsv @@ plainto_tsquery('simple', ?) AND plainto_tsquery('simple', ?)::text <> ''`, []any{query, query}, ownerID, computerID, limit)
}

// SearchQueryTime is when this exact query key (leadQueryKey) was last
// searched on the computer; ok is false when it never was (or was pruned).
func (s *Store) SearchQueryTime(ctx context.Context, ownerID, computerID, queryKey string) (time.Time, bool, error) {
	if !s.searchLeadsReady() {
		return time.Time{}, false, errSearchLeadsNotReady
	}
	var at time.Time
	err := s.corpus().QueryRowContext(ctx, `SELECT last_searched_at FROM search_query_times WHERE owner_id = ? AND computer_id = ? AND query_key = ?`,
		ownerID, computerID, queryKey).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	return at.UTC(), err == nil, err
}

// SearchLeadsBySameQuery returns the computer's leads that a search of this
// query key returned at or after since (the time SearchQueryTime gave): the
// earlier search's leads, newest return first. A lead keeps its latest eight
// query keys, so a lead returned by eight later searches no longer names it.
func (s *Store) SearchLeadsBySameQuery(ctx context.Context, ownerID, computerID, queryKey string, since time.Time, limit int) ([]SearchLead, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	if strings.TrimSpace(queryKey) == "" {
		return nil, nil
	}
	return s.readLeads(ctx, `AND last_returned_at >= ? AND ? = ANY(query_keys)`, []any{since, queryKey}, ownerID, computerID, limit)
}

// leadWordsNeeded is how many of n query words a lead's title and snippet must
// hold: the guest's corpus search rule (all of one or two words, else half,
// rounded up), so a question with a word the headline lacks ("news") still
// finds the headline.
func leadWordsNeeded(n int) int {
	if n <= 2 {
		return n
	}
	return (n + 1) / 2
}

// SearchLeadsByWords returns the computer's leads returned since the given
// time whose title and snippet hold at least leadWordsNeeded of the words
// (already prepared: lower case, function words dropped; see
// captureterms.QueryWords), newest return first. The GIN index narrows by any
// of the words; the count decides. No words match nothing.
func (s *Store) SearchLeadsByWords(ctx context.Context, ownerID, computerID string, words []string, since time.Time, limit int) ([]SearchLead, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	if len(words) == 0 {
		return nil, nil
	}
	var anyOf, count []string
	var args []any
	args = append(args, since)
	for range words {
		anyOf = append(anyOf, `plainto_tsquery('simple', ?)`)
	}
	for _, w := range words {
		args = append(args, w)
	}
	for _, w := range words {
		count = append(count, `(CASE WHEN tsv @@ plainto_tsquery('simple', ?) THEN 1 ELSE 0 END)`)
		args = append(args, w)
	}
	where := `AND last_returned_at >= ? AND tsv @@ (` + strings.Join(anyOf, ` || `) + `) AND ` + strings.Join(count, ` + `) + ` >= ` + fmt.Sprint(leadWordsNeeded(len(words)))
	return s.readLeads(ctx, where, args, ownerID, computerID, limit)
}

var errSearchLeadsConvIndex = errors.New("search leads: the conversation index is not built yet")

// SearchLeadsByConversation returns the computer's leads that a search of the
// conversation returned since the given time, newest return first.
func (s *Store) SearchLeadsByConversation(ctx context.Context, ownerID, computerID, conversationID string, since time.Time, limit int) ([]SearchLead, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	if !s.leads.convIndex.Load() {
		return nil, errSearchLeadsConvIndex
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil
	}
	return s.readLeads(ctx, `AND last_returned_at >= ? AND conversation_ids @> ARRAY[?]::text[]`, []any{since, clipRunes(conversationID, searchLeadConvRunes)}, ownerID, computerID, limit)
}

// SearchLeadsByURL returns a computer's lead for each URL (by capture key).
func (s *Store) SearchLeadsByURL(ctx context.Context, ownerID, computerID string, urls []string, limit int) ([]SearchLead, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	var keys []string
	for _, u := range urls {
		if strings.TrimSpace(u) != "" {
			keys = append(keys, captureURLKey(u))
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	kj, _ := json.Marshal(keys)
	return s.readLeads(ctx, `AND url_key IN (SELECT jsonb_array_elements_text(?::jsonb))`, []any{string(kj)}, ownerID, computerID, limit)
}

// WriteSearchLeads upserts the results of one arrival in one transaction and
// returns the rows written. A URL already held counts one more return, keeps
// its first time and any text the new return lacks, takes a provider date when
// the new return has one, and takes the newest query and conversation first
// (eight at most).
func (s *Store) WriteSearchLeads(ctx context.Context, w SearchLeadsWrite) (rows int, err error) {
	if strings.TrimSpace(w.OwnerID) == "" || strings.TrimSpace(w.ComputerID) == "" {
		return 0, errors.New("search leads: owner_id and computer_id are required")
	}
	if !s.searchLeadsReady() {
		return 0, errSearchLeadsNotReady
	}
	type row struct {
		Key, URL, Title, Snippet string
		Published                *string
	}
	at := map[string]int{}
	var in []row
	for _, h := range w.Results {
		u := strings.TrimSpace(h.URL)
		if u == "" {
			continue
		}
		u = clipRunes(u, searchLeadURLRunes)
		key := captureURLKey(u)
		if _, dup := at[key]; dup {
			continue
		}
		r := row{Key: key, URL: u, Title: clipRunes(strings.TrimSpace(h.Title), searchLeadTitleRunes), Snippet: clipRunes(strings.TrimSpace(h.Snippet), searchLeadSnippetRunes)}
		if t := parseCaptureTime(h.PublishedAt); t.Valid {
			p := t.Time.UTC().Format(time.RFC3339Nano)
			r.Published = &p
		}
		at[key] = len(in)
		in = append(in, r)
	}
	if len(in) == 0 {
		return 0, nil
	}
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	keys := make([]string, len(in))
	for i, r := range in {
		keys[i] = r.Key
	}
	keysJSON, _ := json.Marshal(keys)
	// The held text of these rows, locked: a return that lacks a title or
	// snippet keeps the held one.
	held, err := tx.QueryContext(ctx, `SELECT url_key, title, snippet FROM search_leads
		WHERE owner_id = ? AND computer_id = ? AND url_key IN (SELECT jsonb_array_elements_text(?::jsonb)) ORDER BY url_key FOR UPDATE`, w.OwnerID, w.ComputerID, string(keysJSON))
	if err != nil {
		return 0, err
	}
	for held.Next() {
		var key, title, snippet string
		if err := held.Scan(&key, &title, &snippet); err != nil {
			_ = held.Close()
			return 0, err
		}
		r := &in[at[key]]
		if r.Title == "" {
			r.Title = title
		}
		if r.Snippet == "" {
			r.Snippet = snippet
		}
	}
	if err := held.Err(); err != nil {
		_ = held.Close()
		return 0, err
	}
	_ = held.Close()
	recs, _ := json.Marshal(func() []map[string]any {
		out := make([]map[string]any, len(in))
		for i, r := range in {
			out[i] = map[string]any{"url_key": r.Key, "url": r.URL, "title": r.Title, "snippet": r.Snippet, "published_at": r.Published}
		}
		return out
	}())
	queries, convs := []string{}, []string{}
	if q := leadQueryKey(w.Query); q != "" {
		queries = append(queries, q)
	}
	if c := clipRunes(strings.TrimSpace(w.ConversationID), searchLeadConvRunes); c != "" {
		convs = append(convs, c)
	}
	qJSON, _ := json.Marshal(queries)
	cJSON, _ := json.Marshal(convs)
	merge := func(col string) string {
		return fmt.Sprintf(`ARRAY(SELECT x FROM (SELECT x, min(o) AS o FROM unnest(EXCLUDED.%[1]s || search_leads.%[1]s) WITH ORDINALITY AS u(x, o) GROUP BY x) s ORDER BY o LIMIT %d)`, col, searchLeadKeepRecent)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO search_leads
		(owner_id, computer_id, url_key, url, title, snippet, published_at, first_returned_at, last_returned_at, times_returned, query_keys, conversation_ids)
		SELECT ?::text, ?::text, r.url_key, r.url, r.title, r.snippet, r.published_at, now(), now(), 1,
			ARRAY(SELECT jsonb_array_elements_text(?::jsonb)), ARRAY(SELECT jsonb_array_elements_text(?::jsonb))
		FROM jsonb_to_recordset(?::jsonb) AS r(url_key text, url text, title text, snippet text, published_at timestamptz)
		ON CONFLICT (owner_id, computer_id, url_key) DO UPDATE SET
			title = EXCLUDED.title, snippet = EXCLUDED.snippet,
			published_at = COALESCE(EXCLUDED.published_at, search_leads.published_at),
			last_returned_at = EXCLUDED.last_returned_at,
			times_returned = search_leads.times_returned + 1,
			query_keys = `+merge("query_keys")+`,
			conversation_ids = `+merge("conversation_ids"),
		w.OwnerID, w.ComputerID, string(qJSON), string(cJSON), string(recs)); err != nil {
		return 0, err
	}
	if len(queries) > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO search_query_times (owner_id, computer_id, query_key, last_searched_at) VALUES (?, ?, ?, now())
			ON CONFLICT (owner_id, computer_id, query_key) DO UPDATE SET last_searched_at = EXCLUDED.last_searched_at`,
			w.OwnerID, w.ComputerID, queries[0]); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(in), nil
}

// MarkSearchLeadsTried records that a run tried to read these URLs: tried_at is
// set once (the first time stays), and capture_id when the read made or found a
// capture. A URL that is no lead is no error. It returns the leads marked.
func (s *Store) MarkSearchLeadsTried(ctx context.Context, ownerID, computerID string, urls []string, captureID string) (int, error) {
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(computerID) == "" {
		return 0, errors.New("search leads: owner_id and computer_id are required")
	}
	if !s.searchLeadsReady() {
		return 0, errSearchLeadsNotReady
	}
	captureID = clipRunes(strings.TrimSpace(captureID), searchLeadCaptureIDRunes)
	var keys []string
	seen := map[string]bool{}
	for _, u := range urls {
		if strings.TrimSpace(u) == "" {
			continue
		}
		// Clipped as the write side clips, so a long URL keys the same.
		if k := captureURLKey(clipRunes(strings.TrimSpace(u), searchLeadURLRunes)); !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return 0, nil
	}
	keysJSON, _ := json.Marshal(keys)
	res, err := s.corpus().ExecContext(ctx, `UPDATE search_leads SET tried_at = COALESCE(tried_at, now()),
		capture_id = CASE WHEN ?::text = '' THEN capture_id ELSE ?::text END
		WHERE owner_id = ? AND computer_id = ? AND url_key IN (SELECT jsonb_array_elements_text(?::jsonb))`,
		captureID, captureID, ownerID, computerID, string(keysJSON))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// markSearchLeadsCaptured is the capture side's mark: one statement for the
// captures just stored, matching a lead by the capture's URL key or its
// canonical URL's. It never fails the caller.
func (s *Store) markSearchLeadsCaptured(ctx context.Context, captures []objectgraph.Object) {
	if !s.searchLeadsReady() {
		return
	}
	type rec struct {
		Owner   string `json:"owner_id"`
		Comp    string `json:"computer_id"`
		Key     string `json:"url_key"`
		Capture string `json:"capture_id"`
	}
	var recs []rec
	for _, obj := range captures {
		p, ok := projectCapture(obj)
		if !ok {
			continue
		}
		recs = append(recs, rec{p.owner, p.computer, p.urlKey, p.id})
		if p.canonicalKey != p.urlKey && strings.TrimSpace(p.canonicalURL) != "" {
			recs = append(recs, rec{p.owner, p.computer, p.canonicalKey, p.id})
		}
	}
	if len(recs) == 0 {
		return
	}
	raw, _ := json.Marshal(recs)
	if _, err := s.corpus().ExecContext(ctx, `UPDATE search_leads l SET capture_id = r.capture_id, tried_at = COALESCE(l.tried_at, now())
		FROM jsonb_to_recordset(?::jsonb) AS r(owner_id text, computer_id text, url_key text, capture_id text)
		WHERE l.owner_id = r.owner_id AND l.computer_id = r.computer_id AND l.url_key = r.url_key`, string(raw)); err != nil {
		log.Printf("corpusd search_leads op=lead_capture_mark captures=%d err=%v", len(captures), err)
	}
}

// ReadSearchLeads returns a computer's leads, newest return first. No route
// returns leads to a guest in L1.
func (s *Store) ReadSearchLeads(ctx context.Context, ownerID, computerID string, limit int) ([]SearchLead, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	return s.readLeads(ctx, ``, nil, ownerID, computerID, limit)
}

// readLeads reads a computer's leads under an extra predicate (which starts
// with AND and takes its own args), newest return first.
func (s *Store) readLeads(ctx context.Context, where string, whereArgs []any, ownerID, computerID string, limit int) ([]SearchLead, error) {
	args := append([]any{ownerID, computerID}, whereArgs...)
	args = append(args, limit)
	rows, err := s.corpus().QueryContext(ctx, `SELECT url_key, url, title, snippet, published_at, first_returned_at, last_returned_at, times_returned,
		to_jsonb(query_keys)::text, to_jsonb(conversation_ids)::text, tried_at, COALESCE(capture_id, '')
		FROM search_leads WHERE owner_id = ? AND computer_id = ? `+where+` ORDER BY last_returned_at DESC, url_key LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchLead
	for rows.Next() {
		var l SearchLead
		var pub, tried sql.NullTime
		var qs, cs string
		if err := rows.Scan(&l.URLKey, &l.URL, &l.Title, &l.Snippet, &pub, &l.FirstReturnedAt, &l.LastReturnedAt, &l.TimesReturned, &qs, &cs, &tried, &l.CaptureID); err != nil {
			return nil, err
		}
		if pub.Valid {
			t := pub.Time.UTC()
			l.PublishedAt = &t
		}
		if tried.Valid {
			t := tried.Time.UTC()
			l.TriedAt = &t
		}
		_ = json.Unmarshal([]byte(qs), &l.QueryKeys)
		_ = json.Unmarshal([]byte(cs), &l.ConversationIDs)
		l.FirstReturnedAt, l.LastReturnedAt = l.FirstReturnedAt.UTC(), l.LastReturnedAt.UTC()
		out = append(out, l)
	}
	return out, rows.Err()
}
