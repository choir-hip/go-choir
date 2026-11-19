package platform

// Vectors in pgvector (docs/reference/EMBEDDINGGEMMA.md "The server",
// station V2 of docs/definitions/choir-night-exact-vectors-learning-2026-11-19.md).
//
// Two tables in corpusd's graph database beside the capture projection:
//
//	vec_story_passages  one row per passage of every published story version,
//	                    keyed (story_id, version_id, ordinal)
//	vec_captures        one row per public capture: its title and lead passage
//
// A vector row names the text it came from (text_sha, the SHA-256 of the
// passage text) and the model that made it (the GGUF's SHA-256), and a version
// or capture id never changes its text, so a vector cannot be taken for
// another text's. A vector outlives nothing: the worker's sweep removes the
// rows of a withdrawn story, a deleted or tombstoned capture, and a capture
// that became owner-private.
//
// One writer: corpusd alone writes these tables. The worker (vector_worker.go)
// and the import of the Mac's first pass (vector_import.go) end in the same
// two write functions after the same checks.
//
// Everything here is derived state. The authority is Store B and the text
// blobs; a lost table costs a re-embed and nothing else. The worker's list of
// work is derived at every pass from what has no vector (vector_worker.go);
// nothing is queued, and a restart resumes nothing.
//
// pgvector is created by the host (nix/postgresql.nix, as postgres: the
// extension is not trusted, so the application role cannot create it). The
// check below creates it only where the connecting role may (test clusters).

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
)

const (
	vecStoryTable   = "vec_story_passages"
	vecCaptureTable = "vec_captures"
	vecStoryIndex   = "idx_vsp_hnsw"
	vecCaptureIndex = "idx_vc_hnsw"
	// Advisory lock keys of the two index builds: one process builds an index
	// at a time, and a process that finds the lock taken goes on.
	vecStoryIndexLockKey   = 7330212470811
	vecCaptureIndexLockKey = 7330212470812
)

// The tables. A row with a NULL embedding and a non-empty refused is a passage
// (or a capture's lead) the embedder will not take: it keeps its version or
// capture out of the gap so the model is not asked again, until the model
// (the row's model column) changes; a search ignores it. Created only when
// absent: a CREATE TABLE or INDEX run on every pass would take locks that
// conflict with a concurrent HNSW build.
const vectorTablesDDL = `
CREATE TABLE IF NOT EXISTS vec_story_passages (
	story_id    TEXT NOT NULL,
	version_id  TEXT NOT NULL,
	ordinal     INTEGER NOT NULL,
	text_sha    TEXT NOT NULL,
	day         DATE NOT NULL,
	model       TEXT NOT NULL,
	embedding   halfvec(256),
	refused     TEXT NOT NULL DEFAULT '',
	embedded_on DATE NOT NULL,
	PRIMARY KEY (story_id, version_id, ordinal)
);
CREATE TABLE IF NOT EXISTS vec_captures (
	capture_id  TEXT NOT NULL PRIMARY KEY,
	owner_id    TEXT NOT NULL,
	computer_id TEXT NOT NULL,
	text_sha    TEXT NOT NULL,
	day         DATE NOT NULL,
	model       TEXT NOT NULL,
	embedding   halfvec(256),
	refused     TEXT NOT NULL DEFAULT '',
	embedded_on DATE NOT NULL
);
`

// vectorPlainIndexes are the btrees, each created only when pg_class lacks it.
var vectorPlainIndexes = []struct{ name, ddl string }{
	{"idx_vsp_version", `CREATE INDEX idx_vsp_version ON vec_story_passages(version_id)`},
	{"idx_vsp_day", `CREATE INDEX idx_vsp_day ON vec_story_passages(day)`},
	{"idx_vc_scope_day", `CREATE INDEX idx_vc_scope_day ON vec_captures(owner_id, computer_id, day)`},
	// the per-pass refused counts filter by model and refused
	{"idx_vsp_refused_model", `CREATE INDEX idx_vsp_refused_model ON vec_story_passages(model) WHERE refused <> ''`},
	{"idx_vc_refused_model", `CREATE INDEX idx_vc_refused_model ON vec_captures(model) WHERE refused <> ''`},
}

// vectorState is the vector index's in-memory state: the embedder and the
// counts that coordinate the worker with searches. None of it is durable.
type vectorState struct {
	mu       sync.RWMutex
	embedder embedder.Embedder
	ready    atomic.Bool
	// unsupported is why the installed pgvector is too old ("" when it is
	// not); set by ensureVectorTables, read by the search.
	unsupported atomic.Value
	kick        chan struct{}
	kickOnce    sync.Once
	started     sync.Once
	// remaining is the last pass's count of rows still without a vector.
	remaining atomic.Int64
	importMu  sync.Mutex
	jobs      map[string]*VectorImportJob
	jobOrder  []string
	lastJob   string
	// inboxDeferred maps inbox files that could not be imported now and wait to
	// the time they were last tried; a file not waiting is taken before them, and
	// among them the one tried longest ago. In memory only: a restart forgets it,
	// which only makes every waiter new again.
	inboxDeferred map[string]time.Time
	// inboxShort counts, per inbox file, consecutive imports whose sample fell
	// short of the floor in a draw of only part of the file (countShortDraw).
	inboxShort map[string]int
}

func (v *vectorState) kickChan() chan struct{} {
	v.kickOnce.Do(func() { v.kick = make(chan struct{}, 1) })
	return v.kick
}

// SetEmbedder names the embedder the worker, the search and the import use.
func (s *Store) SetEmbedder(e embedder.Embedder) {
	s.vec.mu.Lock()
	s.vec.embedder = e
	s.vec.mu.Unlock()
}

func (s *Store) embedder() embedder.Embedder {
	s.vec.mu.RLock()
	defer s.vec.mu.RUnlock()
	return s.vec.embedder
}

// KickVectors asks the worker for a pass now (non-blocking).
func (s *Store) KickVectors() {
	select {
	case s.vec.kickChan() <- struct{}{}:
	default:
	}
}

// vecLiteral is a vector as pgvector reads it in text form.
func vecLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'g', 8, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// vectorTablesPresent reports whether both tables exist (a catalog read; no
// lock on them).
func (s *Store) vectorTablesPresent(ctx context.Context) (bool, error) {
	var ok bool
	err := s.corpus().QueryRowContext(ctx, `SELECT to_regclass('vec_story_passages') IS NOT NULL AND to_regclass('vec_captures') IS NOT NULL`).Scan(&ok)
	return ok, err
}

// ensureVectorTables checks pgvector is installed (creating it where the
// role may), creates the tables and plain indexes that are missing and builds
// the HNSW indexes. It is idempotent and safe to call concurrently, and does
// nothing to a table that is complete. A missing extension is an error that
// names the host unit that makes it. It runs DDL, so it is called by the
// worker and the import, never on a search's path.
func (s *Store) ensureVectorTables(ctx context.Context) error {
	db := s.corpus()
	var have bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')`).Scan(&have); err != nil {
		return fmt.Errorf("vectors: read extensions: %w", err)
	}
	if !have {
		if _, err := db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
			return fmt.Errorf("vectors: the pgvector extension is not installed in this database and this role cannot create it (go-choir-postgresql-roles makes it as postgres): %w", err)
		}
	}
	// The code needs iterative index scans (pgvector 0.8.0): below that every
	// search statement would error. Refuse here, once per pass, and say why;
	// the search answers unavailable with the same reason.
	var extver string
	if err := db.QueryRowContext(ctx, `SELECT extversion FROM pg_extension WHERE extname = 'vector'`).Scan(&extver); err != nil {
		return fmt.Errorf("vectors: read the pgvector version: %w", err)
	}
	if versionLess(extver, vectorMinPgvector) {
		why := fmt.Sprintf("pgvector %s is older than the %s the vector search needs (hnsw.iterative_scan)", extver, vectorMinPgvector)
		s.vec.unsupported.Store(why)
		log.Printf("corpusd vectors disabled: %s", why)
		return errors.New("vectors: " + why)
	}
	s.vec.unsupported.Store("")
	present, err := s.vectorTablesPresent(ctx)
	if err != nil {
		return fmt.Errorf("vectors: read tables: %w", err)
	}
	if !present {
		if _, err := db.ExecContext(ctx, vectorTablesDDL); err != nil {
			return fmt.Errorf("vectors: create tables: %w", err)
		}
	}
	// Whether the tables were just made or already there (one of them may be an
	// older schema), the columns are brought current; it reads the catalog first.
	if err := s.migrateVectorColumns(ctx); err != nil {
		return err
	}
	for _, ix := range vectorPlainIndexes {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass(?) IS NOT NULL`, ix.name).Scan(&exists); err != nil {
			return fmt.Errorf("vectors: read index %s: %w", ix.name, err)
		}
		if exists {
			continue
		}
		if _, err := db.ExecContext(ctx, ix.ddl); err != nil {
			return fmt.Errorf("vectors: create index %s: %w", ix.name, err)
		}
	}
	s.vec.ready.Store(true)
	var firstErr error
	for _, ix := range []struct{ table, name, key string }{
		{vecStoryTable, vecStoryIndex, "story"}, {vecCaptureTable, vecCaptureIndex, "capture"},
	} {
		if err := s.ensureVectorIndex(ctx, ix.table, ix.name, ix.key); err != nil {
			log.Printf("corpusd retrieval vector index=%s err=%v", ix.name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// migrateVectorColumns brings a table made by an earlier schema (no refused
// column, a NOT NULL embedding) to the current one. It reads the catalog first
// and alters only what differs: an ALTER on every pass would take an exclusive
// lock that conflicts with the HNSW build.
func (s *Store) migrateVectorColumns(ctx context.Context) error {
	for _, table := range []string{vecStoryTable, vecCaptureTable} {
		var exists bool
		if err := s.corpus().QueryRowContext(ctx, `SELECT to_regclass(?) IS NOT NULL`, table).Scan(&exists); err != nil {
			return fmt.Errorf("vectors: read %s: %w", table, err)
		}
		if !exists {
			continue
		}
		var hasRefused bool
		var embNullable string
		if err := s.corpus().QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = 'refused'),
			COALESCE((SELECT is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = 'embedding'), 'YES')`, table, table).Scan(&hasRefused, &embNullable); err != nil {
			return fmt.Errorf("vectors: read columns of %s: %w", table, err)
		}
		if !hasRefused {
			if _, err := s.corpus().ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN IF NOT EXISTS refused TEXT NOT NULL DEFAULT ''`); err != nil {
				return fmt.Errorf("vectors: add refused to %s: %w", table, err)
			}
		}
		if embNullable == "NO" {
			if _, err := s.corpus().ExecContext(ctx, `ALTER TABLE `+table+` ALTER COLUMN embedding DROP NOT NULL`); err != nil {
				return fmt.Errorf("vectors: embedding of %s nullable: %w", table, err)
			}
		}
	}
	return nil
}

// ensureVectorIndex builds the HNSW index of a table if it is missing, and
// rebuilds it if an earlier build was interrupted and left it invalid. It
// follows ensureCaptureTermsCoverIndex: one connection holds a session
// advisory lock, the build is CONCURRENTLY and outside a transaction (writers
// go on; only schema changes and vacuum wait), and a planner never uses an
// invalid index, so a search is a scan of the table until it is valid. Calls
// come from every worker pass, so an index left invalid by a process that died
// is repaired on the next one.
func (s *Store) ensureVectorIndex(ctx context.Context, table, index, which string) error {
	lockKey := int64(vecStoryIndexLockKey)
	if which == "capture" {
		lockKey = vecCaptureIndexLockKey
	}
	conn, err := s.corpus().Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var got bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(?)`, lockKey).Scan(&got); err != nil {
		// The server may hold the lock though the answer was lost: discard the
		// connection so the session, and the lock, end.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return err
	}
	if !got {
		return nil // another process is building it
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(?)`, lockKey)
	}()
	var valid sql.NullBool
	err = conn.QueryRowContext(ctx, `SELECT i.indisvalid FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = ? AND i.indrelid = ?::regclass`, index, table).Scan(&valid)
	switch {
	case err == nil && valid.Bool:
		return nil
	case err == nil:
		log.Printf("corpusd retrieval migrate index=%s was left invalid by an interrupted build; rebuilding", index)
		if _, err := conn.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS `+index); err != nil {
			return fmt.Errorf("drop invalid %s: %w", index, err)
		}
	case !errors.Is(err, sql.ErrNoRows):
		return err
	}
	start := time.Now()
	// A build over 200,000 vectors wants memory for its graph. CREATE INDEX
	// CONCURRENTLY cannot run in a transaction, so the setting is
	// session-level: it is reset before the connection goes back to the pool.
	_, _ = conn.ExecContext(ctx, `SET maintenance_work_mem = '256MB'`)
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `RESET maintenance_work_mem`) }()
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX CONCURRENTLY IF NOT EXISTS %s ON %s USING hnsw (embedding halfvec_cosine_ops)`, index, table)); err != nil {
		return fmt.Errorf("build %s: %w", index, err)
	}
	log.Printf("corpusd retrieval migrate index=%s table=%s ms=%d concurrently", index, table, time.Since(start).Milliseconds())
	return nil
}

// vectorRefusedCounts is how many story passages and capture leads of the
// current model are recorded as refused by the embedder.
func (s *Store) vectorRefusedCounts(ctx context.Context, model string) (stories, captures int64, err error) {
	if err = s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_story_passages WHERE model = ? AND refused <> ''`, model).Scan(&stories); err != nil {
		return
	}
	err = s.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_captures WHERE model = ? AND refused <> ''`, model).Scan(&captures)
	return
}

// VectorsReady reports whether the tables exist.
func (s *Store) VectorsReady() bool { return s != nil && s.vec.ready.Load() }

// VectorRemaining is the last pass's count of rows still without a vector.
func (s *Store) VectorRemaining() int64 { return s.vec.remaining.Load() }

// versionLess reports whether the dotted version a is below b (numeric parts
// compared in order; a missing part is 0).
func versionLess(a, b string) bool {
	pa, pb := strings.Split(strings.TrimSpace(a), "."), strings.Split(strings.TrimSpace(b), ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func (v *vectorState) unsupportedReason() string {
	s, _ := v.unsupported.Load().(string)
	return s
}
