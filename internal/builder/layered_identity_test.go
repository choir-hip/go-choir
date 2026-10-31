package builder

// Layered guest image, design step 2 (docs/design/layered-guest-image-
// 2026-10-31.md 1.9). The builder's base identity used to be the sha256 of the
// whole 3.4 GB storedisk.erofs, read in full per build. Failure modes:
//
//   - a single-image base changes identity or stops being hashed in full;
//   - a layered base is read in full anyway (the point is not to);
//   - the layered identity forgets the app layer (it changes every deploy) or
//     the base (a swapped base must change the identity);
//   - a base path that does not sit in a <base_id> directory is trusted
//     without being hashed.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

const testBaseID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func sum(b string) string {
	h := sha256.Sum256([]byte(b))
	return hex.EncodeToString(h[:])
}

func TestBaseDiskIdentitySingleImageHashesTheWholeFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "storedisk.erofs")
	if err := os.WriteFile(p, []byte("single"), 0o444); err != nil {
		t.Fatal(err)
	}
	app, baseID, baseSum, err := baseDiskIdentity(p, "")
	if err != nil || app != sum("single") || baseID != "" || baseSum != "" {
		t.Fatalf("single identity = %q %q %q %v; want the file sha256 and no base legs", app, baseID, baseSum, err)
	}
}

func TestBaseDiskIdentityLayeredNeverReadsTheBase(t *testing.T) {
	dir := t.TempDir()
	appPath := filepath.Join(dir, "storedisk-app.erofs")
	if err := os.WriteFile(appPath, []byte("app-1"), 0o444); err != nil {
		t.Fatal(err)
	}
	// The base file is not even readable (a directory would fail os.Open+Read):
	// the identity comes from its <base_id> directory, so no byte is read.
	baseDir := filepath.Join(dir, "guest-bases", testBaseID)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	basePath := filepath.Join(baseDir, "storedisk-base.erofs")
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		t.Fatal(err)
	}
	app, baseID, baseSum, err := baseDiskIdentity(appPath, basePath)
	if err != nil || app != sum("app-1") || baseID != testBaseID || baseSum != "" {
		t.Fatalf("layered identity = %q %q %q %v; want app sha256 and the base id, no base hash", app, baseID, baseSum, err)
	}
	// The app layer changes every deploy: the identity follows it.
	if err := os.WriteFile(appPath+".2", []byte("app-2"), 0o444); err != nil {
		t.Fatal(err)
	}
	app2, _, _, _ := baseDiskIdentity(appPath+".2", basePath)
	if app2 == app {
		t.Fatal("a different app layer gave the same identity")
	}
	// A different base id gives a different identity.
	other := filepath.Join(dir, "guest-bases", "cccccccccccccccccccccccccccccccc")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	_, baseID2, _, _ := baseDiskIdentity(appPath, filepath.Join(other, "storedisk-base.erofs"))
	if baseID2 == baseID {
		t.Fatal("a different base gave the same identity")
	}
}

func TestBaseDiskIdentityBaseOutsideABaseDirIsHashed(t *testing.T) {
	dir := t.TempDir()
	appPath := filepath.Join(dir, "app.erofs")
	basePath := filepath.Join(dir, "base.erofs")
	for p, c := range map[string]string{appPath: "a", basePath: "b"} {
		if err := os.WriteFile(p, []byte(c), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	_, baseID, baseSum, err := baseDiskIdentity(appPath, basePath)
	if err != nil || baseID != "" || baseSum != sum("b") {
		t.Fatalf("identity = id %q sum %q %v; want the base hashed when it has no id directory", baseID, baseSum, err)
	}
	if _, _, _, err := baseDiskIdentity(appPath, filepath.Join(dir, "missing.erofs")); err == nil {
		t.Fatal("an unreadable base outside an id directory was accepted")
	}
	if _, _, _, err := baseDiskIdentity(filepath.Join(dir, "missing-app"), basePath); err == nil {
		t.Fatal("an unreadable app layer was accepted")
	}
}
