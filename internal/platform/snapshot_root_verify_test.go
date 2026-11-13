package platform

// Failure list for snapshot root registration (written before the code;
// docs/problems/snapshot-root-recorded-without-chunk-check-2026-11-12.md):
//  1. a root naming a chunk the host does not hold is recorded, and the older
//     good root is pruned (the newest recovery base is silently unrestorable);
//  2. the refusal is not a 409 that names the missing chunk;
//  3. the collector classified a chunk unrooted, the root is then registered
//     for it, and the collector's unlink still deletes it (stale live set);
//  4. the collector deleted the chunk first and the root registration after
//     it is still accepted;
//  5. the collector can unlink between the registration's check and its
//     commit (the registration does not exclude the unlink);
//  6. the unlink path trusts mtime alone and deletes a chunk a live snapshot
//     root names;
//  7. verifying the paper's roughly 10,500 chunks is too slow for the guest's
//     request.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const snapVerifyComputer = "computer-verify"

func (f *gcFixture) snapChunk(name string, age time.Duration) (string, string) {
	d := gcDigest(snapVerifyComputer + "/" + name)
	return d, f.writeAged("sha256/file-cas-chunks/"+snapVerifyComputer+"/"+d, []byte(name), age)
}

func (f *gcFixture) snapDeclare(h *Handler, name string, head int64, chunks ...string) int {
	f.t.Helper()
	return putSnapshotRoot(f.t, h, snapshotRootRequest{ComputerID: snapVerifyComputer, Root: gcDigest("root/" + name), HeadSequence: head, Chunks: chunks}, true)
}

func (f *gcFixture) snapRootNames() map[string]bool {
	f.t.Helper()
	roots, err := f.store.LiveSnapshotRoots(f.ctx, snapVerifyComputer)
	if err != nil {
		f.t.Fatal(err)
	}
	out := map[string]bool{}
	for _, r := range roots {
		out[r.Root] = true
	}
	return out
}

func putSnapshotRootBody(t *testing.T, h *Handler, body snapshotRootRequest) (string, int) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/internal/computers/files/snapshot-root", bytes.NewReader(raw))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleSnapshotRoot(rec, req)
	return rec.Body.String(), rec.Code
}

// Modes 1 and 2.
func TestSnapshotRootRefusedForMissingChunkAndNothingPruned(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	// Two large roots fill the large-store retention of two.
	big := func(name string) []string {
		out := make([]string, 0, largeSnapshotChunks+1)
		for i := 0; i <= largeSnapshotChunks; i++ {
			out = append(out, gcDigest(fmt.Sprintf("%s/%d", name, i)))
		}
		return out
	}
	for i, name := range []string{"r1", "r2"} {
		chunks := big(name)
		for _, d := range chunks {
			f.writeAged("sha256/file-cas-chunks/"+snapVerifyComputer+"/"+d, []byte(d), time.Hour)
		}
		if code := f.snapDeclare(h, name, int64(10+i), chunks...); code != http.StatusCreated {
			t.Fatalf("declare %s = %d", name, code)
		}
	}
	before := f.snapRootNames()
	if len(before) != 2 {
		t.Fatalf("setup: %v", before)
	}
	chunks := big("r3")
	for _, d := range chunks[1:] { // the first is missing
		f.writeAged("sha256/file-cas-chunks/"+snapVerifyComputer+"/"+d, []byte(d), time.Hour)
	}
	missing := chunks[0]
	raw, code := putSnapshotRootBody(t, h, snapshotRootRequest{ComputerID: snapVerifyComputer, Root: gcDigest("root/r3"), HeadSequence: 12, Chunks: chunks})
	if code != http.StatusConflict {
		t.Fatalf("missing chunk: status %d, want 409 (%s)", code, raw)
	}
	if !strings.Contains(raw, missing) || !strings.Contains(raw, "snapshot root names chunk") {
		t.Fatalf("refusal does not name the chunk: %s", raw)
	}
	after := f.snapRootNames()
	if len(after) != 2 || !after[gcDigest("root/r1")] || !after[gcDigest("root/r2")] || after[gcDigest("root/r3")] {
		t.Fatalf("a refused root changed the live roots: %v", after)
	}
}

// Mode 3: the collector's live set is read before the root exists.
func TestSnapshotRootRegisteredAfterClassificationSurvivesUnlink(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	old, _ := f.snapChunk("old", 30*time.Hour)
	if code := f.snapDeclare(h, "old", 1, old); code != http.StatusCreated {
		t.Fatalf("declare old = %d", code)
	}
	slow, slowPath := f.snapChunk("slow capture", 30*time.Hour)
	f.service.artifactGCBeforeUnlink = func(rel string) {
		if strings.HasSuffix(rel, slow) {
			if code := f.snapDeclare(h, "new", 2, old, slow); code != http.StatusCreated {
				t.Errorf("register during the sweep = %d", code)
			}
		}
	}
	f.run(activeWindow())
	if !exists(slowPath) {
		t.Fatal("a chunk of a root registered after classification was deleted")
	}
}

// Mode 4: the collector wins; the registration must notice.
func TestSnapshotRootRegisteredAfterUnlinkIsRefused(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	old, _ := f.snapChunk("old", 30*time.Hour)
	if code := f.snapDeclare(h, "old", 1, old); code != http.StatusCreated {
		t.Fatalf("declare old = %d", code)
	}
	slow, slowPath := f.snapChunk("slow capture", 30*time.Hour)
	f.run(activeWindow())
	if exists(slowPath) {
		t.Fatal("setup: the unrooted aged chunk should be collected")
	}
	if code := f.snapDeclare(h, "new", 2, old, slow); code != http.StatusConflict {
		t.Fatalf("root for a collected chunk = %d, want 409", code)
	}
	if names := f.snapRootNames(); len(names) != 1 || !names[gcDigest("root/old")] {
		t.Fatalf("live roots after a refusal: %v", names)
	}
}

// Mode 6: mtime alone is not enough.
func TestUnlinkKeepsChunkOfLiveSnapshotRoot(t *testing.T) {
	f := newGCFixture(t)
	d, p := f.snapChunk("live", 40*time.Hour)
	if err := f.store.recordSnapshotRoot(f.ctx, snapVerifyComputer, gcDigest("root/live"), 1, []string{d}); err != nil {
		t.Fatal(err)
	}
	rel := "sha256/file-cas-chunks/" + snapVerifyComputer + "/" + d
	op, _, err := f.service.unlinkIfStillUnreachable(f.ctx, gcCandidate{ns: "file-cas-chunks", rel: rel, path: p}, time.Now().Add(-24*time.Hour))
	if err != nil || op == "deleted" || !exists(p) {
		t.Fatalf("a chunk of a live snapshot root was unlinked: op %q err %v", op, err)
	}
	// An unrooted old chunk is still collectable.
	u, up := f.snapChunk("unrooted", 40*time.Hour)
	op, _, err = f.service.unlinkIfStillUnreachable(f.ctx, gcCandidate{ns: "file-cas-chunks", rel: "sha256/file-cas-chunks/" + snapVerifyComputer + "/" + u, path: up}, time.Now().Add(-24*time.Hour))
	if err != nil || op != "deleted" {
		t.Fatalf("unrooted old chunk: op %q err %v", op, err)
	}
}

// Mode 7: the paper's registration names about 10,500 chunks. The test fails
// if verification takes long enough to need a larger request timeout than the
// guest's (the guest client's timeout is checked by the caller of this
// measure; the bound here is 5 s, far under any request timeout).
func TestSnapshotRootVerifyCostAtPaperScale(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	const n = 10500
	chunks := make([]string, 0, n)
	for i := 0; i < n; i++ {
		d := gcDigest(fmt.Sprintf("paper/%d", i))
		f.writeAged("sha256/file-cas-chunks/"+snapVerifyComputer+"/"+d, []byte{1}, time.Hour)
		chunks = append(chunks, d)
	}
	start := time.Now()
	raw, code := putSnapshotRootBody(t, h, snapshotRootRequest{ComputerID: snapVerifyComputer, Root: gcDigest("root/paper"), HeadSequence: 1, Chunks: chunks})
	took := time.Since(start)
	if code != http.StatusCreated {
		t.Fatalf("register %d chunks = %d (%s)", n, code, raw)
	}
	t.Logf("registered %d chunks (verify + record) in %s", n, took)
	if took > 5*time.Second {
		t.Fatalf("registration of %d chunks took %s", n, took)
	}
	// And the unlink-side recheck over a store of this root.
	start = time.Now()
	rel := "sha256/file-cas-chunks/" + snapVerifyComputer + "/" + gcDigest("paper/0")
	if op, _, err := f.service.unlinkIfStillUnreachable(f.ctx, gcCandidate{ns: "file-cas-chunks", rel: rel, path: f.root + "/" + rel}, time.Now().Add(time.Hour)); err != nil || op == "deleted" {
		t.Fatalf("recheck: %q %v", op, err)
	}
	t.Logf("one unlink recheck against a %d-chunk root: %s", n, time.Since(start))
}
