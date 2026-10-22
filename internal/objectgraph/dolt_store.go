package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
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
	db        *sql.DB
	intercept MutationInterceptor
	validate  WriteValidator
	// engineMu serializes access to the embedded Dolt engine. Two DoltStore
	// instances over one connector (write + read pools) share one engine,
	// which is not safe for concurrent queries - they race on shared internal
	// buffers (unescapeHTMLCodepoints mutating a JSON slice in place).
	engineMu *engineMutex
	// liveColumns records which generated-column access paths are live
	// (column names, and composite index names for the record paths) so the
	// readers use the index where it exists and JSON_EXTRACT otherwise. Set
	// by discovery (EnsureSchema, DiscoverAccessPaths) and after each step of
	// MigrateAccessPaths; replaced whole, never mutated.
	liveColumns atomic.Pointer[map[string]bool]
	// derived is the state of the og_key_index side table (derived_keys.go),
	// shared by the write and read stores of one engine.
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

func (s *DoltStore) SetMutationInterceptor(fn MutationInterceptor) {
	if s == nil {
		return
	}
	s.intercept = fn
}

func (s *DoltStore) SetWriteValidator(fn WriteValidator) {
	if s == nil {
		return
	}
	s.validate = fn
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
	return &DoltStore{db: db, engineMu: newEngineMutex(), derived: newDerivedKeys()}
}

// ShareEngineMutex points this store at another store's engine mutex so two
// DoltStore instances over one embedded connector serialize all access.
func (s *DoltStore) ShareEngineMutex(other *DoltStore) {
	if other != nil && other.engineMu != nil {
		s.engineMu = other.engineMu
	}
}

// WithEngine runs fn against this store's database while holding the shared
// engine mutex, accounted under op. It serves tables outside the object graph
// that share its embedded engine, such as the store's choir_events, so their
// reads serialize with every object-graph query as the engine requires.
func (s *DoltStore) WithEngine(op string, fn func(db *sql.DB) error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
	return fn(s.db)
}

// EnsureSchema creates the og_objects and og_edges tables if they do
// not already exist. Safe to call multiple times.
func (s *DoltStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	doneDDL := BootPhase("store: open", "objectgraph-schema/schema-ddl")
	_, err := s.db.ExecContext(ctx, SchemaDDL)
	doneDDL(err)
	if err != nil {
		return fmt.Errorf("objectgraph dolt: ensure schema: %w", err)
	}
	// The derived-key side table is created empty: nothing here scales with
	// og_objects. Its rows are written by the writers and checked online
	// after the listener is up (derived_keys.go).
	s.ensureDerivedKeysTable(ctx)
	// Derived columns and indexes (generated columns over metadata and body,
	// and the composite indexes of the hot reads) are added here only to an
	// empty table: with no rows there is nothing to rewrite. On a table with
	// rows, open is discovery only: a STORED column or a new index rewrites
	// or scans every row while it holds the engine
	// (docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md),
	// so the missing ones are reported, the readers keep their JSON paths,
	// and adding them is the explicit MigrateAccessPaths operation.
	doneDiscover := BootPhase("store: open", "objectgraph-schema/discover-access-paths")
	plan, err := s.discoverLocked(ctx)
	doneDiscover(err)
	if err != nil {
		log.Printf("objectgraph dolt: discover derived access paths, JSON paths kept: %v", err)
		return nil
	}
	if !plan.HasRows && len(plan.MissingColumns)+len(plan.MissingIndexes) > 0 {
		for _, step := range s.migrationSteps(plan) {
			doneStep := BootPhase("store: open", "objectgraph-schema/empty-table-"+step.kind+" "+step.name)
			err := s.runStepLocked(ctx, step)
			doneStep(err)
			if err != nil {
				log.Printf("objectgraph dolt: %s %s on an empty table unavailable, JSON path kept: %v", step.kind, step.name, err)
			}
		}
		if plan, err = s.discoverLocked(ctx); err != nil {
			log.Printf("objectgraph dolt: discover derived access paths, JSON paths kept: %v", err)
			return nil
		}
	}
	plan.log()
	return nil
}

// commitmentRecordKind is the object kind of the addressed-record ledger
// (internal/store inbox.go). Its inbox, conversation and claim reads are the
// hot scans the record-scoped generated columns below serve
// (docs/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md).
const commitmentRecordKind = ObjectKind("choir.commitment_record")

// generatedColumnMaxChars bounds the record-scoped text columns. A VARCHAR
// generated column refuses the insert of a longer value, so they hold
// LEFT(value, 255), and a lookup value of 255 characters or more keeps the
// JSON path: below that length, LEFT(x) = v exactly when x = v.
const generatedColumnMaxChars = 255

// metadataIndexColumn is one generated column over a metadata field that an
// equality lookup (ListObjectsByMetadata, ListObjectsByMetadataPage) can use
// instead of evaluating JSON_EXTRACT on every row of the kind.
type metadataIndexColumn struct {
	Field  string // JSON path leaf under metadata
	Column string // generated column name
	// Kind restricts the column to one object kind: the expression yields
	// NULL for every other kind, and lookups of other kinds keep the JSON
	// path. Empty means every kind.
	Kind ObjectKind
	// MaxChars is the LEFT() bound of the column; 0 means the column holds
	// the whole value (a longer value refuses the insert, so none exist).
	MaxChars int
	// KeysetIndex marks an equality index of (object_kind, column,
	// canonical_id): a canonical-ID keyset page is then read in index order
	// with no sort. The legacy columns are indexed (object_kind, column,
	// updated_at) for the newest-first lookups.
	KeysetIndex bool
}

// metadataIndexColumns maps hot metadata fields to generated columns so
// lookups resolve as index seeks. The first three (measured under drain:
// JSON_EXTRACT full scans at ~1.4s/call) cover every kind. All are STORED:
// with an indexed VIRTUAL generated column, the embedded engine panics on
// INSERT ... ON DUPLICATE KEY UPDATE (index out of range in the secondary
// index writer; every og_objects write is that upsert). Column names are
// stable identifiers, not part of any wire protocol.
var metadataIndexColumns = []metadataIndexColumn{
	{Field: "run_id", Column: "og_meta_run_id"},
	{Field: "trajectory_id", Column: "og_meta_trajectory_id"},
	{Field: "channel_id", Column: "og_meta_channel_id"},
	{Field: "inbox", Column: "og_rec_inbox", Kind: commitmentRecordKind, MaxChars: generatedColumnMaxChars, KeysetIndex: true},
	{Field: "sender_inbox", Column: "og_rec_sender_inbox", Kind: commitmentRecordKind, MaxChars: generatedColumnMaxChars, KeysetIndex: true},
	{Field: "conversation_id", Column: "og_rec_conversation_id", Kind: commitmentRecordKind, MaxChars: generatedColumnMaxChars, KeysetIndex: true},
}

// Record-scoped access paths that are not single-field equality lookups.
// Each is live only when its columns and its index exist.
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

// recordClaimPredicate is the body predicate the claim pack filtered with
// before the column existed: ListObjectsByOwnerAndBody with "$.directive"
// missing or empty. The column holds exactly it.
const recordClaimPredicate = `(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.directive') IS NULL OR JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.directive')) = '')`

// recordColumns are the record-scoped columns beyond the equality lookups.
// The claim column guards with JSON_VALID: a column over CAST(body AS JSON)
// would refuse the insert of a record whose body is not JSON (the unindexed
// predicate failed the whole query on such a row instead).
var recordColumns = []generatedColumn{
	{Name: recordInboxSeqColumn, Def: fmt.Sprintf(
		`BIGINT GENERATED ALWAYS AS (CASE WHEN object_kind = '%s' THEN CAST(JSON_EXTRACT(CAST(metadata AS JSON), '$.inbox_seq') AS SIGNED) END) STORED`,
		string(commitmentRecordKind))},
	{Name: recordClaimColumn, Def: fmt.Sprintf(
		`BOOLEAN GENERATED ALWAYS AS (CASE WHEN object_kind = '%s' AND (body IS NULL OR JSON_VALID(CAST(body AS CHAR))) THEN %s END) STORED`,
		string(commitmentRecordKind), recordClaimPredicate)},
}

// generatedColumn is one STORED generated column of og_objects: its name
// and its definition after the name.
type generatedColumn struct {
	Name, Def string
}

// generatedColumn returns the column definition of a metadata column.
func (mc metadataIndexColumn) generatedColumn() generatedColumn {
	extract := fmt.Sprintf(`JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.%s'))`, mc.Field)
	if mc.MaxChars > 0 {
		extract = fmt.Sprintf(`LEFT(%s, %d)`, extract, mc.MaxChars)
	}
	if mc.Kind != "" {
		extract = fmt.Sprintf(`CASE WHEN object_kind = '%s' THEN %s END`, string(mc.Kind), extract)
	}
	return generatedColumn{Name: mc.Column, Def: fmt.Sprintf(`VARCHAR(255) GENERATED ALWAYS AS (%s) STORED`, extract)}
}

func (mc metadataIndexColumn) indexName() string { return "idx_og_objects_" + mc.Column }

func (mc metadataIndexColumn) indexColumns() string {
	if mc.KeysetIndex {
		return "object_kind, " + mc.Column + ", canonical_id"
	}
	return "object_kind, " + mc.Column + ", updated_at"
}

// derivedIndex is one index of og_objects beyond the base table's own. An
// index that gates a reader names it in LiveName (a column name for the
// metadata lookups, the index name for the record paths): that path is
// live only when the index and every column in Needs exist.
type derivedIndex struct {
	Name, Columns string
	Needs         []string
	LiveName      string
}

// baseIndexes serve the hot read shapes measured under drain
// (texture-latency-layered-evidence-2026-10-01): snapshot phase 1 filters
// on (owner_id, computer_id[, object_kind]); ListObjects filters on
// object_kind + owner_id + computer_id. The planner uses them unasked, so
// they gate no reader.
var baseIndexes = []derivedIndex{
	{Name: "idx_og_objects_owner_computer_kind", Columns: "owner_id, computer_id, object_kind"},
	{Name: "idx_og_objects_kind_owner_computer", Columns: "object_kind, owner_id, computer_id"},
}

// derivedColumns are the generated columns of og_objects, in the order a
// migration adds them.
func derivedColumns() []generatedColumn {
	columns := make([]generatedColumn, 0, len(metadataIndexColumns)+len(recordColumns))
	for _, mc := range metadataIndexColumns {
		columns = append(columns, mc.generatedColumn())
	}
	return append(columns, recordColumns...)
}

// derivedIndexes are the indexes beyond the base table's, in the order a
// migration builds them.
func derivedIndexes() []derivedIndex {
	indexes := append([]derivedIndex(nil), baseIndexes...)
	for _, mc := range metadataIndexColumns {
		indexes = append(indexes, derivedIndex{Name: mc.indexName(), Columns: mc.indexColumns(), Needs: []string{mc.Column}, LiveName: mc.Column})
	}
	return append(indexes,
		derivedIndex{Name: recordInboxSeqIndex, Columns: "object_kind, og_rec_inbox, " + recordInboxSeqColumn,
			Needs: []string{"og_rec_inbox", recordInboxSeqColumn}, LiveName: recordInboxSeqIndex},
		derivedIndex{Name: recordClaimIndex, Columns: "object_kind, owner_id, " + recordClaimColumn + ", updated_at, canonical_id",
			Needs: []string{recordClaimColumn}, LiveName: recordClaimIndex},
	)
}

// AccessPathPlan is what discovery found on og_objects: whether it has rows,
// which derived columns and indexes are missing, and which reader paths are
// live (the rest read through JSON_EXTRACT).
type AccessPathPlan struct {
	HasRows        bool     `json:"has_rows"`
	MissingColumns []string `json:"missing_columns"`
	MissingIndexes []string `json:"missing_indexes"`
	Live           []string `json:"live"`
}

func (p AccessPathPlan) log() {
	if len(p.MissingColumns)+len(p.MissingIndexes) == 0 {
		log.Printf("objectgraph dolt: derived access paths complete (live: %v)", p.Live)
		return
	}
	log.Printf("objectgraph dolt: derived access paths: %d columns and %d indexes missing on a table with rows=%t, not added at open (columns %v, indexes %v; live: %v); those reads stay on JSON until POST /internal/store/derived-columns?action=migrate",
		len(p.MissingColumns), len(p.MissingIndexes), p.HasRows, p.MissingColumns, p.MissingIndexes, p.Live)
}

// AccessPathStep is one finished step of MigrateAccessPaths.
type AccessPathStep struct {
	Index      int    `json:"index"` // 1-based
	Total      int    `json:"total"`
	Kind       string `json:"kind"` // column or index
	Name       string `json:"name"`
	DurationMS int64  `json:"duration_ms"`
	Err        string `json:"error,omitempty"`
}

type migrationStep struct {
	kind, name, ddl string
}

// DiscoverAccessPaths reads which derived columns and indexes og_objects
// has and replaces the live set from it. It runs no DDL and reads only
// information_schema and one row of og_objects.
func (s *DoltStore) DiscoverAccessPaths(ctx context.Context) (AccessPathPlan, error) {
	if s == nil || s.db == nil {
		return AccessPathPlan{}, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	return s.discoverLocked(ctx)
}

// discoverLocked is DiscoverAccessPaths with engineMu held. On an error the
// live set is left as it was.
func (s *DoltStore) discoverLocked(ctx context.Context) (AccessPathPlan, error) {
	var plan AccessPathPlan
	var anyRow int
	switch err := s.db.QueryRowContext(ctx, `SELECT 1 FROM og_objects LIMIT 1`).Scan(&anyRow); {
	case err == nil:
		plan.HasRows = true
	case !errors.Is(err, sql.ErrNoRows):
		return plan, fmt.Errorf("objectgraph dolt: probe og_objects rows: %w", err)
	}
	columns, err := s.listNames(ctx, `SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'og_objects'`)
	if err != nil {
		return plan, fmt.Errorf("objectgraph dolt: list og_objects columns: %w", err)
	}
	indexes, err := s.listNames(ctx, `SELECT DISTINCT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'og_objects'`)
	if err != nil {
		return plan, fmt.Errorf("objectgraph dolt: list og_objects indexes: %w", err)
	}
	for _, column := range derivedColumns() {
		if !columns[column.Name] {
			plan.MissingColumns = append(plan.MissingColumns, column.Name)
		}
	}
	live := map[string]bool{}
	for _, index := range derivedIndexes() {
		if !indexes[index.Name] {
			plan.MissingIndexes = append(plan.MissingIndexes, index.Name)
			continue
		}
		if index.LiveName == "" {
			continue
		}
		complete := true
		for _, column := range index.Needs {
			complete = complete && columns[column]
		}
		if complete {
			live[index.LiveName] = true
		}
	}
	s.liveColumns.Store(&live)
	plan.Live = s.liveNames()
	return plan, nil
}

func (s *DoltStore) listNames(ctx context.Context, query string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names[strings.ToLower(name)] = true
	}
	return names, rows.Err()
}

// migrationSteps are the DDL statements that add what plan is missing: the
// columns first, one per step, then the indexes.
func (s *DoltStore) migrationSteps(plan AccessPathPlan) []migrationStep {
	missingColumn := map[string]bool{}
	for _, name := range plan.MissingColumns {
		missingColumn[name] = true
	}
	missingIndex := map[string]bool{}
	for _, name := range plan.MissingIndexes {
		missingIndex[name] = true
	}
	var steps []migrationStep
	for _, column := range derivedColumns() {
		if missingColumn[column.Name] {
			steps = append(steps, migrationStep{kind: "column", name: column.Name,
				ddl: `ALTER TABLE og_objects ADD COLUMN ` + column.Name + ` ` + column.Def})
		}
	}
	for _, index := range derivedIndexes() {
		if missingIndex[index.Name] {
			steps = append(steps, migrationStep{kind: "index", name: index.Name,
				ddl: fmt.Sprintf(`CREATE INDEX %s ON og_objects(%s)`, index.Name, index.Columns)})
		}
	}
	return steps
}

func (s *DoltStore) runStepLocked(ctx context.Context, step migrationStep) error {
	_, err := s.db.ExecContext(ctx, step.ddl)
	return err
}

// MigrateAccessPaths adds the derived columns and indexes og_objects is
// missing. It is the explicit operation behind
// /internal/store/derived-columns, never a boot step: on a table with rows
// each column is a rewrite of every row and each index a full pass, about
// 50 us per row per step (docs/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md).
// It runs one statement per step and holds the engine only for that
// statement, so the computer's reads and writes run between steps; after
// each step the live set is rediscovered (progress, if set, is called
// then, outside the engine lock: a shared-engine read store learns it
// through InheritMetadataColumns). A step that fails is logged and
// skipped; its paths stay on JSON. A cancelled ctx stops it before the
// next step; what was added stays, and nothing resumes it at boot.
// og_objects is a projection, so no tape event is written, and the columns
// are additive generated columns no older binary writes, so the store
// schema epoch does not change and a rollback finds them present.
func (s *DoltStore) MigrateAccessPaths(ctx context.Context, progress func(AccessPathStep)) (AccessPathPlan, error) {
	plan, err := s.DiscoverAccessPaths(ctx)
	if err != nil {
		return plan, err
	}
	steps := s.migrationSteps(plan)
	if len(steps) == 0 {
		log.Printf("objectgraph dolt: derived access path migration: nothing missing")
		return plan, nil
	}
	log.Printf("objectgraph dolt: derived access path migration: %d steps on a table with rows=%t (columns %v, indexes %v)",
		len(steps), plan.HasRows, plan.MissingColumns, plan.MissingIndexes)
	begin := time.Now()
	var failed []string
	for i, step := range steps {
		if err := ctx.Err(); err != nil {
			log.Printf("objectgraph dolt: derived access path migration cancelled before step %d/%d after %s",
				i+1, len(steps), time.Since(begin).Round(time.Millisecond))
			return plan, err
		}
		log.Printf("objectgraph dolt: derived access path step %d/%d: %s %s starting", i+1, len(steps), step.kind, step.name)
		start := time.Now()
		s.engineMu.LockAs("(*DoltStore).MigrateAccessPaths")
		stepErr := s.runStepLocked(ctx, step)
		if next, err := s.discoverLocked(ctx); err == nil {
			plan = next
		}
		s.engineMu.UnlockAs("(*DoltStore).MigrateAccessPaths")
		done := AccessPathStep{Index: i + 1, Total: len(steps), Kind: step.kind, Name: step.name, DurationMS: time.Since(start).Milliseconds()}
		if stepErr != nil {
			done.Err = stepErr.Error()
			failed = append(failed, step.name)
			log.Printf("objectgraph dolt: derived access path step %d/%d: %s %s failed after %s, JSON path kept: %v",
				i+1, len(steps), step.kind, step.name, time.Since(start).Round(time.Millisecond), stepErr)
		} else {
			log.Printf("objectgraph dolt: derived access path step %d/%d: %s %s done in %s",
				i+1, len(steps), step.kind, step.name, time.Since(start).Round(time.Millisecond))
		}
		if progress != nil {
			progress(done)
		}
	}
	log.Printf("objectgraph dolt: derived access path migration finished in %s, %d of %d steps failed (live: %v)",
		time.Since(begin).Round(time.Millisecond), len(failed), len(steps), plan.Live)
	if len(failed) > 0 {
		return plan, fmt.Errorf("objectgraph dolt: derived access path steps failed: %v", failed)
	}
	return plan, nil
}

// live reports whether a generated-column access path (a column or a
// composite index name) is usable. The set is replaced whole, never
// mutated, so readers need no lock.
func (s *DoltStore) live(name string) bool {
	set := s.liveColumns.Load()
	return set != nil && (*set)[name]
}

// LiveAccessPaths names the live generated-column access paths, sorted.
func (s *DoltStore) LiveAccessPaths() []string { return s.liveNames() }

func (s *DoltStore) liveNames() []string {
	set := s.liveColumns.Load()
	if set == nil {
		return nil
	}
	names := make([]string, 0, len(*set))
	for name := range *set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// accessPathOp names a metadata read for the engine mutex counters: the
// method, the kind, the field, and whether it reads an index or JSON. The
// value is not part of the name.
func (s *DoltStore) accessPathOp(method, kind, jsonPath, value string) string {
	path := "json"
	if _, ok := s.metadataColumnFor(kind, jsonPath, value); ok {
		path = "index"
	} else if s.derivedMetadataRead(kind, jsonPath, value) != nil {
		path = "keyindex"
	}
	return "(*DoltStore)." + method + " " + kind + " " + jsonPath + " " + path
}

// metadataColumnFor returns the live generated column that answers
// metadata.<jsonPath> = value for kind, if any. The column name comes from
// the fixed metadataIndexColumns table, never from the caller.
func (s *DoltStore) metadataColumnFor(kind, jsonPath, value string) (metadataIndexColumn, bool) {
	leaf, ok := strings.CutPrefix(jsonPath, "$.")
	if !ok {
		return metadataIndexColumn{}, false
	}
	for _, mc := range metadataIndexColumns {
		if mc.Field != leaf || !s.live(mc.Column) {
			continue
		}
		if mc.Kind != "" && string(mc.Kind) != kind {
			continue
		}
		if mc.MaxChars > 0 && utf8.RuneCountInString(value) >= mc.MaxChars {
			continue
		}
		return mc, true
	}
	return metadataIndexColumn{}, false
}

// InheritMetadataColumns copies the live generated-column set from the store
// that ran EnsureSchema to a shared-engine read store. The read store shares
// the same underlying og_objects table, so the columns exist for it — it just
// never ran the DDL to learn which ones.
func (s *DoltStore) InheritMetadataColumns(other *DoltStore) {
	if s == nil || other == nil {
		return
	}
	s.liveColumns.Store(other.liveColumns.Load())
}

// PreflightWrite runs the refusals PutObject and PutBatch apply before they
// hand a mutation to the projection interceptor: the write gate (an
// unregistered kind) and the configured write validator. A caller that holds a
// mutation back and appends it later (the telemetry sealer) calls it at the
// moment the write would have been made, so a refusal still reaches the caller
// that issued the write.
func (s *DoltStore) PreflightWrite(ctx context.Context, objects []Object, edges []Edge) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	if err := CheckWriteAs(ctx, objects, edges); err != nil {
		return err
	}
	if s.validate != nil {
		return s.validate(ctx, objects, edges)
	}
	return nil
}

func (s *DoltStore) PutObject(ctx context.Context, obj Object) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	if err := CheckWriteAs(ctx, []Object{obj}, nil); err != nil {
		return err
	}
	// validate + intercept run OUTSIDE engineMu: intercept is the projection
	// append path (network CAS + event write) and must not hold the engine
	// mutex across a host round-trip. Only the direct-SQL engine write below
	// needs serialization.
	if s.validate != nil {
		if err := s.validate(ctx, []Object{obj}, nil); err != nil {
			return err
		}
	}
	if s.intercept != nil {
		return s.intercept(ctx, []Object{obj}, nil)
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	if s.derived.keyed(obj.ObjectKind) {
		// A kind with derived keys is written with them in one transaction.
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("objectgraph dolt: put object: begin: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, upsertObjectSQL, upsertObjectArgs(obj)...); err != nil {
			return fmt.Errorf("objectgraph dolt: put object: %w", err)
		}
		s.derived.Maintain(ctx, tx, obj.CanonicalID, obj.ObjectKind)
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("objectgraph dolt: put object: commit: %w", err)
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, upsertObjectSQL, upsertObjectArgs(obj)...)
	if err != nil {
		return fmt.Errorf("objectgraph dolt: put object: %w", err)
	}
	return nil
}

// upsertObjectSQL is the og_objects upsert every direct writer shares.
const upsertObjectSQL = `INSERT INTO og_objects
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
			superseded_by = VALUES(superseded_by)`

func upsertObjectArgs(obj Object) []any {
	return []any{obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
		obj.VersionID, obj.ContentHash, obj.Body, string(obj.Metadata),
		obj.CreatedAt.UTC(), obj.UpdatedAt.UTC(), obj.Tombstone,
		obj.SupersededBy}
}

func (s *DoltStore) GetObject(ctx context.Context, id string) (Object, error) {
	if s == nil || s.db == nil {
		return Object{}, fmt.Errorf("objectgraph dolt: nil store")
	}
	op := "(*DoltStore).GetObject " + kindOpLabel(id)
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
	return scanDoltObject(s.db.QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id = ?`, id))
}

// kindOpLabel names an object's kind in an engine-mutex op name, from a
// canonical id (obj:<kind>:<owner>:<suffix>) or a kind. Only a registered
// kind is named, so the label set is bounded by code and not by what a
// caller asks for; anything else is "unknown".
func kindOpLabel(idOrKind string) string {
	kind := idOrKind
	if rest, ok := strings.CutPrefix(idOrKind, "obj:"); ok {
		kind, _, _ = strings.Cut(rest, ":")
	}
	if _, err := writeRegistry.LookupKind(ObjectKind(kind)); err != nil {
		return "unknown"
	}
	return kind
}

func (s *DoltStore) ListObjects(ctx context.Context, filter ListFilter) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	// Named by kind, so a stall's holder log and the engine-mutex counters say
	// which kind's list held the engine (an unnamed op hid it on the paper,
	// docs/problems/actor-wake-outbox-list-full-scan-2026-10-23.md).
	op := "(*DoltStore).ListObjects " + kindOpLabel(string(filter.Kind))
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
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

// ReadObjectSnapshot returns the complete owner/computer object scope from one
// serializable read transaction. Callers filter the immutable result in memory
// so state and its event watermark cannot come from different commits.
func (s *DoltStore) ReadObjectSnapshot(ctx context.Context, ownerID, computerID string) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("objectgraph dolt: snapshot owner_id and computer_id are required")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: begin snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		FROM og_objects WHERE owner_id = ? AND computer_id = ? ORDER BY canonical_id`, ownerID, computerID)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: query snapshot: %w", err)
	}
	defer rows.Close()
	var objects []Object
	for rows.Next() {
		obj, scanErr := scanDoltObject(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		objects = append(objects, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("objectgraph dolt: iterate snapshot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("objectgraph dolt: snapshot commit: %w", err)
	}
	return objects, nil
}

// ReadObjectSnapshotFiltered is the scoped variant of ReadObjectSnapshot for
// hot paths that need only a subset of the computer's objects. It runs both
// phases inside one serializable read-only transaction so the result is
// consistent: phase one scans ref columns (no body LONGBLOB materialization)
// for the requested kinds, applies keep(kind, metadata) to decide membership,
// and phase two fetches full rows by primary key for survivors only.
// Returned objects are sorted by canonical_id, matching ReadObjectSnapshot.
// snapshotFetchBatch bounds each phase-2 IN list in ReadObjectSnapshotFiltered.
const snapshotFetchBatch = 256

func (s *DoltStore) ReadObjectSnapshotFiltered(ctx context.Context, ownerID, computerID string, kinds []ObjectKind, keep func(kind ObjectKind, metadata json.RawMessage) bool) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("objectgraph dolt: snapshot owner_id and computer_id are required")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: begin snapshot tx: %w", err)
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
		return nil, fmt.Errorf("objectgraph dolt: snapshot refs: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id, kindStr, metadata string
		if err := rows.Scan(&id, &kindStr, &metadata); err != nil {
			rows.Close()
			return nil, fmt.Errorf("objectgraph dolt: scan ref: %w", err)
		}
		if keep == nil || keep(ObjectKind(kindStr), json.RawMessage(metadata)) {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("objectgraph dolt: iterate snapshot refs: %w", err)
	}
	rows.Close()
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("objectgraph dolt: snapshot commit: %w", err)
		}
		return nil, nil
	}

	// Phase 2: PK-fetch bodies for survivors, preserving canonical_id order.
	objects, err := snapshotFetchByID(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("objectgraph dolt: snapshot commit: %w", err)
	}
	return objects, nil
}

// trajectoryRefsSQL is one kind's refs for one trajectory, through the
// (object_kind, og_meta_trajectory_id, updated_at) index. Owner and computer
// are checked on the rows, not in the predicate: given them, the planner
// takes the (object_kind, owner_id, computer_id) index and reads every row
// of the kind, the scan this replaces.
func (s *DoltStore) trajectoryRefsSQL(kind ObjectKind, trajectoryID string) (string, []any) {
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
// ok is false, with nothing read, when the trajectory column is not live or
// trajectoryID is empty; the caller then reads with ReadObjectSnapshotFiltered.
// The filtered scan reads the metadata of every object of its kinds in the
// computer: 416 ms per call on the paper (docs/problems/
// research-more-slow-and-hidden-2026-10-21.md, E6).
func (s *DoltStore) ReadTrajectorySnapshot(ctx context.Context, ownerID, computerID, trajectoryID string, kinds []ObjectKind, keep func(kind ObjectKind, metadata json.RawMessage) bool, follow func(have []Object) []string) ([]Object, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("objectgraph dolt: nil store")
	}
	ownerID, computerID, trajectoryID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(trajectoryID)
	if ownerID == "" || computerID == "" {
		return nil, false, fmt.Errorf("objectgraph dolt: snapshot owner_id and computer_id are required")
	}
	if trajectoryID == "" || !s.live("og_meta_trajectory_id") {
		return nil, false, nil
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true})
	if err != nil {
		return nil, false, fmt.Errorf("objectgraph dolt: begin trajectory snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var ids []string
	for _, kind := range kinds {
		query, args := s.trajectoryRefsSQL(kind, trajectoryID)
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, false, fmt.Errorf("objectgraph dolt: trajectory refs: %w", err)
		}
		for rows.Next() {
			var id, kindStr, owner, computer, metadata string
			if err := rows.Scan(&id, &kindStr, &owner, &computer, &metadata); err != nil {
				rows.Close()
				return nil, false, fmt.Errorf("objectgraph dolt: scan trajectory ref: %w", err)
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
			return nil, false, fmt.Errorf("objectgraph dolt: iterate trajectory refs: %w", err)
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
		return nil, false, fmt.Errorf("objectgraph dolt: trajectory snapshot commit: %w", err)
	}
	return objects, true, nil
}

// snapshotFetchByID fetches objects by canonical id inside tx, in id order.
// Fixed-size IN batches: go-mysql-server's range-overlap check is quadratic
// in the IN list, and one list of every survivor cost 42% of guest CPU on a
// long-lived computer (docs/problems/texture-list-cold-latency-2026-10-09.md).
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
			return nil, fmt.Errorf("objectgraph dolt: snapshot bodies: %w", err)
		}
		for objRows.Next() {
			obj, scanErr := scanDoltObject(objRows)
			if scanErr != nil {
				objRows.Close()
				return nil, scanErr
			}
			objects = append(objects, obj)
		}
		if err := objRows.Err(); err != nil {
			objRows.Close()
			return nil, fmt.Errorf("objectgraph dolt: iterate snapshot bodies: %w", err)
		}
		objRows.Close()
	}
	return objects, nil
}

func (s *DoltStore) PutEdge(ctx context.Context, edge Edge) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	if err := CheckWrite(nil, []Edge{edge}); err != nil {
		return err
	}
	// intercept is the projection append path — do not hold engineMu across it.
	if s.intercept != nil {
		return s.intercept(ctx, nil, []Edge{edge})
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
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
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
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
	return s.putBatch(ctx, nil, batch)
}

// PutBatchConditional compares object versions and writes the batch in the
// same transaction. Conditions are locked in caller-supplied deterministic
// order; lifecycle reducers sort them by canonical ID before calling.
func (s *DoltStore) PutBatchConditional(ctx context.Context, conditions []ObjectCondition, batch Batch) error {
	return s.putBatch(ctx, conditions, batch)
}
func (s *DoltStore) putBatch(ctx context.Context, conditions []ObjectCondition, batch Batch) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	if err := CheckWriteAs(ctx, batch.Objects, batch.Edges); err != nil {
		return err
	}
	// validate + intercept run OUTSIDE engineMu. intercept is the projection
	// append path (network CAS + event write); holding the engine mutex across
	// a host round-trip stalls every OG access and can deadlock boot. Only
	// evaluateConditions + the direct-SQL batch tx below need serialization.
	if s.validate != nil {
		if err := s.validate(ctx, batch.Objects, batch.Edges); err != nil {
			return err
		}
	}
	if s.intercept != nil {
		s.engineMu.Lock()
		conditionsErr := s.evaluateConditions(ctx, conditions)
		s.engineMu.Unlock()
		if conditionsErr != nil {
			return conditionsErr
		}
		return s.intercept(ctx, batch.Objects, batch.Edges)
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	if err := s.evaluateConditions(ctx, conditions); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("objectgraph dolt: begin batch tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	orderedConditions := append([]ObjectCondition(nil), conditions...)
	sort.Slice(orderedConditions, func(i, j int) bool {
		return strings.TrimSpace(orderedConditions[i].CanonicalID) < strings.TrimSpace(orderedConditions[j].CanonicalID)
	})
	seen := make(map[string]struct{}, len(orderedConditions))
	for _, condition := range orderedConditions {
		id := strings.TrimSpace(condition.CanonicalID)
		if id == "" {
			return fmt.Errorf("objectgraph dolt: empty conditional canonical_id")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("objectgraph dolt: duplicate condition %s", id)
		}
		seen[id] = struct{}{}

		var versionID, contentHash string
		err := tx.QueryRowContext(ctx,
			`SELECT version_id, content_hash FROM og_objects WHERE canonical_id = ? FOR UPDATE`,
			id,
		).Scan(&versionID, &contentHash)
		switch {
		case err == sql.ErrNoRows && !condition.Exists:
			continue
		case err == sql.ErrNoRows:
			return fmt.Errorf("%w: object %s does not exist", ErrConflict, id)
		case err != nil:
			return fmt.Errorf("objectgraph dolt: compare object %s: %w", id, err)
		case !condition.Exists:
			return fmt.Errorf("%w: object %s already exists", ErrConflict, id)
		case condition.ExpectedVersionID != "" && versionID != condition.ExpectedVersionID:
			return fmt.Errorf("%w: object %s version is %q, expected %q", ErrConflict, id, versionID, condition.ExpectedVersionID)
		case condition.ExpectedContentHash != "" && contentHash != condition.ExpectedContentHash:
			return fmt.Errorf("%w: object %s content is %q, expected %q", ErrConflict, id, contentHash, condition.ExpectedContentHash)
		}
	}

	if err := putBatchTx(ctx, tx, batch, s.derived); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("objectgraph dolt: batch commit: %w", err)
	}
	return nil
}

func putBatchTx(ctx context.Context, tx *sql.Tx, batch Batch, derived *DerivedKeys) error {
	for _, obj := range batch.Objects {
		if _, err := tx.ExecContext(ctx, upsertObjectSQL, upsertObjectArgs(obj)...); err != nil {
			return fmt.Errorf("objectgraph dolt: batch put object %s: %w", obj.CanonicalID, err)
		}
		derived.Maintain(ctx, tx, obj.CanonicalID, obj.ObjectKind)
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
	return nil
}

// GetObjectByMetadata finds a single object by kind + a metadata JSON
// path equality check using JSON_EXTRACT.
func (s *DoltStore) GetObjectByMetadata(ctx context.Context, kind, jsonPath, value string) (Object, error) {
	if s == nil || s.db == nil {
		return Object{}, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	return scanDoltObject(s.db.QueryRowContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?
		 LIMIT 1`, kind, jsonPath, value))
}

const objectColumnsSQL = `canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by`

// metadataListSQL is ListObjectsByMetadata's query: the indexed generated
// column when one is live for (kind, field, value), JSON_EXTRACT otherwise.
// The plan-shape tests EXPLAIN exactly this text.
func (s *DoltStore) metadataListSQL(kind, jsonPath, value string, limit int) (string, []any) {
	if mc, ok := s.metadataColumnFor(kind, jsonPath, value); ok {
		return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND ` + mc.Column + ` = ?
		 ORDER BY updated_at DESC LIMIT ?`, []any{kind, value, NormalizedLimit(limit)}
	}
	return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?
		 ORDER BY updated_at DESC LIMIT ?`, []any{kind, jsonPath, value, NormalizedLimit(limit)}
}

// metadataPageSQL is ListObjectsByMetadataPage's query. With a keyset index
// (object_kind, column, canonical_id) the page orders by every index column:
// object_kind and the column are fixed by equality, so the order is
// canonical_id's, and the engine reads the page in index order with no sort
// (ordering by canonical_id alone makes it a TopN over the whole remaining
// range). A legacy column, indexed for newest-first, still seeks the range.
func (s *DoltStore) metadataPageSQL(kind, jsonPath, value, afterCanonicalID string, limit int) (string, []any) {
	if mc, ok := s.metadataColumnFor(kind, jsonPath, value); ok {
		order := `canonical_id ASC`
		if mc.KeysetIndex {
			order = `object_kind ASC, ` + mc.Column + ` ASC, canonical_id ASC`
		}
		return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND ` + mc.Column + ` = ? AND canonical_id > ?
		 ORDER BY ` + order + ` LIMIT ?`, []any{kind, value, afterCanonicalID, NormalizedLimit(limit)}
	}
	return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ? AND canonical_id > ?
		 ORDER BY canonical_id ASC LIMIT ?`, []any{kind, jsonPath, value, afterCanonicalID, NormalizedLimit(limit)}
}

// ListObjectsByMetadata finds objects by kind + a metadata JSON path
// equality check.
func (s *DoltStore) ListObjectsByMetadata(ctx context.Context, kind, jsonPath, value string, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	op := s.accessPathOp("ListObjectsByMetadata", kind, jsonPath, value)
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
	query, args := s.metadataListSQL(kind, jsonPath, value, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
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

// ListObjectsPage scans one object kind in stable canonical-ID order. The
// exclusive cursor makes full-table maintenance and recovery scans bounded
// without imposing a correctness-limiting fixed window.
func (s *DoltStore) ListObjectsPage(ctx context.Context, kind, afterCanonicalID string, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	op := "(*DoltStore).ListObjectsPage " + kindOpLabel(kind)
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects
		 WHERE object_kind = ? AND canonical_id > ?
		 ORDER BY canonical_id ASC LIMIT ?`,
		kind, afterCanonicalID, NormalizedLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list objects page: %w", err)
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		obj, scanErr := scanDoltObject(rows)
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
func (s *DoltStore) ListObjectsByMetadataPage(ctx context.Context, kind, jsonPath, value, afterCanonicalID string, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	op := s.accessPathOp("ListObjectsByMetadataPage", kind, jsonPath, value)
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
	if !strings.HasPrefix(jsonPath, "$.") {
		return nil, fmt.Errorf("objectgraph dolt: invalid metadata JSON path %q", jsonPath)
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
	query, args := s.metadataPageSQL(kind, jsonPath, value, afterCanonicalID, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list by metadata page: %w", err)
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

// ListObjectsByEdgeTo returns objects of one kind that hold a live edge of
// edgeKind pointing at toID. The join rides the indexed og_edges.to_id lookup
// plus the og_objects primary key — no metadata JSON scan.
func (s *DoltStore) ListObjectsByEdgeTo(ctx context.Context, toID string, edgeKind EdgeKind, objKind string) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	rows, err := s.db.QueryContext(ctx,
		`SELECT o.canonical_id, o.object_kind, o.owner_id, o.computer_id, o.version_id, o.content_hash, o.body, o.metadata, o.created_at, o.updated_at, o.tombstone, o.superseded_by
		 FROM og_objects o
		 JOIN og_edges e ON e.from_id = o.canonical_id
		 WHERE e.to_id = ? AND e.kind = ? AND o.object_kind = ? AND e.tombstone = FALSE
		 ORDER BY o.canonical_id ASC`,
		toID, string(edgeKind), objKind)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list objects by edge to: %w", err)
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		obj, scanErr := scanDoltObject(rows)
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
func (s *DoltStore) CountObjectsByEdgeTo(ctx context.Context, toID string, edgeKind EdgeKind, objKind string) (count int, maxVersion int, err error) {
	if s == nil || s.db == nil {
		return 0, -1, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	maxVersion = -1
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*),
				COALESCE(MAX(CAST(JSON_UNQUOTE(JSON_EXTRACT(o.metadata, '$.version_number')) AS SIGNED)), -1)
		 FROM og_objects o
		 JOIN og_edges e ON e.from_id = o.canonical_id
		 WHERE e.to_id = ? AND e.kind = ? AND o.object_kind = ? AND e.tombstone = FALSE AND o.tombstone = FALSE`,
		toID, string(edgeKind), objKind).Scan(&count, &maxVersion)
	if err != nil {
		return 0, -1, fmt.Errorf("objectgraph dolt: count objects by edge to: %w", err)
	}
	return count, maxVersion, nil
}

// ListObjectsByOwnerAndBody finds objects by kind, owner, and an exact set of
// predicates evaluated against the persisted JSON body. The body is the
// canonical authority for record fields that must not be duplicated into
// independently drifting object metadata.
func (s *DoltStore) ListObjectsByOwnerAndBody(ctx context.Context, kind, ownerID string, matches []JSONFieldMatch, limit int) ([]Object, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	paths := make([]string, 0, len(matches))
	for _, m := range matches {
		paths = append(paths, m.JSONPath)
	}
	op := "(*DoltStore).ListObjectsByOwnerAndBody " + kind + " " + strings.Join(paths, ",")
	fam := s.derivedAddresseeRead(kind, matches)
	if fam != nil {
		op += " keyindex"
	}
	s.engineMu.LockAs(op)
	defer s.engineMu.UnlockAs(op)
	if fam != nil {
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
// of the index instead of every record of the inbox. ok is false when the
// index is not live or the inbox is too long for the column; the caller
// then reads the inbox its unindexed way. Owner, computer and tombstone
// are the caller's filters, as on the metadata lookups.
func (s *DoltStore) ListRecordsByInboxSeq(ctx context.Context, inbox string, afterSeq, throughSeq int64) ([]Object, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("objectgraph dolt: nil store")
	}
	if !s.live(recordInboxSeqIndex) || utf8.RuneCountInString(inbox) >= generatedColumnMaxChars {
		return nil, false, nil
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	query, args := recordsByInboxSeqSQL(inbox, afterSeq, throughSeq)
	objects, err := s.queryObjects(ctx, query, args...)
	if err != nil {
		return nil, true, fmt.Errorf("objectgraph dolt: records by inbox seq: %w", err)
	}
	return objects, true, nil
}

// commitmentClaimSQL is ListCommitmentClaimObjects' query: a reverse walk of
// (object_kind, owner_id, og_rec_claim, updated_at, canonical_id), ordered
// by every index column descending so the engine stops after limit rows.
func commitmentClaimSQL(ownerID string, limit int) (string, []any) {
	return `SELECT ` + objectColumnsSQL + ` FROM og_objects
		 WHERE object_kind = ? AND owner_id = ? AND ` + recordClaimColumn + ` = TRUE
		 ORDER BY object_kind DESC, owner_id DESC, ` + recordClaimColumn + ` DESC, updated_at DESC, canonical_id DESC LIMIT ?`,
		[]any{string(commitmentRecordKind), ownerID, NormalizedLimit(limit)}
}

// ListCommitmentClaimObjects returns the owner's newest commitment records
// whose body has no directive (missing or empty), newest first: the rows
// ListObjectsByOwnerAndBody returns for that predicate, read as a reverse
// index walk instead of a JSON evaluation of every record's body. Rows with
// equal updated_at (second precision) come by canonical_id descending; the
// unindexed read left their order to the engine. ok is false when the
// index is not live.
func (s *DoltStore) ListCommitmentClaimObjects(ctx context.Context, ownerID string, limit int) ([]Object, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("objectgraph dolt: nil store")
	}
	if !s.live(recordClaimIndex) {
		return nil, false, nil
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	query, args := commitmentClaimSQL(ownerID, limit)
	objects, err := s.queryObjects(ctx, query, args...)
	if err != nil {
		return nil, true, fmt.Errorf("objectgraph dolt: commitment claims: %w", err)
	}
	return objects, true, nil
}

// queryObjects runs a full-row object query; the caller holds engineMu.
func (s *DoltStore) queryObjects(ctx context.Context, query string, args ...any) ([]Object, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
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

// ObjectRef is an og_objects row without the body payload. Boot and other
// bounded owner scans use it so JSON_EXTRACT + LONGBLOB materialization of
// every choir.run cannot OOM the guest.
type ObjectRef struct {
	CanonicalID string
	ComputerID  string
	Metadata    json.RawMessage
	UpdatedAt   time.Time
	Tombstone   bool
}

// ListObjectRefsByKindOwner returns the newest objects of one kind for one
// owner using idx_og_objects_kind_owner. It does not JSON_EXTRACT metadata
// and does not load body.
func (s *DoltStore) ListObjectRefsByKindOwner(ctx context.Context, kind, ownerID string, limit int) ([]ObjectRef, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("objectgraph dolt: owner_id is required")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, computer_id, metadata, updated_at, tombstone
		 FROM og_objects
		 WHERE object_kind = ? AND owner_id = ?
		 ORDER BY updated_at DESC LIMIT ?`,
		kind, ownerID, NormalizedLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list refs by kind owner: %w", err)
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
func (s *DoltStore) ListAllObjectRefsByKindOwner(ctx context.Context, kind, ownerID string) ([]ObjectRef, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("objectgraph dolt: owner_id is required")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, computer_id, metadata, updated_at, tombstone
		 FROM og_objects
		 WHERE object_kind = ? AND owner_id = ?
		 ORDER BY updated_at DESC`,
		kind, ownerID)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list all refs by kind owner: %w", err)
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
// materialize LONGBLOB bodies.
type JSONBodyFieldRow struct {
	CanonicalID string
	ComputerID  string
	Fields      []string
}

// ListJSONBodyFieldsByKindOwner extracts JSON body fields for one kind+owner
// using idx_og_objects_kind_owner. It does not SELECT body.
func (s *DoltStore) ListJSONBodyFieldsByKindOwner(ctx context.Context, kind, ownerID string, jsonPaths []string, limit int) ([]JSONBodyFieldRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("objectgraph dolt: owner_id is required")
	}
	if len(jsonPaths) == 0 {
		return nil, fmt.Errorf("objectgraph dolt: json path is required")
	}
	query := `SELECT canonical_id, computer_id`
	args := make([]any, 0, len(jsonPaths)+3)
	for _, path := range jsonPaths {
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("objectgraph dolt: json path is required")
		}
		query += `, JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), ?))`
		args = append(args, path)
	}
	query += ` FROM og_objects WHERE object_kind = ? AND owner_id = ? ORDER BY updated_at DESC LIMIT ?`
	args = append(args, kind, ownerID, NormalizedLimit(limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph dolt: list json body fields: %w", err)
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
func (s *DoltStore) GetEdge(ctx context.Context, fromID string, kind EdgeKind) (Edge, error) {
	if s == nil || s.db == nil {
		return Edge{}, fmt.Errorf("objectgraph dolt: nil store")
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
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
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
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
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
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

func (s *DoltStore) evaluateConditions(ctx context.Context, conditions []ObjectCondition) error {
	if len(conditions) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true})
	if err != nil {
		return fmt.Errorf("objectgraph dolt: begin condition tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	orderedConditions := append([]ObjectCondition(nil), conditions...)
	sort.Slice(orderedConditions, func(i, j int) bool {
		return strings.TrimSpace(orderedConditions[i].CanonicalID) < strings.TrimSpace(orderedConditions[j].CanonicalID)
	})
	seen := make(map[string]struct{}, len(orderedConditions))
	for _, condition := range orderedConditions {
		id := strings.TrimSpace(condition.CanonicalID)
		if id == "" {
			return fmt.Errorf("objectgraph dolt: empty conditional canonical_id")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("objectgraph dolt: duplicate condition %s", id)
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
			return fmt.Errorf("objectgraph dolt: compare object %s: %w", id, err)
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

func (s *DoltStore) DeleteObject(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("objectgraph dolt: nil store")
	}
	if s.intercept != nil {
		// Read the object under engineMu (engine access), then release before
		// the intercept append (network CAS) so the mutex never spans a host
		// round-trip. The GetObject read is inlined to avoid re-entry.
		s.engineMu.Lock()
		obj, err := scanDoltObject(s.db.QueryRowContext(ctx,
			`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE canonical_id = ?`, id))
		s.engineMu.Unlock()
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			return err
		}
		obj.Tombstone = true
		return s.intercept(ctx, []Object{obj}, nil)
	}
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("objectgraph dolt: delete object: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM og_objects WHERE canonical_id = ?`, id); err != nil {
		return fmt.Errorf("objectgraph dolt: delete object: %w", err)
	}
	s.derived.Drop(ctx, tx, id)
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("objectgraph dolt: delete object: commit: %w", err)
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
