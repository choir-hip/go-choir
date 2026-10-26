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
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Carried patch (failure 15, problem 1, option A,
// docs/problems/engineering-completion-path-cluster-2026-10-20.md). When an
// engineering round ends without a finished candidate and the work is retried
// on the same trajectory, the retry runs in a fresh capsule from the base
// tree. The failed round's source tree is already durable (the candidate its
// terminal Report minted); this file derives it into an inert, digest-bound
// patch, stages it content-addressed and read only, and tells the retry's
// engineer about it. No capsule survives its round and nothing is applied: the
// engineer chooses to apply the patch inside its own capsule, re-tests it
// there, and the normal Freeze, verify, approval and apply judge the result.
//
// A finished candidate is carried the same way when its self-development
// operation did not apply: the owner or a reviewer rejected it, or it failed
// after it froze (owner, 2026-10-25: iterate on a partial result instead of
// starting over). The operation is the assignment's own, keyed by
// store.EngineeringFreezeIdempotencyKey (freeze_candidate.go); its state
// (internal/selfdev/operations.go) decides: rejected and failed carry, and
// every other state does not. Applied (an upstreamed stack, in
// internal/sourceproj, is still an applied operation) would apply the change
// twice; awaiting_approval and the other open states would race a decision
// still pending. The rejection reason, the decision event's pinned output
// payload (api_self_development.go), is passed to the engineer with the patch.
//
// A round stopped before it froze (its activation budget, its deadline, a
// run that ended without Complete, a cancel) mints no candidate; with
// engineering.carry_unfinished on, its capsule's source change is kept just
// before the capsule is destroyed and joins the history as an unfinished
// round (unfinished_patch.go), carried by the same rule and refusals and
// named to the engineer as unfinished, unverified work.
//
// The offer is decided once per assignment and remembered
// (<updater root>/carried/offers/), so a replayed spawn after a crash reads
// the same decision. Failing to carry never fails the retry.
//
// Selection (SD-03, SD-04; docs/problems/selfdev-carry-selection-drops-the-newest-candidate-2026-10-26.md):
// each round is a differential next step on the newest candidate the owner
// has seen. A round whose trajectory's newest work still has an open
// operation is held by admission until it settles (carryHoldFor), so it
// never starts from the base while the decision it should build on is a
// minute away; while any operation on the computer is open every
// implementation request waits, since its Freeze would be refused
// (openOperationHold); and the newest carriable candidate outranks every
// newer stopped round that did not descend from it (selectCarriedCandidate).

const (
	carriedOffered = "offered"
	carriedRefused = "refused"

	carriedOfferSchema = "choir.carried_patch_offer.v1"
	carriedFactsSchema = "choir.carried_patch.v1"
)

var errCarriedPatchTampered = errors.New("staged carried patch no longer matches its digest")

// carriedCandidate is one prior round's changed source tree, with what the
// offer needs to know about the round that made it.
type carriedCandidate struct {
	CandidateID  string
	CandidateRef string
	AssignmentID string
	Attempt      uint64
	// BaseRef is the round's own pinned source (its binding's
	// SourceArtifactRef).
	BaseRef   string
	Kind      types.EngineeringAssignmentKind
	Result    types.EngineeringAssignmentResultKind
	CreatedAt time.Time
	// OperationID and OperationState are the round's own self-development
	// operation, empty when it opened none. OperationReason is why it did
	// not apply: the rejection reason, or the failed operation's error.
	OperationID     string
	OperationState  string
	OperationReason string
	// Unfinished marks a round stopped before it froze whose work was kept
	// from its capsule (unfinished_patch.go); CandidateRef then names the
	// kept patch, not a candidate tree.
	Unfinished        bool
	UnfinishedReason  string
	UnfinishedOmitted []string
}

type carriedPatchHistory interface {
	CarriedCandidates(ctx context.Context, ownerID, computerID, trajectoryID string) ([]carriedCandidate, error)
}

type carriedPatchDeriver interface {
	DeriveCarriedPatch(ctx context.Context, baseRef, candidateRef string) (capsule.CarriedPatch, error)
	CheckCarriedPatchApplies(ctx context.Context, patch capsule.CarriedPatch, retryBaseRef string) error
}

type carriedPatchDeps struct {
	History carriedPatchHistory
	Deriver carriedPatchDeriver
	// Root is the updater root; staged patches live under <Root>/carried.
	Root string
	// CarryUnfinished is engineering.carry_unfinished, read for this offer.
	// Off, a recorded offer of a kept unfinished patch is absent: no mount,
	// no notice (the switch is the whole rollback).
	CarryUnfinished bool
}

type carriedPatchInput struct {
	OwnerID, ComputerID, TrajectoryID string
	AssignmentID                      string
	Attempt                           uint64
	// RetryBaseRef is the retry's own pinned source.
	RetryBaseRef string
}

// carriedPatchFacts is carried.json beside the staged patch: what binds the
// patch to where it came from.
type carriedPatchFacts struct {
	Schema             string   `json:"schema"`
	PatchSHA256        string   `json:"patch_sha256"`
	Size               int      `json:"size"`
	BaseRef            string   `json:"base_ref"`
	CandidateID        string   `json:"candidate_id"`
	CandidateRef       string   `json:"candidate_ref"`
	SourceAssignmentID string   `json:"source_assignment_id"`
	SourceAttempt      uint64   `json:"source_attempt"`
	SourceResult       string   `json:"source_result"`
	Files              []string `json:"files"`
	Omitted            []string `json:"omitted,omitempty"`
	// Unfinished: taken from a capsule stopped before it froze; never
	// frozen, verified or approved.
	Unfinished bool `json:"unfinished,omitempty"`
}

// carriedPatchOffer is the decision for one assignment. Status is "" when
// there is nothing to say.
type carriedPatchOffer struct {
	Schema       string `json:"schema"`
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`

	PatchSHA256        string   `json:"patch_sha256,omitempty"`
	Size               int      `json:"size,omitempty"`
	BaseRef            string   `json:"base_ref,omitempty"`
	RetryBaseRef       string   `json:"retry_base_ref,omitempty"`
	BaseMoved          bool     `json:"base_moved,omitempty"`
	CandidateID        string   `json:"candidate_id,omitempty"`
	SourceAssignmentID string   `json:"source_assignment_id,omitempty"`
	SourceAttempt      uint64   `json:"source_attempt,omitempty"`
	SourceResult       string   `json:"source_result,omitempty"`
	Files              []string `json:"files,omitempty"`
	Omitted            []string `json:"omitted,omitempty"`
	OlderCandidates    []string `json:"older_candidates,omitempty"`

	// The source round's operation when it finished a candidate that did not
	// apply, and why (bounded).
	SourceOperationID    string `json:"source_operation_id,omitempty"`
	SourceOperationState string `json:"source_operation_state,omitempty"`
	SourceReason         string `json:"source_reason,omitempty"`
	// SourceUnfinished: the source round was stopped before it froze and its
	// work was kept from its capsule; SourceReason is why it was stopped.
	SourceUnfinished bool `json:"source_unfinished,omitempty"`
	// OlderFrozen* name the newest older round that froze a candidate, when
	// a newer stopped round outranked it; StartedFrom is whether that
	// stopped round was itself offered the candidate as its carried work.
	OlderFrozenCandidateID    string `json:"older_frozen_candidate_id,omitempty"`
	OlderFrozenAssignmentID   string `json:"older_frozen_assignment_id,omitempty"`
	OlderFrozenResult         string `json:"older_frozen_result,omitempty"`
	OlderFrozenOperationState string `json:"older_frozen_operation_state,omitempty"`
	OlderFrozenStartedFrom    bool   `json:"older_frozen_started_from,omitempty"`
	// OutrankedStopped are the assignments of newer stopped rounds passed
	// over because they did not start from the carried candidate.
	OutrankedStopped []string `json:"outranked_stopped,omitempty"`
	OutrankedBy      string   `json:"outranked_by,omitempty"`
	// SourcePending: the trajectory's newest work still has an open
	// self-development operation (SourceOperationState); nothing is carried
	// and the refusal is never recorded, so a replay decides again.
	SourcePending bool `json:"source_pending,omitempty"`
	// ConflictPaths are the files that do not apply to a moved base, and
	// CleanPaths the ones that still would, on a refusal for that cause.
	ConflictPaths []string `json:"conflict_paths,omitempty"`
	CleanPaths    []string `json:"clean_paths,omitempty"`

	// Dir is the staged directory; derived from the root and the digest on
	// every read, never trusted from disk.
	Dir string `json:"-"`
}

// carriedUnfinishedResult is the source result named for a round stopped
// before it froze.
const carriedUnfinishedResult = "stopped before it froze"

// carriedReasonMaxBytes bounds the rejection or failure reason in the offer.
const carriedReasonMaxBytes = 1500

// roundCarriable is whether a round's candidate may be carried: its
// operation was rejected or failed, or it opened no operation and did not
// finish (a failed round, as before). Any other operation state never
// carries, whatever the round reported.
func roundCarriable(entry carriedCandidate) bool {
	switch entry.OperationState {
	case selfdev.StateRejected, selfdev.StateFailed:
		return true
	case "":
		return entry.Result != types.EngineeringResultCompleted
	default:
		return false
	}
}

// carriedOperationOpen is whether a round's operation is still unsettled:
// the same states that refuse a new operation on the computer.
func carriedOperationOpen(state string) bool {
	for _, open := range selfdev.OpenStates {
		if state == open {
			return true
		}
	}
	return false
}

// carrySelection is the carry's choice for one round.
type carrySelection struct {
	// Chosen is the round to carry, nil when nothing is.
	Chosen *carriedCandidate
	// Older are the candidates older than Chosen, named for reference.
	Older []string
	// Pending is the trajectory's newest work when its operation is still
	// open: the round waits for it (admission) and nothing is carried.
	Pending *carriedCandidate
	// Outranked are the assignments of newer stopped rounds passed over
	// because they did not descend from the newest candidate.
	Outranked []string
	// OutrankedBy is the candidate the Outranked rounds did not start from.
	OutrankedBy string
}

// selectCarriedCandidate chooses the one round to carry among the changed
// implementation rounds that are not the retry's own, newest first.
//
//   - Pending: when the newest candidate, or a stopped round newer than it,
//     has an open operation, that round is Pending and nothing is chosen.
//   - Rank: when the newest candidate is carriable (rejected, failed), it
//     outranks every newer stopped round that does not descend from it; the
//     newest of the candidate and its descendants is chosen. viewed returns
//     the candidate a round's own recorded offer mounted, "" when none
//     (carriedOfferParent).
//   - Otherwise the newest round is chosen if it is carriable; when it is not
//     (it finished and was never proposed, or its operation applied) the
//     chain ends and nothing is carried.
func selectCarriedCandidate(entries []carriedCandidate, selfAssignmentID string, viewed func(carriedCandidate) string) carrySelection {
	var eligible []carriedCandidate
	for _, entry := range entries {
		if entry.AssignmentID == selfAssignmentID || entry.Kind != types.EngineeringAssignmentImplementation || strings.TrimSpace(entry.CandidateRef) == "" {
			continue
		}
		eligible = append(eligible, entry)
	}
	if len(eligible) == 0 {
		return carrySelection{}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if !eligible[i].CreatedAt.Equal(eligible[j].CreatedAt) {
			return eligible[i].CreatedAt.After(eligible[j].CreatedAt)
		}
		return eligible[i].CandidateID > eligible[j].CandidateID
	})
	newest := -1
	for i, entry := range eligible {
		if !entry.Unfinished {
			newest = i
			break
		}
	}
	limit := len(eligible) - 1
	if newest >= 0 {
		limit = newest
	}
	for i := 0; i <= limit; i++ {
		if carriedOperationOpen(eligible[i].OperationState) {
			pending := eligible[i]
			return carrySelection{Pending: &pending}
		}
	}
	chosen := 0
	var outranked []string
	var outrankedBy string
	if newest > 0 && roundCarriable(eligible[newest]) {
		chosen = newest
		for i := 0; i < newest; i++ {
			if carriedDescends(eligible[i], eligible[newest].CandidateID, eligible, viewed) {
				chosen = i
				break
			}
		}
		for i := 0; i < chosen; i++ {
			outranked = append(outranked, eligible[i].AssignmentID)
		}
		if len(outranked) > 0 {
			outrankedBy = eligible[newest].CandidateID
		}
	}
	if !roundCarriable(eligible[chosen]) {
		return carrySelection{}
	}
	var older []string
	for _, entry := range eligible[chosen+1:] {
		older = append(older, entry.CandidateID)
	}
	return carrySelection{Chosen: &eligible[chosen], Older: older, Outranked: outranked, OutrankedBy: outrankedBy}
}

// carriedDescends is whether a stopped round descends from the candidate
// targetID: parent (the candidate its recorded offer named) is the target,
// or is a stopped round in entries that descends from it.
func carriedDescends(entry carriedCandidate, targetID string, entries []carriedCandidate, parent func(carriedCandidate) string) bool {
	if !entry.Unfinished || parent == nil {
		return false
	}
	stopped := map[string]carriedCandidate{}
	for _, e := range entries {
		if e.Unfinished {
			stopped[e.CandidateID] = e
		}
	}
	seen := map[string]bool{}
	for current := entry; ; {
		named := parent(current)
		if named == "" || seen[named] {
			return false
		}
		if named == targetID {
			return true
		}
		seen[named] = true
		next, ok := stopped[named]
		if !ok {
			return false
		}
		current = next
	}
}

// carriedOfferParent reads the candidate a round's recorded offer mounted.
// Only an offered patch is descent: a refused one was never mounted, so the
// round started from the base, and the refusal need not recur for the next
// round (review 259, F1). Selection and the notice share this definition.
func carriedOfferParent(root string) func(carriedCandidate) string {
	return func(entry carriedCandidate) string {
		offer, ok := decodeCarriedOffer(root, entry.AssignmentID, entry.Attempt)
		if !ok || offer.Status != carriedOffered {
			return ""
		}
		return offer.CandidateID
	}
}

// carryHoldFrom is the round the assignment must wait for: the trajectory's
// newest work whose operation is still open, or nil. Only an implementation
// request is ever held.
func carryHoldFrom(ctx context.Context, history carriedPatchHistory, root string, assignment types.EngineeringAssignment) (*carriedCandidate, error) {
	binding := assignment.Binding
	if history == nil || binding.Kind != types.EngineeringAssignmentImplementation || strings.TrimSpace(binding.TrajectoryID) == "" {
		return nil, nil
	}
	entries, err := history.CarriedCandidates(ctx, binding.OwnerID, binding.ComputerID, binding.TrajectoryID)
	if err != nil {
		return nil, err
	}
	return selectCarriedCandidate(entries, assignment.AssignmentID, carriedOfferParent(root)).Pending, nil
}

// carryHoldFor is admission's read of carryHoldFrom on this runtime's tape.
func (rt *Runtime) carryHoldFor(ctx context.Context, assignment types.EngineeringAssignment) (*carriedCandidate, error) {
	if rt == nil || rt.store == nil {
		return nil, nil
	}
	return carryHoldFrom(ctx, storeCarriedHistory{rt: rt}, rt.selfdevUpdaterRoot, assignment)
}

// openOperationHold is the computer's oldest open self-development
// operation other than the request's own (keyed by its assignment, as
// openFreezeCandidate opens it), or nil. Freeze refuses per computer while
// one is open (selfdev.ErrOperationOpen), so an implementation request bound
// now could not freeze; it waits instead (review 259, F2). It reads the
// operations themselves, so an open operation whose round left no
// candidate holds too (F4). Only Freeze and the owner's rollback open an
// operation, and neither goes through engineering admission.
func (rt *Runtime) openOperationHold(ctx context.Context, assignment types.EngineeringAssignment) (*selfdev.Operation, error) {
	if rt == nil || rt.selfdevOperations == nil || assignment.Binding.Kind != types.EngineeringAssignmentImplementation {
		return nil, nil
	}
	open, err := rt.selfdevOperations.ListByStates(ctx, assignment.Binding.ComputerID, selfdev.OpenStates...)
	if err != nil {
		return nil, err
	}
	own := store.EngineeringFreezeIdempotencyKey(assignment.AssignmentID)
	for i := range open {
		if open[i].IdempotencyKey != own {
			return &open[i], nil
		}
	}
	return nil, nil
}

func carriedOfferPath(root, assignmentID string, attempt uint64) string {
	return filepath.Join(root, "carried", "offers", url.PathEscape(assignmentID)+fmt.Sprintf("-%d.json", attempt))
}

// offerCarriedPatch decides, once, what the assignment's engineer is told
// about an earlier round's work, and stages the patch when it is offered. It
// never fails: a problem carrying the work is a refusal the desk reads, and
// the retry starts clean.
func offerCarriedPatch(ctx context.Context, deps carriedPatchDeps, in carriedPatchInput) carriedPatchOffer {
	if existing := readCarriedOffer(deps.Root, in.AssignmentID, in.Attempt); existing.Status != "" {
		if existing.SourceUnfinished && !deps.CarryUnfinished {
			return carriedPatchOffer{}
		}
		return existing
	}
	if deps.History == nil || deps.Deriver == nil || strings.TrimSpace(deps.Root) == "" {
		return carriedPatchOffer{}
	}
	entries, err := deps.History.CarriedCandidates(ctx, in.OwnerID, in.ComputerID, in.TrajectoryID)
	if err != nil {
		log.Printf("agentcore: carried patch: read trajectory %s candidates: %v", in.TrajectoryID, err)
		return carriedPatchOffer{}
	}
	sel := selectCarriedCandidate(entries, in.AssignmentID, carriedOfferParent(deps.Root))
	if pending := sel.Pending; pending != nil {
		// Admission holds a round while this is so; an offer that meets it
		// anyway says so, and is not recorded, so a replay decides again.
		return carriedPatchOffer{
			Schema: carriedOfferSchema, AssignmentID: in.AssignmentID, Attempt: in.Attempt, Status: carriedRefused,
			Reason: fmt.Sprintf("the newest round on this trajectory, %s (assignment %s), has its self-development operation %s still %s",
				pending.CandidateID, pending.AssignmentID, pending.OperationID, pending.OperationState),
			BaseRef: pending.BaseRef, RetryBaseRef: in.RetryBaseRef, CandidateID: pending.CandidateID,
			SourceAssignmentID: pending.AssignmentID, SourceAttempt: pending.Attempt, SourceResult: string(pending.Result),
			SourceOperationID: pending.OperationID, SourceOperationState: pending.OperationState, SourcePending: true,
		}
	}
	chosen, older := sel.Chosen, sel.Older
	if chosen == nil {
		return carriedPatchOffer{}
	}
	offer := carriedPatchOffer{
		Schema: carriedOfferSchema, AssignmentID: in.AssignmentID, Attempt: in.Attempt,
		BaseRef: chosen.BaseRef, RetryBaseRef: in.RetryBaseRef, BaseMoved: chosen.BaseRef != in.RetryBaseRef,
		CandidateID: chosen.CandidateID, SourceAssignmentID: chosen.AssignmentID, SourceAttempt: chosen.Attempt,
		SourceResult: string(chosen.Result), OlderCandidates: older, OutrankedStopped: sel.Outranked, OutrankedBy: sel.OutrankedBy,
	}
	// Only a finished candidate (or a rejected one) is described by its
	// operation; a round that reported failed is unfinished work whatever
	// closed its operation.
	switch {
	case chosen.Unfinished:
		offer.SourceUnfinished, offer.SourceResult = true, carriedUnfinishedResult
		offer.SourceReason = boundCarriedReason(chosen.UnfinishedReason)
		nameOlderFrozenCandidate(&offer, deps.Root, entries, in.AssignmentID, *chosen)
	case chosen.OperationState != "" && (chosen.Result == types.EngineeringResultCompleted || chosen.OperationState == selfdev.StateRejected):
		offer.SourceOperationID, offer.SourceOperationState = chosen.OperationID, chosen.OperationState
		offer.SourceReason = boundCarriedReason(chosen.OperationReason)
	}
	refuse := func(persist bool, format string, args ...any) carriedPatchOffer {
		offer.Status, offer.Reason = carriedRefused, fmt.Sprintf(format, args...)
		if persist {
			writeCarriedOffer(deps.Root, offer)
		}
		return offer
	}
	var patch capsule.CarriedPatch
	if chosen.Unfinished {
		// Kept from the stopped capsule, already a patch: read and verified,
		// never derived from a tree.
		patch, err = loadUnfinishedPatch(deps.Root, strings.TrimPrefix(chosen.CandidateRef, carriedUnfinishedRefPrefix))
		if err != nil {
			return refuse(true, "the earlier round's kept unfinished patch could not be used (%v)", err)
		}
		patch.Omitted = chosen.UnfinishedOmitted
	} else {
		patch, err = deps.Deriver.DeriveCarriedPatch(ctx, chosen.BaseRef, chosen.CandidateRef)
	}
	switch {
	case errors.Is(err, capsule.ErrCarriedPatchEmpty):
		return carriedPatchOffer{}
	case errors.Is(err, capsule.ErrCarriedPatchTooLarge):
		return refuse(true, "the earlier round's patch is too large to carry (%v)", err)
	case errors.Is(err, capsule.ErrSubjectArtifactUnavailable):
		return refuse(true, "the earlier round's candidate artifact is unavailable (%v)", err)
	case errors.Is(err, capsule.ErrCarriedPatchOutsideSource):
		return refuse(true, "the earlier round's patch names a path outside the source tree (%v)", err)
	case err != nil:
		log.Printf("agentcore: carried patch: derive candidate %s: %v", chosen.CandidateID, err)
		return refuse(false, "the earlier round's patch could not be derived (%v)", err)
	}
	// The deriver is not trusted with the digest or the paths: both are
	// recomputed here before anything is staged.
	sum := sha256.Sum256(patch.Patch)
	if len(patch.Patch) == 0 || patch.SHA256 != hex.EncodeToString(sum[:]) {
		return refuse(true, "the earlier round's patch does not match its digest")
	}
	if len(patch.Patch) > capsule.CarriedPatchMaxBytes {
		return refuse(true, "the earlier round's patch is too large to carry (%d bytes, bound %d)", len(patch.Patch), capsule.CarriedPatchMaxBytes)
	}
	if _, err := capsule.CarriedPatchPaths(patch.Patch); err != nil {
		return refuse(true, "the earlier round's patch names a path outside the source tree (%v)", err)
	}
	if offer.BaseMoved {
		if err := deps.Deriver.CheckCarriedPatchApplies(ctx, patch, in.RetryBaseRef); err != nil {
			var conflict *capsule.CarriedPatchConflictError
			if errors.As(err, &conflict) {
				offer.ConflictPaths, offer.CleanPaths = conflict.Paths, conflict.Clean
			}
			return refuse(true, "the earlier round's base (%s) differs from this assignment's base (%s) and its patch does not apply: %v",
				carriedBaseLabel(chosen.BaseRef), carriedBaseLabel(in.RetryBaseRef), err)
		}
	}
	facts := carriedPatchFacts{
		Schema: carriedFactsSchema, PatchSHA256: patch.SHA256, Size: len(patch.Patch), BaseRef: chosen.BaseRef,
		CandidateID: chosen.CandidateID, CandidateRef: chosen.CandidateRef,
		SourceAssignmentID: chosen.AssignmentID, SourceAttempt: chosen.Attempt, SourceResult: offer.SourceResult,
		Files: patch.Files, Omitted: patch.Omitted, Unfinished: chosen.Unfinished,
	}
	dir, err := stageCarriedPatch(deps.Root, patch.Patch, facts)
	if err != nil {
		log.Printf("agentcore: carried patch: stage %s: %v", patch.SHA256, err)
		return refuse(false, "the earlier round's patch could not be staged (%v)", err)
	}
	offer.Status, offer.PatchSHA256, offer.Size, offer.Files, offer.Omitted, offer.Dir =
		carriedOffered, patch.SHA256, len(patch.Patch), patch.Files, patch.Omitted, dir
	writeCarriedOffer(deps.Root, offer)
	return offer
}

// stageCarriedPatch writes source.patch and carried.json under
// <root>/carried/<sha256>/, read only. An existing staging is kept when it
// verifies and replaced when it does not.
func stageCarriedPatch(root string, patch []byte, facts carriedPatchFacts) (string, error) {
	parent := filepath.Join(root, "carried")
	target := filepath.Join(parent, facts.PatchSHA256)
	if verifyStagedCarriedPatch(target, facts.PatchSHA256) == nil {
		return target, nil
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(facts)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(parent, ".stage-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, "source.patch"), patch, 0o444); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, "carried.json"), canonical, 0o444); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Lstat(target); err == nil {
		if err := os.RemoveAll(target); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", err
	}
	return target, nil
}

// verifyStagedCarriedPatch re-reads a staged patch: exactly the two files,
// the patch bytes hash to sha, they are within the bound and name only source
// paths, and carried.json is bound to the same digest.
func verifyStagedCarriedPatch(dir, sha string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 2 {
		return errCarriedPatchTampered
	}
	patch, err := os.ReadFile(filepath.Join(dir, "source.patch"))
	if err != nil || len(patch) == 0 || len(patch) > capsule.CarriedPatchMaxBytes {
		return errCarriedPatchTampered
	}
	sum := sha256.Sum256(patch)
	if hex.EncodeToString(sum[:]) != sha {
		return errCarriedPatchTampered
	}
	if _, err := capsule.CarriedPatchPaths(patch); err != nil {
		return fmt.Errorf("%w: %v", errCarriedPatchTampered, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "carried.json"))
	if err != nil {
		return errCarriedPatchTampered
	}
	var facts carriedPatchFacts
	if json.Unmarshal(raw, &facts) != nil || facts.Schema != carriedFactsSchema || facts.PatchSHA256 != sha {
		return errCarriedPatchTampered
	}
	return nil
}

func writeCarriedOffer(root string, offer carriedPatchOffer) {
	path := carriedOfferPath(root, offer.AssignmentID, offer.Attempt)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("agentcore: carried patch: record offer: %v", err)
		return
	}
	raw, err := json.Marshal(offer)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".offer-")
	if err != nil {
		log.Printf("agentcore: carried patch: record offer: %v", err)
		return
	}
	_, writeErr := tmp.Write(raw)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil || os.Chmod(tmp.Name(), 0o444) != nil || os.Rename(tmp.Name(), path) != nil {
		_ = os.Remove(tmp.Name())
		log.Printf("agentcore: carried patch: record offer for %s failed", offer.AssignmentID)
	}
}

// readCarriedOffer returns the recorded decision for an assignment, with an
// offered patch re-verified: bytes that no longer match are a refusal, never a
// mount or a name.
func readCarriedOffer(root, assignmentID string, attempt uint64) carriedPatchOffer {
	offer, ok := decodeCarriedOffer(root, assignmentID, attempt)
	if !ok {
		return carriedPatchOffer{}
	}
	if offer.Status != carriedOffered {
		offer.Dir = ""
		return offer
	}
	dir := filepath.Join(root, "carried", offer.PatchSHA256)
	if err := verifyStagedCarriedPatch(dir, offer.PatchSHA256); err != nil {
		offer.Status = carriedRefused
		offer.Reason = fmt.Sprintf("the staged patch (sha256 %s) no longer matches its digest and was not mounted (%v)", offer.PatchSHA256, err)
		offer.Dir = ""
		return offer
	}
	offer.Dir = dir
	return offer
}

// decodeCarriedOffer reads the recorded decision for an assignment as it was
// recorded, without re-verifying a staged patch.
func decodeCarriedOffer(root, assignmentID string, attempt uint64) (carriedPatchOffer, bool) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(assignmentID) == "" {
		return carriedPatchOffer{}, false
	}
	raw, err := os.ReadFile(carriedOfferPath(root, assignmentID, attempt))
	if err != nil {
		return carriedPatchOffer{}, false
	}
	var offer carriedPatchOffer
	if json.Unmarshal(raw, &offer) != nil || offer.Schema != carriedOfferSchema || offer.AssignmentID != assignmentID || offer.Attempt != attempt {
		return carriedPatchOffer{}, false
	}
	return offer, true
}

// nameOlderFrozenCandidate records, on an offer of a kept unfinished patch,
// the newest older round that froze a candidate, and whether the stopped
// round started from it (its recorded offer mounted that candidate's patch,
// directly or through stopped rounds). The carry already refuses a stopped
// round that did not descend from a carriable newest candidate
// (selectCarriedCandidate); this names what the engineer is building on.
func nameOlderFrozenCandidate(offer *carriedPatchOffer, root string, entries []carriedCandidate, selfAssignmentID string, chosen carriedCandidate) {
	var older *carriedCandidate
	for i := range entries {
		entry := entries[i]
		if entry.Unfinished || entry.AssignmentID == selfAssignmentID || entry.AssignmentID == chosen.AssignmentID ||
			entry.Kind != types.EngineeringAssignmentImplementation || strings.TrimSpace(entry.CandidateRef) == "" ||
			!entry.CreatedAt.Before(chosen.CreatedAt) {
			continue
		}
		if older == nil || entry.CreatedAt.After(older.CreatedAt) {
			older = &entries[i]
		}
	}
	if older == nil {
		return
	}
	offer.OlderFrozenCandidateID, offer.OlderFrozenAssignmentID = older.CandidateID, older.AssignmentID
	offer.OlderFrozenResult, offer.OlderFrozenOperationState = string(older.Result), older.OperationState
	offer.OlderFrozenStartedFrom = carriedDescends(chosen, older.CandidateID, entries, carriedOfferParent(root))
}

// carriedPatchDirForSpawn is the directory the retry's capsule mounts: only
// an offered, verified patch is ever mounted.
func carriedPatchDirForSpawn(offer carriedPatchOffer) string {
	if offer.Status == carriedOffered {
		return offer.Dir
	}
	return ""
}

func carriedBaseLabel(ref string) string {
	label := capsule.SourceArtifactCommit(ref)
	if label == "" {
		label = ref
	}
	if len(label) > 12 {
		label = label[:12]
	}
	return label
}

// carriedPatchNotice is what the engineer reads in its run context. The
// patch is described as untrusted; nothing here says it passed anything. A
// rejected (or failed) finished candidate is named as the starting point,
// with the reason, so the engineer revises the delta instead of rebuilding.
func carriedPatchNotice(offer carriedPatchOffer) string {
	switch offer.Status {
	case carriedRefused:
		var b strings.Builder
		if offer.SourcePending {
			fmt.Fprintf(&b, "Carried work: the newest round on this trajectory, %s (assignment %s), is not carried into this capsule: its self-development operation %s is still %s, so its decision (verify, approve or reject) has not settled. "+
				"This round started before that decision; while the operation is open a Freeze on this computer is refused. "+
				"/workspace/platform is the clean base tree; do the work from the objective.",
				offer.CandidateID, offer.SourceAssignmentID, offer.SourceOperationID, offer.SourceOperationState)
			return b.String()
		}
		if offer.SourceOperationState != "" {
			b.WriteString("Carried work: " + carriedOutcome(offer) + " Its source patch is not carried into this capsule: " + offer.Reason + ".")
		} else {
			b.WriteString("Carried work: an earlier round on this trajectory (assignment " + offer.SourceAssignmentID +
				") left unfinished source changes, but they are not carried into this capsule: " + offer.Reason + ".")
			if offer.SourceUnfinished && offer.SourceReason != "" {
				b.WriteString(" That round was stopped before it froze (the reason: \"" + offer.SourceReason + "\").")
			}
		}
		if len(offer.ConflictPaths) > 0 {
			b.WriteString(" The files that conflict with this base: " + carriedFileList(offer.ConflictPaths) + ".")
			if len(offer.CleanPaths) > 0 {
				b.WriteString(" The rest of that patch would still apply: " + carriedFileList(offer.CleanPaths) + ".")
			}
		}
		b.WriteString(" /workspace/platform is the clean base tree; do the work from the objective.")
		return b.String()
	case carriedOffered:
		var b strings.Builder
		switch {
		case offer.SourceUnfinished:
			stopped := "no reason was recorded"
			if offer.SourceReason != "" {
				stopped = "the reason: \"" + offer.SourceReason + "\""
			}
			fmt.Fprintf(&b, "Carried work: assignment %s (attempt %d) was stopped before it froze (%s). The source changes its capsule held when it stopped were kept as an unfinished patch, sha256 %s, %d bytes, changing: %s.",
				offer.SourceAssignmentID, offer.SourceAttempt, stopped, offer.PatchSHA256, offer.Size, carriedFileList(offer.Files))
			b.WriteString(" This is unfinished, unverified work: it was never frozen, built, tested or approved, and a file may stop mid-change. " +
				"It is mounted read only at /selfdev/carried/source.patch (its binding is /selfdev/carried/carried.json). " +
				"This capsule's /workspace/platform is still the clean base tree: nothing has been applied. " +
				"You may ignore it and do the work from the objective, or apply it from /workspace/platform with `git apply /selfdev/carried/source.patch`, " +
				"read what it left half done, and continue from there instead of starting over; re-run the build, tests and checks in this capsule after your last write, as for any change; Freeze judges only this capsule's own diff.")
			if offer.OlderFrozenCandidateID != "" {
				result := offer.OlderFrozenResult
				if result == "" {
					result = "no result"
				}
				fmt.Fprintf(&b, " An older round on this trajectory froze a candidate: %s (assignment %s, reported %s", offer.OlderFrozenCandidateID, offer.OlderFrozenAssignmentID, result)
				if offer.OlderFrozenOperationState != "" {
					fmt.Fprintf(&b, ", its self-development operation %s", offer.OlderFrozenOperationState)
				}
				b.WriteString("). It is not carried: the newest round's work is carried, and this kept patch is newer.")
				if offer.OlderFrozenStartedFrom {
					b.WriteString(" The stopped round was offered that candidate's patch as its starting point, so this patch may already contain it.")
				} else {
					b.WriteString(" The stopped round did not start from it, so this patch does not contain that candidate's work; if that candidate is the better start, ignore this patch and do the work from the objective.")
				}
			}
		case offer.SourceOperationState != "":
			fmt.Fprintf(&b, "Carried work: %s Its source patch, sha256 %s, %d bytes, changing: %s, is the starting point for this assignment.",
				carriedOutcome(offer), offer.PatchSHA256, offer.Size, carriedFileList(offer.Files))
			b.WriteString(" It is mounted read only at /selfdev/carried/source.patch (its binding is /selfdev/carried/carried.json); " +
				"this capsule's /workspace/platform is the clean base tree until you apply it. " +
				"Apply it from /workspace/platform with `git apply /selfdev/carried/source.patch`, then change only what this assignment's objective (the new directive) and the reason above ask for; do not rebuild the rest. " +
				"The patch is not trusted and carries no test results: re-run the build, tests and checks in this capsule after your last write; Freeze judges only this capsule's own diff.")
		default:
			fmt.Fprintf(&b, "Carried work: assignment %s (attempt %d, reported %s) left an unfinished source patch, sha256 %s, %d bytes, changing: %s.",
				offer.SourceAssignmentID, offer.SourceAttempt, offer.SourceResult, offer.PatchSHA256, offer.Size, carriedFileList(offer.Files))
			b.WriteString(" It is mounted read only at /selfdev/carried/source.patch (its binding is /selfdev/carried/carried.json). " +
				"This capsule's /workspace/platform is still the clean base tree: nothing has been applied. " +
				"The patch is not trusted and carries no test results. You may ignore it and do the work from the objective, or apply it from /workspace/platform with " +
				"`git apply /selfdev/carried/source.patch` and then re-run the build, tests and checks in this capsule after your last write, as for any change; Freeze judges only this capsule's own diff.")
		}
		if offer.BaseMoved {
			fmt.Fprintf(&b, " The base moved: that round started from %s and this capsule starts from %s; the patch was checked and applies to this base.",
				carriedBaseLabel(offer.BaseRef), carriedBaseLabel(offer.RetryBaseRef))
		}
		if len(offer.Omitted) > 0 {
			b.WriteString(" Left out of the patch: " + carriedFileList(offer.Omitted) + ".")
		}
		if len(offer.OlderCandidates) > 0 {
			b.WriteString(" Older rounds' candidates, not carried: " + strings.Join(offer.OlderCandidates, ", ") + ".")
		}
		if len(offer.OutrankedStopped) > 0 {
			if len(offer.OutrankedStopped) == 1 {
				fmt.Fprintf(&b, " A newer round on this trajectory (assignment %s) was stopped before it froze, but it did not start from the newest candidate (%s), so its kept unfinished work is not carried: the work carried here builds on that candidate.",
					offer.OutrankedStopped[0], offer.OutrankedBy)
			} else {
				fmt.Fprintf(&b, " Newer rounds on this trajectory (assignments %s) were stopped before they froze, but they did not start from the newest candidate (%s), so their kept unfinished work is not carried: the work carried here builds on that candidate.",
					strings.Join(offer.OutrankedStopped, ", "), offer.OutrankedBy)
			}
		}
		return b.String()
	}
	return ""
}

// carriedOutcome says what became of a finished candidate that did not
// apply, with its reason.
func carriedOutcome(offer carriedPatchOffer) string {
	what := "was rejected"
	if offer.SourceOperationState == selfdev.StateFailed {
		what = "failed before it applied"
	}
	reason := "no reason could be read"
	if offer.SourceReason != "" {
		reason = "the reason: \"" + offer.SourceReason + "\""
	}
	return fmt.Sprintf("assignment %s (attempt %d) finished a candidate on this trajectory, and its self-development operation %s %s; %s.",
		offer.SourceAssignmentID, offer.SourceAttempt, offer.SourceOperationID, what, reason)
}

// boundCarriedReason puts a reason on one line within carriedReasonMaxBytes,
// cut on a rune boundary.
func boundCarriedReason(reason string) string {
	reason = strings.Join(strings.Fields(reason), " ")
	if len(reason) <= carriedReasonMaxBytes {
		return reason
	}
	cut := carriedReasonMaxBytes
	for cut > 0 && !utf8.RuneStart(reason[cut]) {
		cut--
	}
	return reason[:cut] + "…"
}

func carriedFileList(files []string) string {
	const limit = 20
	if len(files) <= limit {
		return strings.Join(files, ", ")
	}
	return strings.Join(files[:limit], ", ") + fmt.Sprintf(", and %d more", len(files)-limit)
}

// storeCarriedHistory reads a trajectory's candidates from the tape with the
// bits of their rounds the offer needs. A candidate whose round or report
// cannot be read is left out.
type storeCarriedHistory struct{ rt *Runtime }

func (h storeCarriedHistory) CarriedCandidates(ctx context.Context, ownerID, computerID, trajectoryID string) ([]carriedCandidate, error) {
	candidates, err := h.rt.store.ListEngineeringSubjectCandidates(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return nil, err
	}
	out := make([]carriedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		assignment, err := h.rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, candidate.AssignmentID, candidate.Attempt)
		if err != nil {
			continue
		}
		report, err := h.rt.store.GetEngineeringAssignmentReport(ctx, ownerID, computerID, candidate.SourceReportID)
		if err != nil {
			continue
		}
		out = append(out, carriedCandidate{
			CandidateID: candidate.CandidateID, CandidateRef: candidate.ArtifactRef, AssignmentID: candidate.AssignmentID,
			Attempt: candidate.Attempt, BaseRef: assignment.Binding.SourceArtifactRef, Kind: assignment.Binding.Kind,
			Result: report.Result, CreatedAt: candidate.CreatedAt,
		})
	}
	if h.rt.carryUnfinishedOn(ctx, ownerID, computerID) {
		out = mergeUnfinishedRounds(out, readUnfinishedRounds(h.rt.selfdevUpdaterRoot, trajectoryID), trajectoryID,
			func(assignmentID string, attempt uint64) (types.EngineeringAssignment, error) {
				return h.rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, attempt)
			})
	}
	var operations carriedOperationReader
	if h.rt.selfdevOperations != nil {
		operations = h.rt.selfdevOperations
	}
	rejection := func(ctx context.Context, op selfdev.Operation) string {
		return carriedRejectionReason(ctx, h.rt.store, h.rt.eventPayloadReader, op)
	}
	if err := annotateCarriedOperations(ctx, operations, rejection, computerID, out); err != nil {
		return nil, err
	}
	return out, nil
}

type carriedOperationReader interface {
	GetByIdempotency(ctx context.Context, computerID, idempotencyKey string) (selfdev.Operation, bool, error)
}

type carriedDecisionEvents interface {
	EventByDigest(ctx context.Context, computerID, eventDigest string) (computerevent.Event, bool, error)
}

// annotateCarriedOperations joins each round to its own self-development
// operation (keyed by its assignment, as openFreezeCandidate opens it). A
// failed read is an error, so the offer stays undecided and the retry clean;
// a round with no operation keeps the empty state. With no operation store
// nothing is joined.
func annotateCarriedOperations(ctx context.Context, operations carriedOperationReader, rejection func(context.Context, selfdev.Operation) string, computerID string, entries []carriedCandidate) error {
	if operations == nil {
		return nil
	}
	for i := range entries {
		op, found, err := operations.GetByIdempotency(ctx, computerID, store.EngineeringFreezeIdempotencyKey(entries[i].AssignmentID))
		if err != nil {
			return fmt.Errorf("read the operation of assignment %s: %w", entries[i].AssignmentID, err)
		}
		if !found {
			continue
		}
		entries[i].OperationID, entries[i].OperationState = op.OperationID, op.State
		switch op.State {
		case selfdev.StateRejected:
			if rejection != nil {
				entries[i].OperationReason = rejection(ctx, op)
			}
		case selfdev.StateFailed:
			entries[i].OperationReason = op.TerminalError
		}
	}
	return nil
}

// carriedRejectionReason reads the reason the decision pinned as its output
// payload (decideSelfDevelopment): only from this operation's own rejection
// event, and only that event's single output. Anything else is no reason.
func carriedRejectionReason(ctx context.Context, events carriedDecisionEvents, payloads computerevent.ArtifactReader, op selfdev.Operation) string {
	if events == nil || payloads == nil || strings.TrimSpace(op.DecisionEvent) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	event, found, err := events.EventByDigest(ctx, op.ComputerID, op.DecisionEvent)
	if err != nil || !found || event.EventKind != computerevent.EventEffectRejected || event.ParentEventID != op.OperationID || len(event.OutputArtifactRefs) != 1 {
		return ""
	}
	ref, err := computerevent.ParseArtifactRef(event.OutputArtifactRefs[0])
	if err != nil {
		return ""
	}
	raw, err := payloads.FetchPayload(ctx, op.ComputerID, ref.Digest().String())
	if err != nil || !utf8.Valid(raw) {
		return ""
	}
	return string(raw)
}

// offerCarriedPatchForAssignment is the spawn saga's call: the offer for an
// implementation assignment, or nothing for any other kind or a runtime with
// no store, executor or updater root.
func (rt *Runtime) offerCarriedPatchForAssignment(ctx context.Context, assignment types.EngineeringAssignment, retryBaseRef string) carriedPatchOffer {
	binding := assignment.Binding
	if rt == nil || rt.store == nil || rt.capsuleExecutor == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" ||
		binding.Kind != types.EngineeringAssignmentImplementation {
		return carriedPatchOffer{}
	}
	return offerCarriedPatch(ctx, carriedPatchDeps{History: storeCarriedHistory{rt: rt}, Deriver: rt.capsuleExecutor, Root: rt.selfdevUpdaterRoot,
		CarryUnfinished: rt.carryUnfinishedOn(ctx, binding.OwnerID, binding.ComputerID)},
		carriedPatchInput{OwnerID: binding.OwnerID, ComputerID: binding.ComputerID, TrajectoryID: binding.TrajectoryID,
			AssignmentID: assignment.AssignmentID, Attempt: binding.Attempt, RetryBaseRef: retryBaseRef})
}

// carriedPatchNoticeForRun is the run-context text for an implementation
// assignment's engineer, from the recorded decision.
func (rt *Runtime) carriedPatchNoticeForRun(rec *types.RunRecord) string {
	if rt == nil || rec == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" ||
		metadataStringValue(rec.Metadata, "assignment_kind") != string(types.EngineeringAssignmentImplementation) {
		return ""
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	offer := readCarriedOffer(rt.selfdevUpdaterRoot, assignmentID, attempt)
	if offer.SourceUnfinished && !rt.carryUnfinishedOn(context.Background(), rec.OwnerID, rec.ComputerID) {
		return ""
	}
	return carriedPatchNotice(offer)
}
