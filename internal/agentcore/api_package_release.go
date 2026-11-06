package agentcore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// The draft is the read-only answer of
// GET /api/computers/<id>/packages/operations/<op>/release-draft?package=<name>[&head_release_id=&head_tree=][&test_command=]:
// the package release this computer would publish for one package
// operation, with its three patches (Library S2,
// docs/design/library-package-propagation-2026-10-26.md 3.2). The proxy
// publishes it; nothing here writes.

// packageReleaseDraft drafts the release of one package operation (forced
// modularity, docs/design/forced-modularity-build-2026-11-04.md sections 1
// and 2). It reads the operation's frozen bundle: its patch changes this
// package's root only, on the commit the capsule was cast from, and the
// package before it is the base's (the source stack carries no package path).
// The operation must be decided (accepted onward): a release names its
// decision. The host redoes every check it can on the bytes it stores.
func (h *APIHandler) packageReleaseDraft(w http.ResponseWriter, r *http.Request, ownerID, computerID, operationID string) {
	ctx := r.Context()
	query := r.URL.Query()
	name := strings.TrimSpace(query.Get("package"))
	if !objectgraph.ValidPackageName(name) {
		writeAPIJSON(w, http.StatusBadRequest, releaseAPIError{Error: "package must be a package name", Reason: "package_name_invalid"})
		return
	}
	headID, headTree := strings.ToLower(strings.TrimSpace(query.Get("head_release_id"))), strings.ToLower(strings.TrimSpace(query.Get("head_tree")))
	if (headID == "") != (headTree == "") || (headID != "" && (!computerevent.IsSHA256(headID) || !looksLikeGitCommit(headTree))) {
		writeAPIJSON(w, http.StatusBadRequest, releaseAPIError{Error: "head_release_id and head_tree name the track head together", Reason: "request_invalid"})
		return
	}
	operation, err := h.rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "self-development operation not found"})
		return
	}
	switch operation.State {
	case selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateApplied:
	default:
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "operation is " + operation.State + "; only a decided package operation is published", Reason: "operation_not_decided"})
		return
	}
	base, patch, err := h.rt.boundSourcePatch(operation)
	switch {
	case errors.Is(err, errNoSourcePatch):
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "the operation changed no source; a package release is source", Reason: "no_source_patch"})
		return
	case err != nil:
		writeAPIJSON(w, http.StatusServiceUnavailable, releaseAPIError{Error: err.Error(), Reason: "bundle_unavailable"})
		return
	}
	repo := h.rt.releaseRepo()
	// A capsule cast from a projection commit built on the platform stack;
	// the release names the stack base, whose package subtree is the same.
	platformBase := base
	if stackBase, ok := sourceproj.StackBase(ctx, repo, base); ok {
		platformBase = stackBase
	}
	source, err := pkgrelease.DraftPackage(ctx, pkgrelease.DraftInput{
		Repo: repo, Name: name, Base: base, Patch: patch, PlatformBase: platformBase,
		HeadReleaseID: headID, HeadTree: headTree,
	})
	if err != nil {
		writePackageDraftRefusal(w, err)
		return
	}
	precommitments, err := h.rt.releasePrecommitments(r, ownerID, computerID, operation)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, releaseAPIError{Error: "precommitments unreadable: " + err.Error()})
		return
	}
	record := pkgrelease.Record{
		Schema:  pkgrelease.Schema,
		Package: pkgrelease.Package{Name: name, ManifestVersion: source.ManifestVersion},
		Track:   ownerID,
		Parent:  source.Parent,
		Source: pkgrelease.Source{
			Root: source.Root, Tree: source.Tree, TreePatch: packageBlobRef(source.Blobs.TreePatch), ReviewPatch: packageBlobRef(source.Blobs.ReviewPatch),
			ChangedPaths: source.Paths, BuiltOn: platformBase,
		},
		Manifest:  source.Manifest,
		Publisher: pkgrelease.Publisher{OwnerID: ownerID, ComputerID: computerID},
		Operation: pkgrelease.Operation{
			OperationID: operation.OperationID, BundleDigest: operation.BundleDigest, DecisionEvent: operation.DecisionEvent,
			MaterializationReceipt: operation.MaterializationReceipt, CheckpointRef: operation.CheckpointRef, VerifierRefs: operation.VerifierRefs,
		},
		Precommitments: precommitments,
		Declarations:   pkgrelease.Declarations{TestCommand: strings.TrimSpace(query.Get("test_command"))},
	}
	if source.Blobs.ParentPatch != nil {
		parent := packageBlobRef(source.Blobs.ParentPatch)
		record.ParentPatch = &parent
	}
	// The round's accepted screens and video, from the bound bundle only.
	if record.Media, _, err = h.rt.operationMedia(operation); err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, releaseAPIError{Error: err.Error(), Reason: "bundle_unavailable"})
		return
	}
	// No screenshot, no listing: refuse here with the reason, before the
	// owner is asked to confirm a publish the host would refuse.
	if err := pkgrelease.RequireScreenshot(record); err != nil {
		writePackageDraftRefusal(w, err)
		return
	}
	// A release that updates a package shows the same view before and after
	// (owner, 2026-11-04); a new package (empty parent) needs only after shots.
	hasPair, err := h.rt.bundleHasBeforeAfter(operation)
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, releaseAPIError{Error: err.Error(), Reason: "bundle_unavailable"})
		return
	}
	if err := packageUpdateNeedsBeforeAfter(record.Parent, hasPair); err != nil {
		writeAPIJSON(w, http.StatusUnprocessableEntity, releaseAPIError{Error: err.Error(), Reason: capsule.BeforeAfterRequiredReason})
		return
	}
	if err := record.Validate(); err != nil {
		writePackageDraftRefusal(w, err)
		return
	}
	releaseID, err := record.ReleaseID()
	if err != nil {
		writePackageDraftRefusal(w, err)
		return
	}
	canonical, _ := record.Canonical()
	draft := map[string]any{
		"release_id": releaseID, "record": json.RawMessage(canonical),
		"tree_patch_base64":   base64.StdEncoding.EncodeToString(source.Blobs.TreePatch),
		"review_patch_base64": base64.StdEncoding.EncodeToString(source.Blobs.ReviewPatch),
	}
	if source.Blobs.ParentPatch != nil {
		draft["parent_patch_base64"] = base64.StdEncoding.EncodeToString(source.Blobs.ParentPatch)
	}
	writeAPIJSON(w, http.StatusOK, draft)
}

func packageBlobRef(raw []byte) pkgrelease.Blob {
	return pkgrelease.Blob{SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
}

func writePackageDraftRefusal(w http.ResponseWriter, err error) {
	reason := pkgrelease.ReasonOf(err)
	status := http.StatusUnprocessableEntity
	switch reason {
	case pkgrelease.ReasonChangeNotConfined, pkgrelease.ReasonParentUnpublished, pkgrelease.ReasonUnchanged:
		status = http.StatusConflict
	case pkgrelease.ReasonBlobTooLarge:
		status = http.StatusRequestEntityTooLarge
	case "":
		// Not a refusal: a store, ref or git read failed. That is the
		// computer being unable to answer now, not a conflict (bunch279-N1).
		writeAPIJSON(w, http.StatusServiceUnavailable, releaseAPIError{Error: "the package release cannot be drafted on this computer: " + err.Error(), Reason: "tree_unavailable"})
		return
	}
	writeAPIJSON(w, status, releaseAPIError{Error: err.Error(), Reason: reason})
}

// packageUpdateNeedsBeforeAfter refuses a release with a prior version (any
// parent but the empty tree) whose bound bundle holds no before/after
// screenshot pair of one view (capsule.ScreenRole names the pairing).
func packageUpdateNeedsBeforeAfter(parent pkgrelease.Parent, hasPair bool) error {
	if parent.Tree == pkgrelease.EmptyTree || hasPair {
		return nil
	}
	return errors.New(capsule.BeforeAfterRequiredReason + ": this release updates an existing package; its screenshots must hold the same view before (the base build) and after (the candidate), named NN-before-<view>.png and NN-after-<view>.png")
}
