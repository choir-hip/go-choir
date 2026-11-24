package vmctl

import (
	"testing"
	"time"
)

// A person whose computer cannot wake because the host is full gets a prompt
// refusal and the computer is exactly as it was. These run as production
// runs: make-room active (VMCTL_ADMISSION_MAKE_ROOM=active, 3 victims). The
// truth they pin: a wake that make-room can help hibernates idle computers
// (at most three) and is admitted; a wake it cannot help is refused with
// nobody hibernated. Failure modes guarded:
//  1. the wake waiting on the host (a hang the person reads as a broken
//     computer): the answer comes in under five seconds;
//  2. a half-started guest: a boot, resume, recover or refresh issued before
//     the refusal;
//  3. a consumed snapshot or a changed ownership (the computer must wake
//     later, from the same snapshot, when there is room);
//  4. a reservation left in the ledger by the refused start;
//  5. someone else's computer hibernated for a wake that was refused anyway;
//  6. the refusal not typed as host memory pressure (the proxy answers a
//     typed refusal with the plain busy sentence).
func TestAWakeMakeRoomCannotHelpIsRefusedPromptlyAndLeavesNothingStarted(t *testing.T) {
	// Production's make-room (nix/node-b.nix): active, 3 victims, a 10 s
	// settle wait, a 250 ms poll.
	prod := makeRoomAdmission()
	prod.MakeRoomMaxVictims = 3
	prod.MakeRoomSettleWait = 10 * time.Second
	prod.MakeRoomPollInterval = 250 * time.Millisecond
	f := newMakeRoomFixture(t, prod)
	own := hibernated("sleeper", "vm-sleeper", 3*gib)
	own.MemSizeMib = 4096
	addOwnership(f.reg, own)
	// Four idle computers of 1 GiB: all four would cover the shortfall but
	// only three may be hibernated for one start, so make-room cannot help.
	for _, u := range []string{"a", "b", "c", "d"} {
		f.activeComputer(u, "vm-"+u, 2*time.Hour, 1*gib)
	}
	f.setAvail(4*gib + 1*gib + 3*gib - 3584*mib)
	before := *own

	start := time.Now()
	_, err := f.reg.ResolveOrAssign("sleeper")
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("the refusal took %s, want under 5 s", took)
	}
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil || refusal.Kind != RecoveryRefusalHostMemoryPressure {
		t.Fatalf("err = %v, want a typed host_memory_pressure refusal", err)
	}
	m := f.mgr
	if len(m.boots)+len(m.resumes)+len(m.recovers)+len(m.refreshes)+len(m.reattaches) != 0 {
		t.Fatalf("a guest was started: boots %d resumes %v recovers %v refreshes %v reattaches %v", len(m.boots), m.resumes, m.recovers, m.refreshes, m.reattaches)
	}
	if len(m.destroys)+len(m.stops) != 0 {
		t.Fatalf("a refused wake destroyed or stopped something: destroys %v stops %v", m.destroys, m.stops)
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("a refused wake hibernated other people's computers: %v", got)
	}
	f.reg.mu.RLock()
	after := *f.reg.ownerships[ownershipKey("sleeper", PrimaryDesktopID)]
	f.reg.mu.RUnlock()
	if after.State != VMStateHibernated || after.VMID != before.VMID || after.MemSizeMib != before.MemSizeMib || after.MemoryPeakRSSBytes != before.MemoryPeakRSSBytes {
		t.Fatalf("the ownership changed: before %+v after %+v", before, after)
	}
	if s := f.reg.HostAdmissionSummary(); s.ReservedBytes != 0 || len(s.Reservations) != 0 {
		t.Fatalf("the refused start left a reservation: %+v", s)
	}
	for _, u := range []string{"a", "b", "c", "d"} {
		if st, _ := f.state("vm-" + u); st != VMStateActive {
			t.Fatalf("computer %s is %s after a refused wake", u, st)
		}
	}

	// With room, the same computer wakes (its snapshot was not consumed).
	f.setAvail(40 * gib)
	if _, err := f.reg.ResolveOrAssign("sleeper"); err != nil {
		t.Fatalf("the wake with room: %v", err)
	}
}

// The other half of the truth: when make-room can help, it hibernates idle
// computers (at most three) before the wake is admitted. A person's idle
// computer going to sleep for another's wake is the feature, not a refusal.
// This one runs with a 200 ms settle wait, so it proves the count and the
// order of events, not production's latency (a settle of 10 s per victim is
// bounded by the config, not exercised here).
func TestAWakeMakeRoomCanHelpHibernatesAtMostThreeIdleComputers(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	addOwnership(f.reg, hibernated("sleeper", "vm-sleeper", 3*gib))
	for _, u := range []string{"a", "b", "c", "d"} {
		f.activeComputer(u, "vm-"+u, 2*time.Hour, 1*gib)
	}
	// Short by 2.5 GiB: three 1 GiB computers cover it.
	f.setAvail(4*gib + 1*gib + 3*gib - 2560*mib)
	start := time.Now()
	if _, err := f.reg.ResolveOrAssign("sleeper"); err != nil {
		t.Fatalf("a wake make-room could help was refused: %v", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("the wake took %s", took)
	}
	if got := f.mr.hibernates(); len(got) == 0 || len(got) > 3 {
		t.Fatalf("make-room hibernated %v, want between one and three idle computers", got)
	}
}
