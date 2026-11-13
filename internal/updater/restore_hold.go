package updater

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// A restore (tape restore, base restore, package migration, the boot baseline
// import and frontend follow) swaps `current` without an apply, so no journal
// names what it came from, and a failed restore swaps back to it. The prune
// keeps only current and the journal's predecessor, so without a mark of its
// own the prune could remove that release inside the restore. HoldRestore is
// the mark: an advisory lock on operations/.restore-in-flight, held for the
// whole restore including the swap-back window. The lock is the state the
// restore leaves on disk while in flight; a dead process releases it. The name
// has no ".lock" suffix, so UnsettledApply (which the runtime's own follow step
// consults while it holds this) does not see it; the prune checks it itself.
const restoreHoldName = ".restore-in-flight"

func restoreHoldPath(root string) string {
	return filepath.Join(operationsDir(root), restoreHoldName)
}

// HoldRestore marks a restore in flight under root until the returned release
// is called. It waits for another hold to end.
func HoldRestore(root string) (release func(), err error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if err := os.MkdirAll(operationsDir(root), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(restoreHoldPath(root), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	unlock, acquired, err := lockFile(f, true)
	if err != nil {
		f.Close()
		return nil, err
	}
	if !acquired {
		f.Close()
		return nil, errors.New("updater: restore hold not acquired")
	}
	return func() { unlock(); f.Close() }, nil
}

// tryHoldRestore takes the hold without waiting, for the prune: it holds the
// updater's mutex, and a restore holds the hold while it calls Apply, which
// waits for that mutex, so the prune must never wait for the hold. acquired is
// false when a restore holds it. When the hold file cannot be created because
// the volume is full, no restore can take the hold either (each refuses
// without it), so the prune goes ahead: it is what frees the space.
func tryHoldRestore(root string) (release func(), acquired bool, err error) {
	root = filepath.Clean(strings.TrimSpace(root))
	f, err := os.OpenFile(restoreHoldPath(root), os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		if mkErr := os.MkdirAll(operationsDir(root), 0o700); mkErr != nil {
			return nil, false, mkErr
		}
		f, err = os.OpenFile(restoreHoldPath(root), os.O_CREATE|os.O_RDWR, 0o600)
		if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
			return func() {}, true, nil
		}
	}
	if err != nil {
		return nil, false, err
	}
	unlock, got, err := lockFile(f, false)
	if err != nil {
		f.Close()
		return nil, false, err
	}
	if !got {
		f.Close()
		return nil, false, nil
	}
	return func() { unlock(); f.Close() }, true, nil
}

// RestoreInFlight reports whether a restore holds root, for diagnostics; the
// answer can be stale the moment it is returned, so nothing may act on it (the
// prune uses tryHoldRestore). A missing hold file is not held.
func RestoreInFlight(root string) bool {
	release, acquired, err := tryHoldRestore(root)
	if err != nil {
		return true
	}
	if acquired {
		release()
	}
	return !acquired
}
