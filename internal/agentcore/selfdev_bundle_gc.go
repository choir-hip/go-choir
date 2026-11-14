package agentcore

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// bundleGCGrace is the one time rule: a frozen draft is written before the
// operation that will name it exists, so a bundle no operation names yet is
// left for this long (the freeze to the operation row is seconds).
const bundleGCGrace = 10 * time.Minute

// collectSettledBundles removes the frozen bundles and host builds under the
// updater's incoming/ that nothing can still read. The updater cannot do this:
// which operation owns one is in this runtime's operation store, not in the
// updater's root (docs/problems/updater-directory-fills-the-data-volume-2026-11-13.md).
//
// A bundle is kept while an open operation names it, and while it is the bundle
// of the operation behind a release that is still in releases/ (the current
// release, its predecessor, a reachable pre-migration release, and the applied
// operation under a rolled-back release: appliedOperationForRelease is the one
// resolver). The readers of incoming/<bundle digest> that need that:
// sourceProjectionInput and boundSourcePatch (the platform-update gate through
// unmigratedPackageStackRefusal, migratePackageStack, the cast gate),
// computeEffectiveTree and the effective release, runCleanRebase and
// cleanRebaseBeforeNotice, offerRebasePatch, draftPackageRelease. The other
// readers (the patch, screens, media and witness APIs, desk shots, consensus,
// engineer lineage) read the bundle of an operation a person asks about and
// already answer "no longer retained on this computer" (api_self_development_patch.go).
// A host build is kept for the same operations: the shot pipeline reuses it
// for its operation's lifetime.
func (rt *Runtime) collectSettledBundles(ctx context.Context) {
	// One computer id throughout: the resolver below reads rt.cfg.ComputerID.
	if rt == nil || rt.selfdevOperations == nil || rt.cfg.ComputerID == "" || rt.selfdevUpdaterRoot == "" {
		return
	}
	open, err := rt.selfdevOperations.ListByStates(ctx, rt.cfg.ComputerID, selfdev.OpenStates...)
	if err != nil {
		return
	}
	named := map[string]bool{}
	for _, operation := range open {
		if operation.BundleDigest != "" {
			named[operation.BundleDigest] = true
		}
	}
	releases, err := os.ReadDir(filepath.Join(rt.selfdevUpdaterRoot, "releases"))
	if err != nil {
		// What the surviving releases need is unknown: collect nothing.
		return
	}
	for _, entry := range releases {
		if !entry.IsDir() || !computerevent.IsSHA256(entry.Name()) {
			continue
		}
		operation, ok, err := rt.resolveOperationForRelease(ctx, entry.Name())
		if err != nil {
			// What a surviving release needs cannot be read: collect nothing.
			log.Printf("agentcore: bundle gc skipped: the operation behind release %s cannot be resolved: %v", entry.Name()[:12], err)
			return
		}
		if ok && operation.BundleDigest != "" {
			named[operation.BundleDigest] = true
		}
	}
	removed, bytes := removeUnnamedBundles(filepath.Join(rt.selfdevUpdaterRoot, "incoming"), named, time.Now(), bundleGCGrace)
	if removed > 0 {
		log.Printf("agentcore: bundle gc removed=%d bytes_freed=%d", removed, bytes)
	}
}

// removeUnnamedBundles removes each incoming/<digest> directory that carries a
// bundle (bundle.json or bundle.draft.json) and each incoming/host-build-*
// directory whose host-build.json names a bundle, when that bundle is not in
// named and nothing in it is newer than grace. A host build whose record cannot
// be read is judged by age alone. It returns the count and the allocated bytes.
func removeUnnamedBundles(incoming string, named map[string]bool, now time.Time, grace time.Duration) (int, int64) {
	entries, err := os.ReadDir(incoming)
	if err != nil {
		return 0, 0
	}
	removed, freed := 0, int64(0)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(incoming, name)
		switch {
		case computerevent.IsSHA256(name):
			if named[name] {
				continue
			}
			if !fileExists(filepath.Join(dir, "bundle.json")) && !fileExists(filepath.Join(dir, "bundle.draft.json")) {
				continue
			}
		case strings.HasPrefix(name, "host-build-"):
			if raw, err := os.ReadFile(filepath.Join(dir, "host-build.json")); err == nil {
				var record struct {
					BundleDigest string `json:"bundle_digest"`
				}
				if json.Unmarshal(raw, &record) == nil && record.BundleDigest != "" && named[record.BundleDigest] {
					continue
				}
			}
		default:
			continue
		}
		var size int64
		var newest time.Time
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if info, err := d.Info(); err == nil {
				if info.ModTime().After(newest) {
					newest = info.ModTime()
				}
				if info.Mode().IsRegular() {
					if stat, ok := info.Sys().(*syscall.Stat_t); ok {
						size += int64(stat.Blocks) * 512
					} else {
						size += info.Size()
					}
				}
			}
			return nil
		})
		if now.Sub(newest) < grace {
			continue
		}
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("agentcore: bundle gc %s: %v", name, err)
			continue
		}
		removed++
		freed += size
	}
	return removed, freed
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
