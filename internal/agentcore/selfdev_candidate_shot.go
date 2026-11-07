package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The candidate-shot interlock (docs/design/desk-shots-and-review-2026-11-07.md
// 3.2; source-only-freeze-and-candidate-shots-2026-11-06.md 3.3, FM11). A
// platform candidate is shot by management's verbs after the host build and
// before the swap, and the swap is refused without a passing shot verdict
// event bound to (operation, host build job, shot job, bundle, plan). The rule
// is permanent, not a landing-order note: no build can apply an unjudged
// platform candidate.
//
// Failure modes decided first:
//
//   - FM11 an unshot apply: a platform bundle with non-exempt changes and
//     no plan fails closed by name (a legacy screens bundle too: Verify
//     refuses it); one with a plan needs a passing verdict.
//   - A foreign or stale verdict unlocking the swap (FM2): the key names the
//     operation and host build job, and the payload is re-checked against all
//     the bindings, so a pass for a prior build (a clean rebase has a new
//     job) never unlocks this one. The gate reads only its own key and never
//     scans commitment records.
//   - Asking a judge that cannot look (FM4): with no image-capable model in
//     management's chain the operation fails at once, before any record.
//   - Waiting unbounded (FM1): an accepted operation blocks engineering
//     admission, so 45 minutes after the shots-owed record, or the notice cap
//     spent and unanswered for a grace, closes it failed.
//   - The gate holding the materializer lock while it waits: it returns
//     pending before any transition, like the host build.
//   - A journal-only settle: it calls no updater and needs no shot; the
//     caller never runs the gate for it.
//   - A package operation: its candidate is shot in its capsule (S4 moves
//     that later), so the gate does not apply.

// errCandidateShotPending keeps an accepted operation accepted while
// management judges the shots (the pending pattern of errHostBuildPending).
var errCandidateShotPending = errors.New("candidate shot pending")

// candidateShotFailure is a verdict that closes the operation failed with
// "shot review: ...". Nothing is swapped.
type candidateShotFailure struct{ Cause string }

func (f candidateShotFailure) Error() string { return "shot review: " + f.Cause }

// platformShotRelevantPaths are the bundle's non-exempt platform source
// paths: the changes a screen must show (capsule.ScreenExemptPath).
func platformShotRelevantPaths(bundle transaction.CapsuleEffectBundle) []string {
	changes := make([]capsule.FileChange, 0, len(bundle.OrderedFileEffects))
	for _, effect := range bundle.OrderedFileEffects {
		changes = append(changes, capsule.FileChange{Path: effect.Path, Mode: os.FileMode(effect.Mode)})
	}
	return capsule.ScreenRelevantPaths(changes)
}

// candidateShotGate decides whether the swap may proceed: nil to proceed, an
// errCandidateShotPending-wrapped error to stay accepted, a
// candidateShotFailure to close the operation failed. It runs on every
// non-journal-only pass.
func (rt *Runtime) candidateShotGate(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built *hostBuiltRelease) error {
	if operation.IsPackage() {
		return nil
	}
	relevant := platformShotRelevantPaths(bundle)
	if len(relevant) == 0 {
		return nil // all-exempt (docs, tests): no shot, as today
	}
	if bundle.ShotPlan == nil {
		if len(bundle.Screens) > 0 {
			return candidateShotFailure{Cause: "this is a legacy screens bundle; shots are taken after the host build, from a shot plan"}
		}
		return candidateShotFailure{Cause: fmt.Sprintf("this platform candidate changes %s and carries neither screens nor a shot plan, so nothing shows the build that would apply", strings.Join(firstN(relevant, 3), ", "))}
	}
	if built == nil || built.Job.JobID == "" {
		return candidateShotFailure{Cause: "this candidate has a shot plan but no host build to shoot"}
	}
	planDigest, err := bundle.ShotPlan.Digest()
	if err != nil {
		return candidateShotFailure{Cause: "the shot plan does not digest: " + err.Error()}
	}
	key := computerevent.ShotVerdictKey(operation.OperationID, built.Job.JobID)
	event, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, key)
	if err != nil {
		return err
	}
	if found {
		return rt.readShotVerdictEvent(ctx, operation, built.Job.JobID, planDigest, key, event)
	}
	if ok, why := rt.managementCanSeeImages(ctx); !ok {
		return candidateShotFailure{Cause: why}
	}
	if err := rt.askManagementForShots(ctx, operation, bundle, *built, planDigest); err != nil {
		return err
	}
	if rt.shotsAskedHook != nil {
		rt.shotsAskedHook(operation, bundle, *built)
	}
	return fmt.Errorf("%w: operation %s host build %s", errCandidateShotPending, operation.OperationID, built.Job.JobID)
}

// readShotVerdictEvent checks the verdict on the gate's key against every
// binding: pass continues to the swap, fail closes the operation with
// management's reason.
func (rt *Runtime) readShotVerdictEvent(ctx context.Context, operation selfdev.Operation, hostJobID, planDigest, key string, event computerevent.Event) error {
	if event.EventKind != computerevent.EventShotVerdict {
		return candidateShotFailure{Cause: fmt.Sprintf("the event on the shot verdict key is a %s event, not a shot_verdict", event.EventKind)}
	}
	if rt.eventPayloadReader == nil {
		return fmt.Errorf("shot verdict event %s is unreadable", key)
	}
	raw, err := rt.eventPayloadReader.FetchPayload(ctx, operation.ComputerID, event.PayloadCommitment)
	if err != nil {
		return fmt.Errorf("shot verdict payload: %w", err)
	}
	var payload computerevent.ShotVerdictPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Validate() != nil {
		return candidateShotFailure{Cause: "the shot_verdict event does not decode"}
	}
	if payload.OperationID != operation.OperationID || payload.HostBuildJobID != hostJobID ||
		payload.BundleDigest != operation.BundleDigest || payload.PlanSHA256 != planDigest || strings.TrimSpace(payload.ShotJobID) == "" {
		return candidateShotFailure{Cause: "the shot_verdict event is not bound to this operation, host build and plan"}
	}
	if payload.Verdict != computerevent.ShotVerdictPass {
		return candidateShotFailure{Cause: payload.Reason}
	}
	return nil
}

// askManagementForShots commits the shots-owed record once and then keeps the
// clock: a notice when management is idle and the record or last notice is
// old enough, a failure at the deadline or with the notice cap spent.
func (rt *Runtime) askManagementForShots(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease, planDigest string) error {
	ownerID, computerID := rt.shotsOwnerID(), operation.ComputerID
	if ownerID == "" || rt.store == nil {
		return candidateShotFailure{Cause: "this computer has no owner route to ask management for the shots"}
	}
	management := persistentManagementAgentID(ownerID)
	recordID := types.ShotsOwedRecordID(operation.OperationID, built.Job.JobID)
	rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, recordID)
	if err != nil {
		return err
	}
	now := time.Now()
	if rt.shotsNow != nil {
		now = rt.shotsNow()
	}
	if rec == nil {
		record := types.NewShotsOwedRecord(management, computerID, operation.TrajectoryID, operation.OperationID, built.Job.JobID, operation.BundleDigest, planDigest, shotsOwedLine(operation.OperationID))
		return rt.commitShotsOwedRecord(ctx, ownerID, computerID, record)
	}
	committed := now
	if at, perr := time.Parse(time.RFC3339Nano, rec.Provenance.CommittedAt); perr == nil {
		committed = at
	}
	n, last := rt.shotsOwedNotices(ctx, ownerID, computerID, operation.OperationID, built.Job.JobID)
	since := committed
	if !last.IsZero() {
		since = last
	}
	if now.Sub(committed) >= shotsOwedDeadline || (n >= types.ShotsOwedMaxNotices && now.Sub(since) >= shotsOwedGrace) {
		return candidateShotFailure{Cause: shotsNotJudgedInTime}
	}
	if n >= types.ShotsOwedMaxNotices || now.Sub(since) < shotsOwedNoticeAfter {
		return nil
	}
	// Due: a fresh wake, only when management is idle (a working run's guard
	// and its end ask for the verdict).
	active, found, err := rt.activeRunByAgent(ctx, ownerID, management)
	if err != nil {
		return err
	}
	if found && !rt.runHasNoLiveLoop(active, rt.assignmentWaitNow()) {
		return nil
	}
	body := fmt.Sprintf("%s Notice %d of %d.", shotsOwedLine(operation.OperationID), n+1, types.ShotsOwedMaxNotices)
	notice := types.NewShotsOwedNotice(management, computerID, operation.TrajectoryID, operation.OperationID, built.Job.JobID, operation.BundleDigest, planDigest, n+1, body)
	return rt.commitShotsOwedRecord(ctx, ownerID, computerID, notice)
}

func (rt *Runtime) commitShotsOwedRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) error {
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "shots-owed:" + rec.RecordID, Records: []types.CommitmentRecord{rec}}
	var err error
	if req.CommandDigest, err = choirstore.ComputeCommitInboxRecordsDigest(req); err != nil {
		return err
	}
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
		return fmt.Errorf("shots owed: commit %s: %w", rec.RecordID, err)
	}
	return nil
}

// shotsOwedNotices counts the notices already committed and when the last one was.
func (rt *Runtime) shotsOwedNotices(ctx context.Context, ownerID, computerID, operationID, hostJobID string) (int, time.Time) {
	var last time.Time
	n := 0
	for i := 1; i <= types.ShotsOwedMaxNotices; i++ {
		rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.ShotsOwedNoticeID(operationID, hostJobID, i))
		if err != nil || rec == nil {
			break
		}
		n = i
		if at, perr := time.Parse(time.RFC3339Nano, rec.Provenance.CommittedAt); perr == nil {
			last = at
		}
	}
	return n, last
}

func firstN(items []string, n int) []string {
	if len(items) > n {
		return items[:n]
	}
	return items
}
