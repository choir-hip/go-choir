package platform

// Failure modes of snapshot roots (the chunk sweep must not collect a guest
// snapshot; internal/snapshot seals its manifest where the host cannot read):
//  1. a snapshot's chunks are swept because no file root names them, whether
//     the computer has a Files tree root or not;
//  2. the declaration is accepted without the computer's capability, or for
//     another computer;
//  3. keeping every snapshot ever taken: only the newest four stay live (at least the
//     snapshot.MaxCandidates a restore considers) and
//     the rest become collectable, so storage is bounded;
//  4. an older snapshot outliving a newer one (ordering by head sequence, not
//     arrival);
//  5. a malformed digest list is stored and later poisons the sweep;
//  6. a chunk shared by a live and a dropped snapshot is collected;
//  7. the Files tree's latest root is displaced by a snapshot (the two kinds
//     of root stay separate).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

func putSnapshotRoot(t *testing.T, h *Handler, body any, internal bool) int {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/internal/computers/files/snapshot-root", bytes.NewReader(raw))
	if internal {
		req.Header.Set("X-Internal-Caller", "true")
	}
	rec := httptest.NewRecorder()
	h.HandleSnapshotRoot(rec, req)
	return rec.Code
}

func TestSnapshotRootsKeepNewestFourAndSharedChunks(t *testing.T) {
	f := newGCFixture(t)
	handler := NewHandler(f.service)
	old := 9 * 24 * time.Hour
	const computer = "computer-snap"
	chunk := func(name string) (string, string) {
		d := gcDigest(computer + "/" + name)
		return d, f.writeAged("sha256/file-cas-chunks/"+computer+"/"+d, []byte(name), old)
	}
	shared, sharedPath := chunk("shared")
	paths := map[string]string{}
	digests := map[string]string{}
	for _, name := range []string{"s0", "s1", "s2", "s3", "s4"} {
		digests[name], paths[name] = chunk(name)
	}
	// Declared out of order: head sequence, not arrival, decides which stay.
	declare := func(name string, head int64, extra ...string) {
		t.Helper()
		root := gcDigest("root/" + name)
		chunks := append([]string{digests[name]}, extra...)
		if code := putSnapshotRoot(t, handler, snapshotRootRequest{ComputerID: computer, Root: root, HeadSequence: head, Chunks: chunks}, true); code != http.StatusCreated {
			t.Fatalf("declare %s = %d", name, code)
		}
	}
	declare("s4", 40)
	declare("s0", 5, shared) // oldest: dropped, but shares a chunk
	declare("s1", 10)
	declare("s3", 30, shared) // live and shares it
	declare("s2", 20)

	// Mode 7: a Files tree root exists, and snapshot chunks are not in it.
	treeChunk, treePath := chunk("tree")
	manifest, err := filecas.BuildManifest(computer, []filecas.FileEntry{{Path: "f", Mode: 0o600, Size: 1, Chunks: []string{treeChunk}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(manifest)
	ref, err := f.service.pinFileManifest(computer, manifest.Root, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RecordFileRoot(f.ctx, computer, manifest.Root, ref, 1); err != nil {
		t.Fatal(err)
	}
	if latest, err := f.store.LatestFileRoots(f.ctx, computer, 1); err != nil || len(latest) != 1 || latest[0].Root != manifest.Root {
		t.Fatalf("snapshot roots displaced the file root: %v %v", latest, err)
	}

	live, err := f.store.LiveSnapshotRoots(f.ctx, computer)
	if err != nil || len(live) != snapshotRootRetention || live[0].HeadSequence != 40 || live[3].HeadSequence != 10 {
		t.Fatalf("live snapshot roots = %+v, %v", live, err)
	}
	var rows int
	if err := f.store.db.QueryRowContext(f.ctx, `SELECT COUNT(*) FROM computer_snapshot_roots WHERE computer_id=?`, computer).Scan(&rows); err != nil || rows != snapshotRootRetention {
		t.Fatalf("rows kept = %d, %v (storage must stay bounded)", rows, err)
	}

	report := f.run(activeWindow())
	for name, p := range map[string]string{"s1": paths["s1"], "s2": paths["s2"], "s3": paths["s3"], "s4": paths["s4"], "shared": sharedPath, "tree": treePath} {
		if !exists(p) {
			t.Fatalf("live chunk %s was swept (report %+v)", name, report.Namespaces["file-cas-chunks"])
		}
	}
	if exists(paths["s0"]) {
		t.Fatalf("chunk of a dropped snapshot kept (report %+v)", report.Namespaces["file-cas-chunks"])
	}
}

// A restore considers the newest snapshot.MaxCandidates snapshots and falls
// back down the list when the newest is unusable; a fallback whose chunks were
// already swept is no fallback.
func TestSnapshotRootRetentionCoversEveryRestoreCandidate(t *testing.T) {
	if snapshotRootRetention < snapshot.MaxCandidates {
		t.Fatalf("snapshotRootRetention %d keeps fewer roots than the %d snapshots a restore may fall back to", snapshotRootRetention, snapshot.MaxCandidates)
	}
	// A large store trades fallback depth for host space, but always keeps
	// one snapshot to fall back to.
	if largeSnapshotRetention < 2 {
		t.Fatalf("largeSnapshotRetention %d leaves a large store no fallback", largeSnapshotRetention)
	}
}

// A large store keeps two roots, a small one four; a small root declared
// after large ones goes back to four.
func TestSnapshotRootsOfALargeStoreKeepTwo(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := t.Context()
	chunks := func(n int, salt string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = gcDigest(salt + "/" + string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676)))
		}
		return out
	}
	live := func() int {
		roots, err := store.LiveSnapshotRoots(ctx, "computer-large")
		if err != nil {
			t.Fatal(err)
		}
		return len(roots)
	}
	for seq := int64(1); seq <= 4; seq++ {
		if err := store.RecordSnapshotRoot(ctx, "computer-large", gcDigest("large root"+string(rune('0'+seq))), seq, chunks(largeSnapshotChunks+1, string(rune('0'+seq)))); err != nil {
			t.Fatal(err)
		}
	}
	if got := live(); got != largeSnapshotRetention {
		t.Fatalf("a large store keeps %d roots, want %d", got, largeSnapshotRetention)
	}
	for seq := int64(5); seq <= 8; seq++ {
		if err := store.RecordSnapshotRoot(ctx, "computer-large", gcDigest("small root"+string(rune('0'+seq))), seq, chunks(3, string(rune('0'+seq)))); err != nil {
			t.Fatal(err)
		}
	}
	if got := live(); got != snapshotRootRetention {
		t.Fatalf("after small snapshots the store keeps %d roots, want %d", got, snapshotRootRetention)
	}
}

func TestSnapshotRootsAloneProveALiveSetWithoutAFileRoot(t *testing.T) {
	f := newGCFixture(t)
	handler := NewHandler(f.service)
	const computer = "computer-snap-only"
	liveDigest := gcDigest("live")
	deadDigest := gcDigest("dead")
	livePath := f.writeAged("sha256/file-cas-chunks/"+computer+"/"+liveDigest, []byte("live"), 9*24*time.Hour)
	deadPath := f.writeAged("sha256/file-cas-chunks/"+computer+"/"+deadDigest, []byte("dead"), 9*24*time.Hour)
	if code := putSnapshotRoot(t, handler, snapshotRootRequest{ComputerID: computer, Root: gcDigest("r"), HeadSequence: 5, Chunks: []string{liveDigest}}, true); code != http.StatusCreated {
		t.Fatalf("declare = %d", code)
	}
	f.run(activeWindow())
	if !exists(livePath) {
		t.Fatal("snapshot chunk swept on a computer with no file root")
	}
	if exists(deadPath) {
		t.Fatal("unreferenced chunk kept although snapshot roots prove the live set")
	}
	if _, err := os.Stat(deadPath); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestSnapshotRootRejectsBadDeclarations(t *testing.T) {
	f := newGCFixture(t)
	handler := NewHandler(f.service)
	good := gcDigest("c")
	root := gcDigest("r")
	// Mode 2: no capability.
	if code := putSnapshotRoot(t, handler, snapshotRootRequest{ComputerID: "c1", Root: root, HeadSequence: 1, Chunks: []string{good}}, false); code != http.StatusForbidden {
		t.Fatalf("unauthenticated declaration = %d", code)
	}
	// Mode 5: malformed digests, an empty list, a bad root.
	for name, body := range map[string]snapshotRootRequest{
		"bad chunk":  {ComputerID: "c1", Root: root, HeadSequence: 1, Chunks: []string{"../../etc"}},
		"no chunks":  {ComputerID: "c1", Root: root, HeadSequence: 1},
		"bad root":   {ComputerID: "c1", Root: "x", HeadSequence: 1, Chunks: []string{good}},
		"bad sender": {ComputerID: "../c1", Root: root, HeadSequence: 1, Chunks: []string{good}},
	} {
		if code := putSnapshotRoot(t, handler, body, true); code != http.StatusBadRequest {
			t.Fatalf("%s accepted: %d", name, code)
		}
	}
	// Idempotent redeclaration.
	for i := 0; i < 2; i++ {
		if code := putSnapshotRoot(t, handler, snapshotRootRequest{ComputerID: "c1", Root: root, HeadSequence: 1, Chunks: []string{good}}, true); code != http.StatusCreated {
			t.Fatalf("declaration %d = %d", i, code)
		}
	}
	if live, err := f.store.LiveSnapshotRoots(f.ctx, "c1"); err != nil || len(live) != 1 {
		t.Fatalf("live = %+v, %v", live, err)
	}
}
