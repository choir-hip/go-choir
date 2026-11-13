package updater

import (
	"os"
	"path/filepath"
	"strings"
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
	if err != nil || !acquired {
		f.Close()
		return nil, err
	}
	return func() { unlock(); f.Close() }, nil
}

// RestoreInFlight reports whether a restore holds root. A missing hold file or
// an unreadable one that cannot be tested reads as held only when it exists
// and cannot be opened (fail closed); a missing file is not held.
func RestoreInFlight(root string) bool {
	f, err := os.OpenFile(restoreHoldPath(filepath.Clean(strings.TrimSpace(root))), os.O_RDWR, 0)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	defer f.Close()
	unlock, acquired, err := lockFile(f, false)
	if err != nil || !acquired {
		return true
	}
	unlock()
	return false
}
