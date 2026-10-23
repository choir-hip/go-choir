package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Derived columns of og_objects (generated columns and their indexes) are
// added at open only to an empty table. On a store with rows the boot is
// discovery only, and adding them is this explicit, per-computer operation
// (docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md):
// one migration at a time, in the background, cancellable, its progress
// readable while it runs. Nothing starts it but a caller; no boot resumes it.

// Derived column migration states.
const (
	DerivedColumnMigrationIdle      = "idle"
	DerivedColumnMigrationRunning   = "running"
	DerivedColumnMigrationDone      = "done"
	DerivedColumnMigrationFailed    = "failed"
	DerivedColumnMigrationCancelled = "cancelled"
)

// ErrDerivedColumnMigrationRunning refuses a second concurrent migration.
var ErrDerivedColumnMigrationRunning = errors.New("runtime store: derived column migration already running")

// derivedColumnStepHook, when set by a test, runs in the migration
// goroutine before its first step.
var derivedColumnStepHook func()

// DerivedColumnMigrationStatus is the state of this process's last derived
// column migration, and the access-path plan: discovered now when no
// migration has run, otherwise as of the migration's last step.
type DerivedColumnMigrationStatus struct {
	State     string                       `json:"state"`
	StartedAt string                       `json:"started_at,omitempty"`
	EndedAt   string                       `json:"ended_at,omitempty"`
	Steps     []objectgraph.AccessPathStep `json:"steps"`
	Plan      objectgraph.AccessPathPlan   `json:"plan"`
	Error     string                       `json:"error,omitempty"`
	// RunMemoryKindIndex is nil until the store reports it.
	RunMemoryKindIndex *RunMemoryKindIndexStatus `json:"run_memory_kind_index,omitempty"`
	// RunMemoryLoopSeqIndex is the duplicate index the operation drops (R5).
	RunMemoryLoopSeqIndex *RunMemoryLoopSeqIndexStatus `json:"run_memory_loop_seq_index,omitempty"`
}

// RunMemoryKindIndexStatus is the state of the run_memory_entries kind index,
// the one access path of this operation that is not on og_objects.
type RunMemoryKindIndexStatus struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	HasRows bool   `json:"has_rows"`
}

// RunMemoryLoopSeqIndexStatus is the state of the duplicate
// idx_run_memory_entries_loop_seq: present means it still costs a tree per
// append; CoveredByUniqueKey says UNIQUE(loop_id, seq) can take its place.
type RunMemoryLoopSeqIndexStatus struct {
	Name               string `json:"name"`
	Present            bool   `json:"present"`
	HasRows            bool   `json:"has_rows"`
	CoveredByUniqueKey bool   `json:"covered_by_unique_key"`
}

type derivedColumnMigration struct {
	mu     sync.Mutex
	status DerivedColumnMigrationStatus
	cancel context.CancelFunc
}

// DiscoverObjectGraphAccessPaths rediscovers which derived columns and
// indexes og_objects has, for the write and the read store, and logs the
// plan. It runs no DDL; the boot calls it after replay.
func (s *Store) DiscoverObjectGraphAccessPaths(ctx context.Context) (objectgraph.AccessPathPlan, error) {
	if s == nil || s.ogStore == nil {
		return objectgraph.AccessPathPlan{}, fmt.Errorf("runtime store: object graph not initialized")
	}
	plan, err := s.ogStore.DiscoverAccessPaths(ctx)
	if err != nil {
		return plan, err
	}
	s.inheritAccessPaths()
	if len(plan.MissingColumns)+len(plan.MissingIndexes) > 0 {
		log.Printf("runtime store: derived access paths missing (columns %v, indexes %v); reads on JSON until POST /internal/store/derived-columns?action=migrate",
			plan.MissingColumns, plan.MissingIndexes)
	}
	return plan, nil
}

func (s *Store) inheritAccessPaths() {
	if s.ogReadStore != nil {
		s.ogReadStore.InheritMetadataColumns(s.ogStore)
	}
}

// StartDerivedColumnMigration starts adding the missing derived columns and
// indexes in the background and returns the running status. It refuses
// while one runs.
func (s *Store) StartDerivedColumnMigration() (DerivedColumnMigrationStatus, error) {
	if s == nil || s.ogStore == nil {
		return DerivedColumnMigrationStatus{}, fmt.Errorf("runtime store: object graph not initialized")
	}
	m := &s.derivedColumns
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State == DerivedColumnMigrationRunning {
		return m.status, ErrDerivedColumnMigrationRunning
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.status = DerivedColumnMigrationStatus{State: DerivedColumnMigrationRunning, StartedAt: time.Now().UTC().Format(time.RFC3339), Steps: []objectgraph.AccessPathStep{}}
	// A status read during the run must not touch the engine, so the index's
	// state at the start is taken here, bounded, and refreshed when the run ends.
	probeCtx, probeCancel := context.WithTimeout(ctx, 2*time.Second)
	if idx, err := s.discoverRunMemoryKindIndex(probeCtx); err == nil {
		m.status.RunMemoryKindIndex = &idx
	}
	if idx, err := s.discoverRunMemoryLoopSeqIndex(probeCtx); err == nil {
		m.status.RunMemoryLoopSeqIndex = &idx
	}
	probeCancel()
	og := s.ogStore
	go s.runDerivedColumnMigration(ctx, og)
	return m.status, nil
}

func (s *Store) runDerivedColumnMigration(ctx context.Context, og *objectgraph.DoltStore) {
	m := &s.derivedColumns
	if hook := derivedColumnStepHook; hook != nil {
		hook()
	}
	log.Printf("runtime store: derived column migration started")
	_, err := og.MigrateAccessPaths(ctx, func(step objectgraph.AccessPathStep) {
		if og == s.ogStore {
			s.inheritAccessPaths()
		}
		m.mu.Lock()
		m.status.Steps = append(m.status.Steps, step)
		m.mu.Unlock()
	})
	// The run-memory kind index is one more step of the operation, after the
	// og_objects ones; a cancel before it skips it.
	var indexErr error
	if ctx.Err() == nil {
		m.mu.Lock()
		number := len(m.status.Steps) + 1
		m.mu.Unlock()
		step, stepErr := s.migrateRunMemoryKindIndex(ctx, number)
		indexErr = stepErr
		if step != nil {
			m.mu.Lock()
			m.status.Steps = append(m.status.Steps, *step)
			m.mu.Unlock()
		}
	}
	// R5: the duplicate loop_seq index is dropped after the kind index; a
	// cancel or a failure before it skips it.
	var dropErr error
	if ctx.Err() == nil && indexErr == nil {
		m.mu.Lock()
		number := len(m.status.Steps) + 1
		m.mu.Unlock()
		step, stepErr := s.migrateRunMemoryLoopSeqIndex(ctx, number)
		dropErr = stepErr
		if step != nil {
			m.mu.Lock()
			m.status.Steps = append(m.status.Steps, *step)
			m.mu.Unlock()
		}
	}
	indexStatus, indexStatusErr := s.discoverRunMemoryKindIndex(context.Background())
	loopSeqStatus, loopSeqStatusErr := s.discoverRunMemoryLoopSeqIndex(context.Background())
	// The final plan is read fresh: a cancel can land before the first
	// discovery.
	plan, planErr := og.DiscoverAccessPaths(context.Background())
	if planErr == nil && og == s.ogStore {
		s.inheritAccessPaths()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Plan = plan
	if indexStatusErr == nil {
		m.status.RunMemoryKindIndex = &indexStatus
	}
	if loopSeqStatusErr == nil {
		m.status.RunMemoryLoopSeqIndex = &loopSeqStatus
	}
	m.status.EndedAt = time.Now().UTC().Format(time.RFC3339)
	switch {
	case errors.Is(err, context.Canceled):
		m.status.State = DerivedColumnMigrationCancelled
	case err != nil:
		m.status.State = DerivedColumnMigrationFailed
		m.status.Error = err.Error()
	case errors.Is(indexErr, context.Canceled):
		m.status.State = DerivedColumnMigrationCancelled
	case indexErr != nil:
		m.status.State = DerivedColumnMigrationFailed
		m.status.Error = indexErr.Error()
	case errors.Is(dropErr, context.Canceled):
		m.status.State = DerivedColumnMigrationCancelled
	case dropErr != nil:
		m.status.State = DerivedColumnMigrationFailed
		m.status.Error = dropErr.Error()
	case ctx.Err() != nil:
		// A cancel landed after the og_objects part and before the index step
		// (or between): what was not run is not done.
		m.status.State = DerivedColumnMigrationCancelled
	case planErr != nil:
		m.status.State = DerivedColumnMigrationFailed
		m.status.Error = planErr.Error()
	default:
		m.status.State = DerivedColumnMigrationDone
	}
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	log.Printf("runtime store: derived column migration %s (%d steps)", m.status.State, len(m.status.Steps))
}

// CancelDerivedColumnMigration cancels a running migration before its next
// step (a step already holding the engine runs to its end). It reports
// whether one was running.
func (s *Store) CancelDerivedColumnMigration() bool {
	if s == nil {
		return false
	}
	m := &s.derivedColumns
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State != DerivedColumnMigrationRunning || m.cancel == nil {
		return false
	}
	m.cancel()
	return true
}

// DerivedColumnMigrationStatus returns the migration's status. While one
// runs it does not touch the engine.
func (s *Store) DerivedColumnMigrationStatus(ctx context.Context) DerivedColumnMigrationStatus {
	if s == nil || s.ogStore == nil {
		return DerivedColumnMigrationStatus{State: DerivedColumnMigrationIdle, Error: "object graph not initialized"}
	}
	m := &s.derivedColumns
	m.mu.Lock()
	status := m.status
	status.Steps = append([]objectgraph.AccessPathStep{}, m.status.Steps...)
	m.mu.Unlock()
	if status.State == "" {
		status.State = DerivedColumnMigrationIdle
		plan, err := s.ogStore.DiscoverAccessPaths(ctx)
		if err != nil {
			status.Error = err.Error()
		}
		status.Plan = plan
		if idx, err := s.discoverRunMemoryKindIndex(ctx); err == nil {
			status.RunMemoryKindIndex = &idx
		} else if status.Error == "" {
			status.Error = err.Error()
		}
		if idx, err := s.discoverRunMemoryLoopSeqIndex(ctx); err == nil {
			status.RunMemoryLoopSeqIndex = &idx
		} else if status.Error == "" {
			status.Error = err.Error()
		}
	}
	return status
}
