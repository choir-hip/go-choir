package platform

// Bundle authenticity on import (docs/design/bundle-authenticity-2026-10-31.md
// section 2, the import rows), against PostgreSQL and the production pin
// and append entry points:
//
//  B1  a bundle signed by a key the target does not trust is refused before
//      any file or row is written;
//  B2  an export picks only a snapshot root the tape names, and refuses a
//      named --root the tape does not name;
//  B3  genesis mismatch: a target holding the same computer id with another
//      genesis refuses, and says so;
//  B4  incremental overlap mismatch: a target that forked inside the range
//      (its own event after the shared head) refuses the source's next
//      bundle;
//  B5  the overlap is compared byte for byte over the canonical receipt: a
//      target receipt that names another receipt id refuses, a target
//      receipt stored in another JSON spelling of the same receipt imports.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func countFiles(t *testing.T, root string) int {
	t.Helper()
	n := 0
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func TestBundleImportAuthenticity(t *testing.T) {
	ctx := context.Background()
	const id = "computer-bundle-auth"
	src := newDrillComputer(t, id)
	src.appendEvent(computerevent.EventGenesisImported, "", nil)
	src.appendMixed(3, "pre")
	snap := src.snapshot(2)
	src.appendSnapshotRecorded(snap.Root)
	src.appendMixed(4, "post")
	out := filepath.Join(t.TempDir(), "bundle")
	m := mustExport(t, src, out)
	if m.Genesis == "" || m.SnapshotRoot == nil || m.SnapshotRoot.Root != snap.Root {
		t.Fatalf("manifest genesis %q root %+v", m.Genesis, m.SnapshotRoot)
	}

	// B1: the target trusts only its own key.
	stranger := newDrillComputer(t, id)
	files := countFiles(t, stranger.root)
	_, err := bundle.Import(ctx, out, stranger.store.db, stranger.root, stranger.trust())
	if !errors.Is(err, bundle.ErrRefused) || !errors.Is(err, bundle.ErrUnauthentic) || !errors.Is(err, bundle.ErrUntrustedKey) {
		t.Fatalf("untrusted bundle: import = %v, want an untrusted-key refusal", err)
	}
	t.Logf("untrusted signer: %v", err)
	if head, err := stranger.cas.Head(ctx, id); err != nil || head != nil {
		t.Fatalf("a refused import gave the target a head: %v %v", head, err)
	}
	if got := countFiles(t, stranger.root); got != files || len(liveRoots(t, stranger)) != 0 {
		t.Fatalf("a refused import wrote %d files or a snapshot root", got-files)
	}

	// B2: an unnamed root is never exported.
	unnamed := newDrillComputer(t, "computer-bundle-unnamed")
	unnamed.appendEvent(computerevent.EventGenesisImported, "", nil)
	unnamed.appendMixed(2, "x")
	lost := unnamed.snapshot(1) // a crash before its snapshot_recorded event
	unnamed.appendMixed(2, "y")
	if _, err := bundle.Export(ctx, unnamed.store.db, unnamed.root, bundle.ExportOptions{ComputerID: unnamed.id, Out: filepath.Join(t.TempDir(), "u"), FromSeq: -1}); err == nil {
		t.Fatal("exported a computer whose only snapshot root the tape does not name")
	}
	if _, err := bundle.Export(ctx, unnamed.store.db, unnamed.root, bundle.ExportOptions{ComputerID: unnamed.id, Out: filepath.Join(t.TempDir(), "u2"), FromSeq: -1, Root: lost.Root}); err == nil || !strings.Contains(err.Error(), "not named") {
		t.Fatalf("export of an unnamed --root = %v, want a refusal", err)
	}
	// With a named root after it, the named one is picked.
	named := unnamed.snapshot(1)
	unnamed.appendSnapshotRecorded(named.Root)
	if um, err := bundle.Export(ctx, unnamed.store.db, unnamed.root, bundle.ExportOptions{ComputerID: unnamed.id, Out: filepath.Join(t.TempDir(), "u3"), FromSeq: -1}); err != nil || um.SnapshotRoot.Root != named.Root {
		t.Fatalf("export with a named root = %+v, %v", um.SnapshotRoot, err)
	}

	// B3: the same id, another genesis.
	other := newDrillComputer(t, id)
	other.appendEvent(computerevent.EventGenesisImported, "", nil)
	other.appendMixed(int(m.Tape.HeadSeq)+2, "other")
	before := other.head()
	_, err = bundle.Import(ctx, out, other.store.db, other.root, src.trust())
	if !errors.Is(err, bundle.ErrRefused) || !strings.Contains(err.Error(), "genesis") {
		t.Fatalf("genesis mismatch: import = %v, want a genesis refusal", err)
	}
	t.Logf("genesis mismatch: %v", err)
	if !reflect.DeepEqual(before, other.head()) {
		t.Fatal("a refused import moved the head")
	}

	// B4: a target that forked after the shared head.
	forked := newDrillComputer(t, id)
	if _, err := bundle.Import(ctx, out, forked.store.db, forked.root, src.trust()); err != nil {
		t.Fatalf("import: %v", err)
	}
	forked.appendEvent(computerevent.EventToolInvoked, "", nil) // its own event at head+1
	src.appendMixed(2, "later")
	out2 := filepath.Join(t.TempDir(), "bundle2")
	mustExport(t, src, out2)
	before = forked.head()
	_, err = bundle.Import(ctx, out2, forked.store.db, forked.root, src.trust())
	if !errors.Is(err, bundle.ErrRefused) {
		t.Fatalf("fork inside the range: import = %v, want a refusal", err)
	}
	t.Logf("fork inside the range: %v", err)
	if !reflect.DeepEqual(before, forked.head()) {
		t.Fatal("a refused import moved the head")
	}

	// B5: byte for byte over the canonical receipt.
	dst := newDrillComputer(t, id)
	if _, err := bundle.Import(ctx, out, dst.store.db, dst.root, src.trust()); err != nil {
		t.Fatalf("import: %v", err)
	}
	overlap := m.Tape.HeadSeq - 1
	var stored string
	if err := dst.store.db.QueryRow(`SELECT event_head_receipt_json FROM computer_event_append_receipts WHERE computer_id=? AND sequence=?`, id, overlap).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(stored), &fields); err != nil {
		t.Fatal(err)
	}
	spelled, _ := json.MarshalIndent(fields, "", "  ") // Node B's stored rows are not canonical bytes
	if _, err := dst.store.db.Exec(`UPDATE computer_event_append_receipts SET event_head_receipt_json=? WHERE computer_id=? AND sequence=?`, string(spelled), id, overlap); err != nil {
		t.Fatal(err)
	}
	if res, err := bundle.Import(ctx, out2, dst.store.db, dst.root, src.trust()); err != nil || res.ReceiptsInserted != 2 {
		t.Fatalf("import over a respelled overlap receipt = %+v, %v", res, err)
	}
	if _, err := dst.store.db.Exec(`UPDATE computer_event_append_receipts SET event_head_receipt_id='01a00000-0000-7000-8000-000000000000' WHERE computer_id=? AND sequence=?`, id, overlap); err != nil {
		t.Fatal(err)
	}
	before = dst.head()
	_, err = bundle.Import(ctx, out2, dst.store.db, dst.root, src.trust())
	if !errors.Is(err, bundle.ErrRefused) || !strings.Contains(err.Error(), "not the bundle's") {
		t.Fatalf("overlap receipt mismatch: import = %v, want a refusal", err)
	}
	t.Logf("overlap receipt mismatch: %v", err)
	if !reflect.DeepEqual(before, dst.head()) {
		t.Fatal("a refused import moved the head")
	}
}

// compactAll stores every receipt of c in the compact form
// (computerevent.CompactHeadReceipt, the receipt-trim codec) and returns how
// many it compacted.
func compactAll(t *testing.T, c *drillComputer) int {
	t.Helper()
	n := 0
	for _, r := range drillReceipts(t, c, 0) {
		compact, ok := computerevent.CompactHeadReceipt(computerevent.HeadReceiptRow{
			ComputerID: r.ComputerID, Sequence: uint64(r.Sequence), EventKind: r.EventKind, EventDigest: r.EventDigest,
			RequestCommitment: r.RequestCommitment, EventPinReceiptDigest: r.EventPinReceiptDigest, ReceiptID: r.EventHeadReceiptID,
		}, []byte(r.EventHeadReceiptJSON))
		if !ok {
			continue
		}
		if _, err := c.store.db.Exec(`UPDATE computer_event_append_receipts SET event_head_receipt_json=? WHERE computer_id=? AND sequence=?`, compact, c.id, r.Sequence); err != nil {
			t.Fatal(err)
		}
		n++
	}
	return n
}

// TestBundleCompactReceipts (B6): a source and a target that store compact
// receipts export, verify and overlap-compare over the expanded canonical
// bytes.
func TestBundleCompactReceipts(t *testing.T) {
	ctx := context.Background()
	const id = "computer-bundle-compact"
	src := newDrillComputer(t, id)
	src.appendEvent(computerevent.EventGenesisImported, "", nil)
	src.appendMixed(3, "pre")
	snap := src.snapshot(2)
	src.appendSnapshotRecorded(snap.Root)
	src.appendMixed(4, "post")
	if n := compactAll(t, src); n == 0 {
		t.Fatal("no receipt compacted; the drill does not cover the compact form")
	}
	out := filepath.Join(t.TempDir(), "bundle")
	mustExport(t, src, out)
	if _, err := bundle.Verify(out, src.trust()); err != nil {
		t.Fatalf("verify a bundle exported from compact receipts: %v", err)
	}
	dst := newDrillComputer(t, id)
	if _, err := bundle.Import(ctx, out, dst.store.db, dst.root, src.trust()); err != nil {
		t.Fatalf("import: %v", err)
	}
	compactAll(t, dst)
	src.appendMixed(2, "later")
	compactAll(t, src)
	out2 := filepath.Join(t.TempDir(), "bundle2")
	mustExport(t, src, out2)
	res, err := bundle.Import(ctx, out2, dst.store.db, dst.root, src.trust())
	if err != nil || res.ReceiptsInserted != 2 {
		t.Fatalf("incremental import over a compact overlap = %+v, %v", res, err)
	}
}
