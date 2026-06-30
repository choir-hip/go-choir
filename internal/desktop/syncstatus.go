package desktop

import (
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
)

// SyncPhase describes the high-level state of the sync engine.
type SyncPhase string

const (
	PhaseIdle      SyncPhase = "idle"
	PhaseScanning  SyncPhase = "scanning"
	PhaseFetching  SyncPhase = "fetching"
	PhasePlanning  SyncPhase = "planning"
	PhaseExecuting SyncPhase = "executing"
	PhaseConflicts SyncPhase = "conflicts"
	PhaseError     SyncPhase = "error"
	PhaseCancelled SyncPhase = "cancelled"
)

// ItemStatus is the per-item sync state surfaced to the UI. It is derived
// from the planner's actions and conflicts after a sync cycle.
type ItemStatus struct {
	ItemID       model.ItemID    `json:"item_id"`
	LocalItemID  model.ItemID    `json:"local_item_id,omitempty"`
	RemoteItemID model.ItemID    `json:"remote_item_id,omitempty"`
	Path         string          `json:"path,omitempty"`
	State        model.SyncState `json:"state"`
	LocalVer     model.VersionID `json:"local_version_id,omitempty"`
	RemoteVer    model.VersionID `json:"remote_version_id,omitempty"`
	SyncedVer    model.VersionID `json:"synced_version_id,omitempty"`
	UpdatedAt    time.Time       `json:"updated_at,omitempty"`
}

// SyncProgress is the overall sync status reported to the frontend.
type SyncProgress struct {
	Phase          SyncPhase    `json:"phase"`
	LastSyncAt     *time.Time   `json:"last_sync_at,omitempty"`
	Cursor         int64        `json:"cursor"`
	RemoteHead     int64        `json:"remote_head"`
	ActionsTotal   int          `json:"actions_total"`
	ActionsDone    int          `json:"actions_done"`
	ConflictsCount int          `json:"conflicts_count"`
	LastError      string       `json:"last_error,omitempty"`
	Items          []ItemStatus `json:"items,omitempty"`
}

// StatusTracker is a concurrency-safe holder of sync progress and per-item
// status. The sync engine updates it during a cycle; the desktop frontend
// reads it via the Wails service methods.
type StatusTracker struct {
	mu       sync.RWMutex
	progress SyncProgress
	items    map[model.ItemID]ItemStatus
}

// NewStatusTracker returns an empty tracker in the idle phase.
func NewStatusTracker() *StatusTracker {
	return &StatusTracker{
		items: make(map[model.ItemID]ItemStatus),
		progress: SyncProgress{
			Phase: PhaseIdle,
		},
	}
}

// SetPhase updates the current sync phase.
func (s *StatusTracker) SetPhase(p SyncPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress.Phase = p
}

// SetCursor records the current synced cursor and remote head.
func (s *StatusTracker) SetCursor(cursor, head int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress.Cursor = cursor
	s.progress.RemoteHead = head
}

// SetActionTotals records the number of actions to execute and resets the
// done counter.
func (s *StatusTracker) SetActionTotals(total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress.ActionsTotal = total
	s.progress.ActionsDone = 0
}

// ActionDone increments the completed-action counter.
func (s *StatusTracker) ActionDone() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress.ActionsDone++
}

// SetError records the last error and moves to the error phase.
func (s *StatusTracker) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress.LastError = err
	s.progress.Phase = PhaseError
}

// MarkSynced records a successful sync completion timestamp and returns to
// idle.
func (s *StatusTracker) MarkSynced() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.progress.LastSyncAt = &now
	s.progress.Phase = PhaseIdle
	s.progress.LastError = ""
}

// MarkCancelled moves to the cancelled phase.
func (s *StatusTracker) MarkCancelled() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress.Phase = PhaseCancelled
}

// UpdateFromPlan derives per-item status from the planner output and the
// local/remote trees. Each action and conflict becomes an ItemStatus entry.
// This is the bridge between the pure planner and the UI-visible status.
func (s *StatusTracker) UpdateFromPlan(actions []planner.Action, conflicts []planner.Conflict, local, remote planner.Tree) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progress.ConflictsCount = len(conflicts)
	// Reset per-item map for this cycle.
	s.items = make(map[model.ItemID]ItemStatus, len(actions)+len(conflicts))
	now := time.Now().UTC()

	for _, a := range actions {
		st := ItemStatus{
			ItemID:    a.ItemID,
			UpdatedAt: now,
		}
		st.Path = RelPathFromID(local, a.ItemID)
		if st.Path == "" {
			st.Path = RelPathFromID(remote, a.ItemID)
		}
		switch a.Type {
		case planner.ActionUpload, planner.ActionUpdateRemote, planner.ActionMoveRemote:
			st.State = model.StateLocalOnly
			if v, ok := local.Versions[a.ItemID]; ok {
				st.LocalVer = v.VersionID
			}
		case planner.ActionDownload, planner.ActionUpdateLocal, planner.ActionMoveLocal:
			st.State = model.StateRemoteOnly
			if v, ok := remote.Versions[a.ItemID]; ok {
				st.RemoteVer = v.VersionID
			}
		case planner.ActionDeleteLocal, planner.ActionDeleteRemote:
			st.State = model.StateSynced // deletion converges
		default:
			st.State = model.StateSynced
		}
		s.items[a.ItemID] = st
	}

	for _, c := range conflicts {
		localID := conflictLocalItemID(c)
		remoteID := conflictRemoteItemID(c)
		st := ItemStatus{
			ItemID:       c.ItemID,
			LocalItemID:  localID,
			RemoteItemID: remoteID,
			State:        model.StateConflict,
			UpdatedAt:    now,
		}
		st.Path = RelPathFromID(local, localID)
		if st.Path == "" {
			st.Path = RelPathFromID(remote, remoteID)
		}
		st.LocalVer = c.LocalVer.VersionID
		st.RemoteVer = c.RemoteVer.VersionID
		st.SyncedVer = c.SyncedVer.VersionID
		// A conflict overrides any action-derived status for the same item.
		s.items[c.ItemID] = st
	}

	// Flatten into the progress payload for the frontend.
	s.progress.Items = make([]ItemStatus, 0, len(s.items))
	for _, v := range s.items {
		s.progress.Items = append(s.progress.Items, v)
	}
}

// Snapshot returns a copy of the current sync progress. Safe for concurrent
// use by the frontend reader.
func (s *StatusTracker) Snapshot() SyncProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.progress
	if len(s.progress.Items) > 0 {
		out.Items = append([]ItemStatus(nil), s.progress.Items...)
	}
	return out
}
