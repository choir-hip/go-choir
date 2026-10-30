package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Retrieval schema for Store B (rungs 1-3 of
// docs/reports/object-graph-retrieval-research-2026-10-14.md; problem record
// docs/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md).
//
// Every step is idempotent (IF NOT EXISTS, or an index-exists check),
// none runs inside Bootstrap: index builds over millions of rows take
// minutes, so corpusd starts serving on the old query shapes and each new
// shape switches on only once its index or table reports live. A step that
// fails leaves the old shape in use and is retried.

// Index and table names. Plan tests assert on them.
const (
	idxObjectsKindOwnerComputerUpdated = "idx_og_objects_kind_owner_computer_updated"
	idxEdgesToKind                     = "idx_og_edges_to_kind"
	// ogLookupKeysTable holds the one metadata value each registered kind
	// is looked up by, one row per object, written in the same transaction
	// as the object. It is a side table rather than an expression index
	// because the registered kinds are small (publications, routes, checks,
	// proposals) and the seek is a primary-key range.
	ogLookupKeysTable = "og_lookup_keys"
	// ogLookupKeyChars bounds the key. A longer value is truncated in the
	// key; the lookup still compares the full JSON value, so truncation
	// narrows the seek and never changes an answer.
	ogLookupKeyChars = 255
)

const ogLookupKeysDDL = `
CREATE TABLE IF NOT EXISTS og_lookup_keys (
	object_kind  TEXT NOT NULL,
	lookup_key   TEXT COLLATE "C" NOT NULL,
	canonical_id TEXT COLLATE "C" NOT NULL,
	PRIMARY KEY (object_kind, lookup_key, canonical_id)
);
CREATE INDEX IF NOT EXISTS idx_og_lookup_keys_object ON og_lookup_keys(canonical_id);
`

// metadataLookupKey is one (kind, JSON path) pair served by the lookup key.
type metadataLookupKey struct {
	Kind string
	// Field is the top-level metadata field; the path is "$."+Field.
	Field string
}

// metadataLookupKeys are the Store B lookups by metadata value (B6, B7, B9
// of the research note's inventory, and the two publish lookups). A kind
// appears once: the key holds one field per kind.
var metadataLookupKeys = []metadataLookupKey{
	{Kind: "choir.public_route", Field: "route_path"},
	{Kind: "choir.publication_check", Field: "publication_version_id"},
	{Kind: "choir.publication_proposal", Field: "proposal_id"},
	{Kind: "choir.publication", Field: "publication_id"},
	{Kind: "choir.publication_version", Field: "publication_version_id"},
	// The Library (S3, review 291 F1 and F2): a package release is found
	// by its id on another owner's read, and the listing versions that
	// admit readers beyond their owner (widened: "true"; no row otherwise)
	// are what another owner's catalog pages through.
	{Kind: "choir.package_release", Field: "release_id"},
	{Kind: "choir.release_listing", Field: "widened"},
	// The entity layer's duplicate check (R1, review 298 F2): a new
	// entity's normalised name against the owner's entity rows.
	{Kind: "choir.entity", Field: "normalized"},
}

// lookupKeyField returns the keyed field of kind, if any.
func lookupKeyField(kind string) (string, bool) {
	for _, k := range metadataLookupKeys {
		if k.Kind == kind {
			return k.Field, true
		}
	}
	return "", false
}

// lookupKeyFor reports whether (kind, jsonPath) is served by the lookup key.
func lookupKeyFor(kind, jsonPath string) bool {
	field, ok := lookupKeyField(kind)
	return ok && field == strings.TrimPrefix(strings.TrimSpace(jsonPath), "$.")
}

// lookupKeyValue is value as the lookup key holds it.
func lookupKeyValue(value string) string {
	if utf8.RuneCountInString(value) <= ogLookupKeyChars {
		return value
	}
	return string([]rune(value)[:ogLookupKeyChars])
}

// objectLookupKey is an object's key, as metadataJSONEq reads the field: a string's text, any other value's JSON
// text. ok is false for an unregistered kind, unreadable metadata, or a
// missing or null field.
func objectLookupKey(kind string, metadata []byte) (string, bool) {
	field, ok := lookupKeyField(kind)
	if !ok {
		return "", false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &fields); err != nil {
		return "", false
	}
	raw, ok := fields[field]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		text = string(raw)
	}
	return lookupKeyValue(text), true
}

// sqlExecer is a *sql.DB or a *sql.Tx.
type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// writeLookupKey replaces an object's key row, in the object's own
// transaction. It is a no-op for an unregistered kind and until the table
// exists.
func (s *Store) writeLookupKey(ctx context.Context, x sqlExecer, obj objectgraph.Object) error {
	if s == nil || !s.retrievalState.lookupKeysReady.Load() {
		return nil
	}
	if _, registered := lookupKeyField(string(obj.ObjectKind)); !registered {
		return nil
	}
	if _, err := x.ExecContext(ctx, `DELETE FROM og_lookup_keys WHERE canonical_id = ?`, obj.CanonicalID); err != nil {
		return fmt.Errorf("clear lookup key: %w", err)
	}
	key, ok := objectLookupKey(string(obj.ObjectKind), obj.Metadata)
	if !ok {
		return nil
	}
	if _, err := x.ExecContext(ctx, `INSERT INTO og_lookup_keys (object_kind, lookup_key, canonical_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
		string(obj.ObjectKind), key, obj.CanonicalID); err != nil {
		return fmt.Errorf("write lookup key: %w", err)
	}
	return nil
}

// retrievalSchemaState records which retrieval shapes are live. Reads are
// lock-free; the migration goroutine sets each flag once its step lands.
type retrievalSchemaState struct {
	// lookupKeysReady: the key table exists (object writes maintain it).
	lookupKeysReady atomic.Bool
	// lookupKeyLive: every registered object has its key row (lookups
	// seek it).
	lookupKeyLive atomic.Bool
	// captureIndexReady: the capture projection tables exist (capture
	// writes maintain them).
	captureIndexReady atomic.Bool
	// captureIndexComplete: every capture in og_objects is projected (the
	// search and URL routes answer).
	captureIndexComplete atomic.Bool
	// captureIndexDirty: an inline projection write failed; the next
	// reconcile pass repairs it.
	captureIndexDirty atomic.Bool
	// captureIndexGap is captures minus projection rows after the last
	// full pass: captures that cannot be projected.
	captureIndexGap atomic.Int64
	// reconcileKick wakes the periodic reconciler after a failed inline
	// projection write.
	reconcileKick chan struct{}

	migrateOnce sync.Once
	reconcileMu sync.Mutex
}

// MigrateRetrieval runs every retrieval schema step once, in order, and
// returns the first error after trying them all. It is safe to call
// repeatedly and concurrently with reads and writes.
func (s *Store) MigrateRetrieval(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("platform store: nil database")
	}
	var firstErr error
	note := func(step string, err error) {
		if err != nil {
			log.Printf("corpusd retrieval migrate step=%s err=%v", step, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("retrieval migrate %s: %w", step, err)
			}
		}
	}
	note("objects_kind_owner_computer_updated", s.ensureCorpusIndex(ctx, "og_objects", idxObjectsKindOwnerComputerUpdated, "object_kind, owner_id, computer_id, updated_at"))
	note("edges_to_kind", s.ensureCorpusIndex(ctx, "og_edges", idxEdgesToKind, "to_id, kind"))
	note("lookup_keys", s.ensureLookupKeys(ctx))
	note("capture_index_tables", s.ensureCaptureIndexTables(ctx))
	if s.retrievalState.captureIndexReady.Load() {
		note("capture_index_reconcile", s.ReconcileCaptureIndex(ctx))
	}
	return firstErr
}

// StartRetrievalMigrations runs MigrateRetrieval in the background beside
// the listener, retrying failed steps with backoff, then reconciles the
// capture projection every interval. It returns at once.
func (s *Store) StartRetrievalMigrations(ctx context.Context, interval time.Duration) {
	if s == nil {
		return
	}
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	s.retrievalState.migrateOnce.Do(func() {
		go func() {
			backoff := 30 * time.Second
			for {
				start := time.Now()
				err := s.MigrateRetrieval(ctx)
				log.Printf("corpusd retrieval migrate done ms=%d err=%v live=%v",
					time.Since(start).Milliseconds(), err, s.RetrievalLive())
				if err == nil {
					break
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				if backoff < 10*time.Minute {
					backoff *= 2
				}
			}
			s.retrievalPeriodic(ctx, interval)
		}()
	})
}

// retrievalPeriodic reconciles the capture projection every interval until
// ctx ends: a projection write that failed, or a projection table lost,
// is repaired without an operator.
func (s *Store) retrievalPeriodic(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.retrievalState.reconcileKick:
		}
		if !s.retrievalState.captureIndexReady.Load() {
			if err := s.ensureCaptureIndexTables(ctx); err != nil {
				log.Printf("corpusd retrieval capture tables err=%v", err)
				continue
			}
		}
		if err := s.ReconcileCaptureIndex(ctx); err != nil {
			log.Printf("corpusd retrieval reconcile err=%v", err)
		}
	}
}

// RetrievalLive reports which retrieval shapes are live (for /health and
// tests).
func (s *Store) RetrievalLive() map[string]bool {
	if s == nil {
		return map[string]bool{}
	}
	return map[string]bool{
		"lookup_keys":            s.retrievalState.lookupKeyLive.Load(),
		"capture_index":          s.retrievalState.captureIndexReady.Load(),
		"capture_index_complete": s.retrievalState.captureIndexComplete.Load(),
	}
}

func validSchemaIdentifier(value string) bool {
	return value != "" && strings.IndexFunc(value, func(r rune) bool {
		return r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) < 0
}

// corpusIndexExists reports whether a Store B table has the named index.
func (s *Store) corpusIndexExists(ctx context.Context, _, index string) (bool, error) {
	return dbx.IndexExists(ctx, s.corpus(), dbx.Postgres, index)
}

// ensureCorpusIndex creates a Store B index when it is absent.
func (s *Store) ensureCorpusIndex(ctx context.Context, table, index, columns string) error {
	if !validSchemaIdentifier(table) || !validSchemaIdentifier(index) {
		return fmt.Errorf("unsupported schema identifier %q/%q", table, index)
	}
	ok, err := s.corpusIndexExists(ctx, table, index)
	if err != nil || ok {
		return err
	}
	start := time.Now()
	if _, err := s.corpus().ExecContext(ctx, fmt.Sprintf("CREATE INDEX %s ON %s(%s)", index, table, columns)); err != nil {
		// A concurrent boot may have created it first.
		if ok, cerr := s.corpusIndexExists(ctx, table, index); cerr == nil && ok {
			return nil
		}
		return err
	}
	log.Printf("corpusd retrieval migrate index=%s table=%s ms=%d", index, table, time.Since(start).Milliseconds())
	return nil
}

// ensureLookupKeys creates the key table, turns on key writes, backfills
// every registered object's key from og_objects (idempotent; the kinds are
// small), and then switches lookups to it.
func (s *Store) ensureLookupKeys(ctx context.Context) error {
	if _, err := s.corpus().ExecContext(ctx, ogLookupKeysDDL); err != nil {
		return err
	}
	s.retrievalState.lookupKeysReady.Store(true)
	start := time.Now()
	written := 0
	for _, k := range metadataLookupKeys {
		n, err := s.backfillLookupKeys(ctx, k.Kind)
		written += n
		if err != nil {
			return err
		}
	}
	if written > 0 {
		log.Printf("corpusd retrieval migrate lookup_keys backfilled=%d ms=%d", written, time.Since(start).Milliseconds())
	}
	s.retrievalState.lookupKeyLive.Store(true)
	return nil
}

// backfillLookupKeys writes the key row of every object of kind whose row
// is missing or stale.
func (s *Store) backfillLookupKeys(ctx context.Context, kind string) (int, error) {
	have := map[string]string{}
	rows, err := s.corpus().QueryContext(ctx, `SELECT canonical_id, lookup_key FROM og_lookup_keys WHERE object_kind = ?`, kind)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			_ = rows.Close()
			return 0, err
		}
		have[id] = key
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	rows, err = s.corpus().QueryContext(ctx, `SELECT canonical_id, metadata FROM og_objects WHERE object_kind = ?`, kind)
	if err != nil {
		return 0, err
	}
	// Missing keys are inserted in multi-row statements (one round trip
	// per lookupKeysPerInsert on a sql-server); a stale or unreadable one
	// is replaced row by row (rare: the keyed fields are ids).
	var missing []any
	var stale []objectgraph.Object
	for rows.Next() {
		obj := objectgraph.Object{ObjectKind: objectgraph.ObjectKind(kind)}
		var metadata string
		if err := rows.Scan(&obj.CanonicalID, &metadata); err != nil {
			_ = rows.Close()
			return 0, err
		}
		obj.Metadata = json.RawMessage(metadata)
		key, ok := objectLookupKey(kind, obj.Metadata)
		current, present := have[obj.CanonicalID]
		switch {
		case ok == present && (!ok || key == current):
		case ok && !present:
			missing = append(missing, kind, key, obj.CanonicalID)
		default:
			stale = append(stale, obj)
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	for start := 0; start < len(missing); start += 3 * lookupKeysPerInsert {
		chunk := missing[start:min(start+3*lookupKeysPerInsert, len(missing))]
		values := strings.TrimSuffix(strings.Repeat("(?, ?, ?), ", len(chunk)/3), ", ")
		if _, err := s.corpus().ExecContext(ctx, `INSERT INTO og_lookup_keys (object_kind, lookup_key, canonical_id) VALUES `+values+` ON CONFLICT DO NOTHING`, chunk...); err != nil {
			return 0, err
		}
	}
	for _, obj := range stale {
		if err := s.writeLookupKey(ctx, s.corpus(), obj); err != nil {
			return 0, err
		}
	}
	return len(missing)/3 + len(stale), nil
}

// lookupKeysPerInsert bounds one backfill statement.
const lookupKeysPerInsert = 500
