package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Round 3 review findings (Sonnet N1 to N9, DeepSeek 1 to 9) on
// lane/fp0-disk-pressure 440f8ee48, before the trigger is switched on
// (docs/problems/node-b-disk-fills-from-the-paper-data-image-2026-10-22.md,
// "Round 3").

// reportCompactor returns a report whose compacted size is bytesAfter.
type reportCompactor struct {
	fakeCompactor
	bytesAfter int64
}

func (r *reportCompactor) Compact(ctx context.Context, vmID, path, computerID, head string) (storeCompactionReport, error) {
	rep, err := r.fakeCompactor.Compact(ctx, vmID, path, computerID, head)
	rep.BytesAfter = r.bytesAfter
	return rep, err
}

// blockingCompactor hangs until its context ends.
type blockingCompactor struct{}

func (blockingCompactor) Compact(ctx context.Context, _, _, _, _ string) (storeCompactionReport, error) {
	<-ctx.Done()
	return storeCompactionReport{}, ctx.Err()
}

type panickingCompactor struct{}

func (panickingCompactor) Compact(context.Context, string, string, string, string) (storeCompactionReport, error) {
	panic("compactor exploded")
}

// cleaningCompactor records stale-copy cleanups.
type cleaningCompactor struct {
	fakeCompactor
	mu      sync.Mutex
	cleaned []string
}

func (c *cleaningCompactor) CleanupStale(_ context.Context, vmID, dataImagePath string) {
	c.mu.Lock()
	c.cleaned = append(c.cleaned, vmID+"|"+dataImagePath)
	c.mu.Unlock()
}

func recordAttempt(t *testing.T, h *Handler, att storeCompactAttempt) {
	t.Helper()
	if err := h.storeCompaction().attemptsRecord().record(att); err != nil {
		t.Fatal(err)
	}
}

func paperVMID(t *testing.T, h *Handler, user string) string {
	t.Helper()
	return h.registry.GetOwnershipForDesktop(user, PrimaryDesktopID).VMID
}

func postSweep(t *testing.T, h *Handler, body string) (int, storeCompactSweepResponse, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/compact-sweep", bytes.NewBufferString(body))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleCompactSweep(rec, req)
	var resp storeCompactSweepResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	return rec.Code, resp, rec.Body.String()
}

// N1: the floor follows the last compacted size (plus 10 GiB), with the
// configured value as its minimum; 0 still turns the floor off.
func TestStoreCompactFloorFollowsLastCompactedSize(t *testing.T) {
	h, _, marker, _ := pressureFixture(t, "paper-dynfloor", 80, 25)
	rc := &reportCompactor{bytesAfter: 20 << 30}
	state := h.storeCompaction()
	state.compactor = rc
	state.minHostFree = 24 << 30
	vm := paperVMID(t, h, "paper-dynfloor")

	// A compaction records the compacted size it reported.
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-dynfloor","desktop_id":"primary"}`); code != http.StatusOK {
		t.Fatalf("first compaction: %d %+v", code, resp)
	}
	last, _, _ := state.attemptsRecord().lastAttempt(vm)
	if last.StoreBytesAfter != 20<<30 {
		t.Fatalf("recorded compacted size = %d, want 20 GiB", last.StoreBytesAfter)
	}
	code, resp := postCompactStore(t, h, `{"user_id":"paper-dynfloor","desktop_id":"primary","force":true}`)
	if code != http.StatusInsufficientStorage || !strings.Contains(resp.Reason, "below the compaction floor 30.0 GiB (last compacted store 20.0 GiB + 10.0 GiB)") {
		t.Fatalf("25 GiB free against a 20 GiB compacted store: %d %+v", code, resp)
	}
	if len(marker.marks) != 1 {
		t.Fatalf("marks = %v, want only the first compaction's", marker.marks)
	}
	state.minHostFree = 0
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-dynfloor","desktop_id":"primary","force":true}`); code != http.StatusOK {
		t.Fatalf("floor off: %d %+v", code, resp)
	}
}

// Mutant C2: free space exactly at the floor passes.
func TestStoreCompactFloorBoundaryPasses(t *testing.T) {
	h, _, _, _ := pressureFixture(t, "paper-boundary", 80, 24)
	h.storeCompaction().minHostFree = 24 << 30
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-boundary","desktop_id":"primary"}`); code != http.StatusOK || resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("free == floor: %d %+v", code, resp)
	}
}

// N2: the back-off never applies to the 04:00 timer (a 04:00:07 attempt
// must not make the next day's 03:59:54 run a silent skip); the 6 h gap does.
func TestStoreCompactTimerIgnoresBackoff(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-timer-backoff", 80, 72)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	vm := paperVMID(t, h, "paper-timer-backoff")
	recordAttempt(t, h, storeCompactAttempt{VMID: vm, At: time.Now().Add(-(23*time.Hour + 59*time.Minute + 54*time.Second)), FileBytesBefore: 80 << 30, FileBytesAfter: 76 << 30})
	code, resp, body := postSweep(t, h, `{"min_file_bytes":1}`)
	if code != http.StatusOK || len(resp.Selected) != 1 || resp.Selected[0].Result == nil || resp.Selected[0].Result.Outcome != deployRefreshRefreshed {
		t.Fatalf("timer after a small reclaim 23h59m54s ago: %d %s", code, body)
	}
	recordAttempt(t, h, storeCompactAttempt{VMID: vm, At: time.Now().Add(-time.Hour)})
	_, resp, body = postSweep(t, h, `{"min_file_bytes":1}`)
	if resp.Selected[0].Result == nil || resp.Selected[0].Result.Outcome != deployRefreshSkipped || len(compactor.calls) != 1 {
		t.Fatalf("timer inside the 6 h gap: %s calls=%v", body, compactor.calls)
	}
}

// N3 (mutant S2): a large reclaim keeps the plain 6 h gap.
func TestStoreCompactPressureLargeReclaimKeepsSixHourGap(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-large", 80, 72)
	var calls atomic.Int32
	h.storeCompaction().allocatedBytes = func(string) uint64 {
		if calls.Add(1) == 1 {
			return 80 << 30
		}
		return 30 << 30
	}
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	h.storeCompactPressureTick(context.Background(), t0)
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(7*time.Hour)); len(d) != 1 || d[0].Action != pressureActionCompact || len(compactor.calls) != 2 {
		t.Fatalf("+7h after a 50 GiB reclaim = %+v", d)
	}
}

// N3 (mutant S4): a failed compaction records no after-size, so it is not
// read as "reclaimed nothing" and does not back off; it is marked failed.
func TestStoreCompactFailedCompactionRecordsNoAfter(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-failed-size", 80, 72)
	compactor.err = errors.New("projection-compact: refused")
	var calls atomic.Int32
	h.storeCompaction().allocatedBytes = func(string) uint64 {
		if calls.Add(1) == 1 {
			return 80 << 30
		}
		return 76 << 30
	}
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	h.storeCompactPressureTick(context.Background(), t0)
	last, _, _ := h.storeCompaction().attemptsRecord().lastAttempt(paperVMID(t, h, "paper-failed-size"))
	if last.FileBytesAfter != 0 || !last.Failed {
		t.Fatalf("failed attempt = %+v, want no after-size and failed", last)
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(7*time.Hour)); len(d) != 1 || d[0].Action != pressureActionCompact || len(compactor.calls) != 2 {
		t.Fatalf("+7h after a failed compaction = %+v", d)
	}
}

// DeepSeek 6: the timer retries a computer whose last attempt failed; the
// loop and a by-hand call still keep the gap.
func TestStoreCompactTimerRetriesAfterFailedAttempt(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-retry", 80, 72)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	recordAttempt(t, h, storeCompactAttempt{VMID: paperVMID(t, h, "paper-retry"), At: time.Now().Add(-time.Hour), Failed: true})
	if d := h.storeCompactPressureTick(context.Background(), time.Now()); len(d) != 1 || d[0].Action != pressureActionRateLimited {
		t.Fatalf("loop after a failed attempt = %+v", d)
	}
	if code, _ := postCompactStore(t, h, `{"user_id":"paper-retry","desktop_id":"primary"}`); code != http.StatusTooManyRequests {
		t.Fatalf("by hand after a failed attempt: %d", code)
	}
	_, resp, body := postSweep(t, h, `{"min_file_bytes":1}`)
	if resp.Selected[0].Result == nil || resp.Selected[0].Result.Outcome != deployRefreshRefreshed || len(compactor.calls) != 1 {
		t.Fatalf("timer after a failed attempt: %s", body)
	}
}

// N4: a hung compaction ends at its deadline; the computer boots its
// untouched image and the compaction lock is released.
func TestStoreCompactDeadlineBootsOriginal(t *testing.T) {
	h, mgr, _, _ := pressureFixture(t, "paper-hang", 80, 72)
	state := h.storeCompaction()
	state.compactor = blockingCompactor{}
	state.timeout = 50 * time.Millisecond
	done := make(chan struct{})
	var code int
	var resp storeCompactResponse
	go func() {
		code, resp = postCompactStore(t, h, `{"user_id":"paper-hang","desktop_id":"primary"}`)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a hung compaction did not end at its deadline")
	}
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || len(mgr.refreshes) != 1 ||
		resp.Compaction == nil || !strings.Contains(resp.Compaction.Error, "deadline") {
		t.Fatalf("hung compaction: %d %+v", code, resp)
	}
	if !state.mu.TryLock() {
		t.Fatal("compaction lock still held after the deadline")
	}
	state.mu.Unlock()
}

// N4: the transient unit carries the deadline too, so a stuck tool is
// stopped by systemd even if vmctl's wait is cut.
func TestHostStoreCompactorBoundsTheUnit(t *testing.T) {
	c, runner, img := compactorFixture(t, goodReport, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	if _, err := c.Compact(ctx, "vm-1", img, "computer-1", "head"); err != nil {
		t.Fatal(err)
	}
	found := false
	runner.mu.Lock()
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "systemd-run") && !strings.Contains(call, "-cleanup") && strings.Contains(call, "-p RuntimeMaxSec=") {
			found = true
		}
	}
	runner.mu.Unlock()
	if !found {
		t.Fatalf("compaction unit has no RuntimeMaxSec: %v", runner.calls)
	}
}

// N5: a panic inside the maintenance hook boots the untouched image.
func TestStoreCompactHookPanicBootsOriginal(t *testing.T) {
	h, mgr, _, _ := pressureFixture(t, "paper-hookpanic", 80, 72)
	h.storeCompaction().compactor = panickingCompactor{}
	code, resp := postCompactStore(t, h, `{"user_id":"paper-hookpanic","desktop_id":"primary"}`)
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || len(mgr.refreshes) != 1 ||
		resp.Compaction == nil || !strings.Contains(resp.Compaction.Error, "panic") {
		t.Fatalf("hook panic: %d %+v refreshes=%v", code, resp, mgr.refreshes)
	}
}

// N6: a future-dated attempt blocks for at most one gap, and is stored in UTC.
func TestStoreCompactFutureAttemptClamped(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-future", 80, 72)
	path := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), path); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	recordAttempt(t, h, storeCompactAttempt{VMID: paperVMID(t, h, "paper-future"), At: t0.Add(365 * 24 * time.Hour)})
	d := h.storeCompactPressureTick(context.Background(), t0)
	if len(d) != 1 || d[0].Action != pressureActionRateLimited {
		t.Fatalf("future attempt = %+v", d)
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(6*time.Hour+time.Minute)); len(d) != 1 || d[0].Action != pressureActionCompact || len(compactor.calls) != 1 {
		t.Fatalf("one gap after a future-dated attempt = %+v", d)
	}
	var onDisk map[string]map[string]any
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	for _, att := range onDisk {
		if at, _ := att["at"].(string); !strings.HasSuffix(at, "Z") {
			t.Fatalf("attempt time %q not UTC", at)
		}
	}
}

// N7: the gap refusal names force, and a forced bypass is logged.
func TestStoreCompactGapNamesForceAndForceIsLogged(t *testing.T) {
	h, _, _, _ := pressureFixture(t, "paper-force", 80, 72)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	recordAttempt(t, h, storeCompactAttempt{VMID: paperVMID(t, h, "paper-force"), At: time.Now().Add(-time.Hour)})
	code, resp := postCompactStore(t, h, `{"user_id":"paper-force","desktop_id":"primary"}`)
	if code != http.StatusTooManyRequests || !strings.Contains(resp.Reason, `{"force":true}`) {
		t.Fatalf("gap refusal: %d %q", code, resp.Reason)
	}
	logs := captureLog(t)
	if code, _ := postCompactStore(t, h, `{"user_id":"paper-force","desktop_id":"primary","force":true}`); code != http.StatusOK {
		t.Fatalf("forced: %d", code)
	}
	if !strings.Contains(logs.String(), "forced past the gap") || !strings.Contains(logs.String(), "forced=true") {
		t.Fatalf("log = %q", logs.String())
	}
}

// N8: a corrupt record is not replaced when the guest refuses (no
// compaction happened), so the loop keeps refusing.
func TestStoreCompactCorruptRecordSurvivesRefusedCompaction(t *testing.T) {
	h, _, marker, _ := pressureFixture(t, "paper-busy-corrupt", 80, 72)
	path := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), path)
	marker.outcome, marker.detail = guestMarkBusy, "self-development apply in flight"
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-busy-corrupt","desktop_id":"primary"}`); code != http.StatusConflict || resp.Outcome != deployRefreshDeferred {
		t.Fatalf("busy: %d %+v", code, resp)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "null" {
		t.Fatalf("corrupt record replaced by a refused compaction: %q", raw)
	}
	if _, err := os.Stat(path + ".corrupt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt record moved aside by a refused compaction: %v", err)
	}
	if d := h.storeCompactPressureTick(context.Background(), time.Now()); len(d) != 1 || d[0].Action != pressureActionRefused {
		t.Fatalf("tick = %+v, want still refused", d)
	}
}

// N8: when the corrupt record cannot be moved aside, it is not overwritten
// (the load error stays); the compaction still runs (the VM is down).
func TestStoreCompactCorruptRecordKeptWhenMoveFails(t *testing.T) {
	h, mgr, _, compactor := pressureFixture(t, "paper-move-fails", 80, 72)
	path := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path+".corrupt", "occupied"), 0o700); err != nil {
		t.Fatal(err)
	}
	_ = h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), path)
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-move-fails","desktop_id":"primary"}`); code != http.StatusOK || len(compactor.calls) != 1 || len(mgr.refreshes) != 1 {
		t.Fatalf("compaction: %d %+v", code, resp)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "null" {
		t.Fatalf("corrupt record overwritten without being kept: %q", raw)
	}
	if _, _, loadErr := h.storeCompaction().attemptsRecord().lastAttempt("x"); loadErr == nil {
		t.Fatal("load error cleared although the record was not rewritten")
	}
}

// DeepSeek 5: a stale copy from an interrupted run is removed before the
// floor is checked, under the compaction lock.
func TestStoreCompactRemovesStaleCopyBeforeFloor(t *testing.T) {
	h, _, _, _ := pressureFixture(t, "paper-stale", 80, 10)
	cc := &cleaningCompactor{}
	state := h.storeCompaction()
	state.compactor = cc
	state.minHostFree = 24 << 30
	if code, _ := postCompactStore(t, h, `{"user_id":"paper-stale","desktop_id":"primary"}`); code != http.StatusInsufficientStorage {
		t.Fatalf("below floor: %d", code)
	}
	vm := paperVMID(t, h, "paper-stale")
	if len(cc.cleaned) != 1 || !strings.HasPrefix(cc.cleaned[0], vm+"|") || !strings.HasSuffix(cc.cleaned[0], filepath.Join(vm, "data.img")) {
		t.Fatalf("cleanups = %v", cc.cleaned)
	}
}

// DeepSeek 8: a sweep whose compaction failed fails the timer unit.
func TestCompactSweepFailsOnCompactionFailure(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-sweep-fail", 80, 72)
	compactor.err = errors.New("projection-compact: refused")
	if code, _, body := postSweep(t, h, `{"min_file_bytes":1}`); code != http.StatusInternalServerError {
		t.Fatalf("sweep with a failed compaction: %d %s", code, body)
	}
}
