package store

import (
	"context"
	"database/sql"
	"log"
)

// projectStmts are the projection's hot statements, prepared once on the
// write pool. A replayed or appended object op ran its INSERT through
// tx.ExecContext, which parses and compiles it each time; in a CPU profile of
// d@e's replay that was 4.6 of finalize's 8.7 s. tx.StmtContext reuses the
// statement already prepared on the single write connection. A nil set (and
// a set whose prepare failed) falls back to the plain ExecContext.
type projectStmts struct {
	db           *sql.DB
	objectUpsert *sql.Stmt
}

func (p *projectStmts) objectUpsertStmt() *sql.Stmt {
	if p == nil {
		return nil
	}
	return p.objectUpsert
}

// projectionStmts prepares the set on first use. Call it before beginning a
// write transaction: the pool's one connection is the transaction's, and a
// first prepare inside it would wait for itself. The write connection is the
// pool's only one; database/sql prepares the statement again on a new
// connection, and SQLite recompiles it after a schema change.
func (s *Store) projectionStmts(ctx context.Context) *projectStmts {
	if s == nil || s.db == nil {
		return nil
	}
	s.projectStmtsMu.Lock()
	defer s.projectStmtsMu.Unlock()
	// A reopen replaces the database; a statement of the old one is refused
	// by tx.StmtContext, so the set follows s.db.
	if s.projectStmts != nil && s.projectStmts.db == s.db {
		return s.projectStmts
	}
	if s.projectStmts != nil {
		_ = s.projectStmts.objectUpsert.Close()
		s.projectStmts = nil
	}
	stmt, err := s.db.PrepareContext(context.WithoutCancel(ctx), ogObjectUpsertSQL)
	if err != nil {
		log.Printf("store: prepare projection statements: %v; projecting without them", err)
		return nil
	}
	s.projectStmts = &projectStmts{db: s.db, objectUpsert: stmt}
	return s.projectStmts
}
