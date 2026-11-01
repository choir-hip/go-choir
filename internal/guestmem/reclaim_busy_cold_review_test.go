package guestmem

import (
	"context"
	"testing"
	"time"
)

// Review of S5 (Devin #2): the refault guard must see refaults an earlier
// act caused. The ring alone covers only the last 60 s, so a burst re-read
// in the seconds after the previous act rolls into the baseline and the next
// check reads flat. With a previous act's end marked within 30 minutes, the
// before-rate is over the whole window since that act ended; the 60 s
// minimum and the no-baseline refusal after boot stay.

func TestBusyColdGuardSeesThePreviousActsRefaults(t *testing.T) {
	f := newReclaimFixture(t)
	f.setScopeStat(f.daemon, memoryStat(6000, 5000, 100, 0, 0, 0))
	now := time.Now()
	w := busyColdWindow(f, now, 60*time.Second, 120000) // the last minute: flat
	w.MarkActEnd(now.Add(-5*time.Minute), 1000)         // 119,000 refaults since the last act: ~397/s
	f.setRefaults(120000)
	got, err := f.reclaimer().Reclaim(context.Background(), busyColdRequest(w))
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != OutcomeRefaultStop || f.writeCount() != 0 {
		t.Fatalf("outcome %q with %d writes, want refault_stop: the previous act's evictions were re-read", got.Outcome, f.writeCount())
	}
	// A mark older than 30 minutes is not used: the ring's minute decides.
	w2 := busyColdWindow(f, now, 60*time.Second, 120000)
	w2.MarkActEnd(now.Add(-31*time.Minute), 1000)
	got, _ = f.reclaimer().Reclaim(context.Background(), busyColdRequest(w2))
	if got.Outcome != OutcomeReturned {
		t.Fatalf("stale mark: outcome %q, want returned", got.Outcome)
	}
}

// The window stays at least 60 s: a mark younger than that does not shorten
// it, and after boot (no ring sample) a mark alone is not a baseline.
func TestBusyColdGuardKeepsTheMinuteMinimum(t *testing.T) {
	now := time.Now()
	w := &RefaultWindow{Now: func() time.Time { return now }}
	w.Observe(now.Add(-70*time.Second), 0)
	w.MarkActEnd(now.Add(-10*time.Second), 690)
	if rate, ok := w.Rate(700); !ok || rate != 10 {
		t.Fatalf("young mark: rate %v ok %v, want 10/s over the ring's 70 s", rate, ok)
	}
	boot := &RefaultWindow{Now: func() time.Time { return now }}
	boot.MarkActEnd(now.Add(-5*time.Minute), 0)
	if _, ok := boot.Rate(10); ok {
		t.Fatal("a mark with no ring sample was taken as a baseline")
	}
}

// A completed busy_cold act marks its end, so the next act's guard covers
// it; a dry run marks nothing.
func TestBusyColdActMarksItsEnd(t *testing.T) {
	f := newReclaimFixture(t)
	f.setScopeStat(f.daemon, memoryStat(6000, 5000, 100, 0, 0, 0))
	f.setRefaults(80)
	now := time.Now()
	w := busyColdWindow(f, now, 60*time.Second, 20)
	dry := busyColdRequest(w)
	dry.DryRun = true
	if _, err := f.reclaimer().Reclaim(context.Background(), dry); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := w.ActEnd(); ok {
		t.Fatal("a dry run marked an act end")
	}
	if got, _ := f.reclaimer().Reclaim(context.Background(), busyColdRequest(w)); got.Outcome != OutcomeReturned {
		t.Fatalf("outcome %q", got.Outcome)
	}
	at, count, ok := w.ActEnd()
	if !ok || count != 80 || !at.Equal(now) {
		t.Fatalf("act end = %v %d %v, want now and 80", at, count, ok)
	}
}

// The post-act reading: the rate from the act's end to a reading taken
// later (the guest logs it as refault_rate_post).
func TestRefaultWindowPostRate(t *testing.T) {
	now := time.Now()
	w := &RefaultWindow{Now: func() time.Time { return now }}
	if _, ok := w.PostRate(100); ok {
		t.Fatal("a post rate with no act end")
	}
	w.MarkActEnd(now.Add(-60*time.Second), 100)
	if rate, ok := w.PostRate(700); !ok || rate != 10 {
		t.Fatalf("post rate %v ok %v, want 10/s", rate, ok)
	}
}
