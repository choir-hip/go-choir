package platform

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Review round 1 (lane/sweep-survives-restart). Failure list, written before
// the code:
//
//	V1 a chunk written under another extraction format restarts that
//	   computer's walk and the manifest equals a from-scratch run;
//	V2 a golden of the references extracted from a fixed row and a fixed
//	   chain fails when the extraction changes without a tapeRefFormat bump;
//	V3 receipt compaction (the only in-place receipt rewrite) leaves stored
//	   chunks valid and the manifest equal;
//	V4 no non-test code outside receipt_compaction.go updates or deletes
//	   append-receipt rows (such a path must also drop the chunks);
//	M1 loading a 33M-prefix watermark holds one chunk beyond the set (peak
//	   heap measured; run with CHOIR_TAPE_MEM_PROBE=1);
//	P1 one unreadable event keeps the verified prefix before it;
//	P2 compaction never merges a no-reads chunk into a reads chunk;
//	P3 compaction across more than one group;
//	D1 a doomed (destroyed) computer's chunks are reused and a payload it
//	   shares with a resumed live computer is kept;
//	D2 the derived table is absent from the manifest's rows lines, with
//	   chunks present at apply.

// V1
func TestTapeSweepChunkFromAnotherFormatRestarts(t *testing.T) {
	f, _, _ := resumeFixture(t)
	want := itemKeys(f.manifestItems(f.resumeDry()))
	if _, err := f.store.db.Exec(`UPDATE tape_sweep_ref_chunks SET walker_version=walker_version-1 WHERE computer_id='dead-1'`); err != nil {
		t.Fatal(err)
	}
	got := f.resumeDry()
	if got.ChainEventsResumed != 11 || got.ChainEventsWalked != 9 {
		t.Fatalf("resumed %d walked %d, want 11 and 9", got.ChainEventsResumed, got.ChainEventsWalked)
	}
	if !equalKeys(want, itemKeys(f.manifestItems(got))) {
		t.Fatal("manifest differs after a format mismatch")
	}
}

// V2: bump tapeRefFormat (and this golden) whenever any of these change.
func TestTapeRefExtractionGolden(t *testing.T) {
	const goldenFormat = 1
	if tapeRefFormat != goldenFormat {
		t.Fatalf("tapeRefFormat = %d; the golden below is for %d. Update the golden only after deciding the old chunks are stale (the bump discards them)", tapeRefFormat, goldenFormat)
	}
	h := func(c string) string { return strings.Repeat(c, 64) }
	row := tapeChainRow{EventDigest: h("a"), EventPin: h("b"), PayloadPins: []string{h("c")}, PayloadRefs: []string{h("d"), "short", strings.ToUpper(h("e"))}}
	got := fmt.Sprint(tapeRowPrefixes(row))
	want := "[12297829382473034410 13527612320720337851 14757395258967641292 15987178197214944733]"
	if got != want {
		t.Fatalf("extraction changed without a tapeRefFormat bump:\n got %s\nwant %s", got, want)
	}
	text := "x" + h("a") + "Y " + strings.ToUpper(h("b")) + " " + h("c")[:63]
	if refs := tapeHexRefs(text); len(refs) != 1 || refs[0] != h("a") {
		t.Fatalf("tapeHexRefs changed without a tapeRefFormat bump: %v", refs)
	}
	if v, ok := digestPrefix(h("f")); !ok || v != 0xffffffffffffffff {
		t.Fatalf("digestPrefix changed without a tapeRefFormat bump: %x %v", v, ok)
	}
	// A fixed chain through the real walker (pins from receipt bodies, refs
	// from event files).
	f := newTapeFixture(t)
	c := f.chain("golden-1", 2, tapeAged)
	var set digestPrefixSet
	if _, err := f.svc.walkTapeRefs(f.ctx, "golden-1", 2, true, true, &set, nil); err != nil {
		t.Fatal(err)
	}
	set.seal()
	for _, s := range c.seqs {
		for _, d := range []string{s.event, s.eventPin, s.payload, s.payloadPin} {
			if !set.has(d) {
				t.Fatalf("walker dropped reference %s; bump tapeRefFormat if extraction changed on purpose", d)
			}
		}
	}
	if len(set.v) != 8 {
		t.Fatalf("walker extracted %d unique prefixes from the fixed chain, want 8; bump tapeRefFormat if on purpose", len(set.v))
	}
}

// V3
func TestTapeSweepWatermarkSurvivesReceiptCompaction(t *testing.T) {
	f, _, _ := resumeFixture(t)
	want := itemKeys(f.manifestItems(f.resumeDry()))
	f.store.SetReceiptCompaction(ReceiptCompactionPolicy{Computers: map[string]string{"dead-1": ReceiptCompactActive, "live-1": ReceiptCompactActive}})
	if passes := runCompaction(t, noSleepCompactor(f.store)); passes[0].Rewritten == 0 && passes[1].Rewritten == 0 {
		t.Fatalf("compaction rewrote nothing: %+v", passes)
	}
	got := f.resumeDry()
	if got.ChainEventsResumed != 20 || got.ChainEventsWalked != 0 {
		t.Fatalf("after compaction resumed %d walked %d, want 20 and 0", got.ChainEventsResumed, got.ChainEventsWalked)
	}
	if !equalKeys(want, itemKeys(f.manifestItems(got))) {
		t.Fatal("compaction changed the resumed manifest")
	}
}

// V4: the invariant "receipt rows are never changed except losslessly" is
// enforced by listing the writers. A new one must drop the chunks (or be
// lossless), and be added here and to receipt_compaction.go's C-list.
func TestAppendReceiptRowWritersAreKnown(t *testing.T) {
	re := regexp.MustCompile(`(?i)(UPDATE|DELETE\s+FROM)\s+computer_event_append_receipts`)
	allowed := map[string]bool{
		"receipt_compaction.go": true, // lossless; chunks stay valid (V3)
		"tape_purge.go":         true, // deletes the chunks in the same transaction
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if re.Match(raw) && !allowed[name] {
			t.Errorf("%s rewrites or deletes append-receipt rows; it must drop tape_sweep_ref_chunks for the computer (or be lossless) and be listed in this test", name)
		}
	}
}

// P1
func TestTapeSweepOneUnreadableEventKeepsTheVerifiedPrefix(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.svc.tapeSweep.refChunkEvents = 2
	live := f.chain("live-1", 10, tapeAged)
	f.put(tapeNSPayload, []byte("orphan payload"), tapeAged)
	if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, live.seqs[4].event))); err != nil { // chunk (4,6]
		t.Fatal(err)
	}
	first := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	if first.Orphans[tapeNSPayload].Refused == "" {
		t.Fatal("first run did not refuse")
	}
	second := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	if second.Orphans[tapeNSPayload].Refused == "" {
		t.Fatal("the refusal was lost on the second run")
	}
	if second.ChainEventsResumed != 4 || second.ChainEventsWalked != 6 {
		t.Fatalf("second run resumed %d walked %d, want the 4 rows before the unreadable chunk and 6 after", second.ChainEventsResumed, second.ChainEventsWalked)
	}
}

// P2, P3
func TestTapeSweepCompactionGroups(t *testing.T) {
	f, _, _ := resumeFixture(t)
	f.svc.tapeSweep.refChunkEvents = 1
	f.svc.tapeSweep.refCompactAfter = 3
	f.svc.tapeSweep.refCompactMaxEvents = 5
	want := itemKeys(f.manifestItems(f.resumeDry()))
	if n := f.chunkCount("live-1"); n != 3 { // 11 rows: 5 + 5 + 1
		t.Fatalf("live-1 chunks = %d, want 3 groups", n)
	}
	if n := f.chunkCount("dead-1"); n != 2 { // 9 rows: 5 + 4
		t.Fatalf("dead-1 chunks = %d, want 2 groups", n)
	}
	got := f.resumeDry()
	if got.ChainEventsResumed != 20 || !equalKeys(want, itemKeys(f.manifestItems(got))) {
		t.Fatalf("resume after group compaction: resumed %d", got.ChainEventsResumed)
	}
}

func TestTapeSweepCompactionKeepsNoReadsChunksApart(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.svc.tapeSweep.refChunkEvents = 1
	f.svc.tapeSweep.refCompactAfter = 2
	live := f.chain("live-1", 6, tapeAged)
	if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, live.seqs[2].event))); err != nil {
		t.Fatal(err)
	}
	f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	var noReads int64
	if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM tape_sweep_ref_chunks WHERE computer_id='live-1' AND NOT read_events`).Scan(&noReads); err != nil {
		t.Fatal(err)
	}
	var cut uint64
	if err := f.store.db.QueryRow(`SELECT MIN(from_sequence) FROM tape_sweep_ref_chunks WHERE computer_id='live-1' AND NOT read_events`).Scan(&cut); err != nil {
		t.Fatal(err)
	}
	if noReads != 1 || cut != 2 {
		t.Fatalf("no-reads chunks = %d starting at %d, want exactly the one (2,3]", noReads, cut)
	}
}

// D1
func TestTapeSweepResumedDoomedComputerKeepsSharedPayload(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.svc.tapeSweep.refChunkEvents = 2
	live := f.chain("live-1", 4, tapeAged)
	dead := f.chain("dead-1", 4, tapeAged)
	shared := []byte("shared between a destroyed and a live computer")
	f.appendSeq(live, shared, tapeAged)
	deadShared := f.appendSeq(dead, shared, tapeAged)
	first := f.resumeDry()
	if first.DestroyedListed != 1 {
		t.Fatalf("dead-1 was not listed: %+v", first.DestroyedProofs)
	}
	second := f.resumeDry()
	if second.ChainEventsResumed != 10 || second.ChainEventsWalked != 0 {
		t.Fatalf("resumed %d walked %d, want 10 and 0", second.ChainEventsResumed, second.ChainEventsWalked)
	}
	if second.DestroyedListed != 1 || second.DestroyedProofs[0].SharedPayloadsKept != 1 {
		t.Fatalf("resumed run lost the shared-payload rule: %+v", second.DestroyedProofs)
	}
	if listedDigests(f.manifestItems(second), TapeSweepDestroyed)[deadShared.payload] {
		t.Fatal("the shared payload is listed for deletion")
	}
}

// D2
func TestTapeSweepDerivedTableIsNotAManifestRowsLine(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 3)
	f.svc.tapeSweep.refChunkEvents = 2
	dry := f.dryRun(destroyedReq(true))
	if f.chunkCount("dead-1") == 0 {
		t.Fatal("the dry-run left no chunks; the test would prove nothing")
	}
	for _, line := range f.manifestLines(dry) {
		if strings.Contains(string(line), "tape_sweep_ref_chunks") {
			t.Fatalf("manifest names the derived table: %s", line)
		}
	}
	if _, err := f.apply(dry, destroyedReq(true)); err != nil {
		t.Fatal(err)
	}
	if f.chunkCount("dead-1") != 0 {
		t.Fatal("apply left the chunks")
	}
}

// M1: peak heap while loading a watermark of the paper's size. Skipped
// unless CHOIR_TAPE_MEM_PROBE=1 (it writes about 264 MB to the test DB).
func TestTapeSweepWatermarkLoadPeakHeap(t *testing.T) {
	if os.Getenv("CHOIR_TAPE_MEM_PROBE") != "1" {
		t.Skip("set CHOIR_TAPE_MEM_PROBE=1 to measure")
	}
	f := newTapeFixture(t)
	f.live("live-1")
	// The synthetic chunks hold 1M prefixes per event, 300x the real
	// density; compaction (capped in events) would build one 264 MB blob, so
	// it is off here. Its memory is measured by the group size instead.
	f.svc.tapeSweep.refCompactAfter = 1 << 20
	const chunks, perChunk = 33, 1_000_000
	c := f.chain("live-1", chunks, tapeAged)
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < chunks; i++ {
		refs := make([]uint64, perChunk)
		for j := range refs {
			refs[j] = rng.Uint64()
		}
		sort.Slice(refs, func(a, b int) bool { return refs[a] < refs[b] })
		ch := tapeRefChunk{From: uint64(i), To: uint64(i + 1), ReadEvents: true, Anchor: c.seqs[i].event, Refs: refs}
		if err := f.store.saveTapeRefChunk(f.ctx, "live-1", ch); err != nil {
			t.Fatal(err)
		}
	}
	// Warm run: the driver's connection buffers are steady state by now and
	// belong to the baseline, not to the walk.
	var warm digestPrefixSet
	if _, err := f.svc.walkTapeRefs(f.ctx, "live-1", chunks, true, true, &warm, nil); err != nil {
		t.Fatal(err)
	}
	warm.v = nil
	if pct := os.Getenv("CHOIR_TAPE_MEM_GOGC"); pct != "" {
		var n int
		fmt.Sscan(pct, &n)
		debug.SetGCPercent(n)
	}
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak atomic.Uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > peak.Load() {
				peak.Store(m.HeapAlloc)
			}
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()
	var set digestPrefixSet
	res, err := f.svc.walkTapeRefs(f.ctx, "live-1", chunks, true, true, &set, nil)
	close(stop)
	<-done
	if err != nil || res.Resumed != chunks {
		t.Fatalf("resumed %d err %v", res.Resumed, err)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	t.Logf("MEMPROBE live_after_gc_over_baseline=%d", int64(after.HeapAlloc)-int64(base.HeapAlloc))
	t.Logf("MEMPROBE cap=%d len=%d", cap(set.v), len(set.v))
	set.v = nil
	runtime.GC()
	var gone runtime.MemStats
	runtime.ReadMemStats(&gone)
	t.Logf("MEMPROBE after_dropping_set_over_baseline=%d", int64(gone.HeapAlloc)-int64(base.HeapAlloc))
	final := uint64(chunks * perChunk * 8)
	over := int64(peak.Load()) - int64(base.HeapAlloc)
	t.Logf("MEMPROBE set_bytes=%d peak_over_baseline=%d ratio=%.2f", final, over, float64(over)/float64(final))
	// Live heap is the set plus one chunk; the rest of the peak is garbage
	// the collector has not yet reclaimed (GOGC=100 lets the heap reach twice
	// the live size). More than 2.5x means a second copy is being held.
	if float64(over) > 2.5*float64(final) {
		t.Fatalf("peak %d is more than 2.5x the set (%d): a second copy is held", over, final)
	}
}
