package agentcore

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/updater"
)

// errRestoreHoldUnavailable: the updater's restore hold could not be taken (a
// full, read-only or failing volume). A writer of `current` that can swap back
// to a prior release must refuse before it changes anything: without the hold
// the updater's prune (another process) could remove that release mid-restore.
var errRestoreHoldUnavailable = errors.New("the updater's restore hold is unavailable")

// lockCurrentRelease takes currentReleaseMu and the updater's restore hold, so
// the updater's prune skips while a restore may still swap back to the release
// it came from (updater.HoldRestore). It always returns the unlock, with the
// mutex held; err is non-nil when the hold could not be taken, in which case a
// swap-capable caller must unlock and refuse, and only a read-only caller (the
// boot surface read of a verified `current`) may go on.
func (rt *Runtime) lockCurrentRelease() (unlock func(), err error) {
	rt.currentReleaseMu.Lock()
	release := func() {}
	if root := strings.TrimSpace(rt.selfdevUpdaterRoot); root != "" {
		hold, holdErr := updater.HoldRestore(root)
		if holdErr != nil {
			err = fmt.Errorf("%w: %v", errRestoreHoldUnavailable, holdErr)
		} else {
			release = hold
		}
	}
	return func() {
		release()
		rt.currentReleaseMu.Unlock()
	}, err
}
