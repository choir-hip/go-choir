package platform

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Release blobs at the host (packages_blobs.go, B1-B6 there).

func TestPackageBlobsServeTheRecordsPatchesUnderItsVisibility(t *testing.T) {
	w := newPkgWorld(t)
	first := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(first)
	b := w.build("<p>v3</p>\n", "owner-a", &first)
	w.published(b)
	blob := func(id, digest, reader string) *httptest.ResponseRecorder {
		return w.read(packagesRoute + "/releases/" + id + "/blobs/" + digest + "?owner_id=" + reader)
	}

	// Every patch the record names, to any signed-in owner, octet-stream.
	for _, reader := range []string{"owner-a", "owner-b"} {
		for _, raw := range [][]byte{first.blobs.TreePatch, first.blobs.ReviewPatch, first.blobs.ParentPatch} {
			rec := blob(first.id, computerevent.DigestBytes(raw), reader)
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), raw) || rec.Header().Get("Content-Type") != "application/octet-stream" ||
				rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("%s blob = %d %q", reader, rec.Code, rec.Header().Get("Content-Type"))
			}
		}
	}
	if b.record.ParentPatch != nil {
		t.Fatalf("a release-parent record names a parent patch")
	}
	if PackageBlobURL(b.id, "x") != "/api/catalog/releases/"+b.id+"/blobs/x" {
		t.Fatalf("PackageBlobURL = %s", PackageBlobURL(b.id, "x"))
	}

	// B1: a digest the record does not name, though the store holds it
	// (the other release's patch); B3: malformed ids and digests.
	for _, path := range []string{
		"/releases/" + b.id + "/blobs/" + computerevent.DigestBytes(first.blobs.TreePatch),
		"/releases/" + b.id + "/blobs/" + computerevent.DigestBytes(first.blobs.ParentPatch),
		"/releases/" + strings.Repeat("0", 64) + "/blobs/" + computerevent.DigestBytes(b.blobs.TreePatch),
		"/releases/" + b.id + "/blobs/not-a-digest",
		"/releases/" + b.id + "/blobs/..",
		"/releases/not-an-id/blobs/" + computerevent.DigestBytes(b.blobs.TreePatch),
	} {
		if rec := w.read(packagesRoute + path + "?owner_id=owner-a"); rec.Code != http.StatusNotFound {
			t.Fatalf("B1/B3 %s = %d, want 404", path, rec.Code)
		}
	}
	// B3: extra segments never route to the read.
	if rec := w.read(packagesRoute + "/releases/" + b.id + "/blobs/" + computerevent.DigestBytes(b.blobs.TreePatch) + "/x?owner_id=owner-a"); rec.Code == http.StatusOK {
		t.Fatalf("an extra segment = %d", rec.Code)
	}
	// B6: host only, and a read names its owner.
	tree := computerevent.DigestBytes(b.blobs.TreePatch)
	if rec := w.read(packagesRoute + "/releases/" + b.id + "/blobs/" + tree); rec.Code != http.StatusBadRequest {
		t.Fatalf("a blob read naming no owner = %d, want 400", rec.Code)
	}
	if rec := w.do(pkgCall{method: http.MethodGet, path: packagesRoute + "/releases/" + b.id + "/blobs/" + tree + "?owner_id=owner-a", remote: guestTap}); rec.Code != http.StatusForbidden {
		t.Fatalf("a guest blob read = %d, want 403", rec.Code)
	}

	// B2: withdrawn, the blobs are the publisher's alone.
	view := w.wantVisible(b, "owner-b")
	withdrawn := view.Listing
	prev, err := view.Listing.Digest()
	if err != nil {
		t.Fatal(err)
	}
	withdrawn.Version, withdrawn.PrevVersionDigest = withdrawn.Version+1, prev
	withdrawn.State, withdrawn.StateReason, withdrawn.StateActor = pkgrelease.StateWithdrawn, "withdrawn", "owner_act:test"
	obj, err := listingObject("owner-a", "computer-a1", withdrawn, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil {
		t.Fatal(err)
	}
	if rec := blob(b.id, tree, "owner-b"); rec.Code != http.StatusNotFound {
		t.Fatalf("B2 another owner's read of a withdrawn blob = %d, want 404", rec.Code)
	}
	if rec := blob(b.id, tree, "owner-a"); rec.Code != http.StatusOK {
		t.Fatalf("the publisher's read of a withdrawn blob = %d", rec.Code)
	}

	// B4/B5: stored bytes that are not the blob (another body, or one over
	// the cap) answer as missing.
	path, err := w.h.contentStore(pkgrelease.ContentNamespace).path(tree)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{[]byte("not the patch"), make([]byte, pkgrelease.MaxBlobBytes+1)} {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		if rec := blob(b.id, tree, "owner-a"); rec.Code != http.StatusNotFound || rec.Body.Len() > 1024 {
			t.Fatalf("B4/B5 a corrupt %d-byte blob = %d (%d bytes)", len(body), rec.Code, rec.Body.Len())
		}
	}
}

// guestTap is a guest's address on its tap: a caller the host-only routes refuse.
const guestTap = "10.200.1.2:40000"
