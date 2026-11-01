package vmctl

import (
	"context"
	"strings"
	"testing"
	"time"
)

// D6 S5, the periodic busy_cold pass (revision 5, owner goal 2026-11-01:
// "optimize memory usage for backup and the paper computer and in general
// without major cost in compute or storage"). Pressure alone leaves the
// paper at its peak until a start is already refused; the periodic pass asks
// each computer the busy-cold switch names, every interval.
//
// Ways it can fail, written first:
//   - the pass calls a computer the busy-cold switch does not name (the
//     paper by class, an idle-selector computer), or sends a write to an
//     =observe entry (it must be a dry run);
//   - the pass ignores the five-minute per-computer floor, so the pass and
//     the pressure path together squeeze one computer more often;
//   - VMCTL_GUEST_CACHE_RETURN=off, or an interval of 0, still runs it;
//   - the loop does not tick at its interval;
//   - a refault_stop is retried at the next interval, so a computer whose
//     working set came back is asked again and again (it must back off for
//     three intervals);
//   - the pass blocks on a guest (acts run in the background, bounded by
//     the in-flight limit).

func periodicCR(t *testing.T, entries map[string]string) *stepZeroFixture {
	t.Helper()
	f := busyColdCR(t, entries)
	c := f.reg.guestCacheReturnConfig()
	c.BusyColdInterval = 10 * time.Minute
	f.reg.SetGuestCacheReturnConfig(c)
	return f
}

func (f *stepZeroFixture) busyColdPass(t *testing.T) int {
	t.Helper()
	n := f.reg.runBusyColdPass(context.Background())
	if !f.reg.waitGuestCacheReturn(5 * time.Second) {
		t.Fatal("periodic acts did not finish")
	}
	return n
}

// Only the named computers are asked: =active with a write, =observe as a
// dry run; the unnamed paper-class computer and the idle-selector computer
// are not.
func TestBusyColdPeriodicOnlyCallsNamedComputers(t *testing.T) {
	f := periodicCR(t, map[string]string{paperID: GuestCacheReturnObserve, "computer-vm-de": GuestCacheReturnActive})
	paperG, deG, otherG, idleG := newMRGuest(t, "paper", nil), newMRGuest(t, "de", nil), newMRGuest(t, "other", nil), newMRGuest(t, "idle", nil)
	f.paper(paperG)
	f.computer("vm-de", 0, 3*gib, 1*gib, deG)
	other := f.computer("vm-other", 0, 7*gib, 1*gib, otherG)
	f.reg.mu.Lock()
	other.WarmnessClass = WarmnessClassCriticalProtected
	f.reg.mu.Unlock()
	f.computer("vm-idle", 40*time.Minute, 3*gib, 1*gib, idleG) // named only by _ONLY-style class rules: not here
	if n := f.busyColdPass(t); n != 2 {
		t.Fatalf("pass started %d acts, want 2", n)
	}
	if paperG.calls.Load() != 1 || !strings.Contains(paperG.bodies[0], `"dry_run":true`) || !strings.Contains(paperG.bodies[0], `"mode":"busy_cold"`) {
		t.Fatalf("paper (=observe): calls %d bodies %v, want one busy_cold dry run", paperG.calls.Load(), paperG.bodies)
	}
	if deG.calls.Load() != 1 || !strings.Contains(deG.bodies[0], `"dry_run":false`) || !strings.Contains(deG.bodies[0], `"mode":"busy_cold"`) {
		t.Fatalf("d@e (=active): calls %d bodies %v, want one busy_cold write", deG.calls.Load(), deG.bodies)
	}
	if otherG.calls.Load()+idleG.calls.Load() != 0 {
		t.Fatalf("unnamed computers asked: other=%d idle=%d", otherG.calls.Load(), idleG.calls.Load())
	}
	recs := f.reg.GuestCacheReturnSummary().Recent
	for _, r := range recs {
		if r.Trigger != "periodic" || r.Mode != "busy_cold" {
			t.Fatalf("record %+v, want trigger periodic mode busy_cold", r)
		}
	}
}

// The five-minute floor holds across passes (and across the pressure path).
func TestBusyColdPeriodicHonorsTheFloor(t *testing.T) {
	f := periodicCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	g.report = claim(10 * mib)
	f.paper(g)
	f.busyColdPass(t)
	*f.now = f.now.Add(4 * time.Minute)
	f.busyColdPass(t)
	if g.calls.Load() != 1 {
		t.Fatalf("asked %d times within five minutes, want 1", g.calls.Load())
	}
	f.shortBy(5 * gib)
	f.stepZero() // the pressure path shares the floor
	if g.calls.Load() != 1 {
		t.Fatalf("pressure asked inside the floor: %d calls", g.calls.Load())
	}
	*f.now = f.now.Add(2 * time.Minute)
	f.busyColdPass(t)
	if g.calls.Load() != 2 {
		t.Fatalf("asked %d times after the floor, want 2", g.calls.Load())
	}
}

// Off, or an interval of 0, runs nothing; the loop reads the switch every
// tick and is not needed at all when off.
func TestBusyColdPeriodicNeverRunsWhenOff(t *testing.T) {
	f := periodicCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	f.paper(g)
	c := f.reg.guestCacheReturnConfig()
	c.Mode = GuestCacheReturnOff
	f.reg.SetGuestCacheReturnConfig(c)
	if c.NeedsBusyColdLoop() {
		t.Fatal("NeedsBusyColdLoop with the switch off")
	}
	if n := f.busyColdPass(t); n != 0 || g.calls.Load() != 0 {
		t.Fatalf("off: %d acts, %d calls, want none", n, g.calls.Load())
	}
	c.Mode = GuestCacheReturnActive
	c.BusyColdInterval = 0
	f.reg.SetGuestCacheReturnConfig(c)
	if c.NeedsBusyColdLoop() {
		t.Fatal("NeedsBusyColdLoop with interval 0")
	}
	if n := f.busyColdPass(t); n != 0 || g.calls.Load() != 0 {
		t.Fatalf("interval 0: %d acts, %d calls, want none", n, g.calls.Load())
	}
	c.BusyColdInterval = 10 * time.Minute
	if !c.NeedsBusyColdLoop() {
		t.Fatal("NeedsBusyColdLoop false for active, an entry and a 10m interval")
	}
}

// The loop ticks at its interval.
func TestBusyColdPeriodicLoopTicks(t *testing.T) {
	f := periodicCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	f.paper(g)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.reg.StartGuestCacheReturnBusyColdLoop(ctx, 20*time.Millisecond)
	deadline := time.Now().Add(5 * time.Second)
	for g.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	f.reg.waitGuestCacheReturn(5 * time.Second)
	if g.calls.Load() == 0 {
		t.Fatal("the loop never asked the paper")
	}
}

// A refault_stop backs the computer off for three intervals.
func TestBusyColdPeriodicBacksOffAfterRefaultStop(t *testing.T) {
	f := periodicCR(t, map[string]string{paperID: GuestCacheReturnActive})
	g := newMRGuest(t, "paper", nil)
	g.report = `{"outcome":"refault_stop","method":"memory_reclaim","target_bytes":1073741824,"detail":"refaults 40.0/s over the limit 32/s"}`
	f.paper(g)
	f.busyColdPass(t)
	for _, step := range []time.Duration{10 * time.Minute, 10 * time.Minute, 9 * time.Minute} {
		*f.now = f.now.Add(step)
		f.busyColdPass(t)
	}
	if g.calls.Load() != 1 {
		t.Fatalf("asked %d times inside three intervals after refault_stop, want 1", g.calls.Load())
	}
	// The pressure path honors the back-off too.
	f.shortBy(5 * gib)
	f.stepZero()
	if g.calls.Load() != 1 {
		t.Fatalf("pressure asked a backed-off computer: %d calls", g.calls.Load())
	}
	*f.now = f.now.Add(2 * time.Minute) // 31 minutes after the stop
	g.report = claim(10 * mib)
	f.busyColdPass(t)
	if g.calls.Load() != 2 {
		t.Fatalf("asked %d times after the back-off, want 2", g.calls.Load())
	}
}

// The interval parses: default 10m, 0 disables, clamped to 5m..1h,
// malformed is the default.
func TestBusyColdIntervalParse(t *testing.T) {
	for raw, want := range map[string]time.Duration{
		"": 10 * time.Minute, "0": 0, "0s": 0, "15m": 15 * time.Minute, "1m": 5 * time.Minute, "3h": time.Hour, "soon": 10 * time.Minute,
	} {
		if got := GuestCacheReturnConfigFromValues(GuestCacheReturnValues{Mode: "active", BusyColdInterval: raw}).BusyColdInterval; got != want {
			t.Errorf("interval %q = %s, want %s", raw, got, want)
		}
	}
}
