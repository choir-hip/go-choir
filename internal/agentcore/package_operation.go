package agentcore

// The package branch of the self-development materializer
// (docs/design/package-operation-decision-2026-11-06.md 2.3 and 2.5). A
// package operation is decided by the same consensus as a platform one, but
// its effect is not a release of the platform: it publishes a Library release
// by that decision, installs it on its own computer and records
// package_applied. It has no updater journal, no host build and no route.
//
// The order is fixed so a failure after the release is public can only be a
// crash: draft once and persist it; build, check the uses and the install
// review before anything is public; publish; install; record. A crash never
// resumes the branch (the owner rule, "Restarts End Work (Crash) Or Resume It
// (Planned Update)"): a materializing package operation found by any pass or
// boot is settled from facts, never driven again.

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

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/packagepath"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// PackagePublisher is what the package branch needs of corpusd: publish a
// decided operation's release. selfdev.GuestCredentials implements it; tests
// use a fake.
type PackagePublisher interface {
	PublishPackageByDecision(ctx context.Context, request platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error)
}

// PackageReleaseLister is the optional second half: the releases the Library
// holds for one of this computer's operations (the crash settle's fact). No
// guest credential method implements it yet; without it the settle falls back
// to the install records alone.
type PackageReleaseLister interface {
	PackageReleasesForOperation(ctx context.Context, operationID string) ([]PackageReleaseRef, error)
}

// PackageReleaseRef is a release the Library lists for an operation.
type PackageReleaseRef struct {
	ReleaseID   string
	OperationID string
}

// packagePublishClient is the runtime's publisher: the injected one (a test),
// otherwise the guest's control credentials.
func (rt *Runtime) packagePublishClient() PackagePublisher {
	if rt.packagePublisher != nil {
		return rt.packagePublisher
	}
	if rt.selfdevControl != nil {
		return rt.selfdevControl
	}
	return nil
}

// operationKindOfPatch classifies a source patch: selfdev.KindPackage for a
// patch entirely under one package root, otherwise "" (a platform operation).
func operationKindOfPatch(patch []byte) string {
	if len(patch) == 0 {
		return ""
	}
	if paths, err := hostbuild.PatchPaths(patch); err == nil && packagepath.Classify(paths).PackageOnly() {
		return selfdev.KindPackage
	}
	return ""
}

// stagedOperationKind is the kind of the source patch Freeze staged under
// root (its sha is the bundle's), "" when there is none.
func stagedOperationKind(root, patchSHA string) string {
	if strings.TrimSpace(patchSHA) == "" {
		return ""
	}
	patch, err := os.ReadFile(filepath.Join(root, "source.patch"))
	if err != nil || computerevent.DigestBytes(patch) != patchSHA {
		return ""
	}
	return operationKindOfPatch(patch)
}

// packageOperationDraft is the draft a package operation publishes, written
// once and read by every later step and retry: nothing re-drafts. It holds the
// record, its patches and the digests of the media (whose bytes stay in the
// frozen bundle).
type packageOperationDraft struct {
	Schema         string          `json:"schema"`
	OperationID    string          `json:"operation_id"`
	BundleDigest   string          `json:"bundle_digest"`
	DecisionDigest string          `json:"decision_digest"`
	Package        string          `json:"package"`
	ReleaseID      string          `json:"release_id"`
	PlatformBase   string          `json:"platform_base"`
	Record         json.RawMessage `json:"record"`
	TreePatch      []byte          `json:"tree_patch"`
	ReviewPatch    []byte          `json:"review_patch"`
	ParentPatch    []byte          `json:"parent_patch,omitempty"`
	SourcePatch    []byte          `json:"source_patch"`
	MediaDigests   []string        `json:"media_digests"`
}

const packageOperationDraftSchema = "choir-package-operation-draft-v1"

func (rt *Runtime) packageDraftPath(operationID string) string {
	return filepath.Join(rt.selfdevUpdaterRoot, "package-drafts", operationID+".json")
}

// loadPackageDraft reads the persisted draft of an operation; found is false
// when none was written.
func (rt *Runtime) loadPackageDraft(operation selfdev.Operation) (packageOperationDraft, bool, error) {
	raw, err := os.ReadFile(rt.packageDraftPath(operation.OperationID))
	if errors.Is(err, os.ErrNotExist) {
		return packageOperationDraft{}, false, nil
	}
	if err != nil {
		return packageOperationDraft{}, false, err
	}
	var draft packageOperationDraft
	if err := json.Unmarshal(raw, &draft); err != nil || draft.Schema != packageOperationDraftSchema ||
		draft.OperationID != operation.OperationID || draft.BundleDigest != operation.BundleDigest {
		return packageOperationDraft{}, false, fmt.Errorf("the persisted package draft of %s is unreadable or names another operation", operation.OperationID)
	}
	return draft, true, nil
}

// savePackageDraft writes the draft atomically (temp file, sync, rename).
func (rt *Runtime) savePackageDraft(draft packageOperationDraft) error {
	raw, err := json.Marshal(draft)
	if err != nil {
		return err
	}
	path := rt.packageDraftPath(draft.OperationID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".draft-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// reconcilePackageOperation is one materializer pass over one package
// operation. An accepted one starts (unless a crash boot marked it: then it
// is settled, nothing was published); a materializing one is settled from
// facts, never driven again.
func (rt *Runtime) reconcilePackageOperation(ctx context.Context, operation selfdev.Operation) error {
	switch operation.State {
	case selfdev.StateAccepted:
		if rt.selfdevCrashInterruptedOp(operation.OperationID) {
			return rt.settlePackageOperation(ctx, operation, false)
		}
		return rt.materializePackageOperation(ctx, operation)
	case selfdev.StateMaterializing:
		return rt.settlePackageOperation(ctx, operation, false)
	}
	return nil
}

// packageDecisionDigest is the digest of the consensus decision that accepted
// the operation. A package operation is decided by consensus only.
func (rt *Runtime) packageDecisionDigest(ctx context.Context, operation selfdev.Operation) (string, error) {
	transition, found, err := rt.store.FinalizedDecisionForOperation(ctx, operation.ComputerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("operation %s has no finalized decision on the tape", operation.OperationID)
	}
	digest, ok := decisionpolicy.ReceiptDigestFromAuthority(transition.Request.Event.AuthorityRef)
	if !ok {
		return "", fmt.Errorf("a package operation is published only by a consensus decision")
	}
	return digest, nil
}

// materializePackageOperation runs the branch for an accepted package
// operation: materializing, draft once, check, publish, install, record.
func (rt *Runtime) materializePackageOperation(ctx context.Context, operation selfdev.Operation) error {
	defer rt.notifyTextureOfApplyFailure(ctx, operation)
	ownerID, computerID := rt.selfdevRouteOwnerID, operation.ComputerID
	var err error
	// The draft, the checked build and the install review gate run while the
	// operation is still accepted: a review management owes keeps it accepted
	// (errInstallReviewPending, the pattern of errCandidateShotPending), and
	// only a passed gate moves it to materializing, whose passes settle from
	// facts and never drive the branch again.
	fail := func(format string, args ...any) error {
		return rt.failPackageOperation(ctx, operation, fmt.Sprintf(format, args...))
	}

	// 2. Draft once and persist.
	draft, found, err := rt.loadPackageDraft(operation)
	if err != nil {
		return fail("%v", err)
	}
	var drafted *packageDraftResult
	if !found {
		decisionDigest, err := rt.packageDecisionDigest(ctx, operation)
		if err != nil {
			return fail("%v", err)
		}
		_, patch, err := rt.boundSourcePatch(operation)
		if err != nil {
			return fail("the package operation's source patch is unavailable: %v", err)
		}
		paths, err := hostbuild.PatchPaths(patch)
		if err != nil {
			return fail("the package operation's patch is unreadable: %v", err)
		}
		split := packagepath.Classify(paths)
		if !split.PackageOnly() {
			return fail("the operation is not a package-only change")
		}
		name := split.Name()
		if !apppackageDraftableName(name) {
			return fail("package %s is a built-in app; change it through a platform release", name)
		}
		// An update follows the owner's track head, which the decision's
		// package block names (the head corpusd drafted over); a draft with no
		// head would be a new package and corpusd answers track_moved.
		headID, headTree, err := rt.packageDecisionHead(ctx, operation, decisionDigest)
		if err != nil {
			return fail("the decision's track head is unreadable: %v", err)
		}
		var failure *packageDraftFailure
		if drafted, failure = rt.draftPackageRelease(ctx, ownerID, computerID, operation, packageDraftInput{Name: name, HeadReleaseID: headID, HeadTree: headTree}); failure != nil {
			return fail("the release cannot be drafted: %s", failure.Error())
		}
		draft = packageOperationDraft{
			Schema: packageOperationDraftSchema, OperationID: operation.OperationID, BundleDigest: operation.BundleDigest,
			DecisionDigest: decisionDigest, Package: name, ReleaseID: drafted.ReleaseID, PlatformBase: drafted.PlatformBase,
			Record: drafted.Canonical, TreePatch: drafted.TreePatch, ReviewPatch: drafted.ReviewPatch, ParentPatch: drafted.ParentPatch,
			SourcePatch: drafted.SourcePatch,
		}
		for _, item := range drafted.Media {
			draft.MediaDigests = append(draft.MediaDigests, item.SHA256)
		}
		if err := rt.savePackageDraft(draft); err != nil {
			return fail("the package draft cannot be persisted: %v", err)
		}
	}

	// 3. Check before anything is public: the build, the uses, the install
	// review. A refusal closes the operation failed and nothing publishes.
	rec, err := pkgrelease.Decode(draft.Record)
	if err != nil {
		return fail("the persisted release record is invalid: %v", err)
	}
	if err := pkgrelease.VerifyID(rec, draft.ReleaseID); err != nil {
		return fail("the persisted release record does not hash to its id: %v", err)
	}
	reviewRefusal := func(refusal *installRefusal) error {
		switch refusal.reason {
		case reasonInstallReviewOwed:
			return fmt.Errorf("%w: operation %s release %s", errInstallReviewPending, operation.OperationID, draft.ReleaseID)
		case reasonInstallReviewTimedOut:
			return fail("%s", installReviewNotJudged)
		case installreview.ReasonRejected:
			return fail("install review rejected: %s", refusal.detail)
		}
		return fail("refused before publish: %s", refusal.Error())
	}
	// A pass over a review already owed reads the owed record and the ledger and
	// keeps the clock; it does not rebuild the release every 60 seconds.
	if refusal := rt.operationReviewPending(ctx, ownerID, computerID, operation, draft.ReleaseID); refusal != nil {
		return reviewRefusal(refusal)
	}
	prior, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return fail("the installs cannot be read: %v", err)
	}
	if _, _, refusal := rt.buildCheckedModule(ctx, ownerID, computerID, draft.ReleaseID, rec, draft.TreePatch, currentPackageInstalls(prior)[draft.Package], installReviewCtx{operation: &operation}); refusal != nil {
		return reviewRefusal(refusal)
	}
	if operation, err = rt.startSelfDevelopmentMaterialization(ctx, operation); err != nil {
		return err
	}

	// 4. Publish by decision.
	publisher := rt.packagePublishClient()
	if publisher == nil {
		return fail("this computer has no publish authority")
	}
	request, err := rt.packagePublishRequest(operation, draft)
	if err != nil {
		return fail("%v", err)
	}
	response, status, err := publisher.PublishPackageByDecision(ctx, request)
	if err != nil {
		var refused *selfdev.PackagePublishError
		if errors.As(err, &refused) {
			// corpusd answered and refused: nothing is public. The reason
			// (publisher_address_required, decision_expired, decision_consumed,
			// draft_digest_mismatch, media_not_frozen, track_moved, ...) is the
			// operation's.
			return fail("the publish was refused (%d, %s): %s", refused.Status, refused.Reason, refused.Message)
		}
		// No answer: the release may be public. Settle from facts, never retry.
		log.Printf("selfdev package: operation %s: publish unanswered (%d): %v; settling from facts", operation.OperationID, status, err)
		return rt.settlePackageOperation(ctx, operation, true)
	}
	if response.ReleaseID != draft.ReleaseID {
		return fail("published as %s but the draft names %s; install it from the Library", response.ReleaseID, draft.ReleaseID)
	}

	// 5. Install here, trigger selfdev, from the persisted draft.
	view, _, refusal := rt.installPackage(ctx, ownerID, computerID, packageInstallRequest{
		ReleaseID: draft.ReleaseID, Record: draft.Record, TreePatch: draft.TreePatch,
		change: installChange{trigger: store.PackageInstallTriggerSelfdev},
	})
	if refusal != nil {
		return fail("published as %s; not installed here (%s); install it from the Library", draft.ReleaseID, refusal.Error())
	}

	// 6. Record.
	return rt.recordPackageApplied(ctx, operation, draft, view.InstallID)
}

// apppackageDraftableName: a built-in app is changed by a platform release,
// never published from a package operation.
func apppackageDraftableName(name string) bool { return !apppackage.IsBuiltinPackage(name) }

// packagePublishRequest assembles the publish request from the persisted
// draft and the frozen bundle's media, which must be exactly the digests the
// draft recorded.
func (rt *Runtime) packagePublishRequest(operation selfdev.Operation, draft packageOperationDraft) (platform.PackagePublishByDecisionRequest, error) {
	items, data, err := rt.operationMedia(operation)
	if err != nil {
		return platform.PackagePublishByDecisionRequest{}, fmt.Errorf("the release media is unavailable: %v", err)
	}
	byDigest := map[string]int{}
	for i, item := range items {
		byDigest[item.SHA256] = i
	}
	request := platform.PackagePublishByDecisionRequest{
		DecisionDigest: draft.DecisionDigest, OperationID: operation.OperationID, BundleDigest: operation.BundleDigest, ReleaseID: draft.ReleaseID,
		Record: draft.Record, TreePatch: draft.TreePatch, ReviewPatch: draft.ReviewPatch, ParentPatch: draft.ParentPatch,
		SourcePatch: draft.SourcePatch, ClaimedPlatformBase: draft.PlatformBase,
	}
	for _, digest := range draft.MediaDigests {
		i, ok := byDigest[digest]
		if !ok {
			return request, fmt.Errorf("release media %s is no longer in the frozen bundle", digest)
		}
		request.Media = append(request.Media, data[i])
	}
	return request, nil
}

// failPackageOperation closes a package operation failed: the
// materialization_failed event that discharges its pending transition (desired
// returns to effective), then the row.
func (rt *Runtime) failPackageOperation(ctx context.Context, operation selfdev.Operation, cause string) error {
	log.Printf("selfdev package: operation %s failed: %s", operation.OperationID, cause)
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		return fmt.Errorf("package operation %s: event head unavailable", operation.OperationID)
	}
	if operation.DecisionEvent != "" && head.PendingTransitionRef == operation.DecisionEvent {
		if _, err := rt.appendMaterializationFailedEvent(ctx, operation, updater.ApplyResult{Outcome: "package_failed"}); err != nil {
			return err
		}
		if head, err = rt.store.Head(ctx, operation.ComputerID); err != nil || head == nil {
			return fmt.Errorf("package operation %s: event head unavailable", operation.OperationID)
		}
	}
	_, err = rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.DesiredHead, next.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
		next.TerminalError = cause
		return nil
	})
	return err
}

// settlePackageOperation settles an accepted (crash-marked) or materializing
// package operation from facts, without driving the branch again:
//   - nothing was persisted, or the Library lists no release for the
//     operation: failed, "interrupted by a restart";
//   - a release is listed and the install exists: package_applied;
//   - a release is listed and there is no install: failed, naming the release,
//     "install it from the Library".
func (rt *Runtime) settlePackageOperation(ctx context.Context, operation selfdev.Operation, unanswered bool) error {
	draft, persisted, err := rt.loadPackageDraft(operation)
	if err != nil {
		return err
	}
	if operation.State == selfdev.StateAccepted || !persisted {
		// Publish follows the persisted draft: with none, nothing is public.
		return rt.failPackageOperation(ctx, operation, selfdevInterruptedByRestart)
	}
	listed := ""
	lister, hasLister := rt.packagePublishClient().(PackageReleaseLister)
	if hasLister {
		releases, err := lister.PackageReleasesForOperation(ctx, operation.OperationID)
		if err != nil {
			// Unproven: the fate waits for a later pass; nothing is guessed.
			return fmt.Errorf("package operation %s: the Library lookup failed: %w", operation.OperationID, err)
		}
		for _, release := range releases {
			if release.OperationID == operation.OperationID && computerevent.IsSHA256(release.ReleaseID) {
				listed = release.ReleaseID
				break
			}
		}
	} else {
		// No lookup route: an install of the draft's release is proof it was
		// published; otherwise the fact is unknown.
		listed = draft.ReleaseID
		if !rt.packageInstalledRelease(ctx, operation, listed) {
			listed = ""
		}
	}
	if listed == "" {
		return rt.failPackageOperation(ctx, operation, packageUnlistedCause(draft, hasLister, unanswered))
	}
	ownerID, computerID := rt.selfdevRouteOwnerID, operation.ComputerID
	prior, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return fmt.Errorf("package operation %s: the installs cannot be read: %w", operation.OperationID, err)
	}
	for _, item := range currentPackageInstalls(prior) {
		if item.ReleaseID == listed {
			draft.ReleaseID = listed
			return rt.recordPackageApplied(ctx, operation, draft, item.InstallID)
		}
	}
	return rt.failPackageOperation(ctx, operation, "published as "+listed+"; not installed here; install it from the Library")
}

// packageUnlistedCause is the terminal error of a settle that found no listed
// release. With a lookup route, "no release listed" is a fact; without one, a
// persisted draft means the release may be public, and the cause says so
// instead of claiming it was not. A publish that went unanswered in the live
// pass was not interrupted by a restart.
func packageUnlistedCause(draft packageOperationDraft, hasLister, unanswered bool) string {
	switch {
	case hasLister && unanswered:
		return "the publish was not answered and the Library lists no release for this operation"
	case hasLister:
		return selfdevInterruptedByRestart
	case unanswered:
		return "the publish was not answered; the release may be published as " + draft.ReleaseID + ", install it from the Library if it is"
	default:
		return "interrupted; the release may be published as " + draft.ReleaseID + ", install it from the Library if it is"
	}
}

// packageAppliedResult is the package_applied event's payload.
type packageAppliedResult struct {
	OperationID string `json:"operation_id"`
	Package     string `json:"package"`
	ReleaseID   string `json:"release_id"`
	InstallID   string `json:"install_id"`
}

// recordPackageApplied appends package_applied (idempotent on the decision
// event; it discharges the pending transition) and moves the row to applied
// with the release and install ids in their own columns.
func (rt *Runtime) recordPackageApplied(ctx context.Context, operation selfdev.Operation, draft packageOperationDraft, installID string) error {
	idempotency := "selfdev-" + string(computerevent.EventPackageApplied) + "-" + operation.DecisionEvent
	if _, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, idempotency); err != nil {
		return err
	} else if !found {
		head, err := rt.store.Head(ctx, operation.ComputerID)
		if err != nil || head == nil {
			return fmt.Errorf("package operation %s: event head unavailable", operation.OperationID)
		}
		payload, err := computerevent.CanonicalJSON(packageAppliedResult{
			OperationID: operation.OperationID, Package: draft.Package, ReleaseID: draft.ReleaseID, InstallID: installID,
		})
		if err != nil {
			return err
		}
		eventID, err := computerevent.NewEventID()
		if err != nil {
			return err
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
			EventKind: computerevent.EventPackageApplied, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: idempotency, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ActorProfile: platformUpdateActor(), AuthorityRef: "guest-core:package-install", PrivacyClass: "owner",
			ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent,
			ResultingEffectiveCommitment: head.EffectiveStateCommitment,
			ReducerVersion:               computerevent.ReducerVersionV1,
		}
		if _, _, err := rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.package-applied+json", "owner"); err != nil {
			return err
		}
	}
	finalHead, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || finalHead == nil {
		return fmt.Errorf("package operation %s: event head unavailable", operation.OperationID)
	}
	_, err = rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, selfdev.StateMaterializing, selfdev.StateApplied, func(next *selfdev.Operation) error {
		next.DesiredHead, next.EffectiveHead = finalHead.DesiredEventHead, finalHead.EffectiveEventHead
		next.PackageReleaseID, next.PackageInstallID = draft.ReleaseID, installID
		return nil
	})
	return err
}

// revertPackageOperation is the package branch of the revert route: the
// uninstall of the install the operation made, the package_reverted event and
// the row rolled back. It never calls the updater and moves no platform head.
func (rt *Runtime) revertPackageOperation(ctx context.Context, ownerID, computerID string, reverted selfdev.Operation) (selfdev.Operation, int, error) {
	if reverted.PackageReleaseID == "" {
		return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("operation %s published no release", reverted.OperationID)
	}
	// Reverting an update uninstalls its release; it does not restore the
	// prior version (docs/design/package-operation-decision-2026-11-06.md 2.5).
	// A release no longer installed (replaced by a later install, or already
	// uninstalled) is recorded rolled back with that reason.
	note := ""
	if _, _, refusal := rt.uninstallPackageWith(ctx, ownerID, computerID, reverted.PackageReleaseID, installChange{trigger: store.PackageInstallTriggerSelfdev}); refusal != nil {
		if refusal.reason != "package_not_installed" {
			return selfdev.Operation{}, refusal.status, refusal
		}
		note = "the release " + reverted.PackageReleaseID + " was already superseded: it is no longer installed (replaced by a later install or uninstalled), so the revert uninstalled nothing"
	}
	// The event names the package_applied event it undoes.
	appliedEvent, found, err := rt.store.EventByIdempotency(ctx, computerID, "selfdev-"+string(computerevent.EventPackageApplied)+"-"+reverted.DecisionEvent)
	if err != nil || !found {
		return selfdev.Operation{}, http.StatusConflict, fmt.Errorf("the package_applied event of operation %s is not on the tape", reverted.OperationID)
	}
	appliedDigest, err := appliedEvent.Digest()
	if err != nil {
		return selfdev.Operation{}, http.StatusInternalServerError, err
	}
	idempotency := "selfdev-" + string(computerevent.EventPackageReverted) + "-" + reverted.OperationID
	if _, found, err := rt.store.EventByIdempotency(ctx, computerID, idempotency); err != nil {
		return selfdev.Operation{}, http.StatusInternalServerError, err
	} else if !found {
		eventID, err := computerevent.NewEventID()
		if err != nil {
			return selfdev.Operation{}, http.StatusInternalServerError, err
		}
		payload, err := computerevent.CanonicalJSON(map[string]string{
			"operation_id": reverted.OperationID, "release_id": reverted.PackageReleaseID, "install_id": reverted.PackageInstallID,
		})
		if err != nil {
			return selfdev.Operation{}, http.StatusInternalServerError, err
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: computerevent.EventPackageReverted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: idempotency, TrajectoryID: reverted.TrajectoryID, CapsuleID: reverted.CapsuleID,
			ActorProfile: platformUpdateActor(), AuthorityRef: "owner:" + ownerID, PrivacyClass: "owner",
			ProposedEffectRef: reverted.BundleDigest, DecisionRef: appliedDigest,
			ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, _, err := rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.package-reverted+json", "owner"); err != nil {
			return selfdev.Operation{}, http.StatusConflict, err
		}
	}
	// applied -> rolled_back has no edge in the platform state machine
	// (rollback_pending is the rollback's own row); the package row is its
	// own, so it takes the dedicated transition.
	row, err := rt.selfdevOperations.RevertPackage(ctx, computerID, reverted.OperationID, note)
	if err != nil {
		return selfdev.Operation{}, http.StatusConflict, err
	}
	return row, http.StatusOK, nil
}

// packageInstalledRelease reports whether this computer has releaseID
// installed (in force).
func (rt *Runtime) packageInstalledRelease(ctx context.Context, operation selfdev.Operation, releaseID string) bool {
	prior, err := rt.packageInstalls(ctx, rt.selfdevRouteOwnerID, operation.ComputerID)
	if err != nil {
		return false
	}
	for _, item := range currentPackageInstalls(prior) {
		if item.ReleaseID == releaseID {
			return true
		}
	}
	return false
}

// packageDecisionHead is the track head the consensus decision's package block
// names as the draft's parent: the release id and tree, or "" for a new
// package. It reads the decision receipt the poster signed, pinned on the tape
// by its digest, so the guest drafts over the same parent the poster voted on.
func (rt *Runtime) packageDecisionHead(ctx context.Context, operation selfdev.Operation, decisionDigest string) (string, string, error) {
	if rt.eventPayloadReader == nil {
		return "", "", errors.New("no event payload reader")
	}
	raw, err := rt.eventPayloadReader.FetchPayload(ctx, operation.ComputerID, decisionDigest)
	if err != nil {
		return "", "", err
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return "", "", err
	}
	block, _ := receipt.KindFields["package"].(map[string]any)
	if block == nil {
		return "", "", errors.New("the decision carries no package block")
	}
	parent, _ := block["parent"].(map[string]any)
	if parent == nil {
		return "", "", nil
	}
	kind, _ := parent["kind"].(string)
	if kind != pkgrelease.ParentRelease {
		return "", "", nil
	}
	id, _ := parent["release_id"].(string)
	tree, _ := parent["tree"].(string)
	if id == "" || tree == "" {
		return "", "", errors.New("the decision's release parent names no release or tree")
	}
	return id, tree, nil
}

// settlePackageAbandon is the owner's abandon of an open package operation,
// settled from the package facts, not an updater journal: an install of the
// draft's release settles it forward (applied); otherwise it closes failed
// with the owner's reason, naming the persisted draft's release when one may
// be public.
func (rt *Runtime) settlePackageAbandon(ctx context.Context, operation selfdev.Operation, spec selfdevSettleSpec) (selfdev.Operation, error) {
	draft, persisted, err := rt.loadPackageDraft(operation)
	if err != nil {
		return selfdev.Operation{}, err
	}
	if persisted && operation.State == selfdev.StateMaterializing {
		prior, err := rt.packageInstalls(ctx, rt.selfdevRouteOwnerID, operation.ComputerID)
		if err != nil {
			return selfdev.Operation{}, err
		}
		for _, item := range currentPackageInstalls(prior) {
			if item.ReleaseID == draft.ReleaseID {
				if err := rt.recordPackageApplied(ctx, operation, draft, item.InstallID); err != nil {
					return selfdev.Operation{}, settleConflict(err)
				}
				return rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
			}
		}
	}
	cause := spec.TerminalError
	if persisted {
		cause += "; the release may be published as " + draft.ReleaseID + ", install it from the Library if it is"
	}
	if err := rt.failPackageOperation(ctx, operation, cause); err != nil {
		return selfdev.Operation{}, settleConflict(err)
	}
	return rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
}
