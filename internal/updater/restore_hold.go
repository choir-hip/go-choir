package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
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

// restoreHoldTimeout bounds how long HoldRestore waits for the hold. A holder
// is a restore (seconds to a minute) or one prune chunk, which deletes one
// entry: at worst a few GiB of small files on a slow volume, so a minute is
// well past a healthy holder. A holder stuck longer (storage that does not
// answer) must not wedge a boot or a restore silently: the caller gets
// ErrRestoreHoldTimeout, a swap-capable restore refuses, and the boot surface
// reads the verified `current` read-only.
var restoreHoldTimeout = time.Minute

const restoreHoldPoll = 25 * time.Millisecond

// ErrRestoreHoldTimeout: the hold stayed taken for restoreHoldTimeout.
var ErrRestoreHoldTimeout = errors.New("updater: the restore hold stayed taken")

// HoldRestore marks a restore in flight under root until the returned release
// is called. It retries without blocking until restoreHoldTimeout or ctx ends,
// and always creates the hold file, which tryHoldRestore's full-volume rule
// relies on.
func HoldRestore(ctx context.Context, root string) (release func(), err error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if err := os.MkdirAll(operationsDir(root), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(restoreHoldPath(root), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(restoreHoldTimeout)
	for {
		unlock, acquired, lockErr := lockFile(f, false)
		if lockErr != nil {
			f.Close()
			return nil, lockErr
		}
		if acquired {
			return func() { unlock(); f.Close() }, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, ErrRestoreHoldTimeout
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(restoreHoldPoll):
		}
	}
}

// tryHoldRestore takes the hold without waiting, for the prune: it holds the
// updater's mutex, and a restore holds the hold while it calls Apply, which
// waits for that mutex, so the prune must never wait for the hold. acquired is
// false when a restore holds it. When the hold file does not exist and cannot be
// created because the volume is full, no restore can take the hold either
// (HoldRestore always creates the file, and a swap-capable restore refuses
// without the hold), so the prune goes ahead: it is what frees the space. That
// is only sound while HoldRestore creates the file; a hold that did not create
// it would break this.
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
