package store

import (
	"context"
	"testing"
	"time"
)

// PoolStats reports both one-connection pools' sql.DBStats, so the time
// callers spend waiting for the store's only write connection shows without
// a stall (docs/problems/paper-post-refresh-health-windows-2026-10-22.md).
// Ways it could fail: the read pool reported as the write pool; a wait that
// gave up (context deadline) not counted; durations in the wrong unit.
func TestStorePoolStatsCountsAWaitOnTheWriteConnection(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	conn, err := s.DB().Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	before := s.PoolStats()
	if before == nil || before.Write.MaxOpen != 1 || before.Read.MaxOpen != 4 || before.Write.InUse != 1 {
		t.Fatalf("pool stats with the write connection held = %+v", before)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	if _, err := s.DB().ExecContext(waitCtx, "SELECT 1"); err == nil {
		t.Fatal("a query on the held write pool answered")
	}
	after := s.PoolStats()
	if after.Write.WaitCount != before.Write.WaitCount+1 {
		t.Fatalf("write wait_count %d -> %d, want one more", before.Write.WaitCount, after.Write.WaitCount)
	}
	if got := after.Write.WaitDurationMS - before.Write.WaitDurationMS; got < 100 || got > 10_000 {
		t.Fatalf("write wait_duration_ms grew by %d, want about 150", got)
	}
	if after.Read.WaitCount != before.Read.WaitCount {
		t.Fatalf("read pool wait_count moved on a write-pool wait: %+v", after.Read)
	}
}
