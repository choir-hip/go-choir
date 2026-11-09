//go:build linux

package capsule

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// sweepSourceCacheBuilds removes crashed .build-* trees; run once at executor
// init, before any spawn.
func sweepSourceCacheBuilds(stateDir string) {
	cacheRoot := filepath.Join(stateDir, sourceCacheDirName)
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".build-") {
			_ = removePrivateTree(filepath.Join(cacheRoot, entry.Name()))
		}
	}
}

// pinnedSourceLower returns the shared source lower for source, building it
// on a miss. The caller holds e.mu, so builds and evictions do not race.
func (e *Executor) pinnedSourceLower(ctx context.Context, source SourcePreflight, timer *spawnPhaseTimer) (root, key string, hit bool, err error) {
	if strings.TrimSpace(source.SubjectDigest) == "" {
		return "", "", false, fmt.Errorf("capsule spawn requires a subject digest to pin the source")
	}
	cacheRoot := filepath.Join(e.stateDir, sourceCacheDirName)
	// The cache root and the key directories are 0755 and root-owned. The
	// source tree under workspace/platform keeps makeSubjectTreeReadOnly's
	// directory mode (0777): overlayfs checks the lower parent directory with
	// the capsule's mapped uid before copy-up, so tightening it breaks writes
	// to the source. That is safe because the lower is only reachable as an
	// overlay layer: a write lands in the capsule's upper, and the cache path
	// is not mounted into any capsule's namespace.
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return "", "", false, fmt.Errorf("capsule source cache: %w", err)
	}
	_ = os.Chmod(cacheRoot, 0o755)
	key = sourceCacheKeyFor(source.SubjectDigest)
	final := filepath.Join(cacheRoot, key)
	if info, statErr := os.Lstat(filepath.Join(final, "workspace", "platform")); statErr == nil && info.IsDir() {
		now := time.Now()
		_ = os.Chtimes(final, now, now)
		timer.phase("source_cache_hit")
		return final, key, true, nil
	}
	// A miss. Any .build-* or half-named tree here is a crashed build (spawns
	// are serialized under e.mu): clear it.
	if entries, readErr := os.ReadDir(cacheRoot); readErr == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".build-") || entry.Name() == key {
				_ = removePrivateTree(filepath.Join(cacheRoot, entry.Name()))
			}
		}
	}
	tmp, err := os.MkdirTemp(cacheRoot, ".build-")
	if err != nil {
		return "", "", false, fmt.Errorf("capsule source cache build: %w", err)
	}
	built := false
	defer func() {
		if !built {
			_ = removePrivateTree(tmp)
		}
	}()
	_ = os.Chmod(tmp, 0o755)
	if err := os.MkdirAll(filepath.Join(tmp, "workspace"), 0o755); err != nil {
		return "", "", false, err
	}
	_ = os.Chmod(filepath.Join(tmp, "workspace"), 0o755)
	target := filepath.Join(tmp, "workspace", "platform")
	if err := e.copyPinnedSource(ctx, source, target, timer); err != nil {
		return "", "", false, err
	}
	if err := requireFrozenComputerSurfaceSource(target); err != nil {
		return "", "", false, err
	}
	if err := makeSubjectTreeReadOnly(target); err != nil {
		return "", "", false, err
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", "", false, fmt.Errorf("capsule source cache publish: %w", err)
	}
	built = true
	timer.phase("source_cache_built")
	e.evictSourceCacheLocked(cacheRoot, key)
	return final, key, false, nil
}

// copyPinnedSource copies the durable binding's source into target and checks
// it against the subject digest, as every spawn did before the cache.
func (e *Executor) copyPinnedSource(ctx context.Context, source SourcePreflight, target string, timer *spawnPhaseTimer) error {
	var sourceDigest string
	var err error
	if strings.HasPrefix(source.ArtifactRef, subjectArtifactPrefix) {
		artifactRoot, artifactDigest, resolveErr := e.subjectArtifactPath(source.ArtifactRef)
		if resolveErr != nil || source.SubjectDigest == "" || artifactDigest != source.SubjectDigest {
			return fmt.Errorf("capsule spawn requires exact candidate source artifact and subject digest")
		}
		if err := copyCanonicalSubjectTree(ctx, artifactRoot, target); err != nil {
			return fmt.Errorf("capsule pin candidate source: %w", err)
		}
		timer.phase("source_candidate_copy")
		sourceDigest, err = digestCanonicalSubjectTree(ctx, target)
		timer.phase("source_digest_copy")
	} else {
		const prefix = "capsule-source-git:"
		raw := strings.TrimPrefix(source.ArtifactRef, prefix)
		separator := strings.Index(raw, ":sha256:")
		if !strings.HasPrefix(source.ArtifactRef, prefix) || separator <= 0 || raw[separator+len(":sha256:"):] != source.SubjectDigest {
			return fmt.Errorf("capsule spawn requires exact preflight git source and subject digest")
		}
		commit := raw[:separator]
		decoded, decodeErr := hex.DecodeString(commit)
		if decodeErr != nil || (len(decoded) != 20 && len(decoded) != 32) {
			return fmt.Errorf("capsule spawn preflight git commit is invalid")
		}
		// The copy is checked against the digest the binding pins, not
		// against a second read of the commit.
		sourceDigest, err = copyImmutableCommitTreeTimed(ctx, e.sourceDir, commit, target, source.SubjectDigest, timer)
	}
	return spawnSourceRefusal(err, source.SubjectDigest, sourceDigest)
}

// evictSourceCacheLocked keeps the sourceCacheKeep most recently used trees
// and any a live capsule mounts; the rest are removed. Best effort.
func (e *Executor) evictSourceCacheLocked(cacheRoot, justBuilt string) {
	inUse := map[string]bool{justBuilt: true}
	for _, caps := range e.capsules {
		if caps != nil && caps.sourceCacheKey != "" {
			inUse[caps.sourceCacheKey] = true
		}
	}
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		return
	}
	type tree struct {
		name string
		mod  time.Time
	}
	var trees []tree
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		trees = append(trees, tree{entry.Name(), info.ModTime()})
	}
	sort.Slice(trees, func(i, j int) bool { return trees[i].mod.After(trees[j].mod) })
	for i, t := range trees {
		if i < sourceCacheKeep || inUse[t.name] {
			continue
		}
		_ = removePrivateTree(filepath.Join(cacheRoot, t.name))
	}
}
