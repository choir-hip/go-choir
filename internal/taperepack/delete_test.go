package taperepack

// The census-gated delete (owner ruling 2026-10-22, "4. yes"): failure modes
// D1 to D10 of docs/problems/platform-artifact-small-file-growth-2026-10-19.md,
// written before the command. They fail on a8f3768ed, which has no Delete.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// copied builds the world, copies the named computers and takes a census.
func (w *world) copied(t *testing.T, computers ...string) (workDir string, receipt string) {
	t.Helper()
	mustRun(t, w.copyOpts(t, computers...))
	o := w.opts(t)
	rep := mustRun(t, o)
	return o.WorkDir, rep.Receipt
}

func (w *world) delOpts(workDir, receipt string) DeleteOptions {
	return DeleteOptions{Root: w.f.root, WorkDir: workDir, CensusReceipt: receipt, Pause: time.Nanosecond}
}

func (w *world) legacyExists(ns tapestore.NS, digest string) bool {
	_, err := os.Stat(filepath.Join(w.f.root, filepath.FromSlash(ns.LegacyRel(digest))))
	return err == nil
}

func (w *world) allDigests(c string) map[tapestore.NS][]string {
	return map[tapestore.NS][]string{
		tapestore.NSEvent: w.events[c], tapestore.NSPayload: w.payloads[c], tapestore.NSReceipt: w.receipts[c],
	}
}

// D1: no receipt digest, or a wrong one, refuses and unlinks nothing.
func TestD1ADeleteNeedsTheCensusReceiptDigest(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA, compB)
	before := tree(t, w.f.root)
	for name, digest := range map[string]string{
		"empty": "", "malformed": "xyz", "unknown": strings.Repeat("0", 64),
		"another file's digest": hexSum([]byte("not a census")),
	} {
		o := w.delOpts(workDir, digest)
		o.Apply = true
		if _, err := Delete(o); err == nil {
			t.Fatalf("D1 %s: Delete ran", name)
		}
	}
	// A census file whose bytes were edited no longer hashes to its name.
	path := filepath.Join(workDir, "census-"+receipt+".json")
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(raw[:len(raw)-1:len(raw)-1], ' ', '}'), 0o600); err != nil {
		t.Fatal(err)
	}
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	if _, err := Delete(o); err == nil {
		t.Fatal("D1: an edited census file was accepted")
	}
	sameTree(t, "D1", before, tree(t, w.f.root))
}

// D2: apply without a digest does nothing, and a digest without apply is a
// dry run (D6).
func TestD2ApplyWithoutADigestDoesNothing(t *testing.T) {
	w := newWorld(t)
	workDir, _ := w.copied(t, compA, compB)
	before := tree(t, w.f.root)
	o := w.delOpts(workDir, "")
	o.Apply = true
	if _, err := Delete(o); err == nil {
		t.Fatal("D2: --apply without a digest ran")
	}
	sameTree(t, "D2", before, tree(t, w.f.root))
}

// D3: only covered files go. Uncovered (B was not copied), ineligible and
// unreceipted files stay even though the census was clean for what it covers.
func TestD3OnlyCoveredFilesAreUnlinked(t *testing.T) {
	w := newWorld(t)
	otherKind, _ := w.f.pin(compA, "computer-event", "application/vnd.choir.computer-event+json", eventBody(compA, "run_started", "x"))
	orphan := eventBody(compA, string(computerevent.EventProjectionBatchRecorded), "orphan")
	orphanDigest := hexSum(orphan)
	w.f.write("sha256/computer-event/"+orphanDigest, orphan)
	workDir, receipt := w.copied(t, compA)
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	rep, err := Delete(o)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	for ns, ds := range w.allDigests(compA) {
		for _, d := range ds {
			if w.legacyExists(ns, d) {
				t.Fatalf("D3: covered %s %s still exists", ns, d)
			}
		}
	}
	for ns, ds := range w.allDigests(compB) {
		for _, d := range ds {
			if !w.legacyExists(ns, d) {
				t.Fatalf("D3: uncovered %s %s of B was deleted", ns, d)
			}
		}
	}
	if !w.legacyExists(tapestore.NSEvent, otherKind) || !w.legacyExists(tapestore.NSEvent, orphanDigest) {
		t.Fatal("D3: an ineligible or an unreceipted file was deleted")
	}
	// A's 12 plus the receipt of the other-kind event, which the copy writes
	// to the segments too (residual 1 of the plan) and which is then covered.
	if rep.Deleted != int64(w.eligibleA)+1 || rep.Mode != "delete-apply" {
		t.Fatalf("D3: %+v", rep)
	}
	if rep.Refused["uncovered"] != 4 || rep.Refused["unreceipted"] != 1 || rep.Refused["ineligible"] < 1 {
		t.Fatalf("D3: refused %+v", rep.Refused)
	}
}

// D3: a census that was not healthy (a corrupt copy, a blocked computer, a
// legacy file that does not hash) is no gate at all.
func TestD3AnUnhealthyCensusIsNoGate(t *testing.T) {
	cases := map[string]func(w *world){
		"corrupt copy": func(w *world) {
			p := filepath.Join(w.f.root, tapestore.SegDirName, compA, tapestore.SegmentFileName(1))
			raw, _ := os.ReadFile(p)
			raw[len(raw)-3] ^= 0xff
			_ = os.WriteFile(p, raw, 0o640)
		},
		"blocked computer": func(w *world) {
			p := filepath.Join(w.f.root, tapestore.SegDirName, compA, tapestore.IndexFileName(1))
			raw, _ := os.ReadFile(p)
			raw[len(raw)/2] ^= 1
			_ = os.WriteFile(p, raw, 0o640)
		},
		"legacy file that does not hash": func(w *world) {
			w.f.write("sha256/computer-event-payload/"+hexSum([]byte("claimed")), []byte("something else"))
		},
	}
	for name, damage := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			mustRun(t, w.copyOpts(t, compA, compB))
			damage(w)
			o := w.opts(t)
			rep := mustRun(t, o)
			if rep.Healthy() {
				t.Fatal("setup: the census is healthy")
			}
			before := tree(t, w.f.root)
			d := w.delOpts(o.WorkDir, rep.Receipt)
			d.Apply = true
			if _, err := Delete(d); err == nil {
				t.Fatal("D3: Delete accepted an unhealthy census")
			}
			sameTree(t, "D3", before, tree(t, w.f.root))
		})
	}
}

// D4: a stale receipt and a receipt for another root refuse.
func TestD4StaleOrForeignReceiptsRefuse(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA, compB)
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	o.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if _, err := Delete(o); err == nil {
		t.Fatal("D4: a 25 hour old census was accepted")
	}
	other := newWorld(t)
	o2 := other.delOpts(workDir, receipt) // the receipt is for w's root
	o2.Apply = true
	before := tree(t, other.f.root)
	if _, err := Delete(o2); err == nil {
		t.Fatal("D4: a census of another root was accepted")
	}
	sameTree(t, "D4", before, tree(t, other.f.root))
	// A census with --computer A does not authorize B.
	w2 := newWorld(t)
	mustRun(t, w2.copyOpts(t, compA, compB))
	oc := w2.opts(t)
	oc.Computers = []string{compA}
	rep := mustRun(t, oc)
	d := w2.delOpts(oc.WorkDir, rep.Receipt)
	d.Apply, d.Computers = true, []string{compB}
	if _, err := Delete(d); err == nil {
		t.Fatal("D4: a census of A authorized B")
	}
}

// D5: a copy that turns corrupt after the census is caught by the verdict
// recomputed at delete time.
func TestD5ACopyThatTurnsCorruptAfterTheCensusIsNotDeleted(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA, compB)
	p := filepath.Join(w.f.root, tapestore.SegDirName, compA, tapestore.SegmentFileName(1))
	raw, _ := os.ReadFile(p)
	raw[len(raw)-3] ^= 0xff
	if err := os.WriteFile(p, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	rep, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Refused["corrupt"] != 1 || rep.Deleted != int64(w.eligibleA-1)+4 {
		t.Fatalf("D5: %+v", rep)
	}
	remaining := 0
	for _, ds := range w.allDigests(compA) {
		remaining += len(ds)
	}
	left := 0
	for ns, ds := range w.allDigests(compA) {
		for _, d := range ds {
			if w.legacyExists(ns, d) {
				left++
			}
		}
	}
	if left != 1 {
		t.Fatalf("D5: %d of A's files remain, want exactly the corrupt one", left)
	}
}

// D6, D7, D8, D10: the dry run unlinks nothing; the apply leaves records
// readable from segments, touches nothing else, and logs each unlink.
func TestD6D7D8D10DryRunApplyAndTheLog(t *testing.T) {
	w := newWorld(t)
	w.f.write("sha256/other-namespace/"+hexSum([]byte("x")), []byte("x"))
	w.f.write("sha256/computer-event/not-a-digest", []byte("stray"))
	workDir, receipt := w.copied(t, compA, compB)
	before := tree(t, w.f.root)
	o := w.delOpts(workDir, receipt)
	rep, err := Delete(o)
	if err != nil || rep.Mode != "delete-dry-run" || rep.WouldDelete != int64(w.eligibleA)+4 || rep.Deleted != 0 {
		t.Fatalf("D6: %+v %v", rep, err)
	}
	sameTree(t, "D6 dry run", before, tree(t, w.f.root))
	// The digest alone, without apply, is the dry run; apply with it deletes.
	o.Apply = true
	rep, err = Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Deleted != int64(w.eligibleA)+4 {
		t.Fatalf("D7: %+v", rep)
	}
	rd := tapestore.NewReader(w.f.root)
	for _, c := range []string{compA, compB} {
		for ns, ds := range w.allDigests(c) {
			for _, d := range ds {
				if w.legacyExists(ns, d) {
					t.Fatalf("D7: %s %s remains", ns, d)
				}
				body, src, err := rd.Read(c, ns, d)
				if err != nil || src != tapestore.SourceSegment || hexSum(body) != d {
					t.Fatalf("D7: read %s %s: src=%v err=%v", ns, d, src, err)
				}
			}
		}
	}
	after := tree(t, w.f.root)
	for k := range after {
		if strings.HasPrefix(k, "sha256/computer-event") || strings.HasPrefix(k, "sha256/pin-receipts") {
			continue
		}
	}
	for _, keep := range []string{"sha256/other-namespace", "sha256/computer-event/not-a-digest", "tape-seg"} {
		found := false
		for k := range after {
			if strings.HasPrefix(k, keep) {
				found = true
			}
		}
		if !found {
			t.Fatalf("D8: %s is gone", keep)
		}
	}
	// D10: one log line per unlinked file.
	logRaw, err := os.ReadFile(rep.Log)
	if err != nil {
		t.Fatalf("D10: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(logRaw), []byte("\n"))
	if int64(len(lines)) != rep.Deleted {
		t.Fatalf("D10: %d log lines for %d unlinks", len(lines), rep.Deleted)
	}
	var line struct {
		NS, Digest string
		Bytes      int64
	}
	if err := json.Unmarshal(lines[0], &line); err != nil || line.Digest == "" || line.Bytes <= 0 {
		t.Fatalf("D10: first line %q: %v", lines[0], err)
	}
}

// D9: a rerun after a partial delete deletes the rest and nothing more.
func TestD9ARerunAfterAPartialDeleteFinishesTheJob(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA, compB)
	// Part of the job was already done by an earlier run.
	for _, d := range w.events[compA] {
		_ = os.Remove(filepath.Join(w.f.root, "sha256", "computer-event", d))
	}
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	rep, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Deleted != int64(w.eligibleA)+4-3 {
		t.Fatalf("D9: %+v", rep)
	}
	rep, err = Delete(o)
	if err != nil || rep.Deleted != 0 {
		t.Fatalf("D9 second rerun: %+v %v", rep, err)
	}
}

// D3/P4: the unlink helper itself refuses anything that is not deletable.
func TestD3TheUnlinkHelperRefusesANonDeletableVerdict(t *testing.T) {
	w := newWorld(t)
	d := w.events[compA][0]
	for _, v := range []Verdict{{}, {ns: tapestore.NSEvent, digest: d, class: classUncovered}, {ns: tapestore.NSEvent, digest: d, class: classCorrupt}} {
		if _, err := unlinkCovered(w.f.root, v); err == nil {
			t.Fatalf("unlinked a %s verdict", v.Class())
		}
	}
	if !w.legacyExists(tapestore.NSEvent, d) {
		t.Fatal("the file is gone")
	}
}
