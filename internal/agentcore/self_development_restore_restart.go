package agentcore

import (
	"context"
	"errors"
	"fmt"
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
	return restoredExecDiffers(rt.startupManifest().Release, manifest, err)
}

// restoreRestartIdlePoll and restoreRestartIdleWindow pace a restore restart
// deferred by a busy computer: it restarts at the first idle moment within
// the window.
var (
	restoreRestartIdlePoll   = 15 * time.Second
	restoreRestartIdleWindow = time.Hour
)

func (rt *Runtime) restoreRestarter() (func(context.Context) error, error) {
	if rt.selfdevRestartCurrent != nil {
		return rt.selfdevRestartCurrent, nil
	}
	if rt.selfdevUpdater == nil {
		return nil, errors.New("restore restart: no updater")
	}
	return rt.selfdevUpdater.RestartCurrent, nil
}

// writeRestoreRestartMarker writes the planned-restart marker for a restore
// restart. A restore is not an apply: it spends no apply attempt (R4), so a
// fifth restore to the same checkpoint restarts like the first.
//
// The write is one work occurrence through the hibernate fence, taken without
// waiting (D2 S5; review 331, both seats): a closed fence refuses it with
// errHibernateFenced before anything is written, and the fence cannot close
// until the write returns. A marker written between the fence and the pause
// would be captured in the snapshot.
func (rt *Runtime) writeRestoreRestartMarker(target string) error {
	release, ok := rt.hibernate.tryEnter()
	if !ok {
		return errHibernateFenced
	}
	defer release()
	return WritePlannedRestartMarker(rt.cfg.StorePath, PlannedRestart{Reason: PlannedRestartSelfDevelopmentRestore, Target: target})
}

func (rt *Runtime) clearRestoreRestartMarker() {
	if err := ClearPlannedRestartMarker(rt.cfg.StorePath); err != nil {
		log.Printf("restore: clear planned restart marker: %v", err)
	}
}

// restartIntoCurrentAfterRestore writes the planned-restart marker, then asks
// the updater to restart the runtime into current. The marker stays on
// success (the restart reads it at boot) and is cleared on failure, so a
// later crash never reads as planned (R3).
func (rt *Runtime) restartIntoCurrentAfterRestore(ctx context.Context, target string) error {
	restart, err := rt.restoreRestarter()
	if err != nil {
		return err
	}
	if err := rt.writeRestoreRestartMarker(target); err != nil {
		return err
	}
	if err := restart(ctx); err != nil {
		rt.clearRestoreRestartMarker()
		return err
	}
	return nil
}

// restoreRestartBusy reports why a restart must wait (running runs or
// self-development operations mid-transaction), or "" when idle (R5).
func (rt *Runtime) restoreRestartBusy(ctx context.Context) string {
	work, err := rt.plannedRestartBusy(ctx, true)
	switch {
	case err != nil:
		return "busy: self-development operations are unreadable: " + err.Error()
	case work.Running > 0 || work.unsafe():
		return fmt.Sprintf("busy: %d running runs, %d self-development operations mid-transaction, %d live capsules", work.Running, work.SelfdevActive, work.LiveCapsules)
	}
	return ""
}

// requestRestoreRestart asks for the restart into the restored release.
// scheduled is true only once the computer is idle and the planned-restart
// marker is durable; the restart follows shortly after the restore answers
// (R6). A busy computer answers deferred and restarts at its next idle
// moment (R5); a marker that cannot be written answers deferred with the
// reason and never restarts (a markerless restart would read as a crash).
func (rt *Runtime) requestRestoreRestart(ctx context.Context, target string) (scheduled bool, deferred string) {
	if _, err := rt.restoreRestarter(); err != nil {
		return false, err.Error()
	}
	if why := rt.restoreRestartBusy(ctx); why != "" {
		go rt.restartRestoredWhenIdle(target)
		return false, why + "; the restart into the restored release waits for the next idle moment"
	}
	if err := rt.writeRestoreRestartMarker(target); err != nil {
		if errors.Is(err, errHibernateFenced) {
			// Being hibernated: wait for the fence to lift, as for a busy computer.
			go rt.restartRestoredWhenIdle(target)
			return false, "the guest is being hibernated; the restart into the restored release waits for the next idle moment"
		}
		log.Printf("restore: %v; no restart into the restored release", err)
		return false, "the planned-restart marker could not be written (" + err.Error() + "); the previous binary keeps serving until the next restart"
	}
	go func() {
		time.Sleep(restoreRestartDelay)
		// A run may have started while the answer went out.
		if why := rt.restoreRestartBusy(context.Background()); why != "" {
			rt.clearRestoreRestartMarker()
			rt.restartRestoredWhenIdle(target)
			return
		}
		rt.restartRestored(target)
	}()
	return true, ""
}

// restartRestored calls the updater with the marker already on disk.
func (rt *Runtime) restartRestored(target string) {
	restart, err := rt.restoreRestarter()
	if err == nil {
		err = restart(context.Background())
	}
	if err != nil {
		rt.clearRestoreRestartMarker()
		log.Printf("restore: restart into the restored release %s: %v; the previous binary keeps serving until the next restart", target, err)
		return
	}
	log.Printf("restore: restart into the restored release requested (%s)", target)
}

// restartRestoredWhenIdle waits for an idle computer, then restarts into the
// restored release under the planned marker.
func (rt *Runtime) restartRestoredWhenIdle(target string) {
	deadline := time.Now().Add(restoreRestartIdleWindow)
	for time.Now().Before(deadline) {
		time.Sleep(restoreRestartIdlePoll)
		if rt.restoreRestartBusy(context.Background()) != "" {
			continue
		}
		if err := rt.writeRestoreRestartMarker(target); err != nil {
			if errors.Is(err, errHibernateFenced) {
				continue // being hibernated: try again at the next poll
			}
			log.Printf("restore: %v; no restart into the restored release %s", err, target)
			return
		}
		rt.restartRestored(target)
		return
	}
	log.Printf("restore: the computer stayed busy for %s; no restart into the restored release %s; the previous binary keeps serving until the next restart", restoreRestartIdleWindow, target)
}
