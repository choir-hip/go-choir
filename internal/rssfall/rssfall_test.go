package rssfall

import (
	"testing"
	"time"
)

// D6 M22 (docs/problems/density-station-d-failure-modes-2026-10-26.md D6;
// review 320 F1). A guest that returned page cache shows the return in the
// Firecracker process's RSS a while later: free page reporting runs on a 2
// second delay. A fenced or idle guest's RSS is already flat, so a wait for
// "two close readings" would end before the return began.
//
// Ways it could fail:
//   - the wait exits on stability alone, before the fall;
//   - the wait exits on the fall while the readings are still moving;
//   - the wait outlasts its cap, or has none;
//   - a guest that reclaimed too little to see is waited for;
//   - a VM that is gone is waited for until the cap.

const mib = uint64(1 << 20)

func TestWaitDoesNotExitBeforeTheFallAndExitsAfterIt(t *testing.T) {
	polls := 0
	read := func() (uint64, bool) {
		polls++
		if polls <= 12 { // flat for 3 s
			return 4000 * mib, true
		}
		return 3000 * mib, true
	}
	var slept time.Duration
	got := Wait(read, func(d time.Duration) { slept += d }, 4000*mib, 1500*mib, 5*time.Second)
	if got.Exit != ExitFell || got.RSS != 3000*mib {
		t.Fatalf("result = %+v, want fell at 3000 MiB", got)
	}
	if slept < 3*time.Second || slept > 4*time.Second || got.MS != slept.Milliseconds() {
		t.Fatalf("waited %s (reported %d ms), want about 3.5 s", slept, got.MS)
	}
}

func TestWaitStopsAtTheCapWhenNothingFalls(t *testing.T) {
	var slept time.Duration
	got := Wait(func() (uint64, bool) { return 4000 * mib, true }, func(d time.Duration) { slept += d },
		4000*mib, 1500*mib, 5*time.Second)
	if got.Exit != ExitCap || slept != 5*time.Second || got.MS != 5000 || got.RSS != 4000*mib {
		t.Fatalf("result = %+v after %s, want cap at 5 s", got, slept)
	}
	// A shorter cap (the pressure step's remaining time) is honoured.
	slept = 0
	got = Wait(func() (uint64, bool) { return 4000 * mib, true }, func(d time.Duration) { slept += d },
		4000*mib, 1500*mib, 1*time.Second)
	if got.Exit != ExitCap || slept != time.Second {
		t.Fatalf("short cap: %+v after %s", got, slept)
	}
}

func TestWaitDoesNotWaitForTooLittle(t *testing.T) {
	var slept time.Duration
	got := Wait(func() (uint64, bool) { return 4000 * mib, true }, func(d time.Duration) { slept += d },
		4000*mib, 100*mib, 5*time.Second)
	if got.Exit != ExitNoWait || slept != 0 || got.MS != 0 {
		t.Fatalf("result = %+v after %s, want no_wait", got, slept)
	}
}

func TestWaitEndsWhenTheVMIsGone(t *testing.T) {
	got := Wait(func() (uint64, bool) { return 0, false }, func(time.Duration) {}, 4000*mib, 1500*mib, 5*time.Second)
	if got.Exit != ExitGone {
		t.Fatalf("result = %+v, want gone", got)
	}
}

func TestWaitWaitsForAFallingReadingToSettle(t *testing.T) {
	vals := []uint64{3900, 3500, 3100, 3000, 3000}
	i := 0
	got := Wait(func() (uint64, bool) {
		v := vals[min(i, len(vals)-1)]
		i++
		return v * mib, true
	}, func(time.Duration) {}, 4000*mib, 1500*mib, 5*time.Second)
	if got.Exit != ExitFell || got.RSS != 3000*mib {
		t.Fatalf("result = %+v, want fell at 3000 MiB only once two readings agree", got)
	}
}

// A fall smaller than half of what the guest claims is not enough: the
// guest's claim sizes the wait, the host's reading decides the exit.
func TestWaitIsNotSatisfiedByASmallFall(t *testing.T) {
	got := Wait(func() (uint64, bool) { return 3800 * mib, true }, func(time.Duration) {},
		4000*mib, 1500*mib, 5*time.Second)
	if got.Exit != ExitCap {
		t.Fatalf("result = %+v, want cap: a 200 MiB fall for a 1500 MiB claim is not half", got)
	}
}
