package lease_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes the lease policy (MA7.4) must not have, written before the
// code:
//
//  1. a durable lease exceeds the ceiling: a scope wider than the manifest, or
//     a life longer than the platform's durable bound, however strong the
//     record;
//  2. a missing, stale, NaN, infinite or out-of-range posterior buys a durable
//     lease (each must fall back to per-task);
//  3. a thin record (few items, low lower bound, breaker set) gets a durable
//     lease;
//  4. a fidelity event leaves a lease of the class live, or a read in flight
//     when the revoke returns is admitted after it;
//  5. a durable lease renews after a fidelity event (the hook fired, the
//     snapshot has not seen the event yet);
//  6. clock skew: a clock that runs back extends a lease, or one that runs
//     forward renews it on a stale posterior;
//  7. a durable lease weakens a bound per-task leases keep: another owner's run
//     attaches to it, the boot pass leaves it live, a first-party grant
//     becomes durable, or a run that has not asked is admitted.

var readClass = trust.ActionClass{Grant: lease.ClassReadKind, Impact: types.ImpactI1}

func row(desk string, lcb float64, n int) trust.Row {
	return trust.Row{Subject: trust.Subject{Kind: trust.SubjectAgent, ID: desk}, Class: readClass, LCB: lcb, N: n, Mean: lcb}
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func policyFixture(t *testing.T) (*fixture, *clock) {
	f := newFixture(t)
	c := &clock{t: time.Now().UTC()}
	f.svc.SetClock(c.now)
	return f, c
}

func (f *fixture) observe(rows ...trust.Row) {
	f.svc.ObserveTrust(context.Background(), ownerID, rows)
}

func (f *fixture) ask(owner, runID string) (lease.Record, error) {
	return f.svc.Request(context.Background(), lease.Request{
		OwnerID: owner, ComputerID: computer, Desk: "app:" + readerPkg, RunID: runID, AgentID: "agent-" + runID,
		Scope: lease.Scope{Kinds: []string{string(clipKind)}}, Purpose: "edit the clip", Fields: fields(0.8, 0.3),
	})
}

// (proof 1) A strong-record fixture gets a durable lease that renews.
func TestAStrongRecordGetsADurableLeaseThatRenews(t *testing.T) {
	f, c := policyFixture(t)
	f.observe(row("app:"+readerPkg, 0.9, 60))
	rec, err := f.ask(ownerID, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if !rec.Durable || rec.ExpiresAt.IsZero() || rec.Basis == nil || !rec.Basis.Durable {
		t.Fatalf("a strong record got %+v", rec)
	}
	if d := rec.ExpiresAt.Sub(rec.IssuedAt); d < lease.DurableMin || d > lease.DurableMax {
		t.Fatalf("durable life %v outside [%v, %v]", d, lease.DurableMin, lease.DurableMax)
	}
	// The run that asked ends; the lease does not.
	f.svc.EndRun(context.Background(), "run-1", lease.ReasonRunEnded)
	if err := f.read("run-1", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("an ended run still reads through the durable lease: %v", err)
	}
	// A run that has not asked holds nothing.
	if err := f.read("run-2", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a run that never asked read through the durable lease: %v", err)
	}
	again, err := f.ask(ownerID, "run-2")
	if err != nil || again.LeaseID != rec.LeaseID {
		t.Fatalf("the next run's ask did not attach to the durable lease: %+v, %v", again, err)
	}
	if err := f.read("run-2", clipKind); err != nil {
		t.Fatal(err)
	}
	// Past its expiry, with the record still strong, it renews without asking.
	c.set(rec.ExpiresAt.Add(time.Second))
	f.observe(row("app:"+readerPkg, 0.9, 61))
	if err := f.read("run-2", clipKind); err != nil {
		t.Fatalf("a strong durable lease did not renew: %v", err)
	}
	live := f.svc.Live(ownerID)
	if len(live) != 1 || live[0].Renewals != 1 || !live[0].ExpiresAt.After(c.now()) {
		t.Fatalf("live after renewal = %+v", live)
	}
	recs, _ := f.st.Leases().ListLeases(context.Background(), ownerID, computer)
	if len(recs) != 1 || recs[0].Renewals != 1 || recs[0].State != lease.StateActive {
		t.Fatalf("tape after renewal = %+v", recs)
	}

	// Past expiry again with the record now thin: it ends, it does not renew.
	c.set(live[0].ExpiresAt.Add(time.Second))
	f.observe(row("app:"+readerPkg, 0.4, 61))
	if err := f.read("run-2", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a durable lease renewed on a thin record: %v", err)
	}
	recs, _ = f.st.Leases().ListLeases(context.Background(), ownerID, computer)
	if len(recs) != 1 || recs[0].State != lease.StateEnded || recs[0].EndReason != lease.ReasonExpired {
		t.Fatalf("tape = %+v", recs)
	}
}

// (proof 2) A thin record stays per-task; so does every malformed posterior.
func TestAThinOrMissingPosteriorStaysPerTask(t *testing.T) {
	desk := "app:" + readerPkg
	breaker := row(desk, 0.95, 80)
	breaker.Breaker = true
	cases := map[string][]trust.Row{
		"no snapshot":     nil,
		"no row":          {row("app:someone-else", 0.99, 500)},
		"few items":       {row(desk, 0.95, lease.DurableMinN-1)},
		"low bound":       {row(desk, lease.DurableFloor-0.01, 500)},
		"NaN bound":       {row(desk, math.NaN(), 500)},
		"infinite bound":  {row(desk, math.Inf(1), 500)},
		"bound above one": {row(desk, 1.5, 500)},
		"negative bound":  {row(desk, -0.2, 500)},
		"breaker":         {breaker},
		// the agent row is strong but the desk on its model is not: the weakest
		// row decides.
		"weak model row": {row(desk, 0.95, 80), {Subject: trust.Subject{Kind: trust.SubjectAgentModel, ID: desk + "@m1"}, Class: readClass, LCB: 0.3, N: 80}},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			f, _ := policyFixture(t)
			if rows != nil {
				f.observe(rows...)
			}
			rec, err := f.svc.Request(context.Background(), lease.Request{
				OwnerID: ownerID, ComputerID: computer, Desk: desk, RunID: "run-t", AgentID: "a", ModelID: "m1",
				Scope: lease.Scope{Kinds: []string{string(clipKind)}}, Purpose: "edit", Fields: fields(0.8, 0.3),
			})
			if err != nil {
				t.Fatal(err)
			}
			if rec.Durable || !rec.ExpiresAt.IsZero() || len(rec.Runs) > 0 {
				t.Fatalf("%s bought a durable lease: %+v", name, rec)
			}
			if rec.Basis == nil || rec.Basis.Durable || rec.Basis.Reason == "" {
				t.Fatalf("the per-task basis is not recorded: %+v", rec.Basis)
			}
			f.svc.EndRun(context.Background(), "run-t", lease.ReasonRunEnded)
			if err := f.read("run-t", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
				t.Fatalf("a per-task lease outlived its run: %v", err)
			}
		})
	}
}

// A stale snapshot, or a clock that ran back past it, is no posterior.
func TestAStaleOrSkewedSnapshotIsNoPosterior(t *testing.T) {
	f, c := policyFixture(t)
	f.observe(row("app:"+readerPkg, 0.95, 80))
	c.add(lease.TrustMaxAge + time.Second)
	if rec, err := f.ask(ownerID, "run-s1"); err != nil || rec.Durable {
		t.Fatalf("a stale snapshot bought a durable lease: %+v, %v", rec, err)
	}
	f.observe(row("app:"+readerPkg, 0.95, 80))
	c.add(-time.Hour)
	if rec, err := f.ask(ownerID, "run-s2"); err != nil || rec.Durable {
		t.Fatalf("a snapshot from the future bought a durable lease: %+v, %v", rec, err)
	}
}

// (failure 1) However strong the record, the ceiling and the durable bound hold.
func TestADurableLeaseNeverExceedsTheCeiling(t *testing.T) {
	f, _ := policyFixture(t)
	f.observe(row("app:"+readerPkg, 1, 100000))
	// Outside the ceiling: refused, strong record or not.
	if _, err := f.svc.Request(context.Background(), lease.Request{
		OwnerID: ownerID, ComputerID: computer, Desk: "app:" + readerPkg, RunID: "run-c", AgentID: "a",
		Scope: lease.Scope{Kinds: []string{string(secret)}}, Purpose: "x", Fields: fields(0.8, 0.3),
	}); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("a strong record widened past the ceiling: %v", err)
	}
	rec, err := f.ask(ownerID, "run-c")
	if err != nil || !rec.Durable {
		t.Fatalf("%+v, %v", rec, err)
	}
	if len(rec.Scope.Kinds) != 1 || rec.Scope.Kinds[0] != string(clipKind) {
		t.Fatalf("durable scope %+v is not the asked scope", rec.Scope)
	}
	if d := rec.ExpiresAt.Sub(rec.IssuedAt); d > lease.DurableMax {
		t.Fatalf("durable life %v exceeds the bound %v", d, lease.DurableMax)
	}
	// The durable lease opens nothing beyond its scope.
	if err := f.read("run-c", secret); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a durable lease opened another kind: %v", err)
	}
	// A desk's own shorter duration bounds the durable life.
	if err := f.svc.Revoke(context.Background(), rec.LeaseID, lease.ReasonOwner); err != nil {
		t.Fatal(err)
	}
	short, err := f.svc.Request(context.Background(), lease.Request{
		OwnerID: ownerID, ComputerID: computer, Desk: "app:" + readerPkg, RunID: "run-d", AgentID: "a",
		Scope: lease.Scope{Kinds: []string{string(clipKind)}}, Purpose: "x", Duration: 10 * time.Minute, Fields: fields(0.8, 0.3),
	})
	if err != nil || short.ExpiresAt.Sub(short.IssuedAt) > 10*time.Minute {
		t.Fatalf("the asked duration was not a bound: %+v, %v", short, err)
	}
}

// (proof 3) A fidelity event revokes at once, and nothing renews after it
// until the record shows new evidence.
func TestAFidelityEventRevokesAtOnce(t *testing.T) {
	f, c := policyFixture(t)
	desk := "app:" + readerPkg
	f.observe(row(desk, 0.9, 60))
	durable, err := f.ask(ownerID, "run-f1")
	if err != nil || !durable.Durable {
		t.Fatalf("%+v, %v", durable, err)
	}
	if err := f.read("run-f1", clipKind); err != nil {
		t.Fatal(err)
	}
	at := c.now()
	if n := f.svc.RevokeOnFidelity(context.Background(), desk, readClass, at); n != 1 {
		t.Fatalf("the breaker revoked %d leases, want 1", n)
	}
	if err := f.read("run-f1", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a read after the fidelity event was admitted: %v", err)
	}
	recs, _ := f.st.Leases().ListLeases(context.Background(), ownerID, computer)
	if len(recs) != 1 || recs[0].State != lease.StateRevoked || recs[0].EndReason != lease.ReasonFidelity {
		t.Fatalf("tape = %+v", recs)
	}
	// The snapshot still predates the event: no durable lease on it.
	c.add(time.Second)
	if rec, err := f.ask(ownerID, "run-f2"); err != nil || rec.Durable {
		t.Fatalf("a durable lease was issued after a fidelity event on a stale record: %+v, %v", rec, err)
	}
	// The record sees the event with no later positive evidence: still per-task.
	seen := row(desk, 0.9, 61)
	seen.Breaker, seen.LastFidelityAt = true, at
	f.observe(seen)
	if rec, err := f.ask(ownerID, "run-f3"); err != nil || rec.Durable {
		t.Fatalf("a durable lease was issued under the breaker: %+v, %v", rec, err)
	}
	// New positive evidence clears the breaker: a durable lease is earned again.
	cleared := row(desk, 0.9, 70)
	cleared.LastFidelityAt = at
	f.observe(cleared)
	if rec, err := f.ask(ownerID, "run-f4"); err != nil || !rec.Durable {
		t.Fatalf("new evidence did not earn the durable lease back: %+v, %v", rec, err)
	}
}

// The breaker read from the trust view: a row that shows a fidelity event
// revokes the class's durable leases and the per-task leases issued before the
// event, and leaves the per-task leases issued under the breaker alone.
func TestTheTrustViewsBreakerRevokesTheClass(t *testing.T) {
	f, c := policyFixture(t)
	desk := "app:" + readerPkg
	f.observe(row(desk, 0.9, 60))
	if rec, err := f.ask(ownerID, "run-v1"); err != nil || !rec.Durable {
		t.Fatalf("%+v, %v", rec, err)
	}
	f.observe() // no posterior: the next is per-task
	if rec, err := f.ask(ownerID, "run-v2"); err != nil || rec.Durable {
		t.Fatalf("%+v, %v", rec, err)
	}
	event := c.now()
	c.add(time.Minute)
	broken := row(desk, 0.9, 61)
	broken.Breaker, broken.LastFidelityAt = true, event
	f.observe(broken)
	for _, run := range []string{"run-v1", "run-v2"} {
		if err := f.read(run, clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
			t.Fatalf("%s still reads after the breaker: %v", run, err)
		}
	}
	after, err := f.ask(ownerID, "run-v3")
	if err != nil || after.Durable {
		t.Fatalf("%+v, %v", after, err)
	}
	f.observe(broken) // the same view again
	if err := f.read("run-v3", clipKind); err != nil {
		t.Fatalf("a per-task lease issued under the breaker was revoked: %v", err)
	}
}

// (failure 4) A revoke races in-flight admissions: once the revoke has
// returned, no read that starts is admitted.
func TestRevocationRacesInFlightAdmission(t *testing.T) {
	f, _ := policyFixture(t)
	desk := "app:" + readerPkg
	f.observe(row(desk, 0.9, 60))
	if rec, err := f.ask(ownerID, "run-r"); err != nil || !rec.Durable {
		t.Fatalf("%+v, %v", rec, err)
	}
	var revoked atomic.Bool
	var late atomic.Int64
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				after := revoked.Load()
				if f.svc.AdmitsKind(f.reader, "run-r", clipKind) && after {
					late.Add(1)
				}
			}
		}()
	}
	time.Sleep(5 * time.Millisecond)
	f.svc.RevokeOnFidelity(context.Background(), desk, readClass, time.Now())
	revoked.Store(true)
	time.Sleep(5 * time.Millisecond)
	close(stop)
	wg.Wait()
	if n := late.Load(); n != 0 {
		t.Fatalf("%d admissions started after the revoke returned", n)
	}
}

// (failure 6) A clock that runs back neither admits nor extends a durable
// lease; one that runs forward past the snapshot does not renew it.
func TestClockSkewNeverExtendsADurableLease(t *testing.T) {
	f, c := policyFixture(t)
	f.observe(row("app:"+readerPkg, 0.9, 60))
	rec, err := f.ask(ownerID, "run-k")
	if err != nil || !rec.Durable {
		t.Fatalf("%+v, %v", rec, err)
	}
	c.add(-time.Hour)
	if err := f.read("run-k", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a lease was admitted before its issue on a clock that ran back: %v", err)
	}
	for _, l := range f.svc.Live(ownerID) {
		if l.ExpiresAt.Sub(l.IssuedAt) > lease.DurableMax {
			t.Fatalf("a lease's life grew past the bound: %+v", l)
		}
	}
	// Forward past the expiry, the snapshot now older than its bound: no renew.
	c.set(rec.ExpiresAt.Add(time.Second))
	if err := f.read("run-k", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a durable lease renewed on a stale snapshot: %v", err)
	}
}

// (failure 7) Durable leases keep every bound per-task leases keep.
func TestADurableLeaseKeepsTheOwnerAndBootBounds(t *testing.T) {
	f, _ := policyFixture(t)
	f.observe(row("app:"+readerPkg, 0.9, 60))
	mine, err := f.ask(ownerID, "run-o1")
	if err != nil || !mine.Durable {
		t.Fatalf("%+v, %v", mine, err)
	}
	theirs, err := f.ask("another-owner", "run-o2")
	if err != nil {
		t.Fatal(err)
	}
	// The other owner's records were never read: per-task, whatever this
	// owner's record says.
	if theirs.LeaseID == mine.LeaseID || theirs.OwnerID != "another-owner" || theirs.Durable {
		t.Fatalf("another owner's run attached to the durable lease: %+v", theirs)
	}
	// The boot pass ends durable leases like every other.
	booted := lease.New(f.st.Leases())
	defer booted.Close()
	if err := booted.Load(context.Background(), ownerID, computer); err != nil {
		t.Fatal(err)
	}
	if n := booted.EndAll(context.Background(), lease.ReasonRestart); n < 1 {
		t.Fatalf("boot ended %d leases", n)
	}
	f.svc = booted
	if err := f.read("run-o1", clipKind); !errors.Is(err, objectgraph.ErrWrongReader) {
		t.Fatalf("a durable lease survived the boot: %v", err)
	}
}

// First-party grants stay per-task: the policy shapes only a desk's own ask.
func TestFirstPartyGrantsStayPerTask(t *testing.T) {
	f, _ := policyFixture(t)
	f.observe(trust.Row{Subject: trust.Subject{Kind: trust.SubjectAgent, ID: deskmanifest.ResearchTexture},
		Class: trust.ActionClass{Grant: lease.GrantTextureSearch, Impact: types.ImpactI1}, LCB: 0.99, N: 1000})
	rec, err := f.svc.IssueTextureSearch(context.Background(), ownerID, computer, "run-g", deskmanifest.ResearchTexture+":k1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Durable {
		t.Fatalf("a first-party grant became durable: %+v", rec)
	}
}
