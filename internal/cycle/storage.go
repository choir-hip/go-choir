package cycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
	_ "modernc.org/sqlite"
)

type Storage struct {
	DB *sql.DB
}

func NewStorage(dbPath string) (*Storage, error) {
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sourcecycled db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite db: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &Storage{DB: db}, nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sources (
			id                         TEXT PRIMARY KEY,
			type                       TEXT NOT NULL,
			url                        TEXT NOT NULL,
			name                       TEXT NOT NULL,
			verticals                  TEXT NOT NULL DEFAULT '[]',
			languages                  TEXT NOT NULL DEFAULT '[]',
			regions                    TEXT NOT NULL DEFAULT '[]',
			jurisdictions              TEXT NOT NULL DEFAULT '[]',
			tier                       TEXT NOT NULL DEFAULT '',
			poll_interval_seconds      INTEGER NOT NULL DEFAULT 900,
			rate_limit                 TEXT NOT NULL DEFAULT '',
			conditional_request_mode   TEXT NOT NULL DEFAULT '',
			user_agent                 TEXT NOT NULL DEFAULT '',
			tos_class                  TEXT NOT NULL DEFAULT '',
			robots_policy              TEXT NOT NULL DEFAULT '',
			auth_policy                TEXT NOT NULL DEFAULT '',
			store_body_policy          TEXT NOT NULL DEFAULT '',
			retention_days             INTEGER NOT NULL DEFAULT 0,
			official                   INTEGER NOT NULL DEFAULT 0,
			source_standing            TEXT NOT NULL DEFAULT '',
			status                     TEXT NOT NULL DEFAULT 'active',
			last_polled                TEXT NOT NULL DEFAULT '',
			last_etag                  TEXT NOT NULL DEFAULT '',
			last_modified              TEXT NOT NULL DEFAULT '',
			updated_at                 TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS fetches (
			fetch_id          TEXT PRIMARY KEY,
			source_id         TEXT NOT NULL,
			source_type       TEXT NOT NULL,
			request_url       TEXT NOT NULL,
			canonical_url     TEXT NOT NULL DEFAULT '',
			status_code       INTEGER NOT NULL DEFAULT 0,
			status            TEXT NOT NULL,
			started_at        TEXT NOT NULL,
			ended_at          TEXT NOT NULL DEFAULT '',
			response_etag     TEXT NOT NULL DEFAULT '',
			response_modified TEXT NOT NULL DEFAULT '',
			content_hash      TEXT NOT NULL DEFAULT '',
			raw_snapshot_ref  TEXT NOT NULL DEFAULT '',
			error_class       TEXT NOT NULL DEFAULT '',
			error             TEXT NOT NULL DEFAULT '',
			item_count        INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id               TEXT PRIMARY KEY,
			source_id        TEXT NOT NULL,
			source_type      TEXT NOT NULL DEFAULT '',
			fetch_id         TEXT NOT NULL DEFAULT '',
			original_id      TEXT NOT NULL DEFAULT '',
			title            TEXT NOT NULL DEFAULT '',
			body             TEXT NOT NULL DEFAULT '',
			url              TEXT NOT NULL DEFAULT '',
			canonical_url    TEXT NOT NULL DEFAULT '',
			published        TEXT NOT NULL DEFAULT '',
			fetched_at       TEXT NOT NULL DEFAULT '',
			verticals        TEXT NOT NULL DEFAULT '[]',
			language         TEXT NOT NULL DEFAULT '',
			region           TEXT NOT NULL DEFAULT '',
			content_hash      TEXT NOT NULL DEFAULT '',
			raw_json          TEXT NOT NULL DEFAULT '',
			evidence_level    TEXT NOT NULL DEFAULT '',
			vintage_policy    TEXT NOT NULL DEFAULT '',
			lookahead_status  TEXT NOT NULL DEFAULT '',
			release_date      TEXT NOT NULL DEFAULT '',
			created_at        TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_source_published ON items(source_id, published)`,
		`CREATE INDEX IF NOT EXISTS idx_items_content_hash ON items(content_hash)`,
		`CREATE TABLE IF NOT EXISTS cycles (
			cycle_id      TEXT PRIMARY KEY,
			started_at    TEXT NOT NULL,
			ended_at      TEXT NOT NULL DEFAULT '',
			status        TEXT NOT NULL,
			item_count    INTEGER NOT NULL DEFAULT 0,
			fetch_count   INTEGER NOT NULL DEFAULT 0,
			error         TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS cycle_events (
			event_id      TEXT PRIMARY KEY,
			cycle_id      TEXT NOT NULL,
			source_id     TEXT NOT NULL DEFAULT '',
			kind          TEXT NOT NULL,
			message       TEXT NOT NULL DEFAULT '',
			metadata_json TEXT NOT NULL DEFAULT '{}',
			created_at    TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS issues (
			id          TEXT PRIMARY KEY,
			timestamp   TEXT,
			content     TEXT,
			item_ids    TEXT,
			citation_map_json TEXT NOT NULL DEFAULT '{}',
			model       TEXT,
			tokens      INTEGER
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return migrateTables(db)
}

func migrateTables(db *sql.DB) error {
	alterStatements := []string{
		`ALTER TABLE sources ADD COLUMN languages TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE sources ADD COLUMN regions TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE sources ADD COLUMN jurisdictions TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE sources ADD COLUMN tier TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN poll_interval_seconds INTEGER NOT NULL DEFAULT 900`,
		`ALTER TABLE sources ADD COLUMN rate_limit TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN conditional_request_mode TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN user_agent TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN tos_class TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN robots_policy TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN auth_policy TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN store_body_policy TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE sources ADD COLUMN official INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE sources ADD COLUMN source_standing TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sources ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN source_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN fetch_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN canonical_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN language TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN region TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN content_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN evidence_level TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN vintage_policy TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN lookahead_status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN release_date TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE issues ADD COLUMN citation_map_json TEXT NOT NULL DEFAULT '{}'`,
	}
	for _, stmt := range alterStatements {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	return nil
}

func (s *Storage) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *Storage) SaveSources(registry *sources.Registry) error {
	if registry == nil {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO sources (
		id, type, url, name, verticals, languages, regions, jurisdictions, tier,
		poll_interval_seconds, rate_limit, conditional_request_mode, user_agent,
		tos_class, robots_policy, auth_policy, store_body_policy, retention_days,
		official, source_standing, status, last_polled, last_etag, last_modified, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		type=excluded.type, url=excluded.url, name=excluded.name,
		verticals=excluded.verticals, languages=excluded.languages,
		regions=excluded.regions, jurisdictions=excluded.jurisdictions,
		tier=excluded.tier, poll_interval_seconds=excluded.poll_interval_seconds,
		rate_limit=excluded.rate_limit, conditional_request_mode=excluded.conditional_request_mode,
		user_agent=excluded.user_agent, tos_class=excluded.tos_class,
		robots_policy=excluded.robots_policy, auth_policy=excluded.auth_policy,
		store_body_policy=excluded.store_body_policy, retention_days=excluded.retention_days,
		official=excluded.official, source_standing=excluded.source_standing,
		status=excluded.status, last_polled=excluded.last_polled,
		last_etag=excluded.last_etag, last_modified=excluded.last_modified,
		updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, source := range registry.Sources {
		if strings.TrimSpace(source.ID) == "" {
			return fmt.Errorf("source id is required")
		}
		status := strings.TrimSpace(source.Status)
		if status == "" {
			status = "active"
		}
		if _, err := stmt.Exec(
			source.ID, source.Type, source.URL, source.Name,
			mustJSON(source.Verticals), mustJSON(source.Languages), mustJSON(source.Regions), mustJSON(source.Jurisdictions),
			source.Tier, source.PollIntervalSeconds, source.RateLimit, source.ConditionalMode,
			source.EffectiveUserAgent(registry.UserAgent), source.TOSClass, source.RobotsPolicy,
			source.AuthPolicy, source.StoreBodyPolicy, source.RetentionDays, boolInt(source.Official),
			source.SourceStanding, status, formatTime(source.LastPolled), source.LastETag, source.LastModified, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) SaveFetches(fetches []sources.FetchRecord) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO fetches (
		fetch_id, source_id, source_type, request_url, canonical_url, status_code,
		status, started_at, ended_at, response_etag, response_modified,
		content_hash, raw_snapshot_ref, error_class, error, item_count
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, fetch := range fetches {
		if strings.TrimSpace(fetch.FetchID) == "" {
			continue
		}
		if _, err := stmt.Exec(fetch.FetchID, fetch.SourceID, fetch.SourceType, fetch.RequestURL,
			fetch.CanonicalURL, fetch.StatusCode, fetch.Status, formatTime(fetch.StartedAt),
			formatTime(fetch.EndedAt), fetch.ResponseETag, fetch.ResponseModified, fetch.ContentHash,
			fetch.RawSnapshotRef, fetch.ErrorClass, fetch.Error, fetch.ItemCount); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) SaveItems(items []sources.Item) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO items (
		id, source_id, source_type, fetch_id, original_id, title, body, url,
		canonical_url, published, fetched_at, verticals, language, region,
		content_hash, raw_json, evidence_level, vintage_policy,
		lookahead_status, release_date, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		if item.ID == "" {
			return fmt.Errorf("item id is required")
		}
		createdAt := item.FetchedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		_, err := stmt.Exec(item.ID, item.SourceID, item.SourceType, item.FetchID, item.OriginalID,
			item.Title, item.Body, item.URL, item.CanonicalURL, formatTime(item.Published),
			formatTime(item.FetchedAt), mustJSON(item.Verticals), item.Language, item.Region,
			item.ContentHash, item.RawJSON, item.EvidenceLevel, item.VintagePolicy,
			item.LookaheadStatus, item.ReleaseDate, formatTime(createdAt))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Storage) SaveIssue(content string, itemIDs []string, model string, tokens int) error {
	return s.SaveIssueManifest(content, itemIDs, nil, model, tokens)
}

func (s *Storage) SaveIssueManifest(content string, itemIDs []string, citationMap map[string][]string, model string, tokens int) error {
	id := fmt.Sprintf("issue-%d", time.Now().Unix())
	itemIDsJSON, _ := json.Marshal(itemIDs)
	citationMapJSON, _ := json.Marshal(citationMap)
	if citationMap == nil {
		citationMapJSON = []byte("{}")
	}
	_, err := s.DB.Exec(`INSERT INTO issues (id, timestamp, content, item_ids, citation_map_json, model, tokens) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, time.Now().UTC().Format(time.RFC3339), content, string(itemIDsJSON), string(citationMapJSON), model, tokens)
	return err
}

func (s *Storage) StartCycle(ctx context.Context) (string, error) {
	now := time.Now().UTC()
	cycleID := "cycle_" + sources.ContentHash(now.Format(time.RFC3339Nano))[:24]
	_, err := s.DB.ExecContext(ctx, `INSERT INTO cycles (cycle_id, started_at, status) VALUES (?, ?, 'running')`, cycleID, formatTime(now))
	return cycleID, err
}

func (s *Storage) FinishCycle(ctx context.Context, cycleID, status string, itemCount, fetchCount int, cycleErr error) error {
	if status == "" {
		status = "completed"
	}
	errText := ""
	if cycleErr != nil {
		errText = cycleErr.Error()
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE cycles SET ended_at = ?, status = ?, item_count = ?, fetch_count = ?, error = ? WHERE cycle_id = ?`,
		formatTime(time.Now().UTC()), status, itemCount, fetchCount, errText, cycleID)
	return err
}

func (s *Storage) RecordCycleEvent(ctx context.Context, cycleID, sourceID, kind, message string, metadata map[string]any) error {
	now := time.Now().UTC()
	eventID := "cycleevt_" + sources.ContentHash(cycleID, sourceID, kind, message, now.Format(time.RFC3339Nano))[:24]
	_, err := s.DB.ExecContext(ctx, `INSERT INTO cycle_events (event_id, cycle_id, source_id, kind, message, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		eventID, cycleID, sourceID, kind, message, mustJSON(metadata), formatTime(now))
	return err
}

func (s *Storage) CountItems(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count)
	return count, err
}

func (s *Storage) CountFetches(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM fetches`).Scan(&count)
	return count, err
}

func (s *Storage) SearchItems(ctx context.Context, query string, limit int) ([]sources.Item, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query = strings.TrimSpace(strings.ToLower(query))
	sqlQuery := `SELECT id, source_id, source_type, fetch_id, original_id, title, body, url,
		canonical_url, published, fetched_at, verticals, language, region, content_hash,
		raw_json, evidence_level, vintage_policy, lookahead_status, release_date
		FROM items`
	args := []any{}
	if query != "" {
		sqlQuery += ` WHERE lower(title) LIKE ? OR lower(body) LIKE ? OR lower(source_id) LIKE ?`
		needle := "%" + query + "%"
		args = append(args, needle, needle, needle)
	}
	sqlQuery += ` ORDER BY published DESC, fetched_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sources.Item
	for rows.Next() {
		var item sources.Item
		var published, fetchedAt, verticals string
		if err := rows.Scan(&item.ID, &item.SourceID, &item.SourceType, &item.FetchID, &item.OriginalID,
			&item.Title, &item.Body, &item.URL, &item.CanonicalURL, &published, &fetchedAt,
			&verticals, &item.Language, &item.Region, &item.ContentHash, &item.RawJSON,
			&item.EvidenceLevel, &item.VintagePolicy, &item.LookaheadStatus, &item.ReleaseDate); err != nil {
			return nil, err
		}
		item.Published = parseStoredTime(published)
		item.FetchedAt = parseStoredTime(fetchedAt)
		_ = json.Unmarshal([]byte(verticals), &item.Verticals)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Storage) GetItem(ctx context.Context, itemID string) (sources.Item, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return sources.Item{}, fmt.Errorf("item id is required")
	}
	row := s.DB.QueryRowContext(ctx, `SELECT id, source_id, source_type, fetch_id, original_id, title, body, url,
		canonical_url, published, fetched_at, verticals, language, region, content_hash,
		raw_json, evidence_level, vintage_policy, lookahead_status, release_date
		FROM items WHERE id = ?`, itemID)
	var item sources.Item
	var published, fetchedAt, verticals string
	if err := row.Scan(&item.ID, &item.SourceID, &item.SourceType, &item.FetchID, &item.OriginalID,
		&item.Title, &item.Body, &item.URL, &item.CanonicalURL, &published, &fetchedAt,
		&verticals, &item.Language, &item.Region, &item.ContentHash, &item.RawJSON,
		&item.EvidenceLevel, &item.VintagePolicy, &item.LookaheadStatus, &item.ReleaseDate); err != nil {
		return sources.Item{}, err
	}
	item.Published = parseStoredTime(published)
	item.FetchedAt = parseStoredTime(fetchedAt)
	_ = json.Unmarshal([]byte(verticals), &item.Verticals)
	return item, nil
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	if data == nil {
		return "{}"
	}
	return string(data)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseStoredTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
