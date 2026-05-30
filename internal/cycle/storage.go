package cycle

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

type Storage struct {
	DB *sql.DB
}

func NewStorage(dbPath string) (*Storage, error) {
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
			id          TEXT PRIMARY KEY,
			type        TEXT,
			url         TEXT,
			name        TEXT,
			verticals   TEXT,
			poll_interval_secs INTEGER,
			last_polled TEXT,
			last_etag   TEXT,
			last_modified TEXT,
			status      TEXT DEFAULT 'active'
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id          TEXT PRIMARY KEY,
			source_id   TEXT,
			original_id TEXT,
			title       TEXT,
			body        TEXT,
			url         TEXT,
			published   TEXT,
			fetched_at  TEXT,
			verticals   TEXT,
			raw_json    TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS issues (
			id          TEXT PRIMARY KEY,
			timestamp   TEXT,
			content     TEXT,
			item_ids    TEXT,
			model       TEXT,
			tokens      INTEGER
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) SaveItems(items []sources.Item) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO items (id, source_id, original_id, title, body, url, published, fetched_at, verticals, raw_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		verticalsJSON, _ := json.Marshal(item.Verticals)
		_, err := stmt.Exec(item.ID, item.SourceID, item.OriginalID, item.Title, item.Body, item.URL, item.Published.Format(time.RFC3339), item.FetchedAt.Format(time.RFC3339), string(verticalsJSON), item.RawJSON)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Storage) SaveIssue(content string, itemIDs []string, model string, tokens int) error {
	id := fmt.Sprintf("issue-%d", time.Now().Unix())
	itemIDsJSON, _ := json.Marshal(itemIDs)
	_, err := s.DB.Exec(`INSERT INTO issues (id, timestamp, content, item_ids, model, tokens) VALUES (?, ?, ?, ?, ?, ?)`,
		id, time.Now().Format(time.RFC3339), content, string(itemIDsJSON), model, tokens)
	return err
}
