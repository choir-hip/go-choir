package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS og_objects (
	canonical_id  TEXT PRIMARY KEY,
	object_kind   TEXT NOT NULL,
	owner_id      TEXT NOT NULL,
	computer_id   TEXT NOT NULL DEFAULT '',
	version_id    TEXT NOT NULL DEFAULT '',
	content_hash  TEXT NOT NULL,
	body          BLOB,
	metadata      TEXT NOT NULL DEFAULT '{}',
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL,
	tombstone     INTEGER NOT NULL DEFAULT 0,
	superseded_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_og_objects_kind_owner ON og_objects(object_kind, owner_id);
CREATE INDEX IF NOT EXISTS idx_og_objects_updated ON og_objects(updated_at);

CREATE TABLE IF NOT EXISTS og_edges (
	edge_id    TEXT PRIMARY KEY,
	from_id    TEXT NOT NULL,
	to_id      TEXT NOT NULL,
	kind       TEXT NOT NULL,
	metadata   TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	tombstone  INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(from_id) REFERENCES og_objects(canonical_id),
	FOREIGN KEY(to_id) REFERENCES og_objects(canonical_id)
);
CREATE INDEX IF NOT EXISTS idx_og_edges_from ON og_edges(from_id);
CREATE INDEX IF NOT EXISTS idx_og_edges_to ON og_edges(to_id);
`

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("sqlite store: create dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path+"?_busy_timeout=60000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("sqlite store: open: %w", err)
	}
	if _, err := db.Exec(sqliteSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite store: schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) PutObject(ctx context.Context, obj Object) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(canonical_id) DO UPDATE SET
			object_kind = excluded.object_kind,
			owner_id = excluded.owner_id,
			computer_id = excluded.computer_id,
			version_id = excluded.version_id,
			content_hash = excluded.content_hash,
			body = excluded.body,
			metadata = excluded.metadata,
			updated_at = excluded.updated_at,
			tombstone = excluded.tombstone,
			superseded_by = excluded.superseded_by`,
		obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
		obj.VersionID, obj.ContentHash, obj.Body, string(obj.Metadata),
		formatTime(obj.CreatedAt), formatTime(obj.UpdatedAt), boolInt(obj.Tombstone),
		obj.SupersededBy)
	if err != nil {
		return fmt.Errorf("sqlite store: put object: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetObject(ctx context.Context, id string) (Object, error) {
	return scanObject(s.db.QueryRowContext(ctx, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id = ?`, id))
}

func (s *SQLiteStore) ListObjects(ctx context.Context, filter ListFilter) ([]Object, error) {
	query := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE 1=1`
	var args []any
	if filter.Kind != "" {
		query += ` AND object_kind = ?`
		args = append(args, string(filter.Kind))
	}
	if filter.OwnerID != "" {
		query += ` AND owner_id = ?`
		args = append(args, filter.OwnerID)
	}
	if filter.Tombstone != nil {
		query += ` AND tombstone = ?`
		args = append(args, boolInt(*filter.Tombstone))
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, NormalizedLimit(filter.Limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: list objects: %w", err)
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		obj, err := scanObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite store: iterate objects: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) PutEdge(ctx context.Context, edge Edge) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO og_edges
		(edge_id, from_id, to_id, kind, metadata, created_at, tombstone)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(edge_id) DO UPDATE SET
			from_id = excluded.from_id,
			to_id = excluded.to_id,
			kind = excluded.kind,
			metadata = excluded.metadata,
			tombstone = excluded.tombstone`,
		edge.EdgeID, edge.FromID, edge.ToID, string(edge.Kind), string(edge.Metadata),
		formatTime(edge.CreatedAt), boolInt(edge.Tombstone))
	if err != nil {
		return fmt.Errorf("sqlite store: put edge: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListEdges(ctx context.Context, filter EdgeFilter) ([]Edge, error) {
	query := `SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone FROM og_edges WHERE 1=1`
	var args []any
	if filter.FromID != "" {
		query += ` AND from_id = ?`
		args = append(args, filter.FromID)
	}
	if filter.ToID != "" {
		query += ` AND to_id = ?`
		args = append(args, filter.ToID)
	}
	if filter.Kind != "" {
		query += ` AND kind = ?`
		args = append(args, string(filter.Kind))
	}
	if filter.Tombstone != nil {
		query += ` AND tombstone = ?`
		args = append(args, boolInt(*filter.Tombstone))
	}
	query += ` ORDER BY created_at LIMIT ?`
	args = append(args, NormalizedLimit(filter.Limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: list edges: %w", err)
	}
	defer rows.Close()
	var out []Edge
	for rows.Next() {
		edge, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite store: iterate edges: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanObject(row rowScanner) (Object, error) {
	var obj Object
	var created, updated string
	var metadata string
	var tombstone int
	if err := row.Scan(&obj.CanonicalID, &obj.ObjectKind, &obj.OwnerID, &obj.ComputerID, &obj.VersionID, &obj.ContentHash, &obj.Body, &metadata, &created, &updated, &tombstone, &obj.SupersededBy); err != nil {
		if err == sql.ErrNoRows {
			return Object{}, ErrNotFound
		}
		return Object{}, fmt.Errorf("sqlite store: scan object: %w", err)
	}
	obj.Metadata = json.RawMessage(metadata)
	obj.CreatedAt = parseStoredTime(created)
	obj.UpdatedAt = parseStoredTime(updated)
	obj.Tombstone = tombstone != 0
	return obj, nil
}

func scanEdge(row rowScanner) (Edge, error) {
	var edge Edge
	var created string
	var metadata string
	var tombstone int
	if err := row.Scan(&edge.EdgeID, &edge.FromID, &edge.ToID, &edge.Kind, &metadata, &created, &tombstone); err != nil {
		return Edge{}, fmt.Errorf("sqlite store: scan edge: %w", err)
	}
	edge.Metadata = json.RawMessage(metadata)
	edge.CreatedAt = parseStoredTime(created)
	edge.Tombstone = tombstone != 0
	return edge, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseStoredTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
