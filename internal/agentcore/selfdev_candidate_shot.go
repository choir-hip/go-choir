package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// The candidate-shot interlock (docs/design/source-only-freeze-and-candidate-shots-2026-11-06.md
// 3.3, FM11). A platform candidate is shot by the platform after the host
// build and before the swap, and the swap is refused without a passing
// candidate_shot event. The rule is permanent, not a landing-order note: no
// build can apply an unshot platform candidate.
//
// Failure modes decided first:
//
//   - FM11 an unshot apply: a platform bundle with non-exempt changes and
//     neither screens nor a plan fails closed by name; one with a plan needs
//     a passing event bound to (operation, host build job, bundle, plan).
//   - A foreign or stale event unlocking the swap: the key names the
//     operation and host build job, and the payload is re-checked against
//     all four bindings, so a pass for a prior build (a clean rebase has a
//     new job) never unlocks this one.
//   - The gate holding the materializer lock while it waits: it returns
//     pending before any transition, like the host build, and the pass that
//     runs it holds no shot-side lock.
//   - A journal-only settle: it calls no updater and needs no shot; the
//     caller never runs the gate for it.
//   - A package operation: its candidate is shot in its capsule (S4 moves
//     that later), so the gate does not apply.

// errCandidateShotPending keeps an accepted operation accepted while the
// shot worker runs (the pending pattern of errHostBuildPending).
var errCandidateShotPending = errors.New("candidate shot pending")

// candidateShotFailure is a verdict that closes the operation failed with
// "shot review: ...". Nothing is swapped.
type candidateShotFailure struct{ Cause string }

func (f candidateShotFailure) Error() string { return "shot review: " + f.Cause }

// startCandidateShot starts the shot worker for the operation's host-built
// candidate, if it is not already running, and returns at once. The S2
// builder fills this in: it must write exactly one candidate_shot event
// (appendCandidateShot) for (operation, built.Job.JobID), pass or fail, and
// must hold no materializer lock. Until then it only logs, so a plan bundle
// stays pending.
func (rt *Runtime) startCandidateShot(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
	if rt.candidateShotStarter != nil {
		rt.candidateShotStarter(operation, bundle, built)
		return
	}
	log.Printf("selfdev candidate shot: operation %s host build %s: no shot worker is wired; staying pending", operation.OperationID, built.Job.JobID)
}

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
			return nil // the engineer's own screens, reviewed at Verify
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
	key := computerevent.CandidateShotKey(operation.OperationID, built.Job.JobID)
	event, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, key)
	if err != nil {
		return err
	}
	if !found {
		rt.startCandidateShot(operation, bundle, *built)
		return fmt.Errorf("%w: operation %s host build %s", errCandidateShotPending, operation.OperationID, built.Job.JobID)
	}
	if event.EventKind != computerevent.EventCandidateShot || rt.eventPayloadReader == nil {
		return fmt.Errorf("candidate shot event %s is unreadable", key)
	}
	raw, err := rt.eventPayloadReader.FetchPayload(ctx, operation.ComputerID, event.PayloadCommitment)
	if err != nil {
		return fmt.Errorf("candidate shot payload: %w", err)
	}
	var payload computerevent.CandidateShotPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Validate() != nil {
		return candidateShotFailure{Cause: "the candidate_shot event does not decode"}
	}
	if payload.OperationID != operation.OperationID || payload.HostBuildJobID != built.Job.JobID ||
		payload.BundleDigest != operation.BundleDigest || payload.PlanSHA256 != planDigest {
		return candidateShotFailure{Cause: "the candidate_shot event is not bound to this operation, host build and plan"}
	}
	if payload.Verdict != computerevent.CandidateShotPass {
		return candidateShotFailure{Cause: payload.Cause}
	}
	return nil
}

// appendCandidateShot writes the operation's candidate_shot event,
// idempotently on (operation, host build job). It is the one writer: the
// shot worker calls it at the end of a run, pass or fail.
func (rt *Runtime) appendCandidateShot(ctx context.Context, operation selfdev.Operation, payload computerevent.CandidateShotPayload) error {
	payload.OperationID, payload.BundleDigest = operation.OperationID, operation.BundleDigest
	if err := payload.Validate(); err != nil {
		return err
	}
	key := computerevent.CandidateShotKey(operation.OperationID, payload.HostBuildJobID)
	if _, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, key); err != nil {
		return err
	} else if found {
		return nil
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return err
	}
	body, err := computerevent.CanonicalJSON(payload)
	if err != nil {
		return err
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
		EventKind: computerevent.EventCandidateShot, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: key, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
		ActorProfile: platformUpdateActor(), AuthorityRef: "platform-candidate-shot:" + operation.OperationID, PrivacyClass: "owner",
		ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent,
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	_, _, err = rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, body, computerevent.CandidateShotMediaType, "owner")
	return err
}

func firstN(items []string, n int) []string {
	if len(items) > n {
		return items[:n]
	}
	return items
}
