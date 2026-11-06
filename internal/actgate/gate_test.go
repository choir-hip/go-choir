package actgate

// Failure modes this file pins (written before the gate):
//  1. an I0 or I1 act is gated (the gate must not touch non-material acts);
//  2. a material act passes with no trust view, a stale one, or one from the
//     future (thin evidence must not buy autonomy);
//  3. a material act passes for a desk with no record in its class, or with a
//     record in another class only (trust does not transfer across classes);
//  4. a bound below tau passes, or one at tau is gated;
//  5. an open breaker row passes;
//  6. a trip does not gate at once, or does not call the lease hook;
//  7. a trip is forgotten before a view shows the event, or never clears
//     once a view shows it and later positive evidence closed the breaker;
//  8. a wild trip leaves some class autonomous;
//  9. a desk@model row weaker than the desk row is ignored;
// 10. I4 is autonomous for a third-party desk.

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	t0      = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	classI2 = trust.ActionClass{Grant: "desk.request", Impact: types.ImpactI2}
	classI3 = trust.ActionClass{Grant: "desk.request", Impact: types.ImpactI3}
	classI4 = trust.ActionClass{Grant: "desk.request", Impact: types.ImpactI4}
	classI1 = trust.ActionClass{Grant: "desk.request", Impact: types.ImpactI1}
)

type fakeRevoker struct {
	calls []string
}

func (f *fakeRevoker) RevokeOnFidelity(_ context.Context, desk string, class trust.ActionClass, _ time.Time) int {
	f.calls = append(f.calls, desk+"|"+class.String())
	return 1
}

func row(id string, c trust.ActionClass, lcb float64, n int) trust.Row {
	return trust.Row{Subject: trust.Subject{Kind: trust.SubjectAgent, ID: id}, Class: c, LCB: lcb, N: n}
}

func newGate(now *time.Time) (*Gate, *fakeRevoker) {
	r := &fakeRevoker{}
	g := New(r)
	g.SetClock(func() time.Time { return *now })
	return g, r
}

func TestNonMaterialActsAreNotGated(t *testing.T) {
	now := t0
	g, _ := newGate(&now)
	for _, c := range []trust.ActionClass{classI1, {Grant: "desk.act", Impact: types.ImpactI0}} {
		if v := g.Decide("o", []string{"app:notes"}, "", c, true); v.Decision != NotMaterial {
			t.Fatalf("%s: %s (%s)", c, v.Decision, v.Reason)
		}
	}
}

func TestThinOrMissingViewsAreGated(t *testing.T) {
	now := t0
	g, _ := newGate(&now)
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Gated {
		t.Fatalf("no view: %s", v.Decision)
	}
	g.Observe("o", []trust.Row{row("app:notes", classI2, 0.95, 100)})
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Autonomous {
		t.Fatalf("fresh strong view: %s (%s)", v.Decision, v.Reason)
	}
	now = t0.Add(TrustMaxAge + time.Second)
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Gated {
		t.Fatalf("stale view: %s", v.Decision)
	}
	now = t0.Add(-time.Minute)
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Gated {
		t.Fatalf("view from the future: %s", v.Decision)
	}
	now = t0
	if v := g.Decide("other-owner", []string{"app:notes"}, "", classI2, true); v.Decision != Gated {
		t.Fatalf("another owner's view: %s", v.Decision)
	}
}

func TestNoRecordInTheClassIsGated(t *testing.T) {
	now := t0
	g, _ := newGate(&now)
	g.Observe("o", []trust.Row{row("app:notes", classI2, 0.99, 500)})
	if v := g.Decide("o", []string{"app:notes"}, "", classI3, true); v.Decision != Gated {
		t.Fatalf("I2 record carried to I3: %s", v.Decision)
	}
	if v := g.Decide("o", []string{"app:other"}, "", classI2, true); v.Decision != Gated {
		t.Fatalf("another desk's record: %s", v.Decision)
	}
}

func TestTheBoundAgainstTau(t *testing.T) {
	now := t0
	g, _ := newGate(&now)
	tau, _ := trust.Tau(types.ImpactI2)
	g.Observe("o", []trust.Row{row("notes", classI2, tau-0.001, 40)})
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Gated {
		t.Fatalf("below tau: %s", v.Decision)
	}
	g.Observe("o", []trust.Row{row("notes", classI2, tau, 40)})
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Autonomous {
		t.Fatalf("at tau: %s (%s)", v.Decision, v.Reason)
	}
}

func TestAWeakerModelRowGates(t *testing.T) {
	now := t0
	g, _ := newGate(&now)
	weak := trust.Row{Subject: trust.Subject{Kind: trust.SubjectAgentModel, ID: "app:notes@m1"}, Class: classI2, LCB: 0.2, N: 30}
	g.Observe("o", []trust.Row{row("app:notes", classI2, 0.95, 100), weak})
	if v := g.Decide("o", []string{"app:notes"}, "m1", classI2, true); v.Decision != Gated {
		t.Fatalf("weak desk@model row ignored: %s", v.Decision)
	}
	if v := g.Decide("o", []string{"app:notes"}, "m2", classI2, true); v.Decision != Autonomous {
		t.Fatalf("other model: %s (%s)", v.Decision, v.Reason)
	}
}

func TestAnOpenBreakerRowGates(t *testing.T) {
	now := t0
	g, _ := newGate(&now)
	r := row("app:notes", classI2, 0.95, 100)
	r.Breaker, r.LastFidelityAt = true, t0.Add(-time.Hour)
	g.Observe("o", []trust.Row{r})
	v := g.Decide("o", []string{"app:notes"}, "", classI2, true)
	if v.Decision != Gated || !v.Breaker {
		t.Fatalf("open breaker: %+v", v)
	}
}

func TestATripGatesAtOnceAndRevokes(t *testing.T) {
	now := t0
	g, rev := newGate(&now)
	g.Observe("o", []trust.Row{row("app:notes", classI2, 0.95, 100)})
	if n := g.Trip(context.Background(), "app:notes", classI2, t0); n != 1 || len(rev.calls) != 1 || rev.calls[0] != "app:notes|desk.request/I2" {
		t.Fatalf("trip: n=%d calls=%v", n, rev.calls)
	}
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Gated || !v.Breaker {
		t.Fatalf("after trip: %+v", v)
	}
	// A view that does not yet show the event keeps the drop.
	g.Observe("o", []trust.Row{row("app:notes", classI2, 0.95, 100)})
	if v := g.Decide("o", []string{"notes"}, "", classI2, true); v.Decision != Gated {
		t.Fatalf("drop forgotten before the view shows it: %s", v.Decision)
	}
	// A view that shows the event with the breaker still open: gated by the row.
	shown := row("app:notes", classI2, 0.95, 101)
	shown.LastFidelityAt, shown.Breaker = t0, true
	g.Observe("o", []trust.Row{shown})
	if _, dropped := g.Dropped("app:notes", classI2); dropped {
		t.Fatalf("the drop did not settle once the view showed the event")
	}
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Gated {
		t.Fatalf("open breaker row: %s", v.Decision)
	}
	// New positive evidence closes the breaker: autonomy returns.
	shown.Breaker = false
	g.Observe("o", []trust.Row{shown})
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Autonomous {
		t.Fatalf("after new evidence: %s (%s)", v.Decision, v.Reason)
	}
}

func TestAWildTripGatesEveryClass(t *testing.T) {
	now := t0
	g, rev := newGate(&now)
	g.Observe("o", []trust.Row{row("app:notes", classI2, 0.99, 500), row("app:notes", classI3, 0.99, 500)})
	g.Trip(context.Background(), "app:notes", Wild, t0)
	for _, c := range []trust.ActionClass{classI2, classI3} {
		if v := g.Decide("o", []string{"app:notes"}, "", c, true); v.Decision != Gated {
			t.Fatalf("%s after a wild trip: %s", c, v.Decision)
		}
	}
	if len(rev.calls) != 1 || rev.calls[0] != "app:notes|*/I4" {
		t.Fatalf("wild revoke: %v", rev.calls)
	}
	classes := TripClasses(Wild)
	if len(classes) == 0 {
		t.Fatal("a wild trip records no class")
	}
	for _, c := range classes {
		if r, _ := c.Impact.Rank(); r < 1 {
			t.Fatalf("wild trip class %s carries no evidence", c)
		}
	}
}

func TestI4IsNeverAutonomousForAThirdParty(t *testing.T) {
	now := t0
	g, _ := newGate(&now)
	g.Observe("o", []trust.Row{row("app:notes", classI4, 0.999, 5000)})
	if v := g.Decide("o", []string{"app:notes"}, "", classI4, true); v.Decision != Gated {
		t.Fatalf("I4 third-party: %s", v.Decision)
	}
	if v := g.Decide("o", []string{"app:notes"}, "", classI4, false); v.Decision != Autonomous {
		t.Fatalf("I4 first-party with a long record: %s (%s)", v.Decision, v.Reason)
	}
}

// A breaker row filed at the grant's table class gates an act its sender
// raised to a higher class of the same grant.
func TestABreakerRowGatesEveryImpactOfItsGrant(t *testing.T) {
	now := t0
	g, _ := newGate(&now)
	open := row("app:notes", classI1, 0.1, 3)
	open.Breaker, open.LastFidelityAt = true, t0
	g.Observe("o", []trust.Row{row("app:notes", classI2, 0.99, 500), open})
	if v := g.Decide("o", []string{"app:notes"}, "", classI2, true); v.Decision != Gated || !v.Breaker {
		t.Fatalf("I2 act with an open I1 breaker row of its grant: %+v", v)
	}
}
