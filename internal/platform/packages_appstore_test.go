package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// The Library is an app store (owner, 2026-11-04). Failure modes pinned,
// written before the code:
//
//	A1  a release with no screenshot is listed, or its refusal is not
//	    screenshot_required (media_test.go);
//	A2  a release published before the rule (no screenshot) still shows in
//	    the catalog or in another owner's history of the track; it must drop
//	    out of the listing but stay stored, and stay the publisher's own to
//	    see and withdraw;
//	A3  a track whose old release has no screenshot hides its newer release
//	    that has one, or counts the old one;
//	A4  the app card lacks its date or description; a record whose
//	    description is not the manifest's is listed.

// publishLegacy files b's record and a listed listing without the screenshot
// gate: how a release published before the rule sits in the store.
func (w *pkgWorld) publishLegacy(b built) {
	w.t.Helper()
	w.putBlobs(b)
	w.storeRecordOnly(b)
	listing := pkgrelease.Listing{
		RecordKind: pkgrelease.RecordKindPackageRelease, RecordID: b.id, Package: b.record.Package.Name,
		Track: b.record.Track, Seq: 1, Scopes: publishScopes,
		State: pkgrelease.StateListed, StateReason: "published", StateActor: "legacy", Version: 1,
	}
	obj, err := listingObject(b.record.Publisher.OwnerID, b.record.Publisher.ComputerID, listing, time.Now().UTC())
	if err != nil {
		w.t.Fatal(err)
	}
	if ok, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil || !ok {
		w.t.Fatalf("legacy listing: %v %v", ok, err)
	}
}

func (w *pkgWorld) catalogOf(reader string) []PackageCatalogEntry {
	w.t.Helper()
	var out struct {
		Packages []PackageCatalogEntry `json:"packages"`
	}
	rec := w.read(packagesRoute + "/catalog?owner_id=" + reader)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		w.t.Fatalf("catalog = %d %s", rec.Code, rec.Body.String())
	}
	return out.Packages
}

func TestCatalogListsOnlyReleasesWithScreenshotsAndKeepsTheOthersStored(t *testing.T) {
	w := newPkgWorld(t)
	legacy := w.withMedia(w.build("<p>old</p>\n", "owner-a", nil))
	w.publishLegacy(legacy)

	// A2: the catalog drops it for another owner; it stays stored and the
	// publisher still sees it (and can withdraw it).
	if got := w.catalogOf("owner-b"); len(got) != 0 {
		t.Fatalf("A2 the catalog lists a screenshot-less release: %+v", got)
	}
	if got := w.catalogOf("owner-a"); len(got) != 0 {
		t.Fatalf("A2 the catalog lists a screenshot-less release to its publisher: %+v", got)
	}
	if rec := w.read(packagesRoute + "/catalog/reader/releases?owner_id=owner-b&track=owner-a"); !strings.Contains(rec.Body.String(), `"count":0`) {
		t.Fatalf("A2 another owner's history lists it: %s", rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/catalog/reader/releases?owner_id=owner-a&track=owner-a"); !strings.Contains(rec.Body.String(), `"count":1`) {
		t.Fatalf("A2 the publisher's own history lost it: %s", rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/releases/" + legacy.id + "?owner_id=owner-b"); rec.Code != http.StatusOK {
		t.Fatalf("A2 the release was deleted or hidden from a direct read: %d", rec.Code)
	}

	// A3: a screenshot release on top of it is the head and the only count.
	next := w.build("<p>new</p>\n", "owner-a", &legacy)
	w.published(next)
	got := w.catalogOf("owner-b")
	if len(got) != 1 || got[0].HeadReleaseID != next.id || got[0].Releases != 1 || len(got[0].HeadMedia) != 1 {
		t.Fatalf("A3 catalog = %+v", got)
	}
	if got[0].HeadPublishedAt == "" {
		t.Fatalf("A4 the card has no date: %+v", got[0])
	}
}

func TestCatalogCardCarriesTheManifestsDescription(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	b.record.Description = "Read"
	b = w.seal(b.record, b.blobs)
	w.published(b)
	got := w.catalogOf("owner-b")
	if len(got) != 1 || got[0].HeadDescription != "Read" {
		t.Fatalf("A4 catalog = %+v", got)
	}

	// A4: a description the manifest does not hold is refused.
	forged := w.build("<p>v3</p>\n", "owner-c", nil)
	forged.record.Description = "Ships a free phone"
	forged = w.seal(forged.record, forged.blobs)
	wantPkgRefusal(t, w.publish(forged), http.StatusUnprocessableEntity, pkgrelease.ReasonManifestInvalid)
}
