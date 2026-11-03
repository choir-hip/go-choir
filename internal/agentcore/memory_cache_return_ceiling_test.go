package agentcore

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/guestmem"
)

// D6 S6, the route's ceiling mode (the elastic memory cap). Ways it can fail,
// written first:
//   - it refuses because runs are running (it is asked of always-busy
//     computers every minute);
//   - it acts while a capsule is live or a self-development operation is in
//     flight (it must stand aside, so a build or an apply grows freely);
//   - a ceiling under the floor is accepted;
//   - it acts in a scope other than the one asked for.
// Through work (through_work, the ring VMCTL_GUEST_MEMORY_CAP_THROUGH_WORK_
// COMPUTERS names):
//   - it still stands aside for a live capsule or a self-development
//     operation (an arm's round holds its cache while its build waits);
//   - it acts through a hibernate (the snapshot owns the memory) or a store
//     snapshot streaming (it would only make the stream read again);
//   - it reads the store at all (a slow read would stall the minute's pass);
//   - another mode accepts the field and silently changes its busy rule.

func ceilingFixture(t *testing.T) *cacheReturnFixture {
	t.Helper()
	f := busyColdFixture(t)
	// The root cgroup: 3000 MiB of clean file pages. The fixture guest uses
	// 4096 MiB (MemTotal 8 GiB, MemFree 4 GiB).
	root := filepath.Dir(filepath.Dir(f.daemon))
	for name, content := range map[string]string{
		"memory.stat":    fmt.Sprintf("anon 1048576\nfile %d\nshmem 0\nfile_dirty 0\nfile_writeback 0\ninactive_file %d\nactive_file %d\n", 3000<<20, 2500<<20, 500<<20),
		"memory.reclaim": "",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestCacheReturnCeilingActsWhileRunsRun(t *testing.T) {
	f := ceilingFixture(t)
	code, body := postCacheReturn(t, f.rt, hostPeer, `{"mode":"ceiling","scope":"root","ceiling_mib":3072}`)
	if code != http.StatusOK || body["outcome"] != guestmem.OutcomeReturned || body["mode"] != "ceiling" || body["scope"] != "root" {
		t.Fatalf("ceiling on a working computer: %d %v, want 200 returned", code, body)
	}
	if body["target_bytes"] != float64(1024<<20) || body["used_bytes"] != float64(4096<<20) || f.writeCount() != 17 || body["compacted"] != true {
		t.Fatalf("target %v used %v with %d writes (compacted %v), want 1024 MiB of 4096 in 16 steps and one compaction", body["target_bytes"], body["used_bytes"], f.writeCount(), body["compacted"])
	}
}

func TestCacheReturnCeilingStandsAsideForCapsuleAndSelfdev(t *testing.T) {
	for name, work := range map[string]plannedRestartWork{
		"a live capsule":           {Running: 1, LiveCapsules: 1},
		"a self-development apply": {Running: 1, SelfdevActive: 1},
	} {
		f := ceilingFixture(t)
		f.rt.cacheReturnBusyColdWork = func(context.Context) (plannedRestartWork, error) { return work, nil }
		code, body := postCacheReturn(t, f.rt, hostPeer, `{"mode":"ceiling","scope":"root","ceiling_mib":3072}`)
		if code != http.StatusConflict || body["outcome"] != "busy" || f.writeCount() != 0 {
			t.Fatalf("%s: %d %v with %d writes, want 409 busy and none", name, code, body, f.writeCount())
		}
	}
}

func TestCacheReturnCeilingUnderTheFloorIsRefused(t *testing.T) {
	f := ceilingFixture(t)
	code, _ := postCacheReturn(t, f.rt, hostPeer, `{"mode":"ceiling","scope":"root","ceiling_mib":512}`)
	if code != http.StatusBadRequest || f.writeCount() != 0 {
		t.Fatalf("a 512 MiB ceiling: %d with %d writes, want 400 and none", code, f.writeCount())
	}
}

func TestCacheReturnCeilingThroughWorkActsWhileCapsuleAndSelfdevLive(t *testing.T) {
	for name, work := range map[string]plannedRestartWork{
		"a live capsule":           {Running: 1, LiveCapsules: 1},
		"a self-development apply": {Running: 1, SelfdevActive: 1},
	} {
		f := ceilingFixture(t)
		read := 0
		f.rt.cacheReturnBusyColdWork = func(context.Context) (plannedRestartWork, error) { read++; return work, nil }
		code, body := postCacheReturn(t, f.rt, hostPeer, `{"mode":"ceiling","scope":"root","ceiling_mib":3072,"through_work":true}`)
		if code != http.StatusOK || body["outcome"] != guestmem.OutcomeReturned || body["target_bytes"] != float64(1024<<20) {
			t.Fatalf("%s through work: %d %v, want 200 returned 1024 MiB", name, code, body)
		}
		if read != 0 {
			t.Fatalf("%s through work: the busy read ran %d times, want none", name, read)
		}
	}
}

func TestCacheReturnCeilingThroughWorkStandsAsideForHibernateAndSnapshot(t *testing.T) {
	for name, tc := range map[string]struct {
		reason string
		set    func(*cacheReturnFixture)
	}{
		"the hibernate fence is closed": {"hibernating", func(f *cacheReturnFixture) { f.rt.hibernate.close(time.Minute) }},
		"a store snapshot streams":      {"snapshot_in_flight", func(f *cacheReturnFixture) { f.rt.snapshots.inFlight.Store(true) }},
	} {
		f := ceilingFixture(t)
		tc.set(f)
		code, body := postCacheReturn(t, f.rt, hostPeer, `{"mode":"ceiling","scope":"root","ceiling_mib":3072,"through_work":true}`)
		if code != http.StatusConflict || body["outcome"] != "busy" || body["reason"] != tc.reason || f.writeCount() != 0 {
			t.Fatalf("%s: %d %v with %d writes, want 409 busy %s and none", name, code, body, f.writeCount(), tc.reason)
		}
	}
}

func TestCacheReturnThroughWorkIsCeilingOnly(t *testing.T) {
	for _, mode := range []string{"idle", "busy_cold", "hibernate"} {
		f := ceilingFixture(t)
		code, _ := postCacheReturn(t, f.rt, hostPeer, `{"mode":"`+mode+`","scope":"daemon","through_work":true}`)
		if code != http.StatusBadRequest || f.writeCount() != 0 {
			t.Fatalf("through_work on %s: %d with %d writes, want 400 and none", mode, code, f.writeCount())
		}
	}
}
