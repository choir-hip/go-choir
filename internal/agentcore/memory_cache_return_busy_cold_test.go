package agentcore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/guestmem"
)

// D6 S5, the route's busy_cold mode: an always-busy computer (the paper)
// returns cold page cache while it works
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.1, 5.5,
// 9 S5; owner decision 2026-11-01 recorded in section 12).
//
// Ways it can fail, written first:
//   - busy_cold acts while a capsule is live, or while a self-development
//     operation is executing, accepted, materializing or rollback_pending
//     (the CLAUDE.md always-busy exclusion: never mid-apply, never with a
//     live engineering capsule);
//   - a read of those that errors, hangs or cannot be made (no computer id,
//     no store) reads as "none" and the act goes ahead (it must be 409 busy,
//     fail closed, within the read budget);
//   - busy_cold acts while the hibernate fence is closed;
//   - busy_cold refuses because runs are running (that is the point of the
//     mode: it must act while the computer works);
//   - the refault guard is not wired: the route acts with refaults over the
//     limit, or with no baseline;
//   - a hibernate fence that closes mid-act does not stop it.

const busyColdBody = `{"mode":"busy_cold","scope":"daemon"}`

// busyColdFixture is the route fixture with a quiet refault baseline a
// minute old and an always-busy computer's work read: runs running, no
// capsule, no self-development operation.
func busyColdFixture(t *testing.T) *cacheReturnFixture {
	t.Helper()
	f := newCacheReturnFixture(t)
	setVmstatRefaults(t, f, 100)
	f.rt.cacheReturnRefaults = guestmem.RefaultWindow{ProcRoot: f.proc}
	f.rt.cacheReturnRefaults.Observe(time.Now().Add(-65*time.Second), 100)
	f.rt.running["loop-paper"] = func() {}
	f.rt.cacheReturnBusyColdWork = func(context.Context) (plannedRestartWork, error) {
		return plannedRestartWork{Running: 1}, nil
	}
	return f
}

func setVmstatRefaults(t *testing.T, f *cacheReturnFixture, n int64) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.proc, "vmstat"), []byte(fmt.Sprintf("pgsteal_proactive 1\nworkingset_refault_file %d\n", n)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A working computer with nothing unsafe in flight returns cache: runs are
// running and the act still goes ahead, capped, with the guard's readings.
func TestCacheReturnBusyColdActsWhileRunsRun(t *testing.T) {
	f := busyColdFixture(t)
	code, body := postCacheReturn(t, f.rt, hostPeer, busyColdBody)
	if code != http.StatusOK || body["outcome"] != "returned" || body["mode"] != "busy_cold" {
		t.Fatalf("busy_cold on a working computer: %d %v, want 200 returned", code, body)
	}
	// cold 1500, keep max(512, 500) = 512: 988 MiB, 16 steps (15 of 64, one of 28).
	if body["target_bytes"] != float64(988<<20) || f.writeCount() != 16 {
		t.Fatalf("target %v with %d writes, want 988 MiB in 16 steps", body["target_bytes"], f.writeCount())
	}
	if body["refault_limit_per_s"] != guestmem.RefaultLimitPerSecond || body["refault_rate_before_per_s"] == nil {
		t.Fatalf("report lacks the guard's readings: %v", body)
	}
}

// The always-busy exclusion: a live capsule or a self-development operation
// in flight is 409 busy with its reason, and nothing is written.
func TestCacheReturnBusyColdRefusesCapsuleAndSelfdev(t *testing.T) {
	cases := map[string]struct {
		work   plannedRestartWork
		err    error
		reason string
	}{
		"a live capsule":               {plannedRestartWork{Running: 1, LiveCapsules: 1}, nil, "live_capsules"},
		"a self-development apply":     {plannedRestartWork{Running: 1, SelfdevActive: 1}, nil, "selfdev_active"},
		"an unreadable operation list": {plannedRestartWork{}, errors.New("store down"), "store_unanswered"},
		"capsules cannot be counted":   {plannedRestartWork{}, errLiveCapsulesUncountable, "store_unanswered"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := busyColdFixture(t)
			f.rt.cacheReturnBusyColdWork = func(context.Context) (plannedRestartWork, error) { return c.work, c.err }
			code, body := postCacheReturn(t, f.rt, hostPeer, busyColdBody)
			if code != http.StatusConflict || body["outcome"] != "busy" || body["reason"] != c.reason {
				t.Fatalf("%d %v, want 409 busy reason %s", code, body, c.reason)
			}
			if f.writeCount() != 0 {
				t.Fatalf("a refused busy_cold wrote %d times", f.writeCount())
			}
		})
	}
}

// A read that never answers is busy within the budget, not a hang.
func TestCacheReturnBusyColdHungReadIsBusy(t *testing.T) {
	f := busyColdFixture(t)
	release := make(chan struct{})
	defer close(release)
	f.rt.cacheReturnBusyColdWork = func(ctx context.Context) (plannedRestartWork, error) {
		select {
		case <-release:
		case <-time.After(10 * time.Second):
		}
		return plannedRestartWork{}, nil
	}
	done := make(chan int, 1)
	go func() {
		code, _ := postCacheReturn(t, f.rt, hostPeer, busyColdBody)
		done <- code
	}()
	select {
	case code := <-done:
		if code != http.StatusConflict {
			t.Fatalf("hung read: %d, want 409 busy", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("busy_cold did not answer while its read hung")
	}
	if f.writeCount() != 0 {
		t.Fatalf("a hung read let the act write %d times", f.writeCount())
	}
}

// The production read (no seam): on a computer with an id and a store and
// nothing in flight the act runs; on one with no computer id the capsule
// count cannot be made, which is busy, never "none".
func TestCacheReturnBusyColdProductionRead(t *testing.T) {
	handler, _ := newBoundedHealthFixture(t)
	rt := handler.rt
	f := busyColdFixture(t)
	rt.cacheReturn = guestmem.Reclaimer{ProcRoot: f.rt.cacheReturn.ProcRoot, CgroupRoot: f.rt.cacheReturn.CgroupRoot, Write: f.rt.cacheReturn.Write, Quiet: func() {}}
	rt.cacheReturnRefaults = guestmem.RefaultWindow{ProcRoot: f.proc}
	rt.cacheReturnRefaults.Observe(time.Now().Add(-65*time.Second), 100)
	code, body := postCacheReturn(t, rt, hostPeer, busyColdBody)
	if code != http.StatusOK || body["outcome"] != "returned" || f.writeCount() == 0 {
		t.Fatalf("production read, nothing in flight: %d %v with %d writes, want 200 returned", code, body, f.writeCount())
	}

	g := busyColdFixture(t)
	g.rt.cacheReturnBusyColdWork = nil // the production read, no computer id
	code, body = postCacheReturn(t, g.rt, hostPeer, busyColdBody)
	if code != http.StatusConflict || body["reason"] != "store_unanswered" || g.writeCount() != 0 {
		t.Fatalf("no computer id: %d %v with %d writes, want 409 store_unanswered and none", code, body, g.writeCount())
	}
}

// The fence: closed before the call is 409 busy hibernating; closing
// mid-act stops it.
func TestCacheReturnBusyColdRespectsTheHibernateFence(t *testing.T) {
	f := busyColdFixture(t)
	if _, ok := f.rt.hibernate.close(time.Minute); !ok {
		t.Fatal("could not close the fence")
	}
	code, body := postCacheReturn(t, f.rt, hostPeer, busyColdBody)
	if code != http.StatusConflict || body["reason"] != "hibernating" || f.writeCount() != 0 {
		t.Fatalf("fenced: %d %v with %d writes, want 409 hibernating and none", code, body, f.writeCount())
	}

	f = busyColdFixture(t)
	n := 0
	f.rt.cacheReturn.Write = func(string, string) error {
		n++
		if n == 2 {
			f.rt.hibernate.close(time.Minute)
		}
		return nil
	}
	code, body = postCacheReturn(t, f.rt, hostPeer, busyColdBody)
	if code != http.StatusOK || body["outcome"] != "stopped_busy" || n != 2 {
		t.Fatalf("fence closing mid-act: %d %v after %d writes, want stopped_busy after 2", code, body, n)
	}
}

// The refault guard is wired: refaults over the limit in the minute before,
// or no baseline at all, is 200 refault_stop with nothing written.
func TestCacheReturnBusyColdRefaultGuardIsEnforced(t *testing.T) {
	f := busyColdFixture(t)
	setVmstatRefaults(t, f, 100+int64(guestmem.RefaultLimitPerSecond*65)+650) // ~42/s
	code, body := postCacheReturn(t, f.rt, hostPeer, busyColdBody)
	if code != http.StatusOK || body["outcome"] != "refault_stop" || f.writeCount() != 0 {
		t.Fatalf("hot refaults: %d %v with %d writes, want refault_stop and none", code, body, f.writeCount())
	}

	f = busyColdFixture(t)
	f.rt.cacheReturnRefaults = guestmem.RefaultWindow{ProcRoot: f.proc} // just booted: no baseline
	code, body = postCacheReturn(t, f.rt, hostPeer, busyColdBody)
	if code != http.StatusOK || body["outcome"] != "refault_stop" || f.writeCount() != 0 {
		t.Fatalf("no baseline: %d %v with %d writes, want refault_stop and none", code, body, f.writeCount())
	}
}

// A dry run applies the busy rule and the guard and writes nothing.
func TestCacheReturnBusyColdDryRun(t *testing.T) {
	f := busyColdFixture(t)
	code, body := postCacheReturn(t, f.rt, hostPeer, `{"mode":"busy_cold","scope":"daemon","dry_run":true}`)
	if code != http.StatusOK || body["outcome"] != "dry_run" || f.writeCount() != 0 {
		t.Fatalf("dry run: %d %v with %d writes", code, body, f.writeCount())
	}
	f.rt.cacheReturnBusyColdWork = func(context.Context) (plannedRestartWork, error) {
		return plannedRestartWork{LiveCapsules: 1}, nil
	}
	code, body = postCacheReturn(t, f.rt, hostPeer, `{"mode":"busy_cold","scope":"daemon","dry_run":true}`)
	if code != http.StatusConflict || body["reason"] != "live_capsules" {
		t.Fatalf("dry run with a live capsule: %d %v, want 409 live_capsules", code, body)
	}
}
