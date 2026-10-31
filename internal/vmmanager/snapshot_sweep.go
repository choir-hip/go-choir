package vmmanager

import (
	"log"
	"os"
	"sync"
	"time"
)

// Dead-snapshot sweep (lean-system ledger, 2026-10-31).
//
// A hibernated computer's snapshot holds its whole guest memory (8 GiB). A
// snapshot that validateSnapshot would refuse for a reason that can only get
// worse with time (age, a moved release floor, security epoch, ABI or support
// window) can never be loaded, yet it kept its bytes until its computer woke.
// The sweep removes such snapshots on a timer. The computer it belonged to
// cold boots on its next wake, which it would have done anyway.
//
// The sweep reuses the wake path's own predicates (busySnapshotRefusal's
// rules and releaseRefusal) and is conservative where the wake path would
// refuse for a reason that may be transient or unknown: R0 (switch off), R1
// (release directory unreadable) and R2 (current release unknown, e.g. mid
// deploy) are never a reason to delete. Host-fingerprint reasons (firecracker
// version, kernel, cpu, launch config) are not swept: they need a launch
// context.

const (
	snapshotSweepInterval = 15 * time.Minute
	// snapshotSweepPoll is how often the loop checks whether the current
	// release moved, which triggers an extra pass.
	snapshotSweepPoll = time.Minute
)

// SnapshotSweepStats are the sweep's counters, served by GuestReleaseUsage.
type SnapshotSweepStats struct {
	Passes      int64            `json:"passes"`
	Discarded   int64            `json:"discarded"`
	Skipped     int64            `json:"skipped_locked"`
	BytesFreed  int64            `json:"bytes_freed"`
	ByReason    map[string]int64 `json:"by_reason"`
	LastPassUTC string           `json:"last_pass_utc,omitempty"`
}

type snapshotSweepState struct {
	mu    sync.Mutex
	stats SnapshotSweepStats
}

// SnapshotSweepStats returns a copy of the sweep counters.
func (m *Manager) SnapshotSweepStats() SnapshotSweepStats {
	m.sweepState.mu.Lock()
	defer m.sweepState.mu.Unlock()
	out := m.sweepState.stats
	out.ByReason = map[string]int64{}
	for k, v := range m.sweepState.stats.ByReason {
		out.ByReason[k] = v
	}
	return out
}

// snapshotNeverResumable returns a reason when the snapshot can never be
// loaded, else "" (loadable, or not known to be unloadable).
func (m *Manager) snapshotNeverResumable(meta *SnapshotMeta, now time.Time) string {
	if meta == nil || meta.CreatedAt.IsZero() {
		return ""
	}
	if meta.Consumed {
		return snapshotReasonConsumed
	}
	if !now.Before(meta.CreatedAt.Add(m.snapshotMaxAge())) {
		return snapshotReasonMaxAge
	}
	if meta.BusyHostStop {
		if !now.Before(meta.CreatedAt.Add(m.busyHibernateMaxAge())) {
			return snapshotReasonBusyMaxAge
		}
		// A busy snapshot loads only onto the current release; an
		// unknown current release is not "changed".
		if meta.GuestRelease != "" {
			if cur := m.currentReleaseIDFor(meta.ComputerID); cur != "" && cur != meta.GuestRelease {
				return snapshotReasonBusyReleaseChanged
			}
		}
	}
	return releaseDeadReason(m.releaseRefusal(meta, now))
}

// releaseDeadReason keeps only the release refusals that are permanent: the
// floor, security epoch, ABI and support-window moves (R3 to R6).
func releaseDeadReason(refusal string) string {
	switch refusal {
	case snapshotReasonReleaseFloorMoved, snapshotReasonReleaseSecurityEpoch,
		snapshotReasonReleaseABI, snapshotReasonReleaseOutOfSupport:
		return refusal
	}
	return ""
}

// SweepDeadSnapshots runs one pass over every committed snapshot in every
// root and discards those that can never be resumed. It returns the VM ids
// discarded. A VM whose operation lock is held is skipped; the verdict is
// re-taken under the lock from a fresh read, so a snapshot replaced between
// the scan and the lock is judged on its own merits.
func (m *Manager) SweepDeadSnapshots() []string {
	now := time.Now()
	var discarded []string
	var skipped, freed int64
	byReason := map[string]int64{}
	for _, entry := range m.listSnapshots() {
		if m.snapshotNeverResumable(entry.meta, now) == "" {
			continue
		}
		unlock, ok := m.tryLockVMOperation(entry.vmID)
		if !ok {
			skipped++
			continue
		}
		meta, err := readSnapshotMeta(entry.dir)
		reason := ""
		if err == nil {
			reason = m.snapshotNeverResumable(meta, now)
		}
		if reason != "" && m.discardSnapshotFiles(entry.vmID) {
			log.Printf("vmmanager: snapshot sweep: discarded dead snapshot of VM %s (%s, %d bytes, created %s)",
				entry.vmID, reason, entry.bytes, entry.meta.CreatedAt.UTC().Format(time.RFC3339))
			discarded = append(discarded, entry.vmID)
			byReason[reason]++
			freed += entry.bytes
		}
		unlock()
	}
	m.sweepState.mu.Lock()
	st := &m.sweepState.stats
	st.Passes++
	st.Discarded += int64(len(discarded))
	st.Skipped += skipped
	st.BytesFreed += freed
	if st.ByReason == nil {
		st.ByReason = map[string]int64{}
	}
	for k, v := range byReason {
		st.ByReason[k] += v
	}
	st.LastPassUTC = now.UTC().Format(time.RFC3339)
	m.sweepState.mu.Unlock()
	return discarded
}

// snapshotSweepLoop sweeps every snapshotSweepInterval and once more right
// after the current guest release id changes (a deploy moved the floor).
func (m *Manager) snapshotSweepLoop(cancel <-chan struct{}) {
	if _, err := os.Stat(m.cfg.StateDir); err != nil {
		return
	}
	last := m.currentReleaseID()
	lastPass := time.Now()
	ticker := time.NewTicker(snapshotSweepPoll)
	defer ticker.Stop()
	for {
		select {
		case <-cancel:
			return
		case <-ticker.C:
			cur := m.currentReleaseID()
			moved := cur != "" && cur != last
			if cur != "" {
				last = cur
			}
			if moved || time.Since(lastPass) >= snapshotSweepInterval {
				lastPass = time.Now()
				m.SweepDeadSnapshots()
			}
		}
	}
}
