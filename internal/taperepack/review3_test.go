package taperepack

// Review round 3 on 0866a7663 (bunch 140): L1 legacy event bodies are scanned
// for the digests they name, L2 the unreadable segment record paths, L4
// per-computer tallies follow the final class, L5 every 64-hex window of a
// run, L6 a symlinked segment directory. docs/problems/
// platform-artifact-small-file-growth-2026-10-19.md. L1, L4, L5 and L6 fail on
// 0866a7663; L2 pins paths that already hold.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// corruptSegmentBytes flips one byte of raw inside the computer's segment file
// segno, leaving the index (and so the record's location) untouched.
func corruptSegmentBytes(t *testing.T, root, computer string, segno uint64, raw []byte) {
	t.Helper()
	p := filepath.Join(root, tapestore.SegDirName, computer, tapestore.SegmentFileName(segno))
	seg, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(seg, raw)
	if i < 0 {
		t.Fatalf("record not found in %s", p)
	}
	seg[i+len(raw)/2] ^= 0xff
	if err := os.WriteFile(p, seg, 0o640); err != nil {
		t.Fatal(err)
	}
}

// L1: a legacy event of B names X; B has no receipt for X anywhere; A pins X
// and has a sealed copy. The sweep keeps X because the chain names it, so the
// delete must too.
func TestL1ALegacyEventBodyThatNamesAPayloadAttributesTheComputer(t *testing.T) {
	w := newWorld(t)
	x, _ := w.f.batchPayload(compA, "legacychainref")
	ev := []byte(`{"computer_id":"` + compB + `","event_kind":"other","payload_ref":"` + x + `"}`)
	w.f.write("sha256/computer-event/"+hexSum(ev), ev)
	workDir, receipt := w.copied(t, compA)
	_, vs := verdicts(t, w.opts(t))
	if v := vs["payload/"+x]; v.Deletable() {
		t.Fatalf("L1: X is deletable (%q) although B's legacy event names it", v.Class())
	}
	do := w.delOpts(workDir, receipt)
	do.Apply = true
	if _, err := Delete(do); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !w.legacyExists(tapestore.NSPayload, x) {
		t.Fatal("L1: the payload B's chain names was unlinked")
	}
	if w.legacyExists(tapestore.NSPayload, w.payloads[compA][0]) {
		t.Fatal("L1: A's own covered payload was not deleted (the delete did not run)")
	}
}

// L1: a legacy event body that cannot be decoded for its computer names
// digests nobody can see: the pass is unhealthy and deletes nothing.
func TestL1AnUndecodableLegacyEventBlocksEveryDelete(t *testing.T) {
	for name, body := range map[string][]byte{
		"not json":       []byte("{not json"),
		"no computer id": []byte(`{"event_kind":"other"}`),
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			workDir, receipt := w.copied(t, compA, compB)
			w.f.write("sha256/computer-event/"+hexSum(body), body)
			before := tree(t, w.f.root)
			o := w.delOpts(workDir, receipt)
			o.Apply = true
			rep, err := Delete(o)
			if err != nil {
				t.Fatal(err)
			}
			if rep.Deleted != 0 || rep.VerifyHealthy || len(rep.SegmentErrors) == 0 {
				t.Fatalf("L1: %+v", rep)
			}
			sameTree(t, "L1", before, tree(t, w.f.root))
		})
	}
}

// L2: a segment record that cannot be read falls back to the legacy file of
// the same digest when it still hashes to it. Here C's segment holds the
// receipt that names B; the legacy copy attributes C, and C has no sealed
// copy of X, so X stays.
func TestL2AnUnreadableSegmentRecordFallsBackToTheLegacyFile(t *testing.T) {
	w := newWorld(t)
	x, _ := w.f.batchPayload(compA, "fallback")
	raw := w.f.receiptRaw(compB, "computer-event-payload", computerevent.ProjectionBatchMediaType, x, len(`{"batch":"fallback"}`))
	w.f.write("sha256/pin-receipts/"+hexSum(raw)+".json", raw)
	app := tapestore.NewAppender(w.f.root)
	const compC = "computer-c"
	if err := app.Append(compC, tapestore.Item{NS: tapestore.NSReceipt, Body: raw}); err != nil {
		t.Fatal(err)
	}
	if err := app.Seal(compC); err != nil {
		t.Fatal(err)
	}
	mustRun(t, w.copyOpts(t, compA, compB))
	corruptSegmentBytes(t, w.f.root, compC, 1, raw)
	rep, vs := verdicts(t, w.opts(t))
	if len(rep.SegmentErrors) != 0 || !rep.Healthy() {
		t.Fatalf("L2: the legacy fallback was not used: %+v %+v", rep.SegmentErrors, rep.Listing)
	}
	if v := vs["payload/"+x]; v.Deletable() || v.Class() != "uncovered" {
		t.Fatalf("L2: X is %q (deletable %v), want uncovered while C has no sealed copy", v.Class(), v.Deletable())
	}
}

// L2: a receipt that lives only in a segment, with no legacy file, and whose
// record is damaged is unknown: attribution error, nothing is deletable.
// A legacy file of the same name that no longer hashes to it is no fallback.
func TestL2AnUnreadableSegmentOnlyRecordIsAnAttributionError(t *testing.T) {
	for _, withBadLegacy := range []bool{false, true} {
		w := newWorld(t)
		x, _ := w.f.batchPayload(compA, "segonlydamaged")
		raw := w.f.receiptRaw(compB, "computer-event-payload", computerevent.ProjectionBatchMediaType, x, len(`{"batch":"segonlydamaged"}`))
		app := tapestore.NewAppender(w.f.root)
		if err := app.Append(compB, tapestore.Item{NS: tapestore.NSReceipt, Body: raw}); err != nil {
			t.Fatal(err)
		}
		if err := app.Seal(compB); err != nil {
			t.Fatal(err)
		}
		if withBadLegacy {
			w.f.write("sha256/pin-receipts/"+hexSum(raw)+".json", append(append([]byte(nil), raw...), ' '))
		}
		mustRun(t, w.copyOpts(t, compA))
		corruptSegmentBytes(t, w.f.root, compB, 1, raw)
		rep, vs := verdicts(t, w.opts(t))
		if rep.Healthy() || rep.SegmentErrors[compB] == "" {
			t.Fatalf("L2 (bad legacy %v): the damaged record did not make the pass unhealthy: %+v %+v", withBadLegacy, rep.SegmentErrors, rep.Listing)
		}
		for k, v := range vs {
			if v.Deletable() {
				t.Fatalf("L2 (bad legacy %v): %s is deletable beside an attribution error", withBadLegacy, k)
			}
		}
	}
}

// L4: a file the pass refuses (blocked by an attribution error) is not
// counted covered for any computer either.
func TestL4PerComputerTalliesFollowTheFinalClass(t *testing.T) {
	w := newWorld(t)
	w.copied(t, compA, compB)
	p := filepath.Join(w.f.root, tapestore.SegDirName, compB, tapestore.IndexFileName(1))
	raw, _ := os.ReadFile(p)
	raw[len(raw)/2] ^= 1
	if err := os.WriteFile(p, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	rep := mustRun(t, w.opts(t))
	if rep.Global["payload"].Covered != 0 {
		t.Fatalf("setup: %+v", rep.Global["payload"])
	}
	for c, m := range rep.Computers {
		for ns, tl := range m {
			if tl.Covered != 0 {
				t.Fatalf("L4: %s %s counts %d covered files the pass refused", c, ns, tl.Covered)
			}
		}
	}
	if rep.Computers[compA]["payload"].Blocked == 0 {
		t.Fatalf("L4: A's blocked payloads are not counted: %+v", rep.Computers[compA])
	}
}

// L5: the sweep's regexp matches inside longer hex runs, so two digests
// written back to back are both kept; so must the repack's references be.
func TestL5DigestsInsideALongerHexRunAreReferences(t *testing.T) {
	w := newWorld(t)
	x, _ := w.f.batchPayload(compA, "glued1")
	y, _ := w.f.batchPayload(compA, "glued2")
	ev := []byte(`{"computer_id":"` + compB + `","event_kind":"other","payload_ref":"` + x + y + `"}`)
	if err := tapestore.NewAppender(w.f.root).Append(compB, tapestore.Item{NS: tapestore.NSEvent, Body: ev}); err != nil {
		t.Fatal(err)
	}
	mustRun(t, w.copyOpts(t, compA))
	_, vs := verdicts(t, w.opts(t))
	for _, d := range []string{x, y} {
		if v := vs["payload/"+d]; v.Deletable() {
			t.Fatalf("L5: %s is deletable (%q) although a run of hex in B's event contains it", d[:8], v.Class())
		}
	}
}

// L6: a symlink where a computer's segment directory should be is not
// skipped: the segments behind it are unknown.
func TestL6ASymlinkedSegmentDirectoryIsAnAttributionError(t *testing.T) {
	w := newWorld(t)
	mustRun(t, w.copyOpts(t, compA))
	if err := os.Symlink(filepath.Join(w.f.root, tapestore.SegDirName, compA), filepath.Join(w.f.root, tapestore.SegDirName, "computer-s")); err != nil {
		t.Fatal(err)
	}
	rep := mustRun(t, w.opts(t))
	if rep.Healthy() || rep.SegmentErrors["computer-s"] == "" {
		t.Fatalf("L6: %+v %+v", rep.SegmentErrors, rep.Listing)
	}
}
