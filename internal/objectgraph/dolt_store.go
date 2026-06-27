package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	embedded "github.com/dolthub/driver"
)

// DoltStore implements Store over an embedded Dolt workspace.
//
// Dolt uses the MySQL dialect, so the schema and upsert syntax differ from
// the SQLite implementation:
//   - BOOLEAN instead of INTEGER for tombstone flags.
//   - DATETIME instead of TEXT for timestamps.
//   - INSERT ... ON DUPLICATE KEY UPDATE instead of ON CONFLICT ... DO UPDATE.
//   - LONGTEXT for metadata JSON.
type DoltStore struct {
	db *sql.DB
}

const doltSchema = `
CREATE TABLE IF NOT EXISTS og_objects (
	canonical_id  VARCHAR(255) NOT NULL PRIMARY KEY,
	object_kind   VARCHAR(128) NOT NULL,
	owner_id      VARCHAR(255) NOT NULL,
	computer_id   VARCHAR(255) NOT NULL DEFAULT '',
	version_id    VARCHAR(255) NOT NULL DEFAULT '',
	content_hash  VARCHAR(128) NOT NULL,
	body          LONGBLOB,
	metadata      LONGTEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone     BOOLEAN NOT NULL DEFAULT FALSE,
	superseded_by VARCHAR(255) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_og_objects_kind_owner ON og_objects(object_kind, owner_id);
CREATE INDEX IF NOT EXISTS idx_og_objects_updated ON og_objects(updated_at);

CREATE TABLE IF NOT EXISTS og_edges (
	edge_id    VARCHAR(255) NOT NULL PRIMARY KEY,
	from_id    VARCHAR(255) NOT NULL,
	to_id      VARCHAR(255) NOT NULL,
	kind       VARCHAR(128) NOT NULL,
	metadata   LONGTEXT NOT NULL,
	created_at DATETIME NOT NULL,
	tombstone  BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_og_edges_from ON og_edges(from_id);
CREATE INDEX IF NOT EXISTS idx_og_edges_to ON og_edges(to_id);
`

// NewDoltStore accepts an already-open Dolt *sql.DB and applies the object
// graph schema if it does not yet exist.
func NewDoltStore(db *sql.DB) (*DoltStore, error) {
	if db == nil {
		return nil, fmt.Errorf("dolt store: db is nil")
	}
	if _, err := db.Exec(doltSchema); err != nil {
		return nil, fmt.Errorf("dolt store: schema: %w", err)
	}
	return &DoltStore{db: db}, nil
}

// OpenDoltStore creates a new embedded Dolt workspace in workspacePath, creates
// a database named dbName, and returns a DoltStore backed by it. This mirrors
// the pattern used by internal/store for texture workspaces.
func OpenDoltStore(workspacePath, dbName string) (*DoltStore, error) {
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return nil, fmt.Errorf("dolt store: create workspace dir: %w", err)
	}
	rootDSN := fmt.Sprintf(
		"file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true",
		workspacePath,
	)
	cfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		return nil, fmt.Errorf("dolt store: parse root dsn: %w", err)
	}
	rootConnector, err := embedded.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("dolt store: new root connector: %w", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	rootDB.SetMaxOpenConns(1)
	rootDB.SetMaxIdleConns(1)

	createDB := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName)
	if _, err := rootDB.Exec(createDB); err != nil {
		_ = rootDB.Close()
		_ = rootConnector.Close()
		return nil, fmt.Errorf("dolt store: create database: %w", err)
	}
	if err := rootDB.Close(); err != nil {
		_ = rootConnector.Close()
		return nil, fmt.Errorf("dolt store: close root: %w", err)
	}
	if err := rootConnector.Close(); err != nil {
		return nil, fmt.Errorf("dolt store: close root connector: %w", err)
	}

	dbDSN := fmt.Sprintf(
		"file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true",
		workspacePath,
		dbName,
	)
	dbCfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		return nil, fmt.Errorf("dolt store: parse db dsn: %w", err)
	}
	dbConnector, err := embedded.NewConnector(dbCfg)
	if err != nil {
		return nil, fmt.Errorf("dolt store: new db connector: %w", err)
	}
	db := sql.OpenDB(dbConnector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		_ = dbConnector.Close()
		return nil, fmt.Errorf("dolt store: ping: %w", err)
	}

	store, err := NewDoltStore(db)
	if err != nil {
		_ = db.Close()
		_ = dbConnector.Close()
		return nil, err
	}
	return store, nil
}

func (s *DoltStore) PutObject(ctx context.Context, obj Object) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			object_kind = VALUES(object_kind),
			owner_id = VALUES(owner_id),
			computer_id = VALUES(computer_id),
			version_id = VALUES(version_id),
			content_hash = VALUES(content_hash),
			body = VALUES(body),
			metadata = VALUES(metadata),
			updated_at = VALUES(updated_at),
			tombstone = VALUES(tombstone),
			superseded_by = VALUES(superseded_by)`,
		obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
		obj.VersionID, obj.ContentHash, obj.Body, string(obj.Metadata),
		obj.CreatedAt.UTC(), obj.UpdatedAt.UTC(), obj.Tombstone,
		obj.SupersededBy)
	if err != nil {
		return fmt.Errorf("dolt store: put object: %w", err)
	}
	return nil
}

func (s *DoltStore) GetObject(ctx context.Context, id string) (Object, error) {
	return scanDoltObject(s.db.QueryRowContext(ctx, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id = ?`, id))
}

func (s *DoltStore) ListObjects(ctx context.Context, filter ListFilter) ([]Object, error) {
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
		args = append(args, *filter.Tombstone)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, normalizedLimit(filter.Limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("dolt store: list objects: %w", err)
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		obj, err := scanDoltObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dolt store: iterate objects: %w", err)
	}
	return out, nil
}

func (s *DoltStore) PutEdge(ctx context.Context, edge Edge) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO og_edges
		(edge_id, from_id, to_id, kind, metadata, created_at, tombstone)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			from_id = VALUES(from_id),
			to_id = VALUES(to_id),
			kind = VALUES(kind),
			metadata = VALUES(metadata),
			tombstone = VALUES(tombstone)`,
		edge.EdgeID, edge.FromID, edge.ToID, string(edge.Kind), string(edge.Metadata),
		edge.CreatedAt.UTC(), edge.Tombstone)
	if err != nil {
		return fmt.Errorf("dolt store: put edge: %w", err)
	}
	return nil
}

func (s *DoltStore) ListEdges(ctx context.Context, filter EdgeFilter) ([]Edge, error) {
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
		args = append(args, *filter.Tombstone)
	}
	query += ` ORDER BY created_at LIMIT ?`
	args = append(args, normalizedLimit(filter.Limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("dolt store: list edges: %w", err)
	}
	defer rows.Close()
	var out []Edge
	for rows.Next() {
		edge, err := scanDoltEdge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dolt store: iterate edges: %w", err)
	}
	return out, nil
}

func (s *DoltStore) Close() error {
	return s.db.Close()
}

// scanDoltObject scans a row using Dolt-native types (BOOLEAN, DATETIME).
func scanDoltObject(row rowScanner) (Object, error) {
	var obj Object
	var metadata string
	if err := row.Scan(
		&obj.CanonicalID, &obj.ObjectKind, &obj.OwnerID, &obj.ComputerID,
		&obj.VersionID, &obj.ContentHash, &obj.Body, &metadata,
		&obj.CreatedAt, &obj.UpdatedAt, &obj.Tombstone, &obj.SupersededBy,
	); err != nil {
		if err == sql.ErrNoRows {
			return Object{}, ErrNotFound
		}
		return Object{}, fmt.Errorf("dolt store: scan object: %w", err)
	}
	obj.Metadata = json.RawMessage(metadata)
	obj.CreatedAt = obj.CreatedAt.UTC()
	obj.UpdatedAt = obj.UpdatedAt.UTC()
	return obj, nil
}

func scanDoltEdge(row rowScanner) (Edge, error) {
	var edge Edge
	var metadata string
	if err := row.Scan(
		&edge.EdgeID, &edge.FromID, &edge.ToID, &edge.Kind, &metadata,
		&edge.CreatedAt, &edge.Tombstone,
	); err != nil {
		if err == sql.ErrNoRows {
			return Edge{}, ErrNotFound
		}
		return Edge{}, fmt.Errorf("dolt store: scan edge: %w", err)
	}
	edge.Metadata = json.RawMessage(metadata)
	edge.CreatedAt = edge.CreatedAt.UTC()
	return edge, nil
}

// doltFormatTime ensures a non-zero timestamp for Dolt DATETIME columns.
func doltFormatTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}
