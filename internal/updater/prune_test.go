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
//  F3  a prune must never take a bundle (bundle.json: operation evidence the
//      updater cannot judge), an entry it does not recognise, a payload whose
//      release is not staged yet, or anything younger than the grace period
//      (a stage or a host build in progress)
//      (TestPruneKeepsBundlesUnknownEntriesUnstagedPayloadsAndYoungEntries)
//  F4  every release ever applied stays (TestPruneKeepsCurrentItsPredecessorAndTheRecentWindow)
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
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

var pruneNow = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

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

func TestPruneRemovesStagedPayloadOnlyAfterItsReleaseExists(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	staged := filepath.Join(root, "incoming", digest)
	writeTestFile(t, filepath.Join(staged, "bin", "choir"), 1000)
	ageBy(t, staged, old)
	unstaged := filepath.Join(root, "incoming", strings.Repeat("9", 64))
	writeTestFile(t, filepath.Join(unstaged, "bin", "choir"), 1000)
	ageBy(t, unstaged, old)

	report := u.Prune()
	if exists(staged) {
		t.Fatal("the payload of a staged release survived the prune")
	}
	if !exists(unstaged) {
		t.Fatal("a payload with no staged release (its apply has not run) was removed")
	}
	if report.BytesFreed < 1000 || report.IncomingRemoved != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestPruneRemovesOldHostBuildsAndTemporaries(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	for _, name := range []string{"host-build-abc", ".host-build-xyz", ".stage-123"} {
		writeTestFile(t, filepath.Join(root, "incoming", name, "closure.nar"), 500)
		ageBy(t, filepath.Join(root, "incoming", name), old)
	}
	u.Prune()
	for _, name := range []string{"host-build-abc", ".host-build-xyz", ".stage-123"} {
		if exists(filepath.Join(root, "incoming", name)) {
			t.Fatalf("old %s survived the prune", name)
		}
	}
	if !exists(filepath.Join(root, "incoming")) {
		t.Fatal("incoming itself must stay")
	}
}

func TestPruneKeepsBundlesUnknownEntriesUnstagedPayloadsAndYoungEntries(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	incoming := filepath.Join(root, "incoming")
	bundle := filepath.Join(incoming, strings.Repeat("b", 64))
	writeTestFile(t, filepath.Join(bundle, "bundle.json"), 100)
	ageBy(t, bundle, 90*24*time.Hour)
	unknown := filepath.Join(incoming, "operation-7")
	writeTestFile(t, filepath.Join(unknown, "f"), 100)
	ageBy(t, unknown, old)
	loose := filepath.Join(incoming, "loose-file")
	writeTestFile(t, loose, 100)
	ageBy(t, loose, old)
	// Young: a host build and a staged payload from a minute ago, and an old
	// host build with one fresh file inside (a download in progress).
	young := filepath.Join(incoming, "host-build-young")
	writeTestFile(t, filepath.Join(young, "closure.nar"), 100)
	ageBy(t, young, time.Minute)
	youngPayload := filepath.Join(incoming, digest)
	writeTestFile(t, filepath.Join(youngPayload, "bin", "choir"), 100)
	ageBy(t, youngPayload, time.Minute)
	busy := filepath.Join(incoming, "host-build-busy")
	writeTestFile(t, filepath.Join(busy, "old"), 100)
	writeTestFile(t, filepath.Join(busy, "deep", "fresh"), 100)
	ageBy(t, filepath.Join(busy, "old"), old)
	ageBy(t, busy, old)
	if err := os.Chtimes(filepath.Join(busy, "deep", "fresh"), pruneNow, pruneNow); err != nil {
		t.Fatal(err)
	}

	report := u.Prune()
	for _, path := range []string{bundle, unknown, loose, young, youngPayload, busy} {
		if !exists(path) {
			t.Fatalf("%s was removed", path)
		}
	}
	if report.IncomingRemoved != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestPruneKeepsCurrentItsPredecessorAndTheRecentWindow(t *testing.T) {
	u, root := newPruneUpdater(t)
	var digests []string
	for _, name := range []string{"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7"} {
		digests = append(digests, stageTestRelease(t, u, root, name, false))
	}
	// r0 -> r1 -> ... -> r7 applied in order; r7 is current. An older release
	// was rolled to once, recently (r2 is the most recent use of an old one).
	for i := 1; i < len(digests); i++ {
		writeTestJournal(t, root, "idem-"+digests[i], digests[i-1], digests[i], pruneNow.Add(time.Duration(i-10)*time.Hour), "applied")
	}
	setCurrent(t, root, digests[7])
	for _, d := range digests {
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
	}

	report := u.Prune()
	kept := map[string]bool{}
	for _, d := range digests[4:] { // r4 r5 r6 r7: current, its predecessor, and the two most recent others
		kept[d] = true
	}
	// The window is the current release, its predecessor and the three most
	// recently used others: r7, r6, r5, r4, r3.
	kept[digests[3]] = true
	for i, d := range digests {
		if kept[d] != exists(filepath.Join(root, "releases", d)) {
			t.Fatalf("release r%d kept=%v want %v", i, exists(filepath.Join(root, "releases", d)), kept[d])
		}
	}
	if report.ReleasesRemoved != 3 {
		t.Fatalf("report = %+v", report)
	}
	if got, _, err := currentReleaseOf(root); err != nil || got != digests[7] {
		t.Fatalf("current = %q, %v", got, err)
	}
	if _, err := VerifyCurrentRelease(root); err != nil {
		t.Fatalf("current release no longer verifies: %v", err)
	}
	// The recorded predecessor the rollback restores to is still on disk.
	if !exists(filepath.Join(root, "releases", digests[6], "release-manifest.json")) {
		t.Fatal("the rollback predecessor is gone")
	}
	// Nothing hidden is left behind.
	entries, _ := os.ReadDir(filepath.Join(root, "releases"))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Fatalf("leftover %s in releases", entry.Name())
		}
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
	for _, kept := range []string{"shared", "only-new", "orphan-young"} {
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

func TestPruneSkipsWhileAnApplyIsUnsettled(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	old1 := filepath.Join(root, "incoming", "host-build-old")
	writeTestFile(t, filepath.Join(old1, "f"), 100)
	ageBy(t, old1, old)
	open := operationJournal{Phase: "pointer_swapped", TargetReleaseDigest: digest}
	if err := writeJournal(filepath.Join(root, "operations", safeName("open")+".json"), open); err != nil {
		t.Fatal(err)
	}
	report := u.Prune()
	if !exists(old1) || report.Skipped == "" || report.BytesFreed != 0 {
		t.Fatalf("pruned under an unsettled apply: %+v", report)
	}
	// A held apply key (an apply with no journal yet) skips too.
	if err := os.Remove(filepath.Join(root, "operations", safeName("open")+".json")); err != nil {
		t.Fatal(err)
	}
	unlock, acquired, err := lockApplyKey(root, "running", true)
	if err != nil || !acquired {
		t.Fatalf("lock: %v %v", acquired, err)
	}
	// Same-process flock on a second descriptor conflicts, as in the daemon.
	report = u.Prune()
	unlock()
	if !exists(old1) || report.Skipped == "" {
		t.Fatalf("pruned under a held apply key: %+v", report)
	}
	if report = u.Prune(); exists(old1) || report.Skipped != "" {
		t.Fatalf("a settled updater did not prune: %+v", report)
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
	if _, err := VerifyCurrentRelease(root); err != nil {
		t.Fatalf("current does not verify mid-crash: %v", err)
	}
	if _, _, err := ReadPinnedManifest(root, digests[6]); err != nil {
		t.Fatalf("the predecessor is unreadable mid-crash: %v", err)
	}
	u.Prune()
	if exists(hidden) || exists(filepath.Join(root, "gc-roots", digests[0])) || exists(filepath.Join(root, "store", "old-only")) {
		t.Fatal("the next run did not finish the crashed prune")
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
	writeTestFile(t, filepath.Join(root, "incoming", "host-build-a", "f"), 4096)
	ageBy(t, filepath.Join(root, "incoming", "host-build-a"), old)
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
		stale := filepath.Join(root, "incoming", "host-build-stale")
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
	if _, err := VerifyCurrentRelease(root); err != nil {
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
	stale := filepath.Join(root, "incoming", "host-build-stale")
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
