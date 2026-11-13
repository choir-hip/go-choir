package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
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
//
// The row is built by reflection over tapeChainRow: every string field and
// every element of every []string field carries a distinct, non-uniform
// 64-hex value (sha256 of the field's path), and each []string has three
// elements. A new string field that tapeRowPrefixes starts reading, a changed
// slice offset, a changed pins index or a changed digestPrefix width changes
// the list below.
func TestTapeRefExtractionGolden(t *testing.T) {
	const goldenFormat = 1
	if tapeRefFormat != goldenFormat {
		t.Fatalf("tapeRefFormat = %d; the golden below is for %d. Update the golden only after deciding the old chunks are stale (the bump discards them)", tapeRefFormat, goldenFormat)
	}
	hexOf := func(path string) string {
		sum := sha256.Sum256([]byte(path))
		return hex.EncodeToString(sum[:])
	}
	var row tapeChainRow
	rv := reflect.ValueOf(&row).Elem()
	for i := 0; i < rv.NumField(); i++ {
		name := rv.Type().Field(i).Name
		switch rv.Field(i).Kind() {
		case reflect.String:
			rv.Field(i).SetString(hexOf(name))
		case reflect.Slice:
			if rv.Field(i).Type().Elem().Kind() == reflect.String {
				vals := make([]string, 3)
				for j := range vals {
					vals[j] = hexOf(fmt.Sprintf("%s/%d", name, j))
				}
				rv.Field(i).Set(reflect.ValueOf(vals))
			}
		}
	}
	got := fmt.Sprint(tapeRowPrefixes(row))
	want := "[12491266664376620606 11499817458892743072 8000223249922609663 17211481934802546154 6243759859604505432 12240792780506004608 1460053042699466875 11987051296110327409]"
	if got != want {
		t.Fatalf("extraction changed without a tapeRefFormat bump:\n got %s\nwant %s", got, want)
	}
	// Junk is skipped: short, upper case, and 63 hex.
	junk := tapeChainRow{EventDigest: "short", EventPin: strings.ToUpper(hexOf("x")), PayloadPins: []string{hexOf("y")[:63]}}
	if p := tapeRowPrefixes(junk); len(p) != 0 {
		t.Fatalf("invalid digests extracted: %v", p)
	}

	// The regexp: a 128-hex run is two references, not one (hex64RE{64,}
	// would make it one), upper case and 63 hex are none.
	one, two := hexOf("one"), hexOf("two")
	text := "x" + one + "Y " + strings.ToUpper(hexOf("up")) + " " + hexOf("short")[:63] + " " + one + two
	wantRefs := []string{one, one, two}
	if refs := tapeHexRefs(text); !reflect.DeepEqual(refs, wantRefs) {
		t.Fatalf("tapeHexRefs changed without a tapeRefFormat bump: %v", refs)
	}
	if v, ok := digestPrefix(one); !ok || v != mustPrefix(one) {
		t.Fatalf("digestPrefix changed without a tapeRefFormat bump: %x %v", v, ok)
	}

	// The real walker, whose pins and event-file references come from
	// hex64RE directly (not tapeHexRefs): one fixed chain, with a 128-hex run
	// written into an event file.
	f := newTapeFixture(t)
	c := f.chain("golden-1", 2, tapeAged)
	var rows []tapeChainRow
	if err := f.svc.walkTapeChain(f.ctx, "golden-1", 0, 2, true, func(r tapeChainRow) error {
		rows = append(rows, r)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for i, r := range rows {
		s := c.seqs[i]
		if !reflect.DeepEqual(r.PayloadPins, []string{s.payloadPin}) {
			t.Fatalf("seq %d pins = %v, want [%s]; bump tapeRefFormat if the pin source changed on purpose", i+1, r.PayloadPins, s.payloadPin)
		}
		if !reflect.DeepEqual(r.PayloadRefs, []string{s.payload, s.payload}) {
			t.Fatalf("seq %d event refs = %v, want the payload twice; bump tapeRefFormat if the extraction changed on purpose", i+1, r.PayloadRefs)
		}
	}
	if got := hex64RE.FindAllString(`{"refs":"`+one+two+`"}`, -1); !reflect.DeepEqual(got, []string{one, two}) {
		t.Fatalf("hex64RE on a 128-hex run = %v; bump tapeRefFormat if widened on purpose", got)
	}
	var set digestPrefixSet
	if _, err := f.svc.walkTapeRefs(f.ctx, "golden-1", 2, true, true, &set, nil); err != nil {
		t.Fatal(err)
	}
	set.seal()
	if len(set.v) != 8 {
		t.Fatalf("walker extracted %d unique prefixes from the fixed chain, want 8; bump tapeRefFormat if on purpose", len(set.v))
	}
}

func mustPrefix(d string) uint64 {
	v, _ := strconv.ParseUint(d[:16], 16, 64)
	return v
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
// guarded by listing the writers. This is a TRIPWIRE, not a proof: it finds
// files that name the table (schema-qualified or not) and also contain a
// mutating SQL keyword, in non-test Go, nix/ and scripts/, and fails for any
// file not on the allowlist. SQL built from pieces (the purge builds
// "DELETE FROM "+table), a file that mutates through another name, or a
// keyword in a different file can evade it. A new writer that is not lossless
// must delete the computer's tape_sweep_ref_chunks, and be added here with
// its reason and to receipt_compaction.go's C-list.
func TestAppendReceiptRowWritersAreKnown(t *testing.T) {
	table := regexp.MustCompile(`(?i)\bcomputer_event_append_receipts\b`)
	mutates := regexp.MustCompile(`(?i)\b(UPDATE|DELETE|TRUNCATE|MERGE)\b|DO\s+UPDATE`)
	allowed := map[string]string{
		"internal/platform/receipt_compaction.go": "the lossless rewrite (C2, C9)",
		"internal/platform/tape_purge.go":         "deletes the chunks with the rows, same transaction",
		// Readers and inserters whose text merely contains a keyword (comments,
		// ON CONFLICT clauses, "updated_at"-style words, shell tools). Each was
		// read when this list was made; none updates or deletes receipt rows.
		"internal/platform/computer_events.go":  "append insert",
		"internal/platform/clean_rebase.go":     "reader",
		"internal/platform/checkpoint_list.go":  "reader",
		"internal/platform/event_replay.go":     "reader",
		"internal/platform/checkpoints.go":      "reader",
		"internal/platform/verifier_key_pin.go": "reader",
		"internal/platform/tape_sweep.go":       "reader",
		"internal/platform/tape_sweep_refs.go":  "comments",
		"internal/bundle/import.go":             "inserts after the head, overlap refused",
		"internal/bundle/store.go":              "reader",
		"internal/bundle/follow.go":             "reader",
		"internal/bundle/bundle.go":             "reader",
		"nix/node-b.nix":                        "backup and growth-meter queries",
		"scripts/off-host-restore-drill.sh":     "restore drill reads",
		"scripts/test-node-b-growth-meter.sh":   "test fixture",
		"scripts/node-b-growth-meter":           "reader",
		"scripts/chaos/s2-corpusd-crash.sh":     "chaos drill reads",
	}
	root := filepath.Join("..", "..")
	var hits []string
	for _, dir := range []string{"internal", "cmd", "nix", "scripts"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			name := d.Name()
			if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".md") {
				return nil
			}
			if dir == "internal" || dir == "cmd" {
				if !strings.HasSuffix(name, ".go") {
					return nil
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if table.Match(raw) && mutates.Match(raw) {
				rel, _ := filepath.Rel(root, path)
				hits = append(hits, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) < 5 {
		t.Fatalf("the scan found only %v; the repo root is wrong or the scan is vacuous", hits)
	}
	for _, rel := range hits {
		if _, ok := allowed[rel]; !ok {
			t.Errorf("%s names computer_event_append_receipts and a mutating keyword; if it rewrites or deletes receipt rows it must drop tape_sweep_ref_chunks for the computer (or be lossless) and be allowlisted here with a reason", rel)
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
	baseline := itemKeys(f.manifestItems(first)) // from scratch: nothing was stored yet
	second := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	if !equalKeys(baseline, itemKeys(f.manifestItems(second))) {
		t.Fatal("the manifest after a prefix resume differs from the from-scratch one")
	}
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
