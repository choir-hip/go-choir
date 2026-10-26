package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

func newOwnerReadsWorld(t *testing.T, mode ReleaseReadsMode) *ownerReadsWorld {
	t.Helper()
	h, _ := newReleaseHandler(t)
	h.eventAuth = releaseMultiAuth{"computer-a1": true, "computer-a2": true, "computer-b1": true}
	h.guestLookup = releaseTaps{
		tapA1: {UserID: "owner-a", ComputerID: "computer-a1"},
		tapA2: {UserID: "owner-a", ComputerID: "computer-a2"},
		tapB1: {UserID: "owner-b", ComputerID: "computer-b1"},
	}
	h.SetReleaseReads(mode)
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
	patchA := []byte("diff --git a/frontend/public/a.txt b/frontend/public/a.txt\nowner a\n")
	patchB := []byte("diff --git a/frontend/public/b.txt b/frontend/public/b.txt\nowner b\n")
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
