package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Package install (docs/design/package-install-2026-11-04.md). The owner's
// install of a published package release arrives on the P8 adoption route
// the Library already calls, with the release's tree_patch the proxy read
// and verified from the catalog. The computer verifies the record and the
// bytes again, plans the change against its own tree (three-way: L its own
// package root, P the release's parent, R the release), and, only when
// every rung holds, stages the install patch and asks its own desks through
// the owner's prompt path. The change lands as any self-development
// operation does: engineering's capsule build and Freeze, management's
// Verify, the owner's decision, the materializer, and its rollback.

const (
	adoptionKindPackage    = "package"
	packageActionInstall   = "install"
	packageActionUninstall = "uninstall"
	// packageInstallQuietWindow is how long an attempt with no operation
	// yet counts as live. A desk run a crash closed before it froze leaves
	// no operation; after this window the owner may try again.
	packageInstallQuietWindow = 2 * time.Hour
	// packageInstallMaxAttempts bounds the attempts of one release and
	// action on one computer.
	packageInstallMaxAttempts = 50
)

// adoptionRequestMaxBytes bounds the adoption body: a package record, its
// tree_patch in base64, and room.
const adoptionRequestMaxBytes = pkgrelease.MaxRecordBytes + (pkgrelease.MaxBlobBytes/3+1)*4 + (64 << 10)

// isPackageReleaseRecord reports whether raw is a package release record
// (by its schema); anything else is a P8 software release.
func isPackageReleaseRecord(raw json.RawMessage) bool {
	var probe struct {
		Schema string `json:"schema"`
	}
	return json.Unmarshal(raw, &probe) == nil && probe.Schema == pkgrelease.Schema
}

func packageInstallID(computerID, releaseID, action string, attempt int) string {
	key := strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(releaseID) + "\x00" + action + "\x00" + fmt.Sprint(attempt)
	return "install-" + computerevent.DigestBytes([]byte(key))[:24]
}

// installPackage runs the install ladder. created is false when an existing
// install answers the request.
func (rt *Runtime) installPackage(ctx context.Context, ownerID, computerID string, request adoptReleaseRequest) (ReleaseAdoptionView, bool, *adoptionRefusal) {
	if rt.promptSubmitter == nil || rt.store == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || strings.TrimSpace(rt.releaseRepo()) == "" {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "adoption_unavailable", "this computer has no prompt path, update root or source repository")
	}
	releaseID := strings.ToLower(strings.TrimSpace(request.ReleaseID))
	if !computerevent.IsSHA256(releaseID) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusBadRequest, "release_id_invalid", "release_id must be a sha256")
	}
	action := strings.TrimSpace(request.Action)
	if action == "" {
		action = packageActionInstall
	}
	if action != packageActionInstall && action != packageActionUninstall {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusBadRequest, "request_invalid", "action must be install or uninstall")
	}
	uninstall := action == packageActionUninstall

	// 1. The record hashes to the id the owner named.
	rec, err := pkgrelease.Decode(request.Record)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "release_record_invalid", "%v", err)
	}
	if err := pkgrelease.VerifyID(rec, releaseID); err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "release_digest_mismatch", "%v", err)
	}
	// 2. The release's tree, which only the proxy supplies, read from the
	// catalog as this owner.
	if len(request.TreePatch) == 0 {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_unavailable", "the install carries no release tree; an install reaches this computer through the Library")
	}
	// 3-4. The bytes are the record's, the content checks hold, and the
	// change is planned against this computer's own tree.
	repo := rt.releaseRepo()
	head, err := release.Head(ctx, repo)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "source_unavailable", "%v", err)
	}
	plan, err := pkgrelease.PlanInstall(ctx, repo, head, rec, request.TreePatch, uninstall)
	if err != nil {
		switch pkgrelease.ReasonOf(err) {
		case pkgrelease.ReasonBlobMismatch, pkgrelease.ReasonBlobTooLarge:
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_digest_mismatch", "%v", err)
		case pkgrelease.ReasonSecretFound:
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "patch_secret_refused", "%v", err)
		case "":
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "source_unavailable", "%v", err)
		default:
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "package_content_invalid", "%v", err)
		}
	}

	prior, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_store", "%v", err)
	}
	var latest *store.ReleaseAdoption
	for i := range prior {
		if prior[i].ReleaseID == releaseID && prior[i].Action == action && (latest == nil || prior[i].Attempt > latest.Attempt) {
			latest = &prior[i]
		}
	}
	done := (!uninstall && plan.LocalPresent && plan.Local == rec.Source.Tree) || (uninstall && !plan.LocalPresent)
	if latest != nil && (done || rt.packageInstallLive(ctx, *latest)) {
		return rt.adoptionView(ctx, *latest), false, nil
	}
	if done {
		if uninstall {
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "package_not_installed", "this computer holds no %s", rec.Source.Root)
		}
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "package_already_present", "this computer already holds this release's tree at %s", rec.Source.Root)
	}
	// The three-way: never overwrite a first-party package or a local change.
	if uninstall {
		if !plan.LocalPresent || plan.Local != rec.Source.Tree {
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "package_not_installed", "this computer's %s is not this release's tree, so uninstalling it would remove something else", rec.Source.Root)
		}
	} else if plan.LocalPresent && plan.Local != rec.Parent.Tree && !installedTree(prior, rec.Package.Name, plan.Local) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "package_name_taken", "this computer's %s holds a package that is neither this release's parent nor one the Library installed; it is never overwritten", rec.Source.Root)
	}
	// 5. The patch applies to this computer's own tree, checked without
	// writing to the repository, and adds no secret.
	if kinds := sourcePatchAddedSecrets(plan.Patch); len(kinds) > 0 {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "patch_secret_refused", "added lines carry secret content (%s)", strings.Join(kinds, ", "))
	}
	tree, err := release.PatchedTree(ctx, repo, head, plan.Patch)
	if errors.Is(err, release.ErrPatchDoesNotApply) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_does_not_apply", "%v", err)
	}
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "source_unavailable", "%v", err)
	}

	// Every rung held. Stage, ask the desks, record.
	attempt := 1
	if latest != nil {
		attempt = latest.Attempt + 1
	}
	if attempt > packageInstallMaxAttempts {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "install_attempts_exhausted", "%d attempts of this %s already", packageInstallMaxAttempts, action)
	}
	id := packageInstallID(computerID, releaseID, action, attempt)
	canonical, err := rec.Canonical()
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_stage", "%v", err)
	}
	if _, err := rt.stageAdoptionAt(id, releaseID, canonical, plan.Patch); err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_stage", "%v", err)
	}
	patchSHA := computerevent.DigestBytes(plan.Patch)
	submission, err := rt.promptSubmitter(ctx, ownerID, packageInstallRequestText(releaseID, rec, action, plan, patchSHA), id)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusBadGateway, "desks_unavailable", "the install request did not reach this computer's desks: %v", err)
	}
	adoption := store.ReleaseAdoption{
		AdoptionID: id, OwnerID: ownerID, ComputerID: computerID, ReleaseID: releaseID,
		PublisherComputerID: rec.Publisher.ComputerID, PublisherOperationID: rec.Operation.OperationID,
		PatchSHA256: patchSHA, ReleaseBaseCommit: rec.Source.BuiltOn, ReleaseTreeHash: rec.Source.Tree,
		AdopterBaseCommit: head, AdopterTreeHash: tree,
		SameBase:     !plan.LocalPresent || plan.Local == rec.Parent.Tree || uninstall,
		TestCommand:  rec.Declarations.TestCommand,
		TrajectoryID: submission.TrajectoryID, DocID: submission.DocID, SubmissionID: submission.SubmissionID,
		RequestedAt: time.Now().UTC(),
		Kind:        adoptionKindPackage, Package: rec.Package.Name, Action: action, Attempt: attempt,
	}
	if err := rt.store.PutReleaseAdoption(ctx, adoption); err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_store", "the request reached the desks (trajectory %s) but the install was not recorded: %v", submission.TrajectoryID, err)
	}
	return rt.adoptionView(ctx, adoption), true, nil
}

// packageInstalls lists this computer's package installs and uninstalls.
func (rt *Runtime) packageInstalls(ctx context.Context, ownerID, computerID string) ([]store.ReleaseAdoption, error) {
	list, err := rt.store.ListReleaseAdoptions(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	out := list[:0]
	for _, item := range list {
		if item.Kind == adoptionKindPackage {
			out = append(out, item)
		}
	}
	return out, nil
}

// installedTree reports whether tree is the release tree of an earlier
// Library install of package name on this computer: the owner may replace
// it with another release (another track's, for review).
func installedTree(prior []store.ReleaseAdoption, name, tree string) bool {
	for _, item := range prior {
		if item.Package == name && item.Action == packageActionInstall && item.ReleaseTreeHash == tree {
			return true
		}
	}
	return false
}

// packageInstallLive reports whether an attempt still answers a repeat: its
// operation is in flight or applied, or it has none yet and is recent.
func (rt *Runtime) packageInstallLive(ctx context.Context, adoption store.ReleaseAdoption) bool {
	view := rt.adoptionView(ctx, adoption)
	if view.Operation == nil {
		return time.Since(adoption.RequestedAt) < packageInstallQuietWindow
	}
	switch view.Operation.State {
	case selfdev.StateFailed, selfdev.StateRejected, selfdev.StateRolledBack, selfdev.StateApplied:
		return false
	}
	return true
}

// packageInstalledNow is the artifact check of an install: this computer's
// source HEAD holds the release's tree at the package root (an uninstall:
// holds nothing there).
func (rt *Runtime) packageInstalledNow(ctx context.Context, adoption store.ReleaseAdoption) bool {
	repo := rt.releaseRepo()
	if repo == "" || adoption.Package == "" || !objectgraph.ValidPackageName(adoption.Package) {
		return false
	}
	tree, present, err := pkgrelease.Subtree(ctx, repo, "HEAD", pkgrelease.Root(adoption.Package))
	if err != nil {
		return false
	}
	if adoption.Action == packageActionUninstall {
		return !present
	}
	return present && tree == adoption.ReleaseTreeHash
}

// packageInstallRequestText is the owner's request to this computer's desks.
func packageInstallRequestText(releaseID string, rec pkgrelease.Record, action string, plan pkgrelease.InstallPlan, patchSHA string) string {
	root := rec.Source.Root
	what := fmt.Sprintf("Install app package %s from Library release package:%s on this computer", rec.Package.Name, releaseID)
	change := fmt.Sprintf("The patch writes the release's tree into %s/ and changes nothing outside it.", root)
	switch {
	case action == packageActionUninstall:
		what = fmt.Sprintf("Uninstall app package %s (Library release package:%s) from this computer", rec.Package.Name, releaseID)
		change = fmt.Sprintf("The patch removes %s/ and changes nothing outside it.", root)
	case plan.LocalPresent:
		change = fmt.Sprintf("The patch replaces this computer's %s/ with the release's tree and changes nothing outside it.", root)
	}
	// The publisher's declared test command is never run here: the desks
	// apply the verified tree and run the build, nothing the release names.
	check := "git apply --check --reverse /selfdev/adoption/source.patch (it succeeds only when the patch is applied)"
	return strings.Join([]string{
		what + ": change this computer itself by applying the verified install patch, and change nothing else. This is the owner's install from the Library; do not edit the package.",
		fmt.Sprintf("The release was published by computer %s (track %s) from its operation %s. The install patch (sha256 %s) was planned against this computer's own tree. %s", rec.Publisher.ComputerID, rec.Track, rec.Operation.OperationID, patchSHA, change),
		"Only engineering can change this computer: Cast this request to management verbatim in your first cell, with a one-paragraph plan revision, and incorporate management's answer when it arrives.",
		"Management: Cast it to engineering verbatim; then Verify the frozen bundle that engineering's final Report names, and answer this request.",
		"Engineering: the verified patch is mounted read only in your capsule at /selfdev/adoption/source.patch, with the release record at /selfdev/adoption/release.json.",
		"From /workspace/platform run: git apply /selfdev/adoption/source.patch. If it does not apply, report the conflict and do not Freeze.",
		"Then run the frontend build (npm run build in frontend, which runs scripts/check-packages.mjs). If the package check or the build fails, report its message and do not Freeze: nothing is installed.",
		"Deterministic check, to run in the capsule before freezing: " + check + ".",
		"Then freeze the candidate with choir.Freeze and complete.",
	}, " ")
}
