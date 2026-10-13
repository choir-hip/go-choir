package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// Owner rule (AGENTS.md "Restarts End Work (Crash) Or Resume It"): a crash
// restart never resumes work; a planned update restart may. The guest writes
// this marker immediately before a restart it causes on purpose and the next
// boot consumes it once. A boot without a fresh marker is a crash.

// plannedRestartMaxAge bounds how long a marker stays meaningful. A marker
// whose restart never happened must not turn a later crash into a planned
// boot.
const plannedRestartMaxAge = 15 * time.Minute

// CurrentWorkStateEpoch is the work-state schema this binary reads (decision
// D7 of Cut 1). Cut 1 made records the work state and bumped it to 1: a
// marker written before the cut has no epoch, so the cutover boot is a
// crash boot and nothing resumes into the new schema.
const CurrentWorkStateEpoch = 1

// PlannedRestart describes an intentional restart.
type PlannedRestart struct {
	Reason         string    `json:"reason"`
	Target         string    `json:"target,omitempty"`
	RequestedAt    time.Time `json:"requested_at"`
	WorkStateEpoch int       `json:"work_state_epoch,omitempty"`
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

// PlannedRestartPlatformRefresh is the reason the host records when it
// refreshes this guest onto a new platform build for a deploy.
const PlannedRestartPlatformRefresh = "platform update"

// plannedRestartBusy counts the work a restart must not cut: running runs
// and self-development operations mid-transaction (the states the idle sweep
// also treats as busy). It fails closed: an unreadable operation list is an
// error, not idle.
func (rt *Runtime) plannedRestartBusy(ctx context.Context) (running, selfdevActive int, err error) {
	running = rt.RunningCount()
	if rt.selfdevOperations == nil {
		return running, 0, nil
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" {
		return running, 0, nil
	}
	ops, err := rt.selfdevOperations.ListByStates(ctx, computerID,
		selfdev.StateExecuting, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateRollbackPending)
	if err != nil {
		return running, 0, err
	}
	return running, len(ops), nil
}

type plannedRestartRequest struct {
	Target string `json:"target"`
}

type plannedRestartBusyResponse struct {
	Error            string `json:"error"`
	Outcome          string `json:"outcome"`
	RunningRuns      int    `json:"running_runs"`
	SelfdevActiveOps int    `json:"selfdev_active_operations"`
}

// HandleInternalPlannedRestart serves /internal/runtime/planned-restart, the
// host's request to restart this guest onto a new platform build.
//
//   - POST {"target": "<build>"} writes the planned-restart marker (reason
//     "platform update") and answers 200 only once it is durable. A busy
//     guest answers 409 with outcome "busy" and writes nothing, so the host
//     leaves it for its next idle moment.
//   - DELETE clears the marker when the host did not restart after all.
//
// Host-peer only, like the other /internal/runtime routes. The marker carries
// this binary's work-state epoch, so a boot onto a binary of another epoch is
// still a crash boot (D7).
func (h *APIHandler) HandleInternalPlannedRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "planned restart surface unavailable"})
		return
	}
	storePath := h.rt.cfg.StorePath
	if r.Method == http.MethodDelete {
		if err := ClearPlannedRestartMarker(storePath); err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
		return
	}
	var request plannedRestartRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid planned restart request"})
		return
	}
	target := strings.TrimSpace(request.Target)
	if target == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "target is required"})
		return
	}
	running, selfdevActive, err := h.rt.plannedRestartBusy(r.Context())
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "planned restart: cannot read self-development operations: " + err.Error()})
		return
	}
	if running > 0 || selfdevActive > 0 {
		writeAPIJSON(w, http.StatusConflict, plannedRestartBusyResponse{
			Error:       "guest is busy; restart deferred to its next idle moment",
			Outcome:     "busy",
			RunningRuns: running, SelfdevActiveOps: selfdevActive,
		})
		return
	}
	marker := PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: target, RequestedAt: time.Now().UTC()}
	if err := WritePlannedRestartMarker(storePath, marker); err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	marker.WorkStateEpoch = CurrentWorkStateEpoch
	log.Printf("runtime: host requested a planned restart (%s %s)", marker.Reason, marker.Target)
	writeAPIJSON(w, http.StatusOK, marker)
}
