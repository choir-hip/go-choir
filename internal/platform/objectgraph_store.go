package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// Reuse and creation hold the CAS reuse lock shared: the artifact GC
	// re-checks and unlinks under it exclusively, so a reused body is
	// refreshed (mtime) before the GC can collect it as unreferenced.
	o.store.casReuseMu.RLock()
	defer o.store.casReuseMu.RUnlock()
	if _, err := os.Stat(path); err == nil {
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			return "", fmt.Errorf("platform objectgraph: refresh reused body: %w", err)
		}
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
	if err := objectgraph.CheckWriteAs(corpusdWriter(ctx), []objectgraph.Object{obj}, nil); err != nil {
		return err
	}
	bodyRef, err := o.externalizeBody(obj.Body)
	if err != nil {
		return err
	}
	inlineBody := obj.Body
	if bodyRef != "" {
		inlineBody = nil
	}
	_, err = o.execObjectWrite(ctx, obj, func(x sqlExecer) (bool, error) {
		_, err := x.ExecContext(ctx, `INSERT INTO og_objects
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
		return err == nil, err
	})
	if err != nil {
		return fmt.Errorf("platform objectgraph: put object: %w", err)
	}
	o.store.markCorpusDirty("objectgraph put object " + obj.CanonicalID)
	o.store.indexCaptures(ctx, false, obj)
	return nil
}

// InsertObjectIfAbsent writes the object only if no row has its canonical
// id. INSERT IGNORE leaves an existing row untouched, so two writers racing
// on one id cannot replace each other's object; a concurrent insert of the
// same key fails at commit instead of overwriting.
func (o *ObjectGraphStore) InsertObjectIfAbsent(ctx context.Context, obj objectgraph.Object) (bool, error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return false, fmt.Errorf("platform objectgraph: nil store")
	}
	if err := objectgraph.CheckWriteAs(corpusdWriter(ctx), []objectgraph.Object{obj}, nil); err != nil {
		return false, err
	}
	bodyRef, err := o.externalizeBody(obj.Body)
	if err != nil {
		return false, err
	}
	inlineBody := obj.Body
	if bodyRef != "" {
		inlineBody = nil
	}
	inserted, err := o.execObjectWrite(ctx, obj, func(x sqlExecer) (bool, error) {
		res, err := x.ExecContext(ctx, `INSERT IGNORE INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
			obj.VersionID, obj.ContentHash, inlineBody, bodyRef, len(obj.Body), string(obj.Metadata),
			obj.CreatedAt.UTC(), obj.UpdatedAt.UTC(), obj.Tombstone,
			obj.SupersededBy)
		if err != nil {
			return false, err
		}
		n, err := res.RowsAffected()
		return n > 0, err
	})
	if err != nil {
		return false, fmt.Errorf("platform objectgraph: insert object: %w", err)
	}
	if !inserted {
		return false, nil
	}
	o.store.markCorpusDirty("objectgraph insert object " + obj.CanonicalID)
	o.store.indexCaptures(ctx, true, obj)
	return true, nil
}

func (o *ObjectGraphStore) GetObject(ctx context.Context, id string) (_ objectgraph.Object, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return objectgraph.Object{}, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalGetObject)
	defer func() { done("pk", boolRows(err == nil), notFoundIsNil(err)) }()
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

// GetObjects reads objects by canonical ID in one query, keyed by ID. IDs
// that name no object are absent from the result.
func (o *ObjectGraphStore) GetObjects(ctx context.Context, ids []string) (map[string]objectgraph.Object, error) {
	out := map[string]objectgraph.Object{}
	if len(ids) == 0 {
		return out, nil
	}
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	seen := map[string]bool{}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			args = append(args, id)
		}
	}
	if len(args) == 0 {
		return out, nil
	}
	rows, err := o.store.corpus().QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects WHERE canonical_id IN (?`+strings.Repeat(",?", len(args)-1)+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: get objects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		obj, bodyRef, err := scanObjectGraphObject(rows)
		if err != nil {
			return nil, err
		}
		if err := o.hydrateBody(&obj, bodyRef); err != nil {
			return nil, err
		}
		out[obj.CanonicalID] = obj
	}
	return out, rows.Err()
}

func (o *ObjectGraphStore) DeleteObject(ctx context.Context, id string) error {
	if o == nil || o.store == nil || o.store.db == nil {
		return fmt.Errorf("platform objectgraph: nil store")
	}
	_, err := o.store.corpus().ExecContext(ctx, `DELETE FROM og_objects WHERE canonical_id = ?`, id)
	if err != nil {
		return fmt.Errorf("platform objectgraph: delete object: %w", err)
	}
	if o.store.retrievalState.lookupKeysReady.Load() {
		if _, err := o.store.corpus().ExecContext(ctx, `DELETE FROM og_lookup_keys WHERE canonical_id = ?`, id); err != nil {
			return fmt.Errorf("platform objectgraph: delete lookup key: %w", err)
		}
	}
	o.store.unindexCapture(ctx, id)
	return nil
}

// execObjectWrite runs one object write. When the object's kind has a
// lookup key (retrieval_schema.go) the write and the key row share one
// transaction; write reports whether it changed the row.
func (o *ObjectGraphStore) execObjectWrite(ctx context.Context, obj objectgraph.Object, write func(x sqlExecer) (bool, error)) (bool, error) {
	if _, keyed := lookupKeyField(string(obj.ObjectKind)); !keyed || !o.store.retrievalState.lookupKeysReady.Load() {
		return write(o.store.corpus())
	}
	tx, err := o.store.corpus().BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	wrote, err := write(tx)
	if err != nil {
		return false, err
	}
	if wrote {
		if err := o.store.writeLookupKey(ctx, tx, obj); err != nil {
			return false, err
		}
	}
	return wrote, tx.Commit()
}

// listObjectsWindowSQL is one window of ListObjects: the filter's
// equalities, the window's updated_at bounds, newest first with
// canonical_id breaking ties (so every window and the whole answer are
// deterministic), at most want rows. The plan tests explain this text.
func listObjectsWindowSQL(filter objectgraph.ListFilter, w newestWindow, want int) (string, []any) {
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
	cond, condArgs := w.windowSQL("updated_at")
	query += cond + ` ORDER BY updated_at DESC, canonical_id DESC LIMIT ?`
	args = append(append(args, condArgs...), want)
	return query, args
}

func (o *ObjectGraphStore) ListObjects(ctx context.Context, filter objectgraph.ListFilter) (out []objectgraph.Object, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalListObjects)
	shape := ""
	defer func() { done(shape, len(out), err) }()
	// Newest first, time-bounded (retrieval_newest.go).
	limit := objectgraph.NormalizedLimit(filter.Limit)
	shape, err = readNewestWindows(ctx, time.Now(), limit, func(w newestWindow, want int) (int, error) {
		query, qargs := listObjectsWindowSQL(filter, w, want)
		rows, err := o.store.corpus().QueryContext(ctx, query, qargs...)
		if err != nil {
			return 0, fmt.Errorf("platform objectgraph: list objects: %w", err)
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			obj, bodyRef, err := scanObjectGraphObject(rows)
			if err != nil {
				return n, err
			}
			if err := o.hydrateBody(&obj, bodyRef); err != nil {
				return n, err
			}
			out = append(out, obj)
			n++
		}
		if err := rows.Err(); err != nil {
			return n, fmt.Errorf("platform objectgraph: iterate objects: %w", err)
		}
		return n, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (o *ObjectGraphStore) PutEdge(ctx context.Context, edge objectgraph.Edge) error {
	if o == nil || o.store == nil || o.store.db == nil {
		return fmt.Errorf("platform objectgraph: nil store")
	}
	if err := objectgraph.CheckWrite(nil, []objectgraph.Edge{edge}); err != nil {
		return err
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

func (o *ObjectGraphStore) ListEdges(ctx context.Context, filter objectgraph.EdgeFilter) (out []objectgraph.Edge, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalListEdges)
	defer func() { done("", len(out), err) }()
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

// ogLookupKeyMaxIDs bounds the objects one key seek names.
const ogLookupKeyMaxIDs = 1000

// lookupKeySeekSQL names the objects holding one key. The plan tests
// explain this text.
const lookupKeySeekSQL = `SELECT canonical_id FROM og_lookup_keys WHERE object_kind = ? AND lookup_key = ? LIMIT ?`

// metadataJSONEq is the metadata comparison every lookup by value applies.
const metadataJSONEq = `JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?`

// metadataPredicate is the WHERE clause of a lookup by metadata value. A
// registered (kind, path) seeks og_lookup_keys once it is live and reads
// the named rows by primary key; the JSON comparison stays as the deciding
// filter, so a truncated key or a non-string value never changes the
// answer. Any other pair scans the kind, as before. none is true when the
// key names no object (the lookup finds nothing without a second query).
func (o *ObjectGraphStore) metadataPredicate(ctx context.Context, kind, jsonPath, value string) (where string, args []any, shape string, none bool, err error) {
	const jsonEq = metadataJSONEq
	if !o.store.retrievalState.lookupKeyLive.Load() || !lookupKeyFor(kind, jsonPath) {
		return `object_kind = ? AND ` + jsonEq, []any{kind, jsonPath, value}, "json_scan", false, nil
	}
	rows, err := o.store.corpus().QueryContext(ctx,
		lookupKeySeekSQL,
		kind, lookupKeyValue(value), ogLookupKeyMaxIDs)
	if err != nil {
		return "", nil, "lookup_key", false, fmt.Errorf("platform objectgraph: lookup key: %w", err)
	}
	defer rows.Close()
	var ids []any
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", nil, "lookup_key", false, fmt.Errorf("platform objectgraph: lookup key: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", nil, "lookup_key", false, fmt.Errorf("platform objectgraph: lookup key: %w", err)
	}
	if len(ids) == 0 {
		return "", nil, "lookup_key", true, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	args = append(ids, kind, jsonPath, value)
	return `canonical_id IN (` + placeholders + `) AND object_kind = ? AND ` + jsonEq, args, "lookup_key", false, nil
}

// GetObjectByMetadata finds a single object by kind + a metadata JSON path
// equality check. Uses JSON_EXTRACT (supported by Dolt/MySQL).
// Example: GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", "/pub/texture/foo")
func (o *ObjectGraphStore) GetObjectByMetadata(ctx context.Context, kind, jsonPath, value string) (_ objectgraph.Object, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return objectgraph.Object{}, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalGetByMetadata)
	where, args, shape, none, err := o.metadataPredicate(ctx, kind, jsonPath, value)
	defer func() { done(shape, boolRows(err == nil), notFoundIsNil(err)) }()
	if err != nil {
		return objectgraph.Object{}, err
	}
	if none {
		return objectgraph.Object{}, objectgraph.ErrNotFound
	}
	obj, bodyRef, err := scanObjectGraphObject(o.store.corpus().QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE `+where+`
		 LIMIT 1`, args...))
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
func (o *ObjectGraphStore) ListObjectsByMetadata(ctx context.Context, kind, jsonPath, value string, limit int) (out []objectgraph.Object, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalListByMetadata)
	where, args, shape, none, err := o.metadataPredicate(ctx, kind, jsonPath, value)
	defer func() { done(shape, len(out), err) }()
	if err != nil || none {
		return nil, err
	}
	rows, err := o.store.corpus().QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE `+where+`
		 ORDER BY updated_at DESC LIMIT ?`,
		append(args, objectgraph.NormalizedLimit(limit))...)
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list by metadata: %w", err)
	}
	defer rows.Close()
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

// ListObjectsByMetadataForOwner is ListObjectsByMetadata limited to one
// owner's objects in the query itself, so another owner's matching objects
// can never fill the limit.
func (o *ObjectGraphStore) ListObjectsByMetadataForOwner(ctx context.Context, kind, jsonPath, value, ownerID string, limit int) (out []objectgraph.Object, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalListByMetadata)
	where, args, shape, none, err := o.metadataPredicate(ctx, kind, jsonPath, value)
	defer func() { done(shape, len(out), err) }()
	if err != nil || none {
		return nil, err
	}
	rows, err := o.store.corpus().QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE `+where+` AND owner_id = ?
		 ORDER BY updated_at DESC LIMIT ?`,
		append(args, ownerID, objectgraph.NormalizedLimit(limit))...)
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list by metadata for owner: %w", err)
	}
	defer rows.Close()
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

// ListObjectsByMetadataPage is one keyset page of a kind's objects in
// canonical id order, strictly after afterCanonicalID: those whose metadata
// jsonPath equals value (no filter when jsonPath is empty), and only
// ownerID's when it is not empty. A caller reads to the end by passing the
// last id of each full page, so no answer is ever a truncated read (Library
// S3, bunch279-I2). The comparison is the JSON one, never the lookup key,
// whose seek is capped.
func (o *ObjectGraphStore) ListObjectsByMetadataPage(ctx context.Context, kind, jsonPath, value, ownerID, afterCanonicalID string, limit int) (out []objectgraph.Object, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalListByMetadata)
	defer func() { done("json_page", len(out), err) }()
	query, args := metadataPageSQL(kind, jsonPath, value, ownerID, afterCanonicalID, limit)
	rows, err := o.store.corpus().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list by metadata page: %w", err)
	}
	defer rows.Close()
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

// metadataPageSQL is ListObjectsByMetadataPage's query. With an owner it
// reads through idx_og_objects_kind_owner, so its work is bounded by that
// owner's objects of the kind (the plan test pins it); without one it is a
// scan of the kind, which no request path runs (the cross-owner reads go
// through ListKeyedObjectsPage).
func metadataPageSQL(kind, jsonPath, value, ownerID, afterCanonicalID string, limit int) (string, []any) {
	where := `object_kind = ?`
	args := []any{kind}
	if jsonPath != "" {
		where += ` AND ` + metadataJSONEq
		args = append(args, jsonPath, value)
	}
	if ownerID != "" {
		where += ` AND owner_id = ?`
		args = append(args, ownerID)
	}
	return `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE ` + where + ` AND canonical_id > ?
		 ORDER BY canonical_id ASC LIMIT ?`, append(args, afterCanonicalID, objectgraph.NormalizedLimit(limit))
}

// lookupKeyPageSQL is one keyset page of a registered lookup key, in its
// primary key's order (object_kind, lookup_key, canonical_id): the engine
// seeks the key and reads limit index entries in order, with no sort. The
// plan test explains this text.
const lookupKeyPageSQL = `SELECT canonical_id FROM og_lookup_keys WHERE object_kind = ? AND lookup_key = ? AND canonical_id > ? ORDER BY object_kind ASC, lookup_key ASC, canonical_id ASC LIMIT ?`

// errLookupKeysNotLive: the lookup keys are not backfilled yet, so a keyed
// read cannot answer; the caller fails narrow rather than scan.
var errLookupKeysNotLive = errors.New("platform objectgraph: lookup keys are not live yet")

// LookupKeysLive reports whether every registered object has its key row.
func (o *ObjectGraphStore) LookupKeysLive() bool {
	return o != nil && o.store != nil && o.store.retrievalState.lookupKeyLive.Load()
}

// ListKeyedObjectsPage is one page of the objects whose registered lookup
// key (kind, jsonPath) is value, in canonical id order strictly after
// afterCanonicalID: a primary-key page of og_lookup_keys, then the named
// rows by primary key, with the JSON comparison as the deciding filter (a
// truncated key never changes an answer). next is the page's last key id,
// to pass as the next cursor; it is "" when the page is empty, which is
// the end. It refuses an unregistered pair, and errLookupKeysNotLive
// before the backfill switched the keys on: it never falls back to a
// scan (review 291 F1).
func (o *ObjectGraphStore) ListKeyedObjectsPage(ctx context.Context, kind, jsonPath, value, afterCanonicalID string, limit int) (out []objectgraph.Object, next string, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, "", fmt.Errorf("platform objectgraph: nil store")
	}
	if !lookupKeyFor(kind, jsonPath) {
		return nil, "", fmt.Errorf("platform objectgraph: (%s, %s) is not a registered lookup key", kind, jsonPath)
	}
	if !o.LookupKeysLive() {
		return nil, "", errLookupKeysNotLive
	}
	done := o.store.retrievalTimer(retrievalListByMetadata)
	defer func() { done("lookup_key_page", len(out), err) }()
	rows, err := o.store.corpus().QueryContext(ctx, lookupKeyPageSQL, kind, lookupKeyValue(value), afterCanonicalID, objectgraph.NormalizedLimit(limit))
	if err != nil {
		return nil, "", fmt.Errorf("platform objectgraph: lookup key page: %w", err)
	}
	var ids []any
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, "", fmt.Errorf("platform objectgraph: lookup key page: %w", err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil || len(ids) == 0 {
		return nil, "", err
	}
	next = ids[len(ids)-1].(string)
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	objects, err := o.store.corpus().QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, body_ref, body_size, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE canonical_id IN (`+placeholders+`) AND object_kind = ? AND `+metadataJSONEq+`
		 ORDER BY canonical_id ASC`,
		append(ids, kind, jsonPath, value)...)
	if err != nil {
		return nil, "", fmt.Errorf("platform objectgraph: keyed objects: %w", err)
	}
	defer objects.Close()
	for objects.Next() {
		obj, bodyRef, err := scanObjectGraphObject(objects)
		if err != nil {
			return nil, "", err
		}
		if err := o.hydrateBody(&obj, bodyRef); err != nil {
			return nil, "", err
		}
		out = append(out, obj)
	}
	return out, next, objects.Err()
}

// GetEdge finds a single edge by from_id + kind.
func (o *ObjectGraphStore) GetEdge(ctx context.Context, fromID string, kind objectgraph.EdgeKind) (_ objectgraph.Edge, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return objectgraph.Edge{}, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalGetEdge)
	defer func() { done("", boolRows(err == nil), notFoundIsNil(err)) }()
	return scanObjectGraphEdge(o.store.corpus().QueryRowContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = FALSE
		 LIMIT 1`, fromID, string(kind)))
}

// ListEdgesFrom lists all non-tombstoned edges from a given object ID.
func (o *ObjectGraphStore) ListEdgesFrom(ctx context.Context, fromID string) (out []objectgraph.Edge, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalListEdgesFrom)
	defer func() { done("", len(out), err) }()
	rows, err := o.store.corpus().QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND tombstone = FALSE ORDER BY created_at`, fromID)
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list edges from: %w", err)
	}
	defer rows.Close()
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
func (o *ObjectGraphStore) ListEdgesByKind(ctx context.Context, fromID string, kind objectgraph.EdgeKind) (out []objectgraph.Edge, err error) {
	if o == nil || o.store == nil || o.store.db == nil {
		return nil, fmt.Errorf("platform objectgraph: nil store")
	}
	done := o.store.retrievalTimer(retrievalListEdgesByKind)
	defer func() { done("", len(out), err) }()
	rows, err := o.store.corpus().QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = FALSE ORDER BY created_at`,
		fromID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("platform objectgraph: list edges by kind: %w", err)
	}
	defer rows.Close()
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
	if err := objectgraph.CheckWriteAs(corpusdWriter(ctx), batch.Objects, batch.Edges); err != nil {
		return err
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
		if err := o.store.writeLookupKey(ctx, tx, obj); err != nil {
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
	o.store.indexCaptures(ctx, false, batch.Objects...)
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

// boolRows is a single-row read's row count for the retrieval counters.
func boolRows(found bool) int {
	if found {
		return 1
	}
	return 0
}

// notFoundIsNil keeps a miss out of the retrieval error count: a lookup
// that finds nothing has succeeded.
func notFoundIsNil(err error) error {
	if errors.Is(err, objectgraph.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// corpusdWriter declares corpusd as the writer of everything this store
// writes (objectgraph writer gate, SDC ARCH-04). The declaration is made here,
// by the store itself, never taken from a request.
func corpusdWriter(ctx context.Context) context.Context {
	return objectgraph.WithWriter(ctx, objectgraph.WriterCorpusd)
}
