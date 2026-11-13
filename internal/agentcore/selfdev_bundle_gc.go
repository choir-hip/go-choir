package agentcore

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// bundleGCGrace is the one time rule: a frozen draft is written before the
// operation that will name it exists, so a bundle no operation names yet is
// left for this long (the freeze to the operation row is seconds).
const bundleGCGrace = 10 * time.Minute

// collectSettledBundles removes the frozen bundles under the updater's
// incoming/ that no open operation names. The updater cannot do this: which
// operation owns a bundle is in this runtime's operation store, not in the
// updater's root (docs/problems/updater-directory-fills-the-data-volume-2026-11-13.md).
// An applied, rejected, rolled back or failed operation is settled; its bundle
// is evidence the reads of it already answer as gone ("the frozen bundle is no
// longer retained on this computer", api_self_development_patch.go).
func (rt *Runtime) collectSettledBundles(ctx context.Context) {
	if rt == nil || rt.selfdevOperations == nil || rt.selfdevComputerID == "" || rt.selfdevUpdaterRoot == "" {
		return
	}
	open, err := rt.selfdevOperations.ListByStates(ctx, rt.selfdevComputerID, selfdev.OpenStates...)
	if err != nil {
		return
	}
	named := map[string]bool{}
	for _, operation := range open {
		if operation.BundleDigest != "" {
			named[operation.BundleDigest] = true
		}
	}
	removed, bytes := removeUnnamedBundles(filepath.Join(rt.selfdevUpdaterRoot, "incoming"), named, time.Now(), bundleGCGrace)
	if removed > 0 {
		log.Printf("agentcore: bundle gc removed=%d bytes_freed=%d", removed, bytes)
	}
}

// removeUnnamedBundles removes each incoming/<digest> directory that carries a
// bundle (bundle.json or bundle.draft.json), is not in named, and holds nothing
// newer than grace. It returns the count and the apparent bytes removed.
func removeUnnamedBundles(incoming string, named map[string]bool, now time.Time, grace time.Duration) (int, int64) {
	entries, err := os.ReadDir(incoming)
	if err != nil {
		return 0, 0
	}
	removed, freed := 0, int64(0)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !computerevent.IsSHA256(name) || named[name] {
			continue
		}
		dir := filepath.Join(incoming, name)
		if !fileExists(filepath.Join(dir, "bundle.json")) && !fileExists(filepath.Join(dir, "bundle.draft.json")) {
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
					size += info.Size()
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
