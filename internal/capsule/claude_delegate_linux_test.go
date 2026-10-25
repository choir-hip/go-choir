//go:build linux

package capsule

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docs/design/claude-delegate-slice1-2026-10-14.md F6: a claude-mode
// capsule's candidate must never carry the delegate token (the repo is
// public). Pinned: a changed file holding the token refuses; a symlink to a
// file holding it is not followed; a clean change passes; a claude-mode
// capsule with no token in memory refuses rather than freeze unchecked.
func TestFreezeRefusesACandidateCarryingTheDelegateToken(t *testing.T) {
	upper := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(upper, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("workspace/platform/internal/x/clean.go", "package x\n")
	write("workspace/platform/internal/x/leak.go", "package x\n// "+testDelegateToken+"\n")
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte(testDelegateToken), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(upper, "workspace/platform/internal/x/link")); err != nil {
		t.Fatal(err)
	}
	e := &Executor{}
	caps := &Capsule{ID: "capsule-claude", UpperDir: upper, claudeToken: []byte(testDelegateToken)}
	ctx := context.Background()
	clean := []FileChange{{Path: "workspace/platform/internal/x/clean.go", Kind: ChangeModified}, {Path: "workspace/platform/internal/x/link", Kind: ChangeAdded}}
	if err := e.refuseClaudeTokenInChanges(ctx, caps, clean); err != nil {
		t.Fatalf("clean change refused: %v", err)
	}
	// An intermediate symlink is not followed either (bunch223 review F3):
	// the scan reads only what lies beneath the upper.
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "tok.go"), []byte(testDelegateToken), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(upper, "workspace/platform/internal/dirlink")); err != nil {
		t.Fatal(err)
	}
	through := append(clean, FileChange{Path: "workspace/platform/internal/dirlink/tok.go", Kind: ChangeAdded})
	if err := e.refuseClaudeTokenInChanges(ctx, caps, through); err != nil {
		t.Fatalf("the scan followed an intermediate symlink out of the upper: %v", err)
	}
	leak := append(clean, FileChange{Path: "/workspace/platform/internal/x/leak.go", Kind: ChangeAdded})
	err := e.refuseClaudeTokenInChanges(ctx, caps, leak)
	if err == nil || !strings.Contains(err.Error(), "leak.go") || strings.Contains(err.Error(), testDelegateToken) {
		t.Fatalf("leaking change: %v", err)
	}
	if err := e.refuseClaudeTokenInChanges(ctx, &Capsule{ID: "no-token", UpperDir: upper}, clean); err == nil {
		t.Fatal("a claude-mode capsule without its token froze unchecked")
	}
}
