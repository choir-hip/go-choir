package taperepack

// Repack tool v2 (FP10): failure modes P1 to P19 of docs/problems/
// platform-artifact-small-file-growth-2026-10-19.md, "Repack tool v2: plan
// and failure modes", written before the engine. They fail on the base, where
// the package has the manifest tool (Plan, Copy, Census) and none of Run,
// Options, Report, Verdict.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// world builds the standard legacy tree: computer A with three projection
// batch pins (event + payload + two receipts each), computer B with one, and
// the files the tool must never count as covered.
type world struct {
	f *fixture
	// eligibleA is the number of legacy files computer A needs in segments:
	// 3 events, 3 payloads, 6 receipts.
	eligibleA                  int
	events, payloads, receipts map[string][]string // computer -> digests
}

func newWorld(t *testing.T) *world {
	f := newFixture(t)
	w := &world{f: f, eligibleA: 12, events: map[string][]string{}, payloads: map[string][]string{}, receipts: map[string][]string{}}
	pin := func(c, n string) {
		e, er := f.batchEvent(c, n)
		p, pr := f.batchPayload(c, n)
		w.events[c] = append(w.events[c], e)
		w.payloads[c] = append(w.payloads[c], p)
		w.receipts[c] = append(w.receipts[c], er, pr)
	}
	for _, n := range []string{"1", "2", "3"} {
		pin(compA, n)
	}
	pin(compB, "b1")
	return w
}

func (w *world) opts(t *testing.T) Options {
	return Options{Root: w.f.root, WorkDir: t.TempDir()}
}

func mustRun(t *testing.T, o Options) *Report {
	t.Helper()
	rep, err := Run(o)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return rep
}

func (w *world) copyOpts(t *testing.T, computers ...string) Options {
	o := w.opts(t)
	o.Copy, o.Apply, o.Computers = true, true, computers
	return o
}

func verdicts(t *testing.T, o Options) (*Report, map[string]Verdict) {
	t.Helper()
	got := map[string]Verdict{}
	o.OnVerdict = func(v Verdict) error {
		got[v.NS().String()+"/"+v.Digest()] = v
		return nil
	}
	return mustRun(t, o), got
}

func segmentTree(t *testing.T, root string) map[string]string {
	return subtree(tree(t, root), tapestore.SegDirName)
}

// P3, P14, P15, P19: nothing is covered before the copy; after it every
// eligible file is covered, byte for byte, readable from the segment alone.
func TestP3P14CopyCoversEveryEligibleFileAndOnlyThen(t *testing.T) {
	w := newWorld(t)
	rep := mustRun(t, w.opts(t))
	if rep.Global["event"].Files != 4 || rep.Global["payload"].Files != 4 || rep.Global["receipt"].Files != 8 {
		t.Fatalf("listing: %+v", rep.Global)
	}
	if c := rep.Global["event"].Covered + rep.Global["payload"].Covered + rep.Global["receipt"].Covered; c != 0 {
		t.Fatalf("P3: %d files covered before any copy", c)
	}
	if rep.Global["event"].Uncovered != 4 || rep.Global["receipt"].Uncovered != 8 {
		t.Fatalf("P3: %+v", rep.Global)
	}
	if rep.OK() {
		t.Fatal("P3: a census with uncovered files is ok")
	}

	rep = mustRun(t, w.copyOpts(t, compA))
	if rep.Copy.CopiedRecords != int64(w.eligibleA) {
		t.Fatalf("copied %d records, want %d: %+v", rep.Copy.CopiedRecords, w.eligibleA, rep.Copy)
	}
	rep, vs := verdicts(t, w.opts(t))
	// Computer B was not named in the copy: its 4 files are uncovered.
	if g := rep.Global; g["event"].Covered != 3 || g["payload"].Covered != 3 || g["receipt"].Covered != 6 ||
		g["event"].Uncovered != 1 || g["payload"].Uncovered != 1 || g["receipt"].Uncovered != 2 {
		t.Fatalf("after copy: %+v", rep.Global)
	}
	for _, c := range []string{compA} {
		for _, ns := range []string{"event", "payload", "receipt"} {
			tl := rep.Computers[c][ns]
			if tl.Files != tl.Covered || tl.Files == 0 {
				t.Fatalf("P15: computer %s %s: %+v", c, ns, tl)
			}
		}
	}
	for _, d := range w.events[compA] {
		if v := vs["event/"+d]; !v.Deletable() {
			t.Fatalf("event %s of A not deletable: %s", d, v.Class())
		}
		got, found, err := tapestore.NewReader(w.f.root).ReadSegment(compA, tapestore.NSEvent, d)
		legacy, _ := os.ReadFile(filepath.Join(w.f.root, "sha256", "computer-event", d))
		if err != nil || !found || !bytes.Equal(got, legacy) {
			t.Fatalf("P14: segment copy of %s: found=%v err=%v", d, found, err)
		}
	}
	for _, d := range w.events[compB] {
		if v := vs["event/"+d]; v.Deletable() || v.Class() != "uncovered" {
			t.Fatalf("P3: B's event %s: %s", d, v.Class())
		}
	}
	if err := tapestore.NewReader(w.f.root).Scan(compA, func(tapestore.Record) error { return nil }); err != nil {
		t.Fatalf("P14: Scan: %v", err)
	}
	// P19: the report is stored under its own digest.
	if rep.Receipt == "" {
		t.Fatal("P19: no census receipt digest")
	}
}

// P15: the report's tallies equal the files on disk, counted here.
func TestP15TalliesEqualTheFilesOnDisk(t *testing.T) {
	w := newWorld(t)
	w.f.write("sha256/computer-event/not-a-digest", []byte("x"))
	if err := os.Mkdir(filepath.Join(w.f.root, "sha256", "computer-event", strings.Repeat("d", 64)), 0o750); err != nil {
		t.Fatal(err)
	}
	rep := mustRun(t, w.opts(t))
	for ns, dir := range map[string]string{"event": "computer-event", "payload": "computer-event-payload", "receipt": "pin-receipts"} {
		entries, _ := os.ReadDir(filepath.Join(w.f.root, "sha256", dir))
		files := 0
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".json")
			if e.Type().IsRegular() && tapestore.ValidDigest(name) {
				files++
			}
		}
		g := rep.Global[ns]
		sum := g.Covered + g.Uncovered + g.Corrupt + g.Blocked + g.Ineligible + g.Unreceipted + g.LegacyBad + g.TooLarge + g.Vanished + g.OutOfScope
		if g.Files != int64(files) || sum != g.Files || rep.Listing[ns] != int64(files) {
			t.Fatalf("%s: files on disk %d, tally %+v, listing %d", ns, files, g, rep.Listing[ns])
		}
	}
	if rep.Listing["other_files"] != 2 {
		t.Fatalf("other_files = %d, want 2", rep.Listing["other_files"])
	}
}

// P1 (r4): a legacy file whose segment copy fails verification is never
// deletable, however it fails.
func TestP1ACorruptSegmentCopyIsNeverDeletable(t *testing.T) {
	w := newWorld(t)
	mustRun(t, w.copyOpts(t, compA))
	// Damage one record body in the sealed segment; the index stays intact.
	seg := filepath.Join(w.f.root, tapestore.SegDirName, compA, tapestore.SegmentFileName(1))
	raw, err := os.ReadFile(seg)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-3] ^= 0xff
	if err := os.WriteFile(seg, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	rep, vs := verdicts(t, w.opts(t))
	if rep.OK() {
		t.Fatal("P1: ok census with a corrupt copy")
	}
	corrupt, deletable := 0, 0
	for _, v := range vs {
		switch {
		case v.Class() == "corrupt":
			corrupt++
			if v.Deletable() {
				t.Fatalf("P1: corrupt verdict is deletable: %s", v.Digest())
			}
		case v.Deletable():
			deletable++
		}
	}
	g := rep.Global
	if corrupt != 1 || g["event"].Corrupt+g["payload"].Corrupt+g["receipt"].Corrupt != 1 {
		t.Fatalf("P1: corrupt verdicts = %d, tally %+v", corrupt, g)
	}
	if deletable != w.eligibleA-1 {
		t.Fatalf("P1: %d deletable, want %d", deletable, w.eligibleA-1)
	}
	if rep.Computers[compA]["event"].Corrupt+rep.Computers[compA]["payload"].Corrupt+rep.Computers[compA]["receipt"].Corrupt != 1 {
		t.Fatalf("P1: per-computer tally %+v", rep.Computers[compA])
	}
}

// P1: with two copies of a record, one corrupt, the file is not covered
// (the production reader answers with the newest copy and stops at an error).
func TestP1OneCorruptCopyOfTwoIsNotCovered(t *testing.T) {
	w := newWorld(t)
	mustRun(t, w.copyOpts(t, compA))
	d := w.payloads[compA][0]
	legacy, _ := os.ReadFile(filepath.Join(w.f.root, "sha256", "computer-event-payload", d))
	app := tapestore.NewAppender(w.f.root)
	if err := app.Append(compA, tapestore.Item{NS: tapestore.NSPayload, Body: legacy}); err != nil {
		t.Fatal(err)
	}
	if err := app.Seal(compA); err != nil {
		t.Fatal(err)
	}
	// A clean second copy: still covered.
	_, vs := verdicts(t, w.opts(t))
	if v := vs["payload/"+d]; !v.Deletable() {
		t.Fatalf("two clean copies: %s", v.Class())
	}
	seg2 := filepath.Join(w.f.root, tapestore.SegDirName, compA, tapestore.SegmentFileName(2))
	raw, _ := os.ReadFile(seg2)
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(seg2, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	_, vs = verdicts(t, w.opts(t))
	if v := vs["payload/"+d]; v.Deletable() || v.Class() != "corrupt" {
		t.Fatalf("P1: one good and one corrupt copy: %s", v.Class())
	}
}

// P2: one unreadable segment blocks the whole computer.
func TestP2AnUnreadableSegmentBlocksTheComputer(t *testing.T) {
	w := newWorld(t)
	mustRun(t, w.copyOpts(t, compA))
	idx := filepath.Join(w.f.root, tapestore.SegDirName, compA, tapestore.IndexFileName(1))
	raw, err := os.ReadFile(idx)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)/2] ^= 0x01
	if err := os.WriteFile(idx, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	rep, vs := verdicts(t, w.opts(t))
	if rep.OK() || rep.SegmentErrors[compA] == "" {
		t.Fatalf("P2: ok=%v errors=%v", rep.OK(), rep.SegmentErrors)
	}
	for _, d := range w.events[compA] {
		if v := vs["event/"+d]; v.Deletable() || v.Class() != "blocked" {
			t.Fatalf("P2: %s", v.Class())
		}
	}
	if rep.Global["event"].Blocked != 3 || rep.Global["receipt"].Blocked != 6 {
		t.Fatalf("P2: %+v", rep.Global)
	}
}

// P3: a file no receipt names is never covered, even with a copy of its
// bytes in a computer's segments.
func TestP3AnUnreceiptedFileIsNeverCovered(t *testing.T) {
	w := newWorld(t)
	orphan := eventBody(compA, string(computerevent.EventProjectionBatchRecorded), "orphan")
	od := hexSum(orphan)
	w.f.write("sha256/computer-event/"+od, orphan)
	app := tapestore.NewAppender(w.f.root)
	if err := app.Append(compA, tapestore.Item{NS: tapestore.NSEvent, Body: orphan}); err != nil {
		t.Fatal(err)
	}
	rep, vs := verdicts(t, w.opts(t))
	if v := vs["event/"+od]; v.Deletable() || v.Class() != "unreceipted" {
		t.Fatalf("P3: orphan %s", v.Class())
	}
	if rep.Global["event"].Unreceipted != 1 {
		t.Fatalf("P3: %+v", rep.Global["event"])
	}
	// The copy never writes it either.
	mustRun(t, w.copyOpts(t, compA))
	rd := tapestore.NewReader(w.f.root)
	_, found, _ := rd.ReadSegment(compB, tapestore.NSEvent, od)
	if found {
		t.Fatal("P3: orphan copied under B")
	}
}

var bannedUnlinks = []string{"os.Remove", "os.RemoveAll", "os.Rename", "syscall.Unlink", "syscall.Unlinkat", "syscall.Rename", "unix.Unlink", "unix.Unlinkat", "unix.Rename", "unix.Renameat", "unix.Renameat2", "syscall.Renameat", "os.Link", "syscall.Link", "unix.Link", "unix.Linkat", "syscall.Linkat"}

// P4: a Verdict cannot be made deletable by a caller, and the package has
// no unlink or rename outside workdir.go.
func TestP4VerdictIsUnforgeableAndThePackageNeverUnlinks(t *testing.T) {
	var zero Verdict
	if zero.Deletable() || zero.Class() == "covered" {
		t.Fatal("P4: the zero Verdict is deletable")
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go") && fi.Name() != "workdir.go"
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	banned := bannedUnlinks
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			raw, _ := os.ReadFile(name)
			_ = file
			for _, b := range banned {
				if callsBanned(string(raw), b) {
					t.Fatalf("P4: %s calls %s outside workdir.go", name, b)
				}
			}
		}
	}
}

// P4: the work directory may not be inside the artifacts root.
func TestP4WorkDirInsideTheRootIsRefused(t *testing.T) {
	w := newWorld(t)
	o := w.opts(t)
	o.WorkDir = filepath.Join(w.f.root, "work")
	if _, err := Run(o); err == nil {
		t.Fatal("work dir inside the artifacts root accepted")
	}
	if _, err := os.Stat(o.WorkDir); err == nil {
		t.Fatal("work dir created inside the artifacts root")
	}
}

// P5: a payload is attributed by its receipt, not by the computer id inside
// its bytes.
func TestP5APayloadIsAttributedByItsReceipt(t *testing.T) {
	w := newWorld(t)
	body := []byte(`{"computer_id":"` + compB + `","note":"pinned by A, names B"}`)
	d, _ := w.f.pin(compA, "computer-event-payload", computerevent.ProjectionBatchMediaType, body)
	mustRun(t, w.copyOpts(t, compA, compB))
	rd := tapestore.NewReader(w.f.root)
	if _, found, err := rd.ReadSegment(compA, tapestore.NSPayload, d); err != nil || !found {
		t.Fatalf("P5: not under A: found=%v err=%v", found, err)
	}
	if _, found, _ := rd.ReadSegment(compB, tapestore.NSPayload, d); found {
		t.Fatal("P5: copied under B, the computer its bytes name")
	}
}

// P6, P7, P17: files that must not be copied or covered.
func TestP6P7P17IneligibleAndBadFilesAreNeverCopied(t *testing.T) {
	w := newWorld(t)
	f := w.f
	// P7: an event of another kind; a payload of another media type; an event
	// whose computer differs from its receipt's.
	otherKind, _ := f.pin(compA, "computer-event", "application/vnd.choir.computer-event+json", eventBody(compA, "run_started", "x"))
	otherMedia, _ := f.pin(compA, "computer-event-payload", "application/json", []byte(`{"other":1}`))
	mixed, _ := f.pin(compA, "computer-event", "application/vnd.choir.computer-event+json", eventBody(compB, string(computerevent.EventProjectionBatchRecorded), "mixed"))
	// P6: bytes that no longer hash to the name, with a receipt for the name.
	good := []byte(`{"batch":"bad"}`)
	badDigest := hexSum(good)
	f.write("sha256/computer-event-payload/"+badDigest, []byte(`{"batch":"BAD"}`))
	f.receipt(compA, "computer-event-payload", computerevent.ProjectionBatchMediaType, badDigest, len(good))
	// P17: a receipt by another issuer names an artifact: the artifact has no receipt.
	forged := []byte(`{"batch":"forged-issuer"}`)
	forgedDigest := hexSum(forged)
	f.write("sha256/computer-event-payload/"+forgedDigest, forged)
	f.write("sha256/pin-receipts/"+strings.Repeat("e", 64)+".json", []byte(`{"receipt_kind":"PinReceipt","issuer":"someone"}`))

	rep, vs := verdicts(t, w.opts(t))
	for name, want := range map[string]string{
		"event/" + otherKind: "ineligible", "payload/" + otherMedia: "ineligible", "event/" + mixed: "ineligible",
		"payload/" + badDigest: "legacy_bad", "payload/" + forgedDigest: "unreceipted",
	} {
		if v := vs[name]; v.Class() != want || v.Deletable() {
			t.Fatalf("%s: class %q, want %q", name, v.Class(), want)
		}
	}
	mustRun(t, w.copyOpts(t, compA, compB))
	rd := tapestore.NewReader(f.root)
	for _, c := range []struct {
		ns tapestore.NS
		d  string
	}{{tapestore.NSEvent, otherKind}, {tapestore.NSPayload, otherMedia}, {tapestore.NSEvent, mixed}, {tapestore.NSPayload, badDigest}, {tapestore.NSPayload, forgedDigest}} {
		for _, comp := range []string{compA, compB} {
			if _, found, _ := rd.ReadSegment(comp, c.ns, c.d); found {
				t.Fatalf("copied %s %s under %s", c.ns, c.d, comp)
			}
		}
	}
	if rep.Global["payload"].LegacyBad != 1 {
		t.Fatalf("P6: %+v", rep.Global["payload"])
	}
}

// P8: dry run, and a copy that was not asked for with --apply and a computer,
// write nothing.
func TestP8DryRunWritesNothing(t *testing.T) {
	w := newWorld(t)
	before := tree(t, w.f.root)
	o := w.opts(t)
	o.Copy = true // dry run: Apply false
	rep := mustRun(t, o)
	if rep.Copy.WouldCopyRecords != int64(w.eligibleA)+4 || rep.Copy.CopiedRecords != 0 {
		t.Fatalf("P8: %+v", rep.Copy)
	}
	sameTree(t, "P8 dry run", before, tree(t, w.f.root))
	o = w.opts(t)
	o.Copy, o.Apply = true, true // no computer named
	if _, err := Run(o); err == nil {
		t.Fatal("P8: --apply without a computer ran")
	}
	sameTree(t, "P8 apply without a computer", before, tree(t, w.f.root))
	o = w.opts(t)
	if _, err := Run(Options{Root: o.Root, WorkDir: o.WorkDir, Apply: true, Computers: []string{compA}}); err == nil {
		t.Fatal("P8: apply without copy ran")
	}
	sameTree(t, "P8 apply without copy", before, tree(t, w.f.root))
}

// P9: a second apply writes nothing; a killed run resumes without writing a
// record twice and ends with the totals of an uninterrupted run.
func TestP9RerunWritesNothingAndAKilledRunResumes(t *testing.T) {
	w := newWorld(t)
	o := w.copyOpts(t, compA)
	first := mustRun(t, o)
	after := segmentTree(t, w.f.root)
	second := mustRun(t, o)
	if second.Copy.CopiedRecords != 0 {
		t.Fatalf("P9: second run copied %d records", second.Copy.CopiedRecords)
	}
	sameTree(t, "P9 rerun", after, segmentTree(t, w.f.root))
	if first.Copy.CopiedRecords != int64(w.eligibleA) {
		t.Fatalf("first run copied %d", first.Copy.CopiedRecords)
	}

	// Killed after the append of a bucket and before its result file.
	w2 := newWorld(t)
	o2 := w2.copyOpts(t, compA)
	kills := 0
	o2.testAfterAppend = func(bucket int) error {
		kills++
		if kills == 2 {
			return errors.New("killed")
		}
		return nil
	}
	if _, err := Run(o2); err == nil {
		t.Fatal("P9: the injected kill did not stop the run")
	}
	o2.testAfterAppend = nil
	resumed := mustRun(t, o2)
	_, vs := verdicts(t, w2.opts(t))
	dels := 0
	for _, v := range vs {
		if v.Deletable() {
			dels++
		}
	}
	if dels != w2.eligibleA {
		t.Fatalf("P9: %d deletable after the resumed run, want %d", dels, w2.eligibleA)
	}
	// No record twice: the views of every bucket list each (ns, digest) once.
	v, err := tapestore.NewReader(w2.f.root).OpenView(compA)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for b := 0; b < 256; b++ {
		locs, err := v.Bucket(byte(b))
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range locs {
			seen[fmt.Sprintf("%d/%x", l.NS, l.Digest)]++
		}
	}
	if len(seen) != w2.eligibleA {
		t.Fatalf("P9: %d distinct records, want %d", len(seen), w2.eligibleA)
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("P9: %s written %d times", k, n)
		}
	}
	if resumed.Copy.CopiedRecords > int64(w2.eligibleA) {
		t.Fatalf("P9: resumed run reports %d copied", resumed.Copy.CopiedRecords)
	}
}

// P10: a run for A writes nothing under B.
func TestP10ARunForAWritesNothingForB(t *testing.T) {
	w := newWorld(t)
	mustRun(t, w.copyOpts(t, compA))
	if _, err := os.Stat(filepath.Join(w.f.root, tapestore.SegDirName, compB)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("P10: B's segment directory exists: %v", err)
	}
	rep := mustRun(t, w.opts(t))
	if rep.Global["event"].Uncovered != 1 {
		t.Fatalf("P10: %+v", rep.Global["event"])
	}
}

// P11: a legacy file that disappears between the listing and the read.
func TestP11AFileGoneBetweenListAndReadIsVanished(t *testing.T) {
	w := newWorld(t)
	gone := w.events[compA][0]
	o := w.opts(t)
	o.testAfterList = func() {
		if err := os.Remove(filepath.Join(w.f.root, "sha256", "computer-event", gone)); err != nil {
			t.Error(err)
		}
	}
	rep, vs := verdicts(t, o)
	if v := vs["event/"+gone]; v.Class() != "vanished" || v.Deletable() {
		t.Fatalf("P11: %s", v.Class())
	}
	if rep.Global["event"].Vanished != 1 {
		t.Fatalf("P11: %+v", rep.Global["event"])
	}
}

// P12: a live writer appends to the same computer during the copy.
func TestP12ALiveWriterAndTheCopyDoNotLoseOrDuplicateRecords(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 60; i++ {
		w.f.batchEvent(compA, fmt.Sprintf("more-%d", i))
		w.f.batchPayload(compA, fmt.Sprintf("more-%d", i))
	}
	live := tapestore.NewAppender(w.f.root)
	var wg sync.WaitGroup
	wg.Add(1)
	acked := make(chan tapestore.Item, 400)
	go func() {
		defer wg.Done()
		for i := 0; i < 150; i++ {
			it := tapestore.Item{NS: tapestore.NSPayload, Body: []byte(fmt.Sprintf(`{"live":%d}`, i))}
			if err := live.Append(compA, it); err != nil {
				t.Errorf("P12: live append: %v", err)
				return
			}
			acked <- it
		}
	}()
	rep := mustRun(t, w.copyOpts(t, compA))
	wg.Wait()
	close(acked)
	rd := tapestore.NewReader(w.f.root)
	for it := range acked {
		got, found, err := rd.ReadSegment(compA, it.NS, hexSum(it.Body))
		if err != nil || !found || !bytes.Equal(got, it.Body) {
			t.Fatalf("P12: acknowledged live record lost: found=%v err=%v", found, err)
		}
	}
	if err := rd.Scan(compA, func(tapestore.Record) error { return nil }); err != nil {
		t.Fatalf("P12: Scan: %v", err)
	}
	cen := mustRun(t, w.opts(t))
	if cen.Computers[compA]["event"].Uncovered != 0 || cen.Computers[compA]["event"].Covered != 63 {
		t.Fatalf("P12: %+v (copy %+v)", cen.Computers[compA]["event"], rep.Copy)
	}
}

// P13: a failed fsync poisons the appender; the run stops with an error and
// a rerun finishes the job.
func TestP13APoisonedAppenderStopsTheRunAndARerunFinishes(t *testing.T) {
	w := newWorld(t)
	o := w.copyOpts(t, compA)
	calls := 0
	o.testSyncHook = func(*os.File) error {
		calls++
		if calls > 4 {
			return errors.New("injected fsync failure")
		}
		return nil
	}
	if _, err := Run(o); err == nil {
		t.Fatal("P13: an fsync failure did not stop the run")
	}
	o.testSyncHook = nil
	mustRun(t, o)
	_, vs := verdicts(t, w.opts(t))
	n := 0
	for _, v := range vs {
		if v.Deletable() {
			n++
		}
	}
	if n != w.eligibleA {
		t.Fatalf("P13: %d covered after the rerun, want %d", n, w.eligibleA)
	}
}

// P18: an empty tree proves nothing.
func TestP18AnEmptyCensusIsNotOK(t *testing.T) {
	rep := mustRun(t, Options{Root: t.TempDir(), WorkDir: t.TempDir()})
	if rep.OK() {
		t.Fatal("P18: an empty census is ok")
	}
}

// P19: the census report is written under its own digest, and the digest
// covers the report.
func TestP19TheReportIsNamedByItsDigest(t *testing.T) {
	w := newWorld(t)
	o := w.opts(t)
	rep := mustRun(t, o)
	path := filepath.Join(o.WorkDir, "census-"+rep.Receipt+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("P19: %v", err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != rep.Receipt {
		t.Fatalf("P19: file hashes to %x, receipt says %s", sum, rep.Receipt)
	}
}
