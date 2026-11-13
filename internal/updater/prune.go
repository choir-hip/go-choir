package updater

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The updater's root grows with every platform update and every host build:
// before this prune nothing ever deleted from incoming/, releases/, gc-roots/
// or store/ (docs/problems/updater-directory-fills-the-data-volume-2026-11-13.md).
//
// What keeps each directory alive:
//
//   - releases/<digest> is read by `current`, by the rollback of an apply (the
//     journal's PriorReleaseTarget), by RestagePinnedRelease (tape restore,
//     base restore, package migration) and by trustedSourceDir (a pinned
//     release is a trusted source). Nothing but those reads it.
//   - gc-roots/<digest>/<base> are symlinks to store/<base>: the closure of the
//     layered release <digest>, written by materializeReleaseClosure.
//   - store/<base> is a store object. It is live while some surviving release's
//     gc-root dir names it. The guest wrapper overlays store/ over /nix/store,
//     so a surviving layered release's entrypoint must be rooted.
//   - incoming/ holds staged apply sources: platform payloads under
//     incoming/<release digest> (copied into releases/<digest> by stageRelease,
//     which verifies them), host builds under incoming/host-build-<job>,
//     temporaries under incoming/.*, and self-development bundles under
//     incoming/<bundle digest> (bundle.json), which are operation evidence the
//     updater cannot judge.
//
// The prune removes, and only removes:
//
//   - incoming: a payload whose release is staged, a host build, a dot
//     temporary, each older than pruneGrace. Bundles and unrecognised entries
//     stay.
//   - releases: every release but the current one, the predecessor its apply
//     recorded, and the pruneRecentReleases most recently used others; never
//     one younger than pruneGrace (a baseline staged before its swap).
//   - gc-roots of a release that no longer exists, store objects no surviving
//     release roots (older than pruneGrace; a candidate build in use is young).
//
// It runs under the updater's mutex, so never inside an Apply, and skips when
// any apply is unsettled or any apply key is held (UnsettledApply), which is
// where an engineering capsule's or a self-development apply's inputs are kept
// alive. A release is renamed out of releases/ before it is deleted, so a
// crash leaves either the release or an invisible leftover the next run
// removes; `current` and its predecessor are never touched.
const (
	pruneGrace          = 6 * time.Hour
	pruneRecentReleases = 3
)

// PruneReport is what one prune removed.
type PruneReport struct {
	IncomingRemoved int
	ReleasesRemoved int
	GCRootsRemoved  int
	StoreRemoved    int
	BytesFreed      int64
	// Skipped says why nothing was pruned.
	Skipped string
}

// Prune removes what no apply, rollback or restore can still need. It waits
// for an apply in flight and is safe to run at any time, repeatedly.
func (u *Updater) Prune() PruneReport {
	u.mu.Lock()
	defer u.mu.Unlock()
	report := u.prune()
	if report.Skipped != "" {
		log.Printf("updater: prune skipped: %s", report.Skipped)
	} else {
		log.Printf("updater: prune incoming=%d releases=%d gc_roots=%d store=%d bytes_freed=%d",
			report.IncomingRemoved, report.ReleasesRemoved, report.GCRootsRemoved, report.StoreRemoved, report.BytesFreed)
	}
	return report
}

// WaitPrune waits for the prunes a verified apply started.
func (u *Updater) WaitPrune() { u.pruneWG.Wait() }

// pruneAfterApply runs a prune once the apply that called it has returned and
// released the updater.
func (u *Updater) pruneAfterApply() {
	u.pruneWG.Add(1)
	go func() {
		defer u.pruneWG.Done()
		u.Prune()
	}()
}

func (u *Updater) prune() PruneReport {
	var report PruneReport
	if name, unsettled := UnsettledApply(u.root); unsettled {
		report.Skipped = "an apply is unsettled or in flight (" + name + ")"
		return report
	}
	cutoff := u.now().Add(-pruneGrace)
	u.pruneIncoming(cutoff, &report)
	u.pruneReleases(cutoff, &report)
	return report
}

func (u *Updater) pruneIncoming(cutoff time.Time, report *PruneReport) {
	incoming := filepath.Join(u.root, "incoming")
	entries, err := os.ReadDir(incoming)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(incoming, name)
		switch {
		case strings.HasPrefix(name, "."), strings.HasPrefix(name, "host-build-"):
		case computerevent.IsSHA256(name):
			// A bundle is operation evidence; a payload is removable only once
			// its release is staged (stageRelease verified and copied it).
			if pathExists(filepath.Join(path, "bundle.json")) || !pathExists(filepath.Join(u.root, "releases", name, "release-manifest.json")) {
				continue
			}
		default:
			continue
		}
		bytes, newest := treeStat(path)
		if newest.After(cutoff) {
			continue
		}
		if err := removeTree(path); err != nil {
			log.Printf("updater: prune incoming %s: %v", name, err)
			continue
		}
		report.IncomingRemoved++
		report.BytesFreed += bytes
	}
}

func (u *Updater) pruneReleases(cutoff time.Time, report *PruneReport) {
	releasesDir := filepath.Join(u.root, "releases")
	// Finish a crashed prune and sweep stage temporaries first: both are
	// invisible to every reader.
	if entries, err := os.ReadDir(releasesDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || !(strings.HasPrefix(name, ".prune-") || strings.HasPrefix(name, ".stage-")) {
				continue
			}
			path := filepath.Join(releasesDir, name)
			bytes, newest := treeStat(path)
			if strings.HasPrefix(name, ".stage-") && newest.After(cutoff) {
				continue
			}
			if err := removeTree(path); err != nil {
				log.Printf("updater: prune releases %s: %v", name, err)
				continue
			}
			report.BytesFreed += bytes
			if strings.HasPrefix(name, ".prune-") {
				report.ReleasesRemoved++
			}
		}
	}
	current, _, err := currentReleaseOf(u.root)
	if err != nil || current == "" {
		// Without a readable current release nothing proves which releases a
		// rollback needs.
		return
	}
	keep := u.releasesToKeep(current)
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !computerevent.IsSHA256(name) || keep[name] {
			continue
		}
		path := filepath.Join(releasesDir, name)
		info, err := os.Lstat(path)
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		bytes, _ := treeStat(path)
		hidden := filepath.Join(releasesDir, ".prune-"+name)
		if err := os.Rename(path, hidden); err != nil {
			log.Printf("updater: prune release %s: %v", short(name), err)
			continue
		}
		_ = syncDir(releasesDir)
		if err := removeTree(hidden); err != nil {
			// The leftover is invisible and the next run removes it.
			log.Printf("updater: prune release %s: %v", short(name), err)
			continue
		}
		report.ReleasesRemoved++
		report.BytesFreed += bytes
	}
	u.pruneGCRootsAndStore(cutoff, report)
}

// releasesToKeep is the current release, the predecessor the journal of the
// apply that installed it recorded, and the most recently used others.
func (u *Updater) releasesToKeep(current string) map[string]bool {
	lastUse := map[string]time.Time{}
	touch := func(digest string, at time.Time) {
		if computerevent.IsSHA256(digest) && at.After(lastUse[digest]) {
			lastUse[digest] = at
		}
	}
	keep := map[string]bool{current: true}
	var installedAt time.Time
	dir := operationsDir(u.root)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			journal, found, err := readJournal(filepath.Join(dir, entry.Name()))
			if err != nil || !found {
				continue
			}
			touch(journal.TargetReleaseDigest, journal.StartedAt)
			touch(journal.PriorReleaseDigest, journal.StartedAt)
			if journal.TargetReleaseDigest == current && journal.PriorReleaseDigest != "" && !journal.StartedAt.Before(installedAt) {
				installedAt = journal.StartedAt
				if computerevent.IsSHA256(journal.PriorReleaseDigest) {
					keep[journal.PriorReleaseDigest] = true
				}
			}
		}
	}
	releasesDir := filepath.Join(u.root, "releases")
	var others []string
	if entries, err := os.ReadDir(releasesDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || !computerevent.IsSHA256(name) || keep[name] {
				continue
			}
			if _, ok := lastUse[name]; !ok {
				if info, err := entry.Info(); err == nil {
					lastUse[name] = info.ModTime()
				}
			}
			others = append(others, name)
		}
	}
	sort.Slice(others, func(i, j int) bool {
		if !lastUse[others[i]].Equal(lastUse[others[j]]) {
			return lastUse[others[i]].After(lastUse[others[j]])
		}
		return others[i] < others[j]
	})
	for i, name := range others {
		if i < pruneRecentReleases {
			keep[name] = true
		}
	}
	return keep
}

// pruneGCRootsAndStore removes the gc-roots of releases that are gone and the
// store objects no surviving release roots.
func (u *Updater) pruneGCRootsAndStore(cutoff time.Time, report *PruneReport) {
	surviving := map[string]ReleaseManifest{}
	manifestsReadable := true
	if entries, err := os.ReadDir(filepath.Join(u.root, "releases")); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || !computerevent.IsSHA256(name) {
				continue
			}
			manifest, err := readReleaseManifest(filepath.Join(u.root, "releases", name))
			if err != nil {
				manifestsReadable = false
			}
			surviving[name] = manifest
		}
	} else {
		return
	}
	gcRoots := filepath.Join(u.root, "gc-roots")
	live := map[string]bool{}
	rooted := map[string]bool{}
	if entries, err := os.ReadDir(gcRoots); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(gcRoots, name)
			if _, ok := surviving[name]; !ok {
				if err := removeTree(path); err != nil {
					log.Printf("updater: prune gc-roots %s: %v", short(name), err)
					continue
				}
				report.GCRootsRemoved++
				continue
			}
			links, err := os.ReadDir(path)
			if err != nil {
				manifestsReadable = false
				continue
			}
			for _, link := range links {
				base := link.Name()
				if target, err := os.Readlink(filepath.Join(path, base)); err == nil {
					base = filepath.Base(target)
				}
				live[base] = true
				rooted[name] = true
			}
		}
	}
	// A surviving layered release must be rooted, and its entrypoint's store
	// path must be among the roots, or the collection could take the binary
	// the computer boots.
	storeSafe := manifestsReadable
	for digest, manifest := range surviving {
		if manifest.ClosureDigest == "" && manifest.LayeringEntrypoint == "" {
			continue
		}
		if !rooted[digest] {
			storeSafe = false
		}
		if manifest.LayeringEntrypoint != "" {
			first := strings.SplitN(filepath.ToSlash(manifest.LayeringEntrypoint), "/", 2)[0]
			if !live[first] {
				storeSafe = false
			}
		}
	}
	if !storeSafe {
		log.Printf("updater: prune store skipped: a surviving release's roots are incomplete")
		return
	}
	storeDir := filepath.Join(u.root, "store")
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(name, ".staging-") {
			continue
		}
		if live[name] {
			continue
		}
		path := filepath.Join(storeDir, name)
		info, err := os.Lstat(path)
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		// An overlay upper dir records deletions as device nodes; they are
		// not store objects.
		if !info.IsDir() && !info.Mode().IsRegular() && info.Mode()&fs.ModeSymlink == 0 {
			continue
		}
		bytes, newest := treeStat(path)
		if newest.After(cutoff) {
			continue
		}
		if err := removeTree(path); err != nil {
			log.Printf("updater: prune store %s: %v", name, err)
			continue
		}
		report.StoreRemoved++
		report.BytesFreed += bytes
	}
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// treeStat is the apparent size of the regular files under path and the newest
// modification time of anything in it. Symlinks are not followed.
func treeStat(path string) (bytes int64, newest time.Time) {
	_ = filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		if info.Mode().IsRegular() {
			bytes += info.Size()
		}
		return nil
	})
	return bytes, newest
}

// removeTree deletes a tree whose directories may be sealed read-only.
func removeTree(path string) error {
	if info, err := os.Lstat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	} else if !info.IsDir() {
		return os.Remove(path)
	}
	_ = filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
		if entry != nil && entry.IsDir() {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	return os.RemoveAll(path)
}
