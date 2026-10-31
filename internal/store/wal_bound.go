package store

// The WAL bound (docs/design/append-clustered-store-2026-10-31.md part 1,
// rule B.5 of docs/design/small-object-storage-2026-10-31.md).
//
// A long read transaction (the snapshot capture, the compact copy) pins the
// WAL: a checkpoint cannot copy a frame past the oldest reader's mark, so
// every commit made while the read is open stays in the WAL, and the WAL file
// grows by all of them (the paper's state.db-wal measured 584 MB on
// 2026-10-31). Once the reader ends, the next automatic (PASSIVE) checkpoint
// backfills the WAL and the next write restarts it from the top, but SQLite
// never shrinks the file unless journal_size_limit is set: the high-water mark
// stays on disk, and each restart overwrites its head.
//
// The bound is two acts:
//
//  1. every writer connection opens with journal_size_limit =
//     dbx.SQLiteJournalSizeLimit (dbx.SQLiteDSN), so the first commit after a
//     WAL restart truncates the file to the limit;
//  2. a PASSIVE checkpoint on the write connection right after a long read
//     ends (CheckpointAfterLongRead), so the backfill does not wait for the
//     next 1000-page autocheckpoint.
//
// Neither blocks a writer or a reader: PASSIVE never invokes the busy handler
// and copies only frames no reader still needs, and the limit is a size
// applied at commit. There is no switch: both are SQLite's own maintenance
// with no durability or ordering effect, and the release switch cap is full.
// The read itself is not shortened: the head, the pages and the witness of a
// snapshot must be one state (internal/snapshot), and the SQLite backup API in
// steps restarts whenever another connection writes the source, so under the
// paper's write rate it would never finish.
//
// synchronous stays FULL. NORMAL in WAL mode cannot corrupt the file, and the
// boot replay re-derives every tape-projected commit a guest kill rolls back
// (replay_durability.go), but a tape-bound store still writes two families as
// direct SQL while their graph switches are off (co_super_slots,
// agent_evidence): an acknowledged one of those would be lost by a guest kill
// at NORMAL.

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// walBoundState is the bound's checkpoint record.
type walBoundState struct {
	mu   sync.Mutex
	last WALCheckpoint
	runs int64
	errs int64
}

// WALCheckpoint is one checkpoint the bound ran: SQLite's three results
// (busy, frames in the WAL, frames backfilled) and the WAL file size before
// and after.
type WALCheckpoint struct {
	Reason       string    `json:"reason"`
	At           time.Time `json:"at"`
	Busy         int64     `json:"busy"`
	LogFrames    int64     `json:"log_frames"`
	Checkpointed int64     `json:"checkpointed_frames"`
	WALBefore    int64     `json:"wal_bytes_before"`
	WALAfter     int64     `json:"wal_bytes_after"`
	Duration     string    `json:"duration"`
	Error        string    `json:"error,omitempty"`
}

// WALStats is the WAL as /api/observability reports it.
type WALStats struct {
	JournalSizeLimit int64          `json:"journal_size_limit"`
	WALBytes         int64          `json:"wal_bytes"`
	Checkpoints      int64          `json:"checkpoints"`
	CheckpointErrors int64          `json:"checkpoint_errors"`
	LastCheckpoint   *WALCheckpoint `json:"last_checkpoint,omitempty"`
}

// CheckpointAfterLongRead runs one PASSIVE checkpoint after a long read
// transaction has ended. It never blocks a reader or a writer beyond the
// checkpoint itself, which the single write connection serializes like any
// commit.
func (s *Store) CheckpointAfterLongRead(ctx context.Context, reason string) {
	if s == nil || s.db == nil {
		return
	}
	started := time.Now()
	cp := WALCheckpoint{Reason: reason, At: started.UTC(), WALBefore: s.walFileBytes()}
	err := s.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`).Scan(&cp.Busy, &cp.LogFrames, &cp.Checkpointed)
	cp.WALAfter = s.walFileBytes()
	cp.Duration = time.Since(started).Round(time.Millisecond).String()
	s.walBound.mu.Lock()
	s.walBound.runs++
	if err != nil {
		cp.Error = err.Error()
		s.walBound.errs++
	}
	s.walBound.last = cp
	s.walBound.mu.Unlock()
	if err != nil {
		log.Printf("store: wal bound: checkpoint after %s: %v", reason, err)
		return
	}
	log.Printf("store: wal bound: checkpoint after %s: busy=%d log=%d checkpointed=%d wal %d -> %d bytes in %s",
		reason, cp.Busy, cp.LogFrames, cp.Checkpointed, cp.WALBefore, cp.WALAfter, cp.Duration)
}

// WALStats reports the WAL file's current size and the bound's checkpoints.
// It reads no database: a file stat and a mutex.
func (s *Store) WALStats() WALStats {
	if s == nil {
		return WALStats{}
	}
	s.walBound.mu.Lock()
	st := WALStats{JournalSizeLimit: dbx.SQLiteJournalSizeLimit, Checkpoints: s.walBound.runs, CheckpointErrors: s.walBound.errs}
	if s.walBound.runs > 0 {
		last := s.walBound.last
		st.LastCheckpoint = &last
	}
	s.walBound.mu.Unlock()
	st.WALBytes = s.walFileBytes()
	return st
}

func (s *Store) walFileBytes() int64 {
	if s.statePath == "" {
		return 0
	}
	info, err := os.Stat(s.statePath + "-wal")
	if err != nil {
		return 0
	}
	return info.Size()
}
