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

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Package install is runtime install (Library S12, app packages P5;
// docs/design/s12-runtime-install-build-2026-11-04.md). The owner's install
// of a published package release arrives on the install route the Library
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
	installKindPackage     = "package"
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

// installRequestMaxBytes bounds the install body: a package record, its
// tree_patch in base64, and room.
const installRequestMaxBytes = pkgrelease.MaxRecordBytes + (pkgrelease.MaxBlobBytes/3+1)*4 + (64 << 10)

// PackageModuleBuilder builds a verified release into a module/v1 module
// (pkgruntime.Builder on a computer; a fake in tests).
type PackageModuleBuilder interface {
	Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error)
}

// packageInstallRequest is the body the proxy forwards.
type packageInstallRequest struct {
	ReleaseID string          `json:"release_id"`
	Record    json.RawMessage `json:"record"`
	// TreePatch is the release's tree_patch the proxy read from the
	// catalog; Action is install (the default) or uninstall.
	TreePatch []byte `json:"tree_patch,omitempty"`
	Action    string `json:"action,omitempty"`
}

// PackageInstallView is an install record and where it stands now.
type PackageInstallView struct {
	store.PackageInstall
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

// installRefusal is a typed refusal: an HTTP status and a reason code.
type installRefusal struct {
	status int
	reason string
	detail string
}

func (r *installRefusal) Error() string { return r.reason + ": " + r.detail }

func refuseInstall(status int, reason, format string, args ...any) *installRefusal {
	return &installRefusal{status: status, reason: reason, detail: fmt.Sprintf(format, args...)}
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
func (rt *Runtime) installPackage(ctx context.Context, ownerID, computerID string, request packageInstallRequest) (PackageInstallView, bool, *installRefusal) {
	if rt.store == nil {
		return PackageInstallView{}, false, refuseInstall(http.StatusServiceUnavailable, "install_unavailable", "this computer has no install store")
	}
	releaseID := strings.ToLower(strings.TrimSpace(request.ReleaseID))
	if !computerevent.IsSHA256(releaseID) {
		return PackageInstallView{}, false, refuseInstall(http.StatusBadRequest, "release_id_invalid", "release_id must be a sha256")
	}
	action := strings.TrimSpace(request.Action)
	if action == "" {
		action = packageActionInstall
	}
	if action != packageActionInstall && action != packageActionUninstall {
		return PackageInstallView{}, false, refuseInstall(http.StatusBadRequest, "request_invalid", "action must be install or uninstall")
	}
	if action == packageActionUninstall {
		return rt.uninstallPackage(ctx, ownerID, computerID, releaseID)
	}
	if rt.packageBuilder == nil || strings.TrimSpace(rt.packageModules.Dir) == "" {
		return PackageInstallView{}, false, refuseInstall(http.StatusServiceUnavailable, "install_unavailable", "this computer has no package builder or module store")
	}

	// 1. The record hashes to the id the owner named.
	rec, err := pkgrelease.Decode(request.Record)
	if err != nil {
		return PackageInstallView{}, false, refuseInstall(http.StatusUnprocessableEntity, "release_record_invalid", "%v", err)
	}
	if err := pkgrelease.VerifyID(rec, releaseID); err != nil {
		return PackageInstallView{}, false, refuseInstall(http.StatusUnprocessableEntity, "release_digest_mismatch", "%v", err)
	}
	// Backstop of the proxy's rule: a stranger's release never replaces a
	// built-in app, whichever route brought it here.
	if apppackage.IsBuiltinPackage(rec.Package.Name) && rec.Track != ownerID {
		return PackageInstallView{}, false, refuseInstall(http.StatusForbidden, "builtin_name_not_yours", "package %s is a built-in app; only its owner's own track may replace it", rec.Package.Name)
	}
	rt.packageInstallMu.Lock()
	defer rt.packageInstallMu.Unlock()
	prior, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return PackageInstallView{}, false, refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	current := currentPackageInstalls(prior)[rec.Package.Name]

	// 3. Already installed, with its module in the store: nothing to do.
	if current != nil && current.ReleaseID == releaseID {
		if rt.installBytesPresent(*current) {
			return rt.installView(*current, prior), false, nil
		}
		// The module is gone (a restore onto a fresh store): rebuild it below.
	}
	if len(request.TreePatch) == 0 {
		return PackageInstallView{}, false, refuseInstall(http.StatusConflict, "patch_unavailable", "the install carries no release tree; an install reaches this computer through the Library")
	}
	// The record is named before the build, so a refused attempt costs no build.
	record, refusal := rt.newPackageInstallRecord(ctx, ownerID, computerID, releaseID, packageActionInstall, prior)
	if refusal != nil {
		return PackageInstallView{}, false, refusal
	}
	record.Package = rec.Package.Name
	record.PublisherComputerID, record.PublisherOperationID = rec.Publisher.ComputerID, rec.Operation.OperationID
	record.PublisherTrack = rec.Track
	record.ReleaseBaseCommit, record.ReleaseTreeHash = rec.Source.BuiltOn, rec.Source.Tree

	// 4. Verify and build: the bytes are the record's, confined to the
	// package root, carry no secret and a valid manifest; then the module.
	module, err := rt.packageBuilder.Build(ctx, rec, request.TreePatch)
	if err != nil {
		switch reason := pkgrelease.ReasonOf(err); {
		case reason == pkgrelease.ReasonBlobMismatch || reason == pkgrelease.ReasonBlobTooLarge:
			return PackageInstallView{}, false, refuseInstall(http.StatusConflict, "patch_digest_mismatch", "%v", err)
		case reason == pkgrelease.ReasonSecretFound:
			return PackageInstallView{}, false, refuseInstall(http.StatusUnprocessableEntity, "patch_secret_refused", "%v", err)
		case reason != "":
			return PackageInstallView{}, false, refuseInstall(http.StatusUnprocessableEntity, "package_content_invalid", "%v", err)
		case errors.Is(err, pkgruntime.ErrBuild):
			return PackageInstallView{}, false, refuseInstall(http.StatusUnprocessableEntity, "package_build_failed", "%v", err)
		default:
			return PackageInstallView{}, false, refuseInstall(http.StatusServiceUnavailable, "install_unavailable", "%v", err)
		}
	}
	// An app is a module/v1 module; a pack (no app) is data only: no module.
	pack := len(module.Bytes) == 0 && module.ABI == ""
	if !pack && (module.ABI != pkgruntime.ModuleABI || len(module.Bytes) == 0) {
		return PackageInstallView{}, false, refuseInstall(http.StatusInternalServerError, "package_build_failed", "the builder returned no module/v1 module")
	}
	sha := ""
	if !pack {
		if sha, err = rt.packageModules.Put(module.Bytes); err != nil {
			return PackageInstallView{}, false, refuseInstall(http.StatusInternalServerError, "module_store", "%v", err)
		}
	}
	for _, f := range module.Files {
		if _, err := rt.packageModules.PutFile(f.Bytes); err != nil {
			return PackageInstallView{}, false, refuseInstall(http.StatusInternalServerError, "module_store", "%v", err)
		}
		record.Files = append(record.Files, store.PackageInstallFile{Path: f.Path, SHA256: f.SHA256})
	}
	// 5. Record it. The record is the install: it is written after the
	// module is stored, so a crash before it leaves only an unreferenced
	// module, never an install whose module is missing.
	record.PatchSHA256 = computerevent.DigestBytes(request.TreePatch)
	record.ModuleSHA256, record.ModuleABI, record.RequiresSDK, record.Manifest = sha, module.ABI, module.RequiresSDK, module.Manifest
	if err := rt.store.PutPackageInstall(ctx, record); err != nil {
		return PackageInstallView{}, false, refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	return rt.installView(record, append([]store.PackageInstall{record}, prior...)), true, nil
}

// installBytesPresent reports whether everything an install names is in the
// module store: its module (an app) and each data file it carries.
func (rt *Runtime) installBytesPresent(item store.PackageInstall) bool {
	if item.ModuleSHA256 != "" {
		if _, err := rt.packageModules.Read(item.ModuleSHA256); err != nil {
			return false
		}
	}
	for _, f := range item.Files {
		if _, err := rt.packageModules.ReadFile(f.SHA256); err != nil {
			return false
		}
	}
	return true
}

// uninstallPackage removes the runtime install of releaseID that is in
// force. It needs nothing from the catalog: the install record names the
// package, so a withdrawn, suspended, yanked or deleted release, or a
// computer with no builder, can always be uninstalled.
func (rt *Runtime) uninstallPackage(ctx context.Context, ownerID, computerID, releaseID string) (PackageInstallView, bool, *installRefusal) {
	rt.packageInstallMu.Lock()
	defer rt.packageInstallMu.Unlock()
	prior, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return PackageInstallView{}, false, refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	var current *store.PackageInstall
	for _, item := range currentPackageInstalls(prior) {
		if item.ReleaseID == releaseID {
			current = item
			break
		}
	}
	if current == nil {
		return PackageInstallView{}, false, refuseInstall(http.StatusConflict, "package_not_installed", "this computer has no runtime install of release %s in force", releaseID)
	}
	record, refusal := rt.newPackageInstallRecord(ctx, ownerID, computerID, releaseID, packageActionUninstall, prior)
	if refusal != nil {
		return PackageInstallView{}, false, refusal
	}
	record.Package = current.Package
	record.PublisherComputerID, record.PublisherOperationID = current.PublisherComputerID, current.PublisherOperationID
	record.ReleaseBaseCommit, record.ReleaseTreeHash = current.ReleaseBaseCommit, current.ReleaseTreeHash
	if err := rt.store.PutPackageInstall(ctx, record); err != nil {
		return PackageInstallView{}, false, refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	return rt.installView(record, append([]store.PackageInstall{record}, prior...)), true, nil
}

// newPackageInstallRecord names the next runtime record of one release and
// action: its attempt counts the records of that release and action in the
// full history, and its id is one no record holds yet (a free id is searched
// past any taken one, so an id never collides).
func (rt *Runtime) newPackageInstallRecord(ctx context.Context, ownerID, computerID, releaseID, action string, prior []store.PackageInstall) (store.PackageInstall, *installRefusal) {
	attempt := 1
	for _, item := range prior {
		if item.ReleaseID == releaseID && item.Action == action && item.Attempt >= attempt {
			attempt = item.Attempt + 1
		}
	}
	if attempt > packageInstallMaxAttempts {
		return store.PackageInstall{}, refuseInstall(http.StatusConflict, "install_attempts_exhausted", "%d %ss of this release already", packageInstallMaxAttempts, action)
	}
	id := packageInstallID(computerID, releaseID, action, attempt)
	for tries := 0; ; tries++ {
		_, err := rt.store.GetPackageInstall(ctx, ownerID, computerID, id)
		if errors.Is(err, store.ErrNotFound) {
			break
		}
		if err != nil {
			return store.PackageInstall{}, refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
		}
		if tries >= 1000 {
			return store.PackageInstall{}, refuseInstall(http.StatusInternalServerError, "install_store", "no free install id for release %s", releaseID)
		}
		id = packageInstallID(computerID, releaseID, action, attempt) + fmt.Sprintf("-%d", tries+1)
	}
	now := time.Now().UTC()
	// Records order by RequestedAt; never let a new one sort before the
	// newest one already written (a coarse or stepped-back clock).
	for _, item := range prior {
		if !item.RequestedAt.Before(now) {
			now = item.RequestedAt.Add(time.Microsecond)
		}
	}
	return store.PackageInstall{
		InstallID: id, OwnerID: ownerID, ComputerID: computerID, ReleaseID: releaseID,
		RequestedAt: now, Kind: installKindPackage, Action: action, Attempt: attempt,
		Mode: store.PackageInstallModeRuntime,
	}, nil
}

// packageInstalls lists this computer's package install records, newest
// first.
func (rt *Runtime) packageInstalls(ctx context.Context, ownerID, computerID string) ([]store.PackageInstall, error) {
	list, err := rt.store.ListPackageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	out := make([]store.PackageInstall, 0, len(list))
	for _, item := range list {
		if item.Kind == installKindPackage {
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
func currentPackageInstalls(records []store.PackageInstall) map[string]*store.PackageInstall {
	out := map[string]*store.PackageInstall{}
	seen := map[string]bool{}
	for i := range records {
		item := &records[i]
		if item.Mode != store.PackageInstallModeRuntime || seen[item.Package] {
			continue
		}
		seen[item.Package] = true
		if item.Action == packageActionInstall {
			out[item.Package] = item
		}
	}
	return out
}

// installView says where one record stands among all of the computer's
// records (newest first).
func (rt *Runtime) installView(item store.PackageInstall, all []store.PackageInstall) PackageInstallView {
	view := PackageInstallView{PackageInstall: item}
	if item.Mode != store.PackageInstallModeRuntime {
		view.State = installStateSource
		return view
	}
	var latest *store.PackageInstall
	for i := range all {
		if all[i].Mode == store.PackageInstallModeRuntime && all[i].Package == item.Package {
			latest = &all[i]
			break
		}
	}
	inForce := latest != nil && latest.InstallID == item.InstallID
	switch {
	case inForce && item.Action == packageActionInstall:
		view.State = installStateInstalled
		view.ModuleAvailable = rt.installBytesPresent(item)
	case inForce:
		view.State = installStateUninstalled
	default:
		view.State = installStateReplaced
	}
	view.PatchMatchesRelease = inForce
	return view
}
