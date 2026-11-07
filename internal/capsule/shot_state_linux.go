//go:build linux

package capsule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
)

// The state a candidate shot runs over (docs/design/source-only-freeze-and-
// candidate-shots-2026-11-06.md 3.3): the same floor check and the same
// owner-files snapshot an engineering capsule's copy uses, exported for the
// platform's shot worker, which owns its sandbox and has no capsule.

// ShotSandboxIDs are the uid and gid the shot sandbox runs as. Run as root
// (the guest), it is the capsule's unprivileged host id, so the hardlinked
// owner files, which are owned by someone else and not world-writable, are
// read-only to it (FM4). Run unprivileged (tests), it is the caller itself.
func ShotSandboxIDs() (uid, gid int) {
	if os.Geteuid() == 0 {
		return capsuleNamespaceHostID, capsuleNamespaceHostID
	}
	return os.Geteuid(), os.Getegid()
}

// ChownTreeNoFollow gives every entry of root (root included) to uid:gid,
// never following a link.
func ChownTreeNoFollow(root string, uid, gid int) error {
	return chownTreeNoFollow(root, uid, gid)
}

// CheckStateCopyFits refuses a copy that would push dir's volume under the
// state floor: a copy of estimate bytes takes the capsule copy factor and
// allowance, as writeComputerStoreCopy charges it.
func CheckStateCopyFits(dir string, estimate int64) error {
	need := int64(computerStoreCopyFactor*float64(estimate)) + computerStoreAllowance
	free, err := computerStoreFreeBytes(dir)
	if err != nil {
		return fmt.Errorf("free space unreadable: %w", err)
	}
	if free-need < StateFloorBytes {
		return fmt.Errorf("does not fit: the copy needs %d MiB (live store %d MiB), %d MiB free, %d MiB floor", need>>20, estimate>>20, free>>20, StateFloorBytes>>20)
	}
	return nil
}

// StateVolumeAboveFloor says dir's volume still has the state floor free.
func StateVolumeAboveFloor(dir string) error {
	free, err := computerStoreFreeBytes(dir)
	if err != nil {
		return fmt.Errorf("free space unreadable: %w", err)
	}
	if free < StateFloorBytes {
		return fmt.Errorf("the state volume fell under its %d MiB floor", StateFloorBytes>>20)
	}
	return nil
}

// BuildShotFilesLayer hardlinks the computer's files (the engineering
// capsule's snapshot: no mailbox, no source tree, no unreadable file) into
// dest, which must exist, with its directories owned by uid:gid. It holds the
// files write lock for the walk, as the capsule's layer does.
func BuildShotFilesLayer(ctx context.Context, filesRoot, sourceRoot, dest string, uid, gid int) (ComputerStateReceipt, error) {
	receipt := ComputerStateReceipt{}
	root, leaveOut := computerFilesLeaveOutFor(filesRoot, sourceRoot)
	if root == "" {
		return receipt, errors.New("no files root")
	}
	if info, err := os.Lstat(root); err != nil || !info.IsDir() {
		return receipt, errors.New("files root is unavailable")
	}
	if pathWithin(filepath.Clean(dest), root) || pathWithin(root, filepath.Clean(dest)) {
		return receipt, errors.New("files root and the shot directory overlap")
	}
	var walkErr error
	_ = ownerfiles.Exclusive(func() error {
		walkErr = snapshotFilesTreeOwned(ctx, root, dest, leaveOut, &receipt, uid, gid, true)
		return nil
	})
	if walkErr != nil {
		_ = removeLinkedTree(dest)
		return receipt, fmt.Errorf("snapshot: %w", walkErr)
	}
	receipt.Files = true
	return receipt, nil
}
