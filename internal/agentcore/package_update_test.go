package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Updates on the computer (F1), failure modes written before the code:
//
//	U1  a manual installer is updated without pressing Update;
//	U2  an automatic installer takes a release listed less than the soak ago;
//	U3  an automatic installer takes an update that adds an SDK use or a
//	    grant; it must read "needs_approval" and stay;
//	U4  an offer for another track, or for a release that is not the one in
//	    force, changes anything;
//	U5  a platform forced update is not applied for a manual installer, or
//	    its record does not say forced, the reason and the listing version;
//	U6  a platform yank with no eligible head leaves the release installed,
//	    or its uninstall record does not say it was the platform's;
//	U7  the installed list does not show the update state.

// usesBuilder is the fake builder whose modules carry uses by package
// title (read from the manifest bytes in the tree).
type usesBuilder struct {
	fakeModuleBuilder
	uses map[string][]string
}

func (b *usesBuilder) Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error) {
	m, err := b.fakeModuleBuilder.Build(ctx, rec, treePatch)
	if err != nil {
		return m, err
	}
	m.Uses = b.uses[rec.Source.Tree]
	return m, nil
}

type updateRelease struct {
	id   string
	raw  json.RawMessage
	tree []byte
	rec  pkgrelease.Record
}

func (e *installEnv) release(t *testing.T, b *usesBuilder, title string, uses ...string) updateRelease {
	t.Helper()
	rec, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest(title), "App.svelte": "<p>" + title + "</p>\n",
	})
	if uses == nil {
		uses = []string{}
	}
	b.uses[rec.Source.Tree] = uses
	return updateRelease{id: id, raw: raw, tree: tree, rec: rec}
}

func (r updateRelease) track(seq int, state string, listedAt time.Time) pkgrelease.TrackRelease {
	return pkgrelease.TrackRelease{ReleaseID: r.id, Package: installName, Track: r.rec.Track, Seq: seq, State: state, Version: 1, ListedAt: listedAt}
}

func (r updateRelease) candidate(seq int, listedAt time.Time) *PackageUpdateCandidate {
	return &PackageUpdateCandidate{Release: r.track(seq, pkgrelease.StateListed, listedAt), Record: r.raw, TreePatch: r.tree, View: json.RawMessage(`{"media":[]}`)}
}

func (e *installEnv) sync(t *testing.T, offers ...PackageUpdateOffer) []PackageUpdateView {
	t.Helper()
	body, _ := json.Marshal(packageUpdatesRequest{Packages: offers})
	w := releaseRouterCall(t, e.h, http.MethodPost, "/api/computers/"+e.computer+"/packages/updates", e.owner, e.computer, body)
	var out struct {
		Updates []PackageUpdateView `json:"updates"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatalf("sync = %d %s", w.Code, w.Body.String())
	}
	return out.Updates
}

func (e *installEnv) setPolicy(t *testing.T, policy pkgrelease.UpdatePolicy) {
	t.Helper()
	raw, _ := json.Marshal(policy)
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	if _, err := e.rt.store.SaveUserPreference(context.Background(), types.UserPreference{
		OwnerID: e.owner, PreferenceKey: pkgrelease.UpdatePolicyKey, Value: map[string]any{"value": value}, UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
}

func (e *installEnv) inForce(t *testing.T) *store.PackageInstall {
	t.Helper()
	list, err := e.rt.packageInstalls(context.Background(), e.owner, e.computer)
	if err != nil {
		t.Fatal(err)
	}
	return currentPackageInstalls(list)[installName]
}

func TestPackageUpdatesFollowThePolicyTheSoakAndThePlatform(t *testing.T) {
	e := newInstallEnv(t)
	b := &usesBuilder{uses: map[string][]string{}}
	e.rt.packageBuilder = b
	now := time.Now().UTC()
	old := now.Add(-pkgrelease.UpdateSoak - time.Hour)

	r1 := e.release(t, b, "One", "appTitle")
	if code, view, reason := e.post(t, installBody(t, r1.id, r1.raw, r1.tree, "")); code != http.StatusCreated {
		t.Fatalf("install r1 = %d %s %+v", code, reason, view)
	}
	if cur := e.inForce(t); !cur.UsesRecorded || strings.Join(cur.Uses, ",") != "appTitle" {
		t.Fatalf("the install records its uses: %+v", cur)
	}
	r2 := e.release(t, b, "Two", "appTitle")
	offer := PackageUpdateOffer{Package: installName, ReleaseID: r1.id, Installed: ptr(r1.track(1, pkgrelease.StateListed, old)),
		Newest: r2.candidate(2, old), Soaked: r2.candidate(2, old)}

	// U1: manual (no preference) offers and does nothing.
	views := e.sync(t, offer)
	if len(views) != 1 || views[0].State != pkgrelease.UpdateStateAvailable || views[0].Policy != pkgrelease.UpdateManual || e.inForce(t).ReleaseID != r1.id {
		t.Fatalf("U1 = %+v", views)
	}
	// U7: the installed list shows it.
	if pkgs := e.installed(t); len(pkgs) != 1 || pkgs[0].Update == nil || pkgs[0].Update.State != pkgrelease.UpdateStateAvailable || pkgs[0].Update.ReleaseID != r2.id {
		t.Fatalf("U7 installed = %+v", pkgs)
	}
	// U4: another track's offer is ignored.
	stranger := offer
	stranger.Installed = ptr(r1.track(1, pkgrelease.StateListed, old))
	stranger.Installed.Track = "owner-z"
	if views := e.sync(t, stranger); len(views) != 0 {
		t.Fatalf("U4 = %+v", views)
	}

	// U2: automatic, but the only newer release is fresh.
	e.setPolicy(t, pkgrelease.UpdatePolicy{Default: pkgrelease.UpdateManual, Apps: map[string]string{installName: pkgrelease.UpdateAutomatic}})
	fresh := offer
	fresh.Newest, fresh.Soaked = r2.candidate(2, now), nil
	if views := e.sync(t, fresh); views[0].State != pkgrelease.UpdateStateAvailable || !strings.Contains(views[0].Detail, "soak") || e.inForce(t).ReleaseID != r1.id {
		t.Fatalf("U2 = %+v", views)
	}
	// Soaked and narrow: automatic installs it, recorded as automatic.
	views = e.sync(t, offer)
	cur := e.inForce(t)
	if views[0].State != pkgrelease.UpdateStateUpdated || cur.ReleaseID != r2.id || cur.Trigger != store.PackageInstallTriggerAutomatic {
		t.Fatalf("automatic = %+v %+v", views, cur)
	}

	// U3: a soaked update that adds an SDK use waits for approval.
	r3 := e.release(t, b, "Three", "appTitle", "openFile")
	wide := PackageUpdateOffer{Package: installName, ReleaseID: r2.id, Installed: ptr(r2.track(2, pkgrelease.StateListed, old)),
		Newest: r3.candidate(3, old), Soaked: r3.candidate(3, old)}
	views = e.sync(t, wide)
	if views[0].State != pkgrelease.UpdateStateNeedsApproval || views[0].Change == nil || strings.Join(views[0].Change.AddedUses, ",") != "openFile" || e.inForce(t).ReleaseID != r2.id {
		t.Fatalf("U3 = %+v", views)
	}

	// U5: the platform forces r3 onto a manual installer.
	e.setPolicy(t, pkgrelease.UpdatePolicy{Default: pkgrelease.UpdateManual})
	yanked := r2.track(2, pkgrelease.StateYanked, old)
	yanked.StateReason, yanked.StateActor, yanked.ForcedRelease, yanked.Version, yanked.VersionDigest = "CVE-1", "platform:ops", r3.id, 2, strings.Repeat("d", 64)
	forced := PackageUpdateOffer{Package: installName, ReleaseID: r2.id, Installed: &yanked, Newest: r3.candidate(3, now), Forced: r3.candidate(3, now)}
	views = e.sync(t, forced)
	cur = e.inForce(t)
	if views[0].State != pkgrelease.UpdateStatePlatformForce || cur.ReleaseID != r3.id || !cur.Forced || cur.Trigger != store.PackageInstallTriggerPlatform ||
		cur.PlatformReason != "CVE-1" || cur.ListingRelease != r2.id || cur.ListingVersion != 2 || cur.ListingDigest != yanked.VersionDigest {
		t.Fatalf("U5 = %+v %+v", views, cur)
	}

	// U6: a yank with no forced release and no eligible head uninstalls.
	plain := r3.track(3, pkgrelease.StateYanked, old)
	plain.StateReason, plain.StateActor, plain.Version = "malware", "platform:ops", 2
	views = e.sync(t, PackageUpdateOffer{Package: installName, ReleaseID: r3.id, Installed: &plain})
	if views[0].State != pkgrelease.UpdateStatePlatformYank || e.inForce(t) != nil {
		t.Fatalf("U6 = %+v", views)
	}
	list := e.list(t)
	if list[0].Action != packageActionUninstall || list[0].Trigger != store.PackageInstallTriggerPlatform || list[0].PlatformReason != "malware" {
		t.Fatalf("U6 record = %+v", list[0])
	}
}
