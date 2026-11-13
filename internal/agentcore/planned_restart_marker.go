package agentcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// The planned-restart marker has ONE owner: this file. Every write, clear,
// reason read, pending check and consume of the marker file goes through the
// functions here, under one mutex, so "read the reason and remove it only if it
// is the host's" is a single atomic step and no writer can land between a check
// and the delete that depends on it. Three review rounds each found another
// unserialised interleaving on this file; the cure is that no other code
// touches it (TestPlannedRestartMarkerIsTouchedOnlyThroughItsOwner fails the
// build if any does).
//
// The owner enforces what a marker MEANS, not only who may touch the file:
//
//	Rule 1, the marker belongs to whoever wrote it. Clear is reason-scoped
//	(clearPlannedRestartMarkerIfReason): a guest cleanup clears only its own
//	reason, the host's withdrawal only PlannedRestartPlatformRefresh. Write
//	needs a reason, and the host's write never lands over a guest marker (a
//	guest restart is already coming; errGuestRestartPending). The guest's
//	write may replace the host's: the guest's restart is the one that
//	happens, and the boot is planned either way, so no crash boot results
//	(refusing it would leave the guest's own apply without a marker, which
//	is one). There is no unconditional clear outside the boot's consume.
//
//	Rule 2, nothing that protects a coming kill is released while a marker is
//	on disk. That is releaseRestartProtections (planned_restart_release.go).
//
// Lock order, stated once and followed everywhere:
//
//	plannedRestartReqMu (the host's request/withdrawal)
//	  -> hostRestartTelemetryMu
//	    -> plannedRestartMarkerMu (this file; a leaf: nothing is taken inside it)
//
// A path that needs only the marker (markPlannedRestart for platform_update and
// a self-development apply, the boot's consume) takes plannedRestartMarkerMu
// alone, which is always safe because it is last.
var plannedRestartMarkerMu sync.Mutex

// plannedRestartMarkerCheckedHook is a test-only seam, not parallel-safe (a
// package global): called, with the marker mutex held, between
// clearPlannedRestartMarkerIfReason's reason check and its delete.
var plannedRestartMarkerCheckedHook func()

// PlannedRestartMarkerPath is the marker beside the runtime store, on the
// same durable volume.
func PlannedRestartMarkerPath(storePath string) string {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return ""
	}
	return filepath.Clean(storePath) + ".planned-restart"
}

// WritePlannedRestartMarker durably records that the next boot is planned.
//
// It refuses a marker without a reason, and the host's marker
// (PlannedRestartPlatformRefresh) while a guest marker is on disk
// (errGuestRestartPending).
func WritePlannedRestartMarker(storePath string, marker PlannedRestart) error {
	if strings.TrimSpace(marker.Reason) == "" {
		return errors.New("planned restart marker: a reason is required")
	}
	plannedRestartMarkerMu.Lock()
	defer plannedRestartMarkerMu.Unlock()
	if marker.Reason == PlannedRestartPlatformRefresh {
		if m, ok := readFreshMarkerLocked(storePath, time.Now()); ok && m.Reason != "" && m.Reason != PlannedRestartPlatformRefresh {
			return fmt.Errorf("%w (%s)", errGuestRestartPending, m.Reason)
		}
	}
	return writePlannedRestartMarkerLocked(storePath, marker)
}

// errGuestRestartPending refuses the host's marker while a marker the guest
// wrote for its own restart is on disk.
var errGuestRestartPending = errors.New("guest_restart_pending: the guest has a restart of its own pending")

// unnamedMarkerReason is what a marker without a reason (one from before the
// write refused them) is reported as when it is kept.
const unnamedMarkerReason = "(unnamed)"

// readFreshMarkerLocked returns the marker on disk if it is readable and could
// still plan a boot. The caller holds plannedRestartMarkerMu.
func readFreshMarkerLocked(storePath string, now time.Time) (PlannedRestart, bool) {
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return PlannedRestart{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return PlannedRestart{}, false
	}
	var marker PlannedRestart
	if json.Unmarshal(raw, &marker) != nil || marker.RequestedAt.IsZero() {
		return PlannedRestart{}, false
	}
	if now.Sub(marker.RequestedAt) > plannedRestartMaxAge {
		return PlannedRestart{}, false
	}
	return marker, true
}

// guestRestartPendingReason names the guest marker on disk that the host's
// mark must not replace, or "".
func guestRestartPendingReason(storePath string) string {
	plannedRestartMarkerMu.Lock()
	defer plannedRestartMarkerMu.Unlock()
	if m, ok := readFreshMarkerLocked(storePath, time.Now()); ok && m.Reason != "" && m.Reason != PlannedRestartPlatformRefresh {
		return m.Reason
	}
	return ""
}

// clearPlannedRestartMarkerIfReason removes the marker only if its reason is
// one of reasons, as one atomic step with the read. cleared reports that the
// file was removed; keptReason names the marker left in place (the reason, or
// unnamedMarkerReason for one without), so "removed" and "kept" are never
// confused. No marker is (false, "", nil). A marker that cannot be read or
// parsed belongs to nobody and is removed.
func clearPlannedRestartMarkerIfReason(storePath string, reasons ...string) (cleared bool, keptReason string, err error) {
	plannedRestartMarkerMu.Lock()
	defer plannedRestartMarkerMu.Unlock()
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return false, "", nil
	}
	raw, rerr := os.ReadFile(path)
	if errors.Is(rerr, os.ErrNotExist) {
		return false, "", nil
	}
	if rerr == nil {
		var marker PlannedRestart
		if json.Unmarshal(raw, &marker) == nil {
			if plannedRestartMarkerCheckedHook != nil {
				plannedRestartMarkerCheckedHook()
			}
			if !slices.Contains(reasons, marker.Reason) || marker.Reason == "" {
				if marker.Reason == "" {
					return false, unnamedMarkerReason, nil
				}
				return false, marker.Reason, nil
			}
		}
	}
	if err := clearPlannedRestartMarkerLocked(storePath); err != nil {
		return false, "", err
	}
	return true, "", nil
}

func writePlannedRestartMarkerLocked(storePath string, marker PlannedRestart) error {
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return errors.New("planned restart marker: store path is empty")
	}
	if marker.RequestedAt.IsZero() {
		marker.RequestedAt = time.Now().UTC()
	}
	marker.WorkStateEpoch = CurrentWorkStateEpoch
	raw, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("planned restart marker: encode: %w", err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("planned restart marker: create: %w", err)
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return fmt.Errorf("planned restart marker: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("planned restart marker: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("planned restart marker: close: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("planned restart marker: install: %w", err)
	}
	return syncParentDir(path)
}

func clearPlannedRestartMarkerLocked(storePath string) error {
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("planned restart marker: remove: %w", err)
	}
	return syncParentDir(path)
}

// plannedRestartMarkerPending reports whether a marker that could still plan a
// boot is on disk: present and no older than plannedRestartMaxAge. A marker
// that cannot be read or parsed counts by its file time (fail closed while it
// is fresh). No store path, or no marker, is not pending.
func plannedRestartMarkerPending(storePath string, now time.Time) bool {
	plannedRestartMarkerMu.Lock()
	defer plannedRestartMarkerMu.Unlock()
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return false
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if err == nil {
		var marker PlannedRestart
		if json.Unmarshal(raw, &marker) == nil && !marker.RequestedAt.IsZero() {
			return now.Sub(marker.RequestedAt) <= plannedRestartMaxAge
		}
	}
	if info, statErr := os.Stat(path); statErr == nil {
		return now.Sub(info.ModTime()) <= plannedRestartMaxAge
	}
	return true
}

// PlannedRestartMarkerPending reports, before anything consumes it, whether
// this boot is planned: a marker no older than plannedRestartMaxAge at now.
// Boot uses it to swap in a staged compact copy before the store opens
// (store.RebaseOntoCompactCopy).
func PlannedRestartMarkerPending(storePath string, now time.Time) bool {
	return plannedRestartMarkerPending(storePath, now)
}

// PlannedRestartSparesPausedCapsules reports, without consuming the marker,
// whether this boot follows a planned restart whose reason is a platform
// update and may resume paused capsules (sandbox pause design, Decision 6
// and the r3 reason-aware spare). It is evaluated at processStartedAt, the
// instant ConsumePlannedRestartMarker judges the marker at, and answers
// exactly as that consumption will: a readable marker of this binary's
// work-state epoch, no older than plannedRestartMaxAge, with the platform
// update reason. A restore-reason marker (PlannedRestartSelfDevelopmentRestore),
// a crash, an unreadable marker or a stale one spares nothing.
func PlannedRestartSparesPausedCapsules(storePath string, now time.Time) bool {
	plannedRestartMarkerMu.Lock()
	defer plannedRestartMarkerMu.Unlock()
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var marker PlannedRestart
	if json.Unmarshal(raw, &marker) != nil || marker.RequestedAt.IsZero() {
		return false
	}
	if marker.WorkStateEpoch != CurrentWorkStateEpoch {
		return false
	}
	if age := now.Sub(marker.RequestedAt); age < 0 || age > plannedRestartMaxAge {
		return false
	}
	return marker.Reason == PlannedRestartPlatformRefresh
}

// ConsumePlannedRestartMarker reads and removes the marker. It reports a
// planned restart only for a readable marker younger than
// plannedRestartMaxAge at now; anything else is a crash boot. The boot passes
// its process start as now, so slow boot phases before consumption do not
// count against the age. The marker is removed
// before the caller resumes work, so a resume that crashes the guest leads to
// a markerless (crash) boot: one resume per planned restart.
func ConsumePlannedRestartMarker(storePath string, now time.Time) (PlannedRestart, bool) {
	plannedRestartMarkerMu.Lock()
	defer plannedRestartMarkerMu.Unlock()
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return PlannedRestart{}, false
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return PlannedRestart{}, false
	}
	if clearErr := clearPlannedRestartMarkerLocked(storePath); clearErr != nil {
		// A marker that cannot be removed could be reused by a later crash;
		// treat this boot as a crash rather than risk a resume loop.
		log.Printf("agentcore: %v; treating boot as a crash", clearErr)
		return PlannedRestart{}, false
	}
	if err != nil {
		log.Printf("agentcore: planned restart marker unreadable: %v; treating boot as a crash", err)
		return PlannedRestart{}, false
	}
	var marker PlannedRestart
	if err := json.Unmarshal(raw, &marker); err != nil || marker.RequestedAt.IsZero() {
		log.Printf("agentcore: planned restart marker invalid; treating boot as a crash")
		return PlannedRestart{}, false
	}
	if marker.WorkStateEpoch != CurrentWorkStateEpoch {
		log.Printf("agentcore: planned restart marker has work-state epoch %d, this binary reads %d; treating boot as a crash", marker.WorkStateEpoch, CurrentWorkStateEpoch)
		return PlannedRestart{}, false
	}
	if age := now.Sub(marker.RequestedAt); age < 0 || age > plannedRestartMaxAge {
		log.Printf("agentcore: planned restart marker from %s is stale; treating boot as a crash", marker.RequestedAt.Format(time.RFC3339))
		return PlannedRestart{}, false
	}
	return marker, true
}

func syncParentDir(path string) error {
	if err := syncPath(filepath.Dir(path)); err != nil {
		return fmt.Errorf("planned restart marker: sync dir: %w", err)
	}
	return nil
}
