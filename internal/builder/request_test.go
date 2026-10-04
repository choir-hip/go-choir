package builder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBaseManifest(t *testing.T) {
	manifest := "contract=choir-guest-image-v1\n" +
		"build_commit=abc123\n" +
		"autoputer=/nix/store/0wa00l7cpv9vxrmmr0zi85xccg34cmyi-autoputer-0.1.0\n" +
		"updater=/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-updater-0.1.0\n" +
		"kernel=/nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-linux-6.12\n"
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest")
	if err := os.WriteFile(p, []byte(manifest), 0o444); err != nil {
		t.Fatal(err)
	}
	commit, paths, digest, err := parseBaseManifest(p)
	if err != nil {
		t.Fatalf("parseBaseManifest: %v", err)
	}
	if commit != "abc123" {
		t.Errorf("commit=%q want abc123", commit)
	}
	if len(paths) != 3 {
		t.Fatalf("paths=%v want 3 store paths", paths)
	}
	if digest == "" {
		t.Error("digest empty")
	}
	// Second parse of same bytes yields the same digest.
	_, _, digest2, _ := parseBaseManifest(p)
	if digest != digest2 {
		t.Errorf("digest unstable: %q != %q", digest, digest2)
	}
}

func TestStorePathRe(t *testing.T) {
	good := []string{
		"/nix/store/0wa00l7cpv9vxrmmr0zi85xccg34cmyi-autoputer-0.1.0",
		"/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-x",
	}
	bad := []string{
		"/nix/store/short-x",
		"/tmp/foo",
		"/nix/store/0wa00l7cpv9vxrmmr0zi85xccg34cmyi-autoputer-0.1.0/",
		"relative/path",
	}
	for _, p := range good {
		if !storePathRe.MatchString(p) {
			t.Errorf("storePathRe should match %q", p)
		}
	}
	for _, p := range bad {
		if storePathRe.MatchString(p) {
			t.Errorf("storePathRe should not match %q", p)
		}
	}
}
