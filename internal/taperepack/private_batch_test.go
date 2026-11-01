package taperepack

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// The one-off migration of the sealed payloads that the pre-fix host wrote
// as loose files (docs: internal/platform tape_segments_private_test.go). A
// pin receipt for a private envelope cannot say which computer the envelope
// was sealed for, so the engine opens the envelope metadata of the file
// itself.
//
//	M1  A sealed batch envelope, with its receipt, is copied, covered and
//	    deletable; the segment bytes equal the loose bytes.
//	M2  A sealed envelope of another inner media type was ineligible until
//	    2026-11-01; it is now eligible (computerevent.TapeSegmentEligible),
//	    copied, covered and deletable, like M1.
//	M3  Still ineligible and never copied: an envelope sealed for another
//	    computer than its receipt names, and a file that is not an envelope
//	    under a private media type.
func TestSealedBatchEnvelopesAreMigrated(t *testing.T) {
	f := newFixture(t)
	seal := func(computer, event, mediaType, body string) []byte {
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
	const payloadNS = "computer-event-payload"
	batch, _ := f.pin(compA, payloadNS, computerevent.PrivateArtifactMediaType, seal(compA, "ev-1", computerevent.ProjectionBatchMediaType, `{"batch":"1"}`))
	other, _ := f.pin(compA, payloadNS, computerevent.PrivateArtifactMediaType, seal(compA, "ev-2", "application/octet-stream", "ordinary"))
	foreign, _ := f.pin(compA, payloadNS, computerevent.PrivateArtifactMediaType, seal(compB, "ev-3", computerevent.ProjectionBatchMediaType, `{"batch":"3"}`))
	notEnvelope, _ := f.pin(compA, payloadNS, computerevent.PrivateArtifactMediaType, []byte(`{"not":"an envelope"}`))
	// The plaintext batch path is unchanged beside them.
	plain, _ := f.batchPayload(compA, "plain")

	opts := Options{Root: f.root, WorkDir: t.TempDir(), Copy: true, Apply: true, Computers: []string{compA}}
	if _, err := Run(opts); err != nil {
		t.Fatal(err)
	}
	vs := map[string]Verdict{}
	opts = Options{Root: f.root, WorkDir: t.TempDir(), OnVerdict: func(v Verdict) error {
		vs[v.NS().String()+"/"+v.Digest()] = v
		return nil
	}}
	if _, err := Run(opts); err != nil {
		t.Fatal(err)
	}
	rd := tapestore.NewReader(f.root)
	for _, d := range []string{batch, plain, other} {
		v := vs["payload/"+d]
		if !v.Deletable() {
			t.Fatalf("M1: %s is %s, want covered", d, v.Class())
		}
		got, found, err := rd.ReadSegment(compA, tapestore.NSPayload, d)
		loose, _ := os.ReadFile(filepath.Join(f.root, "sha256", payloadNS, d))
		if err != nil || !found || !bytes.Equal(got, loose) {
			t.Fatalf("M1: segment copy of %s: found=%v err=%v", d, found, err)
		}
	}
	for name, d := range map[string]string{"other computer": foreign, "not an envelope": notEnvelope} {
		if v := vs["payload/"+d]; v.Deletable() || v.Class() != "ineligible" {
			t.Fatalf("M2/M3 %s: class %q, want ineligible", name, v.Class())
		}
		for _, c := range []string{compA, compB} {
			if _, found, _ := rd.ReadSegment(c, tapestore.NSPayload, d); found {
				t.Fatalf("M2/M3 %s: copied under %s", name, c)
			}
		}
	}
}

// An envelope the host pinned under a class other than private is not
// inspected on the pin path; since 2026-11-01 it is segmented there like any
// non-public payload (computerevent.TapeSegmentEligible), so the repack
// copies it too, with no envelope check (it was loose before: review
// 2026-10-31, both seats).
func TestOwnerClassEnvelopeIsMigrated(t *testing.T) {
	f := newFixture(t)
	fields := map[string]any{
		"computer_id": compA, "artifact_digest": strings.Repeat("b", 64), "media_type": computerevent.PrivateArtifactMediaType,
		"length": 100, "privacy_class": "owner", "pin_namespace": payloadNamespace,
		"request_commitment": strings.Repeat("a", 64),
	}
	r, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", fields, []computerevent.SigningKey{f.key}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	e, st := classifyReceipt(raw)
	if st != receiptOK || e.private || !e.eligible {
		t.Fatalf("owner-class envelope: status %v private %v eligible %v, want ok, not private, eligible", st, e.private, e.eligible)
	}
}
