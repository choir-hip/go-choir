package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/guestmem"
)

// D6 S1, the guest's cache return route
// (docs/design/density-d6-page-cache-return-2026-10-26.md sections 4.1, 9 S1).
// The sizing, swap, EAGAIN, budget and scope cases are in
// internal/guestmem/reclaim_test.go; these cover what the route adds.
//
// Ways it can fail, written first:
//   - M1: idle mode acts while a run is running, a desk mutation is pending,
//     a self-development operation is active, a capsule is live, or the live
//     capsule count is unreported; or any of those store reads is unanswered
//     and the act goes ahead anyway (an unanswered read is busy: 409);
//   - M1: a run that starts, or a hibernate fence that closes, between two
//     steps does not stop the act;
//   - M23: hibernate mode runs on a guest whose fence is open (it would
//     bypass the busy check), or re-reads the store although the fence
//     already proved the guest idle;
//   - one act at a time: a second call while one is writing waits, or runs;
//   - a guest process (not the host peer) can trigger the act;
//   - an unknown body field or mode is accepted; busy_cold (the always-busy
//     mode, S5) is reachable before it exists;
//   - dry_run writes;
//   - a stalled store makes the route hang instead of answering 409 busy.

type cacheReturnFixture struct {
	t      *testing.T
	rt     *Runtime
	proc   string
	daemon string

	mu     sync.Mutex
	writes []string
	reads  int
}

func newCacheReturnFixture(t *testing.T) *cacheReturnFixture {
	t.Helper()
	root := t.TempDir()
	f := &cacheReturnFixture{t: t, proc: filepath.Join(root, "proc")}
	cgroup := filepath.Join(root, "cgroup")
	f.daemon = filepath.Join(cgroup, "system.slice", "daemon.service")
	put := func(path, content string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const mib = 1 << 20
	put(filepath.Join(f.proc, "self", "cgroup"), "0::/system.slice/daemon.service\n")
	put(filepath.Join(f.proc, "swaps"), "Filename Type Size Used Priority\n")
	put(filepath.Join(f.proc, "meminfo"), "MemTotal: 8388608 kB\nMemFree: 4194304 kB\nMemAvailable: 6291456 kB\n")
	put(filepath.Join(f.proc, "vmstat"), "pgsteal_proactive 1\n")
	put(filepath.Join(f.proc, "sys", "vm", "drop_caches"), "")
	put(filepath.Join(f.daemon, "memory.reclaim"), "")
	put(filepath.Join(f.daemon, "memory.stat"), fmt.Sprintf(
		"file %d\nshmem 0\nfile_dirty 0\nfile_writeback 0\ninactive_file %d\nactive_file %d\n", 2000*mib, 1500*mib, 500*mib))

	rt := &Runtime{running: map[string]context.CancelFunc{}}
	rt.cacheReturn.ProcRoot = f.proc
	rt.cacheReturn.CgroupRoot = cgroup
	rt.cacheReturn.Quiet = func() {}
	rt.cacheReturn.Write = func(path, value string) error {
		f.mu.Lock()
		f.writes = append(f.writes, value)
		f.mu.Unlock()
		return nil
	}
	zero := 0
	rt.cacheReturnReads = func(context.Context, time.Duration) healthStoreReads {
		f.mu.Lock()
		f.reads++
		f.mu.Unlock()
		return healthStoreReads{liveCapsules: &zero}
	}
	f.rt = rt
	return f
}

func (f *cacheReturnFixture) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.writes)
}

func (f *cacheReturnFixture) readCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

func postCacheReturn(t *testing.T, rt *Runtime, remote, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/memory/cache-return", bytes.NewBufferString(body))
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleInternalMemoryCacheReturn(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

const (
	hostPeer      = "192.0.2.1:4000"
	idleBody      = `{"mode":"idle","scope":"daemon"}`
	hibernateBody = `{"mode":"hibernate","scope":"daemon"}`
)

// M1: every busy signal /health reads, plus RunningCount, keeps idle mode
// from writing.
func TestCacheReturnIdleIsBusyOnAnyHealthSignal(t *testing.T) {
	one, zero := 1, 0
	cases := map[string]func(f *cacheReturnFixture){
		"a run is running": func(f *cacheReturnFixture) { f.rt.running["loop-1"] = func() {} },
		"desk mutation pending": func(f *cacheReturnFixture) {
			f.rt.cacheReturnReads = func(context.Context, time.Duration) healthStoreReads {
				return healthStoreReads{deskPending: 1, liveCapsules: &zero}
			}
		},
		"self-development active": func(f *cacheReturnFixture) {
			f.rt.cacheReturnReads = func(context.Context, time.Duration) healthStoreReads {
				return healthStoreReads{selfdevActive: 1, liveCapsules: &zero}
			}
		},
		"a live capsule": func(f *cacheReturnFixture) {
			f.rt.cacheReturnReads = func(context.Context, time.Duration) healthStoreReads {
				return healthStoreReads{liveCapsules: &one}
			}
		},
		"live capsules not reported (nil)": func(f *cacheReturnFixture) {
			f.rt.cacheReturnReads = func(context.Context, time.Duration) healthStoreReads { return healthStoreReads{} }
		},
		"the hibernate fence is closed": func(f *cacheReturnFixture) { f.rt.hibernate.close(time.Minute) },
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newCacheReturnFixture(t)
			arrange(f)
			code, body := postCacheReturn(t, f.rt, hostPeer, idleBody)
			if code != http.StatusConflict || body["outcome"] != "busy" {
				t.Fatalf("%s: %d %v, want 409 busy", name, code, body)
			}
			if f.writeCount() != 0 {
				t.Fatalf("%s: %d writes", name, f.writeCount())
			}
		})
	}
}

// M1: an unanswered store read is busy (the one code, 409 busy, like any
// other busy signal), and writes nothing.
func TestCacheReturnIdleUnansweredStoreReadIsBusy(t *testing.T) {
	zero := 0
	for _, unanswered := range [][]string{{"desk_pending_mutations"}, {"selfdev_active_operations"}, {"live_capsules"}, {"effective_release"}} {
		f := newCacheReturnFixture(t)
		f.rt.cacheReturnReads = func(context.Context, time.Duration) healthStoreReads {
			return healthStoreReads{liveCapsules: &zero, unanswered: unanswered}
		}
		code, body := postCacheReturn(t, f.rt, hostPeer, idleBody)
		if code != http.StatusConflict || body["outcome"] != "busy" {
			t.Fatalf("unanswered %v: %d %v, want 409 busy", unanswered, code, body)
		}
		if f.writeCount() != 0 {
			t.Fatalf("unanswered %v: %d writes", unanswered, f.writeCount())
		}
	}
}

// A computer with nothing in flight acts, and the report says what it did.
func TestCacheReturnIdleRunsWhenNothingIsInFlight(t *testing.T) {
	f := newCacheReturnFixture(t)
	code, body := postCacheReturn(t, f.rt, hostPeer, idleBody)
	if code != http.StatusOK || body["outcome"] != "returned" {
		t.Fatalf("idle act: %d %v, want 200 returned", code, body)
	}
	// cold = min(1500, 2000) MiB, idle target = cold - 256 = 1244 MiB =
	// 19 steps of 64 MiB and one of 28.
	if f.writeCount() != 20 || body["mode"] != "idle" || body["scope"] != "daemon" {
		t.Fatalf("writes %d, report %v", f.writeCount(), body)
	}
	if body["target_bytes"] != float64(1244<<20) {
		t.Fatalf("target_bytes = %v, want %d", body["target_bytes"], 1244<<20)
	}
}

// M1: a run that starts between step 2 and step 3 stops the act after
// step 2, as stopped_busy.
func TestCacheReturnRunStartingMidActStopsIt(t *testing.T) {
	f := newCacheReturnFixture(t)
	n := 0
	f.rt.cacheReturn.Write = func(path, value string) error {
		n++
		if n == 2 {
			f.rt.running["loop-new"] = func() {}
		}
		return nil
	}
	code, body := postCacheReturn(t, f.rt, hostPeer, idleBody)
	if code != http.StatusOK || body["outcome"] != "stopped_busy" || n != 2 {
		t.Fatalf("%d writes, %d %v, want 2 writes and 200 stopped_busy", n, code, body)
	}
}

// M1: a hibernate fence that closes mid-act stops it the same way.
func TestCacheReturnFenceClosingMidActStopsIt(t *testing.T) {
	f := newCacheReturnFixture(t)
	n := 0
	f.rt.cacheReturn.Write = func(path, value string) error {
		n++
		if n == 2 {
			if _, ok := f.rt.hibernate.close(time.Minute); !ok {
				t.Error("the test could not close the fence")
			}
		}
		return nil
	}
	code, body := postCacheReturn(t, f.rt, hostPeer, idleBody)
	if code != http.StatusOK || body["outcome"] != "stopped_busy" || n != 2 {
		t.Fatalf("%d writes, %d %v, want 2 writes and 200 stopped_busy", n, code, body)
	}
}

// M23: hibernate mode with the fence open is 409 not_fenced and writes
// nothing; with it closed the act runs, and it does not re-read the store.
func TestCacheReturnHibernateRequiresTheFence(t *testing.T) {
	f := newCacheReturnFixture(t)
	code, body := postCacheReturn(t, f.rt, hostPeer, hibernateBody)
	if code != http.StatusConflict || body["outcome"] != "not_fenced" {
		t.Fatalf("hibernate with the fence open: %d %v, want 409 not_fenced", code, body)
	}
	if f.writeCount() != 0 {
		t.Fatalf("an unfenced hibernate act wrote %d times", f.writeCount())
	}
	if _, ok := f.rt.hibernate.close(time.Minute); !ok {
		t.Fatal("could not close the fence")
	}
	code, body = postCacheReturn(t, f.rt, hostPeer, hibernateBody)
	if code != http.StatusOK || body["outcome"] != "returned" || body["mode"] != "hibernate" {
		t.Fatalf("hibernate with the fence closed: %d %v, want 200 returned", code, body)
	}
	if f.writeCount() == 0 {
		t.Fatal("the fenced hibernate act wrote nothing")
	}
	if f.readCount() != 0 {
		t.Fatalf("hibernate mode read the store %d times; the fence already proved idle", f.readCount())
	}
}

// M23: a fence whose TTL ran out is not a fence.
func TestCacheReturnHibernateWithAnExpiredFenceIsNotFenced(t *testing.T) {
	f := newCacheReturnFixture(t)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	f.rt.hibernate.now = func() time.Time { return now }
	if _, ok := f.rt.hibernate.close(time.Minute); !ok {
		t.Fatal("could not close the fence")
	}
	now = now.Add(2 * time.Minute)
	code, body := postCacheReturn(t, f.rt, hostPeer, hibernateBody)
	if code != http.StatusConflict || body["outcome"] != "not_fenced" || f.writeCount() != 0 {
		t.Fatalf("expired fence: %d %v with %d writes, want 409 not_fenced and none", code, body, f.writeCount())
	}
}

// One act at a time: a second call while one writes answers 409
// in_progress. Run with -race.
func TestCacheReturnSecondCallWhileWritingIsInProgress(t *testing.T) {
	f := newCacheReturnFixture(t)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	f.rt.cacheReturn.Write = func(string, string) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-time.After(5 * time.Second):
		}
		return nil
	}
	first := make(chan int, 1)
	go func() {
		code, _ := postCacheReturn(t, f.rt, hostPeer, idleBody)
		first <- code
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first act never wrote")
	}
	code, body := postCacheReturn(t, f.rt, hostPeer, idleBody)
	if code != http.StatusConflict || body["outcome"] != "in_progress" {
		t.Fatalf("second call: %d %v, want 409 in_progress", code, body)
	}
	close(release)
	select {
	case code := <-first:
		if code != http.StatusOK {
			t.Fatalf("first call answered %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the first act did not finish")
	}
}

// A guest process or a stranger cannot trigger the act.
func TestCacheReturnIsHostPeerOnly(t *testing.T) {
	f := newCacheReturnFixture(t)
	for _, remote := range []string{"203.0.113.9:4000", "127.0.0.1:4000"} {
		code, _ := postCacheReturn(t, f.rt, remote, idleBody)
		if code != http.StatusForbidden {
			t.Fatalf("%s: %d, want 403", remote, code)
		}
	}
	if f.writeCount() != 0 || f.readCount() != 0 {
		t.Fatalf("a non-host caller reached the act: %d writes, %d reads", f.writeCount(), f.readCount())
	}
}

// The body is decoded strictly.
func TestCacheReturnRejectsBadBodies(t *testing.T) {
	f := newCacheReturnFixture(t)
	for name, body := range map[string]string{
		"unknown field":        `{"mode":"idle","scope":"daemon","reclaim_bytes":1}`,
		"missing mode":         `{"scope":"daemon"}`,
		"unknown mode":         `{"mode":"aggressive","scope":"daemon"}`,
		"unknown scope":        `{"mode":"idle","scope":"cgroup"}`,
		"busy_cold not yet":    `{"mode":"busy_cold","scope":"daemon"}`,
		"not json":             `idle`,
		"trailing second body": `{"mode":"idle","scope":"daemon"}{"mode":"idle"}`,
		"empty":                ``,
	} {
		code, _ := postCacheReturn(t, f.rt, hostPeer, body)
		if code != http.StatusBadRequest {
			t.Fatalf("%s: %d, want 400", name, code)
		}
	}
	if f.writeCount() != 0 {
		t.Fatalf("a rejected body wrote %d times", f.writeCount())
	}
	req := httptest.NewRequest(http.MethodGet, "/internal/runtime/memory/cache-return", nil)
	req.RemoteAddr = hostPeer
	rec := httptest.NewRecorder()
	(&APIHandler{rt: f.rt}).HandleInternalMemoryCacheReturn(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d, want 405", rec.Code)
	}
}

// dry_run sizes and reports and writes nothing; in idle mode it still
// applies the busy rule.
func TestCacheReturnDryRunWritesNothing(t *testing.T) {
	f := newCacheReturnFixture(t)
	code, body := postCacheReturn(t, f.rt, hostPeer, `{"mode":"idle","scope":"daemon","dry_run":true}`)
	if code != http.StatusOK || body["dry_run"] != true || body["outcome"] != "dry_run" {
		t.Fatalf("dry run: %d %v", code, body)
	}
	if f.writeCount() != 0 {
		t.Fatalf("dry run wrote %d times", f.writeCount())
	}
	if body["target_bytes"] == nil || body["target_bytes"].(float64) <= 0 {
		t.Fatalf("dry run has no target: %v", body)
	}
	f.rt.running["loop-1"] = func() {}
	code, body = postCacheReturn(t, f.rt, hostPeer, `{"mode":"idle","scope":"daemon","dry_run":true}`)
	if code != http.StatusConflict || body["outcome"] != "busy" {
		t.Fatalf("dry run on a busy guest: %d %v, want 409 busy", code, body)
	}
}

// A store whose reads cannot answer (the production path, no seam) is
// 409 busy within the /health budget, not a hang.
func TestCacheReturnAnswersBusyWhenTheStoreIsHeld(t *testing.T) {
	handler, productStore := newBoundedHealthFixture(t)
	rt := handler.rt
	f := newCacheReturnFixture(t)
	rt.cacheReturn = guestmem.Reclaimer{ProcRoot: f.rt.cacheReturn.ProcRoot, CgroupRoot: f.rt.cacheReturn.CgroupRoot, Write: f.rt.cacheReturn.Write}
	release := holdStore(t, productStore)
	defer release()
	done := make(chan int, 1)
	go func() {
		code, _ := postCacheReturn(t, rt, hostPeer, idleBody)
		done <- code
	}()
	select {
	case code := <-done:
		if code != http.StatusConflict {
			t.Fatalf("held store: %d, want 409 busy", code)
		}
	case <-time.After(healthAnswerWait):
		t.Fatalf("the route did not answer within %s while the store was held", healthAnswerWait)
	}
	if f.writeCount() != 0 {
		t.Fatalf("a held store let the act write %d times", f.writeCount())
	}
}

// EAGAIN from the kernel is a 200 partial report, not an error status.
func TestCacheReturnEAGAINIsAPartialReport(t *testing.T) {
	f := newCacheReturnFixture(t)
	n := 0
	f.rt.cacheReturn.Write = func(path, value string) error {
		n++
		if n == 2 {
			return &os.PathError{Op: "write", Path: path, Err: syscall.EAGAIN}
		}
		return nil
	}
	code, body := postCacheReturn(t, f.rt, hostPeer, idleBody)
	if code != http.StatusOK || body["outcome"] != "partial" {
		t.Fatalf("%d %v, want 200 partial", code, body)
	}
}
