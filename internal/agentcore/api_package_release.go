package agentcore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
// operation applied, every path it changed lies under the package root
// (package_change_not_confined names the others), and the package before it
// is the base's or the track head's (package_parent_unpublished). The host
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
	before, beforeLabel := h.rt.packageSourceBefore(ctx, repo, operation, base)
	source, err := pkgrelease.DraftPackage(ctx, pkgrelease.DraftInput{
		Repo: repo, Name: name, Base: base, Patch: patch, Before: before, BeforeLabel: beforeLabel,
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
		writeAPIJSON(w, http.StatusConflict, releaseAPIError{Error: "the package release cannot be drafted on this computer: " + err.Error(), Reason: "tree_unavailable"})
		return
	}
	writeAPIJSON(w, status, releaseAPIError{Error: err.Error(), Reason: reason})
}

// packageSourceBefore is the commit whose tree is the effective source just
// before operation applied: the projection commit the source projection
// left for the applied operation before it on the same stack base (U1's
// patches are cumulative from that base), or the base itself. It walks the
// applied operations created before this one, newest first, and takes the
// first with a projection ref; a ref on another stack base means the stack
// restarted (upstreamed or moved), so the base is the source before.
//
// The order is creation order: the cast refuses while an apply is pending,
// so the operation created before this one applied before it was cast. A
// wrong "before" can only refuse a release (a false not-confined or
// parent-unpublished), never let a path outside the root travel: the host
// carries the root's subtree and nothing else.
func (rt *Runtime) packageSourceBefore(ctx context.Context, repo string, operation selfdev.Operation, base string) (string, string) {
	baseLabel := "the base " + base
	operations, err := rt.selfdevOperations.ListByStates(ctx, operation.ComputerID, selfdev.StateApplied)
	if err != nil {
		return base, baseLabel
	}
	refs, err := pkgrelease.AppliedRefs(ctx, repo)
	if err != nil || len(refs) == 0 {
		return base, baseLabel
	}
	index := -1
	for i, candidate := range operations {
		if candidate.OperationID == operation.OperationID {
			index = i
			break
		}
	}
	for i := index - 1; i >= 0; i-- {
		ref, ok := refs[operations[i].OperationID]
		if !ok {
			continue
		}
		if !strings.EqualFold(ref.StackBase, base) {
			break
		}
		return ref.Commit, "operation " + operations[i].OperationID
	}
	return base, baseLabel
}
