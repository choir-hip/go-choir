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
			max_items_per_poll         INTEGER NOT NULL DEFAULT 0,
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
			last_aux_cursor            TEXT NOT NULL DEFAULT '',
			updated_at                 TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS fetches (
			fetch_id          TEXT PRIMARY KEY,
			cycle_id          TEXT NOT NULL DEFAULT '',
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
			body_kind         TEXT NOT NULL DEFAULT '',
			body_length       INTEGER NOT NULL DEFAULT 0,
			reader_snapshot   INTEGER NOT NULL DEFAULT 0,
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
		`CREATE TABLE IF NOT EXISTS processor_requests (
			request_id       TEXT PRIMARY KEY,
			cycle_id         TEXT NOT NULL,
			processor_key    TEXT NOT NULL,
			status           TEXT NOT NULL,
			runtime_run_id   TEXT NOT NULL DEFAULT '',
			runtime_status   TEXT NOT NULL DEFAULT '',
			source_item_ids   TEXT NOT NULL DEFAULT '[]',
			source_count      INTEGER NOT NULL DEFAULT 0,
			source_types_json TEXT NOT NULL DEFAULT '[]',
			verticals_json    TEXT NOT NULL DEFAULT '[]',
			regions_json      TEXT NOT NULL DEFAULT '[]',
			continuity_ref    TEXT NOT NULL DEFAULT '',
			prompt            TEXT NOT NULL DEFAULT '',
			created_at        TEXT NOT NULL,
			updated_at        TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_processor_requests_cycle ON processor_requests(cycle_id, processor_key)`,
		`CREATE TABLE IF NOT EXISTS reconciler_requests (
			request_id                 TEXT PRIMARY KEY,
			cycle_id                   TEXT NOT NULL,
			status                     TEXT NOT NULL,
			runtime_run_id             TEXT NOT NULL DEFAULT '',
			scope                      TEXT NOT NULL,
			source_item_ids            TEXT NOT NULL DEFAULT '[]',
			processor_request_ids      TEXT NOT NULL DEFAULT '[]',
			prompt                     TEXT NOT NULL DEFAULT '',
			created_at                 TEXT NOT NULL,
			updated_at                 TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reconciler_requests_cycle ON reconciler_requests(cycle_id, scope)`,
		`CREATE TABLE IF NOT EXISTS ingestion_events (
			event_id       TEXT PRIMARY KEY,
			cycle_id       TEXT NOT NULL,
			artifact_id    TEXT NOT NULL,
			source_id      TEXT NOT NULL,
			fetch_id       TEXT NOT NULL DEFAULT '',
			content_hash   TEXT NOT NULL DEFAULT '',
			dedupe_key     TEXT NOT NULL DEFAULT '',
			origin         TEXT NOT NULL,
			created_at     TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ingestion_events_cycle ON ingestion_events(cycle_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_ingestion_events_artifact ON ingestion_events(artifact_id)`,
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
		`ALTER TABLE sources ADD COLUMN max_items_per_poll INTEGER NOT NULL DEFAULT 0`,
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
		`ALTER TABLE items ADD COLUMN body_kind TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN body_length INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE items ADD COLUMN reader_snapshot INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE items ADD COLUMN evidence_level TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN vintage_policy TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN lookahead_status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN release_date TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE items ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE issues ADD COLUMN citation_map_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE processor_requests ADD COLUMN runtime_run_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE processor_requests ADD COLUMN runtime_status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE processor_requests ADD COLUMN ingestion_event_ids_json TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE sources ADD COLUMN last_aux_cursor TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reconciler_requests ADD COLUMN runtime_run_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fetches ADD COLUMN cycle_id TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range alterStatements {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	return nil
}

type ProcessorRequest struct {
	RequestID         string
	CycleID           string
	ProcessorKey      string
	Status            string
	RuntimeRunID      string
	RuntimeStatus     string
	SourceItemIDs     []string
	IngestionEventIDs []string
	SourceCount       int
	SourceTypes       []string
	Verticals         []string
	Regions           []string
	ContinuityRef     string
	Prompt            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ReconcilerRequest struct {
	RequestID           string
	CycleID             string
	Status              string
	RuntimeRunID        string
	Scope               string
	SourceItemIDs       []string
	ProcessorRequestIDs []string
	Prompt              string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CycleEvent struct {
	EventID   string
	CycleID   string
	SourceID  string
	Kind      string
	Message   string
	Metadata  map[string]any
	CreatedAt time.Time
}

type CycleSummary struct {
	CycleID            string
	StartedAt          time.Time
	EndedAt            time.Time
	Status             string
	ItemCount          int
	FetchCount         int
	Error              string
	Events             []CycleEvent
	Fetches            []sources.FetchRecord
	ProcessorRequests  []ProcessorRequest
	ReconcilerRequests []ReconcilerRequest
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
		poll_interval_seconds, max_items_per_poll, rate_limit, conditional_request_mode, user_agent,
		tos_class, robots_policy, auth_policy, store_body_policy, retention_days,
		official, source_standing, status, last_polled, last_etag, last_modified, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		type=excluded.type, url=excluded.url, name=excluded.name,
		verticals=excluded.verticals, languages=excluded.languages,
		regions=excluded.regions, jurisdictions=excluded.jurisdictions,
		tier=excluded.tier, poll_interval_seconds=excluded.poll_interval_seconds,
		max_items_per_poll=excluded.max_items_per_poll, rate_limit=excluded.rate_limit, conditional_request_mode=excluded.conditional_request_mode,
		user_agent=excluded.user_agent, tos_class=excluded.tos_class,
		robots_policy=excluded.robots_policy, auth_policy=excluded.auth_policy,
		store_body_policy=excluded.store_body_policy, retention_days=excluded.retention_days,
		official=excluded.official, source_standing=excluded.source_standing,
		status=excluded.status,
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
			source.Tier, source.PollIntervalSeconds, source.MaxItemsPerPoll, source.RateLimit, source.ConditionalMode,
			source.EffectiveUserAgent(registry.UserAgent), source.TOSClass, source.RobotsPolicy,
			source.AuthPolicy, source.StoreBodyPolicy, source.RetentionDays, boolInt(source.Official),
			source.SourceStanding, status, formatTime(source.LastPolled), source.LastETag, source.LastModified, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ApplySourcePollState overlays durable poll cursors from SQLite onto the in-memory registry.
func (s *Storage) ApplySourcePollState(registry *sources.Registry) error {
	if registry == nil {
		return nil
	}
	rows, err := s.DB.Query(`SELECT id, last_polled, last_etag, last_modified, last_aux_cursor FROM sources`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pollState struct {
		lastPolled    time.Time
		lastETag      string
		lastModified  string
		lastAuxCursor string
	}
	byID := map[string]pollState{}
	for rows.Next() {
		var id, lastPolled, lastETag, lastModified, lastAuxCursor string
		if err := rows.Scan(&id, &lastPolled, &lastETag, &lastModified, &lastAuxCursor); err != nil {
			return err
		}
		byID[id] = pollState{
			lastPolled:    parseStoredTime(lastPolled),
			lastETag:      lastETag,
			lastModified:  lastModified,
			lastAuxCursor: lastAuxCursor,
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range registry.Sources {
		state, ok := byID[registry.Sources[i].ID]
		if !ok {
			continue
		}
		registry.Sources[i].LastPolled = state.lastPolled
		registry.Sources[i].LastETag = state.lastETag
		registry.Sources[i].LastModified = state.lastModified
		registry.Sources[i].LastAuxCursor = state.lastAuxCursor
	}
	return nil
}

// SaveSourcePollState persists per-source poll cursors after a fetch cycle.
func (s *Storage) SaveSourcePollState(registry *sources.Registry) error {
	if registry == nil {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`UPDATE sources SET last_polled = ?, last_etag = ?, last_modified = ?, last_aux_cursor = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, source := range registry.Sources {
		if strings.TrimSpace(source.ID) == "" {
			continue
		}
		if _, err := stmt.Exec(formatTime(source.LastPolled), source.LastETag, source.LastModified, source.LastAuxCursor, now, source.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) SaveFetches(fetches []sources.FetchRecord) error {
	return s.SaveCycleFetches("", fetches)
}

func (s *Storage) SaveCycleFetches(cycleID string, fetches []sources.FetchRecord) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO fetches (
		fetch_id, cycle_id, source_id, source_type, request_url, canonical_url, status_code,
		status, started_at, ended_at, response_etag, response_modified,
		content_hash, raw_snapshot_ref, error_class, error, item_count
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, fetch := range fetches {
		if strings.TrimSpace(fetch.FetchID) == "" {
			continue
		}
		if _, err := stmt.Exec(fetch.FetchID, cycleID, fetch.SourceID, fetch.SourceType, fetch.RequestURL,
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
		content_hash, body_kind, body_length, reader_snapshot, raw_json, evidence_level, vintage_policy,
		lookahead_status, release_date, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		if item.ID == "" {
			return fmt.Errorf("item id is required")
		}
		item = sources.NormalizeItemBodyClassification(item)
		createdAt := item.FetchedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		_, err := stmt.Exec(item.ID, item.SourceID, item.SourceType, item.FetchID, item.OriginalID,
			item.Title, item.Body, item.URL, item.CanonicalURL, formatTime(item.Published),
			formatTime(item.FetchedAt), mustJSON(item.Verticals), item.Language, item.Region,
			item.ContentHash, item.BodyKind, item.BodyLength, boolInt(item.ReaderSnapshot),
			item.RawJSON, item.EvidenceLevel, item.VintagePolicy,
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

func (s *Storage) ListCycleEvents(ctx context.Context, cycleID string, limit int) ([]CycleEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT event_id, cycle_id, source_id, kind, message, metadata_json, created_at
		FROM cycle_events WHERE cycle_id = ? ORDER BY created_at, event_id LIMIT ?`, cycleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CycleEvent{}
	for rows.Next() {
		var event CycleEvent
		var metadataJSON, createdAt string
		if err := rows.Scan(&event.EventID, &event.CycleID, &event.SourceID, &event.Kind, &event.Message, &metadataJSON, &createdAt); err != nil {
			return nil, err
		}
		if strings.TrimSpace(metadataJSON) != "" {
			_ = json.Unmarshal([]byte(metadataJSON), &event.Metadata)
		}
		if event.Metadata == nil {
			event.Metadata = map[string]any{}
		}
		event.CreatedAt = parseStoredTime(createdAt)
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Storage) SaveIngestionEvents(ctx context.Context, events []IngestionEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO ingestion_events (
		event_id, cycle_id, artifact_id, source_id, fetch_id, content_hash, dedupe_key, origin, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, event := range events {
		if err := ValidateIngestionEventOrigin(event.Origin); err != nil {
			return err
		}
		if strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.CycleID) == "" || strings.TrimSpace(event.ArtifactID) == "" {
			return fmt.Errorf("ingestion event id, cycle id, and artifact id are required")
		}
		createdAt := event.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx, event.EventID, event.CycleID, event.ArtifactID, event.SourceID,
			event.FetchID, event.ContentHash, event.DedupeKey, event.Origin, formatTime(createdAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) CountIngestionEvents(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM ingestion_events`).Scan(&count)
	return count, err
}

func (s *Storage) ValidateProcessorRequestIngestionEvents(ctx context.Context, req ProcessorRequest) (bool, error) {
	if !ProcessorRequestEligibleForDispatch(req) {
		return false, nil
	}
	for _, eventID := range req.IngestionEventIDs {
		eventID = strings.TrimSpace(eventID)
		if eventID == "" {
			return false, nil
		}
		var cycleID, artifactID, origin string
		err := s.DB.QueryRowContext(ctx, `SELECT cycle_id, artifact_id, origin FROM ingestion_events WHERE event_id = ?`, eventID).
			Scan(&cycleID, &artifactID, &origin)
		if err == sql.ErrNoRows {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if cycleID != strings.TrimSpace(req.CycleID) || origin != IngestionOriginSourceFetch {
			return false, nil
		}
		if !stringSliceContains(req.SourceItemIDs, artifactID) {
			return false, nil
		}
	}
	return true, nil
}

func stringSliceContains(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func (s *Storage) SaveProcessorRequests(ctx context.Context, requests []ProcessorRequest) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO processor_requests (
		request_id, cycle_id, processor_key, status, runtime_run_id, runtime_status, source_item_ids, ingestion_event_ids_json, source_count,
		source_types_json, verticals_json, regions_json, continuity_ref, prompt, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, req := range requests {
		if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.CycleID) == "" || strings.TrimSpace(req.ProcessorKey) == "" {
			return fmt.Errorf("processor request id, cycle id, and processor key are required")
		}
		status := strings.TrimSpace(req.Status)
		if status == "" {
			status = "queued"
		}
		runtimeStatus := strings.TrimSpace(req.RuntimeStatus)
		if runtimeStatus == "" {
			runtimeStatus = status
		}
		now := time.Now().UTC()
		createdAt := req.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		updatedAt := req.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}
		sourceCount := req.SourceCount
		if sourceCount == 0 {
			sourceCount = len(req.SourceItemIDs)
		}
		if _, err := stmt.ExecContext(ctx, req.RequestID, req.CycleID, req.ProcessorKey, status, strings.TrimSpace(req.RuntimeRunID), runtimeStatus,
			mustJSON(req.SourceItemIDs), mustJSON(req.IngestionEventIDs), sourceCount, mustJSON(req.SourceTypes), mustJSON(req.Verticals),
			mustJSON(req.Regions), req.ContinuityRef, req.Prompt, formatTime(createdAt), formatTime(updatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) UpdateProcessorRequestRuntimeRun(ctx context.Context, requestID, status, runtimeRunID string) error {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(status) == "" {
		return fmt.Errorf("processor request id and status are required")
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE processor_requests SET status = ?, runtime_status = ?, runtime_run_id = ?, updated_at = ? WHERE request_id = ?`,
		strings.TrimSpace(status), strings.TrimSpace(status), strings.TrimSpace(runtimeRunID), time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(requestID))
	if err != nil {
		return fmt.Errorf("update processor request status: %w", err)
	}
	return nil
}

func (s *Storage) UpdateProcessorRequestRuntimeStatus(ctx context.Context, requestID, runtimeStatus, runtimeRunID string) error {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(runtimeStatus) == "" {
		return fmt.Errorf("processor request id and runtime status are required")
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE processor_requests SET runtime_status = ?, runtime_run_id = ?, updated_at = ? WHERE request_id = ?`,
		strings.TrimSpace(runtimeStatus), strings.TrimSpace(runtimeRunID), time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(requestID))
	if err != nil {
		return fmt.Errorf("update processor request runtime status: %w", err)
	}
	return nil
}

func (s *Storage) UpdateProcessorRequestVerdictStatus(ctx context.Context, requestID, status string) error {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(status) == "" {
		return fmt.Errorf("processor request id and verdict status are required")
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE processor_requests SET status = ?, updated_at = ? WHERE request_id = ?`,
		strings.TrimSpace(status), time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(requestID))
	if err != nil {
		return fmt.Errorf("update processor request verdict status: %w", err)
	}
	return nil
}

func (s *Storage) SupersedeQueuedProcessorRequests(ctx context.Context, replacements []ProcessorRequest) (int, error) {
	now := formatTime(time.Now().UTC())
	total := 0
	for _, replacement := range replacements {
		continuityRef := strings.TrimSpace(replacement.ContinuityRef)
		if continuityRef == "" {
			continue
		}
		createdAt := replacement.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		result, err := s.DB.ExecContext(ctx, `UPDATE processor_requests
			SET status = 'superseded', updated_at = ?
			WHERE status IN ('queued', 'deferred')
			  AND continuity_ref = ?
			  AND request_id != ?
			  AND created_at < ?`,
			now, continuityRef, strings.TrimSpace(replacement.RequestID), formatTime(createdAt))
		if err != nil {
			return total, fmt.Errorf("supersede queued processor requests: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return total, fmt.Errorf("count superseded processor requests: %w", err)
		}
		total += int(affected)
	}
	return total, nil
}

func (s *Storage) SaveReconcilerRequests(ctx context.Context, requests []ReconcilerRequest) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO reconciler_requests (
		request_id, cycle_id, status, runtime_run_id, scope, source_item_ids, processor_request_ids, prompt, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, req := range requests {
		if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.CycleID) == "" || strings.TrimSpace(req.Scope) == "" {
			return fmt.Errorf("reconciler request id, cycle id, and scope are required")
		}
		status := strings.TrimSpace(req.Status)
		if status == "" {
			status = "queued"
		}
		now := time.Now().UTC()
		createdAt := req.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		updatedAt := req.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = now
		}
		if _, err := stmt.ExecContext(ctx, req.RequestID, req.CycleID, status, strings.TrimSpace(req.RuntimeRunID), req.Scope,
			mustJSON(req.SourceItemIDs), mustJSON(req.ProcessorRequestIDs), req.Prompt,
			formatTime(createdAt), formatTime(updatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) UpdateReconcilerRequestStatus(ctx context.Context, requestID, status string) error {
	return s.UpdateReconcilerRequestRuntimeRun(ctx, requestID, status, "")
}

func (s *Storage) UpdateReconcilerRequestRuntimeRun(ctx context.Context, requestID, status, runtimeRunID string) error {
	if strings.TrimSpace(requestID) == "" || strings.TrimSpace(status) == "" {
		return fmt.Errorf("reconciler request id and status are required")
	}
	if strings.TrimSpace(runtimeRunID) == "" {
		_, err := s.DB.ExecContext(ctx, `UPDATE reconciler_requests SET status = ?, updated_at = ? WHERE request_id = ?`,
			strings.TrimSpace(status), time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(requestID))
		if err != nil {
			return fmt.Errorf("update reconciler request status: %w", err)
		}
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE reconciler_requests SET status = ?, runtime_run_id = ?, updated_at = ? WHERE request_id = ?`,
		strings.TrimSpace(status), strings.TrimSpace(runtimeRunID), time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(requestID))
	if err != nil {
		return fmt.Errorf("update reconciler request status: %w", err)
	}
	return nil
}

func (s *Storage) SupersedeQueuedReconcilersWithSupersededProcessors(ctx context.Context) (int, error) {
	reconcilers, err := s.ListQueuedReconcilerRequests(ctx, 500)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, reconciler := range reconcilers {
		if len(reconciler.ProcessorRequestIDs) == 0 {
			continue
		}
		hasSuperseded, err := s.anyProcessorRequestHasStatus(ctx, reconciler.ProcessorRequestIDs, "superseded")
		if err != nil {
			return total, err
		}
		if !hasSuperseded {
			continue
		}
		if err := s.UpdateReconcilerRequestStatus(ctx, reconciler.RequestID, "superseded"); err != nil {
			return total, err
		}
		total++
	}
	return total, nil
}

const processorRequestColumns = `request_id, cycle_id, processor_key, status, runtime_run_id, runtime_status, source_item_ids,
	ingestion_event_ids_json, source_count, source_types_json, verticals_json, regions_json, continuity_ref, prompt, created_at, updated_at`

func (s *Storage) queryProcessorRequests(ctx context.Context, clause string, args ...any) ([]ProcessorRequest, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+processorRequestColumns+` FROM processor_requests`+clause, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProcessorRequest
	for rows.Next() {
		req, err := scanProcessorRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (s *Storage) ListProcessorRequests(ctx context.Context, cycleID string, limit int) ([]ProcessorRequest, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := ""
	args := []any{}
	if strings.TrimSpace(cycleID) != "" {
		where = " WHERE cycle_id = ?"
		args = append(args, strings.TrimSpace(cycleID))
	}
	args = append(args, limit)
	return s.queryProcessorRequests(ctx, where+` ORDER BY created_at DESC LIMIT ?`, args...)
}

func (s *Storage) ListQueuedProcessorRequests(ctx context.Context, limit int) ([]ProcessorRequest, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	return s.queryProcessorRequests(ctx,
		` WHERE status = 'queued' AND ingestion_event_ids_json != '[]' ORDER BY created_at ASC, request_id ASC LIMIT ?`, limit)
}

func (s *Storage) CountQueuedProcessorRequests(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM processor_requests WHERE status = 'queued' AND ingestion_event_ids_json != '[]'`).Scan(&count)
	return count, err
}

// ListReconcilableProcessorRequests returns processor requests that still need
// verdict reconciliation or are still holding runtime capacity. Request status
// and runtime status are intentionally separate axes.
func (s *Storage) ListReconcilableProcessorRequests(ctx context.Context, limit int) ([]ProcessorRequest, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.queryProcessorRequests(ctx,
		` WHERE status = 'submitted' OR runtime_status = 'submitted' ORDER BY updated_at ASC LIMIT ?`, limit)
}

// CountRecentlySubmittedProcessorRequests returns the number of processor requests
// with runtime_status 'submitted' whose updated_at is >= since. This estimates
// in-flight processor runs that have not yet completed or failed.
func (s *Storage) CountRecentlySubmittedProcessorRequests(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM processor_requests WHERE runtime_status = ? AND updated_at >= ?`,
		"submitted", since.Format(time.RFC3339)).Scan(&count)
	return count, err
}

// ResetProcessorRequestSubmission recovers a single processor request after
// its runtime run disappeared. Unresolved submitted verdicts are requeued;
// already projected verdicts keep their request status and release runtime
// capacity.
func (s *Storage) ResetProcessorRequestSubmission(ctx context.Context, requestID string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE processor_requests
		    SET status = CASE WHEN status = 'submitted' THEN 'queued' ELSE status END,
		        runtime_status = CASE
		          WHEN status = 'submitted' THEN 'queued'
		          WHEN status = 'dispatch_failed' THEN 'failed'
		          ELSE 'completed'
		        END,
		        runtime_run_id = '',
		        updated_at = ?
		  WHERE request_id = ?`,
		time.Now().UTC().Format(time.RFC3339), requestID)
	return err
}

// ResetStaleSubmittedProcessorRequests recovers runtime-capacity state after a
// platform VM restart. Unresolved submitted verdicts are requeued; already
// projected request verdicts keep their status and only release runtime_status.
func (s *Storage) ResetStaleSubmittedProcessorRequests(ctx context.Context, cutoff time.Time) (int, error) {
	result, err := s.DB.ExecContext(ctx,
		`UPDATE processor_requests
		    SET status = CASE WHEN status = 'submitted' THEN 'queued' ELSE status END,
		        runtime_status = CASE
		          WHEN status = 'submitted' THEN 'queued'
		          WHEN status = 'dispatch_failed' THEN 'failed'
		          ELSE 'completed'
		        END,
		        runtime_run_id = '',
		        updated_at = ?
		  WHERE runtime_status = 'submitted' AND updated_at < ?`,
		time.Now().UTC().Format(time.RFC3339), cutoff.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("reset stale submitted processor requests: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

func (s *Storage) ListReconcilerRequests(ctx context.Context, cycleID string, limit int) ([]ReconcilerRequest, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := ""
	args := []any{}
	if strings.TrimSpace(cycleID) != "" {
		where = " WHERE cycle_id = ?"
		args = append(args, strings.TrimSpace(cycleID))
	}
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, `SELECT request_id, cycle_id, status, runtime_run_id, scope, source_item_ids,
		processor_request_ids, prompt, created_at, updated_at
		FROM reconciler_requests`+where+` ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReconcilerRequest
	for rows.Next() {
		req, err := scanReconcilerRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (s *Storage) ListQueuedReconcilerRequests(ctx context.Context, limit int) ([]ReconcilerRequest, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT request_id, cycle_id, status, runtime_run_id, scope, source_item_ids,
		processor_request_ids, prompt, created_at, updated_at
		FROM reconciler_requests WHERE status = 'queued' ORDER BY created_at ASC, request_id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReconcilerRequest
	for rows.Next() {
		req, err := scanReconcilerRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (s *Storage) ProcessorRequestsSubmitted(ctx context.Context, requestIDs []string) (bool, error) {
	ids := make([]string, 0, len(requestIDs))
	for _, requestID := range requestIDs {
		requestID = strings.TrimSpace(requestID)
		if requestID != "" {
			ids = append(ids, requestID)
		}
	}
	if len(ids) == 0 {
		return false, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for idx, id := range ids {
		placeholders[idx] = "?"
		args[idx] = id
	}
	query := `SELECT COUNT(*), SUM(CASE WHEN runtime_status = 'submitted' AND runtime_run_id != '' THEN 1 ELSE 0 END)
		FROM processor_requests WHERE request_id IN (` + strings.Join(placeholders, ",") + `)`
	var total int
	var submitted sql.NullInt64
	if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&total, &submitted); err != nil {
		return false, err
	}
	return total == len(ids) && int(submitted.Int64) == len(ids), nil
}

func (s *Storage) anyProcessorRequestHasStatus(ctx context.Context, requestIDs []string, status string) (bool, error) {
	ids := make([]string, 0, len(requestIDs))
	for _, requestID := range requestIDs {
		requestID = strings.TrimSpace(requestID)
		if requestID != "" {
			ids = append(ids, requestID)
		}
	}
	status = strings.TrimSpace(status)
	if len(ids) == 0 || status == "" {
		return false, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, status)
	for idx, id := range ids {
		placeholders[idx] = "?"
		args = append(args, id)
	}
	query := `SELECT COUNT(*) FROM processor_requests WHERE status = ? AND request_id IN (` + strings.Join(placeholders, ",") + `)`
	var count int
	if err := s.DB.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Storage) LatestCycleSummary(ctx context.Context) (CycleSummary, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT cycle_id, started_at, ended_at, status, item_count, fetch_count, error
		FROM cycles ORDER BY started_at DESC LIMIT 1`)
	var summary CycleSummary
	var startedAt, endedAt string
	if err := row.Scan(&summary.CycleID, &startedAt, &endedAt, &summary.Status, &summary.ItemCount, &summary.FetchCount, &summary.Error); err != nil {
		return CycleSummary{}, err
	}
	summary.StartedAt = parseStoredTime(startedAt)
	summary.EndedAt = parseStoredTime(endedAt)
	processors, err := s.ListProcessorRequests(ctx, summary.CycleID, 200)
	if err != nil {
		return CycleSummary{}, err
	}
	reconcilers, err := s.ListReconcilerRequests(ctx, summary.CycleID, 200)
	if err != nil {
		return CycleSummary{}, err
	}
	fetches, err := s.ListFetchesForCycle(ctx, summary.CycleID)
	if err != nil {
		return CycleSummary{}, err
	}
	events, err := s.ListCycleEvents(ctx, summary.CycleID, 100)
	if err != nil {
		return CycleSummary{}, err
	}
	summary.Events = events
	summary.Fetches = fetches
	summary.ProcessorRequests = processors
	summary.ReconcilerRequests = reconcilers
	return summary, nil
}

func (s *Storage) ListFetchesForCycle(ctx context.Context, cycleID string) ([]sources.FetchRecord, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT fetch_id, source_id, source_type, request_url,
		canonical_url, status_code, status, started_at, ended_at, response_etag,
		response_modified, content_hash, raw_snapshot_ref, error_class, error, item_count
		FROM fetches WHERE cycle_id = ? ORDER BY source_id`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sources.FetchRecord
	for rows.Next() {
		var fetch sources.FetchRecord
		var startedAt, endedAt string
		if err := rows.Scan(&fetch.FetchID, &fetch.SourceID, &fetch.SourceType, &fetch.RequestURL,
			&fetch.CanonicalURL, &fetch.StatusCode, &fetch.Status, &startedAt, &endedAt,
			&fetch.ResponseETag, &fetch.ResponseModified, &fetch.ContentHash,
			&fetch.RawSnapshotRef, &fetch.ErrorClass, &fetch.Error, &fetch.ItemCount); err != nil {
			return nil, err
		}
		fetch.StartedAt = parseStoredTime(startedAt)
		fetch.EndedAt = parseStoredTime(endedAt)
		out = append(out, fetch)
	}
	return out, rows.Err()
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
	if itemIDs := sourceSearchItemIDs(query); len(itemIDs) > 0 {
		return s.searchItemsByID(ctx, itemIDs, limit)
	}
	terms := sourceSearchTerms(query)
	selectFields := `i.id, i.source_id, i.source_type, i.fetch_id, i.original_id, i.title, i.body, i.url,
		i.canonical_url, i.published, i.fetched_at, i.verticals, i.language, i.region, i.content_hash,
		i.body_kind, i.body_length, i.reader_snapshot, COALESCE(s.tos_class, ''), COALESCE(s.robots_policy, ''),
		COALESCE(s.auth_policy, ''), COALESCE(s.store_body_policy, ''), i.raw_json, i.evidence_level,
		i.vintage_policy, i.lookahead_status, i.release_date`
	sqlQuery := `SELECT ` + selectFields
	args := []any{}
	if len(terms) > 0 {
		scoreParts := make([]string, 0, len(terms))
		whereParts := make([]string, 0, len(terms))
		scoreArgs := make([]any, 0, len(terms)*3)
		whereArgs := make([]any, 0, len(terms)*3)
		for _, term := range terms {
			needle := "%" + term + "%"
			clause := `lower(i.title) LIKE ? OR lower(i.body) LIKE ? OR lower(i.source_id) LIKE ?`
			scoreParts = append(scoreParts, `CASE WHEN `+clause+` THEN 1 ELSE 0 END`)
			whereParts = append(whereParts, `(`+clause+`)`)
			scoreArgs = append(scoreArgs, needle, needle, needle)
			whereArgs = append(whereArgs, needle, needle, needle)
		}
		sqlQuery += `, (` + strings.Join(scoreParts, " + ") + `) AS search_score`
		sqlQuery += ` FROM items i LEFT JOIN sources s ON s.id = i.source_id`
		sqlQuery += ` WHERE ` + strings.Join(whereParts, ` OR `)
		args = append(args, scoreArgs...)
		args = append(args, whereArgs...)
	} else {
		sqlQuery += ` FROM items i LEFT JOIN sources s ON s.id = i.source_id`
	}
	if len(terms) > 0 {
		sqlQuery += ` ORDER BY search_score DESC, i.published DESC, i.fetched_at DESC LIMIT ?`
	} else {
		sqlQuery += ` ORDER BY i.published DESC, i.fetched_at DESC LIMIT ?`
	}
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
		var readerSnapshot int
		if len(terms) > 0 {
			var score int
			if err := rows.Scan(&item.ID, &item.SourceID, &item.SourceType, &item.FetchID, &item.OriginalID,
				&item.Title, &item.Body, &item.URL, &item.CanonicalURL, &published, &fetchedAt,
				&verticals, &item.Language, &item.Region, &item.ContentHash, &item.BodyKind,
				&item.BodyLength, &readerSnapshot, &item.SourceTOSClass, &item.SourceRobotsPolicy,
				&item.SourceAuthPolicy, &item.StoreBodyPolicy, &item.RawJSON, &item.EvidenceLevel,
				&item.VintagePolicy, &item.LookaheadStatus, &item.ReleaseDate, &score); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&item.ID, &item.SourceID, &item.SourceType, &item.FetchID, &item.OriginalID,
				&item.Title, &item.Body, &item.URL, &item.CanonicalURL, &published, &fetchedAt,
				&verticals, &item.Language, &item.Region, &item.ContentHash, &item.BodyKind,
				&item.BodyLength, &readerSnapshot, &item.SourceTOSClass, &item.SourceRobotsPolicy,
				&item.SourceAuthPolicy, &item.StoreBodyPolicy, &item.RawJSON, &item.EvidenceLevel,
				&item.VintagePolicy, &item.LookaheadStatus, &item.ReleaseDate); err != nil {
				return nil, err
			}
		}
		item.Published = parseStoredTime(published)
		item.FetchedAt = parseStoredTime(fetchedAt)
		item.ReaderSnapshot = readerSnapshot != 0
		_ = json.Unmarshal([]byte(verticals), &item.Verticals)
		item = sources.NormalizeItemBodyClassification(item)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Storage) searchItemsByID(ctx context.Context, itemIDs []string, limit int) ([]sources.Item, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if len(itemIDs) > limit {
		itemIDs = itemIDs[:limit]
	}
	placeholders := make([]string, 0, len(itemIDs))
	args := make([]any, 0, len(itemIDs)+1)
	for _, itemID := range itemIDs {
		placeholders = append(placeholders, "?")
		args = append(args, itemID)
	}
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, `SELECT i.id, i.source_id, i.source_type, i.fetch_id, i.original_id, i.title, i.body, i.url,
		i.canonical_url, i.published, i.fetched_at, i.verticals, i.language, i.region, i.content_hash,
		i.body_kind, i.body_length, i.reader_snapshot, COALESCE(s.tos_class, ''), COALESCE(s.robots_policy, ''),
		COALESCE(s.auth_policy, ''), COALESCE(s.store_body_policy, ''), i.raw_json, i.evidence_level,
		i.vintage_policy, i.lookahead_status, i.release_date
		FROM items i LEFT JOIN sources s ON s.id = i.source_id
		WHERE i.id IN (`+strings.Join(placeholders, ",")+`) ORDER BY i.published DESC, i.fetched_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sources.Item
	for rows.Next() {
		var item sources.Item
		var published, fetchedAt, verticals string
		var readerSnapshot int
		if err := rows.Scan(&item.ID, &item.SourceID, &item.SourceType, &item.FetchID, &item.OriginalID,
			&item.Title, &item.Body, &item.URL, &item.CanonicalURL, &published, &fetchedAt,
			&verticals, &item.Language, &item.Region, &item.ContentHash, &item.BodyKind,
			&item.BodyLength, &readerSnapshot, &item.SourceTOSClass, &item.SourceRobotsPolicy,
			&item.SourceAuthPolicy, &item.StoreBodyPolicy, &item.RawJSON, &item.EvidenceLevel,
			&item.VintagePolicy, &item.LookaheadStatus, &item.ReleaseDate); err != nil {
			return nil, err
		}
		item.Published = parseStoredTime(published)
		item.FetchedAt = parseStoredTime(fetchedAt)
		item.ReaderSnapshot = readerSnapshot != 0
		_ = json.Unmarshal([]byte(verticals), &item.Verticals)
		item = sources.NormalizeItemBodyClassification(item)
		out = append(out, item)
	}
	return out, rows.Err()
}

func sourceSearchItemIDs(query string) []string {
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	})
	seen := map[string]bool{}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if !strings.HasPrefix(field, "srcitem_") || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
		if len(out) >= 50 {
			break
		}
	}
	return out
}

func sourceSearchTerms(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	seen := map[string]bool{}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) < 3 || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func (s *Storage) GetItem(ctx context.Context, itemID string) (sources.Item, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return sources.Item{}, fmt.Errorf("item id is required")
	}
	row := s.DB.QueryRowContext(ctx, `SELECT i.id, i.source_id, i.source_type, i.fetch_id, i.original_id, i.title, i.body, i.url,
		i.canonical_url, i.published, i.fetched_at, i.verticals, i.language, i.region, i.content_hash,
		i.body_kind, i.body_length, i.reader_snapshot, COALESCE(s.tos_class, ''), COALESCE(s.robots_policy, ''),
		COALESCE(s.auth_policy, ''), COALESCE(s.store_body_policy, ''), i.raw_json, i.evidence_level,
		i.vintage_policy, i.lookahead_status, i.release_date
		FROM items i LEFT JOIN sources s ON s.id = i.source_id WHERE i.id = ?`, itemID)
	var item sources.Item
	var published, fetchedAt, verticals string
	var readerSnapshot int
	if err := row.Scan(&item.ID, &item.SourceID, &item.SourceType, &item.FetchID, &item.OriginalID,
		&item.Title, &item.Body, &item.URL, &item.CanonicalURL, &published, &fetchedAt,
		&verticals, &item.Language, &item.Region, &item.ContentHash, &item.BodyKind,
		&item.BodyLength, &readerSnapshot, &item.SourceTOSClass, &item.SourceRobotsPolicy,
		&item.SourceAuthPolicy, &item.StoreBodyPolicy, &item.RawJSON, &item.EvidenceLevel,
		&item.VintagePolicy, &item.LookaheadStatus, &item.ReleaseDate); err != nil {
		return sources.Item{}, err
	}
	item.Published = parseStoredTime(published)
	item.FetchedAt = parseStoredTime(fetchedAt)
	item.ReaderSnapshot = readerSnapshot != 0
	_ = json.Unmarshal([]byte(verticals), &item.Verticals)
	return sources.NormalizeItemBodyClassification(item), nil
}

func scanProcessorRequest(rows interface{ Scan(...any) error }) (ProcessorRequest, error) {
	var req ProcessorRequest
	var itemIDs, ingestionEventIDs, sourceTypes, verticals, regions, createdAt, updatedAt string
	if err := rows.Scan(&req.RequestID, &req.CycleID, &req.ProcessorKey, &req.Status, &req.RuntimeRunID, &req.RuntimeStatus, &itemIDs,
		&ingestionEventIDs, &req.SourceCount, &sourceTypes, &verticals, &regions, &req.ContinuityRef, &req.Prompt,
		&createdAt, &updatedAt); err != nil {
		return ProcessorRequest{}, err
	}
	_ = json.Unmarshal([]byte(itemIDs), &req.SourceItemIDs)
	_ = json.Unmarshal([]byte(ingestionEventIDs), &req.IngestionEventIDs)
	_ = json.Unmarshal([]byte(sourceTypes), &req.SourceTypes)
	_ = json.Unmarshal([]byte(verticals), &req.Verticals)
	_ = json.Unmarshal([]byte(regions), &req.Regions)
	req.CreatedAt = parseStoredTime(createdAt)
	req.UpdatedAt = parseStoredTime(updatedAt)
	return req, nil
}

func scanReconcilerRequest(rows interface{ Scan(...any) error }) (ReconcilerRequest, error) {
	var req ReconcilerRequest
	var itemIDs, processorRequestIDs, createdAt, updatedAt string
	if err := rows.Scan(&req.RequestID, &req.CycleID, &req.Status, &req.RuntimeRunID, &req.Scope, &itemIDs,
		&processorRequestIDs, &req.Prompt, &createdAt, &updatedAt); err != nil {
		return ReconcilerRequest{}, err
	}
	_ = json.Unmarshal([]byte(itemIDs), &req.SourceItemIDs)
	_ = json.Unmarshal([]byte(processorRequestIDs), &req.ProcessorRequestIDs)
	req.CreatedAt = parseStoredTime(createdAt)
	req.UpdatedAt = parseStoredTime(updatedAt)
	return req, nil
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
