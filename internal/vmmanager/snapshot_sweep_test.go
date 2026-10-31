package vmmanager

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type sweepScenario struct {
	name   string
	mutate func(t *testing.T, r *releaseFixture, meta *SnapshotMeta)
	// sweep is the reason the sweep must give; "" means it must keep.
	sweep string
}

func sweepScenarios() []sweepScenario {
	week := 168 * time.Hour
	return []sweepScenario{
		{"resumable", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) { r.point(t, relR2) }, ""},
		{"R0 switch off is not permanent", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			r.point(t, relR2)
			r.mgr.cfg.SnapshotReleaseRetain = false
		}, ""},
		{"R1 release dir gone is not swept", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			r.point(t, relR2)
			_ = os.RemoveAll(filepath.Join(r.releases, relR1))
		}, ""},
		{"R2 current unknown (deploy in flight) is not swept", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			_ = os.Remove(r.link)
		}, ""},
		{"R2 current has no directory is not swept", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) { r.point(t, relR3) }, ""},
		{"R3 floor moved", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			p := defaultRelPolicy(time.Hour)
			p.Digest = "sha256:floor-b"
			writeTestRelease(t, r.releases, relR3, p)
			r.point(t, relR3)
		}, snapshotReasonReleaseFloorMoved},
		{"R4 epoch raised", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			p := defaultRelPolicy(time.Hour)
			p.Epoch = 2
			writeTestRelease(t, r.releases, relR3, p)
			r.point(t, relR3)
		}, snapshotReasonReleaseSecurityEpoch},
		{"R5 abi below minimum", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			p := defaultRelPolicy(time.Hour)
			p.ABI, p.ABIMin = 2, 2
			writeTestRelease(t, r.releases, relR3, p)
			r.point(t, relR3)
		}, snapshotReasonReleaseABI},
		{"R6 out of support", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			setFirstCurrent(t, r.releases, relR1, week+time.Hour)
			r.point(t, relR2)
		}, snapshotReasonReleaseOutOfSupport},
		{"current release past the window", func(t *testing.T, r *releaseFixture, _ *SnapshotMeta) {
			setFirstCurrent(t, r.releases, relR1, week+time.Hour)
		}, ""},
		{"max age", func(t *testing.T, r *releaseFixture, meta *SnapshotMeta) {
			r.point(t, relR2)
			meta.CreatedAt = time.Now().Add(-r.mgr.snapshotMaxAge() - time.Minute)
		}, snapshotReasonMaxAge},
		{"created in the future (clock skew) is kept", func(t *testing.T, r *releaseFixture, meta *SnapshotMeta) {
			r.point(t, relR2)
			meta.CreatedAt = time.Now().Add(48 * time.Hour)
		}, ""},
		{"busy max age", func(t *testing.T, r *releaseFixture, meta *SnapshotMeta) {
			r.point(t, relR2)
			meta.BusyHostStop = true
			meta.CreatedAt = time.Now().Add(-r.mgr.busyHibernateMaxAge() - time.Minute)
		}, snapshotReasonBusyMaxAge},
		{"busy, current unknown is kept", func(t *testing.T, r *releaseFixture, meta *SnapshotMeta) {
			meta.BusyHostStop = true
			_ = os.Remove(r.link)
		}, ""},
	}
}

// Predicate parity: whatever the sweep calls dead, the wake path refuses, and
// the sweep never deletes on a reason the wake path might not give.
func TestSnapshotSweepPredicateMatchesWake(t *testing.T) {
	for _, tc := range sweepScenarios() {
		t.Run(tc.name, func(t *testing.T) {
			r := newReleaseFixture(t)
			dir, meta := r.hibernate(t)
			tc.mutate(t, r, meta)
			got := r.mgr.snapshotNeverResumable(meta, time.Now())
			if got != tc.sweep {
				t.Fatalf("sweep verdict = %q, want %q", got, tc.sweep)
			}
			wake := r.validate(dir, meta)
			if got != "" && wake == "" {
				t.Fatalf("sweep says dead (%s) but the wake path accepts it", got)
			}
			if got != "" && wake != got && !(meta.BusyHostStop || wake == snapshotReasonMaxAge) {
				t.Fatalf("sweep reason %q, wake reason %q", got, wake)
			}
		})
	}
}

// The release scenarios of the wake-path table, run through the real pass:
// dead snapshots are removed, kept ones stay with their bytes.
func TestSnapshotSweepRemovesOnlyDead(t *testing.T) {
	for _, tc := range sweepScenarios() {
		t.Run(tc.name, func(t *testing.T) {
			r := newReleaseFixture(t)
			dir, meta := r.hibernate(t)
			tc.mutate(t, r, meta)
			if err := writeSnapshotMeta(dir, meta); err != nil {
				t.Fatal(err)
			}
			got := r.mgr.SweepDeadSnapshots()
			_, statErr := os.Stat(dir)
			if tc.sweep != "" {
				if len(got) != 1 || !os.IsNotExist(statErr) {
					t.Fatalf("dead snapshot kept: removed=%v stat=%v", got, statErr)
				}
				if st := r.mgr.SnapshotSweepStats(); st.Discarded != 1 || st.ByReason[tc.sweep] != 1 {
					t.Fatalf("stats = %+v", st)
				}
			} else if len(got) != 0 || statErr != nil {
				t.Fatalf("live snapshot removed: removed=%v stat=%v", got, statErr)
			}
		})
	}
}

// A VM mid-operation (a wake loading its snapshot) is skipped, not raced.
func TestSnapshotSweepSkipsLockedVM(t *testing.T) {
	r := newReleaseFixture(t)
	dir, meta := r.hibernate(t)
	meta.CreatedAt = time.Now().Add(-r.mgr.snapshotMaxAge() - time.Hour)
	if err := writeSnapshotMeta(dir, meta); err != nil {
		t.Fatal(err)
	}
	unlock, ok := r.mgr.tryLockVMOperation(r.vmID)
	if !ok {
		t.Fatal("could not take the VM lock")
	}
	if got := r.mgr.SweepDeadSnapshots(); len(got) != 0 {
		t.Fatalf("swept a locked VM: %v", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("locked VM's snapshot removed")
	}
	if st := r.mgr.SnapshotSweepStats(); st.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", st.Skipped)
	}
	unlock()
	if got := r.mgr.SweepDeadSnapshots(); len(got) != 1 {
		t.Fatalf("after unlock swept %v, want the snapshot", got)
	}
}

// A partial capture directory is not a committed snapshot: the sweep leaves it
// (its capture holds the VM lock; SweepOrphanSnapshots clears it at start).
func TestSnapshotSweepLeavesPartialCapture(t *testing.T) {
	r := newReleaseFixture(t)
	partial := r.mgr.snapshotPartialDir("vm-partial")
	if err := os.MkdirAll(partial, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := r.mgr.SweepDeadSnapshots(); len(got) != 0 {
		t.Fatalf("swept %v", got)
	}
	if _, err := os.Stat(partial); err != nil {
		t.Fatal("partial capture removed")
	}
}

// A dead snapshot in the vault root is swept too.
func TestSnapshotSweepCoversVaultRoot(t *testing.T) {
	r := newReleaseFixture(t)
	vault := filepath.Join(t.TempDir(), "vault")
	r.mgr.cfg.SnapshotVaultDir = vault
	dir := filepath.Join(vault, "vm-vault", "snapshot")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	meta := SnapshotMeta{Version: snapshotMetaVersion, VMID: "vm-vault", GuestRelease: relR1,
		CreatedAt: time.Now().Add(-r.mgr.snapshotMaxAge() - time.Hour)}
	if err := writeSnapshotMeta(dir, &meta); err != nil {
		t.Fatal(err)
	}
	if got := r.mgr.SweepDeadSnapshots(); len(got) != 1 || got[0] != "vm-vault" {
		t.Fatalf("swept %v", got)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("vault snapshot kept")
	}
}

// GuestReleaseUsage stops counting a snapshot the release rules permanently
// refuse, so retention can free the old release; an R2 (unknown) one still pins.
func TestGuestReleaseUsageDropsDeadSnapshotPins(t *testing.T) {
	r := newReleaseFixture(t)
	r.hibernate(t)
	p := defaultRelPolicy(time.Hour)
	p.Digest = "sha256:floor-b"
	writeTestRelease(t, r.releases, relR3, p)

	pins := func() int {
		n := 0
		for _, e := range r.mgr.GuestReleaseUsage().InUse {
			n += len(e.Snapshots)
		}
		return n
	}
	_ = os.Remove(r.link) // R2: current unknown
	if pins() != 1 {
		t.Fatal("a snapshot under an unknown current release must still pin")
	}
	r.point(t, relR3) // R3: floor moved
	if pins() != 0 {
		t.Fatal("a snapshot refused for good must not pin its release")
	}
}
