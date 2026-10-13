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

// RestartWorkKinds start or resume work: after a crash boot, an occurrence
// of one of these kinds recorded before the boot is interrupted_by_restart,
// never run. Cancels and fail-closed deadlines are absent: they close work
// and still apply across a restart, as does selfdev_materialization_retry,
// which finishes a planned apply (bounded by guestApplyAttemptLimit).
var RestartWorkKinds = map[string]bool{
	"initial_dispatch": true, "coagent_result": true, "channel_message": true,
	"owner_revision": true, "lifecycle_work_assigned": true,
	"delegated_assignment_spawn_deadline":   true,
	"fresh_mint_management_resume_deadline": true,
	// Autopaper's source schedule (W1c): a crash never resumes polling.
	"source_poll_due": true,
	// Cut 1 L3a: a record's wake starts a turn; unread records wait for the
	// next post-boot commit.
	"inbox_record": true,
}

// RestartWorkKind reports whether an occurrence of kind starts or resumes
// work.
func RestartWorkKind(kind string) bool { return RestartWorkKinds[kind] }

// preBootWorkWake reports whether a crash boot must interrupt this wake
// rather than deliver it. The projector re-drives a consumed wake under a
// fresh occurrence stamped now, which the actor handler's pre-boot gate
// cannot recognise, so the gate is applied here, on the wake's own
// CreatedAt (restart design v3 §9, finding 7).
func (rt *Runtime) preBootWorkWake(kind string, createdAt time.Time) bool {
	if rt == nil || rt.bootRestartPlanned || rt.bootedAt.IsZero() || createdAt.IsZero() {
		return false
	}
	return createdAt.Before(rt.bootedAt) && RestartWorkKind(kind)
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

// guestApplyAttemptLimit bounds how many times this guest starts one apply
// (a reason and target, such as a platform update id or a self-development
// operation id), across every boot. An apply that crashes the guest leaves
// the marker it wrote, so the next boot reads planned and the resume applies
// again; without a durable count that is a crash loop no host gate sees
// (restart design v3 §9, finding 5). Three covers the first attempt, the
// planned resume that finishes it, and one replay.
const guestApplyAttemptLimit = 3

// ErrGuestApplyAttemptsSpent refuses an apply this guest has already started
// guestApplyAttemptLimit times. The obligation stays where it is, visible,
// for an owner act; nothing retries it automatically.
var ErrGuestApplyAttemptsSpent = errors.New("guest apply attempts spent: this apply has already started the limit of times; an owner act is needed")

// GuestApplyAttemptsPath is the durable attempt count beside the runtime
// store.
func GuestApplyAttemptsPath(storePath string) string {
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		return ""
	}
	return filepath.Clean(storePath) + ".apply-attempts"
}

// spendGuestApplyAttempt durably counts one start of the apply named by
// reason and target, and refuses once the limit is spent. It fails closed:
// an unreadable or unwritable count refuses the apply. A runtime without a
// store path (no durable volume, so no marker either) has nothing to count
// across boots.
func spendGuestApplyAttempt(storePath, reason, target string) error {
	path := GuestApplyAttemptsPath(storePath)
	if path == "" {
		return nil
	}
	counts := map[string]int{}
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return fmt.Errorf("guest apply attempts: read: %w", err)
	default:
		if err := json.Unmarshal(raw, &counts); err != nil {
			return fmt.Errorf("guest apply attempts: decode: %w", err)
		}
	}
	key := reason + " " + target
	if counts[key] >= guestApplyAttemptLimit {
		return fmt.Errorf("%w (%s %s, %d starts)", ErrGuestApplyAttemptsSpent, reason, target, counts[key])
	}
	counts[key]++
	raw, err = json.Marshal(counts)
	if err != nil {
		return fmt.Errorf("guest apply attempts: encode: %w", err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("guest apply attempts: create: %w", err)
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return fmt.Errorf("guest apply attempts: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("guest apply attempts: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("guest apply attempts: close: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("guest apply attempts: install: %w", err)
	}
	return syncParentDir(path)
}

// markPlannedRestart spends one of the apply's attempts and announces that
// the apply about to run restarts this guest on purpose. It refuses the apply
// (and writes no marker) once the attempts are spent. The returned func
// clears the marker once the apply returns: the updater answers only a caller
// that is still alive, so the restart already happened (a resume replayed
// from the journal) or never will, and a marker left behind would read a
// later crash as planned (L1a-3a).
func (rt *Runtime) markPlannedRestart(reason, target string) (func(applyErr error), error) {
	storePath := ""
	if rt != nil {
		storePath = rt.cfg.StorePath
	}
	if err := spendGuestApplyAttempt(storePath, reason, target); err != nil {
		log.Printf("runtime: refuse %s %s: %v", reason, target, err)
		return func(error) {}, err
	}
	if err := WritePlannedRestartMarker(storePath, PlannedRestart{Reason: reason, Target: target}); err != nil {
		// Without the marker the restart reads as a crash and pending work is
		// interrupted, which is safe; the apply itself proceeds.
		log.Printf("runtime: %v (restart will be treated as a crash)", err)
		return func(error) {}, nil
	}
	return func(error) {
		if err := ClearPlannedRestartMarker(storePath); err != nil {
			log.Printf("runtime: clear planned restart marker after apply returned: %v", err)
		}
	}, nil
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
