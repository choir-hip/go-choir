package agentcore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The draft is the read-only answer of
// GET /api/computers/<id>/packages/operations/<op>/release-draft?package=<name>[&head_release_id=&head_tree=][&test_command=]:
// the package release this computer would publish for one applied
// operation, with its three patches (Library S2,
// docs/design/library-package-propagation-2026-10-26.md 3.2). The proxy
// publishes it on the owner's act; nothing here writes.

// packageReleaseDraft drafts the release of one package changed by one
// applied operation, after the publisher-side checks: the setting is on, the
// operation applied, every path it changed lies under the package root or is
// the stack base's again (package_change_not_confined names the others), and the package before it
// is the base's or the track head's, or earlier unpublished rounds of this
// computer's own applied stack, which the release then carries
// (package_parent_unpublished otherwise; pkgrelease.DraftPackage). The host
// redoes every check it can on the bytes it stores.
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
	if !h.rt.computerSettingOn(ctx, ownerID, computerID, store.SettingPackagesPublish) {
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "package publishing is off on this computer; the owner turns on the setting " + store.SettingPackagesPublish, Reason: "packages_publish_off"})
		return
	}
	operation, err := h.rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, releaseAPIError{Error: "self-development operation not found"})
		return
	}
	if operation.State != selfdev.StateApplied {
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "operation is " + operation.State + "; only an applied operation is published", Reason: "operation_not_applied"})
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
	before, beforeLabel, beforeBase, stack, err := h.rt.packageSourceBefore(ctx, repo, operation, base)
	if err != nil {
		writePackageDraftRefusal(w, err)
		return
	}
	source, err := pkgrelease.DraftPackage(ctx, pkgrelease.DraftInput{
		Repo: repo, Name: name, Base: base, Patch: patch, Before: before, BeforeLabel: beforeLabel, BeforeBase: beforeBase,
		HeadReleaseID: headID, HeadTree: headTree, Stack: stack,
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
			ChangedPaths: source.Paths, BuiltOn: base,
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
	// A release that updates a package shows the same view before and after
	// (owner, 2026-11-04); a new package (empty parent) needs only after shots.
	if err := packageUpdateNeedsBeforeAfter(record.Parent, record.Media); err != nil {
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

// packageSourceBefore is the commit whose tree is the effective source just
// before operation applied: the projection commit the source projection
// left for the applied operation immediately before it (U1's patches are
// cumulative from the stack base), or the base itself when there is no
// earlier applied operation or that one's ref is on another stack base
// (the stack restarted: upstreamed or moved).
//
// A previous ref on an older stack base (the stack was re-grafted onto a
// moved base, stack_rematerialized) still stands in, with its base returned as
// beforeBase: the draft grafts its change onto this operation's base, or falls
// back to the base when the base moved at a path that change touches (problem
// 29).
//
// Only the immediately preceding operation stands in. When it changed
// source and left no ref (the projection names only the top of the stack
// it projects, so an operation applied between two projections has none),
// the source before is unknown and the draft refuses
// package_parent_unpublished: an older ref, or the base, would put that
// operation's change inside the root into this release as its own
// (review 274 F2). An earlier operation that changed no source is passed
// over, since it changed no tree.
//
// It also returns the stack: the projection commits of the operations
// applied before this one on the same stack base, newest first, starting
// with the source before (empty when that is the base). The walk stops at
// the first ref on another stack base; an older operation without a ref is
// passed over: every projection commit is the whole source at its point
// (U1's patches are cumulative), so a skipped one loses no change from the
// source before, and the stack is only searched for the track head's tree, so
// a missing point can only refuse.
//
// The order is creation order: the cast refuses while an apply is pending,
// so the operation created before this one applied before it was cast.
func (rt *Runtime) packageSourceBefore(ctx context.Context, repo string, operation selfdev.Operation, base string) (string, string, string, []string, error) {
	baseLabel := "the base " + base
	operations, err := rt.selfdevOperations.ListByStates(ctx, operation.ComputerID, selfdev.StateApplied)
	if err != nil {
		return "", "", "", nil, err
	}
	index := -1
	for i, candidate := range operations {
		if candidate.OperationID == operation.OperationID {
			index = i
			break
		}
	}
	var refs map[string]pkgrelease.AppliedRef
	before, beforeLabel := "", ""
	var stack []string
	beforeBase := ""
	for i := index - 1; i >= 0; i-- {
		previous := operations[i]
		if refs == nil {
			if refs, err = pkgrelease.AppliedRefs(ctx, repo); err != nil {
				return "", "", "", nil, err
			}
		}
		ref, ok := refs[previous.OperationID]
		if !ok {
			_, _, patchErr := rt.boundSourcePatch(previous)
			if errors.Is(patchErr, errNoSourcePatch) {
				continue
			}
			if patchErr != nil && before != "" {
				// Not known to be a source change or not: the stack cannot be
				// shown, so the draft is unavailable now rather than guessed.
				return "", "", "", nil, fmt.Errorf("operation %s: %w", previous.OperationID, patchErr)
			}
			if before == "" {
				return "", "", "", nil, &pkgrelease.Refusal{Reason: pkgrelease.ReasonParentUnpublished, Detail: "operation " + previous.OperationID +
					" applied just before " + operation.OperationID + " and changed source, and the source projection left no ref for it: the package before " +
					operation.OperationID + " is unknown, so the release cannot be shown to carry only this operation's change"}
			}
			continue
		}
		if before == "" {
			// The nearest earlier operation may sit on an older stack base
			// after a re-graft (stack_rematerialized): the draft grafts its
			// change onto this operation's base (problem 29). Older
			// operations must share that base.
			before, beforeLabel = ref.Commit, "operation "+previous.OperationID
			if !strings.EqualFold(ref.StackBase, base) {
				beforeBase = ref.StackBase
			}
		}
		if !strings.EqualFold(ref.StackBase, base) && !strings.EqualFold(ref.StackBase, beforeBase) {
			break
		}
		stack = append(stack, ref.Commit)
	}
	if before == "" {
		return base, baseLabel, "", nil, nil
	}
	return before, beforeLabel, beforeBase, stack, nil
}

// packageUpdateNeedsBeforeAfter refuses a release with a prior version (any
// parent but the empty tree) whose media holds no before/after screenshot
// pair of one view (capsule.ScreenRole names the pairing).
func packageUpdateNeedsBeforeAfter(parent pkgrelease.Parent, media []pkgrelease.Media) error {
	if parent.Tree == pkgrelease.EmptyTree {
		return nil
	}
	var screens []capsule.ScreenFile
	for _, m := range media {
		if m.Kind == pkgrelease.MediaKindScreenshot {
			screens = append(screens, capsule.ScreenFile{Name: m.Name})
		}
	}
	if capsule.HasBeforeAfterPair(screens) {
		return nil
	}
	return errors.New(capsule.BeforeAfterRequiredReason + ": this release updates an existing package; its screenshots must hold the same view before (the base build) and after (the candidate), named NN-before-<view>.png and NN-after-<view>.png")
}
