package agentcore

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/updater"
)

// PlannedRestartSelfDevelopmentRestore is the planned-restart reason a tape
// restore writes when the restored release execs a different binary than the
// one serving (P5 R1).
const PlannedRestartSelfDevelopmentRestore = "self_development_restore"

// restoreRestartDelay lets the restore's HTTP answer reach the caller before
// the runtime restarts.
var restoreRestartDelay = 2 * time.Second

// restoredExecDiffers reports whether the restored release execs a different
// binary than the one this process started as. The running identity is the
// release current/ held at startup (nil: no release, the base binary). A
// layered release execs its own build; a plain one execs the base. An
// unreadable restored release counts as different: a planned restart into
// current is safe, a layered binary left running after a restore is not (R1).
func restoredExecDiffers(running *effectiveReleaseIdentity, restored updater.ReleaseManifest, restoredErr error) bool {
	if restoredErr != nil {
		return true
	}
	runningLayered, runningCommit := false, ""
	if running != nil {
		runningLayered, runningCommit = running.Layered, strings.TrimSpace(running.CodeCommit)
	}
	restoredLayered := restored.LayeringEntrypoint != ""
	if runningLayered != restoredLayered {
		return true
	}
	return restoredLayered && runningCommit != strings.TrimSpace(restored.CodeCommit)
}

// restoreExecChanged reads the restored pinned release and compares it with
// the running one.
func (rt *Runtime) restoreExecChanged(restoredDigest string) bool {
	if rt == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return false
	}
	manifest, _, err := updater.ReadPinnedManifest(rt.selfdevUpdaterRoot, restoredDigest)
	return restoredExecDiffers(rt.selfdevStartupRelease, manifest, err)
}

// restartIntoCurrentAfterRestore writes the planned-restart marker, then asks
// the updater to restart the runtime into current. The marker stays on
// success (the restart reads it at boot) and is cleared on failure, so a
// later crash never reads as planned (R3).
func (rt *Runtime) restartIntoCurrentAfterRestore(ctx context.Context, target string) error {
	restart := rt.selfdevRestartCurrent
	if restart == nil {
		if rt.selfdevUpdater == nil {
			return errors.New("restore restart: no updater")
		}
		restart = rt.selfdevUpdater.RestartCurrent
	}
	clearMarker, err := rt.markPlannedRestart(PlannedRestartSelfDevelopmentRestore, target)
	if err != nil {
		return err
	}
	if err := restart(ctx); err != nil {
		clearMarker(err)
		return err
	}
	return nil
}

// scheduleRestoreRestart restarts into the restored release shortly after the
// restore answers.
func (rt *Runtime) scheduleRestoreRestart(target string) {
	go func() {
		time.Sleep(restoreRestartDelay)
		if err := rt.restartIntoCurrentAfterRestore(context.Background(), target); err != nil {
			log.Printf("restore: restart into the restored release %s: %v; the previous binary keeps serving until the next restart", target, err)
			return
		}
		log.Printf("restore: restart into the restored release requested (%s)", target)
	}()
}
