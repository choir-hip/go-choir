package agentcore

import "context"

// SCAFFOLD of the review-round-1 repair, replaced by the code commit: the
// seams the install protocol's tests reach, and the two signatures they use.
// They do nothing, so the tests fail on behavior, not on a missing name.

// restoreInstallCrashHook, when set, is called at each named point of the
// install (after_prepared, after_release_restaged, after_flip_started,
// mid_flip, before_flip_completed, after_flip_completed, after_reopen). A test
// child kills itself there.
var restoreInstallCrashHook func(point string)

// restoreInstallWriteFault, when set, fails the durable write of the named
// install phase.
var restoreInstallWriteFault func(phase string) error

// restoreReopenFault, when set, fails the reopen of the flipped store.
var restoreReopenFault func() error

// restoreRepairStepHook, when set, is called before each step of the boot
// repair (undo, release, close); returning true stops the repair there, as a
// kill would.
var restoreRepairStepHook func(step string) bool

// RestoreBootReport is what the boot's repair found: the acts it closed and
// the acts it could not resolve.
type RestoreBootReport struct {
	Closed     []string
	Unresolved []string
}

// WithRestoreUnresolved tells the runtime which acts the boot left unresolved.
func WithRestoreUnresolved(ids []string) RuntimeOption { return func(*Runtime) {} }

// snapshotBusyReason is why a snapshot capture must not run now.
func (rt *Runtime) snapshotBusyReason(ctx context.Context) (string, error) { return "", nil }
