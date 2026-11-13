package platform

// Review round 1 (written before the repair):
//  B1. the destroyed-computer purge keeps every chunk its own snapshot root
//      names, so the tree never empties, the rows never go and every retry
//      repeats; registration accepts a root for a tombstoned computer;
//  S1. the collector queries the live roots for every candidate even when no
//      root was committed since it classified; and holding the shared lock
//      across a slow commit would queue every event append behind a waiting
//      collector Lock;
//  S3. nothing pins what happens to an unlink while the commit is pending.

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// B1: the exact fixture. The computer's snapshot root names its own real
// chunk file; the purge must still empty the tree and drop the rows.
func TestPurgeOfADestroyedComputerWhoseSnapshotRootNamesItsOwnChunks(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 2)
	if f.rowCount("computer_snapshot_roots", "dead-1") != 1 {
		t.Fatal("setup: no snapshot root")
	}
	req := destroyedReq(true)
	dry := f.dryRun(req)
	report, err := f.apply(dry, req)
	if err != nil {
		t.Fatal(err)
	}
	if report.Apply.Errors != 0 {
		t.Fatalf("apply errors: %+v", report.Apply)
	}
	if left := f.treeFiles("dead-1"); len(left) != 0 {
		t.Fatalf("files left after the purge: %v", left)
	}
	if f.treeDirsExist("dead-1") {
		t.Fatal("directories left after the purge")
	}
	for _, table := range []string{"computer_snapshot_roots", "computer_key_escrows", "computer_file_roots"} {
		if n := f.rowCount(table, "dead-1"); n != 0 {
			t.Fatalf("%s rows left: %d", table, n)
		}
	}
}

func TestSnapshotRootForATombstonedComputerIsRefused(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	d, _ := f.snapChunk("c", time.Hour)
	if err := f.store.putTapeTombstone(f.ctx, TapeTombstone{ComputerID: snapVerifyComputer, Status: tapeTombstoneArchived, HeadSequence: 5, CanonicalEventHead: platformTestDigest('1'), ManifestRun: "ts-x", ManifestSHA256: platformTestDigest('2'), ArchivePath: "/archive/x.tar.zst", ProofJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	if code := f.snapDeclare(h, "r", 1, d); code != http.StatusConflict {
		t.Fatalf("root for a tombstoned computer = %d, want 409", code)
	}
	if n := len(f.snapRootNames()); n != 0 {
		t.Fatalf("a root was recorded for a tombstoned computer: %d", n)
	}
}

// S1: no root committed since classification: the collector asks the
// database nothing.
func TestCollectorSkipsTheLiveRootQueryWhenNoRootCommittedSinceClassification(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	old, _ := f.snapChunk("old", 30*time.Hour)
	if code := f.snapDeclare(h, "old", 1, old); code != http.StatusCreated {
		t.Fatalf("declare = %d", code)
	}
	var paths []string
	for i := 0; i < 5; i++ {
		_, p := f.snapChunk(fmt.Sprintf("unrooted %d", i), 30*time.Hour)
		paths = append(paths, p)
	}
	f.service.snapshotRootRecheckQueries.Store(0)
	f.run(activeWindow())
	for _, p := range paths {
		if exists(p) {
			t.Fatal("an unrooted aged chunk survived")
		}
	}
	if n := f.service.snapshotRootRecheckQueries.Load(); n != 0 {
		t.Fatalf("the collector queried the live roots %d times with no root committed since classification", n)
	}
}

// S1: a root committed after classification forces the query, and the chunk
// of that root is kept.
func TestCollectorQueriesTheLiveRootsWhenARootCommittedAfterClassification(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	old, _ := f.snapChunk("old", 30*time.Hour)
	if code := f.snapDeclare(h, "old", 1, old); code != http.StatusCreated {
		t.Fatalf("declare = %d", code)
	}
	slow, slowPath := f.snapChunk("slow capture", 30*time.Hour)
	f.service.snapshotRootRecheckQueries.Store(0)
	f.service.artifactGCBeforeUnlink = func(rel string) {
		if strings.HasSuffix(rel, slow) {
			if code := f.snapDeclare(h, "new", 2, old, slow); code != http.StatusCreated {
				t.Errorf("register = %d", code)
			}
		}
	}
	f.run(activeWindow())
	if !exists(slowPath) {
		t.Fatal("the chunk of a root committed after classification was deleted")
	}
	if f.service.snapshotRootRecheckQueries.Load() == 0 {
		t.Fatal("a root committed after classification but the collector did not query")
	}
}

// S1/S3: the registration does not hold the shared lock across the commit (a
// waiting Lock would queue every event append behind a slow commit), yet an
// unlink that lands while the commit is pending keeps the chunk, and the
// chunk stays kept after the commit.
func TestAnUnlinkDuringABlockedRootCommitKeepsTheChunkWithoutHoldingTheLock(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	d, p := f.snapChunk("capture", 40*time.Hour)
	rel := "sha256/file-cas-chunks/" + snapVerifyComputer + "/" + d
	cand := gcCandidate{ns: "file-cas-chunks", rel: rel, path: p}
	f.service.snapshotRootAfterVerify = func() {
		if !f.store.casReuseMu.TryLock() {
			t.Error("the shared lock is still held while the root commit is pending")
			return
		}
		f.store.casReuseMu.Unlock()
		op, _, err := f.service.unlinkIfStillUnreachable(cand, time.Now().Add(-24*time.Hour))
		if err != nil || op == "deleted" || !exists(p) {
			t.Errorf("an unlink during the pending commit deleted the chunk: %q %v", op, err)
		}
	}
	if code := f.snapDeclare(h, "r", 1, d); code != http.StatusCreated {
		t.Fatalf("declare = %d", code)
	}
	f.service.snapshotRootAfterVerify = nil
	op, _, err := f.service.unlinkIfStillUnreachable(cand, time.Now().Add(-24*time.Hour))
	if err != nil || op == "deleted" || !exists(p) {
		t.Fatalf("an unlink after the commit deleted the chunk: %q %v", op, err)
	}
}
