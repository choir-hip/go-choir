package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Library S1 (docs/design/library-package-propagation-2026-10-26.md, 4.3 and
// failure mode L4, docs/problems/release-reads-not-owner-scoped-2026-10-26.md):
// release reads are scoped to the owner. Failure modes pinned here, written
// before the code:
//
//	F1  owner B lists owner A's releases;
//	F2  owner B reads one of A's releases by id (200 instead of 404);
//	F3  owner B reads A's patch blob;
//	F4  an unseen release answers 403 or a different body than a missing one,
//	    so its existence is disclosed (it must be the same 404);
//	F5  a caller widens its scope by naming an owner of its own choosing
//	    (a guest's owner_id query parameter must be ignored);
//	F6  a guest of owner A reads owner B's release;
//	F7  a guest of owner A on a second computer cannot read A's releases
//	    (the scope is the owner, not the publisher computer);
//	F8  the owner cannot see their own releases;
//	F9  the host-internal caller changes behavior (it stays unscoped when it
//	    names no owner, and is scoped when it names one);
//	F10 the switch at open changes today's behavior: every owner sees every
//	    release, byte for byte;
//	F11 the guest's owner cannot be resolved and the read succeeds anyway
//	    (it must fail closed);
//	F12 a capability for one computer is used from another computer's
//	    network binding (the read must be refused);
//	F13 two owners publish the same patch bytes and the blob read answers by
//	    whichever was first (each owner must read the blob they published);
//	F14 an unknown CHOIR_RELEASE_READS value opens reads (it must close them);
//	F15 a tombstoned release is served;
//	F17 a bound guest forges a choir.software_release object under its own
//	    owner (metadata naming another owner's patch digest, with a garbage
//	    body, or with a copy of the victim's real record) and the blob gate,
//	    the single read or the list believes it;
//	F18 forged objects naming a victim's release id shadow the real release
//	    (a LIMIT 1 metadata lookup) so every reader gets a 500;
//	F19 more than 200 forged same-digest objects push the real owner out of
//	    the blob gate's window;
//	F20 a bound guest writes a choir.software_release object through the
//	    generic object routes at all;
//	F16 a missing CHOIR_RELEASE_READS, or a Handler nobody configured, opens
//	    reads (the default must be owner; only an explicit open opens).

func TestParseReleaseReads(t *testing.T) {
	for raw, want := range map[string]ReleaseReadsMode{
		"open": ReleaseReadsOpen, " OPEN ": ReleaseReadsOpen, // only an explicit open opens
		"owner": ReleaseReadsOwner, "Owner": ReleaseReadsOwner,
		"": ReleaseReadsOwner, "   ": ReleaseReadsOwner, // F16: unset closes
		"typo": ReleaseReadsOwner, "owners": ReleaseReadsOwner, // F14
	} {
		if got := ParseReleaseReads(raw); got != want {
			t.Fatalf("ParseReleaseReads(%q) = %q, want %q", raw, got, want)
		}
	}
}

// releaseMultiAuth is a computer capability per computer: the bearer token
// is the computer id.
type releaseMultiAuth map[string]bool

func (a releaseMultiAuth) Authorize(r *http.Request, computerID, scope string) error {
	if !a[computerID] || r.Header.Get("Authorization") != "Bearer "+computerID || scope != "event:read" {
		return fmt.Errorf("refused")
	}
	return nil
}
func (a releaseMultiAuth) AuthorizeRenewal(*http.Request, string) error { return fmt.Errorf("no") }

// releaseTaps is vmctl's join from a guest's network address to the computer
// and owner it is bound to.
type releaseTaps map[string]vmctl.GuestOwnership

func (l releaseTaps) LookupGuestContext(_ context.Context, remoteAddr string) (*vmctl.GuestOwnership, error) {
	if o, ok := l[remoteAddr]; ok {
		o.Found = true
		return &o, nil
	}
	return nil, fmt.Errorf("no guest at %s", remoteAddr)
}

const (
	tapA1 = "10.200.1.2:40000" // owner-a's computer-a1
	tapA2 = "10.200.1.3:40000" // owner-a's second computer
	tapB1 = "10.200.2.2:40000" // owner-b's computer
	tapZ  = "10.200.9.9:40000" // bound to nothing
)

type ownerReadsWorld struct {
	h          *Handler
	idA, idB   string // each owner's release
	shaA, shaB string // each owner's patch blob
	idB2       string // owner-b publishes owner-a's patch bytes as its own release
}

var (
	patchOwnerA = []byte("diff --git a/frontend/public/a.txt b/frontend/public/a.txt\nowner a\n")
	patchOwnerB = []byte("diff --git a/frontend/public/b.txt b/frontend/public/b.txt\nowner b\n")
)

func newOwnerReadsWorld(t *testing.T, mode ReleaseReadsMode) *ownerReadsWorld {
	t.Helper()
	return newOwnerReadsWorldForged(t, mode, nil)
}

// newOwnerReadsWorldForged runs before, if set, after the handler exists and
// before anything is published: forged objects that are already in the store
// when the real releases arrive.
func newOwnerReadsWorldForged(t *testing.T, mode ReleaseReadsMode, before func(h *Handler)) *ownerReadsWorld {
	t.Helper()
	h, _ := newReleaseHandler(t)
	h.eventAuth = releaseMultiAuth{"computer-a1": true, "computer-a2": true, "computer-b1": true}
	h.guestLookup = releaseTaps{
		tapA1: {UserID: "owner-a", ComputerID: "computer-a1"},
		tapA2: {UserID: "owner-a", ComputerID: "computer-a2"},
		tapB1: {UserID: "owner-b", ComputerID: "computer-b1"},
	}
	h.SetReleaseReads(mode)
	if before != nil {
		before(h)
	}
	w := &ownerReadsWorld{h: h}
	publish := func(patch []byte, computer, owner string) (id, sha string) {
		t.Helper()
		sha = computerevent.DigestBytes(patch)
		_, id, raw := testRelease(t, patch, computer, owner)
		if r := (releaseCall{method: http.MethodPut, path: "/internal/platform/releases/blobs/" + sha, internal: true, body: patch}).do(h); r.Code != http.StatusCreated && r.Code != http.StatusOK {
			t.Fatalf("blob put = %d", r.Code)
		}
		if r := (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", internal: true, body: publishBody(t, id, raw, owner, computer)}).do(h); r.Code != http.StatusCreated {
			t.Fatalf("publish = %d %s", r.Code, r.Body.String())
		}
		return id, sha
	}
	patchA, patchB := patchOwnerA, patchOwnerB
	w.idA, w.shaA = publish(patchA, "computer-a1", "owner-a")
	w.idB, w.shaB = publish(patchB, "computer-b1", "owner-b")
	// F13: owner-b publishes owner-a's exact patch bytes as its own release.
	w.idB2, _ = publish(patchA, "computer-b1", "owner-b")
	return w
}

func (w *ownerReadsWorld) one(id, computer, tap, token string) *httptest.ResponseRecorder {
	return (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + id + "?computer_id=" + computer, remote: tap, token: token}).do(w.h)
}

func (w *ownerReadsWorld) blob(sha, computer, tap, token string) *httptest.ResponseRecorder {
	return (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/blobs/" + sha + "?computer_id=" + computer, remote: tap, token: token}).do(w.h)
}

func (w *ownerReadsWorld) list(query string) []string {
	r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases" + query, internal: true}).do(w.h)
	var out struct {
		Releases []struct {
			ReleaseID string `json:"release_id"`
		} `json:"releases"`
	}
	if r.Code != http.StatusOK || json.Unmarshal(r.Body.Bytes(), &out) != nil {
		return []string{fmt.Sprintf("status %d %s", r.Code, r.Body.String())}
	}
	ids := make([]string, 0, len(out.Releases))
	for _, rel := range out.Releases {
		ids = append(ids, rel.ReleaseID)
	}
	return ids
}

func has(ids []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, id := range ids {
			if id == w {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestOwnerReadsListIsScopedToTheNamedOwner(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	// F8: the owner sees their own, and F1: not the other owner's.
	a := w.list("?owner_id=owner-a")
	if !has(a, w.idA) || has(a, w.idB, w.idB2) || len(a) != 1 {
		t.Fatalf("owner-a list = %v, want only %s", a, w.idA)
	}
	b := w.list("?owner_id=owner-b")
	if !has(b, w.idB, w.idB2) || has(b, w.idA) || len(b) != 2 {
		t.Fatalf("owner-b list = %v, want %s and %s", b, w.idB, w.idB2)
	}
	// An owner with nothing published sees an empty list, not an error.
	if none := w.list("?owner_id=owner-z"); len(none) != 0 {
		t.Fatalf("owner-z list = %v, want empty", none)
	}
	// The publisher filter narrows within the owner, never across.
	if got := w.list("?owner_id=owner-a&publisher_computer_id=computer-b1"); len(got) != 0 {
		t.Fatalf("owner-a filtered to owner-b's computer = %v, want empty", got)
	}
	// F9: the host-internal caller that names no owner is unchanged.
	if all := w.list(""); !has(all, w.idA, w.idB, w.idB2) || len(all) != 3 {
		t.Fatalf("internal unscoped list = %v, want all three", all)
	}
}

func TestOwnerReadsListIsGuestRefusedAsBefore(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	if r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases?owner_id=owner-a", remote: tapA1, token: "computer-a1"}).do(w.h); r.Code != http.StatusForbidden {
		t.Fatalf("guest list = %d, want 403", r.Code)
	}
}

func TestOwnerReadsSingleReadByOwner(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	// Through the proxy's host call, which names the authenticated owner.
	host := func(id, owner string) (int, string) {
		r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + id + "?computer_id=host&owner_id=" + owner, internal: true}).do(w.h)
		return r.Code, r.Body.String()
	}
	if code, body := host(w.idA, "owner-a"); code != http.StatusOK || !strings.Contains(body, w.idA) {
		t.Fatalf("owner-a reads own = %d %s", code, body) // F8
	}
	codeOther, bodyOther := host(w.idA, "owner-b")
	if codeOther != http.StatusNotFound { // F2
		t.Fatalf("owner-b reads owner-a's release = %d %s, want 404", codeOther, bodyOther)
	}
	// F4: indistinguishable from a release that does not exist.
	codeMissing, bodyMissing := host(strings.Repeat("8", 64), "owner-b")
	if codeMissing != http.StatusNotFound || bodyMissing != bodyOther {
		t.Fatalf("unseen = %d %q, missing = %d %q; they must match", codeOther, bodyOther, codeMissing, bodyMissing)
	}
	// F9: the host-internal caller naming no owner is unchanged.
	if r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + w.idA + "?computer_id=host", internal: true}).do(w.h); r.Code != http.StatusOK {
		t.Fatalf("internal unscoped read = %d, want 200", r.Code)
	}
}

func TestOwnerReadsGuestsAreScopedByTheirComputersOwner(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	// A guest of owner A reads A's release, from either of A's computers (F7).
	for _, g := range [][3]string{{"computer-a1", tapA1, "computer-a1"}, {"computer-a2", tapA2, "computer-a2"}} {
		if r := w.one(w.idA, g[0], g[1], g[2]); r.Code != http.StatusOK {
			t.Fatalf("%s reads owner-a's release = %d %s", g[0], r.Code, r.Body.String())
		}
		if r := w.blob(w.shaA, g[0], g[1], g[2]); r.Code != http.StatusOK {
			t.Fatalf("%s reads owner-a's blob = %d", g[0], r.Code)
		}
		// F6, F3: and not B's.
		if r := w.one(w.idB, g[0], g[1], g[2]); r.Code != http.StatusNotFound {
			t.Fatalf("%s reads owner-b's release = %d, want 404", g[0], r.Code)
		}
		if r := w.blob(w.shaB, g[0], g[1], g[2]); r.Code != http.StatusNotFound {
			t.Fatalf("%s reads owner-b's blob = %d, want 404", g[0], r.Code)
		}
	}
	// And the reverse.
	if r := w.one(w.idB, "computer-b1", tapB1, "computer-b1"); r.Code != http.StatusOK {
		t.Fatalf("owner-b guest reads own = %d", r.Code)
	}
	if r := w.one(w.idA, "computer-b1", tapB1, "computer-b1"); r.Code != http.StatusNotFound {
		t.Fatalf("owner-b guest reads owner-a's = %d, want 404", r.Code)
	}
	// F4: a missing release answers exactly as an unseen one.
	unseen, missing := w.one(w.idB, "computer-a1", tapA1, "computer-a1"), w.one(strings.Repeat("8", 64), "computer-a1", tapA1, "computer-a1")
	if unseen.Code != missing.Code || unseen.Body.String() != missing.Body.String() {
		t.Fatalf("unseen = %d %q, missing = %d %q", unseen.Code, unseen.Body.String(), missing.Code, missing.Body.String())
	}
	unseenBlob, missingBlob := w.blob(w.shaB, "computer-a1", tapA1, "computer-a1"), w.blob(strings.Repeat("7", 64), "computer-a1", tapA1, "computer-a1")
	if unseenBlob.Code != missingBlob.Code || unseenBlob.Body.String() != missingBlob.Body.String() {
		t.Fatalf("unseen blob = %d %q, missing blob = %d %q", unseenBlob.Code, unseenBlob.Body.String(), missingBlob.Code, missingBlob.Body.String())
	}
}

func TestOwnerReadsGuestCannotWidenItsScope(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	// F5: an owner_id the guest names is ignored; vmctl's binding decides.
	r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + w.idB + "?computer_id=computer-a1&owner_id=owner-b", remote: tapA1, token: "computer-a1"}).do(w.h)
	if r.Code != http.StatusNotFound {
		t.Fatalf("guest naming owner-b = %d %s, want 404", r.Code, r.Body.String())
	}
	r = (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/blobs/" + w.shaB + "?computer_id=computer-a1&owner_id=owner-b", remote: tapA1, token: "computer-a1"}).do(w.h)
	if r.Code != http.StatusNotFound {
		t.Fatalf("guest naming owner-b on the blob = %d, want 404", r.Code)
	}
	// F12: computer-a1's capability presented from owner-b's network binding.
	if r := w.one(w.idA, "computer-a1", tapB1, "computer-a1"); r.Code == http.StatusOK {
		t.Fatalf("capability of computer-a1 from computer-b1's tap read a release")
	}
	// F11: a guest vmctl cannot place fails closed, not open.
	if r := w.one(w.idA, "computer-a1", tapZ, "computer-a1"); r.Code == http.StatusOK {
		t.Fatalf("unplaceable guest read a release")
	}
	if r := w.blob(w.shaA, "computer-a1", tapZ, "computer-a1"); r.Code == http.StatusOK {
		t.Fatalf("unplaceable guest read a blob")
	}
	h2 := newOwnerReadsWorld(t, ReleaseReadsOwner)
	h2.h.guestLookup = nil
	if r := h2.one(h2.idA, "computer-a1", tapA1, "computer-a1"); r.Code == http.StatusOK {
		t.Fatalf("a handler with no guest lookup served a guest in owner mode")
	}
}

func TestOwnerReadsBlobIsTheOwnersEvenWhenAnotherOwnerPublishedTheSameBytes(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	// F13: owner-b published owner-a's patch bytes too (idB2), so owner-b can
	// read that blob, and owner-a still can.
	if r := w.blob(w.shaA, "computer-b1", tapB1, "computer-b1"); r.Code != http.StatusOK {
		t.Fatalf("owner-b reads the blob it published = %d", r.Code)
	}
	if r := w.blob(w.shaA, "computer-a1", tapA1, "computer-a1"); r.Code != http.StatusOK {
		t.Fatalf("owner-a reads the blob it published = %d", r.Code)
	}
	// The host-internal caller naming an owner is scoped; naming none is not.
	for owner, want := range map[string]int{"owner-a": http.StatusNotFound, "owner-b": http.StatusOK} {
		r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/blobs/" + w.shaB + "?computer_id=host&owner_id=" + owner, internal: true}).do(w.h)
		if r.Code != want {
			t.Fatalf("%s on owner-b's blob = %d, want %d", owner, r.Code, want)
		}
	}
	if r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/blobs/" + w.shaB + "?computer_id=host", internal: true}).do(w.h); r.Code != http.StatusOK {
		t.Fatalf("internal unscoped blob read = %d, want 200", r.Code)
	}
}

func TestOwnerReadsTombstonedReleaseIsNotServed(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	og := w.h.service.ogStore()
	obj, err := og.GetObjectByMetadata(t.Context(), string(SoftwareReleaseKind), "$.release_id", w.idA)
	if err != nil {
		t.Fatal(err)
	}
	obj.Tombstone = true
	if err := og.PutObject(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	// F15
	if r := w.one(w.idA, "computer-a1", tapA1, "computer-a1"); r.Code != http.StatusNotFound {
		t.Fatalf("tombstoned release = %d, want 404", r.Code)
	}
}

func TestOpenModeReproducesTodaysReadsExactly(t *testing.T) {
	// F10: at open every owner sees every release, whoever asks and whatever
	// owner they name, and the answers are the ones before S1.
	for _, mode := range []ReleaseReadsMode{ReleaseReadsOpen} {
		w := newOwnerReadsWorld(t, mode)
		if all := w.list("?owner_id=owner-b"); !has(all, w.idA, w.idB, w.idB2) || len(all) != 3 {
			t.Fatalf("open list naming owner-b = %v, want all three", all)
		}
		if all := w.list(""); len(all) != 3 {
			t.Fatalf("open list = %v", all)
		}
		for _, g := range [][3]string{{"computer-a1", tapA1, "computer-a1"}, {"computer-b1", tapB1, "computer-b1"}} {
			for _, id := range []string{w.idA, w.idB} {
				if r := w.one(id, g[0], g[1], g[2]); r.Code != http.StatusOK {
					t.Fatalf("open: %s reads %s = %d", g[0], id, r.Code)
				}
			}
			for _, sha := range []string{w.shaA, w.shaB} {
				if r := w.blob(sha, g[0], g[1], g[2]); r.Code != http.StatusOK {
					t.Fatalf("open: %s reads blob = %d", g[0], r.Code)
				}
			}
		}
		// A guest that vmctl cannot place still reads at open: today's
		// capability-only check.
		if r := w.one(w.idA, "computer-a1", tapZ, "computer-a1"); r.Code != http.StatusOK {
			t.Fatalf("open: unplaceable guest = %d, want 200 as before", r.Code)
		}
		if r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + w.idA + "?computer_id=host&owner_id=owner-b", internal: true}).do(w.h); r.Code != http.StatusOK {
			t.Fatalf("open: host read naming owner-b = %d, want 200", r.Code)
		}
	}
}

func TestUnconfiguredHandlerReadsAreOwnerScoped(t *testing.T) {
	// F16: the zero value of the mode is owner, not open.
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	w.h.releaseReads = "" // as NewHandler leaves it when nothing configures it
	if r := w.one(w.idB, "computer-a1", tapA1, "computer-a1"); r.Code != http.StatusNotFound {
		t.Fatalf("unconfigured handler: guest of owner-a reads owner-b's release = %d, want 404", r.Code)
	}
	if r := w.blob(w.shaB, "computer-a1", tapA1, "computer-a1"); r.Code != http.StatusNotFound {
		t.Fatalf("unconfigured handler: guest of owner-a reads owner-b's blob = %d, want 404", r.Code)
	}
	if got := w.list("?owner_id=owner-a"); len(got) != 1 || got[0] != w.idA {
		t.Fatalf("unconfigured handler list = %v, want only owner-a's", got)
	}
}

// forgedRelease is an object a bound guest could insert under its own
// owner and computer: the kind, owner and canonical id are well formed, and
// the metadata and body say whatever the forger likes.
func forgedRelease(t *testing.T, owner, computer, suffix string, metadata map[string]any, body []byte) objectgraph.Object {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(SoftwareReleaseKind, owner, suffix)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	return objectgraph.Object{
		CanonicalID: id, ObjectKind: SoftwareReleaseKind, OwnerID: owner, ComputerID: computer,
		VersionID: suffix, Body: body, Metadata: meta,
		ContentHash: objectgraph.ContentHash(SoftwareReleaseKind, body, meta),
		CreatedAt:   now, UpdatedAt: now,
	}
}

func putForged(t *testing.T, h *Handler, objs ...objectgraph.Object) {
	t.Helper()
	for _, obj := range objs {
		if err := h.service.ogStore().PutObject(t.Context(), obj); err != nil {
			t.Fatalf("forge %s: %v", obj.CanonicalID, err)
		}
	}
}

func TestForgedReleaseObjectsDoNotOpenAnotherOwnersBlob(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	_, _, rawB := testRelease(t, patchOwnerB, "computer-b1", "owner-b")
	// F17: owner-a forges releases under owner-a naming owner-b's patch.
	putForged(t, w.h,
		forgedRelease(t, "owner-a", "computer-a1", "forged-garbage",
			map[string]any{"release_id": strings.Repeat("c", 64), "patch_sha256": w.shaB}, []byte("not a record")),
		forgedRelease(t, "owner-a", "computer-a1", "forged-copy",
			map[string]any{"release_id": w.idB, "patch_sha256": w.shaB}, rawB), // a real record, not owner-a's
		forgedRelease(t, "owner-a", "computer-a1", "forged-nobody",
			map[string]any{"patch_sha256": w.shaB}, nil),
	)
	if r := w.blob(w.shaB, "computer-a1", tapA1, "computer-a1"); r.Code != http.StatusNotFound {
		t.Fatalf("owner-a's guest reads owner-b's blob through forged metadata = %d, want 404", r.Code)
	}
	r := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/blobs/" + w.shaB + "?computer_id=host&owner_id=owner-a", internal: true}).do(w.h)
	if r.Code != http.StatusNotFound {
		t.Fatalf("host read naming owner-a on owner-b's blob = %d, want 404", r.Code)
	}
	// The forgeries are not listed or readable as releases either.
	if got := w.list("?owner_id=owner-a"); len(got) != 1 || got[0] != w.idA {
		t.Fatalf("owner-a list with forgeries = %v, want only %s", got, w.idA)
	}
	// And the real owner still reads their blob.
	if r := w.blob(w.shaB, "computer-b1", tapB1, "computer-b1"); r.Code != http.StatusOK {
		t.Fatalf("owner-b reads own blob = %d", r.Code)
	}
}

func TestForgedReleaseObjectsCannotShadowARealRelease(t *testing.T) {
	_, idA, rawA := testRelease(t, patchOwnerA, "computer-a1", "owner-a")
	w := newOwnerReadsWorldForged(t, ReleaseReadsOwner, func(h *Handler) {
		// F18, in the store before the real release is published: owner-b
		// forges objects that claim owner-a's release id.
		putForged(t, h,
			forgedRelease(t, "owner-b", "computer-b1", "shadow-garbage",
				map[string]any{"release_id": idA, "publisher_computer_id": "computer-b1"}, []byte("not a record")),
			// A copy of owner-a's real record, filed under owner-b at the
			// canonical id owner-b's own id space allows for that release.
			forgedRelease(t, "owner-b", "computer-b1", idA,
				map[string]any{"release_id": idA, "publisher_computer_id": "computer-b1"}, rawA),
		)
	})
	if w.idA != idA {
		t.Fatalf("test setup: release id %s != %s", w.idA, idA)
	}
	host := func(owner string) *httptest.ResponseRecorder {
		q := "?computer_id=host"
		if owner != "" {
			q += "&owner_id=" + owner
		}
		return (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + idA + q, internal: true}).do(w.h)
	}
	// The owner reads their release, however the lookup would order rows.
	if r := host("owner-a"); r.Code != http.StatusOK {
		t.Fatalf("owner-a reads own release under shadowing = %d %s, want 200", r.Code, r.Body.String())
	}
	if r := w.one(idA, "computer-a1", tapA1, "computer-a1"); r.Code != http.StatusOK {
		t.Fatalf("owner-a's guest reads own release under shadowing = %d", r.Code)
	}
	// The host, naming no owner, gets the verifying one, not a 500.
	if r := host(""); r.Code != http.StatusOK {
		t.Fatalf("unscoped host read under shadowing = %d %s, want 200", r.Code, r.Body.String())
	}
	// The forger does not read owner-a's release by filing a copy of it.
	if r := host("owner-b"); r.Code != http.StatusNotFound {
		t.Fatalf("owner-b reads owner-a's release through a filed copy = %d, want 404", r.Code)
	}
	if r := w.one(idA, "computer-b1", tapB1, "computer-b1"); r.Code != http.StatusNotFound {
		t.Fatalf("owner-b's guest reads owner-a's release through a filed copy = %d, want 404", r.Code)
	}
	if got := w.list("?owner_id=owner-b"); has(got, idA) || len(got) != 2 {
		t.Fatalf("owner-b list with a filed copy = %v, want only its own two", got)
	}
}

func TestBlobGateIsNotPushedOutByManySameDigestObjects(t *testing.T) {
	w := newOwnerReadsWorld(t, ReleaseReadsOwner)
	// F19: 210 newer forged objects by another owner naming owner-a's digest.
	for i := 0; i < 210; i++ {
		flood := forgedRelease(t, "owner-z", "computer-z", fmt.Sprintf("flood-%03d", i),
			map[string]any{"patch_sha256": w.shaA}, []byte("x"))
		flood.UpdatedAt = flood.UpdatedAt.Add(time.Hour) // newer than the real releases, whatever the clock's grain
		putForged(t, w.h, flood)
	}
	if r := w.blob(w.shaA, "computer-a1", tapA1, "computer-a1"); r.Code != http.StatusOK {
		t.Fatalf("owner-a reads its own blob under a flood of same-digest objects = %d, want 200", r.Code)
	}
}

// F20: a bound guest cannot write the kind through the generic object routes.
func TestGenericObjectRoutesRefuseReleases(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	h := NewObjectGraphHandler(svc, mem)
	a := &vmctl.GuestOwnership{Found: true, UserID: "owner-a", ComputerID: "computer-a1"}
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": a}); err != nil {
		t.Fatal(err)
	}
	send := func(method, from string, internal bool, body any) int {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "/internal/platform/objects", bytes.NewReader(raw))
		req.RemoteAddr = from + ":40000"
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		h.HandleObjects(rec, req)
		return rec.Code
	}
	create := map[string]any{"kind": "choir.software_release", "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "k", "metadata": map[string]any{"patch_sha256": strings.Repeat("a", 64)}}
	if code := send(http.MethodPost, "10.200.1.2", false, create); code != http.StatusForbidden {
		t.Fatalf("guest create of a release object = %d, want 403", code)
	}
	obj := forgedRelease(t, "owner-a", "computer-a1", "forged", map[string]any{"patch_sha256": strings.Repeat("a", 64)}, []byte("x"))
	if code := send(http.MethodPut, "10.200.1.2", false, obj); code != http.StatusForbidden {
		t.Fatalf("guest put of a release object = %d, want 403", code)
	}
	if _, err := svc.GetObject(t.Context(), obj.CanonicalID); err == nil {
		t.Fatal("a release object was written through the generic route")
	}
}
