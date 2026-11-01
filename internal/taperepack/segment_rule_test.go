package taperepack

// The repack copies and deletes by the pin path's own rule,
// computerevent.TapeSegmentEligible (scope widened 2026-11-01: every event,
// every payload but a public non-batch one). These cases pin the rule at the
// receipt and carry it through copy, census and delete:
//
//	S1  classifyReceipt's eligibility is the shared rule, plus the id and
//	    length bounds.
//	S2  A public non-batch payload is ineligible: never copied, never
//	    deleted, nor its receipt.
//	S3  An owner-class payload and a private non-batch envelope of the same
//	    computer are copied, covered and deleted, with their receipts.
//	S4  A private envelope sealed for another computer than its receipt
//	    names is never copied under either and never deleted.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

func sealFor(t *testing.T, computer, event, mediaType, body string) []byte {
	t.Helper()
	cipher, err := computerevent.NewPrivateArtifactCipher(computer, bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	env, _, err := cipher.Encrypt(context.Background(), computer, event, mediaType, "private", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// S1
func TestS1ReceiptEligibilityIsThePinRule(t *testing.T) {
	f := newFixture(t)
	const capsule = "application/vnd.choir.capsule-effect+json"
	cases := []struct {
		name, computer, ns, media, class string
		length                           int
		want                             bool
	}{
		{"event of any kind", compA, eventNamespace, "application/vnd.choir.computer-event+json", "private", 10, true},
		{"public projection batch", compA, payloadNamespace, computerevent.ProjectionBatchMediaType, "public", 10, true},
		{"public non-batch (capsule bundle)", compA, payloadNamespace, capsule, "public", 10, false},
		{"owner non-batch", compA, payloadNamespace, "application/json", "owner", 10, true},
		{"owner envelope", compA, payloadNamespace, computerevent.PrivateArtifactMediaType, "owner", 10, true},
		{"private envelope", compA, payloadNamespace, computerevent.PrivateArtifactMediaType, "private", 10, true},
		{"private, not the envelope type", compA, payloadNamespace, "application/json", "private", 10, false},
		{"id that cannot name a segment", "computer/with/slashes", payloadNamespace, "application/json", "owner", 10, false},
		{"over a record's bound", compA, payloadNamespace, "application/json", "owner", tapestore.MaxRecordBytes + 1, false},
	}
	for _, c := range cases {
		raw := f.receiptRawClass(c.computer, c.ns, c.media, c.class, strings.Repeat("c", 64), c.length)
		e, st := classifyReceipt(raw)
		if st != receiptOK || e.eligible != c.want {
			t.Fatalf("%s: status %v eligible %v, want ok and %v", c.name, st, e.eligible, c.want)
		}
		if c.computer == compA && c.length <= tapestore.MaxRecordBytes && e.eligible != computerevent.TapeSegmentEligible(c.ns, c.media, c.class) {
			t.Fatalf("%s: the repack and the pin rule disagree", c.name)
		}
	}
}

// S2
func TestS2PublicNonBatchPayloadIsNeverCopiedOrDeleted(t *testing.T) {
	w := newWorld(t)
	public, publicReceipt := w.f.pinClass(compA, payloadNamespace, "application/vnd.choir.capsule-effect+json", "public", []byte(`{"bundle":"s2"}`))
	workDir, receipt := w.copied(t, compA, compB)
	rd := tapestore.NewReader(w.f.root)
	for _, c := range []string{compA, compB} {
		if _, found, _ := rd.ReadSegment(c, tapestore.NSPayload, public); found {
			t.Fatalf("S2: a public non-batch payload was copied under %s", c)
		}
	}
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	rep, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if !w.legacyExists(tapestore.NSPayload, public) || !w.legacyExists(tapestore.NSReceipt, publicReceipt) {
		t.Fatal("S2: a public non-batch payload or its receipt was deleted")
	}
	if rep.Refused["ineligible"] != 2 || rep.Deleted != int64(w.eligibleA)+4 {
		t.Fatalf("S2: %+v", rep)
	}
}

// S3
func TestS3OwnerAndSealedNonBatchPayloadsAreCopiedCoveredAndDeleted(t *testing.T) {
	w := newWorld(t)
	owner, ownerReceipt := w.f.pinClass(compA, payloadNamespace, "application/json", "owner", []byte(`{"owner":"s3"}`))
	sealed, sealedReceipt := w.f.pin(compA, payloadNamespace, computerevent.PrivateArtifactMediaType, sealFor(t, compA, "ev-s3", "application/octet-stream", "ordinary"))
	loose := map[string][]byte{}
	for _, d := range []string{owner, sealed} {
		b, err := os.ReadFile(filepath.Join(w.f.root, filepath.FromSlash(tapestore.NSPayload.LegacyRel(d))))
		if err != nil {
			t.Fatal(err)
		}
		loose[d] = b
	}
	workDir, receipt := w.copied(t, compA, compB)
	rd := tapestore.NewReader(w.f.root)
	for d, want := range loose {
		got, found, err := rd.ReadSegment(compA, tapestore.NSPayload, d)
		if err != nil || !found || !bytes.Equal(got, want) {
			t.Fatalf("S3: segment copy of %s: found=%v err=%v", d, found, err)
		}
	}
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	rep, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	for d, ns := range map[string]tapestore.NS{owner: tapestore.NSPayload, sealed: tapestore.NSPayload, ownerReceipt: tapestore.NSReceipt, sealedReceipt: tapestore.NSReceipt} {
		if w.legacyExists(ns, d) {
			t.Fatalf("S3: covered %s %s still exists", ns, d)
		}
	}
	if rep.Deleted != int64(w.eligibleA)+4+4 || rep.Refused["ineligible"] != 0 {
		t.Fatalf("S3: %+v", rep)
	}
	for d, want := range loose {
		got, _, err := rd.Read(compA, tapestore.NSPayload, d)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("S3: %s does not read back from the segment after the delete: %v", d, err)
		}
	}
}

// S4
func TestS4EnvelopeNamingAnotherComputerIsNeverCopiedOrDeleted(t *testing.T) {
	w := newWorld(t)
	foreign, _ := w.f.pin(compA, payloadNamespace, computerevent.PrivateArtifactMediaType, sealFor(t, compB, "ev-s4", "application/octet-stream", "for b"))
	workDir, receipt := w.copied(t, compA, compB)
	rd := tapestore.NewReader(w.f.root)
	for _, c := range []string{compA, compB} {
		if _, found, _ := rd.ReadSegment(c, tapestore.NSPayload, foreign); found {
			t.Fatalf("S4: an envelope sealed for B under A's receipt was copied under %s", c)
		}
	}
	o := w.delOpts(workDir, receipt)
	o.Apply = true
	rep, err := Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if !w.legacyExists(tapestore.NSPayload, foreign) {
		t.Fatal("S4: the foreign envelope was deleted")
	}
	if rep.Refused["ineligible"] < 1 {
		t.Fatalf("S4: %+v", rep)
	}
}
