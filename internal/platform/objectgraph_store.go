package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ObjectGraphStore adapts the platform Store to the objectgraph.Store
// interface. It uses the same og_objects and og_edges tables that were
// added to the platform schema DDL, but executes against the platform
// Dolt SQL server connection rather than an embedded Dolt workspace.
type ObjectGraphStore struct {
	store *Store
}

// NewObjectGraphStore returns an objectgraph.Store backed by the platform
// Dolt SQL server. The platform store must already be bootstrapped (the
// og_objects and og_edges tables are created by Bootstrap).
func NewObjectGraphStore(s *Store) *ObjectGraphStore {
	return &ObjectGraphStore{store: s}
}

// ogBodyExternalizeMinBytes is the size threshold above which og_objects
// bodies move out of the versioned chunk store into the platform-artifacts
// filesystem CAS (Move 3, docs/designs/
// platform-dolt-storage-normalization-2026-09-18.md). 1 KiB keeps ~55% of
// rows inline (cheap list paths) while moving ~73% of body bytes off the
// chunk engine.
const ogBodyExternalizeMinBytes = 1024

// externalizeBody writes a large body to the CAS and returns the storage
// ref; small bodies or a missing CAS root return "" (stay inline).
func (o *ObjectGraphStore) externalizeBody(body []byte) (string, error) {
	if len(body) < ogBodyExternalizeMinBytes || o.store.artifactsRoot == "" {
		return "", nil
	}
	sum := sha256.Sum256(body)
	ref := filepath.Join("sha256", "og", hex.EncodeToString(sum[:])+".bin")
	path := filepath.Join(o.store.artifactsRoot, ref)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("platform objectgraph: cas dir: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return ref, nil // content-addressed: already present
	}
	tmp := path + ".tmp-" + shortID(id("ogbody"))
	if err := os.WriteFile(tmp, body, 0o640); err != nil {
		return "", fmt.Errorf("platform objectgraph: cas write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("platform objectgraph: cas install: %w", err)
	}
	return ref, nil
}

// hydrateBody loads a CAS-externalized body back into the object. Inline
// bodies (empty body_ref) are untouched.
func (o *ObjectGraphStore) hydrateBody(obj *objectgraph.Object, bodyRef string) error {
	if bodyRef == "" {
		return nil
	}
	if o.store.artifactsRoot == "" {
		return fmt.Errorf("platform objectgraph: body ref %q present but artifacts root not configured", bodyRef)
	}
	cleaned := filepath.Clean(strings.TrimLeft(bodyRef, string(filepath.Separator)))
	path := filepath.Join(o.store.artifactsRoot, cleaned)
	rel, err := filepath.Rel(o.store.artifactsRoot, path)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("platform objectgraph: invalid body ref %q", bodyRef)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("platform objectgraph: read body ref %q: %w", bodyRef, err)
	}
	obj.Body = data
	return nil
}

func (o *ObjectGraphStore) PutObject(ctx context.Context, obj objectgraph.Object) error {
	if o == nil || o.store == nil || o.store.db == nil {
		return fmt.Errorf("platform objectgraph: nil store")
	}
	bodyRef, err := o.externalizeBody(obj.Body)
	if err != nil {
		return err
	}
	inlineBody := obj.Body
	if bodyRef != "" {
		inlineBody = nil
	}
	_, err = o.store.corpus().ExecContext(ctx, `INSERT INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			object_kind = VALUES(object_kind),
			owner_id = VALUES(owner_id),
			computer_id = VALUES(computer_id),
			version_id = VALUES(version_id),
			content_hash = VALUES(content_hash),
			body = VALUES(body),
			body_ref = VALUES(body_ref),
			body_size = VALUES(body_size),
			metadata = VALUES(metadata),
			updated_at = VALUES(updated_at),
			tombstone = VALUES(tombstone),
			superseded_by = VALUES(superseded_by)`,
		obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
		obj.VersionID, obj.ContentHash, inlineBody, bodyRef, len(obj.Body), string(obj.Metadata),
		obj.CreatedAt.UTC(), obj.UpdatedAt.UTC(), obj.Tombstone,
		obj.SupersededBy)
	if err != nil {
		return fmt.Errorf("platform objectgraph: put object: %w", err)
	}
	o.store.markCorpusDirty("objectgraph put object " + obj.CanonicalID)
	return nil
}

func (o *ObjectGraphStore) GetObject(ctx context.Context, id string) (objectgraph.Object, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return objectgraph.Object{}, fmt.Errorf("platform objectgraph: nil store")
	}
	obj, bodyRef, err := scanObjectGraphObject(o.store.corpus().QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id = ?`, id))
	if err != nil {
		return objectgraph.Object{}, err
	}
	if err := o.hydrateBody(&obj, bodyRef); err != nil {
		return objectgraph.Object{}, err
	}
	return obj, nil
}

func (o *ObjectGraphStore) DeleteObject(ctx context.Context, id string) error {
	if o == nil || o.store == nil || o.store.db == nil {
		return fmt.Errorf("platform objectgraph: nil store")
	}
	_, err := o.store.corpus().ExecContext(ctx, `DELETE FROM og_objects WHERE canonical_id = ?`, id)
	if err != nil {
		return fmt.Errorf("platform objectgraph: delete object: %w", err)
	}
	return nil
}

func (o *ObjectGraphStore) ListObjects(ctx context.Context, filter objectgraph.ListFilter) ([]objectgraph.Object, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	query := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE 1=1`
	var args []any
	if filter.Kind != "" {
		query += ` AND object_kind = ?`
		args = append(args, string(filter.Kind))
	}
	if filter.OwnerID != "" {
		query += ` AND owner_id = ?`
		args = append(args, filter.OwnerID)
	}
	if filter.ComputerID != "" {
		query += ` AND computer_id = ?`
		args = append(args, filter.ComputerID)
	}
	if filter.Tombstone != nil {
		query += ` AND tombstone = ?`
		args = append(args, *filter.Tombstone)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, objectgraph.NormalizedLimit(filter.Limit))
	rows, err := o.store.corpus().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list objects: %w", err)
	}
	defer rows.Close()
	var out []objectgraph.Object
	for rows.Next() {
		obj, bodyRef, err := scanObjectGraphObject(rows)
		if err != nil {
			return nil, err
		}
		if err := o.hydrateBody(&obj, bodyRef); err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform objectgraph: iterate objects: %w", err)
	}
	return out, nil
}

func (o *ObjectGraphStore) PutEdge(ctx context.Context, edge objectgraph.Edge) error {
	if o == nil || o.store == nil || o.store.db == nil {
		return fmt.Errorf("platform objectgraph: nil store")
	}
	_, err := o.store.corpus().ExecContext(ctx, `INSERT INTO og_edges
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
		return fmt.Errorf("platform objectgraph: put edge: %w", err)
	}
	o.store.markCorpusDirty("objectgraph put edge " + edge.EdgeID)
	return nil
}

func (o *ObjectGraphStore) ListEdges(ctx context.Context, filter objectgraph.EdgeFilter) ([]objectgraph.Edge, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
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
	args = append(args, objectgraph.NormalizedLimit(filter.Limit))
	rows, err := o.store.corpus().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list edges: %w", err)
	}
	defer rows.Close()
	var out []objectgraph.Edge
	for rows.Next() {
		edge, err := scanObjectGraphEdge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform objectgraph: iterate edges: %w", err)
	}
	return out, nil
}

func (o *ObjectGraphStore) Close() error {
	// The platform store owns the DB connection; do not close it here.
	return nil
}

// GetObjectByMetadata finds a single object by kind + a metadata JSON path
// equality check. Uses JSON_EXTRACT (supported by Dolt/MySQL).
// Example: GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", "/pub/texture/foo")
func (o *ObjectGraphStore) GetObjectByMetadata(ctx context.Context, kind, jsonPath, value string) (objectgraph.Object, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return objectgraph.Object{}, fmt.Errorf("platform objectgraph: nil store")
	}
	obj, bodyRef, err := scanObjectGraphObject(o.store.corpus().QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?
		 LIMIT 1`, kind, jsonPath, value))
	if err != nil {
		return objectgraph.Object{}, err
	}
	if err := o.hydrateBody(&obj, bodyRef); err != nil {
		return objectgraph.Object{}, err
	}
	return obj, nil
}

// ListObjectsByMetadata finds objects by kind + a metadata JSON path
// equality check.
func (o *ObjectGraphStore) ListObjectsByMetadata(ctx context.Context, kind, jsonPath, value string, limit int) ([]objectgraph.Object, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	rows, err := o.store.corpus().QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?
		 ORDER BY updated_at DESC LIMIT ?`,
		kind, jsonPath, value, objectgraph.NormalizedLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list by metadata: %w", err)
	}
	defer rows.Close()
	var out []objectgraph.Object
	for rows.Next() {
		obj, bodyRef, err := scanObjectGraphObject(rows)
		if err != nil {
			return nil, err
		}
		if err := o.hydrateBody(&obj, bodyRef); err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

// GetEdge finds a single edge by from_id + kind.
func (o *ObjectGraphStore) GetEdge(ctx context.Context, fromID string, kind objectgraph.EdgeKind) (objectgraph.Edge, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return objectgraph.Edge{}, fmt.Errorf("platform objectgraph: nil store")
	}
	return scanObjectGraphEdge(o.store.corpus().QueryRowContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = FALSE
		 LIMIT 1`, fromID, string(kind)))
}

// ListEdgesFrom lists all non-tombstoned edges from a given object ID.
func (o *ObjectGraphStore) ListEdgesFrom(ctx context.Context, fromID string) ([]objectgraph.Edge, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	rows, err := o.store.corpus().QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND tombstone = FALSE ORDER BY created_at`, fromID)
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list edges from: %w", err)
	}
	defer rows.Close()
	var out []objectgraph.Edge
	for rows.Next() {
		edge, err := scanObjectGraphEdge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, edge)
	}
	return out, rows.Err()
}

// ListEdgesByKind lists all non-tombstoned edges of a given kind from a given object.
func (o *ObjectGraphStore) ListEdgesByKind(ctx context.Context, fromID string, kind objectgraph.EdgeKind) ([]objectgraph.Edge, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	rows, err := o.store.corpus().QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = FALSE ORDER BY created_at`,
		fromID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list edges by kind: %w", err)
	}
	defer rows.Close()
	var out []objectgraph.Edge
	for rows.Next() {
		edge, err := scanObjectGraphEdge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, edge)
	}
	return out, rows.Err()
}

// PutBatch writes a batch of objects and edges atomically in a single
// transaction with one Dolt commit at the end.
func (o *ObjectGraphStore) PutBatch(ctx context.Context, batch objectgraph.Batch) error {
	if o == nil || o.store == nil || o.store.db == nil {
		return fmt.Errorf("platform objectgraph: nil store")
	}
	tx, err := o.store.corpus().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("platform objectgraph: begin batch tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, obj := range batch.Objects {
		bodyRef, err := o.externalizeBody(obj.Body)
		if err != nil {
			return fmt.Errorf("platform objectgraph: batch externalize %s: %w", obj.CanonicalID, err)
		}
		inlineBody := obj.Body
		if bodyRef != "" {
			inlineBody = nil
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO og_objects
			(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				object_kind = VALUES(object_kind),
				owner_id = VALUES(owner_id),
				computer_id = VALUES(computer_id),
				version_id = VALUES(version_id),
				content_hash = VALUES(content_hash),
				body = VALUES(body),
				body_ref = VALUES(body_ref),
				body_size = VALUES(body_size),
				metadata = VALUES(metadata),
				updated_at = VALUES(updated_at),
				tombstone = VALUES(tombstone),
				superseded_by = VALUES(superseded_by)`,
			obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
			obj.VersionID, obj.ContentHash, inlineBody, bodyRef, len(obj.Body), string(obj.Metadata),
			obj.CreatedAt.UTC(), obj.UpdatedAt.UTC(), obj.Tombstone,
			obj.SupersededBy); err != nil {
			return fmt.Errorf("platform objectgraph: batch put object %s: %w", obj.CanonicalID, err)
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
			return fmt.Errorf("platform objectgraph: batch put edge %s: %w", edge.EdgeID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("platform objectgraph: batch commit: %w", err)
	}
	o.store.markCorpusDirty(fmt.Sprintf("objectgraph batch: %d objects, %d edges", len(batch.Objects), len(batch.Edges)))
	return nil
}

// Compile-time assertion that ObjectGraphStore satisfies objectgraph interfaces.
var (
	_ objectgraph.Store      = (*ObjectGraphStore)(nil)
	_ objectgraph.BatchStore = (*ObjectGraphStore)(nil)
)

type ogRowScanner interface {
	Scan(dest ...any) error
}

func scanObjectGraphObject(row ogRowScanner) (objectgraph.Object, string, error) {
	var obj objectgraph.Object
	var metadata, bodyRef string
	var bodySize int64
	if err := row.Scan(
		&obj.CanonicalID, &obj.ObjectKind, &obj.OwnerID, &obj.ComputerID,
		&obj.VersionID, &obj.ContentHash, &obj.Body, &bodyRef, &bodySize, &metadata,
		&obj.CreatedAt, &obj.UpdatedAt, &obj.Tombstone, &obj.SupersededBy,
	); err != nil {
		if err == sql.ErrNoRows {
			return objectgraph.Object{}, "", objectgraph.ErrNotFound
		}
		return objectgraph.Object{}, "", fmt.Errorf("platform objectgraph: scan object: %w", err)
	}
	obj.Metadata = json.RawMessage(metadata)
	obj.CreatedAt = obj.CreatedAt.UTC()
	obj.UpdatedAt = obj.UpdatedAt.UTC()
	return obj, bodyRef, nil
}

func scanObjectGraphEdge(row ogRowScanner) (objectgraph.Edge, error) {
	var edge objectgraph.Edge
	var metadata string
	if err := row.Scan(
		&edge.EdgeID, &edge.FromID, &edge.ToID, &edge.Kind, &metadata,
		&edge.CreatedAt, &edge.Tombstone,
	); err != nil {
		if err == sql.ErrNoRows {
			return objectgraph.Edge{}, objectgraph.ErrNotFound
		}
		return objectgraph.Edge{}, fmt.Errorf("platform objectgraph: scan edge: %w", err)
	}
	edge.Metadata = json.RawMessage(metadata)
	edge.CreatedAt = edge.CreatedAt.UTC()
	return edge, nil
}
