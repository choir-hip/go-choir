package taperepack

// Review round 2 on fd09566db (bunch 139): R1 a pin receipt that lives only in
// a computer's segment, R2 a repin after the list, R3 a dropped receipt removes
// a requirement, R4 stale scan state. docs/problems/
// platform-artifact-small-file-growth-2026-10-19.md. R1 to R3 fail on
// fd09566db; R4 pins a behaviour that already holds.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// segmentOnlyPin is the shape of a segmented repin of an artifact whose legacy
// file already exists (stageSegmented's reuse branch): the receipt goes into
// the computer's segment and no legacy receipt file is written.
func (w *world) segmentOnlyPin(computer, artifact string, length int) {
	w.f.t.Helper()
	raw := w.f.receiptRaw(computer, "computer-event-payload", computerevent.ProjectionBatchMediaType, artifact, length)
	if err := tapestore.NewAppender(w.f.root).Append(computer, tapestore.Item{NS: tapestore.NSReceipt, Body: raw}); err != nil {
		w.f.t.Fatal(err)
	}
}

// R1: B's only receipt for X is in B's segment; A has a legacy receipt and a
// sealed copy. B reads X through the legacy file, so it must not be unlinked
// until B has its own sealed copy.
func TestR1AReceiptOnlyInASegmentStillAttributesTheComputer(t *testing.T) {
	w := newWorld(t)
	x, _ := w.f.batchPayload(compA, "segonly")
	w.segmentOnlyPin(compB, x, len(`{"batch":"segonly"}`))
	mustRun(t, w.copyOpts(t, compA))
	co := w.opts(t)
	cen := mustRun(t, co)
	if !cen.Healthy() {
		t.Fatalf("setup: census not healthy: %+v", cen)
	}
	_, vs := verdicts(t, w.opts(t))
	if v := vs["payload/"+x]; v.Deletable() || v.Class() != "uncovered" {
		t.Fatalf("R1: X is %q (deletable %v), want uncovered while B has no copy", v.Class(), v.Deletable())
	}
	do := w.delOpts(co.WorkDir, cen.Receipt)
	do.Apply = true
	if _, err := Delete(do); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !w.legacyExists(tapestore.NSPayload, x) {
		t.Fatal("R1: the payload B reads through the legacy file was unlinked")
	}
	if w.legacyExists(tapestore.NSPayload, w.payloads[compA][0]) {
		t.Fatal("R1: A's own covered payload was not deleted")
	}
	// Once B has a sealed copy of its own, the file may go.
	mustRun(t, w.copyOpts(t, compB))
	co2 := w.opts(t)
	cen2 := mustRun(t, co2)
	do2 := w.delOpts(co2.WorkDir, cen2.Receipt)
	do2.Apply = true
	if _, err := Delete(do2); err != nil {
		t.Fatalf("Delete after B's copy: %v", err)
	}
	if w.legacyExists(tapestore.NSPayload, x) {
		t.Fatal("R1: with both sealed copies the payload was still not deleted")
	}
}

// R1: when a computer's segments cannot be listed, the receipts in them are
// unknown, so a pass deletes nothing and says it is unhealthy.
func TestR1AComputerWhoseSegmentsCannotBeReadBlocksEveryDelete(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA, compB)
	p := filepath.Join(w.f.root, tapestore.SegDirName, compB, tapestore.IndexFileName(1))
	raw, _ := os.ReadFile(p)
	raw[len(raw)/2] ^= 1
	if err := os.WriteFile(p, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	before := tree(t, w.f.root)
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	rep, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Deleted != 0 || rep.VerifyHealthy || rep.SegmentErrors[compB] == "" {
		t.Fatalf("R1: %+v", rep)
	}
	sameTree(t, "R1", before, tree(t, w.f.root))
}

// R2: a file refreshed (repinned) after the list is not unlinked.
func TestR2AFileTouchedAfterTheListIsNotUnlinked(t *testing.T) {
	w := newWorld(t)
	mustRun(t, w.copyOpts(t, compA))
	_, vs := verdicts(t, w.opts(t))
	d := w.payloads[compA][0]
	v := vs["payload/"+d]
	if !v.Deletable() {
		t.Fatalf("setup: %q", v.Class())
	}
	path := filepath.Join(w.f.root, filepath.FromSlash(tapestore.NSPayload.LegacyRel(d)))
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := unlinkCovered(w.f.root, v); err == nil {
		t.Fatal("R2: unlinked a file whose mtime is newer than the list")
	}
	if !w.legacyExists(tapestore.NSPayload, d) {
		t.Fatal("R2: the file is gone")
	}
}

// R3: a receipt the shape guard rejects still names a file; the file is
// refused and the run is unhealthy, not judged on the other computers alone.
func TestR3ARejectedReceiptRefusesTheFileItNames(t *testing.T) {
	w := newWorld(t)
	x, _ := w.f.batchPayload(compA, "odd")
	body := []byte(`{"batch":"odd"}`)
	fields := map[string]any{
		"computer_id": compB, "artifact_digest": x, "media_type": computerevent.ProjectionBatchMediaType,
		"length": len(body), "privacy_class": "owner", "pin_namespace": "computer-event-payload",
		"request_commitment": strings.Repeat("a", 64),
	}
	r, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", fields, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := r.CanonicalBytes()
	w.f.write("sha256/pin-receipts/"+hexSum(raw)+".json", raw)
	mustRun(t, w.copyOpts(t, compA))
	rep, vs := verdicts(t, w.opts(t))
	if v := vs["payload/"+x]; v.Deletable() {
		t.Fatalf("R3: X is deletable (%q) although a receipt for B was rejected", v.Class())
	}
	if rep.Healthy() {
		t.Fatalf("R3: the run is healthy with a rejected receipt: %+v", rep.Listing)
	}
}

// R3: an edge the join skips (here: B's receipt has another media type) beside
// an edge it keeps removes a requirement; the file is refused and counted.
func TestR3ASkippedEdgeBesideAKeptEdgeRefusesTheFile(t *testing.T) {
	w := newWorld(t)
	x, _ := w.f.batchPayload(compA, "mixed")
	w.f.receipt(compB, "computer-event-payload", "text/plain", x, len(`{"batch":"mixed"}`))
	mustRun(t, w.copyOpts(t, compA))
	rep, vs := verdicts(t, w.opts(t))
	if v := vs["payload/"+x]; v.Deletable() {
		t.Fatalf("R3: X is deletable (%q) although B's receipt for it was skipped", v.Class())
	}
	if rep.Healthy() {
		t.Fatal("R3: the run is healthy with a skipped receipt beside a kept one")
	}
}

// R3: a pin receipt that cannot be parsed at all names nothing, so no file can
// be proven free of it: the pass deletes nothing.
func TestR3AnUnparseableReceiptBlocksEveryDelete(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA, compB)
	junk := []byte("{not json")
	w.f.write("sha256/pin-receipts/"+hexSum(junk)+".json", junk)
	before := tree(t, w.f.root)
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	rep, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Deleted != 0 || rep.VerifyHealthy {
		t.Fatalf("R3: %+v", rep)
	}
	sameTree(t, "R3", before, tree(t, w.f.root))
}

// R4: the work directory of a run that stopped midway never feeds a later
// census; the later report describes the tree as it is then.
func TestR4AStoppedCensusLeavesNoStateForTheNextOne(t *testing.T) {
	w := newWorld(t)
	o := w.opts(t)
	stop := o
	n := 0
	stop.OnVerdict = func(Verdict) error {
		if n++; n > 3 {
			return os.ErrClosed
		}
		return nil
	}
	if _, err := Run(stop); err == nil {
		t.Fatal("setup: the stopped run finished")
	}
	mustRun(t, w.copyOpts(t, compA)) // changes the tree after the stopped run
	o.Computers = []string{compA}
	rep := mustRun(t, o)
	if rep.Covered() != int64(w.eligibleA) {
		t.Fatalf("R4: the census after a stop covers %d files, want %d", rep.Covered(), w.eligibleA)
	}
}

// R1 (FP0 note): the sweep keeps a file the chain names, not only one a
// receipt names. An event in B's segment that carries the payload's digest
// requires B to have a sealed copy of it, even with no receipt for B anywhere.
func TestR1AnEventInASegmentThatNamesAPayloadAttributesTheComputer(t *testing.T) {
	w := newWorld(t)
	x, _ := w.f.batchPayload(compA, "chainref")
	ev := []byte(`{"computer_id":"` + compB + `","event_kind":"other","payload_ref":"` + x + `"}`)
	if err := tapestore.NewAppender(w.f.root).Append(compB, tapestore.Item{NS: tapestore.NSEvent, Body: ev}); err != nil {
		t.Fatal(err)
	}
	mustRun(t, w.copyOpts(t, compA))
	_, vs := verdicts(t, w.opts(t))
	if v := vs["payload/"+x]; v.Deletable() {
		t.Fatalf("R1: X is deletable (%q) although B's event names it", v.Class())
	}
}
