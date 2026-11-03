//go:build linux

package capsule

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// Size classes at spawn (S4). Failure modes pinned:
//   - S1 a host refusal does not refuse the build spawn, or the spawn
//     reaches the guest's own admission and state first;
//   - S2 a light capsule asks the host for a reservation;
//   - S3 a failed spawn keeps its host reservation;
//   - S4 an unknown class spawns without its reservation.

type fakeHostMemory struct {
	mu       sync.Mutex
	admitted []string
	released []string
	refuse   bool
	facts    HostMemoryFacts
	fresh    bool
}

func (f *fakeHostMemory) AdmitBuild(_ context.Context, id string, bytes int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refuse {
		return &HostMemoryRefusal{Reason: "host memory available 5120 MiB"}
	}
	f.admitted = append(f.admitted, id)
	return nil
}

func (f *fakeHostMemory) ReleaseBuild(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, id)
}

func (f *fakeHostMemory) Facts() (HostMemoryFacts, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.facts, f.fresh
}

func sizeSpec(id string, class SizeClass) SpawnSpec {
	return SpawnSpec{CapsuleID: id, OwnerRunID: "run:" + id, MemoryMax: 1 << 30, CpuQuota: 100000, CpuPeriod: 100000, PidsMax: 64, SizeClass: class}
}

func TestSpawnBuildClassAsksTheHostFirst(t *testing.T) {
	hm := &fakeHostMemory{refuse: true}
	e := &Executor{initErr: errors.New("guest admission reached")}
	e.SetHostMemory(hm)
	_, err := e.Spawn(context.Background(), sizeSpec("cap-build", SizeClassBuild))
	var refusal *HostMemoryRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("S1: build spawn not refused by the host: %v", err)
	}
	hm.refuse = false
	if _, err := e.Spawn(context.Background(), sizeSpec("cap-build", SizeClassBuild)); err == nil || !strings.Contains(err.Error(), "guest admission reached") {
		t.Fatalf("spawn past the host: %v", err)
	}
	if len(hm.admitted) != 1 || len(hm.released) != 1 || hm.released[0] != "cap-build" {
		t.Fatalf("S3: admitted %v released %v", hm.admitted, hm.released)
	}
	if _, err := e.Spawn(context.Background(), sizeSpec("cap-light", SizeClassLight)); err == nil {
		t.Fatalf("light spawn went past the init error")
	}
	if len(hm.admitted) != 1 {
		t.Fatalf("S2: a light capsule asked the host: %v", hm.admitted)
	}
	if _, err := e.Spawn(context.Background(), sizeSpec("cap-odd", SizeClass("huge"))); err == nil || !strings.Contains(err.Error(), "size class") {
		t.Fatalf("S4: unknown class: %v", err)
	}
}
