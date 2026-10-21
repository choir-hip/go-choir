package platform

// Residual r1 of choir-tape-segments-a2-repack-2026-10-21.md: a pin that is
// answered by an existing segment record (reuse) is acknowledged on bytes
// whose durability this pin did not prove; the record may sit in a tail a
// poisoned writer never synced, which the next writer sealed. So a reuse
// that finds no legacy copy writes one, synced, before it acknowledges:
//
//  RL1 the second pin of the same bytes appends nothing, writes the legacy
//      copy once and counts it; a third pin writes nothing more;
//  RL2 after that, a corrupt segment record is served from the legacy copy;
//  RL3 a legacy copy that cannot be written fails the pin (nothing is
//      acknowledged on the unproven record alone), and appends nothing.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

func pinReuseBatch(f *writeFixture, id string, payload []byte) error {
	_, err := f.artifacts.pinPayload(context.Background(), id, payload, computerevent.ProjectionBatchMediaType, "public", platformTestDigest('c'))
	return err
}

// RL1, RL2
func TestSegmentReuseWritesTheLegacyCopyOnce(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-rl1"
	payload := projectionPayload("pinned twice")
	digest := computerevent.DigestBytes(payload)
	if err := pinReuseBatch(f, id, payload); err != nil {
		t.Fatal(err)
	}
	if got := f.service.TapeDiagnostics().ReuseLegacyWrites; got != 0 {
		t.Fatalf("reuse_legacy_writes after the first pin = %d, want 0", got)
	}
	if err := pinReuseBatch(f, id, payload); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(f.root, "sha256", tapeNSPayload, digest)
	if got, err := os.ReadFile(legacy); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("legacy copy after the reuse: %v", err)
	}
	first := f.service.TapeDiagnostics().ReuseLegacyWrites
	if first == 0 {
		t.Fatal("reuse_legacy_writes did not count the legacy copy")
	}
	if err := pinReuseBatch(f, id, payload); err != nil {
		t.Fatal(err)
	}
	if got := f.service.TapeDiagnostics().ReuseLegacyWrites; got != first {
		t.Fatalf("a third pin wrote %d more legacy copies, want none", got-first)
	}
	if n := segmentRecords(t, f.root, id)[segItem{tapestore.NSPayload, digest}]; n != 1 {
		t.Fatalf("payload records = %d, want exactly 1", n)
	}

	corruptSegments(t, f.root, id) // the first record is the payload
	if got, err := f.artifacts.FetchPayload(id, digest); err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("corrupt reused record after r1 = %v, want the legacy copy", err)
	}
}

// RL3
func TestSegmentReuseFailsWhenTheLegacyCopyCannotBeWritten(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	f := newWriteFixture(t, "write", true)
	const id = "computer-rl3"
	payload := projectionPayload("pinned twice, legacy unwritable")
	if err := pinReuseBatch(f, id, payload); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.root, "sha256", tapeNSPayload)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
	segFile := filepath.Join(f.root, tapestore.SegDirName, id, tapestore.SegmentFileName(1))
	before, _ := os.ReadFile(segFile)
	if err := pinReuseBatch(f, id, payload); err == nil {
		t.Fatal("a reuse whose legacy copy could not be written was acknowledged")
	}
	after, _ := os.ReadFile(segFile)
	if !bytes.Equal(before, after) {
		t.Fatal("the refused reuse changed the segment")
	}
}
