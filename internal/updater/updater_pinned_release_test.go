package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestUpdaterAppliesRetainedPinnedReleaseForExplicitRollback(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	engine, err := New(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{}, testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}})
	if err != nil {
		t.Fatal(err)
	}
	initial := updaterRequestFixture(t, root, "computer-test", "realization-test", "initial", "initial-idem", "retained release")
	if _, err := engine.Apply(context.Background(), initial); err != nil {
		t.Fatal(err)
	}

	rollback := initial
	rollback.OperationID = "rollback"
	rollback.IdempotencyKey = "rollback-idem"
	rollback.AcceptedEventHead = strings.Repeat("b", 64)
	rollback.SourceDir = filepath.Join(root, "releases", initial.Manifest.ContentDigest)
	rollback.Manifest.AcceptedEventHead = rollback.AcceptedEventHead
	rollback.Manifest.ContentDigest = ""
	rollback.Manifest, err = FinalizeManifest(rollback.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	rollback.RequestCommitment, err = ComputeApplyRequestCommitment(rollback)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Apply(context.Background(), rollback)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "applied" || result.ReleaseDigest != rollback.Manifest.ContentDigest || result.PriorReleaseDigest != initial.Manifest.ContentDigest {
		t.Fatalf("rollback result = %+v", result)
	}
}
