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
	restoreStateAccepted      = "accepted"
	restoreStateFetchingBase  = "fetching_base"
	restoreStateReplaying     = "replaying"
	restoreStateVerifying     = "verifying"
	restoreStateInstalling    = "installing"
	restoreStateSucceeded     = "succeeded"
	restoreStateFailed        = "failed"
	restoreStateInterrupted   = "interrupted"
	restoreActKindRestore     = "restore"
	restoreActKindRematerial  = "rematerialize"
	restoreActSchemaVersion   = 1
	restoreActDirName         = "restore-acts"
	restoreActRecordsKept     = 20
	restoreActPersistInterval = 5 * time.Second
	restoreInterruptedReason  = "interrupted by a restart"
)

func restoreStateTerminal(state string) bool {
	return state == restoreStateSucceeded || state == restoreStateFailed || state == restoreStateInterrupted
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
	PriorReleaseDigest string               `json:"prior_release_digest,omitempty"`
	Flip               []RestoreActFlipStep `json:"flip,omitempty"`
	Result             *RematerializeReport `json:"result,omitempty"`
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

// beginInstall records what the install is about to change, durably, before it
// changes it.
func (r *restoreActRun) beginInstall(prior string, flip []renameStep) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec.State = restoreStateInstalling
	r.rec.PriorReleaseDigest = prior
	r.rec.Flip = r.rec.Flip[:0]
	for _, step := range flip {
		r.rec.Flip = append(r.rec.Flip, RestoreActFlipStep{From: step.from, To: step.to})
	}
	r.persistLocked()
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
	// The disk floor, with the claim held so two requests cannot both pass it.
	if err := rt.restoreDiskFloor(); err != nil {
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

	if kind == restoreActKindRestore {
		if err := rt.appendRestoreIntent(ctx, computerID, checkpoint); err != nil {
			acts.mu.Lock()
			release()
			acts.mu.Unlock()
			return RestoreActRecord{}, err
		}
	}
	run.mu.Lock()
	run.persistLocked()
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
	if err != nil {
		refusal := restoreActRefusalOf(err)
		log.Printf("restore act %s failed: %s: %v", run.rec.ActID, refusal.Name, err)
		run.finish(restoreStateFailed, &refusal, nil, "")
	} else {
		run.finish(restoreStateSucceeded, nil, &report, "")
		if report.ExecChanged {
			// The marker is written after the record says succeeded: the
			// restart that follows closes nothing.
			report.RestartScheduled, report.RestartDeferred = rt.requestRestoreRestart(ctx, checkpoint.Digest)
			run.finish(restoreStateSucceeded, nil, &report, "")
		}
	}
	acts.mu.Lock()
	release()
	acts.mu.Unlock()
}

// finish ends the act (or, for a succeeded act, rewrites its result).
func (r *restoreActRun) finish(state string, refusal *RestoreActRefusal, result *RematerializeReport, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec.State, r.rec.Refusal, r.rec.Result, r.rec.Reason = state, refusal, result, reason
	if r.rec.FinishedAt == "" {
		r.rec.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if state == restoreStateSucceeded {
		r.rec.TailEventsApplied = r.applied.Load()
		r.rec.EstimatedSecondsRemaining = 0
	}
	r.persistLocked()
	r.pruneLocked()
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

// RepairRestoreAtBoot is the boot's pass over what a restore left behind, run
// before the store opens. A process that died mid-restore leaves an open act
// record, maybe a staged store (on the paper, the size of a store), and, if
// it died in the install, a half-made flip and a `current` release already
// moved. Nothing is resumed: the flip is undone when it stopped half way, the
// prior release is put back when the flip did not complete, the staged stores
// are removed, and every open act is closed "interrupted by a restart". It
// returns the ids it closed. updaterRoot may be empty (no release to put back).
func RepairRestoreAtBoot(storePath, updaterRoot string) RestoreBootReport {
	dir := filepath.Dir(filepath.Clean(storePath))
	actsDir := filepath.Join(dir, restoreActDirName)
	var closed []string
	for _, rec := range readRestoreActRecords(actsDir) {
		if restoreStateTerminal(rec.State) {
			continue
		}
		reason := restoreInterruptedReason
		if rec.State == restoreStateInstalling {
			switch flipProgress(rec.Flip) {
			case flipComplete:
				reason += "; the flip had completed, so the computer is on the restored state"
			case flipPartial:
				undoFlip(rec.Flip)
				restoreReleaseOf(updaterRoot, rec.PriorReleaseDigest)
				reason += "; a flip half made was undone and the computer is on its pre-restore state"
			default:
				restoreReleaseOf(updaterRoot, rec.PriorReleaseDigest)
			}
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.State, rec.Reason, rec.FinishedAt, rec.UpdatedAt = restoreStateInterrupted, reason, now, now
		if err := writeRestoreActRecord(actsDir, rec); err != nil {
			log.Printf("restore act %s: close its record: %v", rec.ActID, err)
			continue
		}
		log.Printf("restore act %s (%s) was open before this boot: closed as %s", rec.ActID, rec.Kind, reason)
		closed = append(closed, rec.ActID)
	}
	removeRestoreLeftovers(dir)
	return RestoreBootReport{Closed: closed}
}

type flipState int

const (
	flipUntouched flipState = iota
	flipPartial
	flipComplete
)

// flipProgress reads how far the recorded renames got: a step is done when its
// destination exists and its source does not, not done when the source still
// exists; a step whose source never existed is skipped (the flip skips it too).
func flipProgress(steps []RestoreActFlipStep) flipState {
	done, notDone := 0, 0
	for _, step := range steps {
		_, fromErr := os.Lstat(step.From)
		_, toErr := os.Lstat(step.To)
		switch {
		case fromErr != nil && toErr == nil:
			done++
		case fromErr == nil && toErr != nil:
			notDone++
		}
	}
	switch {
	case done == 0:
		return flipUntouched
	case notDone == 0:
		return flipComplete
	}
	return flipPartial
}

// undoFlip puts back the renames that were made, last first.
func undoFlip(steps []RestoreActFlipStep) {
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		_, fromErr := os.Lstat(step.From)
		_, toErr := os.Lstat(step.To)
		if fromErr != nil && toErr == nil {
			if err := os.Rename(step.To, step.From); err != nil {
				log.Printf("restore boot repair: undo %s -> %s: %v", step.To, step.From, err)
			}
		}
	}
}

// restoreReleaseOf puts `current` back on the release an act moved it from.
func restoreReleaseOf(updaterRoot, prior string) {
	updaterRoot, prior = strings.TrimSpace(updaterRoot), strings.TrimSpace(prior)
	if updaterRoot == "" || prior == "" {
		return
	}
	if current, err := updater.ReadCurrentManifest(updaterRoot); err == nil && current.ContentDigest == prior {
		return
	}
	if err := updater.RestagePinnedReleaseBestEffort(updaterRoot, prior); err != nil {
		log.Printf("restore boot repair: put back release %s: %v", shortDigest(prior), err)
	}
}

// removeRestoreLeftovers removes the staged stores and the empty quarantine
// directories no running restore owns (none runs at boot).
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
