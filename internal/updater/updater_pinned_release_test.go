package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
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

func TestRestagePinnedReleaseSwapsWithoutRestart(t *testing.T) {
	root := filepath.Join(t.TempDir(), "updater")
	if err := os.MkdirAll(filepath.Join(root, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	first := writePinnedFrontendRelease(t, root, "computer-test", "<html>one</html>")
	second := writePinnedFrontendRelease(t, root, "computer-test", "<html>two</html>")
	if err := os.Symlink(filepath.Join(root, "releases", first.ContentDigest), filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if err := RestagePinnedRelease(root, second.ContentDigest); err != nil {
		t.Fatal(err)
	}
	current, err := ReadCurrentManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContentDigest != second.ContentDigest {
		t.Fatalf("current digest = %s, want %s", current.ContentDigest, second.ContentDigest)
	}
	served, err := os.ReadFile(filepath.Join(root, "current", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != "<html>two</html>" {
		t.Fatalf("served SPA = %q", served)
	}
	if err := RestagePinnedRelease(root, strings.Repeat("0", 64)); err == nil {
		t.Fatal("unpinned digest was restaged")
	}
}

func writePinnedFrontendRelease(t *testing.T, root, computerID, spaHTML string) ReleaseManifest {
	t.Helper()
	source := t.TempDir()
	for path, body := range map[string]string{"bin/choir": "choir", "frontend/index.html": spaHTML} {
		full := filepath.Join(source, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := BuildBaselineManifest(source, computerID, "code:test", "artifact-program:test")
	if err != nil {
		t.Fatal(err)
	}
	releaseDir := filepath.Join(root, "releases", manifest.ContentDigest)
	if err := os.MkdirAll(releaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, file := range manifest.Files {
		raw, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(releaseDir, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := computerevent.CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "release-manifest.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	return manifest
}
