package taperepack

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// Failure modes, written before the code (station FP9, L2-04; the repack of
// docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md section 6.1
// steps 2 to 4, copy only):
//
//	R1  plan writes, moves or touches anything under the artifacts root.
//	R2  a receipt whose file name is not the SHA-256 of its bytes is trusted.
//	R3  a receipt whose artifact file is absent lands in the manifest.
//	R4  an artifact whose bytes do not hash to its name lands in the manifest.
//	R5  an ineligible pin (an event of another kind, a payload of another
//	    media type) lands in the manifest.
//	R6  another computer's pin lands in the manifest; plan with no computer
//	    named runs over every computer.
//	R7  an event whose computer differs from its receipt's lands in the
//	    manifest.
//	R8  copy writes for a computer that was not named, or runs with none.
//	R9  copy writes, moves or deletes a legacy file.
//	R10 a second copy writes again.
//	R11 the census counts a record equal from a legacy file (fallback on).
//	R12 the census passes over a corrupt segment record.
//	R13 the census on a restored segment tree differs from the live one.
//	R14 a legacy file that no longer hashes is counted equal.
//	R15 a malformed manifest line is skipped instead of refused.
//	R16 an empty manifest passes the census.
//	R17 copied records are not readable by the production reader, or the
//	    segment scan fails.
//	R18 a legacy file gone between plan and copy is skipped silently.

const (
	compA = "computer-a"
	compB = "computer-b"
)

type fixture struct {
	t    *testing.T
	root string
	key  computerevent.SigningKey
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, root: t.TempDir(), key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform", KeyID: "test"}, PrivateKey: priv}}
}

func hexSum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func (f *fixture) write(rel string, body []byte) {
	f.t.Helper()
	path := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o640); err != nil {
		f.t.Fatal(err)
	}
}

// pin writes a legacy artifact and its receipt the way the legacy path did
// and returns (artifact digest, receipt digest).
func (f *fixture) pin(computer, namespace, mediaType string, body []byte) (string, string) {
	f.t.Helper()
	digest := hexSum(body)
	f.write("sha256/"+namespace+"/"+digest, body)
	return digest, f.receipt(computer, namespace, mediaType, digest, len(body))
}

func (f *fixture) receipt(computer, namespace, mediaType, digest string, length int) string {
	f.t.Helper()
	fields := map[string]any{
		"computer_id": computer, "artifact_digest": digest, "media_type": mediaType,
		"length": length, "privacy_class": "owner", "pin_namespace": namespace,
		"request_commitment": strings.Repeat("a", 64),
	}
	r, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", fields, []computerevent.SigningKey{f.key}, time.Now())
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := r.CanonicalBytes()
	if err != nil {
		f.t.Fatal(err)
	}
	rd := hexSum(raw)
	f.write("sha256/pin-receipts/"+rd+".json", raw)
	return rd
}

func eventBody(computer, kind, nonce string) []byte {
	return []byte(`{"computer_id":"` + computer + `","event_kind":"` + kind + `","nonce":"` + nonce + `"}`)
}

func (f *fixture) batchEvent(computer, nonce string) (string, string) {
	return f.pin(computer, "computer-event", "application/vnd.choir.computer-event+json", eventBody(computer, string(computerevent.EventProjectionBatchRecorded), nonce))
}

func (f *fixture) batchPayload(computer, nonce string) (string, string) {
	return f.pin(computer, "computer-event-payload", computerevent.ProjectionBatchMediaType, []byte(`{"batch":"`+nonce+`"}`))
}

// tree records every file under root with its size, mode and mtime.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		sum := ""
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum = hexSum(b)
		}
		out[rel] = info.Mode().String() + " " + info.ModTime().String() + " " + sum
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func subtree(all map[string]string, prefix string) map[string]string {
	out := map[string]string{}
	for k, v := range all {
		if k == prefix || strings.HasPrefix(k, prefix+string(filepath.Separator)) {
			out[k] = v
		}
	}
	return out
}

func sameTree(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("%s: %d entries before, %d after", label, len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("%s: %s changed: %q -> %q", label, k, v, after[k])
		}
	}
}

// standard builds two computers' worth of legacy pins plus every kind of
// bad input, and returns the eligible digests of computer A.
func (f *fixture) standard() (eligible []string) {
	for _, n := range []string{"1", "2", "3"} {
		d, _ := f.batchEvent(compA, n)
		p, _ := f.batchPayload(compA, n)
		eligible = append(eligible, d, p)
	}
	// Computer B: eligible, but not named (R6).
	f.batchEvent(compB, "b1")
	f.batchPayload(compB, "b1")
	// Ineligible kinds (R5).
	f.pin(compA, "computer-event", "application/vnd.choir.computer-event+json", eventBody(compA, "run_started", "x"))
	f.pin(compA, "computer-event-payload", "application/json", []byte(`{"other":1}`))
	// Dangling receipt (R3).
	f.receipt(compA, "computer-event-payload", computerevent.ProjectionBatchMediaType, strings.Repeat("b", 64), 10)
	// Artifact that does not hash to its name (R4).
	bad := []byte(`{"batch":"bad"}`)
	badDigest := hexSum(bad)
	f.write("sha256/computer-event-payload/"+badDigest, []byte(`{"batch":"BAD"}`))
	f.receipt(compA, "computer-event-payload", computerevent.ProjectionBatchMediaType, badDigest, len(bad))
	// Receipt under a wrong name (R2).
	f.write("sha256/pin-receipts/"+strings.Repeat("c", 64)+".json", []byte(`{"receipt_kind":"PinReceipt"}`))
	// Event whose computer differs from its receipt's (R7).
	f.pin(compA, "computer-event", "application/vnd.choir.computer-event+json", eventBody(compB, string(computerevent.EventProjectionBatchRecorded), "mixed"))
	sort.Strings(eligible)
	return eligible
}

func entryDigests(entries []Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Digest)
	}
	sort.Strings(out)
	return out
}

func TestPlanSelectsOnlyEligibleVerifiedPinsAndWritesNothing(t *testing.T) {
	f := newFixture(t)
	want := f.standard()
	before := tree(t, f.root)
	res, err := Plan(PlanOptions{Root: f.root, Computers: []string{compA}})
	if err != nil {
		t.Fatal(err)
	}
	sameTree(t, "R1 plan", before, tree(t, f.root))
	if got := entryDigests(res.Entries); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("manifest digests:\n got %v\nwant %v", got, want)
	}
	for _, e := range res.Entries {
		if e.Computer != compA || !tapestore.ValidDigest(e.ReceiptDigest) || e.Length <= 0 || e.ReceiptLength <= 0 {
			t.Fatalf("bad entry %+v", e)
		}
	}
	c := res.Counts
	if c.ReceiptNameMismatch != 1 {
		t.Fatalf("R2 receipt name mismatch = %d, want 1", c.ReceiptNameMismatch)
	}
	if c.Dangling != 1 {
		t.Fatalf("R3 dangling = %d, want 1", c.Dangling)
	}
	if c.ArtifactMismatch != 1 {
		t.Fatalf("R4 artifact mismatch = %d, want 1", c.ArtifactMismatch)
	}
	if c.Ineligible != 2 {
		t.Fatalf("R5 ineligible = %d, want 2", c.Ineligible)
	}
	if c.OtherComputer != 2 {
		t.Fatalf("R6 other computer = %d, want 2", c.OtherComputer)
	}
	if c.ScopeMismatch != 1 {
		t.Fatalf("R7 scope mismatch = %d, want 1", c.ScopeMismatch)
	}
	if c.Eligible != len(want) || c.PerComputer[compA] != len(want) {
		t.Fatalf("eligible counts %+v", c)
	}
	if _, err := Plan(PlanOptions{Root: f.root}); err == nil {
		t.Fatal("R6 plan with no computer named ran")
	}
}

func TestPlanCountsUnreceiptedArtifacts(t *testing.T) {
	f := newFixture(t)
	f.batchEvent(compA, "1")
	f.write("sha256/computer-event-payload/"+hexSum([]byte("orphan")), []byte("orphan"))
	res, err := Plan(PlanOptions{Root: f.root, Computers: []string{compA}, CountUnreceipted: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts.Unreceipted != 1 {
		t.Fatalf("unreceipted = %d, want 1", res.Counts.Unreceipted)
	}
}

func TestManifestRoundTripAndRefusals(t *testing.T) {
	f := newFixture(t)
	f.standard()
	res, err := Plan(PlanOptions{Root: f.root, Computers: []string{compA}})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteManifest(&buf, res.Entries); err != nil {
		t.Fatal(err)
	}
	got, err := ReadManifest(bytes.NewReader(buf.Bytes()))
	if err != nil || len(got) != len(res.Entries) {
		t.Fatalf("round trip: %d entries, %v", len(got), err)
	}
	for _, bad := range []string{
		`{"computer":"computer-a","ns":"event","digest":"zz","length":1,"receipt_digest":"` + strings.Repeat("a", 64) + `","receipt_length":1}`,
		`{"computer":"computer-a","ns":"receipt","digest":"` + strings.Repeat("a", 64) + `","length":1,"receipt_digest":"` + strings.Repeat("a", 64) + `","receipt_length":1}`,
		`{"computer":"../x","ns":"event","digest":"` + strings.Repeat("a", 64) + `","length":1,"receipt_digest":"` + strings.Repeat("a", 64) + `","receipt_length":1}`,
		`{"computer":"computer-a","ns":"event","digest":"` + strings.Repeat("a", 64) + `","length":1,"receipt_digest":"` + strings.Repeat("a", 64) + `","receipt_length":1,"extra":1}`,
		`not json`,
	} {
		if _, err := ReadManifest(strings.NewReader(buf.String() + bad + "\n")); err == nil {
			t.Fatalf("R15 malformed line accepted: %s", bad)
		}
	}
}

func planA(t *testing.T, f *fixture) []Entry {
	t.Helper()
	res, err := Plan(PlanOptions{Root: f.root, Computers: []string{compA}})
	if err != nil {
		t.Fatal(err)
	}
	return res.Entries
}

func TestCopyIsCopyOnlyIdempotentAndReadable(t *testing.T) {
	f := newFixture(t)
	f.standard()
	entries := planA(t, f)
	// Add a computer B line to the manifest: copy must not write it (R8).
	bEvent, bReceipt := f.batchEvent(compB, "b2")
	entries = append(entries, Entry{Computer: compB, NS: "event", Digest: bEvent, Length: 1, ReceiptDigest: bReceipt, ReceiptLength: 1})

	if _, err := Copy(CopyOptions{Root: f.root, Entries: entries}); err == nil {
		t.Fatal("R8 copy with no computer named ran")
	}
	legacyBefore := subtree(tree(t, f.root), "sha256")
	counts, err := Copy(CopyOptions{Root: f.root, Entries: entries, Computers: []string{compA}})
	if err != nil {
		t.Fatal(err)
	}
	sameTree(t, "R9 copy", legacyBefore, subtree(tree(t, f.root), "sha256"))
	if counts.Pins != 6 || counts.RecordsWritten != 12 || counts.SkippedOtherComputer != 1 || counts.Verified != 12 {
		t.Fatalf("copy counts %+v", counts)
	}
	if _, err := os.Stat(filepath.Join(f.root, tapestore.SegDirName, compB)); !os.IsNotExist(err) {
		t.Fatal("R8 a segment directory for computer B exists")
	}
	again, err := Copy(CopyOptions{Root: f.root, Entries: entries, Computers: []string{compA}})
	if err != nil {
		t.Fatal(err)
	}
	if again.RecordsWritten != 0 || again.AlreadyInSegment != 12 {
		t.Fatalf("R10 second copy: %+v", again)
	}
	// R17: the production reader serves every record from a segment, and
	// the whole-segment audit passes.
	rd := tapestore.NewReader(f.root)
	for _, e := range entries[:len(entries)-1] {
		ns := tapestore.NSEvent
		if e.NS == "payload" {
			ns = tapestore.NSPayload
		}
		if _, src, err := rd.Read(compA, ns, e.Digest); err != nil || src != tapestore.SourceSegment {
			t.Fatalf("R17 read %s: src=%v err=%v", e.Digest, src, err)
		}
		if _, src, err := rd.Read(compA, tapestore.NSReceipt, e.ReceiptDigest); err != nil || src != tapestore.SourceSegment {
			t.Fatalf("R17 read receipt %s: src=%v err=%v", e.ReceiptDigest, src, err)
		}
	}
	n := 0
	if err := rd.Scan(compA, func(tapestore.Record) error { n++; return nil }); err != nil || n != 12 {
		t.Fatalf("R17 scan: %d records, %v", n, err)
	}
}

func TestCopyRefusesAVanishedLegacyFile(t *testing.T) {
	f := newFixture(t)
	f.batchEvent(compA, "1")
	d, _ := f.batchPayload(compA, "2")
	entries := planA(t, f)
	if err := os.Remove(filepath.Join(f.root, "sha256", "computer-event-payload", d)); err != nil {
		t.Fatal(err)
	}
	if _, err := Copy(CopyOptions{Root: f.root, Entries: entries, Computers: []string{compA}}); err == nil {
		t.Fatal("R18 copy passed over a vanished legacy file")
	}
}

func TestCensus(t *testing.T) {
	f := newFixture(t)
	f.standard()
	entries := planA(t, f)

	// R11: before the copy every legacy file exists, and the census still
	// finds nothing equal, because it reads segments only.
	c, err := Census(CensusOptions{SegmentRoot: f.root, Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	if c.OK() || c.Missing != 12 || c.Equal != 0 {
		t.Fatalf("R11 census before copy: %+v", c)
	}
	if _, err := Copy(CopyOptions{Root: f.root, Entries: entries, Computers: []string{compA}}); err != nil {
		t.Fatal(err)
	}
	c, err = Census(CensusOptions{SegmentRoot: f.root, Entries: entries})
	if err != nil || !c.OK() || c.Equal != 12 {
		t.Fatalf("census after copy: %+v %v", c, err)
	}

	// R13: a restored copy of tape-seg alone, legacy compared from live.
	restored := t.TempDir()
	copyTree(t, filepath.Join(f.root, tapestore.SegDirName), filepath.Join(restored, tapestore.SegDirName))
	c, err = Census(CensusOptions{SegmentRoot: restored, LegacyRoot: f.root, Entries: entries})
	if err != nil || !c.OK() || c.Equal != 12 {
		t.Fatalf("R13 restored census: %+v %v", c, err)
	}

	// R14: a legacy file that no longer hashes is counted, not equal.
	victim := filepath.Join(f.root, "sha256", "computer-event", entries[0].Digest)
	if entries[0].NS == "payload" {
		victim = filepath.Join(f.root, "sha256", "computer-event-payload", entries[0].Digest)
	}
	orig, _ := os.ReadFile(victim)
	if err := os.WriteFile(victim, append(bytes.Clone(orig), ' '), 0o640); err != nil {
		t.Fatal(err)
	}
	c, err = Census(CensusOptions{SegmentRoot: restored, LegacyRoot: f.root, Entries: entries})
	if err != nil || c.OK() || c.LegacyBad != 1 || c.Equal != 11 {
		t.Fatalf("R14 census with a bad legacy file: %+v %v", c, err)
	}
	if err := os.WriteFile(victim, orig, 0o640); err != nil {
		t.Fatal(err)
	}

	// R12: a flipped byte in a record body of the restored segment.
	segDir := filepath.Join(restored, tapestore.SegDirName, compA)
	segs, _ := filepath.Glob(filepath.Join(segDir, "*.seg"))
	if len(segs) == 0 {
		t.Fatal("no segment files")
	}
	raw, _ := os.ReadFile(segs[0])
	raw[len(raw)-2] ^= 0xff
	if err := os.WriteFile(segs[0], raw, 0o640); err != nil {
		t.Fatal(err)
	}
	c, err = Census(CensusOptions{SegmentRoot: restored, LegacyRoot: f.root, Entries: entries})
	// An open segment's damaged last record may read as a torn tail
	// (missing) rather than corrupt; either way it is not equal.
	if err != nil || c.OK() || c.Equal == 12 || c.Corrupt+c.Missing == 0 {
		t.Fatalf("R12 census over a corrupt segment: %+v %v", c, err)
	}

	// R16: an empty manifest proves nothing.
	c, err = Census(CensusOptions{SegmentRoot: f.root, Entries: nil})
	if err != nil || c.OK() {
		t.Fatalf("R16 empty census: %+v %v", c, err)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o640)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The manifest is JSON lines with these exact fields; FP0's drill reads it.
func TestManifestFieldNames(t *testing.T) {
	var buf bytes.Buffer
	e := Entry{Computer: compA, NS: "payload", Digest: strings.Repeat("a", 64), Length: 3, ReceiptDigest: strings.Repeat("b", 64), ReceiptLength: 4}
	if err := WriteManifest(&buf, []Entry{e}); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"computer", "ns", "digest", "length", "receipt_digest", "receipt_length"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("manifest line lacks %s: %s", k, buf.String())
		}
	}
}

// Repair round 2 (bunch 31 review, low): copy refuses a store a writer is
// using. It tries the per-computer segment lock the production appender
// takes (internal/tapestore, lockDir) without blocking before each batch,
// and refuses, writing nothing, if a writer holds it. Plan and census only
// read and take no lock.
func TestCopyRefusesWhileAWriterHoldsTheSegmentLock(t *testing.T) {
	f := newFixture(t)
	f.batchEvent(compA, "1")
	entries := planA(t, f)
	dir := filepath.Join(f.root, tapestore.SegDirName, compA)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	d, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := syscall.Flock(int(d.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Copy(CopyOptions{Root: f.root, Entries: entries, Computers: []string{compA}})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "live") {
			t.Fatalf("copy under a held writer lock: err = %v, want a live-store refusal", err)
		}
	case <-time.After(3 * time.Second):
		_ = syscall.Flock(int(d.Fd()), syscall.LOCK_UN)
		<-done
		t.Fatal("copy waited for the writer instead of refusing")
	}
	if names, _ := os.ReadDir(dir); len(names) != 0 {
		t.Fatalf("copy wrote under a held lock: %v", names)
	}
	_ = syscall.Flock(int(d.Fd()), syscall.LOCK_UN)
	if _, err := Copy(CopyOptions{Root: f.root, Entries: entries, Computers: []string{compA}}); err != nil {
		t.Fatalf("copy after the writer released: %v", err)
	}
}
