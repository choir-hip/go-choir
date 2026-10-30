package vmctl

// A crashed computer comes back by itself, with its work closed, unless it is
// crash-looping (CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned
// Update)"; crash_loop.go). The ways this could fail, written before the code:
//
//   - restart loop: a computer that crashes, is restarted by the next request,
//     and crashes again is restarted forever. The third crash inside the
//     window must hold it; the held computer must answer every later request
//     with the visible cause, never boot, never hang a request.
//   - a restart that cannot boot (a crash during boot) is retried by every
//     request without bound: a restart whose guest did not come up counts as
//     a crash, but a refusal that spawned nothing (host memory pressure,
//     another start in progress) and a host or control-plane failure (no
//     credential) do not, or an outage would hold every computer restarted
//     during it.
//   - a planned restart counted as a crash: a deploy refresh, an owner stop
//     and start, a vmctl restart and a refresh claim in flight must leave the
//     crash history empty, or planned updates would trip the bound.
//   - the same crash counted twice: racing lookups, or a realization that
//     flaps healthy/unhealthy, record one crash per realization.
//   - owner start not clearing the hold: `computer start` must clear the
//     crash history and a crash-loop hold, restart the computer, and must not
//     clear a maintenance hold an operator set.
//   - the hold lost on a vmctl restart: history and hold are durable.
//   - the window never slides: crashes older than the window must not count.
//   - double restart from two wake paths racing: concurrent resolves of one
//     crashed computer boot it once.
//   - a restart mid self-development apply or with a live capsule: the
//     automatic restart is refused when the guest answers and reports either.
//   - a wedge stop is a crash: a guest that wedges every few minutes loops
//     through the page-load boot just as a crashing one does.
//   - the platform computer's held answer: a crash-held platform computer
//     answers with the refusal, not an old URL.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type crashLoopEnv struct {
	reg     *OwnershipRegistry
	h       *Handler
	mgr     *mockVMManager
	now     *time.Time
	healthy *bool
	path    string
	epoch   int64
}

const (
	crashUser     = "crash-user"
	crashVM       = "vm-crash"
	crashComputer = "computer-crash-0001"
)

func newCrashLoopEnv(t *testing.T) *crashLoopEnv {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	path := filepath.Join(t.TempDir(), "ownership.json")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	reg.clock = func() time.Time { return now }
	healthy := true
	mgr := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			crashVM: {HostURL: "http://127.0.0.1:9001", Epoch: 7, Healthy: true, State: "running"},
		},
		checkHealthOK: &healthy,
	}
	reg.SetVMManager(mgr)
	// The guest is dead or silent unless a test says it answers.
	reg.setHostStopHealthProbeForTest(func(*VMOwnership) hostStopHealth { return hostStopHealth{} })
	own := &VMOwnership{
		VMID: crashVM, ComputerID: crashComputer, UserID: crashUser, DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, ComputerURL: "http://127.0.0.1:9001", State: VMStateActive, Epoch: 7,
	}
	addOwnership(reg, own)
	return &crashLoopEnv{reg: reg, h: NewHandler(reg), mgr: mgr, now: &now, healthy: &healthy, path: path, epoch: 7}
}

func (e *crashLoopEnv) lookup(t *testing.T) ownershipResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/lookup?computer_id="+crashComputer, nil)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	e.h.HandleLookup(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("lookup status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out ownershipResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode lookup: %v", err)
	}
	return out
}

// crash is the guest dying: the next lookup finds it unhealthy past any grace
// and latches degraded, which is the one place vmctl records a crash.
func (e *crashLoopEnv) crash(t *testing.T) ownershipResponse {
	t.Helper()
	*e.healthy = false
	got := e.lookup(t)
	if got.State != string(VMStateDegraded) {
		t.Fatalf("lookup after the crash = %q, want degraded", got.State)
	}
	return got
}

type resolveOutcome struct {
	status int
	state  string
	kind   string
	reason string
}

func (e *crashLoopEnv) resolveBody(t *testing.T, body string) resolveOutcome {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", strings.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	rec := httptest.NewRecorder()
	e.h.HandleResolve(rec, req.WithContext(ctx))
	out := resolveOutcome{status: rec.Code}
	var decoded struct {
		State  string `json:"state"`
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	out.state, out.kind, out.reason = decoded.State, decoded.Kind, decoded.Reason
	return out
}

// wake is a request's resolve of the crashed computer (the cookie path and the
// bearer wake both arrive here). The next boot gets a fresh epoch.
func (e *crashLoopEnv) wake(t *testing.T) resolveOutcome {
	t.Helper()
	e.epoch++
	e.mgr.mu.Lock()
	e.mgr.recoverResponse = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: e.epoch, Healthy: true, State: "running"}
	e.mgr.mu.Unlock()
	out := e.resolveBody(t, `{"user_id":"`+crashUser+`"}`)
	if out.status == http.StatusOK {
		*e.healthy = true
	}
	return out
}

func (e *crashLoopEnv) ownerStart(t *testing.T) resolveOutcome {
	t.Helper()
	e.epoch++
	e.mgr.mu.Lock()
	e.mgr.recoverResponse = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: e.epoch, Healthy: true, State: "running"}
	e.mgr.mu.Unlock()
	out := e.resolveBody(t, `{"user_id":"`+crashUser+`","owner_start":true}`)
	if out.status == http.StatusOK {
		*e.healthy = true
	}
	return out
}

func (e *crashLoopEnv) stored() *VMOwnership {
	return e.reg.GetOwnershipForDesktop(crashUser, PrimaryDesktopID)
}

func (e *crashLoopEnv) recovers() int {
	e.mgr.mu.Lock()
	defer e.mgr.mu.Unlock()
	return len(e.mgr.recovers)
}

// tripTheBound runs crash, wake, crash, wake, crash: the third crash holds.
func (e *crashLoopEnv) tripTheBound(t *testing.T) {
	t.Helper()
	for i := 0; i < crashLoopMaxCrashes-1; i++ {
		e.crash(t)
		if out := e.wake(t); out.status != http.StatusOK || out.state != string(VMStateActive) {
			t.Fatalf("wake %d after a crash = %+v, want 200 active", i+1, out)
		}
		*e.now = e.now.Add(time.Minute)
	}
	e.crash(t)
}

func TestCrashRestartsByItselfUntilTheThirdCrashInTheWindow(t *testing.T) {
	e := newCrashLoopEnv(t)

	// Crashes one and two: each is restarted by the next request.
	for i := 1; i <= crashLoopMaxCrashes-1; i++ {
		got := e.crash(t)
		if got.Held {
			t.Fatalf("crash %d held the computer: %+v", i, got)
		}
		out := e.wake(t)
		if out.status != http.StatusOK || out.state != string(VMStateActive) {
			t.Fatalf("wake after crash %d = %+v, want 200 active", i, out)
		}
		*e.now = e.now.Add(time.Minute)
	}
	if got := e.recovers(); got != 2 {
		t.Fatalf("recoveries = %d, want 2", got)
	}

	// Crash three inside the window: held, with the cause, not restarted.
	got := e.crash(t)
	if !got.Held || got.HoldReason != "crash loop: 3 crashes in 30m" || got.HeldBy != CrashLoopHeldBy {
		t.Fatalf("lookup after the third crash = held=%v reason=%q by=%q, want the crash-loop hold", got.Held, got.HoldReason, got.HeldBy)
	}
	// Every request, however many, is told why. None hangs, none boots.
	for i := 0; i < 5; i++ {
		out := e.wake(t)
		if out.status != http.StatusServiceUnavailable || out.kind != string(RecoveryRefusalCrashLoop) || out.reason != "crash loop: 3 crashes in 30m" {
			t.Fatalf("wake %d of a crash-looping computer = %+v, want a 503 crash_loop refusal", i+1, out)
		}
	}
	if got := e.recovers(); got != 2 {
		t.Fatalf("a held computer was restarted: recoveries = %d, want 2", got)
	}
	if own := e.stored(); own == nil || own.State != VMStateStopped || !own.IsHeld() {
		t.Fatalf("held computer = %+v, want stopped and held", own)
	}
	if got := e.lookup(t); got.State != string(VMStateStopped) || !got.Held {
		t.Fatalf("lookup of the held computer = %+v, want stopped and held", got)
	}
}

// The browser path meets a crash in resolve, not at the lookup latch: an
// active computer whose guest process exited (the manager's instance is
// failed) is restarted by the request that finds it, and counted there too.
func TestCrashFoundByAResolveIsRecordedAndBounded(t *testing.T) {
	e := newCrashLoopEnv(t)
	for i := 1; i <= crashLoopMaxCrashes; i++ {
		e.mgr.mu.Lock()
		e.mgr.getVMs[crashVM] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: e.epoch, State: "failed"}
		e.epoch++
		e.mgr.refreshResponse = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: e.epoch, Healthy: true, State: "running"}
		e.mgr.mu.Unlock()
		_, err := e.reg.ResolveOrAssignDesktopContext(context.Background(), crashUser, PrimaryDesktopID)
		if i < crashLoopMaxCrashes {
			if err != nil {
				t.Fatalf("resolve %d of a crashed active computer: %v", i, err)
			}
			if own := e.stored(); own.State != VMStateActive || own.Epoch != e.epoch {
				t.Fatalf("after resolve %d: state=%q epoch=%d, want the restarted realization %d", i, own.State, own.Epoch, e.epoch)
			}
			*e.now = e.now.Add(time.Minute)
			continue
		}
		if refusal := RecoveryRefusalFrom(err); refusal == nil || refusal.Kind != RecoveryRefusalCrashLoop {
			t.Fatalf("resolve %d of a crash-looping computer = %v, want the crash_loop refusal", i, err)
		}
	}
	e.mgr.mu.Lock()
	restarts := len(e.mgr.refreshes)
	e.mgr.mu.Unlock()
	if restarts != crashLoopMaxCrashes-1 {
		t.Fatalf("restarts = %d, want %d (the third crash is held, not restarted)", restarts, crashLoopMaxCrashes-1)
	}
	// Held and latched degraded, so the next request meets the hold at once
	// instead of the old URL of a dead guest.
	if own := e.stored(); own.State != VMStateDegraded || !crashLoopHeld(own) {
		t.Fatalf("after the third crash: state=%q held=%v, want degraded and crash-loop held", own.State, own.IsHeld())
	}
	if out := e.wake(t); out.status != http.StatusServiceUnavailable || out.kind != string(RecoveryRefusalCrashLoop) {
		t.Fatalf("request after the hold = %+v, want the crash_loop refusal", out)
	}
}

func TestCrashWindowSlides(t *testing.T) {
	e := newCrashLoopEnv(t)
	e.crash(t)
	e.wake(t)
	*e.now = e.now.Add(time.Minute)
	e.crash(t)
	e.wake(t)
	// The first two crashes leave the window before the third.
	*e.now = e.now.Add(crashLoopWindow + time.Second)
	got := e.crash(t)
	if got.Held {
		t.Fatalf("crashes older than the window held the computer: %+v", got)
	}
	if own := e.stored(); len(own.Crashes) != 1 {
		t.Fatalf("crash history = %+v, want only the crash inside the window", own.Crashes)
	}
	if out := e.wake(t); out.status != http.StatusOK {
		t.Fatalf("wake = %+v, want 200", out)
	}
}

func TestCrashLoopHoldSurvivesAVMCTLRestart(t *testing.T) {
	e := newCrashLoopEnv(t)
	e.tripTheBound(t)

	reloaded := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reloaded.SetPersistencePath(e.path); err != nil {
		t.Fatalf("reload ownership: %v", err)
	}
	own := reloaded.GetOwnershipForDesktop(crashUser, PrimaryDesktopID)
	if own == nil || !own.IsHeld() || own.HoldStatus.HeldBy != CrashLoopHeldBy || len(own.Crashes) != crashLoopMaxCrashes {
		t.Fatalf("reloaded ownership = %+v, want the crash-loop hold and %d recorded crashes", own, crashLoopMaxCrashes)
	}
	if own.State != VMStateStopped {
		t.Fatalf("reloaded state = %q, want stopped", own.State)
	}
	mgr := &mockVMManager{getVMs: map[string]*VMInstanceInfo{}}
	reloaded.SetVMManager(mgr)
	_, err := reloaded.ResolveOrAssignDesktopContext(context.Background(), crashUser, PrimaryDesktopID)
	if refusal := RecoveryRefusalFrom(err); refusal == nil || refusal.Kind != RecoveryRefusalCrashLoop {
		t.Fatalf("resolve after a vmctl restart = %v, want the crash_loop refusal", err)
	}
	if len(mgr.boots) != 0 || len(mgr.recovers) != 0 {
		t.Fatalf("a held computer booted after a vmctl restart: boots=%d recovers=%d", len(mgr.boots), len(mgr.recovers))
	}
}

func TestOwnerStartClearsTheHoldAndTheHistory(t *testing.T) {
	e := newCrashLoopEnv(t)
	e.tripTheBound(t)
	if out := e.wake(t); out.status != http.StatusServiceUnavailable {
		t.Fatalf("wake of the held computer = %+v, want 503", out)
	}

	out := e.ownerStart(t)
	if out.status != http.StatusOK || out.state != string(VMStateActive) {
		t.Fatalf("owner start = %+v, want 200 active", out)
	}
	own := e.stored()
	if own.IsHeld() || len(own.Crashes) != 0 {
		t.Fatalf("after the owner start: held=%v crashes=%+v, want neither", own.IsHeld(), own.Crashes)
	}
	if got := e.recovers(); got != 3 {
		t.Fatalf("recoveries = %d, want 3 (two automatic, then the owner's start)", got)
	}
	// The history started over: one more crash is crash one, not crash four.
	if got := e.crash(t); got.Held {
		t.Fatalf("the crash after an owner start held the computer: %+v", got)
	}
	if out := e.wake(t); out.status != http.StatusOK {
		t.Fatalf("wake after the owner start = %+v, want 200", out)
	}
}

func TestOwnerStartDoesNotClearAMaintenanceHold(t *testing.T) {
	e := newCrashLoopEnv(t)
	e.crash(t)
	if err := e.reg.SetHold(crashComputer, "maintenance", "recovery"); err != nil {
		t.Fatalf("SetHold: %v", err)
	}
	out := e.ownerStart(t)
	if out.status != http.StatusOK {
		// A maintenance hold is answered as it always was (the computer stays
		// stopped); it is never a crash-loop refusal and never cleared here.
		t.Fatalf("owner start of a maintenance-held computer = %+v", out)
	}
	own := e.stored()
	if !own.IsHeld() || own.HoldStatus.HeldBy != "recovery" || own.HoldStatus.Reason != "maintenance" {
		t.Fatalf("maintenance hold after an owner start = %+v, want it untouched", own.HoldStatus)
	}
	if len(own.Crashes) != 0 {
		t.Fatalf("crash history after an owner start = %+v, want cleared", own.Crashes)
	}
	if got := e.recovers(); got != 0 {
		t.Fatalf("a maintenance-held computer was started: recoveries = %d", got)
	}
}

func TestUnholdOfACrashLoopHoldClearsTheHistory(t *testing.T) {
	e := newCrashLoopEnv(t)
	e.tripTheBound(t)
	if err := e.reg.ClearHold(crashComputer); err != nil {
		t.Fatalf("ClearHold: %v", err)
	}
	if own := e.stored(); own.IsHeld() || len(own.Crashes) != 0 {
		t.Fatalf("after unhold: held=%v crashes=%+v, want neither: the next crash would trip the bound at once", own.IsHeld(), own.Crashes)
	}
}

func TestPlannedRestartsAreNotCrashes(t *testing.T) {
	e := newCrashLoopEnv(t)
	key := ownershipKey(crashUser, PrimaryDesktopID)

	// A refresh in flight reads booting, however unhealthy the guest is.
	*e.healthy = false
	e.reg.mu.Lock()
	e.reg.lifecycle[key] = lifecycleRefresh
	e.reg.mu.Unlock()
	if got := e.lookup(t); got.State != string(VMStateBooting) {
		t.Fatalf("lookup during a refresh = %q, want booting", got.State)
	}
	e.reg.releaseLifecycle(key, lifecycleRefresh)
	if own := e.stored(); len(own.Crashes) != 0 || own.State != VMStateActive {
		t.Fatalf("a refresh in flight was recorded: state=%q crashes=%+v", own.State, own.Crashes)
	}

	// A completed deploy refresh.
	*e.healthy = true
	e.mgr.refreshResponse = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: 20, Healthy: true, State: "running"}
	if _, err := e.reg.RefreshVMForDesktop(crashUser, PrimaryDesktopID); err != nil {
		t.Fatalf("RefreshVMForDesktop: %v", err)
	}
	if own := e.stored(); len(own.Crashes) != 0 {
		t.Fatalf("a deploy refresh was recorded as a crash: %+v", own.Crashes)
	}

	// An owner stop, then an owner start.
	if err := e.reg.StopVMForDesktop(crashUser, PrimaryDesktopID); err != nil {
		t.Fatalf("StopVMForDesktop: %v", err)
	}
	if out := e.ownerStart(t); out.status != http.StatusOK {
		t.Fatalf("owner start after a stop = %+v", out)
	}
	if own := e.stored(); len(own.Crashes) != 0 || own.IsHeld() {
		t.Fatalf("an owner stop and start was recorded: held=%v crashes=%+v", own.IsHeld(), own.Crashes)
	}

	// A vmctl restart loads running computers as stopped.
	reloaded := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reloaded.SetPersistencePath(e.path); err != nil {
		t.Fatalf("reload ownership: %v", err)
	}
	if own := reloaded.GetOwnershipForDesktop(crashUser, PrimaryDesktopID); own == nil || len(own.Crashes) != 0 {
		t.Fatalf("a vmctl restart was recorded as a crash: %+v", own)
	}
}

func TestOneRealizationCrashesOnceHoweverOftenItIsSeenUnhealthy(t *testing.T) {
	e := newCrashLoopEnv(t)
	*e.healthy = false
	// Lookups that all read the computer while it was still active (the
	// snapshot a racing lookup holds) latch it once.
	stale := e.reg.GetOwnershipForDesktop(crashUser, PrimaryDesktopID)
	for i := 0; i < 16; i++ {
		e.reg.reconcileLookupReadiness(stale)
	}
	if own := e.stored(); len(own.Crashes) != 1 {
		t.Fatalf("racing lookups recorded %d crashes, want 1", len(own.Crashes))
	}

	// The guest heals (the lookup re-promotes the same realization), then
	// flaps again: same epoch, same crash.
	for i := 0; i < 4; i++ {
		*e.healthy = true
		if got := e.lookup(t); got.State != string(VMStateActive) {
			t.Fatalf("lookup of the healed guest = %q, want active", got.State)
		}
		*e.healthy = false
		e.mgr.mu.Lock()
		e.mgr.getVMs[crashVM].Healthy = false
		e.mgr.mu.Unlock()
		if got := e.lookup(t); got.State != string(VMStateDegraded) {
			t.Fatalf("lookup of the flapped guest = %q, want degraded", got.State)
		}
	}
	if own := e.stored(); len(own.Crashes) != 1 || own.IsHeld() {
		t.Fatalf("a flapping realization: crashes=%d held=%v, want one crash and no hold", len(own.Crashes), own.IsHeld())
	}
}

func TestCrashDuringBootCountsAndEndsTheLoop(t *testing.T) {
	e := newCrashLoopEnv(t)
	e.crash(t) // crash one; the restarts below fail
	e.mgr.mu.Lock()
	e.mgr.recoverError = errors.New("guest process exited before readiness")
	e.mgr.mu.Unlock()

	// Boot failure one is crash two. Boot failure two is crash three.
	for i := 0; i < 2; i++ {
		if out := e.resolveBody(t, `{"user_id":"`+crashUser+`"}`); out.status == http.StatusOK {
			t.Fatalf("a restart whose boot failed answered 200 on attempt %d", i+1)
		}
	}
	own := e.stored()
	if !own.IsHeld() || own.HoldStatus.HeldBy != CrashLoopHeldBy {
		t.Fatalf("after three crashes counting the failed boots: %+v, want the crash-loop hold", own.HoldStatus)
	}
	if got := e.recovers(); got != 2 {
		t.Fatalf("boot attempts = %d, want 2", got)
	}
	// No third boot attempt: the request is refused with the cause.
	out := e.resolveBody(t, `{"user_id":"`+crashUser+`"}`)
	if out.status != http.StatusServiceUnavailable || out.kind != string(RecoveryRefusalCrashLoop) {
		t.Fatalf("request after the failed boots = %+v, want the crash_loop refusal", out)
	}
	if got := e.recovers(); got != 2 {
		t.Fatalf("a held computer was booted again: attempts = %d", got)
	}
}

// Only the guest failing to come up is a crash. A refusal that spawned nothing
// and a host or control-plane failure (an outage of the credential service
// must not hold every computer restarted during it) are not.
func TestAFailedRestartThatIsNotTheGuestsIsNotACrash(t *testing.T) {
	failures := map[string]error{
		"memory pressure":             &RecoveryRefusal{Kind: RecoveryRefusalHostMemoryPressure, Reason: "no room", RetryAfterSeconds: 60},
		"credential unavailable":      errors.New("realization credential unavailable"),
		"firecracker would not start": errors.New("start firecracker: exec format error"),
	}
	for name, failure := range failures {
		t.Run(name, func(t *testing.T) {
			e := newCrashLoopEnv(t)
			e.crash(t)
			e.mgr.mu.Lock()
			e.mgr.recoverError = failure
			e.mgr.mu.Unlock()
			for i := 0; i < 2*crashLoopMaxCrashes; i++ {
				e.resolveBody(t, `{"user_id":"`+crashUser+`"}`)
			}
			own := e.stored()
			if own.IsHeld() || len(own.Crashes) != 1 {
				t.Fatalf("a failed restart that is not the guest's was counted: held=%v crashes=%+v, want only the original crash", own.IsHeld(), own.Crashes)
			}
			if got := e.recovers(); got != 2*crashLoopMaxCrashes {
				t.Fatalf("restart attempts = %d, want %d: nothing bounds a failure that is not a crash", got, 2*crashLoopMaxCrashes)
			}
		})
	}
}

func TestTwoWakePathsRacingBootTheCrashedComputerOnce(t *testing.T) {
	e := newCrashLoopEnv(t)
	e.crash(t)
	e.epoch++
	e.mgr.mu.Lock()
	e.mgr.recoverResponse = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: e.epoch, Healthy: true, State: "running"}
	e.mgr.recoverStarted = make(chan struct{})
	e.mgr.recoverRelease = make(chan struct{})
	started, release := e.mgr.recoverStarted, e.mgr.recoverRelease
	e.mgr.mu.Unlock()

	key := ownershipKey(crashUser, PrimaryDesktopID)
	results := make(chan error, 2)
	resolve := func() {
		_, err := e.reg.ResolveOrAssignDesktopContext(context.Background(), crashUser, PrimaryDesktopID)
		results <- err
	}
	go resolve()
	<-started
	go resolve()
	// The second wake joins the first one's start.
	deadline := time.Now().Add(5 * time.Second)
	for {
		e.reg.mu.RLock()
		waiting := len(e.reg.pendingWaiters[key])
		e.reg.mu.RUnlock()
		if waiting == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the second wake did not join the first (waiters=%d)", waiting)
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatalf("resolve %d: %v", i+1, err)
		}
	}
	if got := e.recovers(); got != 1 {
		t.Fatalf("a crashed computer was booted %d times by two racing wakes, want 1", got)
	}
	if own := e.stored(); own.State != VMStateActive || len(own.Crashes) != 1 {
		t.Fatalf("after the racing wakes: state=%q crashes=%d, want active and one crash", own.State, len(own.Crashes))
	}
}

func TestAutomaticRestartIsRefusedMidApplyOrWithALiveCapsule(t *testing.T) {
	two := 2
	zero := 0
	cases := []struct {
		name    string
		reading hostStopHealth
		refused bool
	}{
		{"dead guest", hostStopHealth{}, false},
		{"answers, idle", hostStopHealth{Answered: true, LiveCapsules: &zero}, false},
		{"mid self-development apply", hostStopHealth{Answered: true, SelfdevActive: 1, LiveCapsules: &zero}, true},
		{"live capsule", hostStopHealth{Answered: true, LiveCapsules: &two}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newCrashLoopEnv(t)
			e.reg.setHostStopHealthProbeForTest(func(*VMOwnership) hostStopHealth { return tc.reading })
			e.crash(t)
			out := e.wake(t)
			if tc.refused {
				if out.status != http.StatusServiceUnavailable || out.kind != string(RecoveryRefusalGuestWorking) {
					t.Fatalf("wake of a working guest = %+v, want the guest_working refusal", out)
				}
				if got := e.recovers(); got != 0 {
					t.Fatalf("a computer mid-work was restarted: recoveries = %d", got)
				}
				if own := e.stored(); own.IsHeld() || own.State != VMStateDegraded {
					t.Fatalf("the refusal changed the computer: held=%v state=%q", own.IsHeld(), own.State)
				}
				return
			}
			if out.status != http.StatusOK || e.recovers() != 1 {
				t.Fatalf("wake = %+v recoveries=%d, want a restart", out, e.recovers())
			}
		})
	}
}

func TestWedgeStopIsACrash(t *testing.T) {
	reg, mgr, now, refusing := wedgeRegistry(t)
	const url = "http://10.0.0.6:8085"
	addOwnership(reg, &VMOwnership{VMID: "vm-wedge", ComputerID: "computer-wedge", UserID: "someone", DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateActive, ComputerURL: url, Epoch: 3})
	mgr.live["vm-wedge"] = []int{5}
	mgr.getVMs["vm-wedge"] = &VMInstanceInfo{HostURL: url, State: "running"}
	refusing[url] = true

	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	*now = now.Add(wedgedAfterDefault)
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	own := reg.GetOwnershipByVMID("vm-wedge")
	if own == nil || own.StoppedBy != stoppedByWedged || len(own.Crashes) != 1 || own.Crashes[0].Kind != crashKindWedged {
		t.Fatalf("after a wedge stop = %+v, want one wedged crash recorded", own)
	}
}

func TestCrashHeldPlatformComputerAnswersWithTheRefusal(t *testing.T) {
	reg := NewOwnershipRegistry("")
	reg.SetVMManager(&mockVMManager{getVMs: map[string]*VMInstanceInfo{}})
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	reg.ownerships[key] = &VMOwnership{
		VMID: UniversalWirePlatformVMID, ComputerID: UniversalWirePlatformComputerID,
		UserID: UniversalWirePlatformOwnerID, DesktopID: UniversalWirePlatformDesktopID, Kind: VMKindInteractive,
		State:      VMStateStopped,
		HoldStatus: &MaintenanceHold{Reason: crashLoopReason(), HeldBy: CrashLoopHeldBy},
	}
	err := reg.EnsureUniversalWirePlatformComputer(context.Background())
	if refusal := RecoveryRefusalFrom(err); refusal == nil || refusal.Kind != RecoveryRefusalCrashLoop {
		t.Fatalf("ensure of a crash-held platform computer = %v, want the crash_loop refusal", err)
	}
}
