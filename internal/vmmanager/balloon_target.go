package vmmanager

// Guest memory caps through the balloon (density D6 S6; owner, 2026-11-01:
// "More disk reads, less memory usage"). A capped guest has the balloon
// inflated to base memory minus the cap, so the guest gives up its page
// cache (then anonymous memory) to the device and Firecracker drops those
// pages from the host. deflate_on_oom is always set on the device
// (balloon.go): a guest out of memory takes balloon pages back before its
// OOM killer runs.
//
// The launch config stays amount_mib 0, so the snapshot launch-config hash
// does not move; the amount is a runtime target (PATCH /balloon) that vmctl
// reconciles every interval, the one writer of balloon targets (RES-09).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// GuestMemoryCapFloorMiB is the smallest cap honored: below it a cap is
// refused, never clamped, so a typo cannot squeeze a guest to nothing.
const GuestMemoryCapFloorMiB = 1024

// balloonTargetTimeout bounds one reconcile (GET and PATCH together).
var balloonTargetTimeout = 5 * time.Second

// BalloonTargetResult is one reconcile of a running VM's balloon.
type BalloonTargetResult struct {
	VMID      string
	BaseMiB   int
	CapMiB    int // 0: uncapped
	BeforeMiB int // the device's amount before
	AfterMiB  int // the amount asked for (== BeforeMiB when unchanged)
	Changed   bool
}

// ErrBalloonBusy means another operation holds the VM; the next pass retries.
var ErrBalloonBusy = errors.New("vm operation in progress")

// SetGuestMemoryCap sets a running VM's balloon so the guest keeps capMiB of
// its base memory (capMiB 0: no cap, amount 0). It reads the device first and
// writes only on a difference. A VM that is not running, has no balloon or
// is busy returns an error and is left alone.
func (m *Manager) SetGuestMemoryCap(vmID string, capMiB int) (BalloonTargetResult, error) {
	res := BalloonTargetResult{VMID: vmID, CapMiB: capMiB}
	if capMiB != 0 && capMiB < GuestMemoryCapFloorMiB {
		return res, fmt.Errorf("cap %d MiB under the %d MiB floor", capMiB, GuestMemoryCapFloorMiB)
	}
	unlock, ok := m.tryLockVMOperation(vmID)
	if !ok {
		return res, ErrBalloonBusy
	}
	defer unlock()
	sock, base, err := m.runningAPISocket(vmID)
	if err != nil {
		return res, err
	}
	res.BaseMiB = base
	want := 0
	if capMiB > 0 && base > capMiB {
		want = base - capMiB
	}
	ctx, cancel := context.WithTimeout(context.Background(), balloonTargetTimeout)
	defer cancel()
	api := newFCAPIClient(sock)
	cur, err := api.Balloon(ctx)
	if err != nil {
		return res, fmt.Errorf("read balloon: %w", err)
	}
	res.BeforeMiB, res.AfterMiB = cur.AmountMiB, cur.AmountMiB
	if cur.AmountMiB == want {
		return res, nil
	}
	if err := api.PatchBalloonAmount(ctx, want); err != nil {
		return res, fmt.Errorf("set balloon to %d MiB: %w", want, err)
	}
	res.AfterMiB, res.Changed = want, true
	return res, nil
}

// PatchBalloonAmount is PATCH /balloon (swagger BalloonUpdate).
func (c *fcAPIClient) PatchBalloonAmount(ctx context.Context, amountMiB int) error {
	return c.do(ctx, http.MethodPatch, "/balloon", map[string]int{"amount_mib": amountMiB})
}
