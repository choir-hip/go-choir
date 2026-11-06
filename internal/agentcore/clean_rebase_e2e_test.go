package agentcore

// The clean rebase end to end (owner ruling 2026-11-04: "I want all this,
// and clean rebases without approval"), on the P4 fixture: the guest runtime
// wired as production, corpusd real where the decision is admitted (owner
// acts in enforce, the event CAS with its admission gate, the host build
// service behind its routes); only the nix build is a stand-in that applies
// the patch at the base with real git.
//
// Ways it can fail, written before the code (also in the commit body):
//   1. it applies something other than what was approved -> the bundle is
//      the approved one with only base and identity fields changed, the patch
//      bytes are the same, and corpusd checks both on the tape
//      (...CleanAppliesWithoutAnOwnerAct, platform TestCleanRebaseGate)
//   2. a clean merge that breaks behaviour -> the host build (frontend build,
//      package check, Go build) runs before the decision; a failed build is no
//      decision and the pair goes to the notice (...BuildFailureGoesToTheNotice)
//   3. a conflict is applied -> ...ConflictMintsTheNotice
//   4. a loop of rebases -> one attempt per (operation, booted base) pair;
//      after it applies the stack is projected, so no further attempt or
//      notice (...CleanAppliesWithoutAnOwnerAct, second fire)
//   5. a crash or planned restart mid-attempt resumes it -> no resumer: an
//      open pre-decision attempt with no live driver is closed "interrupted
//      by a restart" and the pair goes to the notice (...InterruptedAttemptIsClosed)
//   6. a computer that is not a test computer (owner ruling 2026-11-04: every
//      computer works the same) -> the fixture is an owner-tier computer, in
//      neither tier file, and takes the clean rebase through the real gate; an
//      engineering capsule a restart would destroy holds the attempt, before
//      and after the build (...LiveCapsuleHolds..., ...OwnerTierComputer...)
//   7. the base moves again mid-attempt -> the attempt fails, nothing is
//      applied (...SecondBaseMoveFailsTheAttempt)

import (
	"context"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/computertiers"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

type cleanRebaseE2E struct {
	*p4Fixture
	guestRepo string
	b1, prior string // the base the prior change was approved on; its operation
	manifest  string
	owner     string
	// pkg, when set, is a package file (under frontend/src/packages/) that B1
	// holds and the prior change edits with long.txt.
	pkg string
}

func cleanLong(edits map[int]string) string {
	var b strings.Builder
	for i := 1; i <= 40; i++ {
		if line, ok := edits[i]; ok {
			b.WriteString(line + "\n")
			continue
		}
		fmt.Fprintf(&b, "long line %02d\n", i)
	}
	return b.String()
}

// commitPlatform commits files on the host's platform history and returns
// the commit; the history's HEAD and work tree follow it.
func (f *cleanRebaseE2E) commitPlatform(t *testing.T, files map[string]string) string {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(f.history.dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, f.history.dir, "add", "-A")
	gitRun(t, f.history.dir, "commit", "-q", "-m", "platform")
	commit := gitRun(t, f.history.dir, "rev-parse", "HEAD")
	if f.guestRepo != "" {
		gitRun(t, f.guestRepo, "fetch", "-q", f.history.dir, "HEAD:refs/platform/"+commit[:12])
	}
	return commit
}

// boot names commit as the booted base, for the guest and the host alike.
func (f *cleanRebaseE2E) boot(t *testing.T, commit string) {
	t.Helper()
	_ = os.Chmod(f.manifest, 0o644)
	if err := os.WriteFile(f.manifest, []byte("contract=choir-guest-image-v1\nbuild_commit="+commit+"\n"), 0o444); err != nil {
		t.Fatal(err)
	}
}

// tape is the corpusd tape as the fixture keeps it: event bytes from the
// chain, payload bytes from the pinner.
func (f *cleanRebaseE2E) tape(computerID string, ns tapestore.NS, digest string) ([]byte, error) {
	switch ns {
	case tapestore.NSPayload:
		if raw, ok := f.pinner.fetch(digest); ok {
			return raw, nil
		}
	case tapestore.NSEvent:
		// The guest's projection, not the chain fake: the gate runs under the
		// fake's lock.
		if event, found, err := f.store.EventByDigest(context.Background(), computerID, digest); err == nil && found {
			return event.CanonicalBytes()
		}
	}
	return nil, fmt.Errorf("%s/%s not on the tape", ns, digest)
}

// newCleanRebaseE2E applies a prior change (a 40-line file edited at line 5,
// approved by standing consensus) on base B1, with a root Texture so a
// notice can be minted. The computer is in no tier file: an owner-tier computer.
func newCleanRebaseE2E(t *testing.T) *cleanRebaseE2E { return newCleanRebaseE2EWith(t, "") }

func newCleanRebaseE2EWith(t *testing.T, pkg string) *cleanRebaseE2E {
	t.Helper()
	f := &cleanRebaseE2E{p4Fixture: newP4Fixture(t), owner: "owner", pkg: pkg}
	f.manifest = f.rt.selfdevGuestImageManifest
	b1Files := map[string]string{"frontend/public/long.txt": cleanLong(nil)}
	if pkg != "" {
		b1Files[pkg] = cleanLong(nil)
	}
	f.b1 = f.commitPlatform(t, b1Files)
	f.history.base = f.b1
	f.boot(t, f.b1)
	f.guestRepo = t.TempDir()
	gitRun(t, f.guestRepo, "clone", "-q", f.history.dir, ".")
	f.authority.ConfigureCleanRebase(f.tape, f.hostBuilds)

	seedOwnerActDocument(t, f.store, f.owner, f.computerID, "doc-master")
	if _, err := f.store.ClaimTextureRoot(context.Background(), nil, store.TextureRoot{OwnerID: f.owner, ComputerID: f.computerID, DocumentID: "doc-master", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: f.owner, ComputerID: f.computerID, Key: store.SettingTextureRootDesk, Value: true}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	f.configure(func(rt *Runtime) {
		rt.releaseSourceDir = f.guestRepo
		rt.selfdevRouteOwnerID = f.owner
		rt.bootedAt = now.Add(-time.Hour)
		rt.rebaseNow = func() time.Time { return now }
		rt.rebaseAfterFunc = func(time.Duration, func()) rebaseTimerHandle { return noopRebaseTimer{} }
		rt.cleanRebaseStart = func(run func()) { run() }
		rt.cleanRebaseAttemptBudget = time.Minute
	})
	t.Cleanup(f.rt.stopRebaseNotice)

	// The prior change, decided by consensus on a computer with no mode row.
	prior := f.verify(t, f.freezeWithPatch(t, "clean-prior", "the approved change", f.longPatch(t, map[int]string{5: "stack line 05"})))
	if _, err := f.rt.runStandingConsensus(context.Background(), prior.OperationID, "run:management-p4"); err != nil {
		t.Fatalf("prior approval: %v", err)
	}
	f.waitForState(t, prior.OperationID, selfdev.StateApplied)
	f.prior = prior.OperationID
	if result, err := f.rt.ReconcileSourceProjection(context.Background(), "test"); err != nil || result.State != sourceproj.StateProjected {
		t.Fatalf("the prior change is not projected: %+v %v", result, err)
	}
	return f
}

// longPatch is a patch against B1 editing long.txt.
func (f *cleanRebaseE2E) longPatch(t *testing.T, edits map[int]string) []byte {
	t.Helper()
	path := filepath.Join(f.history.dir, "frontend", "public", "long.txt")
	if err := os.WriteFile(path, []byte(cleanLong(edits)), 0o644); err != nil {
		t.Fatal(err)
	}
	if f.pkg != "" {
		if err := os.WriteFile(filepath.Join(f.history.dir, filepath.FromSlash(f.pkg)), []byte(cleanLong(edits)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, f.history.dir, "add", "-A")
	patch := gitRun(t, f.history.dir, "diff", "--cached", "--binary") + "\n"
	gitRun(t, f.history.dir, "reset", "-q", "--hard", f.b1)
	return []byte(patch)
}

func (f *cleanRebaseE2E) fire(t *testing.T) rebaseNoticeOutcome {
	t.Helper()
	return f.rt.fireRebaseNotice(context.Background())
}

func (f *cleanRebaseE2E) cleanOp(t *testing.T, booted string) (selfdev.Operation, bool) {
	t.Helper()
	op, found, err := f.operations.GetByIdempotency(context.Background(), f.computerID, cleanRebaseIdempotencyKey(f.prior, booted))
	if err != nil {
		t.Fatal(err)
	}
	return op, found
}

func (f *cleanRebaseE2E) noticeMinted(t *testing.T, booted string) bool {
	t.Helper()
	rec, err := f.store.GetCommitmentRecord(context.Background(), f.owner, f.computerID, types.StackRebaseNoticeID(f.prior, booted, 1))
	if err != nil {
		t.Fatal(err)
	}
	return rec != nil
}

func TestCleanRebaseEndToEndCleanAppliesWithoutAnOwnerAct(t *testing.T) {
	f := newCleanRebaseE2E(t)
	ctx := context.Background()
	restarts := f.service.restarts
	// A deploy moves the base: long.txt line 35 changes, far from line 5.
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/long.txt": cleanLong(map[int]string{35: "platform line 35"})})
	f.boot(t, b2)
	if result, err := f.rt.ReconcileSourceProjection(ctx, rebaseReconcileReason); err != nil || result.State != sourceproj.StateBaseMoved {
		t.Fatalf("the deploy did not move the stack's base: %+v %v", result, err)
	}

	out := f.fire(t)
	if out.Minted != "" || f.noticeMinted(t, b2) {
		t.Fatalf("a clean rebase minted a notice: %+v", out)
	}
	op, found := f.cleanOp(t, b2)
	if !found {
		t.Fatalf("no clean rebase operation: %+v", out)
	}
	applied := f.waitForState(t, op.OperationID, selfdev.StateApplied)

	// The decision is the receipt's, not an owner's, and corpusd admitted it.
	decision := f.decisionEvent(t, applied.OperationID)
	receiptDigest, ok := selfdevprotocol.CleanRebaseReceiptDigestFromAuthority(decision.AuthorityRef)
	if decision.EventKind != computerevent.EventEffectAccepted || !ok || applied.DecisionActor != decision.AuthorityRef {
		t.Fatalf("decision %q actor %q", decision.AuthorityRef, applied.DecisionActor)
	}
	rawReceipt, found := f.pinner.fetch(receiptDigest)
	if !found {
		t.Fatal("the receipt is not on the tape")
	}
	receipt, err := selfdevprotocol.DecodeCleanRebaseReceipt(rawReceipt)
	if err != nil || receipt.RebaseOf != f.prior || receipt.BootedBase != b2 || receipt.StackBase != f.b1 ||
		receipt.Statement != "clean re-application of "+f.prior+" onto "+b2+", applied without an owner act under the owner's 2026-11-04 ruling" {
		t.Fatalf("receipt = %+v %v", receipt, err)
	}
	for _, record := range f.cas.snapshot() {
		if strings.HasPrefix(record.Request.Event.AuthorityRef, "external-owner:") {
			t.Fatalf("an owner authority reached the chain: %+v", record.Request.Event)
		}
	}
	// Same patch bytes; the applied release is the host build of it on B2.
	priorOp, _ := f.operations.Get(ctx, f.computerID, f.prior)
	priorPatch, _ := os.ReadFile(filepath.Join(f.updaterRoot, "incoming", priorOp.BundleDigest, "source.patch"))
	newPatch, _ := os.ReadFile(filepath.Join(f.updaterRoot, "incoming", applied.BundleDigest, "source.patch"))
	if len(priorPatch) == 0 || string(priorPatch) != string(newPatch) {
		t.Fatal("the clean rebase does not carry the approved patch bytes")
	}
	built := f.hostBuildOf(t, applied)
	current, err := updater.ReadCurrentManifest(f.updaterRoot)
	if err != nil || current.ContentDigest != applied.ReleaseDigest || current.ClosureDigest != built.Result.ClosureDigest || built.BaseCommit != b2 {
		t.Fatalf("applied release is not the host build on B2: job=%+v current=%+v %v", built, current, err)
	}
	if f.service.restarts != restarts+1 {
		t.Fatalf("restarts %d -> %d, want one planned restart", restarts, f.service.restarts)
	}

	// The stack left stack_base_moved, and the next fire does nothing.
	result, err := f.rt.ReconcileSourceProjection(ctx, rebaseReconcileReason)
	if err != nil || result.State != sourceproj.StateProjected || result.OperationID != applied.OperationID || result.Base != b2 {
		t.Fatalf("after the clean rebase the stack is %+v %v", result, err)
	}
	if again := f.fire(t); again.Minted != "" || !strings.Contains(again.Reason, "not stack_base_moved") {
		t.Fatalf("second fire = %+v", again)
	}
}

func TestCleanRebaseEndToEndConflictMintsTheNotice(t *testing.T) {
	f := newCleanRebaseE2E(t)
	// The deploy edits line 5 itself: the approved patch no longer applies.
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/long.txt": cleanLong(map[int]string{5: "platform line 05"})})
	f.boot(t, b2)
	out := f.fire(t)
	if _, found := f.cleanOp(t, b2); found {
		t.Fatal("a conflict opened a clean rebase operation")
	}
	if out.Minted == "" || !f.noticeMinted(t, b2) {
		t.Fatalf("a conflict did not mint the notice: %+v", out)
	}
}

func TestCleanRebaseEndToEndBuildFailureGoesToTheNotice(t *testing.T) {
	f := newCleanRebaseE2E(t)
	restarts := f.service.restarts
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/long.txt": cleanLong(map[int]string{35: "platform line 35"})})
	f.boot(t, b2)
	// The host's build of B2 + the patch fails (its tree no longer takes it).
	if err := os.WriteFile(filepath.Join(f.history.dir, "frontend", "public", "long.txt"), []byte(cleanLong(map[int]string{5: "host-side drift"})), 0o644); err != nil {
		t.Fatal(err)
	}
	out := f.fire(t)
	if out.Minted != "" {
		t.Fatalf("the first fire minted a notice beside a clean attempt: %+v", out)
	}
	op, found := f.cleanOp(t, b2)
	if !found || op.State != selfdev.StateFailed || !strings.Contains(op.TerminalError, "host build failed") {
		t.Fatalf("a failed build must fail the attempt: %+v", op)
	}
	if out := f.fire(t); out.Minted == "" || !f.noticeMinted(t, b2) {
		t.Fatalf("a failed build did not mint the notice: %+v", out)
	}
	if f.service.restarts != restarts {
		t.Fatal("a failed clean rebase restarted the computer")
	}
	if current, _ := updater.ReadCurrentManifest(f.updaterRoot); current.BaseCommit == b2 {
		t.Fatal("a failed clean rebase changed the release")
	}
}

func TestCleanRebaseEndToEndSecondBaseMoveFailsTheAttempt(t *testing.T) {
	f := newCleanRebaseE2E(t)
	restarts := f.service.restarts
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/long.txt": cleanLong(map[int]string{35: "platform line 35"})})
	f.boot(t, b2)
	var run func()
	f.configure(func(rt *Runtime) { rt.cleanRebaseStart = func(r func()) { run = r } })
	if out := f.fire(t); out.Minted != "" || run == nil {
		t.Fatalf("the attempt did not start: %+v", out)
	}
	// While the attempt waits to run, another deploy lands.
	b3 := f.commitPlatform(t, map[string]string{"frontend/public/other.txt": "another deploy\n"})
	f.boot(t, b3)
	run()
	op, _ := f.cleanOp(t, b2)
	if op.State != selfdev.StateFailed {
		t.Fatalf("an attempt whose base moved again ended %s (%s)", op.State, op.TerminalError)
	}
	if f.service.restarts != restarts {
		t.Fatal("an attempt on a stale base restarted the computer")
	}
}

func TestCleanRebaseEndToEndInterruptedAttemptIsClosed(t *testing.T) {
	f := newCleanRebaseE2E(t)
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/long.txt": cleanLong(map[int]string{35: "platform line 35"})})
	f.boot(t, b2)
	// A previous process opened the attempt and died before deciding.
	key := cleanRebaseIdempotencyKey(f.prior, b2)
	priorOp, _ := f.operations.Get(context.Background(), f.computerID, f.prior)
	if _, err := f.operations.Start(context.Background(), selfdev.StartRequest{
		ComputerID: f.computerID, IdempotencyKey: key, PromptArtifactRef: priorOp.PromptArtifactRef,
		OperationID: cleanRebaseOperationID(key), TrajectoryID: priorOp.TrajectoryID,
	}); err != nil {
		t.Fatal(err)
	}
	out := f.fire(t)
	op, _ := f.cleanOp(t, b2)
	if op.State != selfdev.StateFailed || op.TerminalError != selfdevInterruptedByRestart {
		t.Fatalf("an orphaned attempt must close as interrupted: %+v", op)
	}
	if out.Minted == "" || !f.noticeMinted(t, b2) {
		t.Fatalf("the interrupted pair did not go to the notice: %+v", out)
	}
}

// Owner ruling 2026-11-04: automatic rebase works the same on every computer.
// The fixture computer is in neither tier file, so the happy-path tests are
// the owner-tier computer taking a clean rebase through the real corpusd gate;
// this pins that fact so a tier gate cannot return unseen.
func TestCleanRebaseEndToEndOwnerTierComputerIsInNoTierFile(t *testing.T) {
	f := newCleanRebaseE2E(t)
	tiers, err := computertiers.Load()
	if err != nil {
		t.Fatal(err)
	}
	if tiers.TestComputer(f.computerID) || tiers.PlatformComputer(f.computerID) {
		t.Fatalf("the fixture computer %s is in a tier file; it must be an owner-tier computer", f.computerID)
	}
}

// A live engineering capsule is destroyed by the apply's restart (it lives in
// the executor's memory), and it can exist with no operation. On a computer
// with owner work in flight, the pair holds (no notice, no attempt) and the
// attempt proceeds once the capsule ends.
func TestCleanRebaseEndToEndLiveCapsuleHoldsThenProceeds(t *testing.T) {
	f := newCleanRebaseE2E(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(f.store, f.owner, f.computerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-live-capsule")
	if _, err := f.store.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.BindEngineeringAssignment(ctx, store.SeedEngineeringBindRequest(open, "run:"+seed.AssignmentIDs[0], "opaque-live-capsule")); err != nil {
		t.Fatal(err)
	}
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/long.txt": cleanLong(map[int]string{35: "platform line 35"})})
	f.boot(t, b2)
	restarts := f.service.restarts
	out := f.fire(t)
	if out.Minted != "" || f.noticeMinted(t, b2) || out.Rearm <= 0 || !strings.Contains(out.Reason, "live engineering capsule") {
		t.Fatalf("a live capsule did not hold the pair: %+v", out)
	}
	if _, found := f.cleanOp(t, b2); found || f.service.restarts != restarts {
		t.Fatal("a live capsule let a clean rebase start")
	}
	if hold := f.rt.cleanRebaseQuietHold(ctx); hold == "" {
		t.Fatal("the quiet check reads busy as quiet")
	}
}

// Devin F4: a decided clean rebase that never finishes must not hold the
// notice back forever.
func TestCleanRebaseEndToEndWedgedDecidedAttemptFallsThroughToTheNotice(t *testing.T) {
	f := newCleanRebaseE2E(t)
	now := f.rt.rebaseClock()
	for _, state := range []string{selfdev.StateAccepted, selfdev.StateMaterializing} {
		fresh := selfdev.Operation{ComputerID: f.computerID, OperationID: "clean-rebase-x", State: state, UpdatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano)}
		if hold, why := f.rt.cleanRebaseExisting(context.Background(), "k", fresh); !hold {
			t.Fatalf("%s within the bound fell through: %s", state, why)
		}
		wedged := fresh
		wedged.UpdatedAt = now.Add(-3 * time.Minute).Format(time.RFC3339Nano)
		if hold, why := f.rt.cleanRebaseExisting(context.Background(), "k", wedged); hold || !strings.Contains(why, "wedged") {
			t.Fatalf("%s past the bound held: %v %s", state, hold, why)
		}
	}
}

// The rematerialized stack (docs/problems/rematerialized-stack-never-served-
// 2026-11-04.md). Ways it can fail, written before the tests:
//   1. the served build stays on the old base while the projection moves ->
//      a stack_rematerialized stack with a clean patch is rebuilt and applied
//      with no owner act (...RematerializedCleanAppliesWithoutAnOwnerAct)
//   2. a package kept whole over the base's change is applied silently ->
//      it goes to the notice, no operation opens
//      (...RematerializedPackageConflictMintsTheNotice)
//   3. a loop: a computer whose served build is the booted base is acted on
//      again -> nothing is armed, opened or minted (...ServedEqualsBootedDoesNothing)
//   4. a second attempt for the pair -> the second fire opens nothing and
//      mints nothing (...RematerializedSecondFireMakesNoSecondAttempt)

func (f *cleanRebaseE2E) requireRematerialized(t *testing.T, booted string) sourceproj.Result {
	t.Helper()
	result, err := f.rt.ReconcileSourceProjection(context.Background(), rebaseReconcileReason)
	if err != nil || result.State != sourceproj.StateRematerialized || result.ServedBase != f.b1 || result.Base != booted {
		t.Fatalf("the deploy did not rematerialize the stack: %+v %v", result, err)
	}
	return result
}

func TestCleanRebaseEndToEndRematerializedCleanAppliesWithoutAnOwnerAct(t *testing.T) {
	f := newCleanRebaseE2E(t)
	ctx := context.Background()
	restarts := f.service.restarts
	// The deploy changes a file the patch does not touch: a graft, not a move.
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/other.txt": "a platform change\n"})
	f.boot(t, b2)
	result := f.requireRematerialized(t, b2)
	f.rt.armRebaseNotice(ctx, result)
	if _, armed := f.rt.rebaseNoticeDueAt(); !armed {
		t.Fatal("a rematerialized stack behind the booted base did not arm the timer")
	}
	f.rt.stopRebaseNotice()

	out := f.fire(t)
	if out.Minted != "" || f.noticeMinted(t, b2) {
		t.Fatalf("a clean rebase minted a notice: %+v", out)
	}
	op, found := f.cleanOp(t, b2)
	if !found {
		t.Fatalf("no clean rebase operation: %+v", out)
	}
	applied := f.waitForState(t, op.OperationID, selfdev.StateApplied)
	decision := f.decisionEvent(t, applied.OperationID)
	if _, ok := selfdevprotocol.CleanRebaseReceiptDigestFromAuthority(decision.AuthorityRef); !ok || applied.DecisionActor != decision.AuthorityRef {
		t.Fatalf("decision %q actor %q", decision.AuthorityRef, applied.DecisionActor)
	}
	for _, record := range f.cas.snapshot() {
		if strings.HasPrefix(record.Request.Event.AuthorityRef, "external-owner:") {
			t.Fatalf("an owner authority reached the chain: %+v", record.Request.Event)
		}
	}
	if f.service.restarts != restarts+1 {
		t.Fatalf("restarts %d -> %d, want one planned restart", restarts, f.service.restarts)
	}
	// The served build is now the booted base's: projected, not rematerialized.
	after, err := f.rt.ReconcileSourceProjection(ctx, rebaseReconcileReason)
	if err != nil || after.State != sourceproj.StateProjected || after.OperationID != applied.OperationID || after.Base != b2 {
		t.Fatalf("after the clean rebase the stack is %+v %v", after, err)
	}
	if again := f.fire(t); again.Minted != "" || !strings.Contains(again.Reason, "not stack_base_moved") {
		t.Fatalf("second fire = %+v", again)
	}
}

// Devin F2 (remat-serve review): a rematerialized stack admits ordinary Casts,
// so an operation is routinely open at fire time. The pair must hold (no
// notice, no attempt) and proceed at the next fire once the operation closes.
func TestCleanRebaseEndToEndOpenOperationHoldsTheNoticeThenProceeds(t *testing.T) {
	f := newCleanRebaseE2E(t)
	ctx := context.Background()
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/other.txt": "a platform change\n"})
	f.boot(t, b2)
	f.requireRematerialized(t, b2)
	open := f.freezeWithPatch(t, "busy-op", "a change in flight", f.longPatch(t, map[int]string{9: "busy line 09"}))
	if _, isOpen, err := f.operations.OpenOperation(ctx, f.computerID); err != nil || !isOpen {
		t.Fatalf("the fixture has no open operation: %v", err)
	}
	restarts := f.service.restarts
	out := f.fire(t)
	if out.Minted != "" || f.noticeMinted(t, b2) || out.Rearm <= 0 || !strings.Contains(out.Reason, "operation") {
		t.Fatalf("an open operation did not hold the pair: %+v", out)
	}
	if _, found := f.cleanOp(t, b2); found || f.service.restarts != restarts {
		t.Fatal("an open operation let a clean rebase start")
	}
	// The operation closes; the next fire proceeds (the clean rebase here).
	if _, err := f.operations.Transition(ctx, f.computerID, open.OperationID, open.State, selfdev.StateFailed, nil); err != nil {
		t.Fatalf("close the operation: %v", err)
	}
	if next := f.fire(t); next.Minted != "" {
		t.Fatalf("the next fire minted beside a clean attempt: %+v", next)
	}
	if _, found := f.cleanOp(t, b2); !found {
		t.Fatal("the next fire after the operation closed did not proceed")
	}
}

func TestCleanRebaseEndToEndServedEqualsBootedDoesNothing(t *testing.T) {
	f := newCleanRebaseE2E(t)
	ctx := context.Background()
	// No deploy: the served build is the booted base.
	result, err := f.rt.ReconcileSourceProjection(ctx, rebaseReconcileReason)
	if err != nil || result.State != sourceproj.StateProjected {
		t.Fatalf("the stack is %+v %v", result, err)
	}
	if _, behind := rebaseStackBase(result); behind {
		t.Fatalf("a stack on the booted base is behind: %+v", result)
	}
	f.rt.armRebaseNotice(ctx, result)
	if _, armed := f.rt.rebaseNoticeDueAt(); armed {
		t.Fatal("the timer armed on a stack that is on the booted base")
	}
	// The same for a rematerialized result that names the booted base as served.
	same := sourceproj.Result{State: sourceproj.StateRematerialized, ServedBase: result.Base, BootedBase: result.Base, Base: result.Base}
	if _, behind := rebaseStackBase(same); behind {
		t.Fatal("served == booted counts as behind")
	}
	restarts := f.service.restarts
	if out := f.fire(t); out.Minted != "" {
		t.Fatalf("fire minted: %+v", out)
	}
	if _, found := f.cleanOp(t, f.b1); found || f.noticeMinted(t, f.b1) || f.service.restarts != restarts {
		t.Fatal("a stack on the booted base was acted on")
	}
}

func TestCleanRebaseEndToEndRematerializedSecondFireMakesNoSecondAttempt(t *testing.T) {
	f := newCleanRebaseE2E(t)
	ctx := context.Background()
	b2 := f.commitPlatform(t, map[string]string{"frontend/public/other.txt": "a platform change\n"})
	f.boot(t, b2)
	f.requireRematerialized(t, b2)
	// The host's build fails: the pair's one attempt ends failed.
	if err := os.WriteFile(filepath.Join(f.history.dir, "frontend", "public", "long.txt"), []byte(cleanLong(map[int]string{5: "host-side drift"})), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := f.fire(t); out.Minted != "" {
		t.Fatalf("the first fire minted beside a clean attempt: %+v", out)
	}
	op, found := f.cleanOp(t, b2)
	if !found || op.State != selfdev.StateFailed {
		t.Fatalf("the attempt did not fail: %+v", op)
	}
	if out := f.fire(t); out.Minted == "" {
		t.Fatalf("the failed pair did not go to the notice: %+v", out)
	}
	restarts := f.service.restarts
	f.fire(t)
	again, _ := f.cleanOp(t, b2)
	if again.OperationID != op.OperationID || again.State != selfdev.StateFailed {
		t.Fatalf("a second attempt opened: %+v", again)
	}
	if open, isOpen, err := f.operations.OpenOperation(ctx, f.computerID); err != nil || isOpen {
		t.Fatalf("an operation is open after the second fire: %+v %v", open, err)
	}
	if f.service.restarts != restarts {
		t.Fatal("a second fire restarted the computer")
	}
}
