package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Unfinished work (docs/evidence/selfdev-trace-reviews-2026-10-25.md,
// problem 1). An implementation round stopped before it froze (its
// activation budget, its deadline, a run that failed without Complete, a
// cancel) mints no candidate, so the carry (carried_patch.go) had nothing to
// offer the next round and the next round started from the base. Every such
// stop destroys the capsule through revokeAssignedCapsule, after the durable
// revoke intent and the capability revoke and before ForceDestroy; that is
// the last moment the capsule's tree exists, and it is where the snapshot is
// taken (revokeAssignedCapsuleKeeping). The capsule is frozen, its source
// change rendered as Freeze renders source.patch (workspace/platform only,
// read without following a link, bounded; capsule.UnfinishedPatchFromChanges),
// and the patch is kept, content-addressed and read only, under
// <updater root>/carried/unfinished with a record bound to the assignment.
//
// The kept patch is never applied and is never a candidate: it is not on the
// tape, so nothing lists, verifies or approves it. Its one reader is the
// carry's history, which offers it to the next round on the same trajectory
// as unfinished, unverified work, through the same refusals as a candidate's
// patch (digest, bound, source paths, the base check, the operation rule).
// Behind engineering.carry_unfinished, off by default; off takes no snapshot
// and offers none.

const (
	carriedUnfinishedSchema    = "choir.carried_unfinished.v1"
	carriedUnfinishedRefPrefix = "carried-unfinished:sha256:"
	// unfinishedSnapshotTimeout bounds the snapshot inside the revoke, so a
	// capsule that cannot be quiesced delays its destroy by at most this.
	unfinishedSnapshotTimeout = 45 * time.Second
	// A record's Omitted list is capped when it is kept (the rest are
	// counted), and a record above unfinishedRecordMaxBytes is not read:
	// records are read on every spawn on the trajectory.
	unfinishedOmittedMaxEntries = 64
	unfinishedOmittedMaxBytes   = 8 << 10
	unfinishedRecordMaxBytes    = 64 << 10
)

var errUnfinishedPatchUnavailable = errors.New("kept unfinished patch is missing or no longer matches its digest")

// unfinishedPatchSnapshotter is the executor's snapshot (capsule.Executor).
type unfinishedPatchSnapshotter interface {
	SnapshotUnfinishedSourcePatch(ctx context.Context, capsuleID string) (capsule.CarriedPatch, error)
}

// unfinishedRound is the record of one kept patch.
type unfinishedRound struct {
	Schema       string    `json:"schema"`
	PatchSHA256  string    `json:"patch_sha256"`
	Size         int       `json:"size"`
	TrajectoryID string    `json:"trajectory_id"`
	AssignmentID string    `json:"assignment_id"`
	Attempt      uint64    `json:"attempt"`
	BaseRef      string    `json:"base_ref"`
	CapsuleID    string    `json:"capsule_id"`
	Reason       string    `json:"reason,omitempty"`
	Files        []string  `json:"files"`
	Omitted      []string  `json:"omitted,omitempty"`
	TakenAt      time.Time `json:"taken_at"`
}

// carryUnfinishedOn reads engineering.carry_unfinished for a computer.
func (rt *Runtime) carryUnfinishedOn(ctx context.Context, ownerID, computerID string) bool {
	return rt.computerSettingOn(ctx, ownerID, computerID, store.SettingEngineeringCarryUnfinished)
}

// isLowerHexSHA256 is whether s is a sha256 digest in lowercase hex, the
// only form a kept patch's digest takes before it becomes a path.
func isLowerHexSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// unfinishedNameOK is whether an id, escaped, is one local path element.
func unfinishedNameOK(id string) bool {
	escaped := url.PathEscape(id)
	return strings.TrimSpace(id) != "" && escaped != "." && escaped != ".." && filepath.IsLocal(escaped) && !strings.ContainsAny(escaped, `/\`)
}

// capOmitted keeps the first entries of a snapshot's omitted list within
// the entry and byte caps, and counts the rest.
func capOmitted(omitted []string) []string {
	var out []string
	size := 0
	for i, entry := range omitted {
		if len(out) == unfinishedOmittedMaxEntries || size+len(entry) > unfinishedOmittedMaxBytes {
			return append(out, fmt.Sprintf("and %d more left out", len(omitted)-i))
		}
		out = append(out, entry)
		size += len(entry)
	}
	return out
}

func unfinishedRoot(root string) string { return filepath.Join(root, "carried", "unfinished") }

func unfinishedPatchPath(root, sha string) string {
	return filepath.Join(unfinishedRoot(root), sha, "source.patch")
}

func unfinishedRecordDir(root, trajectoryID string) string {
	return filepath.Join(unfinishedRoot(root), "records", url.PathEscape(trajectoryID))
}

func unfinishedRecordPath(root, trajectoryID, assignmentID string, attempt uint64) string {
	return filepath.Join(unfinishedRecordDir(root, trajectoryID), url.PathEscape(assignmentID)+fmt.Sprintf("-%d.json", attempt))
}

// unfinishedSnapshotWanted is whether a round's capsule holds work worth
// keeping when it is destroyed: an implementation round that did not report
// (a Report decided completed or failed, and a failed Report already minted
// the candidate the carry reads).
func unfinishedSnapshotWanted(assignment types.EngineeringAssignment) bool {
	return assignment.Binding.Kind == types.EngineeringAssignmentImplementation &&
		strings.TrimSpace(assignment.Binding.CapsuleID) != "" &&
		assignment.Disposition != types.EngineeringAssignmentCompleted &&
		assignment.Disposition != types.EngineeringAssignmentFailed
}

// keepUnfinishedRoundBeforeDestroy takes and keeps the snapshot of a stopped
// round's capsule. It never fails the caller: the revoke and destroy follow
// whatever happens here.
func (rt *Runtime) keepUnfinishedRoundBeforeDestroy(ctx context.Context, exec assignmentCapsuleRuntime, assignment types.EngineeringAssignment, reason string) {
	if rt == nil || exec == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || !unfinishedSnapshotWanted(assignment) {
		return
	}
	snapshotter, ok := exec.(unfinishedPatchSnapshotter)
	if !ok || !exec.HasCapsule(assignment.Binding.CapsuleID) {
		return
	}
	if !rt.computerSettingOn(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, store.SettingEngineeringCarryUnfinished) {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), unfinishedSnapshotTimeout)
	defer cancel()
	if err := keepUnfinishedRound(ctx, snapshotter, rt.selfdevUpdaterRoot, assignment, reason, time.Now().UTC()); err != nil {
		log.Printf("agentcore: unfinished work of assignment %s (capsule %s) not kept: %v", assignment.AssignmentID, assignment.Binding.CapsuleID, err)
	}
}

// keepUnfinishedRound snapshots the capsule once per assignment attempt and
// keeps the patch and its record. A change that touched no source keeps
// nothing.
func keepUnfinishedRound(ctx context.Context, snapshotter unfinishedPatchSnapshotter, root string, assignment types.EngineeringAssignment, reason string, now time.Time) error {
	binding := assignment.Binding
	if !unfinishedNameOK(binding.TrajectoryID) || !unfinishedNameOK(assignment.AssignmentID) {
		return fmt.Errorf("trajectory %q or assignment %q cannot name a record", binding.TrajectoryID, assignment.AssignmentID)
	}
	recordPath := unfinishedRecordPath(root, binding.TrajectoryID, assignment.AssignmentID, binding.Attempt)
	if _, err := os.Lstat(recordPath); err == nil {
		return nil
	}
	patch, err := snapshotter.SnapshotUnfinishedSourcePatch(ctx, binding.CapsuleID)
	if errors.Is(err, capsule.ErrCarriedPatchEmpty) {
		return nil
	}
	if err != nil {
		return err
	}
	// The executor is not trusted with the digest or the paths.
	sum := sha256.Sum256(patch.Patch)
	if len(patch.Patch) == 0 || patch.SHA256 != hex.EncodeToString(sum[:]) || !isLowerHexSHA256(patch.SHA256) {
		return fmt.Errorf("the snapshot does not match its digest")
	}
	if len(patch.Patch) > capsule.CarriedPatchMaxBytes {
		return fmt.Errorf("%w: %d bytes", capsule.ErrCarriedPatchTooLarge, len(patch.Patch))
	}
	files, err := capsule.CarriedPatchPaths(patch.Patch)
	if err != nil {
		return err
	}
	if err := writeReadOnlyAtomic(unfinishedPatchPath(root, patch.SHA256), patch.Patch); err != nil {
		return fmt.Errorf("keep the patch: %w", err)
	}
	record := unfinishedRound{
		Schema: carriedUnfinishedSchema, PatchSHA256: patch.SHA256, Size: len(patch.Patch),
		TrajectoryID: binding.TrajectoryID, AssignmentID: assignment.AssignmentID, Attempt: binding.Attempt,
		BaseRef: binding.SourceArtifactRef, CapsuleID: binding.CapsuleID, Reason: boundCarriedReason(reason),
		Files: files, Omitted: capOmitted(patch.Omitted), TakenAt: now,
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := writeReadOnlyAtomic(recordPath, raw); err != nil {
		return fmt.Errorf("keep the record: %w", err)
	}
	return nil
}

// writeReadOnlyAtomic writes data to path, read only, through a rename. An
// existing file with the same bytes is kept.
func writeReadOnlyAtomic(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".keep-")
	if err != nil {
		return err
	}
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmp.Name())
		return errors.Join(writeErr, closeErr)
	}
	if err := os.Chmod(tmp.Name(), 0o444); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// readUnfinishedRounds lists a trajectory's kept records, oldest first. A
// record that does not decode, or names another trajectory, is skipped.
func readUnfinishedRounds(root, trajectoryID string) []unfinishedRound {
	if strings.TrimSpace(root) == "" || !unfinishedNameOK(trajectoryID) {
		return nil
	}
	entries, err := os.ReadDir(unfinishedRecordDir(root, trajectoryID))
	if err != nil {
		return nil
	}
	var out []unfinishedRound
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if info, err := entry.Info(); err != nil || info.Size() > unfinishedRecordMaxBytes {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(unfinishedRecordDir(root, trajectoryID), entry.Name()))
		if err != nil {
			continue
		}
		var record unfinishedRound
		if json.Unmarshal(raw, &record) != nil || record.Schema != carriedUnfinishedSchema || record.TrajectoryID != trajectoryID ||
			strings.TrimSpace(record.AssignmentID) == "" || !isLowerHexSHA256(record.PatchSHA256) {
			continue
		}
		out = append(out, record)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TakenAt.Before(out[j].TakenAt) })
	return out
}

// loadUnfinishedPatch reads a kept patch and verifies it as a staged one is
// verified: the bytes hash to sha, are within the bound, and name only
// source paths. Its files are read from the patch, not from the record.
func loadUnfinishedPatch(root, sha string) (capsule.CarriedPatch, error) {
	if !isLowerHexSHA256(sha) {
		return capsule.CarriedPatch{}, fmt.Errorf("%w: the digest is not lowercase hex", errUnfinishedPatchUnavailable)
	}
	raw, err := os.ReadFile(unfinishedPatchPath(root, sha))
	if err != nil {
		return capsule.CarriedPatch{}, fmt.Errorf("%w: %v", errUnfinishedPatchUnavailable, err)
	}
	if len(raw) == 0 || len(raw) > capsule.CarriedPatchMaxBytes {
		return capsule.CarriedPatch{}, fmt.Errorf("%w: %d bytes", errUnfinishedPatchUnavailable, len(raw))
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != sha {
		return capsule.CarriedPatch{}, errUnfinishedPatchUnavailable
	}
	files, err := capsule.CarriedPatchPaths(raw)
	if err != nil {
		return capsule.CarriedPatch{}, fmt.Errorf("%w: %v", errUnfinishedPatchUnavailable, err)
	}
	return capsule.CarriedPatch{Patch: raw, SHA256: sha, Files: files}, nil
}

// mergeUnfinishedRounds adds a trajectory's kept rounds to its candidates.
// The assignment, read from the store, is the authority for the round's
// trajectory, attempt, kind and base; a record that disagrees with it is
// skipped. An assignment that has a candidate keeps the candidate.
func mergeUnfinishedRounds(entries []carriedCandidate, rounds []unfinishedRound, trajectoryID string, lookup func(assignmentID string, attempt uint64) (types.EngineeringAssignment, error)) []carriedCandidate {
	type roundKey struct {
		id      string
		attempt uint64
	}
	have := map[roundKey]bool{}
	for _, entry := range entries {
		have[roundKey{entry.AssignmentID, entry.Attempt}] = true
	}
	for _, round := range rounds {
		key := roundKey{round.AssignmentID, round.Attempt}
		if have[key] {
			continue
		}
		assignment, err := lookup(round.AssignmentID, round.Attempt)
		if err != nil {
			continue
		}
		binding := assignment.Binding
		if binding.TrajectoryID != trajectoryID || binding.Attempt != round.Attempt || binding.SourceArtifactRef != round.BaseRef ||
			assignment.AssignmentID != round.AssignmentID {
			continue
		}
		have[key] = true
		entries = append(entries, carriedCandidate{
			CandidateID: "unfinished:" + round.AssignmentID, CandidateRef: carriedUnfinishedRefPrefix + round.PatchSHA256,
			AssignmentID: round.AssignmentID, Attempt: round.Attempt, BaseRef: binding.SourceArtifactRef, Kind: binding.Kind,
			CreatedAt: round.TakenAt, Unfinished: true, UnfinishedReason: round.Reason, UnfinishedOmitted: round.Omitted,
		})
	}
	return entries
}

// keepStoppedRoundsBeforeReconcile takes the reconcile's snapshots before it
// takes engineeringAssignmentOpenMu, so a snapshot (bounded at
// unfinishedSnapshotTimeout) never serializes the computer's opens. It
// keeps only rounds that are stopped for good: a cancelled assignment, or
// one whose bound run ended (terminal or blocked, never resumed) or is gone.
// Those states never return to live, so freezing the capsule here cannot
// strand work the reconcile would keep; the reconcile then revokes and
// destroys under the mutex as before. A round it does not judge stopped
// (a pending run past its grace never ran a cell) is revoked without a keep.
func (rt *Runtime) keepStoppedRoundsBeforeReconcile(ctx context.Context, ownerID, computerID, trajectoryID string) {
	exec := rt.assignedCapsule()
	if rt == nil || rt.store == nil || exec == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || !rt.carryUnfinishedOn(ctx, ownerID, computerID) {
		return
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if err != nil || trajectory.Status != types.TrajectoryLive {
		return
	}
	assignments, err := rt.store.ListEngineeringAssignments(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return
	}
	for _, assignment := range assignments {
		if assignment.BoundRunID == "" || assignment.CapsuleDisposition == types.EngineeringCapsuleRevoked ||
			assignment.CapsuleDisposition == types.EngineeringCapsuleUnbound || !unfinishedSnapshotWanted(assignment) {
			continue
		}
		reason := assignment.DispositionReason
		stopped := assignment.Disposition == types.EngineeringAssignmentCancelled
		if !stopped {
			run, runErr := rt.store.GetLifecycleRun(ctx, ownerID, computerID, assignment.BoundRunID)
			switch {
			case errors.Is(runErr, store.ErrNotFound):
				stopped, reason = true, "the round's run is gone"
			case runErr == nil && engineeringRunEnded(run.State):
				stopped, reason = true, strings.TrimSpace(run.Error)
				if reason == "" {
					reason = "run_" + string(run.State)
				}
			}
		}
		if stopped {
			rt.keepUnfinishedRoundBeforeDestroy(ctx, exec, assignment, reason)
		}
	}
}
