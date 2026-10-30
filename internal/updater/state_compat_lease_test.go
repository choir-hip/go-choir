package updater

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

// Epoch 3 (the stream_seq lease,
// docs/problems/stream-seq-high-water-mark-design-2026-10-23.md section 3,
// "Binary rollback across the epoch bump"). Ways it can fail, written before
// the code:
//  1. The epoch-2 release is admitted on an epoch-3 store: the lease is
//     ignored by a binary that seeds by D13's path, and the receipt reverts.
//  2. The epoch-3 release refuses an epoch-2 store: the forward deploy is
//     refused by its own fence (the release rule: min_store_schema_version
//     stays at 2 or below).

func TestApplyRefusesAnEpochTwoReleaseOnAnEpochThreeStore(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	if storeschema.Version < 3 {
		t.Fatalf("storeschema.Version = %d, want at least 3", storeschema.Version)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	basePath, baseDigest := compatTestBase(t, "commit-abc123")
	schemaPath := writeSchemaReceipt(t, t.TempDir(), 3)
	engine, _ := compatEngine(t, root, basePath, schemaPath)

	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-epoch-two", baseDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	request.Manifest.StoreSchemaVersion = 2
	request.Manifest.MinStoreSchemaVersion = 1
	request.Manifest.BaseCommit = "commit-abc123"
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(context.Background(), request); err == nil || !strings.Contains(err.Error(), "newer than release") {
		t.Fatalf("epoch-2 release on an epoch-3 store: err=%v, want a pre-mutation refusal", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "releases", request.Manifest.ContentDigest)); !os.IsNotExist(statErr) {
		t.Fatalf("release dir staged despite refusal")
	}
}

// The release rule: an epoch-3 release declaring a minimum of 2 is not refused
// by the state-compatibility check on an epoch-2 store.
func TestStateCompatibilityAdmitsAnEpochTwoStoreForAnEpochThreeRelease(t *testing.T) {
	manifest := ReleaseManifest{StoreSchemaVersion: 3, MinStoreSchemaVersion: 2}
	engine := &Updater{}
	engine.WithStoreSchemaPath(writeSchemaReceipt(t, t.TempDir(), 2))
	if err := engine.checkStateCompatibility(manifest); err != nil {
		t.Fatalf("an epoch-3 release refused an epoch-2 store: %v", err)
	}
	// And it admits its own epoch.
	engine.WithStoreSchemaPath(writeSchemaReceipt(t, t.TempDir(), 3))
	if err := engine.checkStateCompatibility(manifest); err != nil {
		t.Fatalf("an epoch-3 release refused an epoch-3 store: %v", err)
	}
}
