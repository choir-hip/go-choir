//go:build linux

package capsule

import (
	"context"
	"strings"
	"testing"
)

// A paused capsule's resume runs its teardown, digest and moves outside
// e.mu under the resuming guard; a same-id Spawn in that window is refused
// rather than overlapping it.
func TestSpawnRefusesACapsuleBeingResumed(t *testing.T) {
	e := &Executor{
		capsules:    map[string]*Capsule{},
		resumingIDs: map[string]bool{"capsule-resuming": true},
	}
	_, err := e.Spawn(context.Background(), SpawnSpec{
		CapsuleID:  "capsule-resuming",
		OwnerRunID: "run-1",
		MemoryMax:  1 << 30,
		CpuQuota:   100000,
		CpuPeriod:  100000,
		PidsMax:    64,
		SizeClass:  SizeClassLight,
	})
	if err == nil || !strings.Contains(err.Error(), "is being resumed") {
		t.Fatalf("Spawn during a resume: err = %v, want a being-resumed refusal", err)
	}
}
