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
	"github.com/yusefmosiah/go-choir/internal/types"
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
// crash boot and nothing resumes into the new schema. C1e B1 bumped it to 2:
// engineering assignments moved onto records (bindings name conversations,
// not work items), so the first B1 boot is a crash boot and every
// assignment opened under the packet schema ends interrupted_by_restart.
const CurrentWorkStateEpoch = 2

// PlannedRestart describes an intentional restart.
type PlannedRestart struct {
	Reason         string    `json:"reason"`
	Target         string    `json:"target,omitempty"`
	RequestedAt    time.Time `json:"requested_at"`
	WorkStateEpoch int       `json:"work_state_epoch,omitempty"`
	// TelemetryDrainFailed records that the run-telemetry queue could not be
	// sealed before the restart, so the resume's records say the trace may
	// have a gap. Telemetry never blocks the restart.
	TelemetryDrainFailed bool `json:"telemetry_drain_failed,omitempty"`
}

// plannedRestartMarkerPending reports whether a marker that could still plan a
// boot is on disk: present and no older than plannedRestartMaxAge. A marker
// that cannot be read or parsed counts by its file time (fail closed while it
// is fresh). No store path, or no marker, is not pending.
func plannedRestartMarkerPending(storePath string, now time.Time) bool {
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

// errHibernateFenced refuses a planned-restart mark while the hibernate fence
// is closed: the marker would be written into the snapshot being taken.
var errHibernateFenced = errors.New("hibernate_fenced: the guest is being hibernated; a planned restart cannot be marked now")

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
// plannedRestartMaxAge at now; anything else is a crash boot. The boot passes
// its process start as now, so slow boot phases before consumption do not
// count against the age. The marker is removed
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
	if err := syncPath(filepath.Dir(path)); err != nil {
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
	// Autopaper's source schedule (W1c): a crash never resumes the poll a
	// pre-boot wake carries; the crash boot arms the schedule's next tick
	// as a new wake (ruling 6, recoverSourceSchedulesAtBoot).
	"source_poll_due": true,
	// Cut 1 L3a: a record's wake starts a turn; unread records wait for the
	// next post-boot commit.
	"inbox_record": true,
	// PD3 S1: a crash never delivers a pre-boot root digest; the next commit
	// after boot mints a new one.
	TextureRootDigestKind: true,
}

// TextureRootDigestKind is the Texture root's wake (PD3 S1,
// docs/definitions/choir-pd3-texture-root-desk-2026-10-11.md): minted after
// commits on documents other than the master, coalesced and debounced, and
// delivered to TextureRootDigestMailbox, whose handler checks the marked
// master's liveness (and runs the ensure step) before it wakes the master's
// own Texture desk.
const TextureRootDigestKind = "texture_root_digest"

// TextureRootDigestMailbox is the one mailbox digests are scheduled to, per
// owner and computer. It is a mailbox, not an agent: the root is
// texture:<master>, and the master can change (a settled master is
// replaced), so the digest is addressed before the master is known.
const TextureRootDigestMailbox = "texture-root-digest"

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
	return spendGuestAttempt(storePath, reason, target, guestApplyAttemptLimit)
}

// spendGuestAttempt is spendGuestApplyAttempt with the limit the caller's
// reason allows: a crash-boot settlement is allowed fewer starts than an
// apply (selfdevSettlementAttemptLimit).
func spendGuestAttempt(storePath, reason, target string, limit int) error {
	path := GuestApplyAttemptsPath(storePath)
	if path == "" {
		return nil
	}
	counts, err := readGuestAttempts(path)
	if err != nil {
		return err
	}
	key := reason + " " + target
	if counts[key] >= limit {
		return fmt.Errorf("%w (%s %s, %d starts)", ErrGuestApplyAttemptsSpent, reason, target, counts[key])
	}
	counts[key]++
	return writeGuestAttempts(path, counts)
}

// peekGuestAttempts reads how many starts of reason and target are counted
// without spending one.
func peekGuestAttempts(storePath, reason, target string) (int, error) {
	path := GuestApplyAttemptsPath(storePath)
	if path == "" {
		return 0, nil
	}
	counts, err := readGuestAttempts(path)
	if err != nil {
		return 0, err
	}
	return counts[reason+" "+target], nil
}

// clearGuestAttempt forgets the counted starts of reason and target: the
// start finished (it returned, so it did not crash the guest), or the owner
// reopened a closed cadence.
func clearGuestAttempt(storePath, reason, target string) error {
	path := GuestApplyAttemptsPath(storePath)
	if path == "" {
		return nil
	}
	counts, err := readGuestAttempts(path)
	if err != nil {
		return err
	}
	key := reason + " " + target
	if _, counted := counts[key]; !counted {
		return nil
	}
	delete(counts, key)
	return writeGuestAttempts(path, counts)
}

func readGuestAttempts(path string) (map[string]int, error) {
	counts := map[string]int{}
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("guest apply attempts: read: %w", err)
	default:
		if err := json.Unmarshal(raw, &counts); err != nil {
			return nil, fmt.Errorf("guest apply attempts: decode: %w", err)
		}
	}
	return counts, nil
}

func writeGuestAttempts(path string, counts map[string]int) error {
	raw, err := json.Marshal(counts)
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

// errSettlementUnbounded refuses a crash-boot settlement that cannot keep a
// durable attempt count: with no count across boots nothing bounds it, and an
// unbounded settlement is the crash loop the bound exists to stop.
var errSettlementUnbounded = errors.New("crash-boot settlement refused: no durable attempt count, so it cannot be bounded; an owner act is needed")

// spendSettlementAttempt is spendGuestAttempt for a crash-boot settlement,
// which fails closed where an apply's count does not: a runtime without a
// store path refuses the settlement instead of running it unbounded.
func spendSettlementAttempt(storePath, reason, target string, limit int) error {
	if GuestApplyAttemptsPath(storePath) == "" {
		return fmt.Errorf("%w (%s %s)", errSettlementUnbounded, reason, target)
	}
	return spendGuestAttempt(storePath, reason, target, limit)
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
	releaseSlot := func() {}
	if rt != nil {
		storePath = rt.cfg.StorePath
		// The mark is one work occurrence through the hibernate fence, taken
		// without waiting: a closed fence refuses it before an attempt is spent
		// or anything is written, and the fence cannot close until the marker
		// write returns (D2 S5).
		release, ok := rt.hibernate.tryEnter()
		if !ok {
			log.Printf("runtime: refuse %s %s: %v", reason, target, errHibernateFenced)
			return func(error) {}, errHibernateFenced
		}
		releaseSlot = release
	}
	defer releaseSlot()
	if err := spendGuestApplyAttempt(storePath, reason, target); err != nil {
		log.Printf("runtime: refuse %s %s: %v", reason, target, err)
		return func(error) {}, err
	}
	// Queued run telemetry is sealed before the marker is written, so a marker
	// normally means the queue was empty when the restart began and the one
	// resume starts on a tape with no telemetry gap. New telemetry takes the
	// acknowledged path until then. A failed drain is recorded, not fatal.
	drainFailed, restoreTelemetry := rt.drainTelemetryForRestart()
	if err := WritePlannedRestartMarker(storePath, PlannedRestart{Reason: reason, Target: target, TelemetryDrainFailed: drainFailed}); err != nil {
		// Without the marker the restart reads as a crash and pending work is
		// interrupted, which is safe; the apply itself proceeds.
		log.Printf("runtime: %v (restart will be treated as a crash)", err)
		return func(error) { restoreTelemetry() }, nil
	}
	return func(error) {
		restoreTelemetry()
		if err := ClearPlannedRestartMarker(storePath); err != nil {
			log.Printf("runtime: clear planned restart marker after apply returned: %v", err)
		}
	}, nil
}

// PlannedRestartPlatformRefresh is the reason the host records when it
// refreshes this guest onto a new platform build for a deploy.
const PlannedRestartPlatformRefresh = "platform update"

// plannedRestartWork is the work a restart would cut, in two kinds
// (docs/problems/always-busy-computer-never-updates-2026-10-15.md):
//
//   - running runs resume on a planned boot (their runs park and their
//     unconsumed actor occurrences are redelivered), so a host may cut them
//     when it asks with allow_busy;
//   - self-development operations mid-transaction (executing in a capsule,
//     or decided and applying) and live capsules (an engineering assignment
//     that is not terminal) do not survive a restart: capsules live in the
//     executor's memory, and a decided operation is mid-apply. They always
//     refuse.
type plannedRestartWork struct {
	Running       int
	SelfdevActive int
	LiveCapsules  int
	// SnapshotInFlight is a snapshot streaming: a restart would discard it
	// and the next attempt would read the whole store again, so on a busy
	// computer that restarts at every deploy no snapshot would finish.
	SnapshotInFlight bool
}

// unsafe reports work no restart may cut, whatever the host asks.
func (w plannedRestartWork) unsafe() bool {
	return w.SelfdevActive > 0 || w.LiveCapsules > 0 || w.SnapshotInFlight
}

// plannedRestartBusy counts the work a restart would cut; countCapsules adds
// the live capsules (a restart loses them; a memory snapshot does not). It
// fails closed: an unreadable operation or assignment list is an error, not
// idle.
func (rt *Runtime) plannedRestartBusy(ctx context.Context, countCapsules bool) (plannedRestartWork, error) {
	work := plannedRestartWork{Running: rt.RunningCount(), SnapshotInFlight: rt.snapshots.inFlight.Load()}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" {
		return work, nil
	}
	if rt.selfdevOperations != nil {
		ops, err := rt.selfdevOperations.ListByStates(ctx, computerID,
			selfdev.StateExecuting, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateRollbackPending)
		if err != nil {
			return work, fmt.Errorf("cannot read self-development operations: %w", err)
		}
		work.SelfdevActive = len(ops)
	}
	if countCapsules {
		live, err := rt.liveCapsuleCount(ctx)
		if err != nil {
			return work, err
		}
		work.LiveCapsules = live
	}
	return work, nil
}

// errLiveCapsulesUncountable is the answer when there is no computer id or
// store to count for: unknown, never 0 (review 312 Low 3).
var errLiveCapsulesUncountable = errors.New("live capsules cannot be counted: no computer id or store")

func (rt *Runtime) liveCapsulesCountable() bool {
	return strings.TrimSpace(rt.TextureComputerID()) != "" && rt.store != nil
}

// liveCapsuleCount counts the computer's assignments whose capsule a restart
// would destroy (plannedRestartLiveCapsule). It is the one counter: the
// planned-restart refusal and /health's live_capsules (density D3a, which
// vmctl's idle decision reads) both call it. It fails closed: an unreadable
// assignment list is an error, never a 0 that reads as idle.
func (rt *Runtime) liveCapsuleCount(ctx context.Context) (int, error) {
	if !rt.liveCapsulesCountable() {
		return 0, errLiveCapsulesUncountable
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, computerID)
	if err != nil {
		return 0, fmt.Errorf("cannot read engineering assignments: %w", err)
	}
	live := 0
	for _, assignment := range assignments {
		if plannedRestartLiveCapsule(assignment) {
			live++
		}
	}
	return live, nil
}

// plannedRestartLiveCapsule reports an assignment whose capsule a restart
// would destroy: not terminal, and bound to a capsule that is active,
// freezing, frozen and undecided, or being revoked. An unbound assignment
// has no capsule yet (its spawn deadline owns it), and counting it would let
// a stranded pre-bind row block every refresh; a bound one is bounded by its
// fate deadline.
func plannedRestartLiveCapsule(assignment types.EngineeringAssignment) bool {
	if assignment.Disposition.Terminal() {
		return false
	}
	switch assignment.CapsuleDisposition {
	case types.EngineeringCapsuleActive, types.EngineeringCapsuleFreezeRequested,
		types.EngineeringCapsuleFrozen, types.EngineeringCapsuleRevokeRequested:
		return true
	}
	return false
}

type plannedRestartRequest struct {
	Target string `json:"target"`
}

type plannedRestartBusyResponse struct {
	Error   string `json:"error"`
	Outcome string `json:"outcome"`
	// Reason is "hibernate_fenced" when the refusal is the hibernate fence.
	Reason           string `json:"reason,omitempty"`
	AllowBusy        bool   `json:"allow_busy"`
	RunningRuns      int    `json:"running_runs"`
	SelfdevActiveOps int    `json:"selfdev_active_operations"`
	LiveCapsules     int    `json:"live_capsules"`
	SnapshotInFlight bool   `json:"snapshot_in_flight,omitempty"`
}

type plannedRestartWrittenResponse struct {
	PlannedRestart
	// InterruptedRunningRuns is how many running runs this restart cuts
	// (allow_busy); they park for the planned boot's one resume.
	InterruptedRunningRuns int `json:"interrupted_running_runs"`
}

// plannedRestartAllowBusy reads the host's allow_busy request. It is a query
// parameter, not a body field: the handler decodes the body with
// DisallowUnknownFields, so a body field would make every guest built before
// it answer 400; a query parameter is ignored by those guests, which keep
// their old busy refusal.
func plannedRestartAllowBusy(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("allow_busy"))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// HandleInternalPlannedRestart serves /internal/runtime/planned-restart, the
// host's request to restart this guest onto a new platform build.
//
//   - POST {"target": "<build>"} writes the planned-restart marker (reason
//     "platform update") and answers 200 only once it is durable. A busy
//     guest answers 409 with outcome "busy" and writes nothing, so the host
//     retries it at its next idle moment.
//   - POST ...?allow_busy=1 (the host's policy for always-on platform
//     computers, owner ruling 2026-10-15) also writes the marker while runs
//     are running: they park for the planned boot's one resume. It still
//     answers 409 while a self-development operation is mid-transaction or a
//     capsule is live, which a restart would destroy.
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
		// The host did not restart after all: telemetry queues again once
		// the marker is gone.
		if err := h.rt.clearHostPlannedRestartMarker(storePath); err != nil {
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
	// The mark is one work occurrence through the hibernate fence, taken
	// without waiting (D2 S5): a closed fence refuses it before anything is
	// written, and an open fence cannot close until the marker write returns.
	releaseSlot, entered := h.rt.hibernate.tryEnter()
	if !entered {
		writeAPIJSON(w, http.StatusConflict, plannedRestartBusyResponse{
			Error:   "the guest is being hibernated; restart deferred to its next idle moment",
			Outcome: "busy", Reason: "hibernate_fenced",
		})
		return
	}
	defer releaseSlot()
	allowBusy := plannedRestartAllowBusy(r)
	work, err := h.rt.plannedRestartBusy(r.Context(), true)
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "planned restart: " + err.Error()})
		return
	}
	if work.unsafe() || (work.Running > 0 && !allowBusy) {
		// A refused request restarts nothing: an earlier request's
		// synchronous mode is undone unless its marker is still durable.
		h.rt.releaseHostRestartTelemetryIfNoMarker(storePath)
		why := "guest is busy; restart deferred to its next idle moment"
		if work.unsafe() {
			why = "a self-development operation, a live capsule or a streaming snapshot would not survive a restart; restart deferred until it settles"
		}
		writeAPIJSON(w, http.StatusConflict, plannedRestartBusyResponse{
			Error:       why,
			Outcome:     "busy",
			AllowBusy:   allowBusy,
			RunningRuns: work.Running, SelfdevActiveOps: work.SelfdevActive, LiveCapsules: work.LiveCapsules,
			SnapshotInFlight: work.SnapshotInFlight,
		})
		return
	}
	// The host SIGKILLs the VM after this answer, so the shutdown drain never
	// runs: queued run telemetry is sealed here, before the marker is durable,
	// and new telemetry takes the acknowledged path until the kill. A failed
	// drain is recorded on the marker, not fatal (the same seam as
	// markPlannedRestart; design 4.5).
	marker, err := h.rt.writeHostPlannedRestartMarker(storePath, PlannedRestart{Reason: PlannedRestartPlatformRefresh, Target: target, RequestedAt: time.Now().UTC()})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	releaseSlot() // the marker is durable: the fence may close and will see it
	marker.WorkStateEpoch = CurrentWorkStateEpoch
	// The kill skips every fsync the guest's writers left to the kernel (a
	// frozen bundle, the visual captures): flush them all before the answer
	// that lets the host kill (docs/problems/awaiting-candidate-unreadable-after-refresh-2026-10-26.md).
	syncFilesystems()
	log.Printf("runtime: host requested a planned restart (%s %s; allow_busy=%v, interrupting %d running run(s))", marker.Reason, marker.Target, allowBusy, work.Running)
	writeAPIJSON(w, http.StatusOK, plannedRestartWrittenResponse{PlannedRestart: marker, InterruptedRunningRuns: work.Running})
}
