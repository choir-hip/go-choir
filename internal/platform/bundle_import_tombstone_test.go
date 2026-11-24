package platform

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// A tape sweep tombstones a computer under the tombstone gate. A bundle import
// checks the tombstone at its start, but a sweep that tombstones the id after
// that check and before the rows transaction must still refuse the import: the
// rows transaction re-reads the tombstone itself (round-4 review, N3).
func TestBundleImportRefusesATombstoneWrittenAfterItsFirstCheck(t *testing.T) {
	ctx := context.Background()
	const id = "computer-bundle-tombstone"
	src := newDrillComputer(t, id)
	src.appendEvent(computerevent.EventGenesisImported, "", nil)
	src.appendMixed(3, "pre")
	snap := src.snapshot(2)
	src.appendSnapshotRecorded(snap.Root)
	src.appendMixed(2, "post")
	out := filepath.Join(t.TempDir(), "bundle")
	mustExport(t, src, out)

	target := newDrillComputer(t, "computer-bundle-tombstone-target")
	bundle.ImportHooks.AfterPlan = func(string) {
		if err := target.store.writeTapeTombstone(ctx, TapeTombstone{ComputerID: id, Status: tapeTombstoneArchiving, HeadSequence: 1, CanonicalEventHead: platformTestDigest('1'), ManifestRun: "r", ManifestSHA256: platformTestDigest('2'), ProofJSON: "{}"}); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { bundle.ImportHooks.AfterPlan = nil })
	_, err := bundle.Import(ctx, out, target.store.db, target.root, src.trust())
	if !errors.Is(err, bundle.ErrRefused) || !strings.Contains(err.Error(), "tombstone") {
		t.Fatalf("import of a computer tombstoned after the first check = %v, want a tombstone refusal", err)
	}
	var n int
	if err := target.store.db.QueryRow(`SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=?`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("a refused import inserted %d receipts (%v)", n, err)
	}
}
