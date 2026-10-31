package store

// The event index window (docs/definitions/choir-lean-system-2026-10-30.md,
// station L1). 99.3% of the guest's computer_event_index rows are finalized
// projection_batch_recorded events keyed "projection-batch:"+random id, which
// nothing looks up: replay and restore read the host tape, and head and
// continuity come from computer_event_projection_heads. The index keeps only
// the last window of them.
//
// The content witness hashes every row of computer_event_index except its
// timestamp columns (agentcore/restore_snapshot.go replayStateExtractor), and
// the replay-completeness probe compares the live witness with a replay's.
// So the trim is a pure function of the head sequence, never of wall time:
// when a finalize lands on a multiple of projectionBatchTrimInterval, every
// finalized projection_batch_recorded row at or below
// projectionBatchTrimBoundary(head) goes. A replay finalizes the same events
// in the same order and trims at the same sequences, so it ends on the same
// rows. Every other kind, and every prepared row, stays.
//
// The one-time catch-up (CatchUpProjectionBatchTrim) deletes to the same
// boundary in chunks, in a goroutine Open starts and Close stops. It never
// runs on the boot path: Open comes before the credential exchange, and the
// boot envelope expires 15 minutes after vmctl mints it (the paper's
// 11efa88c3 boot spent 10 minutes here and failed).

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

const (
	// projectionBatchTrimWindow is how many sequences of batch rows stay
	// below the newest trim point.
	projectionBatchTrimWindow uint64 = 10000
	// projectionBatchTrimInterval is the head-sequence multiple a trim runs at.
	projectionBatchTrimInterval uint64 = 10000
	// projectionBatchTrimFloorEpoch is the store floor written before the
	// first delete (storeschema.Version 4).
	projectionBatchTrimFloorEpoch  uint64 = 4
	projectionBatchTrimFloorReason        = "computer_event_index windowed: epoch 4 is the floor (an older binary's witness over the trimmed index would not replay)"
)

// Compile-time guard: the build's epoch must reach the floor it writes.
const _ = storeschema.Version - projectionBatchTrimFloorEpoch

var (
	// projectionBatchCatchUpChunk is the rows one catch-up transaction
	// deletes. Each chunk holds the event lock, so it is small enough that a
	// live append waits well under a second.
	projectionBatchCatchUpChunk = 5000
	// projectionBatchCatchUpPause yields the single writer between chunks.
	projectionBatchCatchUpPause = 100 * time.Millisecond
	// projectionBatchCatchUpBudget bounds one catch-up; what remains is
	// finished by the next boot's catch-up.
	projectionBatchCatchUpBudget = 6 * time.Hour
)

// projectionBatchTrimBoundary is the highest sequence whose batch row the trim
// at head deletes: the newest multiple of the interval at or below head, less
// the window. Zero means nothing is deletable yet.
func projectionBatchTrimBoundary(head uint64) uint64 {
	point := head / projectionBatchTrimInterval * projectionBatchTrimInterval
	if point <= projectionBatchTrimWindow {
		return 0
	}
	return point - projectionBatchTrimWindow
}

func (s *Store) ensureProjectionBatchTrimFloor() error {
	if s.texturePath == "" {
		return nil
	}
	return storeschema.WriteFloor(s.texturePath, projectionBatchTrimFloorEpoch, projectionBatchTrimFloorReason)
}

// trimProjectionBatchIndexTx runs inside finalizeBatch's transaction, after
// the row is finalized and the head updated. sequence is the new head. It
// acts only when the finalize lands on a trim point.
func (s *Store) trimProjectionBatchIndexTx(ctx context.Context, tx *sql.Tx, computerID string, sequence uint64) error {
	if sequence == 0 || sequence%projectionBatchTrimInterval != 0 {
		return nil
	}
	boundary := projectionBatchTrimBoundary(sequence)
	if boundary == 0 {
		return nil
	}
	// The floor first, durably: no row is deleted on an unfloored store.
	if err := s.ensureProjectionBatchTrimFloor(); err != nil {
		return fmt.Errorf("computer event projection: trim: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM computer_event_index WHERE computer_id=? AND event_kind=? AND status='finalized' AND sequence<=?`,
		computerID, string(computerevent.EventProjectionBatchRecorded), boundary); err != nil {
		return fmt.Errorf("computer event projection: trim batch rows: %w", err)
	}
	return nil
}

// CatchUpProjectionBatchTrim deletes, in chunks, the finalized batch rows at
// or below each computer's trim boundary at its current head, the state a
// replay to that head would leave. It returns the rows deleted. Each chunk is
// its own transaction under the event lock, with a pause between, so the one
// writer connection is never held for more than a chunk. Idempotent.
func (s *Store) CatchUpProjectionBatchTrim(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT computer_id, sequence FROM computer_event_projection_heads`)
	if err != nil {
		return 0, fmt.Errorf("trim catch-up: read heads: %w", err)
	}
	type headRow struct {
		id  string
		seq uint64
	}
	var heads []headRow
	for rows.Next() {
		var h headRow
		if err := rows.Scan(&h.id, &h.seq); err != nil {
			_ = rows.Close()
			return 0, err
		}
		heads = append(heads, h)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()
	var total int64
	for _, h := range heads {
		boundary := projectionBatchTrimBoundary(h.seq)
		if boundary == 0 {
			continue
		}
		for {
			if err := ctx.Err(); err != nil {
				return total, err
			}
			n, err := s.trimChunk(ctx, h.id, boundary)
			if err != nil {
				return total, err
			}
			total += n
			if n < int64(projectionBatchCatchUpChunk) {
				break
			}
			select {
			case <-ctx.Done():
				return total, ctx.Err()
			case <-time.After(projectionBatchCatchUpPause):
			}
		}
	}
	return total, nil
}

// trimChunk deletes up to one chunk under the event lock (a finalize that
// advanced the head meanwhile only moves the true boundary up, so the
// caller's boundary stays one a replay would also have passed).
func (s *Store) trimChunk(ctx context.Context, computerID string, boundary uint64) (int64, error) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM computer_event_index WHERE computer_id=? AND event_kind=? AND status='finalized' AND sequence<=? LIMIT 1`,
		computerID, string(computerevent.EventProjectionBatchRecorded), boundary).Scan(&exists)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("trim catch-up: probe: %w", err)
	}
	if err := s.ensureProjectionBatchTrimFloor(); err != nil {
		return 0, fmt.Errorf("trim catch-up: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM computer_event_index WHERE rowid IN (SELECT rowid FROM computer_event_index WHERE computer_id=? AND event_kind=? AND status='finalized' AND sequence<=? LIMIT ?)`,
		computerID, string(computerevent.EventProjectionBatchRecorded), boundary, projectionBatchCatchUpChunk)
	if err != nil {
		return 0, fmt.Errorf("trim catch-up: delete: %w", err)
	}
	return res.RowsAffected()
}

// startProjectionBatchCatchUp runs the catch-up in a goroutine that Close
// cancels and drains with the lease goroutines: bounded, logged, never fatal.
// A store that predates the window, or an older snapshot restored under this
// binary, is brought to the state a replay would leave. Open and Reopen start
// it on the published store, never on the boot path.
func (s *Store) startProjectionBatchCatchUp() {
	closing := s.leaseClosingChan()
	ctx, cancel := context.WithTimeout(context.Background(), projectionBatchCatchUpBudget)
	s.lease.workers.Add(1)
	go func() {
		defer s.lease.workers.Add(-1)
		defer cancel()
		go func() {
			select {
			case <-closing:
				cancel()
			case <-ctx.Done():
			}
		}()
		s.runProjectionBatchCatchUp(ctx)
	}()
}

func (s *Store) runProjectionBatchCatchUp(ctx context.Context) {
	start := time.Now()
	n, err := s.CatchUpProjectionBatchTrim(ctx)
	if err != nil {
		log.Printf("store: event index trim catch-up: deleted %d rows in %s, stopped: %v (the next trim point finishes it)", n, time.Since(start).Round(time.Millisecond), err)
		return
	}
	if n > 0 {
		log.Printf("store: event index trim catch-up: deleted %d projection_batch_recorded rows in %s", n, time.Since(start).Round(time.Millisecond))
	}
}
