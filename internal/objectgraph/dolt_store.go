package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// SchemaDDL creates the og_objects and og_edges tables if they do not
// already exist. The schema is identical to the one used by corpusd's
// platform store, so the same queries work in both environments.
const SchemaDDL = `
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

// DoltStore is a Store backed by an embedded Dolt *sql.DB. It is the
// VM-local equivalent of the platform ObjectGraphStore: the same
// og_objects and og_edges tables, but in the VM's own Dolt workspace
// rather than the corpusd SQL server.
//
// Unlike the platform store, DoltStore does not call dolt commit after
// each write. The VM's embedded Dolt auto-commits on connection close
// or explicit dolt commit calls from the store layer. Callers that
// need transactional batch writes should use PutBatch.
type DoltStore struct {
	db *sql.DB
}

// JSONFieldMatch is one JSON field predicate for a Dolt object lookup.
// JSONPath is passed as a query argument (for example, "$.run_id"), never
// interpolated into SQL. MissingMatchesEmpty lets legacy JSON bodies whose
// omitempty field is absent match the current empty-string representation.
type JSONFieldMatch struct {
	JSONPath            string
	Value               string
	MissingMatchesEmpty bool
}

// NewDoltStore returns a DoltStore backed by the given *sql.DB. The
// caller must call EnsureSchema before using the store.
func NewDoltStore(db *sql.DB) *DoltStore {
	return &DoltStore{db: db}
}

// EnsureSchema creates the og_objects and og_edges tables if they do
// not already exist. Safe to call multiple times.
func (s *DoltStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	_, err := s.db.ExecContext(ctx, SchemaDDL)
	if err != nil {
		return fmt.Errorf("objectgraph dolt: ensure schema: %w", err)
	}
	return nil
}

func (s *DoltStore) PutObject(ctx context.Context, obj Object) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
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
		return fmt.Errorf("objectgraph dolt: put object: %w", err)
	}
	return nil
}

func (s *DoltStore) GetObject(ctx context.Context, id string) (Object, error) {
	if s == nil || s.db == nil {
		return Object{}, fmt.Errorf("objectgraph dolt: nil store")
	}
	return scanDoltObject(s.db.QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id = ?`, id))
}

func (s *DoltStore) ListObjects(ctx context.Context, filter ListFilter) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
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
	args = append(args, NormalizedLimit(filter.Limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list objects: %w", err)
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
		return nil, fmt.Errorf("objectgraph dolt: iterate objects: %w", err)
	}
	return out, nil
}

func (s *DoltStore) PutEdge(ctx context.Context, edge Edge) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
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
		return fmt.Errorf("objectgraph dolt: put edge: %w", err)
	}
	return nil
}

func (s *DoltStore) ListEdges(ctx context.Context, filter EdgeFilter) ([]Edge, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
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
	args = append(args, NormalizedLimit(filter.Limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list edges: %w", err)
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
		return nil, fmt.Errorf("objectgraph dolt: iterate edges: %w", err)
	}
	return out, nil
}

// PutBatch writes a batch of objects and edges atomically in a single
// transaction.
func (s *DoltStore) PutBatch(ctx context.Context, batch Batch) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("objectgraph dolt: begin batch tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, obj := range batch.Objects {
		if _, err := tx.ExecContext(ctx, `INSERT INTO og_objects
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
			obj.SupersededBy); err != nil {
			return fmt.Errorf("objectgraph dolt: batch put object %s: %w", obj.CanonicalID, err)
		}
	}

	for _, edge := range batch.Edges {
		if _, err := tx.ExecContext(ctx, `INSERT INTO og_edges
			(edge_id, from_id, to_id, kind, metadata, created_at, tombstone)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				from_id = VALUES(from_id),
				to_id = VALUES(to_id),
				kind = VALUES(kind),
				metadata = VALUES(metadata),
				tombstone = VALUES(tombstone)`,
			edge.EdgeID, edge.FromID, edge.ToID, string(edge.Kind), string(edge.Metadata),
			edge.CreatedAt.UTC(), edge.Tombstone); err != nil {
			return fmt.Errorf("objectgraph dolt: batch put edge %s: %w", edge.EdgeID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("objectgraph dolt: batch commit: %w", err)
	}
	return nil
}

// GetObjectByMetadata finds a single object by kind + a metadata JSON
// path equality check using JSON_EXTRACT.
func (s *DoltStore) GetObjectByMetadata(ctx context.Context, kind, jsonPath, value string) (Object, error) {
	if s == nil || s.db == nil {
		return Object{}, fmt.Errorf("objectgraph dolt: nil store")
	}
	return scanDoltObject(s.db.QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?
		 LIMIT 1`, kind, jsonPath, value))
}

// ListObjectsByMetadata finds objects by kind + a metadata JSON path
// equality check.
func (s *DoltStore) ListObjectsByMetadata(ctx context.Context, kind, jsonPath, value string, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?
		 ORDER BY updated_at DESC LIMIT ?`,
		kind, jsonPath, value, NormalizedLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list by metadata: %w", err)
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
	return out, rows.Err()
}

// ListObjectsByOwnerAndBody finds objects by kind, owner, and an exact set of
// predicates evaluated against the persisted JSON body. The body is the
// canonical authority for record fields that must not be duplicated into
// independently drifting object metadata.
func (s *DoltStore) ListObjectsByOwnerAndBody(ctx context.Context, kind, ownerID string, matches []JSONFieldMatch, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	query := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND owner_id = ?`
	args := []any{kind, ownerID}
	for _, match := range matches {
		if match.JSONPath == "" {
			return nil, fmt.Errorf("objectgraph dolt: body JSON path is required")
		}
		if match.MissingMatchesEmpty {
			if match.Value != "" {
				return nil, fmt.Errorf("objectgraph dolt: missing body field can only match an empty value")
			}
			query += ` AND (JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), ?) IS NULL OR JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), ?)) = ?)`
			args = append(args, match.JSONPath, match.JSONPath, match.Value)
			continue
		}
		query += ` AND JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), ?)) = ?`
		args = append(args, match.JSONPath, match.Value)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, NormalizedLimit(limit))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list by owner and body: %w", err)
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
	return out, rows.Err()
}

// GetEdge finds a single non-tombstoned edge by from_id + kind.
func (s *DoltStore) GetEdge(ctx context.Context, fromID string, kind EdgeKind) (Edge, error) {
	if s == nil || s.db == nil {
		return Edge{}, fmt.Errorf("objectgraph dolt: nil store")
	}
	return scanDoltEdge(s.db.QueryRowContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = FALSE
		 LIMIT 1`, fromID, string(kind)))
}

// ListEdgesFrom lists all non-tombstoned edges from a given object ID.
func (s *DoltStore) ListEdgesFrom(ctx context.Context, fromID string) ([]Edge, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND tombstone = FALSE ORDER BY created_at`, fromID)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list edges from: %w", err)
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
	return out, rows.Err()
}

// ListEdgesByKind lists all non-tombstoned edges of a given kind from
// a given object.
func (s *DoltStore) ListEdgesByKind(ctx context.Context, fromID string, kind EdgeKind) ([]Edge, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = FALSE ORDER BY created_at`,
		fromID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list edges by kind: %w", err)
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
	return out, rows.Err()
}

func (s *DoltStore) DeleteObject(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM og_objects WHERE canonical_id = ?`, id)
	if err != nil {
		return fmt.Errorf("objectgraph dolt: delete object: %w", err)
	}
	return nil
}

func (s *DoltStore) Close() error {
	// The caller owns the *sql.DB; do not close it here.
	return nil
}

// Compile-time assertions.
var (
	_ Store      = (*DoltStore)(nil)
	_ BatchStore = (*DoltStore)(nil)
)

type doltRowScanner interface {
	Scan(dest ...any) error
}

func scanDoltObject(row doltRowScanner) (Object, error) {
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
		return Object{}, fmt.Errorf("objectgraph dolt: scan object: %w", err)
	}
	obj.Metadata = json.RawMessage(metadata)
	obj.CreatedAt = obj.CreatedAt.UTC()
	obj.UpdatedAt = obj.UpdatedAt.UTC()
	return obj, nil
}

func scanDoltEdge(row doltRowScanner) (Edge, error) {
	var edge Edge
	var metadata string
	if err := row.Scan(
		&edge.EdgeID, &edge.FromID, &edge.ToID, &edge.Kind, &metadata,
		&edge.CreatedAt, &edge.Tombstone,
	); err != nil {
		if err == sql.ErrNoRows {
			return Edge{}, ErrNotFound
		}
		return Edge{}, fmt.Errorf("objectgraph dolt: scan edge: %w", err)
	}
	edge.Metadata = json.RawMessage(metadata)
	edge.CreatedAt = edge.CreatedAt.UTC()
	return edge, nil
}
