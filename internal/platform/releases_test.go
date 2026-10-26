package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/release"
)

// Failure modes from docs/reports/p6-p8-release-adopt-2026-10-15.md pinned
// here: R1 (id is not the digest), R2 (patch blob absent or altered), R3 (a
// guest publishes with its own capability), R4 (publisher fields name
// another computer or owner), R7 (the write bypasses the gate), R8 (a
// republish duplicates or replaces), R9 (oversize), R10 (a guest lists or
// writes), A3 (the stored record or blob is altered after publish).

// releaseGuestAuth binds a bearer token to one computer, the way the
// computer capability does.
type releaseGuestAuth struct{ computer string }

func (a releaseGuestAuth) Authorize(r *http.Request, computerID, scope string) error {
	if r.Header.Get("Authorization") != "Bearer "+a.computer || computerID != a.computer || scope != "event:read" {
		return fmt.Errorf("refused")
	}
	return nil
}
func (a releaseGuestAuth) AuthorizeRenewal(*http.Request, string) error { return fmt.Errorf("no") }

func testRelease(t *testing.T, patch []byte, computer, owner string) (release.Record, string, []byte) {
	t.Helper()
	record := release.Record{
		Schema:    release.Schema,
		Publisher: release.Publisher{ComputerID: computer, OwnerID: owner},
		Operation: release.Operation{
			OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), ReleaseDigest: strings.Repeat("c", 64),
			CodeRef: "code:sha256:" + strings.Repeat("d", 64), DecisionEvent: strings.Repeat("e", 64),
		},
		Source: release.Source{
			BaseCommit: strings.Repeat("1", 40), PatchSHA256: computerevent.DigestBytes(patch), PatchBytes: len(patch),
			TreeHash: strings.Repeat("2", 40), ChangedPaths: []string{"frontend/public/x.txt"},
		},
	}
	id, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := record.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return record, id, raw
}

type releaseCall struct {
	method, path, remote, token string
	internal                    bool
	body                        []byte
}

func (c releaseCall) do(h *Handler) *httptest.ResponseRecorder {
	r := httptest.NewRequest(c.method, c.path, bytes.NewReader(c.body))
	if c.remote != "" {
		r.RemoteAddr = c.remote
	}
	if c.internal {
		r.Header.Set("X-Internal-Caller", "true")
	}
	if c.token != "" {
		r.Header.Set("Authorization", "Bearer "+c.token)
	}
	w := httptest.NewRecorder()
	h.HandleReleases(w, r)
	return w
}

const guestTap = "10.200.1.2:40000"

func newReleaseHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	store, root := openTestPlatformStore(t)
	h := NewHandler(NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing-key")))
	h.eventAuth = releaseGuestAuth{computer: "computer-b"}
	// These tests pin the publish rules and today's open reads; the owner
	// scope has its own file (releases_owner_reads_test.go).
	h.SetReleaseReads(ReleaseReadsOpen)
	return h, root
}

func publishBody(t *testing.T, id string, raw []byte, owner, computer string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"release_id": id, "record": json.RawMessage(raw), "owner_id": owner, "computer_id": computer})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestReleasePublishIsHostOnlyBindsThePublisherAndRehashesTheBlob(t *testing.T) {
	h, root := newReleaseHandler(t)
	patch := []byte("diff --git a/frontend/public/x.txt b/frontend/public/x.txt\nnew file mode 100644\n--- /dev/null\n+++ b/frontend/public/x.txt\n@@ -0,0 +1 @@\n+token\n")
	sha := computerevent.DigestBytes(patch)
	_, id, raw := testRelease(t, patch, "computer-a", "owner-a")
	blobPath := "/internal/platform/releases/blobs/" + sha

	// R3/R10: a guest (even with its own capability) neither uploads nor publishes.
	if w := (releaseCall{method: http.MethodPut, path: blobPath, remote: guestTap, token: "computer-b", body: patch}).do(h); w.Code != http.StatusForbidden {
		t.Fatalf("guest blob put = %d, want 403", w.Code)
	}
	if w := (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", remote: guestTap, token: "computer-b", body: publishBody(t, id, raw, "owner-a", "computer-a")}).do(h); w.Code != http.StatusForbidden {
		t.Fatalf("guest publish = %d, want 403", w.Code)
	}
	if w := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases", remote: guestTap, token: "computer-b"}).do(h); w.Code != http.StatusForbidden {
		t.Fatalf("guest list = %d, want 403", w.Code)
	}

	// R2: publish before the blob is in the store.
	if w := (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", internal: true, body: publishBody(t, id, raw, "owner-a", "computer-a")}).do(h); w.Code != http.StatusConflict {
		t.Fatalf("publish without blob = %d %s, want 409", w.Code, w.Body.String())
	}
	// A blob upload whose bytes hash elsewhere is refused and leaves nothing.
	if w := (releaseCall{method: http.MethodPut, path: blobPath, internal: true, body: []byte("other bytes")}).do(h); w.Code != http.StatusBadRequest {
		t.Fatalf("mismatched blob = %d, want 400", w.Code)
	}
	if w := (releaseCall{method: http.MethodPut, path: blobPath, internal: true, body: patch}).do(h); w.Code != http.StatusCreated {
		t.Fatalf("blob put = %d %s", w.Code, w.Body.String())
	}

	// R1: the id is not the record's digest.
	if w := (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", internal: true, body: publishBody(t, strings.Repeat("9", 64), raw, "owner-a", "computer-a")}).do(h); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("id mismatch = %d %s, want 422", w.Code, w.Body.String())
	}
	// R4: the authorized computer or owner is not the record's publisher.
	for _, who := range [][2]string{{"owner-a", "computer-z"}, {"owner-z", "computer-a"}} {
		if w := (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", internal: true, body: publishBody(t, id, raw, who[0], who[1])}).do(h); w.Code != http.StatusForbidden {
			t.Fatalf("publisher %v = %d %s, want 403", who, w.Code, w.Body.String())
		}
	}

	w := (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", internal: true, body: publishBody(t, id, raw, "owner-a", "computer-a")}).do(h)
	var published struct {
		ReleaseID   string          `json:"release_id"`
		CanonicalID string          `json:"canonical_id"`
		Published   bool            `json:"published"`
		Record      json.RawMessage `json:"record"`
	}
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &published) != nil || published.ReleaseID != id || !published.Published {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	// R8: a republish is idempotent and names the same object.
	w = (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", internal: true, body: publishBody(t, id, raw, "owner-a", "computer-a")}).do(h)
	var again struct {
		CanonicalID string `json:"canonical_id"`
		Published   bool   `json:"published"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &again) != nil || again.Published || again.CanonicalID != published.CanonicalID {
		t.Fatalf("republish = %d %s", w.Code, w.Body.String())
	}
	objects, err := h.service.ogStore().ListObjects(t.Context(), objectgraph.ListFilter{Kind: SoftwareReleaseKind, Limit: 10})
	if err != nil || len(objects) != 1 {
		t.Fatalf("stored releases = %d, %v; want exactly one", len(objects), err)
	}

	// A3: a stored blob altered after publish fails the next publish that
	// names it (the store re-hashes, it never trusts a file by its name).
	_, _, raw2 := testRelease(t, patch, "computer-a", "owner-a")
	alteredPath := filepath.Join(root, "artifacts", "sha256", "release", sha)
	if err := os.Chmod(alteredPath, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alteredPath, []byte("tampered"), 0o640); err != nil {
		t.Fatal(err)
	}
	record2, err := release.Decode(raw2)
	if err != nil {
		t.Fatal(err)
	}
	record2.Declarations.TestCommand = "true"
	id3, _ := record2.ReleaseID()
	raw3, _ := record2.Canonical()
	if w := (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", internal: true, body: publishBody(t, id3, raw3, "owner-a", "computer-a")}).do(h); w.Code != http.StatusConflict {
		t.Fatalf("publish over an altered blob = %d %s, want 409", w.Code, w.Body.String())
	}
}

func TestReleaseReadsAdmitTheHostAndABoundGuestOnly(t *testing.T) {
	h, _ := newReleaseHandler(t)
	patch := []byte("diff --git a/x b/x\n")
	sha := computerevent.DigestBytes(patch)
	_, id, raw := testRelease(t, patch, "computer-a", "owner-a")
	if w := (releaseCall{method: http.MethodPut, path: "/internal/platform/releases/blobs/" + sha, internal: true, body: patch}).do(h); w.Code != http.StatusCreated {
		t.Fatalf("blob put = %d", w.Code)
	}
	if w := (releaseCall{method: http.MethodPost, path: "/internal/platform/releases", internal: true, body: publishBody(t, id, raw, "owner-a", "computer-a")}).do(h); w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}

	one := "/internal/platform/releases/" + id + "?computer_id=computer-b"
	if w := (releaseCall{method: http.MethodGet, path: one, remote: guestTap}).do(h); w.Code != http.StatusForbidden {
		t.Fatalf("guest without capability = %d, want 403", w.Code)
	}
	if w := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + id + "?computer_id=computer-c", remote: guestTap, token: "computer-b"}).do(h); w.Code != http.StatusForbidden {
		t.Fatalf("guest naming another computer = %d, want 403", w.Code)
	}
	w := (releaseCall{method: http.MethodGet, path: one, remote: guestTap, token: "computer-b"}).do(h)
	var got struct {
		ReleaseID string          `json:"release_id"`
		Record    json.RawMessage `json:"record"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.ReleaseID != id {
		t.Fatalf("guest read = %d %s", w.Code, w.Body.String())
	}
	decoded, err := release.Decode(got.Record)
	if err != nil || release.VerifyID(decoded, id) != nil {
		t.Fatalf("served record does not verify against its id: %v", err)
	}
	if w := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + strings.Repeat("8", 64) + "?computer_id=computer-b", remote: guestTap, token: "computer-b"}).do(h); w.Code != http.StatusNotFound {
		t.Fatalf("unpublished = %d, want 404", w.Code)
	}
	blob := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/blobs/" + sha + "?computer_id=computer-b", remote: guestTap, token: "computer-b"}).do(h)
	if blob.Code != http.StatusOK || !bytes.Equal(blob.Body.Bytes(), patch) {
		t.Fatalf("guest blob read = %d", blob.Code)
	}
	list := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases?limit=5", internal: true}).do(h)
	var listed struct {
		Releases []struct {
			ReleaseID string `json:"release_id"`
		} `json:"releases"`
	}
	if list.Code != http.StatusOK || json.Unmarshal(list.Body.Bytes(), &listed) != nil || len(listed.Releases) != 1 || listed.Releases[0].ReleaseID != id {
		t.Fatalf("host list = %d %s", list.Code, list.Body.String())
	}
}

func TestSoftwareReleaseKindPassesTheWriteGate(t *testing.T) {
	if err := objectgraph.CheckWrite([]objectgraph.Object{{CanonicalID: "obj:x", ObjectKind: SoftwareReleaseKind}}, nil); err != nil {
		t.Fatalf("R7: %v", err)
	}
	if string(SoftwareReleaseKind) != release.ObjectKind {
		t.Fatalf("kind %s and release.ObjectKind %s disagree", SoftwareReleaseKind, release.ObjectKind)
	}
}

func TestReleaseBlobUploadOverTheCapIsRefusedWithoutBuffering(t *testing.T) {
	h, root := newReleaseHandler(t)
	store := h.contentStore(contentNamespaceRelease)
	store.maxBytes = 16
	big := bytes.Repeat([]byte("x"), 17)
	if err := store.Put(computerevent.DigestBytes(big), bytes.NewReader(big)); err == nil {
		t.Fatal("R9: an upload over the cap was stored")
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "artifacts", "sha256", "release", "*")); len(matches) != 0 {
		t.Fatalf("a refused upload left files: %v", matches)
	}
}
