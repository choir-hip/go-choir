package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrUnitClosed is returned when starting a unit that was closed by crashes
// or by a restart. Only Reopen, an explicit act, clears it.
var ErrUnitClosed = errors.New("kernel: unit closed")

// ErrUnitRunning is returned when starting a unit already supervised.
var ErrUnitRunning = errors.New("kernel: unit already running")

// Unit states.
const (
	UnitIdle    = "idle"
	UnitRunning = "running"
	UnitStopped = "stopped"
	UnitClosed  = "closed"
)

// Intensity bounds restarts: MaxCrashes crashes inside Window close the unit.
type Intensity struct {
	MaxCrashes int
	Window     time.Duration
}

// DefaultIntensity is two crashes in ten minutes.
var DefaultIntensity = Intensity{MaxCrashes: 2, Window: 10 * time.Minute}

// Unit is a supervised piece of work. Run gets a fresh scope for each
// occurrence and nothing from a crashed one; it returns nil for a clean end.
type Unit struct {
	Name string
	Run  func(ctx context.Context, s *Scope) error
}

type unitRec struct {
	name    string
	state   string
	cause   string
	crashes []time.Time
	starts  int
	done    chan struct{}
}

// Supervisor runs units, restarting a crashed one fresh until its restart
// intensity is exceeded.
type Supervisor struct {
	parent  *Scope
	in      Intensity
	journal Journal
	now     func() time.Time

	mu    sync.Mutex
	units map[string]*unitRec
	wg    sync.WaitGroup
}

func newSupervisor(parent *Scope, in Intensity, j Journal, now func() time.Time) *Supervisor {
	if in.MaxCrashes <= 0 || in.Window <= 0 {
		in = DefaultIntensity
	}
	return &Supervisor{parent: parent, in: in, journal: j, now: now, units: map[string]*unitRec{}}
}

func (sv *Supervisor) record(unit, kind, cause string) {
	_ = sv.journal.Append(Record{Unit: unit, Kind: kind, Cause: cause, At: sv.now()})
}

// Start supervises u. It returns ErrUnitClosed for a closed unit.
func (sv *Supervisor) Start(u Unit) error {
	sv.mu.Lock()
	rec, ok := sv.units[u.Name]
	if !ok {
		rec = &unitRec{name: u.Name, state: UnitIdle}
		sv.units[u.Name] = rec
		sv.record(u.Name, RecordDeclared, "")
	}
	switch rec.state {
	case UnitClosed:
		cause := rec.cause
		sv.mu.Unlock()
		return fmt.Errorf("%w: %s: %s", ErrUnitClosed, u.Name, cause)
	case UnitRunning:
		sv.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrUnitRunning, u.Name)
	}
	rec.state = UnitRunning
	rec.done = make(chan struct{})
	sv.wg.Add(1)
	sv.mu.Unlock()
	go sv.supervise(u, rec)
	return nil
}

func (sv *Supervisor) supervise(u Unit, rec *unitRec) {
	defer sv.wg.Done()
	defer close(rec.done)
	for {
		occ, err := sv.parent.Child("unit:" + u.Name)
		if err != nil { // the kernel is shutting down
			sv.finish(rec, UnitStopped, "")
			return
		}
		sv.mu.Lock()
		rec.starts++
		sv.mu.Unlock()
		sv.record(u.Name, RecordStarted, "")
		runErr := safeRun(u, occ)
		// The occurrence's effects run whatever the outcome.
		closeErr := occ.Close(context.Background(), runErr)
		if runErr == nil && closeErr != nil {
			runErr = closeErr
		}
		if runErr == nil || sv.parent.Context().Err() != nil {
			sv.finish(rec, UnitStopped, "")
			return
		}
		now := sv.now()
		sv.mu.Lock()
		kept := rec.crashes[:0]
		for _, t := range rec.crashes {
			if now.Sub(t) < sv.in.Window {
				kept = append(kept, t)
			}
		}
		rec.crashes = append(kept, now)
		n := len(rec.crashes)
		sv.mu.Unlock()
		if n >= sv.in.MaxCrashes {
			cause := fmt.Sprintf("%d crashes in %s; last: %v", n, sv.in.Window, runErr)
			sv.finish(rec, UnitClosed, cause)
			return
		}
		// Below intensity: a fresh occurrence. Nothing carries over.
	}
}

func safeRun(u Unit, s *Scope) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return u.Run(s.Context(), s)
}

func (sv *Supervisor) finish(rec *unitRec, state, cause string) {
	sv.mu.Lock()
	rec.state, rec.cause = state, cause
	sv.mu.Unlock()
	kind := RecordStopped
	if state == UnitClosed {
		kind = RecordClosed
	}
	sv.record(rec.name, kind, cause)
}

// Await blocks until the unit's supervision ends or ctx is done, then returns
// the unit's state and cause.
func (sv *Supervisor) Await(ctx context.Context, name string) (state, cause string, err error) {
	sv.mu.Lock()
	rec, ok := sv.units[name]
	var done chan struct{}
	if ok {
		done = rec.done
	}
	sv.mu.Unlock()
	if !ok {
		return "", "", fmt.Errorf("kernel: unknown unit %q", name)
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return "", "", ctx.Err()
		}
	}
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return rec.state, rec.cause, nil
}

// Reopen clears a closed unit so it may be started again. It is the owner
// act that restarts a unit the supervisor closed.
func (sv *Supervisor) Reopen(name string) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	rec, ok := sv.units[name]
	if !ok || rec.state != UnitClosed {
		return fmt.Errorf("kernel: unit %q is not closed", name)
	}
	rec.state, rec.cause, rec.crashes = UnitIdle, "", nil
	sv.record(name, RecordDeclared, "reopened")
	return nil
}

// fold rebuilds unit state from journal records. It never runs a unit. A unit
// whose last record is "started" was cut off by a restart and is closed with
// CauseInterrupted.
func (sv *Supervisor) fold(recs []Record) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	for _, r := range recs {
		u, ok := sv.units[r.Unit]
		if !ok {
			u = &unitRec{name: r.Unit}
			sv.units[r.Unit] = u
		}
		switch r.Kind {
		case RecordDeclared:
			u.state, u.cause = UnitIdle, ""
		case RecordStarted:
			u.state, u.cause = UnitRunning, ""
		case RecordStopped:
			u.state, u.cause = UnitStopped, ""
		case RecordClosed:
			u.state, u.cause = UnitClosed, r.Cause
		}
	}
	for _, u := range sv.units {
		if u.state == UnitRunning && u.done == nil {
			u.state, u.cause = UnitClosed, CauseInterrupted
			sv.record(u.name, RecordClosed, CauseInterrupted)
		}
	}
}

func (sv *Supervisor) wait() { sv.wg.Wait() }
