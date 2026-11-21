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
// In L1 nothing returned a lead to a guest. Two routes now return what a
// lead's page became, never a lead's snippet: the conversations whose
// searches returned the lead of a capture (by-capture, management's candidate
// read), and the capture id, URL and provider title of the pages a
// conversation's searches returned and a run read (by-conversation, a story's
// research context). Both answer only the caller's own owner and computer.
// ReadSearchLeads is for tests.
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

// searchLeadIndexes are created only when pg_class lacks them. The GIN index
// serves SearchLeadsByText; the last_returned_at index serves the retention
// delete; the scope index serves a computer's newest leads.
var searchLeadIndexes = []struct{ name, ddl string }{
	{"idx_sl_tsv", `CREATE INDEX idx_sl_tsv ON search_leads USING GIN (tsv)`},
	{"idx_sl_returned", `CREATE INDEX idx_sl_returned ON search_leads(last_returned_at)`},
	{"idx_sl_scope_recent", `CREATE INDEX idx_sl_scope_recent ON search_leads(owner_id, computer_id, last_returned_at DESC)`},
	// idx_sl_capture serves SearchLeadConversationsByCapture and
	// SearchLeadPagesOfConversation: the leads whose page became a capture. A
	// plain CREATE INDEX on a live table takes a SHARE lock, so lead writes and
	// the prune wait while it builds and the routes answer 503 until the table is
	// ready again. Measured on the paper's corpusd (graph database, read only,
	// 2026-11-21): 74,771 rows, 105 MB of heap, 5,513 with a capture; a scan of
	// the heap with that filter takes 52 ms, so the build, which indexes only
	// the 5,513, is about 0.1 s: under a second, so it is not built
	// CONCURRENTLY (which would also leave an invalid index to drop after a
	// failed build). The table grows about 190,000 rows a day against a 30-day
	// retention; the build happens once, at first start after this lands.
	{"idx_sl_capture", `CREATE INDEX idx_sl_capture ON search_leads(owner_id, computer_id, capture_id) WHERE capture_id IS NOT NULL`},
	// idx_sl_conv serves SearchLeadPagesOfConversation, which every story
	// research run starts with, under a timeout that drops the story's context
	// when missed: a partial GIN index on conversation_ids over the same rows
	// (those with a capture). Measured on the paper's corpusd (graph database,
	// read only; the index built on a session-private copy of the table,
	// 2026-11-21): 94,308 rows, 6,815 with a capture; the read takes 29.7 ms
	// with a sequential scan today (14,662 buffers read) and 0.23 ms with the
	// index; the GIN build over the 6,815 takes 49 ms and the index is 136 kB
	// (idx_sl_capture's 310 ms and 1.8 MB for comparison). A plain CREATE INDEX
	// takes a SHARE lock: lead writes and the prune wait for that 0.05 to 0.3 s
	// (the routes answer 503 for it) once, at first start after this lands.
	{"idx_sl_conv", `CREATE INDEX idx_sl_conv ON search_leads USING GIN (conversation_ids) WHERE capture_id IS NOT NULL`},
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
	ready   atomic.Bool
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
	s.leads.ready.Store(true)
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
// every word of the query, newest return first. It is what L2 calls to find a
// held lead by words; a query of no words matches nothing.
func (s *Store) SearchLeadsByText(ctx context.Context, ownerID, computerID, query string, limit int) ([]SearchLead, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	return s.readLeads(ctx, `AND tsv @@ plainto_tsquery('simple', ?) AND plainto_tsquery('simple', ?)::text <> ''`, []any{query, query}, ownerID, computerID, limit)
}

// SearchLeadConversationsByCapture returns, for each named capture that a
// lead's page became, the conversations whose searches returned that lead
// (a story's research run searches in its story's conversation, so these name
// the stories whose research already met the capture's page), newest lead
// first within a capture. A capture no lead made is absent. Management's
// open-or-feed decision reads it (agentcore/management_candidates.go).
func (s *Store) SearchLeadConversationsByCapture(ctx context.Context, ownerID, computerID string, captureIDs []string) (map[string][]string, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	var ids []string
	for _, id := range captureIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	out := map[string][]string{}
	if len(ids) == 0 {
		return out, nil
	}
	raw, _ := json.Marshal(ids)
	rows, err := s.corpus().QueryContext(ctx, `SELECT capture_id, to_jsonb(conversation_ids)::text FROM search_leads
		WHERE owner_id = ? AND computer_id = ? AND capture_id IN (SELECT jsonb_array_elements_text(?::jsonb))
		ORDER BY last_returned_at DESC, url_key`, ownerID, computerID, string(raw))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, convs string
		if err := rows.Scan(&id, &convs); err != nil {
			return nil, err
		}
		var list []string
		_ = json.Unmarshal([]byte(convs), &list)
		for _, c := range list {
			if c = strings.TrimSpace(c); c != "" {
				out[id] = append(out[id], c)
			}
		}
	}
	return out, rows.Err()
}

// LeadPage is a page a conversation's searches returned and a run then read:
// the capture that read made, its URL and the title the provider gave it.
type LeadPage struct {
	CaptureID string `json:"capture_id"`
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
}

// searchLeadPagesSQL is the by-conversation read. Its array containment
// predicate and its capture filter are the ones idx_sl_conv (a partial GIN
// index) serves.
const searchLeadPagesSQL = `SELECT capture_id, url, title FROM search_leads
		WHERE owner_id = ? AND computer_id = ? AND capture_id IS NOT NULL AND conversation_ids @> ARRAY[?::text]
		ORDER BY last_returned_at DESC, url_key LIMIT ?`

// SearchLeadPagesOfConversation returns the pages read from leads that the
// named conversation's searches returned (a story's research searches in its
// story's conversation), newest return first, at most limit. A lead nobody
// read has no capture and is not listed.
func (s *Store) SearchLeadPagesOfConversation(ctx context.Context, ownerID, computerID, conversationID string, limit int) ([]LeadPage, error) {
	if !s.searchLeadsReady() {
		return nil, errSearchLeadsNotReady
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := s.corpus().QueryContext(ctx, searchLeadPagesSQL, ownerID, computerID, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeadPage
	for rows.Next() {
		var p LeadPage
		if err := rows.Scan(&p.CaptureID, &p.URL, &p.Title); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
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
