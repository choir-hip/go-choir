package agentcore

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// A frozen self-development bundle must survive a hard kill
// (docs/problems/awaiting-candidate-unreadable-after-refresh-2026-10-26.md):
// a vmctl refresh SIGKILLs the guest about 35 s after Verify wrote
// bundle.json with a plain WriteFile and renamed its directory unsynced, and
// the next boot read the candidate as a 409 "frozen bundle binding mismatch".
//
// Ways each change could fail, written before the code:
//
// Durable Freeze and finalize writes:
//   - a file in the frozen tree (bundle.draft.json, source.patch, a release
//     file, a visual capture) is not fsynced before the directory rename, so
//     the rename is durable and the file is short or empty;
//   - a directory in the tree is not fsynced, so a synced file's entry is
//     missing after the kill;
//   - incoming/ is not fsynced after the rename, so the published name is
//     lost and the operation names a digest with no directory;
//   - the final bundle.json is written in place in a published directory, so
//     a kill mid-write leaves a torn file under the final name;
//   - the bundle-finalizations marker is not durable before the rename, so a
//     kill after the rename strands the operation with no draft and no
//     mapping ("verified bundle draft unavailable");
//   - a sync error is swallowed, so Freeze or Verify reports success for a
//     bundle that is not on disk; or the failure publishes the directory
//     anyway.
//
// The reader names which check failed:
//   - the 409 keeps one message for three failures, so a refresh casualty
//     (a bundle that does not decode) reads the same as tampering;
//   - the status changes from 409, or the "frozen bundle binding mismatch: "
//     prefix clients match on is dropped;
//   - the materializer still says only "invalid frozen bundle".
//
// Guest sync before a planned kill:
//   - the planned-restart answer is sent before the filesystems are synced,
//     so the host's SIGKILL drops whatever the guest wrote without fsync;
//   - the sync runs before the marker is durable, or on a busy refusal
//     (which restarts nothing).

// syncRecorder replaces durableSync and records what each fsync saw.
type syncRecorder struct {
	mu      sync.Mutex
	entries []syncEntry
	fail    func(name string) bool
}

type syncEntry struct {
	path     string
	dir      bool
	content  string
	children []string
}

func installSyncRecorder(t *testing.T) *syncRecorder {
	t.Helper()
	recorder := &syncRecorder{}
	previous := durableSync
	durableSync = func(f *os.File) error {
		name := f.Name()
		if recorder.fail != nil && recorder.fail(name) {
			return errors.New("injected sync failure")
		}
		entry := syncEntry{path: filepath.Clean(name)}
		if info, err := os.Stat(name); err == nil && info.IsDir() {
			entry.dir = true
			children, _ := os.ReadDir(name)
			for _, child := range children {
				entry.children = append(entry.children, child.Name())
			}
		} else if raw, err := os.ReadFile(name); err == nil {
			entry.content = string(raw)
		}
		recorder.mu.Lock()
		recorder.entries = append(recorder.entries, entry)
		recorder.mu.Unlock()
		return previous(f)
	}
	t.Cleanup(func() { durableSync = previous })
	return recorder
}

// find returns the index of the first entry matching, or -1.
func (r *syncRecorder) find(match func(syncEntry) bool) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index, entry := range r.entries {
		if match(entry) {
			return index
		}
	}
	return -1
}

func hasChild(entry syncEntry, name string) bool {
	for _, child := range entry.children {
		if child == name {
			return true
		}
	}
	return false
}

func TestFreezePublishSyncsEveryFileAndDirectoryBeforeTheRename(t *testing.T) {
	incoming := filepath.Join(t.TempDir(), "incoming")
	temporary := filepath.Join(incoming, ".freeze-test")
	files := map[string]string{
		"source.patch":                        "diff --git a/x b/x\n",
		"release/frontend/index.html":         "<html>candidate</html>\n",
		"evidence/visual/route-home.png":      "png bytes",
		"evidence/visual/visual-receipt.json": "{}",
	}
	for name, content := range files {
		path := filepath.Join(temporary, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	draft := []byte(`{"bundle_version":1}`)
	frozenRoot := filepath.Join(incoming, strings.Repeat("d", 64))
	recorder := installSyncRecorder(t)

	if err := publishFrozenDraft(temporary, frozenRoot, draft); err != nil {
		t.Fatalf("publishFrozenDraft: %v", err)
	}
	if raw, err := os.ReadFile(filepath.Join(frozenRoot, "bundle.draft.json")); err != nil || string(raw) != string(draft) {
		t.Fatalf("published draft = %q (%v)", raw, err)
	}
	published := recorder.find(func(e syncEntry) bool {
		return e.dir && e.path == filepath.Clean(incoming) && hasChild(e, filepath.Base(frozenRoot))
	})
	if published < 0 {
		t.Fatalf("incoming/ was not fsynced after the rename; syncs: %+v", recorder.entries)
	}
	files["bundle.draft.json"] = string(draft)
	for name, content := range files {
		path := filepath.Join(temporary, filepath.FromSlash(name))
		index := recorder.find(func(e syncEntry) bool { return !e.dir && e.path == path && e.content == content })
		if index < 0 || index > published {
			t.Fatalf("%s was not fsynced with its bytes before the publish (index %d, publish %d)", name, index, published)
		}
	}
	for _, dir := range []string{"", "release", "release/frontend", "evidence", "evidence/visual"} {
		path := filepath.Clean(filepath.Join(temporary, filepath.FromSlash(dir)))
		index := recorder.find(func(e syncEntry) bool { return e.dir && e.path == path })
		if index < 0 || index > published {
			t.Fatalf("directory %q was not fsynced before the publish (index %d, publish %d)", dir, index, published)
		}
	}
}

func TestFreezePublishRefusesWhenASyncFailsAndPublishesNothing(t *testing.T) {
	incoming := filepath.Join(t.TempDir(), "incoming")
	temporary := filepath.Join(incoming, ".freeze-test")
	if err := os.MkdirAll(temporary, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temporary, "source.patch"), []byte("diff\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	recorder := installSyncRecorder(t)
	recorder.fail = func(name string) bool { return filepath.Base(name) == "source.patch" }
	frozenRoot := filepath.Join(incoming, strings.Repeat("d", 64))
	if err := publishFrozenDraft(temporary, frozenRoot, []byte(`{}`)); err == nil {
		t.Fatal("a failed fsync of source.patch still published the frozen bundle")
	}
	if _, err := os.Stat(frozenRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a failed sync left the frozen root published: %v", err)
	}
}

// newBundleDurabilityFixture is the P4 fixture's freeze and Verify path
// without corpusd: the derivable fixture's chain, store and updater root.
func newBundleDurabilityFixture(t *testing.T) *p4Fixture {
	t.Helper()
	return &p4Fixture{
		derivableSelfDevFixture: newDerivableSelfDevFixture(t, "computer-bundle-durable"),
		history:                 platformHistory{base: strings.Repeat("1", 40)},
	}
}

func TestVerifyFinalizeWritesTheFinalBundleDurablyBeforeItsDirectoryIsRenamed(t *testing.T) {
	f := newBundleDurabilityFixture(t)
	frozen := f.freezeWithPatch(t, "durable-final", "durable final", []byte("diff --git a/frontend/x b/frontend/x\n"))
	draftRoot := filepath.Clean(filepath.Join(f.updaterRoot, "incoming", frozen.BundleDigest))
	recorder := installSyncRecorder(t)

	verified := f.verify(t, frozen)
	incoming := filepath.Clean(filepath.Join(f.updaterRoot, "incoming"))
	finalRoot := filepath.Join(incoming, verified.BundleDigest)
	finalBytes, err := os.ReadFile(filepath.Join(finalRoot, "bundle.json"))
	if err != nil || computerevent.DigestBytes(finalBytes) != verified.BundleDigest {
		t.Fatalf("final bundle.json unreadable or not the operation's digest: %v", err)
	}
	published := recorder.find(func(e syncEntry) bool {
		return e.dir && e.path == incoming && hasChild(e, verified.BundleDigest)
	})
	if published < 0 {
		t.Fatalf("incoming/ was not fsynced after the final rename; syncs: %+v", recorder.entries)
	}
	finalSynced := recorder.find(func(e syncEntry) bool {
		return !e.dir && filepath.Dir(e.path) == draftRoot && e.content == string(finalBytes)
	})
	if finalSynced < 0 || finalSynced > published {
		t.Fatalf("final bundle.json was not fsynced in the draft root before the publish (index %d, publish %d)", finalSynced, published)
	}
	draftDirSynced := recorder.find(func(e syncEntry) bool { return e.dir && e.path == draftRoot && hasChild(e, "bundle.json") })
	if draftDirSynced < 0 || draftDirSynced > published {
		t.Fatalf("the draft root was not fsynced with bundle.json in it before the publish (index %d)", draftDirSynced)
	}
	mappingRoot := filepath.Join(f.updaterRoot, "bundle-finalizations")
	mappingSynced := recorder.find(func(e syncEntry) bool {
		return !e.dir && filepath.Dir(e.path) == mappingRoot && e.content == verified.BundleDigest
	})
	mappingDirSynced := recorder.find(func(e syncEntry) bool {
		return e.dir && e.path == mappingRoot && hasChild(e, frozen.BundleDigest)
	})
	if mappingSynced < 0 || mappingDirSynced < 0 || mappingSynced > published || mappingDirSynced > published {
		t.Fatalf("the bundle-finalizations marker was not durable before the publish (file %d, dir %d, publish %d)", mappingSynced, mappingDirSynced, published)
	}
	patchSynced := recorder.find(func(e syncEntry) bool { return !e.dir && e.path == filepath.Join(draftRoot, "source.patch") })
	if patchSynced < 0 || patchSynced > published {
		t.Fatalf("source.patch was not fsynced before the publish (index %d)", patchSynced)
	}
}

func TestVerifyFinalizeRefusesWhenTheFinalBundleSyncFails(t *testing.T) {
	f := newBundleDurabilityFixture(t)
	frozen := f.freezeWithPatch(t, "durable-fail", "durable fail", []byte("diff --git a/frontend/y b/frontend/y\n"))
	recorder := installSyncRecorder(t)
	recorder.fail = func(name string) bool { return strings.Contains(filepath.Base(name), "bundle.json") }
	toolCtx := &CapsuleToolCtx{AgentRunID: "run:management-durable", ComputerID: f.computerID, OwnerID: "owner",
		EventAppender: f.appender, OperationStore: f.operations, UpdaterRoot: f.updaterRoot, EventProjection: f.store}
	if _, err := recordSelfDevelopmentVerdict(context.Background(), toolCtx, frozen.TrajectoryID, frozen.OperationID, frozen.BundleDigest, "pass",
		[]string{"capsule-go-eval:sha256:" + strings.Repeat("a", 64)}, map[string]any{"verifier_desk": "management"}); err == nil {
		t.Fatal("Verify succeeded although the final bundle.json never reached the disk")
	}
	if _, err := os.Stat(filepath.Join(f.updaterRoot, "incoming", frozen.BundleDigest, "bundle.draft.json")); err != nil {
		t.Fatalf("a failed final sync moved the draft root away: %v", err)
	}
	operation, err := f.operations.Get(context.Background(), f.computerID, frozen.OperationID)
	if err != nil || operation.State != selfdev.StateFrozen {
		t.Fatalf("operation after a failed finalize = %s (%v), want frozen", operation.State, err)
	}
}

func TestReadBoundBundleNamesWhichCheckFailed(t *testing.T) {
	root := t.TempDir()
	operation := selfdev.Operation{
		OperationID: "op-named", ComputerID: "computer-named", TrajectoryID: "trajectory-named",
		CapsuleID: "capsule-named", BaseHead: strings.Repeat("a", 64),
	}
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: operation.ComputerID, BaseEventHead: operation.BaseHead,
		TrajectoryRef: operation.TrajectoryID, CapsuleIdentity: operation.CapsuleID,
		VerifierReceipts: []string{strings.Repeat("f", 64)},
	}
	good, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	other := bundle
	other.CapsuleIdentity = "capsule-other"
	otherRaw, err := computerevent.CanonicalJSON(other)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		raw    []byte
		digest string
		want   string
	}{
		// The refresh casualty: a torn or empty file under the final name.
		{"empty", []byte{}, computerevent.DigestBytes([]byte{}), "frozen bundle binding mismatch: frozen bundle does not decode"},
		{"torn", good[:len(good)/2], computerevent.DigestBytes(good[:len(good)/2]), "frozen bundle binding mismatch: frozen bundle does not decode"},
		{"digest", good, strings.Repeat("e", 64), "frozen bundle binding mismatch: frozen bundle digest does not match the operation"},
		{"identity", otherRaw, computerevent.DigestBytes(otherRaw), "frozen bundle binding mismatch: frozen bundle names another computer, trajectory, capsule or base"},
	}
	for _, tc := range cases {
		dir := filepath.Join(root, tc.name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bundle.json"), tc.raw, 0o400); err != nil {
			t.Fatal(err)
		}
		bound := operation
		bound.BundleDigest = tc.digest
		_, status, err := readBoundSelfDevelopmentBundle(dir, bound)
		if status != http.StatusConflict || err == nil || err.Error() != tc.want {
			t.Fatalf("%s: status=%d err=%v, want 409 %q", tc.name, status, err, tc.want)
		}
	}
	// A draft (frozen, before Verify) whose recomputed digest is not the
	// operation's names the digest check too.
	draftDir := filepath.Join(root, "draft")
	if err := os.MkdirAll(draftDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftDir, "bundle.draft.json"), good, 0o400); err != nil {
		t.Fatal(err)
	}
	bound := operation
	bound.BundleDigest = strings.Repeat("e", 64)
	if _, status, err := readBoundSelfDevelopmentBundle(draftDir, bound); status != http.StatusConflict || err == nil ||
		err.Error() != "frozen bundle binding mismatch: frozen bundle digest does not match the operation" {
		t.Fatalf("draft digest: status=%d err=%v", status, err)
	}
}

func TestMaterializerNamesWhichFrozenBundleCheckFailed(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.selfdevUpdaterRoot = t.TempDir()
	operation := selfdev.Operation{
		OperationID: "op-mat", ComputerID: "computer-mat", TrajectoryID: "trajectory-mat",
		CapsuleID: "capsule-mat", BaseHead: strings.Repeat("a", 64),
	}
	write := func(digest string, raw []byte) {
		t.Helper()
		dir := filepath.Join(rt.selfdevUpdaterRoot, "incoming", digest)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bundle.json"), raw, 0o400); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name   string
		raw    []byte
		digest string
		want   string
	}{
		{"missing", nil, strings.Repeat("1", 64), "materializer: frozen bundle unavailable"},
		{"digest", []byte(`{"bundle_version":1}`), strings.Repeat("2", 64), "materializer: frozen bundle digest does not match the operation"},
		{"decode", []byte(`{"bundle_version":`), computerevent.DigestBytes([]byte(`{"bundle_version":`)), "materializer: invalid frozen bundle: does not decode"},
	}
	for _, tc := range cases {
		if tc.raw != nil {
			write(tc.digest, tc.raw)
		}
		bound := operation
		bound.BundleDigest = tc.digest
		err := rt.materializeSelfDevelopmentOperation(context.Background(), bound, true)
		if err == nil || err.Error() != tc.want {
			t.Fatalf("%s: err=%v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestPlannedRestartSyncsFilesystemsAfterTheMarkerIsDurable(t *testing.T) {
	rt, _ := testRuntime(t)
	t.Cleanup(func() { _ = ClearPlannedRestartMarker(rt.cfg.StorePath) })
	_ = ClearPlannedRestartMarker(rt.cfg.StorePath)
	var syncs int
	var markerAtSync bool
	previous := syncFilesystems
	syncFilesystems = func() {
		syncs++
		_, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath))
		markerAtSync = err == nil
	}
	t.Cleanup(func() { syncFilesystems = previous })
	handler := &APIHandler{rt: rt}

	code, body := postPlannedRestart(t, handler, "192.0.2.1:4000", `{"target":"bundle-fsync"}`)
	if code != http.StatusOK {
		t.Fatalf("idle guest: status=%d body=%s", code, body)
	}
	if syncs != 1 || !markerAtSync {
		t.Fatalf("planned restart answered with syncs=%d markerAtSync=%v; want one filesystem sync after the marker is durable", syncs, markerAtSync)
	}
}
