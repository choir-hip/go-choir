package taperepack

// Review round 1 on beaca6d75 (bunch 134): F1 shared file, F2 unsealed tail,
// F3 receipt shape, F5 fresh-pass health, F6 size at unlink, F7 guard scope.
// docs/problems/platform-artifact-small-file-growth-2026-10-19.md. They fail
// on beaca6d75.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// sharedPayload pins one projection batch payload for A and for B (two
// receipts, one legacy file) and returns its digest.
func (w *world) sharedPayload(nonce string) string {
	d, _ := w.f.batchPayload(compA, nonce)
	w.f.receipt(compB, "computer-event-payload", computerevent.ProjectionBatchMediaType, d, len(`{"batch":"`+nonce+`"}`))
	return d
}

// F1: a file needed by A and by B is covered only when both have a verified
// copy. B is not copied, so a run or a delete scoped to A must not unlink it.
func TestF1AFileSharedWithAnUnnamedComputerIsNotDeleted(t *testing.T) {
	for _, name := range []string{"flag", "census filter"} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			shared := w.sharedPayload("shared")
			mustRun(t, w.copyOpts(t, compA))
			// The gate: a healthy census, unfiltered or filtered to A.
			co := w.opts(t)
			if name == "census filter" {
				co.Computers = []string{compA}
			}
			cen := mustRun(t, co)
			if !cen.Healthy() {
				t.Fatalf("census not healthy: %+v", cen.Global)
			}
			_, vs := verdicts(t, Options{Root: w.f.root, WorkDir: t.TempDir(), Computers: []string{compA}})
			if v := vs["payload/"+shared]; v.Deletable() || v.Class() != "uncovered" {
				t.Fatalf("F1: shared payload under --computer A is %q (deletable %v), want uncovered", v.Class(), v.Deletable())
			}
			do := w.delOpts(co.WorkDir, cen.Receipt)
			do.Apply = true
			if name == "flag" {
				do.Computers = []string{compA}
			}
			if _, err := Delete(do); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if !w.legacyExists(tapestore.NSPayload, shared) {
				t.Fatal("F1: the payload B needs was unlinked on A's copy alone")
			}
			// A's other files did go.
			if w.legacyExists(tapestore.NSPayload, w.payloads[compA][0]) {
				t.Fatal("F1: A's own covered payload was not deleted")
			}
		})
	}
}

// F1: a copy for A alone still writes nothing under B.
func TestF1ACopyForAWritesNothingForBEvenForASharedFile(t *testing.T) {
	w := newWorld(t)
	shared := w.sharedPayload("shared2")
	mustRun(t, w.copyOpts(t, compA))
	if _, found, _ := tapestore.NewReader(w.f.root).ReadSegment(compB, tapestore.NSPayload, shared); found {
		t.Fatal("F1: the copy for A wrote a shared payload under B")
	}
	if _, found, _ := tapestore.NewReader(w.f.root).ReadSegment(compA, tapestore.NSPayload, shared); !found {
		t.Fatal("F1: the copy for A did not write the shared payload under A")
	}
}

// F2: a record that only an unsealed (open tail) segment holds is not a
// verified copy: it may not be fsynced. After the copy seals it, it is.
func TestF2ARecordInAnUnsealedTailIsNotCovered(t *testing.T) {
	w := newWorld(t)
	body := []byte(`{"batch":"tail"}`)
	d, _ := w.f.batchPayload(compA, "tail")
	live := tapestore.NewAppender(w.f.root)
	if err := live.Append(compA, tapestore.Item{NS: tapestore.NSPayload, Body: body}); err != nil {
		t.Fatal(err)
	}
	_, vs := verdicts(t, w.opts(t))
	if v := vs["payload/"+d]; v.Deletable() || v.Class() != "uncovered" {
		t.Fatalf("F2: a record only in the unsealed tail is %q (deletable %v), want uncovered", v.Class(), v.Deletable())
	}
	mustRun(t, w.copyOpts(t, compA))
	_, vs = verdicts(t, w.opts(t))
	if v := vs["payload/"+d]; !v.Deletable() {
		t.Fatalf("F2: after the copy the record is %q, want covered", v.Class())
	}
}

// F3: a receipt with no signature, or of another version, attributes nothing.
func TestF3AReceiptWithoutASignatureAttributesNothing(t *testing.T) {
	w := newWorld(t)
	body := []byte(`{"batch":"unsigned"}`)
	d := hexSum(body)
	w.f.write("sha256/computer-event-payload/"+d, body)
	fields := map[string]any{
		"computer_id": compA, "artifact_digest": d, "media_type": computerevent.ProjectionBatchMediaType,
		"length": len(body), "privacy_class": "owner", "pin_namespace": "computer-event-payload",
		"request_commitment": strings.Repeat("a", 64),
	}
	r, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", fields, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := r.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	w.f.write("sha256/pin-receipts/"+hexSum(raw)+".json", raw)
	rep, vs := verdicts(t, w.opts(t))
	// Round 2 (R3): the rejected receipt still names the file, so the file is
	// refused and the run is unhealthy; it never counts as a copy requirement.
	if v := vs["payload/"+d]; v.Deletable() || v.Class() != "ineligible" {
		t.Fatalf("F3: payload of an unsigned receipt is %q, want ineligible (refused)", v.Class())
	}
	if rep.Healthy() {
		t.Fatal("F3: a rejected receipt left the run healthy")
	}
}

// F5: a store that turns bad after the gate census shows in the delete's own
// report, so the command can exit non-zero.
func TestF5TheDeleteReportCarriesTheFreshPassHealth(t *testing.T) {
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
	b, _ := json.Marshal(rep)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if healthy, ok := m["verify_healthy"].(bool); !ok || healthy {
		t.Fatalf("F5: verify_healthy = %v (present %v) after a corrupt copy: %s", m["verify_healthy"], ok, b)
	}
}

// F6: a legacy file whose size changed since the engine read it is not
// unlinked.
func TestF6AFileThatChangedSizeSinceTheReadIsNotUnlinked(t *testing.T) {
	w := newWorld(t)
	mustRun(t, w.copyOpts(t, compA))
	_, vs := verdicts(t, w.opts(t))
	d := w.payloads[compA][0]
	v := vs["payload/"+d]
	if !v.Deletable() {
		t.Fatalf("setup: %q", v.Class())
	}
	path := filepath.Join(w.f.root, filepath.FromSlash(tapestore.NSPayload.LegacyRel(d)))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("x")
	f.Close()
	if _, err := unlinkCovered(w.f.root, v); err == nil {
		t.Fatal("F6: unlinked a file whose size changed")
	}
	if !w.legacyExists(tapestore.NSPayload, d) {
		t.Fatal("F6: the file is gone")
	}
}

// F7: the no-unlink guard also covers the command.
func TestF7TheCommandNeverUnlinksEither(t *testing.T) {
	entries, err := os.ReadDir("../../cmd/taperepack")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, _ := os.ReadFile(filepath.Join("../../cmd/taperepack", e.Name()))
		for _, b := range bannedUnlinks {
			if callsBanned(string(raw), b) {
				t.Fatalf("F7: cmd/taperepack/%s calls %s", e.Name(), b)
			}
		}
	}
}
