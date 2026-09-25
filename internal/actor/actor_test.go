package actor

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func testLog(t *testing.T) *SQLiteLog {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "actor.db")+"?_pragma=busy_timeout(60000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	log, err := NewSQLiteLog(db)
	if err != nil {
		t.Fatalf("new log: %v", err)
	}
	return log
}

func TestAppendIdempotent(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	u := Update{UpdateID: "u1", ToAgentID: "a1", Content: "hello", CreatedAt: time.Now().UTC()}
	first, err := log.Append(ctx, u)
	if err != nil || !first {
		t.Fatalf("first append = (%v, %v), want (true, nil)", first, err)
	}
	second, err := log.Append(ctx, u)
	if err != nil || second {
		t.Fatalf("second append = (%v, %v), want (false, nil)", second, err)
	}
	backlog, err := log.Unprocessed(ctx, "a1")
	if err != nil || len(backlog) != 1 {
		t.Fatalf("backlog = %v (err %v), want exactly 1", backlog, err)
	}
}

func TestSQLiteLogRebindMailboxPreservesUpdatesAndSnapshot(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	legacyID, scopedID := "legacy-agent", "owner\x00computer\x00legacy-agent"
	if appended, err := log.Append(ctx, Update{UpdateID: "legacy-update", ToAgentID: legacyID, Content: "retained", CreatedAt: time.Now().UTC()}); err != nil || !appended {
		t.Fatalf("append legacy update: appended=%v err=%v", appended, err)
	}
	if appended, err := log.Append(ctx, Update{UpdateID: "processed-update", ToAgentID: legacyID, Content: "settled", CreatedAt: time.Now().UTC()}); err != nil || !appended {
		t.Fatalf("append processed legacy update: appended=%v err=%v", appended, err)
	}
	if err := log.MarkProcessed(ctx, legacyID, "processed-update"); err != nil {
		t.Fatalf("mark legacy update processed: %v", err)
	}
	if err := log.SaveSnapshot(ctx, legacyID, []byte("retained-memory")); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	migrated, err := log.RebindMailbox(ctx, legacyID, scopedID)
	if err != nil || !migrated {
		t.Fatalf("rebind mailbox: migrated=%v err=%v", migrated, err)
	}
	if updates, err := log.Unprocessed(ctx, legacyID); err != nil || len(updates) != 0 {
		t.Fatalf("legacy updates after rebind: %v, %v", updates, err)
	}
	updates, err := log.Unprocessed(ctx, scopedID)
	if err != nil || len(updates) != 1 || updates[0].UpdateID != "legacy-update" {
		t.Fatalf("scoped updates after rebind: %+v, %v", updates, err)
	}
	memory, err := log.LoadSnapshot(ctx, scopedID)
	if err != nil || string(memory) != "retained-memory" {
		t.Fatalf("scoped snapshot after rebind: %q, %v", memory, err)
	}
	var processedMailbox string
	var processedAt sql.NullTime
	if err := log.db.QueryRowContext(ctx, `SELECT to_agent_id, processed_at FROM actor_updates WHERE update_id = ?`, "processed-update").Scan(&processedMailbox, &processedAt); err != nil {
		t.Fatalf("load processed update after rebind: %v", err)
	}
	if processedMailbox != scopedID || !processedAt.Valid {
		t.Fatalf("processed update after rebind: mailbox=%q processed=%v", processedMailbox, processedAt.Valid)
	}
	migrated, err = log.RebindMailbox(ctx, legacyID, scopedID)
	if err != nil || migrated {
		t.Fatalf("idempotent rebind: migrated=%v err=%v", migrated, err)
	}
}

func TestSQLiteLogRebindMailboxMixedSnapshotsKeepsNewestDestination(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	if err := log.SaveSnapshot(ctx, "legacy", []byte("legacy-memory")); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := log.SaveSnapshot(ctx, "scoped", []byte("scoped-memory")); err != nil {
		t.Fatalf("save scoped snapshot: %v", err)
	}
	legacyAt := time.Date(2026, time.July, 23, 1, 0, 0, 0, time.UTC)
	scopedAt := legacyAt.Add(time.Minute)
	if _, err := log.db.ExecContext(ctx, `UPDATE actor_snapshots SET updated_at = CASE agent_id WHEN 'legacy' THEN ? ELSE ? END`, legacyAt, scopedAt); err != nil {
		t.Fatalf("set snapshot times: %v", err)
	}
	if migrated, err := log.RebindMailbox(ctx, "legacy", "scoped"); err != nil || !migrated {
		t.Fatalf("merge snapshots: migrated=%v err=%v", migrated, err)
	}
	if memory, err := log.LoadSnapshot(ctx, "legacy"); err != nil || memory != nil {
		t.Fatalf("legacy snapshot after merge: %q, %v", memory, err)
	}
	if memory, err := log.LoadSnapshot(ctx, "scoped"); err != nil || string(memory) != "scoped-memory" {
		t.Fatalf("scoped snapshot after merge: %q, %v", memory, err)
	}
	if migrated, err := log.RebindMailbox(ctx, "legacy", "scoped"); err != nil || migrated {
		t.Fatalf("repeated merge: migrated=%v err=%v", migrated, err)
	}
}

func TestSQLiteLogRebindMailboxMergesDestinationUpdates(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	for _, update := range []Update{
		{UpdateID: "legacy-update", ToAgentID: "legacy", CreatedAt: time.Now().UTC()},
		{UpdateID: "scoped-update", ToAgentID: "scoped", CreatedAt: time.Now().UTC()},
	} {
		if appended, err := log.Append(ctx, update); err != nil || !appended {
			t.Fatalf("append %s: appended=%v err=%v", update.UpdateID, appended, err)
		}
	}
	if migrated, err := log.RebindMailbox(ctx, "legacy", "scoped"); err != nil || !migrated {
		t.Fatalf("merge updates: migrated=%v err=%v", migrated, err)
	}
	legacy, legacyErr := log.Unprocessed(ctx, "legacy")
	scoped, scopedErr := log.Unprocessed(ctx, "scoped")
	if legacyErr != nil || scopedErr != nil || len(legacy) != 0 || len(scoped) != 2 {
		t.Fatalf("backlogs after merge: legacy=%+v (%v), scoped=%+v (%v)", legacy, legacyErr, scoped, scopedErr)
	}
}

func TestSQLiteLogRebindMailboxMixedSnapshotsMovesNewerLegacy(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	if err := log.SaveSnapshot(ctx, "legacy", []byte("legacy-memory")); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := log.SaveSnapshot(ctx, "scoped", []byte("scoped-memory")); err != nil {
		t.Fatalf("save scoped snapshot: %v", err)
	}
	scopedAt := time.Date(2026, time.July, 23, 1, 0, 0, 0, time.UTC)
	legacyAt := scopedAt.Add(time.Minute)
	if _, err := log.db.ExecContext(ctx, `UPDATE actor_snapshots SET updated_at = CASE agent_id WHEN 'legacy' THEN ? ELSE ? END`, legacyAt, scopedAt); err != nil {
		t.Fatalf("set snapshot times: %v", err)
	}
	if migrated, err := log.RebindMailbox(ctx, "legacy", "scoped"); err != nil || !migrated {
		t.Fatalf("merge snapshots: migrated=%v err=%v", migrated, err)
	}
	if memory, err := log.LoadSnapshot(ctx, "scoped"); err != nil || string(memory) != "legacy-memory" {
		t.Fatalf("scoped snapshot after merge: %q, %v", memory, err)
	}
}
