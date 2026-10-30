package store

import (
	"context"
	"errors"
	"fmt"
)

// A replay commits each event at least twice (Prepare, then the finalize
// transaction), and the write connection runs at synchronous=FULL, so each
// commit syncs the WAL: about 2.7 ms an event on d@e (2026-10-30). In WAL mode
// synchronous=NORMAL cannot corrupt the database; a crash can only roll back
// the newest commits, and a replayed commit is re-derivable from the tape, so
// the next boot replays it again from the durable head. The write connection
// is the store's only one (dbx: one open, one idle, no lifetime), so the
// pragma holds until restored; a reopened connection comes back at FULL.

// RelaxReplayDurability runs the write connection at synchronous=NORMAL for
// a replay and returns the restore, which sets FULL again and checkpoints the
// WAL so every replayed commit is on disk before a live write follows
// (implements computerevent.ReplayDurability).
func (s *Store) RelaxReplayDurability(ctx context.Context) (func(context.Context) error, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store: relax replay durability: nil store")
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA synchronous=NORMAL`); err != nil {
		return nil, fmt.Errorf("store: relax replay durability: %w", err)
	}
	s.replayRelaxed.Store(true)
	return func(ctx context.Context) error {
		s.replayRelaxed.Store(false)
		_, setErr := s.db.ExecContext(ctx, `PRAGMA synchronous=FULL`)
		_, syncErr := s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(FULL)`)
		return errors.Join(setErr, syncErr)
	}, nil
}

// syncRelaxedReplay makes a relaxed replay's commits durable at a replay
// checkpoint: the WAL checkpoint syncs the WAL before it copies frames.
func (s *Store) syncRelaxedReplay(ctx context.Context) error {
	if s == nil || s.db == nil || !s.replayRelaxed.Load() {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
		return fmt.Errorf("store: replay checkpoint sync: %w", err)
	}
	return nil
}
