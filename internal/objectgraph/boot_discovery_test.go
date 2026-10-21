package objectgraph

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// Boot is discovery only on a store with rows
// (docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md).
// Ways it can fail, written before the code:
//
//  1. Open (EnsureSchema) or the boot's record-index step runs ALTER TABLE
//     or CREATE INDEX on an og_objects table that has rows: a missing record
//     column, a missing legacy column, or a column whose index is missing.
//  2. Discovery calls a path live when only half of it exists (the column
//     without its index, the index without its columns), so a reader names
//     a column that is not there or reads without the index it assumes.
//  3. A read on a missing path answers differently from the indexed read
//     (the JSON fallback is not taken, or is taken wrongly).
//  4. A fresh store no longer gets the derived columns at open.
//  5. The explicit migration adds nothing, or everything in one rewrite
//     with no progress, or is not idempotent, or leaves the live set stale
//     on the write or the read handle.
//  6. A cancelled migration leaves the store unusable or half-live, or the
//     next open resumes it.

// tableHash is Dolt's content hash of og_objects: any rewrite, column or
// index change moves it.
func tableHash(t *testing.T, db *sql.DB) string {
	t.Helper()
	var hash string
	if err := db.QueryRow(`SELECT dolt_hashof_table('og_objects')`).Scan(&hash); err != nil {
		t.Fatalf("hash of og_objects: %v", err)
	}
	return hash
}

func ogColumns(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'og_objects'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	return names
}

func ogIndexes(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT DISTINCT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'og_objects'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	return names
}

// legacyPopulatedStore is a store from before every derived column: the
// base table only, with records and legacy-keyed rows in it.
func legacyPopulatedStore(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDoltDB(t)
	if _, err := db.Exec(SchemaDDL); err != nil {
		t.Fatal(err)
	}
	old := NewDoltStore(db)
	at := time.Now().UTC()
	for i := 0; i < 30; i++ {
		putTestObject(t, old, fmt.Sprintf("rec-%02d", i), commitmentRecordKind,
			map[string]any{"inbox": fmt.Sprintf("texture:doc-%d", i%3), "inbox_seq": i, "run_id": fmt.Sprintf("run-%d", i%5)}, `{}`, at.Add(time.Duration(i)*time.Second))
	}
	return db
}

func TestBootDiscoveryLeavesPopulatedStoreMissingColumnsUntouched(t *testing.T) {
	ctx := context.Background()
	db := legacyPopulatedStore(t)
	hash, columns, indexes := tableHash(t, db), ogColumns(t, db), ogIndexes(t, db)

	for boot := 0; boot < 2; boot++ {
		store := NewDoltStore(db)
		if err := store.EnsureSchema(ctx); err != nil {
			t.Fatalf("boot %d: open: %v", boot, err)
		}
		plan, err := store.DiscoverAccessPaths(ctx)
		if err != nil {
			t.Fatalf("boot %d: discover: %v", boot, err)
		}
		if got := tableHash(t, db); got != hash {
			t.Fatalf("boot %d rewrote og_objects: hash %s -> %s", boot, hash, got)
		}
		if got := ogColumns(t, db); strings.Join(got, ",") != strings.Join(columns, ",") {
			t.Fatalf("boot %d changed og_objects columns: %v -> %v", boot, columns, got)
		}
		if got := ogIndexes(t, db); strings.Join(got, ",") != strings.Join(indexes, ",") {
			t.Fatalf("boot %d changed og_objects indexes: %v -> %v", boot, indexes, got)
		}
		if !plan.HasRows || len(plan.Live) != 0 {
			t.Fatalf("boot %d: plan %+v, want rows and nothing live", boot, plan)
		}
		if len(plan.MissingColumns) != len(metadataIndexColumns)+len(recordColumns) {
			t.Fatalf("boot %d: missing columns %v, want every derived column", boot, plan.MissingColumns)
		}
		if len(plan.MissingIndexes) == 0 {
			t.Fatalf("boot %d: no missing indexes reported: %+v", boot, plan)
		}
		// Reads answer on their JSON paths.
		got, err := store.ListObjectsByMetadataPage(ctx, string(commitmentRecordKind), "$.inbox", "texture:doc-1", "", 100)
		if err != nil || len(got) != 10 {
			t.Fatalf("boot %d: JSON inbox read = %d, %v", boot, len(got), err)
		}
		if _, ok, err := store.ListCommitmentClaimObjects(ctx, "owner", 10); ok || err != nil {
			t.Fatalf("boot %d: claim index claimed live: ok=%v err=%v", boot, ok, err)
		}
		if got, err := store.ListObjectsByMetadata(ctx, string(commitmentRecordKind), "$.run_id", "run-2", 100); err != nil || len(got) != 6 {
			t.Fatalf("boot %d: JSON run read = %d, %v", boot, len(got), err)
		}
	}
}

func TestBootDiscoveryLeavesMissingIndexMissing(t *testing.T) {
	ctx := context.Background()
	db := legacyPopulatedStore(t)
	// Every column present, one index of each kind dropped: a store whose
	// earlier index build failed.
	full := NewDoltStore(db)
	if _, err := full.MigrateAccessPaths(ctx, nil); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`DROP INDEX idx_og_objects_og_rec_inbox ON og_objects`,
		`DROP INDEX ` + recordClaimIndex + ` ON og_objects`,
		`DROP INDEX idx_og_objects_og_meta_run_id ON og_objects`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	hash := tableHash(t, db)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if got := tableHash(t, db); got != hash {
		t.Fatalf("open built an index on a store with rows: hash %s -> %s", hash, got)
	}
	if store.live("og_rec_inbox") || store.live(recordClaimIndex) || store.live("og_meta_run_id") {
		t.Fatalf("a column without its index is live: %v", store.liveNames())
	}
	if !store.live("og_rec_sender_inbox") || !store.live(recordInboxSeqIndex) {
		t.Fatalf("complete paths not live: %v", store.liveNames())
	}
	if got, err := store.ListObjectsByMetadataPage(ctx, string(commitmentRecordKind), "$.inbox", "texture:doc-1", "", 100); err != nil || len(got) != 10 {
		t.Fatalf("JSON inbox read = %d, %v", len(got), err)
	}
	// The inbox-seq index exists, but it needs og_rec_inbox: dropping that
	// column's own index does not drop the column, so it stays live.
	if _, err := db.Exec(`ALTER TABLE og_objects DROP COLUMN og_rec_inbox_seq`); err == nil {
		reopened := NewDoltStore(db)
		if err := reopened.EnsureSchema(ctx); err != nil {
			t.Fatal(err)
		}
		if reopened.live(recordInboxSeqIndex) {
			t.Fatalf("an index whose column is gone is live: %v", reopened.liveNames())
		}
	}
}

func TestBootDiscoveryFreshStoreTakesEveryPath(t *testing.T) {
	ctx := context.Background()
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	plan, err := store.DiscoverAccessPaths(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasRows || len(plan.MissingColumns) != 0 || len(plan.MissingIndexes) != 0 {
		t.Fatalf("fresh store plan %+v, want everything present", plan)
	}
	for _, mc := range metadataIndexColumns {
		if !store.live(mc.Column) {
			t.Fatalf("fresh store: %s not live: %v", mc.Column, store.liveNames())
		}
	}
	if !store.live(recordInboxSeqIndex) || !store.live(recordClaimIndex) {
		t.Fatalf("fresh store: record paths not live: %v", store.liveNames())
	}
}

func TestDerivedColumnMigrationStepsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := legacyPopulatedStore(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	reader := NewDoltStore(db)
	reader.ShareEngineMutex(store)
	reader.InheritMetadataColumns(store)
	before, err := store.DiscoverAccessPaths(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unmigrated := tableHash(t, db)

	var steps []AccessPathStep
	plan, err := store.MigrateAccessPaths(ctx, func(step AccessPathStep) {
		steps = append(steps, step)
		reader.InheritMetadataColumns(store)
	})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if want := len(before.MissingColumns) + len(before.MissingIndexes); len(steps) != want {
		t.Fatalf("steps = %d, want one per missing column and index (%d): %+v", len(steps), want, steps)
	}
	for i, step := range steps {
		if step.Err != "" || step.Name == "" || (step.Kind != "column" && step.Kind != "index") || step.Index != i+1 || step.Total != len(steps) {
			t.Fatalf("step %d: %+v", i, step)
		}
	}
	if len(plan.MissingColumns) != 0 || len(plan.MissingIndexes) != 0 {
		t.Fatalf("after migrate, plan %+v", plan)
	}
	if tableHash(t, db) == unmigrated {
		t.Fatal("og_objects hash did not move across the migration; the hash assertions prove nothing")
	}
	if !reader.live(recordClaimIndex) || !reader.live("og_rec_inbox") || !reader.live("og_meta_run_id") {
		t.Fatalf("read handle did not learn the migrated paths: %v", reader.liveNames())
	}
	claims, ok, err := reader.ListCommitmentClaimObjects(ctx, "owner", 100)
	if err != nil || !ok || len(claims) != 30 {
		t.Fatalf("migrated claims = %d ok=%v err=%v", len(claims), ok, err)
	}
	if got, err := reader.ListObjectsByMetadataPage(ctx, string(commitmentRecordKind), "$.inbox", "texture:doc-1", "", 100); err != nil || len(got) != 10 {
		t.Fatalf("indexed inbox read = %d, %v", len(got), err)
	}

	hash := tableHash(t, db)
	again, err := store.MigrateAccessPaths(ctx, func(step AccessPathStep) { t.Fatalf("second migrate ran a step: %+v", step) })
	if err != nil || len(again.MissingColumns) != 0 {
		t.Fatalf("second migrate: %+v, %v", again, err)
	}
	if got := tableHash(t, db); got != hash {
		t.Fatalf("second migrate changed og_objects: %s -> %s", hash, got)
	}
}

func TestDerivedColumnMigrationCancelledLeavesStoreUsable(t *testing.T) {
	db := legacyPopulatedStore(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	steps := 0
	_, err := store.MigrateAccessPaths(ctx, func(AccessPathStep) {
		steps++
		if steps == 2 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled migrate err = %v", err)
	}
	if steps != 2 {
		t.Fatalf("migrate ran %d steps after cancel at 2", steps)
	}
	// The next boot is discovery only: what was added stays, the rest stays
	// missing, nothing resumes.
	hash := tableHash(t, db)
	reopened := NewDoltStore(db)
	if err := reopened.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := tableHash(t, db); got != hash {
		t.Fatalf("open after a cancelled migration changed og_objects: %s -> %s", hash, got)
	}
	plan, err := reopened.DiscoverAccessPaths(context.Background())
	if err != nil || len(plan.MissingColumns) == 0 {
		t.Fatalf("plan after cancel: %+v, %v", plan, err)
	}
	if got, err := reopened.ListObjectsByMetadataPage(context.Background(), string(commitmentRecordKind), "$.inbox", "texture:doc-1", "", 100); err != nil || len(got) != 10 {
		t.Fatalf("read after a cancelled migration = %d, %v", len(got), err)
	}
	putTestObject(t, reopened, "after-cancel", commitmentRecordKind, map[string]any{"inbox": "texture:doc-1"}, `{}`, time.Now().UTC())
	if _, err := reopened.MigrateAccessPaths(context.Background(), nil); err != nil {
		t.Fatalf("migrate after cancel: %v", err)
	}
	if got, err := reopened.ListObjectsByMetadataPage(context.Background(), string(commitmentRecordKind), "$.inbox", "texture:doc-1", "", 100); err != nil || len(got) != 11 {
		t.Fatalf("indexed read after the resumed migration = %d, %v", len(got), err)
	}
}
