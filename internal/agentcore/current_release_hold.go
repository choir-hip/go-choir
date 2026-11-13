package agentcore

import (
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/updater"
)

// lockCurrentRelease takes currentReleaseMu and, for a writer of `current` that
// is not an apply (tape restore, base restore, package migration, the boot
// baseline import and follow), the updater's restore hold, so the updater's
// prune (another process) skips while a restore may still swap back to the
// release it came from (updater.HoldRestore). The returned func releases both.
func (rt *Runtime) lockCurrentRelease() func() {
	rt.currentReleaseMu.Lock()
	release := func() {}
	if root := strings.TrimSpace(rt.selfdevUpdaterRoot); root != "" {
		if hold, err := updater.HoldRestore(root); err != nil {
			log.Printf("runtime: restore hold unavailable, a prune could overlap this restore: %v", err)
		} else {
			release = hold
		}
	}
	return func() {
		release()
		rt.currentReleaseMu.Unlock()
	}
}
