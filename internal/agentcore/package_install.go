package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Package install is runtime install (Library S12, app packages P5;
// docs/design/s12-runtime-install-build-2026-11-04.md). The owner's install
// of a published package release arrives on the adoption route the Library
// calls, with the release's tree_patch the proxy read and verified from the
// catalog. The computer verifies the record and the bytes again, builds the
// package into one module/v1 ES module with its own locked dependencies,
// puts the module in its module store (outside the source tree) and records
// the install on its tape. The shell lists installed packages from
// GET /api/packages/installed and loads each module by digest. No source
// changes, no self-development apply, no SPA build and no restart; an
// uninstall is one more record. The earlier source-patch install (the
// desks applied the patch in a capsule) is deleted: this is the one path.

const (
	adoptionKindPackage    = "package"
	packageActionInstall   = "install"
	packageActionUninstall = "uninstall"
	// packageInstallMaxAttempts bounds the records of one release and
	// action on one computer.
	packageInstallMaxAttempts = 50

	installStateInstalled   = "installed"
	installStateReplaced    = "replaced"
	installStateUninstalled = "uninstalled"
	installStateSource      = "source"
)

// adoptionRequestMaxBytes bounds the adoption body: a package record, its
// tree_patch in base64, and room.
const adoptionRequestMaxBytes = pkgrelease.MaxRecordBytes + (pkgrelease.MaxBlobBytes/3+1)*4 + (64 << 10)

// PackageModuleBuilder builds a verified release into a module/v1 module
// (pkgruntime.Builder on a computer; a fake in tests).
type PackageModuleBuilder interface {
	Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error)
}

// adoptReleaseRequest is the body the proxy forwards.
type adoptReleaseRequest struct {
	ReleaseID string          `json:"release_id"`
	Record    json.RawMessage `json:"record"`
	// TreePatch is the release's tree_patch the proxy read from the
	// catalog; Action is install (the default) or uninstall.
	TreePatch []byte `json:"tree_patch,omitempty"`
	Action    string `json:"action,omitempty"`
}

// ReleaseAdoptionView is an install record and where it stands now.
type ReleaseAdoptionView struct {
	store.ReleaseAdoption
	// State: installed (this record is the package's current install),
	// replaced (a later install or uninstall of the package superseded it),
	// uninstalled (this uninstall is the package's latest record), or
	// source (a source-patch install from before runtime install).
	State string `json:"state"`
	// PatchMatchesRelease is true when the record is in force: an install
	// is the package's current install, an uninstall is its latest record.
	// (The Library reads this name; it predates runtime install.)
	PatchMatchesRelease bool `json:"patch_matches_release"`
	// ModuleAvailable: the install's module is in this computer's store.
	ModuleAvailable bool `json:"module_available,omitempty"`
}

// adoptionRefusal is a typed refusal: an HTTP status and a reason code.
type adoptionRefusal struct {
	status int
	reason string
	detail string
}

func (r *adoptionRefusal) Error() string { return r.reason + ": " + r.detail }

func refuseAdoption(status int, reason, format string, args ...any) *adoptionRefusal {
	return &adoptionRefusal{status: status, reason: reason, detail: fmt.Sprintf(format, args...)}
}

// isPackageReleaseRecord reports whether raw is a package release record
// (by its schema).
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

// installPackage installs or uninstalls a release at run time. created is
// false when the computer already holds what was asked.
func (rt *Runtime) installPackage(ctx context.Context, ownerID, computerID string, request adoptReleaseRequest) (ReleaseAdoptionView, bool, *adoptionRefusal) {
	if rt.store == nil || rt.packageBuilder == nil || strings.TrimSpace(rt.packageModules.Dir) == "" {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "install_unavailable", "this computer has no package builder or module store")
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
	// 2. Same-origin code is the owner's own until the sandboxed frame
	// (S13): a release another owner published never installs here. An
	// uninstall removes code, so it is never refused for this.
	if !uninstall && strings.TrimSpace(rec.Publisher.OwnerID) != strings.TrimSpace(ownerID) {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusForbidden, "cross_owner_before_frame", "release %s was published by another owner; another owner's app runs only in the sandboxed frame, which is not built yet", releaseID)
	}

	rt.packageInstallMu.Lock()
	defer rt.packageInstallMu.Unlock()
	prior, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_store", "%v", err)
	}
	current := currentPackageInstalls(prior)[rec.Package.Name]

	if uninstall {
		if current == nil || current.ReleaseID != releaseID {
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "package_not_installed", "this computer has no runtime install of release %s of %s", releaseID, rec.Package.Name)
		}
		record := rt.newPackageInstallRecord(ownerID, computerID, releaseID, rec, action, prior)
		if record.Attempt > packageInstallMaxAttempts {
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "install_attempts_exhausted", "%d uninstalls of this release already", packageInstallMaxAttempts)
		}
		if err := rt.store.PutReleaseAdoption(ctx, record); err != nil {
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_store", "%v", err)
		}
		return rt.adoptionView(record, append([]store.ReleaseAdoption{record}, prior...)), true, nil
	}

	// 3. Already installed, with its module in the store: nothing to do.
	if current != nil && current.ReleaseID == releaseID {
		if _, err := rt.packageModules.Read(current.ModuleSHA256); err == nil {
			return rt.adoptionView(*current, prior), false, nil
		}
		// The module is gone (a restore onto a fresh store): rebuild it below.
	}
	if len(request.TreePatch) == 0 {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_unavailable", "the install carries no release tree; an install reaches this computer through the Library")
	}
	// 4. Verify and build: the bytes are the record's, confined to the
	// package root, carry no secret and a valid manifest; then the module.
	module, err := rt.packageBuilder.Build(ctx, rec, request.TreePatch)
	if err != nil {
		switch reason := pkgrelease.ReasonOf(err); {
		case reason == pkgrelease.ReasonBlobMismatch || reason == pkgrelease.ReasonBlobTooLarge:
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "patch_digest_mismatch", "%v", err)
		case reason == pkgrelease.ReasonSecretFound:
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "patch_secret_refused", "%v", err)
		case reason != "":
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "package_content_invalid", "%v", err)
		case errors.Is(err, pkgruntime.ErrBuild):
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusUnprocessableEntity, "package_build_failed", "%v", err)
		default:
			return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusServiceUnavailable, "install_unavailable", "%v", err)
		}
	}
	if module.ABI != pkgruntime.ModuleABI || len(module.Bytes) == 0 {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "package_build_failed", "the builder returned no module/v1 module")
	}
	sha, err := rt.packageModules.Put(module.Bytes)
	if err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "module_store", "%v", err)
	}
	// 5. Record it. The record is the install: it is written after the
	// module is stored, so a crash before it leaves only an unreferenced
	// module, never an install whose module is missing.
	record := rt.newPackageInstallRecord(ownerID, computerID, releaseID, rec, action, prior)
	if record.Attempt > packageInstallMaxAttempts {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusConflict, "install_attempts_exhausted", "%d installs of this release already", packageInstallMaxAttempts)
	}
	record.PatchSHA256 = computerevent.DigestBytes(request.TreePatch)
	record.ModuleSHA256, record.ModuleABI, record.RequiresSDK, record.Manifest = sha, module.ABI, module.RequiresSDK, module.Manifest
	if err := rt.store.PutReleaseAdoption(ctx, record); err != nil {
		return ReleaseAdoptionView{}, false, refuseAdoption(http.StatusInternalServerError, "adoption_store", "%v", err)
	}
	return rt.adoptionView(record, append([]store.ReleaseAdoption{record}, prior...)), true, nil
}

func (rt *Runtime) newPackageInstallRecord(ownerID, computerID, releaseID string, rec pkgrelease.Record, action string, prior []store.ReleaseAdoption) store.ReleaseAdoption {
	attempt := 1
	for _, item := range prior {
		if item.ReleaseID == releaseID && item.Action == action && item.Attempt >= attempt {
			attempt = item.Attempt + 1
		}
	}
	now := time.Now().UTC()
	// Records order by RequestedAt; never let a new one sort before the
	// newest one already written (a coarse or stepped-back clock).
	for _, item := range prior {
		if !item.RequestedAt.Before(now) {
			now = item.RequestedAt.Add(time.Microsecond)
		}
	}
	return store.ReleaseAdoption{
		AdoptionID: packageInstallID(computerID, releaseID, action, attempt), OwnerID: ownerID, ComputerID: computerID, ReleaseID: releaseID,
		PublisherComputerID: rec.Publisher.ComputerID, PublisherOperationID: rec.Operation.OperationID,
		ReleaseBaseCommit: rec.Source.BuiltOn, ReleaseTreeHash: rec.Source.Tree, SameBase: true,
		RequestedAt: now, Kind: adoptionKindPackage, Package: rec.Package.Name, Action: action, Attempt: attempt,
		Mode: store.ReleaseAdoptionModeRuntime,
	}
}

// packageInstalls lists this computer's package install records, newest
// first.
func (rt *Runtime) packageInstalls(ctx context.Context, ownerID, computerID string) ([]store.ReleaseAdoption, error) {
	list, err := rt.store.ListReleaseAdoptions(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	out := make([]store.ReleaseAdoption, 0, len(list))
	for _, item := range list {
		if item.Kind == adoptionKindPackage {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

// currentPackageInstalls folds runtime records (newest first) into the
// installed set: per package, its latest runtime record, when that is an
// install. Source-patch records from before runtime install are not part
// of it: what they did is in the source tree.
func currentPackageInstalls(records []store.ReleaseAdoption) map[string]*store.ReleaseAdoption {
	out := map[string]*store.ReleaseAdoption{}
	seen := map[string]bool{}
	for i := range records {
		item := &records[i]
		if item.Mode != store.ReleaseAdoptionModeRuntime || seen[item.Package] {
			continue
		}
		seen[item.Package] = true
		if item.Action == packageActionInstall {
			out[item.Package] = item
		}
	}
	return out
}

// adoptionView says where one record stands among all of the computer's
// records (newest first).
func (rt *Runtime) adoptionView(item store.ReleaseAdoption, all []store.ReleaseAdoption) ReleaseAdoptionView {
	view := ReleaseAdoptionView{ReleaseAdoption: item}
	if item.Mode != store.ReleaseAdoptionModeRuntime {
		view.State = installStateSource
		return view
	}
	var latest *store.ReleaseAdoption
	for i := range all {
		if all[i].Mode == store.ReleaseAdoptionModeRuntime && all[i].Package == item.Package {
			latest = &all[i]
			break
		}
	}
	inForce := latest != nil && latest.AdoptionID == item.AdoptionID
	switch {
	case inForce && item.Action == packageActionInstall:
		view.State = installStateInstalled
		_, err := rt.packageModules.Read(item.ModuleSHA256)
		view.ModuleAvailable = err == nil
	case inForce:
		view.State = installStateUninstalled
	default:
		view.State = installStateReplaced
	}
	view.PatchMatchesRelease = inForce
	return view
}
