package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDropRestoredToBaseIsNotAChange(t *testing.T) {
	lower := t.TempDir()
	merged := t.TempDir()
	write := func(root, rel, body string, mode os.FileMode) {
		p := filepath.Join(root, "workspace/platform", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
	}
	write(lower, "frontend/src/Same.svelte", "base", 0o644)
	write(merged, "frontend/src/Same.svelte", "base", 0o644) // restored
	write(lower, "frontend/src/Edited.svelte", "base", 0o644)
	write(merged, "frontend/src/Edited.svelte", "edited", 0o644)
	write(lower, "frontend/src/Mode.sh", "base", 0o644)
	write(merged, "frontend/src/Mode.sh", "base", 0o755)
	write(merged, "frontend/src/New.svelte", "new", 0o644)
	changes := []FileChange{
		{Path: "/workspace/platform/frontend/src/Same.svelte", Kind: ChangeModified, Mode: 0o644},
		{Path: "/workspace/platform/frontend/src/Edited.svelte", Kind: ChangeModified, Mode: 0o644},
		{Path: "/workspace/platform/frontend/src/Mode.sh", Kind: ChangeModified, Mode: 0o755},
		{Path: "/workspace/platform/frontend/src/New.svelte", Kind: ChangeAdded, Mode: 0o644},
		{Path: "/workspace/platform/frontend/src/Gone.svelte", Kind: ChangeDeleted},
	}
	got := dropRestoredToBase(changes, merged, lower)
	if len(got) != 4 {
		t.Fatalf("got %v, want only the restored file dropped", got)
	}
	for _, c := range got {
		if strings.HasSuffix(c.Path, "Same.svelte") {
			t.Fatalf("restored file still a change: %v", got)
		}
	}
	// A restored file with a newer mtime must not make a before shot late.
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	mods := map[string]time.Time{
		"/workspace/platform/frontend/src/Same.svelte":   t0.Add(time.Hour), // restored last
		"/workspace/platform/frontend/src/Edited.svelte": t0,
		"/workspace/platform/frontend/src/Mode.sh":       t0,
		"/workspace/platform/frontend/src/New.svelte":    t0,
	}
	_, newest, path, err := sourceChangeTimes(got, func(p string) (time.Time, error) { return mods[p], nil })
	if err != nil || !newest.Equal(t0) || strings.Contains(path, "Same") {
		t.Fatalf("newest %v (%s) err %v, want %v and not the restored file", newest, path, err, t0)
	}
	if only := dropRestoredToBase(changes[:1], merged, lower); len(only) != 0 {
		t.Fatalf("a lone restored file left %v", only)
	}
}
