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

const vectorDDL = `
CREATE TABLE IF NOT EXISTS vec_story_passages (
	story_id    TEXT NOT NULL,
	version_id  TEXT NOT NULL,
	ordinal     INTEGER NOT NULL,
	text_sha    TEXT NOT NULL,
	day         DATE NOT NULL,
	model       TEXT NOT NULL,
	embedding   halfvec(256) NOT NULL,
	embedded_on DATE NOT NULL,
	PRIMARY KEY (story_id, version_id, ordinal)
);
CREATE INDEX IF NOT EXISTS idx_vsp_version ON vec_story_passages(version_id);
CREATE INDEX IF NOT EXISTS idx_vsp_day ON vec_story_passages(day);

CREATE TABLE IF NOT EXISTS vec_captures (
	capture_id  TEXT NOT NULL PRIMARY KEY,
	owner_id    TEXT NOT NULL,
	computer_id TEXT NOT NULL,
	text_sha    TEXT NOT NULL,
	day         DATE NOT NULL,
	model       TEXT NOT NULL,
	embedding   halfvec(256) NOT NULL,
	embedded_on DATE NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_vc_scope_day ON vec_captures(owner_id, computer_id, day);
`

// vectorState is the vector index's in-memory state: the embedder and the
// counts that coordinate the worker with searches. None of it is durable.
type vectorState struct {
	mu       sync.RWMutex
	embedder embedder.Embedder
	ready    atomic.Bool
	kick     chan struct{}
	kickOnce sync.Once
	started  sync.Once
	// remaining is the last pass's count of rows still without a vector.
	remaining atomic.Int64
	importMu  sync.Mutex
	jobs      map[string]*VectorImportJob
	lastJob   string
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

// ensureVectorTables checks pgvector is installed (creating it where the
// role may), creates the two tables and builds their indexes. It is
// idempotent and safe to call concurrently. A missing extension is an error
// that names the host unit that makes it.
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
	if _, err := db.ExecContext(ctx, vectorDDL); err != nil {
		return fmt.Errorf("vectors: create tables: %w", err)
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
	// A build over 200,000 vectors wants memory for its graph; the session
	// setting ends with this connection.
	_, _ = conn.ExecContext(ctx, `SET maintenance_work_mem = '256MB'`)
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE INDEX CONCURRENTLY IF NOT EXISTS %s ON %s USING hnsw (embedding halfvec_cosine_ops)`, index, table)); err != nil {
		return fmt.Errorf("build %s: %w", index, err)
	}
	log.Printf("corpusd retrieval migrate index=%s table=%s ms=%d concurrently", index, table, time.Since(start).Milliseconds())
	return nil
}

// VectorsReady reports whether the tables exist.
func (s *Store) VectorsReady() bool { return s != nil && s.vec.ready.Load() }

// VectorRemaining is the last pass's count of rows still without a vector.
func (s *Store) VectorRemaining() int64 { return s.vec.remaining.Load() }
