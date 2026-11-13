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
//      or pruned, or the prune after it misbehaves; and a root pruned by a
//      newer registration whose append then lands (the tape names a root the
//      host no longer keeps, its chunks collectable): the rows registered
//      within snapshotRootInFlightWindow stay: the one just registered, the
//      newest above the named roots and snapshotRootMaxYoungUnnamed more, so
//      four registrations (A, B, C, D) with A's append still in flight keep A;
//  N6. unnamed roots accumulate without bound (young ones within the
//      window, old ones past it);
//  N12. the root just registered is pruned when its head is below a named
//      root's (a head inversion after a restore);
//  N13. receipt tripwire (TestAppendReceiptRowWritersAreKnown) flags a file
//      that merely reads the receipts table beside an unrelated DELETE.
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

// maxUnnamedRows is the bound on unnamed rows the prune keeps: the root just
// registered, the newest above the named roots, and the young ones.
const maxUnnamedRows = 2 + snapshotRootMaxYoungUnnamed

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
				// N2: the newest attempt stays beside the named root, and the
				// unnamed rows stay bounded.
				kept := c.keptRoots()
				if !containsRoot(kept, last) || !containsRoot(kept, named) || len(kept) > tc.retention+maxUnnamedRows {
					t.Fatalf("kept %v after failure %d, want the newest attempt and the named root within the bound", short(kept), i)
				}
				requireBase(t, c, named, namedHead) // N3: never the unappended root
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

func containsRoot(roots []string, root string) bool {
	for _, r := range roots {
		if r == root {
			return true
		}
	}
	return false
}

// ageRoots moves every row's registration time back by d, as if the
// attempts had been made d ago.
func (c *drillComputer) ageRoots(d time.Duration) {
	c.t.Helper()
	if _, err := c.store.db.ExecContext(context.Background(), `UPDATE computer_snapshot_roots SET created_at=created_at - (? * interval '1 second') WHERE computer_id=?`, int64(d.Seconds()), c.id); err != nil {
		c.t.Fatal(err)
	}
}

// N6: young unnamed rows are bounded; old ones go at the next registration.
func TestUnnamedRootsDoNotAccumulate(t *testing.T) {
	c := newDrillComputer(t, "computer-accumulate")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	bound := maxUnnamedRows
	for i := 0; i < 30; i++ {
		c.appendMixed(1, fmt.Sprintf("n%d", i))
		c.registerRoot(fmt.Sprintf("none-%d", i), 3)
		if got := len(c.keptRoots()); got > bound {
			t.Fatalf("%d rows after %d unappended registrations with none named, bound %d", got, i+1, bound)
		}
	}
	c.appendMixed(1, "m")
	named, _ := c.registerRoot("named", 3)
	c.appendSnapshotRecorded(named)
	for i := 0; i < 30; i++ {
		c.appendMixed(1, fmt.Sprintf("m%d", i))
		c.registerRoot(fmt.Sprintf("more-%d", i), 3)
		if got := len(c.keptRoots()); got > 1+bound {
			t.Fatalf("%d rows with one named root after %d failures, bound %d", got, i+1, 1+bound)
		}
	}
	// Past the window the failed attempts are gone; the next registration
	// leaves the named root and itself.
	c.ageRoots(snapshotRootInFlightWindow + time.Minute)
	c.appendMixed(1, "late")
	last, _ := c.registerRoot("after-window", 3)
	requireKept(t, c, last, named)
}

// N5 (the pruned variant): A registers, B registers at a higher head, then A's
// snapshot_recorded lands. A's row must still be there, so the tape's root is
// protected from the collector and is the recovery base.
func TestAnAppendThatLandsAfterANewerRegistrationFindsItsRow(t *testing.T) {
	c := newDrillComputer(t, "computer-late-after")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	c.appendMixed(2, "pre")
	a, aHead := c.registerRoot("a", largeSnapshotChunks+1)
	c.appendMixed(1, "x")
	b, _ := c.registerRoot("b", largeSnapshotChunks+1)
	if !containsRoot(c.keptRoots(), a) {
		t.Fatalf("A was pruned by B's registration: kept %v", short(c.keptRoots()))
	}
	c.appendSnapshotRecorded(a) // lands after B registered
	requireBase(t, c, a, aHead)
	_ = b
	// Past the window an attempt that never landed is pruned.
	c.ageRoots(snapshotRootInFlightWindow + time.Minute)
	c.appendMixed(1, "y")
	d, _ := c.registerRoot("d", largeSnapshotChunks+1)
	requireKept(t, c, d, a)
}

// N12: a root registered at a head below the newest named root's is kept,
// whatever its head.
func TestTheRootJustRegisteredIsNeverPruned(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()
	const computer = "computer-inversion"
	for i, head := range []int64{50, 60} {
		root := gcDigest(fmt.Sprintf("named/%d", i))
		nameRootRow(t, store, computer, root, head)
		if err := store.recordSnapshotRoot(ctx, computer, root, head, []string{gcDigest("c/" + root)}); err != nil {
			t.Fatal(err)
		}
	}
	low := gcDigest("low")
	if err := store.recordSnapshotRoot(ctx, computer, low, 10, []string{gcDigest("c/low")}); err != nil {
		t.Fatal(err)
	}
	live, err := store.LiveSnapshotRoots(ctx, computer)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range live {
		found = found || r.Root == low
	}
	if !found {
		t.Fatalf("the root just registered at head 10 was pruned in its own transaction: %d rows", len(live))
	}
}

// A named root below the head of an older unappended one is still named; an
// unappended root BELOW the newest named root is dropped, not kept.
func TestAnOldUnappendedRootBelowTheNewestNamedRootIsDropped(t *testing.T) {
	c := newDrillComputer(t, "computer-below")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	c.appendMixed(1, "a")
	old, _ := c.registerRoot("old-unappended", 3)
	c.appendMixed(1, "b")
	named, _ := c.registerRoot("named", 3)
	c.appendSnapshotRecorded(named)
	// Past the in-flight window, the next registration prunes: old sits below
	// named and is gone.
	c.ageRoots(snapshotRootInFlightWindow + time.Minute)
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
	// A young unnamed root below the named heads (an attempt whose append
	// lands after a newer registration), then the newest unnamed one.
	digests["y"], paths["y"] = register("y", 15, false)
	digests["u"], paths["u"] = register("u", 50, false)
	live, err := f.store.LiveSnapshotRoots(f.ctx, computer)
	if err != nil || len(live) != snapshotRootRetention+2 {
		t.Fatalf("kept %d rows, %v; want four named and two unnamed", len(live), err)
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
	nameRootAt(t, store, computer, root, head+1)
}

// nameRootAt writes the naming receipt at an explicit sequence.
func nameRootAt(t *testing.T, store *Store, computer, root string, seq int64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := store.db.ExecContext(context.Background(), `INSERT INTO computer_event_append_receipts (computer_id,idempotency_key,request_commitment,sequence,event_kind,event_digest,event_pin_receipt_digest,event_head_receipt_id,event_head_receipt_json,created_at) VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT (computer_id, idempotency_key) DO NOTHING`,
		computer, "snapshot:"+root, platformTestDigest('a'), seq, string(computerevent.EventSnapshotRecorded), gcDigest("event/"+root), platformTestDigest('b'), "receipt-"+root[:8], "{}", now); err != nil {
		t.Fatal(err)
	}
}

// N1 at full retention and at an unchanged head: every named slot filled, then
// failed registrations at one head (the paper's 2026-11-12 condition: roots at
// head 10,914,341 registered one after another with the store wedged).
func TestFailuresAtAnUnchangedHeadKeepEveryNamedRootAtFullRetention(t *testing.T) {
	for _, tc := range []struct {
		name      string
		chunks    int
		retention int
	}{{"small", 3, snapshotRootRetention}, {"large", largeSnapshotChunks + 1, largeSnapshotRetention}} {
		t.Run(tc.name, func(t *testing.T) {
			c := newDrillComputer(t, "computer-full-"+tc.name)
			c.appendEvent(computerevent.EventGenesisImported, "", nil)
			var named []string
			for i := 0; i < tc.retention; i++ {
				c.appendMixed(1, fmt.Sprintf("k%d", i))
				root, _ := c.registerRoot(fmt.Sprintf("named-%d", i), tc.chunks)
				c.appendSnapshotRecorded(root)
				named = append(named, root)
			}
			newest := named[len(named)-1]
			c.appendMixed(1, "wedged") // the head stops here: the store is wedged
			wedgedHead := int64(c.head().Sequence)
			for i := 0; i < 8; i++ {
				c.registerRoot(fmt.Sprintf("wedged-%d", i), tc.chunks)
				kept := c.keptRoots()
				for _, root := range named {
					if !containsRoot(kept, root) {
						t.Fatalf("after %d failures at head %d the named root %.8s is gone: %v", i+1, wedgedHead, root, short(kept))
					}
				}
				if len(kept) > tc.retention+maxUnnamedRows {
					t.Fatalf("%d rows kept, bound %d", len(kept), tc.retention+maxUnnamedRows)
				}
				seq, base, err := c.store.RecoveryBase(context.Background(), c.id)
				if err != nil || base != newest || seq >= wedgedHead {
					t.Fatalf("recovery base = %d %.8s %v, want the newest named root %.8s below head %d", seq, base, err, newest, wedgedHead)
				}
			}
		})
	}
}

// S1: A, B, C, D registered while A's append is in flight (retention 4, none
// named): the kept unnamed set is at least as protective as the newest four by
// head, so A is still a row when its append lands, and is the recovery base.
func TestFourRegistrationsWhileAnAppendIsInFlightKeepTheFirst(t *testing.T) {
	c := newDrillComputer(t, "computer-four-in-flight")
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	roots := map[string]string{}
	var aHead int64
	for i, name := range []string{"a", "b", "c", "d"} {
		c.appendMixed(1, name)
		root, head := c.registerRoot(name, 3)
		roots[name] = root
		if i == 0 {
			aHead = head
		}
	}
	kept := c.keptRoots()
	for _, name := range []string{"a", "b", "c", "d"} {
		if !containsRoot(kept, roots[name]) {
			t.Fatalf("root %s was pruned with four unappended registrations: kept %v", name, short(kept))
		}
	}
	c.appendSnapshotRecorded(roots["a"])
	requireBase(t, c, roots["a"], aHead)
}

// S3: the named roots are the newest N by the sequence of the event that names
// them, not by head. After a head inversion (a point-in-time restore: the
// next snapshots are taken at heads below the old ones) the roots the tape
// named last are the ones kept and advertised, because boot restore tries the
// newest naming event first.
func TestNamedRootsAreOrderedByTheirNamingEventUnderAHeadInversion(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()
	const computer = "computer-head-inversion"
	register := func(name string, head int64) string {
		root := gcDigest("inv/" + name)
		if err := store.recordSnapshotRoot(ctx, computer, root, head, []string{gcDigest("c/" + name)}); err != nil {
			t.Fatal(err)
		}
		return root
	}
	var old []string
	for i := int64(0); i < 4; i++ {
		root := register(fmt.Sprintf("old%d", i), 100+i)
		nameRootAt(t, store, computer, root, 101+i)
		old = append(old, root)
	}
	var fresh []string
	for i := int64(0); i < 4; i++ {
		root := register(fmt.Sprintf("fresh%d", i), 50+i) // registered, then named
		nameRootAt(t, store, computer, root, 201+i)       // the tape names it after the old ones
		fresh = append(fresh, root)
	}
	if _, base, err := store.RecoveryBase(ctx, computer); err != nil || base != fresh[3] {
		t.Fatalf("recovery base = %.8s %v, want the root named last (head 53)", base, err)
	}
	next := register("fresh4", 54) // unappended
	live, err := store.LiveSnapshotRoots(ctx, computer)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range live {
		got[r.Root] = true
	}
	for _, root := range append(fresh, next) {
		if !got[root] {
			t.Fatalf("root %.8s was dropped; kept %d rows", root, len(live))
		}
	}
	for _, root := range old {
		if got[root] {
			t.Fatalf("an old root %.8s named before the four fresh ones survived (four newest named kept)", root)
		}
	}
}

// A root that is already named and beyond the retention is still kept when the
// host registers it again (a retried PUT): the root just registered is never
// pruned in its own transaction.
func TestRegisteringANamedRootAgainKeepsIt(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()
	const computer = "computer-reregister"
	var roots []string
	for i := int64(1); i <= 5; i++ {
		root := gcDigest(fmt.Sprintf("rr/%d", i))
		nameRootAt(t, store, computer, root, i*10+1)
		if err := store.recordSnapshotRoot(ctx, computer, root, i*10, []string{gcDigest("c/" + root)}); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, root)
	}
	if err := store.recordSnapshotRoot(ctx, computer, roots[0], 10, []string{gcDigest("c/" + roots[0])}); err != nil {
		t.Fatal(err)
	}
	live, _ := store.LiveSnapshotRoots(ctx, computer)
	found := false
	for _, r := range live {
		found = found || r.Root == roots[0]
	}
	if !found {
		t.Fatal("the root registered again was pruned in its own transaction")
	}
}
