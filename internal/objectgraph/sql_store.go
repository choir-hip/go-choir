package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// commitmentRecordKind is the object kind of the addressed-record ledger
// (internal/store inbox.go). Its inbox, conversation and claim reads are the
// hot scans the record-scoped generated columns below serve
// (docs/archive/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md).
const commitmentRecordKind = ObjectKind("choir.commitment_record")

// metadataIndexColumn is one generated column over a metadata field that an
// equality lookup (ListObjectsByMetadata, ListObjectsByMetadataPage) answers
// by index seek instead of evaluating json_extract on every row of the kind.
type metadataIndexColumn struct {
	Field  string // JSON path leaf under metadata
	Column string // generated column name
	// Kind restricts the column to one object kind: the expression yields
	// NULL for every other kind, and lookups of other kinds use the JSON
	// path. Empty means every kind.
	Kind ObjectKind
	// Every column has the equality index (object_kind, column,
	// canonical_id): a canonical-ID keyset page is read in index order with
	// no sort. RecencyIndex adds (object_kind, column, updated_at) for the
	// newest-first lookups of the columns whose lists are read newest first.
	RecencyIndex bool
}

// metadataIndexColumns maps hot metadata fields to generated columns so
// lookups resolve as index seeks. The first three cover every kind. Column
// names are stable identifiers, not part of any wire protocol.
var metadataIndexColumns = []metadataIndexColumn{
	{Field: "run_id", Column: "og_meta_run_id", RecencyIndex: true},
	{Field: "trajectory_id", Column: "og_meta_trajectory_id", RecencyIndex: true},
	{Field: "channel_id", Column: "og_meta_channel_id", RecencyIndex: true},
	{Field: "inbox", Column: "og_rec_inbox", Kind: commitmentRecordKind},
	{Field: "sender_inbox", Column: "og_rec_sender_inbox", Kind: commitmentRecordKind},
	{Field: "conversation_id", Column: "og_rec_conversation_id", Kind: commitmentRecordKind},
}

// Record-scoped access paths that are not single-field equality lookups.
const (
	// recordInboxSeqIndex serves an inbox's records in a sequence range
	// (ListRecordsByInboxSeq): a cell reads only its unread range.
	recordInboxSeqColumn = "og_rec_inbox_seq"
	recordInboxSeqIndex  = "idx_og_objects_rec_inbox_seq"
	// recordClaimIndex serves the newest records that are not directives
	// (ListCommitmentClaimObjects) as a reverse index walk.
	recordClaimColumn = "og_rec_claim"
	recordClaimIndex  = "idx_og_objects_rec_claim"
)

// ChoirEventUpdatedIndex is the partial index a telemetry retire slice reads
// (store/object_retire.go): the slice's select must carry the literal
// predicate object_kind = 'choir.event' for the planner to use it.
const ChoirEventUpdatedIndex = "idx_og_objects_choir_event_updated"

// ChoirEventUpdatedIndexDDL creates it; idempotent.
const ChoirEventUpdatedIndexDDL = `CREATE INDEX IF NOT EXISTS ` + ChoirEventUpdatedIndex + ` ON og_objects(updated_at) WHERE object_kind = 'choir.event'`

// bodyTextSQL is the body as text. The body column is a BLOB, and SQLite
// reads a BLOB argument of its JSON functions as JSONB, so a body is always
// cast before it meets one.
const bodyTextSQL = `CAST(body AS TEXT)`

// jsonTextSQL is the text of the JSON scalar at path in src, as MySQL's
// JSON_UNQUOTE(JSON_EXTRACT(src, path)) gave it (the MySQL form this replaces): true and false as words,
// a missing path and a JSON null as NULL. path appears twice, so a bound
// path is bound twice (jsonTextArgs).
func jsonTextSQL(src, path string) string {
	return `CASE json_type(` + src + `, ` + path + `) WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE CAST(json_extract(` + src + `, ` + path + `) AS TEXT) END`
}

// jsonTextArgs binds the path of a jsonTextSQL whose path is a placeholder.
func jsonTextArgs(path string) []any { return []any{path, path} }

// recordClaimPredicate is the body predicate the claim pack filters with:
// ListObjectsByOwnerAndBody with "$.directive" missing or empty. The claim
// column holds exactly it.
const recordClaimPredicate = `(json_extract(` + bodyTextSQL + `, '$.directive') IS NULL OR CAST(json_extract(` + bodyTextSQL + `, '$.directive') AS TEXT) = '')`

// generatedColumn is one VIRTUAL generated column of og_objects.
type generatedColumn struct {
	Name, Type, Expr string
}

func (g generatedColumn) ddl() string {
	return g.Name + ` ` + g.Type + ` GENERATED ALWAYS AS (` + g.Expr + `) VIRTUAL`
}

func (mc metadataIndexColumn) generatedColumn() generatedColumn {
	expr := jsonTextSQL(`metadata`, fmt.Sprintf(`'$.%s'`, mc.Field))
	if mc.Kind != "" {
		expr = fmt.Sprintf(`CASE WHEN object_kind = '%s' THEN %s END`, string(mc.Kind), expr)
	}
	return generatedColumn{Name: mc.Column, Type: "TEXT", Expr: expr}
}

func (mc metadataIndexColumn) indexName() string { return "idx_og_objects_" + mc.Column }

func (mc metadataIndexColumn) recencyIndexName() string { return mc.indexName() + "_recent" }

// recordColumns are the record-scoped columns beyond the equality lookups.
// The claim column guards with json_valid: a body that is not JSON makes
// json_extract fail, and a generated column that fails refuses the insert of
// the record (the unindexed predicate fails the whole query on such a row).
var recordColumns = []generatedColumn{
	{Name: recordInboxSeqColumn, Type: "INTEGER", Expr: fmt.Sprintf(
		`CASE WHEN object_kind = '%s' THEN CAST(json_extract(metadata, '$.inbox_seq') AS INTEGER) END`,
		string(commitmentRecordKind))},
	{Name: recordClaimColumn, Type: "INTEGER", Expr: fmt.Sprintf(
		`CASE WHEN object_kind = '%s' THEN CASE WHEN body IS NULL OR json_valid(%s) THEN %s END END`,
		string(commitmentRecordKind), bodyTextSQL, recordClaimPredicate)},
}

// derivedColumns are the generated columns of og_objects.
func derivedColumns() []generatedColumn {
	columns := make([]generatedColumn, 0, len(metadataIndexColumns)+len(recordColumns))
	for _, mc := range metadataIndexColumns {
		columns = append(columns, mc.generatedColumn())
	}
	return append(columns, recordColumns...)
}

// objectIndexes are the indexes of og_objects beyond its primary key, as
// "name ON og_objects(columns)[ WHERE ...]". SQLite secondary indexes carry
// the rowid, not canonical_id, so every index that serves a keyset page
// names canonical_id. The generated-column indexes are partial: most rows
// hold NULL there, and an equality lookup implies NOT NULL. The snapshot
// phase 1 filters on (owner_id, computer_id[, object_kind]); ListObjects
// filters on object_kind + owner_id + computer_id.
func objectIndexes() []string {
	indexes := []string{
		"idx_og_objects_updated ON og_objects(updated_at)",
		"idx_og_objects_kind_id ON og_objects(object_kind, canonical_id)",
		"idx_og_objects_owner_computer_kind ON og_objects(owner_id, computer_id, object_kind, canonical_id)",
		"idx_og_objects_kind_owner_computer ON og_objects(object_kind, owner_id, computer_id, canonical_id)",
	}
	for _, mc := range metadataIndexColumns {
		indexes = append(indexes, fmt.Sprintf("%s ON og_objects(object_kind, %s, canonical_id) WHERE %s IS NOT NULL", mc.indexName(), mc.Column, mc.Column))
		if mc.RecencyIndex {
			indexes = append(indexes, fmt.Sprintf("%s ON og_objects(object_kind, %s, updated_at) WHERE %s IS NOT NULL", mc.recencyIndexName(), mc.Column, mc.Column))
		}
	}
	return append(indexes,
		fmt.Sprintf("%s ON og_objects(object_kind, og_rec_inbox, %s) WHERE %s IS NOT NULL", recordInboxSeqIndex, recordInboxSeqColumn, recordInboxSeqColumn),
		fmt.Sprintf("%s ON og_objects(object_kind, owner_id, %s, updated_at, canonical_id) WHERE %s IS NOT NULL", recordClaimIndex, recordClaimColumn, recordClaimColumn),
	)
}

// buildSchemaDDL is the og_objects and og_edges DDL: the current schema,
// written once, with the generated columns in the CREATE TABLE (SQLite adds
// no STORED generated column afterwards, and a fresh database needs none).
func buildSchemaDDL() string {
	var b strings.Builder
	b.WriteString(`
CREATE TABLE IF NOT EXISTS og_objects (
	canonical_id  TEXT NOT NULL PRIMARY KEY,
	object_kind   TEXT NOT NULL,
	owner_id      TEXT NOT NULL,
	computer_id   TEXT NOT NULL DEFAULT '',
	version_id    TEXT NOT NULL DEFAULT '',
	content_hash  TEXT NOT NULL,
	body          BLOB,
	metadata      TEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone     INTEGER NOT NULL DEFAULT 0,
	superseded_by TEXT NOT NULL DEFAULT ''`)
	for _, column := range derivedColumns() {
		b.WriteString(",\n\t" + column.ddl())
	}
	b.WriteString("\n);\n")
	for _, index := range objectIndexes() {
		b.WriteString("CREATE INDEX IF NOT EXISTS " + index + ";\n")
	}
	b.WriteString(`
CREATE TABLE IF NOT EXISTS og_edges (
	edge_id    TEXT NOT NULL PRIMARY KEY,
	from_id    TEXT NOT NULL,
	to_id      TEXT NOT NULL,
	kind       TEXT NOT NULL,
	metadata   TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	tombstone  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_og_edges_from ON og_edges(from_id);
CREATE INDEX IF NOT EXISTS idx_og_edges_to ON og_edges(to_id);
`)
	return b.String()
}

// SchemaDDL creates the og_objects and og_edges tables if they do not
// already exist.
var SchemaDDL = buildSchemaDDL()

// SQLStore is a Store backed by a SQLite *sql.DB. It is the VM-local
// equivalent of the platform ObjectGraphStore: the same og_objects and
// og_edges tables, in the computer's own database file.
//
// A SQLStore over the write handle of a dbx.SQLiteDB writes (its one
// connection is the single writer); a SQLStore over the read pool serves
// reads and never writes. Write paths here run every statement of a
// transaction on the transaction, never on the database handle, because the
// write handle holds one connection.
type SQLStore struct {
	db        *sql.DB
	intercept MutationInterceptor
	validate  WriteValidator
	// derived is the state of the og_key_index side table (derived_keys.go),
	// shared by the write and read stores of one database.
	derived *DerivedKeys
}

// MutationInterceptor receives durable object/edge mutations before SQL.
// When set, PutObject/PutEdge/PutBatch/DeleteObject do not write SQL;
// the interceptor must append+project instead. Projector SQL bypasses this.
type MutationInterceptor func(ctx context.Context, objects []Object, edges []Edge) error

// WriteValidator inspects durable object/edge mutations before SQL and
// before any MutationInterceptor. A non-nil error refuses the write. The
// vocabulary cutover uses this to fail closed on stale-desk writes.
type WriteValidator func(ctx context.Context, objects []Object, edges []Edge) error

func (s *SQLStore) SetMutationInterceptor(fn MutationInterceptor) {
	if s == nil {
		return
	}
	s.intercept = fn
}

func (s *SQLStore) SetWriteValidator(fn WriteValidator) {
	if s == nil {
		return
	}
	s.validate = fn
}

// JSONFieldMatch is one JSON field predicate for an object lookup.
// JSONPath is passed as a query argument (for example, "$.run_id"), never
// interpolated into SQL. MissingMatchesEmpty lets legacy JSON bodies whose
// omitempty field is absent match the current empty-string representation.
type JSONFieldMatch struct {
	JSONPath            string
	Value               string
	MissingMatchesEmpty bool
}

// NewSQLStore returns a SQLStore backed by the given *sql.DB. The caller
// must call EnsureSchema (on the write handle) before using the store.
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db, derived: newDerivedKeys()}
}

// EnsureSchema creates the og_objects and og_edges tables if they do
// not already exist. Safe to call multiple times.
func (s *SQLStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph sql: nil store")
	}
	doneDDL := BootPhase("store: open", "objectgraph-schema/schema-ddl")
	_, err := s.db.ExecContext(ctx, SchemaDDL)
	doneDDL(err)
	if err != nil {
		return fmt.Errorf("objectgraph sql: ensure schema: %w", err)
	}
	// The partial index a telemetry retire slice reads through. It is its own
	// step, not in objectIndexes, so its duration shows in the boot log: on
	// an existing store the build reads every choir.event row (updated_at
	// follows the body in the row), about 30 s for 2.5 million rows on the
	// paper's copy. Not inside any batch transaction.
	doneIdx := BootPhase("store: open", "objectgraph-schema/choir-event-updated-index")
	_, err = s.db.ExecContext(ctx, ChoirEventUpdatedIndexDDL)
	doneIdx(err)
	if err != nil {
		return fmt.Errorf("objectgraph sql: ensure schema: %w", err)
	}
	// The derived-key side table is created empty: nothing here scales with
	// og_objects. Its rows are written by the writers and checked online
	// after the listener is up (derived_keys.go).
	s.ensureDerivedKeysTable(ctx)
	return nil
}

// metadataColumnFor returns the generated column that answers
// metadata.<jsonPath> = value for kind, if any. The column name comes from
// the fixed metadataIndexColumns table, never from the caller.
func metadataColumnFor(kind, jsonPath string) (metadataIndexColumn, bool) {
	leaf, ok := strings.CutPrefix(jsonPath, "$.")
	if !ok {
		return metadataIndexColumn{}, false
	}
	for _, mc := range metadataIndexColumns {
		if mc.Field != leaf {
			continue
		}
		if mc.Kind != "" && string(mc.Kind) != kind {
			continue
		}
		return mc, true
	}
	return metadataIndexColumn{}, false
}

// PreflightWrite runs the refusals PutObject and PutBatch apply before they
// hand a mutation to the projection interceptor: the write gate (an
// unregistered kind) and the configured write validator. A caller that holds a
// mutation back and appends it later (the telemetry sealer) calls it at the
// moment the write would have been made, so a refusal still reaches the caller
// that issued the write.
func (s *SQLStore) PreflightWrite(ctx context.Context, objects []Object, edges []Edge) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph sql: nil store")
	}
	if err := CheckWriteAs(ctx, objects, edges); err != nil {
		return err
	}
	if s.validate != nil {
		return s.validate(ctx, objects, edges)
	}
	return nil
}

func (s *SQLStore) PutObject(ctx context.Context, obj Object) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph sql: nil store")
	}
	if err := CheckWriteAs(ctx, []Object{obj}, nil); err != nil {
		return err
	}
	// validate + intercept run before any database work: intercept is the
	// projection append path (network CAS + event write) and must not hold
	// the write connection across a host round-trip.
	if s.validate != nil {
		if err := s.validate(ctx, []Object{obj}, nil); err != nil {
			return err
		}
	}
	if s.intercept != nil {
		return s.intercept(ctx, []Object{obj}, nil)
	}
	if s.derived.keyed(obj.ObjectKind) {
		// A kind with derived keys is written with them in one transaction.
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("objectgraph sql: put object: begin: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, upsertObjectSQL, upsertObjectArgs(obj)...); err != nil {
			return fmt.Errorf("objectgraph sql: put object: %w", err)
		}
		s.derived.Maintain(ctx, tx, obj.CanonicalID, obj.ObjectKind)
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("objectgraph sql: put object: commit: %w", err)
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, upsertObjectSQL, upsertObjectArgs(obj)...)
	if err != nil {
		return fmt.Errorf("objectgraph sql: put object: %w", err)
	}
	return nil
}

// upsertObjectSQL is the og_objects upsert every direct writer shares.
const upsertObjectSQL = `INSERT INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (canonical_id) DO UPDATE SET
			object_kind = excluded.object_kind,
			owner_id = excluded.owner_id,
			computer_id = excluded.computer_id,
			version_id = excluded.version_id,
			content_hash = excluded.content_hash,
			body = excluded.body,
			metadata = excluded.metadata,
			updated_at = excluded.updated_at,
			tombstone = excluded.tombstone,
			superseded_by = excluded.superseded_by`

func upsertObjectArgs(obj Object) []any {
	return []any{obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
		obj.VersionID, obj.ContentHash, obj.Body, string(obj.Metadata),
		obj.CreatedAt.UTC(), obj.UpdatedAt.UTC(), obj.Tombstone,
		obj.SupersededBy}
}

func (s *SQLStore) GetObject(ctx context.Context, id string) (Object, error) {
	if s == nil || s.db == nil {
		return Object{}, fmt.Errorf("objectgraph sql: nil store")
	}
	return scanObject(s.db.QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id = ?`, id))
}

func (s *SQLStore) ListObjects(ctx context.Context, filter ListFilter) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
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
	if filter.ComputerID != "" {
		query += ` AND computer_id = ?`
		args = append(args, filter.ComputerID)
	}
	if filter.Tombstone != nil {
		query += ` AND tombstone = ?`
		args = append(args, *filter.Tombstone)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, NormalizedLimit(filter.Limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list objects: %w", err)
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
		return nil, fmt.Errorf("objectgraph sql: iterate objects: %w", err)
	}
	return out, nil
}

// ReadObjectSnapshot returns the complete owner/computer object scope from one
// serializable read transaction. Callers filter the immutable result in memory
// so state and its event watermark cannot come from different commits.
func (s *SQLStore) ReadObjectSnapshot(ctx context.Context, ownerID, computerID string) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("objectgraph sql: snapshot owner_id and computer_id are required")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: begin snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		FROM og_objects WHERE owner_id = ? AND computer_id = ? ORDER BY canonical_id`, ownerID, computerID)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: query snapshot: %w", err)
	}
	defer rows.Close()
	var objects []Object
	for rows.Next() {
		obj, scanErr := scanObject(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		objects = append(objects, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("objectgraph sql: iterate snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("objectgraph sql: snapshot commit: %w", err)
	}
	return objects, nil
}

// ReadObjectSnapshotFiltered is the scoped variant of ReadObjectSnapshot for
// hot paths that need only a subset of the computer's objects. It runs both
// phases inside one serializable read-only transaction so the result is
// consistent: phase one scans ref columns (no body blob materialization)
// for the requested kinds, applies keep(kind, metadata) to decide membership,
// and phase two fetches full rows by primary key for survivors only.
// Returned objects are sorted by canonical_id, matching ReadObjectSnapshot.
// snapshotFetchBatch bounds each phase-2 IN list in ReadObjectSnapshotFiltered.
const snapshotFetchBatch = 256

func (s *SQLStore) ReadObjectSnapshotFiltered(ctx context.Context, ownerID, computerID string, kinds []ObjectKind, keep func(kind ObjectKind, metadata json.RawMessage) bool) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("objectgraph sql: snapshot owner_id and computer_id are required")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: begin snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Phase 1: refs only for the requested kinds.
	refsQuery := `SELECT canonical_id, object_kind, metadata FROM og_objects WHERE owner_id = ? AND computer_id = ?`
	args := []any{ownerID, computerID}
	if len(kinds) > 0 {
		refsQuery += ` AND object_kind IN (`
		for i, k := range kinds {
			if i > 0 {
				refsQuery += `,`
			}
			refsQuery += `?`
			args = append(args, string(k))
		}
		refsQuery += `)`
	}
	refsQuery += ` ORDER BY canonical_id`
	rows, err := tx.QueryContext(ctx, refsQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: snapshot refs: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id, kindStr, metadata string
		if err := rows.Scan(&id, &kindStr, &metadata); err != nil {
			rows.Close()
			return nil, fmt.Errorf("objectgraph sql: scan ref: %w", err)
		}
		if keep == nil || keep(ObjectKind(kindStr), json.RawMessage(metadata)) {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("objectgraph sql: iterate snapshot refs: %w", err)
	}
	rows.Close()
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("objectgraph sql: snapshot commit: %w", err)
		}
		return nil, nil
	}

	// Phase 2: PK-fetch bodies for survivors, preserving canonical_id order.
	objects, err := snapshotFetchByID(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("objectgraph sql: snapshot commit: %w", err)
	}
	return objects, nil
}

// trajectoryRefsSQL is one kind's refs for one trajectory, through the
// (object_kind, og_meta_trajectory_id, updated_at) index. Owner and computer
// are checked on the rows, not in the predicate: given them, the planner
// takes the (object_kind, owner_id, computer_id) index and reads every row
// of the kind, the scan this replaces.
func trajectoryRefsSQL(kind ObjectKind, trajectoryID string) (string, []any) {
	return `SELECT canonical_id, object_kind, owner_id, computer_id, metadata FROM og_objects
WHERE object_kind = ? AND og_meta_trajectory_id = ?`,
		[]any{string(kind), trajectoryID}
}

// trajectoryFollowRounds bounds ReadTrajectorySnapshot's follow rounds.
const trajectoryFollowRounds = 4

// ReadTrajectorySnapshot reads one trajectory's objects in one read-only
// serializable transaction, the same isolation ReadObjectSnapshotFiltered
// gives. First the rows of kinds whose metadata trajectory_id is
// trajectoryID, through the og_meta_trajectory_id index, kept by keep (the
// exact check: the column is a superset under the engine's collation); then
// up to trajectoryFollowRounds rounds of the objects follow names by
// canonical id, given everything read so far, until it names nothing new.
// The result is in canonical_id order, as the scan's is.
//
// ok is false, with nothing read, when trajectoryID is empty; the caller then reads with ReadObjectSnapshotFiltered.
// The filtered scan reads the metadata of every object of its kinds in the
// computer: 416 ms per call on the paper (docs/problems/
// research-more-slow-and-hidden-2026-10-21.md, E6).
func (s *SQLStore) ReadTrajectorySnapshot(ctx context.Context, ownerID, computerID, trajectoryID string, kinds []ObjectKind, keep func(kind ObjectKind, metadata json.RawMessage) bool, follow func(have []Object) []string) ([]Object, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("objectgraph sql: nil store")
	}
	ownerID, computerID, trajectoryID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(trajectoryID)
	if ownerID == "" || computerID == "" {
		return nil, false, fmt.Errorf("objectgraph sql: snapshot owner_id and computer_id are required")
	}
	if trajectoryID == "" {
		return nil, false, nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, false, fmt.Errorf("objectgraph sql: begin trajectory snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var ids []string
	for _, kind := range kinds {
		query, args := trajectoryRefsSQL(kind, trajectoryID)
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, false, fmt.Errorf("objectgraph sql: trajectory refs: %w", err)
		}
		for rows.Next() {
			var id, kindStr, owner, computer, metadata string
			if err := rows.Scan(&id, &kindStr, &owner, &computer, &metadata); err != nil {
				rows.Close()
				return nil, false, fmt.Errorf("objectgraph sql: scan trajectory ref: %w", err)
			}
			if owner != ownerID || computer != computerID {
				continue
			}
			if keep == nil || keep(ObjectKind(kindStr), json.RawMessage(metadata)) {
				ids = append(ids, id)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, false, fmt.Errorf("objectgraph sql: iterate trajectory refs: %w", err)
		}
		rows.Close()
	}
	sort.Strings(ids)
	objects, err := snapshotFetchByID(ctx, tx, ids)
	if err != nil {
		return nil, false, err
	}
	seen := make(map[string]bool, len(objects))
	for _, obj := range objects {
		seen[obj.CanonicalID] = true
	}
	for round := 0; follow != nil && round < trajectoryFollowRounds; round++ {
		var next []string
		for _, id := range follow(objects) {
			if id = strings.TrimSpace(id); id != "" && !seen[id] {
				seen[id] = true
				next = append(next, id)
			}
		}
		if len(next) == 0 {
			break
		}
		sort.Strings(next)
		more, err := snapshotFetchByID(ctx, tx, next)
		if err != nil {
			return nil, false, err
		}
		for _, obj := range more {
			if obj.OwnerID == ownerID && obj.ComputerID == computerID {
				objects = append(objects, obj)
			}
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].CanonicalID < objects[j].CanonicalID })
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("objectgraph sql: trajectory snapshot commit: %w", err)
	}
	return objects, true, nil
}

// snapshotFetchByID fetches objects by canonical id inside tx, in id order.
// Fixed-size IN batches: a short list keeps the statement cheap
// in the IN list, and one list of every survivor cost 42% of guest CPU on a
// long-lived computer (docs/archive/problems/texture-list-cold-latency-2026-10-09.md).
func snapshotFetchByID(ctx context.Context, tx *sql.Tx, ids []string) ([]Object, error) {
	objects := make([]Object, 0, len(ids))
	for start := 0; start < len(ids); start += snapshotFetchBatch {
		batch := ids[start:min(start+snapshotFetchBatch, len(ids))]
		objectsQuery := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		FROM og_objects WHERE canonical_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",") + `) ORDER BY canonical_id`
		objArgs := make([]any, len(batch))
		for i, id := range batch {
			objArgs[i] = id
		}
		objRows, err := tx.QueryContext(ctx, objectsQuery, objArgs...)
		if err != nil {
			return nil, fmt.Errorf("objectgraph sql: snapshot bodies: %w", err)
		}
		for objRows.Next() {
			obj, scanErr := scanObject(objRows)
			if scanErr != nil {
				objRows.Close()
				return nil, scanErr
			}
			objects = append(objects, obj)
		}
		if err := objRows.Err(); err != nil {
			objRows.Close()
			return nil, fmt.Errorf("objectgraph sql: iterate snapshot bodies: %w", err)
		}
		objRows.Close()
	}
	return objects, nil
}

func (s *SQLStore) PutEdge(ctx context.Context, edge Edge) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph sql: nil store")
	}
	if err := CheckWrite(nil, []Edge{edge}); err != nil {
		return err
	}
	if s.intercept != nil {
		return s.intercept(ctx, nil, []Edge{edge})
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO og_edges
		(edge_id, from_id, to_id, kind, metadata, created_at, tombstone)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (edge_id) DO UPDATE SET
			from_id = excluded.from_id,
			to_id = excluded.to_id,
			kind = excluded.kind,
			metadata = excluded.metadata,
			tombstone = excluded.tombstone`,
		edge.EdgeID, edge.FromID, edge.ToID, string(edge.Kind), string(edge.Metadata),
		edge.CreatedAt.UTC(), edge.Tombstone)
	if err != nil {
		return fmt.Errorf("objectgraph sql: put edge: %w", err)
	}
	return nil
}

func (s *SQLStore) ListEdges(ctx context.Context, filter EdgeFilter) ([]Edge, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
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
		return nil, fmt.Errorf("objectgraph sql: list edges: %w", err)
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
		return nil, fmt.Errorf("objectgraph sql: iterate edges: %w", err)
	}
	return out, nil
}

// PutBatch writes a batch of objects and edges atomically in a single
// transaction.
func (s *SQLStore) PutBatch(ctx context.Context, batch Batch) error {
	return s.putBatch(ctx, nil, batch)
}

// PutBatchConditional compares object versions and writes the batch in the
// same transaction. Conditions are locked in caller-supplied deterministic
// order; lifecycle reducers sort them by canonical ID before calling.
func (s *SQLStore) PutBatchConditional(ctx context.Context, conditions []ObjectCondition, batch Batch) error {
	return s.putBatch(ctx, conditions, batch)
}
func (s *SQLStore) putBatch(ctx context.Context, conditions []ObjectCondition, batch Batch) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph sql: nil store")
	}
	if err := CheckWriteAs(ctx, batch.Objects, batch.Edges); err != nil {
		return err
	}
	// validate + intercept run before any database work: intercept is the
	// projection append path (network CAS + event write), and holding the
	// write connection across a host round-trip stalls every write.
	if s.validate != nil {
		if err := s.validate(ctx, batch.Objects, batch.Edges); err != nil {
			return err
		}
	}
	if s.intercept != nil {
		if err := s.evaluateConditions(ctx, conditions); err != nil {
			return err
		}
		return s.intercept(ctx, batch.Objects, batch.Edges)
	}
	// The write transaction takes the database's write lock at BEGIN, so the
	// condition check and the writes below are one atomic step.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("objectgraph sql: begin batch tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := checkConditions(ctx, tx, conditions); err != nil {
		return err
	}

	if err := putBatchTx(ctx, tx, batch, s.derived); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("objectgraph sql: batch commit: %w", err)
	}
	return nil
}

func putBatchTx(ctx context.Context, tx *sql.Tx, batch Batch, derived *DerivedKeys) error {
	for _, obj := range batch.Objects {
		if _, err := tx.ExecContext(ctx, upsertObjectSQL, upsertObjectArgs(obj)...); err != nil {
			return fmt.Errorf("objectgraph sql: batch put object %s: %w", obj.CanonicalID, err)
		}
		derived.Maintain(ctx, tx, obj.CanonicalID, obj.ObjectKind)
	}

	for _, edge := range batch.Edges {
		if _, err := tx.ExecContext(ctx, `INSERT INTO og_edges
			(edge_id, from_id, to_id, kind, metadata, created_at, tombstone)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (edge_id) DO UPDATE SET
			from_id = excluded.from_id,
			to_id = excluded.to_id,
			kind = excluded.kind,
			metadata = excluded.metadata,
			tombstone = excluded.tombstone`,
			edge.EdgeID, edge.FromID, edge.ToID, string(edge.Kind), string(edge.Metadata),
			edge.CreatedAt.UTC(), edge.Tombstone); err != nil {
			return fmt.Errorf("objectgraph sql: batch put edge %s: %w", edge.EdgeID, err)
		}
	}
	return nil
}

// GetObjectByMetadata finds a single object by kind + a metadata JSON
// path equality check using json_extract.
func (s *SQLStore) GetObjectByMetadata(ctx context.Context, kind, jsonPath, value string) (Object, error) {
	if s == nil || s.db == nil {
		return Object{}, fmt.Errorf("objectgraph sql: nil store")
	}
	return scanObject(s.db.QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND `+jsonTextSQL(`metadata`, `?`)+` = ?
		 LIMIT 1`, append(append([]any{kind}, jsonTextArgs(jsonPath)...), value)...))
}

const objectColumnsSQL = `canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by`

// metadataListSQL is ListObjectsByMetadata's query: the indexed generated
// column when one covers (kind, field), json_extract otherwise.
// The plan-shape tests EXPLAIN exactly this text.
func metadataListSQL(kind, jsonPath, value string, limit int) (string, []any) {
	if mc, ok := metadataColumnFor(kind, jsonPath); ok {
		return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND ` + mc.Column + ` = ?
		 ORDER BY updated_at DESC LIMIT ?`, []any{kind, value, NormalizedLimit(limit)}
	}
	return metadataListJSONSQL(kind, jsonPath, value, limit)
}

// metadataListJSONSQL is the list read on the JSON path, which the generated
// columns answer identically.
func metadataListJSONSQL(kind, jsonPath, value string, limit int) (string, []any) {
	return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND ` + jsonTextSQL(`metadata`, `?`) + ` = ?
		 ORDER BY updated_at DESC LIMIT ?`, append(append([]any{kind}, jsonTextArgs(jsonPath)...), value, NormalizedLimit(limit))
}

// metadataPageSQL is ListObjectsByMetadataPage's query. With a keyset index
// (object_kind, column, canonical_id) the page orders by every index column:
// object_kind and the column are fixed by equality, so the order is
// canonical_id's, and the engine reads the page in index order with no sort
// (ordering by canonical_id alone makes it a TopN over the whole remaining
// range).
func metadataPageSQL(kind, jsonPath, value, afterCanonicalID string, limit int) (string, []any) {
	if mc, ok := metadataColumnFor(kind, jsonPath); ok {
		order := `object_kind ASC, ` + mc.Column + ` ASC, canonical_id ASC`
		return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND ` + mc.Column + ` = ? AND canonical_id > ?
		 ORDER BY ` + order + ` LIMIT ?`, []any{kind, value, afterCanonicalID, NormalizedLimit(limit)}
	}
	return metadataPageJSONSQL(kind, jsonPath, value, afterCanonicalID, limit)
}

// metadataPageJSONSQL is the page read on the JSON path.
func metadataPageJSONSQL(kind, jsonPath, value, afterCanonicalID string, limit int) (string, []any) {
	return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND ` + jsonTextSQL(`metadata`, `?`) + ` = ? AND canonical_id > ?
		 ORDER BY canonical_id ASC LIMIT ?`, append(append([]any{kind}, jsonTextArgs(jsonPath)...), value, afterCanonicalID, NormalizedLimit(limit))
}

// ListObjectsByMetadata finds objects by kind + a metadata JSON path
// equality check.
func (s *SQLStore) ListObjectsByMetadata(ctx context.Context, kind, jsonPath, value string, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	query, args := metadataListSQL(kind, jsonPath, value, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list by metadata: %w", err)
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
	return out, rows.Err()
}

// ListObjectsPage scans one object kind in stable canonical-ID order. The
// exclusive cursor makes full-table maintenance and recovery scans bounded
// without imposing a correctness-limiting fixed window.
func (s *SQLStore) ListObjectsPage(ctx context.Context, kind, afterCanonicalID string, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND canonical_id > ?
		 ORDER BY canonical_id ASC LIMIT ?`,
		kind, afterCanonicalID, NormalizedLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list objects page: %w", err)
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		obj, scanErr := scanObject(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

// ObjectsByKindOwnerComputerPageSQL is one keyset page of the objects of one
// kind, one owner and one computer_id (the empty string is a value: the
// legacy, computer-less rows). The (object_kind, owner_id, computer_id) index
// carries canonical_id as its suffix, so the engine seeks the scope and the
// cursor and reads `limit` index entries in order, with no sort; ListObjectsPage's
// plan for the same cursor reads every remaining row of the kind into a TopN.
// A store without that index (a populated store never migrated) answers the
// same rows through idx_og_objects_kind_owner, which every store has.
const ObjectsByKindOwnerComputerPageSQL = `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
	 FROM og_objects
	 WHERE object_kind = ? AND owner_id = ? AND computer_id = ? AND canonical_id > ?
	 ORDER BY canonical_id ASC LIMIT ?`

// ListObjectsByKindOwnerComputerPage pages the objects of one kind, owner and
// computer_id in canonical-ID order, strictly after afterCanonicalID. Use it
// instead of ListObjectsPage plus a filter when the caller keeps one owner's
// rows of one computer scope: work is bounded by the scope, not the kind.
func (s *SQLStore) ListObjectsByKindOwnerComputerPage(ctx context.Context, kind, ownerID, computerID, afterCanonicalID string, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	rows, err := s.db.QueryContext(ctx, ObjectsByKindOwnerComputerPageSQL,
		kind, ownerID, computerID, afterCanonicalID, NormalizedLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list objects by kind owner computer page: %w", err)
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		obj, scanErr := scanObject(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

// ListObjectsByMetadataPage finds objects by kind + a metadata JSON path
// equality check, continuing strictly after afterCanonicalID in canonical ID
// order.
func (s *SQLStore) ListObjectsByMetadataPage(ctx context.Context, kind, jsonPath, value, afterCanonicalID string, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	if !strings.HasPrefix(jsonPath, "$.") {
		return nil, fmt.Errorf("objectgraph sql: invalid metadata JSON path %q", jsonPath)
	}
	if fam := s.derivedMetadataRead(kind, jsonPath, value); fam != nil {
		// Validated side table: ids from the key index, rows by primary key.
		// Any doubt (a stale row) withdraws the family and falls through to
		// the JSON read below.
		if objs, ok, err := s.listByDerivedKeyPage(ctx, fam, value, afterCanonicalID, limit); err != nil {
			return nil, err
		} else if ok {
			return objs, nil
		}
	}
	query, args := metadataPageSQL(kind, jsonPath, value, afterCanonicalID, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list by metadata page: %w", err)
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
	return out, rows.Err()
}

// ListObjectsByEdgeTo returns objects of one kind that hold a live edge of
// edgeKind pointing at toID. The join rides the indexed og_edges.to_id lookup
// plus the og_objects primary key — no metadata JSON scan.
func (s *SQLStore) ListObjectsByEdgeTo(ctx context.Context, toID string, edgeKind EdgeKind, objKind string) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT o.canonical_id, o.object_kind, o.owner_id, o.computer_id, o.version_id, o.content_hash, o.body, o.metadata, o.created_at, o.updated_at, o.tombstone, o.superseded_by
		 FROM og_objects o
		 JOIN og_edges e ON e.from_id = o.canonical_id
		 WHERE e.to_id = ? AND e.kind = ? AND o.object_kind = ? AND e.tombstone = 0
		 ORDER BY o.canonical_id ASC`,
		toID, string(edgeKind), objKind)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list objects by edge to: %w", err)
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		obj, scanErr := scanObject(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}

// CountObjectsByEdgeTo returns the count of objects of one kind holding a live
// edge of edgeKind to toID, plus the maximum version_number lifted from object
// metadata — the indexed edge join without decoding a single body. Used by the
// Texture recent-documents list, which needs count + head version only.
func (s *SQLStore) CountObjectsByEdgeTo(ctx context.Context, toID string, edgeKind EdgeKind, objKind string) (count int, maxVersion int, err error) {
	if s == nil || s.db == nil {
		return 0, -1, fmt.Errorf("objectgraph sql: nil store")
	}
	maxVersion = -1
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*),
				COALESCE(MAX(CAST(json_extract(o.metadata, '$.version_number') AS INTEGER)), -1)
		 FROM og_objects o
		 JOIN og_edges e ON e.from_id = o.canonical_id
		 WHERE e.to_id = ? AND e.kind = ? AND o.object_kind = ? AND e.tombstone = 0 AND o.tombstone = 0`,
		toID, string(edgeKind), objKind).Scan(&count, &maxVersion)
	if err != nil {
		return 0, -1, fmt.Errorf("objectgraph sql: count objects by edge to: %w", err)
	}
	return count, maxVersion, nil
}

// ListObjectsByOwnerAndBody finds objects by kind, owner, and an exact set of
// predicates evaluated against the persisted JSON body. The body is the
// canonical authority for record fields that must not be duplicated into
// independently drifting object metadata.
func (s *SQLStore) ListObjectsByOwnerAndBody(ctx context.Context, kind, ownerID string, matches []JSONFieldMatch, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	if fam := s.derivedAddresseeRead(kind, matches); fam != nil {
		// The owner's newest rows of the key, from the validated side table;
		// a stale row withdraws the family and falls through to the scan.
		if objs, ok, err := s.listByDerivedKeyNewest(ctx, fam, matches[0].Value, ownerID, limit); err != nil {
			return nil, err
		} else if ok {
			return objs, nil
		}
	}
	query := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND owner_id = ?`
	args := []any{kind, ownerID}
	for _, match := range matches {
		if match.JSONPath == "" {
			return nil, fmt.Errorf("objectgraph sql: body JSON path is required")
		}
		if match.MissingMatchesEmpty {
			if match.Value != "" {
				return nil, fmt.Errorf("objectgraph sql: missing body field can only match an empty value")
			}
			query += ` AND (json_extract(` + bodyTextSQL + `, ?) IS NULL OR ` + jsonTextSQL(bodyTextSQL, `?`) + ` = ?)`
			args = append(args, match.JSONPath)
			args = append(args, jsonTextArgs(match.JSONPath)...)
			args = append(args, match.Value)
			continue
		}
		query += ` AND ` + jsonTextSQL(bodyTextSQL, `?`) + ` = ?`
		args = append(args, jsonTextArgs(match.JSONPath)...)
		args = append(args, match.Value)
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, NormalizedLimit(limit))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list by owner and body: %w", err)
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
	return out, rows.Err()
}

// recordsByInboxSeqSQL is ListRecordsByInboxSeq's query: an index range on
// (object_kind, inbox, inbox_seq), ordered by every index column so the
// engine reads it in index order. The plan-shape tests EXPLAIN this text.
func recordsByInboxSeqSQL(inbox string, afterSeq, throughSeq int64) (string, []any) {
	query := `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND og_rec_inbox = ? AND ` + recordInboxSeqColumn + ` > ?`
	args := []any{string(commitmentRecordKind), inbox, afterSeq}
	if throughSeq > 0 {
		query += ` AND ` + recordInboxSeqColumn + ` <= ?`
		args = append(args, throughSeq)
	}
	query += ` ORDER BY object_kind ASC, og_rec_inbox ASC, ` + recordInboxSeqColumn + ` ASC`
	return query, args
}

// ListRecordsByInboxSeq returns the commitment records whose metadata inbox
// is inbox and whose metadata inbox_seq lies in (afterSeq, throughSeq]
// (throughSeq <= 0: no upper bound), by sequence. It reads only that range
// of the index instead of every record of the inbox. ok is always true; it
// stays so callers that kept an unindexed fallback compile. Owner, computer and tombstone
// are the caller's filters, as on the metadata lookups.
func (s *SQLStore) ListRecordsByInboxSeq(ctx context.Context, inbox string, afterSeq, throughSeq int64) ([]Object, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("objectgraph sql: nil store")
	}
	query, args := recordsByInboxSeqSQL(inbox, afterSeq, throughSeq)
	objects, err := s.queryObjects(ctx, query, args...)
	if err != nil {
		return nil, true, fmt.Errorf("objectgraph sql: records by inbox seq: %w", err)
	}
	return objects, true, nil
}

// commitmentClaimSQL is ListCommitmentClaimObjects' query: a reverse walk of
// (object_kind, owner_id, og_rec_claim, updated_at, canonical_id), ordered
// by every index column descending so the engine stops after limit rows.
func commitmentClaimSQL(ownerID string, limit int) (string, []any) {
	return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND owner_id = ? AND ` + recordClaimColumn + ` = 1
		 ORDER BY object_kind DESC, owner_id DESC, ` + recordClaimColumn + ` DESC, updated_at DESC, canonical_id DESC LIMIT ?`,
		[]any{string(commitmentRecordKind), ownerID, NormalizedLimit(limit)}
}

// ListCommitmentClaimObjects returns the owner's newest commitment records
// whose body has no directive (missing or empty), newest first: the rows
// ListObjectsByOwnerAndBody returns for that predicate, read as a reverse
// index walk instead of a JSON evaluation of every record's body. Rows with
// equal updated_at (second precision) come by canonical_id descending; the
// unindexed read left their order to the engine. ok is always true.
func (s *SQLStore) ListCommitmentClaimObjects(ctx context.Context, ownerID string, limit int) ([]Object, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("objectgraph sql: nil store")
	}
	query, args := commitmentClaimSQL(ownerID, limit)
	objects, err := s.queryObjects(ctx, query, args...)
	if err != nil {
		return nil, true, fmt.Errorf("objectgraph sql: commitment claims: %w", err)
	}
	return objects, true, nil
}

// queryObjects runs a full-row object query.
func (s *SQLStore) queryObjects(ctx context.Context, query string, args ...any) ([]Object, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
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
	return out, rows.Err()
}

// ObjectRef is an og_objects row without the body payload. Boot and other
// bounded owner scans use it so json_extract + blob materialization of
// every choir.run cannot OOM the guest.
type ObjectRef struct {
	CanonicalID string
	ComputerID  string
	Metadata    json.RawMessage
	UpdatedAt   time.Time
	Tombstone   bool
}

// ListObjectRefsByKindOwner returns the newest objects of one kind for one
// owner using idx_og_objects_kind_owner_computer. It does not json_extract metadata
// and does not load body.
func (s *SQLStore) ListObjectRefsByKindOwner(ctx context.Context, kind, ownerID string, limit int) ([]ObjectRef, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("objectgraph sql: owner_id is required")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, computer_id, metadata, updated_at, tombstone
		 FROM og_objects
		 WHERE object_kind = ? AND owner_id = ?
		 ORDER BY updated_at DESC LIMIT ?`,
		kind, ownerID, NormalizedLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list refs by kind owner: %w", err)
	}
	defer rows.Close()
	var out []ObjectRef
	for rows.Next() {
		var ref ObjectRef
		var metadata string
		if scanErr := rows.Scan(&ref.CanonicalID, &ref.ComputerID, &metadata, &ref.UpdatedAt, &ref.Tombstone); scanErr != nil {
			return nil, scanErr
		}
		ref.Metadata = json.RawMessage(metadata)
		out = append(out, ref)
	}
	return out, rows.Err()
}

// ListAllObjectRefsByKindOwner returns every object header of one kind for one
// owner using idx_og_objects_kind_owner, without LIMIT and without body.
// Prefer it over ReadObjectSnapshot when the caller filters or sorts after
// decoding: run-kind rows are a small fraction of the object graph.
func (s *SQLStore) ListAllObjectRefsByKindOwner(ctx context.Context, kind, ownerID string) ([]ObjectRef, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("objectgraph sql: owner_id is required")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, computer_id, metadata, updated_at, tombstone
		 FROM og_objects
		 WHERE object_kind = ? AND owner_id = ?
		 ORDER BY updated_at DESC`,
		kind, ownerID)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list all refs by kind owner: %w", err)
	}
	defer rows.Close()
	var out []ObjectRef
	for rows.Next() {
		var ref ObjectRef
		var metadata string
		if scanErr := rows.Scan(&ref.CanonicalID, &ref.ComputerID, &metadata, &ref.UpdatedAt, &ref.Tombstone); scanErr != nil {
			return nil, scanErr
		}
		ref.Metadata = json.RawMessage(metadata)
		out = append(out, ref)
	}
	return out, rows.Err()
}

// JSONBodyFieldRow is extracted JSON body fields plus computer_id. Management
// rewarm indexes pending delivered worker-updates this way so it does not
// materialize blob bodies.
type JSONBodyFieldRow struct {
	CanonicalID string
	ComputerID  string
	Fields      []string
}

// ListJSONBodyFieldsByKindOwner extracts JSON body fields for one kind+owner
// using idx_og_objects_kind_owner. It does not SELECT body.
func (s *SQLStore) ListJSONBodyFieldsByKindOwner(ctx context.Context, kind, ownerID string, jsonPaths []string, limit int) ([]JSONBodyFieldRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("objectgraph sql: owner_id is required")
	}
	if len(jsonPaths) == 0 {
		return nil, fmt.Errorf("objectgraph sql: json path is required")
	}
	query := `SELECT canonical_id, computer_id`
	args := make([]any, 0, len(jsonPaths)+3)
	for _, path := range jsonPaths {
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("objectgraph sql: json path is required")
		}
		query += `, ` + jsonTextSQL(bodyTextSQL, `?`)
		args = append(args, jsonTextArgs(path)...)
	}
	query += ` FROM og_objects WHERE object_kind = ? AND owner_id = ? ORDER BY updated_at DESC LIMIT ?`
	args = append(args, kind, ownerID, NormalizedLimit(limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list json body fields: %w", err)
	}
	defer rows.Close()
	var out []JSONBodyFieldRow
	for rows.Next() {
		var canonicalID, computerID sql.NullString
		nulls := make([]sql.NullString, len(jsonPaths))
		dest := make([]any, 2+len(jsonPaths))
		dest[0] = &canonicalID
		dest[1] = &computerID
		for i := range nulls {
			dest[i+2] = &nulls[i]
		}
		if scanErr := rows.Scan(dest...); scanErr != nil {
			return nil, scanErr
		}
		fields := make([]string, len(jsonPaths))
		for i, ns := range nulls {
			if ns.Valid && ns.String != "null" {
				fields[i] = ns.String
			}
		}
		out = append(out, JSONBodyFieldRow{CanonicalID: canonicalID.String, ComputerID: computerID.String, Fields: fields})
	}
	return out, rows.Err()
}

// GetEdge finds a single non-tombstoned edge by from_id + kind.
func (s *SQLStore) GetEdge(ctx context.Context, fromID string, kind EdgeKind) (Edge, error) {
	if s == nil || s.db == nil {
		return Edge{}, fmt.Errorf("objectgraph sql: nil store")
	}
	return scanEdge(s.db.QueryRowContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = 0
		 LIMIT 1`, fromID, string(kind)))
}

// ListEdgesFrom lists all non-tombstoned edges from a given object ID.
func (s *SQLStore) ListEdgesFrom(ctx context.Context, fromID string) ([]Edge, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND tombstone = 0 ORDER BY created_at`, fromID)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list edges from: %w", err)
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
	return out, rows.Err()
}

// ListEdgesByKind lists all non-tombstoned edges of a given kind from
// a given object.
func (s *SQLStore) ListEdgesByKind(ctx context.Context, fromID string, kind EdgeKind) ([]Edge, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = 0 ORDER BY created_at`,
		fromID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: list edges by kind: %w", err)
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
	return out, rows.Err()
}

// evaluateConditions checks conditions in a read transaction, for a write the
// projection interceptor will make later.
func (s *SQLStore) evaluateConditions(ctx context.Context, conditions []ObjectCondition) error {
	if len(conditions) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("objectgraph sql: begin condition tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	return checkConditions(ctx, tx, conditions)
}

// checkConditions compares each condition with the stored object, in
// canonical-ID order, and returns ErrConflict on the first that fails.
func checkConditions(ctx context.Context, tx *sql.Tx, conditions []ObjectCondition) error {
	orderedConditions := append([]ObjectCondition(nil), conditions...)
	sort.Slice(orderedConditions, func(i, j int) bool {
		return strings.TrimSpace(orderedConditions[i].CanonicalID) < strings.TrimSpace(orderedConditions[j].CanonicalID)
	})
	seen := make(map[string]struct{}, len(orderedConditions))
	for _, condition := range orderedConditions {
		id := strings.TrimSpace(condition.CanonicalID)
		if id == "" {
			return fmt.Errorf("objectgraph sql: empty conditional canonical_id")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("objectgraph sql: duplicate condition %s", id)
		}
		seen[id] = struct{}{}
		var versionID, contentHash string
		err := tx.QueryRowContext(ctx,
			`SELECT version_id, content_hash FROM og_objects WHERE canonical_id = ?`,
			id,
		).Scan(&versionID, &contentHash)
		switch {
		case err == sql.ErrNoRows && !condition.Exists:
			continue
		case err == sql.ErrNoRows:
			return fmt.Errorf("%w: object %s does not exist", ErrConflict, id)
		case err != nil:
			return fmt.Errorf("objectgraph sql: compare object %s: %w", id, err)
		case !condition.Exists:
			return fmt.Errorf("%w: object %s already exists", ErrConflict, id)
		case condition.ExpectedVersionID != "" && versionID != condition.ExpectedVersionID:
			return fmt.Errorf("%w: object %s version is %q, expected %q", ErrConflict, id, versionID, condition.ExpectedVersionID)
		case condition.ExpectedContentHash != "" && contentHash != condition.ExpectedContentHash:
			return fmt.Errorf("%w: object %s content is %q, expected %q", ErrConflict, id, contentHash, condition.ExpectedContentHash)
		}
	}
	return nil
}

func (s *SQLStore) DeleteObject(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph sql: nil store")
	}
	if s.intercept != nil {
		// Read the object, then append the tombstone through the interceptor
		// (a host round-trip) with no connection held.
		obj, err := scanObject(s.db.QueryRowContext(ctx,
			`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id = ?`, id))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			return err
		}
		obj.Tombstone = true
		return s.intercept(ctx, []Object{obj}, nil)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("objectgraph sql: delete object: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM og_objects WHERE canonical_id = ?`, id); err != nil {
		return fmt.Errorf("objectgraph sql: delete object: %w", err)
	}
	s.derived.Drop(ctx, tx, id)
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("objectgraph sql: delete object: commit: %w", err)
	}
	return nil
}

func (s *SQLStore) Close() error {
	// The caller owns the *sql.DB; do not close it here.
	return nil
}

// Compile-time assertions.
var (
	_ Store      = (*SQLStore)(nil)
	_ BatchStore = (*SQLStore)(nil)
)

type rowScanner interface {
	Scan(dest ...any) error
}

func scanObject(row rowScanner) (Object, error) {
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
		return Object{}, fmt.Errorf("objectgraph sql: scan object: %w", err)
	}
	obj.Metadata = json.RawMessage(metadata)
	obj.CreatedAt = obj.CreatedAt.UTC()
	obj.UpdatedAt = obj.UpdatedAt.UTC()
	return obj, nil
}

func scanEdge(row rowScanner) (Edge, error) {
	var edge Edge
	var metadata string
	if err := row.Scan(
		&edge.EdgeID, &edge.FromID, &edge.ToID, &edge.Kind, &metadata,
		&edge.CreatedAt, &edge.Tombstone,
	); err != nil {
		if err == sql.ErrNoRows {
			return Edge{}, ErrNotFound
		}
		return Edge{}, fmt.Errorf("objectgraph sql: scan edge: %w", err)
	}
	edge.Metadata = json.RawMessage(metadata)
	edge.CreatedAt = edge.CreatedAt.UTC()
	return edge, nil
}
