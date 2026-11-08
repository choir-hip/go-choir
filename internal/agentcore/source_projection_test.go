package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
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
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// U1 wiring (docs/design/computer-update-architecture-rnd-2026-10-24.md,
// "U1 build notes"): what agentcore gathers from `current` and the operations
// table, and what the cast gate and Freeze do with the result. The git
// behavior itself is pinned in internal/sourceproj.

func projectionRuntime(t *testing.T, computerID string) (rt *Runtime, updaterRoot, repo, base string) {
	t.Helper()
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
	patch, _ := releasePatch(t, repo, stackChange())
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatal(err)
	}

	// A deploy moved the booted base to a commit that does not hold A.
	// (It also touched long.txt, which the stack touches, so it is not grafted.)
	other := commitOnTop(t, repo, base, movedBaseChange())
	setBootedBase(t, rt, other)
	err := rt.sourceProjectionCastGate(ctx)
	if err == nil || !strings.HasPrefix(err.Error(), "stack_base_moved") || !strings.Contains(err.Error(), base) || !strings.Contains(err.Error(), other) {
		t.Fatalf("B4: %v", err)
	}
	// S4: this computer has no root Texture RLM, and the refusal says so.
	if !strings.HasSuffix(err.Error(), rebaseNeedsRoot) {
		t.Fatalf("S4: the refusal must name rebase_needs_root: %v", err)
	}

	// The owner promoted A to main and the computer took that base (B5).
	promoted := commitOnTop(t, repo, base, rebasedChange(map[string]string{"frontend/public/other.txt": "platform\n"}))
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

// Capsule overlay S4: there is no switch. A computer with no stored setting
// projects at boot, and the deleted setting key is refused by the API.
func TestSourceProjectionRunsWithNoSettingAndTheKeyIsGone(t *testing.T) {
	const computerID = "computer-setting"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	owner := "owner-setting"
	t.Setenv("CHOIR_OWNER_ID", owner)
	ctx := context.Background()
	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	if result, err := rt.ReconcileSourceProjection(ctx, "boot"); err != nil || result.State != sourceproj.StateProjected || gitHead(t, repo) == base {
		t.Fatalf("boot with no setting: %+v %v", result, err)
	}
	for _, key := range []string{"source.projection", "source.rebase_offer"} {
		req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(fmt.Sprintf(`{"key":%q,"value":false}`, key)))
		req.Header.Set("X-Authenticated-User", owner)
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleComputerSettings(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatalf("%s: the deleted switch was accepted: %s", key, rec.Body)
		}
	}
}

// S4 (failure modes 7, 10, 12): a moved base that left the stack's paths alone
// is grafted; the ordinary Cast is admitted with a ledger warning, and Freeze
// composes a patch on the booted base.
func TestAGraftedStackAdmitsTheCastWithALedgerWarning(t *testing.T) {
	const computerID = "computer-graft"
	t.Setenv("CHOIR_OWNER_ID", "owner-graft")
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()
	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatal(err)
	}
	moved := commitOnTop(t, repo, base, map[string]string{"frontend/public/other.txt": "platform\n"})
	setBootedBase(t, rt, moved)
	cast := castLedgerRef{OwnerID: "owner-graft", ComputerID: computerID, CastRecordID: "cast-1", ManagementAgentID: persistentManagementAgentID("owner-graft"), TrajectoryID: "trajectory-graft"}
	_, pending, err := rt.sourceProjectionCastGateFor(ctx, rebaseSpec{}, cast)
	if err != nil || pending == nil {
		t.Fatalf("7: the Cast on a grafted stack must be admitted with a warning: %v %v", pending, err)
	}
	// 12: the gate records nothing itself; a Cast refused after it leaves no
	// warning. The open records it (openDelegatedCastAssignment).
	if rec, err := rt.store.GetCommitmentRecord(ctx, cast.OwnerID, computerID, pending.RecordID); err != nil || rec != nil {
		t.Fatalf("12: the gate must not record the warning before the open: %+v %v", rec, err)
	}
	rt.recordRematerializedCastWarning(ctx, cast.OwnerID, computerID, pending)
	status := rt.sourceProjectionStatus()
	if status == nil || status.State != sourceproj.StateRematerialized || status.Base != moved || status.ServedBase != base || !status.Diverged {
		t.Fatalf("7: %+v", status)
	}
	if identity := rt.EffectiveRelease(ctx); identity.SourceProjection == nil || identity.SourceProjection.State != sourceproj.StateRematerialized {
		t.Fatalf("7: /health must show stack_rematerialized: %+v", identity.SourceProjection)
	}
	head := gitHead(t, repo)
	if releaseGit(t, repo, "rev-parse", head+"^") != moved {
		t.Fatalf("10: the graft's parent must be the booted base")
	}
	warning, err := rt.store.GetCommitmentRecord(ctx, cast.OwnerID, computerID, rematerializedCastWarningID("cast-1", "op-a", moved))
	if err != nil || warning == nil || warning.ParentID != "cast-1" || warning.Addressee != cast.ManagementAgentID ||
		!strings.Contains(warning.Prediction.Hypothesis, base) || !strings.Contains(warning.Prediction.Hypothesis, moved) || !strings.Contains(warning.Prediction.Hypothesis, "also approves") {
		t.Fatalf("12: the ledger warning: %+v %v", warning, err)
	}
	// A replay of the same Cast gives the same id, so it records nothing new.
	if _, again, err := rt.sourceProjectionCastGateFor(ctx, rebaseSpec{}, cast); err != nil || again == nil || again.RecordID != pending.RecordID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	// A rebase Cast is admitted on a graft (2026-11-04 remat-serve): the
	// capsule starts from the booted base.
	if rebaseBase, _, err := rt.sourceProjectionCastGateFor(ctx, rebaseSpec{Notice: "n", RebaseOf: "op-a", Base: moved}, cast); err != nil || rebaseBase != moved {
		t.Fatalf("a rebase on a graft: base %q err %v, want %s", rebaseBase, err, moved)
	}
	// 10: a capsule cast from the graft freezes a cumulative patch on B'.
	patchB := stagedPatchFrom(t, repo, head, map[string]string{"frontend/public/b.txt": "B\n"})
	staged := stageFreeze(t, patchB, head)
	if err := composeFreezePatch(ctx, repo, staged); err != nil || staged.SourceBaseCommit != moved {
		t.Fatalf("10: Freeze on a graft: base %s err %v, want %s", staged.SourceBaseCommit, err, moved)
	}
}

// S4 (failure modes 1, 5): an owner edit in Source/platform survives a
// reconcile as a saved ref, and the save is on the tape before the reset.
func TestAnOwnerEditIsSavedOnTheTapeBeforeTheReset(t *testing.T) {
	const computerID = "computer-saved"
	ctx := context.Background()
	// A runtime with a real tape (genesis minted), wired for the projection.
	rt := chainBootstrapRuntime(t, computerID)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	if rt.eventAppender, err = computerevent.NewComputerEventAppender(computerID, telemetryTestPinner{rollbackTestPinner{key}}, rt.store, rollbackTestCAS{key: key, projection: rt.store}, rollbackTestReceiptVerifier{}); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.BootstrapChain(ctx, "owner-saved", computerID); err != nil {
		t.Fatal(err)
	}
	operations, err := selfdev.NewStore(rt.store, rt.store)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	updaterRoot := t.TempDir()
	rt.selfdevUpdaterRoot = updaterRoot
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	setBootedBase(t, rt, base)
	writeCurrentRelease(t, updaterRoot, computerID, "base", nil)
	if _, err := rt.ReconcileSourceProjection(ctx, "boot"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "frontend", "public", "index.txt"), []byte("an owner edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := rt.ReconcileSourceProjection(ctx, "boot")
	if err != nil || !result.Changed || len(result.Saved) != 1 {
		t.Fatalf("1: %+v %v", result, err)
	}
	saved := result.Saved[0]
	if got := releaseGit(t, repo, "show", saved.Commit+":frontend/public/index.txt"); got != "an owner edit" {
		t.Fatalf("1: the saved commit holds %q", got)
	}
	if gitHead(t, repo) != base {
		t.Fatalf("the reset must still happen")
	}
	event, found, err := rt.store.EventByIdempotency(ctx, computerID, sourceProjectionSavedIdempotency(saved.Commit))
	if err != nil || !found || event.EventKind != computerevent.EventRecoveryRecorded || event.ProposedEffectRef != saved.Commit {
		t.Fatalf("5: the save must be on the tape: %+v found=%v err=%v", event, found, err)
	}
	raw, err := os.ReadFile(filepath.Join(updaterRoot, filepath.FromSlash(sourceProjectionFactsFile)))
	if err != nil || !strings.Contains(string(raw), sourceproj.FactSaved) || !strings.Contains(string(raw), saved.Ref) {
		t.Fatalf("the guest-local fact: %v\n%s", err, raw)
	}

	// 5: a tape append that fails stops the reset, and the Cast is refused.
	if err := os.WriteFile(filepath.Join(repo, "frontend", "public", "index.txt"), []byte("a second edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	real := appendEventPayload
	t.Cleanup(func() { appendEventPayload = real })
	appendEventPayload = func(*Runtime, context.Context, computerevent.Event, computerevent.TransitionInput, []byte, string, string) (computerevent.Receipt, string, error) {
		return computerevent.Receipt{}, "", errors.New("the tape is down")
	}
	err = rt.sourceProjectionCastGate(ctx)
	if err == nil || !strings.HasPrefix(err.Error(), "source_projection_failed") || !strings.Contains(err.Error(), "the tape is down") {
		t.Fatalf("5: %v", err)
	}
	if raw, _ := os.ReadFile(filepath.Join(repo, "frontend", "public", "index.txt")); string(raw) != "a second edit\n" {
		t.Fatalf("5: the edit was reset without its record: %q", raw)
	}
}

// The b@c shape (rollback-6b427100, 2026-11-08): three applied platform
// operations, the middle one reverted. The rollback re-mints the target's
// manifest (a new content digest), so the served release is owned by no
// applied row: only the rolled_back rollback row carries its digest and the
// target's bundle. The projection must settle on the target's stack, not read
// pending forever.
func TestRevertOfAMidStackOperationSettlesTheProjectionOnTheTargetStack(t *testing.T) {
	const computerID = "computer-midrevert"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()

	patchA, treeA := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	patchB, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n", "frontend/public/b.txt": "B\n"})
	patchC, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n", "frontend/public/b.txt": "B\n", "frontend/public/c.txt": "C\n"})
	bundleA := releaseFrozenBundle(t, updaterRoot, computerID, "op-a", base, patchA)
	releaseA := writeCurrentRelease(t, updaterRoot, computerID, "selfdev-op-a", nil)
	insertReleaseOperation(t, rt.store, releaseOpRow{operationID: "op-a", computerID: computerID, state: selfdev.StateApplied, bundleDigest: bundleA, releaseDigest: releaseA})
	applyOperation(t, rt, updaterRoot, computerID, "op-b", base, patchB)
	applyOperation(t, rt, updaterRoot, computerID, "op-c", base, patchC)

	// Revert op-b: the rollback to op-a installs a re-minted copy of op-a's release.
	reminted := writeCurrentRelease(t, updaterRoot, computerID, "selfdev-op-a", func(m *updater.ReleaseManifest) { m.AcceptedEventHead = strings.Repeat("f", 64) })
	if reminted == releaseA {
		t.Fatal("the rollback must serve a new digest")
	}
	insertReleaseOperation(t, rt.store, releaseOpRow{operationID: "rollback-mid", computerID: computerID, state: selfdev.StateRolledBack, bundleDigest: bundleA, releaseDigest: reminted})

	result, err := rt.ReconcileSourceProjection(ctx, "rollback")
	if err != nil || result.State != sourceproj.StateProjected || result.OperationID != "op-a" {
		t.Fatalf("after the mid-stack revert = %+v, %v (want projected on op-a)", result, err)
	}
	if got := releaseGit(t, repo, "rev-parse", "HEAD^{tree}"); got != treeA {
		t.Fatalf("tree %s, want the target's %s", got, treeA)
	}
	if err := rt.sourceProjectionCastGate(ctx); err != nil {
		t.Fatalf("the gate must open after the revert settles: %v", err)
	}
}

// A base restore serves the booted base manifest (marker genesis-baseline) and
// settles to base with the repository back on the booted base.
func TestBaseRestoreRollbackSettlesTheProjectionToBase(t *testing.T) {
	const computerID = "computer-baserestore"
	rt, updaterRoot, repo, base := projectionRuntime(t, computerID)
	ctx := context.Background()

	patch, _ := releasePatch(t, repo, map[string]string{"frontend/public/a.txt": "A\n"})
	applyOperation(t, rt, updaterRoot, computerID, "op-a", base, patch)
	if r, err := rt.ReconcileSourceProjection(ctx, "apply"); err != nil || r.State != sourceproj.StateProjected {
		t.Fatalf("apply = %+v, %v", r, err)
	}
	baseRelease := writeCurrentRelease(t, updaterRoot, computerID, "genesis-baseline", nil)
	insertReleaseOperation(t, rt.store, releaseOpRow{operationID: "rollback-base", computerID: computerID, state: selfdev.StateRolledBack, releaseDigest: baseRelease})
	result, err := rt.ReconcileSourceProjection(ctx, "rollback")
	if err != nil || result.State != sourceproj.StateBase || gitHead(t, repo) != base {
		t.Fatalf("base restore = %+v, %v", result, err)
	}
}
