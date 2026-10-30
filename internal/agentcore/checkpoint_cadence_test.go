package agentcore

// Automatic checkpoints (slice 3 of
// docs/archive/design/apply-checkpoint-without-replay-2026-10-26.md), written before
// the code. The owner: "We shouldn't have to bind checkpoints manually. It
// shouldn't be a cli command but done automatically after n events or time."
// Ways it could fail:
//
//  1. the cadence binds an empty checkpoint (no event since the last), or
//     does not bind when 2,500 events or 60 minutes have passed;
//  2. the defaults or the env overrides are wrong, or an unparsable override
//     turns the cadence off silently instead of keeping the default;
//  3. a bind starts at boot, or in the boot window, or while an apply is
//     materializing: resumed work (CLAUDE.md, restarts end work);
//  4. a tick that crashed the computer twice is tried a third time (a crash
//     loop), or a tick that merely failed counts as a crash;
//  5. a failing platform is hit every tick (no backoff), or a slow bind is
//     repeated so often that it stalls the computer (no duty-cycle bound);
//  6. a runtime that cannot keep a durable attempt count binds anyway;
//  7. the cadence consults the platform every minute for a computer that has
//     made no event;
//  8. a closed cadence cannot be reopened by the owner act, or reopens itself;
//  9. the cadence never stops with the runtime.

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Review 329 (FP0): the cadence compiles OFF. The host selects the computers
// that run it, with the values (vmctl's VM_CHECKPOINT_CADENCE_ONLY), so the
// first computer is chosen and its hold measured before the cadence widens.
func TestCheckpointCadenceConfig(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	for _, tc := range []struct {
		name string
		env  map[string]string
		want checkpointCadenceConfig
	}{
		{"compiled default is off", nil, checkpointCadenceConfig{}},
		{"the host's selection", map[string]string{"CHOIR_CHECKPOINT_EVERY_EVENTS": "2500", "CHOIR_CHECKPOINT_EVERY": "60m"}, checkpointCadenceConfig{EveryEvents: 2500, Every: 60 * time.Minute}},
		{"events only", map[string]string{"CHOIR_CHECKPOINT_EVERY_EVENTS": "100"}, checkpointCadenceConfig{EveryEvents: 100}},
		{"time only", map[string]string{"CHOIR_CHECKPOINT_EVERY": "10m"}, checkpointCadenceConfig{Every: 10 * time.Minute}},
		{"zero turns a trigger off", map[string]string{"CHOIR_CHECKPOINT_EVERY_EVENTS": "0", "CHOIR_CHECKPOINT_EVERY": "0"}, checkpointCadenceConfig{}},
		{"garbage stays off", map[string]string{"CHOIR_CHECKPOINT_EVERY_EVENTS": "many", "CHOIR_CHECKPOINT_EVERY": "soon"}, checkpointCadenceConfig{}},
		{"negative stays off", map[string]string{"CHOIR_CHECKPOINT_EVERY_EVENTS": "-5", "CHOIR_CHECKPOINT_EVERY": "-1m"}, checkpointCadenceConfig{}},
		{"one good value is kept beside a bad one", map[string]string{"CHOIR_CHECKPOINT_EVERY_EVENTS": "2500", "CHOIR_CHECKPOINT_EVERY": "soon"}, checkpointCadenceConfig{EveryEvents: 2500}},
	} {
		if got := checkpointCadenceConfigFromEnv(env(tc.env)); got != tc.want {
			t.Fatalf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// An unselected computer's runtime, reading no selection, never binds and says
// so.
func TestCheckpointCadenceIsDisabledWhenTheHostSelectedNothing(t *testing.T) {
	t.Setenv("CHOIR_CHECKPOINT_EVERY_EVENTS", "")
	t.Setenv("CHOIR_CHECKPOINT_EVERY", "")
	cf := newCadenceFixture(t, "computer-cadence-unselected")
	cf.rt.cadence.cfg = nil // read the environment, as a booted runtime does
	cf.events(t, 5, "busy")
	if out, _ := cf.tick(t, time.Now().UTC().Add(100*time.Hour)); out != cadenceSkipped || cf.binds.Load() != 0 {
		t.Fatalf("an unselected computer: %s binds=%d", out, cf.binds.Load())
	}
	if status := cf.rt.CheckpointCadenceStatus(); status.State != "disabled" {
		t.Fatalf("status = %+v, want disabled", status)
	}
}

// Review 329 (DeepSeek 8): an attempt file that cannot be read means a crashing
// bind could not be bounded. The cadence closes with that cause instead of
// reading "on" while skipping every tick.
func TestCheckpointCadenceClosesWhenTheAttemptFileCannotBeRead(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-corrupt-attempts")
	cf.events(t, 5, "busy")
	if err := os.WriteFile(GuestApplyAttemptsPath(cf.rt.cfg.StorePath), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _ := cf.tick(t, time.Now().UTC())
	if out != cadenceOff || cf.binds.Load() != 0 {
		t.Fatalf("a corrupt attempt file: %s binds=%d", out, cf.binds.Load())
	}
	status := cf.rt.CheckpointCadenceStatus()
	if status.State != "off" || !strings.Contains(status.Cause, "attempt") {
		t.Fatalf("status = %+v, want off naming the attempt file", status)
	}
}

func TestCheckpointCadenceDue(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	cfg := checkpointCadenceConfig{EveryEvents: 2500, Every: 60 * time.Minute}
	last := checkpointMark{Sequence: 10_000, At: now.Add(-10 * time.Minute)}
	for _, tc := range []struct {
		name string
		cfg  checkpointCadenceConfig
		head uint64
		last checkpointMark
		want bool
	}{
		{"nothing new", cfg, 10_000, last, false},
		{"one event, recent", cfg, 10_001, last, false},
		{"2,499 events", cfg, 12_499, last, false},
		{"2,500 events", cfg, 12_500, last, true},
		{"an hour and one event", cfg, 10_001, checkpointMark{Sequence: 10_000, At: now.Add(-61 * time.Minute)}, true},
		{"an hour and no event: no empty checkpoint", cfg, 10_000, checkpointMark{Sequence: 10_000, At: now.Add(-5 * time.Hour)}, false},
		{"head behind the mark", cfg, 9_000, checkpointMark{Sequence: 10_000, At: now.Add(-5 * time.Hour)}, false},
		{"event trigger off", checkpointCadenceConfig{Every: 60 * time.Minute}, 50_000, last, false},
		{"time trigger off", checkpointCadenceConfig{EveryEvents: 2500}, 10_001, checkpointMark{Sequence: 10_000, At: now.Add(-9 * time.Hour)}, false},
		{"both off", checkpointCadenceConfig{}, 99_999, checkpointMark{Sequence: 10_000, At: now.Add(-9 * time.Hour)}, false},
	} {
		got, why := tc.cfg.due(tc.head, tc.last, now)
		if got != tc.want || (got && why == "") {
			t.Fatalf("%s: due=%v (%q), want %v", tc.name, got, why, tc.want)
		}
	}
}

// cadenceFixture is a computer with a head, a fake platform view of its
// newest checkpoint, and a counting bind.
type cadenceFixture struct {
	*checkpointHeadFixture
	mark       checkpointMark
	markFound  bool
	markErr    error
	markReads  atomic.Int32
	binds      atomic.Int32
	bindErr    error
	bindTook   time.Duration
	bindAtHead bool
}

func newCadenceFixture(t *testing.T, computerID string) *cadenceFixture {
	t.Helper()
	cf := &cadenceFixture{checkpointHeadFixture: newCheckpointHeadFixture(t, computerID), markFound: true}
	cf.mark = checkpointMark{Sequence: 1, At: time.Now().UTC()}
	rt := cf.rt
	cfg := checkpointCadenceConfig{EveryEvents: 3, Every: time.Hour}
	rt.cadence.cfg = &cfg
	rt.cadence.last = func(context.Context) (checkpointMark, bool, error) {
		cf.markReads.Add(1)
		return cf.mark, cf.markFound, cf.markErr
	}
	rt.cadence.bind = func(ctx context.Context) (uint64, error) {
		cf.binds.Add(1)
		if cf.bindErr != nil {
			return 0, cf.bindErr
		}
		head, err := rt.store.Head(ctx, computerID)
		if err != nil {
			return 0, err
		}
		cf.mark = checkpointMark{Sequence: head.Sequence, At: time.Now().UTC()}
		return head.Sequence, nil
	}
	return cf
}

func (cf *cadenceFixture) events(t *testing.T, n int, prefix string) {
	t.Helper()
	for i := 0; i < n; i++ {
		cf.appendDeskEvent(t, prefix+"-"+string(rune('a'+i)))
	}
}

func (cf *cadenceFixture) tick(t *testing.T, now time.Time) (cadenceOutcome, error) {
	t.Helper()
	return cf.rt.checkpointCadenceTick(context.Background(), now)
}

// Failure mode 1.
func TestCheckpointCadenceBindsOnEventsAndNeverEmpty(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-events")
	now := time.Now().UTC()
	if out, err := cf.tick(t, now); out != cadenceIdle || err != nil {
		t.Fatalf("no new event: %s %v", out, err)
	}
	cf.events(t, 2, "few") // head 3, mark 1: 2 events, under 3
	if out, _ := cf.tick(t, now); out != cadenceIdle || cf.binds.Load() != 0 {
		t.Fatalf("2 events under the threshold bound: %s", out)
	}
	cf.events(t, 1, "enough") // 3 events since the mark
	if out, err := cf.tick(t, now); out != cadenceBound || err != nil || cf.binds.Load() != 1 {
		t.Fatalf("3 events: %s %v binds=%d", out, err, cf.binds.Load())
	}
	// Nothing since the bind: an hour on, still no empty checkpoint.
	if out, _ := cf.tick(t, now.Add(3*time.Hour)); out != cadenceIdle || cf.binds.Load() != 1 {
		t.Fatalf("an empty checkpoint was bound: %s binds=%d", out, cf.binds.Load())
	}
}

func TestCheckpointCadenceBindsOnTime(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-time")
	cf.events(t, 1, "one") // 1 event since the mark: under the event bound
	now := time.Now().UTC()
	if out, _ := cf.tick(t, now.Add(30*time.Minute)); out != cadenceIdle {
		t.Fatalf("30 minutes: %s", out)
	}
	if out, err := cf.tick(t, now.Add(61*time.Minute)); out != cadenceBound || err != nil {
		t.Fatalf("61 minutes with one event: %s %v", out, err)
	}
}

// Failure mode 7: a tick that cannot be due asks the platform nothing.
func TestCheckpointCadenceDoesNotAskThePlatformWhenNothingChanged(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-quiet")
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if out, _ := cf.tick(t, now.Add(time.Duration(i)*time.Minute)); out != cadenceIdle {
			t.Fatalf("tick %d: %s", i, out)
		}
	}
	if reads := cf.markReads.Load(); reads > 1 {
		t.Fatalf("the platform was asked %d times about a computer with no new event", reads)
	}
}

func TestCheckpointCadenceSkipsWhenThePlatformCannotAnswer(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-platform-down")
	cf.events(t, 5, "busy")
	cf.markErr = errors.New("corpusd unreachable")
	out, err := cf.tick(t, time.Now().UTC())
	if out != cadenceSkipped || err == nil || cf.binds.Load() != 0 {
		t.Fatalf("a platform that cannot answer: %s %v binds=%d", out, err, cf.binds.Load())
	}
}

// Failure mode 3.
func TestCheckpointCadenceNeverBindsInTheBootWindowOrDuringAnApply(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-boot")
	cf.events(t, 5, "busy")
	cf.rt.bootInProgress.Store(true)
	if out, _ := cf.tick(t, time.Now().UTC()); out != cadenceSkipped || cf.binds.Load() != 0 {
		t.Fatalf("a bind started while Start was running: %s", out)
	}
	cf.rt.bootInProgress.Store(false)
	t.Setenv("RUNTIME_MAINTENANCE_HOLD", "1")
	if out, _ := cf.tick(t, time.Now().UTC()); out != cadenceSkipped || cf.binds.Load() != 0 {
		t.Fatalf("a bind started under the maintenance hold: %s", out)
	}
}

func TestCheckpointCadenceLoopWaitsOutTheInitialDelayThenBinds(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-loop")
	cf.events(t, 5, "busy")
	cf.rt.cadence.initialDelay = time.Hour
	cf.rt.cadence.tickEvery = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cf.rt.startCheckpointCadence(ctx)
	time.Sleep(150 * time.Millisecond)
	if cf.binds.Load() != 0 {
		t.Fatal("the cadence bound inside its initial delay: a restart resumed work")
	}
	cf.rt.stopCheckpointCadence()

	cf2 := newCadenceFixture(t, "computer-cadence-loop-go")
	cf2.events(t, 5, "busy")
	cf2.rt.cadence.initialDelay = 0
	cf2.rt.cadence.tickEvery = 5 * time.Millisecond
	cf2.rt.startCheckpointCadence(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for cf2.binds.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if cf2.binds.Load() == 0 {
		t.Fatal("the cadence loop never bound")
	}
	// Failure mode 9: it stops, and stays stopped.
	cf2.rt.stopCheckpointCadence()
	settled := cf2.binds.Load()
	cf2.events(t, 5, "later")
	cf2.mark = checkpointMark{Sequence: 1, At: time.Now().UTC().Add(-3 * time.Hour)}
	time.Sleep(100 * time.Millisecond)
	if cf2.binds.Load() != settled {
		t.Fatal("the cadence kept binding after it was stopped")
	}
}

// Failure mode 4: the schedule rule of CLAUDE.md for a cadence.
func TestCheckpointCadenceCrashedTwiceTurnsItOffUntilTheOwnerResumes(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-crash")
	cf.events(t, 5, "busy")
	storePath := cf.rt.cfg.StorePath
	computerID := cf.rt.cfg.ComputerID
	// Two ticks that started and never finished: the computer crashed in each.
	for i := 0; i < checkpointCadenceAttemptLimit; i++ {
		if err := spendSettlementAttempt(storePath, "checkpoint_cadence", computerID, checkpointCadenceAttemptLimit); err != nil {
			t.Fatalf("simulated crash %d: %v", i+1, err)
		}
	}
	out, _ := cf.tick(t, time.Now().UTC())
	if out != cadenceOff || cf.binds.Load() != 0 {
		t.Fatalf("a third start after two crashes: %s binds=%d", out, cf.binds.Load())
	}
	status := cf.rt.CheckpointCadenceStatus()
	if status.State != "off" || !strings.Contains(status.Cause, "did not finish") {
		t.Fatalf("status = %+v, want off with the cause", status)
	}
	// It does not reopen itself.
	if out, _ := cf.tick(t, time.Now().UTC().Add(2*time.Hour)); out != cadenceOff {
		t.Fatalf("the closed cadence ran again on its own: %s", out)
	}
	// The owner act reopens it.
	if err := cf.rt.resumeCheckpointCadence(); err != nil {
		t.Fatal(err)
	}
	if status := cf.rt.CheckpointCadenceStatus(); status.State != "on" {
		t.Fatalf("status after resume = %+v", status)
	}
	if out, err := cf.tick(t, time.Now().UTC().Add(3*time.Hour)); out != cadenceBound || err != nil {
		t.Fatalf("after the owner act: %s %v", out, err)
	}
}

// A tick that returned (success or error) is not a crash.
func TestCheckpointCadenceFinishedTicksDoNotCountAsCrashes(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-finished")
	now := time.Now().UTC()
	for i := 0; i < 4; i++ {
		cf.events(t, 3, "round"+string(rune('a'+i)))
		if i == 1 {
			cf.bindErr = errors.New("platform refused")
		} else {
			cf.bindErr = nil
		}
		out, _ := cf.tick(t, now.Add(time.Duration(i)*24*time.Hour))
		if want := cadenceBound; i != 1 && out != want {
			t.Fatalf("tick %d = %s, want %s: a finished tick was counted as a crash", i, out, want)
		}
		if i == 1 && out != cadenceFailed {
			t.Fatalf("tick %d = %s, want failed", i, out)
		}
	}
	if cf.rt.CheckpointCadenceStatus().State != "on" {
		t.Fatalf("status = %+v", cf.rt.CheckpointCadenceStatus())
	}
}

// Failure mode 5.
func TestCheckpointCadenceBacksOffAFailingBindAndBoundsTheDutyCycle(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-backoff")
	cf.events(t, 5, "busy")
	cf.bindErr = errors.New("platform refused")
	now := time.Now().UTC()
	if out, _ := cf.tick(t, now); out != cadenceFailed {
		t.Fatalf("first: %s", out)
	}
	if out, _ := cf.tick(t, now.Add(time.Minute)); out != cadenceBackoff || cf.binds.Load() != 1 {
		t.Fatalf("a failing bind was retried at once: %s binds=%d", out, cf.binds.Load())
	}
	if out, _ := cf.tick(t, now.Add(6*time.Minute)); out != cadenceFailed || cf.binds.Load() != 2 {
		t.Fatalf("after the first backoff (5m): %s binds=%d", out, cf.binds.Load())
	}
	// The second backoff is longer than the first.
	if out, _ := cf.tick(t, now.Add(6*time.Minute+6*time.Minute)); out != cadenceBackoff {
		t.Fatalf("the second backoff was not longer than the first: %s", out)
	}

	// A slow bind spaces the next one out: at most about 5% of the time held.
	cf2 := newCadenceFixture(t, "computer-cadence-duty")
	cf2.events(t, 5, "busy")
	cf2.rt.cadence.measure = func() time.Duration { return 30 * time.Second }
	if out, _ := cf2.tick(t, now); out != cadenceBound {
		t.Fatalf("duty: first %s", out)
	}
	cf2.events(t, 5, "more")
	if out, _ := cf2.tick(t, now.Add(5*time.Minute)); out != cadenceBackoff {
		t.Fatalf("a 30 s bind was repeated within 5 minutes (over 10%% of the time held): %s", out)
	}
	if out, _ := cf2.tick(t, now.Add(11*time.Minute)); out != cadenceBound {
		t.Fatalf("a 30 s bind after 11 minutes (20x): %s", out)
	}
	if got := cf2.rt.CheckpointCadenceStatus().LastBindMillis; got != 30_000 {
		t.Fatalf("status reports the last bind as %d ms, want the measured 30000", got)
	}
}

// Failure mode 6.
func TestCheckpointCadenceRefusesWithoutADurableAttemptCount(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-unbounded")
	cf.events(t, 5, "busy")
	cf.rt.cfg.StorePath = ""
	out, _ := cf.tick(t, time.Now().UTC())
	if out == cadenceBound || cf.binds.Load() != 0 {
		t.Fatalf("a cadence with no durable attempt count bound: %s", out)
	}
	if status := cf.rt.CheckpointCadenceStatus(); status.State != "off" || status.Cause == "" {
		t.Fatalf("status = %+v, want off with a cause", status)
	}
}

func TestCheckpointCadenceDisabledByConfigNeverBinds(t *testing.T) {
	cf := newCadenceFixture(t, "computer-cadence-disabled")
	cf.events(t, 5, "busy")
	cfg := checkpointCadenceConfig{}
	cf.rt.cadence.cfg = &cfg
	if out, _ := cf.tick(t, time.Now().UTC().Add(100*time.Hour)); out != cadenceSkipped || cf.binds.Load() != 0 {
		t.Fatalf("a disabled cadence: %s", out)
	}
	if status := cf.rt.CheckpointCadenceStatus(); status.State != "disabled" {
		t.Fatalf("status = %+v", status)
	}
}
