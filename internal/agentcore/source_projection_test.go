package agentcore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// U1 wiring (docs/design/computer-update-architecture-rnd-2026-10-24.md,
// "U1 build notes"): what agentcore gathers from `current` and the operations
// table, and what the cast gate and Freeze do with the result. The git
// behavior itself is pinned in internal/sourceproj.

func projectionRuntime(t *testing.T, computerID string) (rt *Runtime, updaterRoot, repo, base string) {
	t.Helper()
	t.Setenv(sourceproj.EnvSwitch, "1")
	rt, _, updaterRoot = releaseTestRuntime(t, computerID)
	repo, base = releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	setBootedBase(t, rt, base)
	return rt, updaterRoot, repo, base
}

func setBootedBase(t *testing.T, rt *Runtime, commit string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "guest-image.manifest")
	if err := os.WriteFile(path, []byte("name=guest\nbuild_commit="+commit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.selfdevGuestImageManifest = path
}

// applyOperation records an applied operation whose release is current.
func applyOperation(t *testing.T, rt *Runtime, updaterRoot, computerID, operationID, baseCommit string, patch []byte) string {
	t.Helper()
	productStore := rt.store
	bundle := releaseFrozenBundle(t, updaterRoot, computerID, operationID, baseCommit, patch)
	release := writeCurrentRelease(t, updaterRoot, computerID, "selfdev-"+operationID, nil)
	insertReleaseOperation(t, productStore, releaseOpRow{operationID: operationID, computerID: computerID, state: selfdev.StateApplied, bundleDigest: bundle, releaseDigest: release})
	return release
}

func gitHead(t *testing.T, repo string) string { return releaseGit(t, repo, "rev-parse", "HEAD") }

func TestReconcileProjectsTheEffectiveReleaseAndTheGateOpens(t *testing.T) {
	const computerID = "computer-projection"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()

	// A base computer: an empty stack on the booted base.
	writeCurrentRelease(t, updaterRoot, computerID, "base", nil)
	result, err := rt.ReconcileSourceProjection(ctx, "boot")
	if err != nil || result.State != sourceproj.StateBase || result.Diverged {
		t.Fatalf("base computer = %+v, %v", result, err)
	}

	// Apply A: the projection follows, divergence is derived, the fact is kept.
	patch, wantTree := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("the gate must open on a stack on the booted base: %v", err)
	}
	head := gitHead(t, repo)
	if head == base || releaseGit(t, repo, "rev-parse", "HEAD^{tree}") != wantTree {
		t.Fatalf("HEAD %s does not carry the applied tree %s", head, wantTree)
	}
	if out := releaseGit(t, repo, "status", "--porcelain", "--untracked-files=no"); out != "" {
		t.Fatalf("work tree not clean: %s", out)
	}
	status := rt.sourceProjectionStatus()
	if status == nil || status.State != sourceproj.StateProjected || !status.Diverged || status.OperationID != "op-a" {
		t.Fatalf("status = %+v", status)
	}
	identity := rt.EffectiveRelease(ctx)
	if identity.Diverged == nil || !*identity.Diverged || identity.SourceProjection == nil || identity.SourceProjection.Head != head {
		t.Fatalf("effective release must carry divergence and the projection: %+v", identity)
	}
	facts, err := os.ReadFile(filepath.Join(updaterRoot, "source-projection", "facts.jsonl"))
	if err != nil || !strings.Contains(string(facts), sourceproj.FactBuilt) || !strings.Contains(string(facts), "op-a") {
		t.Fatalf("facts = %q (%v)", facts, err)
	}

	// Reboot: a no-op that records no new fact.
	before := string(facts)
	if result, err := rt.ReconcileSourceProjection(ctx, "boot"); err != nil || result.Changed {
		t.Fatalf("reboot = %+v, %v", result, err)
	}
	if after, _ := os.ReadFile(filepath.Join(updaterRoot, "source-projection", "facts.jsonl")); string(after) != before {
		t.Fatalf("a no-op reconcile appended a fact")
	}
}

func TestPendingApplyAndMissingBundleRefuseTheCast(t *testing.T) {
	const computerID = "computer-pending"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()

	// P2: current names a selfdev release whose operation is not recorded.
	writeCurrentRelease(t, updaterRoot, computerID, "selfdev-op-in-flight", nil)
	err := rt.sourceProjectionCastGate(ctx)
	if err == nil || !strings.HasPrefix(err.Error(), "source_projection_pending") {
		t.Fatalf("P2: %v", err)
	}
	if gitHead(t, repo) != base {
		t.Fatalf("P2: the repository must not move while an apply is in flight")
	}

	// P5: the operation is applied but its bundle is gone.
	release := writeCurrentRelease(t, updaterRoot, computerID, "selfdev-op-x", nil)
	insertReleaseOperation(t, rt.store, releaseOpRow{operationID: "op-x", computerID: computerID, state: selfdev.StateApplied, bundleDigest: strings.Repeat("1", 64), releaseDigest: release})
	err = rt.sourceProjectionCastGate(ctx)
	if err == nil || !strings.HasPrefix(err.Error(), "source_projection_unrebuildable") || !strings.Contains(err.Error(), "retained") {
		t.Fatalf("P5: %v", err)
	}
	if gitHead(t, repo) != base {
		t.Fatalf("P5: never fall back to the base silently: HEAD moved")
	}
}

func TestBaseMoveRefusesVisiblyAndUpstreamedRetires(t *testing.T) {
	const computerID = "computer-basemove"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()
	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatal(err)
	}

	// A deploy moved the booted base to a commit that does not hold A.
	other := commitOnTop(t, repo, base, map[string]string{"frontend/public/other.txt": "platform\n"})
	setBootedBase(t, rt, other)
	err := rt.sourceProjectionCastGate(ctx)
	if err == nil || !strings.HasPrefix(err.Error(), "stack_base_moved") || !strings.Contains(err.Error(), base) || !strings.Contains(err.Error(), other) {
		t.Fatalf("B4: %v", err)
	}

	// The owner promoted A to main and the computer took that base (B5).
	promoted := commitOnTop(t, repo, base, map[string]string{"frontend/public/a.txt": "A\n", "frontend/public/other.txt": "platform\n"})
	setBootedBase(t, rt, promoted)
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("B5: an upstreamed stack must not refuse: %v", err)
	}
	status := rt.sourceProjectionStatus()
	if status == nil || status.State != sourceproj.StateUpstreamed || status.Diverged || gitHead(t, repo) != promoted {
		t.Fatalf("B5: status %+v head %s", status, gitHead(t, repo))
	}
	identity := rt.EffectiveRelease(ctx)
	if identity.Diverged == nil || *identity.Diverged {
		t.Fatalf("B5: an upstreamed stack is not divergence: %+v", identity.Diverged)
	}
}

// commitOnTop makes a commit on top of base holding files, in repo's object
// store, and returns its id. HEAD and the work tree are not touched.
func commitOnTop(t *testing.T, repo, base string, files map[string]string) string {
	t.Helper()
	scratch := t.TempDir()
	releaseGit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	releaseGit(t, scratch, "checkout", "-q", "-b", "platform", base)
	for rel, content := range files {
		full := filepath.Join(scratch, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, scratch, "add", "-A")
	releaseGit(t, scratch, "commit", "-q", "-m", "platform")
	commit := releaseGit(t, scratch, "rev-parse", "HEAD")
	releaseGit(t, repo, "fetch", "-q", scratch, "platform:refs/platform/"+commit[:8])
	return commit
}

func TestSwitchOffLeavesEverythingAlone(t *testing.T) {
	const computerID = "computer-off"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	t.Setenv(sourceproj.EnvSwitch, "")
	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	ctx := context.Background()
	if result, err := rt.ReconcileSourceProjection(ctx, "boot"); err != nil || result.State != sourceproj.StateDisabled {
		t.Fatalf("P8: %+v %v", result, err)
	}
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("P8: the gate must be open with the switch off: %v", err)
	}
	if gitHead(t, repo) != base {
		t.Fatalf("P8: HEAD moved")
	}
	if identity := rt.EffectiveRelease(ctx); identity.Diverged != nil || identity.SourceProjection != nil {
		t.Fatalf("P8: nothing is reported with the switch off: %+v", identity)
	}
	if _, err := os.Stat(filepath.Join(updaterRoot, "source-projection")); !os.IsNotExist(err) {
		t.Fatalf("P8: no fact log with the switch off")
	}
}

func TestFreezeWritesTheCumulativePatch(t *testing.T) {
	const computerID = "computer-freeze"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()
	patchA, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patchA)
	if _, err := rt.ReconcileSourceProjection(ctx, "apply"); err != nil {
		t.Fatal(err)
	}
	pinned := gitHead(t, repo)

	// The capsule was cast from the projection commit and changed b.txt.
	patchB := stagedPatchFrom(t, repo, pinned, map[string]string{"frontend/public/b.txt": "B\n"})
	staged := stageFreeze(t, patchB, pinned)
	if err := composeFreezePatch(ctx, repo, staged); err != nil {
		t.Fatalf("composeFreezePatch: %v", err)
	}
	if staged.SourceBaseCommit != base {
		t.Fatalf("F1: the bundle's base must be the stack base %s, got %s", base, staged.SourceBaseCommit)
	}
	written, err := os.ReadFile(filepath.Join(staged.TemporaryRoot, "source.patch"))
	if err != nil || computerevent.DigestBytes(written) != staged.SourcePatchSHA {
		t.Fatalf("the digest must bind the written patch: %v", err)
	}
	if !strings.Contains(string(written), "a.txt") || !strings.Contains(string(written), "b.txt") {
		t.Fatalf("F1: the patch must carry A and B:\n%s", written)
	}
	// What the host builder will do: apply it to the booted base alone.
	got, err := release.PatchedTree(ctx, repo, base, written)
	want, wantErr := release.PatchedTree(ctx, repo, pinned, patchB)
	if err != nil || wantErr != nil || got != want {
		t.Fatalf("the cumulative patch on the base must equal the candidate on its pin: %s vs %s (%v, %v)", got, want, err, wantErr)
	}

	// F2/F3: a capsule cast from the plain base keeps today's bytes.
	plain := stageFreeze(t, patchA, base)
	if err := composeFreezePatch(ctx, repo, plain); err != nil || plain.SourceBaseCommit != base || plain.SourcePatchSHA != computerevent.DigestBytes(patchA) {
		t.Fatalf("F2: %v %+v", err, plain)
	}
	// With the switch off nothing is composed.
	t.Setenv(sourceproj.EnvSwitch, "")
	off := stageFreeze(t, patchB, pinned)
	if err := composeFreezePatch(ctx, repo, off); err != nil || off.SourceBaseCommit != pinned || off.SourcePatchSHA != computerevent.DigestBytes(patchB) {
		t.Fatalf("P8: %v %+v", err, off)
	}
}

func stagedPatchFrom(t *testing.T, repo, rev string, files map[string]string) []byte {
	t.Helper()
	scratch := t.TempDir()
	releaseGit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	releaseGit(t, scratch, "checkout", "-q", rev)
	for rel, content := range files {
		full := filepath.Join(scratch, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, scratch, "add", "-A")
	return []byte(releaseGit(t, scratch, "diff", "--cached", "--no-color") + "\n")
}

func stageFreeze(t *testing.T, patch []byte, pinned string) *capsule.StagedRelease {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.patch"), patch, 0o444); err != nil {
		t.Fatal(err)
	}
	return &capsule.StagedRelease{SourcePatch: filepath.Join(root, "source.patch"), SourcePatchSHA: computerevent.DigestBytes(patch), SourceBaseCommit: pinned, TemporaryRoot: root}
}

func TestConcurrentTriggersConvergeOnTheEffectiveRelease(t *testing.T) {
	const computerID = "computer-race"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()
	patch, wantTree := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)

	// P7: boot, apply and cast triggers race; every pass reads the tape under
	// the same lock it reconciles under, so the last one cannot be stale.
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			_, err := rt.ReconcileSourceProjection(ctx, "race")
			done <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatalf("P7: %v", err)
		}
	}
	if releaseGit(t, repo, "rev-parse", "HEAD^{tree}") != wantTree {
		t.Fatalf("P7: HEAD does not carry the applied tree")
	}
}

// An unreadable effective release is unknown, not an empty stack: reconcile
// must refuse before it moves anything (F1, "never falls back to the base").
func TestUnreadableEffectiveReleaseNeverReadsAsAnEmptyStack(t *testing.T) {
	const computerID = "computer-unknown"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()
	patch, wantTree := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	release := applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatal(err)
	}
	head := gitHead(t, repo)

	refuse := func(name string) {
		t.Helper()
		err := rt.sourceProjectionCastGate(ctx)
		if err == nil || !strings.HasPrefix(err.Error(), "source_projection_unrebuildable") {
			t.Fatalf("%s: %v", name, err)
		}
		if gitHead(t, repo) != head || releaseGit(t, repo, "rev-parse", "HEAD^{tree}") != wantTree {
			t.Fatalf("%s: the repository moved off the applied source", name)
		}
	}

	// A manifest the digest check rejects.
	manifestPath := filepath.Join(updaterRoot, "releases", release, "release-manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(manifestPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(raw[:len(raw)-1:len(raw)-1], []byte(" ")...), 0o644); err != nil {
		t.Fatal(err)
	}
	refuse("digest mismatch")
	if err := os.WriteFile(manifestPath, raw, 0o444); err != nil {
		t.Fatal(err)
	}

	// A dangling link: current exists, its target does not.
	if err := os.Remove(filepath.Join(updaterRoot, "current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("releases", strings.Repeat("0", 64)), filepath.Join(updaterRoot, "current")); err != nil {
		t.Fatal(err)
	}
	refuse("dangling current")
	if err := os.Remove(filepath.Join(updaterRoot, "current")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("releases", release), filepath.Join(updaterRoot, "current")); err != nil {
		t.Fatal(err)
	}
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("restored: %v", err)
	}

	// No operation store: the effective release cannot be read at all.
	operations := rt.selfdevOperations
	rt.selfdevOperations = nil
	refuse("nil operation store")
	rt.selfdevOperations = operations

	// Only a missing current link is "no release": a base computer.
	if err := os.Remove(filepath.Join(updaterRoot, "current")); err != nil {
		t.Fatal(err)
	}
	result, err := rt.ReconcileSourceProjection(ctx, "boot")
	if err != nil || result.State != sourceproj.StateBase || gitHead(t, repo) != base {
		t.Fatalf("no current link = %+v, %v", result, err)
	}
}

func TestPendingRefusalNamesTheOperationAndTheOwnerAct(t *testing.T) {
	const computerID = "computer-held"
	rt, updaterRoot, _, _ := projectionRuntime(t, computerID)
	writeCurrentRelease(t, updaterRoot, computerID, "selfdev-op-held", nil)
	insertReleaseOperation(t, rt.store, releaseOpRow{operationID: "op-held", computerID: computerID, state: selfdev.StateMaterializing, bundleDigest: strings.Repeat("1", 64), releaseDigest: strings.Repeat("2", 64)})
	err := rt.sourceProjectionCastGate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "op-held") || !strings.Contains(err.Error(), "materializing") || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("F4: %v", err)
	}
}

// The per-computer switch (FP0 direction 2026-10-24): the owner's setting
// source.projection turns U1 on for one computer at runtime, no deploy; it
// defaults off; the environment switch is independent of it.
func TestSourceProjectionSettingTurnsItOnPerComputerAtRuntime(t *testing.T) {
	const computerID = "computer-setting"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	t.Setenv(sourceproj.EnvSwitch, "")
	sourceproj.SetSetting(false)
	t.Cleanup(func() { sourceproj.SetSetting(false) })
	owner := "owner-setting"
	t.Setenv("CHOIR_OWNER_ID", owner)
	ctx := context.Background()
	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)

	// Default off: nothing runs, and a boot with no stored setting stays off.
	rt.configureSourceProjectionSetting(ctx)
	if sourceproj.Enabled() {
		t.Fatal("the switch reads on with no setting")
	}
	if result, _ := rt.ReconcileSourceProjection(ctx, "boot"); result.State != sourceproj.StateDisabled || gitHead(t, repo) != base {
		t.Fatalf("off: %+v", result)
	}

	post := func(value bool) {
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":"source.projection","value":%v}`, value)))
		req.Header.Set("X-Authenticated-User", owner)
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleComputerSettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("set %v = %d %s", value, rec.Code, rec.Body)
		}
	}
	post(true)
	if !sourceproj.Enabled() {
		t.Fatal("the switch is off after the owner set it")
	}
	// The setting's own reconcile runs in the background; the gate reconciles
	// too, so the outcome does not depend on which runs first.
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("gate: %v", err)
	}
	if gitHead(t, repo) == base {
		t.Fatal("the projection did not follow once the setting was on")
	}

	// A restart reads the stored setting.
	sourceproj.SetSetting(false)
	rt.configureSourceProjectionSetting(ctx)
	if !sourceproj.Enabled() {
		t.Fatal("the stored setting was not read at boot")
	}
	if got, err := rt.store.GetComputerSetting(ctx, owner, rt.cfg.ComputerID, store.SettingSourceProjection); err != nil || !got.Value {
		t.Fatalf("stored = %+v %v", got, err)
	}

	// Off is the rollback: every entry point stops; the repository stays.
	post(false)
	if sourceproj.Enabled() {
		t.Fatal("the switch is on after the owner turned it off")
	}
	head := gitHead(t, repo)
	if result, _ := rt.ReconcileSourceProjection(ctx, "boot"); result.State != sourceproj.StateDisabled || gitHead(t, repo) != head {
		t.Fatalf("off again: %+v", result)
	}
}
