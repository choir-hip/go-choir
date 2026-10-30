package store

// The embedded-engine mutex and its per-call-path counters are gone with the
// move to SQLite (one writer plus a read pool; no process-wide engine lock).
// Tests that asserted "this read path is not scanned" through those counters
// keep their functional assertions; the counters read empty.

type engineMutexOpStats struct{ Calls uint64 }

func (s *Store) EngineMutexStats() map[string]engineMutexOpStats { return nil }

type engineMutexLive struct{ HolderOp string }

func (s *Store) EngineMutexLive() *engineMutexLive { return nil }
