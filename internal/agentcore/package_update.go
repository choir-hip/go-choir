package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/mediastate"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Updates (slice F1, docs/design/forced-modularity-build-2026-11-04.md
// section 5). The proxy reads the catalog's update facts for this
// computer's installs (a guest never reads the catalog) and hands them here
// with the candidates' records and trees:
//
//	POST /api/computers/<id>/packages/updates
//
// The computer decides, by its owner's update policy
// (packages.update_policy, a user preference; manual by default):
//
//   - a yanked installed release: the platform's act, which overrides the
//     policy. With a forced release, install it; otherwise install the
//     track's newest soaked release if one exists and does not widen, and
//     uninstall when none does. The record carries the reason and the
//     listing version acted on;
//   - otherwise the newest listed release of the same track is compared by
//     its extracted uses, its grants and its behaviour-surface hash:
//     "available", or "needs_approval" when it widens. An automatic app
//     takes the newest soaked release that does not widen, through the same
//     install path (the old module stays until the new one stores).
//
// The views it computes are what the installed list shows as each
// package's update.

// packageUpdateMaxBytes bounds the sync body: up to three candidates per
// package, each a record and a tree patch, for a bounded set of packages.
const packageUpdateMaxBytes = 64 << 20

// PackageUpdateCandidate is one release the proxy offers, with its record
// and verified tree, and the catalog view (media, review patch) for the
// Library.
type PackageUpdateCandidate struct {
	Release   pkgrelease.TrackRelease `json:"release"`
	Record    json.RawMessage         `json:"record"`
	TreePatch []byte                  `json:"tree_patch"`
	View      json.RawMessage         `json:"view,omitempty"`
}

// PackageUpdateOffer is one installed package's update facts.
type PackageUpdateOffer struct {
	Package   string                   `json:"package"`
	ReleaseID string                   `json:"release_id"`
	Installed *pkgrelease.TrackRelease `json:"installed,omitempty"`
	Newest    *PackageUpdateCandidate  `json:"newest,omitempty"`
	Soaked    *PackageUpdateCandidate  `json:"soaked,omitempty"`
	Forced    *PackageUpdateCandidate  `json:"forced,omitempty"`
}

type packageUpdatesRequest struct {
	Packages []PackageUpdateOffer `json:"packages"`
}

// PackageUpdateView is where one installed package stands against its
// track.
type PackageUpdateView struct {
	Package string `json:"package"`
	// State: none, available, needs_approval, updated (the automatic policy
	// installed it), removed_by_platform or updated_by_platform.
	State     string `json:"state"`
	Policy    string `json:"policy"`
	ReleaseID string `json:"release_id,omitempty"`
	Seq       int    `json:"seq,omitempty"`
	// Change is how the offered release differs from the installed one.
	Change *pkgrelease.UpdateChange `json:"change,omitempty"`
	// Soaked: the offered release has been listed for the soak (an
	// automatic app takes only a soaked release).
	Soaked bool `json:"soaked,omitempty"`
	// Detail says why an automatic app was not updated, or what the
	// platform did.
	Detail string `json:"detail,omitempty"`
	// View is the offered release's catalog view (what changed: its
	// review patch and before and after screens).
	View json.RawMessage `json:"view,omitempty"`
	// Error is a failed automatic or platform change; the installed
	// version stands.
	Error string `json:"error,omitempty"`
}

func (h *APIHandler) handlePackageUpdates(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, releaseAPIError{Error: "method not allowed"})
		return
	}
	var request packageUpdatesRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, packageUpdateMaxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, releaseAPIError{Error: "invalid update sync", Reason: "request_invalid"})
		return
	}
	views, refusal := h.rt.syncPackageUpdates(r.Context(), ownerID, computerID, request.Packages, time.Now().UTC())
	if refusal != nil {
		writeAPIJSON(w, refusal.status, releaseAPIError{Error: refusal.detail, Reason: refusal.reason})
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{"updates": views, "policy": mediastate.ReadUpdatePolicy(r.Context(), h.rt.store, ownerID)})
}

// syncPackageUpdates decides and acts on every offer whose installed
// release is the one in force here.
func (rt *Runtime) syncPackageUpdates(ctx context.Context, ownerID, computerID string, offers []PackageUpdateOffer, now time.Time) ([]PackageUpdateView, *installRefusal) {
	if rt.store == nil {
		return nil, refuseInstall(http.StatusServiceUnavailable, "install_unavailable", "this computer has no install store")
	}
	policy := mediastate.ReadUpdatePolicy(ctx, rt.store, ownerID)
	records, err := rt.packageInstalls(ctx, ownerID, computerID)
	if err != nil {
		return nil, refuseInstall(http.StatusInternalServerError, "install_store", "%v", err)
	}
	current := currentPackageInstalls(records)
	views := []PackageUpdateView{}
	for _, offer := range offers {
		item := current[offer.Package]
		if item == nil || item.ReleaseID != strings.ToLower(strings.TrimSpace(offer.ReleaseID)) || offer.Installed == nil ||
			offer.Installed.ReleaseID != item.ReleaseID || offer.Installed.Track != item.PublisherTrack {
			continue
		}
		views = append(views, rt.decideUpdate(ctx, ownerID, computerID, *item, offer, policy.For(offer.Package), now))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Package < views[j].Package })
	rt.packageUpdateMu.Lock()
	rt.packageUpdateViews = map[string]PackageUpdateView{}
	for _, v := range views {
		rt.packageUpdateViews[v.Package] = v
	}
	rt.packageUpdateMu.Unlock()
	return views, nil
}

func (rt *Runtime) decideUpdate(ctx context.Context, ownerID, computerID string, item store.PackageInstall, offer PackageUpdateOffer, mode string, now time.Time) PackageUpdateView {
	view := PackageUpdateView{Package: offer.Package, State: pkgrelease.UpdateStateNone, Policy: mode}
	installed := *offer.Installed
	if yanked, forced := installed.PlatformAct(); yanked {
		// The platform's break glass: it overrides the owner's policy.
		change := installChange{trigger: store.PackageInstallTriggerPlatform, platformReason: installed.StateReason, listing: installed}
		if forced != "" && offer.Forced != nil && offer.Forced.Release.ReleaseID == forced {
			change.forced = true
			view.State, view.ReleaseID, view.Seq, view.Detail = pkgrelease.UpdateStatePlatformForce, forced, offer.Forced.Release.Seq, installed.StateReason
			if refusal := rt.installCandidate(ctx, ownerID, computerID, *offer.Forced, change); refusal != nil {
				view.Error = refusal.Error()
			}
			return view
		}
		view.State, view.Detail = pkgrelease.UpdateStatePlatformYank, installed.StateReason
		if offer.Soaked != nil {
			if surface, ok := rt.updateProbe(ctx, *offer.Soaked); ok {
				c := pkgrelease.CompareSurface(installedSurface(item), surface)
				if !c.Widening() {
					view.ReleaseID, view.Seq = offer.Soaked.Release.ReleaseID, offer.Soaked.Release.Seq
					if refusal := rt.installCandidate(ctx, ownerID, computerID, *offer.Soaked, change); refusal == nil {
						return view
					}
				}
			}
		}
		if _, _, refusal := rt.installPackage(ctx, ownerID, computerID, packageInstallRequest{ReleaseID: item.ReleaseID, Action: packageActionUninstall, change: change}); refusal != nil {
			view.Error = refusal.Error()
		}
		return view
	}
	if offer.Newest == nil {
		return view
	}
	view.ReleaseID, view.Seq, view.View = offer.Newest.Release.ReleaseID, offer.Newest.Release.Seq, offer.Newest.View
	view.Soaked = offer.Newest.Release.Soaked(now)
	surface, ok := rt.updateProbe(ctx, *offer.Newest)
	if !ok {
		// A candidate that does not build here is offered as needing
		// approval: the owner's press runs the install path, which says why.
		view.State, view.Detail = pkgrelease.UpdateStateNeedsApproval, "the update could not be built on this computer to compare it"
		return view
	}
	cur := installedSurface(item)
	newest := pkgrelease.CompareSurface(cur, surface)
	view.Change = &newest
	view.State = pkgrelease.UpdateStateAvailable
	if newest.Widening() {
		view.State = pkgrelease.UpdateStateNeedsApproval
	}
	if mode != pkgrelease.UpdateAutomatic {
		return view
	}
	if offer.Soaked == nil {
		view.Detail = "no release of this track has been listed for the soak yet"
		return view
	}
	soaked := newest
	if offer.Soaked.Release.ReleaseID != offer.Newest.Release.ReleaseID {
		s, ok := rt.updateProbe(ctx, *offer.Soaked)
		if !ok {
			view.Detail = "the soaked release could not be built on this computer"
			return view
		}
		soaked = pkgrelease.CompareSurface(cur, s)
	}
	take, why := pkgrelease.AutomaticCandidate(mode, offer.Soaked.Release, soaked, now)
	if !take {
		view.Detail = why
		return view
	}
	if refusal := rt.installCandidate(ctx, ownerID, computerID, *offer.Soaked, installChange{trigger: store.PackageInstallTriggerAutomatic}); refusal != nil {
		view.Error = refusal.Error()
		return view
	}
	view.State, view.ReleaseID, view.Seq, view.Change, view.Soaked = pkgrelease.UpdateStateUpdated, offer.Soaked.Release.ReleaseID, offer.Soaked.Release.Seq, &soaked, true
	if offer.Soaked.Release.ReleaseID != offer.Newest.Release.ReleaseID {
		// A newer release is still soaking: it stays offered next time.
		view.Detail = "updated to the newest soaked release; a newer one is still soaking"
	}
	return view
}

// installCandidate installs an offered release through the one install
// path, with the change it records.
func (rt *Runtime) installCandidate(ctx context.Context, ownerID, computerID string, c PackageUpdateCandidate, change installChange) *installRefusal {
	_, _, refusal := rt.installPackage(ctx, ownerID, computerID, packageInstallRequest{
		ReleaseID: c.Release.ReleaseID, Record: c.Record, TreePatch: c.TreePatch, Action: packageActionInstall, change: change,
	})
	return refusal
}

// installedSurface is what the install in force is compared on.
func installedSurface(item store.PackageInstall) pkgrelease.UpdateSurface {
	return pkgrelease.UpdateSurface{
		Uses: item.Uses, UsesKnown: item.UsesRecorded,
		Grants: installreview.Ceiling(lenientManifest(item.Manifest)), BehaviourSurface: item.BehaviourSurface,
	}
}

// updateProbe builds an offered release (verified against its id first) to
// read its extracted uses and grants, once per release id. It stores
// nothing: the install path builds again when the release installs.
func (rt *Runtime) updateProbe(ctx context.Context, c PackageUpdateCandidate) (pkgrelease.UpdateSurface, bool) {
	id := strings.ToLower(strings.TrimSpace(c.Release.ReleaseID))
	rt.packageUpdateMu.Lock()
	if s, ok := rt.packageUpdateProbes[id]; ok {
		rt.packageUpdateMu.Unlock()
		return s, true
	}
	rt.packageUpdateMu.Unlock()
	if rt.packageBuilder == nil {
		return pkgrelease.UpdateSurface{}, false
	}
	rec, err := pkgrelease.Decode(c.Record)
	if err == nil {
		err = pkgrelease.VerifyID(rec, id)
	}
	if err != nil {
		return pkgrelease.UpdateSurface{}, false
	}
	module, err := rt.packageBuilder.Build(ctx, rec, c.TreePatch)
	if err != nil {
		return pkgrelease.UpdateSurface{}, false
	}
	s := pkgrelease.UpdateSurface{
		Uses: append([]string{}, module.Uses...), UsesKnown: true,
		Grants: installreview.Ceiling(lenientManifest(module.Manifest)), BehaviourSurface: rec.Manifest.BehaviourSurface,
	}
	rt.packageUpdateMu.Lock()
	if rt.packageUpdateProbes == nil {
		rt.packageUpdateProbes = map[string]pkgrelease.UpdateSurface{}
	}
	rt.packageUpdateProbes[id] = s
	rt.packageUpdateMu.Unlock()
	return s, true
}

// packageUpdateView is the last sync's view of one package, if any.
func (rt *Runtime) packageUpdateView(name string) *PackageUpdateView {
	rt.packageUpdateMu.Lock()
	defer rt.packageUpdateMu.Unlock()
	if v, ok := rt.packageUpdateViews[name]; ok {
		return &v
	}
	return nil
}
