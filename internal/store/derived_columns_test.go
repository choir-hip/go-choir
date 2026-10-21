package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The explicit derived-column migration of a runtime store
// (docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md).
// Ways it can fail, written before the code:
//
//  1. Open, or the boot's discovery call, rewrites a populated og_objects
//     that lacks the record columns.
//  2. Two migrations run at once (two operators, a retried request).
//  3. The read pool's store keeps a stale live set after a step, so reads
//     stay on JSON after the migration, or name a path before it exists.
//  4. Status hides progress or the end state, or blocks while a step holds
//     the engine.
//  5. A cancel is lost, or Close leaves the migration running.

func ogTableHash(t *testing.T, s *Store) string {
	t.Helper()
	var hash string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('og_objects')`).Scan(&hash); err != nil {
		t.Fatalf("hash of og_objects: %v", err)
	}
	return hash
}

// populatedStoreMissingRecordColumns is a runtime store with og_objects
// rows whose record columns and indexes are gone, reopened.
func populatedStoreMissingRecordColumns(t *testing.T) (*Store, string) {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	at := time.Now().UTC()
	for i := 0; i < 20; i++ {
		obj := objectgraph.Object{CanonicalID: fmt.Sprintf("rec-%02d", i), ObjectKind: "choir.commitment_record",
			OwnerID: "owner", ComputerID: "computer", VersionID: "v", ContentHash: fmt.Sprintf("h-%d", i),
			Body: []byte(`{}`), Metadata: []byte(fmt.Sprintf(`{"inbox":"texture:doc-%d"}`, i%2)), CreatedAt: at, UpdatedAt: at}
		if err := s.ogStore.PutObject(ctx, obj); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{
		`DROP INDEX idx_og_objects_og_rec_inbox ON og_objects`,
		`DROP INDEX idx_og_objects_og_rec_sender_inbox ON og_objects`,
		`DROP INDEX idx_og_objects_og_rec_conversation_id ON og_objects`,
		`DROP INDEX idx_og_objects_rec_inbox_seq ON og_objects`,
		`DROP INDEX idx_og_objects_rec_claim ON og_objects`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_inbox_seq`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_claim`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_inbox`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_sender_inbox`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_conversation_id`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	hash := ogTableHash(t, s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, hash
}

func waitDerivedColumnMigration(t *testing.T, s *Store) DerivedColumnMigrationStatus {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		status := s.DerivedColumnMigrationStatus(context.Background())
		if status.State != DerivedColumnMigrationRunning {
			return status
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("derived column migration did not finish")
	return DerivedColumnMigrationStatus{}
}

func TestDerivedColumnMigrationRuntimeStore(t *testing.T) {
	ctx := context.Background()
	s, hash := populatedStoreMissingRecordColumns(t)
	if got := ogTableHash(t, s); got != hash {
		t.Fatalf("reopen rewrote og_objects: %s -> %s", hash, got)
	}
	plan, err := s.DiscoverObjectGraphAccessPaths(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := ogTableHash(t, s); got != hash {
		t.Fatalf("boot discovery rewrote og_objects: %s -> %s", hash, got)
	}
	if len(plan.MissingColumns) != 5 || len(plan.MissingIndexes) != 5 {
		t.Fatalf("plan %+v, want the five record columns and indexes missing", plan)
	}
	idle := s.DerivedColumnMigrationStatus(ctx)
	if idle.State != DerivedColumnMigrationIdle || len(idle.Plan.MissingColumns) != 5 {
		t.Fatalf("idle status %+v", idle)
	}

	release := make(chan struct{})
	derivedColumnStepHook = func() { <-release }
	t.Cleanup(func() { derivedColumnStepHook = nil })
	started, err := s.StartDerivedColumnMigration()
	if err != nil || started.State != DerivedColumnMigrationRunning {
		t.Fatalf("start: %+v, %v", started, err)
	}
	if _, err := s.StartDerivedColumnMigration(); !errors.Is(err, ErrDerivedColumnMigrationRunning) {
		t.Fatalf("second start err = %v, want ErrDerivedColumnMigrationRunning", err)
	}
	// Status answers while the migration runs, without the engine.
	if running := s.DerivedColumnMigrationStatus(ctx); running.State != DerivedColumnMigrationRunning {
		t.Fatalf("running status %+v", running)
	}
	close(release)
	done := waitDerivedColumnMigration(t, s)
	if done.State != DerivedColumnMigrationDone || done.Error != "" || len(done.Steps) != 10 {
		t.Fatalf("done status %+v", done)
	}
	if len(done.Plan.MissingColumns) != 0 || len(done.Plan.MissingIndexes) != 0 {
		t.Fatalf("done plan %+v", done.Plan)
	}
	readLive := map[string]bool{}
	for _, name := range s.ogReadStore.LiveAccessPaths() {
		readLive[name] = true
	}
	if !readLive["og_rec_inbox"] || !readLive["idx_og_objects_rec_claim"] {
		t.Fatalf("read store live set after migrate: %v", s.ogReadStore.LiveAccessPaths())
	}
	got, err := s.ogReadStore.ListObjectsByMetadataPage(ctx, "choir.commitment_record", "$.inbox", "texture:doc-1", "", 100)
	if err != nil || len(got) != 10 {
		t.Fatalf("indexed read after migrate = %d, %v", len(got), err)
	}
	again, err := s.StartDerivedColumnMigration()
	if err != nil {
		t.Fatalf("restart after done: %+v, %v", again, err)
	}
	if final := waitDerivedColumnMigration(t, s); final.State != DerivedColumnMigrationDone || len(final.Steps) != 0 {
		t.Fatalf("second migration ran steps: %+v", final)
	}
}

func TestDerivedColumnMigrationCancel(t *testing.T) {
	s, _ := populatedStoreMissingRecordColumns(t)
	release := make(chan struct{})
	derivedColumnStepHook = func() { <-release }
	t.Cleanup(func() { derivedColumnStepHook = nil })
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	if !s.CancelDerivedColumnMigration() {
		t.Fatal("cancel of a running migration reported nothing to cancel")
	}
	close(release)
	status := waitDerivedColumnMigration(t, s)
	if status.State != DerivedColumnMigrationCancelled || len(status.Plan.MissingColumns) == 0 {
		t.Fatalf("cancelled status %+v", status)
	}
	if s.CancelDerivedColumnMigration() {
		t.Fatal("cancel with nothing running reported a cancel")
	}
}
