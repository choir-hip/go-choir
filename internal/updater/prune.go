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
// root holds the current release, the one rollback predecessor, the store
// objects those two root, and inputs of an operation that is in flight.
// Nothing else, however many updates have been applied.
//
// What reads each directory, and so what keeps an entry:
//
//   - releases/<digest>: `current`; the rollback of an apply, which restores the
//     journal's PriorReleaseTarget; RestagePinnedRelease (base restore, the boot
//     frontend follow, package-migration rollback, tape restore); and
//     trustedSourceDir (a pinned release is a trusted apply source). Kept:
//     current; the Prior of the newest apply that installed it, and the Prior of
//     the newest restage that installed it (RestagePinnedRelease journals each
//     swap; the two are kept as a union because a swap and its swap-back would
//     otherwise shadow the apply's predecessor, which the revert of the
//     operation behind current still reads: at most three releases); what a
//     journal with no result names, Prior and Target (a platform update that
//     crashed mid-apply is resumed from it); the pre-migration release of a
//     package migration whose rollback is still reachable
//     (package-migration/<pair>.json: last attempt not rolled_back or refused,
//     and `current` is exactly one of its pre, target or served releases; after
//     a boot frontend follow restages `current` onto a newer baseline the
//     migration's rollback finds its pre-migration release gone and refuses
//     cleanly, which is the zero-cushion price). Any other release is deleted. What this costs, accepted by the owner: a tape restore
//     to a checkpoint whose release is neither current nor its predecessor finds
//     the pinned release absent and refuses before anything is changed
//     (agentcore verifyPinnedFrontend: "served SPA is underivable"); nothing
//     re-fetches a release from the host, so those checkpoints are no longer
//     restorable to their binary.
//   - gc-roots/<digest>/<base>: symlinks to store/<base>, the closure of a
//     layered release (materializeReleaseClosure). Kept with its release.
//   - store/<base>: live while a surviving release's gc-root names it. The guest
//     wrapper overlays store/ on /nix/store, so a surviving layered release's
//     entrypoint must be rooted or the collection does not run.
//   - incoming/: staged inputs. Apply deletes its own source once the release is
//     verified and staged, and when it refuses before staging (dropStagedSource),
//     and does not need it again (a resumed apply of a staged release skips the
//     source). So here: a `<digest>` payload whose release is staged goes at
//     once, a markerless `<digest>` payload with no staged release goes once
//     older than pruneInputGrace (a refused or aborted update), a dot temporary
//     or an unrecognised entry goes once older than pruneInputGrace. Two kinds
//     are left to the runtime, which holds the operation store the updater
//     cannot see: a `<digest>` directory with bundle.json or bundle.draft.json,
//     and `host-build-*` (the shot pipeline reuses a host build for its
//     operation's lifetime). agentcore collectSettledBundles removes them when
//     no open operation, and no operation behind a surviving release, names
//     their bundle.
//
// The prune works in chunks, one entry per chunk. Each chunk takes the
// updater's mutex, so never runs inside an Apply and never makes an RPC wait
// behind a long deletion, takes the restore hold (HoldRestore) without waiting
// and keeps it until the chunk ends, and stands down, naming why, when the hold
// is not free, when an apply key is held (an apply or a settle is running) or
// when a journal cannot be read. It recomputes what to keep from the journals
// and `current` in every chunk, so a restore that starts between two chunks is
// seen by the next one. A journal with no result whose key nobody holds is a
// dead apply: it does not stop the prune, it only keeps what it names.
// A release is renamed out of releases/ before it is deleted, so a crash leaves
// either the release or an invisible leftover the next run removes; `current`
// and its predecessor are never touched.
//
// pruneInputGrace is the one time rule. It covers inputs written by a process
// that holds no state the updater can read: a download being written under
// incoming/, a platform payload written before its apply runs, a baseline
// release staged by StageBaselineRelease and waiting for its swap, and a store
// object materialized for a candidate screenshot run that no release roots yet.
// Each of those windows is seconds to a few minutes.
const pruneInputGrace = 10 * time.Minute

// PruneReport is what one prune removed.
type PruneReport struct {
	IncomingRemoved int
	ReleasesRemoved int
	GCRootsRemoved  int
	StoreRemoved    int
	// BytesFreed is allocated bytes (st_blocks), the unit the data_volume
	// observability section reports.
	BytesFreed int64
	// Skipped says why the prune stood down; it may have freed something first.
	Skipped string
}

// Prune removes what no apply, rollback or restore can still need. It is safe
// to run at any time and repeatedly; it yields the updater between chunks.
func (u *Updater) Prune() PruneReport {
	var report PruneReport
	failed := map[string]bool{}
	for {
		progressed, stop := u.pruneChunk(&report, failed)
		if stop != "" {
			report.Skipped = stop
			break
		}
		if !progressed {
			break
		}
		if u.pruneBetween != nil {
			u.pruneBetween()
		}
	}
	if report.Skipped != "" {
		log.Printf("updater: prune skipped: %s (incoming=%d releases=%d gc_roots=%d store=%d bytes_freed=%d)", report.Skipped,
			report.IncomingRemoved, report.ReleasesRemoved, report.GCRootsRemoved, report.StoreRemoved, report.BytesFreed)
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

// pruneChunk removes at most one entry under the updater's mutex and the
// restore hold. stop is non-empty when the prune must stand down.
func (u *Updater) pruneChunk(report *PruneReport, failed map[string]bool) (progressed bool, stop string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	release, acquired, err := tryHoldRestore(u.root)
	if err != nil {
		return false, "the restore hold cannot be taken: " + err.Error()
	}
	if !acquired {
		return false, "a restore holds the updater root"
	}
	defer release()
	if u.pruneHeld != nil {
		u.pruneHeld()
	}
	if reason := pruneBlocked(u.root); reason != "" {
		return false, reason
	}
	cutoff := u.now().Add(-pruneInputGrace)
	return u.pruneIncoming(cutoff, report, failed) ||
		u.pruneReleaseLeftovers(cutoff, report, failed) ||
		u.pruneReleases(cutoff, report, failed) ||
		u.pruneGCRootsAndStore(cutoff, report, failed), ""
}

// pruneBlocked names why no chunk may run: an apply key is held (an apply or a
// settle is running), or a journal or the operations directory cannot be read
// (what it names is unknown). A journal with no result whose key is free is a
// dead apply and does not block; releasesToKeep keeps what it names.
func pruneBlocked(root string) string {
	dir := operationsDir(root)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		return "operations cannot be listed: " + err.Error()
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch {
		case strings.HasSuffix(name, ".json"):
			if _, _, err := readJournal(filepath.Join(dir, name)); err != nil {
				return "journal " + name + " cannot be read"
			}
		case strings.HasSuffix(name, ".lock"):
			f, err := os.OpenFile(filepath.Join(dir, name), os.O_RDWR, 0)
			if err != nil {
				return "apply key " + name + " cannot be probed"
			}
			unlock, acquired, lockErr := lockFile(f, false)
			if acquired {
				unlock()
			}
			f.Close()
			if lockErr != nil || !acquired {
				return "an apply key is held (" + name + ")"
			}
		}
	}
	return ""
}

func (u *Updater) pruneIncoming(cutoff time.Time, report *PruneReport, failed map[string]bool) bool {
	incoming := filepath.Join(u.root, "incoming")
	entries, err := os.ReadDir(incoming)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(incoming, name)
		if failed[path] {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		staged := false
		if info.IsDir() {
			if strings.HasPrefix(name, "host-build-") {
				continue // the runtime owns it
			}
			if computerevent.IsSHA256(name) {
				if pathExists(filepath.Join(path, "bundle.json")) || pathExists(filepath.Join(path, "bundle.draft.json")) {
					continue // a bundle: the runtime owns it
				}
				staged = pathExists(filepath.Join(u.root, "releases", name, "release-manifest.json"))
			}
		}
		bytes, newest := treeStat(path)
		if !staged && newest.After(cutoff) {
			continue
		}
		if err := removeTree(path); err != nil {
			log.Printf("updater: prune incoming %s: %v", name, err)
			failed[path] = true
			continue
		}
		report.IncomingRemoved++
		report.BytesFreed += bytes
		return true
	}
	return false
}

// pruneReleaseLeftovers finishes a crashed prune and sweeps stage temporaries:
// both are invisible to every reader.
func (u *Updater) pruneReleaseLeftovers(cutoff time.Time, report *PruneReport, failed map[string]bool) bool {
	releasesDir := filepath.Join(u.root, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !(strings.HasPrefix(name, ".prune-") || strings.HasPrefix(name, ".stage-")) {
			continue
		}
		path := filepath.Join(releasesDir, name)
		if failed[path] {
			continue
		}
		bytes, newest := treeStat(path)
		if strings.HasPrefix(name, ".stage-") && newest.After(cutoff) {
			continue
		}
		if err := removeTree(path); err != nil {
			log.Printf("updater: prune releases %s: %v", name, err)
			failed[path] = true
			continue
		}
		report.BytesFreed += bytes
		if strings.HasPrefix(name, ".prune-") {
			report.ReleasesRemoved++
		}
		return true
	}
	return false
}

func (u *Updater) pruneReleases(cutoff time.Time, report *PruneReport, failed map[string]bool) bool {
	current, _, err := currentReleaseOf(u.root)
	if err != nil || current == "" {
		// Without a readable current release nothing proves which releases a
		// rollback needs.
		return false
	}
	keep := u.releasesToKeep(current)
	if keep["*"] {
		return false
	}
	releasesDir := filepath.Join(u.root, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !computerevent.IsSHA256(name) || keep[name] {
			continue
		}
		path := filepath.Join(releasesDir, name)
		if failed[path] {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil || info.ModTime().After(cutoff) {
			// Young: a baseline staged and waiting for its swap.
			continue
		}
		bytes, _ := treeStat(path)
		hidden := filepath.Join(releasesDir, ".prune-"+name)
		if err := os.Rename(path, hidden); err != nil {
			log.Printf("updater: prune release %s: %v", short(name), err)
			failed[path] = true
			continue
		}
		_ = syncDir(releasesDir)
		if err := removeTree(hidden); err != nil {
			// The leftover is invisible and the next run removes it.
			log.Printf("updater: prune release %s: %v", short(name), err)
			failed[path] = true
		}
		report.ReleasesRemoved++
		report.BytesFreed += bytes
		return true
	}
	return false
}

// releasesToKeep is the current release, the Prior of the newest apply that
// installed it, the Prior of the newest restage that installed it (a union, not
// a choice: a swap and its swap-back journal "B -> R2" newer than the apply
// "R1 -> R2", and the revert of the operation behind R2 still reads R1), what a
// journal with no result names, and the releases a package migration's
// rollback still restores. At most three releases without a dead journal.
func (u *Updater) releasesToKeep(current string) map[string]bool {
	keep := map[string]bool{current: true}
	var appliedAt, restagedAt time.Time
	appliedPrior, restagedPrior := "", ""
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
			installs := journal.TargetReleaseDigest == current && computerevent.IsSHA256(journal.PriorReleaseDigest) && journal.PriorReleaseDigest != current
			switch outcome := journal.Result.Outcome; {
			case outcome == "":
				// A dead apply, resumed from its journal: it needs both ends.
				for _, digest := range []string{journal.PriorReleaseDigest, journal.TargetReleaseDigest} {
					if computerevent.IsSHA256(digest) {
						keep[digest] = true
					}
				}
			case outcome == "applied" && installs && (appliedPrior == "" || journal.StartedAt.After(appliedAt)):
				appliedAt, appliedPrior = journal.StartedAt, journal.PriorReleaseDigest
			case outcome == restagedOutcome && installs && (restagedPrior == "" || journal.StartedAt.After(restagedAt)):
				restagedAt, restagedPrior = journal.StartedAt, journal.PriorReleaseDigest
			}
		}
	}
	for _, prior := range []string{appliedPrior, restagedPrior} {
		if prior != "" {
			keep[prior] = true
		}
	}
	pins := filepath.Join(u.root, "package-migration")
	if entries, err := os.ReadDir(pins); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), "hold-") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(pins, entry.Name()))
			if err != nil {
				keep["*"] = true // unreadable: its pin is unknown, so no release may go
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

// readManifestLight parses a release's manifest without hashing its files: the
// prune only needs which closure and entrypoint a surviving release declares.
func readManifestLight(releaseDir string) (ReleaseManifest, error) {
	raw, err := os.ReadFile(filepath.Join(releaseDir, "release-manifest.json"))
	if err != nil {
		return ReleaseManifest{}, err
	}
	var manifest ReleaseManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return ReleaseManifest{}, err
	}
	return manifest, nil
}

// pruneGCRootsAndStore removes the gc-roots of a release that is gone, then
// the store objects no surviving release roots.
func (u *Updater) pruneGCRootsAndStore(cutoff time.Time, report *PruneReport, failed map[string]bool) bool {
	surviving := map[string]ReleaseManifest{}
	manifestsReadable := true
	entries, err := os.ReadDir(filepath.Join(u.root, "releases"))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !computerevent.IsSHA256(name) {
			continue
		}
		manifest, err := readManifestLight(filepath.Join(u.root, "releases", name))
		if err != nil {
			manifestsReadable = false
		}
		surviving[name] = manifest
	}
	gcRoots := filepath.Join(u.root, "gc-roots")
	live := map[string]bool{}
	rooted := map[string]bool{}
	if entries, err := os.ReadDir(gcRoots); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(gcRoots, name)
			if _, ok := surviving[name]; !ok {
				if failed[path] {
					continue
				}
				if err := removeTree(path); err != nil {
					log.Printf("updater: prune gc-roots %s: %v", short(name), err)
					failed[path] = true
					continue
				}
				report.GCRootsRemoved++
				return true
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
		return false
	}
	storeDir := filepath.Join(u.root, "store")
	storeEntries, err := os.ReadDir(storeDir)
	if err != nil {
		return false
	}
	for _, entry := range storeEntries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(name, ".staging-") {
			continue
		}
		if live[name] {
			continue
		}
		path := filepath.Join(storeDir, name)
		if failed[path] {
			continue
		}
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
			failed[path] = true
			continue
		}
		report.StoreRemoved++
		report.BytesFreed += bytes
		return true
	}
	return false
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// treeStat is the allocated size (st_blocks) of the regular files under path
// and the newest modification time of anything in it. Symlinks are not
// followed.
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
			bytes += allocatedBytes(info)
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
// release is verified and staged, or once the apply refused before staging:
// the release dir holds a verified copy and Apply does not read the source
// again, and a caller restages a payload when it retries. A directory carrying
// a bundle is never a source and is left alone, and a host build stays for the
// runtime (the shot pipeline reuses it, and a resumed pass must mint the same
// manifest). Best effort: the prune is the backstop.
func (u *Updater) dropStagedSource(sourceDir string) {
	incoming, err := filepath.EvalSymlinks(filepath.Join(u.root, "incoming"))
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Clean(sourceDir)); err == nil {
		sourceDir = resolved
	}
	relative, err := filepath.Rel(incoming, sourceDir)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return
	}
	if pathExists(filepath.Join(sourceDir, "bundle.json")) || pathExists(filepath.Join(sourceDir, "bundle.draft.json")) {
		return
	}
	if strings.HasPrefix(relative, "host-build-") {
		return
	}
	if err := removeTree(sourceDir); err != nil {
		log.Printf("updater: drop staged source %s: %v", relative, err)
	}
}
