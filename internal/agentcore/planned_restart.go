package agentcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Owner rule (AGENTS.md "Restarts End Work (Crash) Or Resume It"): a crash
// restart never resumes work; a planned update restart may. The guest writes
// this marker immediately before a restart it causes on purpose and the next
// boot consumes it once. A boot without a fresh marker is a crash.

// plannedRestartMaxAge bounds how long a marker stays meaningful. A marker
// whose restart never happened must not turn a later crash into a planned
// boot.
const plannedRestartMaxAge = 15 * time.Minute

// PlannedRestart describes an intentional restart.
type PlannedRestart struct {
	Reason      string    `json:"reason"`
	Target      string    `json:"target,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
}

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
func WritePlannedRestartMarker(storePath string, marker PlannedRestart) error {
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return errors.New("planned restart marker: store path is empty")
	}
	if marker.RequestedAt.IsZero() {
		marker.RequestedAt = time.Now().UTC()
	}
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

// ClearPlannedRestartMarker removes the marker, e.g. when the restart it
// announced did not happen.
func ClearPlannedRestartMarker(storePath string) error {
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("planned restart marker: remove: %w", err)
	}
	return syncParentDir(path)
}

// ConsumePlannedRestartMarker reads and removes the marker. It reports a
// planned restart only for a readable marker younger than
// plannedRestartMaxAge; anything else is a crash boot. The marker is removed
// before the caller resumes work, so a resume that crashes the guest leads to
// a markerless (crash) boot: one resume per planned restart.
func ConsumePlannedRestartMarker(storePath string, now time.Time) (PlannedRestart, bool) {
	path := PlannedRestartMarkerPath(storePath)
	if path == "" {
		return PlannedRestart{}, false
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return PlannedRestart{}, false
	}
	if clearErr := ClearPlannedRestartMarker(storePath); clearErr != nil {
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
	if age := now.Sub(marker.RequestedAt); age < 0 || age > plannedRestartMaxAge {
		log.Printf("agentcore: planned restart marker from %s is stale; treating boot as a crash", marker.RequestedAt.Format(time.RFC3339))
		return PlannedRestart{}, false
	}
	return marker, true
}

func syncParentDir(path string) error {
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("planned restart marker: open dir: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("planned restart marker: sync dir: %w", err)
	}
	return nil
}

// WithBootRestart records how this boot began (consumed by the boot path
// before any actor runs).
func WithBootRestart(marker PlannedRestart, planned bool) RuntimeOption {
	return func(rt *Runtime) {
		rt.bootRestart = marker
		rt.bootRestartPlanned = planned
	}
}

// BootWasPlannedRestart reports whether this boot followed a planned restart.
func (rt *Runtime) BootWasPlannedRestart() (PlannedRestart, bool) {
	if rt == nil {
		return PlannedRestart{}, false
	}
	return rt.bootRestart, rt.bootRestartPlanned
}

// markPlannedRestart announces that the apply about to run restarts this
// guest on purpose. The returned func clears the marker when the apply
// returned without restarting.
func (rt *Runtime) markPlannedRestart(reason, target string) func(applyErr error) {
	storePath := ""
	if rt != nil {
		storePath = rt.cfg.StorePath
	}
	if err := WritePlannedRestartMarker(storePath, PlannedRestart{Reason: reason, Target: target}); err != nil {
		// Without the marker the restart reads as a crash and pending work is
		// interrupted, which is safe; the apply itself proceeds.
		log.Printf("runtime: %v (restart will be treated as a crash)", err)
		return func(error) {}
	}
	return func(applyErr error) {
		if applyErr == nil {
			return
		}
		if err := ClearPlannedRestartMarker(storePath); err != nil {
			log.Printf("runtime: clear planned restart marker after failed apply: %v", err)
		}
	}
}
