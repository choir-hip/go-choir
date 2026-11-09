package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// The owner's own T2 update of a base package (plan 12.1, 2026-11-08):
// admitted only on the by-decision route, filed owner-only, offered only to
// its owner. "reader" stands in for a base package (it is in the built-in
// list); the fixture turns it into a context-menu-style provider.
//
//	S1 the by-decision publish lists the T2 release, owner_listed;
//	S2 no other owner reads it, finds it in the catalog, or is told of it;
//	S3 the owner is told of it (an installed release of the owner's track);
//	S4 the owner-act withdraw works and a reinstate returns to owner_listed,
//	   never to listed;
//	S5 the host check without the by-decision flag keeps the T2 refusal.

const pkgShellManifest = `{
  "schema": "choir.package/v2",
  "name": "reader",
  "version": "1.0.1",
  "display": {"title": "Reader", "icon": "R", "description": "Read"},
  "kinds": [],
  "uses_kinds": [],
  "contributes": {"desks": []},
  "provides": [{"service": "contextMenu.renderer", "abi": "choir.service/contextMenu.renderer/v1", "entry": "Menu.surface.svelte"}],
  "capabilities": [{"grant": "ui.shell"}]
}
`

func (p *pdFixture) shellPatch() []byte {
	tree := p.w.fullTree(map[string]string{
		pkgRoot + "/choir-package.json":  pkgShellManifest,
		pkgRoot + "/Menu.surface.svelte": "<p>menu</p>\n",
	})
	return []byte(pkgGit(p.t(), p.w.repo, nil, "", "diff", "--binary", p.w.base, tree) + "\n")
}

func newShellWorld(t *testing.T) (*pdFixture, ConsensusResponse, string) {
	t.Helper()
	f := newPDFixture(t)
	f.patch = f.shellPatch()
	f.setBundle("c0ffee0000000000000000000000000000000000", f.patch, f.readerEffect())
	response, bundleDigest := f.decideAdmitted("op-shell", f.w.base)
	return f, response, bundleDigest
}

func TestShellUpdateIsPublishedByDecisionOwnerOnly(t *testing.T) {
	f, decision, bundle := newShellWorld(t)
	b := f.record(f.patch, "op-shell", bundle, nil)
	if !b.record.IsShell() || !b.record.OwnShellUpdate() {
		t.Fatalf("fixture record is not a T2 own-track update: %+v", b.record.Manifest)
	}
	w := f.publish(b, decision.DecisionDigest, nil)
	var view PackageReleaseView
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &view) != nil {
		t.Fatalf("by-decision T2 publish = %d %s", w.Code, w.Body.String())
	}
	ctx := context.Background()
	listings, _, err := f.handler.trackListings(ctx, pdOwner, "reader")
	if err != nil {
		t.Fatal(err)
	}
	listing := listings[b.id]
	if listing.State != pkgrelease.StateOwnerListed || len(listing.Scopes) != 1 || listing.Scopes[0] != pkgrelease.ScopeOwner {
		t.Fatalf("T2 release filed as %s %v, want owner_listed [owner]", listing.State, listing.Scopes)
	}
	// S2: another owner sees nothing.
	if !f.handler.listingVisible(listing, pdOwner) || f.handler.listingVisible(listing, "owner-stranger") {
		t.Fatal("owner_listed must be visible to its owner alone")
	}
	// S3: the owner's computers are told of it; a stranger is not.
	install := PackageUpdateInstall{Package: "reader", Track: pdOwner, ReleaseID: b.id}
	own, err := f.handler.updateFacts(ctx, pdOwner, install, time.Now().UTC())
	if err != nil || own.Installed == nil || own.Installed.State != pkgrelease.StateOwnerListed {
		t.Fatalf("owner told %+v, %v", own.Installed, err)
	}
	foreign, err := f.handler.updateFacts(ctx, "owner-stranger", install, time.Now().UTC())
	if err != nil || foreign.Installed != nil || foreign.Newest != nil {
		t.Fatalf("stranger told %+v %+v, %v", foreign.Installed, foreign.Newest, err)
	}
	// The track head is the T2 release, so the next draft's parent is right.
	if head, found, _ := trackHead(listings); !found || head.RecordID != b.id {
		t.Fatalf("head = %+v found=%v", head, found)
	}
}

func TestShellUpdateWithdrawAndReinstateStayOwnerOnly(t *testing.T) {
	f, decision, bundle := newShellWorld(t)
	b := f.record(f.patch, "op-shell", bundle, nil)
	if w := f.publish(b, decision.DecisionDigest, nil); w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	act := func(state string) pkgrelease.Listing {
		body, _ := json.Marshal(PackageListingRequest{OwnerID: pdOwner, ComputerID: f.computerID, ListingChange: ListingChange{State: state, Reason: "test"}})
		r := httptest.NewRequest(http.MethodPut, listingsRoute+"/"+b.id, strings.NewReader(string(body)))
		r.Header.Set("X-Internal-Caller", "true")
		rec := httptest.NewRecorder()
		f.handler.HandleListings(rec, r)
		if rec.Code != http.StatusOK {
			t.Fatalf("listing act %s = %d %s", state, rec.Code, rec.Body.String())
		}
		listings, _, err := f.handler.trackListings(context.Background(), pdOwner, "reader")
		if err != nil {
			t.Fatal(err)
		}
		return listings[b.id]
	}
	if l := act(pkgrelease.StateListed); l.State != pkgrelease.StateOwnerListed {
		t.Fatalf("a listed act widened the T2 release to %s", l.State)
	}
	if l := act(pkgrelease.StateWithdrawn); l.State != pkgrelease.StateWithdrawn {
		t.Fatalf("withdraw left %s", l.State)
	}
	if l := act(pkgrelease.StateListed); l.State != pkgrelease.StateOwnerListed || len(l.Scopes) != 1 {
		t.Fatalf("reinstate returned %s %v, want owner_listed [owner]", l.State, l.Scopes)
	}
}

func TestShellHostCheckWithoutDecisionKeepsTheT2Refusal(t *testing.T) {
	f, _, bundle := newShellWorld(t)
	b := f.record(f.patch, "op-shell", bundle, nil)
	in := pkgrelease.HostInput{Record: b.record, TreePatch: b.blobs.TreePatch, ReviewPatch: b.blobs.ReviewPatch, ParentPatch: b.blobs.ParentPatch, Base: f.handler.packageBase}
	if _, err := pkgrelease.VerifyHostFacts(context.Background(), in); pkgrelease.ReasonOf(err) != pkgrelease.ReasonPlatformReviewRequired {
		t.Fatalf("owner-act host check: %v", err)
	}
	in.ByDecision = true
	if _, err := pkgrelease.VerifyHostFacts(context.Background(), in); err != nil {
		t.Fatalf("by-decision host check: %v", err)
	}
	// Another owner's track, or a package the base image does not provide,
	// is refused even by decision.
	other := b.record
	other.Package.Name = "reader-extra"
	in.Record = other
	if _, err := pkgrelease.VerifyHostFacts(context.Background(), in); err == nil {
		t.Fatal("a T2 release of a package the base does not provide was admitted")
	}
}
