package platform

// Failure list for "an unappended snapshot root never evicts a root the tape
// names" (docs/problems/unappended-snapshot-root-evicts-the-on-tape-root-2026-11-13.md,
// Stage 0 of docs/design/snapshots-cost-the-change-2026-11-13.md). Written
// before the code; each test names its item.
//
//  N1. two registrations whose appends fail evict the newest root the tape
//      names (both the 4-root and the 2-root retention);
//  N2. the root just registered is pruned before its append runs;
//  N3. RecoveryBase / the watermark route advertise a root the tape does not
//      name, and a computer with no named root does not answer 404;
//  N4. the key the guest writes is not 'snapshot:'||root (internal/snapshot
//      TestSnapshotRecordedKeyIsTheRegisteredRootTheHostMatches; here the
//      receipts come through the real append path of the drill computer);
//  N5. an append that lands late under the same key leaves the root unnamed
//      or pruned, or the prune after it misbehaves;
//  N6. old unnamed roots accumulate without bound;
//  N7. the collector (the sweep's live set or the root-time check) misses a
//      kept row, because of the old LIMIT retention (a test with 4 named
//      and 1 unnamed row);
//  N8. the retention no longer covers snapshot.MaxCandidates
//      (TestSnapshotRootRetentionCoversEveryRestoreCandidate, unchanged);
//  N9. a root bundle.Import writes looks unnamed forever, so it is pruned at
//      the next registration or never advertised;
//  N10. receipt compaction removes the naming receipt of a kept root (it
//      rewrites only the body column: receipt_compaction.go header and the
//      test below, which names a root, compacts and still sees it named);
//  N11. a computer with no named root is refused by vmctl's admission as
//      before (internal/vmctl TestRecoveryAdmissionNoNamedRoot...).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// registerRoot registers a root at the drill computer's head, as the host
// does for the guest, with synthetic chunk digests (the collector is not
// involved). n chunks above largeSnapshotChunks gives retention 2.
func (c *drillComputer) registerRoot(name string, n int) (string, int64) {
	c.t.Helper()
	head := int64(c.head().Sequence)
	root := gcDigest("root/" + c.id + "/" + name)
	chunks := make([]string, n)
	for i := range chunks {
		chunks[i] = gcDigest(fmt.Sprintf("%s/%s/%d", c.id, name, i))
	}
	if err := c.store.recordSnapshotRoot(context.Background(), c.id, root, head, chunks); err != nil {
		c.t.Fatal(err)
	}
	return root, head
}

func (c *drillComputer) keptRoots() []string {
	c.t.Helper()
	roots, err := c.store.LiveSnapshotRoots(context.Background(), c.id)
	if err != nil {
		c.t.Fatal(err)
	}
	out := make([]string, len(roots))
	for i, r := range roots {
		out[i] = r.Root
	}
	return out
}

func requireKept(t *testing.T, c *drillComputer, want ...string) {
	t.Helper()
	got := c.keptRoots()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("kept roots = %v, want %v", short(got), short(want))
	}
}

func short(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = s[:8]
	}
	return out
}

func requireBase(t *testing.T, c *drillComputer, wantRoot string, wantSeq int64) {
	t.Helper()
	seq, root, err := c.store.RecoveryBase(context.Background(), c.id)
	if err != nil || root != wantRoot || seq != wantSeq {
		t.Fatalf("recovery base = %d %.8s %v, want %d %.8s", seq, root, err, wantSeq, wantRoot)
	}
}

// N1, N2, N3.
func TestUnappendedRegistrationsNeverEvictTheRootTheTapeNames(t *testing.T) {
	for _, tc := range []struct {
		name      string
		chunks    int
		retention int
	}{{"small store keeps four", 3, snapshotRootRetention}, {"large store keeps two", largeSnapshotChunks + 1, largeSnapshotRetention}} {
		t.Run(tc.name, func(t *testing.T) {
			c := newDrillComputer(t, "computer-named-"+strings.Fields(tc.name)[0])
			c.appendEvent(computerevent.EventGenesisImported, "", nil)
			c.appendMixed(3, "pre")
			named, namedHead := c.registerRoot("on-tape", tc.chunks)
			c.appendSnapshotRecorded(named)
			var last string
			for i := 0; i < tc.retention+3; i++ {
				c.appendMixed(1, fmt.Sprintf("gap%d", i))
				last, _ = c.registerRoot(fmt.Sprintf("failed-%d", i), tc.chunks) // its append never runs
				requireKept(t, c, last, named)                                   // N2: the newest attempt stays
				requireBase(t, c, named, namedHead)                              // N3: never the unappended root
			}
		})
	}
}

// N3: a computer none of whose roots the tape names has no recovery base.
func TestRecoveryBaseIsAbsentWhenTheTapeNamesNoRoot(t *testing.T) {
	c := newDrillComputer(t, "computer-no-named")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	c.registerRoot("failed", 3)
	if _, _, err := c.store.RecoveryBase(context.Background(), c.id); !errors.Is(err, ErrNoWatermark) {
		t.Fatalf("recovery base of an unnamed root = %v, want ErrNoWatermark", err)
	}
	h := NewHandler(c.service)
	req := httptest.NewRequest(http.MethodGet, "/internal/computers/files/watermark?computer_id="+c.id, nil)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleFileCASWatermark(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("watermark = %d %s, want 404", rec.Code, rec.Body.String())
	}
}

// N5: the append lands later under the same key (a retry), and the prune
// then behaves as for any kept root. The late root is not unnamed forever.
func TestALateAppendMakesAnUnappendedRootAnOrdinaryKeptRoot(t *testing.T) {
	c := newDrillComputer(t, "computer-late")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	c.appendMixed(2, "pre")
	a, aHead := c.registerRoot("a", largeSnapshotChunks+1)
	c.appendSnapshotRecorded(a)
	c.appendMixed(1, "x")
	b, bHead := c.registerRoot("b", largeSnapshotChunks+1) // unappended
	requireKept(t, c, b, a)
	requireBase(t, c, a, aHead)
	c.appendSnapshotRecorded(b) // lands late
	requireBase(t, c, b, bHead)
	c.appendMixed(1, "y")
	d, _ := c.registerRoot("d", largeSnapshotChunks+1) // unappended
	requireKept(t, c, d, b, a)                         // retention 2 named + the newest attempt
	requireBase(t, c, b, bHead)
	c.appendSnapshotRecorded(d)
	c.appendMixed(1, "z")
	e, _ := c.registerRoot("e", largeSnapshotChunks+1)
	requireKept(t, c, e, d, b) // a is now the third named root
}

// N6.
func TestOldUnappendedRootsDoNotAccumulate(t *testing.T) {
	c := newDrillComputer(t, "computer-accumulate")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	for i := 0; i < 30; i++ {
		c.appendMixed(1, fmt.Sprintf("n%d", i))
		c.registerRoot(fmt.Sprintf("none-%d", i), 3)
	}
	if got := len(c.keptRoots()); got != 1 {
		t.Fatalf("%d rows after 30 unappended registrations with none named, want 1", got)
	}
	c.appendMixed(1, "m")
	named, _ := c.registerRoot("named", 3)
	c.appendSnapshotRecorded(named)
	for i := 0; i < 30; i++ {
		c.appendMixed(1, fmt.Sprintf("m%d", i))
		c.registerRoot(fmt.Sprintf("more-%d", i), 3)
	}
	if got := len(c.keptRoots()); got != 2 {
		t.Fatalf("%d rows with one named root and 30 failures, want 2", got)
	}
}

// A named root below the head of an older unappended one is still named; an
// unappended root BELOW the newest named root is dropped, not kept.
func TestAnUnappendedRootBelowTheNewestNamedRootIsDropped(t *testing.T) {
	c := newDrillComputer(t, "computer-below")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	c.appendMixed(1, "a")
	old, _ := c.registerRoot("old-unappended", 3)
	c.appendMixed(1, "b")
	named, _ := c.registerRoot("named", 3)
	c.appendSnapshotRecorded(named)
	// The next registration prunes: old sits below named and is gone.
	c.appendMixed(1, "c")
	newest, _ := c.registerRoot("newest", 3)
	got := c.keptRoots()
	if len(got) != 2 || got[0] != newest || got[1] != named {
		t.Fatalf("kept %v, want [newest named] (%.8s dropped)", short(got), old)
	}
}

// N10: compaction rewrites only the body, so a naming receipt stays.
func TestCompactingReceiptsKeepsTheNamingReceipt(t *testing.T) {
	c := newDrillComputer(t, "computer-compact")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	c.appendMixed(2, "pre")
	named, namedHead := c.registerRoot("named", 3)
	c.appendSnapshotRecorded(named)
	var before int
	if err := c.store.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=?`, c.id).Scan(&before); err != nil {
		t.Fatal(err)
	}
	c.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{c.id: ReceiptCompactActive}})
	if passes := runCompaction(t, noSleepCompactor(c.store)); len(passes) != 1 || passes[0].Rewritten == 0 {
		t.Fatalf("compaction pass = %+v, want rows rewritten", passes)
	}
	var after int
	if err := c.store.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=?`, c.id).Scan(&after); err != nil || after != before {
		t.Fatalf("receipt rows %d -> %d, %v: compaction must not delete rows", before, after, err)
	}
	requireBase(t, c, named, namedHead)
}

// N9: a bundle imported into a fresh store brings its naming receipt with
// its root, so the root is a recovery base and survives later failures.
func TestAnImportedBundlesRootIsNamedAndSurvivesUnappendedRegistrations(t *testing.T) {
	ctx := context.Background()
	const id = "computer-import-named"
	src := newDrillComputer(t, id)
	src.appendEvent(computerevent.EventGenesisImported, "", nil)
	src.appendMixed(3, "pre")
	snap := src.snapshot(3)
	src.appendSnapshotRecorded(snap.Root)
	src.appendMixed(4, "post")
	out := filepath.Join(t.TempDir(), "bundle")
	mustExport(t, src, out)

	dst := newDrillComputer(t, id)
	if _, err := bundle.Import(ctx, out, dst.store.db, dst.root, src.trust()); err != nil {
		t.Fatal(err)
	}
	requireBase(t, dst, snap.Root, snap.HeadSequence)
	for i := 0; i < 5; i++ {
		dst.appendMixed(1, fmt.Sprintf("after%d", i))
		dst.registerRoot(fmt.Sprintf("failed-%d", i), largeSnapshotChunks+1)
	}
	requireBase(t, dst, snap.Root, snap.HeadSequence)
	found := false
	for _, r := range dst.keptRoots() {
		found = found || r == snap.Root
	}
	if !found {
		t.Fatalf("the imported root %.8s was pruned: kept %v", snap.Root, short(dst.keptRoots()))
	}
}

// N7: every kept row's chunks are protected from the collector, including a
// fifth row (four named and the newest unnamed), by the sweep's live set and
// by the root-time check; a dropped root's chunk is collectable.
func TestEveryKeptSnapshotRootRowIsProtectedFromTheCollector(t *testing.T) {
	f := newGCFixture(t)
	const computer = "computer-gc-named"
	old := 9 * 24 * time.Hour
	chunkOf := func(name string) (string, string) {
		d := gcDigest(computer + "/" + name)
		return d, f.writeAged("sha256/file-cas-chunks/"+computer+"/"+d, []byte(name), old)
	}
	register := func(name string, head int64, named bool) (string, string) {
		d, p := chunkOf(name)
		root := gcDigest("root/" + name)
		if err := f.store.recordSnapshotRoot(f.ctx, computer, root, head, []string{d}); err != nil {
			t.Fatal(err)
		}
		if named {
			f.nameRoot(computer, root, head)
		}
		return d, p
	}
	dropped, droppedPath := register("n0", 5, true)
	_ = dropped
	paths := map[string]string{}
	digests := map[string]string{}
	for i, head := range []int64{10, 20, 30, 40} {
		name := fmt.Sprintf("n%d", i+1)
		digests[name], paths[name] = register(name, head, true)
	}
	digests["u"], paths["u"] = register("u", 50, false)
	live, err := f.store.LiveSnapshotRoots(f.ctx, computer)
	if err != nil || len(live) != snapshotRootRetention+1 {
		t.Fatalf("kept %d rows, %v; want four named and one unnamed", len(live), err)
	}
	report := f.run(activeWindow())
	for name, p := range paths {
		if !exists(p) {
			t.Fatalf("chunk of kept root %s was swept (report %+v)", name, report.Namespaces["file-cas-chunks"])
		}
	}
	if exists(droppedPath) {
		t.Fatalf("chunk of the dropped root kept (report %+v)", report.Namespaces["file-cas-chunks"])
	}
	for name, d := range digests {
		rel := "sha256/file-cas-chunks/" + computer + "/" + d
		protected, err := f.service.chunkProtectedBySnapshotRoot(f.ctx, gcCandidate{rel: rel})
		if err != nil || !protected {
			t.Fatalf("root-time check does not protect kept root %s: %v %v", name, protected, err)
		}
	}
}

// nameRoot writes the receipt row (idempotent per root; a sequence clash with
// another root's receipt fails loudly) the host holds once the guest's
// snapshot_recorded append for root has landed above head.
func (f *gcFixture) nameRoot(computer, root string, head int64) {
	f.t.Helper()
	nameRootRow(f.t, f.store, computer, root, head)
}

func nameRootRow(t *testing.T, store *Store, computer, root string, head int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := store.db.ExecContext(context.Background(), `INSERT INTO computer_event_append_receipts (computer_id,idempotency_key,request_commitment,sequence,event_kind,event_digest,event_pin_receipt_digest,event_head_receipt_id,event_head_receipt_json,created_at) VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT (computer_id, idempotency_key) DO NOTHING`,
		computer, "snapshot:"+root, platformTestDigest('a'), head+1, string(computerevent.EventSnapshotRecorded), gcDigest("event/"+root), platformTestDigest('b'), "receipt-"+root[:8], "{}", now); err != nil {
		t.Fatal(err)
	}
}
