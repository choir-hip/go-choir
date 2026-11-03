package modelpolicy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The platform default replaces an older default by rename, never in place,
// so the capsule overlay's files snapshot link keeps the old bytes
// (docs/design/capsule-overlay-computer-state-2026-11-03.md, S1).
func TestDefaultPolicyRewriteReplacesByRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old := []byte("# an older platform default\n")
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "snapshot-link")
	if err := os.Link(path, link); err != nil {
		t.Skipf("hardlink unsupported: %v", err)
	}
	if err := ensureDefaultPolicyFile(path, provideriface.Config{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(link); string(got) != string(old) {
		t.Fatalf("the link saw the rewrite: %q", got)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) == string(old) {
		t.Fatalf("the default was not rewritten: %v", err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
}
