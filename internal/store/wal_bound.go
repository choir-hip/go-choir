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
//  2. a PASSIVE checkpoint right after a long read ends
//     (CheckpointAfterLongRead; on its own connection, so the one write
//     connection stays free for commits while it backfills), so the backfill
//     does not wait for the
//     next 1000-page autocheckpoint.
//
// Neither blocks a writer or a reader: PASSIVE never invokes the busy handler
// and copies only frames no reader still needs, and the limit is a size
// applied at commit. There is no switch: both are SQLite's own maintenance
// with no durability or ordering effect, and the release switch cap is full.
// The snapshot capture's read is not shortened: the head, the witness and the
// pages of a snapshot must be one state, so the capture runs the witness and
// streams the pages inside the one read transaction that read the head, and the
// WAL grows by every commit made until the last page is handed over (about
// 2 MiB/s in one reading on the paper, 2026-11-12; internal/snapshot,
// docs/design/snapshot-without-scratch-copy-2026-11-20.md). A scratch copy
// that shortened the pin (842e0fda6, 2026-11-15) cost the store's size in disk
// instead and was removed. The SQLite backup API is not used: it restarts
// whenever another connection writes the source, so under the paper's write
// rate it would never finish. The compact copy (VACUUM INTO) pins the WAL for
// its whole run too.
//
// synchronous stays FULL. NORMAL in WAL mode cannot corrupt the file, and the
// boot replay re-derives every tape-projected commit a guest kill rolls back
// (replay_durability.go), but a tape-bound store still writes two families as
// direct SQL while their graph switches are off (co_super_slots,
// agent_evidence): an acknowledged one of those would be lost by a guest kill
// at NORMAL.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// The rules of SQLite this relies on (3.50, the amalgamation behind
// modernc.org/sqlite v1.49: wal.c and sqlite3.c):
//
//   - A PASSIVE checkpoint never calls the busy handler and takes only the
//     checkpoint lock (and, for the copy, WAL_READ_LOCK(0) exclusively). It never
//     takes the WAL write lock, so it blocks no writer. It copies only the frames
//     up to the smallest read mark of any reader (mxSafeFrame), so it never
//     disturbs a reader. If another checkpoint holds the checkpoint lock, or a
//     connection reading straight from the database file holds read lock 0, it
//     returns BUSY (wal_checkpoint answers busy=1, log=-1, checkpointed=-1) and
//     does nothing: that is retried, never read as a count.
//   - The WAL restarts at the first commit by a writer after every frame has been
//     backfilled (walRestartLog tests nBackfill == mxFrame in shared memory), by
//     whichever connection did the backfill; journal_size_limit then truncates
//     the file on that commit.
//   - wal_autocheckpoint (default 1000 frames, set per connection and never
//     overridden here) runs a PASSIVE checkpoint inside the commit on the
//     committing connection (doWalCallbacks), and database/sql holds the store's
//     single write connection until it returns: after a long pin the first commit
//     would backfill the whole WAL (about 22 s per GiB on the paper's 2026-11-12
//     reading) with every write, append and /health (which reads through that
//     connection) waiting. SuspendAutoCheckpoint turns it off for the capture's
//     pin; CheckpointAfterLongRead then does the backfill on its own connection.
//     The pragma belongs to the connection, so a process that dies comes up with
//     the default.

// postLongReadCheckpointBudget bounds the checkpoint after a long read: at 22 s
// per GiB on the paper's one reading, 8 GiB is about 3 minutes, so 15 minutes is
// five times that, and a bounded retry loop never runs longer. After it the
// automatic checkpoint is back and does what is left.
const postLongReadCheckpointBudget = 15 * time.Minute

// ErrWALFramesUnknown means the WAL's frame count could not be read: the
// checkpoint that reads it was busy twice.
var ErrWALFramesUnknown = errors.New("store: the WAL's frame count is unknown (the checkpoint lock was busy)")

// checkpointKnown reports whether a wal_checkpoint answer is a count: busy=1
// comes with log=-1 and checkpointed=-1.
func checkpointKnown(busy, logFrames, checkpointed int64) bool {
	return busy == 0 && logFrames >= 0 && checkpointed >= 0
}

// passiveCheckpoint runs one PASSIVE checkpoint on its own short-lived
// connection, not on the store's single write connection. It opens the existing
// file read-write (mode=rw, so a missing path is an error and never an empty
// database). Without a state path it uses the write connection.
func (s *Store) passiveCheckpoint(ctx context.Context) (busy, logFrames, checkpointed int64, err error) {
	if s.statePath == "" {
		err = s.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &checkpointed)
		return
	}
	cdb, err := sql.Open("sqlite", dbx.SQLiteDSN(s.statePath, dbx.SQLiteOptions{}, false)+"&mode=rw")
	if err != nil {
		return 0, 0, 0, err
	}
	defer cdb.Close()
	cdb.SetMaxOpenConns(1)
	err = cdb.QueryRowContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &checkpointed)
	return
}

// SuspendAutoCheckpoint turns off the automatic checkpoint of the store's write
// connection and returns the function that restores what it was. A snapshot
// capture's read pin makes the WAL large; without this the first commit after
// the pin is released would backfill it on the write connection (see the rules
// above). The caller must call resume on every path, after the checkpoint after
// the long read (CheckpointAfterLongRead) has run; resume is idempotent. The
// setting is per connection, so a crash restarts with the default.
func (s *Store) SuspendAutoCheckpoint(ctx context.Context) (resume func(), err error) {
	if s == nil || s.db == nil {
		return func() {}, errors.New("store: no state database")
	}
	var prior int64
	if err := s.db.QueryRowContext(ctx, `PRAGMA wal_autocheckpoint`).Scan(&prior); err != nil {
		return func() {}, err
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA wal_autocheckpoint=0`); err != nil {
		return func() {}, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			rctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if _, err := s.db.ExecContext(rctx, fmt.Sprintf(`PRAGMA wal_autocheckpoint=%d`, prior)); err != nil {
				log.Printf("store: wal bound: could not restore wal_autocheckpoint=%d (the setting is per connection and a restart restores it): %v", prior, err)
			}
		})
	}, nil
}

// CheckpointAfterLongRead backfills the WAL after a long read transaction has
// ended, on its own connection (passiveCheckpoint) and in a bounded retry loop:
// a BUSY pass, or a pass that a reader's mark kept from finishing, is retried
// after a pause, until every frame is backfilled or postLongReadCheckpointBudget
// (or ctx) is over. Each pass logs the frames backfilled and the time it took;
// the final pass is the one WALStats reports. It blocks no reader and no writer
// for its own sake (the rules above); a writer's automatic checkpoint, if on,
// can still stall the write connection, which is why the capture suspends it.
func (s *Store) CheckpointAfterLongRead(ctx context.Context, reason string) {
	if s == nil || s.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, postLongReadCheckpointBudget)
	defer cancel()
	started := time.Now()
	cp := WALCheckpoint{Reason: reason, At: started.UTC(), WALBefore: s.walFileBytes()}
	var err error
	pause := 250 * time.Millisecond
	for pass := 1; ; pass++ {
		passStart := time.Now()
		var busy, logFrames, checkpointed int64
		busy, logFrames, checkpointed, err = s.passiveCheckpoint(ctx)
		if err == nil {
			cp.Busy, cp.LogFrames, cp.Checkpointed = busy, logFrames, checkpointed
			log.Printf("store: wal bound: checkpoint after %s, pass %d: busy=%d log=%d checkpointed=%d in %s",
				reason, pass, busy, logFrames, checkpointed, time.Since(passStart).Round(time.Millisecond))
			if checkpointKnown(busy, logFrames, checkpointed) && checkpointed >= logFrames {
				break
			}
		}
		if ctx.Err() != nil {
			if err == nil {
				err = fmt.Errorf("the %s budget for the checkpoint after %s ended with the WAL not fully backfilled (log=%d checkpointed=%d)", postLongReadCheckpointBudget, reason, cp.LogFrames, cp.Checkpointed)
			}
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(pause):
		}
		if pause < 5*time.Second {
			pause *= 2
		}
	}
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

// WALFrames reports the number of frames in the WAL (the `log` counter of
// SQLite's wal_checkpoint) by running a PASSIVE checkpoint on its own
// connection (passiveCheckpoint), which also backfills what no reader holds
// back. Unlike the WAL file's size, which stays at the journal size limit's
// slack and is reused from its start, the count shows how much a long read has
// made the WAL hold. A busy answer (log=-1) is retried once and then reported
// as ErrWALFramesUnknown, never as a count.
func (s *Store) WALFrames(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store: no state database")
	}
	for attempt := 0; attempt < 2; attempt++ {
		busy, logFrames, checkpointed, err := s.passiveCheckpoint(ctx)
		if err != nil {
			return 0, err
		}
		if checkpointKnown(busy, logFrames, checkpointed) {
			return logFrames, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return 0, ErrWALFramesUnknown
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
