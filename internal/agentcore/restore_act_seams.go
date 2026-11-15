package agentcore

// Seams of the restore install protocol (restore_act.go). Each is nil in
// production. They exist so that a test can stop the real engine at a named
// point, fail one durable write, fail the reopen, or stop the boot repair
// between its steps; none changes what the code does when it is nil.

// restoreInstallCrashHook, when set, is called at each named point of the
// install: after_prepared, after_release_restaged, after_flip_started,
// mid_flip, before_flip_completed, after_flip_completed, after_reopen. A test
// child kills itself there.
var restoreInstallCrashHook func(point string)

func restoreInstallCrash(point string) {
	if hook := restoreInstallCrashHook; hook != nil {
		hook(point)
	}
}

// restoreInstallWriteFault, when set, fails the durable write of the named
// install phase (prepared, release_restaged, flip_started, flip_completed).
var restoreInstallWriteFault func(phase string) error

// restoreReopenFault, when set, fails the reopen of the flipped store.
var restoreReopenFault func() error

// restoreRepairStepHook, when set, is called before each step of the boot
// repair (undo, release, close); returning true stops the repair there, as a
// kill would.
var restoreRepairStepHook func(step string) bool

func restoreRepairStops(step string) bool {
	hook := restoreRepairStepHook
	return hook != nil && hook(step)
}

// restoreDirSyncHook, when set, is told each directory the install syncs.
var restoreDirSyncHook func(dir string)
