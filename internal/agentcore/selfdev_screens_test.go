package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// Screens become files (docs/problems/selfdev-screens-are-not-files-2026-11-03.md).
// At a fresh Freeze the runtime saves each staged screen as a file in the
// computer's file tree, Self-development/<operation>/screens/<name>, and in the
// asset store under the same sha256 (the bytes a Texture embed and a vision
// call read). Ways it could fail, written before the code:
//   - bytes on disk that are not the bundle's digest are saved (they must be
//     refused, per screen, and named in the result);
//   - a failed save fails the Freeze (it must not: the bundle is the record);
//   - an operation id that is not a plain name escapes the screens directory;
//   - the listing Texture reads names screens before the freeze, for a bundle
//     it does not bind, or for another computer's operation;
//   - the internal key reaches the cell.

func screenFixture(t *testing.T) (frozenRoot string, screens []capsule.ScreenFile, data []byte) {
	t.Helper()
	frozenRoot = t.TempDir()
	data = assetPNG(t)
	dir := filepath.Join(frozenRoot, filepath.FromSlash(capsule.BundleScreensDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "home.png"), data, 0o444); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return frozenRoot, []capsule.ScreenFile{{Name: "home.png", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data)), Width: 40, Height: 30}}, data
}

func TestRecordFreezeScreensSavesFilesTextureCanRead(t *testing.T) {
	rt, _ := testRuntime(t)
	filesRoot := t.TempDir()
	frozenRoot, screens, data := screenFixture(t)
	out := map[string]any{}
	recordFreezeScreens(rt.imageStore(), filesRoot, frozenRoot, "selfdev-op-1", screens, out)

	saved, err := os.ReadFile(filepath.Join(filesRoot, "Self-development", "selfdev-op-1", "screens", "home.png"))
	if err != nil || string(saved) != string(data) {
		t.Fatalf("the screen is not a file in the computer's tree: %v", err)
	}
	if !rt.imageStore().Has(screens[0].SHA256) {
		t.Fatal("the asset store does not hold the screen")
	}
	rows, _ := out["screens"].([]map[string]any)
	if len(rows) != 1 || rows[0]["file"] != "Self-development/selfdev-op-1/screens/home.png" || rows[0]["image_ref"] != "artifact:sha256:"+screens[0].SHA256 || rows[0]["saved"] != true {
		t.Fatalf("rows %v", out["screens"])
	}
	if n, _ := out[freezeScreensKey].(int); n != 1 {
		t.Fatalf("observation %v", out[freezeScreensKey])
	}
	if _, present := out["screen_errors"]; present {
		t.Fatalf("errors %v", out["screen_errors"])
	}
}

func TestRecordFreezeScreensRefusesSwappedBytesWithoutFailingTheFreeze(t *testing.T) {
	rt, _ := testRuntime(t)
	filesRoot := t.TempDir()
	frozenRoot, screens, _ := screenFixture(t)
	screens[0].SHA256 = strings.Repeat("e", 64)
	out := map[string]any{}
	recordFreezeScreens(rt.imageStore(), filesRoot, frozenRoot, "selfdev-op-1", screens, out)
	if _, err := os.Stat(filepath.Join(filesRoot, "Self-development", "selfdev-op-1", "screens", "home.png")); !os.IsNotExist(err) {
		t.Fatalf("swapped bytes were saved: %v", err)
	}
	errs, _ := out["screen_errors"].([]string)
	rows, _ := out["screens"].([]map[string]any)
	if len(errs) != 1 || len(rows) != 1 || rows[0]["saved"] != false {
		t.Fatalf("errors %v rows %v", out["screen_errors"], out["screens"])
	}
	if n, _ := out[freezeScreensKey].(int); n != 0 {
		t.Fatalf("an unsaved screen counts for the Texture embed: %d", n)
	}

	for _, bad := range []string{"../x", "a/b", ""} {
		out := map[string]any{}
		frozenRoot, screens, _ := screenFixture(t)
		recordFreezeScreens(rt.imageStore(), filesRoot, frozenRoot, bad, screens, out)
		if rows, _ := out["screens"].([]map[string]any); len(rows) == 1 && rows[0]["saved"] == true {
			t.Fatalf("operation id %q saved a file", bad)
		}
	}
	empty := map[string]any{}
	recordFreezeScreens(rt.imageStore(), filesRoot, frozenRoot, "selfdev-op-2", nil, empty)
	if len(empty) != 0 {
		t.Fatalf("no screens wrote %v", empty)
	}
}

func TestCandidateScreensListsOnlyTheBoundBundlesScreens(t *testing.T) {
	ctx := context.Background()
	rt, productStore := testRuntime(t)
	computerID := "computer-selfdev-screens"
	rt.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	updaterRoot := t.TempDir()
	rt.selfdevUpdaterRoot = updaterRoot
	_, screens, _ := screenFixture(t)

	writeBundle := func(operationID string, s []capsule.ScreenFile) string {
		bundle := transaction.CapsuleEffectBundle{
			BundleVersion: 1, ComputerID: computerID, BaseEventHead: strings.Repeat("a", 64),
			TrajectoryRef: "trajectory-" + operationID, CapsuleIdentity: "capsule-" + operationID,
			VerifierReceipts: []string{}, Screens: s,
		}
		bundle.ContentDigest, err = bundle.ComputeContentDigest()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := computerevent.CanonicalJSON(bundle)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(updaterRoot, "incoming", bundle.ContentDigest)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bundle.draft.json"), raw, 0o400); err != nil {
			t.Fatal(err)
		}
		return bundle.ContentDigest
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	insert := func(operationID, owner, state, bundleDigest string) {
		if _, err := productStore.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,capsule_id,base_head,prompt_artifact_ref,bundle_digest,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,'[]',?,?,?,?,?)`,
			operationID, owner, "key-"+operationID, strings.Repeat("d", 64), "trajectory-"+operationID, "capsule-"+operationID, strings.Repeat("a", 64),
			"artifact:sha256:"+strings.Repeat("b", 64), bundleDigest, strings.Repeat("c", 64), strings.Repeat("c", 64), state, now, now); err != nil {
			t.Fatal(err)
		}
	}
	insert("op-frozen", computerID, selfdev.StateFrozen, writeBundle("op-frozen", screens))
	insert("op-executing", computerID, selfdev.StateExecuting, "")
	insert("op-plain", computerID, selfdev.StateFrozen, writeBundle("op-plain", nil))
	insert("op-elsewhere", "computer-other", selfdev.StateFrozen, writeBundle("op-elsewhere", screens))

	listing, err := rt.CandidateScreens(ctx, computerID, "op-frozen")
	if err != nil || listing.TrajectoryID != "trajectory-op-frozen" || listing.ComputerID != computerID || len(listing.Screens) != 1 {
		t.Fatalf("listing %+v, %v", listing, err)
	}
	if s := listing.Screens[0]; s.SHA256 != screens[0].SHA256 || s.File != "Self-development/op-frozen/screens/home.png" || s.Width != 40 || s.Resolvable {
		t.Fatalf("screen %+v", s)
	}
	for _, op := range []string{"op-executing", "op-plain", "op-elsewhere", "op-missing"} {
		if _, err := rt.CandidateScreens(ctx, computerID, op); err == nil {
			t.Errorf("%s listed", op)
		}
	}
}

// A crash between the Executing->Frozen transition and recordFreezeScreens
// leaves the bundle citing screens that were never saved. A replayed Freeze
// re-saves them, by digest, and says so.
func TestReplayedFreezeResavesMissingScreensIdempotently(t *testing.T) {
	rt, _ := testRuntime(t)
	updaterRoot, filesRoot := t.TempDir(), t.TempDir()
	staged, screens, data := screenFixture(t)
	computerID := "computer-replay"
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: computerID, BaseEventHead: strings.Repeat("a", 64),
		TrajectoryRef: "trajectory-op-replay", CapsuleIdentity: "capsule-op-replay",
		VerifierReceipts: []string{}, Screens: screens,
	}
	var err error
	bundle.ContentDigest, err = bundle.ComputeContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(updaterRoot, "incoming", bundle.ContentDigest)
	if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(staged, "evidence", "screens"), filepath.Join(dir, "evidence", "screens")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.draft.json"), raw, 0o400); err != nil {
		t.Fatal(err)
	}
	operation := selfdev.Operation{
		OperationID: "op-replay", ComputerID: computerID, TrajectoryID: "trajectory-op-replay", CapsuleID: "capsule-op-replay",
		BaseHead: strings.Repeat("a", 64), BundleDigest: bundle.ContentDigest, State: selfdev.StateFrozen,
	}
	toolCtx := &CapsuleToolCtx{UpdaterRoot: updaterRoot, FilesRoot: filesRoot, Images: rt.imageStore()}
	target := filepath.Join(filesRoot, "Self-development", "op-replay", "screens", "home.png")

	out := map[string]any{}
	toolCtx.resaveReplayedScreens(operation, out)
	if got, err := os.ReadFile(target); err != nil || string(got) != string(data) {
		t.Fatalf("missing screen not re-saved: %v", err)
	}
	if n, _ := out[freezeScreensKey].(int); n != 1 {
		t.Fatalf("repaired count = %v, want 1", out[freezeScreensKey])
	}
	if _, present := out["screen_errors"]; present {
		t.Fatalf("errors %v", out["screen_errors"])
	}
	again := map[string]any{}
	toolCtx.resaveReplayedScreens(operation, again)
	if n, _ := again[freezeScreensKey].(int); n != 0 {
		t.Fatalf("a second replay repaired %v, want 0", again[freezeScreensKey])
	}
	if rows, _ := again["screens"].([]map[string]any); len(rows) != 1 || rows[0]["saved"] != true {
		t.Fatalf("rows %v", again["screens"])
	}
}

// The Files-tree writer never follows a symlink out of the tree.
func TestSaveFrozenScreenRefusesASymlinkedDirectory(t *testing.T) {
	rt, _ := testRuntime(t)
	filesRoot, outside := t.TempDir(), t.TempDir()
	frozenRoot, screens, _ := screenFixture(t)
	if err := os.Symlink(outside, filepath.Join(filesRoot, "Self-development")); err != nil {
		t.Fatal(err)
	}
	out := map[string]any{}
	recordFreezeScreens(rt.imageStore(), filesRoot, frozenRoot, "op-link", screens, out)
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("a screen was written through a symlink: %v", entries)
	}
	if _, present := out["screen_errors"]; !present {
		t.Fatal("a symlinked directory was not reported")
	}

	// A symlink at the file itself is replaced, never followed.
	filesRoot = t.TempDir()
	dir := filepath.Join(filesRoot, "Self-development", "op-leaf", "screens")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(dir, "home.png")); err != nil {
		t.Fatal(err)
	}
	recordFreezeScreens(rt.imageStore(), filesRoot, frozenRoot, "op-leaf", screens, map[string]any{})
	if got, _ := os.ReadFile(victim); string(got) != "keep" {
		t.Fatalf("symlink target overwritten: %q", got)
	}
}
