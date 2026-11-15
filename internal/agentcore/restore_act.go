package agentcore

// Restore as an accepted act.
//
// A restore (and rematerialize-from-tape, the same engine) used to be one
// request that held its connection for the whole replay: scan, install a
// snapshot, replay the tail at about 113 events a second, check the witness,
// flip. Every budget on its path is ten minutes, and the paper's tail is 46
// minutes today and can reach 80 (docs/problems/restore-cannot-finish-inside-
// its-ten-minute-request-2026-11-14.md, "Repair design"). Now the request is
// answered when the act is accepted, and the work runs here, in the guest's
// background, while a status route reports it.
//
// The record of an act is a small JSON file on the data volume beside the
// store (restore-acts/<act id>.json), written atomically, like the capsule job
// records. It is not in the store: the flip replaces the store. The tape holds
// the intent (the restore_requested event, idempotent per checkpoint). An act
// never resumes: a boot closes whatever it finds open as "interrupted by a
// restart" (RepairRestoreAtBoot), after undoing a half-made flip.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Act states. The first five are open; the last three end the act.
const (
	restoreStateAccepted     = "accepted"
	restoreStateFetchingBase = "fetching_base"
	restoreStateReplaying    = "replaying"
	restoreStateVerifying    = "verifying"
	restoreStateInstalling   = "installing"
	restoreStateSucceeded    = "succeeded"
	restoreStateFailed       = "failed"
	restoreStateInterrupted  = "interrupted"
	// restoreStateRestored ends an act whose flip completed but that did not
	// finish as a success: a failed reopen (the computer needs a restart), or a
	// process that ended after the flip (the boot rolled it forward). The
	// computer is on the restored store and the restored release, together.
	restoreStateRestored      = "restored"
	restoreActKindRestore     = "restore"
	restoreActKindRematerial  = "rematerialize"
	restoreActSchemaVersion   = 1
	restoreActDirName         = "restore-acts"
	restoreActRecordsKept     = 20
	restoreActPersistInterval = 5 * time.Second
	restoreInterruptedReason  = "interrupted by a restart"
)

func restoreStateTerminal(state string) bool {
	return state == restoreStateSucceeded || state == restoreStateFailed || state == restoreStateInterrupted || state == restoreStateRestored
}

// The install's durable facts, in order. Each is written (file fsync, atomic
// rename, directory fsync) before the step it announces, except
// flip_completed, written right after the last rename. Boot decides from the
// last one written, never from which files exist.
const (
	restoreInstallPrepared        = "prepared"
	restoreInstallReleaseRestaged = "release_restaged"
	restoreInstallFlipStarted     = "flip_started"
	restoreInstallFlipCompleted   = "flip_completed"
)

// What the act says is on the computer when it ends.
const (
	restoreLivePreRestore = "pre_restore"
	restoreLiveRestored   = "restored"
	restoreLiveUnresolved = "unresolved"
)

// restoreForwardNotice is said on every success: the restored state is not
// forward-durable (restore-is-neither-forward-nor-durable-2026-10-30).
const restoreForwardNotice = "restored to the checkpoint; the restored state is not forward-durable: events the tape holds after the target (the restore intent among them) are applied again at the next append or boot (docs/problems/restore-is-neither-forward-nor-durable-2026-10-30.md)"

// restoreInstallPhaseKnown is a phase boot understands (empty: the install had
// not begun).
func restoreInstallPhaseKnown(phase string) bool {
	switch phase {
	case "", restoreInstallPrepared, restoreInstallReleaseRestaged, restoreInstallFlipStarted, restoreInstallFlipCompleted:
		return true
	}
	return false
}

// restoreActApplyHook, when set, is called after each tail event a restore
// act applies, with the number applied so far. It is a seam for tests that
// hold a replay mid-way; production leaves it nil.
var restoreActApplyHook func(applied int)

// restoreActFreeBytes is the free-space reading the disk floor uses.
var restoreActFreeBytes = freeBytesOnVolume

// restoreActIDPattern is what an act id looks like: it names a file, so
// nothing else is looked up.
var restoreActIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// RestoreActRefusal names why an act, or a request for one, was refused. The
// HTTP status is the one the synchronous route answered with.
type RestoreActRefusal struct {
	Name       string `json:"name"`
	HTTPStatus int    `json:"http_status"`
	Message    string `json:"message"`
}

// RestoreActFlipStep is one rename of the realization flip, recorded before
// the first is made so a boot can tell how far a dead process got.
type RestoreActFlipStep struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RestoreActEstimate is what the acceptance answer says about the work ahead.
// The rate is the one measured on this computer when there is one, else the
// assumed one, and ReplayRateSource says which.
type RestoreActEstimate struct {
	TailEvents       uint64  `json:"tail_events"`
	ReplayRate       float64 `json:"replay_rate"`
	ReplayRateSource string  `json:"replay_rate_source"`
	EstimatedSeconds float64 `json:"estimated_seconds"`
	// Basis says where the tail came from: the local event index at
	// acceptance, or nothing when the index does not know the target (the plan,
	// which reads the tape, then makes it exact).
	Basis string `json:"basis"`
}

// estimateRestore is the estimate of a tail at a rate. measured is whether the
// rate was measured on this computer; otherwise the assumed rate is used.
func estimateRestore(tail uint64, measuredRate float64, measured bool) RestoreActEstimate {
	rate, source := snapshotAssumedReplayRate, "assumed"
	if measured && measuredRate > 0 {
		rate, source = measuredRate, "measured"
	}
	return RestoreActEstimate{TailEvents: tail, ReplayRate: rate, ReplayRateSource: source, EstimatedSeconds: float64(tail) / rate}
}

// RestoreActRecord is an act as it is stored and as the status route answers.
type RestoreActRecord struct {
	SchemaVersion             int                `json:"schema_version"`
	ActID                     string             `json:"act_id"`
	Kind                      string             `json:"kind"`
	ComputerID                string             `json:"computer_id"`
	CheckpointDigest          string             `json:"checkpoint_digest"`
	TargetHead                string             `json:"target_head,omitempty"`
	IdempotencyKey            string             `json:"idempotency_key"`
	State                     string             `json:"state"`
	Reason                    string             `json:"reason,omitempty"`
	Refusal                   *RestoreActRefusal `json:"refusal,omitempty"`
	Estimate                  RestoreActEstimate `json:"estimate"`
	BaseSequence              uint64             `json:"base_sequence,omitempty"`
	TargetSequence            uint64             `json:"target_sequence,omitempty"`
	TailEventsTotal           uint64             `json:"tail_events_total"`
	TailEventsExact           bool               `json:"tail_events_exact"`
	TailEventsApplied         uint64             `json:"tail_events_applied"`
	ReplayRate                float64            `json:"replay_rate"`
	ReplayRateSource          string             `json:"replay_rate_source"`
	EstimatedSecondsRemaining float64            `json:"estimated_seconds_remaining"`
	AcceptedAt                string             `json:"accepted_at"`
	StartedAt                 string             `json:"started_at,omitempty"`
	UpdatedAt                 string             `json:"updated_at"`
	FinishedAt                string             `json:"finished_at,omitempty"`
	StatusURL                 string             `json:"status_url"`
	// PriorReleaseDigest and Flip are what the installing step is about to
	// change, written before it changes them: the boot puts the release back
	// and undoes a half flip from them.
	PriorReleaseDigest    string               `json:"prior_release_digest,omitempty"`
	RestoredReleaseDigest string               `json:"restored_release_digest,omitempty"`
	QuarantineDir         string               `json:"quarantine_dir,omitempty"`
	Flip                  []RestoreActFlipStep `json:"flip,omitempty"`
	// InstallPhase is the last durable install fact; LiveState is what the
	// computer is on when the act ends: pre_restore, restored or unresolved.
	InstallPhase string               `json:"install_phase,omitempty"`
	LiveState    string               `json:"live_state,omitempty"`
	Result       *RematerializeReport `json:"result,omitempty"`
	// AlreadyRunning is set only on an acceptance answer that returned the act
	// already running for the same request.
	AlreadyRunning bool `json:"already_running,omitempty"`
}

// errRestoreInProgress is a request that meets a running act.
type errRestoreInProgress struct{ act RestoreActRecord }

func (e errRestoreInProgress) Error() string {
	return fmt.Sprintf("restore: act %s is already running (%s); read %s", e.act.ActID, e.act.State, e.act.StatusURL)
}

// errRestoreInsufficientDisk is the disk floor.
var errRestoreInsufficientDisk = errors.New("restore: not enough free space")

// errRestoreUnresolved is a boot that left an act it could not resolve.
var errRestoreUnresolved = errors.New("restore: an earlier restore act is unresolved")

// errRestoreBlocked is live work a restore must not run beside.
var errRestoreBlocked = errors.New("restore: blocked by live work")

// errRestoredRestartRequired is a failure after the flip completed: the
// computer is on the restored store and release and needs a restart.
var errRestoredRestartRequired = errors.New("restored; restart required")

// errRestoreInvalid is a request whose checkpoint the engine's own validation
// refuses (it was a 500 from the synchronous route, for want of a name).
var errRestoreInvalid = errors.New("restore: invalid request")

// restoreActs is the runtime's registry of restore acts: at most one runs.
// Its zero value is ready.
type restoreActs struct {
	mu       sync.Mutex
	running  *restoreActRun
	inFlight atomic.Bool
}

// restoreActRun is an act in this process.
type restoreActRun struct {
	mu      sync.Mutex
	rec     RestoreActRecord
	dir     string
	applied atomic.Uint64
	// replayStarted is when the replay began (zero before).
	replayStarted time.Time
	persistedAt   time.Time
}

func (r *restoreActRun) path() string { return filepath.Join(r.dir, r.rec.ActID+".json") }

// view is the record with the live figures computed.
func (r *restoreActRun) view() RestoreActRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.viewLocked()
}

func (r *restoreActRun) viewLocked() RestoreActRecord {
	rec := r.rec
	if !restoreStateTerminal(rec.State) {
		rec.TailEventsApplied = r.applied.Load()
		rec.ReplayRate, rec.ReplayRateSource = rec.Estimate.ReplayRate, rec.Estimate.ReplayRateSource
		if rec.TailEventsApplied > 0 && !r.replayStarted.IsZero() {
			if elapsed := time.Since(r.replayStarted).Seconds(); elapsed > 0 {
				rec.ReplayRate, rec.ReplayRateSource = float64(rec.TailEventsApplied)/elapsed, "measured"
			}
		}
		remaining := uint64(0)
		if rec.TailEventsTotal > rec.TailEventsApplied {
			remaining = rec.TailEventsTotal - rec.TailEventsApplied
		}
		if rec.ReplayRate > 0 {
			rec.EstimatedSecondsRemaining = float64(remaining) / rec.ReplayRate
		}
	}
	return rec
}

// persistLocked writes the record; a failure is logged, never fatal to the act
// (the act is still running and the status route reads memory).
func (r *restoreActRun) persistLocked() {
	r.rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	r.persistedAt = time.Now()
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		log.Printf("restore act %s: record directory: %v", r.rec.ActID, err)
		return
	}
	if err := writeRestoreActRecord(r.dir, r.viewLocked()); err != nil {
		log.Printf("restore act %s: persist its record: %v", r.rec.ActID, err)
	}
}

func writeRestoreActRecord(dir string, rec RestoreActRecord) error {
	rec.AlreadyRunning = false
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "."+rec.ActID+".json.tmp")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, rec.ActID+".json"))
}

// writeRestoreActRecordDurable is writeRestoreActRecord for the records that
// decide what boot does: the temporary file is synced before the rename and the
// directory after it, so a record that was written is a record a power cut
// finds. phase names the fact for the test seam that fails a write.
func writeRestoreActRecordDurable(dir string, rec RestoreActRecord, phase string) error {
	if hook := restoreInstallWriteFault; hook != nil {
		if err := hook(phase); err != nil {
			return err
		}
	}
	rec.AlreadyRunning = false
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmpPath := filepath.Join(dir, "."+rec.ActID+".json.tmp")
	tmp, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filepath.Join(dir, rec.ActID+".json")); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func syncDirectory(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// ---- progress, as the engine reports it ----

type restoreActCtxKey struct{}

// restoreProgressFrom is the act the engine runs for, or nil: a direct call of
// the engine (a test, a boot path) has none, and every method is nil-safe.
func restoreProgressFrom(ctx context.Context) *restoreActRun {
	run, _ := ctx.Value(restoreActCtxKey{}).(*restoreActRun)
	return run
}

func (r *restoreActRun) enter(state string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if restoreStateTerminal(r.rec.State) {
		return
	}
	r.rec.State = state
	if r.rec.StartedAt == "" {
		r.rec.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if state == restoreStateReplaying {
		r.replayStarted = time.Now()
	}
	r.persistLocked()
}

// planned records the plan's exact tail: the installed base and the target.
func (r *restoreActRun) planned(base, target uint64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec.BaseSequence, r.rec.TargetSequence = base, target
	if target >= base {
		r.rec.TailEventsTotal = target - base
	}
	r.rec.TailEventsExact = true
	r.persistLocked()
}

// install writes one durable install fact: the record, with the phase and
// whatever mutate sets, is written file fsync, atomic rename, directory fsync.
// A failed write returns the error and changes nothing, in memory or on disk,
// so the caller stops where it is. A nil act (a direct call of the engine)
// records nothing.
func (r *restoreActRun) install(phase string, mutate func(*RestoreActRecord)) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	next := r.rec
	next.Flip = append([]RestoreActFlipStep(nil), r.rec.Flip...)
	next.State, next.InstallPhase = restoreStateInstalling, phase
	next.TailEventsApplied = r.applied.Load()
	if mutate != nil {
		mutate(&next)
	}
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := writeRestoreActRecordDurable(r.dir, next, phase); err != nil {
		return fmt.Errorf("restore: record the %s fact before going on: %w", phase, err)
	}
	r.rec, r.persistedAt = next, time.Now()
	return nil
}

// setLiveState records what the computer is on, for the end of the act.
func (r *restoreActRun) setLiveState(state string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.rec.LiveState = state
	r.mu.Unlock()
}

// onApplied is the replay observer's per-event callback.
func (r *restoreActRun) onApplied(applied int) {
	if r == nil {
		return
	}
	r.applied.Store(uint64(applied))
	r.mu.Lock()
	if time.Since(r.persistedAt) >= restoreActPersistInterval {
		r.persistLocked()
	}
	r.mu.Unlock()
	if hook := restoreActApplyHook; hook != nil {
		hook(applied)
	}
}

// The observer the replay reports to is the one the exactly-once receipt
// reads, with this act's counter on its callback.
var _ computerevent.ReplayObserver = (*restoreReplayObserver)(nil)

// ---- the refusal names ----

// restoreActRefusalOf is the named refusal an error stands for, and the HTTP
// status the synchronous route answered it with.
func restoreActRefusalOf(err error) RestoreActRefusal {
	name := "error"
	switch {
	case errors.Is(err, recoveryplan.ErrBaseRefused):
		name = "base_refused"
	case errors.Is(err, ErrCheckpointNotDerivable):
		name = "checkpoint_not_derivable"
	case errors.Is(err, selfdevprotocol.ErrWitnessMismatch):
		name = "witness_mismatch"
	case errors.Is(err, selfdevprotocol.ErrWitnessVersionRefused):
		name = "witness_version_refused"
	case errors.Is(err, ErrRematerializeUnavailable):
		name = "unavailable"
	case errors.Is(err, errRestoreInsufficientDisk):
		name = "insufficient_disk"
	case isRestoreInProgress(err):
		name = "restore_in_progress"
	case errors.Is(err, errRestoreInvalid):
		name = "invalid_checkpoint"
	case errors.Is(err, errRestoreUnresolved):
		name = "restore_unresolved"
	case errors.Is(err, errRestoreBlocked):
		name = "blocked_by_live_work"
	}
	return RestoreActRefusal{Name: name, HTTPStatus: rematerializeRefusalStatus(err), Message: err.Error()}
}

func isRestoreInProgress(err error) bool {
	var busy errRestoreInProgress
	return errors.As(err, &busy)
}

// ---- acceptance ----

func (rt *Runtime) restoreActDir() string {
	if rt == nil || rt.store == nil {
		return ""
	}
	return filepath.Join(filepath.Dir(rt.store.Path()), restoreActDirName)
}

func newRestoreActID() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "ra-" + hex.EncodeToString(raw), nil
}

func restoreActStatusURL(computerID, actID string) string {
	return "/api/computers/" + computerID + "/lifecycle/restore-status?act=" + actID
}

// restoreInFlight reports whether a restore act is running: the work a
// restart, an idle hibernate or an automatic restart must not cut.
func (rt *Runtime) restoreInFlight() bool {
	return rt != nil && rt.restoreActs.inFlight.Load()
}

// acceptRestoreAct validates a restore or rematerialize request as far as it
// can without the tape, claims the one act slot, records the intent (restore
// only), writes the act's record and starts the work. It returns the act as
// accepted, or as already running when the same request is in flight.
func (rt *Runtime) acceptRestoreAct(ctx context.Context, kind, computerID string, checkpoint selfdevprotocol.Checkpoint, key string) (RestoreActRecord, error) {
	computerID = strings.TrimSpace(computerID)
	if rt == nil || rt.store == nil || rt.eventAppender == nil {
		return RestoreActRecord{}, fmt.Errorf("%w: event projection authority is not configured", ErrRematerializeUnavailable)
	}
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return RestoreActRecord{}, fmt.Errorf("%w: computer binding does not match runtime", ErrRematerializeUnavailable)
	}
	originalMarker, originalWorkspace := strings.TrimSpace(rt.store.Path()), strings.TrimSpace(rt.store.TexturePath())
	if originalMarker == "" || originalWorkspace == "" {
		return RestoreActRecord{}, fmt.Errorf("%w: original realization paths are required", ErrRematerializeUnavailable)
	}
	// What the engine checks first, checked now, so the owner is told at once:
	// the checkpoint and its witness version, and the release it pins.
	if err := selfdevprotocol.RematerializeFromRequest(rt.rematerializeRequest(computerID, checkpoint, originalMarker, originalWorkspace,
		filepath.Join(filepath.Dir(originalMarker), "rematerialize-staging-check"))); err != nil {
		if errors.Is(err, selfdevprotocol.ErrWitnessVersionRefused) {
			return RestoreActRecord{}, err
		}
		return RestoreActRecord{}, fmt.Errorf("%w: %w", errRestoreInvalid, err)
	}
	if !computerevent.IsSHA256(strings.TrimSpace(checkpoint.Request.AcceptedEventHead)) {
		return RestoreActRecord{}, fmt.Errorf("%w: recovery target head is required", ErrRematerializeUnavailable)
	}
	rt.currentReleaseMu.Lock()
	_, err := rt.verifyPinnedFrontend(checkpoint)
	rt.currentReleaseMu.Unlock()
	if err != nil {
		return RestoreActRecord{}, err
	}
	if open := rt.restoreUnresolvedNow(); len(open) > 0 {
		return RestoreActRecord{}, fmt.Errorf("%w: %s; the runbook is in docs/problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-14.md (\"The runbook for an unresolved act\"); nothing of its staged or quarantined stores was deleted", errRestoreUnresolved, strings.Join(open, ", "))
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = strings.TrimSpace(checkpoint.Digest)
	}

	acts := &rt.restoreActs
	acts.mu.Lock()
	// An act that has ended but has not yet let go of the slot (it records its
	// end, then releases) does not hold it.
	if running := acts.running; running != nil && restoreStateTerminal(running.view().State) {
		acts.running = nil
		acts.inFlight.Store(false)
	}
	if running := acts.running; running != nil {
		view := running.view()
		same := view.IdempotencyKey == key && view.CheckpointDigest == checkpoint.Digest && view.Kind == kind
		acts.mu.Unlock()
		if same {
			view.AlreadyRunning = true
			return view, nil
		}
		return RestoreActRecord{}, errRestoreInProgress{act: view}
	}
	// The disk floor and the live work, with the claim held so two requests
	// cannot both pass them.
	if err := rt.restoreDiskFloor(); err != nil {
		acts.mu.Unlock()
		return RestoreActRecord{}, err
	}
	if err := rt.restoreLiveWork(ctx); err != nil {
		acts.mu.Unlock()
		return RestoreActRecord{}, err
	}
	actID, err := newRestoreActID()
	if err != nil {
		acts.mu.Unlock()
		return RestoreActRecord{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	run := &restoreActRun{dir: rt.restoreActDir(), rec: RestoreActRecord{
		SchemaVersion: restoreActSchemaVersion, ActID: actID, Kind: kind, ComputerID: computerID,
		CheckpointDigest: checkpoint.Digest, TargetHead: checkpoint.Request.AcceptedEventHead, IdempotencyKey: key,
		State: restoreStateAccepted, AcceptedAt: now, UpdatedAt: now, StatusURL: restoreActStatusURL(computerID, actID),
	}}
	run.rec.Estimate = rt.restoreEstimate(ctx, computerID, checkpoint.Request.AcceptedEventHead)
	run.rec.TailEventsTotal = run.rec.Estimate.TailEvents
	run.rec.ReplayRate, run.rec.ReplayRateSource = run.rec.Estimate.ReplayRate, run.rec.Estimate.ReplayRateSource
	run.rec.EstimatedSecondsRemaining = run.rec.Estimate.EstimatedSeconds
	acts.running = run
	acts.inFlight.Store(true)
	// release lets go of the slot, if it is still this act's (a request that
	// found it ended has already taken it over). The caller holds acts.mu.
	release := func() {
		if acts.running == run {
			acts.running = nil
			acts.inFlight.Store(false)
		}
	}
	acts.mu.Unlock()

	// The first record is durable (a torn first record reads as unreadable, and
	// beside a staging directory that refuses every restore), and it is written
	// before the intent so a refused request leaves none.
	run.mu.Lock()
	err = writeRestoreActRecordDurable(run.dir, run.viewLocked(), "accepted")
	run.mu.Unlock()
	if err != nil {
		acts.mu.Lock()
		release()
		acts.mu.Unlock()
		return RestoreActRecord{}, fmt.Errorf("restore: record the accepted act: %w", err)
	}
	if kind == restoreActKindRestore {
		if err := rt.appendRestoreIntent(ctx, computerID, checkpoint); err != nil {
			refusal := restoreActRefusalOf(err)
			run.finish(restoreStateFailed, &refusal, nil, "", restoreLivePreRestore)
			acts.mu.Lock()
			release()
			acts.mu.Unlock()
			return RestoreActRecord{}, err
		}
	}
	run.mu.Lock()
	run.pruneLocked()
	view := run.viewLocked()
	run.mu.Unlock()
	go rt.runRestoreAct(run, release, checkpoint)
	return view, nil
}

// rematerializeRequest is the engine's request for these paths.
func (rt *Runtime) rematerializeRequest(computerID string, checkpoint selfdevprotocol.Checkpoint, originalMarker, originalWorkspace, stagingRoot string) selfdevprotocol.RematerializeRequest {
	return selfdevprotocol.RematerializeRequest{
		ComputerID:            computerID,
		Checkpoint:            checkpoint,
		Method:                selfdevprotocol.RematerializeMethodTape,
		OriginalMarkerPath:    originalMarker,
		OriginalWorkspacePath: originalWorkspace,
		StagedMarkerPath:      filepath.Join(stagingRoot, filepath.Base(originalMarker)),
		StagedWorkspacePath:   filepath.Join(stagingRoot, filepath.Base(originalWorkspace)+"-pending"),
	}
}

// restoreDiskFloor refuses a restore the volume cannot hold: the staged store
// grows beside the live one, so the live store's size plus the snapshot
// producer's floor must be free. A volume that cannot be measured does not
// block (a development platform).
func (rt *Runtime) restoreDiskFloor() error {
	state := rt.store.StatePath()
	var need uint64
	for _, path := range []string{state, state + "-wal"} {
		if info, err := os.Stat(path); err == nil {
			need += uint64(info.Size())
		}
	}
	need += snapshotFreeFloor
	free, err := restoreActFreeBytes(filepath.Dir(state))
	if err != nil {
		log.Printf("restore: free space is not measurable, the disk floor is not checked: %v", err)
		return nil
	}
	if free < need {
		return fmt.Errorf("%w: %d bytes are free on the data volume and the staged store needs about %d (the live store plus the %d byte floor)", errRestoreInsufficientDisk, free, need, snapshotFreeFloor)
	}
	return nil
}

// restoreLiveWork refuses while a self-development operation is mid-transaction,
// an engineering capsule is live or a snapshot is streaming. Acceptance asks
// it, and the engine asks it again under the release lock immediately before
// it touches anything live: the flip closes the store, and a store closed under
// an apply, a capsule or a snapshot's read transaction ends them half done.
func (rt *Runtime) restoreLiveWork(ctx context.Context) error {
	work, err := rt.plannedRestartBusy(ctx, true)
	if err != nil {
		return fmt.Errorf("restore: the work a restore must not run beside cannot be counted: %w", err)
	}
	var why []string
	if work.SelfdevActive > 0 {
		why = append(why, fmt.Sprintf("%d self-development operation(s) mid-transaction", work.SelfdevActive))
	}
	if work.LiveCapsules > 0 {
		why = append(why, fmt.Sprintf("%d engineering capsule(s) live", work.LiveCapsules))
	}
	if work.SnapshotInFlight {
		why = append(why, "a snapshot is streaming")
	}
	if len(why) > 0 {
		return fmt.Errorf("%w: %s", errRestoreBlocked, strings.Join(why, "; "))
	}
	return nil
}

// snapshotBusyReason is why a snapshot capture must not run now: work a
// snapshot must not run beside is live, or a restore is in flight (the capture
// holds a read transaction the flip's store close would meet).
func (rt *Runtime) snapshotBusyReason(ctx context.Context) (string, error) {
	work, err := rt.plannedRestartBusy(ctx, true)
	if err != nil {
		return "", err
	}
	if work.RestoreInFlight {
		return "a restore is in flight", nil
	}
	if work.SelfdevActive > 0 {
		return fmt.Sprintf("%d self-development operation(s) in flight", work.SelfdevActive), nil
	}
	if work.LiveCapsules > 0 {
		return fmt.Sprintf("%d engineering capsule(s) live", work.LiveCapsules), nil
	}
	return "", nil
}

// WithRestoreUnresolved tells the runtime which acts the boot's repair could
// not resolve: it refuses new restore acts and /health names them.
func WithRestoreUnresolved(ids []string) RuntimeOption {
	return func(rt *Runtime) { rt.restoreUnresolved = append([]string(nil), ids...) }
}

// restoreUnresolvedNow is the set of acts that refuse a new restore: the ones
// the boot could not resolve, every open record of an act that is not running
// (a kill, a failed write or an unresolved live state leaves one), and an
// unreadable record beside a staging or quarantine directory. Read from the
// records each time, so no boot has to come between a failure and the refusal.
func (rt *Runtime) restoreUnresolvedNow() []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, id := range rt.restoreUnresolved {
		add(id)
	}
	dir := rt.restoreActDir()
	if dir == "" {
		return out
	}
	running := ""
	rt.restoreActs.mu.Lock()
	if rt.restoreActs.running != nil {
		running = rt.restoreActs.running.rec.ActID
	}
	rt.restoreActs.mu.Unlock()
	records, unreadable := readRestoreActRecordsStrict(dir)
	for _, rec := range records {
		if !restoreStateTerminal(rec.State) && rec.ActID != running {
			add(rec.ActID)
		}
	}
	if len(unreadable) > 0 && len(restoreLeftoverDirs(filepath.Dir(dir))) > 0 {
		for _, id := range unreadable {
			add(id)
		}
	}
	return out
}

// restoreEstimate is the acceptance answer's estimate. The tail is read from
// the local event index; the rate is this computer's measured replay rate
// when it has one, else the assumed one.
func (rt *Runtime) restoreEstimate(ctx context.Context, computerID, targetHead string) RestoreActEstimate {
	rate, measured := rt.eventAppender.ReplayRate()
	target, base, found, err := rt.store.RestoreTailEstimate(ctx, computerID, targetHead)
	est := estimateRestore(0, rate, measured)
	if err != nil || !found {
		est.Basis = "the local event index does not know the target; the tail is exact once the plan has read the tape"
		return est
	}
	est = estimateRestore(target-min(target, base), rate, measured)
	est.Basis = "the local event index: the target's sequence less the newest snapshot at or below it; a snapshot the host pruned makes the tail longer"
	return est
}

// pruneLocked keeps the newest restoreActRecordsKept ended records of this
// computer; it never touches an open record or another computer's.
func (r *restoreActRun) pruneLocked() {
	records := readRestoreActRecords(r.dir)
	var ended []RestoreActRecord
	for _, rec := range records {
		if rec.ComputerID == r.rec.ComputerID && restoreStateTerminal(rec.State) {
			ended = append(ended, rec)
		}
	}
	sort.Slice(ended, func(i, j int) bool { return ended[i].AcceptedAt > ended[j].AcceptedAt })
	for i := restoreActRecordsKept; i < len(ended); i++ {
		_ = os.Remove(filepath.Join(r.dir, ended[i].ActID+".json"))
	}
}

// ---- the work ----

func (rt *Runtime) runRestoreAct(run *restoreActRun, release func(), checkpoint selfdevprotocol.Checkpoint) {
	acts := &rt.restoreActs
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), restoreActCtxKey{}, run))
	defer cancel()
	stopPersist := make(chan struct{})
	go func() {
		ticker := time.NewTicker(restoreActPersistInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopPersist:
				return
			case <-ticker.C:
				run.mu.Lock()
				if !restoreStateTerminal(run.rec.State) {
					run.persistLocked()
				}
				run.mu.Unlock()
			}
		}
	}()
	var report RematerializeReport
	var err error
	func() {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("restore act panicked: %v", p)
				log.Printf("restore act %s: %v", run.rec.ActID, err)
			}
		}()
		run.enter(restoreStateFetchingBase)
		report, err = rt.RematerializeFromTape(ctx, run.rec.ComputerID, checkpoint)
	}()
	close(stopPersist)
	switch {
	case err == nil:
		// The restart marker first, then the record that says succeeded: the
		// flip_completed fact is already durable, so a restart that races the
		// record rolls the act forward at boot instead of losing it.
		if report.ExecChanged {
			report.RestartScheduled, report.RestartDeferred = rt.requestRestoreRestart(ctx, checkpoint.Digest)
		}
		run.finish(restoreStateSucceeded, nil, &report, restoreForwardNotice, restoreLiveRestored)
	case errors.Is(err, errRestoredRestartRequired):
		// The computer is on the restored store with a store it could not
		// reopen: it needs the restart, and nothing may move `current` under it.
		log.Printf("restore act %s: %v", run.rec.ActID, err)
		rt.restoreRestaged.Store(true)
		report.RestartScheduled, report.RestartDeferred = rt.requestRestoreRestart(ctx, checkpoint.Digest)
		run.finish(restoreStateRestored, nil, &report, err.Error(), restoreLiveRestored)
	default:
		refusal := restoreActRefusalOf(err)
		log.Printf("restore act %s failed: %s: %v", run.rec.ActID, refusal.Name, err)
		run.finish(restoreStateFailed, &refusal, nil, "", "")
	}
	acts.mu.Lock()
	release()
	acts.mu.Unlock()
}

// finish ends the act: the outcome is written durably first, the staged and
// quarantined directories are removed second (nothing else removes them), and
// only then does the act read as ended. liveState is what the computer is on;
// empty means the pre-restore state unless the engine said otherwise. An act
// whose live state is unresolved is never written terminal: it stays open, so
// a boot repairs it and acceptance refuses over it.
func (r *restoreActRun) finish(state string, refusal *RestoreActRefusal, result *RematerializeReport, reason, liveState string) {
	r.mu.Lock()
	next := r.rec
	next.State, next.Refusal, next.Result, next.Reason = state, refusal, result, reason
	switch {
	case liveState != "":
		next.LiveState = liveState
	case next.LiveState == "":
		next.LiveState = restoreLivePreRestore
	}
	unresolved := next.LiveState == restoreLiveUnresolved
	if unresolved {
		next.State = restoreStateInstalling
	} else if next.FinishedAt == "" {
		next.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if state == restoreStateSucceeded {
		next.TailEventsApplied = r.applied.Load()
		next.EstimatedSecondsRemaining = 0
	}
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	dir := r.dir
	r.mu.Unlock()

	// Outside the lock: removing a staged store takes as long as the store is
	// big, and a status read must not wait for it.
	if err := writeRestoreActRecordDurable(dir, next, "finish"); err != nil {
		log.Printf("restore act %s: persist its end: %v (the record stays open; the next boot repairs it)", next.ActID, err)
	} else if !unresolved {
		removeRestoreLeftovers(filepath.Dir(dir))
	}
	r.mu.Lock()
	r.rec, r.persistedAt = next, time.Now()
	r.pruneLocked()
	r.mu.Unlock()
}

// ---- the routes ----

func writeRestoreActRefusal(w http.ResponseWriter, err error) {
	refusal := restoreActRefusalOf(err)
	body := map[string]any{"error": err.Error(), "refusal": refusal}
	var busy errRestoreInProgress
	if errors.As(err, &busy) {
		body["running_act"] = busy.act.ActID
		body["status_url"] = busy.act.StatusURL
	}
	writeAPIJSON(w, refusal.HTTPStatus, body)
}

// acceptRestoreRoute is the POST of restore and of rematerialize-from-tape.
func (h *APIHandler) acceptRestoreRoute(w http.ResponseWriter, r *http.Request, kind, computerID string, checkpoint selfdevprotocol.Checkpoint, key string) {
	view, err := h.rt.acceptRestoreAct(r.Context(), kind, computerID, checkpoint, key)
	if err != nil {
		writeRestoreActRefusal(w, err)
		return
	}
	writeAPIJSON(w, http.StatusAccepted, view)
}

// restoreActStatus is GET /lifecycle/restore-status[?act=ID]: the named act,
// else the running one, else the newest.
func (h *APIHandler) restoreActStatus(w http.ResponseWriter, r *http.Request, computerID string) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil || h.rt.store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "tape restore authority unavailable"})
		return
	}
	actID := strings.TrimSpace(r.URL.Query().Get("act"))
	if actID != "" && !restoreActIDPattern.MatchString(actID) {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no such restore act"})
		return
	}
	acts := &h.rt.restoreActs
	acts.mu.Lock()
	running := acts.running
	acts.mu.Unlock()
	if running != nil && (actID == "" || actID == running.rec.ActID) {
		writeAPIJSON(w, http.StatusOK, running.view())
		return
	}
	var found *RestoreActRecord
	for _, rec := range readRestoreActRecords(h.rt.restoreActDir()) {
		if rec.ComputerID != strings.TrimSpace(computerID) {
			continue
		}
		if actID != "" {
			if rec.ActID == actID {
				rec := rec
				found = &rec
			}
			continue
		}
		if found == nil || rec.AcceptedAt > found.AcceptedAt {
			rec := rec
			found = &rec
		}
	}
	if found == nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no such restore act"})
		return
	}
	writeAPIJSON(w, http.StatusOK, *found)
}

// readRestoreActRecords reads every readable record in dir.
func readRestoreActRecords(dir string) []RestoreActRecord {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []RestoreActRecord
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var rec RestoreActRecord
		if json.Unmarshal(raw, &rec) != nil || rec.ActID+".json" != name {
			continue
		}
		out = append(out, rec)
	}
	return out
}

// ---- the boot ----

// RestoreBootReport is what the boot's repair found: the acts it closed and
// the acts it could not resolve (it touched nothing of theirs).
type RestoreBootReport struct {
	Closed     []string
	Unresolved []string
}

// RepairRestoreAtBoot is the boot's pass over what a restore left behind, run
// before the store opens. It decides from the install fact the act's record
// holds, never from which files exist:
//
//   - before flip_completed the install is undone: every recorded rename last
//     first (each only when its destination exists and its source does not),
//     and `current` put back on the prior release. The computer is on exactly
//     its pre-restore state and the act is closed "interrupted by a restart".
//   - at flip_completed the install is rolled forward: nothing is undone and
//     `current` is made the restored release if it is not. The act is closed
//     "restored". This is not a resumer of work: no replay, no fetch and no
//     staged store is read; by the fact the work finished, and the boot makes
//     at most the one pointer swap an earlier step made.
//
// Nothing is resumed. The repair is idempotent: every step checks before it
// moves, and the act is closed last. What it cannot read, or whose paths leave
// the data volume, it does not touch: the act is reported unresolved and the
// staged and quarantined stores are kept. updaterRoot may be empty.
func RepairRestoreAtBoot(storePath, updaterRoot string) RestoreBootReport {
	var report RestoreBootReport
	storePath = strings.TrimSpace(storePath)
	if storePath == "" {
		log.Printf("restore boot repair: no store path; nothing to repair")
		return report
	}
	dir := filepath.Dir(filepath.Clean(storePath))
	actsDir := filepath.Join(dir, restoreActDirName)
	records, unreadable := readRestoreActRecordsStrict(actsDir)
	leftovers := restoreLeftoverDirs(dir)
	for _, id := range unreadable {
		log.Printf("restore boot repair: the act record %s cannot be read", id)
		if len(leftovers) > 0 {
			report.Unresolved = append(report.Unresolved, id)
		}
	}
	for _, rec := range records {
		if restoreStateTerminal(rec.State) {
			continue
		}
		if err := validateRestoreActRecord(dir, rec); err != nil {
			log.Printf("restore boot repair: act %s is left as it is: %v", rec.ActID, err)
			report.Unresolved = append(report.Unresolved, rec.ActID)
			continue
		}
		closed, stopped, err := repairRestoreAct(actsDir, updaterRoot, rec)
		switch {
		case stopped:
			return report
		case err != nil:
			log.Printf("restore boot repair: act %s is unresolved: %v", rec.ActID, err)
			report.Unresolved = append(report.Unresolved, rec.ActID)
		case closed:
			report.Closed = append(report.Closed, rec.ActID)
		}
	}
	if len(report.Unresolved) > 0 {
		log.Printf("restore boot repair: %d act(s) unresolved (%s): the staged and quarantined stores are kept and restore acts are refused until they are resolved", len(report.Unresolved), strings.Join(report.Unresolved, ", "))
		return report
	}
	removeRestoreLeftovers(dir)
	return report
}

// installFacts are what the one undo routine reads: the last durable install
// fact, the renames the flip_started fact listed, and the two releases.
type installFacts struct {
	Phase    string
	Flip     []RestoreActFlipStep
	Prior    string
	Restored string
}

func (r RestoreActRecord) installFacts() installFacts {
	return installFacts{Phase: r.InstallPhase, Flip: r.Flip, Prior: r.PriorReleaseDigest, Restored: r.RestoredReleaseDigest}
}

// errRepairStopped is the test seam standing in for a kill between steps.
var errRepairStopped = errors.New("restore repair stopped by a seam")

// undoInstall is the only code that undoes an install. The boot calls it with
// the record it read; the engine's failure path calls it with the facts it has
// just written. It is driven by those facts alone, deletes nothing, and is safe
// to run again after being killed at any step (each step checks before it
// moves):
//
//  1. at flip_started: the listed renames, last first, each only when its
//     source is gone and its destination is there (the list holds only renames
//     whose source existed, so nothing is moved by inference);
//  2. the directories those renames touched are synced;
//  3. from prepared on: `current` is made the prior release again, and checked.
//     A release that cannot be put back is an error, and an error is
//     unresolved: never reported clean.
func undoInstall(updaterRoot string, f installFacts) error {
	if f.Phase == restoreInstallFlipStarted {
		for i := len(f.Flip) - 1; i >= 0; i-- {
			step := f.Flip[i]
			_, fromErr := os.Lstat(step.From)
			_, toErr := os.Lstat(step.To)
			if fromErr != nil && toErr == nil {
				if restoreRepairStops("undo") {
					return errRepairStopped
				}
				if err := os.Rename(step.To, step.From); err != nil {
					return fmt.Errorf("undo %s -> %s: %w", step.To, step.From, err)
				}
			}
		}
		if err := syncFlipDirs(f.Flip); err != nil {
			return fmt.Errorf("sync the undone renames: %w", err)
		}
		restoreInstallCrash("after_undo")
	}
	if f.Phase == "" {
		return nil
	}
	if restoreRepairStops("release") {
		return errRepairStopped
	}
	return ensureReleaseOf(updaterRoot, f.Prior)
}

// syncFlipDirs syncs every directory the listed renames moved something out of
// or into, so a rename that was made is a rename a power cut finds (the fact
// that says so is written after).
func syncFlipDirs(steps []RestoreActFlipStep) error {
	seen := map[string]bool{}
	for _, step := range steps {
		for _, path := range []string{step.From, step.To} {
			dir := filepath.Dir(path)
			if seen[dir] {
				continue
			}
			seen[dir] = true
			if _, err := os.Stat(dir); err != nil {
				continue
			}
			if hook := restoreDirSyncHook; hook != nil {
				hook(dir)
			}
			if err := syncDirectory(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

// repairRestoreAct repairs one open act at boot. stopped is the test seam
// standing in for a kill between steps: the rest of the boot is skipped, as a
// kill would skip it.
func repairRestoreAct(actsDir, updaterRoot string, rec RestoreActRecord) (closed, stopped bool, err error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if rec.InstallPhase == restoreInstallFlipCompleted {
		// Roll forward is bookkeeping: the flip finished and the release was
		// restaged durably before it. Nothing is moved.
		rec.State, rec.LiveState = restoreStateRestored, restoreLiveRestored
		rec.Reason = "restored; the process ended after the flip completed and before the result was recorded: the computer is on the restored store and the restored release"
	} else {
		switch err := undoInstall(updaterRoot, rec.installFacts()); {
		case errors.Is(err, errRepairStopped):
			return false, true, nil
		case err != nil:
			return false, false, err
		}
		rec.State, rec.LiveState = restoreStateInterrupted, restoreLivePreRestore
		rec.Reason = restoreInterruptedReason + "; the computer is on its pre-restore state"
	}
	if restoreRepairStops("close") {
		return false, true, nil
	}
	rec.FinishedAt, rec.UpdatedAt = now, now
	if err := writeRestoreActRecordDurable(actsDir, rec, "closed"); err != nil {
		return false, false, fmt.Errorf("close the record: %w", err)
	}
	log.Printf("restore act %s (%s) was open before this boot at %q: closed as %s (%s)", rec.ActID, rec.Kind, rec.InstallPhase, rec.State, rec.LiveState)
	return true, false, nil
}

// readRestoreActRecordsStrict reads every record in dir and names the files it
// cannot read (a record that does not parse, or whose id is not its file's).
func readRestoreActRecordsStrict(dir string) (records []RestoreActRecord, unreadable []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		raw, err := os.ReadFile(filepath.Join(dir, name))
		var rec RestoreActRecord
		if err != nil || json.Unmarshal(raw, &rec) != nil || rec.ActID != id {
			unreadable = append(unreadable, id)
			continue
		}
		records = append(records, rec)
	}
	return records, unreadable
}

// restoreDigestPattern is a release digest as a record may name one: it is
// handed to the updater, so nothing path-like is.
var restoreDigestPattern = regexp.MustCompile(`^[A-Za-z0-9:._-]{1,128}$`)

// validateRestoreActRecord refuses a record whose paths leave the data volume
// or whose digests are not digests: the repair renames what a record names.
func validateRestoreActRecord(dir string, rec RestoreActRecord) error {
	if !restoreActIDPattern.MatchString(rec.ActID) {
		return fmt.Errorf("act id %q", rec.ActID)
	}
	if !restoreInstallPhaseKnown(rec.InstallPhase) {
		return fmt.Errorf("install phase %q is not one this release knows", rec.InstallPhase)
	}
	within := func(path string) bool {
		clean := filepath.Clean(path)
		return filepath.IsAbs(clean) && strings.HasPrefix(clean, filepath.Clean(dir)+string(filepath.Separator))
	}
	for _, step := range rec.Flip {
		if !within(step.From) || !within(step.To) {
			return fmt.Errorf("the rename %q -> %q leaves the data volume %s", step.From, step.To, dir)
		}
	}
	for _, digest := range []string{rec.PriorReleaseDigest, rec.RestoredReleaseDigest} {
		if digest != "" && !restoreDigestPattern.MatchString(digest) {
			return fmt.Errorf("release digest %q", digest)
		}
	}
	return nil
}

// ensureReleaseOf makes `current` the release and checks that it is. An empty
// updater root (a computer with no updater) has nothing to put back. An empty
// digest with a `current` present means the prior release was never recorded:
// it cannot be put back, so it is an error.
func ensureReleaseOf(updaterRoot, digest string) error {
	updaterRoot, digest = strings.TrimSpace(updaterRoot), strings.TrimSpace(digest)
	if updaterRoot == "" {
		return nil
	}
	current, err := updater.ReadCurrentManifest(updaterRoot)
	if digest == "" {
		if err == nil {
			return fmt.Errorf("the prior release was not recorded and `current` is on %s: it cannot be put back", shortDigest(current.ContentDigest))
		}
		return nil
	}
	if err == nil && current.ContentDigest == digest {
		return nil
	}
	if err := updater.RestagePinnedReleaseBestEffort(updaterRoot, digest); err != nil {
		return fmt.Errorf("make release %s current: %w", shortDigest(digest), err)
	}
	if now, err := updater.ReadCurrentManifest(updaterRoot); err != nil || now.ContentDigest != digest {
		return fmt.Errorf("release %s is not current after it was restaged", shortDigest(digest))
	}
	return nil
}

// restoreLeftoverDirs are the staged and quarantined stores beside the store.
func restoreLeftoverDirs(dir string) []string {
	staged, _ := filepath.Glob(filepath.Join(dir, "rematerialize-staging-*"))
	quarantined, _ := filepath.Glob(filepath.Join(dir, "rematerialize-quarantine-*"))
	return append(staged, quarantined...)
}

// removeRestoreLeftovers removes the staged stores and the empty quarantine
// directories no running restore owns (none runs at boot). A quarantine with
// the old store in it stays.
func removeRestoreLeftovers(dir string) {
	staged, _ := filepath.Glob(filepath.Join(dir, "rematerialize-staging-*"))
	for _, path := range staged {
		if err := os.RemoveAll(path); err != nil {
			log.Printf("restore boot repair: remove %s: %v", path, err)
		}
	}
	quarantined, _ := filepath.Glob(filepath.Join(dir, "rematerialize-quarantine-*"))
	for _, path := range quarantined {
		if entries, err := os.ReadDir(path); err == nil && len(entries) == 0 {
			_ = os.Remove(path)
		}
	}
}
