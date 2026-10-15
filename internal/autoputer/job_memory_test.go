package autoputer

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The guest's job admission hook (docs/reports/memory-hotplug-controller-
// design-2026-10-14.md section 9.4): J1 unavailable falls back to the
// budget the guest has, J2 a refusal refuses the job, J4 the hook waits for
// MemTotal to show the plug.

type fakeJobLeaser struct {
	acquired []int
	released int
	err      error
}

func (f *fakeJobLeaser) Acquire(_ context.Context, needMiB int) (*vmctl.MemoryHotplugView, func(), error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	f.acquired = append(f.acquired, needMiB)
	return &vmctl.MemoryHotplugView{BaseMiB: 4096, CurrentMiB: 4096 + needMiB}, func() { f.released++ }, nil
}

func TestJobMemoryAdmission(t *testing.T) {
	spec := capsulejob.Spec{ID: "job-m", Argv: []string{"go", "build"}}
	var memTotal atomic.Int64
	memTotal.Store(4000 << 20)
	read := func() int64 { return memTotal.Load() }

	leaser := &fakeJobLeaser{}
	admit := newJobMemoryAdmission(leaser, read, 2*time.Second)
	if release, err := admit("capsule-1", spec); release != nil || err != nil || len(leaser.acquired) != 0 {
		t.Fatalf("no need: release=%v err=%v acquired=%v", release != nil, err, leaser.acquired)
	}

	spec.MemoryMiB = 768
	go func() {
		time.Sleep(100 * time.Millisecond)
		memTotal.Add(768 << 20) // the guest onlines the plugged blocks
	}()
	start := time.Now()
	release, err := admit("capsule-1", spec)
	if err != nil || release == nil || len(leaser.acquired) != 1 || leaser.acquired[0] != 768 {
		t.Fatalf("granted: err=%v acquired=%v", err, leaser.acquired)
	}
	if waited := time.Since(start); waited < 100*time.Millisecond || waited > 1500*time.Millisecond {
		t.Fatalf("waited %s for MemTotal, want about 100ms", waited)
	}
	release()
	if leaser.released != 1 {
		t.Fatalf("released %d times", leaser.released)
	}

	// MemTotal never shows it: bounded by the settle wait, not an error
	// (the executor's live budget decides).
	slow := newJobMemoryAdmission(&fakeJobLeaser{}, read, 150*time.Millisecond)
	if release, err := slow("capsule-1", spec); err != nil || release == nil {
		t.Fatalf("unsettled: release=%v err=%v", release != nil, err)
	}

	for _, status := range []int{http.StatusNotFound, http.StatusConflict} {
		unavailable := newJobMemoryAdmission(&fakeJobLeaser{err: &vmctl.MemoryLeaseError{Status: status, Reason: "computer is not selected for memory hotplug"}}, read, time.Second)
		if release, err := unavailable("capsule-1", spec); release != nil || err != nil {
			t.Fatalf("unavailable %d: release=%v err=%v", status, release != nil, err)
		}
	}
	refused := newJobMemoryAdmission(&fakeJobLeaser{err: &vmctl.MemoryLeaseError{Status: http.StatusServiceUnavailable, Reason: "host memory PSI some avg10 2.00 is above 1.00"}}, read, time.Second)
	if release, err := refused("capsule-1", spec); release != nil || err == nil || !strings.Contains(err.Error(), "PSI") || !strings.Contains(err.Error(), "memory_mib") {
		t.Fatalf("refused: release=%v err=%v", release != nil, err)
	}
	if release, err := newJobMemoryAdmission(nil, read, time.Second)("capsule-1", spec); release != nil || err != nil {
		t.Fatalf("no vmctl: release=%v err=%v", release != nil, err)
	}
}
