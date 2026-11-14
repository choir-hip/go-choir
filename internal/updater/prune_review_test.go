package updater

// Review round 1 on the boot-time prune (3d39bf5c4). Failure list, each pinned
// by the test named:
//
//  R1  a settled-forward apply (the updater died after the swap, the runtime's
//      crash-boot settlement fenced the journal as an installed verdict) must
//      still keep its predecessor, the release the operation's revert reads;
//      a nothing-installed or unknown fence must keep nothing extra
//      (TestPruneKeepsThePredecessorOfAFencedInstalledApply,
//      TestPruneKeepsNothingExtraForANothingInstalledOrUnknownFence)
//  R2  a symlink in any of the four directories, or inside a payload, is
//      unlinked and never followed (TestPruneUnlinksSymlinksAndLeavesTheirTargets,
//      TestDropStagedSourceUnlinksASymlinkAndLeavesItsTarget)
//  R3  a missing root is not created and not an error (TestPruneOfAMissingRootDoesNothing)
//  R4  a layering entrypoint with a `..` segment is no root
//      (TestPruneEntrypointWithDotDotSegmentIsNoRoot)
//  R5  a release directory with no manifest is a corpse of an interrupted
//      deletion and goes at once (TestPruneRemovesAReleaseCorpseAtOnce)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fenceJournal writes a dead apply journal for key (prior -> target) and
// fences it through the real HoldApplyKey and Fence, as the runtime's
// crash-boot settlement does (selfdev_settle_open.go).
func fenceJournal(t *testing.T, root, key, prior, target string, startedAt time.Time) ApplyVerdict {
	t.Helper()
	dead := operationJournal{Phase: "pointer_swapped", PriorReleaseDigest: prior, PriorReleaseTarget: filepath.Join(root, "releases", prior),
		TargetReleaseDigest: target, StartedAt: startedAt}
	if err := writeJournal(filepath.Join(root, "operations", safeName(key)+".json"), dead); err != nil {
		t.Fatal(err)
	}
	hold, err := HoldApplyKey(root, key)
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Release()
	if !hold.Dead() {
		t.Fatalf("verdict %s is not a dead one", hold.Verdict)
	}
	if err := hold.Fence("settled by the test"); err != nil {
		t.Fatal(err)
	}
	return hold.Verdict
}

func stageAged(t *testing.T, u *Updater, root string, names ...string) []string {
	t.Helper()
	var digests []string
	for _, name := range names {
		d := stageTestRelease(t, u, root, name, false)
		ageBy(t, filepath.Join(root, "releases", d), 30*24*time.Hour)
		digests = append(digests, d)
	}
	return digests
}

func TestPruneKeepsThePredecessorOfAFencedInstalledApply(t *testing.T) {
	u, root := newPruneUpdater(t)
	d := stageAged(t, u, root, "r0", "r1", "r2", "r9")
	r0, r1, r2, r9 := d[0], d[1], d[2], d[3]
	// R1 -> R2 swapped, the updater died before the result, and the runtime's
	// crash-boot settlement found current == target and fenced it.
	writeTestJournal(t, root, "op-r0-r1", r0, r1, pruneNow.Add(-20*time.Hour), "applied")
	setCurrent(t, root, r2)
	if v := fenceJournal(t, root, "op-r1-r2", r1, r2, pruneNow.Add(-10*time.Hour)); v != VerdictInstalled {
		t.Fatalf("verdict = %s, want installed", v)
	}
	report := pruneRoot(root)
	if report.Skipped != "" {
		t.Fatalf("skipped: %+v", report)
	}
	if !exists(filepath.Join(root, "releases", r1)) {
		t.Fatal("the predecessor of a settled-forward apply was pruned: the revert of its operation needs it")
	}
	if exists(filepath.Join(root, "releases", r0)) || exists(filepath.Join(root, "releases", r9)) {
		t.Fatal("an older or unrelated release survived")
	}
	if _, _, err := ReadPinnedManifest(root, r1); err != nil {
		t.Fatalf("the revert input is unreadable: %v", err)
	}
	// The bound: current, one applied-or-fenced predecessor, one restage
	// predecessor, so three releases with no dead journal.
	b := stageAged(t, u, root, "base", "extra")
	if err := RestagePinnedRelease(root, b[0]); err != nil {
		t.Fatal(err)
	}
	if err := RestagePinnedRelease(root, r2); err != nil {
		t.Fatal(err)
	}
	report = pruneRoot(root)
	if report.ReleasesKept != 3 || !exists(filepath.Join(root, "releases", r1)) || !exists(filepath.Join(root, "releases", b[0])) || exists(filepath.Join(root, "releases", b[1])) {
		t.Fatalf("the bound is current + 2 predecessors: %+v", report)
	}
}

func TestPruneKeepsNothingExtraForANothingInstalledOrUnknownFence(t *testing.T) {
	// Nothing installed: current is still the prior; later current reaches the
	// target by another path (a restage), which journals itself.
	u, root := newPruneUpdater(t)
	d := stageAged(t, u, root, "r1", "r2")
	setCurrent(t, root, d[0])
	if v := fenceJournal(t, root, "op-nothing", d[0], d[1], pruneNow.Add(-10*time.Hour)); v != VerdictNothingInstalled {
		t.Fatalf("verdict = %s", v)
	}
	setCurrent(t, root, d[1])
	pruneRoot(root)
	if exists(filepath.Join(root, "releases", d[0])) {
		t.Fatal("a nothing-installed fence kept its prior")
	}
	// Unknown: current was neither end when it was fenced; later current
	// becomes the journal's target. The fence says unknown, so it installed nothing.
	u2, root2 := newPruneUpdater(t)
	e := stageAged(t, u2, root2, "r1", "r2", "r3")
	setCurrent(t, root2, e[2])
	if v := fenceJournal(t, root2, "op-unknown", e[0], e[1], pruneNow.Add(-10*time.Hour)); v != VerdictUnknown {
		t.Fatalf("verdict = %s", v)
	}
	setCurrent(t, root2, e[1])
	pruneRoot(root2)
	if exists(filepath.Join(root2, "releases", e[0])) || exists(filepath.Join(root2, "releases", e[2])) {
		t.Fatal("an unknown fence kept a release")
	}
}

func TestPruneUnlinksSymlinksAndLeavesTheirTargets(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	outside := filepath.Join(t.TempDir(), "outside")
	writeTestFile(t, filepath.Join(outside, "precious"), 100)
	sha := strings.Repeat("7", 64)
	links := []string{
		filepath.Join(root, "incoming", ".stage-link"),
		filepath.Join(root, "incoming", "loose"),
		filepath.Join(root, "releases", sha),
		filepath.Join(root, "releases", ".stage-link"),
		filepath.Join(root, "gc-roots", sha),
		filepath.Join(root, "store", "linked-object"),
	}
	for _, link := range links {
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
	}
	// A symlink inside a payload that is itself deleted.
	nested := filepath.Join(root, "incoming", ".stage-nested")
	writeTestFile(t, filepath.Join(nested, "f"), 100)
	if err := os.Symlink(outside, filepath.Join(nested, "inner")); err != nil {
		t.Fatal(err)
	}
	ageBy(t, nested, old)
	pruneRoot(root)
	if !exists(filepath.Join(outside, "precious")) {
		t.Fatal("the prune followed a symlink out of the root")
	}
	for _, link := range append(links, nested) {
		if exists(link) {
			t.Fatalf("%s survived the prune", link)
		}
	}
	if hidden := hiddenLeftovers(t, root); len(hidden) != 0 {
		t.Fatalf("leftovers: %v", hidden)
	}
}

func TestDropStagedSourceUnlinksASymlinkAndLeavesItsTarget(t *testing.T) {
	u, root, _ := newApplyRig(t, "")
	outside := filepath.Join(t.TempDir(), "outside")
	writeTestFile(t, filepath.Join(outside, "precious"), 100)
	link := filepath.Join(root, "incoming", strings.Repeat("5", 64))
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	u.dropStagedSource(link)
	if !exists(filepath.Join(outside, "precious")) {
		t.Fatal("dropStagedSource removed what a symlink pointed to")
	}
	if exists(link) {
		t.Fatal("the symlink source stayed in incoming")
	}
	// A source that is not under incoming is never touched, link or not.
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.Symlink(outside, elsewhere); err != nil {
		t.Fatal(err)
	}
	u.dropStagedSource(elsewhere)
	if !exists(elsewhere) || !exists(filepath.Join(outside, "precious")) {
		t.Fatal("dropStagedSource acted outside incoming")
	}
}

func TestPruneOfAMissingRootDoesNothing(t *testing.T) {
	root := filepath.Join(t.TempDir(), "no-such-root")
	report := pruneRoot(root)
	if report.Entries() != 0 || report.Errors != 0 || report.BytesFreed != 0 {
		t.Fatalf("report = %+v", report)
	}
	if exists(root) {
		t.Fatal("the prune created the root")
	}
}

func TestPruneEntrypointWithDotDotSegmentIsNoRoot(t *testing.T) {
	// "../aaaa-layered/..." resolves (cleaned under "/") to the live object
	// aaaa-layered, but a release that declares a `..` is not rooted there.
	u, root := newPruneUpdater(t)
	layered := stageTestReleaseEntrypoint(t, u, root, "layered", "../aaaa-layered/bin/autoputer")
	setCurrent(t, root, layered)
	rootStoreObjects(t, root, layered, "aaaa-layered")
	writeTestFile(t, filepath.Join(root, "store", "orphan", "f"), 100)
	for _, name := range []string{"aaaa-layered", "orphan"} {
		ageBy(t, filepath.Join(root, "store", name), old)
	}
	pruneRoot(root)
	if !exists(filepath.Join(root, "store", "orphan")) {
		t.Fatal("the store was collected under an entrypoint with a `..` segment")
	}
}

func TestPruneRemovesAReleaseCorpseAtOnce(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	// What an interrupted in-place deletion leaves: a release directory whose
	// manifest is gone, young because the deletion touched it.
	corpse := filepath.Join(root, "releases", strings.Repeat("c", 64))
	writeTestFile(t, filepath.Join(corpse, "bin", "choir"), 100)
	ageBy(t, corpse, time.Minute)
	pruneRoot(root)
	if exists(corpse) {
		t.Fatal("a release directory with no manifest survived")
	}
	if !exists(filepath.Join(root, "releases", digest)) {
		t.Fatal("current was removed")
	}
}
