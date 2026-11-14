package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The updater's root grew with every platform update and every host build:
// nothing deleted from incoming/, releases/, gc-roots/ or store/
// (docs/problems/updater-directory-fills-the-data-volume-2026-11-13.md).
// Owner ruling 2026-11-13: every computer will look like this one, so the
// directory is bounded, not conservatively pruned. After a prune the root holds
// the current release, at most two rollback predecessors, the store objects
// those roots, and the inputs of an operation that may still be in flight.
//
// The prune runs once per boot, as a oneshot ordered before both the updater
// daemon and the runtime (nix/autoputer-vm.nix, go-choir-updater-prune). While
// it runs it is the only process touching the root: no apply, no restore, no
// swap of `current`. That is the whole concurrency design
// (docs/problems/updater-prune-three-rounds-structural-assessment-2026-11-14.md):
// there is no hold, no chunk loop and no lock order to get wrong. What it
// still checks, because a journal or a key lock outlives a process, is that no
// apply key is held and that every journal reads.
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
//     cleanly, which is the zero-cushion price). Any other release is deleted.
//     What this costs, accepted by the owner: a tape restore to a checkpoint
//     whose release is neither current nor its predecessor finds the pinned
//     release absent and refuses before anything is changed (agentcore
//     verifyPinnedFrontend: "served SPA is underivable"); nothing re-fetches a
//     release from the host, so those checkpoints are no longer restorable to
//     their binary.
//   - gc-roots/<digest>/<base>: symlinks to store/<base>, the closure of a
//     layered release (materializeReleaseClosure). Kept with its release.
//   - store/<base>: live while a surviving release's gc-root names it. The guest
//     wrapper overlays store/ on /nix/store, so a surviving layered release's
//     entrypoint must be rooted or the collection does not run.
//   - incoming/: staged inputs. Apply deletes its own source once the release is
//     verified and staged, and when it refuses before staging for a
//     deterministic reason (base mismatch, state incompatibility;
//     dropStagedSource); a failed stage, an unreadable floor and a failed
//     closure replay keep it: the failure may be transient (no space) and the
//     payload is the only local copy. The refusal is journaled terminally for
//     that idempotency key, so a kept payload serves diagnosis, and a retry
//     under a new key, until this prune collects it past its window (a resumed
//     apply of a staged release never needs the source). So here: a `<digest>` payload whose release is staged goes at
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
// A dead journal (no result, no process holds its key: the apply died, or a
// platform update was left pending on purpose) does not stop the prune, it
// keeps what it names. The runtime's crash-boot settlement fences a dead
// self-development journal (journal_fence.go), so that one pins for a boot; a
// platform update left pending (platform_update.go settlePendingPlatformUpdateAfterCrash)
// is never fenced and its two releases stay until the pending state is resolved.
// The report counts them (DeadJournals) so the pin is seen.
//
// Every removal is a rename out of its directory to `.prune-<name>` followed by
// a deletion of the hidden tree. Everything the run decides to delete is hidden
// first (cheap renames, one consistent decision), then the hidden trees are
// deleted until the budget is spent. A crash or a spent budget leaves invisible
// leftovers that the next run deletes first; `current` and what it needs are
// never touched, and a store object is never half-deleted under its real name
// (MaterializeClosure skips an object that exists, so a half-deleted one would
// be reused).
//
// pruneInputGrace is the one time rule. At boot every process that wrote an
// input is gone, so it only protects what a planned restart may resume from
// (a payload written just before its apply, a baseline staged and waiting for
// its swap, a store object a candidate run materialized): minutes old at most.
const pruneInputGrace = 10 * time.Minute

// DefaultPruneBudget is the time a boot waits for the prune. Deleting does not
// have to finish: the next boot continues from the hidden leftovers.
const DefaultPruneBudget = 60 * time.Second

// pruneHiddenPrefix names an entry the prune has decided to delete and removed
// from its readers' view.
const pruneHiddenPrefix = ".prune-"

// PruneOptions configures one prune of an updater root.
type PruneOptions struct {
	Root string
	// Budget bounds the deleting; zero means DefaultPruneBudget.
	Budget time.Duration
	// Now is the wall clock the input window is measured against; nil means
	// time.Now. Clock is the clock the budget is measured on; nil means time.Now.
	Now   func() time.Time
	Clock func() time.Time
	// RunDir is where the updater daemon keeps its socket and its started
	// marker ("/run/choir" on a guest). The prune is for the boot transaction,
	// before the daemon starts: when either exists a daemon is or was running
	// in this boot, so the prune refuses. Empty skips the check (tests).
	RunDir string

	// rename and freeChunk are test seams: the rename that hides an entry, and
	// the bytes freed in place before the rename is retried.
	rename    func(oldpath, newpath string) error
	freeChunk int64
}

// pruneLiveMarkers are the files, under RunDir, that say the updater daemon is
// running or has run in this boot.
var pruneLiveMarkers = []string{"updater.sock", "updater-unit-entered", "runtime-unit-entered"}

// pruneFreeChunk is how much an in-place deletion frees before it syncs and
// retries the rename that failed for want of space.
const pruneFreeChunk = 1 << 20

// PruneReport is what one prune did.
type PruneReport struct {
	IncomingRemoved int
	ReleasesRemoved int
	GCRootsRemoved  int
	StoreRemoved    int
	// ReleasesKept is the release directories left in releases/.
	ReleasesKept int
	// BytesFreed is allocated bytes (st_blocks), the unit the data_volume
	// observability section reports.
	BytesFreed int64
	// Hidden is the entries decided and hidden whose deletion has not finished
	// (the budget ran out, or a deletion failed); the next run finishes them.
	Hidden int
	// Errors is the deletions that failed.
	Errors int
	// ENOSPCSkipped is the store entries whose rename failed for want of space
	// and that were left for a boot with room: a store object is never freed in
	// place (it has no marker, and MaterializeClosure trusts an existing one).
	ENOSPCSkipped int
	// InPlaceUnfinished is the entries freed in place (a full volume) that the
	// budget cut short: partial under their real name, finished by the next
	// boot. A release among them has no manifest, so no reader trusts it.
	InPlaceUnfinished int
	// DeadJournals is the journals with no result that pin what they name.
	DeadJournals int
	// BudgetExhausted is set when the budget stopped the deleting.
	BudgetExhausted bool
	Elapsed         time.Duration
	// Skipped says why the prune stood down.
	Skipped string
}

// Entries is every entry whose deletion finished.
func (r PruneReport) Entries() int {
	return r.IncomingRemoved + r.ReleasesRemoved + r.GCRootsRemoved + r.StoreRemoved
}

// Line is the one log line a prune writes.
func (r PruneReport) Line() string {
	var b strings.Builder
	b.WriteString("updater: prune")
	if r.Skipped != "" {
		// A run that stood down measured nothing: no counters.
		return fmt.Sprintf("updater: prune skipped (%s): elapsed=%s", r.Skipped, r.Elapsed.Round(time.Millisecond))
	}
	if r.BudgetExhausted {
		b.WriteString(" budget exhausted, the next boot continues")
	}
	fmt.Fprintf(&b, ": releases_kept=%d entries_deleted=%d incoming=%d releases=%d gc_roots=%d store=%d bytes_freed=%d hidden_left=%d errors=%d enospc_skipped=%d inplace_unfinished=%d dead_journals=%d elapsed=%s",
		r.ReleasesKept, r.Entries(), r.IncomingRemoved, r.ReleasesRemoved, r.GCRootsRemoved, r.StoreRemoved, r.BytesFreed, r.Hidden, r.Errors, r.ENOSPCSkipped, r.InPlaceUnfinished, r.DeadJournals, r.Elapsed.Round(time.Millisecond))
	return b.String()
}

// Prune removes what no apply, rollback or restore can still need from an
// updater root nothing else is using, and stops deleting at the budget. It is
// safe to run repeatedly, to kill at any point, and needs no signer, socket,
// network or runtime.
func Prune(opts PruneOptions) PruneReport {
	p := &pruner{root: filepath.Clean(opts.Root), now: opts.Now, clock: opts.Clock, runDir: opts.RunDir, rename: opts.rename, freeChunk: opts.freeChunk}
	if p.rename == nil {
		p.rename = os.Rename
	}
	if p.freeChunk <= 0 {
		p.freeChunk = pruneFreeChunk
	}
	if p.now == nil {
		p.now = func() time.Time { return time.Now().UTC() }
	}
	if p.clock == nil {
		p.clock = time.Now
	}
	budget := opts.Budget
	if budget <= 0 {
		budget = DefaultPruneBudget
	}
	start := p.clock()
	p.deadline = start.Add(budget)
	if info, err := os.Stat(p.root); err != nil || !info.IsDir() {
		p.report.Skipped = "no root: " + p.root
	} else {
		p.run()
	}
	if p.report.Skipped == "" {
		p.report.ReleasesKept = countReleaseDirs(p.root)
	}
	p.report.Elapsed = p.clock().Sub(start)
	return p.report
}

type pruner struct {
	root     string
	now      func() time.Time
	clock    func() time.Time
	deadline time.Time
	report   PruneReport
	runDir   string
	rename   func(oldpath, newpath string) error
	// freeChunk: see PruneOptions.
	freeChunk int64
}

func (p *pruner) expired() bool {
	if p.clock().After(p.deadline) {
		p.report.BudgetExhausted = true
		return true
	}
	return false
}

func (p *pruner) run() {
	if p.runDir != "" {
		for _, marker := range pruneLiveMarkers {
			path := filepath.Join(p.runDir, marker)
			if pathExists(path) {
				p.report.Skipped = "the updater is running: " + path
				return
			}
		}
	}
	if reason := pruneBlocked(p.root); reason != "" {
		p.report.Skipped = reason
		return
	}
	// First, finish what an earlier run hid: on a full volume this is what
	// makes room for the renames below.
	p.sweepAll()
	if p.report.BudgetExhausted {
		return
	}
	cutoff := p.now().Add(-pruneInputGrace)
	// Decide in phases, expendable bytes first. Each decision is a rename; the
	// renames of a directory are made durable once, after its phase (a rename
	// lost to a crash only means the entry is decided again at the next boot,
	// and an entry renamed but not yet deleted is invisible either way).
	// incoming/ is judged against the releases as they were.
	p.hideIncoming(cutoff)
	p.hideReleases(cutoff)
	// Delete incoming/ and releases/ before the gc-roots and store/ entries are
	// hidden: on a full volume those gigabytes must exist before the many store
	// renames need directory blocks.
	p.sweep(filepath.Join(p.root, "incoming"), &p.report.IncomingRemoved)
	p.sweep(filepath.Join(p.root, "releases"), &p.report.ReleasesRemoved)
	if p.report.BudgetExhausted {
		return
	}
	p.hideGCRootsAndStore(cutoff)
	p.sweepAll()
}

func (p *pruner) sweepAll() {
	for _, dir := range []struct {
		name string
		kind *int
	}{
		{"incoming", &p.report.IncomingRemoved},
		{"releases", &p.report.ReleasesRemoved},
		{"gc-roots", &p.report.GCRootsRemoved},
		{"store", &p.report.StoreRemoved},
	} {
		p.sweep(filepath.Join(p.root, dir.name), dir.kind)
	}
}

// pruneBlocked names why the prune must not run: an apply key is held (an apply
// or a settle is running), or a journal or the operations directory cannot be
// read (what it names is unknown). A journal with no result whose key is free
// is a dead apply and does not block; releasesToKeep keeps what it names.
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

// hide renames path to .prune-<name> beside it. A name that already exists
// (a leftover of an earlier run for a release staged and pruned again) gets a
// suffix. A rename can fail for want of space on a full volume, exactly where
// the prune is needed; then the entry is freed in place (freeInPlace) and
// counted in kind.
func (p *pruner) hide(path string, kind *int, inPlaceOK bool) {
	dir, name := filepath.Split(path)
	hidden := filepath.Join(dir, pruneHiddenPrefix+name)
	if pathExists(hidden) {
		hidden += "-" + strconv.FormatInt(p.clock().UnixNano(), 36)
	}
	err := p.rename(path, hidden)
	if err == nil {
		return
	}
	if !errors.Is(err, syscall.ENOSPC) {
		log.Printf("updater: prune hide %s: %v", name, err)
		p.report.Errors++
		return
	}
	if !inPlaceOK {
		// A store object has no marker and MaterializeClosure trusts an existing
		// one as whole, so it is never half deleted under its real name: it
		// waits for a boot with room.
		log.Printf("updater: prune hide %s: no space to rename; left for a later boot", name)
		p.report.ENOSPCSkipped++
		return
	}
	p.freeInPlace(path, hidden, kind)
}

// freeInPlace deletes an entry whose rename failed for want of space. It is
// safe because nothing else is running, but a tree half deleted under its real
// name must not be trusted by what reads it next, so the order is chosen:
//
//   - a release's release-manifest.json goes first: ReadPinnedManifest, the
//     boot follow and every restore read it, and a release directory without
//     one is a corpse that the next run removes whatever its age
//     (hideReleases); Apply restages from its source, never from a corpse;
//   - a store object is never freed in place (hide refuses it): it has no
//     marker, and MaterializeClosure trusts an existing object as whole, so a
//     kill inside an in-place delete would leave a partial object that is
//     reused. A store rename that fails for want of space is counted
//     (ENOSPCSkipped) and left for a boot with room; incoming/ and releases/
//     are deleted before the store is touched, which is what makes the room.
//
// After each MiB freed it syncs (ext4 returns freed blocks to the allocator at
// commit) and retries the rename, so only the remainder is hidden.
func (p *pruner) freeInPlace(path, hidden string, kind *int) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		_ = os.Chmod(path, 0o700)
		var files []string
		_ = filepath.WalkDir(path, func(q string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				_ = os.Chmod(q, 0o700)
				return nil
			}
			files = append(files, q)
			return nil
		})
		sort.SliceStable(files, func(i, j int) bool {
			return filepath.Base(files[i]) == "release-manifest.json" && filepath.Base(files[j]) != "release-manifest.json"
		})
		var freed int64
		for _, file := range files {
			if p.expired() {
				p.report.InPlaceUnfinished++
				return
			}
			fi, err := os.Lstat(file)
			if err != nil {
				continue
			}
			size := int64(0)
			if fi.Mode().IsRegular() {
				size = allocatedBytes(fi)
			}
			if err := os.Remove(file); err != nil {
				p.fail(file, err)
				return
			}
			p.report.BytesFreed += size
			freed += size
			if freed >= p.freeChunk {
				syncFilesystems()
				freed = 0
				if p.rename(path, hidden) == nil {
					return // what is left is hidden; the sweep finishes it
				}
			}
		}
	}
	if !p.removeTree(path, true) {
		p.report.InPlaceUnfinished++
	} else {
		*kind++
	}
}

func (p *pruner) hideIncoming(cutoff time.Time) {
	incoming := filepath.Join(p.root, "incoming")
	entries, err := os.ReadDir(incoming)
	if err != nil {
		return
	}
	defer func() { _ = syncDir(incoming) }()
	for _, entry := range entries {
		if p.expired() {
			return
		}
		name := entry.Name()
		if strings.HasPrefix(name, pruneHiddenPrefix) {
			continue
		}
		path := filepath.Join(incoming, name)
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
				staged = pathExists(filepath.Join(p.root, "releases", name, "release-manifest.json"))
			}
		}
		if !staged {
			if _, newest := treeStat(path); newest.After(cutoff) {
				continue
			}
		}
		p.hide(path, &p.report.IncomingRemoved, true)
	}
}

func (p *pruner) hideReleases(cutoff time.Time) {
	releasesDir := filepath.Join(p.root, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return
	}
	defer func() { _ = syncDir(releasesDir) }()
	// Stage temporaries are invisible to every reader; old ones go. So does
	// anything in releases/ that is not a directory (a symlink or a file:
	// nothing writes one there).
	for _, entry := range entries {
		if p.expired() {
			return
		}
		name := entry.Name()
		if strings.HasPrefix(name, pruneHiddenPrefix) {
			continue
		}
		path := filepath.Join(releasesDir, name)
		if !entry.IsDir() {
			if info, err := os.Lstat(path); err == nil && !info.ModTime().After(cutoff) {
				p.hide(path, &p.report.ReleasesRemoved, true)
			}
			continue
		}
		if strings.HasPrefix(name, ".stage-") {
			if _, newest := treeStat(path); !newest.After(cutoff) {
				p.hide(path, &p.report.ReleasesRemoved, true)
			}
		}
	}
	current, _, err := currentReleaseOf(p.root)
	if err != nil || current == "" {
		// Without a readable current release nothing proves which releases a
		// rollback needs.
		return
	}
	keep := p.releasesToKeep(current)
	if keep["*"] {
		return
	}
	for _, entry := range entries {
		if p.expired() {
			return
		}
		name := entry.Name()
		if !entry.IsDir() || !computerevent.IsSHA256(name) || keep[name] {
			continue
		}
		path := filepath.Join(releasesDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		// A release directory with no manifest is the corpse of a deletion that
		// was interrupted in place (freeInPlace removes the manifest first):
		// no reader trusts it, and the deletion touched it, so its age says
		// nothing.
		if info.ModTime().After(cutoff) && pathExists(filepath.Join(path, "release-manifest.json")) {
			// Young: a baseline staged and waiting for its swap.
			continue
		}
		p.hide(path, &p.report.ReleasesRemoved, true)
	}
}

// releasesToKeep is the current release, the Prior of the newest apply that
// installed it, the Prior of the newest restage that installed it (a union, not
// a choice: a swap and its swap-back journal "B -> R2" newer than the apply
// "R1 -> R2", and the revert of the operation behind R2 still reads R1), what a
// journal with no result names, and the releases a package migration's
// rollback still restores. At most three releases without a dead journal.
func (p *pruner) releasesToKeep(current string) map[string]bool {
	keep := map[string]bool{current: true}
	var appliedAt, restagedAt time.Time
	appliedPrior, restagedPrior := "", ""
	dir := operationsDir(p.root)
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
				p.report.DeadJournals++
				for _, digest := range []string{journal.PriorReleaseDigest, journal.TargetReleaseDigest} {
					if computerevent.IsSHA256(digest) {
						keep[digest] = true
					}
				}
			case (outcome == "applied" || installedFence(journal)) && installs && (appliedPrior == "" || journal.StartedAt.After(appliedAt)):
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
	pins := filepath.Join(p.root, "package-migration")
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

// installedFence is a journal the runtime's crash-boot settlement (or an owner
// settle) rewrote through Fence after arbitrating, on the current pointer, that
// the apply had installed its target: Phase "arbitrated", Outcome "refused".
// The operation it belongs to is applied and revertible, and its revert reads
// the Prior, so it counts as an apply that installed the target. A fence of
// nothing installed (Phase "fenced") installed nothing, and an unknown verdict
// is no proof of anything: neither names a predecessor. A fence written before
// the Verdict field existed carries none; it counts when its target is
// current, which errs towards keeping one release.
func installedFence(journal operationJournal) bool {
	return journal.Result.Outcome == "refused" && journal.Phase == "arbitrated" &&
		(journal.Verdict == "" || journal.Verdict == string(VerdictInstalled))
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

// entrypointStoreBase is the first path segment of a release's layering
// entrypoint: the store object the binary it boots lives in. The entrypoint is
// normalised the way resolveLayeringEntrypoint resolves it (a leading slash or
// `./` does not make the first segment empty); "" means a plain release or an
// entrypoint that does not resolve.
func entrypointStoreBase(manifest ReleaseManifest) string {
	if manifest.LayeringEntrypoint == "" {
		return ""
	}
	for _, segment := range strings.Split(filepath.ToSlash(manifest.LayeringEntrypoint), "/") {
		if segment == ".." {
			return ""
		}
	}
	rel := strings.TrimPrefix(filepath.Clean("/"+filepath.ToSlash(manifest.LayeringEntrypoint)), "/")
	if rel == "" {
		return ""
	}
	return strings.SplitN(rel, "/", 2)[0]
}

// hideGCRootsAndStore hides the gc-roots of a release that is gone, then the
// store objects no surviving release roots.
func (p *pruner) hideGCRootsAndStore(cutoff time.Time) {
	surviving := map[string]ReleaseManifest{}
	manifestsReadable := true
	entries, err := os.ReadDir(filepath.Join(p.root, "releases"))
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !computerevent.IsSHA256(name) {
			continue
		}
		manifest, err := readManifestLight(filepath.Join(p.root, "releases", name))
		if err != nil {
			manifestsReadable = false
		}
		surviving[name] = manifest
	}
	gcRoots := filepath.Join(p.root, "gc-roots")
	live := map[string]bool{}
	rooted := map[string]bool{}
	if entries, err := os.ReadDir(gcRoots); err == nil {
		for _, entry := range entries {
			if p.expired() {
				_ = syncDir(gcRoots)
				return // the live set is incomplete: the store waits for the next boot
			}
			name := entry.Name()
			path := filepath.Join(gcRoots, name)
			if strings.HasPrefix(name, pruneHiddenPrefix) {
				continue
			}
			if _, ok := surviving[name]; !ok {
				p.hide(path, &p.report.GCRootsRemoved, true)
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
		// The gc-roots phase is over: make its renames durable now, before the
		// store phase renames anything.
		_ = syncDir(gcRoots)
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
			if first := entrypointStoreBase(manifest); first == "" || !live[first] {
				storeSafe = false
			}
		}
	}
	if !storeSafe {
		return
	}
	storeDir := filepath.Join(p.root, "store")
	storeEntries, err := os.ReadDir(storeDir)
	if err != nil {
		return
	}
	defer func() { _ = syncDir(storeDir) }()
	for _, entry := range storeEntries {
		if p.expired() {
			return
		}
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
		if _, newest := treeStat(path); newest.After(cutoff) {
			continue
		}
		p.hide(path, &p.report.StoreRemoved, false)
	}
}

// sweep deletes the hidden entries of one directory, in order, until the budget
// is spent.
func (p *pruner) sweep(dir string, counter *int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), pruneHiddenPrefix) {
			continue
		}
		if p.expired() {
			p.report.Hidden++
			continue
		}
		if p.removeTree(filepath.Join(dir, entry.Name()), true) {
			*counter++
		} else {
			p.report.Hidden++
		}
	}
}

// removeTree deletes a hidden tree whose directories may be sealed read-only,
// file by file, counting allocated bytes as they go. It returns false when the
// budget ran out or a deletion failed; what is left is still hidden.
func (p *pruner) removeTree(path string, budgeted bool) bool {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	if err != nil {
		p.fail(path, err)
		return false
	}
	if !info.IsDir() {
		size := int64(0)
		if info.Mode().IsRegular() {
			size = allocatedBytes(info)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			p.fail(path, err)
			return false
		}
		p.report.BytesFreed += size
		return true
	}
	_ = os.Chmod(path, 0o700)
	children, err := os.ReadDir(path)
	if err != nil {
		p.fail(path, err)
		return false
	}
	for _, child := range children {
		if budgeted && p.expired() {
			return false
		}
		if !p.removeTree(filepath.Join(path, child.Name()), budgeted) {
			return false
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		p.fail(path, err)
		return false
	}
	return true
}

func (p *pruner) fail(path string, err error) {
	p.report.Errors++
	log.Printf("updater: prune delete %s: %v", filepath.Base(path), err)
}

func countReleaseDirs(root string) int {
	entries, err := os.ReadDir(filepath.Join(root, "releases"))
	if err != nil {
		return 0
	}
	n := 0
	for _, entry := range entries {
		if entry.IsDir() && computerevent.IsSHA256(entry.Name()) {
			n++
		}
	}
	return n
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

// removeTree is a full, unbudgeted deletion of a tree whose directories may be
// sealed read-only.
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
// manifest). Best effort: the boot prune is the backstop.
func (u *Updater) dropStagedSource(sourceDir string) {
	incoming, err := filepath.EvalSymlinks(filepath.Join(u.root, "incoming"))
	if err != nil {
		return
	}
	// A symlink source directly under incoming/ is unlinked, never followed.
	if info, err := os.Lstat(filepath.Clean(sourceDir)); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		if parent, err := filepath.EvalSymlinks(filepath.Dir(filepath.Clean(sourceDir))); err == nil && parent == incoming {
			if err := os.Remove(filepath.Clean(sourceDir)); err != nil {
				log.Printf("updater: drop staged source %s: %v", filepath.Base(sourceDir), err)
			}
		}
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
