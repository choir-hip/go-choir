package taperepack

// Review round 4 on 8722c4932 (bunch 141): M2 the database-side keep set is a
// --keep-digests file enforced at the unlink, F2 a non-regular entry in a
// legacy directory is an attribution error, F3 hexRefs follows the sweep's own
// match offsets. docs/problems/platform-artifact-small-file-growth-2026-10-19.md.
// M2 and F2 and F3 fail on 8722c4932 (the M2 tests do not build there: no
// KeepSet, no Keep option, no KeptByList).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

func keepOf(t *testing.T, digests ...string) *KeepSet {
	t.Helper()
	in := ""
	if len(digests) > 0 {
		in = strings.Join(digests, "\n") + "\n"
	}
	k, err := ParseKeepDigests(strings.NewReader(in), true)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// M2: a covered file whose digest the keep file names survives the apply (in
// any namespace), is counted as kept_by_list, and the rest of the covered
// files still go.
func TestM2ACoveredFileNamedInTheKeepFileSurvives(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA)
	kept := w.payloads[compA][0]
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	o.Keep = keepOf(t, kept)
	rep, err := Delete(o)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !w.legacyExists(tapestore.NSPayload, kept) {
		t.Fatal("M2: a digest named in the keep file was unlinked")
	}
	if rep.KeptByList != 1 || rep.KeepDigests != 1 {
		t.Fatalf("M2: kept_by_list %d keep_digests %d, want 1 and 1", rep.KeptByList, rep.KeepDigests)
	}
	if rep.Deleted == 0 {
		t.Fatal("M2: nothing else was deleted (the delete did not run)")
	}
	for _, d := range w.payloads[compA][1:] {
		if w.legacyExists(tapestore.NSPayload, d) {
			t.Fatalf("M2: covered %s not in the keep file still exists", d[:8])
		}
	}
}

// M2: the dry run counts a listed digest as kept, not as would-delete.
func TestM2TheDryRunCountsKeptByList(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA)
	o := w.delOpts(workDir, receipt)
	o.Keep = keepOf(t)
	base, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	o.Keep = keepOf(t, w.payloads[compA][0])
	rep, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if rep.KeptByList != 1 || rep.WouldDelete != base.WouldDelete-1 {
		t.Fatalf("M2 dry run: kept %d would_delete %d (without the list %d)", rep.KeptByList, rep.WouldDelete, base.WouldDelete)
	}
}

// M2: the enforcement is at the unlink itself.
func TestM2UnlinkCoveredRefusesAListedDigest(t *testing.T) {
	w := newWorld(t)
	mustRun(t, w.copyOpts(t, compA))
	_, vs := verdicts(t, w.opts(t))
	d := w.payloads[compA][0]
	v := vs["payload/"+d]
	if !v.Deletable() {
		t.Fatalf("setup: %q", v.Class())
	}
	if _, err := unlinkCovered(w.f.root, v, keepOf(t, d)); err == nil {
		t.Fatal("M2: unlinkCovered removed a listed digest")
	}
	if !w.legacyExists(tapestore.NSPayload, d) {
		t.Fatal("M2: the file is gone")
	}
}

// M2: apply without a keep list does not start; neither does an empty list
// that was not explicitly allowed. Nothing is unlinked.
func TestM2ApplyWithoutAKeepListRefuses(t *testing.T) {
	w := newWorld(t)
	workDir, receipt := w.copied(t, compA)
	before := tree(t, w.f.root)
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	o.Keep = nil
	if _, err := Delete(o); err == nil {
		t.Fatal("M2: --apply ran without a keep list")
	}
	sameTree(t, "M2", before, tree(t, w.f.root))
}

func TestM2KeepFileParsing(t *testing.T) {
	good := strings.Repeat("ab", 32)
	for name, in := range map[string]string{
		"short":             good[:63] + "\n",
		"long":              good + "0\n",
		"uppercase":         strings.ToUpper(good) + "\n",
		"not hex":           strings.Repeat("g", 64) + "\n",
		"blank line":        good + "\n\n" + good + "\n",
		"trailing sp":       good + " \n",
		"two per line":      good + " " + good + "\n",
		"crlf":              good + "\r\n",
		"bad in the middle": good + "\n" + good[:10] + "\n" + good + "\n",
	} {
		if _, err := ParseKeepDigests(strings.NewReader(in), true); err == nil {
			t.Fatalf("M2 %s: a malformed keep file was accepted", name)
		}
	}
	k, err := ParseKeepDigests(strings.NewReader(good+"\n"+good+"\n"+strings.Repeat("01", 32)), false)
	if err != nil || k.Len() != 2 || !k.Has(good) {
		t.Fatalf("M2: a good keep file (no final newline, a duplicate): %v %v", k, err)
	}
	if _, err := ParseKeepDigests(strings.NewReader(""), false); err == nil {
		t.Fatal("M2: an empty keep file was accepted without the explicit flag")
	}
	if k, err := ParseKeepDigests(strings.NewReader(""), true); err != nil || k.Len() != 0 {
		t.Fatalf("M2: an empty keep file with the flag: %v %v", k, err)
	}
	var nilSet *KeepSet
	if nilSet.Has(good) || nilSet.Len() != 0 {
		t.Fatal("M2: a nil keep set keeps something")
	}
}

// F2: a symlink, a directory or any non-regular entry whose name is a digest in
// one of the three legacy directories is read by tapestore (a symlink is
// followed) but yields no edges here: an attribution error, so the pass is
// unhealthy and every covered file is blocked.
func TestF2ANonRegularLegacyEntryIsAnAttributionError(t *testing.T) {
	for name, setup := range map[string]func(t *testing.T, w *world){
		"symlinked event": func(t *testing.T, w *world) {
			body := eventBody(compB, "other", "symlinked")
			target := filepath.Join(t.TempDir(), "ev")
			if err := os.WriteFile(target, body, 0o640); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(w.f.root, "sha256", "computer-event", hexSum(body))); err != nil {
				t.Fatal(err)
			}
		},
		"symlinked receipt": func(t *testing.T, w *world) {
			d := hexSum([]byte("some receipt"))
			if err := os.Symlink("/nonexistent", filepath.Join(w.f.root, "sha256", "pin-receipts", d+".json")); err != nil {
				t.Fatal(err)
			}
		},
		"directory named as a payload": func(t *testing.T, w *world) {
			d := hexSum([]byte("a payload dir"))
			if err := os.MkdirAll(filepath.Join(w.f.root, "sha256", "computer-event-payload", d), 0o750); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			workDir, receipt := w.copied(t, compA, compB)
			setup(t, w)
			before := tree(t, w.f.root)
			o := w.delOpts(workDir, receipt)
			o.Apply = true
			o.Keep = keepOf(t)
			rep, err := Delete(o)
			if err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if rep.VerifyHealthy || rep.SegmentErrors[legacyNonRegularKey] == "" {
				t.Fatalf("F2: healthy %v, segment errors %v", rep.VerifyHealthy, rep.SegmentErrors)
			}
			if rep.Deleted != 0 {
				t.Fatalf("F2: %d files deleted beside a non-regular legacy entry", rep.Deleted)
			}
			sameTree(t, "F2", before, tree(t, w.f.root))
		})
	}
}

// F3: hexRefs yields the sweep's matches (hex64RE.FindAllString: the run start
// plus multiples of 64), not every window, so a long run is linear work.
func TestF3HexRefsFollowTheSweepsOffsets(t *testing.T) {
	x, y := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if got := hexRefs([]byte(`"` + x + y + `"`)); len(got) != 2 {
		t.Fatalf("F3: glued x+y gives %d refs, want 2", len(got))
	}
	// 70 hex characters: the sweep matches the first 64 only.
	run := strings.Repeat("0123456789abcdef", 5)[:70]
	if got := hexRefs([]byte(run)); len(got) != 1 {
		t.Fatalf("F3: a 70-character run gives %d refs, want 1", len(got))
	}
	// A large run of distinct windows stays at len/64 refs.
	big := bytes.Repeat([]byte("0123456789abcdef"), 1<<14) // 256 KiB
	if got := hexRefs(big); len(got) != 1 {                // all windows equal at 64k offsets
		t.Fatalf("F3: a periodic run gives %d distinct refs, want 1", len(got))
	}
	mixed := make([]byte, 0, 64*1000)
	for i := 0; i < 1000; i++ {
		mixed = append(mixed, []byte(hexSum([]byte{byte(i), byte(i >> 8)}))...)
	}
	if got := hexRefs(mixed); len(got) != 1000 {
		t.Fatalf("F3: 1000 glued digests give %d refs", len(got))
	}
}
