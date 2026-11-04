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
	// refuseJobs is the host's answer to a job lease while set; jobErr a
	// host that cannot be asked.
	refuseJobs *HostMemoryRefusal
	jobErr     error
	leases     map[string]int64 // capsule/job -> bytes
	baselines  map[string]int64
	jobAsks    int
	jobsFreed  []string
	reader     func(string) (int64, bool)
	asked      []int64 // lease bytes, per ask
	// gate, when set, holds AdmitJob until it is closed.
	gate chan struct{}
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

func (f *fakeHostMemory) AdmitJob(_ context.Context, capsuleID, jobID string, baseline, lease int64) error {
	f.mu.Lock()
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobAsks++
	f.asked = append(f.asked, lease)
	if f.baselines == nil {
		f.baselines = map[string]int64{}
	}
	f.baselines[capsuleID] = baseline
	if f.refuseJobs != nil {
		return f.refuseJobs
	}
	if f.leases == nil {
		f.leases = map[string]int64{}
	}
	f.leases[capsuleID+"/"+jobID] = lease
	return f.jobErr
}

func (f *fakeHostMemory) ReleaseJob(capsuleID, jobID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.leases, capsuleID+"/"+jobID)
	f.jobsFreed = append(f.jobsFreed, capsuleID+"/"+jobID)
}

func (f *fakeHostMemory) SetBaselineReader(r func(string) (int64, bool)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reader = r
}

func (f *fakeHostMemory) setRefusal(r *HostMemoryRefusal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuseJobs = r
}

func (f *fakeHostMemory) lease(key string) (int64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.leases[key]
	return b, ok
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
