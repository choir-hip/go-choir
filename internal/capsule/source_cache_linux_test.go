//go:build linux

package capsule

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Failure modes of the shared source lower (owner, 2026-11-09, near-instant
// startup):
//  1. a hit copies or walks the tree again (the 741 ms the cache removes);
//  2. a crashed build leaves a half tree that a later spawn mounts;
//  3. eviction removes a tree a live capsule mounts, or keeps more than N;
//  4. the readers of the pinned base bytes look in the per-capsule
//     source-lower and find nothing.
func TestSourceCacheHitDoesNotTouchTheSourceAndStaleBuildsAreCleared(t *testing.T) {
	e := &Executor{stateDir: t.TempDir()} // no source dir: any copy would fail
	digest := "sha256:abc"
	key := sourceCacheKeyFor(digest)
	final := filepath.Join(e.stateDir, sourceCacheDirName, key)
	if err := os.MkdirAll(filepath.Join(final, "workspace", "platform"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, gotKey, hit, err := e.pinnedSourceLower(context.Background(), SourcePreflight{SubjectDigest: digest}, nil)
	if err != nil || !hit || root != final || gotKey != key {
		t.Fatalf("hit = %v root=%q key=%q err=%v", hit, root, gotKey, err)
	}

	// A miss clears a crashed build and a half-named tree before building;
	// the build itself fails here (no source), and leaves no tree of its own.
	other := "sha256:def"
	otherKey := sourceCacheKeyFor(other)
	cacheRoot := filepath.Join(e.stateDir, sourceCacheDirName)
	stale := filepath.Join(cacheRoot, ".build-crashed")
	half := filepath.Join(cacheRoot, otherKey)
	for _, d := range []string{stale, half} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, err := e.pinnedSourceLower(context.Background(), SourcePreflight{SubjectDigest: other, ArtifactRef: "capsule-source-git:zz:sha256:def"}, nil); err == nil {
		t.Fatal("a build with no source succeeded")
	}
	for _, d := range []string{stale, half} {
		if _, err := os.Lstat(d); err == nil {
			t.Fatalf("%s survived a miss", d)
		}
	}
	entries, _ := os.ReadDir(cacheRoot)
	for _, entry := range entries {
		if entry.Name() != key {
			t.Fatalf("a failed build left %s", entry.Name())
		}
	}
}

func TestSourceCacheEvictsOldestExceptInUse(t *testing.T) {
	e := &Executor{stateDir: t.TempDir(), capsules: map[string]*Capsule{}}
	cacheRoot := filepath.Join(e.stateDir, sourceCacheDirName)
	now := time.Now()
	var names []string
	for i := 0; i < 7; i++ {
		name := string(rune('a' + i))
		names = append(names, name)
		dir := filepath.Join(cacheRoot, name, "workspace", "platform")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		mod := now.Add(time.Duration(i-10) * time.Minute) // a is oldest, g newest
		if err := os.Chtimes(filepath.Join(cacheRoot, name), mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	e.capsules["live"] = &Capsule{sourceCacheKey: "a"}
	e.evictSourceCacheLocked(cacheRoot, "g")
	want := map[string]bool{"g": true, "f": true, "e": true, "d": true, "a": true}
	for _, name := range names {
		_, err := os.Lstat(filepath.Join(cacheRoot, name))
		if (err == nil) != want[name] {
			t.Errorf("tree %s present=%v, want %v", name, err == nil, want[name])
		}
	}
}

func TestPinnedSourceLowerRootFollowsTheCapsulesReference(t *testing.T) {
	base := t.TempDir()
	merged := filepath.Join(base, "root")
	if got := pinnedSourceLowerRoot(merged); got != filepath.Join(base, "source-lower") {
		t.Fatalf("no reference: %q", got)
	}
	cache := t.TempDir()
	if err := writeSourceLowerRef(base, cache); err != nil {
		t.Fatal(err)
	}
	if got := pinnedSourceLowerRoot(merged); got != cache {
		t.Fatalf("reference: %q, want %q", got, cache)
	}
}

// Freeze reads the pinned base through the capsule's reference (B1): a file
// restored to its base bytes is dropped from the screen rules, and the base's
// directories are seen, though the capsule has no per-capsule source-lower.
func TestFreezeBaseReadsGoThroughTheSourceLowerRef(t *testing.T) {
	base := t.TempDir()
	merged := filepath.Join(base, "root")
	cache := t.TempDir()
	write := func(root, rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(cache, "workspace/platform/frontend/a.css", "body{}")
	write(merged, "workspace/platform/frontend/a.css", "body{}")
	if err := writeSourceLowerRef(base, cache); err != nil {
		t.Fatal(err)
	}
	lowerRoot := pinnedSourceLowerRoot(merged)
	changes := []FileChange{{Path: "/workspace/platform/frontend/a.css", Kind: ChangeModified, Mode: 0o644}}
	if got := dropRestoredToBase(changes, merged, lowerRoot); len(got) != 0 {
		t.Fatalf("a file restored to the cached base was kept: %v", got)
	}
	if _, isDir, err := readSourceBeneath(lowerRoot, platformSourcePrefix+"/frontend"); err != nil || !isDir {
		t.Fatalf("baseHasDir through the ref: isDir=%v err=%v", isDir, err)
	}
}
