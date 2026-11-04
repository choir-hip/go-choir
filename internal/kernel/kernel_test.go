package kernel

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func tctx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestScopeCloseCancelsDescendantsAndRunsEffectsLIFO(t *testing.T) {
	root := NewRoot("r")
	child, _ := root.Child("c")
	grand, _ := child.Child("g")
	var order []string
	add := func(s *Scope, n string) {
		if err := s.OnClose(n, func(context.Context) error { order = append(order, n); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	add(root, "r1")
	add(root, "r2")
	add(child, "c1")
	add(grand, "g1")
	add(grand, "g2")
	if err := root.Close(tctx(t), errors.New("why")); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(order, ","), "g2,g1,c1,r2,r1"; got != want {
		t.Fatalf("order %s want %s", got, want)
	}
	if grand.Context().Err() == nil || !strings.Contains(context.Cause(grand.Context()).Error(), "why") {
		t.Fatalf("descendant not cancelled with cause: %v", context.Cause(grand.Context()))
	}
	if err := root.OnClose("late", nil); !errors.Is(err, ErrScopeClosed) {
		t.Fatalf("late OnClose = %v", err)
	}
	if _, err := root.Child("late"); !errors.Is(err, ErrScopeClosed) {
		t.Fatalf("late Child = %v", err)
	}
}

func TestEffectsAreAwaitedAndAllRunDespiteFailure(t *testing.T) {
	s := NewRoot("r")
	var finished, ran atomic.Int32
	_ = s.OnClose("a", func(context.Context) error { ran.Add(1); return nil })
	_ = s.OnClose("panics", func(context.Context) error { ran.Add(1); panic("boom") })
	_ = s.OnClose("slow", func(context.Context) error {
		time.Sleep(50 * time.Millisecond)
		finished.Add(1)
		return errors.New("slow failed")
	})
	err := s.Close(tctx(t), nil)
	if finished.Load() != 1 {
		t.Fatal("Close returned before the slow effect finished")
	}
	if ran.Load() != 2 {
		t.Fatal("effects after a failing one did not run")
	}
	if err == nil || !strings.Contains(err.Error(), "slow failed") || !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("errors not joined: %v", err)
	}
}

func TestAbandonedEffectIsNamed(t *testing.T) {
	s := NewRoot("r")
	release := make(chan struct{})
	defer close(release)
	_ = s.OnClose("stuck", func(context.Context) error { <-release; return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := s.Close(ctx, nil)
	if err == nil || !strings.Contains(err.Error(), `"stuck"`) || !strings.Contains(err.Error(), "abandoned") {
		t.Fatalf("abandoned effect not reported: %v", err)
	}
}

func TestConcurrentCloseRunsEffectsOnce(t *testing.T) {
	s := NewRoot("r")
	var n atomic.Int32
	_ = s.OnClose("e", func(context.Context) error { time.Sleep(20 * time.Millisecond); n.Add(1); return nil })
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { errs <- s.Close(tctx(t), nil) }()
	}
	<-errs
	<-errs
	if n.Load() != 1 {
		t.Fatalf("effect ran %d times", n.Load())
	}
}

func TestProvideInject(t *testing.T) {
	root := NewRoot("r")
	child, _ := root.Child("c")
	sib, _ := root.Child("s")
	if err := Provide(root, "greeting", "hi"); err != nil {
		t.Fatal(err)
	}
	if err := Provide(root, "greeting", "again"); !errors.Is(err, ErrServiceExists) {
		t.Fatalf("dup = %v", err)
	}
	if v, err := Inject[string](child, "greeting"); err != nil || v != "hi" {
		t.Fatalf("inject = %q %v", v, err)
	}
	if _, err := Inject[int](child, "greeting"); !errors.Is(err, ErrServiceType) {
		t.Fatalf("type = %v", err)
	}
	_ = Provide(child, "private", 7)
	if _, err := Inject[int](sib, "private"); !errors.Is(err, ErrNotProvided) {
		t.Fatalf("sibling saw child service: %v", err)
	}
	_ = child.Close(tctx(t), nil)
	if _, err := Inject[int](child, "private"); !errors.Is(err, ErrNotProvided) {
		t.Fatalf("closed scope served: %v", err)
	}
	if err := Provide(child, "x", 1); !errors.Is(err, ErrScopeClosed) {
		t.Fatalf("provide on closed = %v", err)
	}
}

// fakeClock returns successive times from a settable instant.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func TestSupervisorTwoCrashesInWindowCloseWithCause(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	k := New(Options{Now: clk.now})
	_ = k.Boot()
	var runs atomic.Int32
	var cleaned atomic.Int32
	err := k.Supervisor().Start(Unit{Name: "u", Run: func(ctx context.Context, s *Scope) error {
		runs.Add(1)
		_ = s.OnClose("cleanup", func(context.Context) error { cleaned.Add(1); return nil })
		return errors.New("kaput")
	}})
	if err != nil {
		t.Fatal(err)
	}
	state, cause, err := k.Supervisor().Await(tctx(t), "u")
	if err != nil || state != UnitClosed || !strings.Contains(cause, "kaput") || !strings.Contains(cause, "2 crashes") {
		t.Fatalf("state=%s cause=%q err=%v", state, cause, err)
	}
	if runs.Load() != 2 || cleaned.Load() != 2 {
		t.Fatalf("runs=%d cleaned=%d, want 2 and 2", runs.Load(), cleaned.Load())
	}
	if err := k.Supervisor().Start(Unit{Name: "u", Run: func(context.Context, *Scope) error { return nil }}); !errors.Is(err, ErrUnitClosed) {
		t.Fatalf("restart of closed unit = %v", err)
	}
	if err := k.Supervisor().Reopen("u"); err != nil {
		t.Fatal(err)
	}
	if err := k.Supervisor().Start(Unit{Name: "u", Run: func(context.Context, *Scope) error { return nil }}); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := k.Supervisor().Await(tctx(t), "u"); s != UnitStopped {
		t.Fatalf("after reopen state = %s", s)
	}
	_ = k.Shutdown(tctx(t))
}

func TestSupervisorCrashesOutsideWindowDoNotClose(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	k := New(Options{Now: clk.now, Intensity: Intensity{MaxCrashes: 2, Window: time.Minute}})
	_ = k.Boot()
	var runs atomic.Int32
	_ = k.Supervisor().Start(Unit{Name: "u", Run: func(context.Context, *Scope) error {
		if runs.Add(1) < 3 {
			clk.t = clk.t.Add(2 * time.Minute) // the next crash lands outside the window
			return errors.New("crash")
		}
		return nil
	}})
	state, _, _ := k.Supervisor().Await(tctx(t), "u")
	if state != UnitStopped || runs.Load() != 3 {
		t.Fatalf("state=%s runs=%d", state, runs.Load())
	}
	_ = k.Shutdown(tctx(t))
}

func TestSupervisorPanicIsACrash(t *testing.T) {
	k := New(Options{})
	_ = k.Boot()
	_ = k.Supervisor().Start(Unit{Name: "u", Run: func(context.Context, *Scope) error { panic("p") }})
	state, cause, _ := k.Supervisor().Await(tctx(t), "u")
	if state != UnitClosed || !strings.Contains(cause, "panic: p") {
		t.Fatalf("state=%s cause=%q", state, cause)
	}
	_ = k.Shutdown(tctx(t))
}

func TestBootFoldRebuildsStateAndReplaysNothing(t *testing.T) {
	j := &MemJournal{}
	at := time.Unix(1, 0)
	for _, r := range []Record{
		{Unit: "inflight", Kind: RecordDeclared, At: at},
		{Unit: "inflight", Kind: RecordStarted, At: at},
		{Unit: "clean", Kind: RecordDeclared, At: at},
		{Unit: "clean", Kind: RecordStarted, At: at},
		{Unit: "clean", Kind: RecordStopped, At: at},
		{Unit: "dead", Kind: RecordClosed, Cause: "2 crashes", At: at},
	} {
		_ = j.Append(r)
	}
	var ran atomic.Int32
	k := New(Options{Journal: j})
	if err := k.Boot(); err != nil {
		t.Fatal(err)
	}
	got := map[string]UnitInfo{}
	for _, u := range k.Snapshot().Units {
		got[u.Name] = u
	}
	if got["inflight"].State != UnitClosed || got["inflight"].Cause != CauseInterrupted {
		t.Fatalf("inflight = %+v", got["inflight"])
	}
	if got["clean"].State != UnitStopped || got["dead"].State != UnitClosed || got["dead"].Cause != "2 crashes" {
		t.Fatalf("fold = %+v", got)
	}
	// Booting does not start anything, and the interrupted unit cannot be
	// re-dispatched.
	if err := k.Supervisor().Start(Unit{Name: "inflight", Run: func(context.Context, *Scope) error { ran.Add(1); return nil }}); !errors.Is(err, ErrUnitClosed) {
		t.Fatalf("interrupted unit restarted: %v", err)
	}
	if ran.Load() != 0 {
		t.Fatal("an occurrence was replayed")
	}
	// A second boot over the extended journal folds to the same state.
	k2 := New(Options{Journal: j})
	_ = k2.Boot()
	if a, b := k.Snapshot().Units, k2.Snapshot().Units; len(a) != len(b) || a[0] != b[0] || a[1] != b[1] || a[2] != b[2] {
		t.Fatalf("fold not stable: %+v vs %+v", a, b)
	}
	_ = k.Shutdown(tctx(t))
	_ = k2.Shutdown(tctx(t))
}

func TestSnapshotListsScopesEffectsServicesAndLeavesNoResidue(t *testing.T) {
	k := New(Options{})
	_ = k.Boot()
	base := k.Snapshot()
	c, _ := k.Root().Child("plug")
	_ = c.OnClose("one", func(context.Context) error { return nil })
	_ = c.OnClose("two", func(context.Context) error { return nil })
	_ = Provide(c, "svc", 1)
	s := k.Snapshot()
	if len(s.Scope.Children) != 1 || strings.Join(s.Scope.Children[0].Effects, ",") != "two,one" || s.Scope.Children[0].Services[0] != "svc" {
		t.Fatalf("snapshot = %+v", s)
	}
	_ = c.Close(tctx(t), nil)
	after := k.Snapshot()
	if len(after.Scope.Children) != 0 || after.Scope.Name != base.Scope.Name || len(after.Units) != 0 {
		t.Fatalf("residue after close: %+v", after)
	}
	_ = k.Shutdown(tctx(t))
}

func TestShutdownHonorsCtxAndNamesStuckUnits(t *testing.T) {
	k := New(Options{CloseTimeout: 20 * time.Millisecond})
	_ = k.Boot()
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{})
	// Run ignores ctx cancellation.
	_ = k.Supervisor().Start(Unit{Name: "wedged", Run: func(context.Context, *Scope) error {
		close(started)
		<-release
		return nil
	}})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- k.Shutdown(ctx) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), `"wedged"`) {
			t.Fatalf("want ctx error naming wedged, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown hung on a ctx-ignoring Run")
	}
}

func TestWedgedOnCloseIsBoundedPerOccurrence(t *testing.T) {
	k := New(Options{CloseTimeout: 20 * time.Millisecond})
	_ = k.Boot()
	release := make(chan struct{})
	defer close(release)
	_ = k.Supervisor().Start(Unit{Name: "u", Run: func(_ context.Context, s *Scope) error {
		return s.OnClose("stuck", func(context.Context) error { <-release; return nil })
	}})
	state, cause, err := k.Supervisor().Await(tctx(t), "u")
	if err != nil || state != UnitClosed || !strings.Contains(cause, "abandoned") {
		t.Fatalf("state=%s cause=%q err=%v", state, cause, err)
	}
	if err := k.Shutdown(tctx(t)); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestStartRefusesNilRunAndClosedKernel(t *testing.T) {
	k := New(Options{})
	_ = k.Boot()
	if err := k.Supervisor().Start(Unit{Name: "nil"}); err == nil {
		t.Fatal("nil Run accepted")
	}
	_ = k.Shutdown(tctx(t))
	err := k.Supervisor().Start(Unit{Name: "late", Run: func(context.Context, *Scope) error { return nil }})
	if !errors.Is(err, ErrScopeClosed) {
		t.Fatalf("start after shutdown: %v", err)
	}
}
