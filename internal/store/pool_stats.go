package store

import "database/sql"

// PoolStats is the store's two connection pools as database/sql counts
// them. The write pool holds one connection, so
// WaitCount and WaitDurationMS are how often and how long callers waited for
// it: their growth between store_stalled 503s shows how loaded the one write
// connection is (docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md).
type PoolStats struct {
	Write PoolStat `json:"write"`
	Read  PoolStat `json:"read"`
}

// PoolStat is one pool's sql.DBStats, durations in milliseconds.
type PoolStat struct {
	MaxOpen        int   `json:"max_open"`
	Open           int   `json:"open"`
	InUse          int   `json:"in_use"`
	Idle           int   `json:"idle"`
	WaitCount      int64 `json:"wait_count"`
	WaitDurationMS int64 `json:"wait_duration_ms"`
}

func poolStat(db *sql.DB) PoolStat {
	if db == nil {
		return PoolStat{}
	}
	st := db.Stats()
	return PoolStat{
		MaxOpen:        st.MaxOpenConnections,
		Open:           st.OpenConnections,
		InUse:          st.InUse,
		Idle:           st.Idle,
		WaitCount:      st.WaitCount,
		WaitDurationMS: st.WaitDuration.Milliseconds(),
	}
}

// PoolStats reads both pools' counters. It takes no lock and never waits for
// a connection, so it answers during a stall.
func (s *Store) PoolStats() *PoolStats {
	if s == nil {
		return nil
	}
	return &PoolStats{Write: poolStat(s.db), Read: poolStat(s.readDB)}
}
