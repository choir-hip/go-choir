package agentcore

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// A package install registers the desks its manifest contributes (modular
// architecture definition, drill P4).
//
// The desk set of a computer is the fold of the desk-package event log plus
// the image's required set (appdesk.Boot). A runtime install is a fact of its
// own record, so the set the installs imply is brought into the log by one
// writer, syncInstalledDesks: an install in force whose manifest contributes a
// desk appends the install (or update) event the boot reconcile appends; an
// uninstall (and so a revert, which is an uninstall or an install of the
// earlier release) appends a disable; then the registry is reconciled. It runs
// after every install record and at boot, and is idempotent, so a crash between
// the record and the event only means the next boot does it.
//
// Origin rule: AdmitsDesksOf decides. A Library release is always origin
// published, so only a first-party package's desk (deskmanifest.FirstPartyAppSlug,
// whose row the image ships in internal/appdesk/firstparty.json) is admitted,
// and only on the owner's own track. The desk row is the image's manifest, not
// anything the release writes. Any other desk is refused while
// apppackage.CurrentPlatform().DeskSandbox is false (R24).

// WithDeskLog binds the desk-package event log the install path appends to.
// ownerID is the log's owner key (the computer's owner).
func WithDeskLog(l appdesk.Log, ownerID string) RuntimeOption {
	return func(rt *Runtime) { rt.deskLog, rt.deskLogOwner = l, strings.TrimSpace(ownerID) }
}

// refuseDeskInstall is the install-time gate: a release that carries desks
// installs only as a first-party package's, on the owner's own track.
func refuseDeskInstall(ownerID string, rec pkgrelease.Record) *installRefusal {
	if rec.Manifest.DesksCount == 0 {
		return nil
	}
	pkg := rec.Package.Name
	if !apppackage.CurrentPlatform().AdmitsDesksOf(apppackage.OriginPublished, pkg) {
		return refuseInstall(http.StatusUnprocessableEntity, pkgrelease.ReasonDeskNeedsSandbox, "package %s carries a desk; a desk from another publisher waits for the sandbox (R24)", pkg)
	}
	if !deskmanifest.FirstPartyAppSlug(pkg) {
		// Another publisher's desk, on a computer whose worker sandbox is
		// ready (AdmitsDesksOf said so): one desk, under a name no built-in or
		// first-party desk holds. Its row is the release's own, capped by the
		// install review's ceiling (desk_release.go).
		if rec.Manifest.DesksCount != 1 || deskmanifest.IsReservedAppSlug(pkg) {
			return refuseInstall(http.StatusUnprocessableEntity, pkgrelease.ReasonDeskNeedsSandbox, "package %s: a release carries exactly one desk under a name that no built-in or first-party desk holds", pkg)
		}
		return nil
	}
	if _, ok := appdesk.FirstPartyDesk(pkg); !ok || rec.Manifest.DesksCount != 1 {
		return refuseInstall(http.StatusUnprocessableEntity, pkgrelease.ReasonDeskNeedsSandbox, "package %s has no first-party desk in this image", pkg)
	}
	if rec.Track != ownerID {
		return refuseInstall(http.StatusForbidden, "desk_track_not_yours", "package %s carries a first-party desk; only its owner's own track may install one", pkg)
	}
	return nil
}

// reasonDeskRowRefused names an install whose desk the sync could never
// register: the manifest's desk is not the one row a release may carry.
const reasonDeskRowRefused = "desk_row_refused"

// checkDeskRow runs the sync's own registration test on the built module's
// manifest, so an install never says yes (and the review never seals a verdict)
// for a desk that would not exist: one desk, named for the package, and for
// another publisher's package a row releaseDeskManifest accepts.
func checkDeskRow(rec pkgrelease.Record, module pkgruntime.Module) *installRefusal {
	if rec.Manifest.DesksCount == 0 {
		return nil
	}
	pkg := rec.Package.Name
	m, err := apppackage.ParseManifest(module.Manifest, apppackage.OriginPublished)
	if err != nil {
		return refuseInstall(http.StatusUnprocessableEntity, reasonDeskRowRefused, "package %s: the built manifest does not parse: %v", pkg, err)
	}
	if len(m.Contributes.Desks) != 1 || m.Contributes.Desks[0].Name != pkg {
		return refuseInstall(http.StatusUnprocessableEntity, reasonDeskRowRefused, "package %s: a release carries exactly one desk, named for the package", pkg)
	}
	if deskmanifest.FirstPartyAppSlug(pkg) {
		return nil
	}
	if _, err := releaseDeskManifest(m); err != nil {
		return refuseInstall(http.StatusUnprocessableEntity, reasonDeskRowRefused, "%v", err)
	}
	return nil
}

// releaseDeskPromptOf reads a release desk's prompt from the install's data
// files in the module store (digest-checked on read).
func (rt *Runtime) releaseDeskPromptOf(item store.PackageInstall, path string) (string, error) {
	for _, f := range item.Files {
		if f.Path != path {
			continue
		}
		raw, err := rt.packageModules.ReadFile(f.SHA256)
		if err != nil {
			return "", fmt.Errorf("the desk prompt %s is not in the module store: %w", path, err)
		}
		if strings.TrimSpace(string(raw)) == "" {
			return "", fmt.Errorf("the desk prompt %s is empty", path)
		}
		return string(raw), nil
	}
	return "", fmt.Errorf("the install carries no desk prompt %s", path)
}

// installedDeskPackages is the desk packages the computer's installs put in
// force: package -> the desk manifest for it (the image's, for a first-party
// package; the release's own, capped by its ceiling, for another publisher's
// on a ready sandbox).
func (rt *Runtime) installedDeskPackages(ctx context.Context, ownerID, computerID string) (map[string]appdesk.Required, error) {
	prior, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	out := map[string]appdesk.Required{}
	for pkg, item := range currentPackageInstalls(prior) {
		if len(item.Manifest) == 0 {
			continue
		}
		m, err := apppackage.ParseManifest(item.Manifest, apppackage.OriginPublished)
		if err != nil || len(m.Contributes.Desks) != 1 || m.Contributes.Desks[0].Name != pkg {
			continue
		}
		if deskmanifest.FirstPartyAppSlug(pkg) {
			if fp, ok := appdesk.FirstPartyDesk(pkg); ok && item.PublisherTrack == ownerID {
				out[pkg] = fp
			}
			continue
		}
		// Another publisher's desk: only while this computer's worker sandbox
		// is ready, from the release's own manifest and prompt.
		if !apppackage.CurrentPlatform().AdmitsDesksOf(apppackage.OriginPublished, pkg) {
			rt.releaseDeskPrompts.drop(pkg)
			continue
		}
		dm, err := releaseDeskManifest(m)
		if err != nil {
			log.Printf("runtime: desk sync: %v", err)
			continue
		}
		prompt, err := rt.releaseDeskPromptOf(*item, m.Contributes.Desks[0].Prompt)
		if err != nil {
			log.Printf("runtime: desk sync: package %s: %v", pkg, err)
			continue
		}
		rt.releaseDeskPrompts.set(pkg, prompt)
		out[pkg] = appdesk.Required{Package: pkg, Version: m.Version, Manifest: dm}
	}
	return out, nil
}

// syncInstalledDesks brings the desk-package log to the installs' desired
// desks and reconciles the registry. A computer with no desk log (a test, a
// bring-up) does nothing.
func (rt *Runtime) syncInstalledDesks(ctx context.Context, ownerID, computerID string) error {
	if rt == nil || rt.deskLog == nil || rt.store == nil {
		return nil
	}
	want, err := rt.installedDeskPackages(ctx, ownerID, computerID)
	if err != nil {
		return fmt.Errorf("desk sync: read installs: %w", err)
	}
	required, err := appdesk.ImageRequired()
	if err != nil {
		return err
	}
	events, err := rt.deskLog.Events(ctx)
	if err != nil {
		return fmt.Errorf("desk sync: read the log: %w", err)
	}
	set, _ := appdesk.DeriveRequired(events, required)
	retired := map[string]bool{}
	for _, ev := range events {
		switch ev.Action {
		case appdesk.ActionDisable:
			retired[ev.Package] = true
		case appdesk.ActionInstall:
			delete(retired, ev.Package)
		}
	}
	// Desks registered from another publisher's release (not first-party).
	releaseDesk := map[string]bool{}
	for _, d := range set.Desks {
		if appdesk.PackageOrigin(d) && !d.FirstParty && !strings.HasSuffix(d.Name, "-candidate") {
			releaseDesk[strings.TrimPrefix(d.Name, "app:")] = true
		}
	}
	// Until this process has run its sandbox selftest the platform's answer is
	// "not checked", which proves nothing about a desk already registered:
	// the sync only adds then (the wake still refuses on a not-ready platform).
	sandboxUnchecked := yaegikernel.DeskSandboxStatusNow().Source == ""
	isRequired := map[string]bool{}
	for _, r := range required {
		isRequired[r.Package] = true
	}
	for pkg, fp := range want {
		m := fp.Manifest
		ev := appdesk.Event{Package: pkg, ManifestDigest: appdesk.Digest(m), Manifest: &m, Version: fp.Version}
		have, live := set.Versions[pkg]
		switch {
		case !live:
			ev.Action, ev.Reapproved = appdesk.ActionInstall, retired[pkg]
		case appdesk.VersionLess(have, fp.Version):
			ev.Action = appdesk.ActionUpdate
		default:
			continue
		}
		if _, err := rt.deskLog.Append(ctx, ev); err != nil {
			return fmt.Errorf("desk sync: append %s %s: %w", ev.Action, pkg, err)
		}
	}
	// A first-party desk package that is live and no longer installed is
	// disabled. When the log's owner key is a default (CHOIR_OWNER_ID unset,
	// "local"), the installs read above may not be the owner's, so the sync is
	// add-only: an unset owner id never silently disables an installed desk.
	for pkg := range set.Versions {
		if rt.deskOwnerUnknown(ownerID) {
			break
		}
		if _, ok := want[pkg]; ok || isRequired[pkg] {
			continue
		}
		if !deskmanifest.FirstPartyAppSlug(pkg) && (!releaseDesk[pkg] || sandboxUnchecked) {
			continue
		}
		rt.releaseDeskPrompts.drop(pkg)
		if _, err := rt.deskLog.Append(ctx, appdesk.Event{Action: appdesk.ActionDisable, Package: pkg}); err != nil {
			return fmt.Errorf("desk sync: append disable %s: %w", pkg, err)
		}
	}
	if rt.deskReconciler != nil {
		return rt.deskReconciler(ctx)
	}
	return nil
}

// syncInstalledDesksAfterInstall is the install path's call: a failure is
// logged and left to the next boot, which repeats it, and never undoes the
// install record.
func (rt *Runtime) syncInstalledDesksAfterInstall(ctx context.Context, ownerID, computerID string) {
	if err := rt.syncInstalledDesks(context.WithoutCancel(ctx), ownerID, computerID); err != nil {
		log.Printf("runtime: install desk sync: %v", err)
	}
}

// deskOwnerUnknown reports that ownerID is the placeholder an unset
// CHOIR_OWNER_ID leaves, so the installs read under it prove nothing about
// which desks are gone.
func (rt *Runtime) deskOwnerUnknown(ownerID string) bool {
	o := strings.TrimSpace(ownerID)
	return o == "" || o == "local"
}
