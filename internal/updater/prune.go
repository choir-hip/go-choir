package updater

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The updater's root grew with every platform update and every host build:
// nothing deleted from incoming/, releases/, gc-roots/ or store/
// (docs/problems/updater-directory-fills-the-data-volume-2026-11-13.md).
// Owner ruling 2026-11-13: every computer will look like this one, so the
// directory is bounded, not conservatively pruned. After a settled apply the
// root holds the current release, the one rollback predecessor its apply
// recorded, the store objects those two root, and inputs of an operation that
// is in flight. Nothing else, however many updates have been applied.
//
// What reads each directory, and so what keeps an entry:
//
//   - releases/<digest>: `current`; the rollback of an apply, which restores the
//     journal's PriorReleaseTarget; RestagePinnedRelease (base restore, the boot
//     frontend follow, package-migration rollback, tape restore); and
//     trustedSourceDir (a pinned release is a trusted apply source). Kept:
//     current; the Prior of the journal that installed it; the pre-migration
//     release of a package migration whose rollback is still reachable
//     (package-migration/<pair>.json: last attempt not rolled_back or refused,
//     and `current` is one of its pre, target or served releases). A release
//     that is none of these is deleted. What this costs: a tape restore to a
//     checkpoint whose release is neither current nor its predecessor finds the
//     pinned release absent and refuses before anything is changed
//     (agentcore verifyPinnedFrontend: "served SPA is underivable"); nothing
//     re-fetches a release from the host, so those checkpoints are no longer
//     restorable to their binary.
//   - gc-roots/<digest>/<base>: symlinks to store/<base>, the closure of a
//     layered release (materializeReleaseClosure). Kept with its release.
//   - store/<base>: live while a surviving release's gc-root names it. The guest
//     wrapper overlays store/ on /nix/store, so a surviving layered release's
//     entrypoint must be rooted or the collection does not run.
//   - incoming/: staged inputs. Apply deletes its own source once the release
//     is verified and staged (dropStagedSource; a host build is the exception,
//     it ages out below), and does not need it again
//     (a resumed apply of a staged release skips the source). So a payload
//     `<release digest>` whose release is staged is removed here at once. A
//     `<digest>` directory that is not a staged payload is a self-development
//     bundle (or a frozen draft): the updater has no record of which operation
//     owns it, the runtime does, so the runtime removes it when no open
//     operation names it (agentcore collectSettledBundles) and the prune
//     leaves it. Everything else (host-build-*, dot temporaries, anything
//     unrecognised) is removed once pruneInputGrace old; see that constant.
//
// It runs under the updater's mutex, so never inside an Apply, and skips
// entirely while any apply is unsettled or any apply key is held
// (UnsettledApply), which is what an in-flight operation's inputs are tied to.
// A release is renamed out of releases/ before it is deleted, so a crash leaves
// either the release or an invisible leftover the next run removes; `current`
// and its predecessor are never touched.
//
// pruneInputGrace is the one time rule. It covers inputs written by a process
// that holds no state the updater can read: a download or host build being
// written under incoming/, a host build staged and waiting for its caller to
// take the apply key (the caller mints receipts between the two), a baseline
// release staged by StageBaselineRelease and waiting for its swap, and a store
// object materialized for a candidate screenshot run that no release roots
// yet. Each of those windows is seconds to a few minutes.
const pruneInputGrace = 10 * time.Minute

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
	cutoff := u.now().Add(-pruneInputGrace)
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
		name := entry.Name()
		path := filepath.Join(incoming, name)
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		bytes, newest := treeStat(path)
		if computerevent.IsSHA256(name) && info.IsDir() {
			if pathExists(filepath.Join(u.root, "releases", name, "release-manifest.json")) &&
				!pathExists(filepath.Join(path, "bundle.json")) && !pathExists(filepath.Join(path, "bundle.draft.json")) {
				// A payload of a release that is staged and verified: Apply no
				// longer reads it.
			} else {
				// A bundle or frozen draft: the runtime owns it.
				continue
			}
		} else if newest.After(cutoff) {
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
	if keep["*"] {
		log.Printf("updater: prune releases skipped: a package migration record is unreadable")
		return
	}
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
			// Young: a baseline staged and waiting for its swap.
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
// apply that installed it recorded, and the releases a package migration's
// rollback still restores.
func (u *Updater) releasesToKeep(current string) map[string]bool {
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
			if journal.TargetReleaseDigest == current && computerevent.IsSHA256(journal.PriorReleaseDigest) && !journal.StartedAt.Before(installedAt) {
				installedAt = journal.StartedAt
				keep[journal.PriorReleaseDigest] = true
			}
		}
	}
	// A restage without an apply (base restore, tape restore, package
	// migration) swaps `current` with no journal; the journal predecessor is
	// then not what rollback restores, but it is still the one the last apply
	// recorded, so it stays only when a journal targets current. The package
	// migration record names its own pre-migration release.
	pins := filepath.Join(u.root, "package-migration")
	if entries, err := os.ReadDir(pins); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), "hold-") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(pins, entry.Name()))
			if err != nil {
				// Unreadable: its pin is unknown, so no release may go.
				keep["*"] = true
				continue
			}
			var record struct {
				Attempts []struct {
					Outcome             string `json:"outcome"`
					PreReleaseDigest    string `json:"pre_release_digest"`
					TargetReleaseDigest string `json:"target_release_digest"`
					ServedReleaseDigest string `json:"served_release_digest"`
				} `json:"attempts"`
			}
			if json.Unmarshal(raw, &record) != nil {
				keep["*"] = true
				continue
			}
			if len(record.Attempts) == 0 {
				continue
			}
			last := record.Attempts[len(record.Attempts)-1]
			if last.Outcome == "rolled_back" || last.Outcome == "refused" || !computerevent.IsSHA256(last.PreReleaseDigest) {
				continue
			}
			if current == last.PreReleaseDigest || current == last.TargetReleaseDigest || current == last.ServedReleaseDigest {
				keep[last.PreReleaseDigest] = true
			}
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

// dropStagedSource removes an apply's source directory from incoming/ once its
// release is verified and staged: the release dir holds a verified copy and
// Apply does not read the source again. A directory carrying a bundle is never
// a source and is left alone. Best effort: the prune is the backstop.
func (u *Updater) dropStagedSource(sourceDir string) {
	incoming, err := filepath.EvalSymlinks(filepath.Join(u.root, "incoming"))
	if err != nil {
		return
	}
	relative, err := filepath.Rel(incoming, sourceDir)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return
	}
	if pathExists(filepath.Join(sourceDir, "bundle.json")) || pathExists(filepath.Join(sourceDir, "bundle.draft.json")) {
		return
	}
	// A host build stays for its own window: the runtime reuses it on a resumed
	// pass (a re-fetch must not change the minted manifest) and reads its job
	// record while the operation settles; the prune removes it once the window
	// has passed and no apply is unsettled.
	if strings.HasPrefix(relative, "host-build-") {
		return
	}
	if err := removeTree(sourceDir); err != nil {
		log.Printf("updater: drop staged source %s: %v", relative, err)
	}
}
