package autoputer

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Capsule jobs take memory leases (docs/reports/memory-hotplug-controller-
// design-2026-10-14.md section 9). A job that declares memory_mib adds it
// to this computer's lease before it starts; the executor then admits it
// against the live capsule budget the plug raised, and the need leaves the
// lease when the job's memory is returned.

const (
	// jobMemoryPlugWait bounds how long vmctl waits for the guest to plug a
	// grow before answering.
	jobMemoryPlugWait = 60 * time.Second
	// jobMemoryOnlineWait bounds how long the hook waits for MemTotal to
	// show the plugged memory (J4).
	jobMemoryOnlineWait = 10 * time.Second
	// jobMemoryAcquireTimeout bounds one acquire: the plug wait plus the
	// base read and slack.
	jobMemoryAcquireTimeout = 90 * time.Second
)

// jobMemoryLeaser is vmctl.GuestMemoryLeases as the hook uses it.
type jobMemoryLeaser interface {
	Acquire(ctx context.Context, needMiB int) (*vmctl.MemoryHotplugView, func(), error)
}

// newJobMemoryAdmission is the executor's job admission hook. A job with no
// need passes untouched. A lease this computer cannot give (hotplug off,
// not selected, no vmctl) is no refusal: the executor admits the need
// against the budget the guest already has (J1). A grow vmctl refused
// refuses the job (J2).
func newJobMemoryAdmission(leases jobMemoryLeaser, memTotal func() int64, onlineWait time.Duration) capsule.JobAdmission {
	return func(capsuleID string, spec capsulejob.Spec) (func(), error) {
		if spec.MemoryMiB <= 0 || leases == nil {
			return nil, nil
		}
		before := memTotal()
		ctx, cancel := context.WithTimeout(context.Background(), jobMemoryAcquireTimeout)
		defer cancel()
		view, release, err := leases.Acquire(ctx, spec.MemoryMiB)
		if err != nil {
			if vmctl.MemoryLeaseUnavailable(err) {
				log.Printf("autoputer: job %s memory_mib %d: no memory lease on this computer (%v); admitting it against the capsule budget the guest has", spec.ID, spec.MemoryMiB, err)
				return nil, nil
			}
			return nil, fmt.Errorf("memory lease of %d MiB refused: %w; start it without memory_mib to run within the capsule's own size, or later", spec.MemoryMiB, err)
		}
		// The plug is done when vmctl answers; onlining the blocks into the
		// guest's MemTotal can trail it. Wait, bounded; the executor's live
		// budget decides either way.
		want := before + int64(spec.MemoryMiB-2)<<20
		deadline := time.Now().Add(onlineWait)
		for memTotal() < want && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if view != nil {
			log.Printf("autoputer: job %s memory lease +%d MiB in capsule %s: current %d MiB, plugged %d MiB", spec.ID, spec.MemoryMiB, capsuleID, view.CurrentMiB, view.PluggedMiB)
		}
		return release, nil
	}
}

// guestMemTotal reads the guest's MemTotal in bytes, or 0.
func guestMemTotal() int64 {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	return capsule.MemTotalBytes(string(raw))
}

// guestVMCTLURL is the vmctl base URL the guest reaches over its tap.
func guestVMCTLURL() string {
	if u := strings.TrimSpace(os.Getenv("RUNTIME_VMCTL_URL")); u != "" {
		return u
	}
	return strings.TrimSpace(os.Getenv("PROXY_VMCTL_URL"))
}

// configureJobMemory installs the live capsule budget and, with a vmctl URL,
// the lease hook, and releases any lease an earlier guest process left
// (J10). staticBudget is true when CHOIR_CAPSULE_MEMORY_TOTAL_BYTES pins it.
func configureJobMemory(executor *capsule.Executor, staticBudget bool) {
	if !staticBudget {
		executor.SetLiveMemoryBudget(capsule.LiveMemoryBudget(nil))
	}
	baseURL := guestVMCTLURL()
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		log.Printf("autoputer: capsule job memory leases off: no vmctl URL")
		return
	}
	// The client timeout covers vmctl's plug wait.
	leases := vmctl.NewGuestMemoryLeases(vmctl.NewClientWithTimeout(baseURL, jobMemoryAcquireTimeout), 0, jobMemoryPlugWait)
	executor.SetJobAdmission(newJobMemoryAdmission(leases, guestMemTotal, jobMemoryOnlineWait))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := leases.ReleaseStale(ctx); err != nil {
			log.Printf("autoputer: release a memory lease from before this start: %v (its TTL ends it)", err)
		}
	}()
}
