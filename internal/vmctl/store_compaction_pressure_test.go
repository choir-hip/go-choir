package vmctl

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Failure modes written before the code
// (docs/problems/node-b-disk-fills-from-the-paper-data-image-2026-10-22.md,
// "Pressure trigger design"): flapping, a vmctl restart forgetting the last
// attempt, an unreadable or unwritable attempts record, a compaction with no
// room for the copy, a busy guest, an account's computer, observe mode
// acting, unreadable free space.

// pressureFixture is a platform computer whose data image holds fileGiB of
// a 125 GiB cap, on a host with freeGiB free.
func pressureFixture(t *testing.T, user string, fileGiB, freeGiB uint64) (*Handler, *mockVMManager, *fakeGuestMarker, *fakeCompactor) {
	t.Helper()
	h, mgr, marker, compactor := compactFixture(t, user)
	makePlatformComputer(t, h, user)
	h.dataImageStatsFor = func(string) (DataImageStats, bool) {
		return DataImageStats{FileBytes: fileGiB << 30, CapBytes: 125 << 30}, true
	}
	state := h.storeCompaction()
	state.freeBytes = func() (uint64, error) { return freeGiB << 30, nil }
	marker.outcome = guestMarkWritten
	return h, mgr, marker, compactor
}

func pressureConfig(mode string) StoreCompactPressureConfig {
	return StoreCompactPressureConfig{
		Mode:          mode,
		Interval:      5 * time.Minute,
		HostFreeBelow: 64 << 30,
		FillFraction:  0.6,
		MinGap:        6 * time.Hour,
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestStoreCompactPressureConfigFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cfg := StoreCompactPressureConfigFromEnv(env(nil))
	if cfg.Mode != StoreCompactPressureOff || cfg.Interval != 5*time.Minute || cfg.HostFreeBelow != 64<<30 ||
		cfg.FillFraction != 0.6 || cfg.MinGap != 6*time.Hour {
		t.Fatalf("defaults = %+v", cfg)
	}
	cfg = StoreCompactPressureConfigFromEnv(env(map[string]string{
		"VMCTL_STORE_COMPACT_PRESSURE":               "observe",
		"VMCTL_STORE_COMPACT_PRESSURE_INTERVAL":      "1m",
		"VMCTL_STORE_COMPACT_PRESSURE_HOST_FREE_GIB": "80",
		"VMCTL_STORE_COMPACT_PRESSURE_FILL_FRACTION": "0.5",
		"VMCTL_STORE_COMPACT_PRESSURE_MIN_GAP":       "12h",
	}))
	if cfg.Mode != StoreCompactPressureObserve || cfg.Interval != time.Minute || cfg.HostFreeBelow != 80<<30 ||
		cfg.FillFraction != 0.5 || cfg.MinGap != 12*time.Hour {
		t.Fatalf("parsed = %+v", cfg)
	}
	// An unknown mode is off; a gap under an hour is raised to an hour (a
	// misread setting must not compact the paper every tick); invalid
	// numbers keep their defaults.
	cfg = StoreCompactPressureConfigFromEnv(env(map[string]string{
		"VMCTL_STORE_COMPACT_PRESSURE":               "yes",
		"VMCTL_STORE_COMPACT_PRESSURE_MIN_GAP":       "5m",
		"VMCTL_STORE_COMPACT_PRESSURE_FILL_FRACTION": "1.5",
		"VMCTL_STORE_COMPACT_PRESSURE_HOST_FREE_GIB": "lots",
	}))
	if cfg.Mode != StoreCompactPressureOff || cfg.MinGap != time.Hour || cfg.FillFraction != 0.6 || cfg.HostFreeBelow != 64<<30 {
		t.Fatalf("guarded = %+v", cfg)
	}
}

func TestStoreCompactMinHostFreeFromEnv(t *testing.T) {
	if got := storeCompactMinHostFreeFromEnv(func(string) string { return "" }); got != 32<<30 {
		t.Fatalf("default floor = %d, want 32 GiB", got)
	}
	if got := storeCompactMinHostFreeFromEnv(func(string) string { return "0" }); got != 0 {
		t.Fatalf("floor 0 = %d, want off", got)
	}
	if got := storeCompactMinHostFreeFromEnv(func(string) string { return "bad" }); got != 32<<30 {
		t.Fatalf("invalid floor = %d, want the default", got)
	}
}

// Observe mode names the computer and the reason and does nothing: no
// planned-restart request, no refresh, no compaction (staging proof shape).
func TestStoreCompactPressureObserveLogsAndDoesNothing(t *testing.T) {
	h, mgr, marker, compactor := pressureFixture(t, "paper-observe", 80, 72)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureObserve), ""); err != nil {
		t.Fatal(err)
	}
	logs := captureLog(t)
	decisions := h.storeCompactPressureTick(context.Background(), time.Now())
	if len(decisions) != 1 || decisions[0].Action != pressureActionObserve || !strings.Contains(decisions[0].Reason, "64% of cap") {
		t.Fatalf("decisions = %+v, want one observe decision on the image fill", decisions)
	}
	if len(marker.marks) != 0 || len(mgr.refreshes) != 0 || len(compactor.calls) != 0 {
		t.Fatalf("observe acted: marks=%v refreshes=%v compactions=%v", marker.marks, mgr.refreshes, compactor.calls)
	}
	line := logs.String()
	if !strings.Contains(line, "store compaction pressure (observe): would compact "+decisions[0].VMID) || !strings.Contains(line, "64% of cap") {
		t.Fatalf("log = %q", line)
	}
}

func TestStoreCompactPressureOffDoesNothing(t *testing.T) {
	h, _, marker, compactor := pressureFixture(t, "paper-off", 120, 10)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOff), ""); err != nil {
		t.Fatal(err)
	}
	if d := h.storeCompactPressureTick(context.Background(), time.Now()); len(d) != 0 || len(marker.marks) != 0 || len(compactor.calls) != 0 {
		t.Fatalf("off acted: %+v %v %v", d, marker.marks, compactor.calls)
	}
}

func TestStoreCompactPressureNotDueDoesNothing(t *testing.T) {
	h, _, marker, _ := pressureFixture(t, "paper-calm", 20, 200)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	if d := h.storeCompactPressureTick(context.Background(), time.Now()); len(d) != 0 || len(marker.marks) != 0 {
		t.Fatalf("calm host acted: %+v %v", d, marker.marks)
	}
}

func TestStoreCompactPressureHostFreeTriggers(t *testing.T) {
	h, _, _, _ := pressureFixture(t, "paper-free", 20, 50)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureObserve), ""); err != nil {
		t.Fatal(err)
	}
	d := h.storeCompactPressureTick(context.Background(), time.Now())
	if len(d) != 1 || !strings.Contains(d[0].Reason, "host free 50.0 GiB below 64.0 GiB") {
		t.Fatalf("decisions = %+v", d)
	}
}

// Flapping and restarts: one compaction per computer per gap, counted from
// the attempt and persisted, so a new vmctl process honours it.
func TestStoreCompactPressureOnceThenRateLimitedAcrossRestart(t *testing.T) {
	h, _, marker, compactor := pressureFixture(t, "paper-on", 80, 72)
	attempts := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), attempts); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	d := h.storeCompactPressureTick(context.Background(), t0)
	if len(d) != 1 || d[0].Action != pressureActionCompact || d[0].Result == nil || d[0].Result.Outcome != deployRefreshRefreshed {
		t.Fatalf("first tick = %+v, want a planned-restart compaction", d)
	}
	if len(compactor.calls) != 1 || len(marker.marks) != 1 || !strings.HasSuffix(marker.marks[0], "@"+storeCompactTarget) {
		t.Fatalf("compactions=%v marks=%v", compactor.calls, marker.marks)
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(time.Hour)); len(d) != 1 || d[0].Action != pressureActionRateLimited {
		t.Fatalf("second tick = %+v, want rate-limited", d)
	}
	if len(compactor.calls) != 1 || len(marker.marks) != 1 {
		t.Fatalf("rate-limited tick acted: %v %v", compactor.calls, marker.marks)
	}

	// A new vmctl process: fresh compaction state, the record read back.
	fresh := &storeCompactState{compactor: compactor, freeBytes: h.storeCompaction().freeBytes}
	h.storeCompact = fresh
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), attempts); err != nil {
		t.Fatal(err)
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(2*time.Hour)); len(d) != 1 || d[0].Action != pressureActionRateLimited {
		t.Fatalf("after restart = %+v, want rate-limited", d)
	}
	if len(compactor.calls) != 1 || len(marker.marks) != 1 {
		t.Fatalf("restarted vmctl compacted inside the gap: %v %v", compactor.calls, marker.marks)
	}
	if d := h.storeCompactPressureTick(context.Background(), t0.Add(7*time.Hour)); len(d) != 1 || d[0].Action != pressureActionCompact || len(compactor.calls) != 2 {
		t.Fatalf("after the gap = %+v, want a compaction", d)
	}
}

// The timer's compaction counts as an attempt too.
func TestStoreCompactPressureCountsTimerCompaction(t *testing.T) {
	h, _, _, _ := pressureFixture(t, "paper-timer", 80, 72)
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	if code, resp := postCompactStore(t, h, `{"user_id":"paper-timer","desktop_id":"primary"}`); code != http.StatusOK {
		t.Fatalf("compact-store: %d %+v", code, resp)
	}
	if d := h.storeCompactPressureTick(context.Background(), time.Now()); len(d) != 1 || d[0].Action != pressureActionRateLimited {
		t.Fatalf("tick after a by-hand compaction = %+v, want rate-limited", d)
	}
}

// A self-development apply or a live capsule refuses the mark: not an
// attempt, asked again next tick, never restarted.
func TestStoreCompactPressureBusyGuestIsNotAnAttempt(t *testing.T) {
	h, mgr, marker, compactor := pressureFixture(t, "paper-busy", 80, 72)
	marker.outcome, marker.detail = guestMarkBusy, "engineering capsule live"
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	d := h.storeCompactPressureTick(context.Background(), t0)
	if len(d) != 1 || d[0].Result == nil || d[0].Result.Outcome != deployRefreshDeferred {
		t.Fatalf("busy tick = %+v", d)
	}
	h.storeCompactPressureTick(context.Background(), t0.Add(5*time.Minute))
	if len(marker.marks) != 2 || len(mgr.refreshes) != 0 || len(compactor.calls) != 0 {
		t.Fatalf("busy guest: marks=%v refreshes=%v compactions=%v; want asked twice, never restarted", marker.marks, mgr.refreshes, compactor.calls)
	}
}

// An account's computer is never selected, however full.
func TestStoreCompactPressureSkipsAccountComputer(t *testing.T) {
	h, _, marker, _ := compactFixture(t, "account-full")
	h.dataImageStatsFor = func(string) (DataImageStats, bool) {
		return DataImageStats{FileBytes: 120 << 30, CapBytes: 125 << 30}, true
	}
	h.storeCompaction().freeBytes = func() (uint64, error) { return 40 << 30, nil }
	marker.outcome = guestMarkWritten
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	if d := h.storeCompactPressureTick(context.Background(), time.Now()); len(d) != 0 || len(marker.marks) != 0 {
		t.Fatalf("account computer selected: %+v %v", d, marker.marks)
	}
}

// An unreadable attempts record refuses every pressure compaction.
func TestStoreCompactPressureUnreadableAttemptsRefuses(t *testing.T) {
	h, _, marker, _ := pressureFixture(t, "paper-corrupt", 80, 72)
	attempts := filepath.Join(t.TempDir(), "store-compact-attempts.json")
	if err := os.WriteFile(attempts, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), attempts); err == nil {
		t.Fatalf("a corrupt attempts record loaded without error")
	}
	d := h.storeCompactPressureTick(context.Background(), time.Now())
	if len(d) != 1 || d[0].Action != pressureActionRefused || !strings.Contains(d[0].Reason, "attempts record") || len(marker.marks) != 0 {
		t.Fatalf("decisions = %+v marks=%v", d, marker.marks)
	}
}

// An attempts record that cannot be written fails the maintenance hook
// before the compaction: the computer boots its untouched image.
func TestStoreCompactAttemptWriteFailureSkipsCompaction(t *testing.T) {
	h, mgr, _, compactor := pressureFixture(t, "paper-ro", 80, 72)
	attempts := filepath.Join(t.TempDir(), "missing-dir", "store-compact-attempts.json")
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), attempts); err != nil {
		t.Fatal(err)
	}
	d := h.storeCompactPressureTick(context.Background(), time.Now())
	if len(d) != 1 || d[0].Result == nil || d[0].Result.Compaction == nil || !strings.Contains(d[0].Result.Compaction.Error, "attempt") {
		t.Fatalf("decisions = %+v", d)
	}
	if len(compactor.calls) != 0 || len(mgr.refreshes) != 1 {
		t.Fatalf("compactions=%v refreshes=%v; want no compaction and the computer booted", compactor.calls, mgr.refreshes)
	}
}

// The floor refuses every caller before the guest is asked: below it the
// compacted copy has no room.
func TestCompactStoreRefusesBelowHostFreeFloor(t *testing.T) {
	h, mgr, marker, compactor := pressureFixture(t, "paper-floor", 80, 20)
	h.storeCompaction().minHostFree = 32 << 30
	code, resp := postCompactStore(t, h, `{"user_id":"paper-floor","desktop_id":"primary"}`)
	if code != http.StatusInsufficientStorage || resp.Outcome != deployRefreshSkipped ||
		!strings.Contains(resp.Reason, "host free 20.0 GiB below the compaction floor 32.0 GiB") {
		t.Fatalf("floor: %d %+v", code, resp)
	}
	if len(marker.marks) != 0 || len(mgr.refreshes) != 0 || len(compactor.calls) != 0 {
		t.Fatalf("floor refusal acted: %v %v %v", marker.marks, mgr.refreshes, compactor.calls)
	}
}

func TestCompactStoreRefusesWhenHostFreeUnreadable(t *testing.T) {
	h, _, marker, _ := pressureFixture(t, "paper-statfs", 80, 72)
	state := h.storeCompaction()
	state.minHostFree = 32 << 30
	state.freeBytes = func() (uint64, error) { return 0, errors.New("statfs: no such file") }
	code, resp := postCompactStore(t, h, `{"user_id":"paper-statfs","desktop_id":"primary"}`)
	if code != http.StatusInsufficientStorage || resp.Outcome != deployRefreshSkipped || !strings.Contains(resp.Reason, "unreadable") || len(marker.marks) != 0 {
		t.Fatalf("unreadable free space: %d %+v marks=%v", code, resp, marker.marks)
	}
}

// In observe and on modes the loop reports a floor refusal instead of a
// would-compact, so the log says why nothing would happen.
func TestStoreCompactPressureReportsFloor(t *testing.T) {
	h, _, marker, _ := pressureFixture(t, "paper-low", 80, 20)
	h.storeCompaction().minHostFree = 32 << 30
	if err := h.SetStoreCompactPressure(pressureConfig(StoreCompactPressureOn), ""); err != nil {
		t.Fatal(err)
	}
	d := h.storeCompactPressureTick(context.Background(), time.Now())
	if len(d) != 1 || d[0].Action != pressureActionRefused || !strings.Contains(d[0].Reason, "compaction floor") || len(marker.marks) != 0 {
		t.Fatalf("decisions = %+v marks=%v", d, marker.marks)
	}
}
