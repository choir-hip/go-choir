package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Failure modes of the prepared projection statement: a prepared upsert that
// writes a different row than the plain one (the insert or the conflict
// update); a statement that fails once its transaction ends, so the next
// event's projection breaks; a statement that sees a stale schema after the
// store adds an index.
func TestPreparedObjectUpsertWritesWhatThePlainOneWrites(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	stmts := s.projectionStmts(ctx)
	if stmts.objectUpsertStmt() == nil {
		t.Fatal("the store prepared no upsert statement")
	}
	kind := objectgraph.ObjectKind("choir.event")
	project := func(id, hash string, at time.Time, with *projectStmts) {
		t.Helper()
		body, err := json.Marshal(objectgraph.Object{CanonicalID: id, ObjectKind: kind, OwnerID: "owner", ContentHash: hash, Metadata: json.RawMessage(`{"kind":"tool.result"}`), CreatedAt: at, UpdatedAt: at})
		if err != nil {
			t.Fatal(err)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := projectObject(ctx, tx, body, nil, with); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	read := func(id string) (string, string) {
		t.Helper()
		var hash, updated string
		if err := s.db.QueryRowContext(ctx, `SELECT content_hash, updated_at FROM og_objects WHERE canonical_id = ?`, id).Scan(&hash, &updated); err != nil {
			t.Fatal(err)
		}
		return hash, updated
	}
	t0 := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(90 * time.Minute)
	for i, with := range []*projectStmts{nil, stmts} {
		id := []string{"plain", "prepared"}[i]
		project(id, "sha256:a", t0, with)
		project(id, "sha256:b", t1, with) // the conflict update, in a second transaction
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_test_og_hash ON og_objects(content_hash)`); err != nil {
		t.Fatal(err)
	}
	project("prepared-after-index", "sha256:c", t1, stmts)
	ph, pu := read("plain")
	qh, qu := read("prepared")
	if ph != qh || pu != qu || qh != "sha256:b" {
		t.Fatalf("prepared upsert wrote (%s, %s), plain wrote (%s, %s)", qh, qu, ph, pu)
	}
	if h, _ := read("prepared-after-index"); h != "sha256:c" {
		t.Fatalf("after a schema change the prepared upsert wrote %q", h)
	}
}
