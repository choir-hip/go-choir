package kernel

import (
	"context"
	"time"
)

// Options configures a Kernel. Zero values select defaults.
type Options struct {
	Journal   Journal
	Intensity Intensity
	Now       func() time.Time
}

// Kernel binds the scope tree, the supervisor and the journal.
type Kernel struct {
	root *Scope
	sup  *Supervisor
	j    Journal
}

// New makes a kernel. Call Boot before starting units.
func New(o Options) *Kernel {
	if o.Journal == nil {
		o.Journal = &MemJournal{}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	root := NewRoot("kernel")
	return &Kernel{root: root, sup: newSupervisor(root, o.Intensity, o.Journal, o.Now), j: o.Journal}
}

// Root is the root scope.
func (k *Kernel) Root() *Scope { return k.root }

// Supervisor is the kernel's supervisor.
func (k *Kernel) Supervisor() *Supervisor { return k.sup }

// Boot folds the journal into unit state. It runs no unit and replays no
// occurrence: work in flight at the last stop is closed as interrupted.
func (k *Kernel) Boot() error {
	recs, err := k.j.Load()
	if err != nil {
		return err
	}
	k.sup.fold(recs)
	return nil
}

// Shutdown closes the root scope (cancelling every unit, running every
// effect LIFO) and waits for supervision to end.
func (k *Kernel) Shutdown(ctx context.Context) error {
	err := k.root.Close(ctx, context.Canceled)
	k.sup.wait()
	return err
}
