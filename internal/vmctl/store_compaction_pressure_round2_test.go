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
	"sync/atomic"
	"testing"
	"time"
)

// Round 2 review findings on lane/fp0-disk-pressure 4515538eb
// (docs/problems/node-b-disk-fills-from-the-paper-data-image-2026-10-22.md,
// "Round 2"). Each test names the finding or the surviving mutant it kills.

// Finding 1: a record that is valid JSON but not an object (null, an array,
// a number) or truncated is a load error, and recording over it never
// panics (a panic in the maintenance hook left the paper killed and unbooted).
func TestLoadStoreCompactAttemptsRejectsNonObjects(t *testing.T) {
	for _, body := range []string{"null", "[]", "42", `"text"`, `{"vm-a":`, ""} {
		path := filepath.Join(t.TempDir(), "store-compact-attempts.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		a := loadStoreCompactAttempts(path)
		if a.loadErr == nil {
			t.Fatalf("%q loaded without error", body)
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%q: record panicked: %v", body, r)
				}
			}()
			if err := a.record(storeCompactAttempt{VMID: "vm-a", At: time.Now()}); err != nil {
				t.Fatalf("%q: record: %v", body, err)
			}
		}()
	}
	path := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if a := loadStoreCompactAttempts(path); a.loadErr != nil {
		t.Fatalf("{} is a valid empty record: %v", a.loadErr)
	}
	var zero storeCompactAttempts
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("record on a zero value panicked: %v", r)
			}
		}()
		_ = zero.record(storeCompactAttempt{VMID: "vm-a", At: time.Now()})
	}()
}

// Finding 1: a panic in a tick is recovered and logged; the loop survives.
func TestStoreCompactPressureTickRecoversPanic(t *testing.T) {
	h, _, _, _ := pressureFixture(t, "paper-panic", 80, 72)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	h.dataImageStatsFor = func(string) (DataImageStats, bool) { panic("stats exploded") }
	logs := captureLog(t)
	h.runStoreCompactPressureTick(context.Background(), time.Now())
	if !strings.Contains(logs.String(), "store compaction pressure: tick panicked: stats exploded") {
		t.Fatalf("log = %q", logs.String())
	}
}

// Finding 8: a corrupt record is moved aside by the next successful write,
// and that write clears the load error, so the trigger resumes.
func TestStoreCompactRewriteClearsLoadError(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-heal", 80, 72)
	path := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), path); err == nil {
		t.Fatal("null record loaded without error")
	}
	// The timer's compaction is not blocked by the corrupt record.
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-heal","desktop_id":"primary"}`); code != http.StatusOK || resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("compact-store over a corrupt record: %d %+v", code, resp)
	}
	if len(compactor.calls) != 1 {
		t.Fatalf("compactions = %v", compactor.calls)
	}
	if raw, err := os.ReadFile(path + ".corrupt"); err != nil || string(raw) != "null" {
		t.Fatalf("corrupt record not kept aside: %q %v", raw, err)
	}
	var onDisk map[string]storeCompactAttempt
	raw, _ := os.ReadFile(path)
	if err := json.Unmarshal(raw, &onDisk); err != nil || len(onDisk) != 1 {
		t.Fatalf("rewritten record = %s (%v)", raw, err)
	}
	if d := h.storeCompactPressureTick(context.Background(), time.Now()); len(d) != 1 || d[0].Action != pressureActionRateLimited {
		t.Fatalf("tick after the rewrite = %+v, want rate-limited, not refused", d)
	}
}

// Finding 5: the gap holds for every caller; only a by-hand force passes it.
func TestCompactStoreHonoursGapForEveryCallerUnlessForced(t *testing.T) {
	h, _, marker, compactor := pressureFixture(t, "paper-gap", 80, 72)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	if d := h.storeCompactPressureTick(context.Background(), time.Now()); len(d) != 1 || d[0].Action != pressureActionCompact {
		t.Fatalf("pressure tick = %+v", d)
	}
	// The 04:00 timer minutes later: skipped, not a second restart.
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/compact-sweep", bytes.NewBufferString(`{"min_file_bytes":1}`))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleCompactSweep(rec, req)
	var sweep storeCompactSweepResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sweep); err != nil {
		t.Fatalf("decode %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	if rec.Code != http.StatusOK || len(sweep.Selected) != 1 || sweep.Selected[0].Result == nil ||
		sweep.Selected[0].Result.Outcome != deployRefreshSkipped || !strings.Contains(sweep.Selected[0].Result.Reason, "last compaction attempt") {
		t.Fatalf("timer inside the gap: %d %s", rec.Code, rec.Body.String())
	}
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-gap","desktop_id":"primary"}`); code != http.StatusTooManyRequests || resp.Outcome != deployRefreshSkipped {
		t.Fatalf("by hand inside the gap: %d %+v", code, resp)
	}
	if len(marker.marks) != 1 || len(compactor.calls) != 1 {
		t.Fatalf("gap passed: marks=%v compactions=%v", marker.marks, compactor.calls)
	}
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-gap","desktop_id":"primary","force":true}`); code != http.StatusOK || resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("forced by hand: %d %+v", code, resp)
	}
	if len(compactor.calls) != 2 {
		t.Fatalf("force did not compact: %v", compactor.calls)
	}
}

// Finding 6: a compaction never crash-boots a guest that cannot write the
// planned-restart marker, even an idle one; a plain deploy still may.
func TestCompactStoreNeverCrashBootsLegacyGuest(t *testing.T) {
	h, mgr, marker, compactor := compactFixture(t, "owner-legacy-compact")
	marker.outcome = guestMarkUnsupported
	h.registry.setGuestBusyProbeForTest(func(*VMOwnership) bool { return false })
	code, resp := postCompactStore(t, h, `{"user_id":"owner-legacy-compact","desktop_id":"primary"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || !strings.Contains(resp.Reason, "never crash-boots") {
		t.Fatalf("legacy guest compaction: %d %+v", code, resp)
	}
	if len(mgr.refreshes) != 0 || len(compactor.calls) != 0 {
		t.Fatalf("legacy guest refreshed=%v compacted=%v", mgr.refreshes, compactor.calls)
	}
}

// Finding 3: a timer run refused at the floor fails the unit (non-2xx) and
// says why; a dry run only reports.
func TestCompactSweepFailsLoudlyBelowFloor(t *testing.T) {
	h, _, marker, _ := pressureFixture(t, "paper-sweep-floor", 80, 10)
	h.storeCompaction().minHostFree = 24 << 30
	post := func(body string) (int, string) {
		req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/compact-sweep", bytes.NewBufferString(body))
		req.Header.Set("X-Internal-Caller", "true")
		rec := httptest.NewRecorder()
		h.HandleCompactSweep(rec, req)
		return rec.Code, rec.Body.String()
	}
	if code, body := post(`{"min_file_bytes":1,"dry_run":true}`); code != http.StatusOK {
		t.Fatalf("dry run below floor: %d %s", code, body)
	}
	logs := captureLog(t)
	code, body := post(`{"min_file_bytes":1}`)
	if code != http.StatusInsufficientStorage || !strings.Contains(body, "below the compaction floor 24.0 GiB") {
		t.Fatalf("sweep below floor: %d %s", code, body)
	}
	if !strings.Contains(logs.String(), "store compaction sweep refused") || len(marker.marks) != 0 {
		t.Fatalf("log = %q marks=%v", logs.String(), marker.marks)
	}
}

// Finding 4: low host free space alone does not restart a computer whose
// image is too small to give much back.
func TestStoreCompactPressureDueNeedsMinimumImage(t *testing.T) {
	cfg := pressureConfig(StoreCompactPressureOn)
	small := &dataImageResponse{FileBytes: 20 << 30, CapBytes: 125 << 30}
	big := &dataImageResponse{FileBytes: 40 << 30, CapBytes: 125 << 30}
	if r := pressureDue(cfg, 10<<30, nil, small); r != "" {
		t.Fatalf("20 GiB image due: %q", r)
	}
	if r := pressureDue(cfg, 10<<30, nil, big); !strings.Contains(r, "host free") {
		t.Fatalf("40 GiB image not due: %q", r)
	}
}

// Finding 4, revised in round 3 (N2): a compaction that gave back little
// backs the pressure loop off until the image has regrown, not for a span
// of wall-clock time that could alias with the daily timer.
func TestStoreCompactPressureBacksOffAfterSmallReclaim(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-backoff", 80, 72)
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
	if d := h.storeCompactPressureTick(context.Background(), t0); len(d) != 1 || d[0].Action != pressureActionCompact {
		t.Fatalf("first = %+v", d)
	}
	// The image (80 GiB) has not regrown 8 GiB past the 76 GiB the
	// compaction left: backed off, however long it has been.
	for _, after := range []time.Duration{7 * time.Hour, 25 * time.Hour, 49 * time.Hour} {
		d := h.storeCompactPressureTick(context.Background(), t0.Add(after))
		if len(d) != 1 || d[0].Action != pressureActionRateLimited || !strings.Contains(d[0].Reason, "reclaimed 4.0 GiB") {
			t.Fatalf("+%s after a small reclaim = %+v, want backed off", after, d)
		}
	}
	// Regrown to 85 GiB (9 GiB past 76): due again.
	h.dataImageStatsFor = func(string) (DataImageStats, bool) {
		return DataImageStats{FileBytes: 85 << 30, CapBytes: 125 << 30}, true
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(7*time.Hour)); len(d) != 1 || d[0].Action != pressureActionCompact || len(compactor.calls) != 2 {
		t.Fatalf("after regrowth = %+v", d)
	}
}

// Mutant M8: the attempt is kept in memory even when its write fails inside
// the hook (the VM is already down; the compaction proceeds).
func TestStoreCompactAttemptKeptInMemoryWhenHookWriteFails(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-m8", 80, 72)
	path := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), path); err != nil {
		t.Fatal(err)
	}
	var writes atomic.Int32
	h.storeCompaction().attemptsRecord().writeFile = func(path string, raw []byte) error {
		if writes.Add(1) > 1 {
			return errors.New("disk full")
		}
		return writeFileAtomic(path, raw)
	}
	t0 := time.Now()
	if d := h.storeCompactPressureTick(context.Background(), t0); len(d) != 1 || d[0].Action != pressureActionCompact || len(compactor.calls) != 1 {
		t.Fatalf("first = %+v calls=%v", d, compactor.calls)
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(time.Hour)); len(d) != 1 || d[0].Action != pressureActionRateLimited {
		t.Fatalf("after an unwritten attempt = %+v, want rate-limited from memory", d)
	}
}

// Mutant M9d: a compaction that fails still counts as an attempt, and the
// log names its error.
func TestStoreCompactFailedCompactionStillCountsAsAttempt(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-m9", 80, 72)
	compactor.err = errors.New("projection-compact: refused")
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	logs := captureLog(t)
	t0 := time.Now()
	h.storeCompactPressureTick(context.Background(), t0)
	if !strings.Contains(logs.String(), `compaction_error="projection-compact: refused"`) {
		t.Fatalf("log = %q", logs.String())
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(time.Hour)); len(d) != 1 || d[0].Action != pressureActionRateLimited {
		t.Fatalf("after a failed compaction = %+v, want rate-limited", d)
	}
}

// Mutants M11/M12: the real state constructor reads the floor from the
// environment and free space from statfs on the VM state dir.
func TestStoreCompactionStateFromEnvReadsStatfs(t *testing.T) {
	t.Setenv("VMCTL_STORE_COMPACT_MIN_HOST_FREE_GIB", "7")
	reg := NewOwnershipRegistry("")
	dir := t.TempDir()
	reg.SetPressureReclaimConfig(PressureReclaimConfig{StateDir: dir})
	h := NewHandler(reg)
	state := h.storeCompaction()
	if state.minHostFree != 7<<30 {
		t.Fatalf("floor = %d, want 7 GiB from the environment", state.minHostFree)
	}
	free, err := state.hostFree()
	want, _, _ := statfsAvailable(dir)
	if err != nil || free == 0 || free > want+(1<<30) || free+(1<<30) < want {
		t.Fatalf("hostFree = %d, %v; statfs says %d", free, err, want)
	}
	reg.SetPressureReclaimConfig(PressureReclaimConfig{StateDir: filepath.Join(dir, "missing")})
	if r := state.hostFloorRefusal(); !strings.Contains(r, "unreadable") {
		t.Fatalf("missing state dir: refusal %q", r)
	}
}

// Mutant M13: observe never records an attempt; turning the trigger on
// afterwards compacts at once.
func TestStoreCompactObserveNeverRecordsAnAttempt(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-m13", 80, 72)
	path := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureObserve), path); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	h.storeCompactPressureTick(context.Background(), t0)
	h.storeCompactPressureTick(context.Background(), t0.Add(5*time.Minute))
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("observe wrote the attempts record: %v", err)
	}
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), path); err != nil {
		t.Fatal(err)
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(10*time.Minute)); len(d) != 1 || d[0].Action != pressureActionCompact || len(compactor.calls) != 1 {
		t.Fatalf("on after observe = %+v", d)
	}
}

// Mutant M16b: one compaction per tick, even with two due computers.
func TestStoreCompactPressureOneCompactionPerTick(t *testing.T) {
	h, _, _, compactor := pressureFixture(t, "paper-one", 80, 72)
	if _, err := h.registry.ResolveOrAssign("paper-two"); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, user := range []string{"paper-one", "paper-two"} {
		ids[h.registry.GetOwnershipForDesktop(user, PrimaryDesktopID).ComputerID] = true
	}
	h.registry.SetWarmnessPolicyConfig(WarmnessPolicyConfig{PlatformComputerIDs: ids})
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	d := h.storeCompactPressureTick(context.Background(), t0)
	compacts := 0
	for _, x := range d {
		if x.Action == pressureActionCompact {
			compacts++
		}
	}
	if compacts != 1 || len(compactor.calls) != 1 {
		t.Fatalf("first tick: %d compactions (%v), want one", compacts, compactor.calls)
	}
	h.storeCompactPressureTick(context.Background(), t0.Add(5*time.Minute))
	if len(compactor.calls) != 2 || compactor.calls[0] == compactor.calls[1] {
		t.Fatalf("second tick compactions = %v, want the other computer", compactor.calls)
	}
}
