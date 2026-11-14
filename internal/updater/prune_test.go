package updater

// Failure list for the boot-time prune of the updater root
// (docs/problems/updater-directory-fills-the-data-volume-2026-11-13.md; the
// design is in updater-prune-three-rounds-structural-assessment-2026-11-14.md:
// the prune runs when nothing else does, so there is no restore hold, no chunk
// loop and no lock order). Each item is a thing the updater does wrong today
// (nothing ever deletes from incoming/, releases/, store/ or gc-roots/) or a
// way a prune could break a computer, and the test that pins it:
//
//  F1  a verified platform payload under incoming/<release digest> stays
//      forever after its release is staged (TestPruneRemovesStagedPayloadsAtOnceAndUnstagedOnesPastTheWindow)
//  F2  a finished host build under incoming/host-build-<job> stays forever
//      (TestPruneRemovesOldTemporariesAndLeavesHostBuildsToTheRuntime)
//  F3  a prune must never take a bundle, an entry in its input window, or a
//      payload whose release is not staged yet
//      (TestPruneLeavesBundlesToTheRuntimeAndKeepsInputsInTheirWindow)
//  F4  every release ever applied stays (TestPruneKeepsOnlyCurrentAndItsRecordedPredecessor,
//      TestPruneKeepsThePreMigrationReleaseOfAReachableRollbackOnly,
//      TestPruneKeepsThePredecessorAcrossFailedAndRepeatedApplies,
//      TestPruneKeepsTheAppliedPredecessorAcrossARestageAndSwapBack)
//  F5  a deleted release's gc-roots and its exclusively-held store objects
//      stay; a store object a kept release still roots must not go
//      (TestPruneCollectsStoreObjectsNoKeptReleaseRoots)
//  F6  a kept layered release with no gc-root dir must stop the store
//      collection, not lose its binary (TestPruneLeavesTheStoreAloneWhenAKeptLayeredReleaseIsUnrooted);
//      a leading slash in its entrypoint must not turn the collection off
//      (TestPruneStoreCollectionSurvivesALeadingSlashEntrypoint)
//  F7  no prune while an apply key is held or a journal cannot be read, and
//      none when `current` cannot be read; a dead journal neither disables it
//      nor loses what it names (TestPruneSkipsOnlyOnHeldKeysAndBrokenJournals,
//      TestPruneIsNotDisabledByADeadJournalAndKeepsWhatItNames,
//      TestPruneLeavesReleasesAloneWhenCurrentIsUnreadable)
//  F8  a crash mid-prune leaves a bootable, rollback-capable computer, and the
//      next run finishes the job (TestPruneFinishesACrashedPruneAndIsIdempotent,
//      TestPruneNeverLeavesAHalfDeletedObjectUnderItsRealName)
//  F9  the budget stops the deleting between files and a second run finishes
//      (TestPruneStopsAtItsBudgetAndTheNextRunFinishes)
//  F10 one log line with counts, bytes and why it skipped (TestPruneReportLine)
//  F11 Apply leaves its source in incoming/ after the release is staged
//      (TestApplyDropsItsStagedSource); a resumed apply of a staged release
//      must not need that source (TestApplyResumesAStagedReleaseWithoutItsSource);
//      a refused payload goes (TestApplyDropsARefusedPayload)
//  F12 the root grows with the number of applies, not with what one leaves
//      (TestUpdaterRootIsBoundedOverManyApplies); rollback still works after
//      (TestRollbackStillWorksAfterThePrune); a restore to a pruned release
//      refuses cleanly (TestRestoreToAPrunedReleaseRefusesCleanly)
//  F13 a restage records the release it came from (TestRestagePinnedReleaseRecordsItsPredecessor,
//      TestRestagePinnedReleaseRefusesWhenItCannotJournalButSwapBackProceeds)
//  F14 sizes are allocated bytes (TestPruneReportsAllocatedBytes)

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// pruneNow is the updater's clock in these tests: an hour after the wall
// clock, so releases and store objects the real code just wrote are older than
// the input window, and ageBy can place a file inside or outside it.
var pruneNow = time.Now().UTC().Add(time.Hour)

// pruneRoot is one boot's prune of root, on the tests' clock, with the
// default budget.
func pruneRoot(root string) PruneReport {
	return Prune(PruneOptions{Root: root, Now: func() time.Time { return pruneNow }})
}

func newPruneUpdater(t *testing.T) (*Updater, string) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	u, err := New(root, "computer-test", "realization-test", &fakeServiceManager{}, &fakeHealthProber{},
		testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}})
	if err != nil {
		t.Fatal(err)
	}
	u.now = func() time.Time { return pruneNow }
	return u, root
}

// stageTestRelease stages a real release dir (read-only, verified manifest)
// and returns its digest.
func stageTestRelease(t *testing.T, u *Updater, root, name string, layered bool) string {
	t.Helper()
	entrypoint := ""
	if layered {
		entrypoint = "aaaa-" + name + "/bin/autoputer"
	}
	return stageTestReleaseEntrypoint(t, u, root, name, entrypoint)
}

// stageTestReleaseEntrypoint stages a layered release declaring entrypoint ("" is
// a plain release).
func stageTestReleaseEntrypoint(t *testing.T, u *Updater, root, name, entrypoint string) string {
	t.Helper()
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", name, "idem-"+name, name)
	if entrypoint != "" {
		request.Manifest.ClosureDigest = strings.Repeat("1", 64)
		request.Manifest.LayeringEntrypoint = entrypoint
		if err := refinalizeRequest(&request); err != nil {
			t.Fatal(err)
		}
	}
	digest := request.Manifest.ContentDigest
	if err := u.stageRelease(request.SourceDir, filepath.Join(root, "releases", digest), request.Manifest); err != nil {
		t.Fatal(err)
	}
	// The fixture's source dir is itself an incoming entry named for the
	// operation; it is not what these tests are about.
	makeTreeWritable(request.SourceDir)
	if err := os.RemoveAll(request.SourceDir); err != nil {
		t.Fatal(err)
	}
	return digest
}

func writeTestJournal(t *testing.T, root, key, prior, target string, startedAt time.Time, outcome string) {
	t.Helper()
	journal := operationJournal{Phase: "completed", PriorReleaseDigest: prior, TargetReleaseDigest: target, StartedAt: startedAt}
	if prior != "" {
		journal.PriorReleaseTarget = filepath.Join(root, "releases", prior)
	}
	journal.Result = ApplyResult{ReleaseDigest: target, PriorReleaseDigest: prior, Outcome: outcome}
	if err := writeJournal(filepath.Join(root, "operations", safeName(key)+".json"), journal); err != nil {
		t.Fatal(err)
	}
}

func setCurrent(t *testing.T, root, digest string) {
	t.Helper()
	if err := swapCurrentPointer(root, filepath.Join(root, "releases", digest)); err != nil {
		t.Fatal(err)
	}
}

func ageBy(t *testing.T, path string, age time.Duration) {
	t.Helper()
	when := pruneNow.Add(-age)
	if err := filepath.WalkDir(path, func(p string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(p, when, when)
	}); err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o600); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// rootStoreObjects makes store/<base>/payload objects and gc-roots/<digest>/<base> links.
func rootStoreObjects(t *testing.T, root, digest string, bases ...string) {
	t.Helper()
	for _, base := range bases {
		if !exists(filepath.Join(root, "store", base)) {
			writeTestFile(t, filepath.Join(root, "store", base, "bin", "tool"), 100)
		}
		if err := os.MkdirAll(filepath.Join(root, "gc-roots", digest), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "store", base), filepath.Join(root, "gc-roots", digest, base)); err != nil {
			t.Fatal(err)
		}
	}
}

const old = 48 * time.Hour

func TestPruneRemovesStagedPayloadsAtOnceAndUnstagedOnesPastTheWindow(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	staged := filepath.Join(root, "incoming", digest)
	writeTestFile(t, filepath.Join(staged, "bin", "choir"), 1000)
	ageBy(t, staged, old)
	unstaged := filepath.Join(root, "incoming", strings.Repeat("9", 64))
	writeTestFile(t, filepath.Join(unstaged, "bin", "choir"), 1000)
	ageBy(t, unstaged, old)
	inWindow := filepath.Join(root, "incoming", strings.Repeat("8", 64))
	writeTestFile(t, filepath.Join(inWindow, "bin", "choir"), 1000)
	ageBy(t, inWindow, time.Minute)

	report := pruneRoot(root)
	if exists(staged) {
		t.Fatal("the payload of a staged release survived the prune")
	}
	if exists(unstaged) {
		t.Fatal("a refused or aborted payload survived past the window")
	}
	if !exists(inWindow) {
		t.Fatal("a payload inside its window (its apply has not run) was removed")
	}
	if report.BytesFreed < 1000 || report.IncomingRemoved != 2 {
		t.Fatalf("report = %+v", report)
	}
}

func TestPruneRemovesOldTemporariesAndLeavesHostBuildsToTheRuntime(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	for _, name := range []string{"host-build-abc", ".host-build-xyz", ".stage-123"} {
		writeTestFile(t, filepath.Join(root, "incoming", name, "closure.nar"), 500)
		ageBy(t, filepath.Join(root, "incoming", name), old)
	}
	pruneRoot(root)
	// A host build belongs to an operation the updater cannot see (the shot
	// pipeline reuses it for the operation's lifetime): the runtime removes it.
	if !exists(filepath.Join(root, "incoming", "host-build-abc")) {
		t.Fatal("the updater removed a host build the runtime owns")
	}
	for _, name := range []string{".host-build-xyz", ".stage-123"} {
		if exists(filepath.Join(root, "incoming", name)) {
			t.Fatalf("old %s survived the prune", name)
		}
	}
}

func TestPruneLeavesBundlesToTheRuntimeAndKeepsInputsInTheirWindow(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	incoming := filepath.Join(root, "incoming")
	bundle := filepath.Join(incoming, strings.Repeat("b", 64))
	writeTestFile(t, filepath.Join(bundle, "bundle.json"), 100)
	ageBy(t, bundle, 90*24*time.Hour)
	draft := filepath.Join(incoming, strings.Repeat("d", 64))
	writeTestFile(t, filepath.Join(draft, "bundle.draft.json"), 100)
	ageBy(t, draft, 90*24*time.Hour)
	// A payload dir whose release is not staged, and a `<digest>` dir with no
	// marker: the runtime owns every `<digest>` dir that is not a staged payload.
	// A refused or aborted platform update leaves a markerless payload with no
	// staged release: past the window it goes; inside it, it may still be written.
	unstaged := filepath.Join(incoming, strings.Repeat("9", 64))
	writeTestFile(t, filepath.Join(unstaged, "bin", "choir"), 100)
	ageBy(t, unstaged, 90*24*time.Hour)
	youngPayload := filepath.Join(incoming, strings.Repeat("8", 64))
	writeTestFile(t, filepath.Join(youngPayload, "bin", "choir"), 100)
	ageBy(t, youngPayload, time.Minute)
	// A host build inside its window, and an old one with one fresh file (a
	// download in progress).
	young := filepath.Join(incoming, "host-build-young")
	writeTestFile(t, filepath.Join(young, "closure.nar"), 100)
	ageBy(t, young, time.Minute)
	busy := filepath.Join(incoming, "host-build-busy")
	writeTestFile(t, filepath.Join(busy, "old"), 100)
	writeTestFile(t, filepath.Join(busy, "deep", "fresh"), 100)
	ageBy(t, filepath.Join(busy, "old"), old)
	ageBy(t, busy, old)
	if err := os.Chtimes(filepath.Join(busy, "deep", "fresh"), pruneNow, pruneNow); err != nil {
		t.Fatal(err)
	}
	// Unrecognised entries are not an unbounded bucket: nothing writes them, so
	// once outside the window they go.
	unknown := filepath.Join(incoming, "operation-7")
	writeTestFile(t, filepath.Join(unknown, "f"), 100)
	ageBy(t, unknown, old)
	loose := filepath.Join(incoming, "loose-file")
	writeTestFile(t, loose, 100)
	ageBy(t, loose, old)

	report := pruneRoot(root)
	for _, path := range []string{bundle, draft, youngPayload, young, busy} {
		if !exists(path) {
			t.Fatalf("%s was removed", path)
		}
	}
	for _, path := range []string{unknown, loose, unstaged} {
		if exists(path) {
			t.Fatalf("%s survived", path)
		}
	}
	if report.IncomingRemoved != 3 {
		t.Fatalf("report = %+v", report)
	}
}

func TestPruneKeepsOnlyCurrentAndItsRecordedPredecessor(t *testing.T) {
	u, root := newPruneUpdater(t)
	var digests []string
	for _, name := range []string{"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7"} {
		digests = append(digests, stageTestRelease(t, u, root, name, false))
	}
	for i := 1; i < len(digests); i++ {
		writeTestJournal(t, root, "idem-"+digests[i], digests[i-1], digests[i], pruneNow.Add(time.Duration(i-10)*time.Hour), "applied")
	}
	setCurrent(t, root, digests[7])
	for _, d := range digests {
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
	}
	// A release staged a minute ago, waiting for its swap (a baseline), is inside its window.
	staged := stageTestRelease(t, u, root, "baseline", false)
	ageBy(t, filepath.Join(root, "releases", staged), time.Minute)

	report := pruneRoot(root)
	for i, d := range digests {
		want := i >= 6
		if exists(filepath.Join(root, "releases", d)) != want {
			t.Fatalf("release r%d kept=%v want %v", i, !want, want)
		}
	}
	if !exists(filepath.Join(root, "releases", staged)) {
		t.Fatal("a release staged a minute ago, awaiting its swap, was removed")
	}
	if report.ReleasesRemoved != 6 {
		t.Fatalf("report = %+v", report)
	}
	if got, _, err := currentReleaseOf(root); err != nil || got != digests[7] {
		t.Fatalf("current = %q, %v", got, err)
	}
	if _, err := ReadCurrentManifest(root); err != nil {
		t.Fatalf("current release no longer reads: %v", err)
	}
	if _, _, err := ReadPinnedManifest(root, digests[6]); err != nil {
		t.Fatalf("the rollback predecessor is gone: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "releases"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Fatalf("leftover %s in releases", entry.Name())
		}
	}
}

func TestPruneKeepsThePreMigrationReleaseOfAReachableRollbackOnly(t *testing.T) {
	u, root := newPruneUpdater(t)
	pre := stageTestRelease(t, u, root, "pre", false)
	done := stageTestRelease(t, u, root, "migrated", false)
	rolled := stageTestRelease(t, u, root, "oldpre", false)
	for _, d := range []string{pre, done, rolled} {
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
	}
	setCurrent(t, root, done)
	record := func(name, outcome, pre string) {
		if err := os.MkdirAll(filepath.Join(root, "package-migration"), 0o700); err != nil {
			t.Fatal(err)
		}
		raw := `{"attempts":[{"outcome":"` + outcome + `","pre_release_digest":"` + pre + `","served_release_digest":"` + done + `"}]}`
		if err := os.WriteFile(filepath.Join(root, "package-migration", name+".json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	record("a", "done", pre)
	record("b", "rolled_back", rolled)
	pruneRoot(root)
	if !exists(filepath.Join(root, "releases", pre)) {
		t.Fatal("the pre-migration release of a reachable rollback was removed")
	}
	if exists(filepath.Join(root, "releases", rolled)) {
		t.Fatal("the release of a rolled-back migration survived")
	}
	// An unreadable record means no pin is known: nothing is removed.
	extra := stageTestRelease(t, u, root, "extra", false)
	ageBy(t, filepath.Join(root, "releases", extra), 30*24*time.Hour)
	if err := os.WriteFile(filepath.Join(root, "package-migration", "c.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	pruneRoot(root)
	if !exists(filepath.Join(root, "releases", extra)) {
		t.Fatal("a release was removed although a migration record could not be read")
	}
}

func TestPruneCollectsStoreObjectsNoKeptReleaseRoots(t *testing.T) {
	u, root := newPruneUpdater(t)
	var digests []string
	for _, name := range []string{"r0", "r1", "r2", "r3", "r4", "r5"} {
		digests = append(digests, stageTestRelease(t, u, root, name, false))
	}
	for i := 1; i < len(digests); i++ {
		writeTestJournal(t, root, "idem-"+digests[i], digests[i-1], digests[i], pruneNow.Add(time.Duration(i-10)*time.Hour), "applied")
	}
	setCurrent(t, root, digests[5])
	// r0 (to be deleted) roots "only-old" and "shared"; r5 (kept) roots "shared" and "only-new".
	rootStoreObjects(t, root, digests[0], "only-old", "shared")
	rootStoreObjects(t, root, digests[5], "shared", "only-new")
	// The kept predecessor (r4) roots its own object.
	rootStoreObjects(t, root, digests[4], "pred-only")
	ageBy(t, filepath.Join(root, "store", "pred-only"), old)
	// An unrooted old object, an unrooted young one (a candidate build in use),
	// and a stale staging dir.
	writeTestFile(t, filepath.Join(root, "store", "orphan-old", "f"), 700)
	ageBy(t, filepath.Join(root, "store", "orphan-old"), old)
	writeTestFile(t, filepath.Join(root, "store", "orphan-young", "f"), 700)
	ageBy(t, filepath.Join(root, "store", "orphan-young"), time.Minute)
	writeTestFile(t, filepath.Join(root, "store", ".staging-abc", "f"), 700)
	ageBy(t, filepath.Join(root, "store", ".staging-abc"), old)
	for _, d := range digests {
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
	}
	ageBy(t, filepath.Join(root, "store", "only-old"), old)
	ageBy(t, filepath.Join(root, "store", "shared"), old)
	ageBy(t, filepath.Join(root, "store", "only-new"), old)
	// make the store objects read-only like real ones
	for _, base := range []string{"only-old", "shared", "only-new", "orphan-old"} {
		_ = os.Chmod(filepath.Join(root, "store", base, "bin"), 0o555)
		_ = os.Chmod(filepath.Join(root, "store", base), 0o555)
	}

	report := pruneRoot(root)
	if exists(filepath.Join(root, "releases", digests[0])) || exists(filepath.Join(root, "gc-roots", digests[0])) {
		t.Fatal("the deleted release or its gc-roots survived")
	}
	for _, gone := range []string{"only-old", "orphan-old", ".staging-abc"} {
		if exists(filepath.Join(root, "store", gone)) {
			t.Fatalf("store/%s survived", gone)
		}
	}
	for _, kept := range []string{"shared", "only-new", "pred-only", "orphan-young"} {
		if !exists(filepath.Join(root, "store", kept)) {
			t.Fatalf("store/%s was collected", kept)
		}
	}
	if !exists(filepath.Join(root, "gc-roots", digests[5], "shared")) {
		t.Fatal("a kept release lost its gc-root")
	}
	if report.StoreRemoved != 3 || report.GCRootsRemoved != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestPruneLeavesTheStoreAloneWhenAKeptLayeredReleaseIsUnrooted(t *testing.T) {
	u, root := newPruneUpdater(t)
	layered := stageTestRelease(t, u, root, "layered", true)
	setCurrent(t, root, layered)
	writeTestFile(t, filepath.Join(root, "store", "aaaa-layered", "bin", "autoputer"), 100)
	writeTestFile(t, filepath.Join(root, "store", "orphan", "f"), 100)
	ageBy(t, filepath.Join(root, "store"), old)
	for _, name := range []string{"aaaa-layered", "orphan"} {
		ageBy(t, filepath.Join(root, "store", name), old)
	}
	pruneRoot(root)
	if !exists(filepath.Join(root, "store", "aaaa-layered", "bin", "autoputer")) || !exists(filepath.Join(root, "store", "orphan")) {
		t.Fatal("the store was collected although a kept layered release has no gc-roots")
	}
	// Once its entrypoint is rooted the collection runs.
	rootStoreObjects(t, root, layered, "aaaa-layered")
	pruneRoot(root)
	if !exists(filepath.Join(root, "store", "aaaa-layered", "bin", "autoputer")) {
		t.Fatal("the rooted entrypoint was collected")
	}
	if exists(filepath.Join(root, "store", "orphan")) {
		t.Fatal("the orphan survived a collection that was allowed to run")
	}
}

func TestPruneSkipsOnlyOnHeldKeysAndBrokenJournals(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	stale := filepath.Join(root, "incoming", ".stage-old")
	writeTestFile(t, filepath.Join(stale, "f"), 100)
	ageBy(t, stale, old)
	// An unreadable journal: its release is unknown, so the prune stands down.
	if err := os.WriteFile(filepath.Join(root, "operations", safeName("broken")+".json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := pruneRoot(root)
	if !exists(stale) || report.Skipped == "" || report.BytesFreed != 0 {
		t.Fatalf("pruned under an unreadable journal: %+v", report)
	}
	if err := os.Remove(filepath.Join(root, "operations", safeName("broken")+".json")); err != nil {
		t.Fatal(err)
	}
	// A held apply key (an apply running, or a settle) skips too.
	unlock, acquired, err := lockApplyKey(root, "running", true)
	if err != nil || !acquired {
		t.Fatalf("lock: %v %v", acquired, err)
	}
	report = pruneRoot(root)
	unlock()
	if !exists(stale) || report.Skipped == "" {
		t.Fatalf("pruned under a held apply key: %+v", report)
	}
	if report = pruneRoot(root); exists(stale) || report.Skipped != "" {
		t.Fatalf("a settled updater did not prune: %+v", report)
	}
}

// A platform update that crashed mid-apply is left pending on purpose
// (platform_update.go resumes it): its journal has no result, nobody holds its
// key. That must not disable the prune for good, and what its journal names
// (the release it was installing and the one it restores to) must survive.
func TestPruneIsNotDisabledByADeadJournalAndKeepsWhatItNames(t *testing.T) {
	u, root := newPruneUpdater(t)
	var digests []string
	for _, name := range []string{"r0", "r1", "r2", "r3", "r4"} {
		digests = append(digests, stageTestRelease(t, u, root, name, false))
		ageBy(t, filepath.Join(root, "releases", digests[len(digests)-1]), 30*24*time.Hour)
	}
	for i := 1; i < 4; i++ {
		writeTestJournal(t, root, "idem-"+digests[i], digests[i-1], digests[i], pruneNow.Add(time.Duration(i-10)*time.Hour), "applied")
	}
	setCurrent(t, root, digests[3])
	// The dead journal: installing r4 over r3 died after the swap prepare.
	dead := operationJournal{Phase: "prepared", PriorReleaseDigest: digests[3], PriorReleaseTarget: filepath.Join(root, "releases", digests[3]),
		TargetReleaseDigest: digests[4], StartedAt: pruneNow.Add(-72 * time.Hour)}
	if err := writeJournal(filepath.Join(root, "operations", safeName("dead")+".json"), dead); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, "incoming", ".stage-old")
	writeTestFile(t, filepath.Join(stale, "f"), 100)
	ageBy(t, stale, old)
	report := pruneRoot(root)
	if report.Skipped != "" || exists(stale) {
		t.Fatalf("a dead journal disabled the prune: %+v", report)
	}
	if report.DeadJournals != 1 {
		t.Fatalf("the pin of a dead journal is not reported: %+v", report)
	}
	for i, want := range []bool{false, false, true, true, true} {
		if exists(filepath.Join(root, "releases", digests[i])) != want {
			t.Fatalf("release r%d kept=%v want %v", i, !want, want)
		}
	}
}

func TestPruneLeavesReleasesAloneWhenCurrentIsUnreadable(t *testing.T) {
	u, root := newPruneUpdater(t)
	var digests []string
	for _, name := range []string{"r0", "r1", "r2", "r3", "r4", "r5", "r6"} {
		digests = append(digests, stageTestRelease(t, u, root, name, false))
		ageBy(t, filepath.Join(root, "releases", digests[len(digests)-1]), 30*24*time.Hour)
	}
	// No current pointer at all, then one that escapes the release store.
	for _, link := range []string{"", "/etc"} {
		if link != "" {
			_ = os.Remove(filepath.Join(root, "current"))
			if err := os.Symlink(link, filepath.Join(root, "current")); err != nil {
				t.Fatal(err)
			}
		}
		pruneRoot(root)
		for _, d := range digests {
			if !exists(filepath.Join(root, "releases", d)) {
				t.Fatalf("a release was removed with current=%q", link)
			}
		}
	}
}

func TestPruneFinishesACrashedPruneAndIsIdempotent(t *testing.T) {
	u, root := newPruneUpdater(t)
	var digests []string
	for _, name := range []string{"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7"} {
		digests = append(digests, stageTestRelease(t, u, root, name, false))
	}
	for i := 1; i < len(digests); i++ {
		writeTestJournal(t, root, "idem-"+digests[i], digests[i-1], digests[i], pruneNow.Add(time.Duration(i-20)*time.Hour), "applied")
	}
	setCurrent(t, root, digests[7])
	for _, d := range digests {
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
	}
	rootStoreObjects(t, root, digests[0], "old-only")
	ageBy(t, filepath.Join(root, "store", "old-only"), old)
	// The crash: r0 was renamed out of releases/ and the process died; its
	// gc-roots and a half-deleted tree remain.
	hidden := filepath.Join(root, "releases", ".prune-"+digests[0])
	if err := os.Rename(filepath.Join(root, "releases", digests[0]), hidden); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(hidden, "release-manifest.json")); err != nil {
		// sealed dir: relax it first, as the prune does.
		makeTreeWritable(hidden)
		_ = os.Remove(filepath.Join(hidden, "release-manifest.json"))
	}
	// The computer is bootable and rollback-capable in this state: current and
	// its predecessor verify.
	if _, err := ReadCurrentManifest(root); err != nil {
		t.Fatalf("current does not verify mid-crash: %v", err)
	}
	if _, _, err := ReadPinnedManifest(root, digests[6]); err != nil {
		t.Fatalf("the predecessor is unreadable mid-crash: %v", err)
	}
	pruneRoot(root)
	if exists(hidden) || exists(filepath.Join(root, "gc-roots", digests[0])) || exists(filepath.Join(root, "store", "old-only")) {
		t.Fatal("the next run did not finish the crashed prune")
	}
	for i, d := range digests {
		if exists(filepath.Join(root, "releases", d)) != (i >= 6) {
			t.Fatalf("after the finished prune, release r%d kept=%v", i, !(i >= 6))
		}
	}
	if _, err := ReadCurrentManifest(root); err != nil {
		t.Fatalf("current unreadable after the finished prune: %v", err)
	}
	if _, _, err := ReadPinnedManifest(root, digests[6]); err != nil {
		t.Fatalf("the predecessor is gone after the finished prune: %v", err)
	}
	second := pruneRoot(root)
	if second.BytesFreed != 0 || second.ReleasesRemoved+second.IncomingRemoved+second.StoreRemoved+second.GCRootsRemoved != 0 {
		t.Fatalf("a second run freed something: %+v", second)
	}
}

func newApplyRig(t *testing.T, base string) (*Updater, string, *fakeHealthProber) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	prober := &fakeHealthProber{}
	u, err := NewWithBase(root, "computer-test", "realization-test", &fakeServiceManager{}, prober,
		testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}}, base)
	if err != nil {
		t.Fatal(err)
	}
	u.now = func() time.Time { return pruneNow }
	return u, root, prober
}

func TestApplyDropsItsStagedSource(t *testing.T) {
	u, root, _ := newApplyRig(t, "")
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", "op-1", "idem-1", "one")
	if _, err := u.Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	pruneRoot(root)
	if exists(request.SourceDir) {
		t.Fatal("the staged source stayed in incoming after the release was staged")
	}
	// A pinned release used as the source (a rollback to it) is never dropped.
	pinned := filepath.Join(root, "releases", request.Manifest.ContentDigest)
	if !exists(filepath.Join(pinned, "release-manifest.json")) {
		t.Fatal("the release itself was dropped")
	}
	// A bundle directory is never an apply source and is never dropped.
	bundle := filepath.Join(root, "incoming", strings.Repeat("c", 64))
	writeTestFile(t, filepath.Join(bundle, "bundle.json"), 10)
	u.dropStagedSource(bundle)
	if !exists(bundle) {
		t.Fatal("dropStagedSource removed a bundle")
	}
}

func TestApplyResumesAStagedReleaseWithoutItsSource(t *testing.T) {
	u, root, _ := newApplyRig(t, "")
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", "op-1", "idem-1", "one")
	// The first run staged the release, swapped, and died before the probe;
	// its source was dropped when it staged.
	if err := u.stageRelease(request.SourceDir, filepath.Join(root, "releases", request.Manifest.ContentDigest), request.Manifest); err != nil {
		t.Fatal(err)
	}
	if err := swapCurrentPointer(root, filepath.Join(root, "releases", request.Manifest.ContentDigest)); err != nil {
		t.Fatal(err)
	}
	journal := operationJournal{RequestCommitment: request.RequestCommitment, Phase: "pointer_swapped", TargetReleaseDigest: request.Manifest.ContentDigest,
		RestartPublished: true, StartedAt: pruneNow}
	if err := writeJournal(filepath.Join(root, "operations", safeName("idem-1")+".json"), journal); err != nil {
		t.Fatal(err)
	}
	makeTreeWritable(request.SourceDir)
	if err := os.RemoveAll(request.SourceDir); err != nil {
		t.Fatal(err)
	}
	result, err := u.Apply(context.Background(), request)
	if err != nil || result.Outcome != "applied" {
		t.Fatalf("a resumed apply of a staged release without its source = %+v, %v", result, err)
	}
	pruneRoot(root)
}

// The root's size must not depend on how many updates were applied: after the
// last settled apply it holds current and its predecessor, what they root, and
// nothing else (owner ruling 2026-11-13).
func TestUpdaterRootIsBoundedOverManyApplies(t *testing.T) {
	single, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	dirBlob, err := os.ReadFile(filepath.Join("testdata", "closure-dir.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	baseManifestPath, bootedDigest := layeredTestBase(t)
	u, root, _ := newApplyRig(t, baseManifestPath)
	probeStore := filepath.Join(t.TempDir(), "probe-store")
	t.Cleanup(func() { makeTreeWritable(probeStore) })
	dirPaths, err := MaterializeClosure(dirBlob, probeStore)
	if err != nil || len(dirPaths) != 1 {
		t.Fatalf("dir fixture: %v %v", dirPaths, err)
	}
	t.Cleanup(func() { makeTreeWritable(filepath.Dir(root)) })
	census := func() (releases, incoming, gcRoots, store int) {
		count := func(dir string) int {
			entries, _ := os.ReadDir(filepath.Join(root, dir))
			n := 0
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), "host-build-") {
					n++
				}
			}
			return n
		}
		return count("releases"), count("incoming"), count("gc-roots"), count("store")
	}
	const applies = 12
	var afterSecond [4]int
	// A platform update that crashed mid-apply long ago: its journal has no
	// result and nobody holds its key (it names releases that are not here).
	dead := operationJournal{Phase: "prepared", PriorReleaseDigest: strings.Repeat("e", 64), TargetReleaseDigest: strings.Repeat("f", 64), StartedAt: pruneNow.Add(-90 * 24 * time.Hour)}
	if err := writeJournal(filepath.Join(root, "operations", safeName("dead")+".json"), dead); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= applies; i++ {
		// Between applies the runtime leaves its usual residue: an old host
		// build, an old payload of a release it staged, a candidate build's
		// store object no release roots.
		name := fmt.Sprintf("op-%02d", i)
		if i > 1 {
			residue := filepath.Join(root, "incoming", fmt.Sprintf(".stage-%02d", i))
			writeTestFile(t, filepath.Join(residue, "closure.nar"), 4096)
			ageBy(t, residue, old)
			// A real host build belongs to the runtime (collectSettledBundles
			// removes it): the prune leaves it, and the census below does not
			// count it.
			build := filepath.Join(root, "incoming", fmt.Sprintf("host-build-%02d", i))
			writeTestFile(t, filepath.Join(build, "closure.nar"), 4096)
			ageBy(t, build, old)
			orphan := filepath.Join(root, "store", fmt.Sprintf("orphan-%02d", i))
			writeTestFile(t, filepath.Join(orphan, "f"), 1024)
			ageBy(t, orphan, old)
		}
		// A refused or aborted platform update: its payload under
		// incoming/<release digest>, no release staged, no bundle marker.
		refused := filepath.Join(root, "incoming", fmt.Sprintf("%064x", i))
		writeTestFile(t, filepath.Join(refused, "bin", "choir"), 2048)
		ageBy(t, refused, old)
		blob, entry := single, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt"
		if i%2 == 0 {
			blob, entry = dirBlob, dirPaths[0]
		}
		request := layeredRequestFixture(t, root, "computer-test", "realization-test", name, bootedDigest, blob, entry)
		if result, err := u.Apply(context.Background(), request); err != nil || result.Outcome != "applied" {
			t.Fatalf("apply %d = %+v, %v", i, result, err)
		}
		pruneRoot(root)
		if i == 2 {
			afterSecond[0], afterSecond[1], afterSecond[2], afterSecond[3] = census()
		}
	}
	releases, incoming, gcRoots, store := census()
	if releases != 2 || incoming != 0 || gcRoots != 2 || store != 2 {
		t.Fatalf("after %d applies: releases=%d incoming=%d gc-roots=%d store=%d; want 2/0/2/2", applies, releases, incoming, gcRoots, store)
	}
	if got := [4]int{releases, incoming, gcRoots, store}; got != afterSecond {
		t.Fatalf("the root after %d applies %v differs from after 2 applies %v", applies, got, afterSecond)
	}
}

func TestRollbackStillWorksAfterThePrune(t *testing.T) {
	single, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	dirBlob, err := os.ReadFile(filepath.Join("testdata", "closure-dir.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	baseManifestPath, bootedDigest := layeredTestBase(t)
	u, root, prober := newApplyRig(t, baseManifestPath)
	probeStore := filepath.Join(t.TempDir(), "probe-store")
	t.Cleanup(func() { makeTreeWritable(probeStore) })
	dirPaths, err := MaterializeClosure(dirBlob, probeStore)
	if err != nil || len(dirPaths) != 1 {
		t.Fatalf("dir fixture: %v %v", dirPaths, err)
	}
	var last ApplyRequest
	for i := 1; i <= 4; i++ {
		blob, entry := single, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt"
		if i%2 == 0 {
			blob, entry = dirBlob, dirPaths[0]
		}
		last = layeredRequestFixture(t, root, "computer-test", "realization-test", fmt.Sprintf("op-%d", i), bootedDigest, blob, entry)
		if result, err := u.Apply(context.Background(), last); err != nil || result.Outcome != "applied" {
			t.Fatalf("apply %d = %+v, %v", i, result, err)
		}
		pruneRoot(root)
	}
	wantEntry := currentEntrypoint(t, root)
	// The next apply fails its probe: recovery restores the predecessor the
	// prune kept, with its store object and its entrypoint.
	bad := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-bad", bootedDigest, single, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	prober.failDigest = bad.Manifest.ContentDigest
	result, err := u.Apply(context.Background(), bad)
	if err == nil || result.Outcome != "failed" || result.RecoveryReceipt == nil {
		t.Fatalf("failed apply = %+v, %v", result, err)
	}
	if got := currentEntrypoint(t, root); got != wantEntry {
		t.Fatalf("entrypoint after recovery = %q, want %q", got, wantEntry)
	}
	if _, err := os.Stat(wantEntry); err != nil {
		t.Fatalf("the restored release's store object is gone: %v", err)
	}
	if got, _, err := currentReleaseOf(root); err != nil || got != last.Manifest.ContentDigest {
		t.Fatalf("current after recovery = %q, %v", got, err)
	}
}

// The accepted price of a bounded root: a restore to a release the prune
// removed refuses cleanly, names why, and changes nothing.
func TestRestoreToAPrunedReleaseRefusesCleanly(t *testing.T) {
	u, root := newPruneUpdater(t)
	var digests []string
	for _, name := range []string{"r0", "r1", "r2", "r3"} {
		digests = append(digests, stageTestRelease(t, u, root, name, false))
	}
	for i := 1; i < len(digests); i++ {
		writeTestJournal(t, root, "idem-"+digests[i], digests[i-1], digests[i], pruneNow.Add(time.Duration(i-10)*time.Hour), "applied")
	}
	for _, d := range digests {
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
	}
	setCurrent(t, root, digests[3])
	pruneRoot(root)
	if exists(filepath.Join(root, "releases", digests[0])) {
		t.Fatal("r0 should be pruned")
	}
	before, _ := os.Readlink(filepath.Join(root, "current"))
	err := RestagePinnedRelease(root, digests[0])
	if !errors.Is(err, ErrPinnedReleaseUnavailable) {
		t.Fatalf("restage of a pruned release = %v, want ErrPinnedReleaseUnavailable", err)
	}
	if _, _, err := ReadPinnedManifest(root, digests[0]); !errors.Is(err, ErrPinnedReleaseUnavailable) {
		t.Fatalf("ReadPinnedManifest = %v", err)
	}
	if after, _ := os.Readlink(filepath.Join(root, "current")); after != before {
		t.Fatalf("current moved from %s to %s", before, after)
	}
}

// S3: the predecessor is the Prior of the newest apply (or restage) that
// installed current, whatever order the directory lists journals in, and a
// failed apply names no predecessor.
func TestPruneKeepsThePredecessorAcrossFailedAndRepeatedApplies(t *testing.T) {
	u, root := newPruneUpdater(t)
	var d []string
	for _, name := range []string{"r1", "r2", "r3"} {
		d = append(d, stageTestRelease(t, u, root, name, false))
		ageBy(t, filepath.Join(root, "releases", d[len(d)-1]), 30*24*time.Hour)
	}
	r1, r2, r3 := d[0], d[1], d[2]
	// r1 -> r2 applied; r3 -> r2 failed and was restored to r3; later current is r2.
	writeTestJournal(t, root, "a-r1-r2", r1, r2, pruneNow.Add(-10*time.Hour), "applied")
	writeTestJournal(t, root, "b-r3-r2-failed", r3, r2, pruneNow.Add(-5*time.Hour), "failed")
	setCurrent(t, root, r2)
	pruneRoot(root)
	if !exists(filepath.Join(root, "releases", r1)) || exists(filepath.Join(root, "releases", r3)) {
		t.Fatalf("a failed apply was taken for the predecessor: r1=%v r3=%v", exists(filepath.Join(root, "releases", r1)), exists(filepath.Join(root, "releases", r3)))
	}
	// r1 -> r2 -> r1: the newest apply installing r1 names r2.
	u2, root2 := newPruneUpdater(t)
	var e []string
	for _, name := range []string{"r1", "r2", "r3"} {
		e = append(e, stageTestRelease(t, u2, root2, name, false))
		ageBy(t, filepath.Join(root2, "releases", e[len(e)-1]), 30*24*time.Hour)
	}
	writeTestJournal(t, root2, "z-first", e[2], e[0], pruneNow.Add(-30*time.Hour), "applied")
	writeTestJournal(t, root2, "a-second", e[0], e[1], pruneNow.Add(-20*time.Hour), "applied")
	writeTestJournal(t, root2, "m-third", e[1], e[0], pruneNow.Add(-10*time.Hour), "applied")
	setCurrent(t, root2, e[0])
	pruneRoot(root2)
	if !exists(filepath.Join(root2, "releases", e[1])) || exists(filepath.Join(root2, "releases", e[2])) {
		t.Fatal("after r1 -> r2 -> r1 the predecessor must be r2 alone")
	}
}

// S5: a restage swaps current with no apply. It records where it came from, so
// the one predecessor survives a boot frontend follow or a tape restore.
func TestRestagePinnedReleaseRecordsItsPredecessor(t *testing.T) {
	u, root := newPruneUpdater(t)
	r1 := stageTestRelease(t, u, root, "r1", false)
	r2 := stageTestRelease(t, u, root, "r2", false)
	r3 := stageTestRelease(t, u, root, "r3", false)
	for _, d := range []string{r1, r2, r3} {
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
	}
	setCurrent(t, root, r1)
	if err := RestagePinnedRelease(root, r2); err != nil {
		t.Fatal(err)
	}
	pruneRoot(root)
	if !exists(filepath.Join(root, "releases", r1)) || exists(filepath.Join(root, "releases", r3)) {
		t.Fatal("after a restage the release it came from must be the kept predecessor")
	}
	if name, unsettled := UnsettledApply(root); unsettled {
		t.Fatalf("a restage record reads as an unsettled apply: %s", name)
	}
}

// S4: ImportBaseline swaps current, so it takes the updater's mutex.
// S7: bytes freed are allocated bytes (st_blocks), as the data_volume section
// reports them: a sparse file of a megabyte frees almost nothing.
func TestPruneReportsAllocatedBytes(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	sparse := filepath.Join(root, "incoming", ".stage-sparse", "big")
	if err := os.MkdirAll(filepath.Dir(sparse), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(sparse)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	ageBy(t, filepath.Dir(sparse), old)
	if report := pruneRoot(root); report.BytesFreed >= 1<<20 {
		t.Fatalf("a sparse file was counted by its apparent size: %+v", report)
	}
}

// B4 at the source: an apply refused before it stages anything drops its
// payload; the caller restages on a retry.
func TestApplyDropsARefusedPayload(t *testing.T) {
	baseManifestPath, _ := layeredTestBase(t)
	u, root, _ := newApplyRig(t, baseManifestPath)
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", "op-refused", "idem-refused", "refused")
	request.Manifest.BaseImageManifestDigest = strings.Repeat("0", 64)
	request.Manifest.ClosureDigest = strings.Repeat("1", 64)
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Apply(context.Background(), request); err == nil {
		t.Fatal("the mismatched base was accepted")
	}
	if exists(request.SourceDir) {
		t.Fatal("a refused apply left its payload in incoming")
	}
}

// G1: the kept predecessors are a union, not a newest-wins choice. A restage
// and its swap-back (a package migration that is rolled back, a tape restore
// that fails) journal "B -> R2" newer than the apply "R1 -> R2"; the revert of
// the operation that applied R2 still reads R1.
func TestPruneKeepsTheAppliedPredecessorAcrossARestageAndSwapBack(t *testing.T) {
	u, root := newPruneUpdater(t)
	r1 := stageTestRelease(t, u, root, "r1", false)
	r2 := stageTestRelease(t, u, root, "r2", false)
	b := stageTestRelease(t, u, root, "base", false)
	r9 := stageTestRelease(t, u, root, "unrelated", false)
	for _, d := range []string{r1, r2, b, r9} {
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
	}
	writeTestJournal(t, root, "op2", r1, r2, pruneNow.Add(-10*time.Hour), "applied")
	setCurrent(t, root, r2)
	// A migration swaps to the baseline and is rolled back; a tape restore swaps
	// and fails: each swap-back journals "x -> R2".
	if err := RestagePinnedRelease(root, b); err != nil {
		t.Fatal(err)
	}
	if err := RestagePinnedRelease(root, r2); err != nil {
		t.Fatal(err)
	}
	pruneRoot(root)
	if !exists(filepath.Join(root, "releases", r1)) {
		t.Fatal("the release the operation that applied current reverts to was pruned after a swap and swap-back")
	}
	if !exists(filepath.Join(root, "releases", b)) {
		t.Fatal("the release current was restaged from was not kept")
	}
	if exists(filepath.Join(root, "releases", r9)) {
		t.Fatal("an unrelated release survived: the union is at most three")
	}
	if _, _, err := ReadPinnedManifest(root, r1); err != nil {
		t.Fatalf("the revert input is unreadable: %v", err)
	}
}

// A restage that cannot be journaled refuses before it swaps (the caller can
// still refuse); the swap-back is best effort.
func TestRestagePinnedReleaseRefusesWhenItCannotJournalButSwapBackProceeds(t *testing.T) {
	u, root := newPruneUpdater(t)
	r1 := stageTestRelease(t, u, root, "r1", false)
	r2 := stageTestRelease(t, u, root, "r2", false)
	setCurrent(t, root, r1)
	if err := os.RemoveAll(filepath.Join(root, "operations")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "operations"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RestagePinnedRelease(root, r2); err == nil {
		t.Fatal("a restage that could not be journaled swapped anyway")
	}
	if got, _, _ := currentReleaseOf(root); got != r1 {
		t.Fatalf("current moved to %s", got)
	}
	if err := RestagePinnedReleaseBestEffort(root, r2); err != nil {
		t.Fatalf("the swap-back must proceed: %v", err)
	}
	if got, _, _ := currentReleaseOf(root); got != r2 {
		t.Fatalf("current = %s after the swap-back", got)
	}
}

// A leading slash (or `./`) in LayeringEntrypoint is resolved the same way at
// run time (resolveLayeringEntrypoint); the prune must read the same first
// segment, or the store is never collected on a computer whose release declares
// one.
func TestPruneStoreCollectionSurvivesALeadingSlashEntrypoint(t *testing.T) {
	for _, entrypoint := range []string{"/aaaa-layered/bin/autoputer", "./aaaa-layered/bin/autoputer", "//aaaa-layered/bin/autoputer"} {
		u, root := newPruneUpdater(t)
		layered := stageTestReleaseEntrypoint(t, u, root, "layered", entrypoint)
		setCurrent(t, root, layered)
		rootStoreObjects(t, root, layered, "aaaa-layered")
		writeTestFile(t, filepath.Join(root, "store", "orphan", "f"), 100)
		for _, name := range []string{"aaaa-layered", "orphan"} {
			ageBy(t, filepath.Join(root, "store", name), old)
		}
		report := pruneRoot(root)
		if exists(filepath.Join(root, "store", "orphan")) || report.StoreRemoved != 1 {
			t.Fatalf("entrypoint %q: the store collection was disabled: %+v", entrypoint, report)
		}
		if !exists(filepath.Join(root, "store", "aaaa-layered", "bin", "tool")) {
			t.Fatalf("entrypoint %q: the rooted entrypoint object was collected", entrypoint)
		}
	}
	// An entrypoint that does not resolve (it escapes the store) is never taken
	// for a root: the collection stands down.
	u, root := newPruneUpdater(t)
	escaping := stageTestReleaseEntrypoint(t, u, root, "escaping", "../etc/passwd")
	setCurrent(t, root, escaping)
	rootStoreObjects(t, root, escaping, "aaaa-other")
	writeTestFile(t, filepath.Join(root, "store", "orphan", "f"), 100)
	ageBy(t, filepath.Join(root, "store", "orphan"), old)
	pruneRoot(root)
	if !exists(filepath.Join(root, "store", "orphan")) {
		t.Fatal("the store was collected under an entrypoint that does not resolve")
	}
}

// fakeClock advances by step on every read: a budget of n steps allows about n
// checks, so a test stops a prune between two files without sleeping.
func fakeClock(step time.Duration) func() time.Time {
	now := time.Unix(1_700_000_000, 0)
	return func() time.Time {
		now = now.Add(step)
		return now
	}
}

func staleIncoming(t *testing.T, root string, n, files int) []string {
	t.Helper()
	var paths []string
	for i := 0; i < n; i++ {
		dir := filepath.Join(root, "incoming", fmt.Sprintf(".stage-%02d", i))
		for f := 0; f < files; f++ {
			writeTestFile(t, filepath.Join(dir, fmt.Sprintf("f%02d", f)), 4096)
		}
		ageBy(t, dir, old)
		paths = append(paths, dir)
	}
	return paths
}

func hiddenLeftovers(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	for _, dir := range []string{"incoming", "releases", "gc-roots", "store"} {
		entries, _ := os.ReadDir(filepath.Join(root, dir))
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), pruneHiddenPrefix) {
				found = append(found, filepath.Join(root, dir, entry.Name()))
			}
		}
	}
	return found
}

func TestPruneStopsAtItsBudgetAndTheNextRunFinishes(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	paths := staleIncoming(t, root, 6, 5)
	first := Prune(PruneOptions{Root: root, Now: func() time.Time { return pruneNow }, Clock: fakeClock(time.Second), Budget: 20 * time.Second})
	if !first.BudgetExhausted || first.Hidden == 0 {
		t.Fatalf("the budget did not stop the prune: %+v", first)
	}
	if first.Entries() == 0 || first.Entries() >= 6 {
		t.Fatalf("the budget should stop between entries, after some work: %+v", first)
	}
	for _, path := range paths {
		if exists(path) {
			t.Fatalf("%s is still visible under its real name after the run decided to delete it", path)
		}
	}
	if len(hiddenLeftovers(t, root)) == 0 {
		t.Fatal("a stopped prune left no leftovers to finish")
	}
	if _, err := ReadCurrentManifest(root); err != nil {
		t.Fatalf("current unreadable after a stopped prune: %v", err)
	}
	second := pruneRoot(root)
	if second.BudgetExhausted || second.Hidden != 0 || second.Errors != 0 {
		t.Fatalf("the second run did not finish: %+v", second)
	}
	if first.Entries()+second.Entries() != 6 {
		t.Fatalf("entries across the two runs = %d + %d, want 6", first.Entries(), second.Entries())
	}
	if left := hiddenLeftovers(t, root); len(left) != 0 {
		t.Fatalf("leftovers after the second run: %v", left)
	}
	if first.BytesFreed+second.BytesFreed < 6*5*4096 {
		t.Fatalf("bytes freed across the runs = %d + %d", first.BytesFreed, second.BytesFreed)
	}
}

// MaterializeClosure skips a store object that exists under its name, so a
// half-deleted one under its real name would be reused as whole. Every deletion
// is a rename first; a prune killed or stopped at any point leaves the name
// absent.
func TestPruneNeverLeavesAHalfDeletedObjectUnderItsRealName(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	for i := 0; i < 3; i++ {
		for f := 0; f < 20; f++ {
			writeTestFile(t, filepath.Join(root, "store", fmt.Sprintf("orphan-%d", i), "bin", fmt.Sprintf("f%02d", f)), 100)
		}
		ageBy(t, filepath.Join(root, "store", fmt.Sprintf("orphan-%d", i)), old)
	}
	for _, budget := range []time.Duration{time.Nanosecond, 3 * time.Second, 12 * time.Second, 40 * time.Second} {
		report := Prune(PruneOptions{Root: root, Now: func() time.Time { return pruneNow }, Clock: fakeClock(time.Second), Budget: budget})
		for i := 0; i < 3; i++ {
			object := filepath.Join(root, "store", fmt.Sprintf("orphan-%d", i))
			if !exists(object) {
				continue // renamed away (hidden or deleted): never partial
			}
			files, _ := os.ReadDir(filepath.Join(object, "bin"))
			if len(files) != 20 {
				t.Fatalf("budget %s: store/orphan-%d is partial under its real name (%d of 20 files): %+v", budget, i, len(files), report)
			}
		}
	}
	if left := hiddenLeftovers(t, root); len(left) != 0 {
		t.Logf("leftovers remain for the next boot: %v", left)
	}
}

// A killed run leaves invisible `.prune-*` entries in any of the four
// directories, some half deleted and some sealed read-only; the next run
// finishes all of them whatever their age.
func TestPruneFinishesLeftoversInEveryDirectory(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	made := []string{
		filepath.Join(root, "incoming", ".prune-"+strings.Repeat("a", 64)),
		filepath.Join(root, "releases", ".prune-"+strings.Repeat("b", 64)),
		filepath.Join(root, "gc-roots", ".prune-"+strings.Repeat("c", 64)),
		filepath.Join(root, "store", ".prune-abc-thing"),
	}
	for _, dir := range made {
		writeTestFile(t, filepath.Join(dir, "bin", "tool"), 500)
		if err := os.Chmod(filepath.Join(dir, "bin"), 0o555); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
	}
	// Young leftovers are finished too: a hidden entry is already decided.
	report := pruneRoot(root)
	for _, dir := range made {
		if exists(dir) {
			t.Fatalf("%s survived", dir)
		}
	}
	if report.Entries() != 4 || report.BytesFreed < 4*500 {
		t.Fatalf("report = %+v", report)
	}
}

func TestPruneReportLine(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	writeTestFile(t, filepath.Join(root, "incoming", ".host-build-a", "f"), 4096)
	ageBy(t, filepath.Join(root, "incoming", ".host-build-a"), old)
	line := pruneRoot(root).Line()
	for _, want := range []string{"updater: prune:", "releases_kept=1", "entries_deleted=1", "incoming=1", "releases=0", "gc_roots=0", "store=0", "bytes_freed=4096", "hidden_left=0", "errors=0", "elapsed="} {
		if !strings.Contains(line, want) || strings.Contains(line, "\n") {
			t.Fatalf("line %q lacks %q", line, want)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "operations", safeName("broken")+".json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if line := pruneRoot(root).Line(); !strings.Contains(line, "skipped (journal ") {
		t.Fatalf("a skip does not say why: %q", line)
	}
}
