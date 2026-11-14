package updater

// Failure list for the updater's prune
// (docs/problems/updater-directory-fills-the-data-volume-2026-11-13.md). Each
// item is a thing the updater does wrong today (nothing ever deletes from
// incoming/, releases/, store/ or gc-roots/) or a way a prune could break a
// computer, and the test that pins it:
//
//  F1  a verified platform payload under incoming/<release digest> stays
//      forever after its release is staged (TestPruneRemovesStagedPayloadOnlyAfterItsReleaseExists)
//  F2  a finished host build under incoming/host-build-<job> stays forever
//      (TestPruneRemovesOldHostBuildsAndTemporaries)
//  F12 Apply leaves its source in incoming/ after the release is staged
//      (TestApplyDropsItsStagedSource); a resumed apply of a staged release
//      must not need that source (TestApplyResumesAStagedReleaseWithoutItsSource)
//  F13 the root grows with the number of applies, not with what one leaves
//      (TestUpdaterRootIsBoundedOverManyApplies); rollback still works after
//      (TestRollbackStillWorksAfterThePrune)
//  F3  a prune must never take a bundle (bundle.json: operation evidence the
//      updater cannot judge), an entry it does not recognise, a payload whose
//      release is not staged yet, or anything younger than the grace period
//      (a stage or a host build in progress)
//      (TestPruneLeavesBundlesToTheRuntimeAndKeepsInputsInTheirWindow)
//  F4  every release ever applied stays (TestPruneKeepsOnlyCurrentAndItsRecordedPredecessor,
//      TestPruneKeepsThePreMigrationReleaseOfAReachableRollbackOnly)
//  F5  a deleted release's gc-roots and its exclusively-held store objects
//      stay; a store object a kept release still roots must not go
//      (TestPruneCollectsStoreObjectsNoKeptReleaseRoots)
//  F6  a kept layered release with no gc-root dir must stop the store
//      collection, not lose its binary (TestPruneLeavesTheStoreAloneWhenAKeptLayeredReleaseIsUnrooted)
//  F7  no prune while an apply is unsettled or in flight, and none when
//      `current` cannot be read (TestPruneSkipsWhileAnApplyIsUnsettled, TestPruneLeavesReleasesAloneWhenCurrentIsUnreadable)
//  F8  a crash mid-prune leaves a bootable, rollback-capable computer, and the
//      next run finishes the job (TestPruneFinishesACrashedPruneAndIsIdempotent)
//  F9  one log line with counts and bytes (TestPruneLogsOneLineWithCountsAndBytes)
//  F10 a successful apply runs it, a failed apply does not
//      (TestApplyPrunesAfterAVerifiedApplyOnly)
//  F11 it never runs mid-apply (TestPruneWaitsForAnApplyInFlight)

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// pruneNow is the updater's clock in these tests: an hour after the wall
// clock, so releases and store objects the real code just wrote are older than
// the input window, and ageBy can place a file inside or outside it.
var pruneNow = time.Now().UTC().Add(time.Hour)

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
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", name, "idem-"+name, name)
	if layered {
		request.Manifest.ClosureDigest = strings.Repeat("1", 64)
		request.Manifest.LayeringEntrypoint = "aaaa-" + name + "/bin/autoputer"
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

	report := u.Prune()
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
	u.Prune()
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

	report := u.Prune()
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

	report := u.Prune()
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
	u.Prune()
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
	u.Prune()
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

	report := u.Prune()
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
	u.Prune()
	if !exists(filepath.Join(root, "store", "aaaa-layered", "bin", "autoputer")) || !exists(filepath.Join(root, "store", "orphan")) {
		t.Fatal("the store was collected although a kept layered release has no gc-roots")
	}
	// Once its entrypoint is rooted the collection runs.
	rootStoreObjects(t, root, layered, "aaaa-layered")
	u.Prune()
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
	report := u.Prune()
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
	report = u.Prune()
	unlock()
	if !exists(stale) || report.Skipped == "" {
		t.Fatalf("pruned under a held apply key: %+v", report)
	}
	if report = u.Prune(); exists(stale) || report.Skipped != "" {
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
	report := u.Prune()
	if report.Skipped != "" || exists(stale) {
		t.Fatalf("a dead journal disabled the prune: %+v", report)
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
		u.Prune()
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
	u.Prune()
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
	second := u.Prune()
	if second.BytesFreed != 0 || second.ReleasesRemoved+second.IncomingRemoved+second.StoreRemoved+second.GCRootsRemoved != 0 {
		t.Fatalf("a second run freed something: %+v", second)
	}
}

func TestPruneLogsOneLineWithCountsAndBytes(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	writeTestFile(t, filepath.Join(root, "incoming", ".host-build-a", "f"), 4096)
	ageBy(t, filepath.Join(root, "incoming", ".host-build-a"), old)
	var buffer bytes.Buffer
	log.SetOutput(&buffer)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	u.Prune()
	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "updater: prune") || !strings.Contains(lines[0], "incoming=1") ||
		!strings.Contains(lines[0], "releases=0") || !strings.Contains(lines[0], "store=0") || !strings.Contains(lines[0], "bytes_freed=4096") {
		t.Fatalf("log = %q", buffer.String())
	}
}

func TestApplyPrunesAfterAVerifiedApplyOnly(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	prober := &fakeHealthProber{}
	u, err := New(root, "computer-test", "realization-test", &fakeServiceManager{}, prober,
		testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}})
	if err != nil {
		t.Fatal(err)
	}
	u.now = func() time.Time { return pruneNow }
	makeStale := func() string {
		stale := filepath.Join(root, "incoming", ".host-build-stale")
		writeTestFile(t, filepath.Join(stale, "f"), 100)
		ageBy(t, stale, old)
		return stale
	}
	stale := makeStale()
	good := updaterRequestFixture(t, root, "computer-test", "realization-test", "op-good", "idem-good", "good")
	if _, err := u.Apply(context.Background(), good); err != nil {
		t.Fatal(err)
	}
	u.WaitPrune()
	if exists(stale) {
		t.Fatal("a verified apply did not prune")
	}
	if _, err := ReadCurrentManifest(root); err != nil {
		t.Fatalf("current after prune: %v", err)
	}
	// A failed apply restores the prior release and is not a verified apply.
	stale = makeStale()
	bad := updaterRequestFixture(t, root, "computer-test", "realization-test", "op-bad", "idem-bad", "bad")
	prober.failDigest = bad.Manifest.ContentDigest
	if result, err := u.Apply(context.Background(), bad); err == nil || result.Outcome != "failed" {
		t.Fatalf("the bad apply = %+v, %v", result, err)
	}
	u.WaitPrune()
	if !exists(stale) {
		t.Fatal("a failed apply pruned")
	}
}

func TestPruneWaitsForAnApplyInFlight(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	stale := filepath.Join(root, "incoming", ".host-build-stale")
	writeTestFile(t, filepath.Join(stale, "f"), 100)
	ageBy(t, stale, old)
	u.mu.Lock() // Apply holds this for its whole run
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); u.Prune() }()
	time.Sleep(100 * time.Millisecond)
	if !exists(stale) {
		t.Fatal("the prune ran while an apply held the updater")
	}
	u.mu.Unlock()
	wg.Wait()
	if exists(stale) {
		t.Fatal("the prune did not run after the apply released the updater")
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
	u.WaitPrune()
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
	u.WaitPrune()
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
			return len(entries)
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
			residue := filepath.Join(root, "incoming", fmt.Sprintf(".host-build-%02d", i))
			writeTestFile(t, filepath.Join(residue, "closure.nar"), 4096)
			ageBy(t, residue, old)
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
		u.WaitPrune()
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
		u.WaitPrune()
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

// A restore swaps `current` to an older release and, when it fails, back to the
// release it came from. Nothing in the journals says so (it is no apply), so
// the prune must honour the restore's own hold: with it held, the release the
// restore may swap back to survives.
func TestPruneSkipsWhileARestoreHoldsTheUpdaterRoot(t *testing.T) {
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
	if restoreInFlight(root) {
		t.Fatal("no restore is in flight")
	}
	release, err := HoldRestore(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !restoreInFlight(root) {
		t.Fatal("a held restore is not seen")
	}
	// The restore moves current to the predecessor; the release it came from
	// (r3) is now neither current nor journal-recorded predecessor of r2's
	// install... r1 is. Only the hold keeps r3.
	if err := RestagePinnedRelease(root, digests[2]); err != nil {
		t.Fatal(err)
	}
	report := u.Prune()
	if report.Skipped == "" || !exists(filepath.Join(root, "releases", digests[3])) {
		t.Fatalf("pruned under a restore: %+v", report)
	}
	// The swap-back succeeds.
	if err := RestagePinnedRelease(root, digests[3]); err != nil {
		t.Fatalf("swap-back to the prior release: %v", err)
	}
	release()
	if restoreInFlight(root) {
		t.Fatal("a released restore is still seen")
	}
	if report := u.Prune(); report.Skipped != "" {
		t.Fatalf("a settled updater did not prune: %+v", report)
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
	u.Prune()
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

// B1: the prune does not test the restore hold, it holds it. A restore that
// starts between two chunks of a running prune makes the next chunk stand
// down; the release the restore may swap back to survives. And S1: the updater
// mutex is free between chunks, so an RPC never waits behind a long deletion.
func TestPruneHoldsTheRestoreHoldAndYieldsTheUpdaterBetweenChunks(t *testing.T) {
	u, root := newPruneUpdater(t)
	var digests []string
	for _, name := range []string{"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7"} {
		digests = append(digests, stageTestRelease(t, u, root, name, false))
		ageBy(t, filepath.Join(root, "releases", digests[len(digests)-1]), 30*24*time.Hour)
	}
	for i := 1; i < len(digests); i++ {
		writeTestJournal(t, root, "idem-"+digests[i], digests[i-1], digests[i], pruneNow.Add(time.Duration(i-20)*time.Hour), "applied")
	}
	setCurrent(t, root, digests[7])
	for i := 0; i < 12; i++ {
		temp := filepath.Join(root, "incoming", fmt.Sprintf(".stage-%02d", i))
		writeTestFile(t, filepath.Join(temp, "f"), 100)
		ageBy(t, temp, old)
	}
	chunks := 0
	var release func()
	u.pruneBetween = func() {
		chunks++
		if !u.mu.TryLock() {
			t.Error("the updater mutex was held between two chunks of the prune")
		} else {
			u.mu.Unlock()
		}
		if chunks == 3 {
			// A restore starts mid-prune: it holds the root, moves current to
			// r6 without a journal and has r7 to swap back to.
			var err error
			if release, err = HoldRestore(context.Background(), root); err != nil {
				t.Error(err)
				return
			}
			if err := swapCurrentPointer(root, filepath.Join(root, "releases", digests[6])); err != nil {
				t.Error(err)
			}
		}
	}
	report := u.Prune()
	if release == nil {
		t.Fatal("the hook never ran")
	}
	if report.Skipped == "" {
		t.Fatalf("the prune did not stand down for a restore that started mid-run: %+v", report)
	}
	for _, d := range digests {
		if !exists(filepath.Join(root, "releases", d)) {
			t.Fatalf("a release was removed after a restore took the root")
		}
	}
	if err := swapCurrentPointer(root, filepath.Join(root, "releases", digests[7])); err != nil {
		t.Fatalf("swap-back: %v", err)
	}
	release()
	u.pruneBetween = nil
	if report := u.Prune(); report.Skipped != "" {
		t.Fatalf("a settled updater did not prune: %+v", report)
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
	u.Prune()
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
	u2.Prune()
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
	u.Prune()
	if !exists(filepath.Join(root, "releases", r1)) || exists(filepath.Join(root, "releases", r3)) {
		t.Fatal("after a restage the release it came from must be the kept predecessor")
	}
	if name, unsettled := UnsettledApply(root); unsettled {
		t.Fatalf("a restage record reads as an unsettled apply: %s", name)
	}
}

// S4: ImportBaseline swaps current, so it takes the updater's mutex.
func TestImportBaselineWaitsForTheUpdater(t *testing.T) {
	u, _ := newPruneUpdater(t)
	u.mu.Lock()
	done := make(chan struct{})
	go func() {
		_, _ = u.ImportBaseline(BaselineImportRequest{})
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("ImportBaseline ran while an apply held the updater")
	case <-time.After(100 * time.Millisecond):
	}
	u.mu.Unlock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ImportBaseline did not run after the updater was released")
	}
}

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
	if report := u.Prune(); report.BytesFreed >= 1<<20 {
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

// restoreInFlight samples the hold for tests; nothing in production may act on
// such a sample (the prune holds the hold instead).
func restoreInFlight(root string) bool {
	release, acquired, err := tryHoldRestore(root)
	if err != nil {
		return true
	}
	if acquired {
		release()
	}
	return !acquired
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
	u.Prune()
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

// D1: the prune holds the restore hold for the whole chunk. A probe at chunk
// entry would pass this test's first half; the second half fails a hold that is
// released early.
func TestPruneChunkHoldsTheRestoreHoldWhileItMutates(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	temp := filepath.Join(root, "incoming", ".stage-old")
	writeTestFile(t, filepath.Join(temp, "f"), 100)
	ageBy(t, temp, old)
	seen := 0
	u.pruneHeld = func() {
		seen++
		if seen != 1 {
			return
		}
		release, acquired, err := tryHoldRestore(root)
		if err != nil || acquired {
			if acquired {
				release()
			}
			t.Errorf("the restore hold was free inside a chunk (acquired=%v err=%v)", acquired, err)
		}
		got := make(chan struct{})
		go func() {
			rel, err := HoldRestore(context.Background(), root)
			if err == nil {
				rel()
			}
			close(got)
		}()
		select {
		case <-got:
			t.Error("a concurrent restore took the hold while the chunk was mutating")
		case <-time.After(200 * time.Millisecond):
		}
	}
	u.Prune()
	if seen == 0 {
		t.Fatal("the in-chunk seam never ran")
	}
}

// D2: the hold is taken within a deadline and honours the context; a stuck
// holder yields a named error, not a silent wait.
func TestHoldRestoreIsBoundedByADeadlineAndTheContext(t *testing.T) {
	root := t.TempDir()
	held, err := HoldRestore(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	previous := restoreHoldTimeout
	restoreHoldTimeout = 150 * time.Millisecond
	defer func() { restoreHoldTimeout = previous }()
	start := time.Now()
	if _, err := HoldRestore(context.Background(), root); !errors.Is(err, ErrRestoreHoldTimeout) {
		t.Fatalf("err = %v, want ErrRestoreHoldTimeout", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("the deadline was not honoured")
	}
	restoreHoldTimeout = time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := HoldRestore(ctx, root); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the context's", err)
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

// A candidate shot reuses a store object that may be old and unrooted; reuse
// refreshes its mtime so the prune's input window starts again. The same for a
// baseline release reused by StageBaselineRelease.
func TestReuseOfAnExistingStoreObjectOrStagedReleaseRefreshesItsMtime(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	store := filepath.Join(t.TempDir(), "store")
	t.Cleanup(func() { makeTreeWritable(store) })
	paths, err := MaterializeClosure(blob, store)
	if err != nil || len(paths) != 1 {
		t.Fatalf("%v %v", paths, err)
	}
	object := filepath.Join(store, paths[0])
	longAgo := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(object, longAgo, longAgo); err != nil {
		t.Fatal(err)
	}
	if _, err := MaterializeClosure(blob, store); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(object); err != nil || time.Since(info.ModTime()) > time.Hour {
		t.Fatalf("a reused store object kept its old mtime: %v %v", info.ModTime(), err)
	}

	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	releaseDir := filepath.Join(root, "releases", digest)
	if err := os.Chtimes(releaseDir, longAgo, longAgo); err != nil {
		t.Fatal(err)
	}
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", "r1", "idem-r1", "r1")
	if err := u.stageRelease(request.SourceDir, releaseDir, request.Manifest); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(releaseDir); err != nil || time.Since(info.ModTime()) > time.Hour {
		t.Fatalf("a reused release dir kept its old mtime: %v %v", info.ModTime(), err)
	}
}
