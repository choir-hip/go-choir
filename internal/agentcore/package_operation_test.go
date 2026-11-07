package agentcore

// The package branch of the materializer, the package revert and the platform
// layer's separation from it (docs/design/package-operation-decision-2026-11-06.md
// 2.3 and 2.5). Ways this could fail, written first:
//   - a build, uses or install-review refusal still publishes the release;
//   - a crash between publish and record leaves the operation materializing,
//     or settles it by driving the branch again (a second publish);
//   - a listed release with no install is recorded applied, or one with an
//     install is closed failed;
//   - a package revert calls the updater, creates a rollback row, or leaves the
//     install in force;
//   - a platform revert whose prior is a package event restores the package's
//     state, or undoes the package install;
//   - a package operation enters stack bookkeeping (a release readers match,
//     a rebase offer, a clean rebase, the live platform set).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

type packageBundleSpec struct {
	patch   []byte
	base    string
	screens map[string][]byte
}

const newPackageName = "demo-new"
const newPackageRoot = "frontend/src/packages/" + newPackageName

// fakePackagePublisher stands in for corpusd's publish-by-decision route.
type fakePackagePublisher struct {
	mu        sync.Mutex
	publishes []platform.PackagePublishByDecisionRequest
	// publish answers a publish; nil echoes the request's release id.
	publish func(platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error)
	// list answers the Library lookup; nil means the publisher offers no
	// lookup route (the fallback on install records).
	list func(operationID string) ([]PackageReleaseRef, error)
}

func (f *fakePackagePublisher) PublishPackageByDecision(_ context.Context, request platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
	f.mu.Lock()
	f.publishes = append(f.publishes, request)
	publish := f.publish
	f.mu.Unlock()
	if publish != nil {
		return publish(request)
	}
	return platform.PackageReleaseView{ReleaseID: request.ReleaseID}, http.StatusCreated, nil
}

// fakeListingPublisher adds the lookup half.
type fakeListingPublisher struct{ *fakePackagePublisher }

func (f fakeListingPublisher) PackageReleasesForOperation(_ context.Context, operationID string) ([]PackageReleaseRef, error) {
	f.mu.Lock()
	list := f.list
	f.mu.Unlock()
	if list == nil {
		return nil, nil
	}
	return list(operationID)
}

func (f *fakePackagePublisher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.publishes)
}

// uses-widening builder: a module that imports an SDK name its manifest does
// not hold the grant of.
type mutatingBuilder struct {
	inner  *fakeModuleBuilder
	mutate func(*pkgruntime.Module)
}

func (b *mutatingBuilder) Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error) {
	module, err := b.inner.Build(ctx, rec, treePatch)
	if err == nil && b.mutate != nil {
		b.mutate(&module)
	}
	return module, err
}

type packageOpWorld struct {
	fx      *derivableSelfDevFixture
	repo    string
	base    string
	builder *fakeModuleBuilder
	pub     *fakePackagePublisher
	files   map[string]string
}

func newPackageOpWorld(t *testing.T, name string) *packageOpWorld {
	t.Helper()
	fx := joinFixture(t, "pkg-"+name, joinTestBaseA, joinTestBaseA)
	repo, base := packageSourceRepo(t)
	w := &packageOpWorld{fx: fx, repo: repo, base: base, builder: &fakeModuleBuilder{}, pub: &fakePackagePublisher{}}
	w.files = map[string]string{
		newPackageRoot + "/choir-package.json":  strings.ReplaceAll(strings.ReplaceAll(draftManifest, `"reader"`, `"`+newPackageName+`"`), "Reader", "Demo new"),
		newPackageRoot + "/App.svelte":          "<p>demo new</p>\n",
		newPackageRoot + "/choir-contract.json": `{"schema": "choir.package-contract/v1", "ready": "[data-demo]"}` + "\n",
	}
	fx.configure(func(rt *Runtime) {
		rt.releaseSourceDir = repo
		rt.packageBuilder = w.builder
		rt.packageModules = pkgruntime.Store{Dir: t.TempDir()}
		rt.packagePublisher = fakeListingPublisher{w.pub}
	})
	return w
}

// propose freezes a package operation (kind package), has it decided by a
// consensus authority and returns it. The materializer's observer drives it
// from there.
func (w *packageOpWorld) propose(t *testing.T, key string) selfdev.Operation {
	t.Helper()
	patch, _ := releasePatch(t, w.repo, w.files)
	w.fx.packageSpec = &packageBundleSpec{patch: patch, base: w.base, screens: map[string][]byte{"home.png": mediaPNG(t, 64, 40, 3, false)}}
	defer func() { w.fx.packageSpec = nil }()
	operation, bundleDigest, _ := w.fx.seedOperationToAwaitingApproval(t, key)
	if !operation.IsPackage() {
		// The seed returns the awaiting row; re-read for the kind.
		var err error
		if operation, err = w.fx.operations.Get(context.Background(), w.fx.computerID, operation.OperationID); err != nil || !operation.IsPackage() {
			t.Fatalf("the frozen operation is not a package operation: %+v err=%v", operation, err)
		}
	}
	w.appendConsensusDecision(t, operation, bundleDigest, "decision-"+key)
	return operation
}

func (w *packageOpWorld) appendConsensusDecision(t *testing.T, operation selfdev.Operation, bundleDigest, idempotencyKey string) {
	t.Helper()
	head := w.fx.waitForStableHead(t)
	id, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	consensus := computerevent.DigestBytes([]byte("consensus-" + operation.OperationID))
	decision := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: id, ComputerID: w.fx.computerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotencyKey, RequestCommitment: computerevent.ZeroHead,
		TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID, PreviousHead: head.CanonicalEventHead,
		ParentEventID: operation.OperationID,
		ActorProfile:  agentprofile.Management, AuthorityRef: decisionpolicy.DecisionAuthorityRef(consensus), PrivacyClass: "owner",
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		RequireExpectedHead: true, PayloadCommitment: computerevent.ZeroHead, ProposedEffectRef: bundleDigest,
		DecisionRef: strings.Repeat("d", 64), VerifierRefs: append([]string(nil), operation.VerifierRefs...), ReducerVersion: computerevent.ReducerVersionV1,
		InputArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("f", 64), "artifact:sha256:" + consensus},
	}
	target, err := computerevent.CanonicalJSON(map[string]string{"base_head": operation.BaseHead, "bundle_digest": bundleDigest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.fx.appender.AppendNew(context.Background(), decision, computerevent.TransitionInput{TargetStateCommitment: computerevent.DigestBytes(target)}, nil); err != nil {
		t.Fatal(err)
	}
}

func (w *packageOpWorld) installs(t *testing.T) []store.PackageInstall {
	t.Helper()
	list, err := w.fx.rt.packageInstalls(context.Background(), "owner", w.fx.computerID)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func (w *packageOpWorld) pendingCleared(t *testing.T) {
	t.Helper()
	head := w.fx.waitForStableHead(t)
	if head.PendingTransitionRef != "" {
		t.Fatalf("the pending transition is not discharged: %+v", head)
	}
}

func TestPackageOperationPublishesInstallsAndRecordsWithNoOwnerAct(t *testing.T) {
	w := newPackageOpWorld(t, "apply")
	applyCalls := w.fx.applyCalls.Load()
	operation := w.propose(t, "op-pkg-apply")
	applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)

	if !applied.IsPackage() || applied.PackageReleaseID == "" || applied.PackageInstallID == "" || applied.ReleaseDigest != "" {
		t.Fatalf("applied package operation = %+v; want the release and install ids and no release digest", applied)
	}
	if w.pub.count() != 1 {
		t.Fatalf("publishes = %d, want exactly 1", w.pub.count())
	}
	request := w.pub.publishes[0]
	if request.OperationID != operation.OperationID || request.ClaimedPlatformBase != w.base || len(request.SourcePatch) == 0 || len(request.Media) != 1 ||
		!computerevent.IsSHA256(request.DecisionDigest) || len(request.TreePatch) == 0 || len(request.ReviewPatch) == 0 {
		t.Fatalf("publish request = %+v; want the persisted draft, the claimed base, the source patch, one screen and the decision digest", request)
	}
	if computerevent.DigestBytes(request.Media[0]) != computerevent.DigestBytes(mediaPNG(t, 64, 40, 3, false)) || request.ReleaseID != applied.PackageReleaseID {
		t.Fatalf("publish media or release id = %d bytes %s", len(request.Media[0]), request.ReleaseID)
	}
	installs := w.installs(t)
	if len(installs) != 1 || installs[0].InstallID != applied.PackageInstallID || installs[0].ReleaseID != applied.PackageReleaseID ||
		installs[0].Trigger != store.PackageInstallTriggerSelfdev || installs[0].Action != packageActionInstall {
		t.Fatalf("installs = %+v; want one selfdev install of %s", installs, applied.PackageReleaseID)
	}
	kinds := w.fx.countEventKinds()
	if kinds[computerevent.EventPackageApplied] != 1 || kinds[computerevent.EventMaterializationApplied] != 0 ||
		kinds[computerevent.EventCheckpointPublished] != 0 || kinds[computerevent.EventRouteProjectionUpdated] != 0 {
		t.Fatalf("events = %v; want one package_applied and no platform apply, checkpoint or route", kinds)
	}
	w.pendingCleared(t)
	if w.fx.applyCalls.Load() != applyCalls || w.fx.service.restarts != 0 {
		t.Fatalf("the updater was called (applies %d -> %d, restarts %d)", applyCalls, w.fx.applyCalls.Load(), w.fx.service.restarts)
	}
	// The draft was persisted once, keyed by the operation.
	if draft, found, err := w.fx.rt.loadPackageDraft(applied); err != nil || !found || draft.ReleaseID != applied.PackageReleaseID || draft.Package != newPackageName {
		t.Fatalf("persisted draft = %+v found=%v err=%v", draft, found, err)
	}
	// A second pass over the settled state does nothing.
	w.fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	if w.pub.count() != 1 || len(w.installs(t)) != 1 {
		t.Fatal("a later pass published or installed again")
	}
}

func TestPackageOperationRefusedBeforePublishPublishesNothing(t *testing.T) {
	cases := []struct {
		name   string
		inner  func(*packageOpWorld) PackageModuleBuilder
		reason string
	}{
		{"build", func(w *packageOpWorld) PackageModuleBuilder {
			return &fakeModuleBuilder{fail: errors.Join(pkgruntime.ErrBuild, errors.New("syntax error"))}
		}, "package_build_failed"},
		{"uses", func(w *packageOpWorld) PackageModuleBuilder {
			return &mutatingBuilder{inner: w.builder, mutate: func(m *pkgruntime.Module) {
				m.Uses = []string{"logout"}
				m.Manifest = json.RawMessage(`{"name":"demo-new"}`)
			}}
		}, pkgrelease.ReasonUsesExceedGrants},
		{"install review", func(w *packageOpWorld) PackageModuleBuilder {
			return &mutatingBuilder{inner: w.builder, mutate: func(m *pkgruntime.Module) {
				m.Manifest = json.RawMessage(`{"name":"demo-new","contributes":{"desks":[{"id":"d"}]}}`)
			}}
		}, "install_review_unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newPackageOpWorld(t, "refuse-"+strings.ReplaceAll(tc.name, " ", "-"))
			w.fx.configure(func(rt *Runtime) { rt.packageBuilder = tc.inner(w) })
			operation := w.propose(t, "op-pkg-refuse")
			failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
			if !strings.Contains(failed.TerminalError, "refused before publish") || !strings.Contains(failed.TerminalError, tc.reason) {
				t.Fatalf("terminal error = %q; want a refusal before publish naming %s", failed.TerminalError, tc.reason)
			}
			if w.pub.count() != 0 {
				t.Fatalf("a refused package was published %d time(s)", w.pub.count())
			}
			if len(w.installs(t)) != 0 {
				t.Fatalf("a refused package was installed: %+v", w.installs(t))
			}
			if kinds := w.fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 1 || kinds[computerevent.EventPackageApplied] != 0 {
				t.Fatalf("events = %v; want the failure discharged and no package_applied", kinds)
			}
			w.pendingCleared(t)
		})
	}
}

// The crash settle: a materializing package operation is settled from facts,
// by any later pass, never driven again.
func TestPackageOperationCrashSettlesFromFacts(t *testing.T) {
	unanswered := func(platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
		return platform.PackageReleaseView{}, 0, errors.New("connection reset by peer")
	}
	listDraftRelease := func(w *packageOpWorld) func(string) ([]PackageReleaseRef, error) {
		return func(operationID string) ([]PackageReleaseRef, error) {
			draft, found, err := w.fx.rt.loadPackageDraft(selfdev.Operation{OperationID: operationID, BundleDigest: w.bundleOf(t, operationID)})
			if err != nil || !found {
				return nil, errors.New("no draft")
			}
			return []PackageReleaseRef{{ReleaseID: draft.ReleaseID, OperationID: operationID}}, nil
		}
	}

	t.Run("nothing listed closes failed as interrupted", func(t *testing.T) {
		w := newPackageOpWorld(t, "settle-none")
		w.pub.publish = unanswered
		operation := w.propose(t, "op-pkg-settle-none")
		failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
		if failed.TerminalError != selfdevInterruptedByRestart || w.pub.count() != 1 || len(w.installs(t)) != 0 {
			t.Fatalf("operation = %+v publishes=%d; want failed %q after one publish", failed, w.pub.count(), selfdevInterruptedByRestart)
		}
		w.pendingCleared(t)
	})

	t.Run("listed with no install closes failed naming the release", func(t *testing.T) {
		w := newPackageOpWorld(t, "settle-listed")
		w.pub.publish = unanswered
		w.pub.list = listDraftRelease(w)
		operation := w.propose(t, "op-pkg-settle-listed")
		failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
		draft, _, _ := w.fx.rt.loadPackageDraft(failed)
		want := "published as " + draft.ReleaseID + "; not installed here; install it from the Library"
		if failed.TerminalError != want || failed.PackageReleaseID != "" {
			t.Fatalf("terminal error = %q release=%q; want %q", failed.TerminalError, failed.PackageReleaseID, want)
		}
		w.pendingCleared(t)
	})

	t.Run("listed with its install records package_applied", func(t *testing.T) {
		w := newPackageOpWorld(t, "settle-installed")
		w.pub.list = listDraftRelease(w)
		w.pub.publish = func(request platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
			// The release went public and the install landed, then the
			// computer lost the answer.
			id := request.ReleaseID
			if _, _, refusal := w.fx.rt.installPackage(context.Background(), "owner", w.fx.computerID, packageInstallRequest{
				ReleaseID: id, Record: request.Record, TreePatch: request.TreePatch, change: installChange{trigger: store.PackageInstallTriggerSelfdev},
			}); refusal != nil {
				t.Errorf("setup install: %v", refusal)
			}
			return platform.PackageReleaseView{}, 0, errors.New("connection reset by peer")
		}
		operation := w.propose(t, "op-pkg-settle-installed")
		applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
		if applied.PackageReleaseID == "" || applied.PackageInstallID != w.installs(t)[0].InstallID || len(w.installs(t)) != 1 {
			t.Fatalf("settled operation = %+v installs=%+v", applied, w.installs(t))
		}
		if kinds := w.fx.countEventKinds(); kinds[computerevent.EventPackageApplied] != 1 {
			t.Fatalf("events = %v; want one package_applied", kinds)
		}
		w.pendingCleared(t)
	})

	t.Run("a later pass settles a materializing operation and never publishes again", func(t *testing.T) {
		w := newPackageOpWorld(t, "settle-later")
		w.pub.publish = unanswered
		lookupFails := true
		var mu sync.Mutex
		w.pub.list = func(string) ([]PackageReleaseRef, error) {
			mu.Lock()
			defer mu.Unlock()
			if lookupFails {
				return nil, errors.New("the Library is unreachable")
			}
			return nil, nil
		}
		operation := w.propose(t, "op-pkg-settle-later")
		held := w.fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)
		time.Sleep(300 * time.Millisecond)
		if w.pub.count() != 1 {
			t.Fatalf("publishes = %d while the lookup is unproven, want 1", w.pub.count())
		}
		mu.Lock()
		lookupFails = false
		mu.Unlock()
		// A planned-restart pass or a crash-boot pass is the same pass: the
		// operation is materializing, so it is settled, not resumed.
		w.fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
		failed := w.fx.waitForState(t, held.OperationID, selfdev.StateFailed)
		if failed.TerminalError != selfdevInterruptedByRestart || w.pub.count() != 1 {
			t.Fatalf("operation = %+v publishes=%d; want failed as interrupted with no second publish", failed, w.pub.count())
		}
	})
}

// bundleOf reads an operation's bundle digest (the draft is keyed by it).
func (w *packageOpWorld) bundleOf(t *testing.T, operationID string) string {
	t.Helper()
	op, err := w.fx.operations.Get(context.Background(), w.fx.computerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	return op.BundleDigest
}

func (w *packageOpWorld) revertCall(t *testing.T, operationID string) (int, selfdev.Operation) {
	t.Helper()
	return w.fx.revert(t, operationID)
}

func TestPackageRevertUninstallsAndNeverCallsTheUpdater(t *testing.T) {
	w := newPackageOpWorld(t, "revert")
	operation := w.propose(t, "op-pkg-revert")
	applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	applyCalls, restarts := w.fx.applyCalls.Load(), w.fx.service.restarts

	code, reverted := w.revertCall(t, applied.OperationID)
	if code != http.StatusOK || reverted.State != selfdev.StateRolledBack || reverted.OperationID != applied.OperationID {
		t.Fatalf("revert status=%d operation=%+v; want the package row rolled back", code, reverted)
	}
	if w.fx.applyCalls.Load() != applyCalls || w.fx.service.restarts != restarts {
		t.Fatalf("the package revert touched the updater")
	}
	if ops := w.fx.rollbackOps(t); len(ops) != 1 || ops[0].OperationID != applied.OperationID {
		t.Fatalf("rollback-state operations = %+v; want only the reverted package row (no rollback row)", ops)
	}
	installs := w.installs(t)
	if len(installs) != 2 || installs[0].Action != packageActionUninstall || installs[0].ReleaseID != applied.PackageReleaseID || len(currentPackageInstalls(installs)) != 0 {
		t.Fatalf("installs = %+v; want the install followed by its uninstall", installs)
	}
	if kinds := w.fx.countEventKinds(); kinds[computerevent.EventPackageReverted] != 1 || kinds[computerevent.EventRollbackApplied] != 0 || kinds[computerevent.EventRollbackRequested] != 0 {
		t.Fatalf("events = %v; want one package_reverted and no rollback", kinds)
	}
	// A second tap is the same act; the install being gone is tolerated.
	code, again := w.revertCall(t, applied.OperationID)
	if code != http.StatusOK || again.OperationID != applied.OperationID || len(w.installs(t)) != 2 {
		t.Fatalf("second tap status=%d operation=%+v installs=%d", code, again, len(w.installs(t)))
	}
}

func TestPackageRevertToleratesAnInstallAlreadyGone(t *testing.T) {
	w := newPackageOpWorld(t, "revert-gone")
	operation := w.propose(t, "op-pkg-revert-gone")
	applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if _, _, refusal := w.fx.rt.uninstallPackage(context.Background(), "owner", w.fx.computerID, applied.PackageReleaseID); refusal != nil {
		t.Fatal(refusal)
	}
	if code, reverted := w.revertCall(t, applied.OperationID); code != http.StatusOK || reverted.State != selfdev.StateRolledBack {
		t.Fatalf("revert status=%d operation=%+v; want rolled back though the install was already uninstalled", code, reverted)
	}
}

func TestPlatformRevertPastAPackageOperationFindsThePlatformPrior(t *testing.T) {
	w := newPackageOpWorld(t, "past")
	first := w.fx.applyPlain(t, "op-past-1")
	pkg := w.fx.waitForState(t, w.propose(t, "op-past-pkg").OperationID, selfdev.StateApplied)
	second := w.fx.applyPlain(t, "op-past-2")
	w.fx.setBootedBuildCommit(t, joinTestBaseB)

	// The platform operation after the package accepted on the package's
	// event: its expected prior is not a platform state.
	accepted, found, err := w.fx.store.EventByDigest(context.Background(), w.fx.computerID, second.DecisionEvent)
	if err != nil || !found {
		t.Fatalf("accept event: found=%v err=%v", found, err)
	}
	if event, ok, err := w.fx.store.EventByDigest(context.Background(), w.fx.computerID, accepted.ExpectedEffectiveEventHead); err != nil || !ok || event.EventKind != computerevent.EventPackageApplied {
		t.Fatalf("setup: the platform operation's expected prior is %+v ok=%v err=%v; want the package_applied event", event.EventKind, ok, err)
	}

	code, rollback := w.fx.revert(t, second.OperationID)
	if code != http.StatusCreated || selfdev.IsBaseRestore(rollback) || rollback.BaseHead != first.EffectiveHead {
		t.Fatalf("revert status=%d operation=%+v; want the rollback to the first platform operation (%s)", code, rollback, first.EffectiveHead)
	}
	w.fx.requireRolledBackTo(t, rollback, first)
	// A platform rollback never undoes a package install.
	if current := currentPackageInstalls(w.installs(t)); len(current) != 1 || current[newPackageName].ReleaseID != pkg.PackageReleaseID {
		t.Fatalf("package installs after the platform revert = %+v; want the package still installed", current)
	}
	still, err := w.fx.operations.Get(context.Background(), w.fx.computerID, pkg.OperationID)
	if err != nil || still.State != selfdev.StateApplied {
		t.Fatalf("package operation after the platform revert = %+v err=%v; want applied", still, err)
	}
}

func TestPackageOperationsNeverEnterStackBookkeeping(t *testing.T) {
	w := newPackageOpWorld(t, "bookkeeping")
	first := w.fx.applyPlain(t, "op-book-1")
	pkg := w.fx.waitForState(t, w.propose(t, "op-book-pkg").OperationID, selfdev.StateApplied)
	ctx := context.Background()
	rt := w.fx.rt

	// The release readers match no package operation.
	for _, digest := range []string{pkg.ReleaseDigest, pkg.BundleDigest, first.ReleaseDigest, strings.Repeat("0", 64)} {
		if got, ok := rt.appliedOperationForRelease(ctx, digest); ok && got.OperationID == pkg.OperationID {
			t.Fatalf("appliedOperationForRelease(%q) returned the package operation", digest)
		}
	}
	// The rebase offer reads only applied platform operations' patches.
	if _, _, err := rt.rebaseOfferDeps().SourcePatch(ctx, pkg.OperationID); err == nil {
		t.Fatal("the rebase offer read a package operation's patch")
	}
	// A clean rebase of a package operation is refused.
	if retry, why := rt.cleanRebaseBeforeNotice(ctx, "owner", w.fx.computerID, pkg.OperationID, pkg.BaseHead, joinTestBaseB); retry || !strings.Contains(why, "package") {
		t.Fatalf("clean rebase of a package operation = retry=%v %q; want a refusal naming the layer", retry, why)
	}
	// The live platform set and the landed anchor are per layer.
	rows, err := w.fx.operations.ListByStates(ctx, w.fx.computerID, selfdev.StateApplied, selfdev.StateRolledBack)
	if err != nil {
		t.Fatal(err)
	}
	live := liveOperationSet(rows)
	if live[pkg.OperationID] || !live[first.OperationID] {
		t.Fatalf("live platform set = %v; want the first operation only", live)
	}
	h := &APIHandler{rt: rt}
	platformLatest, err := h.latestLandedOperation(ctx, w.fx.computerID, false)
	if err != nil || platformLatest == nil || platformLatest.OperationID != first.OperationID {
		t.Fatalf("latest landed platform operation = %+v err=%v; want the first operation", platformLatest, err)
	}
	packageLatest, err := h.latestLandedOperation(ctx, w.fx.computerID, true)
	if err != nil || packageLatest == nil || packageLatest.OperationID != pkg.OperationID {
		t.Fatalf("latest landed package operation = %+v err=%v; want the package operation", packageLatest, err)
	}
}

func TestStagedOperationKindClassifiesThePatch(t *testing.T) {
	dir := t.TempDir()
	repo, _ := packageSourceRepo(t)
	pkgPatch, _ := releasePatch(t, repo, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	platformPatch, _ := releasePatch(t, repo, map[string]string{"frontend/src/app.css": "body { color: red }\n"})
	mixedPatch, _ := releasePatch(t, repo, map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n", "frontend/src/app.css": "body { color: blue }\n"})
	for name, tc := range map[string]struct {
		patch []byte
		want  string
	}{"package": {pkgPatch, selfdev.KindPackage}, "platform": {platformPatch, ""}, "mixed": {mixedPatch, ""}} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(root+"/source.patch", tc.patch, 0o600); err != nil {
				t.Fatal(err)
			}
			if got := stagedOperationKind(root, computerevent.DigestBytes(tc.patch)); got != tc.want {
				t.Fatalf("kind = %q, want %q", got, tc.want)
			}
		})
	}
	if got := stagedOperationKind(dir, ""); got != "" {
		t.Fatalf("no source patch: kind = %q", got)
	}
}

// A refusal corpusd answers (no publisher address, an expired decision) closes
// the operation failed with its reason; nothing is installed.
func TestPackageOperationPublishRefusalClosesFailedWithTheReason(t *testing.T) {
	w := newPackageOpWorld(t, "refused-publish")
	w.pub.publish = func(platform.PackagePublishByDecisionRequest) (platform.PackageReleaseView, int, error) {
		return platform.PackageReleaseView{}, http.StatusConflict, &selfdev.PackagePublishError{Status: http.StatusConflict, Reason: "publisher_address_required", Message: "no publisher address"}
	}
	operation := w.propose(t, "op-pkg-publish-refused")
	failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, "publisher_address_required") || len(w.installs(t)) != 0 || w.pub.count() != 1 {
		t.Fatalf("operation = %+v installs=%d publishes=%d", failed, len(w.installs(t)), w.pub.count())
	}
	w.pendingCleared(t)
}
